package analytics

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/config"
	extensioninstallation "github.com/cuihairu/croupier/internal/core/extension/installation"
	"github.com/cuihairu/croupier/internal/model"
	extensiongorm "github.com/cuihairu/croupier/internal/repo/gorm/extension"
	"github.com/cuihairu/croupier/internal/svc"
	gin "github.com/gin-gonic/gin"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// ---------------------------------------------------------------------------
// Shared helpers
// ---------------------------------------------------------------------------

func newCoverageGinContext(method, target, body string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(method, target, strings.NewReader(body))
	if body != "" {
		ctx.Request.Header.Set("Content-Type", "application/json")
	}
	return ctx, rec
}

// ---------------------------------------------------------------------------
// Invocations handlers (previously 0% covered)
// ---------------------------------------------------------------------------

func TestInvocationsHandlersReturnAggregates(t *testing.T) {
	db := setupInvocationsTestDB(t)
	now := time.Now().UTC()
	seedInvocation(t, db, "game-a", "prod", "player.ban", "success", 120)
	seedInvocation(t, db, "game-a", "prod", "player.ban", "failure", 80)
	_ = now

	handler := NewHandler(NewService(&svc.ServiceContext{DB: db}), config.SSEConfig{})

	ctx, rec := newCoverageGinContext(http.MethodGet, "/api/v1/analytics/invocations/trend?gameId=game-a&env=prod&interval=day", "")
	handler.InvocationsTrend(ctx)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"points"`)
	assert.Contains(t, rec.Body.String(), `"total":2`)

	ctx, rec = newCoverageGinContext(http.MethodGet, "/api/v1/analytics/invocations/summary?gameId=game-a", "")
	handler.InvocationsSummary(ctx)
	require.Equal(t, http.StatusOK, rec.Code)
	var summary InvocationsSummaryResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &summary))
	assert.Equal(t, int64(2), summary.Total)
	assert.Equal(t, int64(1), summary.Failed)
	assert.InDelta(t, 0.5, summary.SuccessRate, 0.001)
	require.NotEmpty(t, summary.TopFunctions)
	assert.Equal(t, "player.ban", summary.TopFunctions[0].FunctionID)

	ctx, rec = newCoverageGinContext(http.MethodGet, "/api/v1/analytics/invocations?gameId=game-a&outcome=failure&page=1&pageSize=10", "")
	handler.InvocationsList(ctx)
	require.Equal(t, http.StatusOK, rec.Code)
	var list InvocationsListResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &list))
	assert.Equal(t, int64(1), list.Total)
	require.Len(t, list.Items, 1)
	assert.Equal(t, "player.ban", list.Items[0].FunctionID)
	assert.Equal(t, "failure", list.Items[0].Outcome)
}

func TestInvocationsHandlersRejectMalformedJSON(t *testing.T) {
	db := setupInvocationsTestDB(t)
	handler := NewHandler(NewService(&svc.ServiceContext{DB: db}), config.SSEConfig{})

	cases := []struct {
		name string
		fn   func(*gin.Context)
	}{
		{"trend", handler.InvocationsTrend},
		{"summary", handler.InvocationsSummary},
		{"list", handler.InvocationsList},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, rec := newCoverageGinContext(http.MethodPost, "/api/v1/analytics/invocations", "{")
			tc.fn(ctx)
			assert.Equal(t, http.StatusBadRequest, rec.Code)
		})
	}
}

func TestInvocationTimeWindowDefaultsToHourly(t *testing.T) {
	layout, since := invocationTimeWindow("")
	assert.Equal(t, "2006-01-02 15:00:00", layout)
	assert.WithinDuration(t, time.Now().UTC().Add(-24*time.Hour), since, time.Minute)

	layout, since = invocationTimeWindow("day")
	assert.Equal(t, "2006-01-02", layout)
	assert.WithinDuration(t, time.Now().UTC().AddDate(0, 0, -30), since, time.Minute)
}

// ---------------------------------------------------------------------------
// Warehouse handlers (previously 0% covered)
// ---------------------------------------------------------------------------

func TestWarehouseHandlersReturnPoints(t *testing.T) {
	conn := &fakeConn{rows: [][]any{
		{time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC), "game-a", "prod", uint64(11), uint64(3)},
	}}
	withFakeConn(t, conn)
	handler := NewHandler(NewService(nil), config.SSEConfig{})

	ctx, rec := newCoverageGinContext(http.MethodGet, "/api/v1/analytics/warehouse/dau?gameId=game-a&days=7", "")
	handler.WarehouseDAU(ctx)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"dau":11`)

	conn = &fakeConn{rows: [][]any{
		{time.Date(2026, 8, 24, 6, 0, 0, 0, time.UTC), uint64(9)},
	}}
	withFakeConn(t, conn)
	ctx, rec = newCoverageGinContext(http.MethodGet, "/api/v1/analytics/warehouse/online?minutes=30", "")
	handler.WarehouseOnline(ctx)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"online":9`)

	conn = &fakeConn{rows: [][]any{
		{time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC), "game-a", "prod", uint64(100), uint64(5), uint64(1)},
	}}
	withFakeConn(t, conn)
	ctx, rec = newCoverageGinContext(http.MethodGet, "/api/v1/analytics/warehouse/revenue?gameId=game-a", "")
	handler.WarehouseRevenue(ctx)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"revenueCents":100`)
}

