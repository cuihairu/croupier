package incident

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/outlet"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// captureOutlet records the envelopes the external chain receives (for
// distribution assertions).
type captureOutlet struct {
	mu     sync.Mutex
	events []outlet.AlertEvent
	fail   bool
}

func (c *captureOutlet) Name() string { return "capture" }

func (c *captureOutlet) Deliver(_ context.Context, ev outlet.AlertEvent) (outlet.DeliveryOutcome, error) {
	c.mu.Lock()
	c.events = append(c.events, ev)
	c.mu.Unlock()
	if c.fail {
		return outlet.DeliveryOutcome{Delivered: false, Channel: "capture"}, errDeliveryDown{}
	}
	return outlet.DeliveryOutcome{Delivered: true, Channel: "capture"}, nil
}

func (c *captureOutlet) snapshot() []outlet.AlertEvent {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]outlet.AlertEvent{}, c.events...)
}

// reportEvents filters out the report distribution events (excluding async
// lifecycle envelopes such as incident.created).
func reportEvents(all []outlet.AlertEvent) []outlet.AlertEvent {
	out := make([]outlet.AlertEvent, 0, len(all))
	for _, ev := range all {
		if strings.HasPrefix(ev.EventID, "report:") {
			out = append(out, ev)
		}
	}
	return out
}

func reportEventCount(all []outlet.AlertEvent) int {
	return len(reportEvents(all))
}

type errDeliveryDown struct{}

func (errDeliveryDown) Error() string { return "capture down" }

// recordingSink records in-app notifications (for first-hop in-app assertions).
type recordingSink struct {
	mu   sync.Mutex
	msgs []model.Message
}

func (s *recordingSink) Create(_ context.Context, n outlet.StationNotice) error {
	raw, _ := json.Marshal(n.Scope)
	s.mu.Lock()
	s.msgs = append(s.msgs, model.Message{
		To: n.To, Type: n.Type, Title: n.Title, Content: n.Content,
		Level: n.Level, Source: n.Source, RefType: n.RefType, RefID: n.RefID,
		Scope: model.JSON(raw),
	})
	s.mu.Unlock()
	return nil
}

func (s *recordingSink) all() []model.Message {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]model.Message{}, s.msgs...)
}

type generatorHarness struct {
	svc     *Service
	manager *outlet.Manager
	capture *captureOutlet
	sink    *recordingSink
	db      *gorm.DB
}

func setupGenerator(t *testing.T) *generatorHarness {
	t.Helper()
	s := setupService(t)
	if err := s.svcCtx.DB.AutoMigrate(&model.IncidentReport{}); err != nil {
		t.Fatalf("migrate incident_reports: %v", err)
	}
	h := &generatorHarness{svc: s, capture: &captureOutlet{}, sink: &recordingSink{}, db: s.svcCtx.DB}
	h.manager = outlet.NewManager()
	h.manager.Register(outlet.NewInternalOutlet(h.sink, outlet.DefaultStationRecipient))
	h.manager.Register(h.capture)
	s.svcCtx.OutletManager = h.manager
	s.svcCtx.StationSink = h.sink
	return h
}

func (h *generatorHarness) setLeader(t *testing.T, slug, leader string) uint {
	t.Helper()
	dto, err := h.svc.UpdateCategory(context.Background(), mustCategory(t, h.svc, slug).ID,
		&CategoryUpsertRequest{Leader: &leader})
	if err != nil {
		t.Fatalf("set leader on %s: %v", slug, err)
	}
	return dto.ID
}

func (h *generatorHarness) addIncident(t *testing.T, categoryID uint, sub string, at time.Time, severity, respType, respID string) {
	t.Helper()
	dto, _, err := h.svc.CreateIncident(context.Background(), &IncidentCreateRequest{
		Title: "inc", CategoryID: categoryID, Subcategory: sub, Severity: severity,
		DetectedAt: &at, ResponsibleType: respType, ResponsibleID: respID,
	}, "tester")
	if err != nil {
		t.Fatalf("create incident: %v", err)
	}
	if severity == model.IncidentSeverityCritical {
		// Left unresolved (the default status is open)
		_ = dto
	}
}

