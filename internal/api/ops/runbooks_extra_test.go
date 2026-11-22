package ops

// runbooks.go 补测：runbookStore.Delete 与 ListRunning 适配层、七个 Service
// 编排器未装配早退（"runbook store unavailable"）、runbookBatchRunner 创建
// 失败透传、runbookDeployRunner.RollbackDeployment 错误透传、以及启动恢复
// RecoverStaleRunbookRuns 的 nil 守卫与真实收敛（stale running → failed）。

import (
	"context"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/ops/batchop"
	"github.com/cuihairu/croupier/internal/svc"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func runbookDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open(t.TempDir()+"/rb.db"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Runbook{}, &model.RunbookRun{}))
	return db
}

func TestRunbookStoreDeleteAndListRunning(t *testing.T) {
	ctx := context.Background()
	db := runbookDB(t)
	store := &runbookStore{
		runbooks: model.NewRunbookModel(db),
		runs:     model.NewRunbookRunModel(db),
	}

	require.NoError(t, store.Create(ctx, &model.Runbook{
		RunbookID: "rb1", Name: "restart", GameID: "g", Env: "e", StepsJSON: "[]",
	}))
	require.NoError(t, store.Delete(ctx, "g", "e", "rb1"))
	_, err := store.Find(ctx, "g", "e", "rb1")
	require.Error(t, err, "deleted runbook must be gone")

	// 三条运行：一条 48h 前仍 running（stale）、一条新鲜 running（保留）、一条已完成
	stale := time.Now().Add(-48 * time.Hour)
	require.NoError(t, store.CreateRun(ctx, &model.RunbookRun{
		RunID: "run-stale", RunbookID: "rb1", GameID: "g", Env: "e",
		Status: model.RunbookRunStatusRunning, StartedAt: stale,
	}))
	require.NoError(t, store.CreateRun(ctx, &model.RunbookRun{
		RunID: "run-fresh", RunbookID: "rb1", GameID: "g", Env: "e",
		Status: model.RunbookRunStatusRunning, StartedAt: time.Now().Add(-time.Minute),
	}))
	require.NoError(t, store.CreateRun(ctx, &model.RunbookRun{
		RunID: "run-done", RunbookID: "rb1", GameID: "g", Env: "e",
		Status: model.RunbookRunStatusSucceeded, StartedAt: stale,
	}))

	runs, err := store.ListRunning(ctx, time.Now().Add(-RunbookStaleRunGrace))
	require.NoError(t, err)
	require.Len(t, runs, 1, "only the stale running row qualifies")
	require.Equal(t, "run-stale", runs[0].RunID)
}

func TestRunbookServiceOrchestratorUnavailable(t *testing.T) {
	s := NewService(&svc.ServiceContext{})
	ctx := context.Background()

	_, err := s.RunbookList(ctx, "g", "e", 10)
	require.ErrorContains(t, err, "runbook store unavailable")
	_, err = s.RunbookGet(ctx, "g", "e", "rb1")
	require.ErrorContains(t, err, "runbook store unavailable")
	err = s.RunbookDelete(ctx, "g", "e", "rb1")
	require.ErrorContains(t, err, "runbook store unavailable")
	_, err = s.RunbookRun(ctx, "g", "e", "rb1", nil, "ops")
	require.ErrorContains(t, err, "runbook store unavailable")
	_, err = s.RunbookRunGet(ctx, "g", "e", "r1")
	require.ErrorContains(t, err, "runbook store unavailable")
	_, err = s.RunbookRunList(ctx, "g", "e", "rb1", 10)
	require.ErrorContains(t, err, "runbook store unavailable")
}

func TestRunbookBatchRunnerCreateError(t *testing.T) {
	// BatchOpCreate 在 DB/registry 未装配时早退 → RunBatch 透传
	r := &runbookBatchRunner{service: NewService(&svc.ServiceContext{})}
	_, err := r.RunBatch(context.Background(), "g", "e", &batchop.CreateRequest{GameID: "g", Env: "e"})
	require.Error(t, err)
}