func TestWarehouseHandlersMapErrors(t *testing.T) {
	withDisabledWarehouse(t)
	handler := NewHandler(NewService(nil), config.SSEConfig{})

	for _, tc := range []struct {
		name string
		fn   func(*gin.Context)
	}{
		{"dau", handler.WarehouseDAU},
		{"online", handler.WarehouseOnline},
		{"revenue", handler.WarehouseRevenue},
	} {
		t.Run(tc.name+"_disabled", func(t *testing.T) {
			ctx, rec := newCoverageGinContext(http.MethodGet, "/api/v1/analytics/warehouse/"+tc.name, "")
			tc.fn(ctx)
			assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
		})
	}

	conn := &fakeConn{queryErr: fmt.Errorf("dial refused")}
	withFakeConn(t, conn)
	for _, tc := range []struct {
		name string
		fn   func(*gin.Context)
	}{
		{"dau", handler.WarehouseDAU},
		{"online", handler.WarehouseOnline},
		{"revenue", handler.WarehouseRevenue},
	} {
		t.Run(tc.name+"_failure", func(t *testing.T) {
			ctx, rec := newCoverageGinContext(http.MethodGet, "/api/v1/analytics/warehouse/"+tc.name, "")
			tc.fn(ctx)
			assert.Equal(t, http.StatusInternalServerError, rec.Code)
		})

		t.Run(tc.name+"_badrequest", func(t *testing.T) {
			ctx, rec := newCoverageGinContext(http.MethodPost, "/api/v1/analytics/warehouse/"+tc.name, "{")
			tc.fn(ctx)
			assert.Equal(t, http.StatusBadRequest, rec.Code)
		})
	}
}

func TestDefaultWarehouseConnectDisabledWithoutDSN(t *testing.T) {
	if os.Getenv("CLICKHOUSE_DSN") != "" {
		t.Skip("CLICKHOUSE_DSN is set; cannot assert disabled behaviour")
	}
	conn, err := defaultWarehouseConnect()
	require.Error(t, err)
	assert.ErrorIs(t, err, errWarehouseDisabled)
	assert.Nil(t, conn)
}

// ---------------------------------------------------------------------------
// Realtime SSE handler (previously 10% covered)
// ---------------------------------------------------------------------------

type sseCapture struct {
	gin.ResponseWriter
	mu     sync.Mutex
	writes chan string
	closed chan struct{}
	done   func()
}

func (w *sseCapture) Write(p []byte) (int, error) {
	w.mu.Lock()
	w.ResponseWriter.Write(p)
	w.mu.Unlock()
	select {
	case w.writes <- string(p):
	case <-w.closed:
	}
	return len(p), nil
}

func (w *sseCapture) WriteString(s string) (int, error) {
	return w.Write([]byte(s))
}

func (w *sseCapture) Flush() {}

func runRealtimeSSE(t *testing.T, svcCtx *svc.ServiceContext) []string {
	t.Helper()
	gin.SetMode(gin.TestMode)

	reqCtx, cancel := context.WithCancel(context.Background())
	rec := httptest.NewRecorder()
	ginCtx, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/analytics/realtime?gameId=demo&env=prod", nil).WithContext(reqCtx)
	ginCtx.Request = req
	baseWriter := ginCtx.Writer
	capture := &sseCapture{
		ResponseWriter: baseWriter,
		writes:         make(chan string, 16),
		closed:         make(chan struct{}),
		done:           cancel,
	}
	ginCtx.Writer = capture

	finished := make(chan struct{})
	go func() {
		defer close(finished)
		NewHandler(NewService(svcCtx), config.SSEConfig{UpdateInterval: 1}).Realtime(ginCtx)
	}()

	var chunks []string
	collect := func(n int) {
		for i := 0; i < n; i++ {
			select {
			case chunk := <-capture.writes:
				chunks = append(chunks, chunk)
			case <-time.After(5 * time.Second):
				cancel()
				<-finished
				t.Fatalf("timed out waiting for SSE write #%d", i+1)
			}
		}
	}

	collect(2) // connected event + data payload
	require.True(t, strings.HasPrefix(strings.Join(chunks, ""), "event: connected"))

	collect(2) // first tick: message event + data payload (or error event + data)
	joined := strings.Join(chunks, "")
	require.True(t, strings.Contains(joined, "event: "), "expected a tick event, got %q", joined)

	close(capture.closed)
	cancel()
	select {
	case <-finished:
	case <-time.After(5 * time.Second):
		t.Fatal("realtime handler did not return after cancellation")
	}
	return chunks
}

func TestRealtimeHandlerStreamsSuccessEventsThenStopsOnDisconnect(t *testing.T) {
	db := setupServiceTestDB(t)
	svcCtx := setupFullServiceTestContext(t, db)
	event := &model.BehaviorEvent{
		GameID:     "demo",
		Env:        "prod",
		EventType:  "login",
		UserID:     "u-1",
		OccurredAt: time.Now().UTC(),
	}
	require.NoError(t, svcCtx.BehaviorModel.RecordEvent(context.Background(), event))

	chunks := runRealtimeSSE(t, svcCtx)
	joined := strings.Join(chunks, "")
	assert.Contains(t, joined, "event: message")
	assert.Contains(t, joined, `"onlineUsers":1`)
}

func TestRealtimeHandlerEmitsErrorEventWhenServiceFails(t *testing.T) {
	chunks := runRealtimeSSE(t, &svc.ServiceContext{})
	joined := strings.Join(chunks, "")
	assert.Contains(t, joined, "event: error")
}

