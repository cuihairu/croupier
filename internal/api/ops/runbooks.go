package ops

import (
	"context"
	"errors"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/ops/batchop"
	"github.com/cuihairu/croupier/internal/ops/runbook"
)

// runbookStore adapts the two model helpers into the engine Store interface.
type runbookStore struct {
	runbooks *model.RunbookModel
	runs     *model.RunbookRunModel
}

func (s *runbookStore) Create(ctx context.Context, rb *model.Runbook) error {
	return s.runbooks.Create(ctx, rb)
}
func (s *runbookStore) Find(ctx context.Context, gameID, env, runbookID string) (*model.Runbook, error) {
	return s.runbooks.Find(ctx, gameID, env, runbookID)
}
func (s *runbookStore) List(ctx context.Context, gameID, env string, limit int) ([]model.Runbook, error) {
	return s.runbooks.List(ctx, gameID, env, limit)
}
func (s *runbookStore) Delete(ctx context.Context, gameID, env, runbookID string) error {
	return s.runbooks.Delete(ctx, gameID, env, runbookID)
}
func (s *runbookStore) CreateRun(ctx context.Context, run *model.RunbookRun) error {
	return s.runs.Create(ctx, run)
}
func (s *runbookStore) FindRun(ctx context.Context, gameID, env, runID string) (*model.RunbookRun, error) {
	return s.runs.Find(ctx, gameID, env, runID)
}
func (s *runbookStore) ListRuns(ctx context.Context, gameID, env, runbookID string, limit int) ([]model.RunbookRun, error) {
	return s.runs.ListByRunbook(ctx, gameID, env, runbookID, limit)
}
func (s *runbookStore) UpdateRunStatus(ctx context.Context, id uint, status string, currentStep int, stepsResultJSON, errMsg string) error {
	return s.runs.UpdateStatus(ctx, id, status, currentStep, stepsResultJSON, errMsg)
}

// runbookBatchRunner executes an inline batch step by reusing the OPS-006
// Service surface (create draft + execute, non-dry-run semantics preserved).
type runbookBatchRunner struct{ service *Service }

func (r *runbookBatchRunner) RunBatch(ctx context.Context, gameID, env string, req *batchop.CreateRequest) (*batchop.Result, error) {
	op, err := r.service.BatchOpCreate(ctx, req)
	if err != nil {
		return nil, err
	}
	// 非 dry-run 在后台 ctx 上执行（客户端断开不取消整批，与 BatchOpExecute 一致）。
	bg, cancel := context.WithCancel(context.Background())
	defer cancel()
	return r.service.BatchOpExecute(bg, gameID, env, op.BatchID, req.DryRun)
}

// runbookDeployRunner executes a deploy step by reusing the OPS-004 Service
// surface (start/rollback resolution + orchestrator drive).
type runbookDeployRunner struct{ service *Service }

func (r *runbookDeployRunner) StartDeployment(ctx context.Context, deploymentID string) error {
	_, err := r.service.DeploymentStart(ctx, deploymentID)
	return err
}

func (r *runbookDeployRunner) RollbackDeployment(ctx context.Context, deploymentID, rollbackPackageID string) error {
	_, err := r.service.DeploymentRollback(ctx, deploymentID, &DeploymentRollbackRequest{RollbackPackageID: rollbackPackageID})
	return err
}

// runbookOrchestrator wires the OPS-009 engine to the meta DB store and the
// OPS-004/OPS-006 capability runners. Requires the meta DB (schema 0062).
func (s *Service) runbookOrchestrator() (*runbook.Orchestrator, error) {
	if s.svcCtx == nil || s.svcCtx.DB == nil {
		return nil, errors.New("runbook store unavailable")
	}
	store := &runbookStore{
		runbooks: model.NewRunbookModel(s.svcCtx.DB),
		runs:     model.NewRunbookRunModel(s.svcCtx.DB),
	}
	auditFn := func(ctx context.Context, gameID, env, event, ref, status string, detail map[string]any) {
		recordOpsAudit(ctx, s.svcCtx, auditEventTypeFromString(event), ref, status, detail)
	}
	return runbook.New(store, &runbookBatchRunner{service: s}, &runbookDeployRunner{service: s}, auditFn), nil
}

// RunbookCreate defines a runbook (persisted, not executed).
func (s *Service) RunbookCreate(ctx context.Context, req *runbook.DefineRequest) (*model.Runbook, error) {
	o, err := s.runbookOrchestrator()
	if err != nil {
		return nil, err
	}
	return o.Define(ctx, req)
}

func (s *Service) RunbookList(ctx context.Context, gameID, env string, limit int) ([]model.Runbook, error) {
	o, err := s.runbookOrchestrator()
	if err != nil {
		return nil, err
	}
	return o.List(ctx, gameID, env, limit)
}

func (s *Service) RunbookGet(ctx context.Context, gameID, env, runbookID string) (*model.Runbook, error) {
	o, err := s.runbookOrchestrator()
	if err != nil {
		return nil, err
	}
	return o.Get(ctx, gameID, env, runbookID)
}

func (s *Service) RunbookDelete(ctx context.Context, gameID, env, runbookID string) error {
	o, err := s.runbookOrchestrator()
	if err != nil {
		return err
	}
	return o.Delete(ctx, gameID, env, runbookID)
}

// RunbookRun executes a runbook once (sequential steps + params). Uses a
// background context so a client disconnect does not cancel the run mid-flight.
func (s *Service) RunbookRun(ctx context.Context, gameID, env, runbookID string, params map[string]string, operator string) (*runbook.RunResult, error) {
	o, err := s.runbookOrchestrator()
	if err != nil {
		return nil, err
	}
	bg, cancel := context.WithCancel(context.Background())
	defer cancel()
	return o.Run(bg, gameID, env, runbookID, params, operator)
}

func (s *Service) RunbookRunGet(ctx context.Context, gameID, env, runID string) (*model.RunbookRun, error) {
	o, err := s.runbookOrchestrator()
	if err != nil {
		return nil, err
	}
	return o.GetRun(ctx, gameID, env, runID)
}

func (s *Service) RunbookRunList(ctx context.Context, gameID, env, runbookID string, limit int) ([]model.RunbookRun, error) {
	o, err := s.runbookOrchestrator()
	if err != nil {
		return nil, err
	}
	return o.ListRuns(ctx, gameID, env, runbookID, limit)
}
