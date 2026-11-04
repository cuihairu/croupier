// Package batchop implements OPS-006 batch config push / restricted command
// execution orchestration: select agents by scope/label, dry-run preview,
// execute per-agent via the ops tunnel, aggregate a result report.
package batchop

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/model"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
)

// AgentSelector resolves the agent set for a batch. The implementation reads
// the registry sessions (or cluster directory) and filters by scope/labels.
type AgentSelector interface {
	Select(ctx context.Context, gameID, env string, labels map[string]string, agentIDs []string) ([]string, error)
}

// AgentExecutor dispatches one operation to one agent via the ops tunnel.
type AgentExecutor interface {
	ExecuteCommand(ctx context.Context, agentID string, req *opsv1.ExecuteCommandRequest) (*opsv1.ExecuteCommandResponse, error)
	PushFile(ctx context.Context, agentID string, req *opsv1.PushFileRequest) (*opsv1.PushFileResponse, error)
}

// BatchStore persists batch ops.
type BatchStore interface {
	Create(ctx context.Context, op *model.BatchOp) error
	Find(ctx context.Context, gameID, env, batchID string) (*model.BatchOp, error)
	List(ctx context.Context, gameID, env string, limit int) ([]model.BatchOp, error)
	UpdateStatus(ctx context.Context, id uint, status string, fields map[string]any) error
	// CompareAndSwapStatus 仅当当前 status == from 时原子更新为 to（含回读行）。
	// 返回 (成功, 当前行)；ok=false 表示并发已把状态改走（状态闸）。
	CompareAndSwapStatus(ctx context.Context, id uint, from, to string) (bool, *model.BatchOp, error)
}

// Orchestrator runs a batch operation.
type Orchestrator struct {
	selector AgentSelector
	executor AgentExecutor
	store    BatchStore
	// auditFn records an audit event (nil = skip).
	auditFn func(ctx context.Context, gameID, env, event, ref, status string, detail map[string]any)
}

// New builds an orchestrator.
func New(sel AgentSelector, exec AgentExecutor, store BatchStore, auditFn func(ctx context.Context, gameID, env, event string, ref, status string, detail map[string]any)) *Orchestrator {
	return &Orchestrator{selector: sel, executor: exec, store: store, auditFn: auditFn}
}

// ConfigPayload 配置下发载荷。
type ConfigPayload struct {
	Path string `json:"path"`
	// ContentBase64 文件内容（base64）。dry-run 不落敏感明文。
	ContentBase64 string `json:"contentBase64"`
	Mode          int32  `json:"mode"`
	Force         bool   `json:"force"`
	Sha256        string `json:"sha256"`
}

// ExecPayload 受限命令载荷。
type ExecPayload struct {
	Command        string   `json:"command"`
	Args           []string `json:"args"`
	TimeoutSeconds int32    `json:"timeoutSeconds"`
	WorkingDir     string   `json:"workingDir"`
}

// CreateRequest 创建批次。
type CreateRequest struct {
	GameID    string            `json:"gameId"`
	Env       string            `json:"env"`
	Kind      string            `json:"kind"`
	Labels    map[string]string `json:"labels"`
	AgentIDs  []string          `json:"agentIds"`
	Config    *ConfigPayload    `json:"config"`
	Exec      *ExecPayload      `json:"exec"`
	DryRun    bool              `json:"dryRun"`
	CreatedBy string            `json:"createdBy"`
}

// TargetResult 单机结果。
type TargetResult struct {
	AgentID  string `json:"agentId"`
	Status   string `json:"status"` // success | failed | skipped | dry_run
	Message  string `json:"message,omitempty"`
	ExitCode int32  `json:"exitCode,omitempty"`
}

// Result 聚合报表。
type Result struct {
	BatchID   string         `json:"batchId"`
	Status    string         `json:"status"`
	Targets   int            `json:"targets"`
	Succeeded int            `json:"succeeded"`
	Failed    int            `json:"failed"`
	Skipped   int            `json:"skipped"`
	Items     []TargetResult `json:"items"`
}

