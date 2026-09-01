package model

import (
	"context"
	"testing"
	"time"

	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func newModelTestDB(t *testing.T, models ...interface{}) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(models...))
	return db
}

// ---- PlatformSettingModel（此前 0%） ----

func TestPlatformSettingModelMethods(t *testing.T) {
	db := newModelTestDB(t, &PlatformSetting{})
	m := NewPlatformSettingModel(db)
	ctx := context.Background()

	// Get 不存在 → ok=false 无错误
	_, ok, err := m.Get(ctx, "site.name")
	require.NoError(t, err)
	assert.False(t, ok)

	// Set 空值 → 错误
	require.Error(t, m.Set(ctx, "site.name", []byte("  "), "admin"))

	// Set → Get → List
	require.NoError(t, m.Set(ctx, "site.name", []byte(`"demo"`), "admin"))
	raw, ok, err := m.Get(ctx, "site.name")
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, `"demo"`, string(raw))

	// 再 Set 同 key → upsert 覆盖
	require.NoError(t, m.Set(ctx, "site.name", []byte(`"v2"`), "admin2"))
	raw, _, err = m.Get(ctx, "site.name")
	require.NoError(t, err)
	assert.Equal(t, `"v2"`, string(raw))

	require.NoError(t, m.Set(ctx, "features.dev", []byte(`true`), "admin"))
	all, err := m.List(ctx)
	require.NoError(t, err)
	assert.Len(t, all, 2)

	// Clear 存在/不存在
	require.NoError(t, m.Clear(ctx, "site.name"))
	require.ErrorIs(t, m.Clear(ctx, "site.name"), gorm.ErrRecordNotFound)
}

// ---- ToolLink 校验与列表（此前 0%） ----

func TestValidateToolLink(t *testing.T) {
	assert.ErrorContains(t, ValidateToolLink(&ToolLink{Name: "", URL: "https://x"}), "name")
	assert.ErrorContains(t, ValidateToolLink(&ToolLink{Name: "n", URL: "ftp://x"}), "http")
	assert.ErrorContains(t, ValidateToolLink(&ToolLink{Name: "n", URL: "https://x", Category: "bogus"}), "category")

	tool := &ToolLink{Name: "  jenkins  ", URL: "  https://ci.example.com  "}
	require.NoError(t, ValidateToolLink(tool))
	assert.Equal(t, "jenkins", tool.Name)
	assert.Equal(t, "other", tool.Category) // 默认类别
}

func TestToolLinkModelListScopes(t *testing.T) {
	db := newModelTestDB(t, &ToolLink{})
	m := NewToolLinkModel(db)
	ctx := context.Background()

	require.NoError(t, db.Create(&ToolLink{Name: "global-tool", URL: "https://g", Category: "ci", Enabled: true}).Error)
	require.NoError(t, db.Create(&ToolLink{Name: "scoped-tool", URL: "https://s", Category: "repo", GameID: "demo", Env: "prod", Enabled: true}).Error)
	// gorm default:true 会把零值 false 变 true——用显式 Update 置为禁用
	require.NoError(t, db.Create(&ToolLink{Name: "disabled-tool", URL: "https://d", Category: "ci"}).Error)
	require.NoError(t, db.Model(&ToolLink{}).Where("name = ?", "disabled-tool").Update("enabled", false).Error)

	// 带游戏范围：全局 + 本游戏
	items, err := m.List(ctx, ToolQueryOptions{GameID: "demo", Env: "prod"})
	require.NoError(t, err)
	assert.Len(t, items, 2)

	// 无范围：仅全局
	items, err = m.List(ctx, ToolQueryOptions{})
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "global-tool", items[0].Name)
}

// ---- FAQ Vote / SlugExists（此前 0%） ----

func TestFAQModelVoteAndSlugExists(t *testing.T) {
	db := newModelTestDB(t, &FAQ{})
	m := NewFAQModel(db)
	ctx := context.Background()

	faq := &FAQ{Question: "q", Answer: "a", Slug: "how-to"}
	require.NoError(t, db.Create(faq).Error)

	// 空 slug → false
	exists, err := m.SlugExists(ctx, "", 0)
	require.NoError(t, err)
	assert.False(t, exists)

	exists, err = m.SlugExists(ctx, "how-to", 0)
	require.NoError(t, err)
	assert.True(t, exists)

	// 排除自身
	exists, err = m.SlugExists(ctx, "how-to", faq.ID)
	require.NoError(t, err)
	assert.False(t, exists)

	require.NoError(t, m.Vote(ctx, faq.ID, true))
	require.NoError(t, m.Vote(ctx, faq.ID, true))
	require.NoError(t, m.Vote(ctx, faq.ID, false))
	var got FAQ
	require.NoError(t, db.First(&got, faq.ID).Error)
	assert.Equal(t, 2, got.HelpfulCount)
	assert.Equal(t, 1, got.UnhelpfulCount)

	// 不存在的 FAQ → ErrRecordNotFound
	require.ErrorIs(t, m.Vote(ctx, 9999, true), gorm.ErrRecordNotFound)
}

// ---- Hotpatch：FindOne/Create/BucketHit/NormalizeFramework（此前 0%） ----

