package analytics

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/cache"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/service/permission"
	"github.com/cuihairu/croupier/internal/svc"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

var (
	serviceTestDB      *gorm.DB
	serviceTestDBOnce  sync.Once
	serviceTestDBMutex sync.Mutex
)

// setupServiceTestDB creates a shared in-memory SQLite database for testing
func setupServiceTestDB(t *testing.T) *gorm.DB {
	serviceTestDBMutex.Lock()
	defer serviceTestDBMutex.Unlock()

	serviceTestDBOnce.Do(func() {
		var err error
		serviceTestDB, err = gorm.Open(gsqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
		if err != nil {
			panic(err)
		}
		err = model.AutoMigrate(serviceTestDB)
		if err != nil {
			panic(err)
		}
	})

	// Clean up before each test
	serviceTestDB.Exec("DELETE FROM behavior_events")
	serviceTestDB.Exec("DELETE FROM feature_adoptions")

	return serviceTestDB
}

// setupServiceTestContext creates a test service context
func setupServiceTestContext(t *testing.T, db *gorm.DB) *svc.ServiceContext {
	permSvc := permission.NewPermissionService(db)
	nullCache := cache.NewNullCache()
	cacheHelper := cache.NewCacheHelper(nullCache)

	return &svc.ServiceContext{
		DB:                db,
		BehaviorModel:     model.NewBehaviorModel(db),
		PermissionService: permSvc,
		Cache:             nullCache,
		CacheHelper:       cacheHelper,
		RegistryStore:     registry.NewStore(),
	}
}

// Tests for helper functions

func TestSafeDivide_Normal(t *testing.T) {
	result := safeDivide(10, 2)
	assert.Equal(t, 5.0, result)
}

func TestSafeDivide_ZeroDenominator(t *testing.T) {
	result := safeDivide(10, 0)
	assert.Equal(t, 0.0, result)
}

func TestSafeDivide_NegativeNumbers(t *testing.T) {
	result := safeDivide(-10, 2)
	assert.Equal(t, -5.0, result)
}

func TestSafeDivide_ZeroNumerator(t *testing.T) {
	result := safeDivide(0, 5)
	assert.Equal(t, 0.0, result)
}

func TestSafeDivide_FloatResult(t *testing.T) {
	result := safeDivide(1, 2)
	assert.Equal(t, 0.5, result)
}

func TestResolveRange_ValidDates(t *testing.T) {
	start := "2024-01-01"
	end := "2024-01-31"

	startTime, endTime, err := resolveRange(start, end, 7)

	require.NoError(t, err)
	assert.False(t, startTime.IsZero())
	assert.False(t, endTime.IsZero())
}

func TestResolveRange_EmptyEnd(t *testing.T) {
	start := "2024-01-01"

	startTime, endTime, err := resolveRange(start, "", 7)

	require.NoError(t, err)
	assert.False(t, startTime.IsZero())
	assert.False(t, endTime.IsZero())
}

func TestResolveRange_EmptyStart(t *testing.T) {
	end := "2024-01-31"
	fallbackDays := 7

	startTime, endTime, err := resolveRange("", end, fallbackDays)

	require.NoError(t, err)
	assert.False(t, startTime.IsZero())
	assert.False(t, endTime.IsZero())
	// Start should be approximately fallbackDays before end
	expectedStart := endTime.Add(-time.Duration(fallbackDays) * 24 * time.Hour)
	assert.WithinDuration(t, expectedStart, startTime, time.Minute)
}

func TestResolveRange_EmptyBoth(t *testing.T) {
	startTime, endTime, err := resolveRange("", "", 7)

	require.NoError(t, err)
	assert.False(t, startTime.IsZero())
	assert.False(t, endTime.IsZero())
}

func TestResolveRange_DefaultFallbackDays(t *testing.T) {
	end := "2024-01-31"

	startTime, endTime, err := resolveRange("", end, 0)

	require.NoError(t, err)
	assert.False(t, startTime.IsZero())
	assert.False(t, endTime.IsZero())
	// Should use default 7 days
	expectedStart := endTime.Add(-7 * 24 * time.Hour)
	assert.WithinDuration(t, expectedStart, startTime, time.Minute)
}

func TestResolveRange_InvalidDates(t *testing.T) {
	_, _, err := resolveRange("invalid", "2024-01-31", 7)

	assert.Error(t, err)
}

func TestTrimString_Normal(t *testing.T) {
	result := trimString("  hello  ")
	assert.Equal(t, "hello", result)
}

func TestTrimString_Empty(t *testing.T) {
	result := trimString("")
	assert.Equal(t, "", result)
}

func TestTrimString_WhitespaceOnly(t *testing.T) {
	result := trimString("   ")
	assert.Equal(t, "", result)
}

func TestTrimString_NoTrimNeeded(t *testing.T) {
	result := trimString("hello")
	assert.Equal(t, "hello", result)
}

func TestTrimString_TabsAndNewlines(t *testing.T) {
	result := trimString("\n\t hello \t\n")
	assert.Equal(t, "hello", result)
}

func TestParseFloat_ValidInteger(t *testing.T) {
	result, ok := parseFloat("42")
	assert.True(t, ok)
	assert.Equal(t, 42.0, result)
}

func TestParseFloat_ValidFloat(t *testing.T) {
	result, ok := parseFloat("3.14")
	assert.True(t, ok)
	assert.Equal(t, 3.14, result)
}

func TestParseFloat_WithWhitespace(t *testing.T) {
	result, ok := parseFloat("  3.14  ")
	assert.True(t, ok)
	assert.Equal(t, 3.14, result)
}

func TestParseFloat_Empty(t *testing.T) {
	result, ok := parseFloat("")
	assert.False(t, ok)
	assert.Equal(t, 0.0, result)
}

func TestParseFloat_WhitespaceOnly(t *testing.T) {
	result, ok := parseFloat("   ")
	assert.False(t, ok)
	assert.Equal(t, 0.0, result)
}

func TestParseFloat_Invalid(t *testing.T) {
	result, ok := parseFloat("abc")
	assert.False(t, ok)
	assert.Equal(t, 0.0, result)
}

func TestParseFloat_Negative(t *testing.T) {
	result, ok := parseFloat("-5.5")
	assert.True(t, ok)
	assert.Equal(t, -5.5, result)
}

func TestParseFloat_Scientific(t *testing.T) {
	result, ok := parseFloat("1.5e2")
	assert.True(t, ok)
	assert.Equal(t, 150.0, result)
}

func TestAggregateAgentMetrics_NilStore(t *testing.T) {
	latency, errorRate := aggregateAgentMetrics(nil, "game1", "prod")

	assert.Equal(t, 0.0, latency)
	assert.Equal(t, 0.0, errorRate)
}

func TestAggregateAgentMetrics_EmptyStore(t *testing.T) {
	store := registry.NewStore()

	latency, errorRate := aggregateAgentMetrics(store, "game1", "prod")

	assert.Equal(t, 0.0, latency)
	assert.Equal(t, 0.0, errorRate)
}

// Note: Tests with UpsertAgent are skipped due to potential deadlock issues
// These can be added back once the registry Store lock behavior is investigated

// Tests for loadBehaviorEvents

func TestLoadBehaviorEvents_NilSvcCtx(t *testing.T) {
	ctx := context.Background()
	start := time.Now().Add(-24 * time.Hour)
	end := time.Now()

	events, err := loadBehaviorEvents(ctx, nil, "game1", "prod", start, end, []string{"login"}, 100)

	assert.Error(t, err)
	assert.Nil(t, events)
	assert.Contains(t, err.Error(), "unavailable")
}

func TestLoadBehaviorEvents_NilBehaviorModel(t *testing.T) {
	ctx := context.Background()
	svcCtx := &svc.ServiceContext{
		BehaviorModel: nil,
	}
	start := time.Now().Add(-24 * time.Hour)
	end := time.Now()

	events, err := loadBehaviorEvents(ctx, svcCtx, "game1", "prod", start, end, []string{"login"}, 100)

	assert.Error(t, err)
	assert.Nil(t, events)
}

func TestLoadBehaviorEvents_DefaultPageSize(t *testing.T) {
	db := setupServiceTestDB(t)
	svcCtx := setupServiceTestContext(t, db)
	ctx := context.Background()

	// First verify BehaviorModel is working
	baseTime := time.Now().UTC()
	ev := createTestEvent("test", "user1", "game1", "prod", baseTime.Add(-1*time.Hour), nil)
	require.NoError(t, svcCtx.BehaviorModel.RecordEvent(ctx, &ev), "Failed to record test event")

	start := baseTime.Add(-24 * time.Hour)
	end := baseTime.Add(1 * time.Hour)

	events, err := loadBehaviorEvents(ctx, svcCtx, "game1", "prod", start, end, []string{}, 0)

	require.NoError(t, err)
	assert.NotNil(t, events)
}

func TestLoadBehaviorEvents_WithEventTypes(t *testing.T) {
	db := setupServiceTestDB(t)
	svcCtx := setupServiceTestContext(t, db)
	ctx := context.Background()

	// Create test events
	baseTime := time.Now().UTC()
	ev1 := createTestEvent("login", "user1", "game1", "prod", baseTime.Add(-2*time.Hour), nil)
	ev2 := createTestEvent("logout", "user1", "game1", "prod", baseTime.Add(-1*time.Hour), nil)

	require.NoError(t, svcCtx.BehaviorModel.RecordEvent(ctx, &ev1))
	require.NoError(t, svcCtx.BehaviorModel.RecordEvent(ctx, &ev2))

	start := baseTime.Add(-24 * time.Hour)
	end := baseTime.Add(1 * time.Hour)

	// Query for only login events
	events, err := loadBehaviorEvents(ctx, svcCtx, "game1", "prod", start, end, []string{"login"}, 100)

	require.NoError(t, err)
	assert.NotNil(t, events)
}

func TestLoadBehaviorEvents_MultipleEventTypes(t *testing.T) {
	db := setupServiceTestDB(t)
	svcCtx := setupServiceTestContext(t, db)
	ctx := context.Background()

	baseTime := time.Now().UTC()
	ev1 := createTestEvent("login", "user1", "game1", "prod", baseTime.Add(-3*time.Hour), nil)
	ev2 := createTestEvent("logout", "user2", "game1", "prod", baseTime.Add(-2*time.Hour), nil)
	ev3 := createTestEvent("purchase", "user1", "game1", "prod", baseTime.Add(-1*time.Hour), nil)

	require.NoError(t, svcCtx.BehaviorModel.RecordEvent(ctx, &ev1))
	require.NoError(t, svcCtx.BehaviorModel.RecordEvent(ctx, &ev2))
	require.NoError(t, svcCtx.BehaviorModel.RecordEvent(ctx, &ev3))

	start := baseTime.Add(-24 * time.Hour)
	end := baseTime.Add(1 * time.Hour)

	// Query for login and logout events
	events, err := loadBehaviorEvents(ctx, svcCtx, "game1", "prod", start, end, []string{"login", "logout"}, 100)

	require.NoError(t, err)
	assert.NotNil(t, events)
}

// Tests for Service creation

func TestNewService(t *testing.T) {
	svcCtx := &svc.ServiceContext{}
	service := NewService(svcCtx)

	assert.NotNil(t, service)
	assert.Equal(t, svcCtx, service.svcCtx)
}

// Basic behavior service delegation tests (already covered by behavior_test.go)
// These tests verify the delegation works correctly

func TestService_Behavior_Delegation(t *testing.T) {
	db := setupServiceTestDB(t)
	svcCtx := setupServiceTestContext(t, db)
	service := NewService(svcCtx)

	baseTime := time.Now().UTC()
	ev := createTestEvent("login", "user1", "game1", "prod", baseTime.Add(-1*time.Hour), nil)
	require.NoError(t, svcCtx.BehaviorModel.RecordEvent(context.Background(), &ev))

	req := &BehaviorRequest{
		GameId:    "game1",
		Env:       "prod",
		StartDate: baseTime.Add(-24 * time.Hour).Format(time.RFC3339),
		EndDate:   baseTime.Add(1 * time.Hour).Format(time.RFC3339),
	}

	resp, err := service.Behavior(context.Background(), req)

	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestService_Behavior_NilRequest(t *testing.T) {
	db := setupServiceTestDB(t)
	svcCtx := setupServiceTestContext(t, db)
	service := NewService(svcCtx)

	resp, err := service.Behavior(context.Background(), nil)

	assert.Error(t, err)
	assert.Nil(t, resp)
}

// Tests for PaymentsIngest service method

func TestService_PaymentsIngest_NilRequest(t *testing.T) {
	svcCtx := &svc.ServiceContext{}
	service := NewService(svcCtx)

	resp, err := service.PaymentsIngest(context.Background(), nil)

	// PaymentsModel is nil, so it returns "unavailable" error before checking request
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "unavailable")
}

func TestService_PaymentsIngest_EmptyGameId(t *testing.T) {
	svcCtx := &svc.ServiceContext{}
	service := NewService(svcCtx)

	resp, err := service.PaymentsIngest(context.Background(), &PaymentsIngestRequest{
		GameId: "",
	})

	// PaymentsModel is nil, so it returns "unavailable" error before checking gameId
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "unavailable")
}

