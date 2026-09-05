package executionlog

import (
	"context"
	"testing"
	"time"

	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/db/dbctx"
	"github.com/cuihairu/croupier/internal/model"
)

func seedRetentionFixtures(t *testing.T, db *gorm.DB) {
	t.Helper()
	now := time.Now().UTC()
	old := now.AddDate(0, 0, -30)
	recent := now.Add(-time.Hour)

	logs := []model.ExecutionLog{
		{GameID: "g1", Env: "prod", Source: SourceInvoke, FunctionID: "f.old", Actor: "alice", Status: StatusOK, CreatedAt: old},
		{GameID: "g1", Env: "prod", Source: SourceInvoke, FunctionID: "f.new", Actor: "alice", Status: StatusOK, CreatedAt: recent},
	}
	require.NoError(t, db.Create(&logs).Error)

	runs := []model.TaskRun{
		{Model: gorm.Model{CreatedAt: old, UpdatedAt: old}, TaskID: "t-old", FunctionID: "job.old", GameID: "g1", Env: "prod", Status: "success"},
		{Model: gorm.Model{CreatedAt: recent, UpdatedAt: recent}, TaskID: "t-new", FunctionID: "job.new", GameID: "g1", Env: "prod", Status: "success"},
	}
	require.NoError(t, db.Create(&runs).Error)

	events := []model.TaskEvent{
		// TaskEvent 自带外层 CreatedAt（遮蔽 gorm.Model 的），必须直接赋值
		{TaskID: "t-old", Seq: 1, Type: "progress", CreatedAt: old},
		{TaskID: "t-new", Seq: 1, Type: "progress", CreatedAt: recent},
	}
	require.NoError(t, db.Create(&events).Error)
}

func TestRetentionSweepDeletesOnlyExpired(t *testing.T) {
	db := newTestDB(t)
	seedRetentionFixtures(t, db)

	r := NewRetention(db, RetentionConfig{ExecutionLogDays: 7, TaskLogDays: 7})
	summary := r.Sweep(context.Background())
	t.Logf("summary: exec=%d runs=%d events=%d execCutoff=%v taskCutoff=%v",
		summary.ExecutionLogsDeleted, summary.TaskRunsDeleted, summary.TaskEventsDeleted,
		summary.ExecutionLogCutoff, summary.TaskLogCutoff)

	assert.Equal(t, int64(1), summary.ExecutionLogsDeleted)
	assert.Equal(t, int64(1), summary.TaskRunsDeleted)
	assert.Equal(t, int64(1), summary.TaskEventsDeleted)

	var logCount, runCount, eventCount int64
	require.NoError(t, db.Model(&model.ExecutionLog{}).Count(&logCount).Error)
	require.NoError(t, db.Model(&model.TaskRun{}).Count(&runCount).Error)
	require.NoError(t, db.Model(&model.TaskEvent{}).Count(&eventCount).Error)
	assert.Equal(t, int64(1), logCount)
	assert.Equal(t, int64(1), runCount)
	assert.Equal(t, int64(1), eventCount)

	// 剩下的是新记录
	var kept model.ExecutionLog
	require.NoError(t, db.First(&kept).Error)
	assert.Equal(t, "f.new", kept.FunctionID)
}

func TestRetentionZeroDaysKeepsEverything(t *testing.T) {
	db := newTestDB(t)
	seedRetentionFixtures(t, db)

	// 0=永久保留
	r := NewRetention(db, RetentionConfig{ExecutionLogDays: 0, TaskLogDays: 0})
	summary := r.Sweep(context.Background())

	assert.Equal(t, int64(0), summary.ExecutionLogsDeleted)
	assert.Equal(t, int64(0), summary.TaskRunsDeleted)
	assert.Equal(t, int64(0), summary.TaskEventsDeleted)

	var logCount, runCount int64
	require.NoError(t, db.Model(&model.ExecutionLog{}).Count(&logCount).Error)
	require.NoError(t, db.Model(&model.TaskRun{}).Count(&runCount).Error)
	assert.Equal(t, int64(2), logCount)
	assert.Equal(t, int64(2), runCount)
}

func TestConfigRetentionDefaults(t *testing.T) {
	// 未配置 → 默认 7 天
	var unset config.ExecutionLogConfig
	assert.Equal(t, 7, unset.EffectiveRetentionDays())
	assert.True(t, unset.IsEnabled())

	// 显式 0 → 永久
	zero := 0
	explicit := config.ExecutionLogConfig{RetentionDays: &zero}
	assert.Equal(t, 0, explicit.EffectiveRetentionDays())

	seven := 7
	set := config.TaskLogConfig{RetentionDays: &seven}
	assert.Equal(t, 7, set.EffectiveRetentionDays())
}

// sweepRouter 内存替身：badGame 解析失败（continue 分支），noTable 返回
// 缺表的库（DeleteBefore 失败 → 告警 continue 分支），其余返回指定库。
type sweepRouter struct {
	badGame string
	noTable string
	db      *gorm.DB
}

