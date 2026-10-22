package outlet

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// heraldDispatchRecorder 模拟 herald dispatch face（§13.3 wire）。
type heraldDispatchRecorder struct {
	gotAuth  string
	gotPath  string
	gotBody  map[string]any
	status   int
	response string
}

func (r *heraldDispatchRecorder) serve(w http.ResponseWriter, req *http.Request) {
	r.gotAuth = req.Header.Get("Authorization")
	r.gotPath = req.URL.Path
	raw, _ := io.ReadAll(req.Body)
	_ = json.Unmarshal(raw, &r.gotBody)
	if r.status != 0 && r.status != http.StatusOK {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(r.status)
		_, _ = w.Write([]byte(r.response))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":{"notification_id":"n1","category":"agent-alerts","urgency":"critical"}}`))
}

func newHeraldTestOutlet(t *testing.T, rec *heraldDispatchRecorder) *HeraldOutlet {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(rec.serve))
	t.Cleanup(srv.Close)
	return NewHeraldOutlet(srv.URL, "croupier", "tok-1", "group:gm-ops")
}

func TestHeraldOutletDeliverMapsEnvelope(t *testing.T) {
	rec := &heraldDispatchRecorder{}
	o := newHeraldTestOutlet(t, rec)

	ev := sampleEvent()
	outcome, err := o.Deliver(context.Background(), ev)
	if err != nil {
		t.Fatalf("Deliver: %v", err)
	}
	if !outcome.Delivered || outcome.Channel != HeraldCategoryAgentAlerts {
		t.Fatalf("outcome = %+v, want delivered agent-alerts", outcome)
	}
	if rec.gotPath != "/api/v1/apps/croupier/dispatch" {
		t.Fatalf("path = %s", rec.gotPath)
	}
	if rec.gotAuth != "Bearer tok-1" {
		t.Fatalf("auth = %s", rec.gotAuth)
	}
	if rec.gotBody["category"] != HeraldCategoryAgentAlerts {
		t.Fatalf("category = %v, want agent-alerts", rec.gotBody["category"])
	}
	if rec.gotBody["urgency"] != "critical" {
		t.Fatalf("urgency = %v, want critical", rec.gotBody["urgency"])
	}
	aud, ok := rec.gotBody["audiences"].([]any)
	if !ok || len(aud) != 1 || aud[0] != "group:gm-ops" {
		t.Fatalf("audiences = %v, want [group:gm-ops]", rec.gotBody["audiences"])
	}
	if rec.gotBody["event_id"] != "agent-1:42" {
		t.Fatalf("event_id = %v", rec.gotBody["event_id"])
	}
	if rec.gotBody["dedup_key"] != "supervisor.breaker_tripped:agent-1:42" {
		t.Fatalf("dedup_key = %v", rec.gotBody["dedup_key"])
	}
	if rec.gotBody["title"] != "breaker tripped" || rec.gotBody["body"] != "too many restart failures" {
		t.Fatalf("title/body = %v / %v", rec.gotBody["title"], rec.gotBody["body"])
	}
}

// ev.Target 覆盖默认受众（报表分片按类别 leader 定向，incident-reports §6）。
func TestHeraldOutletDeliverHonorsTarget(t *testing.T) {
	rec := &heraldDispatchRecorder{}
	o := newHeraldTestOutlet(t, rec)

	ev := sampleEvent()
	ev.Target = "category-leader:qa"
	if _, err := o.Deliver(context.Background(), ev); err != nil {
		t.Fatalf("Deliver: %v", err)
	}
	aud, ok := rec.gotBody["audiences"].([]any)
	if !ok || len(aud) != 1 || aud[0] != "category-leader:qa" {
		t.Fatalf("audiences = %v, want [category-leader:qa]", rec.gotBody["audiences"])
	}
}