func weekBounds(t *testing.T) (curAt, prevAt, prev2At time.Time, curKey, prevKey, prev2Key string) {
	t.Helper()
	now := time.Now()
	p, err := resolvePeriod(PeriodWeek, "", now)
	if err != nil {
		t.Fatalf("resolve cur: %v", err)
	}
	pp, err := resolvePeriod(PeriodWeek, "", p.PrevStart)
	if err != nil {
		t.Fatalf("resolve prev: %v", err)
	}
	p2, err := resolvePeriod(PeriodWeek, "", pp.PrevStart)
	if err != nil {
		t.Fatalf("resolve prev2: %v", err)
	}
	return p.Start.Add(time.Hour), pp.Start.Add(time.Hour), p2.Start.Add(time.Hour),
		p.Key, pp.Key, p2.Key
}

func sliceBySlug(pieces []ReportCategorySlice, slug string) *ReportCategorySlice {
	for i := range pieces {
		if pieces[i].Slug == slug {
			return &pieces[i]
		}
	}
	return nil
}

func pushBySlug(entries []PushStatusEntry, slug string) *PushStatusEntry {
	return pushBySlugChannel(entries, slug, "")
}

func pushBySlugChannel(entries []PushStatusEntry, slug, channel string) *PushStatusEntry {
	for i := range entries {
		if entries[i].Slug == slug && (channel == "" || entries[i].Channel == channel) {
			return &entries[i]
		}
	}
	return nil
}

// ---- End-to-end generation + distribution ----

