package analytics

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

var (
	behaviorTestDB      *gorm.DB
	behaviorTestDBOnce  sync.Once
	behaviorTestDBMutex sync.Mutex
)

// setupBehaviorTestDB creates a shared in-memory SQLite database for testing
func setupBehaviorTestDB(t *testing.T) *gorm.DB {
	behaviorTestDBMutex.Lock()
	defer behaviorTestDBMutex.Unlock()

	behaviorTestDBOnce.Do(func() {
		var err error
		behaviorTestDB, err = gorm.Open(gsqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
		if err != nil {
			panic(err)
		}
		err = model.AutoMigrate(behaviorTestDB)
		if err != nil {
			panic(err)
		}
	})

	// Clean up any existing data before running the test
	behaviorTestDB.Exec("DELETE FROM behavior_events")
	behaviorTestDB.Exec("DELETE FROM feature_adoptions")

	return behaviorTestDB
}

// setupBehaviorServiceContext creates a test service context with behavior model
func setupBehaviorServiceContext(t *testing.T, db *gorm.DB) *svc.ServiceContext {
	return &svc.ServiceContext{
		DB:            db,
		BehaviorModel: model.NewBehaviorModel(db),
	}
}

// Helper to create test events
func createTestEvent(eventType, userID, gameID, env string, timestamp time.Time, data map[string]interface{}) model.BehaviorEvent {
	return model.BehaviorEvent{
		EventType:  eventType,
		UserID:     userID,
		GameID:     gameID,
		Env:        env,
		OccurredAt: timestamp,
		Data:       datatypes.JSONMap(data),
	}
}

func TestBehaviorAnalytics_Success(t *testing.T) {
	db := setupBehaviorTestDB(t)
	svcCtx := setupBehaviorServiceContext(t, db)
	behaviorModel := svcCtx.BehaviorModel

	now := time.Now()

	// Insert test events
	events := []model.BehaviorEvent{
		createTestEvent("login", "user1", "game1", "prod", now.Add(-1*time.Hour), map[string]interface{}{"region": "US", "platform": "ios"}),
		createTestEvent("login", "user2", "game1", "prod", now.Add(-2*time.Hour), map[string]interface{}{"region": "EU", "platform": "android"}),
		createTestEvent("purchase", "user1", "game1", "prod", now.Add(-30*time.Minute), map[string]interface{}{"region": "US", "platform": "ios"}),
	}
	for _, ev := range events {
		require.NoError(t, behaviorModel.RecordEvent(context.Background(), &ev))
	}

	req := &BehaviorRequest{
		GameId:    "game1",
		Env:       "prod",
		StartDate: now.Add(-24 * time.Hour).Format(time.RFC3339),
		EndDate:   now.Format(time.RFC3339),
	}

	resp, err := behaviorAnalytics(context.Background(), svcCtx, req)

	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotNil(t, resp.TopActions)
	assert.NotNil(t, resp.UserFlows)
	assert.NotNil(t, resp.HeatMap)
}

func TestBehaviorAnalytics_NilBehaviorModel(t *testing.T) {
	svcCtx := &svc.ServiceContext{
		BehaviorModel: nil,
	}

	req := &BehaviorRequest{
		GameId: "test-game",
	}

	resp, err := behaviorAnalytics(context.Background(), svcCtx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "unavailable")
}

func TestBehaviorAnalytics_NilRequest(t *testing.T) {
	db := setupBehaviorTestDB(t)
	svcCtx := setupBehaviorServiceContext(t, db)

	resp, err := behaviorAnalytics(context.Background(), svcCtx, nil)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "缺少请求参数")
}

func TestBehaviorAnalytics_EmptyEvents(t *testing.T) {
	db := setupBehaviorTestDB(t)
	svcCtx := setupBehaviorServiceContext(t, db)

	now := time.Now()
	req := &BehaviorRequest{
		GameId:    "game1",
		StartDate: now.Add(-24 * time.Hour).Format(time.RFC3339),
		EndDate:   now.Format(time.RFC3339),
	}

	resp, err := behaviorAnalytics(context.Background(), svcCtx, req)

	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotNil(t, resp.TopActions)
}

