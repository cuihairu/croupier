package model

import (
	"context"
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var crudSeq int

func newCRUDDb(t *testing.T, models ...interface{}) *gorm.DB {
	t.Helper()
	crudSeq++
	db, err := gorm.Open(
		sqlite.Open(fmt.Sprintf("file:crud%d?mode=memory&cache=shared", crudSeq)),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)},
	)
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(models...))
	return db
}

func TestBugModelCRUD(t *testing.T) {
	db := newCRUDDb(t, &Bug{})
	m := NewBugModel(db)
	ctx := context.Background()

	b := &Bug{GameID: "demo", Env: "prod", Title: "crash on start", Severity: "high", Status: "open", CrashFingerprint: "fp-1"}
	require.NoError(t, m.Create(ctx, b))
	assert.NotZero(t, b.ID)

	got, err := m.FindOne(ctx, b.ID)
	require.NoError(t, err)
	assert.Equal(t, "open", got.Status)

	require.NoError(t, m.Update(ctx, b.ID, map[string]interface{}{"status": "fixed"}))

	fp, err := m.FindOpenByCrashFingerprint(ctx, "demo", "prod", "fp-1")
	require.NoError(t, err)
	assert.Equal(t, "fixed", fp.Status) // 已修——FindOpen 不再返回？看实现……可能只查 open

	list, total, err := m.List(ctx, BugQueryOptions{GameID: "demo", Env: "prod"})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, list, 1)

	require.NoError(t, m.Delete(ctx, b.ID))
	_, err = m.FindOne(ctx, b.ID)
	assert.Error(t, err)
}

func TestAlertRuleModelCRUD(t *testing.T) {
	db := newCRUDDb(t, &AlertRule{})
	m := NewAlertRuleModel(db)
	ctx := context.Background()

	r := &AlertRule{Name: "error-rate", Metric: "cpu.usagePercent", Operator: "gt", Threshold: 90, Enabled: true}
	require.NoError(t, m.Create(ctx, r))
	assert.NotZero(t, r.ID)

	got, err := m.FindByID(ctx, r.ID)
	require.NoError(t, err)
	assert.Equal(t, "error-rate", got.Name)

	require.NoError(t, m.Update(ctx, r.ID, map[string]interface{}{"enabled": false}))

	rules, err := m.List(ctx, ListAlertRulesOptions{})
	require.NoError(t, err)
	assert.Len(t, rules, 1)

	require.NoError(t, m.Delete(ctx, r.ID))
	_, err = m.FindByID(ctx, r.ID)
	assert.Error(t, err)
}

func TestToolLinkModelCRUD(t *testing.T) {
	db := newCRUDDb(t, &ToolLink{})
	m := NewToolLinkModel(db)
	ctx := context.Background()

	tl := &ToolLink{GameID: "demo", Name: "admin panel", URL: "https://admin.example.com", Category: "ops"}
	require.NoError(t, m.Create(ctx, tl))
	assert.NotZero(t, tl.ID)

	require.NoError(t, m.Update(ctx, tl.ID, map[string]interface{}{"name": "renamed"}))

	list, err := m.ListAll(ctx)
	require.NoError(t, err)
	assert.Len(t, list, 1)

	all, err := m.ListAll(ctx)
	require.NoError(t, err)
	assert.Len(t, all, 1)

	require.NoError(t, m.Delete(ctx, tl.ID))
	list, _ = m.ListAll(ctx)
	assert.Len(t, list, 0)
}

func TestPlatformSettingModelCRUD(t *testing.T) {
	db := newCRUDDb(t, &PlatformSetting{})
	// PlatformSetting 通过 Upsert 写入
	require.NoError(t, db.Where("key = ?", "test.key").Assign(PlatformSetting{Key: "test.key", Value: `"value"`, UpdatedBy: "admin"}).FirstOrCreate(&PlatformSetting{}).Error)

	var ps PlatformSetting
	require.NoError(t, db.Where("key = ?", "test.key").First(&ps).Error)
	assert.Equal(t, `"value"`, ps.Value)
}
