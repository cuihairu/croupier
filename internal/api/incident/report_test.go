package incident

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/gin-gonic/gin"
)

// seedIncident lands an incident row directly in the database (bypassing
// registration validation, so timestamps are precisely controllable).
func seedIncident(t *testing.T, s *Service, catID uint, detected, resolved time.Time,
	refType, refID, respType, respID string) {
	t.Helper()
	row := &model.Incident{
		Title: "seed", CategoryID: catID, Severity: model.IncidentSeverityWarning,
		Source: model.IncidentSourceManual, DetectedAt: detected,
		ResponsibleType: respType, ResponsibleID: respID,
		RefType: refType, RefID: refID,
	}
	if resolved.IsZero() {
		row.Status = model.IncidentStatusOpen
	} else {
		row.Status = model.IncidentStatusResolved
		row.ResolvedAt = &resolved
	}
	if err := s.svcCtx.DB.Create(row).Error; err != nil {
		t.Fatalf("seed incident: %v", err)
	}
}

func seedBug(t *testing.T, s *Service, catID uint, created time.Time) {
	t.Helper()
	if err := s.svcCtx.DB.Create(&model.Bug{
		Title: "bug", CategoryID: catID, CreatedAt: created,
	}).Error; err != nil {
		t.Fatalf("seed bug: %v", err)
	}
}

// ---- Period parsing ----

func TestResolvePeriod_WeekTriplet(t *testing.T) {
	now := time.Now()
	p, err := resolvePeriod(PeriodWeek, "", now)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if p.Type != PeriodWeek {
		t.Fatalf("type = %q", p.Type)
	}
	if p.Start.Weekday() != time.Monday {
		t.Fatalf("start weekday = %v, want Monday", p.Start.Weekday())
	}
	if !p.End.Equal(p.Start.AddDate(0, 0, 7)) {
		t.Fatalf("end = %v, want start+7d", p.End)
	}
	if !p.PrevStart.Equal(p.Start.AddDate(0, 0, -7)) || !p.PrevEnd.Equal(p.Start) {
		t.Fatalf("prev = [%v,%v)", p.PrevStart, p.PrevEnd)
	}
	// YoY = the same ISO week number last year
	if p.YoyStart.Weekday() != time.Monday {
		t.Fatalf("yoy start weekday = %v, want Monday", p.YoyStart.Weekday())
	}
	if p.YoyStart.Year() != p.Start.Year()-1 {
		t.Fatalf("yoy year = %d, want %d", p.YoyStart.Year(), p.Start.Year()-1)
	}
	// Key format YYYY-Www
	if len(p.Key) != 8 || p.Key[4] != '-' || p.Key[5] != 'W' {
		t.Fatalf("key = %q, want YYYY-Www", p.Key)
	}
}

func TestResolvePeriod_YearHasNoYoy(t *testing.T) {
	p, err := resolvePeriod(PeriodYear, "2026", time.Now())
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if !p.YoyMissing {
		t.Fatal("year period should have YoyMissing=true")
	}
	if !p.Start.Equal(time.Date(2026, 1, 1, 0, 0, 0, 0, time.Local)) {
		t.Fatalf("start = %v", p.Start)
	}
}

func TestResolvePeriod_BadInputs(t *testing.T) {
	for _, tc := range []struct {
		typ, key string
	}{
		{"decade", ""},
		{PeriodWeek, "2026-W99"},
		{PeriodWeek, "2026-W0"},
		{PeriodMonth, "2026-13"},
		{PeriodQuarter, "2026-Q5"},
		{PeriodYear, "26"},
	} {
		if _, err := resolvePeriod(tc.typ, tc.key, time.Now()); err == nil {
			t.Fatalf("resolvePeriod(%q,%q) should fail", tc.typ, tc.key)
		}
	}
}

// ---- Summary (stat cards) ----

