// AUD-087：调度终态回读按 schedule 所属 (gameID, env) 路由到 per-game 库。
//
// multiGame 形态：meta 库只有调度定义（task_schedules/task_schedule_run_logs
// ——multiGame 部署手动建，svc/scheduler.go 种植注释同口径），task_runs 只在
// game 库（model.GameModels）。缺陷形态按模型自身（meta）连接查 → meta 无
// task_runs 表 → 恒判「查库失败/不存在」：失败计数永不推进（dead_letter 阈值
// 失效）、成功也永不清零。修复后三种结果可区分：记录不存在与未终态均
// ("", nil)（中性语义），作用域解析失败与查库错误返回 error。
package model

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/cuihairu/croupier/internal/db/dbctx"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// aud087Scopes 是内存版 ScheduleScopeResolver（与 cmd/server 的
// routerTaskScopes 同型）：每个作用域映射自己的 game 库，ScopeContext 经
// dbctx.WithDB 注入覆盖——与 Router.Resolve 一致。failOn 非空时对该作用域
// 返回错误（模拟 game_envs 绑定缺失 / 路由器不可用）。
type aud087Scopes struct {
	dbs    map[string]*gorm.DB // key: gameID + "/" + env
	failOn string
}

func (r *aud087Scopes) ScopeContext(ctx context.Context, gameID, env string) (context.Context, error) {
	if r.failOn == gameID+"/"+env {
		return nil, errors.New("scope down")
	}
	db, ok := r.dbs[gameID+"/"+env]
	if !ok {
		return nil, fmt.Errorf("no db for scope %s/%s", gameID, env)
	}
	return dbctx.WithDB(ctx, db), nil
}

// setupAud087MultiGame 建 multiGame 形态的三库：meta 只有调度定义两张表
// （绝无 task_runs——断言缺陷形态的依赖），每个 game 库只有 task_runs。
// 文件库（跨句柄可见性不受 ：memory: 单连接池限制，先例 tasks/store_scopes_test）。
func setupAud087MultiGame(t *testing.T) (*TaskScheduleModel, *gorm.DB, map[string]*gorm.DB) {
	t.Helper()
	newDB := func(name string) *gorm.DB {
		t.Helper()
		db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), name+".db")), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Silent),
		})
		require.NoError(t, err)
		t.Cleanup(func() {
			if sqlDB, err := db.DB(); err == nil {
				_ = sqlDB.Close()
			}
		})
		return db
	}
	meta := newDB("aud087_meta")
	require.NoError(t, meta.AutoMigrate(&TaskSchedule{}, &TaskScheduleRunLog{}))
	gameA := newDB("aud087_game_a")
	gameB := newDB("aud087_game_b")
	require.NoError(t, gameA.AutoMigrate(&TaskRun{}))
	require.NoError(t, gameB.AutoMigrate(&TaskRun{}))

	dbs := map[string]*gorm.DB{"alpha/prod": gameA, "beta/prod": gameB}
	m := NewTaskScheduleModel(meta)
	m.SetScopeResolver(&aud087Scopes{dbs: dbs})
	return m, meta, dbs
}

func aud087Run(taskID, status string) *TaskRun {
	return &TaskRun{TaskID: taskID, FunctionID: "fn-1", Status: status}
}

