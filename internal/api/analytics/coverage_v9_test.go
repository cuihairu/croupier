package analytics

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/config"
	extensioninstallation "github.com/cuihairu/croupier/internal/core/extension/installation"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
	gin "github.com/gin-gonic/gin"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// ---------------------------------------------------------------------------
// Warehouse: generic connect failure is wrapped
// ---------------------------------------------------------------------------

func TestWarehouseQueryWrapsConnectErrorV9(t *testing.T) {
	orig := warehouseConnect
	warehouseConnect = func() (warehouseConn, error) { return nil, errors.New("dial boom") }
	t.Cleanup(func() { warehouseConnect = orig })

	_, err := NewService(nil).WarehouseDAU(context.Background(), &WarehouseDAURequest{})
	require.Error(t, err)
	assert.ErrorIs(t, err, errWarehouseQuery)
}

// ---------------------------------------------------------------------------
// Service helpers
// ---------------------------------------------------------------------------

func TestAggregateAgentMetricsSkipsNilAgentV9(t *testing.T) {
	store := registry.NewStore()
	store.Mu().Lock()
	store.AgentsUnsafe()["ghost-v9"] = nil
	store.Mu().Unlock()

	latency, errRate := aggregateAgentMetrics(store, "", "")
	assert.Equal(t, 0.0, latency)
	assert.Equal(t, 0.0, errRate)
}

func TestLoadBehaviorEventsEmptyTypesQueryErrorV9(t *testing.T) {
	db := setupAnalyticsFreshDB(t)
	require.NoError(t, db.Migrator().DropTable("behavior_events"))
	svcCtx := setupFullServiceTestContext(t, db)

	_, err := loadBehaviorEvents(context.Background(), svcCtx, "g", "prod",
		time.Now().Add(-time.Hour), time.Now(), []string{}, 10)
	require.Error(t, err)
}

// ---------------------------------------------------------------------------
// Invocations: scan errors over partially-shaped audit tables
// ---------------------------------------------------------------------------

func createPartialAuditDBV9(t *testing.T, columns string) *svc.ServiceContext {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec("CREATE TABLE audit_records ("+columns+")").Error)
	return &svc.ServiceContext{DB: db}
}

func TestInvocationsQueriesSurfaceScanErrorsV9(t *testing.T) {
	ctx := context.Background()

	// Missing every promoted column: the trend scan fails.
	empty := createPartialAuditDBV9(t, "id integer primary key")
	_, err := invocationsTrend(ctx, empty, &InvocationsTrendRequest{})
	require.Error(t, err)

	// Totals aggregate works, but duration_ms is missing → latency scan fails.
	noDuration := createPartialAuditDBV9(t, "id integer primary key, event_type text, game_id text, env text, outcome text, timestamp datetime, function_id text")
	_, err = invocationsSummary(ctx, noDuration, &InvocationsSummaryRequest{})
	require.Error(t, err)

	// Totals + latency fine, but function_id missing → top-function scan fails.
	noFunction := createPartialAuditDBV9(t, "id integer primary key, event_type text, game_id text, env text, outcome text, timestamp datetime, duration_ms integer")
	_, err = invocationsSummary(ctx, noFunction, &InvocationsSummaryRequest{})
	require.Error(t, err)

	// Count works but the detail select references missing columns.
	noDetails := createPartialAuditDBV9(t, "id integer primary key, event_type text, game_id text, env text, outcome text, timestamp datetime")
	_, err = invocationsList(ctx, noDetails, &InvocationsListRequest{})
	require.Error(t, err)
}

// ---------------------------------------------------------------------------
// Overview / realtime / ingest error branches
// ---------------------------------------------------------------------------

func TestOverviewRejectsInvalidRangeV9(t *testing.T) {
	db := setupAnalyticsFreshDB(t)
	svcCtx := setupFullServiceTestContext(t, db)
	_, err := overview(context.Background(), svcCtx, &OverviewRequest{
		GameId: "g", StartDate: "bad", EndDate: "2026-08-01",
	})
	require.Error(t, err)
}

