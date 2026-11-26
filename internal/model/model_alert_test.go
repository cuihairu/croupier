package model

import (
	"context"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"path/filepath"
	"testing"
	"time"
)

// Split from model_test.go; test bodies moved verbatim.

func TestNewAlertModel(t *testing.T) {
	db := setupAlertTestDB(t)
	model := NewAlertModel(db)
	assert.NotNil(t, model)
}

func TestAlertModel_Create(t *testing.T) {
	db := setupAlertTestDB(t)
	model := NewAlertModel(db)
	ctx := context.Background()

	alert := &Alert{
		AlertID: "test-alert-001",
		Type:    "system",
		Level:   "warning",
		Message: "Test alert message",
		Source:  "test",
		Status:  "active",
	}

	err := model.Create(ctx, alert)
	require.NoError(t, err)
	assert.NotZero(t, alert.ID)
}

func TestAlertModel_FindByAlertID(t *testing.T) {
	db := setupAlertTestDB(t)
	model := NewAlertModel(db)
	ctx := context.Background()

	// Create test alert
	alert := &Alert{
		AlertID: "test-find-alert",
		Type:    "system",
		Level:   "error",
		Message: "Find test alert",
		Source:  "test",
		Status:  "active",
	}
	err := model.Create(ctx, alert)
	require.NoError(t, err)

	// Test finding existing alert
	found, err := model.FindByAlertID(ctx, "test-find-alert")
	require.NoError(t, err)
	assert.Equal(t, alert.AlertID, found.AlertID)
	assert.Equal(t, alert.Message, found.Message)

	// Test finding non-existent alert
	_, err = model.FindByAlertID(ctx, "non-existent")
	assert.Error(t, err)

	// Test empty alert ID
	_, err = model.FindByAlertID(ctx, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "alert_id is required")
}

func TestAlertModel_UpdateStatus(t *testing.T) {
	db := setupAlertTestDB(t)
	model := NewAlertModel(db)
	ctx := context.Background()

	alert := &Alert{
		AlertID: "test-update-alert",
		Type:    "system",
		Level:   "warning",
		Message: "Update status test",
		Source:  "test",
		Status:  "active",
	}
	err := model.Create(ctx, alert)
	require.NoError(t, err)

	// Update status
	err = model.UpdateStatus(ctx, alert.ID, "resolved")
	require.NoError(t, err)

	// Verify update
	found, err := model.FindByAlertID(ctx, "test-update-alert")
	require.NoError(t, err)
	assert.Equal(t, "resolved", found.Status)
}

func TestAlertModel_List(t *testing.T) {
	db := setupAlertTestDB(t)
	model := NewAlertModel(db)
	ctx := context.Background()

	// Create test alerts with different levels
	alerts := []*Alert{
		{AlertID: "list-001", Type: "system", Level: "error", Message: "Error message", Source: "test", Status: "active"},
		{AlertID: "list-002", Type: "system", Level: "warning", Message: "Warning message", Source: "test", Status: "active"},
		{AlertID: "list-003", Type: "system", Level: "info", Message: "Info message", Source: "test", Status: "resolved"},
	}
	for _, a := range alerts {
		err := model.Create(ctx, a)
		require.NoError(t, err)
	}

	// Test list all
	items, total, err := model.List(ctx, ListAlertsOptions{})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, total, int64(3))
	assert.GreaterOrEqual(t, len(items), 3)

	// Test filter by level
	_, total, err = model.List(ctx, ListAlertsOptions{Level: "error"})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, total, int64(1))

	// Test filter by status
	_, total, err = model.List(ctx, ListAlertsOptions{Status: "resolved"})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, total, int64(1))
}

