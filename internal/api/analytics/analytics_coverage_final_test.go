// Covers the remaining uncovered branches of the analytics package: query failure
// paths for overview/realtime/payments, levels truncation, buildPaths truncation,
// filtersUpdate extension save failure, payload marshal failure, the pipeline
// monitor's second-round trigger state transition, and warehouse connection error
// branches.
package analytics

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	extensioninstallation "github.com/cuihairu/croupier/internal/core/extension/installation"
	"github.com/cuihairu/croupier/internal/model"
	extensiongorm "github.com/cuihairu/croupier/internal/repo/gorm/extension"
	"github.com/cuihairu/croupier/internal/svc"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// newFinalCoverageDB opens a fresh isolated sqlite DB per test with the full
// analytics schema so query-failure injection does not disturb shared fixtures.
func newFinalCoverageDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", analyticsTestDBName("analytics_final"))
	db, err := gorm.Open(gsqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))
	return db
}

// failNthQuery registers gorm callbacks (query + row processors; Scan goes
// through the Row processor in gorm v1.31) that force the n-th query executed
// on db to fail, letting callers pinpoint a specific aggregate call.
func failNthQuery(t *testing.T, db *gorm.DB, n int) {
	t.Helper()
	counter := 0
	inject := func(tx *gorm.DB) {
		counter++
		if counter == n {
			_ = tx.AddError(errors.New("forced query failure"))
		}
	}
	require.NoError(t, db.Callback().Query().Before("gorm:query").Register("test/analytics_fail_nth_query", inject))
	require.NoError(t, db.Callback().Row().Before("gorm:row").Register("test/analytics_fail_nth_row", inject))
	t.Cleanup(func() {
		_ = db.Callback().Query().Remove("test/analytics_fail_nth_query")
		_ = db.Callback().Row().Remove("test/analytics_fail_nth_row")
	})
}

func newFullModelsContext(db *gorm.DB) *svc.ServiceContext {
	return &svc.ServiceContext{
		DB:            db,
		BehaviorModel: model.NewBehaviorModel(db),
		PaymentsModel: model.NewPaymentsModel(db),
		PlayerModel:   model.NewPlayerModel(db),
	}
}

// Overview's 2nd/3rd/6th/7th/8th queries failing each hit the MAU/activeUsers/
// DailyActivity/DailyRevenue/DailyNewPlayers error branches.
func TestOverview_AggregateQueryFailures(t *testing.T) {
	cases := []struct {
		name string
		nth  int
	}{
		{"mau_query_fails", 2},
		{"active_users_query_fails", 3},
		{"daily_activity_fails", 6},
		{"daily_revenue_fails", 7},
		{"daily_new_players_fails", 8},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := newFinalCoverageDB(t)
			failNthQuery(t, db, tc.nth)
			svcCtx := newFullModelsContext(db)
			req := &OverviewRequest{StartDate: "2026-01-01", EndDate: "2026-02-01"}
			_, err := overview(context.Background(), svcCtx, req)
			require.Error(t, err)
		})
	}
}

// Realtime's 2nd/3rd queries failing each hit the CountEvents/EventTypeCounts error branches.
func TestRealtime_AggregateQueryFailures(t *testing.T) {
	cases := []struct {
		name string
		nth  int
	}{
		{"count_events_fails", 2},
		{"event_type_counts_fails", 3},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := newFinalCoverageDB(t)
			failNthQuery(t, db, tc.nth)
			svcCtx := &svc.ServiceContext{BehaviorModel: model.NewBehaviorModel(db)}
			_, err := realtime(context.Background(), svcCtx, &RealtimeRequest{GameId: "g"})
			require.Error(t, err)
		})
	}
}

