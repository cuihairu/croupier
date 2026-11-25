package incident

// Extra coverage for report.go / report_generator.go (failpoint injection
// scope): every "model-layer error passthrough" arm on the aggregation chains.
// The query order is deterministic (periodStats cur/prev/yoy -> epoch ->
// categoryShare/categoryIndex -> groupStats -> preWindowRows ->
// unattributedList), so injecting on the n-th query for a table pins each arm
// precisely; the report upsert's unique-conflict recovery arm uses a Before
// create callback to pre-insert a same-key row that genuinely hits the unique
// index.
//
// Honestly skipped (defensive arms where json.Marshal cannot fail): the
// Marshal error arm of upsertReportRow (report_generator.go 305-306) and the
// Marshal error arm of writePushStatus (343-344) -- all fields of
// ReportPayload/PushStatusEntry are string/bool/time/serializable structs, so
// Marshal always succeeds; filterAndSortLeaderboard covers the zero-count
// filter arm via a direct test of the pure function.

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// insertConflictReportBeforeCreate lands a same-key row (autocommitted)
// before the create transaction opens, so the subsequent Create genuinely hits
// the unique index (simulating a concurrent double-write race window). It must
// hook Before("gorm:begin_transaction"): if hooked at the create stage the
// default transaction is already open and Session(NewDB) inherits the
// transaction connection, so the injected row would roll back together with
// the failed Create and the recovery query would come up empty. At this stage
// the Schema is not resolved yet, so the target Create is identified by the
// concrete type of Dest; with a single-connection pool no connection is held
// at this point, so Exec can safely autocommit.
func insertConflictReportBeforeCreate(t *testing.T, db *gorm.DB, periodType, periodKey string) {
	t.Helper()
	require.NoError(t, db.Callback().Create().Before("gorm:begin_transaction").
		Register("test:conflict_incident_reports", func(tx *gorm.DB) {
			if _, ok := tx.Statement.Dest.(*model.IncidentReport); !ok {
				return
			}
			if err := db.Exec(
				"INSERT INTO incident_reports (created_at, updated_at, period_type, period_start, report_kind, payload, generated_at) "+
					"VALUES (datetime('now'), datetime('now'), ?, ?, 'summary', '{}', datetime('now'))",
				periodType, periodKey).Error; err != nil {
				_ = tx.AddError(err)
			}
		}))
}

// ---- report.go: summary chain ----

func TestReportSummary_ErrorArms(t *testing.T) {
	cases := []struct {
		name string
		n    int // inject on the n-th incidents query
	}{
		{"cur-scan", 1},   // periodStats(cur) scan fails
		{"prev-scan", 2},  // periodStats(prev) scan fails
		{"yoy-scan", 3},   // periodStats(yoy) scan fails
		{"epoch-scan", 4}, // oldestTime failure is swallowed (treated as no base point) -> the report still succeeds
		{"share-scan", 5}, // categoryShare's incidents scan fails
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := setupService(t)
			applyDBFailpoint(t, s.svcCtx.DB, dbFailpoint{table: "incidents", op: "query", n: tc.n, err: errFailpoint})
			_, err := s.ReportSummary(context.Background(), PeriodWeek, "")
			if tc.name == "epoch-scan" {
				require.NoError(t, err, "epoch 读取失败按无基点处理，不阻塞报告")
				return
			}
			require.ErrorIs(t, err, errFailpoint)
		})
	}
}

func TestReportSummary_BugsAndCategoryIndexErrors(t *testing.T) {
	ctx := context.Background()

	// periodStats's bug count fails (after the incidents scan succeeds)
	s := setupService(t)
	applyDBFailpoint(t, s.svcCtx.DB, dbFailpoint{table: "bugs", op: "query", n: 1, err: errFailpoint})
	_, err := s.ReportSummary(ctx, PeriodWeek, "")
	require.ErrorIs(t, err, errFailpoint)

	// categoryShare's bugs scan fails (the 5th bugs query)
	s = setupService(t)
	applyDBFailpoint(t, s.svcCtx.DB, dbFailpoint{table: "bugs", op: "query", n: 5, err: errFailpoint})
	_, err = s.ReportSummary(ctx, PeriodWeek, "")
	require.ErrorIs(t, err, errFailpoint)

	// categoryIndex's category listing fails
	s = setupService(t)
	applyDBFailpoint(t, s.svcCtx.DB, dbFailpoint{table: "incident_categories", op: "query", n: 1, err: errFailpoint})
	_, err = s.ReportSummary(ctx, PeriodWeek, "")
	require.ErrorIs(t, err, errFailpoint)
}

