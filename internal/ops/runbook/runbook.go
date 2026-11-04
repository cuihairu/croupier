// Package runbook implements OPS-009 runbook automation: a parameterized
// ordered step sequence that references OPS-004 deploy capabilities
// (start/rollback) and OPS-006 batch capabilities (config push / restricted
// exec), executed one-click with per-step results and audit.
package runbook

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/ops/batchop"
)

// BatchRunner executes an inline batch step (OPS-006). The production adapter
// creates a draft batch then executes it; tests use a fake.
type BatchRunner interface {
	RunBatch(ctx context.Context, gameID, env string, req *batchop.CreateRequest) (*batchop.Result, error)
}

// DeployRunner executes a deploy step (OPS-004). The production adapter
// resolves the deployment + package and drives the deploy orchestrator; the
// engine only needs the outcome (error), not the deployment row.
type DeployRunner interface {
	StartDeployment(ctx context.Context, deploymentID string) error
	RollbackDeployment(ctx context.Context, deploymentID, rollbackPackageID string) error
}

// Store is the runbook persistence surface (model helpers; test-friendly).
type Store interface {
	Create(ctx context.Context, rb *model.Runbook) error
	Find(ctx context.Context, gameID, env, runbookID string) (*model.Runbook, error)
	List(ctx context.Context, gameID, env string, limit int) ([]model.Runbook, error)
	Delete(ctx context.Context, gameID, env, runbookID string) error
	CreateRun(ctx context.Context, run *model.RunbookRun) error
	FindRun(ctx context.Context, gameID, env, runID string) (*model.RunbookRun, error)
	ListRuns(ctx context.Context, gameID, env, runbookID string, limit int) ([]model.RunbookRun, error)
	UpdateRunStatus(ctx context.Context, id uint, status string, currentStep int, stepsResultJSON, errMsg string) error
}

// StepSpec is one runbook step. Batch steps (kind batch_exec/batch_push)
// carry labels/agentIds + exec/config payload; deploy steps (kind
// deploy_start/deploy_rollback) carry deploymentId (+ rollbackPackageId).
// String fields may contain {{param}} placeholders substituted at run time.
type StepSpec struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
	// ContinueOnFailure 步失败不中断后续步骤（最终状态 partial）。
	ContinueOnFailure bool `json:"continueOnFailure"`
	// Batch step fields (OPS-006).
	Labels   map[string]string      `json:"labels,omitempty"`
	AgentIDs []string               `json:"agentIds,omitempty"`
	Exec     *batchop.ExecPayload   `json:"exec,omitempty"`
	Config   *batchop.ConfigPayload `json:"config,omitempty"`
	DryRun   bool                   `json:"dryRun"`
	// Deploy step fields (OPS-004).
	DeploymentID      string `json:"deploymentId,omitempty"`
	RollbackPackageID string `json:"rollbackPackageId,omitempty"`
}

// DefineRequest creates a runbook.
type DefineRequest struct {
	Name        string     `json:"name"`
	Description string     `json:"description"`
	GameID      string     `json:"gameId"`
	Env         string     `json:"env"`
	Steps       []StepSpec `json:"steps"`
	CreatedBy   string     `json:"createdBy"`
}

// StepResult is the outcome of one step within a run.
type StepResult struct {
	Name       string `json:"name"`
	Kind       string `json:"kind"`
	Status     string `json:"status"` // success | failed
	Detail     string `json:"detail,omitempty"`
	StartedAt  string `json:"startedAt,omitempty"`
	FinishedAt string `json:"finishedAt,omitempty"`
}

// RunResult is the aggregate outcome of a run.
type RunResult struct {
	RunID       string       `json:"runId"`
	RunbookID   string       `json:"runbookId"`
	Status      string       `json:"status"`
	CurrentStep int          `json:"currentStep"`
	Steps       []StepResult `json:"steps"`
	Error       string       `json:"error,omitempty"`
}

// Orchestrator runs runbooks.
type Orchestrator struct {
	store   Store
	batch   BatchRunner
	deploy  DeployRunner
	auditFn func(ctx context.Context, gameID, env, event, ref, status string, detail map[string]any)
}

// New builds an orchestrator. batch/deploy may be nil (steps of that kind then
// fail honestly at run time); auditFn nil skips audit.
func New(store Store, batch BatchRunner, deploy DeployRunner, auditFn func(ctx context.Context, gameID, env, event, ref, status string, detail map[string]any)) *Orchestrator {
	return &Orchestrator{store: store, batch: batch, deploy: deploy, auditFn: auditFn}
}