func TestHotpatchModelAndBucketHit(t *testing.T) {
	db := newModelTestDB(t, &Hotpatch{})
	m := NewHotpatchModel(db)
	ctx := context.Background()

	// FindOne 不存在
	_, err := m.FindOne(ctx, 42)
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)

	hp := &Hotpatch{GameID: "demo", Env: "prod", Framework: "skynet", Status: "draft", RolloutPercent: 100, RolloutSeed: "s1"}
	require.NoError(t, m.Create(ctx, hp))
	got, err := m.FindOne(ctx, hp.ID)
	require.NoError(t, err)
	assert.Equal(t, "skynet", got.Framework)

	// Update 部分字段
	require.NoError(t, m.Update(ctx, hp.ID, map[string]interface{}{"status": "testing"}))

	// BucketHit 边界
	assert.True(t, (&Hotpatch{RolloutPercent: 100}).BucketHit("any"))
	assert.False(t, (&Hotpatch{RolloutPercent: 0, RolloutSeed: "s"}).BucketHit("any"))
	assert.True(t, (&Hotpatch{RolloutPercent: 99, RolloutSeed: "seed"}).BucketHit("n1")) // 哈希几乎必中

	// NormalizeHotpatchFramework
	n, ok := NormalizeHotpatchFramework("  SKYNET ")
	assert.True(t, ok)
	assert.Equal(t, "skynet", n)
	_, ok = NormalizeHotpatchFramework("nope")
	assert.False(t, ok)
}

// ---- GameRelease Create/Update（此前 0%） ----

func TestGameReleaseModelCreateUpdate(t *testing.T) {
	db := newModelTestDB(t, &GameRelease{})
	m := NewGameReleaseModel(db)
	ctx := context.Background()

	rel := &GameRelease{GameID: "demo", Env: "prod", Channel: "official", Platform: "android", Version: "1.0.0", Type: ReleaseTypeFull, Status: ReleaseStatusDraft}
	require.NoError(t, m.Create(ctx, rel))
	require.NotZero(t, rel.ID)

	require.NoError(t, m.Update(ctx, rel.ID, map[string]interface{}{"status": ReleaseStatusUploading}))
	got, err := m.FindOne(ctx, rel.ID)
	require.NoError(t, err)
	assert.Equal(t, ReleaseStatusUploading, got.Status)
}

// ---- TaskSchedule：LastRunStatus 分支 + ListRunLogs（此前 0%） ----

func TestTaskScheduleLastRunStatusAndRunLogs(t *testing.T) {
	db := newModelTestDB(t, &TaskRun{}, &TaskScheduleRunLog{})
	m := NewTaskScheduleModel(db)
	ctx := context.Background()

	// 空 ID → ""
	status, err := m.LastRunStatus(ctx, "")
	require.NoError(t, err)
	assert.Equal(t, "", status)

	// 不存在 → ""
	status, err = m.LastRunStatus(ctx, "nope")
	require.NoError(t, err)
	assert.Equal(t, "", status)

	// 非终态 → ""
	require.NoError(t, db.Create(&TaskRun{TaskID: "t-run", Status: "running"}).Error)
	status, err = m.LastRunStatus(ctx, "t-run")
	require.NoError(t, err)
	assert.Equal(t, "", status)

	// 终态 → 原样
	require.NoError(t, db.Model(&TaskRun{}).Where("task_id = ?", "t-run").Update("status", "succeeded").Error)
	status, err = m.LastRunStatus(ctx, "t-run")
	require.NoError(t, err)
	assert.Equal(t, "succeeded", status)

	// ListRunLogs 分页
	for i := 0; i < 3; i++ {
		require.NoError(t, db.Create(&TaskScheduleRunLog{ScheduleID: 7, Slot: time.Now(), Status: "dispatched"}).Error)
	}
	logs, total, err := m.ListRunLogs(ctx, 7, 1, 2)
	require.NoError(t, err)
	assert.Equal(t, int64(3), total)
	assert.Len(t, logs, 2)
	logs, total, err = m.ListRunLogs(ctx, 7, 0, 0) // 无分页 → 全量
	require.NoError(t, err)
	assert.Equal(t, int64(3), total)
	assert.Len(t, logs, 3)
}

// ---- 纯函数 / 访问器 ----

func TestUnmarshalTermDisplayText(t *testing.T) {
	assert.Nil(t, UnmarshalTermDisplayText(""))
	assert.Nil(t, UnmarshalTermDisplayText("not json"))
	m := UnmarshalTermDisplayText(`{"zh-CN":"玩家"}`)
	assert.Equal(t, "玩家", m["zh-CN"])
}

func TestBackupUpdateByBackupID(t *testing.T) {
	db := newModelTestDB(t, &Backup{})
	m := NewBackupModel(db)
	ctx := context.Background()

	b := &Backup{BackupID: "bk-1", Name: "n", Type: "full", Status: "running"}
	require.NoError(t, db.Create(b).Error)
	require.NoError(t, m.UpdateByBackupID(ctx, "bk-1", map[string]interface{}{"status": "completed"}))

	var got Backup
	require.NoError(t, db.Where("backup_id = ?", "bk-1").First(&got).Error)
	assert.Equal(t, "completed", got.Status)
}

func TestModelDBAccessors(t *testing.T) {
	db := newModelTestDB(t, &ConfigVersion{}, &Ticket{})
	assert.Same(t, db, NewConfigVersionModel(db).DB())
	assert.Same(t, db, NewTicketModel(db).DB())
}