func TestReportSummary_ThreePeriodComparison(t *testing.T) {
	s := setupService(t)
	ctx := context.Background()
	now := time.Now()
	p, err := resolvePeriod(PeriodWeek, "", now)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	qa := mustCategory(t, s, model.IncidentCatQA)
	ops := mustCategory(t, s, model.IncidentCatOps)

	// Current period: 2 incidents (2h/90min resolved) + 1 bug; each gets its
	// own ref so they do not hit a manual signature across periods (same
	// category re-firing within 7 days would wrongly form a recurrence chain)
	seedIncident(t, s, qa.ID, p.Start.Add(2*time.Hour), p.Start.Add(4*time.Hour), model.IncidentRefBug, "s-1", model.IncidentRespOperator, "alice")
	seedIncident(t, s, ops.ID, p.Start.Add(24*time.Hour), p.Start.Add(25*time.Hour+30*time.Minute), model.IncidentRefBug, "s-2", model.IncidentRespOperator, "bob")
	seedBug(t, s, qa.ID, p.Start.Add(3*time.Hour))
	// Previous period: 1 incident (4h) + 1 bug
	seedIncident(t, s, qa.ID, p.PrevStart.Add(1*time.Hour), p.PrevStart.Add(5*time.Hour), model.IncidentRefBug, "s-3", model.IncidentRespOperator, "alice")
	seedBug(t, s, ops.ID, p.PrevStart.Add(2*time.Hour))
	// Same week last year: 1 incident (1h)
	seedIncident(t, s, qa.ID, p.YoyStart.Add(1*time.Hour), p.YoyStart.Add(2*time.Hour), model.IncidentRefBug, "s-4", model.IncidentRespOperator, "carol")

	resp, err := s.ReportSummary(ctx, PeriodWeek, "")
	if err != nil {
		t.Fatalf("summary: %v", err)
	}
	if resp.PeriodKey != p.Key {
		t.Fatalf("key = %q want %q", resp.PeriodKey, p.Key)
	}
	if resp.Incidents != 2 || resp.Bugs != 1 {
		t.Fatalf("incidents/bugs = %d/%d, want 2/1", resp.Incidents, resp.Bugs)
	}
	m := resp.Metrics
	// Total: cur=3 prev=2 yoy=1
	if m.Total.Value == nil || *m.Total.Value != 3 {
		t.Fatalf("total cur = %v", m.Total.Value)
	}
	if m.Total.Prev == nil || m.Total.Prev.Value == nil || *m.Total.Prev.Value != 2 {
		t.Fatalf("total prev = %+v", m.Total.Prev)
	}
	if m.Total.Prev.Delta == nil || *m.Total.Prev.Delta != 1 {
		t.Fatalf("total prev delta = %+v", m.Total.Prev.Delta)
	}
	if m.Total.Prev.Pct == nil || *m.Total.Prev.Pct != 50 {
		t.Fatalf("total prev pct = %+v", m.Total.Prev.Pct)
	}
	if m.Total.Yoy == nil || m.Total.Yoy.Value == nil || *m.Total.Yoy.Value != 1 {
		t.Fatalf("total yoy = %+v", m.Total.Yoy)
	}
	if m.Total.Yoy.Delta == nil || *m.Total.Yoy.Delta != 2 {
		t.Fatalf("total yoy delta = %+v", m.Total.Yoy.Delta)
	}
	if m.Total.Yoy.Pct == nil || *m.Total.Yoy.Pct != 200 {
		t.Fatalf("total yoy pct = %+v", m.Total.Yoy.Pct)
	}
	// MTTR: cur=(2h+90min)/2=105min, prev=4h, yoy=1h
	wantMTTR := float64((2*time.Hour + 90*time.Minute).Milliseconds() / 2)
	if m.MTTR.Value == nil || *m.MTTR.Value != wantMTTR {
		t.Fatalf("mttr cur = %v want %v", m.MTTR.Value, wantMTTR)
	}
	if m.MTTR.Prev == nil || m.MTTR.Prev.Value == nil || *m.MTTR.Prev.Value != float64(4*time.Hour.Milliseconds()) {
		t.Fatalf("mttr prev = %+v", m.MTTR.Prev)
	}
	if m.MTTR.Yoy == nil || m.MTTR.Yoy.Value == nil || *m.MTTR.Yoy.Value != float64(time.Hour.Milliseconds()) {
		t.Fatalf("mttr yoy = %+v", m.MTTR.Yoy)
	}
	// Recurrence rate: 0 chains / 2 resolved = 0
	if m.Recurrence.Value == nil || *m.Recurrence.Value != 0 {
		t.Fatalf("recurrence cur = %v", m.Recurrence.Value)
	}
	if resp.ResolvedSample != 2 || resp.RecurrenceChains != 0 {
		t.Fatalf("samples = %d/%d, want 2/0", resp.ResolvedSample, resp.RecurrenceChains)
	}
	// Category share: qa=2 (1 incident + 1 bug), ops=1
	if len(resp.Breakdown) != 2 {
		t.Fatalf("breakdown = %+v", resp.Breakdown)
	}
	if resp.Breakdown[0].CategoryID != qa.ID || resp.Breakdown[0].Count != 2 {
		t.Fatalf("breakdown[0] = %+v", resp.Breakdown[0])
	}
	if resp.Breakdown[0].SharePct == nil || *resp.Breakdown[0].SharePct > 66.67 || *resp.Breakdown[0].SharePct < 66.66 {
		t.Fatalf("breakdown[0] share = %v", resp.Breakdown[0].SharePct)
	}
}

