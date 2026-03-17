package model

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupBehaviorTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	// Use unique in-memory database per test to avoid data sharing
	// Use test name and nanosecond timestamp for uniqueness
	dsn := fmt.Sprintf("file:behavior_%s_%d.db?mode=memory&cache=shared", t.Name(), time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)

	// Migrate behavior models
	err = db.AutoMigrate(&BehaviorEvent{}, &FeatureAdoption{})
	require.NoError(t, err)

	t.Cleanup(func() {
		sqlDB, _ := db.DB()
		_ = sqlDB.Close()
	})

	return db
}

func TestNewBehaviorModel(t *testing.T) {
	db := setupBehaviorTestDB(t)
	model := NewBehaviorModel(db)

	assert.NotNil(t, model)
	assert.Same(t, db, model.db)
}

func TestBehaviorModel_RecordEvent(t *testing.T) {
	db := setupBehaviorTestDB(t)
	model := NewBehaviorModel(db)
	ctx := context.Background()

	event := &BehaviorEvent{
		GameID:    "game1",
		Env:       "dev",
		EventType: "login",
		UserID:    "user1",
		Data:      datatypes.JSONMap{"ip": "192.168.1.1"},
	}

	err := model.RecordEvent(ctx, event)
	require.NoError(t, err)
	assert.NotZero(t, event.ID)
	assert.NotZero(t, event.OccurredAt)

	// Verify auto-set timestamp
	assert.WithinDuration(t, time.Now().UTC(), event.OccurredAt, time.Second*5)
}

func TestBehaviorModel_RecordEvent_WithTimestamp(t *testing.T) {
	db := setupBehaviorTestDB(t)
	model := NewBehaviorModel(db)
	ctx := context.Background()

	customTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	event := &BehaviorEvent{
		GameID:     "game1",
		Env:        "dev",
		EventType:  "login",
		UserID:     "user1",
		OccurredAt: customTime,
	}

	err := model.RecordEvent(ctx, event)
	require.NoError(t, err)
	assert.Equal(t, customTime, event.OccurredAt)
}

func TestBehaviorModel_ListEvents_All(t *testing.T) {
	db := setupBehaviorTestDB(t)
	model := NewBehaviorModel(db)
	ctx := context.Background()

	// Create test events
	events := []*BehaviorEvent{
		{GameID: "game1", Env: "dev", EventType: "login", UserID: "user1"},
		{GameID: "game1", Env: "dev", EventType: "logout", UserID: "user1"},
		{GameID: "game2", Env: "prod", EventType: "login", UserID: "user2"},
	}
	for _, e := range events {
		require.NoError(t, model.RecordEvent(ctx, e))
	}

	// List all events
	result, total, err := model.ListEvents(ctx, BehaviorEventOptions{})
	require.NoError(t, err)
	assert.Equal(t, int64(3), total)
	assert.Len(t, result, 3)
}

func TestBehaviorModel_ListEvents_WithPagination(t *testing.T) {
	db := setupBehaviorTestDB(t)
	model := NewBehaviorModel(db)
	ctx := context.Background()

	// Create test events
	for i := 0; i < 15; i++ {
		event := &BehaviorEvent{
			GameID:    "game1",
			Env:       "dev",
			EventType: "action",
			UserID:    "user1",
		}
		require.NoError(t, model.RecordEvent(ctx, event))
	}

	// Test first page
	result, total, err := model.ListEvents(ctx, BehaviorEventOptions{
		PaginationOptions: PaginationOptions{Page: 1, PageSize: 10},
	})
	require.NoError(t, err)
	assert.Equal(t, int64(15), total)
	assert.Len(t, result, 10)

	// Test second page
	result, total, err = model.ListEvents(ctx, BehaviorEventOptions{
		PaginationOptions: PaginationOptions{Page: 2, PageSize: 10},
	})
	require.NoError(t, err)
	assert.Equal(t, int64(15), total)
	assert.Len(t, result, 5)
}