// ---- report.go: trend chain ----

func TestReportTrend_ErrorArms(t *testing.T) {
	ctx := context.Background()

	// Current-window incidents / bugs / categoryIndex failures
	s := setupService(t)
	applyDBFailpoint(t, s.svcCtx.DB, dbFailpoint{table: "incidents", op: "query", n: 1, err: errFailpoint})
	_, err := s.ReportTrend(ctx, BucketDay, nil, nil)
	require.ErrorIs(t, err, errFailpoint)

	s = setupService(t)
	applyDBFailpoint(t, s.svcCtx.DB, dbFailpoint{table: "bugs", op: "query", n: 1, err: errFailpoint})
	_, err = s.ReportTrend(ctx, BucketDay, nil, nil)
	require.ErrorIs(t, err, errFailpoint)

	s = setupService(t)
	applyDBFailpoint(t, s.svcCtx.DB, dbFailpoint{table: "incident_categories", op: "query", n: 1, err: errFailpoint})
	_, err = s.ReportTrend(ctx, BucketDay, nil, nil)
	require.ErrorIs(t, err, errFailpoint)
}

// YoY window: after the data epoch predates one year (the epoch gate passes),
// the YoY trendPoints incidents scan (the 3rd query) fails.
func TestReportTrend_YoyScanError(t *testing.T) {
	s := setupService(t)
	old := time.Now().Add(-400 * 24 * time.Hour)
	seedIncident(t, s, 1, old, time.Time{}, "", "", "", "")
	applyDBFailpoint(t, s.svcCtx.DB, dbFailpoint{table: "incidents", op: "query", n: 3, err: errFailpoint})
	_, err := s.ReportTrend(context.Background(), BucketDay, nil, nil)
	require.ErrorIs(t, err, errFailpoint)
}

// ---- report.go: leaderboard chain ----

func TestReportLeaderboard_BadPeriod(t *testing.T) {
	s := setupService(t)
	_, err := s.ReportLeaderboard(context.Background(), "bogus", "", ViewResponsible, BoardIncidents)
	require.Error(t, err)
}

func TestReportLeaderboard_ErrorArms(t *testing.T) {
	cases := []struct {
		name string
		n    int
	}{
		{"cur-scan", 1},  // groupStats(cur) scan fails
		{"prev-scan", 2}, // groupStats(prev) scan fails
		{"yoy-scan", 3},  // groupStats(yoy) scan fails
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := setupService(t)
			applyDBFailpoint(t, s.svcCtx.DB, dbFailpoint{table: "incidents", op: "query", n: tc.n, err: errFailpoint})
			_, err := s.ReportLeaderboard(context.Background(), PeriodWeek, "", ViewResponsible, BoardIncidents)
			require.ErrorIs(t, err, errFailpoint)
		})
	}
}

func TestReportLeaderboard_BugsAndCategoryIndexErrors(t *testing.T) {
	ctx := context.Background()

	// groupStats's bugs scan fails (after scanIncidents succeeds)
	s := setupService(t)
	applyDBFailpoint(t, s.svcCtx.DB, dbFailpoint{table: "bugs", op: "query", n: 1, err: errFailpoint})
	_, err := s.ReportLeaderboard(ctx, PeriodWeek, "", ViewResponsible, BoardIncidents)
	require.ErrorIs(t, err, errFailpoint)

	// groupStats's category index fails
	s = setupService(t)
	applyDBFailpoint(t, s.svcCtx.DB, dbFailpoint{table: "incident_categories", op: "query", n: 1, err: errFailpoint})
	_, err = s.ReportLeaderboard(ctx, PeriodWeek, "", ViewResponsible, BoardIncidents)
	require.ErrorIs(t, err, errFailpoint)
}