func TestReportSummary_MissingWhenPeriodUncovered(t *testing.T) {
	s := setupService(t)
	ctx := context.Background()
	now := time.Now()
	p, err := resolvePeriod(PeriodWeek, "", now)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	qa := mustCategory(t, s, model.IncidentCatQA)
	// Seed only the current period: the previous period and the same week last
	// year have no data at all → missing (not coerced to 0, which would be a
	// fabricated number)
	seedIncident(t, s, qa.ID, p.Start.Add(1*time.Hour), p.Start.Add(2*time.Hour), "", "", model.IncidentRespOperator, "alice")

	resp, err := s.ReportSummary(ctx, PeriodWeek, "")
	if err != nil {
		t.Fatalf("summary: %v", err)
	}
	m := resp.Metrics
	if m.Total.Prev == nil || !m.Total.Prev.Missing {
		t.Fatalf("total prev should be missing: %+v", m.Total.Prev)
	}
	if m.Total.Yoy == nil || !m.Total.Yoy.Missing {
		t.Fatalf("total yoy should be missing: %+v", m.Total.Yoy)
	}
	if m.MTTR.Prev == nil || !m.MTTR.Prev.Missing {
		t.Fatalf("mttr prev should be missing (no samples): %+v", m.MTTR.Prev)
	}
	// Current period value behaves as usual
	if m.Total.Value == nil || *m.Total.Value != 1 {
		t.Fatalf("total cur = %v", m.Total.Value)
	}
}

func TestReportSummary_YoyMissingYearPeriod(t *testing.T) {
	s := setupService(t)
	qa := mustCategory(t, s, model.IncidentCatQA)
	p, err := resolvePeriod(PeriodYear, "2026", time.Now())
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	seedIncident(t, s, qa.ID, p.Start.Add(48*time.Hour), p.Start.Add(50*time.Hour), "", "", model.IncidentRespOperator, "alice")

	resp, err := s.ReportSummary(context.Background(), PeriodYear, "2026")
	if err != nil {
		t.Fatalf("summary: %v", err)
	}
	if resp.Metrics.Total.Yoy == nil || !resp.Metrics.Total.Yoy.Missing {
		t.Fatal("year period yoy should be missing")
	}
	// Period-over-period is fine (no data last year → also missing)
	if resp.Metrics.Total.Prev == nil || !resp.Metrics.Total.Prev.Missing {
		t.Fatalf("prev should be missing: %+v", resp.Metrics.Total.Prev)
	}
}

func TestReportSummary_OpenIncidentTail(t *testing.T) {
	s := setupService(t)
	ctx := context.Background()
	now := time.Now()
	current, err := resolvePeriod(PeriodWeek, "", now)
	if err != nil {
		t.Fatalf("resolve current week: %v", err)
	}
	// Query the previous complete week so the test cannot cross a week boundary
	// between seeding and ReportSummary's independent clock read.
	previousStart := current.Start.AddDate(0, 0, -7)
	year, week := previousStart.ISOWeek()
	period, err := resolvePeriod(PeriodWeek, fmt.Sprintf("%d-W%02d", year, week), now)
	if err != nil {
		t.Fatalf("resolve previous week: %v", err)
	}
	qa := mustCategory(t, s, model.IncidentCatQA)
	// Open incident in the previous complete week; its tail ends at period.End.
	seedIncident(t, s, qa.ID, period.Start.Add(time.Hour), time.Time{}, "", "", model.IncidentRespOperator, "alice")

	resp, err := s.ReportSummary(ctx, PeriodWeek, period.Key)
	if err != nil {
		t.Fatalf("summary: %v", err)
	}
	if resp.Metrics.Duration.Value == nil || *resp.Metrics.Duration.Value <= 0 {
		t.Fatalf("open incident should contribute duration: %+v", resp.Metrics.Duration.Value)
	}
	// Empty samples stay out of MTTR (resolved=0 → missing)
	if resp.Metrics.MTTR.Value != nil {
		t.Fatal("mttr should be null with zero resolved samples")
	}
}