func TestRealtimeAndSeriesModelErrorsV9(t *testing.T) {
	db := setupAnalyticsFreshDB(t)
	require.NoError(t, db.Migrator().DropTable("behavior_events"))
	svcCtx := setupFullServiceTestContext(t, db)

	_, err := realtime(context.Background(), svcCtx, &RealtimeRequest{GameId: "g"})
	require.Error(t, err)
	_, err = realtimeSeries(context.Background(), svcCtx, &RealtimeSeriesRequest{GameId: "g"})
	require.Error(t, err)
}

func TestIngestNilRequestAndRecordFailureV9(t *testing.T) {
	db := setupAnalyticsFreshDB(t)
	svcCtx := setupFullServiceTestContext(t, db)

	_, err := ingest(context.Background(), svcCtx, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "请求参数不能为空")

	require.NoError(t, db.Migrator().DropTable("behavior_events"))
	resp, err := ingest(context.Background(), svcCtx, &IngestRequest{
		GameId: "g", Env: "prod",
		Events: []map[string]interface{}{{"eventType": "login", "userId": "u-1"}},
	})
	require.NoError(t, err)
	assert.Equal(t, 0, resp.Accepted)
	assert.Equal(t, 1, resp.Rejected)
}

// ---------------------------------------------------------------------------
// Filters file error branches
// ---------------------------------------------------------------------------

func TestFiltersFileErrorBranchesV9(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir() // a directory: reading it fails

	// filtersGet with lock: read error surfaces.
	_, err := filtersGet(ctx, &svc.ServiceContext{
		Config:               config.Config{Registry: config.RegistryConfig{AnalyticsFiltersPath: dir}},
		AnalyticsFiltersLock: &sync.RWMutex{},
	}, &FiltersGetRequest{})
	require.Error(t, err)

	// filtersUpdate: locked read error.
	_, err = filtersUpdate(ctx, &svc.ServiceContext{
		Config:               config.Config{Registry: config.RegistryConfig{AnalyticsFiltersPath: dir}},
		AnalyticsFiltersLock: &sync.RWMutex{},
	}, &FiltersUpdateRequest{GameId: "g", Filters: map[string]any{}})
	require.Error(t, err)

	// filtersUpdate: unlocked read error.
	_, err = filtersUpdate(ctx, &svc.ServiceContext{
		Config: config.Config{Registry: config.RegistryConfig{AnalyticsFiltersPath: dir}},
	}, &FiltersUpdateRequest{GameId: "g", Filters: map[string]any{}})
	require.Error(t, err)

	// filtersUpdate: file present but invalid JSON → load parse error.
	badPath := filepath.Join(t.TempDir(), "bad.json")
	require.NoError(t, os.WriteFile(badPath, []byte("{nope"), 0o644))
	_, err = filtersUpdate(ctx, &svc.ServiceContext{
		Config: config.Config{Registry: config.RegistryConfig{AnalyticsFiltersPath: badPath}},
	}, &FiltersUpdateRequest{GameId: "g", Filters: map[string]any{}})
	require.Error(t, err)
}

func TestFiltersUpdateWriteFailureV9(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "filters.json")
	// A read-only dir forces WriteAnalyticsFiltersFile to fail when it tries
	// to create the (non-existent) filters file; the load step succeeds with
	// the default empty document.
	require.NoError(t, os.Chmod(dir, 0o500))
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

	_, err := filtersUpdate(context.Background(), &svc.ServiceContext{
		Config: config.Config{Registry: config.RegistryConfig{AnalyticsFiltersPath: path}},
	}, &FiltersUpdateRequest{GameId: "g", Filters: map[string]any{"env": "prod"}})
	require.Error(t, err)
}