// Zero-count rows do not enter the leaderboard (the incidents+bugs==0 filter
// arm, tested directly as a pure function).
func TestFilterAndSortLeaderboard_DropsZeroRowsOnDefaultBoard(t *testing.T) {
	out := filterAndSortLeaderboard([]LeaderboardRow{
		{Key: "zero", Incidents: 0, Bugs: 0, DurationMS: 9},
		{Key: "hit", Incidents: 2, Bugs: 1},
	}, BoardIncidents)
	require.Len(t, out, 1)
	require.Equal(t, "hit", out[0].Key)
}

// ---- report.go: responsibility chain (12 incidents queries injected one by one) ----

func TestReportResponsibility_ErrorArms(t *testing.T) {
	cases := []struct {
		name string
		n    int
	}{
		{"cur-period", 1},    // periodStats(cur)
		{"prev-period", 2},   // periodStats(prev)
		{"yoy-period", 3},    // periodStats(yoy)
		{"vc-cur", 5},        // groupStats(category) cur
		{"vc-prev", 6},       // groupStats(category) prev
		{"vc-yoy", 7},        // groupStats(category) yoy
		{"vr-cur", 8},        // groupStats(responsible) cur
		{"vr-prev", 9},       // groupStats(responsible) prev
		{"vr-yoy", 10},       // groupStats(responsible) yoy
		{"pre-window", 11},   // preWindowRows failure is silently swallowed (the report still succeeds)
		{"unattributed", 12}, // unattributedList failure is passed through
	}
	for _, tc := range cases {
		if tc.n == 0 {
			continue
		}
		t.Run(tc.name, func(t *testing.T) {
			s := setupService(t)
			applyDBFailpoint(t, s.svcCtx.DB, dbFailpoint{table: "incidents", op: "query", n: tc.n, err: errFailpoint})
			resp, err := s.ReportResponsibility(context.Background(), PeriodWeek, "")
			if tc.name == "pre-window" {
				require.NoError(t, err, "期前窗口失败应静默降级")
				require.NotNil(t, resp)
				return
			}
			require.ErrorIs(t, err, errFailpoint)
		})
	}

	// categoryIndex (the 1st incident_categories query) and categoryLeaders
	// (the 2nd)
	s := setupService(t)
	applyDBFailpoint(t, s.svcCtx.DB, dbFailpoint{table: "incident_categories", op: "query", n: 1, err: errFailpoint})
	_, err := s.ReportResponsibility(context.Background(), PeriodWeek, "")
	require.ErrorIs(t, err, errFailpoint)

	s = setupService(t)
	applyDBFailpoint(t, s.svcCtx.DB, dbFailpoint{table: "incident_categories", op: "query", n: 2, err: errFailpoint})
	_, err = s.ReportResponsibility(context.Background(), PeriodWeek, "")
	require.ErrorIs(t, err, errFailpoint)
}

// ---- report_generator.go: generation chain ----

func TestGenerateReport_ErrorArms(t *testing.T) {
	cases := []struct {
		name  string
		table string
		n     int
	}{
		{"summary", "incidents", 1},               // ReportSummary fails
		{"slices-period", "incidents", 6},         // buildCategorySlices's periodStats
		{"slices-prevshare", "incidents", 7},      // prevShare's incidents scan
		{"slices-crit", "incidents", 9},           // unresolved critical scan
		{"slices-curshare", "incidents", 10},      // curShare's incidents scan
		{"slices-cats", "incident_categories", 3}, // category listing (before upsert)
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := setupGenerator(t)
			applyDBFailpoint(t, h.db, dbFailpoint{table: tc.table, op: "query", n: tc.n, err: errFailpoint})
			_, err := h.svc.GenerateReport(context.Background(), PeriodWeek, "")
			require.ErrorIs(t, err, errFailpoint)
		})
	}
}

func TestGenerateReport_CategoryIndexInsidePrevShare(t *testing.T) {
	h := setupGenerator(t)
	applyDBFailpoint(t, h.db, dbFailpoint{table: "incident_categories", op: "query", n: 2, err: errFailpoint})
	_, err := h.svc.GenerateReport(context.Background(), PeriodWeek, "")
	require.ErrorIs(t, err, errFailpoint)
}

