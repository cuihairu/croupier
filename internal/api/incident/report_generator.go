package incident

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/outlet"
	"gorm.io/gorm"
)

// Report generation and distribution (docs/design/incident-reports.md §6):
// the scheduler Kind=incident_report fires through scheduler.LocalRunner
// (server-local execution, not the agent dispatch chain; the svc wiring is
// injected by cmd/server). Flow = aggregate (§5 definitions) -> category
// severity classification (fixed thresholds, not configurable) -> upsert
// incident_reports -> distribute (in-app chain first + optional external
// chain degrade) -> write back PushStatus.

// Fixed classification thresholds (§6 v1 definition): PoP doubling /
// recurrence above 20% -> warn; two consecutive worsening periods, or an
// unresolved critical incident, -> critical.
const (
	escalateDoubleFactor  = 2.0
	escalateRecurrencePct = 20.0
)

// ReportCategorySlice is one category piece (the smallest unit for
// classification and distribution; part of the payload landed in the database,
// and a repush resends the stored value verbatim without rewriting it).
type ReportCategorySlice struct {
	CategoryID         uint    `json:"categoryId"`
	Slug               string  `json:"slug,omitempty"`
	Name               string  `json:"name,omitempty"`
	Leader             string  `json:"leader,omitempty"`
	Count              int64   `json:"count"`
	PrevCount          int64   `json:"prevCount"`
	RecurrencePct      float64 `json:"recurrencePct"`      // 0-100; 0 with no sample
	UnresolvedCritical int64   `json:"unresolvedCritical"` // unresolved critical incidents within the period
	Level              string  `json:"level"`              // info|warn|critical
}

// ReportPayload is the top-level shape of incident_reports.Payload: the full
// summary + per-category piece levels + the highest level across the report.
type ReportPayload struct {
	Summary    *ReportSummaryResponse `json:"summary"`
	Categories []ReportCategorySlice  `json:"categories"`
	Level      string                 `json:"level"` // info|warn|critical
}

// PushStatusEntry is a distribution receipt (a §6 PushStatus array element):
// one per piece, with channel being the terminal outlet of the external chain
// (herald/noop/...) or skipped (no leader configured / outlet not wired).
type PushStatusEntry struct {
	Slug     string    `json:"slug"`
	Leader   string    `json:"leader,omitempty"`
	Channel  string    `json:"channel"`
	PushedAt time.Time `json:"pushedAt"`
	EventID  string    `json:"eventId"`
	OK       bool      `json:"ok"`
	Error    string    `json:"error,omitempty"`
}

// GenerateReport generates and distributes one report period (periodType:
// week|month|quarter|year; empty periodKey = the current period). Idempotent:
// upserts overwrite the same period within the same tier; external outlets
// fold resends by the deterministic event_id.
func (s *Service) GenerateReport(ctx context.Context, periodType, periodKey string) (*model.IncidentReport, error) {
	now := time.Now()
	p, err := resolvePeriod(periodType, periodKey, now)
	if err != nil {
		return nil, err
	}
	summary, err := s.ReportSummary(ctx, p.Type, p.Key)
	if err != nil {
		return nil, err
	}
	pieces, err := s.buildCategorySlices(ctx, p, now)
	if err != nil {
		return nil, err
	}
	level := model.MessageLevelInfo
	for i := range pieces {
		if sliceLevelRank(pieces[i].Level) > sliceLevelRank(level) {
			level = pieces[i].Level
		}
	}
	payload := ReportPayload{Summary: summary, Categories: pieces, Level: level}
	row, err := s.upsertReportRow(ctx, p.Type, p.Key, payload, now)
	if err != nil {
		return nil, err
	}
	status := s.distribute(ctx, p.Type, p.Key, &payload, now)
	if err := s.writePushStatus(ctx, row.ID, status); err != nil {
		return nil, err
	}
	raw, _ := json.Marshal(status)
	row.PushStatus = model.JSON(raw)
	return row, nil
}