// ---- Recurrence chains (§5.3) ----

func TestRecurrenceChains_RefSignature(t *testing.T) {
	base := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	periodStart := base
	periodEnd := base.AddDate(0, 0, 14)
	resolved := base.Add(2 * time.Hour)
	rows := []incidentAggRow{
		{DetectedAt: base, ResolvedAt: &resolved, Source: "alert", RefType: model.IncidentRefAlert, RefID: "a-1"},
		// re-fires 1 day after resolve → recurrence chain head (inside this period)
		{DetectedAt: base.AddDate(0, 0, 2), Source: "alert", RefType: model.IncidentRefAlert, RefID: "a-1"},
		// 3 days later again → same chain, not counted separately
		{DetectedAt: base.AddDate(0, 0, 5), Source: "alert", RefType: model.IncidentRefAlert, RefID: "a-1"},
		// different refID → different signature, not counted
		{DetectedAt: base.AddDate(0, 0, 2), Source: "alert", RefType: model.IncidentRefAlert, RefID: "a-2"},
	}
	if got := countRecurrenceChains(rows, periodStart, periodEnd); got != 1 {
		t.Fatalf("chains = %d, want 1 (head only counts once)", got)
	}
}

func TestRecurrenceChains_WindowBoundary(t *testing.T) {
	base := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	rt := base.Add(2 * time.Hour)
	first := incidentAggRow{DetectedAt: base, ResolvedAt: &rt, Source: "manual"}
	// The window is measured from resolve: re-firing exactly at resolve+7d is
	// outside the window (strictly less than)
	exact7 := []incidentAggRow{first, {DetectedAt: rt.Add(7 * 24 * time.Hour), Source: "manual"}}
	if got := countRecurrenceChains(exact7, base, base.AddDate(0, 0, 14)); got != 0 {
		t.Fatalf("chains at exactly resolve+7d = %d, want 0", got)
	}
	// resolve+7d−1min → inside the window
	within := []incidentAggRow{first, {DetectedAt: rt.Add(7*24*time.Hour - time.Minute), Source: "manual"}}
	if got := countRecurrenceChains(within, base, base.AddDate(0, 0, 14)); got != 1 {
		t.Fatalf("chains within window = %d, want 1", got)
	}
}

func TestRecurrenceChains_ManualSignatureByCategory(t *testing.T) {
	base := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	rt := base.Add(2 * time.Hour)
	rows := []incidentAggRow{
		{DetectedAt: base, ResolvedAt: &rt, CategoryID: 6, Subcategory: "漏测"},
		{DetectedAt: base.AddDate(0, 0, 1), CategoryID: 6, Subcategory: "漏测"},
		// same category, different subcategory → different signature
		{DetectedAt: base.AddDate(0, 0, 1), CategoryID: 6, Subcategory: "用例"},
	}
	if got := countRecurrenceChains(rows, base, base.AddDate(0, 0, 14)); got != 1 {
		t.Fatalf("chains = %d, want 1", got)
	}
}

func TestReportSummary_RecurrenceRate(t *testing.T) {
	s := setupService(t)
	ctx := context.Background()
	now := time.Now()
	p, err := resolvePeriod(PeriodWeek, "", now)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	qa := mustCategory(t, s, model.IncidentCatQA)
	// Automated-source signature: alert|a-1 re-fires 1 day after resolve →
	// 1 chain / 2 resolved
	base := p.Start.Add(2 * time.Hour)
	seedIncident(t, s, qa.ID, base, base.Add(time.Hour),
		model.IncidentRefAlert, "a-1", model.IncidentRespOperator, "alice")
	seedIncident(t, s, qa.ID, base.AddDate(0, 0, 1), base.AddDate(0, 0, 1).Add(time.Hour),
		model.IncidentRefAlert, "a-1", model.IncidentRespOperator, "alice")
	// The denominator requires resolved: adding one unresolved incident
	seedIncident(t, s, qa.ID, p.Start.Add(5*time.Hour), time.Time{},
		"", "", model.IncidentRespOperator, "bob")

	resp, err := s.ReportSummary(ctx, PeriodWeek, "")
	if err != nil {
		t.Fatalf("summary: %v", err)
	}
	if resp.RecurrenceChains != 1 {
		t.Fatalf("chains = %d, want 1", resp.RecurrenceChains)
	}
	// 2 resolved samples, 1 chain → 0.5
	if resp.Metrics.Recurrence.Value == nil || *resp.Metrics.Recurrence.Value != 0.5 {
		t.Fatalf("recurrence rate = %v, want 0.5", resp.Metrics.Recurrence.Value)
	}
}

