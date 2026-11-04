// 覆盖目标：Scheduler 定期备份调度——枚举注册启用的数据源触发真实备份、
// 禁用源跳过、无存储报错、在途防重、Loop tick 触发 RunOnce。
package backupexec

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/objstore"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// newSchedulerTestEnv 建文件元库 + 真实 sqlite 源 + 文件对象存储。
func newSchedulerTestEnv(t *testing.T) (*Scheduler, *model.BackupModel, *model.DBSourceModel, string) {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open(filepath.Join(t.TempDir(), "meta.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Backup{}, &model.DBSource{}))

	srcPath := filepath.Join(t.TempDir(), "src.db")
	srcDB, err := gorm.Open(gsqlite.Open(srcPath), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, srcDB.Exec("CREATE TABLE t (id integer)").Error)
	require.NoError(t, srcDB.Exec("INSERT INTO t VALUES (1)").Error)
	require.NoError(t, srcDB.Exec("INSERT INTO t VALUES (2)").Error)

	store, err := objstore.OpenFile(context.Background(), objstore.Config{BaseDir: t.TempDir()})
	require.NoError(t, err)

	backups := model.NewBackupModel(db)
	sources := model.NewDBSourceModel(db)
	return NewScheduler(backups, sources, store), backups, sources, srcPath
}

// insertSource 建数据源。注意：Enabled 列带 gorm default:true，Create 传
// false 会被当作零值省略（落库为 true）——禁用必须走 Update(map) 强制写。
func insertSource(t *testing.T, sources *model.DBSourceModel, name, driver, dsn, game, env string, enabled bool) uint {
	t.Helper()
	src := &model.DBSource{
		Name: name, Driver: driver, DSN: dsn, GameID: game, Env: env, Enabled: true,
	}
	require.NoError(t, sources.Create(context.Background(), src))
	if !enabled {
		require.NoError(t, sources.Update(context.Background(), src.ID, map[string]interface{}{"enabled": false}))
	}
	return src.ID
}

// TestScheduler_RunOnce_BacksUpEnabledSources 枚举启用源触发真实备份并
// succeeded；禁用源不产生备份记录。
func TestScheduler_RunOnce_BacksUpEnabledSources(t *testing.T) {
	ctx := context.Background()
	sched, backups, sources, srcPath := newSchedulerTestEnv(t)
	insertSource(t, sources, "g1-prod", "sqlite", srcPath, "game1", "prod", true)
	insertSource(t, sources, "g2-staging", "sqlite", srcPath, "game2", "staging", false)

	require.NoError(t, sched.RunOnce(ctx))

	require.Eventually(t, func() bool {
		items, _, lerr := backups.List(ctx, model.ListBackupsOptions{
			PaginationOptions: model.NewPagination(1, 100),
			GameID:            "game1",
			Env:               "prod",
		})
		return lerr == nil && len(items) > 0 && items[0].Status == "succeeded"
	}, 10*time.Second, 100*time.Millisecond)

	// 禁用源不产生备份记录
	items, _, err := backups.List(ctx, model.ListBackupsOptions{
		PaginationOptions: model.NewPagination(1, 100),
		GameID:            "game2",
		Env:               "staging",
	})
	require.NoError(t, err)
	assert.Empty(t, items, "disabled source must not be backed up")

	// 已成功备份记录带归属与源名
	got, _, err := backups.List(ctx, model.ListBackupsOptions{
		PaginationOptions: model.NewPagination(1, 100),
		GameID:            "game1",
		Env:               "prod",
	})
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "game1", got[0].GameID)
	assert.Equal(t, "prod", got[0].Env)
	assert.Equal(t, "g1-prod", got[0].SourceName)
	assert.NotEmpty(t, got[0].Location)
	assert.NotEmpty(t, got[0].Checksum)
}

// TestScheduler_RunOnce_NotInitialized 无源/无存储时报错。
func TestScheduler_RunOnce_NotInitialized(t *testing.T) {
	ctx := context.Background()
	err := (*Scheduler)(nil).RunOnce(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")

	s := NewScheduler(nil, nil, nil)
	err = s.RunOnce(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

// TestScheduler_RunOnce_NoStore 无对象存储报错（未装配）。
func TestScheduler_RunOnce_NoStore(t *testing.T) {
	ctx := context.Background()
	db, err := gorm.Open(gsqlite.Open(filepath.Join(t.TempDir(), "meta.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Backup{}, &model.DBSource{}))
	s := NewScheduler(model.NewBackupModel(db), model.NewDBSourceModel(db), nil)
	err = s.RunOnce(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "object store not configured")
}

// TestScheduler_InflightGuard begin/end 防重。
func TestScheduler_InflightGuard(t *testing.T) {
	s := NewScheduler(nil, nil, nil)
	require.True(t, s.begin("game1/prod"))
	require.False(t, s.begin("game1/prod"), "already in-flight must be rejected")
	s.end("game1/prod")
	require.True(t, s.begin("game1/prod"), "after end must be accepted again")
}

// TestScheduler_Loop_TickFires Loop 触发 RunOnce ≥1 次后 cancel 退出。
func TestScheduler_Loop_TickFires(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sched, backups, sources, srcPath := newSchedulerTestEnv(t)
	insertSource(t, sources, "g1-prod", "sqlite", srcPath, "game1", "prod", true)

	go sched.Loop(ctx, time.Millisecond)

	require.Eventually(t, func() bool {
		items, _, lerr := backups.List(context.Background(), model.ListBackupsOptions{
			PaginationOptions: model.NewPagination(1, 100),
			GameID:            "game1",
			Env:               "prod",
		})
		return lerr == nil && len(items) > 0
	}, 5*time.Second, 20*time.Millisecond)

	cancel()
}
