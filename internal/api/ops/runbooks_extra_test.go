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