// realtimeSeries: the single window scan now fails on its first query.
func TestRealtimeSeries_UsersQueryFails(t *testing.T) {
	db := newFinalCoverageDB(t)
	failNthQuery(t, db, 1)
	svcCtx := &svc.ServiceContext{BehaviorModel: model.NewBehaviorModel(db)}
	_, err := realtimeSeries(context.Background(), svcCtx, &RealtimeSeriesRequest{
		GameId: "g", Interval: "1m", Duration: 5,
	})
	require.Error(t, err)
}

// payments: DailyRevenue fails after AggregateRevenue succeeds (2nd query without a BehaviorModel).
func TestPayments_DailyRevenueQueryFails(t *testing.T) {
	db := newFinalCoverageDB(t)
	failNthQuery(t, db, 2)
	svcCtx := &svc.ServiceContext{PaymentsModel: model.NewPaymentsModel(db)}
	_, err := payments(context.Background(), svcCtx, &PaymentsRequest{})
	require.Error(t, err)
}

// paymentsSummary: the first DailyRevenue query fails.
func TestPaymentsSummary_DailyRevenueQueryFails(t *testing.T) {
	db := newFinalCoverageDB(t)
	failNthQuery(t, db, 1)
	svcCtx := &svc.ServiceContext{PaymentsModel: model.NewPaymentsModel(db)}
	_, err := paymentsSummary(context.Background(), svcCtx, &PaymentsSummaryRequest{})
	require.Error(t, err)
}

// invocationsSummary: the audit table is missing -> the totals aggregate Scan fails.
func TestInvocationsSummary_TotalsScanFails(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", analyticsTestDBName("analytics_final"))), &gorm.Config{})
	require.NoError(t, err)
	svcCtx := &svc.ServiceContext{DB: db}
	_, err = invocationsSummary(context.Background(), svcCtx, &InvocationsSummaryRequest{})
	require.Error(t, err)
}

func seedBehaviorEvent(t *testing.T, svcCtx *svc.ServiceContext, eventType, userID, gameID string, data map[string]interface{}) {
	t.Helper()
	eventData := datatypes.JSONMap{}
	for k, v := range data {
		eventData[k] = v
	}
	require.NoError(t, svcCtx.BehaviorModel.RecordEvent(context.Background(), &model.BehaviorEvent{
		EventType:  eventType,
		UserID:     userID,
		GameID:     gameID,
		Data:       eventData,
		OccurredAt: time.Now().UTC(),
	}))
}

// levels: a level_attempt event without levelId -> that event is skipped.
func TestLevels_SkipsEventsWithoutLevelId(t *testing.T) {
	db := newFinalCoverageDB(t)
	svcCtx := newFullModelsContext(db)
	seedBehaviorEvent(t, svcCtx, "level_attempt", "u1", "g1", nil)

	resp, err := levels(context.Background(), svcCtx, &LevelsRequest{})
	require.NoError(t, err)
	assert.Empty(t, resp.Levels)
}

// levels: more than maxLevelEntries(100) levels -> truncated to 100.
// Note: ListEvents paging clamps pageSize to 100, so 103 distinct levelIds spread
// across two event types are needed to cross the truncation threshold.
func TestLevels_TruncatesToMaxEntries(t *testing.T) {
	db := newFinalCoverageDB(t)
	svcCtx := newFullModelsContext(db)
	for i := 0; i < 100; i++ {
		seedBehaviorEvent(t, svcCtx, "level_attempt", fmt.Sprintf("u%d", i), "g1", map[string]interface{}{
			"levelId": fmt.Sprintf("level-%d", i),
		})
	}
	for i := 100; i < 103; i++ {
		seedBehaviorEvent(t, svcCtx, "level_complete", fmt.Sprintf("u%d", i), "g1", map[string]interface{}{
			"levelId": fmt.Sprintf("level-%d", i),
		})
	}

	resp, err := levels(context.Background(), svcCtx, &LevelsRequest{})
	require.NoError(t, err)
	require.Len(t, resp.Levels, maxLevelEntries)
}