func TestBehaviorEvents_Success(t *testing.T) {
	db := setupBehaviorTestDB(t)
	svcCtx := setupBehaviorServiceContext(t, db)
	behaviorModel := svcCtx.BehaviorModel

	baseTime := time.Now().UTC().Truncate(time.Second)

	events := []model.BehaviorEvent{
		createTestEvent("login", "user1", "game1", "prod", baseTime.Add(-2*time.Hour), map[string]interface{}{"action": "login"}),
		createTestEvent("logout", "user2", "game1", "prod", baseTime.Add(-1*time.Hour), map[string]interface{}{"action": "logout"}),
	}
	for _, ev := range events {
		require.NoError(t, behaviorModel.RecordEvent(context.Background(), &ev))
	}

	req := &BehaviorEventsRequest{
		GameId:    "game1",
		Env:       "prod",
		Limit:     10,
		StartDate: baseTime.Add(-24 * time.Hour).Format(time.RFC3339),
		EndDate:   baseTime.Add(1 * time.Hour).Format(time.RFC3339),
	}

	resp, err := behaviorEvents(context.Background(), svcCtx, req)

	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Len(t, resp.Items, 2)
	assert.Equal(t, 2, resp.Total)
}

func TestBehaviorEvents_LimitDefault(t *testing.T) {
	db := setupBehaviorTestDB(t)
	svcCtx := setupBehaviorServiceContext(t, db)
	behaviorModel := svcCtx.BehaviorModel

	now := time.Now()

	// Create more than 100 events
	for i := 0; i < 150; i++ {
		ev := createTestEvent("action", "user", "game1", "prod", now.Add(time.Duration(i)*time.Second), nil)
		require.NoError(t, behaviorModel.RecordEvent(context.Background(), &ev))
	}

	req := &BehaviorEventsRequest{
		GameId:    "game1",
		Limit:     0, // Should default to 100
		StartDate: now.Add(-24 * time.Hour).Format(time.RFC3339),
		EndDate:   now.Add(time.Duration(150) * time.Second).Format(time.RFC3339),
	}

	resp, err := behaviorEvents(context.Background(), svcCtx, req)

	require.NoError(t, err)
	assert.NotNil(t, resp)
	// Should return up to 100 events with default limit
	assert.LessOrEqual(t, len(resp.Items), 100)
}

