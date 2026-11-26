package executionlog

// AUD-057：task_runs/task_events 是 game 域模型，multiGame 模式下任务留痕
// 落在各 game 物理库——后台 Sweep 与手动 PurgeBefore 必须逐 scope 清理实际
// 持有数据的库，并把失败 scope 汇报进 RetentionSummary.FailedScopes。
// 修复前基线：taskLogTablesPresent()/sweepTaskLogs 只看 meta 库，game 库的
// 历史任务留痕永不清理（meta 缺表时周期清理还会整体短路）。

import (
	"context"
	"testing"
	"time"

	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/cuihairu/croupier/internal/db/dbctx"
	"github.com/cuihairu/croupier/internal/model"
)

// mapRouter 多物理库路由替身：按 "gameID/env" 返回对应库；badGame 解析失败。
type mapRouter struct {
	badGame string
	dbs     map[string]*gorm.DB
}

func (m *mapRouter) Resolve(ctx context.Context, gameID, env string) (context.Context, *gorm.DB, error) {
	if gameID == m.badGame {
		return ctx, nil, context.DeadlineExceeded
	}
	db := m.dbs[gameID+"/"+env]
	if db == nil {
		return ctx, nil, gorm.ErrInvalidDB
	}
	// 生产 router.Router 通过 ctx 注入 per-game 库（调用方只消费 ctx）
	return dbctx.WithDB(ctx, db), db, nil
}

func seedTaskLogFixture(t *testing.T, db *gorm.DB, taskID string, ageDays int) {
	t.Helper()
	at := time.Now().UTC().AddDate(0, 0, -ageDays)
	require.NoError(t, db.Create(&model.TaskRun{
		Model:  gorm.Model{CreatedAt: at, UpdatedAt: at},
		TaskID: taskID, FunctionID: "job." + taskID, GameID: "g1", Env: "prod", Status: "success",
	}).Error)
	require.NoError(t, db.Create(&model.TaskEvent{
		TaskID: taskID, Seq: 1, Type: "progress", CreatedAt: at,
	}).Error)
}

func countTaskLogs(t *testing.T, db *gorm.DB) (runs, events int64) {
	t.Helper()
	require.NoError(t, db.Model(&model.TaskRun{}).Count(&runs).Error)
	require.NoError(t, db.Model(&model.TaskEvent{}).Count(&events).Error)
	return runs, events
}

// 后台 Sweep：meta（历史残留兜底）+ 两个 game 物理库——历史任务与事件被清、
// 新记录保留、失败 scope 汇报。
func TestRetentionSweepTaskLogsMultiGamePhysicalDBs(t *testing.T) {
	meta := newTestDB(t)
	seedTaskLogFixture(t, meta, "t-meta-old", 30)

	g1 := newTestDB(t)
	seedTaskLogFixture(t, g1, "t-g1-old", 30)
	seedTaskLogFixture(t, g1, "t-g1-new", 0) // 新记录保留

	g2 := newTestDB(t)
	seedTaskLogFixture(t, g2, "t-g2-old", 30)

	r := NewRetention(meta, RetentionConfig{
		ExecutionLogDays: 0,
		TaskLogDays:      7,
		Router: &mapRouter{
			badGame: "gbad",
			dbs:     map[string]*gorm.DB{"g1/prod": g1, "g2/prod": g2},
		},
		GameScopes: func(ctx context.Context) ([]GameScopeRef, error) {
			return []GameScopeRef{
				{GameID: "g1", Env: "prod"},
				{GameID: "g2", Env: "prod"},
				{GameID: "gbad", Env: "prod"}, // resolve 失败 → 失败范围
			}, nil
		},
	})
	summary := r.Sweep(context.Background())

	// meta 1 + g1 1 + g2 1（gbad 解析失败跳过并汇报）
	assert.Equal(t, int64(3), summary.TaskRunsDeleted)
	assert.Equal(t, int64(3), summary.TaskEventsDeleted)
	assert.Equal(t, []string{"gbad/prod"}, summary.FailedScopes)

	// g1 只剩新记录；g2 与 meta 清空
	runs, events := countTaskLogs(t, g1)
	assert.Equal(t, int64(1), runs)
	assert.Equal(t, int64(1), events)
	runs, events = countTaskLogs(t, g2)
	assert.Equal(t, int64(0), runs)
	assert.Equal(t, int64(0), events)
	runs, events = countTaskLogs(t, meta)
	assert.Equal(t, int64(0), runs)
	assert.Equal(t, int64(0), events)
}

