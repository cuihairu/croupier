package server

import (
	"context"
	"encoding/json"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/outlet"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	gosqlite "github.com/glebarez/sqlite"
	"google.golang.org/protobuf/proto"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// newAlertTestService：:memory: sqlite + Alert 表 + 注入 ControlService
// （单连接防 :memory: 多实例库漂移）。
func newAlertTestService(t *testing.T) (*ControlService, *model.AlertModel) {
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
	svc.SetAlertModel(alertModel)
	return svc, alertModel
}

func mustMarshalHit(t *testing.T, req *opsv1.CaptureRuleHitReport) []byte {
	t.Helper()
	data, err := proto.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return data
}

func TestHandleAlertEvent_PersistsIdempotently(t *testing.T) {
	svc, alertModel := newAlertTestService(t)
	req := &opsv1.CaptureRuleHitReport{
		AgentId:       "capture-01",
		GameId:        "demo",
		Env:           "prod",
		Rule:          "item-ledger",
		Severity:      "warn",
		Table:         "game.items",
		Key:           "42",
		Detail:        "no ledger credit within 300s",
		Gtid:          "1-2-33",
		EventOp:       "update",
		EventTsUnixMs: 1760000000000,
	}
	data := mustMarshalHit(t, req)

	ack := &opsv1.CaptureRuleHitAck{}
	respData, err := svc.handleAlertEvent(context.Background(), data)
	if err != nil {
		t.Fatalf("handleAlertEvent: %v", err)
	}
	if err := proto.Unmarshal(respData, ack); err != nil {
		t.Fatalf("unmarshal ack: %v", err)
	}
	if !ack.GetAccepted() || ack.GetDuplicate() {
		t.Fatalf("ack = %+v, want accepted non-duplicate", ack)
	}

	// 同输入重推（重连重放）→ duplicate 折叠。
	respData2, err := svc.handleAlertEvent(context.Background(), data)
	if err != nil {
		t.Fatalf("handleAlertEvent replay: %v", err)
	}
	ack2 := &opsv1.CaptureRuleHitAck{}
	if err := proto.Unmarshal(respData2, ack2); err != nil {
		t.Fatalf("unmarshal ack2: %v", err)
	}
	if !ack2.GetAccepted() || !ack2.GetDuplicate() {
		t.Fatalf("ack2 = %+v, want duplicate", ack2)
	}

	row, err := alertModel.FindByAlertID(context.Background(), captureAlertID("capture-01", "item-ledger", "1-2-33", "game.items", "42", 1760000000000))
	if err != nil {
		t.Fatalf("find alert: %v", err)
	}
	if row.Type != outlet.KindCaptureRuleHit || row.Level != outlet.SeverityWarning {
		t.Fatalf("alert type/level = %s/%s", row.Type, row.Level)
	}
	if row.Source != "capture" || row.Status != "firing" {
		t.Fatalf("alert source/status = %s/%s", row.Source, row.Status)
	}
	if row.Message != "[item-ledger] game.items: no ledger credit within 300s" {
		t.Fatalf("alert message = %q", row.Message)
	}
	details := row.Details
	if details["rule"] != "item-ledger" || details["eventOp"] != "update" {
		t.Fatalf("details = %+v", details)
	}
	scope, ok := details["scope"].(map[string]interface{})
	if !ok || scope["gameId"] != "demo" || scope["env"] != "prod" || scope["agentId"] != "capture-01" {
		t.Fatalf("scope = %+v", details["scope"])
	}
}

func TestHandleAlertEvent_SeverityMapping(t *testing.T) {
	svc, alertModel := newAlertTestService(t)

	critical := &opsv1.CaptureRuleHitReport{
		AgentId: "capture-01", Rule: "r-crit", Severity: "critical",
		Table: "game.player_wallet", EventTsUnixMs: 1760000001000,
	}
	if _, err := svc.handleAlertEvent(context.Background(), mustMarshalHit(t, critical)); err != nil {
		t.Fatalf("critical: %v", err)
	}
	unknown := &opsv1.CaptureRuleHitReport{
		AgentId: "capture-01", Rule: "r-bogus", Severity: "bogus",
		Table: "game.player_wallet", EventTsUnixMs: 1760000002000,
	}
	if _, err := svc.handleAlertEvent(context.Background(), mustMarshalHit(t, unknown)); err != nil {
		t.Fatalf("unknown severity: %v", err)
	}

	alerts, _, err := alertModel.List(context.Background(), model.ListAlertsOptions{PageSize: 100})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	levels := map[string]string{}
	for i := range alerts {
		if rule, ok := alerts[i].Details["rule"].(string); ok {
			levels[rule] = alerts[i].Level
		}
	}
	if levels["r-crit"] != outlet.SeverityCritical {
		t.Fatalf("r-crit level = %q", levels["r-crit"])
	}
	if levels["r-bogus"] != outlet.SeverityWarning {
		t.Fatalf("r-bogus level = %q (want defaulted warning)", levels["r-bogus"])
	}
}

func TestHandleAlertEvent_ValidationAndNilModel(t *testing.T) {
	svc, _ := newAlertTestService(t)

	// 空载荷 → unmarshal 或校验失败。
	if _, err := svc.handleAlertEvent(context.Background(), []byte("not-proto")); err == nil {
		t.Fatal("garbage payload accepted")
	}
	missingTable := &opsv1.CaptureRuleHitReport{Rule: "r", Table: ""}
	if _, err := svc.handleAlertEvent(context.Background(), mustMarshalHit(t, missingTable)); err == nil {
		t.Fatal("missing table accepted")
	}
	missingRule := &opsv1.CaptureRuleHitReport{Rule: "", Table: "game.items"}
	if _, err := svc.handleAlertEvent(context.Background(), mustMarshalHit(t, missingRule)); err == nil {
		t.Fatal("missing rule accepted")
	}

	// 未配置 alertModel → 显式错误（同 task store 语义）。
	bare := NewControlService(nil, nil)
	req := &opsv1.CaptureRuleHitReport{Rule: "r", Table: "game.items"}
	if _, err := bare.handleAlertEvent(context.Background(), mustMarshalHit(t, req)); err == nil {
		t.Fatal("nil alert model accepted")
	}
}

func TestHandleAlertEvent_OccurredAtDefaults(t *testing.T) {
	svc, alertModel := newAlertTestService(t)
	req := &opsv1.CaptureRuleHitReport{Rule: "r", Table: "t", Severity: "warn"}
	if _, err := svc.handleAlertEvent(context.Background(), mustMarshalHit(t, req)); err != nil {
		t.Fatalf("handleAlertEvent: %v", err)
	}
	alerts, _, err := alertModel.List(context.Background(), model.ListAlertsOptions{PageSize: 10})
	if err != nil || len(alerts) != 1 {
		t.Fatalf("list = %d, %v", len(alerts), err)
	}
	// JSON 列回读数值形态随驱动浮动（int64/float64/json.Number），统一取毫秒比对。
	var occ int64
	switch v := alerts[0].Details["occurredAtUnixMs"].(type) {
	case int64:
		occ = v
	case float64:
		occ = int64(v)
	case string:
		parsed, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			t.Fatalf("occurredAtUnixMs unparsable = %v", v)
		}
		occ = parsed
	case json.Number:
		parsed, err := v.Int64()
		if err != nil {
			t.Fatalf("occurredAtUnixMs unparsable = %v", v)
		}
		occ = parsed
	default:
		t.Fatalf("occurredAtUnixMs type = %T", alerts[0].Details["occurredAtUnixMs"])
	}
	if occ <= 0 || time.Now().UnixMilli()-occ > 60_000 {
		t.Fatalf("occurredAtUnixMs = %d, want defaulted to now", occ)
	}
}

