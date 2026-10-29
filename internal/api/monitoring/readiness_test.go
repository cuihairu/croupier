package monitoring

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/cluster"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	gsqlite "gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newReadinessTestSvcCtx(t *testing.T, clusterRequired bool) *svc.ServiceContext {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	svcCtx := &svc.ServiceContext{
		DB:            db,
		RegistryStore: reg.NewStoreWithDB(db),
		OpsStateStore: svc.NewOpsStateStore(t.TempDir()),
	}
	if clusterRequired {
		svcCtx.ClusterReadiness = cluster.NewReadiness(5*time.Second, 15*time.Second)
	}
	return svcCtx
}

// TestClusterReadinessStatus 各集群就绪分支（ARCH-004 判定真值源）。
func TestClusterReadinessStatus(t *testing.T) {
	cases := []struct {
		name       string
		svcCtx     *svc.ServiceContext
		readiness  func(r *cluster.Readiness)
		wantReady  bool
		wantReason string
	}{
		{
			name:      "no cluster requirement",
			svcCtx:    &svc.ServiceContext{},
			wantReady: true,
		},
		{
			name:       "nil svc ctx",
			svcCtx:     nil,
			wantReady:  false,
			wantReason: "cluster: service context unavailable",
		},
		{
			name: "joined healthy",
			svcCtx: &svc.ServiceContext{
				ClusterReadiness: cluster.NewReadiness(5*time.Second, 15*time.Second),
			},
			readiness: func(r *cluster.Readiness) {
				r.SetJoined(true)
				r.RenewOK()
				r.SetInterconnect(true, nil)
			},
			wantReady: true,
		},
		{
			name: "joined but lease failing beyond window",
			svcCtx: &svc.ServiceContext{
				ClusterReadiness: cluster.NewReadiness(5*time.Second, 15*time.Second),
			},
			readiness: func(r *cluster.Readiness) {
				r.SetJoined(true)
				r.SetInterconnect(true, nil)
				for i := 0; i < 3; i++ {
					r.RenewFail(errors.New("coordination down"))
				}
			},
			wantReady:  false,
			wantReason: "cluster: member lease unavailable",
		},
		{
			name: "joined but interconnect down",
			svcCtx: &svc.ServiceContext{
				ClusterReadiness: cluster.NewReadiness(5*time.Second, 15*time.Second),
			},
			readiness: func(r *cluster.Readiness) {
				r.SetJoined(true)
				r.RenewOK()
				r.SetInterconnect(false, errors.New("address in use"))
			},
			wantReady:  false,
			wantReason: "cluster: interconnect listener unavailable",
		},
		{
			name: "startup degrade without knob",
			svcCtx: &svc.ServiceContext{
				ClusterReadiness: cluster.NewReadiness(5*time.Second, 15*time.Second),
			},
			readiness: func(r *cluster.Readiness) {
				r.SetJoined(false)
				r.RecordDegrade("register: connection refused")
			},
			wantReady:  false,
			wantReason: "cluster: coordination store unavailable: register: connection refused",
		},
		{
			name: "startup degrade with explicit standalone knob",
			svcCtx: &svc.ServiceContext{
				ClusterReadiness: cluster.NewReadiness(5*time.Second, 15*time.Second),
			},
			readiness: func(r *cluster.Readiness) {
				r.SetJoined(false)
				r.RecordDegrade("register: connection refused")
			},
			wantReady: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.name == "startup degrade with explicit standalone knob" {
				tc.svcCtx.Config.Cluster.AllowStandaloneDegrade = true
			}
			if tc.readiness != nil {
				tc.readiness(tc.svcCtx.ClusterReadiness)
			}
			ready, reasons, comp := clusterReadinessStatus(tc.svcCtx)
			if ready != tc.wantReady {
				t.Fatalf("ready = %v, want %v (reasons=%v comp=%v)", ready, tc.wantReady, reasons, comp)
			}
			if tc.wantReason != "" {
				joined := strings.Join(reasons, "; ")
				if !strings.Contains(joined, tc.wantReason) {
					t.Fatalf("reasons = %v, want containing %q", reasons, tc.wantReason)
				}
			}
		})
	}
}

// TestService_Ready_ClusterRequiredDegradedNoKnob 服务级端到端：组件健康
// 但集群要求未满足（未降级许可）→ not_ready。
func TestService_Ready_ClusterRequiredDegradedNoKnob(t *testing.T) {
	svcCtx := newReadinessTestSvcCtx(t, true)
	svcCtx.ClusterReadiness.SetJoined(false)
	svcCtx.ClusterReadiness.RecordDegrade("register: connection refused")

	resp, err := NewService(svcCtx).Ready(context.Background(), &ReadyRequest{})
	if err != nil {
		t.Fatalf("ready: %v", err)
	}
	if resp.Status != "not_ready" {
		t.Fatalf("status = %q, want not_ready", resp.Status)
	}
	found := false
	for _, r := range resp.Reasons {
		if strings.Contains(r, "coordination store unavailable") {
			found = true
		}
	}
	if !found {
		t.Fatalf("reasons = %v, want coordination failure", resp.Reasons)
	}
}

// TestService_Ready_ClusterHealthyAndDegradedWithKnob 全组件健康 + 集群
// 健康 → ok；显式降级许可下 standalone → ok。
func TestService_Ready_ClusterHealthyAndDegradedWithKnob(t *testing.T) {
	svcCtx := newReadinessTestSvcCtx(t, true)
	svcCtx.ClusterReadiness.SetJoined(true)
	svcCtx.ClusterReadiness.RenewOK()
	svcCtx.ClusterReadiness.SetInterconnect(true, nil)

	service := NewService(svcCtx)
	resp, err := service.Ready(context.Background(), &ReadyRequest{})
	if err != nil {
		t.Fatalf("ready: %v", err)
	}
	if resp.Status != "ok" {
		t.Fatalf("healthy cluster status = %q, want ok (reasons=%v)", resp.Status, resp.Reasons)
	}

	// 显式降级许可：standalone 也 ok。
	svcCtx.Config.Cluster.AllowStandaloneDegrade = true
	svcCtx.ClusterReadiness.SetJoined(false)
	svcCtx.ClusterReadiness.RecordDegrade("register: connection refused")
	resp, err = service.Ready(context.Background(), &ReadyRequest{})
	if err != nil {
		t.Fatalf("ready degraded: %v", err)
	}
	if resp.Status != "ok" {
		t.Fatalf("degraded-with-knob status = %q, want ok (reasons=%v)", resp.Status, resp.Reasons)
	}
}

// TestHandler_Ready_StatusMapping HTTP 层：ok → 200；not_ready → 503。
func TestHandler_Ready_StatusMapping(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svcCtx := newReadinessTestSvcCtx(t, true)
	svcCtx.ClusterReadiness.SetJoined(true)
	svcCtx.ClusterReadiness.RenewOK()
	svcCtx.ClusterReadiness.SetInterconnect(true, nil)
	h := NewHandler(NewService(svcCtx))

	r := gin.New()
	r.GET("/readyz", h.Ready)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("healthy readyz = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"status":"ok"`) {
		t.Fatalf("body = %s, want status ok", rec.Body.String())
	}

	svcCtx.ClusterReadiness.SetInterconnect(false, errors.New("listener dead"))
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("unhealthy readyz = %d, want 503 (body=%s)", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "not_ready") {
		t.Fatalf("body = %s, want not_ready", rec.Body.String())
	}
}
