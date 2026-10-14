package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"strconv"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	gosqlite "github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// newProbeLifecycleTestService：:memory: sqlite + Alert 表 + ControlService
// 装配生命周期（单连接防 :memory: 多实例库漂移）。
func newProbeLifecycleTestService(t *testing.T) (*ControlService, *model.AlertModel) {
	t.Helper()
	db, err := gorm.Open(gosqlite.Open("file:"+t.Name()+"?mode=memory&cache=private"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("sql db: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&model.Alert{}); err != nil {
		t.Fatalf("migrate alert: %v", err)
	}
	alertModel := model.NewAlertModel(db)
	svc := NewControlService(nil, nil)
	svc.SetLogger(slog.Default())
	svc.SetAlertModel(alertModel)
	svc.SetProbeLifecycleModel(alertModel)
	return svc, alertModel
}

// probeWindowEvent 完整窗口事件（含 window_kind 与时间轴）。
func probeWindowEvent(target, kind string, startTs, endTs, durationMs int64) *opsv1.AlertEvent {
	return &opsv1.AlertEvent{
		Kind:    "probe.window",
		AgentId: "agent-1",
		GameId:  "demo",
		Env:     "prod",
		Window: &opsv1.ProbeWindow{
			Target:        target,
			Semantics:     "readiness",
			WindowKind:    kind,
			StartTsUnixMs: startTs,
			EndTsUnixMs:   endTs,
			DurationMs:    durationMs,
			LastReason:    "connection refused",
		},
	}
}

// probeDetailEvent 失败明细事件（atTsUnixMs 必须 >= 窗口起点才会被计入序列）。
func probeDetailEvent(target, err string, atTs int64, open bool) *opsv1.AlertEvent {
	return &opsv1.AlertEvent{
		Kind:    "probe.detail",
		AgentId: "agent-1",
		GameId:  "demo",
		Env:     "prod",
		Detail: &opsv1.ProbeDetail{
			Target:              target,
			Semantics:           "readiness",
			Err:                 err,
			AtTsUnixMs:          atTs,
			LatencyMs:           2000,
			ConsecutiveFailures: 1,
			WindowOpen:          open,
			Details:             map[string]string{"probeType": "tcp"},
		},
	}
}

// jsonInt64 JSON 列回读数值形态随驱动浮动（int64/float64/json.Number），统一取毫秒比对。
func jsonInt64(t *testing.T, v interface{}) int64 {
	t.Helper()
	switch n := v.(type) {
	case int64:
		return n
	case float64:
		return int64(n)
	case string:
		p, err := strconv.ParseInt(n, 10, 64)
		if err != nil {
			t.Fatalf("json number unparsable = %v", v)
		}
		return p
	case json.Number:
		p, err := n.Int64()
		if err != nil {
			t.Fatalf("json number unparsable = %v", v)
		}
		return p
	default:
		t.Fatalf("json value type = %T", v)
	}
	return 0
}

// failuresOf 取 details.recentFailures 的明细条数与首条错误。
func failuresOf(t *testing.T, details map[string]interface{}) []map[string]interface{} {
	t.Helper()
	v, ok := details["recentFailures"].([]interface{})
	if !ok {
		t.Fatalf("recentFailures type = %T (%v)", details["recentFailures"], details["recentFailures"])
	}
	out := make([]map[string]interface{}, 0, len(v))
	for _, item := range v {
		m, ok := item.(map[string]interface{})
		if !ok {
			t.Fatalf("recentFailures entry type = %T", item)
		}
		out = append(out, m)
	}
	return out
}

func TestProbeAlertIDFormat(t *testing.T) {
	if got := probeAlertID("agent-1", "redis:6379"); got != "probe:agent-1:redis:6379" {
		t.Fatalf("probeAlertID = %q", got)
	}
}

func TestProbeLifecycleFiringThenResolved(t *testing.T) {
	svc, alertModel := newProbeLifecycleTestService(t)

	// 3 次失败明细（开窗前到达，诊断环先行）。
	for i, err := range []string{"refused", "timeout", "refused"} {
		if _, e := svc.handleProbeEvent(context.Background(), marshalAlertEvent(t, probeDetailEvent("redis:6379", err, int64(1000+i), true))); e != nil {
			t.Fatalf("detail: %v", e)
		}
	}

	// 开窗 → firing。
	if _, err := svc.handleProbeEvent(context.Background(), marshalAlertEvent(t, probeWindowEvent("redis:6379", "unavailable", 1000, 0, 0))); err != nil {
		t.Fatalf("window unavailable: %v", err)
	}
	row, err := alertModel.FindByAlertID(context.Background(), probeAlertID("agent-1", "redis:6379"))
	if err != nil {
		t.Fatalf("find alert: %v", err)
	}
	if row.Status != "firing" || row.Type != probeAlertKind || row.Source != probeAlertSource {
		t.Fatalf("row status/type/source = %s/%s/%s", row.Status, row.Type, row.Source)
	}
	if row.Level != "warning" {
		t.Fatalf("level = %q", row.Level)
	}
	if row.GameID != "demo" || row.Env != "prod" {
		t.Fatalf("scope = %s/%s (want demo/prod from event payload)", row.GameID, row.Env)
	}
	if row.Message != "探针目标 redis:6379 探测不可用（readiness）" {
		t.Fatalf("message = %q", row.Message)
	}
	failures := failuresOf(t, row.Details)
	if len(failures) != 3 {
		t.Fatalf("recentFailures = %d, want 3", len(failures))
	}
	if row.Details["startTsUnixMs"] == nil || jsonInt64(t, row.Details["startTsUnixMs"]) != 1000 {
		t.Fatalf("startTsUnixMs = %v", row.Details["startTsUnixMs"])
	}

	// 闭窗 → resolved（同一行）。
	if _, err := svc.handleProbeEvent(context.Background(), marshalAlertEvent(t, probeWindowEvent("redis:6379", "recovered", 1000, 13000, 12000))); err != nil {
		t.Fatalf("window recovered: %v", err)
	}
	row, err = alertModel.FindByAlertID(context.Background(), probeAlertID("agent-1", "redis:6379"))
	if err != nil {
		t.Fatalf("find alert after resolve: %v", err)
	}
	if row.Status != "resolved" {
		t.Fatalf("status = %q (want resolved)", row.Status)
	}
	if row.Message != "探针目标 redis:6379 恢复可用（故障持续 12000ms）" {
		t.Fatalf("message = %q", row.Message)
	}
	if jsonInt64(t, row.Details["durationMs"]) != 12000 {
		t.Fatalf("durationMs = %v", row.Details["durationMs"])
	}
	if got := failuresOf(t, row.Details); len(got) != 3 {
		t.Fatalf("resolved row must keep the failure sequence, got %d", len(got))
	}

	alerts, _, err := alertModel.List(context.Background(), model.ListAlertsOptions{PageSize: 10})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(alerts) != 1 {
		t.Fatalf("alerts = %d, want 1 row across the lifecycle", len(alerts))
	}
}

func TestProbeLifecycleReplayIsIdempotent(t *testing.T) {
	svc, alertModel := newProbeLifecycleTestService(t)

	payload := marshalAlertEvent(t, probeWindowEvent("redis:6379", "unavailable", 1000, 0, 0))
	for i := 0; i < 3; i++ {
		if _, err := svc.handleProbeEvent(context.Background(), payload); err != nil {
			t.Fatalf("replay %d: %v", i, err)
		}
	}
	alerts, _, err := alertModel.List(context.Background(), model.ListAlertsOptions{PageSize: 10})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(alerts) != 1 {
		t.Fatalf("replays created %d rows, want 1 (重连重放不得重复落库)", len(alerts))
	}
	if alerts[0].Status != "firing" {
		t.Fatalf("status = %q", alerts[0].Status)
	}
}

func TestProbeLifecycleRefireAfterResolve(t *testing.T) {
	svc, alertModel := newProbeLifecycleTestService(t)

	fire := func(startTs, endTs, durationMs int64, kind string) {
		t.Helper()
		if _, err := svc.handleProbeEvent(context.Background(), marshalAlertEvent(t, probeWindowEvent("redis:6379", kind, startTs, endTs, durationMs))); err != nil {
			t.Fatalf("window %s: %v", kind, err)
		}
	}

	fire(1000, 0, 0, "unavailable")
	fire(1000, 2000, 1000, "recovered")
	// 第二个故障窗口：resolved → firing 复活，不产生新行。
	fire(3000, 0, 0, "unavailable")

	alerts, _, err := alertModel.List(context.Background(), model.ListAlertsOptions{PageSize: 10})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(alerts) != 1 {
		t.Fatalf("alerts = %d, want 1 row across two windows", len(alerts))
	}
	if alerts[0].Status != "firing" {
		t.Fatalf("refire status = %q", alerts[0].Status)
	}
	if jsonInt64(t, alerts[0].Details["startTsUnixMs"]) != 3000 {
		t.Fatalf("details must refresh to the latest window, startTs = %v", alerts[0].Details["startTsUnixMs"])
	}
}

func TestProbeLifecycleResolvedWithoutObservedOpen(t *testing.T) {
	svc, alertModel := newProbeLifecycleTestService(t)

	// server 重启错过开窗事件，只收到闭窗：事实优先，仍落 resolved 行。
	if _, err := svc.handleProbeEvent(context.Background(), marshalAlertEvent(t, probeWindowEvent("redis:6379", "recovered", 1000, 2000, 1000))); err != nil {
		t.Fatalf("recovered: %v", err)
	}
	row, err := alertModel.FindByAlertID(context.Background(), probeAlertID("agent-1", "redis:6379"))
	if err != nil {
		t.Fatalf("find alert: %v", err)
	}
	if row.Status != "resolved" {
		t.Fatalf("status = %q (want resolved)", row.Status)
	}
}

func TestProbeLifecycleRecentFailuresWindowScoped(t *testing.T) {
	svc, alertModel := newProbeLifecycleTestService(t)

	// 上一个窗口的明细（at < 本窗口起点）不得串台。
	if _, err := svc.handleProbeEvent(context.Background(), marshalAlertEvent(t, probeDetailEvent("redis:6379", "stale", 500, true))); err != nil {
		t.Fatalf("stale detail: %v", err)
	}
	// 本窗口 7 条明细 → 上限 5 条（取最近 5，时序正序）。
	for i := 0; i < 7; i++ {
		if _, err := svc.handleProbeEvent(context.Background(), marshalAlertEvent(t, probeDetailEvent("redis:6379", "err"+strconv.Itoa(i), int64(1000+i*10), true))); err != nil {
			t.Fatalf("detail %d: %v", i, err)
		}
	}
	// 其他 target 的明细不混入。
	if _, err := svc.handleProbeEvent(context.Background(), marshalAlertEvent(t, probeDetailEvent("mysql:3306", "other", 1100, true))); err != nil {
		t.Fatalf("other target detail: %v", err)
	}

	if _, err := svc.handleProbeEvent(context.Background(), marshalAlertEvent(t, probeWindowEvent("redis:6379", "unavailable", 1000, 0, 0))); err != nil {
		t.Fatalf("window: %v", err)
	}
	row, err := alertModel.FindByAlertID(context.Background(), probeAlertID("agent-1", "redis:6379"))
	if err != nil {
		t.Fatalf("find alert: %v", err)
	}
	failures := failuresOf(t, row.Details)
	if len(failures) != probeWindowRecentFailures {
		t.Fatalf("recentFailures = %d, want cap %d", len(failures), probeWindowRecentFailures)
	}
	if err := failures[len(failures)-1]["err"].(string); err != "err6" {
		t.Fatalf("last failure err = %q (want the most recent)", err)
	}
	for _, f := range failures {
		if at := jsonInt64(t, f["atTsUnixMs"]); at < 1000 {
			t.Fatalf("stale failure leaked into the window: atTs = %d", at)
		}
		if f["target"] != "redis:6379" {
			t.Fatalf("other-target failure leaked: %v", f["target"])
		}
	}
}

func TestProbeLifecycleNilSafe(t *testing.T) {
	// 未装配 alertModel / lifecycle：事件仍接收（诊断环留证），不落库不 panic。
	svc := newTestControlService()
	if _, err := svc.handleProbeEvent(context.Background(), marshalAlertEvent(t, probeWindowEvent("redis:6379", "unavailable", 1000, 0, 0))); err != nil {
		t.Fatalf("event must still be accepted without lifecycle: %v", err)
	}
	if got := svc.ProbeEventSink().recent("agent-1", 0); len(got) != 1 {
		t.Fatalf("sink entries = %d, want 1", len(got))
	}

	var lifecycle *probeLifecycle
	ctx := context.Background()
	lifecycle.onWindow(ctx, "agent-1", probeWindowEvent("redis:6379", "unavailable", 1000, 0, 0)) // 不 panic
	lifecycle.onWindow(ctx, "agent-1", &opsv1.AlertEvent{})
}

func TestProbeLifecycleRejectsBadWindowKind(t *testing.T) {
	svc, alertModel := newProbeLifecycleTestService(t)

	if _, err := svc.handleProbeEvent(context.Background(), marshalAlertEvent(t, probeWindowEvent("redis:6379", "exploded", 1000, 0, 0))); err != nil {
		t.Fatalf("handler: %v", err)
	}
	alerts, _, err := alertModel.List(context.Background(), model.ListAlertsOptions{PageSize: 10})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(alerts) != 0 {
		t.Fatalf("unsupported window_kind created %d alert rows", len(alerts))
	}
}

func TestProbeLifecycleNilSinkKeepsDetails(t *testing.T) {
	// sink 为 nil 时 details 仍含窗口本体字段（明细序列缺省）。
	lifecycle := newProbeLifecycle(&model.AlertModel{}, nil, nil)
	details := lifecycle.buildDetails("agent-1", probeWindowEvent("redis:6379", "unavailable", 1000, 0, 0))
	if details["recentFailures"] != nil {
		t.Fatalf("recentFailures = %v, want nil without sink", details["recentFailures"])
	}
	if details["target"] != "redis:6379" || details["windowKind"] != "unavailable" {
		t.Fatalf("window details = %v", details)
	}
}

// 空 target 与空 window_kind 边界：不建行、不 panic。
func TestProbeLifecycleEmptyTargetAndNilWindow(t *testing.T) {
	lifecycle := newProbeLifecycle(&model.AlertModel{}, nil, nil)
	lifecycle.onWindow(context.Background(), "agent-1", &opsv1.AlertEvent{Window: &opsv1.ProbeWindow{Target: " "}})
	lifecycle.onWindow(context.Background(), "agent-1", probeWindowEvent("redis:6379", "", 1000, 0, 0))
}