func TestGenerateReport_EndToEnd(t *testing.T) {
	h := setupGenerator(t)
	clientID := h.setLeader(t, model.IncidentCatClient, "leader-zhu")
	opsID := h.setLeader(t, model.IncidentCatOps, "")
	curAt, prevAt, _, curKey, _, _ := weekBounds(t)

	// Current period: client 2 (1 critical resolved) + ops 1; previous period
	// client 1.
	h.addIncident(t, clientID, "崩溃", curAt, model.IncidentSeverityWarning, model.IncidentRespOperator, "op-1")
	h.addIncident(t, clientID, "性能", curAt.Add(time.Hour), model.IncidentSeverityInfo, "", "")
	h.addIncident(t, opsID, "部署", curAt, model.IncidentSeverityInfo, "", "")
	h.addIncident(t, clientID, "崩溃", prevAt, model.IncidentSeverityInfo, "", "")

	row, err := h.svc.GenerateReport(context.Background(), PeriodWeek, "")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if row.PeriodType != PeriodWeek || row.PeriodStart != curKey {
		t.Fatalf("row period = %s/%s, want week/%s", row.PeriodType, row.PeriodStart, curKey)
	}
	var payload ReportPayload
	if err := json.Unmarshal(row.Payload, &payload); err != nil {
		t.Fatalf("parse payload: %v", err)
	}
	if payload.Summary == nil || payload.Summary.Metrics.Total.Value == nil || *payload.Summary.Metrics.Total.Value != 3 {
		t.Fatalf("summary total = %v, want 3（环比期事故不计入本期）", payload.Summary.Metrics.Total.Value)
	}
	// Classification: client 2 vs prev 1 (doubled) -> warn; ops prev=0 is not
	// judged by PoP -> info.
	client := sliceBySlug(payload.Categories, model.IncidentCatClient)
	if client == nil || client.Level != model.MessageLevelWarn {
		t.Fatalf("client slice = %+v, want warn（环比×2）", client)
	}
	if client.PrevCount != 1 {
		t.Fatalf("client prevCount = %d, want 1", client.PrevCount)
	}
	ops := sliceBySlug(payload.Categories, model.IncidentCatOps)
	if ops == nil || ops.Level != model.MessageLevelInfo {
		t.Fatalf("ops slice = %+v, want info", ops)
	}
	if payload.Level != model.MessageLevelWarn {
		t.Fatalf("overall level = %s, want warn", payload.Level)
	}

	// Distribution receipts: aggregate + client piece (leader set) +
	// ops skipped(no leader).
	var status []PushStatusEntry
	if err := json.Unmarshal(row.PushStatus, &status); err != nil {
		t.Fatalf("parse push status: %v", err)
	}
	// One per piece per channel: aggregate internal+capture, client
	// internal+capture, ops skipped(no leader).
	aggStation := pushBySlugChannel(status, "", "internal")
	if aggStation == nil || !aggStation.OK || aggStation.EventID != "report:week:"+curKey+":all" {
		t.Fatalf("aggregate station entry = %+v", aggStation)
	}
	agg := pushBySlugChannel(status, "", "capture")
	if agg == nil || !agg.OK || agg.EventID != "report:week:"+curKey+":all" {
		t.Fatalf("aggregate external entry = %+v", agg)
	}
	clientPush := pushBySlugChannel(status, model.IncidentCatClient, "internal")
	if clientPush == nil || clientPush.Leader != "leader-zhu" || !clientPush.OK ||
		clientPush.EventID != "report:week:"+curKey+":"+model.IncidentCatClient {
		t.Fatalf("client station entry = %+v", clientPush)
	}
	if ext := pushBySlugChannel(status, model.IncidentCatClient, "capture"); ext == nil || !ext.OK {
		t.Fatalf("client external entry = %+v", ext)
	}
	opsPush := pushBySlug(status, model.IncidentCatOps)
	if opsPush == nil || opsPush.Channel != "skipped" || opsPush.OK || opsPush.Error != "no leader" {
		t.Fatalf("ops entry = %+v", opsPush)
	}

	// Envelopes: the piece carries Target/Severity/Scope; the aggregate has no
	// Target.
	events := h.capture.snapshot()
	var clientEv, aggEv *outlet.AlertEvent
	for i := range events {
		if events[i].EventID == clientPush.EventID {
			clientEv = &events[i]
		}
		if events[i].EventID == agg.EventID {
			aggEv = &events[i]
		}
	}
	if clientEv == nil || clientEv.Target != "category-leader:"+model.IncidentCatClient ||
		clientEv.Scope.CategorySlug != model.IncidentCatClient || clientEv.Severity != outlet.SeverityWarning {
		t.Fatalf("client event = %+v", clientEv)
	}
	if aggEv == nil || aggEv.Target != "" || aggEv.Severity != outlet.SeverityWarning {
		t.Fatalf("aggregate event = %+v", aggEv)
	}

	// In-app: the aggregate lands in the default group, the client piece lands
	// with the leader (scope carries the category).
	msgs := h.sink.all()
	var aggMsg, leaderMsg *model.Message
	for i := range msgs {
		if msgs[i].RefID == agg.EventID && msgs[i].To == outlet.DefaultStationRecipient {
			aggMsg = &msgs[i]
		}
		if msgs[i].RefID == clientPush.EventID && msgs[i].To == "leader-zhu" {
			leaderMsg = &msgs[i]
		}
	}
	if aggMsg == nil || aggMsg.Type != "report" || aggMsg.Source != "incident_report" {
		t.Fatalf("aggregate message = %+v", aggMsg)
	}
	if leaderMsg == nil || leaderMsg.Level != model.MessageLevelWarn {
		t.Fatalf("leader message = %+v", leaderMsg)
	}
	var scope map[string][]string
	if err := json.Unmarshal(leaderMsg.Scope, &scope); err != nil || len(scope["categories"]) != 1 || scope["categories"][0] != model.IncidentCatClient {
		t.Fatalf("leader scope = %s (err %v)", leaderMsg.Scope, err)
	}
}

// ---- Severity classification definition ----

func TestGenerateReport_CriticalByUnresolvedCritical(t *testing.T) {
	h := setupGenerator(t)
	clientID := h.setLeader(t, model.IncidentCatClient, "leader-a")
	curAt, _, _, _, _, _ := weekBounds(t)
	h.addIncident(t, clientID, "崩溃", curAt, model.IncidentSeverityCritical, "", "")

	row, err := h.svc.GenerateReport(context.Background(), PeriodWeek, "")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	var payload ReportPayload
	_ = json.Unmarshal(row.Payload, &payload)
	client := sliceBySlug(payload.Categories, model.IncidentCatClient)
	if client == nil || client.Level != model.MessageLevelCritical || client.UnresolvedCritical != 1 {
		t.Fatalf("client slice = %+v, want critical", client)
	}
	if payload.Level != model.MessageLevelCritical {
		t.Fatalf("overall = %s, want critical", payload.Level)
	}
	for _, ev := range h.capture.snapshot() {
		if ev.EventID == "report:week:"+row.PeriodStart+":"+model.IncidentCatClient && ev.Severity != outlet.SeverityCritical {
			t.Fatalf("client severity = %s, want critical", ev.Severity)
		}
	}
}