// Run implements scheduler.LocalRunner: the schedule entry point
// (task_schedules.Kind=incident_report). Payload {"period":"week"}; a missing
// period or an invalid tier is an error (the scheduler counts consecutive
// failures and moves to dead_letter at the ceiling).
func (s *Service) Run(ctx context.Context, sched *model.TaskSchedule) error {
	if sched == nil {
		return errors.New("incident report runner: nil schedule")
	}
	var p struct {
		Period string `json:"period"`
	}
	if len(sched.Payload) > 0 {
		if err := json.Unmarshal(sched.Payload, &p); err != nil {
			return fmt.Errorf("incident report runner: parse payload: %w", err)
		}
	}
	if p.Period == "" {
		return errors.New("incident report runner: schedule payload missing period")
	}
	row, err := s.GenerateReport(ctx, p.Period, "")
	if err != nil {
		return fmt.Errorf("incident report runner: %w", err)
	}
	slog.InfoContext(ctx, "incident report generated", "schedule", sched.Name,
		"period", row.PeriodType, "periodKey", row.PeriodStart)
	return nil
}

// RepushReport is the manual repush (§6: resends with the same event_id from
// the stored payload so outlets fold it idempotently; does not change the
// payload). Receipts overwrite PushStatus.
func (s *Service) RepushReport(ctx context.Context, id uint) (*model.IncidentReport, error) {
	var row model.IncidentReport
	if err := s.svcCtx.DB.WithContext(ctx).First(&row, id).Error; err != nil {
		return nil, err
	}
	var payload ReportPayload
	if err := json.Unmarshal(row.Payload, &payload); err != nil {
		return nil, errorx.NewBadRequest("报表 payload 损坏: " + err.Error())
	}
	if payload.Summary == nil {
		return nil, errorx.NewBadRequest("报表 payload 缺少 summary，无法重推")
	}
	status := s.distribute(ctx, row.PeriodType, row.PeriodStart, &payload, time.Now())
	if err := s.writePushStatus(ctx, row.ID, status); err != nil {
		return nil, err
	}
	raw, _ := json.Marshal(status)
	row.PushStatus = model.JSON(raw)
	return &row, nil
}

// ---- Category classification ----

// buildCategorySlices aggregates each category's count / PoP / recurrence rate
// / unresolved critical and classifies under the fixed thresholds. Counts
// include bugs (consistent with the summary definition).
func (s *Service) buildCategorySlices(ctx context.Context, p *resolvedPeriod, now time.Time) ([]ReportCategorySlice, error) {
	cur, err := s.periodStats(ctx, p.Start, p.End, now)
	if err != nil {
		return nil, err
	}
	prevShare, err := s.categoryShare(ctx, p.PrevStart, p.PrevEnd, cur.Total)
	if err != nil {
		return nil, err
	}
	prevByID := make(map[uint]int64, len(prevShare))
	for _, sh := range prevShare {
		prevByID[sh.CategoryID] = sh.Count
	}
	// Recurrence rate numerator/denominator per category: chain heads come
	// from the current period plus the pre-period window, the denominator is
	// resolved incidents within the period.
	rows := append(append([]incidentAggRow{}, cur.rows...), s.preWindowRows(ctx, p.Start)...)
	heads := recurrenceHeads(rows, p.Start, p.End)
	recurByCat := map[uint]int64{}
	for _, h := range heads {
		recurByCat[h.CategoryID]++
	}
	resolvedByCat := map[uint]int64{}
	for _, r := range cur.rows {
		if r.ResolvedAt != nil {
			resolvedByCat[r.CategoryID]++
		}
	}
	// Unresolved critical incidents within the period, counted per category.
	type critRow struct {
		CategoryID uint
	}
	crit := []critRow{}
	if err := s.svcCtx.DB.WithContext(ctx).Model(&model.Incident{}).
		Select("category_id").
		Where("detected_at >= ? AND detected_at < ?", p.Start, p.End).
		Where("severity = ? AND status <> ?", model.IncidentSeverityCritical, model.IncidentStatusResolved).
		Limit(reportScanLimit).Scan(&crit).Error; err != nil {
		return nil, err
	}
	critByCat := map[uint]int64{}
	for _, r := range crit {
		critByCat[r.CategoryID]++
	}
	// leader/slug/name index (disabled rows also participate -- reports cover
	// pre-existing categories).
	cats, err := model.NewIncidentCategoryModel(s.svcCtx.DB).List(ctx, false)
	if err != nil {
		return nil, err
	}
	catByID := make(map[uint]model.IncidentCategory, len(cats))
	for _, c := range cats {
		catByID[c.ID] = c
	}
	// Counts per category (incidents + bugs, reusing the same definition as
	// categoryShare).
	curShare, err := s.categoryShare(ctx, p.Start, p.End, cur.Total)
	if err != nil {
		return nil, err
	}
	pieces := make([]ReportCategorySlice, 0, len(curShare))
	for _, sh := range curShare {
		piece := ReportCategorySlice{
			CategoryID:         sh.CategoryID,
			Slug:               sh.Slug,
			Name:               sh.Name,
			Count:              sh.Count,
			PrevCount:          prevByID[sh.CategoryID],
			UnresolvedCritical: critByCat[sh.CategoryID],
		}
		if den := resolvedByCat[sh.CategoryID]; den > 0 {
			piece.RecurrencePct = float64(recurByCat[sh.CategoryID]) / float64(den) * 100
		}
		if c, ok := catByID[sh.CategoryID]; ok {
			piece.Leader = c.Leader
			if piece.Name == "" {
				piece.Name = c.Name
			}
			if piece.Slug == "" {
				piece.Slug = c.Slug
			}
		}
		classifySlice(ctx, s, p, &piece)
		pieces = append(pieces, piece)
	}
	return pieces, nil
}