// ---------------------------------------------------------------------------
// Filters get/update branches
// ---------------------------------------------------------------------------

func TestFiltersGetUsesLockedReadAndRejectsInvalidFile(t *testing.T) {
	dir := t.TempDir()

	// Locked read path with a valid filters document.
	path := filepath.Join(dir, "filters.json")
	data, err := SaveAnalyticsFiltersJSON([]AnalyticsFilters{{GameId: "tower"}})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, data, 0o644))

	resp, err := filtersGet(context.Background(), &svc.ServiceContext{
		Config:               config.Config{Registry: config.RegistryConfig{AnalyticsFiltersPath: path}},
		AnalyticsFiltersLock: &sync.RWMutex{},
	}, &FiltersGetRequest{GameId: "tower"})
	require.NoError(t, err)
	require.Len(t, resp.Items, 1)
	assert.Equal(t, "tower", resp.Items[0].GameId)

	// Invalid JSON content surfaces the parse error on both lock states.
	badPath := filepath.Join(dir, "broken.json")
	require.NoError(t, os.WriteFile(badPath, []byte("{invalid"), 0o644))
	_, err = filtersGet(context.Background(), &svc.ServiceContext{
		Config: config.Config{Registry: config.RegistryConfig{AnalyticsFiltersPath: badPath}},
	}, &FiltersGetRequest{})
	require.Error(t, err)

	_, err = filtersGet(context.Background(), &svc.ServiceContext{
		Config:               config.Config{Registry: config.RegistryConfig{AnalyticsFiltersPath: badPath}},
		AnalyticsFiltersLock: &sync.RWMutex{},
	}, &FiltersGetRequest{})
	require.Error(t, err)
}

func TestLoadAnalyticsFiltersVariants(t *testing.T) {
	items, err := LoadAnalyticsFilters([]byte{})
	require.NoError(t, err)
	assert.Empty(t, items)

	items, err = LoadAnalyticsFilters([]byte(`{"items":[{"gameId":"a"},{"gameId":"a"},{"gameId":"  "}],"updatedAt":"x"}`))
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "a", items[0].GameId)

	_, err = LoadAnalyticsFilters([]byte("nope"))
	require.Error(t, err)
}

func TestFiltersUpdatePersistsFileSource(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "filters.json")

	svcCtx := &svc.ServiceContext{
		Config:               config.Config{Registry: config.RegistryConfig{AnalyticsFiltersPath: path}},
		AnalyticsFiltersLock: &sync.RWMutex{},
	}
	ctx := context.WithValue(context.Background(), "username", "ops-user")

	resp, err := filtersUpdate(ctx, svcCtx, &FiltersUpdateRequest{
		GameId:  "tower",
		Filters: map[string]any{"env": "prod"},
	})
	require.NoError(t, err)
	require.Len(t, resp.Items, 1)
	assert.Equal(t, "tower", resp.Items[0].GameId)

	// The update is durable and visible to subsequent reads.
	reread, err := filtersGet(ctx, svcCtx, &FiltersGetRequest{GameId: "tower"})
	require.NoError(t, err)
	require.Len(t, reread.Items, 1)

	// Upserting an existing game replaces its filters in place.
	_, err = filtersUpdate(ctx, svcCtx, &FiltersUpdateRequest{
		GameId:  "tower",
		Filters: map[string]any{"env": "stage"},
	})
	require.NoError(t, err)
	svcCtx.AnalyticsFiltersLock.RLock()
	raw, readErr := os.ReadFile(path)
	svcCtx.AnalyticsFiltersLock.RUnlock()
	require.NoError(t, readErr)
	assert.Contains(t, string(raw), "stage")
}

func TestFiltersGetExtensionInstallationErrorPaths(t *testing.T) {
	svcCtx, installationSvc := newFiltersExtensionService(t)

	_, err := installationSvc.Install(context.Background(), extensioninstallation.InstallRequest{
		ExtensionID:    officialAnalyticsID,
		ReleaseVersion: "1.0.0",
		ScopeType:      "system",
		ScopeID:        "global",
		TargetType:     "agent_group",
		TargetID:       "default",
		Config:         map[string]any{"filters": []map[string]any{{"gameId": "ext"}}},
		Operator:       "tester",
	})
	require.NoError(t, err)

	// Corrupt the stored config JSON so parsing fails.
	require.NoError(t, svcCtx.DB.Model(&model.ExtensionInstallation{}).
		Where("1 = 1").
		Update("config_json", "{not-json").Error)
	_, err = filtersGet(context.Background(), svcCtx, &FiltersGetRequest{})
	require.Error(t, err)

	// The same corruption surfaces through the update path.
	_, err = filtersUpdate(context.Background(), svcCtx, &FiltersUpdateRequest{
		GameId:  "tower",
		Filters: map[string]any{"env": "prod"},
	})
	require.Error(t, err)
}

// ---------------------------------------------------------------------------
// Overview / ingest error branches
// ---------------------------------------------------------------------------

func TestOverviewModelErrorBranches(t *testing.T) {
	cases := []struct {
		name         string
		droppedTable string
	}{
		{"dau_query", "behavior_events"},
		{"new_players", "players"},
		{"revenue_aggregate", "payment_transactions"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := setupAnalyticsFreshDB(t)
			require.NoError(t, db.Migrator().DropTable(tc.droppedTable))
			svcCtx := setupFullServiceTestContext(t, db)
			_, err := overview(context.Background(), svcCtx, &OverviewRequest{GameId: "g", Env: "prod"})
			require.Error(t, err)
		})
	}

	// Nil request is rejected before any model access.
	db := setupAnalyticsFreshDB(t)
	svcCtx := setupFullServiceTestContext(t, db)
	_, err := overview(context.Background(), svcCtx, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "请求参数不能为空")
}