func TestGenerateReport_CriticalByWorsenedTwoPeriods(t *testing.T) {
	h := setupGenerator(t)
	clientID := h.setLeader(t, model.IncidentCatClient, "leader-a")
	curAt, _, _, _, prevKey, prev2Key := weekBounds(t)

	// Stored history: the previous two periods rise 1 -> 2 consecutively, the
	// current period is 3 -> two consecutive worsening periods -> critical.
	storeHistory := func(key string, count int64) {
		raw, _ := json.Marshal(ReportPayload{Categories: []ReportCategorySlice{
			{Slug: model.IncidentCatClient, Count: count},
		}})
		if err := h.db.Create(&model.IncidentReport{
			PeriodType: PeriodWeek, PeriodStart: key, ReportKind: model.ReportKindSummary,
			Payload: model.JSON(raw), GeneratedAt: time.Now(),
		}).Error; err != nil {
			t.Fatalf("store history %s: %v", key, err)
		}
	}
	storeHistory(prevKey, 2)
	storeHistory(prev2Key, 1)
	for i := 0; i < 3; i++ {
		h.addIncident(t, clientID, "崩溃", curAt.Add(time.Duration(i)*time.Hour), model.IncidentSeverityInfo, "", "")
	}

	row, err := h.svc.GenerateReport(context.Background(), PeriodWeek, "")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	var payload ReportPayload
	_ = json.Unmarshal(row.Payload, &payload)
	client := sliceBySlug(payload.Categories, model.IncidentCatClient)
	if client == nil || client.Level != model.MessageLevelCritical {
		t.Fatalf("client slice = %+v, want critical（3>2>1）", client)
	}
}

func TestGenerateReport_NoHistoryNoTrendCritical(t *testing.T) {
	h := setupGenerator(t)
	clientID := h.setLeader(t, model.IncidentCatClient, "leader-a")
	curAt, _, _, _, _, _ := weekBounds(t)
	h.addIncident(t, clientID, "崩溃", curAt, model.IncidentSeverityInfo, "", "")

	row, err := h.svc.GenerateReport(context.Background(), PeriodWeek, "")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	var payload ReportPayload
	_ = json.Unmarshal(row.Payload, &payload)
	client := sliceBySlug(payload.Categories, model.IncidentCatClient)
	// prev=0 (no PoP base) and no history: the first period is not judged as
	// worsening.
	if client == nil || client.Level != model.MessageLevelInfo {
		t.Fatalf("client slice = %+v, want info（无环比基数/无历史）", client)
	}
}

func TestGenerateReport_RecurrenceRateWarn(t *testing.T) {
	h := setupGenerator(t)
	clientID := h.setLeader(t, model.IncidentCatClient, "leader-a")
	curAt, _, _, _, _, _ := weekBounds(t)

	// Same signature (source+refId): a recurrence within 7 days of a resolved
	// incident forms a recurrence chain, recurrence rate 100% -> warn. The
	// first incident has an explicit ResolvedAt (earlier than the recurrence,
	// so the chain condition holds).
	resolvedAt := curAt.Add(30 * time.Minute)
	first, _, err := h.svc.CreateIncident(context.Background(), &IncidentCreateRequest{
		Title: "first", CategoryID: clientID, Subcategory: "崩溃", Severity: model.IncidentSeverityWarning,
		DetectedAt: ptrTime(curAt), RefType: "alert", RefID: "srv-1",
		ResponsibleType: model.IncidentRespOperator, ResponsibleID: "srv-1",
	}, "tester")
	if err != nil {
		t.Fatalf("create first: %v", err)
	}
	if _, err := h.svc.TransitionIncident(context.Background(), first.ID, &IncidentStatusRequest{
		Status: model.IncidentStatusResolved, ResolvedAt: &resolvedAt,
	}); err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if _, _, err := h.svc.CreateIncident(context.Background(), &IncidentCreateRequest{
		Title: "second", CategoryID: clientID, Subcategory: "崩溃", Severity: model.IncidentSeverityWarning,
		DetectedAt: ptrTime(curAt.Add(time.Hour)), RefType: "alert", RefID: "srv-1",
		ResponsibleType: model.IncidentRespOperator, ResponsibleID: "srv-1",
	}, "tester"); err != nil {
		t.Fatalf("create second: %v", err)
	}

	row, err := h.svc.GenerateReport(context.Background(), PeriodWeek, "")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	var payload ReportPayload
	_ = json.Unmarshal(row.Payload, &payload)
	client := sliceBySlug(payload.Categories, model.IncidentCatClient)
	if client == nil || client.RecurrencePct <= escalateRecurrencePct || client.Level != model.MessageLevelWarn {
		t.Fatalf("client slice = %+v, want warn（复发率>20%%）", client)
	}
}