// ---- Trend ----

func TestReportTrend_DayBucketsAndYoyMissing(t *testing.T) {
	s := setupService(t)
	ctx := context.Background()
	now := time.Now()
	qa := mustCategory(t, s, model.IncidentCatQA)
	from := now.AddDate(0, 0, -10)
	to := now
	seedIncident(t, s, qa.ID, from.Add(24*time.Hour), from.Add(25*time.Hour), "", "", model.IncidentRespOperator, "a")
	seedIncident(t, s, qa.ID, from.Add(48*time.Hour), from.Add(49*time.Hour), "", "", model.IncidentRespOperator, "a")
	seedBug(t, s, qa.ID, from.Add(48*time.Hour))

	resp, err := s.ReportTrend(ctx, BucketDay, &from, &to)
	if err != nil {
		t.Fatalf("trend: %v", err)
	}
	if len(resp.Points) != 2 {
		t.Fatalf("points = %+v", resp.Points)
	}
	for _, pt := range resp.Points {
		if len(pt.Bucket) != 10 { // 2006-01-02
			t.Fatalf("bucket key = %q", pt.Bucket)
		}
	}
	if resp.Points[1].Total != 2 { // bug + incident on the same day
		t.Fatalf("points[1].total = %d, want 2", resp.Points[1].Total)
	}
	// The data epoch falls within the last 10 days → no data for the same
	// window last year
	if !resp.YoyMissing || len(resp.YoyPoints) != 0 {
		t.Fatalf("yoyMissing = %v, yoyPoints = %+v", resp.YoyMissing, resp.YoyPoints)
	}
	if len(resp.Categories) != 1 || resp.Categories[0].ID != qa.ID {
		t.Fatalf("categories = %+v", resp.Categories)
	}
}

func TestReportTrend_YoyCoveredWithOlderData(t *testing.T) {
	s := setupService(t)
	ctx := context.Background()
	qa := mustCategory(t, s, model.IncidentCatQA)
	now := time.Now()
	from := now.AddDate(-2, 0, 0)
	to := now
	seedIncident(t, s, qa.ID, now.AddDate(-1, 0, -5), now.AddDate(-1, 0, -5).Add(time.Hour), "", "", model.IncidentRespOperator, "a")

	resp, err := s.ReportTrend(ctx, BucketMonth, &from, &to)
	if err != nil {
		t.Fatalf("trend: %v", err)
	}
	if resp.YoyMissing {
		t.Fatal("yoy should be covered (data exists 1y ago)")
	}
	if len(resp.YoyPoints) != 1 || resp.YoyPoints[0].Total != 1 {
		t.Fatalf("yoyPoints = %+v", resp.YoyPoints)
	}
}

// ---- Leaderboard ----