func TestIngestRejectsMalformedPayloads(t *testing.T) {
	db := setupAnalyticsFreshDB(t)
	svcCtx := setupFullServiceTestContext(t, db)

	// Non-list payload fails decoding.
	_, err := ingest(context.Background(), svcCtx, &IngestRequest{
		GameId: "g",
		Events: "not-a-list",
	})
	require.Error(t, err)

	// Entries without userId are rejected but do not fail the batch.
	resp, err := ingest(context.Background(), svcCtx, &IngestRequest{
		GameId: "g",
		Env:    "prod",
		Events: []map[string]interface{}{
			{"eventType": "login"},
			{"eventType": "login", "userId": "u-1"},
			nil,
		},
	})
	require.NoError(t, err)
	assert.Equal(t, 1, resp.Accepted)
	assert.Equal(t, 2, resp.Rejected)

	// Missing gameId is rejected up front.
	_, err = ingest(context.Background(), svcCtx, &IngestRequest{Env: "prod"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "gameId 不能为空")
}

// ---------------------------------------------------------------------------
// Payments service gaps
// ---------------------------------------------------------------------------

func TestPaymentsProductTrendFiltersWindowsAndLimit(t *testing.T) {
	db := setupAnalyticsFreshDB(t)
	svcCtx := setupFullServiceTestContext(t, db)
	ctx := context.Background()

	windowStart := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	windowEnd := windowStart.Add(24 * time.Hour)
	outOfRangeStart := windowStart.Add(-48 * time.Hour)
	require.NoError(t, svcCtx.PaymentsModel.UpsertProductTrend(ctx, &model.ProductTrend{
		GameID: "g", Env: "prod", ProductID: "p-old", ProductName: "Old",
		Revenue: 10, Sales: 1, WindowStart: outOfRangeStart, WindowEnd: outOfRangeStart.Add(24 * time.Hour),
	}))
	require.NoError(t, svcCtx.PaymentsModel.UpsertProductTrend(ctx, &model.ProductTrend{
		GameID: "g", Env: "prod", ProductID: "p-in", ProductName: "In",
		Revenue: 99, Sales: 5, Growth: 0.5, WindowStart: windowStart, WindowEnd: windowEnd,
	}))

	start := windowStart.Format("2006-01-02")
	end := windowEnd.Format("2006-01-02")
	resp, err := paymentsProductTrend(ctx, svcCtx, &PaymentsProductTrendRequest{
		GameId: "g", Env: "prod", StartDate: start, EndDate: end,
	})
	require.NoError(t, err)
	require.Len(t, resp.Items, 1)
	assert.Equal(t, "p-in", resp.Items[0].ProductId)
	assert.InDelta(t, 99.0, resp.Items[0].Revenue, 0.001)

	// No window returns every trend row; default limit is generous enough.
	unscoped, err := paymentsProductTrend(ctx, svcCtx, &PaymentsProductTrendRequest{})
	require.NoError(t, err)
	assert.Len(t, unscoped.Items, 2)

	// Limit caps the number of returned rows.
	limited, err := paymentsProductTrend(ctx, svcCtx, &PaymentsProductTrendRequest{Limit: 1})
	require.NoError(t, err)
	assert.Len(t, limited.Items, 1)

	// Nil model / nil request guards.
	_, err = paymentsProductTrend(ctx, &svc.ServiceContext{}, &PaymentsProductTrendRequest{})
	require.Error(t, err)
	_, err = paymentsProductTrend(ctx, svcCtx, nil)
	require.Error(t, err)
	_, err = paymentsProductTrend(ctx, svcCtx, &PaymentsProductTrendRequest{StartDate: "bad", EndDate: "worse"})
	require.Error(t, err)
}

func TestPaymentsIngestAcceptsValidTransactionsAndRejectsBadRows(t *testing.T) {
	db := setupAnalyticsFreshDB(t)
	svcCtx := setupFullServiceTestContext(t, db)
	ctx := context.Background()

	resp, err := paymentsIngest(ctx, svcCtx, &PaymentsIngestRequest{
		GameId: "g",
		Env:    "prod",
		Transactions: []map[string]interface{}{
			{"id": "tx-1", "userId": "u-1", "amount": 9.9, "currency": "USD", "status": "success"},
			{"amount": 5}, // missing userId → rejected
		},
	})
	require.NoError(t, err)
	assert.Equal(t, 1, resp.Accepted)
	assert.Equal(t, 1, resp.Rejected)

	// The accepted row is visible through transactions listing.
	list, total, err := svcCtx.PaymentsModel.ListTransactions(ctx, model.PaymentQueryOptions{
		PaginationOptions: model.NewPagination(1, 10),
		GameID:            "g",
	})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, list, 1)
	assert.Equal(t, "tx-1", list[0].TransactionID)

	_, err = paymentsIngest(ctx, svcCtx, &PaymentsIngestRequest{GameId: ""})
	require.Error(t, err)
}