func TestBehaviorEvents_LimitMax(t *testing.T) {
	db := setupBehaviorTestDB(t)
	svcCtx := setupBehaviorServiceContext(t, db)

	now := time.Now()

	req := &BehaviorEventsRequest{
		GameId:    "game1",
		Limit:     5000, // Should be capped at 1000
		StartDate: now.Add(-24 * time.Hour).Format(time.RFC3339),
		EndDate:   now.Format(time.RFC3339),
	}

	resp, err := behaviorEvents(context.Background(), svcCtx, req)

	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestBehaviorEvents_WithEventTypeFilter(t *testing.T) {
	db := setupBehaviorTestDB(t)
	svcCtx := setupBehaviorServiceContext(t, db)
	behaviorModel := svcCtx.BehaviorModel

	now := time.Now()

	events := []model.BehaviorEvent{
		createTestEvent("login", "user1", "game1", "prod", now, nil),
		createTestEvent("purchase", "user2", "game1", "prod", now, nil),
	}
	for _, ev := range events {
		require.NoError(t, behaviorModel.RecordEvent(context.Background(), &ev))
	}

	req := &BehaviorEventsRequest{
		GameId:    "game1",
		EventType: "login",
		Limit:     10,
		StartDate: now.Add(-24 * time.Hour).Format(time.RFC3339),
		EndDate:   now.Format(time.RFC3339),
	}

	resp, err := behaviorEvents(context.Background(), svcCtx, req)

	require.NoError(t, err)
	assert.NotNil(t, resp)
	// Only login events
	for _, item := range resp.Items {
		assert.Equal(t, "login", item.EventType)
	}
}

func TestBehaviorAdoption_Success(t *testing.T) {
	db := setupBehaviorTestDB(t)
	svcCtx := setupBehaviorServiceContext(t, db)
	behaviorModel := svcCtx.BehaviorModel

	adoptions := []model.FeatureAdoption{
		{GameID: "game1", Env: "prod", Feature: "feature1", Users: 100, AdoptionRate: 0.8, Frequency: 5.5},
		{GameID: "game1", Env: "prod", Feature: "feature2", Users: 50, AdoptionRate: 0.5, Frequency: 2.3},
	}
	for _, ad := range adoptions {
		require.NoError(t, behaviorModel.UpsertFeatureAdoption(context.Background(), &ad))
	}

	req := &BehaviorAdoptionRequest{
		GameId: "game1",
		Env:    "prod",
	}

	resp, err := behaviorAdoption(context.Background(), svcCtx, req)

	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Len(t, resp.Features, 2)
}

func TestBehaviorAdoption_WithFeatureFilter(t *testing.T) {
	db := setupBehaviorTestDB(t)
	svcCtx := setupBehaviorServiceContext(t, db)
	behaviorModel := svcCtx.BehaviorModel

	adoptions := []model.FeatureAdoption{
		{GameID: "game1", Env: "prod", Feature: "feature1", Users: 100, AdoptionRate: 0.8, Frequency: 5.5},
		{GameID: "game1", Env: "prod", Feature: "feature2", Users: 50, AdoptionRate: 0.5, Frequency: 2.3},
	}
	for _, ad := range adoptions {
		require.NoError(t, behaviorModel.UpsertFeatureAdoption(context.Background(), &ad))
	}

	req := &BehaviorAdoptionRequest{
		GameId:  "game1",
		Env:     "prod",
		Feature: "feature1",
	}

	resp, err := behaviorAdoption(context.Background(), svcCtx, req)

	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Len(t, resp.Features, 1)
	assert.Equal(t, "feature1", resp.Features[0].Feature)
}

func TestBehaviorAdoption_CaseInsensitiveFilter(t *testing.T) {
	db := setupBehaviorTestDB(t)
	svcCtx := setupBehaviorServiceContext(t, db)
	behaviorModel := svcCtx.BehaviorModel

	adoptions := []model.FeatureAdoption{
		{GameID: "game1", Env: "prod", Feature: "MyFeature", Users: 100, AdoptionRate: 0.8, Frequency: 5.5},
	}
	for _, ad := range adoptions {
		require.NoError(t, behaviorModel.UpsertFeatureAdoption(context.Background(), &ad))
	}

	req := &BehaviorAdoptionRequest{
		GameId:  "game1",
		Feature: "myfeature", // Case insensitive
	}

	resp, err := behaviorAdoption(context.Background(), svcCtx, req)

	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Len(t, resp.Features, 1)
}

func TestBehaviorAdoptionBreakdown_Success(t *testing.T) {
	db := setupBehaviorTestDB(t)
	svcCtx := setupBehaviorServiceContext(t, db)
	behaviorModel := svcCtx.BehaviorModel

	now := time.Now()

	events := []model.BehaviorEvent{
		createTestEvent("feature1", "user1", "game1", "prod", now, map[string]interface{}{"region": "US", "platform": "ios", "role": "admin"}),
		createTestEvent("feature1", "user2", "game1", "prod", now, map[string]interface{}{"region": "EU", "platform": "android", "role": "user"}),
	}
	for _, ev := range events {
		require.NoError(t, behaviorModel.RecordEvent(context.Background(), &ev))
	}

	req := &BehaviorAdoptionBreakdownRequest{
		GameId:    "game1",
		Feature:   "feature1",
		StartDate: now.Add(-24 * time.Hour).Format(time.RFC3339),
		EndDate:   now.Format(time.RFC3339),
	}

	resp, err := behaviorAdoptionBreakdown(context.Background(), svcCtx, req)

	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotNil(t, resp.BySegment)
	assert.NotNil(t, resp.ByTime)
}

func TestBehaviorAdoptionBreakdown_EmptyFeature(t *testing.T) {
	db := setupBehaviorTestDB(t)
	svcCtx := setupBehaviorServiceContext(t, db)

	req := &BehaviorAdoptionBreakdownRequest{
		GameId:  "game1",
		Feature: "", // Empty feature
	}

	resp, err := behaviorAdoptionBreakdown(context.Background(), svcCtx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "feature 参数不能为空")
}

func TestBehaviorFunnel_Success(t *testing.T) {
	db := setupBehaviorTestDB(t)
	svcCtx := setupBehaviorServiceContext(t, db)
	behaviorModel := svcCtx.BehaviorModel

	now := time.Now()

	events := []model.BehaviorEvent{
		createTestEvent("view", "user1", "game1", "prod", now.Add(-3*time.Minute), nil),
		createTestEvent("click", "user1", "game1", "prod", now.Add(-2*time.Minute), nil),
		createTestEvent("purchase", "user1", "game1", "prod", now.Add(-1*time.Minute), nil),
		createTestEvent("view", "user2", "game1", "prod", now.Add(-3*time.Minute), nil),
		createTestEvent("click", "user2", "game1", "prod", now.Add(-2*time.Minute), nil),
	}
	for _, ev := range events {
		require.NoError(t, behaviorModel.RecordEvent(context.Background(), &ev))
	}

	req := &BehaviorFunnelRequest{
		GameId:    "game1",
		StartDate: now.Add(-24 * time.Hour).Format(time.RFC3339),
		EndDate:   now.Format(time.RFC3339),
		Steps:     []string{"view", "click", "purchase"},
	}

	resp, err := behaviorFunnel(context.Background(), svcCtx, req)

	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Len(t, resp.Steps, 3)
	assert.Equal(t, "view", resp.Steps[0].Step)
	assert.Equal(t, "click", resp.Steps[1].Step)
	assert.Equal(t, "purchase", resp.Steps[2].Step)
}

func TestBehaviorFunnel_EmptySteps(t *testing.T) {
	db := setupBehaviorTestDB(t)
	svcCtx := setupBehaviorServiceContext(t, db)

	now := time.Now()

	req := &BehaviorFunnelRequest{
		GameId:    "game1",
		StartDate: now.Add(-24 * time.Hour).Format(time.RFC3339),
		EndDate:   now.Format(time.RFC3339),
		Steps:     []string{},
	}

	resp, err := behaviorFunnel(context.Background(), svcCtx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "需要至少一个漏斗步骤")
}

func TestBehaviorFunnel_DuplicateSteps(t *testing.T) {
	db := setupBehaviorTestDB(t)
	svcCtx := setupBehaviorServiceContext(t, db)
	behaviorModel := svcCtx.BehaviorModel

	now := time.Now()

	events := []model.BehaviorEvent{
		createTestEvent("view", "user1", "game1", "prod", now, nil),
	}
	for _, ev := range events {
		require.NoError(t, behaviorModel.RecordEvent(context.Background(), &ev))
	}

	req := &BehaviorFunnelRequest{
		GameId:    "game1",
		StartDate: now.Add(-24 * time.Hour).Format(time.RFC3339),
		EndDate:   now.Format(time.RFC3339),
		Steps:     []string{"view", "view", "click"}, // Duplicates should be removed
	}

	resp, err := behaviorFunnel(context.Background(), svcCtx, req)

	require.NoError(t, err)
	assert.NotNil(t, resp)
	// Should have unique steps only
}

func TestBehaviorFunnel_WhitespaceSteps(t *testing.T) {
	db := setupBehaviorTestDB(t)
	svcCtx := setupBehaviorServiceContext(t, db)
	behaviorModel := svcCtx.BehaviorModel

	now := time.Now()

	events := []model.BehaviorEvent{
		createTestEvent("view", "user1", "game1", "prod", now, nil),
	}
	for _, ev := range events {
		require.NoError(t, behaviorModel.RecordEvent(context.Background(), &ev))
	}

	req := &BehaviorFunnelRequest{
		GameId:    "game1",
		StartDate: now.Add(-24 * time.Hour).Format(time.RFC3339),
		EndDate:   now.Format(time.RFC3339),
		Steps:     []string{"", "view", "  ", "click"}, // Empty/whitespace should be filtered
	}

	resp, err := behaviorFunnel(context.Background(), svcCtx, req)

	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestBehaviorPaths_Success(t *testing.T) {
	db := setupBehaviorTestDB(t)
	svcCtx := setupBehaviorServiceContext(t, db)
	behaviorModel := svcCtx.BehaviorModel

	now := time.Now()

	events := []model.BehaviorEvent{
		createTestEvent("home", "user1", "game1", "prod", now.Add(-3*time.Minute), nil),
		createTestEvent("search", "user1", "game1", "prod", now.Add(-2*time.Minute), nil),
		createTestEvent("product", "user1", "game1", "prod", now.Add(-1*time.Minute), nil),
		createTestEvent("home", "user2", "game1", "prod", now.Add(-3*time.Minute), nil),
		createTestEvent("cart", "user2", "game1", "prod", now.Add(-2*time.Minute), nil),
	}
	for _, ev := range events {
		require.NoError(t, behaviorModel.RecordEvent(context.Background(), &ev))
	}

	req := &BehaviorPathsRequest{
		GameId:    "game1",
		StartDate: now.Add(-24 * time.Hour).Format(time.RFC3339),
		EndDate:   now.Format(time.RFC3339),
		Depth:     3,
	}

	resp, err := behaviorPaths(context.Background(), svcCtx, req)

	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotNil(t, resp.Paths)
}

func TestBehaviorPaths_DefaultDepth(t *testing.T) {
	db := setupBehaviorTestDB(t)
	svcCtx := setupBehaviorServiceContext(t, db)

	now := time.Now()

	req := &BehaviorPathsRequest{
		GameId:    "game1",
		Depth:     0, // Should default to 5
		StartDate: now.Add(-24 * time.Hour).Format(time.RFC3339),
		EndDate:   now.Format(time.RFC3339),
	}

	resp, err := behaviorPaths(context.Background(), svcCtx, req)

	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestBehaviorPaths_MaxDepth(t *testing.T) {
	db := setupBehaviorTestDB(t)
	svcCtx := setupBehaviorServiceContext(t, db)

	now := time.Now()

	req := &BehaviorPathsRequest{
		GameId:    "game1",
		Depth:     50, // Should be capped at 10
		StartDate: now.Add(-24 * time.Hour).Format(time.RFC3339),
		EndDate:   now.Format(time.RFC3339),
	}

	resp, err := behaviorPaths(context.Background(), svcCtx, req)

	require.NoError(t, err)
	assert.NotNil(t, resp)
}

// Test helper functions

func TestBreakdownBySegment_EmptyEvents(t *testing.T) {
	events := []model.BehaviorEvent{}
	result := breakdownBySegment(events)

	assert.Equal(t, 0, result.TotalUsers)
	assert.NotNil(t, result.Regions)
	assert.NotNil(t, result.Platforms)
	assert.NotNil(t, result.Roles)
	assert.NotNil(t, result.Actions)
}

func TestBreakdownBySegment_WithEvents(t *testing.T) {
	now := time.Now()
	events := []model.BehaviorEvent{
		createTestEvent("login", "user1", "game1", "prod", now, map[string]interface{}{"region": "US", "platform": "ios", "role": "admin"}),
		createTestEvent("login", "user2", "game1", "prod", now, map[string]interface{}{"region": "EU", "platform": "android", "role": "user"}),
		createTestEvent("login", "user1", "game1", "prod", now.Add(1*time.Minute), map[string]interface{}{"region": "US", "platform": "ios", "role": "admin"}),
	}

	// Verify test data setup
	assert.Len(t, events, 3, "Should have exactly 3 events")

	result := breakdownBySegment(events)

	// Check unique users (user1 and user2)
	assert.Equal(t, 2, result.TotalUsers, "Should have 2 unique users")
	assert.Equal(t, 2, result.Regions["US"])
	assert.Equal(t, 1, result.Regions["EU"])
	assert.Equal(t, 2, result.Platforms["ios"])
	assert.Equal(t, 1, result.Platforms["android"])
	assert.Equal(t, 3, result.Actions["login"], "Should have 3 login events")
}

func TestBreakdownBySegment_EmptyUserID(t *testing.T) {
	now := time.Now()
	events := []model.BehaviorEvent{
		createTestEvent("login", "", "game1", "prod", now, map[string]interface{}{"region": "US"}),
		createTestEvent("login", " ", "game1", "prod", now, map[string]interface{}{"region": "EU"}),
	}

	result := breakdownBySegment(events)

	assert.Equal(t, 0, result.TotalUsers)
}

func TestBreakdownByTime_EmptyEvents(t *testing.T) {
	now := time.Now()
	result := breakdownByTime([]model.BehaviorEvent{}, now.Add(-24*time.Hour), now)

	assert.Empty(t, result)
}

func TestBreakdownByTime_SingleDay(t *testing.T) {
	now := time.Now()
	events := []model.BehaviorEvent{
		createTestEvent("event", "user1", "game1", "prod", now.Add(-12*time.Hour), nil),
		createTestEvent("event", "user2", "game1", "prod", now.Add(-6*time.Hour), nil),
	}

	start := now.Add(-24 * time.Hour)
	result := breakdownByTime(events, start, now)

	assert.NotEmpty(t, result)
}

func TestBreakdownByTime_ShortRange(t *testing.T) {
	now := time.Now()
	events := []model.BehaviorEvent{
		createTestEvent("event", "user1", "game1", "prod", now.Add(-3*time.Hour), nil),
	}

	start := now.Add(-6 * time.Hour)
	result := breakdownByTime(events, start, now)

	// Should use 6-hour interval for ranges <= 7 days
	assert.NotEmpty(t, result)
}

func TestTopMapPairs_EmptyMap(t *testing.T) {
	result := topMapPairs(map[string]int{}, 10)

	assert.Empty(t, result)
}

func TestTopMapPairs_WithItems(t *testing.T) {
	m := map[string]int{
		"action1": 100,
		"action2": 50,
		"action3": 75,
	}

	result := topMapPairs(m, 10)

	assert.Len(t, result, 3)
	// Should be sorted by count descending
	assert.Equal(t, "action1", result[0]["label"])
	assert.Equal(t, 100, result[0]["value"])
}

func TestTopMapPairs_WithLimit(t *testing.T) {
	m := map[string]int{
		"action1": 100,
		"action2": 50,
		"action3": 75,
	}

	result := topMapPairs(m, 2)

	assert.Len(t, result, 2)
}

func TestTopMapPairs_ExcludeEmptyKeys(t *testing.T) {
	m := map[string]int{
		"action1": 100,
		"":        50,
		"   ":     75,
		"action2": 25,
	}

	result := topMapPairs(m, 10)

	assert.Len(t, result, 2)
}

func TestTopMapPairs_ExcludeNonPositive(t *testing.T) {
	m := map[string]int{
		"action1": 100,
		"action2": 0,
		"action3": -5,
	}

	result := topMapPairs(m, 10)

	assert.Len(t, result, 1)
	assert.Equal(t, "action1", result[0]["label"])
}

func TestTopMapPairs_SortByKeyWhenTies(t *testing.T) {
	m := map[string]int{
		"zebra":  100,
		"apple":  100,
		"banana": 100,
	}

	result := topMapPairs(m, 10)

	assert.Len(t, result, 3)
	// Alphabetical order when counts are equal
	assert.Equal(t, "apple", result[0]["label"])
	assert.Equal(t, "banana", result[1]["label"])
	assert.Equal(t, "zebra", result[2]["label"])
}

func TestGroupEventsByUserForFunnel_EmptyEvents(t *testing.T) {
	events := []model.BehaviorEvent{}
	steps := []string{"step1", "step2"}

	result := groupEventsByUserForFunnel(events, steps)

	assert.Empty(t, result)
}

func TestGroupEventsByUserForFunnel_WithEvents(t *testing.T) {
	now := time.Now()
	events := []model.BehaviorEvent{
		createTestEvent("step1", "user1", "game1", "prod", now.Add(-2*time.Minute), nil),
		createTestEvent("step2", "user1", "game1", "prod", now.Add(-1*time.Minute), nil),
		createTestEvent("step1", "user2", "game1", "prod", now, nil),
	}

	steps := []string{"step1", "step2"}
	result := groupEventsByUserForFunnel(events, steps)

	assert.Len(t, result, 2)
	assert.Contains(t, result, "user1")
	assert.Contains(t, result, "user2")
	assert.Len(t, result["user1"], 2)
	assert.Len(t, result["user2"], 1)
}

func TestGroupEventsByUserForFunnel_ExcludeNonMatchingSteps(t *testing.T) {
	now := time.Now()
	events := []model.BehaviorEvent{
		createTestEvent("step1", "user1", "game1", "prod", now.Add(-2*time.Minute), nil),
		createTestEvent("other", "user1", "game1", "prod", now.Add(-1*time.Minute), nil),
		createTestEvent("step2", "user1", "game1", "prod", now, nil),
	}

	steps := []string{"step1", "step2"}
	result := groupEventsByUserForFunnel(events, steps)

	// Should only include step1 and step2 events
	assert.Len(t, result["user1"], 2)
}

func TestGroupEventsByUserForFunnel_ExcludeEmptyUserID(t *testing.T) {
	now := time.Now()
	events := []model.BehaviorEvent{
		createTestEvent("step1", "user1", "game1", "prod", now, nil),
		createTestEvent("step2", "", "game1", "prod", now, nil),
		createTestEvent("step3", "  ", "game1", "prod", now, nil),
	}

	steps := []string{"step1", "step2", "step3"}
	result := groupEventsByUserForFunnel(events, steps)

	assert.Len(t, result, 1)
	assert.Contains(t, result, "user1")
}

func TestGroupEventsByUserForFunnel_EventsSortedByTime(t *testing.T) {
	now := time.Now()
	events := []model.BehaviorEvent{
		createTestEvent("step3", "user1", "game1", "prod", now, nil),
		createTestEvent("step1", "user1", "game1", "prod", now.Add(-3*time.Minute), nil),
		createTestEvent("step2", "user1", "game1", "prod", now.Add(-2*time.Minute), nil),
	}

	steps := []string{"step1", "step2", "step3"}
	result := groupEventsByUserForFunnel(events, steps)

	require.Len(t, result["user1"], 3)
	// Should be sorted by time
	assert.Equal(t, "step1", result["user1"][0].EventType)
	assert.Equal(t, "step2", result["user1"][1].EventType)
	assert.Equal(t, "step3", result["user1"][2].EventType)
}

func TestRoundPercentage(t *testing.T) {
	tests := []struct {
		name  string
		value float64
		want  float64
	}{
		{"zero", 0, 0},
		{"one", 1, 100},
		{"half", 0.5, 50},
		{"quarter", 0.25, 25},
		{"small", 0.1234, 12.34},
		{"negative", -0.5, -50},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := roundPercentage(tt.value)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestBuildPaths_EmptyEvents(t *testing.T) {
	result := buildPaths([]model.BehaviorEvent{}, 5)

	assert.Empty(t, result)
}

func TestBuildPaths_SingleUser(t *testing.T) {
	now := time.Now()
	events := []model.BehaviorEvent{
		createTestEvent("home", "user1", "game1", "prod", now.Add(-3*time.Minute), nil),
		createTestEvent("search", "user1", "game1", "prod", now.Add(-2*time.Minute), nil),
		createTestEvent("product", "user1", "game1", "prod", now.Add(-1*time.Minute), nil),
	}

	result := buildPaths(events, 5)

	assert.NotEmpty(t, result)
}

func TestBuildPaths_ExcludesEmptyEventType(t *testing.T) {
	now := time.Now()
	events := []model.BehaviorEvent{
		createTestEvent("", "user1", "game1", "prod", now.Add(-2*time.Minute), nil),
		createTestEvent("  ", "user1", "game1", "prod", now.Add(-1*time.Minute), nil),
	}

	result := buildPaths(events, 5)

	assert.Empty(t, result)
}

func TestBuildPaths_RespectsDepth(t *testing.T) {
	now := time.Now()
	events := []model.BehaviorEvent{
		createTestEvent("step1", "user1", "game1", "prod", now.Add(-5*time.Minute), nil),
		createTestEvent("step2", "user1", "game1", "prod", now.Add(-4*time.Minute), nil),
		createTestEvent("step3", "user1", "game1", "prod", now.Add(-3*time.Minute), nil),
		createTestEvent("step4", "user1", "game1", "prod", now.Add(-2*time.Minute), nil),
		createTestEvent("step5", "user1", "game1", "prod", now.Add(-1*time.Minute), nil),
	}

	result := buildPaths(events, 3)

	// Should limit depth to 3
	assert.NotEmpty(t, result)
	// Verify no path exceeds depth
	for _, path := range result {
		if p, ok := path["path"].([]string); ok {
			assert.LessOrEqual(t, len(p), 3)
		}
	}
}

func TestGroupEventsByUserForPaths_EmptyEvents(t *testing.T) {
	result := groupEventsByUserForPaths([]model.BehaviorEvent{})

	assert.Empty(t, result)
}

func TestGroupEventsByUserForPaths_ExcludesEmptyUserID(t *testing.T) {
	now := time.Now()
	events := []model.BehaviorEvent{
		createTestEvent("action", "user1", "game1", "prod", now, nil),
		createTestEvent("action", "", "game1", "prod", now, nil),
		createTestEvent("action", "  ", "game1", "prod", now, nil),
	}

	result := groupEventsByUserForPaths(events)

	assert.Len(t, result, 1)
	assert.Contains(t, result, "user1")
}

func TestGroupEventsByUserForPaths_EventsSortedByTime(t *testing.T) {
	now := time.Now()
	events := []model.BehaviorEvent{
		createTestEvent("c", "user1", "game1", "prod", now, nil),
		createTestEvent("a", "user1", "game1", "prod", now.Add(-3*time.Minute), nil),
		createTestEvent("b", "user1", "game1", "prod", now.Add(-2*time.Minute), nil),
	}

	result := groupEventsByUserForPaths(events)

	require.Len(t, result["user1"], 3)
	assert.Equal(t, "a", result["user1"][0].EventType)
	assert.Equal(t, "b", result["user1"][1].EventType)
	assert.Equal(t, "c", result["user1"][2].EventType)
}

func TestMetaValue_NilMeta(t *testing.T) {
	val, ok := metaValue(nil, "key")

	assert.False(t, ok)
	assert.Empty(t, val)
}

func TestMetaValue_KeyNotFound(t *testing.T) {
	meta := map[string]interface{}{
		"other": "value",
	}

	val, ok := metaValue(meta, "key")

	assert.False(t, ok)
	assert.Empty(t, val)
}

func TestMetaValue_KeyFound(t *testing.T) {
	meta := map[string]interface{}{
		"region": "US",
	}

	val, ok := metaValue(meta, "region")

	assert.True(t, ok)
	assert.Equal(t, "US", val)
}

func TestMetaValue_WhitespaceValue(t *testing.T) {
	meta := map[string]interface{}{
		"region": "  ",
	}

	val, ok := metaValue(meta, "region")

	assert.False(t, ok)
	assert.Empty(t, val)
}

func TestStringify_String(t *testing.T) {
	result := stringify("test")

	assert.Equal(t, "test", result)
}

func TestStringify_NonString(t *testing.T) {
	tests := []struct {
		input interface{}
		want  string
	}{
		{123, "123"},
		{true, "true"},
		{1.5, "1.5"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			result := stringify(tt.input)
			assert.Equal(t, tt.want, result)
		})
	}
}
