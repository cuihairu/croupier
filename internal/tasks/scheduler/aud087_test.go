// AUD-087：终态回查按 schedule 所属 (gameID, env) 路由，查库失败不得静默吞掉。
//
// 缺陷形态：triggerSchedule 的回查不带作用域、由模型按 meta 连接执行
// （task_runs 是 game 模型，multiGame 下 meta 库无该表）→ 恒判「不存在」：
// 失败计数永不推进（dead_letter 阈值失效）、成功也永不清零；且回查 error
// 被丢弃，「查库失败」与「无记录」不可区分。
//
// 修复形态：回查携带 schedule 的 (GameID, Env)；error 分支记 warn、计数保持
// 原值（不猜），继续派发不阻塞调度循环。各 scope 的成功/失败只影响本
// schedule 的计数。
package scheduler

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/db/dbctx"
	"github.com/cuihairu/croupier/internal/model"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// 回查必须携带到期 schedule 自身的 (GameID, Env)：两个作用域不同的计划同轮
// 到期，各自的回查参数一一对应（不串、不缺）。
func TestManager_AUD087_LookupRoutesByScheduleScope(t *testing.T) {
	now := time.Date(2026, 11, 27, 10, 30, 0, 0, time.UTC)
	store := newFakeStore()
	s1 := dueSchedule(1, "30 10 * * *", "run-1", 0, 5, Slot(now))
	s2 := dueSchedule(2, "30 10 * * *", "run-2", 0, 5, Slot(now))
	s1.GameID, s1.Env = "alpha", "prod"
	s2.GameID, s2.Env = "beta", "stage"
	store.schedules = []model.TaskSchedule{s1, s2}

	var dispatches int32
	d := dispatcherFunc(func(ctx context.Context, req *sdkv1.InvokeRequest) (*sdkv1.StartTaskResponse, error) {
		dispatches++
		return &sdkv1.StartTaskResponse{TaskId: "run-new"}, nil
	})
	NewManager(store, d).tick(now)

	assert.Equal(t, int32(2), dispatches)
	store.mu.Lock()
	defer store.mu.Unlock()
	require.Len(t, store.runLookups, 2, "每条到期计划恰一次回查")
	assert.Equal(t, runLookup{gameID: "alpha", env: "prod", taskRunID: "run-1"}, store.runLookups[0])
	assert.Equal(t, runLookup{gameID: "beta", env: "stage", taskRunID: "run-2"}, store.runLookups[1])
}

// captureSlog 把默认 slog 换成缓冲 handler（测试期），断言调度器的错误面
// 上抛形态；t.Cleanup 恢复原 logger。
func captureSlog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(old) })
	return &buf
}

// 回查查库失败：与「记录不存在」「未终态」（同为中性、计数不动、不告警）
// 可区分——记 warn、计数保持原值、继续派发不阻塞循环。
func TestManager_AUD087_LookupErrorSurfacedAndNeutral(t *testing.T) {
	now := time.Date(2026, 11, 27, 10, 30, 0, 0, time.UTC)

	t.Run("query error logs warn, keeps count, still dispatches", func(t *testing.T) {
		logs := captureSlog(t)
		store := newFakeStore()
		store.failRunLookup = errors.New("no such table: task_runs")
		// 计数 1、阈值 2：查库失败若被计为失败会直接推进到 2 并影响下一轮
		// dead_letter；若被静默吞掉则无告警记录。两者都必须不发生。
		store.schedules = []model.TaskSchedule{dueSchedule(1, "30 10 * * *", "run-x", 1, 2, Slot(now))}

		var dispatches int32
		d := dispatcherFunc(func(ctx context.Context, req *sdkv1.InvokeRequest) (*sdkv1.StartTaskResponse, error) {
			dispatches++
			return &sdkv1.StartTaskResponse{TaskId: "run-new"}, nil
		})
		NewManager(store, d).tick(now)

		assert.Equal(t, int32(1), dispatches, "回查失败不得阻塞派发（调度循环不中断）")
		updates := store.updates[1]
		require.NotEmpty(t, updates)
		assert.Equal(t, 1, updates[len(updates)-1]["consecutive_failures"],
			"查库失败计数保持原值（不猜：不计失败也不清零）")
		assert.Equal(t, model.ScheduleStatusActive, store.schedules[0].Status)
		assert.Contains(t, logs.String(), "last run status lookup failed",
			"查库失败必须上抛（warn），不得静默吞掉")
		assert.Contains(t, logs.String(), "run-x")
	})

	t.Run("record not found is neutral and silent", func(t *testing.T) {
		logs := captureSlog(t)
		store := newFakeStore()
		// run-unknown 不在 fakeStore 已知清单 → ("", nil)：记录不存在。
		store.schedules = []model.TaskSchedule{dueSchedule(1, "30 10 * * *", "run-unknown", 1, 5, Slot(now))}

		var dispatches int32
		d := dispatcherFunc(func(ctx context.Context, req *sdkv1.InvokeRequest) (*sdkv1.StartTaskResponse, error) {
			dispatches++
			return &sdkv1.StartTaskResponse{TaskId: "run-new"}, nil
		})
		NewManager(store, d).tick(now)

		assert.Equal(t, int32(1), dispatches)
		updates := store.updates[1]
		require.NotEmpty(t, updates)
		assert.Equal(t, 1, updates[len(updates)-1]["consecutive_failures"],
			"记录不存在：计数保持原值")
		assert.NotContains(t, logs.String(), "last run status lookup failed",
			"记录不存在是正常形态，不得告警（与查库失败可区分）")
	})

	t.Run("non-terminal last run is neutral too", func(t *testing.T) {
		logs := captureSlog(t)
		store := newFakeStore()
		// run-pending → ("", nil)：未终态。
		store.schedules = []model.TaskSchedule{dueSchedule(1, "30 10 * * *", "run-pending", 1, 5, Slot(now))}
		NewManager(store, &fakeDispatcher{}).tick(now)

		updates := store.updates[1]
		require.NotEmpty(t, updates)
		assert.Equal(t, 1, updates[len(updates)-1]["consecutive_failures"],
			"未终态：计数保持原值")
		assert.NotContains(t, logs.String(), "last run status lookup failed")
	})
}