// ---- upsert / repush / schedule entry point ----

func TestGenerateReport_UpsertSamePeriod(t *testing.T) {
	h := setupGenerator(t)
	curAt, _, _, curKey, _, _ := weekBounds(t)
	h.addIncident(t, mustCategory(t, h.svc, model.IncidentCatQA).ID, "", curAt, model.IncidentSeverityInfo, "", "")

	first, err := h.svc.GenerateReport(context.Background(), PeriodWeek, "")
	if err != nil {
		t.Fatalf("generate 1: %v", err)
	}
	second, err := h.svc.GenerateReport(context.Background(), PeriodWeek, "")
	if err != nil {
		t.Fatalf("generate 2: %v", err)
	}
	if first.ID != second.ID {
		t.Fatalf("ids differ: %d vs %d（同档同期应覆盖）", first.ID, second.ID)
	}
	var count int64
	_ = h.db.Model(&model.IncidentReport{}).Where("period_type = ? AND period_start = ?", PeriodWeek, curKey).Count(&count).Error
	if count != 1 {
		t.Fatalf("rows = %d, want 1", count)
	}
}

func TestRepushReport_RedispatchesSameEventIDs(t *testing.T) {
	h := setupGenerator(t)
	clientID := h.setLeader(t, model.IncidentCatClient, "leader-a")
	curAt, _, _, curKey, _, _ := weekBounds(t)
	h.addIncident(t, clientID, "崩溃", curAt, model.IncidentSeverityInfo, "", "")

	row, err := h.svc.GenerateReport(context.Background(), PeriodWeek, "")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	before := reportEventCount(h.capture.snapshot())
	if before == 0 {
		t.Fatal("no events on first push")
	}
	payloadBefore := string(row.Payload)

	repushed, err := h.svc.RepushReport(context.Background(), row.ID)
	if err != nil {
		t.Fatalf("repush: %v", err)
	}
	if string(repushed.Payload) != payloadBefore {
		t.Fatal("repush must not rewrite payload")
	}
	// Count only report events: incident creation asynchronously dispatches
	// the incident.created lifecycle webhook (§9.3), which is unrelated to the
	// report repush under test, so it must be excluded from the count.
	events := reportEvents(h.capture.snapshot())
	if len(events) != before*2 {
		t.Fatalf("events after repush = %d, want %d", len(events), before*2)
	}
	for _, ev := range events[before:] {
		if ev.EventID != "report:week:"+curKey+":all" && ev.EventID != "report:week:"+curKey+":"+model.IncidentCatClient {
			t.Fatalf("unexpected eventID %s", ev.EventID)
		}
	}
	var status []PushStatusEntry
	if err := json.Unmarshal(repushed.PushStatus, &status); err != nil {
		t.Fatalf("parse status: %v", err)
	}
	if agg := pushBySlug(status, ""); agg == nil || !agg.OK {
		t.Fatalf("aggregate entry after repush = %+v", agg)
	}
}

