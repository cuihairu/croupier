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
	"log/slog"
	"strings"
	"sync"
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
	// ListRunning 返回 started_at 早于 olderThan 且仍为 running 的运行行（启动
	// 恢复用：只取足够旧的，避免误伤另一实例仍在执行的运行）。
	ListRunning(ctx context.Context, olderThan time.Time) ([]*model.RunbookRun, error)
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
	// recovery 是单一后台恢复循环（审核发现：每次终态持久化失败都创建无限
	// 重试的独立 goroutine，持续错误下会累积任务并周期性冲击数据库——改为
	// 一个共享有界循环，无论多少运行只占一个 goroutine）。
	recovery *recoveryLoop
}

// New builds an orchestrator. batch/deploy may be nil (steps of that kind then
// fail honestly at run time); auditFn nil skips audit.
func New(store Store, batch BatchRunner, deploy DeployRunner, auditFn func(ctx context.Context, gameID, env, event, ref, status string, detail map[string]any)) *Orchestrator {
	return &Orchestrator{store: store, batch: batch, deploy: deploy, auditFn: auditFn, recovery: newRecoveryLoop(store)}
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
			sr.FinishedAt = time.Now().UTC().Format(time.RFC3339)
			sr.Detail = err.Error()
			result.Status = model.RunbookRunStatusFailed
			result.Error = err.Error()
			result.Steps = append(result.Steps, sr)
			break
		}
		if err := o.executeStep(ctx, gameID, env, step); err != nil {
			sr.Status = "failed"
			sr.FinishedAt = time.Now().UTC().Format(time.RFC3339)
			sr.Detail = err.Error()
			result.Steps = append(result.Steps, sr)
			if step.ContinueOnFailure {
				result.Status = model.RunbookRunStatusPartial
				// 部分成功也把失败信息写入 RunResult.Error（错误列聚合面），
				// 逐步 FinishedAt 已设，面板可按 error 列聚合失败步。
				if result.Error == "" {
					result.Error = err.Error()
				} else {
					result.Error = result.Error + "; " + err.Error()
				}
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
	// 终态落库必须可靠：若复用已取消/出错的 ctx（客户端断开、executeStep
	// 取消），UpdateRunStatus 失败会把 runbook_runs 永久留在 running（重启/
	// 重试均不收敛）。用独立于请求 ctx 的有界上下文重试落库，避免 DB 随
	// ctx 中止。
	if err := o.persistTerminal(ctx, run.ID, result); err != nil {
		return result, fmt.Errorf("runbook: persist terminal state: %w", err)
	}
	if o.auditFn != nil {
		o.auditFn(ctx, gameID, env, string(audit.EventRunbookRun), runbookID, result.Status, map[string]any{
			"runId": result.RunID, "steps": len(result.Steps), "currentStep": result.CurrentStep,
		})
	}
	return result, nil
}

// persistTerminal 用独立于请求 ctx 的有界上下文写终态，失败重试最多
// persistTerminalAttempts 次（每次有界超时）。重试窗口内仍未写成的，交由
// 后台恢复循环（detached goroutine + 指数退避）继续写，避免数据库短暂故
// 障超过重试窗口时 runbook_runs 永久停留 running；运行结果照常返回，行在
// 后台收敛，不因终态写库失败丢掉已执行的结果。
func (o *Orchestrator) persistTerminal(parent context.Context, runID uint, result *RunResult) error {
	stepsJSON, _ := json.Marshal(result.Steps)
	var lastErr error
	for attempt := 0; attempt < persistTerminalAttempts; attempt++ {
		pctx, cancel := context.WithTimeout(context.WithoutCancel(parent), persistTerminalTimeout)
		err := o.store.UpdateRunStatus(pctx, runID, result.Status, result.CurrentStep, string(stepsJSON), result.Error)
		cancel()
		if err == nil {
			return nil
		}
		lastErr = err
	}
	slog.Warn("runbook: terminal state not persisted after retries; handing off to background recovery",
		"run_id", runID, "status", result.Status, "error", lastErr)
	o.recovery.enqueue(&pendingTerminal{
		runID:       runID,
		status:      result.Status,
		currentStep: result.CurrentStep,
		stepsJSON:   string(stepsJSON),
		errMsg:      result.Error,
	})
	return nil
}

// RecoverStaleRuns 是启动恢复（审核发现：终态只保存在进程内的后台重试任务中，
// 数据库恢复前进程重启会丢失结果且没有启动恢复机制，已返回成功的 run 仍会永久
// 停留 running）。对启动前已遗留的 running 行——其终态结果已随原进程丢失，
// 无法诚实重建——收敛为 failed 并记录原因，使行不再永久 running。调用方须传
// olderThan 界（通常 now-宽限窗口），避免 HA 下误标记另一实例仍在执行的运行。
// 返回本次收敛的行数。
func (o *Orchestrator) RecoverStaleRuns(ctx context.Context, olderThan time.Time) (int, error) {
	if o.store == nil {
		return 0, errors.New("runbook: store not initialized")
	}
	stale, err := o.store.ListRunning(ctx, olderThan)
	if err != nil {
		return 0, fmt.Errorf("runbook: list stale runs: %w", err)
	}
	recovered := 0
	for _, run := range stale {
		msg := "runbook: run interrupted by process restart before terminal state was persisted; converged to failed on startup recovery"
		if err := o.store.UpdateRunStatus(ctx, run.ID, model.RunbookRunStatusFailed, run.CurrentStep, run.StepsResultJSON, msg); err != nil {
			return recovered, fmt.Errorf("runbook: recover stale run %d: %w", run.ID, err)
		}
		slog.Info("runbook: stale run converged to failed", "run_id", run.RunID, "id", run.ID)
		recovered++
	}
	return recovered, nil
}

// pendingTerminal 是一次待后台收敛的终态写。
type pendingTerminal struct {
	runID       uint
	status      string
	currentStep int
	stepsJSON   string
	errMsg      string
}

// recoveryLoop 是共享的、有界的终态后台恢复循环：无论累积多少待收敛运行，
// 只占一个 goroutine，按批次顺序写库，指数退避封顶，无泄漏/风暴风险。
type recoveryLoop struct {
	store   Store
	mu      sync.Mutex
	pending map[uint]*pendingTerminal
	wake    chan struct{}
	once    sync.Once
}

func newRecoveryLoop(store Store) *recoveryLoop {
	return &recoveryLoop{
		store:   store,
		pending: map[uint]*pendingTerminal{},
		wake:    make(chan struct{}, 1),
	}
}

// enqueue 记录一次待收敛终态（同 runID 取最新，去重），并确保循环已启动。
func (r *recoveryLoop) enqueue(t *pendingTerminal) {
	r.mu.Lock()
	r.pending[t.runID] = t
	r.mu.Unlock()
	r.once.Do(func() { go r.loop() })
	select {
	case r.wake <- struct{}{}:
	default:
	}
}

func (r *recoveryLoop) loop() {
	backoff := persistTerminalBackoffMin
	for {
		r.mu.Lock()
		if len(r.pending) == 0 {
			r.mu.Unlock()
			<-r.wake
			backoff = persistTerminalBackoffMin
			continue
		}
		items := make([]*pendingTerminal, 0, len(r.pending))
		for _, t := range r.pending {
			items = append(items, t)
		}
		r.mu.Unlock()

		allOK := true
		for _, t := range items {
			pctx, cancel := context.WithTimeout(context.Background(), persistTerminalTimeout)
			err := r.store.UpdateRunStatus(pctx, t.runID, t.status, t.currentStep, t.stepsJSON, t.errMsg)
			cancel()
			if err != nil {
				allOK = false
				continue
			}
			r.mu.Lock()
			delete(r.pending, t.runID)
			r.mu.Unlock()
			slog.Info("runbook: terminal state recovered in background", "run_id", t.runID, "status", t.status)
		}

		r.mu.Lock()
		more := len(r.pending) > 0
		r.mu.Unlock()
		if !more {
			backoff = persistTerminalBackoffMin
			<-r.wake
			continue
		}
		if allOK {
			backoff = persistTerminalBackoffMin
		}
		time.Sleep(backoff)
		if backoff < persistTerminalBackoffMax {
			backoff *= 2
			if backoff > persistTerminalBackoffMax {
				backoff = persistTerminalBackoffMax
			}
		}
	}
}

const (
	// persistTerminalTimeout 单次终态写库超时。
	persistTerminalTimeout = 5 * time.Second
	// persistTerminalAttempts 终态写库重试次数（含首次）。
	persistTerminalAttempts = 3
	// persistTerminalBackoffMin 后台恢复循环首次重试前等待。
	persistTerminalBackoffMin = 1 * time.Second
	// persistTerminalBackoffMax 后台恢复循环退避封顶。
	persistTerminalBackoffMax = 30 * time.Second
)

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