// aud087ScopeResolver 把 (gameID, env) 映射到各自 game 库（与 Router.Resolve
// 同型：dbctx.WithDB 注入覆盖）。
type aud087ScopeResolver struct {
	dbs map[string]*gorm.DB // key: gameID + "/" + env
}

func (r aud087ScopeResolver) ScopeContext(ctx context.Context, gameID, env string) (context.Context, error) {
	db, ok := r.dbs[gameID+"/"+env]
	if !ok {
		return nil, errors.New("no db for scope")
	}
	return dbctx.WithDB(ctx, db), nil
}

// multiGame 端到端：meta 库（调度定义，无 task_runs 表）+ 两个 game 库
// （task_runs）。同 task_id 在两个 game 库终态相反——各 schedule 只读自己
// 作用域的行：alpha 的 failed 推进计数、beta 的 succeeded 清零，互不影响。
// 缺陷形态（按 meta 连接查）下两者都会失效（meta 无 task_runs 表）。
func TestManager_AUD087_MultiGameRealStoreScopesIsolated(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Minute)
	newDB := func(name string) *gorm.DB {
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
	meta := newDB("aud087_sched_meta")
	require.NoError(t, meta.AutoMigrate(&model.TaskSchedule{}, &model.TaskScheduleRunLog{}))
	gameA := newDB("aud087_sched_game_a")
	gameB := newDB("aud087_sched_game_b")
	require.NoError(t, gameA.AutoMigrate(&model.TaskRun{}))
	require.NoError(t, gameB.AutoMigrate(&model.TaskRun{}))
	// multiGame 形态守卫：meta 库没有 task_runs 表（缺陷按 meta 查即报错）。
	require.False(t, meta.Migrator().HasTable(&model.TaskRun{}))

	store := model.NewTaskScheduleModel(meta)
	store.SetScopeResolver(aud087ScopeResolver{dbs: map[string]*gorm.DB{
		"alpha/prod": gameA, "beta/prod": gameB,
	}})
	ctx := context.Background()

	seed := func(name, gameID string) *model.TaskSchedule {
		s, err := store.Create(ctx, model.CreateScheduleInput{
			Name: name, CronExpr: "* * * * *", GameID: gameID, Env: "prod",
			FunctionID: "fn-1", FirstNext: &now,
		})
		require.NoError(t, err)
		require.NoError(t, store.UpdateSchedule(ctx, s.ID, map[string]interface{}{
			"last_run_id":          "run-shared",
			"consecutive_failures": 1,
			"max_failed_runs":      3,
		}))
		return s
	}
	schedA := seed("alpha-schedule", "alpha")
	schedB := seed("beta-schedule", "beta")

	// 同 task_id、相反终态：作用域隔离必须各自读到自己的行。
	require.NoError(t, gameA.Create(&model.TaskRun{TaskID: "run-shared", FunctionID: "fn-1", Status: "failed"}).Error)
	require.NoError(t, gameB.Create(&model.TaskRun{TaskID: "run-shared", FunctionID: "fn-1", Status: "succeeded"}).Error)

	var mu sync.Mutex
	d := dispatcherFunc(func(ctx context.Context, req *sdkv1.InvokeRequest) (*sdkv1.StartTaskResponse, error) {
		mu.Lock()
		defer mu.Unlock()
		return &sdkv1.StartTaskResponse{TaskId: "run-new"}, nil
	})
	NewManager(store, d).tick(now.Add(time.Second))

	gotA, err := store.FindByID(ctx, schedA.ID)
	require.NoError(t, err)
	gotB, err := store.FindByID(ctx, schedB.ID)
	require.NoError(t, err)
	assert.Equal(t, 2, gotA.ConsecutiveFailures,
		"alpha 调度读到本库 failed 终态：计数 1→2（缺陷形态恒停在 1）")
	assert.Equal(t, 0, gotB.ConsecutiveFailures,
		"beta 调度读到本库 succeeded 终态：计数清零（缺陷形态恒停在 1）")
	assert.Equal(t, model.ScheduleStatusActive, gotA.Status)
	assert.Equal(t, model.ScheduleStatusActive, gotB.Status)
}