func TestFiltersExtensionBadFiltersValueV9(t *testing.T) {
	svcCtx, installationSvc := newFiltersExtensionService(t)
	_, err := installationSvc.Install(context.Background(), extensioninstallation.InstallRequest{
		ExtensionID:    officialAnalyticsID,
		ReleaseVersion: "1.0.0",
		ScopeType:      "system",
		ScopeID:        "global",
		TargetType:     "agent_group",
		TargetID:       "default",
		Config:         map[string]any{"filters": 123},
		Operator:       "tester",
	})
	require.NoError(t, err)

	_, err = filtersGet(context.Background(), svcCtx, &FiltersGetRequest{})
	require.Error(t, err)
}

func TestFindActiveAnalyticsInstallationSkipsUninstalledV9(t *testing.T) {
	svcCtx, installationSvc := newFiltersExtensionService(t)
	_, err := installationSvc.Install(context.Background(), extensioninstallation.InstallRequest{
		ExtensionID:    officialAnalyticsID,
		ReleaseVersion: "1.0.0",
		ScopeType:      "system",
		ScopeID:        "global",
		TargetType:     "agent_group",
		TargetID:       "default",
		Config:         map[string]any{},
		Operator:       "tester",
	})
	require.NoError(t, err)

	require.NoError(t, svcCtx.DB.Model(&model.ExtensionInstallation{}).
		Where("1 = 1").Update("status", "uninstalled").Error)
	item, ok, err := findActiveAnalyticsInstallation(context.Background(), svcCtx)
	require.NoError(t, err)
	assert.False(t, ok)
	assert.Nil(t, item)
}