// 手动 PurgeBefore：multiGame 的 meta（croupier_meta 形态）不建任务表——
// 缺表不再让清理整体短路，逐 game 库清理；半迁移库（只有 task_runs）的
// event 清理失败记入失败范围，run 仍清掉。
func TestRetentionPurgeBeforeTaskScopesMultiGameMetaWithoutTaskTables(t *testing.T) {
	bare, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, bare.AutoMigrate(&model.ExecutionLog{})) // meta 无任务表

	g1 := newTestDB(t)
	seedTaskLogFixture(t, g1, "t-g1-old", 30)
	seedTaskLogFixture(t, g1, "t-g1-new", 0)

	g2 := newTestDB(t)
	seedTaskLogFixture(t, g2, "t-g2-old", 30)

	// g3 半迁移库：只有 task_runs——run 可删，event 报错 → 失败范围
	g3, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, g3.AutoMigrate(&model.TaskRun{}))
	at := time.Now().UTC().AddDate(0, 0, -30)
	require.NoError(t, g3.Create(&model.TaskRun{
		Model:  gorm.Model{CreatedAt: at, UpdatedAt: at},
		TaskID: "t-g3-old", FunctionID: "job.g3", GameID: "g3", Env: "prod", Status: "success",
	}).Error)

	r := NewRetention(bare, RetentionConfig{
		Router: &mapRouter{
			dbs: map[string]*gorm.DB{"g1/prod": g1, "g2/prod": g2, "g3/prod": g3},
		},
		GameScopes: func(ctx context.Context) ([]GameScopeRef, error) {
			return []GameScopeRef{
				{GameID: "g1", Env: "prod"},
				{GameID: "g2", Env: "prod"},
				{GameID: "g3", Env: "prod"},
			}, nil
		},
	})
	summary, err := r.PurgeBefore(context.Background(), time.Now().UTC().Add(-24*time.Hour), "task")
	require.NoError(t, err)

	// runs：g1 1 + g2 1 + g3 1；events：g1 1 + g2 1（g3 缺表 → 失败范围）
	assert.Equal(t, int64(3), summary.TaskRunsDeleted)
	assert.Equal(t, int64(2), summary.TaskEventsDeleted)
	assert.Equal(t, []string{"g3/prod"}, summary.FailedScopes)

	// g1 只剩新记录；g2 清空；g3 runs 清空
	runs, events := countTaskLogs(t, g1)
	assert.Equal(t, int64(1), runs)
	assert.Equal(t, int64(1), events)
	runs, events = countTaskLogs(t, g2)
	assert.Equal(t, int64(0), runs)
	assert.Equal(t, int64(0), events)
	// g3 半迁移库只有 task_runs，只查 runs
	var g3Runs int64
	require.NoError(t, g3.Model(&model.TaskRun{}).Count(&g3Runs).Error)
	assert.Equal(t, int64(0), g3Runs)
}

// 周期 Sweep 不因 meta 缺任务表而短路（修复前门卫只看 meta，game 库历史
// 任务留痕永不清理）。
func TestRetentionSweepTaskLogsMetaWithoutTaskTablesStillFansOut(t *testing.T) {
	bare, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, bare.AutoMigrate(&model.ExecutionLog{})) // meta 无任务表

	g1 := newTestDB(t)
	seedTaskLogFixture(t, g1, "t-g1-old", 30)

	r := NewRetention(bare, RetentionConfig{
		ExecutionLogDays: 0,
		TaskLogDays:      7,
		Router:           &mapRouter{dbs: map[string]*gorm.DB{"g1/prod": g1}},
		GameScopes: func(ctx context.Context) ([]GameScopeRef, error) {
			return []GameScopeRef{{GameID: "g1", Env: "prod"}}, nil
		},
	})
	summary := r.Sweep(context.Background())

	assert.Equal(t, int64(1), summary.TaskRunsDeleted)
	assert.Equal(t, int64(1), summary.TaskEventsDeleted)
	assert.Empty(t, summary.FailedScopes)
	runs, events := countTaskLogs(t, g1)
	assert.Equal(t, int64(0), runs)
	assert.Equal(t, int64(0), events)
}