func TestRunbookDeployRunnerRollbackError(t *testing.T) {
	r := &runbookDeployRunner{service: NewService(&svc.ServiceContext{})}
	require.Error(t, r.RollbackDeployment(context.Background(), "d1", "p1"))
}

func TestRunbookStoreListRunningError(t *testing.T) {
	ctx := context.Background()
	db := runbookDB(t)
	require.NoError(t, db.Migrator().DropTable("runbook_runs"))
	store := &runbookStore{runbooks: model.NewRunbookModel(db), runs: model.NewRunbookRunModel(db)}
	_, err := store.ListRunning(ctx, time.Now())
	require.Error(t, err)
}

func TestRunbookGetDeleteSuccess(t *testing.T) {
	ctx := context.Background()
	db := runbookDB(t)
	require.NoError(t, model.NewRunbookModel(db).Create(ctx, &model.Runbook{
		RunbookID: "rb1", Name: "restart", GameID: "g", Env: "e", StepsJSON: "[]",
	}))
	s := NewService(&svc.ServiceContext{DB: db})

	got, err := s.RunbookGet(ctx, "g", "e", "rb1")
	require.NoError(t, err)
	require.Equal(t, "rb1", got.RunbookID)

	require.NoError(t, s.RunbookDelete(ctx, "g", "e", "rb1"))
	_, err = s.RunbookGet(ctx, "g", "e", "rb1")
	require.Error(t, err)
}

func TestRecoverStaleRunbookRuns(t *testing.T) {
	ctx := context.Background()

	// nil 守卫
	_, err := RecoverStaleRunbookRuns(ctx, nil, time.Now())
	require.ErrorContains(t, err, "runbook store unavailable")
	_, err = RecoverStaleRunbookRuns(ctx, &svc.ServiceContext{}, time.Now())
	require.ErrorContains(t, err, "runbook store unavailable")

	// 真实收敛：stale running → failed
	db := runbookDB(t)
	require.NoError(t, model.NewRunbookRunModel(db).Create(ctx, &model.RunbookRun{
		RunID: "run-recover", RunbookID: "rb1", GameID: "g", Env: "e",
		Status: model.RunbookRunStatusRunning, CurrentStep: 2, StepsResultJSON: `[{"ok":true}]`,
		StartedAt: time.Now().Add(-48 * time.Hour),
	}))
	n, err := RecoverStaleRunbookRuns(ctx, &svc.ServiceContext{DB: db}, time.Now().Add(-RunbookStaleRunGrace))
	require.NoError(t, err)
	require.Equal(t, 1, n)

	got, err := model.NewRunbookRunModel(db).Find(ctx, "g", "e", "run-recover")
	require.NoError(t, err)
	require.Equal(t, model.RunbookRunStatusFailed, got.Status)
	require.Contains(t, got.Error, "interrupted by process restart")
}

