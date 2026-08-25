package analytics

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/svc"
)

// setupInvocationsTestDB opens a fresh in-memory SQLite DB per test and
// creates the audit table (which also creates the promoted columns).
func setupInvocationsTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(gsqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	// Schema is normally created by the migration baseline; tests own their
	// fixtures.
	require.NoError(t, db.AutoMigrate(&audit.AuditModel{}))
	return db
}

func setupInvocationsSvcCtx(db *gorm.DB) *svc.ServiceContext {
	return &svc.ServiceContext{DB: db}
}

// seedInvocation writes one function.invoke audit record through the audit
// service, exercising the write-time promotion of game/env/function/duration.
func seedInvocation(t *testing.T, db *gorm.DB, gameID, env, functionID, outcome string, durationMs int64) {
	t.Helper()
	svcAudit := audit.NewAuditService(mustAuditStore(t, db), nil)
	details := map[string]interface{}{
		"function_id": functionID,
		"duration_ms": durationMs,
		"game_id":     gameID,
		"env":         env,
		"trace_id":    "trace-" + functionID,
	}
	_, err := svcAudit.Log(context.Background(), audit.EventFunctionInvoke,
		audit.WithActorID("tester", "user", "tester"),
		audit.WithResourceID("function", functionID),
		audit.WithDetails(details),
		audit.WithOutcome(outcome, ""),
	)
	require.NoError(t, err)
}

func mustAuditStore(t *testing.T, db *gorm.DB) *audit.SQLAuditStore {
	t.Helper()
	store, err := audit.NewSQLAuditStore(db)
	require.NoError(t, err)
	return store
}

func TestInvocationsSummary_ScopeFiltering(t *testing.T) {
	db := setupInvocationsTestDB(t)
	seedInvocation(t, db, "game-a", "prod", "player.ban", "success", 120)
	seedInvocation(t, db, "game-a", "prod", "player.ban", "failure", 80)
	seedInvocation(t, db, "game-b", "prod", "player.list", "success", 50)

	svcCtx := setupInvocationsSvcCtx(db)

	// Scoped to game-a: only its two rows count.
	scoped, err := invocationsSummary(context.Background(), svcCtx, &InvocationsSummaryRequest{GameId: "game-a", Env: "prod", Hours: 24})
	require.NoError(t, err)
	assert.Equal(t, int64(2), scoped.Total)
	assert.Equal(t, int64(1), scoped.Failed)
	assert.InDelta(t, 0.5, scoped.SuccessRate, 0.001)
	require.Len(t, scoped.TopFunctions, 1)
	assert.Equal(t, "player.ban", scoped.TopFunctions[0].FunctionID)
	assert.Equal(t, int64(2), scoped.TopFunctions[0].Total)
	assert.InDelta(t, 100.0, scoped.TopFunctions[0].AvgDurMs, 0.001)
	assert.InDelta(t, 100.0, scoped.AvgDurationMs, 0.001)

	// Unscoped: all three rows.
	unscoped, err := invocationsSummary(context.Background(), svcCtx, &InvocationsSummaryRequest{Hours: 24})
	require.NoError(t, err)
	assert.Equal(t, int64(3), unscoped.Total)
	assert.Len(t, unscoped.TopFunctions, 2)
}

func TestInvocationsSummary_LatencyPercentile(t *testing.T) {
	db := setupInvocationsTestDB(t)
	for i := 1; i <= 20; i++ {
		seedInvocation(t, db, "game-a", "prod", "fn", "success", int64(i*10))
	}
	// Rows without duration stay out of the latency stats.
	seedInvocation(t, db, "game-a", "prod", "fn", "success", 0)

	svcCtx := setupInvocationsSvcCtx(db)
	resp, err := invocationsSummary(context.Background(), svcCtx, &InvocationsSummaryRequest{GameId: "game-a", Hours: 24})
	require.NoError(t, err)
	assert.Equal(t, int64(21), resp.Total)
	assert.InDelta(t, 105.0, resp.AvgDurationMs, 0.001)
	assert.InDelta(t, 190.0, resp.P95DurationMs, 0.001)
}