// classifySlice classifies under fixed thresholds (§6): warn = PoP x2
// (meaningful only with a base > 0) or recurrence > 20%; critical = an
// unresolved critical incident, or two consecutive worsening periods (stored
// history: cur > prev > prev-prev; verdict withheld when any period's history
// is missing). Levels only escalate, never de-escalate.
func classifySlice(ctx context.Context, s *Service, p *resolvedPeriod, piece *ReportCategorySlice) {
	level := model.MessageLevelInfo
	if piece.PrevCount > 0 && piece.Count > 0 &&
		float64(piece.Count) >= float64(piece.PrevCount)*escalateDoubleFactor {
		level = model.MessageLevelWarn
	}
	if piece.RecurrencePct > escalateRecurrencePct {
		level = model.MessageLevelWarn
	}
	if piece.UnresolvedCritical > 0 {
		level = model.MessageLevelCritical
	}
	if worsened, err := s.worsenedTwoPeriods(ctx, p.Type, p.Key, piece.Slug, piece.Count); err == nil && worsened {
		level = model.MessageLevelCritical
	} else if err != nil {
		slog.WarnContext(ctx, "incident report: read history for escalation failed", "error", err)
	}
	piece.Level = level
}

// worsenedTwoPeriods decides sustained worsening: reads stored reports of the
// same tier (period_start earlier than the current one, the two most recent in
// reverse order) and requires cur > prev > prev-prev for the category. Missing
// history for any period -> false (the first report has no trend to judge).
func (s *Service) worsenedTwoPeriods(ctx context.Context, periodType, periodKey, slug string, curCount int64) (bool, error) {
	if slug == "" {
		return false, nil
	}
	var rows []model.IncidentReport
	err := s.svcCtx.DB.WithContext(ctx).
		Where("period_type = ? AND period_start < ?", periodType, periodKey).
		Order("period_start DESC").Limit(2).Find(&rows).Error
	if err != nil {
		return false, err
	}
	if len(rows) < 2 {
		return false, nil
	}
	counts := make([]int64, 0, 2)
	for _, r := range rows {
		var payload ReportPayload
		if err := json.Unmarshal(r.Payload, &payload); err != nil {
			return false, nil // corrupt history payload treated as no history, does not block generation
		}
		for _, c := range payload.Categories {
			if c.Slug == slug {
				counts = append(counts, c.Count)
				break
			}
		}
	}
	if len(counts) < 2 {
		return false, nil
	}
	return curCount > counts[0] && counts[0] > counts[1], nil
}

// ---- Persistence ----