func TestReportLeaderboard_BoardsAndSort(t *testing.T) {
	s := setupService(t)
	ctx := context.Background()
	now := time.Now()
	p, err := resolvePeriod(PeriodWeek, "", now)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	qa := mustCategory(t, s, model.IncidentCatQA)
	ops := mustCategory(t, s, model.IncidentCatOps)
	// Current period: alice has 2 incidents (1 resolved in 2h), bob has 1
	// incident (4h), carol only 1 bug
	seedIncident(t, s, qa.ID, p.Start.Add(1*time.Hour), p.Start.Add(3*time.Hour), "", "", model.IncidentRespOperator, "alice")
	seedIncident(t, s, ops.ID, p.Start.Add(4*time.Hour), time.Time{}, "", "", model.IncidentRespOperator, "alice")
	seedIncident(t, s, qa.ID, p.Start.Add(5*time.Hour), p.Start.Add(9*time.Hour), "", "", model.IncidentRespOperator, "bob")
	seedBug(t, s, qa.ID, p.Start.Add(6*time.Hour)) // unattributed bug (empty assignee)
	// Previous period: alice has 1 incident
	seedIncident(t, s, qa.ID, p.PrevStart.Add(1*time.Hour), p.PrevStart.Add(2*time.Hour), "", "", model.IncidentRespOperator, "alice")

	// incidents board: alice=2, bob=1, unknown=1 (a bug with no assignee falls
	// into the unknown bucket)
	resp, err := s.ReportLeaderboard(ctx, PeriodWeek, "", ViewResponsible, BoardIncidents)
	if err != nil {
		t.Fatalf("leaderboard: %v", err)
	}
	if len(resp.Rows) != 3 {
		t.Fatalf("rows = %+v", resp.Rows)
	}
	if resp.Rows[0].Key != "alice" || resp.Rows[0].Value != 2 {
		t.Fatalf("row0 = %+v", resp.Rows[0])
	}
	if resp.Rows[0].Prev == nil || resp.Rows[0].Prev.Value == nil || *resp.Rows[0].Prev.Value != 1 {
		t.Fatalf("alice prev = %+v", resp.Rows[0].Prev)
	}
	if resp.Rows[0].Prev.Delta == nil || *resp.Rows[0].Prev.Delta != 1 {
		t.Fatalf("alice prev delta = %+v", resp.Rows[0].Prev.Delta)
	}

	// mttr board: carol/unknown have no resolved samples and stay off the
	// board; alice (2h) ranks before bob (4h)
	mttr, err := s.ReportLeaderboard(ctx, PeriodWeek, "", ViewResponsible, BoardMTTR)
	if err != nil {
		t.Fatalf("mttr board: %v", err)
	}
	if len(mttr.Rows) != 2 {
		t.Fatalf("mttr rows = %+v (empty samples must be excluded)", mttr.Rows)
	}
	if mttr.Rows[0].Key != "alice" || mttr.Rows[0].MTTRMS == nil || *mttr.Rows[0].MTTRMS != float64(2*time.Hour.Milliseconds()) {
		t.Fatalf("mttr row0 = %+v", mttr.Rows[0])
	}

	// duration board: the unknown bucket with 0 duration is filtered out
	dur, err := s.ReportLeaderboard(ctx, PeriodWeek, "", ViewResponsible, BoardDuration)
	if err != nil {
		t.Fatalf("duration board: %v", err)
	}
	for _, r := range dur.Rows {
		if r.Key == "unknown" {
			t.Fatal("zero-duration group should be filtered")
		}
	}

	// 类别视图：按 category_id 分组
	cat, err := s.ReportLeaderboard(ctx, PeriodWeek, "", ViewCategory, BoardIncidents)
	if err != nil {
		t.Fatalf("category board: %v", err)
	}
	if len(cat.Rows) != 2 { // qa=2 incidents+1 bug? 不：qa 2 incidents（alice+bob），ops 1 incident + 1 open
		t.Fatalf("cat rows = %+v", cat.Rows)
	}
	if cat.Rows[0].CategoryID != qa.ID || cat.Rows[0].Value != 3 {
		t.Fatalf("cat row0 = %+v (want qa=3: 2 inc + 1 bug)", cat.Rows[0])
	}
}

func TestReportLeaderboard_InvalidArgs(t *testing.T) {
	s := setupService(t)
	ctx := context.Background()
	if _, err := s.ReportLeaderboard(ctx, PeriodWeek, "", "nobody", BoardIncidents); err == nil {
		t.Fatal("bad view should fail")
	}
	if _, err := s.ReportLeaderboard(ctx, PeriodWeek, "", ViewResponsible, "velocity"); err == nil {
		t.Fatal("bad board should fail")
	}
}

// ---- 责任人报告 ----