// worsenedTwoPeriods history read failure: warns but does not block generation
// (generation itself succeeds).
func TestGenerateReport_WorsenedHistoryReadWarns(t *testing.T) {
	h := setupGenerator(t)
	qa := mustCategory(t, h.svc, model.IncidentCatQA)
	h.addIncident(t, qa.ID, "崩溃", time.Now(), model.IncidentSeverityWarning, model.IncidentRespOperator, "op-1")
	applyDBFailpoint(t, h.db, dbFailpoint{table: "incident_reports", op: "query", n: 1, err: errFailpoint})
	row, err := h.svc.GenerateReport(context.Background(), PeriodWeek, "")
	require.NoError(t, err, "历史读取失败只告警，不阻塞生成")
	require.NotNil(t, row)
}

// upsert's initial lookup fails (not NotFound) -> overall failure (the 2nd
// incident_reports query: the 1st is the worsened history read during category
// classification).
func TestGenerateReport_UpsertLookupError(t *testing.T) {
	h := setupGenerator(t)
	qa := mustCategory(t, h.svc, model.IncidentCatQA)
	h.addIncident(t, qa.ID, "崩溃", time.Now(), model.IncidentSeverityWarning, model.IncidentRespOperator, "op-2")
	applyDBFailpoint(t, h.db, dbFailpoint{table: "incident_reports", op: "query", n: 2, err: errFailpoint})
	_, err := h.svc.GenerateReport(context.Background(), PeriodWeek, "")
	require.ErrorIs(t, err, errFailpoint)
}

// Existing row -> upsert takes the Updates path; an Updates failure is passed
// through.
func TestGenerateReport_UpsertUpdateError(t *testing.T) {
	h := setupGenerator(t)
	ctx := context.Background()
	_, err := h.svc.GenerateReport(ctx, PeriodWeek, "")
	require.NoError(t, err)
	applyDBFailpoint(t, h.db, dbFailpoint{table: "incident_reports", op: "update", n: 1, err: errFailpoint})
	_, err = h.svc.GenerateReport(ctx, PeriodWeek, "")
	require.ErrorIs(t, err, errFailpoint)
}

// Create fails (not a unique conflict) -> passed through.
func TestGenerateReport_UpsertCreateError(t *testing.T) {
	h := setupGenerator(t)
	applyDBFailpoint(t, h.db, dbFailpoint{table: "incident_reports", op: "create", n: 1, err: errFailpoint})
	_, err := h.svc.GenerateReport(context.Background(), PeriodWeek, "")
	require.ErrorIs(t, err, errFailpoint)
}

// Concurrent double-write race window: inject a same-key row before Create ->
// hit the real unique index -> the conflict-recovery query finds it -> return
// the existing row (no error).
func TestGenerateReport_UpsertConflictRecovers(t *testing.T) {
	h := setupGenerator(t)
	ctx := context.Background()
	p, err := resolvePeriod(PeriodWeek, "", time.Now())
	require.NoError(t, err)
	insertConflictReportBeforeCreate(t, h.db, p.Type, p.Key)

	row, err := h.svc.GenerateReport(ctx, PeriodWeek, "")
	require.NoError(t, err, "唯一冲突应恢复为已存在行")
	require.NotNil(t, row)
	require.Equal(t, p.Key, row.PeriodStart)
}

// writePushStatus failure is passed through (after upsert succeeds).
func TestGenerateReport_WritePushStatusError(t *testing.T) {
	h := setupGenerator(t)
	applyDBFailpoint(t, h.db, dbFailpoint{table: "incident_reports", op: "update", n: 1, err: errFailpoint})
	_, err := h.svc.GenerateReport(context.Background(), PeriodWeek, "")
	require.ErrorIs(t, err, errFailpoint)
}