func TestRepushReport_NotFoundAndCorruptPayload(t *testing.T) {
	h := setupGenerator(t)
	if _, err := h.svc.RepushReport(context.Background(), 9999); err == nil {
		t.Fatal("missing row must error")
	}
	raw, _ := json.Marshal(map[string]string{"summary": "corrupt"})
	if err := h.db.Create(&model.IncidentReport{
		PeriodType: PeriodWeek, PeriodStart: "2099-W01", Payload: model.JSON(raw), GeneratedAt: time.Now(),
	}).Error; err != nil {
		t.Fatalf("create corrupt row: %v", err)
	}
	var row model.IncidentReport
	_ = h.db.Where("period_start = ?", "2099-W01").First(&row).Error
	if _, err := h.svc.RepushReport(context.Background(), row.ID); err == nil {
		t.Fatal("corrupt payload must error")
	}
}

func TestRunnerRun_ParsesSchedulePayload(t *testing.T) {
	h := setupGenerator(t)
	curAt, _, _, _, _, _ := weekBounds(t)
	h.addIncident(t, mustCategory(t, h.svc, model.IncidentCatQA).ID, "", curAt, model.IncidentSeverityInfo, "", "")

	period := PeriodWeek
	raw, _ := json.Marshal(map[string]string{"period": period})
	if err := h.svc.Run(context.Background(), &model.TaskSchedule{
		Name: "s", Payload: model.JSON(raw),
	}); err != nil {
		t.Fatalf("run: %v", err)
	}
	var count int64
	_ = h.db.Model(&model.IncidentReport{}).Count(&count).Error
	if count != 1 {
		t.Fatalf("reports = %d, want 1", count)
	}

	// Missing period / invalid period / nil schedule -> error (the scheduler
	// counts failures).
	if err := h.svc.Run(context.Background(), &model.TaskSchedule{Name: "s"}); err == nil {
		t.Fatal("missing period must error")
	}
	if err := h.svc.Run(context.Background(), &model.TaskSchedule{
		Name: "s", Payload: model.JSON(`{"period":"bogus"}`),
	}); err == nil {
		t.Fatal("bogus period must error")
	}
	if err := h.svc.Run(context.Background(), nil); err == nil {
		t.Fatal("nil schedule must error")
	}
}

func TestListStoredReports_PaginationAndFilter(t *testing.T) {
	h := setupGenerator(t)
	curAt, _, _, _, prevKey, _ := weekBounds(t)
	h.addIncident(t, mustCategory(t, h.svc, model.IncidentCatQA).ID, "", curAt, model.IncidentSeverityInfo, "", "")
	if _, err := h.svc.GenerateReport(context.Background(), PeriodWeek, ""); err != nil {
		t.Fatalf("generate: %v", err)
	}
	raw := model.JSON(`{"summary":{"metrics":{"total":{"value":3}}},"categories":[],"level":"info"}`)
	if err := h.db.Create(&model.IncidentReport{
		PeriodType: PeriodWeek, PeriodStart: prevKey, Payload: raw, GeneratedAt: time.Now(),
	}).Error; err != nil {
		t.Fatalf("create: %v", err)
	}
	resp, err := h.svc.ListStoredReports(context.Background(), PeriodWeek, 1, 10)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if resp.Total != 2 || len(resp.Items) != 2 {
		t.Fatalf("list = %d/%d, want 2/2", len(resp.Items), resp.Total)
	}
	// Descending order: the newest period first.
	if resp.Items[0].PeriodStart <= resp.Items[1].PeriodStart {
		t.Fatalf("order = %s then %s, want desc", resp.Items[0].PeriodStart, resp.Items[1].PeriodStart)
	}
	if resp.Items[1].Level != model.MessageLevelInfo || resp.Items[1].Total == nil || *resp.Items[1].Total != 3 {
		t.Fatalf("payload summary parse: %+v", resp.Items[1])
	}
	// Tier filter.
	monthResp, err := h.svc.ListStoredReports(context.Background(), PeriodMonth, 1, 10)
	if err != nil || monthResp.Total != 0 {
		t.Fatalf("month filter: %v/%d", err, monthResp.Total)
	}
	// Invalid tier.
	if _, err := h.svc.ListStoredReports(context.Background(), "bogus", 1, 10); err == nil {
		t.Fatal("bogus period must error")
	}
}