// Create validates and persists a draft batch (does not execute).
func (o *Orchestrator) Create(ctx context.Context, req *CreateRequest) (*model.BatchOp, error) {
	if req == nil {
		return nil, errors.New("create request required")
	}
	gameID := strings.TrimSpace(req.GameID)
	env := strings.TrimSpace(req.Env)
	if gameID == "" || env == "" {
		return nil, errors.New("gameId and env required")
	}
	switch req.Kind {
	case model.BatchOpKindConfig:
		if req.Config == nil || strings.TrimSpace(req.Config.Path) == "" || req.Config.ContentBase64 == "" {
			return nil, errors.New("config payload requires path and contentBase64")
		}
	case model.BatchOpKindExec:
		if req.Exec == nil || strings.TrimSpace(req.Exec.Command) == "" {
			return nil, errors.New("exec payload requires command")
		}
	default:
		return nil, fmt.Errorf("unknown batch kind %q", req.Kind)
	}
	if len(req.Labels) == 0 && len(req.AgentIDs) == 0 {
		return nil, errors.New("labels or agentIds required")
	}

	labelsJSON, err := json.Marshal(req.Labels)
	if err != nil {
		return nil, err
	}
	agentsJSON, err := json.Marshal(req.AgentIDs)
	if err != nil {
		return nil, err
	}
	payload, err := payloadJSON(req)
	if err != nil {
		return nil, err
	}

	op := &model.BatchOp{
		BatchID:     newBatchID(),
		GameID:      gameID,
		Env:         env,
		Kind:        req.Kind,
		LabelsJSON:  string(labelsJSON),
		AgentsJSON:  string(agentsJSON),
		PayloadJSON: payload,
		DryRun:      req.DryRun,
		Status:      model.BatchOpStatusDraft,
		CreatedBy:   req.CreatedBy,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	if err := o.store.Create(ctx, op); err != nil {
		return nil, err
	}
	if o.auditFn != nil {
		o.auditFn(ctx, gameID, env, string(audit.EventBatchOpCreate), op.BatchID, "success", map[string]any{"kind": op.Kind, "dryRun": op.DryRun})
	}
	return op, nil
}

// Execute runs a created batch (non-dry-run) against the resolved target set.
// Dry-run batches only preview (Execute on a dry-run batch returns the
// preview without dispatching). Returns the aggregate result.
func (o *Orchestrator) Execute(ctx context.Context, gameID, env, batchID string, dryRun bool) (*Result, error) {
	op, err := o.store.Find(ctx, gameID, env, batchID)
	if err != nil {
		return nil, err
	}
	// 状态闸：仅 draft 可进入 running（已完成/运行中/失败不得重复派发）。
	ok, current, err := o.store.CompareAndSwapStatus(ctx, op.ID, model.BatchOpStatusDraft, model.BatchOpStatusRunning)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("batchop: execute rejected: batch %s already %s", batchID, current.Status)
	}
	op = current
	labels, agentIDs, err := decodeSelectors(op)
	if err != nil {
		_ = o.store.UpdateStatus(ctx, op.ID, model.BatchOpStatusFailed, map[string]any{"results_json": "{\"error\":\"decode selectors\"}"})
		return nil, err
	}
	targets, err := o.selector.Select(ctx, gameID, env, labels, agentIDs)
	if err != nil {
		_ = o.store.UpdateStatus(ctx, op.ID, model.BatchOpStatusFailed, map[string]any{"results_json": "{\"error\":\"select agents\"}"})
		return nil, err
	}
	sort.Strings(targets)

	op.Targets = len(targets)
	if err := o.store.UpdateStatus(ctx, op.ID, model.BatchOpStatusRunning, map[string]any{"targets": len(targets)}); err != nil {
		return nil, err
	}

	// Dry-run: never dispatch.
	if op.DryRun || dryRun {
		items := make([]TargetResult, len(targets))
		for i, a := range targets {
			items[i] = TargetResult{AgentID: a, Status: "dry_run", Message: "preview"}
		}
		res := &Result{BatchID: batchID, Status: model.BatchOpStatusDryRun, Targets: len(targets), Items: items}
		if err := o.store.UpdateStatus(ctx, op.ID, model.BatchOpStatusDryRun, map[string]any{"targets": len(targets)}); err != nil {
			return nil, err
		}
		if o.auditFn != nil {
			o.auditFn(ctx, gameID, env, string(audit.EventBatchOpDryRun), batchID, "success", map[string]any{"targets": len(targets)})
		}
		return res, nil
	}

	// Execute per target concurrently but with a small concurrency cap.
	res := &Result{BatchID: batchID, Status: model.BatchOpStatusCompleted, Targets: len(targets)}
	res.Items = make([]TargetResult, len(targets))
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, 8)
	for i, agentID := range targets {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, agentID string) {
			defer wg.Done()
			defer func() { <-sem }()
			tr, err := o.dispatch(ctx, op, agentID)
			mu.Lock()
			res.Items[i] = tr
			mu.Unlock()
			if err != nil {
				_ = err // 已由 aggregate 阶段按 status 统计
			}
		}(i, agentID)
	}
	wg.Wait()

	// Aggregate counters.
	for _, tr := range res.Items {
		switch tr.Status {
		case "success":
			res.Succeeded++
		case "failed":
			res.Failed++
		case "skipped":
			res.Skipped++
		}
	}
	resJSON, _ := json.Marshal(res.Items)
	status := model.BatchOpStatusCompleted
	if res.Failed > 0 {
		status = model.BatchOpStatusFailed
	}
	res.Status = status
	if err := o.store.UpdateStatus(ctx, op.ID, status, map[string]any{
		"succeeded": res.Succeeded, "failed": res.Failed, "skipped": res.Skipped,
		"targets": res.Targets, "results_json": string(resJSON),
	}); err != nil {
		return res, err
	}
	if o.auditFn != nil {
		o.auditFn(ctx, gameID, env, string(audit.EventBatchOpExecute), batchID, status, map[string]any{"succeeded": res.Succeeded, "failed": res.Failed})
	}
	return res, nil
}