// recordingOutlet 捕获 Dispatch 事件（Manager 扇出异步，测试轮询等待）。
type recordingOutlet struct {
	mu     sync.Mutex
	events []outlet.AlertEvent
}

func (o *recordingOutlet) Name() string { return "recording" }

func (o *recordingOutlet) Deliver(_ context.Context, ev outlet.AlertEvent) (outlet.DeliveryOutcome, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.events = append(o.events, ev)
	return outlet.DeliveryOutcome{Delivered: true, Channel: "recording"}, nil
}

func (o *recordingOutlet) waitEvent(t *testing.T, kind string) outlet.AlertEvent {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		o.mu.Lock()
		for _, ev := range o.events {
			if ev.Kind == kind {
				o.mu.Unlock()
				return ev
			}
		}
		o.mu.Unlock()
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("no %s event dispatched within timeout", kind)
	return outlet.AlertEvent{}
}

func TestHandleAlertEvent_DispatchesOutlet(t *testing.T) {
	svc, _ := newAlertTestService(t)
	out := &recordingOutlet{}
	mgr := outlet.NewManager()
	mgr.Register(out)
	svc.SetOutletManager(mgr)

	req := &opsv1.CaptureRuleHitReport{
		AgentId: "capture-01", GameId: "demo", Env: "prod",
		Rule: "wallet-drain", Severity: "critical",
		Table: "game.player_wallet", Key: "7",
		Detail: "single delta 150000 >= 100000",
		Gtid:   "1-2-34", EventOp: "update", EventTsUnixMs: 1760000010000,
	}
	if _, err := svc.handleAlertEvent(context.Background(), mustMarshalHit(t, req)); err != nil {
		t.Fatalf("handleAlertEvent: %v", err)
	}

	ev := out.waitEvent(t, outlet.KindCaptureRuleHit)
	if ev.Severity != outlet.SeverityCritical {
		t.Fatalf("event severity = %s", ev.Severity)
	}
	if ev.Scope.GameID != "demo" || ev.Scope.Env != "prod" || ev.Scope.AgentID != "capture-01" {
		t.Fatalf("event scope = %+v", ev.Scope)
	}
	if ev.Title != "[wallet-drain] game.player_wallet" || ev.Body != req.Detail {
		t.Fatalf("event title/body = %q / %q", ev.Title, ev.Body)
	}
	if ev.EventID == "" || ev.DedupKey != ev.EventID {
		t.Fatalf("event id/dedup = %q / %q", ev.EventID, ev.DedupKey)
	}
}

func TestAlertEventMsgIDPair(t *testing.T) {
	if !protocol.IsRequest(protocol.MsgAlertEventRequest) || !protocol.IsResponse(protocol.MsgAlertEventResponse) {
		t.Fatal("alert msg id pair violates odd/even contract")
	}
	if protocol.GetResponseMsgID(protocol.MsgAlertEventRequest) != protocol.MsgAlertEventResponse {
		t.Fatal("response pairing broken")
	}
	if protocol.MsgIDString(protocol.MsgAlertEventRequest) != "AlertEventRequest" ||
		protocol.MsgIDString(protocol.MsgAlertEventResponse) != "AlertEventResponse" {
		t.Fatal("MsgIDString branch missing")
	}
}