func TestCategoryForKindClosedPrefixMap(t *testing.T) {
	// probe.* → availability；其余闭集 kind → agent-alerts。
	probes := []string{KindProbeUnavailable, KindProbeRecovered}
	for _, k := range probes {
		if got := categoryForKind(k); got != HeraldCategoryAvailability {
			t.Fatalf("categoryForKind(%s) = %s, want availability", k, got)
		}
	}
	agents := []string{KindCaptureRuleHit, KindDevopsBuildFailed, KindSupervisorBreaker, KindAlertInbound}
	for _, k := range agents {
		if got := categoryForKind(k); got != HeraldCategoryAgentAlerts {
			t.Fatalf("categoryForKind(%s) = %s, want agent-alerts", k, got)
		}
	}
	// 未知 kind 兜底 agent-alerts，不静默造新品类。
	if got := categoryForKind("unknown.kind"); got != HeraldCategoryAgentAlerts {
		t.Fatalf("unknown kind category = %s, want agent-alerts fallback", got)
	}
}

func TestUrgencyForSeverityClosedMap(t *testing.T) {
	cases := map[string]string{
		SeverityCritical: "critical",
		SeverityWarning:  "urgent",
		SeverityInfo:     "normal",
		"":               "normal", // 空档兜底 normal
		"weird":          "normal", // 闭集外兜底 normal
	}
	for sev, want := range cases {
		if got := urgencyForSeverity(sev); got != want {
			t.Fatalf("urgencyForSeverity(%q) = %q, want %q", sev, got, want)
		}
	}
}

func TestHeraldOutletClassifiesRefusalAsPermanent(t *testing.T) {
	rec := &heraldDispatchRecorder{
		status:   http.StatusUnprocessableEntity,
		response: `{"code":1,"message":"category agent-alerts is not registered in this namespace"}`,
	}
	o := newHeraldTestOutlet(t, rec)

	outcome, err := o.Deliver(context.Background(), sampleEvent())
	if err == nil {
		t.Fatal("want error on 422")
	}
	if outcome.Delivered || outcome.Retryable {
		t.Fatalf("outcome = %+v, want undelivered non-retryable", outcome)
	}
	if !isPermanent(err) {
		t.Fatalf("refusal must be Permanent, got %v", err)
	}
	if !strings.Contains(err.Error(), "herald: 422") {
		t.Fatalf("error text = %v", err)
	}
}

func TestHeraldOutletClassifiesTransportAsRetryable(t *testing.T) {
	// 关闭的服务器 → 网络层错误 → 可重试（非 Permanent）。
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	srv.Close()
	o := NewHeraldOutlet(srv.URL, "croupier", "tok-1", "group:gm-ops")

	outcome, err := o.Deliver(context.Background(), sampleEvent())
	if err == nil {
		t.Fatal("want error on closed server")
	}
	if !outcome.Retryable {
		t.Fatalf("outcome = %+v, want retryable", outcome)
	}
	if isPermanent(err) {
		t.Fatalf("transport error must stay retryable, got %v", err)
	}
}

func TestNewHeraldOutletDefaultsAndOverrides(t *testing.T) {
	// app/target 留空取内置默认。
	o := NewHeraldOutlet("http://herald:8080", "", "tok", "")
	if o.Name() != "herald" {
		t.Fatalf("Name = %s", o.Name())
	}
	if o.client == nil {
		t.Fatal("client must be built")
	}
	// 默认值经常量锁定，防静默漂移。
	if HeraldDefaultApp != "croupier" || HeraldDefaultTokenEnv != "HERALD_TRIGGER_TOKEN" || HeraldDefaultTarget != "group:gm-ops" {
		t.Fatalf("defaults drifted: app=%s tokenEnv=%s target=%s", HeraldDefaultApp, HeraldDefaultTokenEnv, HeraldDefaultTarget)
	}
	// 显式命名空间覆盖默认（herald 侧可播种多命名空间）。
	o2 := NewHeraldOutlet("http://herald:8080", "ns-x", "tok", "group:ops")
	if o2.client == nil || o2.target != "group:ops" {
		t.Fatalf("override lost: target=%s", o2.target)
	}
}