// 同一 task_id 在两个 game 库各有一行、终态相反——回查答案必须各自跟随
// schedule 的作用域（作用域隔离的直接证据），且 meta 库全程无 task_runs 表。
func TestTaskScheduleModel_AUD087_LastRunStatusRoutesByScope(t *testing.T) {
	m, meta, dbs := setupAud087MultiGame(t)
	gameA, gameB := dbs["alpha/prod"], dbs["beta/prod"]
	ctx := context.Background()

	require.NoError(t, gameA.Create(aud087Run("run-shared", "failed")).Error)
	require.NoError(t, gameB.Create(aud087Run("run-shared", "succeeded")).Error)
	require.NoError(t, gameB.Create(aud087Run("run-live", "running")).Error)

	// meta 库保持 multiGame 形态：无 task_runs 表（回查必须离开 meta）。
	assert.False(t, meta.Migrator().HasTable(&TaskRun{}), "meta 库不应有 task_runs 表")

	// alpha 调度读到 alpha 库的 failed（不是 beta 的 succeeded，更不是 meta）。
	st, err := m.LastRunStatus(ctx, "alpha", "prod", "run-shared")
	require.NoError(t, err)
	assert.Equal(t, "failed", st, "必须读到 schedule 所属 game 库的终态")

	// beta 调度读同一 task_id 得到相反答案：作用域各自隔离。
	st, err = m.LastRunStatus(ctx, "beta", "prod", "run-shared")
	require.NoError(t, err)
	assert.Equal(t, "succeeded", st, "同 task_id 在另一作用域必须读到自己的行")

	// 未终态 → ("", nil)（中性，与失败/成功可区分）。
	st, err = m.LastRunStatus(ctx, "beta", "prod", "run-live")
	require.NoError(t, err)
	assert.Equal(t, "", st)

	// 记录不存在 → ("", nil)。
	st, err = m.LastRunStatus(ctx, "beta", "prod", "run-absent")
	require.NoError(t, err)
	assert.Equal(t, "", st)

	// 作用域解析失败 → error（与不存在/未终态可区分），且不影响其他作用域
	// 的后续回查（失败只影响本 schedule）。
	_, err = m.LastRunStatus(ctx, "gamma", "prod", "run-shared")
	require.Error(t, err, "解析不到绑定必须报错，不得静默回退 meta 恒判不存在")
	assert.Contains(t, err.Error(), "gamma")
	st, err = m.LastRunStatus(ctx, "alpha", "prod", "run-shared")
	require.NoError(t, err)
	assert.Equal(t, "failed", st, "他作用域解析失败后本作用域回查不受影响")

	// 真实查库失败 → error（非 ("", nil)）：关掉 gameB 连接池后回查报错，
	// 与「记录不存在」可区分。
	sqlB, err := gameB.DB()
	require.NoError(t, err)
	require.NoError(t, sqlB.Close())
	_, err = m.LastRunStatus(ctx, "beta", "prod", "run-live")
	require.Error(t, err, "game 库连接已关：查库失败必须以 error 返回")
}

// 单库模式（默认 dev/CI，多表同库）行为不变：无解析器时按模型自身连接查，
// 各分支照旧；解析器已注入但作用域为空（legacy 行）同样回退自身连接。
func TestTaskScheduleModel_AUD087_SingleDBUnchanged(t *testing.T) {
	db := setupTaskSchedDB(t) // TaskSchedule + TaskScheduleRunLog + TaskRun 同库
	m := NewTaskScheduleModel(db)
	ctx := context.Background()

	require.NoError(t, db.Create(aud087Run("run-ok", "succeeded")).Error)
	require.NoError(t, db.Create(aud087Run("run-bad", "failed")).Error)
	require.NoError(t, db.Create(aud087Run("run-mid", "running")).Error)

	st, err := m.LastRunStatus(ctx, "demo", "prod", "run-ok")
	require.NoError(t, err)
	assert.Equal(t, "succeeded", st)
	st, err = m.LastRunStatus(ctx, "demo", "prod", "run-bad")
	require.NoError(t, err)
	assert.Equal(t, "failed", st)
	st, err = m.LastRunStatus(ctx, "demo", "prod", "run-mid")
	require.NoError(t, err)
	assert.Equal(t, "", st, "未终态 → 空")
	st, err = m.LastRunStatus(ctx, "demo", "prod", "run-absent")
	require.NoError(t, err)
	assert.Equal(t, "", st, "不存在 → 空")
	st, err = m.LastRunStatus(ctx, "demo", "prod", "")
	require.NoError(t, err)
	assert.Equal(t, "", st, "空 taskRunID 直接空，不触库")

	// 解析器已注入 + 空 gameID（legacy 行）：回退模型自身连接，同一库照常可读。
	m2 := NewTaskScheduleModel(db)
	m2.SetScopeResolver(&aud087Scopes{}) // 空表：任何非空作用域都会报错
	st, err = m2.LastRunStatus(ctx, "", "", "run-ok")
	require.NoError(t, err)
	assert.Equal(t, "succeeded", st, "空作用域回退自身连接（单库形态）")
}
