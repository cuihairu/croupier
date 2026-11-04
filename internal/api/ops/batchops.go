package ops

import (
	"context"
	"errors"

	"github.com/cuihairu/croupier/internal/audit"
	agentops "github.com/cuihairu/croupier/internal/logic/ops"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/ops/batchop"
	"github.com/cuihairu/croupier/internal/svc"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
)

// batchOpOrchestrator returns an OPS-006 orchestrator wired to the registry
// selector, the global agent-ops executor, the BatchOp store and the audit
// helper. Returns an error when the store is unavailable.
func batchOpOrchestrator(ctx context.Context, svcCtx *svc.ServiceContext) (*batchop.Orchestrator, error) {
	if svcCtx == nil || svcCtx.DB == nil || svcCtx.RegistryStore == nil {
		return nil, errors.New("batch op store or registry unavailable")
	}
	store := model.NewBatchOpModel(svcCtx.DB)
	selector := batchop.NewRegistrySelector(svcCtx.RegistryStore)
	executor := &agentOpsExecutor{}
	auditFn := func(ctx context.Context, gameID, env, event, ref, status string, detail map[string]any) {
		recordOpsAudit(ctx, svcCtx, auditEventTypeFromString(event), ref, status, detail)
	}
	return batchop.New(selector, executor, store, auditFn), nil
}

// agentOpsExecutor dispatches through the global agent-ops client.
type agentOpsExecutor struct{}

func (e *agentOpsExecutor) ExecuteCommand(ctx context.Context, agentID string, req *opsv1.ExecuteCommandRequest) (*opsv1.ExecuteCommandResponse, error) {
	client := agentops.GetAgentOpsClient()
	wrapper, err := client.GetClient(ctx, agentID)
	if err != nil {
		return nil, err
	}
	return wrapper.ExecuteCommand(ctx, req)
}

func (e *agentOpsExecutor) PushFile(ctx context.Context, agentID string, req *opsv1.PushFileRequest) (*opsv1.PushFileResponse, error) {
	client := agentops.GetAgentOpsClient()
	wrapper, err := client.GetClient(ctx, agentID)
	if err != nil {
		return nil, err
	}
	return wrapper.PushFile(ctx, req)
}

// BatchOpCreate creates a draft batch (dry-run default). The handler is
// wired in routes.go.
func (s *Service) BatchOpCreate(ctx context.Context, req *batchop.CreateRequest) (*model.BatchOp, error) {
	o, err := batchOpOrchestrator(ctx, s.svcCtx)
	if err != nil {
		return nil, err
	}
	return o.Create(ctx, req)
}

func (s *Service) BatchOpExecute(ctx context.Context, gameID, env, batchID string, dryRun bool) (*batchop.Result, error) {
	o, err := batchOpOrchestrator(ctx, s.svcCtx)
	if err != nil {
		return nil, err
	}
	if dryRun {
		return o.Execute(ctx, gameID, env, batchID, true)
	}
	// 非 dry-run：在请求 ctx 上执行会在客户端断开时取消整个批次（#664），
	// 改用后台 context 完成派发并落终态，避免批次停 Running 且终态不落库。
	bg, cancel := context.WithCancel(context.Background())
	defer cancel()
	return o.Execute(bg, gameID, env, batchID, false)
}

func (s *Service) BatchOpGet(ctx context.Context, gameID, env, batchID string) (*model.BatchOp, error) {
	o, err := batchOpOrchestrator(ctx, s.svcCtx)
	if err != nil {
		return nil, err
	}
	return o.Get(ctx, gameID, env, batchID)
}

func (s *Service) BatchOpList(ctx context.Context, gameID, env string, limit int) ([]model.BatchOp, error) {
	o, err := batchOpOrchestrator(ctx, s.svcCtx)
	if err != nil {
		return nil, err
	}
	return o.List(ctx, gameID, env, limit)
}

func auditEventTypeFromString(s string) audit.AuditEventType {
	return audit.AuditEventType(s)
}