// TestRecoverStaleRunbookRuns_WithinGraceSkippedThenReScanConverges 覆盖 AUD-025：
// 启动恢复只跑一次，宽限内的遗留 running 行被跳过——若不再重扫将永久停留 running。
// 本用例先以宽限界扫描（启动时点）断言宽限内的遗留行与活跃运行均不被误收；再以
// 已越过宽限的界（等效于时间推进、run 年龄超过宽限）重扫，断言遗留行被收敛为
// failed、活跃运行仍保留 running。
func TestRecoverStaleRunbookRuns_WithinGraceSkippedThenReScanConverges(t *testing.T) {
	ctx := context.Background()
	db := runbookDB(t)
	svcCtx := &svc.ServiceContext{DB: db}
	now := time.Now().UTC()

	// 遗留运行：启动时 2h 龄（宽限 24h 内），会被跳过。
	require.NoError(t, model.NewRunbookRunModel(db).Create(ctx, &model.RunbookRun{
		RunID: "run-within-grace", RunbookID: "rb1", GameID: "g", Env: "e",
		Status: model.RunbookRunStatusRunning, StartedAt: now.Add(-2 * time.Hour),
	}))
	// 另一实例的活跃运行：刚启动，同样在宽限内，绝不能被误收。
	require.NoError(t, model.NewRunbookRunModel(db).Create(ctx, &model.RunbookRun{
		RunID: "run-fresh-active", RunbookID: "rb1", GameID: "g", Env: "e",
		Status: model.RunbookRunStatusRunning, StartedAt: now.Add(-time.Minute),
	}))

	// 启动时点扫描（cutoff = now-24h）：两行均不满足 olderThan，均跳过。
	n, err := RecoverStaleRunbookRuns(ctx, svcCtx, now.Add(-RunbookStaleRunGrace))
	require.NoError(t, err)
	require.Equal(t, 0, n, "宽限内的遗留行不得在启动时点被误收")

	// 时间推进、run 年龄越过宽限后重扫（cutoff = now-1h 等效于遗留行已超 24h）：
	// 遗留行收敛，活跃运行保留。
	n, err = RecoverStaleRunbookRuns(ctx, svcCtx, now.Add(-time.Hour))
	require.NoError(t, err)
	require.Equal(t, 1, n, "越过宽限后必须能巡检收敛遗留行")

	stale, err := model.NewRunbookRunModel(db).Find(ctx, "g", "e", "run-within-grace")
	require.NoError(t, err)
	require.Equal(t, model.RunbookRunStatusFailed, stale.Status)

	active, err := model.NewRunbookRunModel(db).Find(ctx, "g", "e", "run-fresh-active")
	require.NoError(t, err)
	require.Equal(t, model.RunbookRunStatusRunning, active.Status, "活跃运行不得被误收")
}

// TestStartRunbookStaleRecoverySweep_ConvergesAndStops 覆盖 AUD-025 的周期重扫：
// 越过宽限的遗留 running 行在后台重扫周期内被收敛为 failed；取消 ctx 后重扫停止
// （无 panic、无泄漏）。
func TestStartRunbookStaleRecoverySweep_ConvergesAndStops(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	db := runbookDB(t)
	require.NoError(t, model.NewRunbookRunModel(db).Create(ctx, &model.RunbookRun{
		RunID: "run-stale-sweep", RunbookID: "rb1", GameID: "g", Env: "e",
		Status: model.RunbookRunStatusRunning, StartedAt: time.Now().Add(-48 * time.Hour),
	}))
	svcCtx := &svc.ServiceContext{DB: db}

	StartRunbookStaleRecoverySweep(ctx, svcCtx, 10*time.Millisecond)
	require.Eventually(t, func() bool {
		got, err := model.NewRunbookRunModel(db).Find(ctx, "g", "e", "run-stale-sweep")
		if err != nil {
			return false
		}
		return got.Status == model.RunbookRunStatusFailed
	}, 5*time.Second, 10*time.Millisecond, "周期重扫必须把越过宽限的遗留行收敛为 failed")

	// 取消后重扫退出；再等一轮 tick 窗口确认不再改动（此处仅验证取消不 panic）。
	cancel()
	time.Sleep(30 * time.Millisecond)
}

// TestStartRunbookStaleRecoverySweep_Guards 覆盖未装配 DB / interval<=0 / nil 入参
// 的早退守卫：不启动、不 panic。
func TestStartRunbookStaleRecoverySweep_Guards(t *testing.T) {
	var nilCtx context.Context
	StartRunbookStaleRecoverySweep(nilCtx, &svc.ServiceContext{DB: runbookDB(t)}, time.Hour)
	StartRunbookStaleRecoverySweep(context.Background(), nil, time.Hour)
	StartRunbookStaleRecoverySweep(context.Background(), &svc.ServiceContext{}, time.Hour)
	StartRunbookStaleRecoverySweep(context.Background(), &svc.ServiceContext{DB: runbookDB(t)}, 0)
	StartRunbookStaleRecoverySweep(context.Background(), &svc.ServiceContext{DB: runbookDB(t)}, -time.Minute)
}