func TestService_PaymentsIngest_NilPaymentsModel(t *testing.T) {
	svcCtx := &svc.ServiceContext{}
	service := NewService(svcCtx)

	resp, err := service.PaymentsIngest(context.Background(), &PaymentsIngestRequest{
		GameId: "test-game",
		Env:    "prod",
	})

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "unavailable")
}

func TestService_PaymentsIngest_WithValidData(t *testing.T) {
	db := setupServiceTestDB(t)
	svcCtx := setupServiceTestContext(t, db)
	service := NewService(svcCtx)

	// This test verifies the method is called
	// The actual logic is tested in payments_test.go
	resp, err := service.PaymentsIngest(context.Background(), &PaymentsIngestRequest{
		GameId:       "test-game",
		Env:          "prod",
		Transactions: `[]`,
	})

	// Since PaymentsModel is not set in the test context, it will return error
	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestService_PaymentsProductTrend_NilRequest(t *testing.T) {
	svcCtx := &svc.ServiceContext{}
	service := NewService(svcCtx)

	resp, err := service.PaymentsProductTrend(context.Background(), nil)

	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestService_PaymentsSummary_NilRequest(t *testing.T) {
	svcCtx := &svc.ServiceContext{}
	service := NewService(svcCtx)

	resp, err := service.PaymentsSummary(context.Background(), nil)

	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestService_PaymentsTransactions_NilRequest(t *testing.T) {
	svcCtx := &svc.ServiceContext{}
	service := NewService(svcCtx)

	resp, err := service.PaymentsTransactions(context.Background(), nil)

	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestService_Retention_NilRequest(t *testing.T) {
	svcCtx := &svc.ServiceContext{}
	service := NewService(svcCtx)

	resp, err := service.Retention(context.Background(), nil)

	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestService_Levels_NilRequest(t *testing.T) {
	svcCtx := &svc.ServiceContext{}
	service := NewService(svcCtx)

	resp, err := service.Levels(context.Background(), nil)

	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestService_LevelsEpisodes_NilRequest(t *testing.T) {
	svcCtx := &svc.ServiceContext{}
	service := NewService(svcCtx)

	resp, err := service.LevelsEpisodes(context.Background(), nil)

	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestService_LevelsMaps_NilRequest(t *testing.T) {
	svcCtx := &svc.ServiceContext{}
	service := NewService(svcCtx)

	resp, err := service.LevelsMaps(context.Background(), nil)

	assert.Error(t, err)
	assert.Nil(t, resp)
}

// Tests for other 0% coverage service methods

func TestService_BehaviorAdoptionBreakdown_NilRequest(t *testing.T) {
	svcCtx := &svc.ServiceContext{}
	service := NewService(svcCtx)

	resp, err := service.BehaviorAdoptionBreakdown(context.Background(), nil)

	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestService_BehaviorAdoptionBreakdown_EmptyGameId(t *testing.T) {
	svcCtx := &svc.ServiceContext{}
	service := NewService(svcCtx)

	resp, err := service.BehaviorAdoptionBreakdown(context.Background(), &BehaviorAdoptionBreakdownRequest{
		GameId: "",
	})

	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestService_BehaviorFunnel_NilRequest(t *testing.T) {
	svcCtx := &svc.ServiceContext{}
	service := NewService(svcCtx)

	resp, err := service.BehaviorFunnel(context.Background(), nil)

	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestService_BehaviorFunnel_EmptyGameId(t *testing.T) {
	svcCtx := &svc.ServiceContext{}
	service := NewService(svcCtx)

	resp, err := service.BehaviorFunnel(context.Background(), &BehaviorFunnelRequest{
		GameId: "",
	})

	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestService_Realtime_NilRequest(t *testing.T) {
	svcCtx := &svc.ServiceContext{}
	service := NewService(svcCtx)

	resp, err := service.Realtime(context.Background(), nil)

	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestService_Realtime_EmptyGameId(t *testing.T) {
	svcCtx := &svc.ServiceContext{}
	service := NewService(svcCtx)

	resp, err := service.Realtime(context.Background(), &RealtimeRequest{
		GameId: "",
	})

	assert.Error(t, err)
	assert.Nil(t, resp)
}