// Define validates and persists a runbook.
func (o *Orchestrator) Define(ctx context.Context, req *DefineRequest) (*model.Runbook, error) {
	if o.store == nil {
		return nil, errors.New("runbook: store not initialized")
	}
	if req == nil {
		return nil, errors.New("runbook: define request required")
	}
	gameID := strings.TrimSpace(req.GameID)
	env := strings.TrimSpace(req.Env)
	if gameID == "" || env == "" {
		return nil, errors.New("runbook: gameId and env required")
	}
	if strings.TrimSpace(req.Name) == "" {
		return nil, errors.New("runbook: name required")
	}
	if len(req.Steps) == 0 {
		return nil, errors.New("runbook: at least one step required")
	}
	for i := range req.Steps {
		if err := validateStep(&req.Steps[i]); err != nil {
			return nil, fmt.Errorf("runbook: step %d (%s): %w", i, req.Steps[i].Name, err)
		}
	}
	stepsJSON, err := json.Marshal(req.Steps)
	if err != nil {
		return nil, err
	}
	rb := &model.Runbook{
		RunbookID:   newRunbookID(),
		Name:        strings.TrimSpace(req.Name),
		Description: strings.TrimSpace(req.Description),
		GameID:      gameID,
		Env:         env,
		StepsJSON:   string(stepsJSON),
		CreatedBy:   req.CreatedBy,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	if err := o.store.Create(ctx, rb); err != nil {
		return nil, err
	}
	if o.auditFn != nil {
		o.auditFn(ctx, gameID, env, string(audit.EventRunbookCreate), rb.RunbookID, "success", map[string]any{"name": rb.Name, "steps": len(req.Steps)})
	}
	return rb, nil
}

// Get returns a runbook by id.
func (o *Orchestrator) Get(ctx context.Context, gameID, env, runbookID string) (*model.Runbook, error) {
	if o.store == nil {
		return nil, errors.New("runbook: store not initialized")
	}
	return o.store.Find(ctx, gameID, env, runbookID)
}

// List returns recent runbooks in scope.
func (o *Orchestrator) List(ctx context.Context, gameID, env string, limit int) ([]model.Runbook, error) {
	if o.store == nil {
		return nil, errors.New("runbook: store not initialized")
	}
	return o.store.List(ctx, gameID, env, limit)
}

// Delete removes a runbook in scope.
func (o *Orchestrator) Delete(ctx context.Context, gameID, env, runbookID string) error {
	if o.store == nil {
		return errors.New("runbook: store not initialized")
	}
	if err := o.store.Delete(ctx, gameID, env, runbookID); err != nil {
		return err
	}
	if o.auditFn != nil {
		o.auditFn(ctx, gameID, env, string(audit.EventRunbookDelete), runbookID, "success", nil)
	}
	return nil
}

// GetRun returns one run.
func (o *Orchestrator) GetRun(ctx context.Context, gameID, env, runID string) (*model.RunbookRun, error) {
	if o.store == nil {
		return nil, errors.New("runbook: store not initialized")
	}
	return o.store.FindRun(ctx, gameID, env, runID)
}

// ListRuns returns recent runs of a runbook in scope.
func (o *Orchestrator) ListRuns(ctx context.Context, gameID, env, runbookID string, limit int) ([]model.RunbookRun, error) {
	if o.store == nil {
		return nil, errors.New("runbook: store not initialized")
	}
	return o.store.ListRuns(ctx, gameID, env, runbookID, limit)
}

// Run executes a runbook once (sequential steps, params substituted).
// Returns the aggregate result; a step-level failure surfaces in the result,
// not as a top-level error (so the caller can inspect per-step outcomes).
func (o *Orchestrator) Run(ctx context.Context, gameID, env, runbookID string, params map[string]string, operator string) (*RunResult, error) {
	if o.store == nil {
		return nil, errors.New("runbook: store not initialized")
	}
	rb, err := o.store.Find(ctx, gameID, env, runbookID)
	if err != nil {
		return nil, err
	}
	var steps []StepSpec
	if err := json.Unmarshal([]byte(rb.StepsJSON), &steps); err != nil {
		return nil, fmt.Errorf("runbook: decode steps: %w", err)
	}
	if len(steps) == 0 {
		return nil, errors.New("runbook: runbook has no steps")
	}

	paramsJSON, _ := json.Marshal(params)
	run := &model.RunbookRun{
		RunID:      newRunID(),
		RunbookID:  runbookID,
		GameID:     gameID,
		Env:        env,
		ParamsJSON: string(paramsJSON),
		Status:     model.RunbookRunStatusRunning,
		CreatedBy:  operator,
		StartedAt:  time.Now().UTC(),
	}
	if err := o.store.CreateRun(ctx, run); err != nil {
		return nil, err
	}

	result := &RunResult{RunID: run.RunID, RunbookID: runbookID, Status: model.RunbookRunStatusRunning, CurrentStep: 0}
	for i := range steps {
		step := &steps[i]
		result.CurrentStep = i + 1
		sr := StepResult{Name: step.Name, Kind: step.Kind, StartedAt: time.Now().UTC().Format(time.RFC3339)}
		if err := substituteStep(step, params); err != nil {
			sr.Status = "failed"
			sr.Detail = err.Error()
			result.Status = model.RunbookRunStatusFailed
			result.Error = err.Error()
			result.Steps = append(result.Steps, sr)
			break
		}
		if err := o.executeStep(ctx, gameID, env, step); err != nil {
			sr.Status = "failed"
			sr.Detail = err.Error()
			result.Steps = append(result.Steps, sr)
			if step.ContinueOnFailure {
				result.Status = model.RunbookRunStatusPartial
				continue
			}
			result.Status = model.RunbookRunStatusFailed
			result.Error = err.Error()
			break
		}
		sr.Status = "success"
		sr.FinishedAt = time.Now().UTC().Format(time.RFC3339)
		result.Steps = append(result.Steps, sr)
	}
	if result.Status == model.RunbookRunStatusRunning {
		result.Status = model.RunbookRunStatusSucceeded
	}
	stepsJSON, _ := json.Marshal(result.Steps)
	if err := o.store.UpdateRunStatus(ctx, run.ID, result.Status, result.CurrentStep, string(stepsJSON), result.Error); err != nil {
		return result, err
	}
	if o.auditFn != nil {
		o.auditFn(ctx, gameID, env, string(audit.EventRunbookRun), runbookID, result.Status, map[string]any{
			"runId": result.RunID, "steps": len(result.Steps), "currentStep": result.CurrentStep,
		})
	}
	return result, nil
}

func (o *Orchestrator) executeStep(ctx context.Context, gameID, env string, step *StepSpec) error {
	switch step.Kind {
	case model.RunbookStepBatchExec, model.RunbookStepBatchPush:
		if o.batch == nil {
			return errors.New("runbook: batch runner unavailable")
		}
		req := &batchop.CreateRequest{
			GameID:   gameID,
			Env:      env,
			Labels:   step.Labels,
			AgentIDs: step.AgentIDs,
			DryRun:   step.DryRun,
		}
		if step.Kind == model.RunbookStepBatchExec {
			req.Kind = model.BatchOpKindExec
			req.Exec = step.Exec
		} else {
			req.Kind = model.BatchOpKindConfig
			req.Config = step.Config
		}
		res, err := o.batch.RunBatch(ctx, gameID, env, req)
		if err != nil {
			return err
		}
		if res.Status == model.BatchOpStatusFailed {
			return fmt.Errorf("runbook: batch %s failed (%d failed, %d succeeded)", res.BatchID, res.Failed, res.Succeeded)
		}
		return nil
	case model.RunbookStepDeployStart:
		if o.deploy == nil {
			return errors.New("runbook: deploy runner unavailable")
		}
		return o.deploy.StartDeployment(ctx, step.DeploymentID)
	case model.RunbookStepDeployRollback:
		if o.deploy == nil {
			return errors.New("runbook: deploy runner unavailable")
		}
		return o.deploy.RollbackDeployment(ctx, step.DeploymentID, step.RollbackPackageID)
	default:
		return fmt.Errorf("runbook: unknown step kind %q", step.Kind)
	}
}

func validateStep(step *StepSpec) error {
	if strings.TrimSpace(step.Kind) == "" {
		return errors.New("kind required")
	}
	if step.Name == "" {
		step.Name = step.Kind
	}
	switch step.Kind {
	case model.RunbookStepBatchExec:
		if step.Exec == nil || strings.TrimSpace(step.Exec.Command) == "" {
			return errors.New("batch_exec requires exec.command")
		}
		if len(step.Labels) == 0 && len(step.AgentIDs) == 0 {
			return errors.New("batch_exec requires labels or agentIds")
		}
	case model.RunbookStepBatchPush:
		if step.Config == nil || strings.TrimSpace(step.Config.Path) == "" || step.Config.ContentBase64 == "" {
			return errors.New("batch_push requires config.path and config.contentBase64")
		}
		if len(step.Labels) == 0 && len(step.AgentIDs) == 0 {
			return errors.New("batch_push requires labels or agentIds")
		}
	case model.RunbookStepDeployStart:
		if strings.TrimSpace(step.DeploymentID) == "" {
			return errors.New("deploy_start requires deploymentId")
		}
	case model.RunbookStepDeployRollback:
		if strings.TrimSpace(step.DeploymentID) == "" {
			return errors.New("deploy_rollback requires deploymentId")
		}
		if strings.TrimSpace(step.RollbackPackageID) == "" {
			return errors.New("deploy_rollback requires rollbackPackageId")
		}
	default:
		return fmt.Errorf("unknown step kind %q", step.Kind)
	}
	return nil
}

// substituteStep replaces {{param}} placeholders in string fields with the
// run params. It round-trips through JSON so every string leaf is covered
// (keys are not rewritten; only values).
func substituteStep(step *StepSpec, params map[string]string) error {
	if len(params) == 0 {
		return nil
	}
	raw, err := json.Marshal(step)
	if err != nil {
		return err
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return err
	}
	substituteValue(v, params)
	out, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return json.Unmarshal(out, step)
}

func substituteValue(v any, params map[string]string) {
	switch x := v.(type) {
	case map[string]any:
		for k, val := range x {
			x[k] = substituted(val, params)
		}
	case []any:
		for i := range x {
			x[i] = substituted(x[i], params)
		}
	}
}

func substituted(v any, params map[string]string) any {
	switch x := v.(type) {
	case string:
		out := x
		for k, val := range params {
			out = strings.ReplaceAll(out, "{{"+k+"}}", val)
		}
		return out
	case map[string]any:
		substituteValue(x, params)
		return x
	case []any:
		substituteValue(x, params)
		return x
	default:
		return v
	}
}