// levelsEpisodes: more than maxLevelEntries(100) episodes -> truncated to 100.
func TestLevelsEpisodes_TruncatesToMaxEntries(t *testing.T) {
	db := newFinalCoverageDB(t)
	svcCtx := newFullModelsContext(db)
	for i := 0; i < 100; i++ {
		seedBehaviorEvent(t, svcCtx, "episode_progress", fmt.Sprintf("u%d", i), "g1", map[string]interface{}{
			"episodeId": fmt.Sprintf("ep-%d", i),
		})
	}
	for i := 100; i < 103; i++ {
		seedBehaviorEvent(t, svcCtx, "episode_complete", fmt.Sprintf("u%d", i), "g1", map[string]interface{}{
			"episodeId": fmt.Sprintf("ep-%d", i),
		})
	}

	resp, err := levelsEpisodes(context.Background(), svcCtx, &LevelsEpisodesRequest{})
	require.NoError(t, err)
	require.Len(t, resp.Episodes, maxLevelEntries)
}

// toFloat's float32 and int branches.
func TestToFloat_Float32AndInt(t *testing.T) {
	v, ok := toFloat(float32(2.5))
	require.True(t, ok)
	assert.Equal(t, 2.5, v)

	v, ok = toFloat(7)
	require.True(t, ok)
	assert.Equal(t, 7.0, v)
}

// buildPaths: more than 50 paths -> truncated.
func TestBuildPaths_TruncatesOver50Results(t *testing.T) {
	events := make([]model.BehaviorEvent, 0, 60)
	for i := 0; i < 60; i++ {
		events = append(events, model.BehaviorEvent{
			EventType:  fmt.Sprintf("evt_%d", i),
			UserID:     fmt.Sprintf("u%d", i),
			OccurredAt: time.Now().UTC(),
		})
	}
	paths := buildPaths(events, 5)
	require.Len(t, paths, 50)
}

// addSequence: lazily initializes root.Children when it is nil.
func TestAddSequence_NilChildrenInitialized(t *testing.T) {
	root := &pathNode{}
	addSequence(root, []string{"step_a", "step_b"}, 3)
	require.NotNil(t, root.Children)
	child, ok := root.Children["step_a"]
	require.True(t, ok)
	assert.Equal(t, 1, child.Count)
	assert.NotNil(t, child.Children["step_b"])
}

// extractAnalyticsFiltersFromConfig: a non-serializable filters value -> marshal failure.
func TestExtractAnalyticsFiltersFromConfig_MarshalError(t *testing.T) {
	_, _, err := extractAnalyticsFiltersFromConfig(map[string]any{
		"filters": make(chan int),
	})
	require.Error(t, err)
}

// decodeEventsPayload: non-serializable raw -> marshal failure.
func TestDecodeEventsPayload_MarshalError(t *testing.T) {
	_, err := decodeEventsPayload(make(chan int))
	require.Error(t, err)
}

// decodeTransactionsPayload: non-serializable raw -> marshal failure.
func TestDecodeTransactionsPayload_MarshalError(t *testing.T) {
	_, err := decodeTransactionsPayload(make(chan int))
	require.Error(t, err)
}

// aggregateBy: keyFn returning an empty string -> falls back to the date string key.
func TestAggregateBy_EmptyKeyFallsBackToDate(t *testing.T) {
	day := time.Date(2026, 3, 14, 0, 0, 0, 0, time.UTC)
	items := aggregateBy([]model.DailyRevenueStat{{Day: day, Revenue: 12.5}}, func(time.Time) string {
		return ""
	})
	require.Len(t, items, 1)
	assert.Equal(t, "2026-03-14", items[0].Date)
	assert.Equal(t, 12.5, items[0].Revenue)
}