func TestFiltersUpdateHandlerSurfacesServiceErrorV9(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewHandler(NewService(&svc.ServiceContext{
		Config: config.Config{Registry: config.RegistryConfig{AnalyticsFiltersPath: t.TempDir()}},
	}), config.SSEConfig{})

	ctx, rec := newCoverageGinContext(http.MethodPost, "/api/v1/analytics/filters",
		`{"gameId":"g","filters":{"env":"prod"}}`)
	handler.FiltersUpdate(ctx)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

// ---------------------------------------------------------------------------
// Behavior error branches
// ---------------------------------------------------------------------------

func TestBehaviorErrorBranchesV9(t *testing.T) {
	ctx := context.Background()
	db := setupAnalyticsFreshDB(t)
	require.NoError(t, db.Migrator().DropTable("behavior_events", "feature_adoptions"))
	svcCtx := setupFullServiceTestContext(t, db)

	_, err := behaviorAnalytics(ctx, svcCtx, &BehaviorRequest{StartDate: "bad", EndDate: "nope"})
	require.Error(t, err)
	_, err = behaviorAnalytics(ctx, svcCtx, &BehaviorRequest{})
	require.Error(t, err)

	_, err = behaviorEvents(ctx, svcCtx, nil)
	require.Error(t, err)
	_, err = behaviorEvents(ctx, svcCtx, &BehaviorEventsRequest{StartDate: "bad"})
	require.Error(t, err)
	_, err = behaviorEvents(ctx, svcCtx, &BehaviorEventsRequest{})
	require.Error(t, err)

	_, err = behaviorAdoption(ctx, svcCtx, nil)
	require.Error(t, err)
	_, err = behaviorAdoption(ctx, svcCtx, &BehaviorAdoptionRequest{})
	require.Error(t, err)

	_, err = behaviorAdoptionBreakdown(ctx, svcCtx, nil)
	require.Error(t, err)
	_, err = behaviorAdoptionBreakdown(ctx, svcCtx, &BehaviorAdoptionBreakdownRequest{Feature: "f", StartDate: "bad"})
	require.Error(t, err)
	_, err = behaviorAdoptionBreakdown(ctx, svcCtx, &BehaviorAdoptionBreakdownRequest{Feature: "f"})
	require.Error(t, err)

	_, err = behaviorFunnel(ctx, svcCtx, nil)
	require.Error(t, err)
	_, err = behaviorFunnel(ctx, svcCtx, &BehaviorFunnelRequest{Steps: []string{"  "}})
	require.Error(t, err)
	_, err = behaviorFunnel(ctx, svcCtx, &BehaviorFunnelRequest{Steps: []string{"a"}, StartDate: "bad"})
	require.Error(t, err)
	_, err = behaviorFunnel(ctx, svcCtx, &BehaviorFunnelRequest{Steps: []string{"a"}})
	require.Error(t, err)

	_, err = behaviorPaths(ctx, svcCtx, nil)
	require.Error(t, err)
	_, err = behaviorPaths(ctx, svcCtx, &BehaviorPathsRequest{StartDate: "bad"})
	require.Error(t, err)
	_, err = behaviorPaths(ctx, svcCtx, &BehaviorPathsRequest{})
	require.Error(t, err)
}

func TestBehaviorFunnelProgressBreakV9(t *testing.T) {
	db := setupAnalyticsFreshDB(t)
	svcCtx := setupFullServiceTestContext(t, db)
	ctx := context.Background()
	base := time.Now().UTC().Add(-2 * time.Hour)
	for i := 0; i < 3; i++ {
		ev := createTestEvent("login", "u-1", "g", "prod", base.Add(time.Duration(i)*time.Minute), nil)
		require.NoError(t, svcCtx.BehaviorModel.RecordEvent(ctx, &ev))
	}

	resp, err := behaviorFunnel(ctx, svcCtx, &BehaviorFunnelRequest{GameId: "g", Steps: []string{"login"}})
	require.NoError(t, err)
	require.Len(t, resp.Steps, 1)
	assert.Equal(t, 1, resp.Steps[0].Users)
	assert.InDelta(t, 100.0, resp.Steps[0].ConversionRate, 0.001)
}

func TestBuildPathsSkipsBlankEventNamesV9(t *testing.T) {
	paths := buildPaths([]model.BehaviorEvent{
		{UserID: "u-1", EventType: "   ", OccurredAt: time.Now()},
	}, 3)
	assert.Empty(t, paths)
}

// ---------------------------------------------------------------------------
// Retention / levels error and skip branches
// ---------------------------------------------------------------------------

func TestRetentionErrorBranchesV9(t *testing.T) {
	ctx := context.Background()
	db := setupAnalyticsFreshDB(t)
	svcCtx := setupFullServiceTestContext(t, db)

	_, err := retention(ctx, svcCtx, &RetentionRequest{StartDate: "bad"})
	require.Error(t, err)

	require.NoError(t, db.Migrator().DropTable("retention_cohorts"))
	_, err = retention(ctx, svcCtx, &RetentionRequest{})
	require.Error(t, err)
}

func TestRetentionFiltersFutureCohortsV9(t *testing.T) {
	db := setupAnalyticsFreshDB(t)
	svcCtx := setupFullServiceTestContext(t, db)
	ctx := context.Background()

	future := time.Now().UTC().Add(72 * time.Hour)
	require.NoError(t, svcCtx.RetentionModel.UpsertCohort(ctx, &model.RetentionCohort{
		GameID: "g", Env: "prod", Cohort: "future", Users: 5,
		Retention: datatypesJSON(`[1]`), WindowStart: future, WindowEnd: future.Add(time.Hour),
	}))

	resp, err := retention(ctx, svcCtx, &RetentionRequest{
		GameId: "g", Env: "prod",
		StartDate: time.Now().UTC().Add(-48 * time.Hour).Format("2006-01-02"),
		EndDate:   time.Now().UTC().Format("2006-01-02"),
	})
	require.NoError(t, err)
	assert.Empty(t, resp.Cohorts)
}

func TestLevelsAggregatesDurationAndRetriesV9(t *testing.T) {
	db := setupAnalyticsFreshDB(t)
	svcCtx := setupFullServiceTestContext(t, db)
	ctx := context.Background()
	base := time.Now().UTC().Add(-1 * time.Hour)
	attempt := createTestEvent("level_attempt", "u-1", "g", "prod", base, map[string]interface{}{
		"levelId": "lv-1", "duration": 120.5, "retries": 2,
	})
	require.NoError(t, svcCtx.BehaviorModel.RecordEvent(ctx, &attempt))

	resp, err := levels(ctx, svcCtx, &LevelsRequest{GameId: "g", Env: "prod"})
	require.NoError(t, err)
	require.Len(t, resp.Levels, 1)
	assert.InDelta(t, 120.5, resp.Levels[0].AvgDuration, 0.001)
	assert.InDelta(t, 2.0, resp.Levels[0].AvgRetries, 0.001)
}

func TestLevelsEpisodesErrorAndSkipBranchesV9(t *testing.T) {
	ctx := context.Background()
	db := setupAnalyticsFreshDB(t)
	svcCtx := setupFullServiceTestContext(t, db)

	_, err := levelsEpisodes(ctx, svcCtx, nil)
	require.Error(t, err)
	_, err = levelsEpisodes(ctx, svcCtx, &LevelsEpisodesRequest{StartDate: "bad"})
	require.Error(t, err)

	base := time.Now().UTC().Add(-1 * time.Hour)
	noID := createTestEvent("episode_progress", "u-1", "g", "prod", base, map[string]interface{}{"progress": 50})
	require.NoError(t, svcCtx.BehaviorModel.RecordEvent(ctx, &noID))

	resp, err := levelsEpisodes(ctx, svcCtx, &LevelsEpisodesRequest{GameId: "g"})
	require.NoError(t, err)
	assert.Empty(t, resp.Episodes)
}

func TestLevelsEpisodesTruncatesBeyondMaxEntriesV9(t *testing.T) {
	db := setupAnalyticsFreshDB(t)
	svcCtx := setupFullServiceTestContext(t, db)
	ctx := context.Background()
	base := time.Now().UTC().Add(-1 * time.Hour)
	for i := 0; i < maxLevelEntries+5; i++ {
		ev := createTestEvent("episode_progress", fmt.Sprintf("u-%d", i), "g", "prod",
			base.Add(time.Duration(i)*time.Second),
			map[string]interface{}{"episodeId": fmt.Sprintf("ep-%03d", i), "progress": 10})
		require.NoError(t, svcCtx.BehaviorModel.RecordEvent(ctx, &ev))
	}
	resp, err := levelsEpisodes(ctx, svcCtx, &LevelsEpisodesRequest{GameId: "g"})
	require.NoError(t, err)
	assert.Len(t, resp.Episodes, maxLevelEntries)
}

func TestLevelsMapsErrorAndSkipBranchesV9(t *testing.T) {
	ctx := context.Background()
	db := setupAnalyticsFreshDB(t)
	svcCtx := setupFullServiceTestContext(t, db)

	_, err := levelsMaps(ctx, svcCtx, nil)
	require.Error(t, err)
	_, err = levelsMaps(ctx, svcCtx, &LevelsMapsRequest{StartDate: "bad"})
	require.Error(t, err)

	base := time.Now().UTC().Add(-1 * time.Hour)
	noMap := createTestEvent("map_heat", "u-1", "g", "prod", base, map[string]interface{}{"x": 1.0})
	require.NoError(t, svcCtx.BehaviorModel.RecordEvent(ctx, &noMap))

	resp, err := levelsMaps(ctx, svcCtx, &LevelsMapsRequest{GameId: "g"})
	require.NoError(t, err)
	assert.Empty(t, resp.Maps)
}

func TestEventBoolAndToFloatVariantsV9(t *testing.T) {
	ev := model.BehaviorEvent{Data: datatypes.JSONMap{
		"floatFlag": float64(2), "intFlag": 3, "yesFlag": " yes ", "oneFlag": "1", "noFlag": "no",
	}}
	assert.True(t, eventBool(ev, "floatFlag"))
	assert.True(t, eventBool(ev, "intFlag"))
	assert.True(t, eventBool(ev, "yesFlag"))
	assert.True(t, eventBool(ev, "oneFlag"))
	assert.False(t, eventBool(ev, "noFlag"))
	assert.False(t, eventBool(ev, "missing"))

	f, ok := toFloat(json.Number("12.5"))
	assert.True(t, ok)
	assert.InDelta(t, 12.5, f, 0.001)
	_, ok = toFloat(json.Number("oops"))
	assert.False(t, ok)
	f, ok = toFloat(float32(1.5))
	assert.True(t, ok)
	assert.InDelta(t, 1.5, f, 0.001)
	f, ok = toFloat(int64(7))
	assert.True(t, ok)
	assert.InDelta(t, 7.0, f, 0.001)
	f, ok = toFloat(" 8.5 ")
	assert.True(t, ok)
	assert.InDelta(t, 8.5, f, 0.001)
	_, ok = toFloat("nope")
	assert.False(t, ok)
	_, ok = toFloat([]string{"x"})
	assert.False(t, ok)
}

// ---------------------------------------------------------------------------
// Payments error branches
// ---------------------------------------------------------------------------

func TestPaymentsErrorBranchesV9(t *testing.T) {
	ctx := context.Background()
	db := setupAnalyticsFreshDB(t)
	svcCtx := setupFullServiceTestContext(t, db)

	_, err := payments(ctx, svcCtx, nil)
	require.Error(t, err)
	_, err = payments(ctx, svcCtx, &PaymentsRequest{StartDate: "bad"})
	require.Error(t, err)
	require.NoError(t, db.Migrator().DropTable("payment_transactions"))
	_, err = payments(ctx, svcCtx, &PaymentsRequest{})
	require.Error(t, err)

	_, err = paymentsIngest(ctx, svcCtx, nil)
	require.Error(t, err)
	_, err = paymentsIngest(ctx, svcCtx, &PaymentsIngestRequest{GameId: "g", Transactions: "not-a-list"})
	require.Error(t, err)

	_, err = paymentsSummary(ctx, svcCtx, nil)
	require.Error(t, err)
	_, err = paymentsSummary(ctx, svcCtx, &PaymentsSummaryRequest{StartDate: "bad"})
	require.Error(t, err)

	_, err = paymentsTransactions(ctx, svcCtx, nil)
	require.Error(t, err)
	_, err = paymentsTransactions(ctx, svcCtx, &PaymentsTransactionsRequest{StartDate: "bad"})
	require.Error(t, err)
}

func TestPaymentsProductTrendFutureItemFilteredV9(t *testing.T) {
	db := setupAnalyticsFreshDB(t)
	svcCtx := setupFullServiceTestContext(t, db)
	ctx := context.Background()

	past := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	future := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)
	require.NoError(t, svcCtx.PaymentsModel.UpsertProductTrend(ctx, &model.ProductTrend{
		GameID: "g", Env: "prod", ProductID: "p-past", WindowStart: past, WindowEnd: past.Add(24 * time.Hour),
	}))
	require.NoError(t, svcCtx.PaymentsModel.UpsertProductTrend(ctx, &model.ProductTrend{
		GameID: "g", Env: "prod", ProductID: "p-future", WindowStart: future, WindowEnd: future.Add(24 * time.Hour),
	}))

	resp, err := paymentsProductTrend(ctx, svcCtx, &PaymentsProductTrendRequest{
		GameId: "g", Env: "prod",
		StartDate: "2026-08-01", EndDate: "2026-08-31",
	})
	require.NoError(t, err)
	require.Len(t, resp.Items, 1)
	assert.Equal(t, "p-past", resp.Items[0].ProductId)

	require.NoError(t, db.Migrator().DropTable("payment_product_trends"))
	_, err = paymentsProductTrend(ctx, svcCtx, &PaymentsProductTrendRequest{GameId: "g"})
	require.Error(t, err)
}
