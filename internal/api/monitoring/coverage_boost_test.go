package monitoring

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
)

// 死分支实证：Healthz/Metrics/Status 在空 svcCtx、有 store、全配置三种
// 构造下均恒返回 nil error，handler.go 三个 err != nil 分支不可达。
func TestService_Methods_NeverReturnError_Matrix(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	store := registry.NewStore()
	require.NoError(t, store.UpsertAgent(&registry.AgentSession{
		AgentID:   "boost-agent",
		GameID:    "game1",
		Env:       "dev",
		Addr:      "127.0.0.1:19091",
		ExpireAt:  time.Now().Add(time.Minute),
		Functions: map[string]registry.FunctionMeta{"f.one": {Enabled: true}},
	}))

	cases := []struct {
		name   string
		svcCtx *svc.ServiceContext
	}{
		{"all stores nil", &svc.ServiceContext{}},
		{"registry only", &svc.ServiceContext{RegistryStore: store}},
		{"fully configured", &svc.ServiceContext{
			DB:            db,
			RegistryStore: store,
			OpsStateStore: svc.NewOpsStateStore(t.TempDir()),
			Config:        config.Config{Database: config.DatabaseConfig{Driver: "sqlite"}},
		}},
	}
	ctx := context.Background()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := NewService(tc.svcCtx)

			healthResp, err := s.Healthz(ctx, &HealthzRequest{})
			assert.NoError(t, err)
			assert.NotNil(t, healthResp)

			metricsResp, err := s.Metrics(ctx, &MetricsRequest{})
			assert.NoError(t, err)
			assert.NotNil(t, metricsResp)

			statusResp, err := s.Status(ctx, &StatusRequest{})
			assert.NoError(t, err)
			assert.NotNil(t, statusResp)
		})
	}
}

// HTTP 层实证：三个端点在空 svcCtx（所有组件不健康）下仍走成功响应路径返回 200，
// 佐证 handler 的 error 分支不可达（Service 恒 nil error）。
func TestHandler_Endpoints_AllComponentsDown(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandler(NewService(&svc.ServiceContext{}))

	r := gin.New()
	r.GET("/healthz", h.Healthz)
	r.GET("/metrics", h.Metrics)
	r.GET("/status", h.Status)

	for _, path := range []string{"/healthz", "/metrics", "/status"} {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		assert.Equal(t, http.StatusOK, rec.Code, "path %s", path)
	}
}

// Collect 在底层连接关闭（Exec 失败）时 db_up 必须为 0 且不 panic。
func TestPlatformCollector_CollectWithBrokenDB(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	collector := newPlatformCollector(&svc.ServiceContext{
		DB:            db,
		RegistryStore: registry.NewStore(),
		Config:        config.Config{Database: config.DatabaseConfig{Driver: "sqlite"}},
	})

	ch := make(chan prometheus.Metric, 8)
	collector.Collect(ch)
	close(ch)

	dbUpDesc := prometheus.NewDesc("croupier_db_up", "Database connectivity (1=up 0=down)", nil, nil).String()
	foundDown := false
	for m := range ch {
		pb := &dto.Metric{}
		require.NoError(t, m.Write(pb))
		if pb.GetGauge() != nil && m.Desc().String() == dbUpDesc {
			foundDown = pb.GetGauge().GetValue() == 0
		}
	}
	assert.True(t, foundDown, "db_up should be 0 when Exec fails")
}

// 完整 exposition 抓取：注册 agent 后平台指标输出真实统计值。
func TestNewPrometheusHandler_GatherWithAgents(t *testing.T) {
	store := registry.NewStore()
	require.NoError(t, store.UpsertAgent(&registry.AgentSession{
		AgentID:   "expo-agent",
		GameID:    "game1",
		Env:       "dev",
		Addr:      "127.0.0.1:19091",
		ExpireAt:  time.Now().Add(time.Minute),
		Functions: map[string]registry.FunctionMeta{"a.b": {Enabled: true}},
	}))

	handler := NewPrometheusHandler(&svc.ServiceContext{RegistryStore: store})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	require.Equal(t, http.StatusOK, rec.Code)

	body := rec.Body.String()
	assert.Contains(t, body, "croupier_agents_total 1")
	assert.Contains(t, body, "croupier_agents_healthy 1")
	assert.Contains(t, body, "croupier_functions_registered 1")
	assert.Contains(t, body, "croupier_db_up")
}

// Collect 并发调用（registry 与 DB 状态读取互不干扰）。
func TestPlatformCollector_CollectConcurrent(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	collector := newPlatformCollector(&svc.ServiceContext{
		DB:            db,
		RegistryStore: registry.NewStore(),
		Config:        config.Config{Database: config.DatabaseConfig{Driver: "sqlite"}},
	})

	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ch := make(chan prometheus.Metric, 8)
			collector.Collect(ch)
			close(ch)
			count := 0
			for range ch {
				count++
			}
			assert.GreaterOrEqual(t, count, 1, "collector must emit db_up at minimum")
		}()
	}
	wg.Wait()
}

// failingMonitoringService 经 serviceAPI 缝隙注入故障：三个端点必须以
// 500 internal_error 响应并携带注入的错误消息。
type failingMonitoringService struct {
	method string
	err    error
}

func (f failingMonitoringService) Healthz(_ context.Context, _ *HealthzRequest) (*HealthzResponse, error) {
	return nil, f.err
}

func (f failingMonitoringService) Metrics(_ context.Context, _ *MetricsRequest) (*MetricsResponse, error) {
	return nil, f.err
}

func (f failingMonitoringService) Status(_ context.Context, _ *StatusRequest) (*StatusResponse, error) {
	return nil, f.err
}

func TestHandler_ServiceErrorBranches(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &Handler{service: failingMonitoringService{err: assert.AnError}}

	r := gin.New()
	r.GET("/healthz", h.Healthz)
	r.GET("/metrics", h.Metrics)
	r.GET("/status", h.Status)

	for _, path := range []string{"/healthz", "/metrics", "/status"} {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		assert.Equal(t, http.StatusInternalServerError, rec.Code, "path %s", path)
		body := rec.Body.String()
		assert.Contains(t, body, "internal_error", "path %s", path)
		assert.Contains(t, body, assert.AnError.Error(), "path %s", path)
	}
}

// checkDatabaseHealth 的 latencyMs 恒为 int64（time.Duration.Milliseconds），
// Collect 的 float64 分支仅在缝隙注入时可触达：延迟必须以原值暴露。
func TestPlatformCollector_CollectFloat64Latency(t *testing.T) {
	prev := collectDBStatusFn
	collectDBStatusFn = func(_ context.Context, _ *svc.ServiceContext) map[string]interface{} {
		return map[string]interface{}{"ok": true, "latencyMs": 12.5, "driver": "sqlite"}
	}
	t.Cleanup(func() { collectDBStatusFn = prev })

	collector := newPlatformCollector(&svc.ServiceContext{})
	ch := make(chan prometheus.Metric, 8)
	collector.Collect(ch)
	close(ch)

	latencyDesc := prometheus.NewDesc("croupier_db_latency_ms", "Database ping latency in milliseconds", nil, nil).String()
	found := false
	for m := range ch {
		pb := &dto.Metric{}
		require.NoError(t, m.Write(pb))
		if pb.GetGauge() != nil && m.Desc().String() == latencyDesc {
			found = true
			assert.InDelta(t, 12.5, pb.GetGauge().GetValue(), 1e-9)
		}
	}
	assert.True(t, found, "croupier_db_latency_ms must be emitted for float64 latencyMs")
}