// ---- Outlet failure paths ----

func TestGenerateReport_OutletFailureRecordsError(t *testing.T) {
	h := setupGenerator(t)
	h.capture.fail = true
	curAt, _, _, _, _, _ := weekBounds(t)
	h.addIncident(t, mustCategory(t, h.svc, model.IncidentCatQA).ID, "", curAt, model.IncidentSeverityInfo, "", "")

	row, err := h.svc.GenerateReport(context.Background(), PeriodWeek, "")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	var status []PushStatusEntry
	if err := json.Unmarshal(row.PushStatus, &status); err != nil {
		t.Fatalf("parse: %v", err)
	}
	agg := pushBySlugChannel(status, "", "capture")
	if agg == nil || agg.OK || agg.Error == "" {
		t.Fatalf("aggregate external entry = %+v, want failed with error", agg)
	}
	// The in-app leg is unaffected by the external failure.
	if st := pushBySlugChannel(status, "", "internal"); st == nil || !st.OK {
		t.Fatalf("aggregate station entry = %+v, want ok", st)
	}
}

func TestGenerateReport_OutletsNotWired(t *testing.T) {
	h := setupGenerator(t)
	h.svc.svcCtx.OutletManager = nil
	curAt, _, _, _, _, _ := weekBounds(t)
	h.addIncident(t, mustCategory(t, h.svc, model.IncidentCatQA).ID, "", curAt, model.IncidentSeverityInfo, "", "")

	row, err := h.svc.GenerateReport(context.Background(), PeriodWeek, "")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	var status []PushStatusEntry
	_ = json.Unmarshal(row.PushStatus, &status)
	aggStation := pushBySlugChannel(status, "", "internal")
	if aggStation == nil || !aggStation.OK {
		t.Fatalf("aggregate station entry = %+v, want ok（sink 直写不受出口影响）", aggStation)
	}
	agg := pushBySlugChannel(status, "", "skipped")
	if agg == nil || agg.OK || agg.Error != "outlets not wired" {
		t.Fatalf("aggregate external entry = %+v, want skipped(not wired)", agg)
	}
}

func ptrTime(v time.Time) *time.Time { return &v }

// ---- HTTP entry points ----

func TestReportHandlers_RepushAndStored(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := setupGenerator(t)
	clientID := h.setLeader(t, model.IncidentCatClient, "leader-a")
	curAt, _, _, _, _, _ := weekBounds(t)
	h.addIncident(t, clientID, "崩溃", curAt, model.IncidentSeverityInfo, "", "")
	row, err := h.svc.GenerateReport(context.Background(), PeriodWeek, "")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	handler := NewHandler(h.svc)
	router := gin.New()
	rg := router.Group("/incident-reports")
	rg.POST("/:id/repush", handler.RepushReport)
	rg.GET("/stored", handler.ListStoredReports)

	// Repush: 200 + payload unchanged.
	if w := doJSON(t, router, http.MethodPost, "/incident-reports/9999", ""); w.Code != http.StatusNotFound {
		t.Fatalf("repush missing = %d, want 404", w.Code)
	}
	if w := doJSON(t, router, http.MethodPost, fmt.Sprintf("/incident-reports/%d/repush", row.ID), ""); w.Code != http.StatusOK {
		t.Fatalf("repush = %d: %s", w.Code, w.Body.String())
	}
	// History: 200 + one row for the current period.
	wList := doJSON(t, router, http.MethodGet, "/incident-reports/stored?period=week", "")
	if wList.Code != http.StatusOK {
		t.Fatalf("stored = %d: %s", wList.Code, wList.Body.String())
	}
	var list StoredReportListResponse
	if err := json.Unmarshal(wList.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if list.Total != 1 || len(list.Items) != 1 || list.Items[0].ID != row.ID {
		t.Fatalf("list = %+v, want 1 row id=%d", list, row.ID)
	}
	// Invalid tier -> 400.
	if w := doJSON(t, router, http.MethodGet, "/incident-reports/stored?period=decade", ""); w.Code != http.StatusBadRequest {
		t.Fatalf("bad period = %d, want 400", w.Code)
	}
}