// upsertReportRow upserts one period within one tier (unique index
// uidx_incident_reports_period; concurrent double-write falls back to
// conflict-then-query).
func (s *Service) upsertReportRow(ctx context.Context, periodType, periodKey string, payload ReportPayload, now time.Time) (*model.IncidentReport, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	db := s.svcCtx.DB.WithContext(ctx)
	var row model.IncidentReport
	err = db.Where("period_type = ? AND period_start = ?", periodType, periodKey).First(&row).Error
	if err == nil {
		updates := map[string]interface{}{
			"payload":      model.JSON(raw),
			"generated_at": now,
			"report_kind":  model.ReportKindSummary,
		}
		if err := db.Model(&model.IncidentReport{}).Where("id = ?", row.ID).Updates(updates).Error; err != nil {
			return nil, err
		}
		row.Payload, row.GeneratedAt, row.ReportKind = model.JSON(raw), now, model.ReportKindSummary
		return &row, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	row = model.IncidentReport{
		PeriodType: periodType, PeriodStart: periodKey,
		ReportKind: model.ReportKindSummary, Payload: model.JSON(raw), GeneratedAt: now,
	}
	if err := db.Create(&row).Error; err != nil {
		if isUniqueViolation(err) {
			if ferr := db.Where("period_type = ? AND period_start = ?", periodType, periodKey).First(&row).Error; ferr == nil {
				return &row, nil
			}
		}
		return nil, err
	}
	return &row, nil
}

func (s *Service) writePushStatus(ctx context.Context, id uint, status []PushStatusEntry) error {
	raw, err := json.Marshal(status)
	if err != nil {
		return err
	}
	return s.svcCtx.DB.WithContext(ctx).Model(&model.IncidentReport{}).
		Where("id = ?", id).Update("push_status", model.JSON(raw)).Error
}

// ---- Distribution ----

// distribute performs distribution (§6): in-app writes go directly through
// MessageSink (the aggregate lands in the default recipient group and pieces
// land in the leader account -- only the generator can see the category->leader
// mapping); external chains go through the outlet DispatchExternal
// (target=category-leader:<slug>). One PushStatus per piece per channel; pieces
// without a configured leader are recorded as skipped(no leader) and not
// delivered -- the data still lives in the aggregate and on the panel, nothing
// is dropped. A repush uses the same event_id: folded by in-app upsert and
// deduped on the outlet side.
func (s *Service) distribute(ctx context.Context, periodType, periodKey string, payload *ReportPayload, now time.Time) []PushStatusEntry {
	base := eventIDBase(periodType, periodKey)
	status := make([]PushStatusEntry, 0, 2*len(payload.Categories)+2)

	// Aggregate piece: in-app default recipient group (admin aggregate) +
	// external default audience.
	aggEv := s.aggregateEvent(base, periodType, periodKey, payload, now)
	status = append(status, s.deliverSlice(ctx, sliceRef{
		slug: "", leader: "", level: payload.Level,
		title: aggEv.Title, body: aggEv.Body, eventID: aggEv.EventID,
		scope: map[string]any{}, ev: &aggEv,
	}, now)...)

	for i := range payload.Categories {
		piece := &payload.Categories[i]
		if piece.Count <= 0 {
			continue // zero-count category does not get a piece delivered (no zero-event noise for the leader; data lives in the aggregate)
		}
		if piece.Leader == "" {
			status = append(status, PushStatusEntry{
				Slug: piece.Slug, Channel: "skipped", PushedAt: now,
				EventID: base + ":" + piece.Slug, OK: false, Error: "no leader",
			})
			continue
		}
		ev := s.sliceEvent(base, periodType, periodKey, piece, now)
		status = append(status, s.deliverSlice(ctx, sliceRef{
			slug: piece.Slug, leader: piece.Leader, level: piece.Level,
			title: ev.Title, body: ev.Body, eventID: ev.EventID,
			scope: map[string]any{"categories": []string{piece.Slug}}, ev: &ev,
		}, now)...)
	}
	return status
}

// sliceRef is the full context of one delivery (in-app and external envelopes
// share the title/body/event key).
type sliceRef struct {
	slug    string
	leader  string
	level   string
	title   string
	body    string
	eventID string
	scope   map[string]any
	ev      *outlet.AlertEvent
}

// deliverSlice delivers one piece: one in-app receipt (channel=internal,
// recipient the leader or the default recipient group) plus one external
// receipt (channel = the actual terminal outlet or skipped).
func (s *Service) deliverSlice(ctx context.Context, ref sliceRef, now time.Time) []PushStatusEntry {
	entries := make([]PushStatusEntry, 0, 2)

	// In-app: writes straight into messages (To=the leader account; a piece
	// without a leader produces no in-app row).
	station := PushStatusEntry{
		Slug: ref.slug, Leader: ref.leader, Channel: "internal", PushedAt: now,
		EventID: ref.eventID, OK: false, Error: "station sink not wired",
	}
	if ref.leader != "" || s.svcCtx.StationSink != nil {
		recipient := ref.leader
		if recipient == "" {
			recipient = outlet.DefaultStationRecipient
		}
		if sink := s.svcCtx.StationSink; sink != nil {
			err := sink.Create(ctx, outlet.StationNotice{
				To: recipient, Type: "report", Title: ref.title, Content: ref.body,
				Level: ref.level, Source: "incident_report",
				RefType: "report", RefID: ref.eventID, Scope: ref.scope,
			})
			if err != nil {
				station.Error = err.Error()
			} else {
				station.OK = true
				station.Error = ""
			}
		}
	}
	entries = append(entries, station)

	// External: a synchronous terminal receipt from the outlet chain.
	external := PushStatusEntry{
		Slug: ref.slug, Leader: ref.leader, Channel: "skipped", PushedAt: now,
		EventID: ref.eventID, OK: false, Error: "outlets not wired",
	}
	if manager := s.svcCtx.OutletManager; manager != nil && ref.ev != nil {
		outcome, err := manager.DispatchExternal(ctx, *ref.ev)
		external.Channel = outcome.Channel
		if err != nil {
			external.Error = err.Error()
		} else if outcome.Delivered {
			external.OK = true
			external.Error = ""
		} else {
			external.Error = "no external outlet accepted"
		}
	}
	entries = append(entries, external)
	return entries
}

// aggregateEvent builds the aggregate envelope: the title carries the total
// count and PoP, the body lists a top-categories summary.
func (s *Service) aggregateEvent(base, periodType, periodKey string, payload *ReportPayload, now time.Time) outlet.AlertEvent {
	sum := payload.Summary
	title := fmt.Sprintf("%s %s：共 %s 起", periodLabel(periodType), periodKey, formatCount(sum.Metrics.Total.Value))
	body := fmt.Sprintf("事故 %d + bug %d；影响时长 %s；MTTR %s；复发率 %s。",
		sum.Incidents, sum.Bugs,
		formatMs(sum.Metrics.Duration.Value), formatMs(sum.Metrics.MTTR.Value), formatPct(sum.Metrics.Recurrence.Value))
	top := payload.Categories
	if len(top) > 3 {
		top = top[:3]
	}
	for _, c := range top {
		name := c.Name
		if name == "" {
			name = c.Slug
		}
		body += fmt.Sprintf("\n· %s：%d 起（%s）", name, c.Count, c.Level)
	}
	return outlet.AlertEvent{
		Kind:             outlet.KindIncidentReport,
		Severity:         levelToSeverity(payload.Level),
		EventID:          base + ":all",
		DedupKey:         base + ":all",
		Title:            title,
		Body:             body,
		OccurredAtUnixMs: now.UnixMilli(),
	}
}

// sliceEvent builds the category piece envelope: in-app it goes to the leader
// via the category-leader prefix, external carries the same target; the scope
// carries the category slug (messages filters by category audience at read
// time, batch 4d).
func (s *Service) sliceEvent(base, periodType, periodKey string, piece *ReportCategorySlice, now time.Time) outlet.AlertEvent {
	name := piece.Name
	if name == "" {
		name = piece.Slug
	}
	title := fmt.Sprintf("%s %s · %s：%d 起（%s）", periodLabel(periodType), periodKey, name, piece.Count, piece.Level)
	body := fmt.Sprintf("环比 %s；复发率 %.0f%%；critical 未解决 %d。",
		formatDelta(piece.Count, piece.PrevCount), piece.RecurrencePct, piece.UnresolvedCritical)
	return outlet.AlertEvent{
		Kind:             outlet.KindIncidentReport,
		Severity:         levelToSeverity(piece.Level),
		EventID:          base + ":" + piece.Slug,
		DedupKey:         base + ":" + piece.Slug,
		Title:            title,
		Body:             body,
		OccurredAtUnixMs: now.UnixMilli(),
		Target:           "category-leader:" + piece.Slug,
		Scope:            outlet.Scope{CategorySlug: piece.Slug},
	}
}

// eventIDBase is the deterministic event key prefix
// (§6: report:<PeriodType>:<PeriodStart>).
func eventIDBase(periodType, periodKey string) string {
	return "report:" + periodType + ":" + periodKey
}

// ---- Stored report list ----

// StoredReportItem is a report history row (payload digest + distribution
// receipts; the data source for the panel history page and repush button).
type StoredReportItem struct {
	ID          uint              `json:"id"`
	PeriodType  string            `json:"periodType"`
	PeriodStart string            `json:"periodStart"`
	ReportKind  string            `json:"reportKind"`
	GeneratedAt time.Time         `json:"generatedAt"`
	Level       string            `json:"level"`
	Total       *float64          `json:"total"`
	PushStatus  []PushStatusEntry `json:"pushStatus"`
}

// StoredReportListResponse is the report history page (GET
// /incident-reports/stored).
type StoredReportListResponse struct {
	Items []StoredReportItem `json:"items"`
	Total int64              `json:"total"`
}

// ListStoredReports pages through report history (empty periodType = all
// tiers).
func (s *Service) ListStoredReports(ctx context.Context, periodType string, page, pageSize int) (*StoredReportListResponse, error) {
	if periodType != "" &&
		periodType != PeriodWeek && periodType != PeriodMonth &&
		periodType != PeriodQuarter && periodType != PeriodYear {
		return nil, errorx.NewBadRequest("无效的报表档位: " + periodType)
	}
	db := s.svcCtx.DB.WithContext(ctx).Model(&model.IncidentReport{})
	if periodType != "" {
		db = db.Where("period_type = ?", periodType)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, err
	}
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	var rows []model.IncidentReport
	if err := db.Order("period_start DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).Find(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]StoredReportItem, 0, len(rows))
	for i := range rows {
		item := StoredReportItem{
			ID: rows[i].ID, PeriodType: rows[i].PeriodType, PeriodStart: rows[i].PeriodStart,
			ReportKind: rows[i].ReportKind, GeneratedAt: rows[i].GeneratedAt,
		}
		var payload ReportPayload
		if json.Unmarshal(rows[i].Payload, &payload) == nil && payload.Summary != nil {
			item.Level = payload.Level
			item.Total = payload.Summary.Metrics.Total.Value
		}
		if len(rows[i].PushStatus) > 0 {
			_ = json.Unmarshal(rows[i].PushStatus, &item.PushStatus)
		}
		items = append(items, item)
	}
	return &StoredReportListResponse{Items: items, Total: total}, nil
}

// ---- Small helpers ----

func sliceLevelRank(level string) int {
	switch level {
	case model.MessageLevelCritical:
		return 2
	case model.MessageLevelWarn:
		return 1
	default:
		return 0
	}
}

func levelToSeverity(level string) string {
	switch level {
	case model.MessageLevelCritical:
		return outlet.SeverityCritical
	case model.MessageLevelWarn:
		return outlet.SeverityWarning
	default:
		return outlet.SeverityInfo
	}
}

// periodLabel is the Chinese display name of a tier (used in notification
// titles).
func periodLabel(periodType string) string {
	switch periodType {
	case PeriodWeek:
		return "事故周报"
	case PeriodMonth:
		return "事故月报"
	case PeriodQuarter:
		return "事故季报"
	case PeriodYear:
		return "事故年报"
	default:
		return "事故报表"
	}
}

func formatCount(v *float64) string {
	if v == nil {
		return "-"
	}
	return fmt.Sprintf("%.0f", *v)
}

func formatMs(v *float64) string {
	if v == nil {
		return "-"
	}
	d := time.Duration(*v) * time.Millisecond
	if d >= 24*time.Hour {
		return fmt.Sprintf("%.1fd", d.Hours()/24)
	}
	if d >= time.Hour {
		return fmt.Sprintf("%.1fh", d.Hours())
	}
	if d >= time.Minute {
		return fmt.Sprintf("%.0fm", d.Minutes())
	}
	return fmt.Sprintf("%.0fs", d.Seconds())
}

func formatPct(v *float64) string {
	if v == nil {
		return "-"
	}
	return fmt.Sprintf("%.0f%%", *v*100)
}

// formatDelta renders the PoP delta (cur-prev with a direction arrow; a base
// of 0 is recorded as "+N").
func formatDelta(cur, prev int64) string {
	switch {
	case prev <= 0:
		return fmt.Sprintf("+%d", cur)
	case cur > prev:
		return fmt.Sprintf("+%d（+%d%%）", cur-prev, (cur-prev)*100/prev)
	case cur < prev:
		return fmt.Sprintf("-%d（-%d%%）", prev-cur, (prev-cur)*100/prev)
	default:
		return "持平"
	}
}