func TestPaymentsPrefersBehaviorActiveUsersWhenAvailable(t *testing.T) {
	db := setupAnalyticsFreshDB(t)
	svcCtx := setupFullServiceTestContext(t, db)
	ctx := context.Background()

	require.NoError(t, svcCtx.PaymentsModel.CreateTransaction(ctx, &model.PaymentTransaction{
		TransactionID: "tx-1", GameID: "g", Env: "prod", UserID: "u-1",
		Amount: 30, Currency: "USD", Status: "success", OccurredAt: time.Now().UTC(),
	}))
	require.NoError(t, svcCtx.BehaviorModel.RecordEvent(ctx, &model.BehaviorEvent{
		GameID: "g", Env: "prod", EventType: "login", UserID: "u-2", OccurredAt: time.Now().UTC(),
	}))

	resp, err := payments(ctx, svcCtx, &PaymentsRequest{GameId: "g", Env: "prod"})
	require.NoError(t, err)
	// Active users come from behavior (u-2), payers from payments (u-1).
	assert.InDelta(t, float64(30)/1.0, resp.Metrics.ARPU, 0.001)
	assert.InDelta(t, 1.0, resp.Metrics.ConversionRate, 0.001)
}

func TestPaymentsSummaryGroupsByMonthThroughService(t *testing.T) {
	db := setupAnalyticsFreshDB(t)
	svcCtx := setupFullServiceTestContext(t, db)
	ctx := context.Background()

	day := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	for _, tx := range []model.PaymentTransaction{
		{TransactionID: "tx-1", GameID: "g", Env: "prod", UserID: "u-1", Amount: 10, Status: "success", OccurredAt: day},
		{TransactionID: "tx-2", GameID: "g", Env: "prod", UserID: "u-2", Amount: 5, Status: "success", OccurredAt: day.Add(2 * time.Hour)},
	} {
		require.NoError(t, svcCtx.PaymentsModel.CreateTransaction(ctx, &tx))
	}

	resp, err := paymentsSummary(ctx, svcCtx, &PaymentsSummaryRequest{
		GameId: "g", Env: "prod", GroupBy: "month",
		StartDate: "2026-07-01", EndDate: "2026-07-31",
	})
	require.NoError(t, err)
	require.Len(t, resp.Items, 1)
	assert.Equal(t, "2026-07", resp.Items[0].Date)
	assert.Equal(t, 15.0, resp.Items[0].Revenue)

	_, err = paymentsSummary(ctx, svcCtx, nil)
	require.Error(t, err)
}

// ---------------------------------------------------------------------------
// Retention service gaps
// ---------------------------------------------------------------------------

func TestRetentionFiltersCohortsByWindowAndName(t *testing.T) {
	db := setupAnalyticsFreshDB(t)
	svcCtx := setupFullServiceTestContext(t, db)
	ctx := context.Background()

	inWindow := time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC)
	beforeWindow := inWindow.Add(-72 * time.Hour)
	retentionJSON := model.JSON(`[40,20]`)
	for _, cohort := range []model.RetentionCohort{
		{GameID: "g", Env: "prod", Cohort: "2026-08-10", Users: 50, Retention: retentionJSON, WindowStart: inWindow, WindowEnd: inWindow.Add(24 * time.Hour)},
		{GameID: "g", Env: "prod", Cohort: "too-early", Users: 10, Retention: retentionJSON, WindowStart: beforeWindow, WindowEnd: beforeWindow.Add(24 * time.Hour)},
	} {
		require.NoError(t, svcCtx.RetentionModel.UpsertCohort(ctx, &cohort))
	}

	resp, err := retention(ctx, svcCtx, &RetentionRequest{
		GameId: "g", Env: "prod",
		StartDate: "2026-08-09", EndDate: "2026-08-12",
	})
	require.NoError(t, err)
	require.Len(t, resp.Cohorts, 1)
	assert.Equal(t, "2026-08-10", resp.Cohorts[0].Cohort)
	assert.Equal(t, 50, resp.Cohorts[0].Users)
	require.NotEmpty(t, resp.Cohorts[0].Retention)

	// Cohort name narrows further and unknown names yield empty results.
	named, err := retention(ctx, svcCtx, &RetentionRequest{GameId: "g", Cohort: "nope"})
	require.NoError(t, err)
	assert.Empty(t, named.Cohorts)

	_, err = retention(ctx, svcCtx, nil)
	require.Error(t, err)
}

func TestResolveRetentionRangeEndOnly(t *testing.T) {
	end := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)
	start, gotEnd, err := resolveRetentionRange("", end.Format("2006-01-02"))
	require.NoError(t, err)
	assert.False(t, start.IsZero())
	assert.Equal(t, end.Day(), gotEnd.UTC().Day())
}

// ---------------------------------------------------------------------------
// Test DB helper
// ---------------------------------------------------------------------------

func openAnalyticsFreshDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(gsqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))
	return db
}

func setupAnalyticsFreshDB(t *testing.T) *gorm.DB {
	t.Helper()
	return openAnalyticsFreshDB(t)
}

// ---------------------------------------------------------------------------
// Extension-backed filters
// ---------------------------------------------------------------------------

func newFiltersExtensionService(t *testing.T) (*svc.ServiceContext, *extensioninstallation.Service) {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.ExtensionInstallation{}, &model.ExtensionRuntimeBinding{}, &model.ExtensionEvent{}))

	repos := extensiongorm.NewBundle(db)
	installationSvc := extensioninstallation.NewService(repos.Installation, repos.Event, repos.Binding)
	svcCtx := &svc.ServiceContext{
		DB:         db,
		Extensions: &svc.ExtensionServices{Installation: installationSvc},
	}
	return svcCtx, installationSvc
}