func (s *sweepRouter) Resolve(ctx context.Context, gameID, env string) (context.Context, *gorm.DB, error) {
	switch gameID {
	case s.badGame:
		return ctx, nil, context.DeadlineExceeded
	case s.noTable:
		bare, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
		if err != nil {
			return ctx, nil, err
		}
		return dbctx.WithDB(ctx, bare), bare, nil
	default:
		// 生产 router.Router 通过 ctx 注入 per-game 库（调用方只消费 ctx）
		return dbctx.WithDB(ctx, s.db), s.db, nil
	}
}

func TestRetentionSweepMultiGameScopes(t *testing.T) {
	db := newTestDB(t)
	seedRetentionFixtures(t, db) // meta 库一条过期记录

	// 独立 game 物理库：自带一条过期记录（multiGame 下 execution_logs
	// 分散在各 game 库，需逐 scope 清理）
	gameDB := newTestDB(t)
	require.NoError(t, gameDB.Create(&model.ExecutionLog{
		GameID: "g1", Env: "prod", Source: SourceInvoke,
		FunctionID: "f.game-old", Actor: "bob", Status: StatusOK,
		CreatedAt: time.Now().UTC().AddDate(0, 0, -30),
	}).Error)

	r := NewRetention(db, RetentionConfig{
		ExecutionLogDays: 7,
		Router:           &sweepRouter{badGame: "gbad", db: gameDB},
		GameScopes: func(ctx context.Context) ([]GameScopeRef, error) {
			return []GameScopeRef{
				{GameID: "g1", Env: "prod"},   // 正常清理 game 库
				{GameID: "gbad", Env: "prod"}, // resolve 失败 → continue
			}, nil
		},
	})
	summary := r.Sweep(context.Background())

	// meta 库 1 条 + g1 game 库 1 条（badGame 跳过）
	assert.Equal(t, int64(2), summary.ExecutionLogsDeleted)
	var gameCount int64
	require.NoError(t, gameDB.Model(&model.ExecutionLog{}).Count(&gameCount).Error)
	assert.Equal(t, int64(0), gameCount)
}

func TestRetentionSweepGameScopesError(t *testing.T) {
	db := newTestDB(t)
	seedRetentionFixtures(t, db)

	// GameScopes 本身失败：返回 meta 已删的计数与错误（不 panic、不重复清理）
	r := NewRetention(db, RetentionConfig{
		ExecutionLogDays: 7,
		Router:           &sweepRouter{db: db},
		GameScopes: func(ctx context.Context) ([]GameScopeRef, error) {
			return nil, context.DeadlineExceeded
		},
	})
	summary := r.Sweep(context.Background())
	assert.Equal(t, int64(1), summary.ExecutionLogsDeleted)
}

func TestRetentionSweepScopeDeleteFailureContinues(t *testing.T) {
	db := newTestDB(t)
	seedRetentionFixtures(t, db)

	// noTable scope 删除失败：告警 continue，不影响 meta 计数与其余 scope
	r := NewRetention(db, RetentionConfig{
		ExecutionLogDays: 7,
		Router:           &sweepRouter{noTable: "gbad", db: db},
		GameScopes: func(ctx context.Context) ([]GameScopeRef, error) {
			return []GameScopeRef{{GameID: "gbad", Env: "prod"}}, nil
		},
	})
	summary := r.Sweep(context.Background())
	assert.Equal(t, int64(1), summary.ExecutionLogsDeleted)
}

func TestRetentionSweepTaskEventFailure(t *testing.T) {
	// 只迁移 task_runs：task_events 清理失败 → 返回已删 run 数与错误
	bare, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, bare.AutoMigrate(&model.TaskRun{}))

	r := NewRetention(bare, RetentionConfig{TaskLogDays: 7})
	summary := r.Sweep(context.Background())
	assert.Equal(t, int64(0), summary.TaskRunsDeleted)
	assert.Equal(t, int64(0), summary.TaskEventsDeleted)
}

func TestRetentionRunPeriodicUntilCancel(t *testing.T) {
	db := newTestDBSingleConn(t)
	seedRetentionFixtures(t, db)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r := NewRetention(db, RetentionConfig{
		ExecutionLogDays: 7,
		TaskLogDays:      7,
		Interval:         10 * time.Millisecond,
	})
	r.Run(ctx)
	// 启动即清一次积压（覆盖 Run + sweepLogged 的删除>0 日志分支）
	assert.Eventually(t, func() bool {
		var logCount, runCount, eventCount int64
		db.Model(&model.ExecutionLog{}).Count(&logCount)
		db.Model(&model.TaskRun{}).Count(&runCount)
		db.Model(&model.TaskEvent{}).Count(&eventCount)
		return logCount == 1 && runCount == 1 && eventCount == 1
	}, 2*time.Second, 10*time.Millisecond)
	cancel()
}

func TestRetentionSweepLoggedNoDeletions(t *testing.T) {
	db := newTestDB(t)
	seedRetentionFixtures(t, db)
	r := NewRetention(db, RetentionConfig{ExecutionLogDays: 0, TaskLogDays: 0})
	// 0=永久保留 → 删除总数 0，走 sweepLogged 的静默分支
	assert.NotPanics(t, func() { r.sweepLogged(context.Background()) })
}