func TestReportResponsibility_MatrixAndUnattributed(t *testing.T) {
	s := setupService(t)
	ctx := context.Background()
	now := time.Now()
	p, err := resolvePeriod(PeriodWeek, "", now)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	qa := mustCategory(t, s, model.IncidentCatQA)
	ops := mustCategory(t, s, model.IncidentCatOps)
	// alice 跨两类别各 1 条；1 条未归因
	seedIncident(t, s, qa.ID, p.Start.Add(1*time.Hour), p.Start.Add(3*time.Hour), "", "", model.IncidentRespOperator, "alice")
	seedIncident(t, s, ops.ID, p.Start.Add(4*time.Hour), p.Start.Add(6*time.Hour), "", "", model.IncidentRespOperator, "alice")
	seedIncident(t, s, qa.ID, p.Start.Add(7*time.Hour), p.Start.Add(9*time.Hour), "", "", model.IncidentRespUnknown, "")

	resp, err := s.ReportResponsibility(ctx, PeriodWeek, "")
	if err != nil {
		t.Fatalf("responsibility: %v", err)
	}
	// 总览：3 事故，2 已解决
	if resp.Overview.Total.Value == nil || *resp.Overview.Total.Value != 3 {
		t.Fatalf("overview total = %v", resp.Overview.Total.Value)
	}
	if resp.Overview.MTTR.Value == nil || *resp.Overview.MTTR.Value != float64(2*time.Hour.Milliseconds()) {
		t.Fatalf("overview mttr = %v", resp.Overview.MTTR.Value)
	}
	// 类别矩阵：qa=2（alice 1 + 未归因 1），ops=1
	if len(resp.Categories) != 2 {
		t.Fatalf("categories = %+v", resp.Categories)
	}
	qaRow := resp.Categories[0]
	if qaRow.CategoryID != qa.ID || qaRow.Count != 2 {
		t.Fatalf("cat row0 = %+v", qaRow)
	}
	if qaRow.ByResponsible["alice"] != 1 || qaRow.ByResponsible["unknown"] != 1 {
		t.Fatalf("cat row0 byResponsible = %+v", qaRow.ByResponsible)
	}
	// 责任人明细：alice total=2，跨两类
	if len(resp.Responsibles) != 2 { // alice + unknown
		t.Fatalf("responsibles = %+v", resp.Responsibles)
	}
	alice := resp.Responsibles[0]
	if alice.Key != "alice" || alice.Total != 2 {
		t.Fatalf("alice row = %+v", alice)
	}
	if alice.ByCategory[strconv.FormatUint(uint64(qa.ID), 10)] != 1 ||
		alice.ByCategory[strconv.FormatUint(uint64(ops.ID), 10)] != 1 {
		t.Fatalf("alice byCategory = %+v", alice.ByCategory)
	}
	// 未归因清单
	if len(resp.Unattributed) != 1 || resp.Unattributed[0].CategoryID != qa.ID {
		t.Fatalf("unattributed = %+v", resp.Unattributed)
	}
}

func TestReportResponsibility_OnlyWeekMonth(t *testing.T) {
	s := setupService(t)
	if _, err := s.ReportResponsibility(context.Background(), PeriodQuarter, ""); err == nil {
		t.Fatal("quarter should fail")
	}
	if _, err := s.ReportResponsibility(context.Background(), PeriodYear, ""); err == nil {
		t.Fatal("year should fail")
	}
}

// ---- HTTP 入口 ----

func TestReportHandlers_QueryParsing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := setupService(t)
	h := NewHandler(s)
	router := gin.New()
	rg := router.Group("/incident-reports")
	rg.GET("/summary", h.GetSummary)
	rg.GET("/trend", h.GetTrend)
	rg.GET("/leaderboard", h.GetLeaderboard)
	rg.GET("/responsibility", h.GetResponsibility)

	if w := doJSON(t, router, http.MethodGet, "/incident-reports/summary", ""); w.Code != http.StatusOK {
		t.Fatalf("summary = %d: %s", w.Code, w.Body.String())
	}
	if w := doJSON(t, router, http.MethodGet, "/incident-reports/summary?period=decade", ""); w.Code != http.StatusBadRequest {
		t.Fatalf("bad period = %d, want 400", w.Code)
	}
	if w := doJSON(t, router, http.MethodGet, "/incident-reports/trend?bucket=decade", ""); w.Code != http.StatusBadRequest {
		t.Fatalf("bad bucket = %d, want 400", w.Code)
	}
	if w := doJSON(t, router, http.MethodGet, "/incident-reports/leaderboard?period=month&view=nobody", ""); w.Code != http.StatusBadRequest {
		t.Fatalf("bad view = %d, want 400", w.Code)
	}
	if w := doJSON(t, router, http.MethodGet, "/incident-reports/responsibility?period=quarter", ""); w.Code != http.StatusBadRequest {
		t.Fatalf("quarter responsibility = %d, want 400", w.Code)
	}
	// 非法 key 形态 → 400
	if w := doJSON(t, router, http.MethodGet, "/incident-reports/summary?period=week&key=2026-W99", ""); w.Code != http.StatusBadRequest {
		t.Fatalf("bad key = %d, want 400", w.Code)
	}
}