// Empty-name category fallback arm: piece.Name/Slug empty -> fall back to the
// category table values; the empty-name fallback arms of both the aggregate
// envelope and the piece envelope are covered as well; the worsened check
// short-circuits directly when slug is empty.
func TestGenerateReport_EmptyNameCategoryFallbacks(t *testing.T) {
	h := setupGenerator(t)
	ctx := context.Background()
	require.NoError(t, h.db.Exec(
		"INSERT INTO incident_categories (created_at, updated_at, name, slug, leader, enabled, sort) "+
			"VALUES (datetime('now'), datetime('now'), '', '', 'boss', 1, 99)").Error)
	var cat model.IncidentCategory
	require.NoError(t, h.db.Where("slug = ''").First(&cat).Error)
	seedIncident(t, h.svc, cat.ID, time.Now().Add(-2*time.Hour), time.Now().Add(-time.Hour),
		"", "", model.IncidentRespOperator, "op-9")

	row, err := h.svc.GenerateReport(ctx, PeriodWeek, "")
	require.NoError(t, err)
	require.NotNil(t, row)

	var payload ReportPayload
	require.NoError(t, json.Unmarshal(row.Payload, &payload))
	var slice *ReportCategorySlice
	for i := range payload.Categories {
		if payload.Categories[i].CategoryID == cat.ID {
			slice = &payload.Categories[i]
		}
	}
	require.NotNil(t, slice)
	require.Equal(t, "boss", slice.Leader)
}

func TestGenerateReport_ZeroCountPieceSkipped(t *testing.T) {
	h := setupGenerator(t)
	status := h.svc.distribute(context.Background(), PeriodWeek, "2026-W46",
		&ReportPayload{
			Summary:    &ReportSummaryResponse{},
			Categories: []ReportCategorySlice{{Slug: "zero", Count: 0, Leader: "boss", Level: model.MessageLevelInfo}},
			Level:      model.MessageLevelInfo,
		}, time.Now())
	for _, e := range status {
		require.NotEqual(t, "zero", e.Slug, "零量分片不投递")
	}
}

func TestAggregateAndSliceEvent_EmptyNameFallbacks(t *testing.T) {
	s := setupService(t)
	now := time.Now()
	agg := s.aggregateEvent("report:week:x", PeriodWeek, "x",
		&ReportPayload{
			Summary:    &ReportSummaryResponse{},
			Categories: []ReportCategorySlice{{Slug: "anon", Name: "", Count: 1, Level: model.MessageLevelWarn}},
			Level:      model.MessageLevelWarn,
		}, now)
	require.Contains(t, agg.Body, "anon", "空名回退到 slug")

	ev := s.sliceEvent("report:week:x", PeriodWeek, "x",
		&ReportCategorySlice{Slug: "anon2", Name: "", Count: 3, Level: model.MessageLevelInfo}, now)
	require.Equal(t, "category-leader:anon2", ev.Target)
}

func TestWorsenedTwoPeriods_EmptySlugShortCircuit(t *testing.T) {
	s := setupService(t)
	worsened, err := s.worsenedTwoPeriods(context.Background(), PeriodWeek, "2026-W46", "", 5)
	require.NoError(t, err)
	require.False(t, worsened)
}

// ---- report_generator.go: stored report list ----

func TestListStoredReports_CountAndFindErrors(t *testing.T) {
	h := setupGenerator(t)
	ctx := context.Background()

	applyDBFailpoint(t, h.db, dbFailpoint{table: "incident_reports", op: "query", n: 1, err: errFailpoint})
	_, err := h.svc.ListStoredReports(ctx, PeriodWeek, 1, 20)
	require.ErrorIs(t, err, errFailpoint)

	applyDBFailpoint(t, h.db, dbFailpoint{table: "incident_reports", op: "query", n: 2, err: errFailpoint})
	_, err = h.svc.ListStoredReports(ctx, PeriodWeek, 1, 20)
	require.ErrorIs(t, err, errFailpoint)
}

func TestRepushReport_MissingRowError(t *testing.T) {
	h := setupGenerator(t)
	_, err := h.svc.RepushReport(context.Background(), 999999)
	require.Error(t, err)
}

// The writePushStatus failure on the Repush path is passed through
// (report_generator 144-145).
func TestRepushReport_WritePushStatusError(t *testing.T) {
	h := setupGenerator(t)
	ctx := context.Background()
	row, err := h.svc.GenerateReport(ctx, PeriodWeek, "")
	require.NoError(t, err)

	applyDBFailpoint(t, h.db, dbFailpoint{table: "incident_reports", op: "update", n: 1, err: errFailpoint})
	_, err = h.svc.RepushReport(ctx, row.ID)
	require.ErrorIs(t, err, errFailpoint)
}