func TestFiltersGetExtensionConfigWithoutFiltersKeyYieldsEmpty(t *testing.T) {
	svcCtx, installationSvc := newFiltersExtensionService(t)

	_, err := installationSvc.Install(context.Background(), extensioninstallation.InstallRequest{
		ExtensionID:    officialAnalyticsID,
		ReleaseVersion: "1.0.0",
		ScopeType:      "system",
		ScopeID:        "global",
		TargetType:     "agent_group",
		TargetID:       "default",
		Config:         map[string]any{"unrelated": true},
		Operator:       "tester",
	})
	require.NoError(t, err)

	resp, err := filtersGet(context.Background(), svcCtx, &FiltersGetRequest{})
	require.NoError(t, err)
	assert.Empty(t, resp.Items)
}

func TestFiltersUpdateSavesToExtensionInstallation(t *testing.T) {
	svcCtx, installationSvc := newFiltersExtensionService(t)

	_, err := installationSvc.Install(context.Background(), extensioninstallation.InstallRequest{
		ExtensionID:    officialAnalyticsID,
		ReleaseVersion: "1.0.0",
		ScopeType:      "system",
		ScopeID:        "global",
		TargetType:     "agent_group",
		TargetID:       "default",
		Config: map[string]any{
			"filters": []map[string]any{{"gameId": "other", "filters": map[string]any{"env": "dev"}}},
		},
		Operator: "tester",
	})
	require.NoError(t, err)

	ctx := context.WithValue(context.Background(), "username", "ops-user")
	resp, err := filtersUpdate(ctx, svcCtx, &FiltersUpdateRequest{
		GameId:  "tower",
		Filters: map[string]any{"env": "prod"},
	})
	require.NoError(t, err)
	require.Len(t, resp.Items, 1)
	assert.Equal(t, "tower", resp.Items[0].GameId)

	// The extension config now carries both entries (upsert preserved the
	// unrelated game filter).
	items, source, err := loadAnalyticsFiltersForUpdate(ctx, svcCtx)
	require.NoError(t, err)
	assert.Equal(t, "extension", source)
	require.Len(t, items, 2)
}

// ---------------------------------------------------------------------------
// Handler service-error branches
// ---------------------------------------------------------------------------

func TestAnalyticsHandlersSurfaceServiceErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	empty := NewHandler(NewService(&svc.ServiceContext{}), config.SSEConfig{})
	dbless := NewHandler(NewService(&svc.ServiceContext{DB: nil}), config.SSEConfig{})

	getCases := []struct {
		name string
		path string
		fn   func(*gin.Context)
	}{
		{"behavior", "/api/v1/analytics/behavior?", empty.Behavior},
		{"behaviorEvents", "/api/v1/analytics/events?", empty.BehaviorEvents},
		{"behaviorAdoption", "/api/v1/analytics/adoption?", empty.BehaviorAdoption},
		{"adoptionBreakdown", "/api/v1/analytics/adoption/breakdown?feature=login", empty.BehaviorAdoptionBreakdown},
		{"funnel", "/api/v1/analytics/funnel?steps=login", empty.BehaviorFunnel},
		{"paths", "/api/v1/analytics/paths?", empty.BehaviorPaths},
		{"overview", "/api/v1/analytics/overview?", empty.Overview},
		{"realtimeSeries", "/api/v1/analytics/series?", empty.RealtimeSeries},
		{"payments", "/api/v1/analytics/payments?", empty.Payments},
		{"productTrend", "/api/v1/analytics/payments/products?", empty.PaymentsProductTrend},
		{"paymentsSummary", "/api/v1/analytics/payments/summary?", empty.PaymentsSummary},
		{"transactions", "/api/v1/analytics/payments/transactions?", empty.PaymentsTransactions},
		{"retention", "/api/v1/analytics/retention?", empty.Retention},
		{"levels", "/api/v1/analytics/levels?", empty.Levels},
		{"levelsEpisodes", "/api/v1/analytics/levels/episodes?", empty.LevelsEpisodes},
		{"levelsMaps", "/api/v1/analytics/levels/maps?", empty.LevelsMaps},
		{"invocationsTrend", "/api/v1/analytics/invocations/trend", dbless.InvocationsTrend},
		{"invocationsSummary", "/api/v1/analytics/invocations/summary", dbless.InvocationsSummary},
		{"invocationsList", "/api/v1/analytics/invocations", dbless.InvocationsList},
	}
	for _, tc := range getCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, rec := newCoverageGinContext(http.MethodGet, tc.path+"&gameId=g&env=prod", "")
			tc.fn(ctx)
			assert.Equal(t, http.StatusInternalServerError, rec.Code)
		})
	}

	// FiltersUpdate rejects requests without gameId / filters payload.
	ctx, rec := newCoverageGinContext(http.MethodPost, "/api/v1/analytics/filters", `{}`)
	empty.FiltersUpdate(ctx)
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	// FiltersGet surfaces read errors from an unreadable filters path.
	dir := t.TempDir()
	broken := &svc.ServiceContext{
		Config: config.Config{Registry: config.RegistryConfig{AnalyticsFiltersPath: dir}},
	}
	ctx, rec = newCoverageGinContext(http.MethodGet, "/api/v1/analytics/filters", "")
	NewHandler(NewService(broken), config.SSEConfig{}).FiltersGet(ctx)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestFindActiveAnalyticsInstallationPropagatesListError(t *testing.T) {
	svcCtx := &svc.ServiceContext{}

	item, ok, err := findActiveAnalyticsInstallation(context.Background(), svcCtx)
	require.NoError(t, err)
	assert.False(t, ok)
	assert.Nil(t, item)

	// An installation service whose tables do not exist surfaces the query error.
	svcCtx.Extensions = &svc.ExtensionServices{Installation: newBrokenInstallationService(t)}
	_, _, err = findActiveAnalyticsInstallation(context.Background(), svcCtx)
	require.Error(t, err)

	_, err = filtersGet(context.Background(), svcCtx, &FiltersGetRequest{})
	require.Error(t, err)

	_, err = filtersUpdate(context.Background(), svcCtx, &FiltersUpdateRequest{
		GameId:  "g",
		Filters: map[string]any{"env": "prod"},
	})
	require.Error(t, err)
}