func TestInvocationsList_TotalConsistentWithFunctionFilter(t *testing.T) {
	db := setupInvocationsTestDB(t)
	for i := 0; i < 15; i++ {
		seedInvocation(t, db, "game-a", "prod", "fn-x", "success", 10)
	}
	for i := 0; i < 7; i++ {
		seedInvocation(t, db, "game-a", "prod", "fn-y", "failure", 20)
	}

	svcCtx := setupInvocationsSvcCtx(db)

	// Regression: total must reflect the filtered set (previously the
	// unfiltered count), and every returned item must match the filter.
	resp, err := invocationsList(context.Background(), svcCtx, &InvocationsListRequest{
		FunctionId: "fn-x", Page: 1, PageSize: 10,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(15), resp.Total)
	assert.Len(t, resp.Items, 10)
	for _, item := range resp.Items {
		assert.Equal(t, "fn-x", item.FunctionID)
	}

	page2, err := invocationsList(context.Background(), svcCtx, &InvocationsListRequest{
		FunctionId: "fn-x", Page: 2, PageSize: 10,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(15), page2.Total)
	assert.Len(t, page2.Items, 5)

	// Scope + outcome compose with the function filter.
	filtered, err := invocationsList(context.Background(), svcCtx, &InvocationsListRequest{
		GameId: "game-a", Env: "prod", Outcome: "failure", Page: 1, PageSize: 20,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(7), filtered.Total)
	assert.Len(t, filtered.Items, 7)
	assert.Equal(t, "fn-y", filtered.Items[0].FunctionID)
	assert.Equal(t, "game-a", filtered.Items[0].GameId)
}

func TestInvocationsTrend_ScopedBuckets(t *testing.T) {
	db := setupInvocationsTestDB(t)
	seedInvocation(t, db, "game-a", "prod", "fn", "success", 10)
	seedInvocation(t, db, "game-a", "prod", "fn", "failure", 10)
	seedInvocation(t, db, "game-b", "prod", "fn", "success", 10)

	svcCtx := setupInvocationsSvcCtx(db)
	resp, err := invocationsTrend(context.Background(), svcCtx, &InvocationsTrendRequest{GameId: "game-a", Interval: "day"})
	require.NoError(t, err)
	require.Len(t, resp.Points, 1)
	assert.Equal(t, int64(2), resp.Points[0].Total)
	assert.Equal(t, int64(1), resp.Points[0].Failed)

	unscoped, err := invocationsTrend(context.Background(), svcCtx, &InvocationsTrendRequest{Interval: "day"})
	require.NoError(t, err)
	require.Len(t, unscoped.Points, 1)
	assert.Equal(t, int64(3), unscoped.Points[0].Total)
}

func TestAuditStore_BackfillsPromotedFieldsForLegacyRows(t *testing.T) {
	db := setupInvocationsTestDB(t)

	// Insert a legacy-style row: promoted columns empty, dimensions only in
	// details_json (as written before the promotion existed).
	details := `{"function_id":"legacy.fn","game_id":"g1","env":"prod","duration_ms":42,"trace_id":"t-1"}`
	err := db.Exec(
		"INSERT INTO audit_records (audit_id, timestamp, event_type, category, severity, details_json, outcome, chain_hash, chain_sequence, created_at) VALUES (?,?,?,?,?,?,?,?,?,?)",
		"legacy-1", time.Now(), string(audit.EventFunctionInvoke), "operational", "info", details, "success", "h1", 999001, time.Now(),
	).Error
	require.NoError(t, err)

	// Re-opening the store triggers the backfill.
	_ = mustAuditStore(t, db)

	var row struct {
		GameID     string
		Env        string
		FunctionID string
		DurationMs int64
	}
	require.NoError(t, db.Table("audit_records").Where("audit_id = ?", "legacy-1").
		Select("game_id, env, function_id, duration_ms").Scan(&row).Error)
	assert.Equal(t, "g1", row.GameID)
	assert.Equal(t, "prod", row.Env)
	assert.Equal(t, "legacy.fn", row.FunctionID)
	assert.Equal(t, int64(42), row.DurationMs)

	// Backfilled rows become visible to scoped analytics.
	svcCtx := setupInvocationsSvcCtx(db)
	resp, err := invocationsSummary(context.Background(), svcCtx, &InvocationsSummaryRequest{GameId: "g1", Env: "prod", Hours: 24})
	require.NoError(t, err)
	assert.Equal(t, int64(1), resp.Total)
}