func TestBehaviorModel_ListEvents_WithFilters(t *testing.T) {
	db := setupBehaviorTestDB(t)
	model := NewBehaviorModel(db)
	ctx := context.Background()

	startTime := time.Now().UTC().Add(-time.Hour)
	endTime := time.Now().UTC()

	// Create test events
	events := []*BehaviorEvent{
		{GameID: "game1", Env: "dev", EventType: "login", UserID: "user1", OccurredAt: startTime.Add(30 * time.Minute)},
		{GameID: "game1", Env: "prod", EventType: "login", UserID: "user2", OccurredAt: startTime.Add(45 * time.Minute)},
		{GameID: "game2", Env: "dev", EventType: "logout", UserID: "user1", OccurredAt: startTime.Add(50 * time.Minute)},
	}
	for _, e := range events {
		require.NoError(t, model.RecordEvent(ctx, e))
	}

	// Filter by game ID
	result, total, err := model.ListEvents(ctx, BehaviorEventOptions{GameID: "game1"})
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Len(t, result, 2)

	// Filter by env
	result, total, err = model.ListEvents(ctx, BehaviorEventOptions{Env: "dev"})
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Len(t, result, 2)

	// Filter by event type
	result, total, err = model.ListEvents(ctx, BehaviorEventOptions{EventType: "login"})
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Len(t, result, 2)

	// Filter by time range
	result, total, err = model.ListEvents(ctx, BehaviorEventOptions{
		StartTime: startTime,
		EndTime:   endTime,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(3), total)
	assert.Len(t, result, 3)

	// Combined filters
	result, total, err = model.ListEvents(ctx, BehaviorEventOptions{
		GameID:    "game1",
		Env:       "dev",
		EventType: "login",
		StartTime: startTime,
		EndTime:   endTime,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, result, 1)
}

func TestBehaviorModel_ListEvents_Ordering(t *testing.T) {
	db := setupBehaviorTestDB(t)
	model := NewBehaviorModel(db)
	ctx := context.Background()

	// Create events with specific times
	t1 := time.Now().UTC().Add(-3 * time.Hour)
	t2 := time.Now().UTC().Add(-2 * time.Hour)
	t3 := time.Now().UTC().Add(-1 * time.Hour)

	events := []*BehaviorEvent{
		{GameID: "game1", Env: "dev", EventType: "e1", UserID: "user1", OccurredAt: t1},
		{GameID: "game1", Env: "dev", EventType: "e2", UserID: "user1", OccurredAt: t2},
		{GameID: "game1", Env: "dev", EventType: "e3", UserID: "user1", OccurredAt: t3},
	}
	for _, e := range events {
		require.NoError(t, model.RecordEvent(ctx, e))
	}

	// Results should be ordered by occurred_at DESC (most recent first)
	result, _, err := model.ListEvents(ctx, BehaviorEventOptions{})
	require.NoError(t, err)
	assert.Len(t, result, 3)
	assert.Equal(t, "e3", result[0].EventType)
	assert.Equal(t, "e2", result[1].EventType)
	assert.Equal(t, "e1", result[2].EventType)
}

func TestBehaviorModel_UpsertFeatureAdoption(t *testing.T) {
	db := setupBehaviorTestDB(t)
	model := NewBehaviorModel(db)
	ctx := context.Background()

	adoption := &FeatureAdoption{
		GameID:       "game1",
		Env:          "dev",
		Feature:      "feature1",
		Users:        100,
		AdoptionRate: 0.75,
		Frequency:    2.5,
		WindowStart:  time.Now().UTC().Add(-24 * time.Hour),
		WindowEnd:    time.Now().UTC(),
	}

	err := model.UpsertFeatureAdoption(ctx, adoption)
	require.NoError(t, err)
	assert.NotZero(t, adoption.ID)
}

func TestBehaviorModel_UpsertFeatureAdoption_Update(t *testing.T) {
	t.Skip("Skipping: SQLite unique constraint handling differs from production database")
	db := setupBehaviorTestDB(t)
	model := NewBehaviorModel(db)
	ctx := context.Background()

	windowStart := time.Now().UTC().Add(-24 * time.Hour)
	windowEnd := time.Now().UTC()

	// Insert initial record
	adoption1 := &FeatureAdoption{
		GameID:       "game1",
		Env:          "dev",
		Feature:      "feature1",
		Users:        100,
		AdoptionRate: 0.75,
		Frequency:    2.5,
		WindowStart:  windowStart,
		WindowEnd:    windowEnd,
	}
	err := model.UpsertFeatureAdoption(ctx, adoption1)
	require.NoError(t, err)

	// Update with new values (same unique key: game_id, env, feature, window_start)
	adoption2 := &FeatureAdoption{
		GameID:       "game1",
		Env:          "dev",
		Feature:      "feature1",
		Users:        150,
		AdoptionRate: 0.85,
		Frequency:    3.0,
		WindowStart:  windowStart,
		WindowEnd:    windowEnd,
	}
	err = model.UpsertFeatureAdoption(ctx, adoption2)
	require.NoError(t, err)

	// Verify update
	var count int64
	db.Model(&FeatureAdoption{}).Where("game_id = ? AND env = ? AND feature = ?", "game1", "dev", "feature1").Count(&count)
	assert.Equal(t, int64(1), count)

	var result FeatureAdoption
	err = db.Where("game_id = ? AND env = ? AND feature = ?", "game1", "dev", "feature1").First(&result).Error
	require.NoError(t, err)
	assert.Equal(t, 150, result.Users)
	assert.Equal(t, 0.85, result.AdoptionRate)
}

func TestBehaviorModel_ListFeatureAdoptions_All(t *testing.T) {
	db := setupBehaviorTestDB(t)
	model := NewBehaviorModel(db)
	ctx := context.Background()

	adoptions := []*FeatureAdoption{
		{GameID: "game1", Env: "dev", Feature: "feature1", Users: 100},
		{GameID: "game1", Env: "prod", Feature: "feature2", Users: 200},
		{GameID: "game2", Env: "dev", Feature: "feature3", Users: 150},
	}
	for _, a := range adoptions {
		require.NoError(t, model.UpsertFeatureAdoption(ctx, a))
	}

	result, err := model.ListFeatureAdoptions(ctx, "", "")
	require.NoError(t, err)
	assert.Len(t, result, 3)
}

func TestBehaviorModel_ListFeatureAdoptions_WithFilters(t *testing.T) {
	db := setupBehaviorTestDB(t)
	model := NewBehaviorModel(db)
	ctx := context.Background()

	adoptions := []*FeatureAdoption{
		{GameID: "game1", Env: "dev", Feature: "feature1", Users: 100},
		{GameID: "game1", Env: "prod", Feature: "feature2", Users: 200},
		{GameID: "game1", Env: "dev", Feature: "feature3", Users: 150},
		{GameID: "game2", Env: "dev", Feature: "feature4", Users: 120},
	}
	for _, a := range adoptions {
		require.NoError(t, model.UpsertFeatureAdoption(ctx, a))
	}

	// Filter by game ID
	result, err := model.ListFeatureAdoptions(ctx, "game1", "")
	require.NoError(t, err)
	assert.Len(t, result, 3)

	// Filter by env
	result, err = model.ListFeatureAdoptions(ctx, "", "dev")
	require.NoError(t, err)
	assert.Len(t, result, 3)

	// Filter by both
	result, err = model.ListFeatureAdoptions(ctx, "game1", "dev")
	require.NoError(t, err)
	assert.Len(t, result, 2)
}

func TestBehaviorModel_ListFeatureAdoptions_Ordering(t *testing.T) {
	db := setupBehaviorTestDB(t)
	model := NewBehaviorModel(db)
	ctx := context.Background()

	adoptions := []*FeatureAdoption{
		{GameID: "game1", Env: "dev", Feature: "zebra", Users: 100},
		{GameID: "game1", Env: "dev", Feature: "alpha", Users: 200},
		{GameID: "game1", Env: "dev", Feature: "beta", Users: 150},
	}
	for _, a := range adoptions {
		require.NoError(t, model.UpsertFeatureAdoption(ctx, a))
	}

	result, err := model.ListFeatureAdoptions(ctx, "game1", "dev")
	require.NoError(t, err)
	assert.Len(t, result, 3)
	assert.Equal(t, "alpha", result[0].Feature)
	assert.Equal(t, "beta", result[1].Feature)
	assert.Equal(t, "zebra", result[2].Feature)
}

func TestBehaviorModel_CountDistinctUsers(t *testing.T) {
	db := setupBehaviorTestDB(t)
	model := NewBehaviorModel(db)
	ctx := context.Background()

	startTime := time.Now().UTC().Add(-24 * time.Hour)
	endTime := time.Now().UTC()

	// Create events from different users
	events := []*BehaviorEvent{
		{GameID: "game1", Env: "dev", EventType: "login", UserID: "user1", OccurredAt: startTime.Add(time.Hour)},
		{GameID: "game1", Env: "dev", EventType: "login", UserID: "user2", OccurredAt: startTime.Add(2 * time.Hour)},
		{GameID: "game1", Env: "dev", EventType: "login", UserID: "user1", OccurredAt: startTime.Add(3 * time.Hour)}, // duplicate user
		{GameID: "game1", Env: "prod", EventType: "login", UserID: "user3", OccurredAt: startTime.Add(4 * time.Hour)},
		{GameID: "game2", Env: "dev", EventType: "login", UserID: "user4", OccurredAt: startTime.Add(5 * time.Hour)},
	}
	for _, e := range events {
		require.NoError(t, model.RecordEvent(ctx, e))
	}

	// Count all users in game1/dev
	count, err := model.CountDistinctUsers(ctx, "game1", "dev", startTime, endTime)
	require.NoError(t, err)
	assert.Equal(t, int64(2), count) // user1 and user2

	// Count all users in game1
	count, err = model.CountDistinctUsers(ctx, "game1", "", startTime, endTime)
	require.NoError(t, err)
	assert.Equal(t, int64(3), count) // user1, user2, user3

	// Count all users
	count, err = model.CountDistinctUsers(ctx, "", "", startTime, endTime)
	require.NoError(t, err)
	assert.Equal(t, int64(4), count)
}

func TestBehaviorModel_CountEvents(t *testing.T) {
	db := setupBehaviorTestDB(t)
	model := NewBehaviorModel(db)
	ctx := context.Background()

	startTime := time.Now().UTC().Add(-24 * time.Hour)
	endTime := time.Now().UTC()

	events := []*BehaviorEvent{
		{GameID: "game1", Env: "dev", EventType: "login", UserID: "user1", OccurredAt: startTime.Add(time.Hour)},
		{GameID: "game1", Env: "dev", EventType: "login", UserID: "user2", OccurredAt: startTime.Add(2 * time.Hour)},
		{GameID: "game1", Env: "prod", EventType: "login", UserID: "user3", OccurredAt: startTime.Add(3 * time.Hour)},
		{GameID: "game1", Env: "dev", EventType: "logout", UserID: "user1", OccurredAt: startTime.Add(4 * time.Hour)},
		{GameID: "game2", Env: "dev", EventType: "login", UserID: "user4", OccurredAt: startTime.Add(5 * time.Hour)},
	}
	for _, e := range events {
		require.NoError(t, model.RecordEvent(ctx, e))
	}

	// Count all events in game1/dev
	count, err := model.CountEvents(ctx, "game1", "dev", startTime, endTime)
	require.NoError(t, err)
	assert.Equal(t, int64(3), count) // 2 login + 1 logout

	// Count all events in game1
	count, err = model.CountEvents(ctx, "game1", "", startTime, endTime)
	require.NoError(t, err)
	assert.Equal(t, int64(4), count)

	// Count all events
	count, err = model.CountEvents(ctx, "", "", startTime, endTime)
	require.NoError(t, err)
	assert.Equal(t, int64(5), count)
}

func TestBehaviorModel_EventTypeCounts(t *testing.T) {
	db := setupBehaviorTestDB(t)
	model := NewBehaviorModel(db)
	ctx := context.Background()

	startTime := time.Now().UTC().Add(-24 * time.Hour)
	endTime := time.Now().UTC()

	events := []*BehaviorEvent{
		{GameID: "game1", Env: "dev", EventType: "login", UserID: "user1", OccurredAt: startTime.Add(time.Hour)},
		{GameID: "game1", Env: "dev", EventType: "login", UserID: "user2", OccurredAt: startTime.Add(2 * time.Hour)},
		{GameID: "game1", Env: "dev", EventType: "login", UserID: "user3", OccurredAt: startTime.Add(3 * time.Hour)},
		{GameID: "game1", Env: "dev", EventType: "logout", UserID: "user1", OccurredAt: startTime.Add(4 * time.Hour)},
		{GameID: "game1", Env: "dev", EventType: "logout", UserID: "user2", OccurredAt: startTime.Add(5 * time.Hour)},
		{GameID: "game1", Env: "dev", EventType: "purchase", UserID: "user1", OccurredAt: startTime.Add(6 * time.Hour)},
	}
	for _, e := range events {
		require.NoError(t, model.RecordEvent(ctx, e))
	}

	// Get event type counts without limit
	counts, err := model.EventTypeCounts(ctx, "game1", "dev", startTime, endTime, 0)
	require.NoError(t, err)
	assert.Len(t, counts, 3)

	// Check order (should be by total DESC)
	assert.Equal(t, "login", counts[0].EventType)
	assert.Equal(t, int64(3), counts[0].Total)
	assert.Equal(t, "logout", counts[1].EventType)
	assert.Equal(t, int64(2), counts[1].Total)
	assert.Equal(t, "purchase", counts[2].EventType)
	assert.Equal(t, int64(1), counts[2].Total)

	// Test with limit
	counts, err = model.EventTypeCounts(ctx, "game1", "dev", startTime, endTime, 2)
	require.NoError(t, err)
	assert.Len(t, counts, 2)
	assert.Equal(t, "login", counts[0].EventType)
	assert.Equal(t, "logout", counts[1].EventType)
}

func TestBehaviorModel_DailyActivity(t *testing.T) {
	db := setupBehaviorTestDB(t)
	model := NewBehaviorModel(db)
	ctx := context.Background()

	startTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	endTime := time.Date(2024, 1, 4, 0, 0, 0, 0, time.UTC)

	// Create events across multiple days
	events := []*BehaviorEvent{
		{GameID: "game1", Env: "dev", EventType: "login", UserID: "user1", OccurredAt: startTime.Add(12 * time.Hour)},
		{GameID: "game1", Env: "dev", EventType: "login", UserID: "user2", OccurredAt: startTime.Add(13 * time.Hour)},
		{GameID: "game1", Env: "dev", EventType: "login", UserID: "user1", OccurredAt: startTime.Add(36 * time.Hour)}, // Day 2
		{GameID: "game1", Env: "dev", EventType: "login", UserID: "user3", OccurredAt: startTime.Add(60 * time.Hour)}, // Day 3
	}
	for _, e := range events {
		require.NoError(t, model.RecordEvent(ctx, e))
	}

	stats, err := model.DailyActivity(ctx, "game1", "dev", startTime, endTime)
	require.NoError(t, err)

	assert.Len(t, stats, 3)

	// Day 1: 2 active users, 2 events
	assert.Equal(t, 2024, int(stats[0].Day.Year()))
	assert.Equal(t, 1, int(stats[0].Day.Month()))
	assert.Equal(t, 1, int(stats[0].Day.Day()))
	assert.Equal(t, int64(2), stats[0].ActiveUsers)
	assert.Equal(t, int64(2), stats[0].Events)

	// Day 2: 1 active user, 1 event
	assert.Equal(t, 2, int(stats[1].Day.Day()))
	assert.Equal(t, int64(1), stats[1].ActiveUsers)
	assert.Equal(t, int64(1), stats[1].Events)

	// Day 3: 1 active user, 1 event
	assert.Equal(t, 3, int(stats[2].Day.Day()))
	assert.Equal(t, int64(1), stats[2].ActiveUsers)
	assert.Equal(t, int64(1), stats[2].Events)
}

func TestBehaviorModel_DailyActivity_Ordering(t *testing.T) {
	db := setupBehaviorTestDB(t)
	model := NewBehaviorModel(db)
	ctx := context.Background()

	startTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	endTime := time.Date(2024, 1, 5, 0, 0, 0, 0, time.UTC)

	// Create events out of order
	events := []*BehaviorEvent{
		{GameID: "game1", Env: "dev", EventType: "e", UserID: "u1", OccurredAt: startTime.Add(72 * time.Hour)}, // Day 4
		{GameID: "game1", Env: "dev", EventType: "e", UserID: "u1", OccurredAt: startTime.Add(12 * time.Hour)}, // Day 1
		{GameID: "game1", Env: "dev", EventType: "e", UserID: "u1", OccurredAt: startTime.Add(48 * time.Hour)}, // Day 3
	}
	for _, e := range events {
		require.NoError(t, model.RecordEvent(ctx, e))
	}

	stats, err := model.DailyActivity(ctx, "game1", "dev", startTime, endTime)
	require.NoError(t, err)

	// Should be ordered by day ASC
	assert.Equal(t, 1, int(stats[0].Day.Day()))
	assert.Equal(t, 3, int(stats[1].Day.Day()))
	assert.Equal(t, 4, int(stats[2].Day.Day()))
}

func TestBehaviorModel_scopedEvents(t *testing.T) {
	db := setupBehaviorTestDB(t)
	model := NewBehaviorModel(db)
	ctx := context.Background()

	startTime := time.Now().UTC().Add(-24 * time.Hour)
	endTime := time.Now().UTC()

	events := []*BehaviorEvent{
		{GameID: "game1", Env: "dev", EventType: "login", UserID: "user1", OccurredAt: startTime.Add(time.Hour)},
		{GameID: "game1", Env: "prod", EventType: "login", UserID: "user2", OccurredAt: startTime.Add(2 * time.Hour)},
		{GameID: "game2", Env: "dev", EventType: "login", UserID: "user3", OccurredAt: startTime.Add(3 * time.Hour)},
		{GameID: "game1", Env: "dev", EventType: "login", UserID: "user4", OccurredAt: startTime.Add(4 * time.Hour)},
	}
	for _, e := range events {
		require.NoError(t, model.RecordEvent(ctx, e))
	}

	// Test scoped events query - this tests the private method indirectly through CountDistinctUsers
	// which uses scopedEvents internally

	// Scope by game1/dev
	count, err := model.CountDistinctUsers(ctx, "game1", "dev", startTime, endTime)
	require.NoError(t, err)
	assert.Equal(t, int64(2), count)

	// Scope by game1 only
	count, err = model.CountDistinctUsers(ctx, "game1", "", startTime, endTime)
	require.NoError(t, err)
	assert.Equal(t, int64(3), count)
}