func TestAlertModel_ListScopeFilter(t *testing.T) {
	// 独立文件库：setupAlertTestDB 用 cache=shared，数据跨用例泄漏。
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "alert_scope_test.db")), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Alert{}, &AlertSilence{}))
	model := NewAlertModel(db)
	ctx := context.Background()

	// 4862：scope 过滤——非空 scope 命中当前 scope 告警 + 平台级（空 game_id/env）告警；
	// 空 scope 不过滤（全局视野）。
	alerts := []*Alert{
		{AlertID: "scope-a-1", Type: "system", Level: "error", Message: "game A", Source: "test", Status: "active", GameID: "game_a", Env: "prod"},
		{AlertID: "scope-b-1", Type: "system", Level: "error", Message: "game B", Source: "test", Status: "active", GameID: "game_b", Env: "prod"},
		{AlertID: "scope-p1", Type: "system", Level: "info", Message: "platform", Source: "test", Status: "active"}, // 平台级
	}
	for _, a := range alerts {
		require.NoError(t, model.Create(ctx, a))
	}

	// 无 scope（空）= 全局视野：命中全部
	items, total, err := model.List(ctx, ListAlertsOptions{})
	require.NoError(t, err)
	assert.Equal(t, int64(3), total)
	assert.Len(t, items, 3)

	// scope=game_a/prod：命中 game_a 告警 + 平台级告警，不含 game_b
	items, total, err = model.List(ctx, ListAlertsOptions{ScopeGameID: "game_a", ScopeEnv: "prod"})
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	for _, a := range items {
		assert.NotEqual(t, "scope-b-1", a.AlertID, "scope 过滤不应泄漏其他 game 告警")
	}

	// scope=game_c/stage：仅命中平台级告警
	items, total, err = model.List(ctx, ListAlertsOptions{ScopeGameID: "game_c", ScopeEnv: "stage"})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Equal(t, "scope-p1", items[0].AlertID)

	// scope 仅 gameID（env 空）：不过滤，全局视野（与 GameScopeFromContext 语义一致，
	// 实际调用方总是同时设置 game_id/env）
	items, total, err = model.List(ctx, ListAlertsOptions{ScopeGameID: "game_a"})
	require.NoError(t, err)
	assert.Equal(t, int64(3), total)

	// scope 与 ExcludeSources 正交：排除 source=test 后仅平台级（source=test 的 game_a 被排除）
	items, total, err = model.List(ctx, ListAlertsOptions{
		ScopeGameID:    "game_a",
		ScopeEnv:       "prod",
		ExcludeSources: []string{"test"},
	})
	require.NoError(t, err)
	assert.Equal(t, int64(0), total)
}

func TestAlertModel_CreateSilence(t *testing.T) {
	db := setupAlertTestDB(t)
	model := NewAlertModel(db)
	ctx := context.Background()

	silence := &AlertSilence{
		AlertID:        1,
		Reason:         "Testing silence",
		DurationMinute: 60,
		CreatedBy:      "test-user",
	}

	err := model.CreateSilence(ctx, silence)
	require.NoError(t, err)
	assert.NotZero(t, silence.ID)
	assert.False(t, silence.ExpiresAt.IsZero())
}

func TestAlertModel_ListSilences(t *testing.T) {
	db := setupAlertTestDB(t)
	model := NewAlertModel(db)
	ctx := context.Background()

	// Create test silences
	silence1 := &AlertSilence{
		AlertID:        1,
		Reason:         "Active silence",
		DurationMinute: 60,
		CreatedBy:      "test-user",
	}
	silence2 := &AlertSilence{
		AlertID:        2,
		Reason:         "Expired silence",
		DurationMinute: 0, // Don't auto-calculate
		ExpiresAt:      time.Now().UTC().Add(-1 * time.Hour),
		CreatedBy:      "test-user",
	}
	err := model.CreateSilence(ctx, silence1)
	require.NoError(t, err)
	err = model.CreateSilence(ctx, silence2)
	require.NoError(t, err)

	// Verify silence2 is actually expired
	var checkSilence AlertSilence
	db.First(&checkSilence, silence2.ID)
	assert.True(t, checkSilence.ExpiresAt.Before(NowUTC()))

	// Test list all
	silences, err := model.ListSilences(ctx, ListSilencesOptions{})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(silences), 2)

	// Test active only - should only get silence1 (the non-expired one)
	active, err := model.ListSilences(ctx, ListSilencesOptions{ActiveOnly: true})
	require.NoError(t, err)
	// All active silences should have future expiration times
	for _, s := range active {
		assert.True(t, s.ExpiresAt.After(NowUTC().Add(-time.Minute)), "Active silence should not be expired")
	}
	// We should have at least silence1 in the active list
	foundActive := false
	for _, s := range active {
		if s.ID == silence1.ID {
			foundActive = true
			break
		}
	}
	assert.True(t, foundActive, "Active silence should be in the list")
}