func newBrokenInstallationService(t *testing.T) *extensioninstallation.Service {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	repos := extensiongorm.NewBundle(db)
	return extensioninstallation.NewService(repos.Installation, repos.Event, repos.Binding)
}

func TestSetAnalyticsFiltersToConfigNilMap(t *testing.T) {
	config := setAnalyticsFiltersToConfig(nil, []AnalyticsFilters{{GameId: "g"}})
	require.NotNil(t, config)
	require.Contains(t, config, analyticsFiltersKey)
	items := config[analyticsFiltersKey].([]AnalyticsFilters)
	require.Len(t, items, 1)
	assert.Equal(t, "g", items[0].GameId)
}

// ---------------------------------------------------------------------------
// Warehouse scan failures + payments/invocations DB failures
// ---------------------------------------------------------------------------

// scanFailureRows yields exactly one row whose Scan always fails.
type scanFailureRows struct {
	called bool
	err    error
}

func (r *scanFailureRows) Next() bool {
	if r.called {
		return false
	}
	r.called = true
	return true
}
func (r *scanFailureRows) Scan(_ ...any) error { return r.err }
func (r *scanFailureRows) Close() error        { return nil }

type scanFailureConn struct {
	rowsErr error
}

func (c *scanFailureConn) Query(context.Context, string, ...any) (warehouseRows, error) {
	return &scanFailureRows{err: c.rowsErr}, nil
}

func withScanFailureConn(t *testing.T, err error) {
	t.Helper()
	orig := warehouseConnect
	warehouseConnect = func() (warehouseConn, error) {
		return &scanFailureConn{rowsErr: err}, nil
	}
	t.Cleanup(func() { warehouseConnect = orig })
}