// Get returns a batch by id.
func (o *Orchestrator) Get(ctx context.Context, gameID, env, batchID string) (*model.BatchOp, error) {
	return o.store.Find(ctx, gameID, env, batchID)
}

// List returns recent batches.
func (o *Orchestrator) List(ctx context.Context, gameID, env string, limit int) ([]model.BatchOp, error) {
	return o.store.List(ctx, gameID, env, limit)
}

func (o *Orchestrator) dispatch(ctx context.Context, op *model.BatchOp, agentID string) (TargetResult, error) {
	switch op.Kind {
	case model.BatchOpKindExec:
		payload, err := decodeExecPayload(op.PayloadJSON)
		if err != nil {
			return TargetResult{AgentID: agentID, Status: "failed", Message: err.Error()}, err
		}
		resp, err := o.executor.ExecuteCommand(ctx, agentID, &opsv1.ExecuteCommandRequest{
			Command:        payload.Command,
			Args:           payload.Args,
			TimeoutSeconds: payload.TimeoutSeconds,
			WorkingDir:     payload.WorkingDir,
		})
		if err != nil {
			return TargetResult{AgentID: agentID, Status: "failed", Message: err.Error()}, err
		}
		if resp.GetExitCode() != 0 {
			return TargetResult{AgentID: agentID, Status: "failed", Message: resp.GetStdOut(), ExitCode: resp.GetExitCode()}, errors.New("exit non-zero")
		}
		return TargetResult{AgentID: agentID, Status: "success", Message: resp.GetStdOut(), ExitCode: resp.GetExitCode()}, nil

	case model.BatchOpKindConfig:
		payload, err := decodeConfigPayload(op.PayloadJSON)
		if err != nil {
			return TargetResult{AgentID: agentID, Status: "failed", Message: err.Error()}, err
		}
		content, err := base64.StdEncoding.DecodeString(payload.ContentBase64)
		if err != nil {
			return TargetResult{AgentID: agentID, Status: "failed", Message: "bad base64"}, err
		}
		resp, err := o.executor.PushFile(ctx, agentID, &opsv1.PushFileRequest{
			Path:           payload.Path,
			Content:        content,
			Mode:           payload.Mode,
			Force:          payload.Force,
			ExpectedSha256: payload.Sha256,
		})
		if err != nil {
			return TargetResult{AgentID: agentID, Status: "failed", Message: err.Error()}, err
		}
		if !resp.GetSuccess() {
			return TargetResult{AgentID: agentID, Status: "failed", Message: resp.GetMessage()}, errors.New(resp.GetMessage())
		}
		return TargetResult{AgentID: agentID, Status: "success", Message: resp.GetMessage(), ExitCode: 0}, nil
	}
	return TargetResult{AgentID: agentID, Status: "failed", Message: "unknown kind"}, errors.New("unknown kind")
}

// ---- encoding helpers ----

func payloadJSON(req *CreateRequest) (string, error) {
	if req.Kind == model.BatchOpKindConfig {
		raw, err := json.Marshal(req.Config)
		return string(raw), err
	}
	raw, err := json.Marshal(req.Exec)
	return string(raw), err
}

func decodeConfigPayload(raw string) (*ConfigPayload, error) {
	var p ConfigPayload
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		return nil, err
	}
	return &p, nil
}

func decodeExecPayload(raw string) (*ExecPayload, error) {
	var p ExecPayload
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		return nil, err
	}
	return &p, nil
}

func decodeSelectors(op *model.BatchOp) (map[string]string, []string, error) {
	var labels map[string]string
	if err := json.Unmarshal([]byte(op.LabelsJSON), &labels); err != nil {
		return nil, nil, err
	}
	var agents []string
	if err := json.Unmarshal([]byte(op.AgentsJSON), &agents); err != nil {
		return nil, nil, err
	}
	return labels, agents, nil
}