func TestAlertModel_DeleteSilence(t *testing.T) {
	db := setupAlertTestDB(t)
	model := NewAlertModel(db)
	ctx := context.Background()

	silence := &AlertSilence{
		AlertID:        1,
		Reason:         "To be deleted",
		DurationMinute: 60,
		CreatedBy:      "test-user",
	}
	err := model.CreateSilence(ctx, silence)
	require.NoError(t, err)

	// Delete silence
	err = model.DeleteSilence(ctx, silence.ID)
	require.NoError(t, err)

	// Verify deletion
	var count int64
	db.Model(&AlertSilence{}).Where("id = ?", silence.ID).Count(&count)
	assert.Equal(t, int64(0), count)
}

func TestAlertModel_BootstrapAlerts(t *testing.T) {
	db := setupAlertTestDB(t)
	model := NewAlertModel(db)
	ctx := context.Background()

	alerts := []Alert{
		{AlertID: "boot-001", Type: "system", Level: "error", Message: "Bootstrap 1", Source: "test", Status: "active"},
		{AlertID: "boot-002", Type: "system", Level: "warning", Message: "Bootstrap 2", Source: "test", Status: "active"},
	}

	// First bootstrap should create
	err := model.BootstrapAlerts(ctx, alerts)
	require.NoError(t, err)

	var count int64
	db.Model(&Alert{}).Where("alert_id IN ?", []string{"boot-001", "boot-002"}).Count(&count)
	assert.Equal(t, int64(2), count)

	// Second bootstrap should skip existing
	err = model.BootstrapAlerts(ctx, alerts)
	require.NoError(t, err)

	db.Model(&Alert{}).Where("alert_id IN ?", []string{"boot-001", "boot-002"}).Count(&count)
	assert.Equal(t, int64(2), count) // No duplicates
}

func TestAlertModel_PruneExpiredSilences(t *testing.T) {
	db := setupAlertTestDB(t)
	model := NewAlertModel(db)
	ctx := context.Background()

	// Create expired silence (DurationMinute=0 so ExpiresAt is not overridden)
	expiredTime := time.Now().UTC().Add(-1 * time.Hour)
	expired := &AlertSilence{
		AlertID:        1,
		Reason:         "Expired",
		DurationMinute: 0, // Don't auto-calculate ExpiresAt
		ExpiresAt:      expiredTime,
		CreatedBy:      "test-user",
	}
	err := model.CreateSilence(ctx, expired)
	require.NoError(t, err)

	// Verify it was created with the expired time
	var createdAlertSilence AlertSilence
	db.First(&createdAlertSilence, expired.ID)
	assert.True(t, createdAlertSilence.ExpiresAt.Before(time.Now().UTC()))

	// Prune
	err = model.PruneExpiredSilences(ctx)
	require.NoError(t, err)

	// Verify the specific expired silence was pruned
	var count int64
	db.Model(&AlertSilence{}).Where("id = ?", expired.ID).Count(&count)
	assert.Equal(t, int64(0), count)
}

func TestAlertModel_FindByIDs(t *testing.T) {
	db := setupAlertTestDB(t)
	model := NewAlertModel(db)
	ctx := context.Background()

	// Create test alerts
	alert1 := &Alert{AlertID: "findbyid-001", Type: "system", Level: "error", Message: "Alert 1", Source: "test", Status: "active"}
	alert2 := &Alert{AlertID: "findbyid-002", Type: "system", Level: "warning", Message: "Alert 2", Source: "test", Status: "active"}
	err := model.Create(ctx, alert1)
	require.NoError(t, err)
	err = model.Create(ctx, alert2)
	require.NoError(t, err)

	// Find by IDs
	result, err := model.FindByIDs(ctx, []uint{alert1.ID, alert2.ID})
	require.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, alert1.AlertID, result[alert1.ID].AlertID)
	assert.Equal(t, alert2.AlertID, result[alert2.ID].AlertID)

	// Empty slice
	result, err = model.FindByIDs(ctx, []uint{})
	require.NoError(t, err)
	assert.Empty(t, result)
}

// ===== MessageModel Tests =====