func TestWarehouseServicesRejectUnscannableRows(t *testing.T) {
	withScanFailureConn(t, errors.New("unsupported scan"))
	_, err := NewService(nil).WarehouseDAU(context.Background(), &WarehouseDAURequest{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "scan daily_users")

	_, err = NewService(nil).WarehouseOnline(context.Background(), &WarehouseOnlineRequest{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "scan minute_online")

	_, err = NewService(nil).WarehouseRevenue(context.Background(), &WarehouseRevenueRequest{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "scan daily_revenue")
}

func TestPaymentsTransactionsSurfacesListError(t *testing.T) {
	db := setupAnalyticsFreshDB(t)
	require.NoError(t, db.Migrator().DropTable("payment_transactions"))
	svcCtx := setupFullServiceTestContext(t, db)

	_, err := paymentsTransactions(context.Background(), svcCtx, &PaymentsTransactionsRequest{
		GameId: "g",
		Status: "success",
	})
	require.Error(t, err)
}

func TestPaymentsIngestCountsRejectedWhenPersistFails(t *testing.T) {
	db := setupAnalyticsFreshDB(t)
	require.NoError(t, db.Migrator().DropTable("payment_transactions"))
	svcCtx := setupFullServiceTestContext(t, db)

	resp, err := paymentsIngest(context.Background(), svcCtx, &PaymentsIngestRequest{
		GameId: "g",
		Env:    "prod",
		Transactions: []map[string]interface{}{
			{"id": "tx-1", "userId": "u-1"},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, 0, resp.Accepted)
	assert.Equal(t, 1, resp.Rejected)
}

func TestInvocationsListSurfacesCountError(t *testing.T) {
	db := setupInvocationsTestDB(t)
	require.NoError(t, db.Migrator().DropTable("audit_records"))

	_, err := invocationsList(context.Background(), &svc.ServiceContext{DB: db}, &InvocationsListRequest{})
	require.Error(t, err)
}

func TestLevelsSurfacesBehaviorQueryError(t *testing.T) {
	db := setupAnalyticsFreshDB(t)
	require.NoError(t, db.Migrator().DropTable("behavior_events"))
	svcCtx := setupFullServiceTestContext(t, db)

	_, err := levels(context.Background(), svcCtx, &LevelsRequest{GameId: "g"})
	require.Error(t, err)
}

// ---------------------------------------------------------------------------
// Handler happy paths over a seeded store
// ---------------------------------------------------------------------------

func seedCoverageBehaviorEvents(t *testing.T, svcCtx *svc.ServiceContext) {
	t.Helper()
	base := time.Now().UTC().Add(-48 * time.Hour)
	for i, user := range []string{"u-1", "u-2", "u-3"} {
		event := &model.BehaviorEvent{
			GameID:     "g",
			Env:        "prod",
			EventType:  "login",
			UserID:     user,
			Data:       datatypes.JSONMap{"region": "cn", "platform": "ios", "role": "warrior"},
			OccurredAt: base.Add(time.Duration(i) * time.Hour),
		}
		require.NoError(t, svcCtx.BehaviorModel.RecordEvent(context.Background(), event))
	}
}

func TestAnalyticsHandlersReturnSuccessPayloads(t *testing.T) {
	db := setupServiceTestDB(t)
	svcCtx := setupFullServiceTestContext(t, db)
	seedCoverageBehaviorEvents(t, svcCtx)

	ctx := context.Background()
	day := time.Now().UTC().AddDate(0, 0, -1).Format("2006-01-02")
	require.NoError(t, svcCtx.PaymentsModel.CreateTransaction(ctx, &model.PaymentTransaction{
		TransactionID: "tx-h1", GameID: "g", Env: "prod", UserID: "u-1",
		Amount: 12.5, Currency: "USD", Status: "success", OccurredAt: time.Now().UTC(),
	}))
	require.NoError(t, svcCtx.RetentionModel.UpsertCohort(ctx, &model.RetentionCohort{
		GameID: "g", Env: "prod", Cohort: "c1", Users: 3,
		Retention:   datatypesJSON(`[30]`),
		WindowStart: time.Now().UTC(), WindowEnd: time.Now().UTC(),
	}))

	handler := NewHandler(NewService(svcCtx), config.SSEConfig{})

	getPaths := map[string]func(*gin.Context){
		"/api/v1/analytics/behavior?gameId=g&env=prod":                handler.Behavior,
		"/api/v1/analytics/events?gameId=g&eventType=login":           handler.BehaviorEvents,
		"/api/v1/analytics/adoption?gameId=g":                         handler.BehaviorAdoption,
		"/api/v1/analytics/adoption/breakdown?gameId=g&feature=login": handler.BehaviorAdoptionBreakdown,
		"/api/v1/analytics/funnel?gameId=g&steps=login&steps=pay":     handler.BehaviorFunnel,
		"/api/v1/analytics/paths?gameId=g&depth=3":                    handler.BehaviorPaths,
		"/api/v1/analytics/overview?gameId=g&env=prod":                handler.Overview,
		"/api/v1/analytics/series?gameId=g&duration=60":               handler.RealtimeSeries,
		"/api/v1/analytics/payments?gameId=g":                         handler.Payments,
		"/api/v1/analytics/payments/products?gameId=g":                handler.PaymentsProductTrend,
		"/api/v1/analytics/payments/summary?gameId=g&groupBy=week":    handler.PaymentsSummary,
		"/api/v1/analytics/payments/transactions?gameId=g":            handler.PaymentsTransactions,
		"/api/v1/analytics/retention?gameId=g":                        handler.Retention,
		"/api/v1/analytics/levels?gameId=g":                           handler.Levels,
		"/api/v1/analytics/levels/episodes?gameId=g":                  handler.LevelsEpisodes,
		"/api/v1/analytics/levels/maps?gameId=g":                      handler.LevelsMaps,
	}
	for path, fn := range getPaths {
		t.Run(path, func(t *testing.T) {
			reqCtx, rec := newCoverageGinContext(http.MethodGet, path, "")
			fn(reqCtx)
			assert.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())
		})
	}

	postCases := []struct {
		path string
		body string
		fn   func(*gin.Context)
	}{
		{"/api/v1/analytics/ingest", `{"gameId":"g","env":"prod","events":[{"eventType":"login","userId":"u-9"}]}`, handler.Ingest},
		{"/api/v1/analytics/payments/ingest", `{"gameId":"g","transactions":[{"id":"tx-p1","userId":"u-1","amount":3}]}`, handler.PaymentsIngest},
		{"/api/v1/analytics/filters", fmt.Sprintf(`{"gameId":"g","filters":{"dateRange":{"start":%q}}}`, day), handler.FiltersUpdate},
	}
	for _, tc := range postCases {
		t.Run(tc.path, func(t *testing.T) {
			reqCtx, rec := newCoverageGinContext(http.MethodPost, tc.path, tc.body)
			tc.fn(reqCtx)
			assert.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())
		})
	}
}

func datatypesJSON(raw string) model.JSON {
	return model.JSON([]byte(raw))
}

func TestBreakdownByTimeFallbackAndSwapBranches(t *testing.T) {
	now := time.Now().UTC()
	events := []model.BehaviorEvent{
		{UserID: "u-1", EventType: "e1", OccurredAt: now.Add(-2 * time.Hour)},
		{UserID: "u-2", EventType: "e2", OccurredAt: now.Add(-1 * time.Hour)},
	}

	// Zero start/end fall back to first/last event timestamps.
	points := breakdownByTime(events, time.Time{}, time.Time{})
	require.NotEmpty(t, points)

	// Swapped range is normalised.
	swapped := breakdownByTime(events, now, now.Add(-24*time.Hour))
	require.NotEmpty(t, swapped)

	// Long ranges keep the daily interval.
	longStart := now.AddDate(0, 0, -90)
	longPoints := breakdownByTime([]model.BehaviorEvent{
		{UserID: "u-1", EventType: "e1", OccurredAt: now},
	}, longStart, now)
	require.NotEmpty(t, longPoints)

	// Events outside of the window are skipped.
	outside := breakdownByTime([]model.BehaviorEvent{
		{UserID: "u-1", EventType: "e1", OccurredAt: now.Add(-48 * time.Hour)},
	}, now.Add(-time.Hour), now)
	assert.NotEmpty(t, outside)
}