// filtersUpdate: UpdateConfig fails while saving the extension source (the dropped
// event table makes appendEvent fail) -> the error is passed through.
func TestFiltersUpdate_ExtensionSaveFails(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", analyticsTestDBName("analytics_final"))), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.ExtensionInstallation{}, &model.ExtensionRuntimeBinding{}, &model.ExtensionEvent{}))
	repos := extensiongorm.NewBundle(db)
	instSvc := extensioninstallation.NewService(repos.Installation, repos.Event, repos.Binding)
	_, err = instSvc.Install(context.Background(), extensioninstallation.InstallRequest{
		ExtensionID:    officialAnalyticsID,
		ReleaseVersion: "1.0.0",
		ScopeType:      "system",
		ScopeID:        "global",
		TargetType:     "agent_group",
		TargetID:       "default",
		Config: map[string]any{
			"filters": []map[string]any{
				{"gameId": "tower", "filters": map[string]any{"env": "prod"}},
			},
		},
		Operator: "tester",
	})
	require.NoError(t, err)
	require.NoError(t, db.Migrator().DropTable(&model.ExtensionEvent{}))

	svcCtx := &svc.ServiceContext{
		Extensions: &svc.ExtensionServices{Installation: instSvc},
	}
	_, err = filtersUpdate(context.Background(), svcCtx, &FiltersUpdateRequest{
		GameId:  "tower",
		Filters: map[string]interface{}{"env": "stage"},
	})
	require.Error(t, err)
}

// pipeline monitor: the same stalled scope fires two rounds in a row -> hits the
// fired[key] continue and the event_ prefix state cleanup branches.
func TestMonitorEventStreams_SecondRoundStillFiring(t *testing.T) {
	sink := newMonitorTestSink(t)
	stallRows := func() *scopeRowsV9 {
		return &scopeRowsV9{rows: [][]any{
			{"stall-game", "prod", int64(0), int64(1000)},
		}}
	}
	m := newScopeMonitorV9(sink, stallRows(), nil)
	require.NoError(t, m.Check(context.Background()))
	assert.True(t, m.states["event_stream_stalled:stall-game/prod"])

	// The second round uses fresh rows (the same scope is still stalled).
	m.conn = func() (warehouseConn, error) { return &stubConn{rows: stallRows()}, nil }
	require.NoError(t, m.Check(context.Background()))
	assert.True(t, m.states["event_stream_stalled:stall-game/prod"])
}

// fireAlert's nil-sink guard.
func TestMonitorFireAlert_NilSink(t *testing.T) {
	m := NewPipelineMonitor(nil, nil)
	m.fireAlert(context.Background(), "k", AlertTypeDataPipeline, "critical", "msg", nil)
}

// ---------------------------------------------------------------------------
// warehouse connection branches
// ---------------------------------------------------------------------------

func resetWarehouseConnectState() {
	warehouseOnce = sync.Once{}
	warehouseOpened = nil
	warehouseOpenErr = nil
}

// Invalid CLICKHOUSE_DSN -> the ParseDSN failure branch.
func TestDefaultWarehouseConnect_ParseDSNError(t *testing.T) {
	resetWarehouseConnectState()
	t.Cleanup(resetWarehouseConnectState)
	t.Setenv("CLICKHOUSE_DSN", "clickhouse://127.0.0.1:9000/%zz")

	conn, err := defaultWarehouseConnect()
	require.Error(t, err)
	require.ErrorContains(t, err, "parse clickhouse dsn")
	assert.Nil(t, conn)
}

// Valid DSN (lazily Opened) -> returns driverConnAdapter; its Query errors on the
// unreachable port, but the adapter forwarding branch has been exercised.
func TestDefaultWarehouseConnect_AdapterQueryDialsAndFails(t *testing.T) {
	resetWarehouseConnectState()
	t.Cleanup(resetWarehouseConnectState)
	t.Setenv("CLICKHOUSE_DSN", "clickhouse://127.0.0.1:1/default?dial_timeout=500ms")

	conn, err := defaultWarehouseConnect()
	require.NoError(t, err)
	require.NotNil(t, conn)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, err = conn.Query(ctx, "SELECT 1")
	require.Error(t, err)
}
