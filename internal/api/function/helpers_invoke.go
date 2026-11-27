package function

import (
	"context"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/model"
	functioninvoke "github.com/cuihairu/croupier/internal/platform/functioninvoke"
	"github.com/cuihairu/croupier/internal/policy"
	"github.com/cuihairu/croupier/internal/svc"
)

func functionInvoke(ctx context.Context, svcCtx *svc.ServiceContext, req *FunctionInvokeRequest) (*FunctionInvokeResponse, error) {
	shared := functioninvokeRequestFromDTO(req)
	resp, err := functioninvoke.Invoke(ctx, svcCtx, shared)
	// The unified chain overwrites GameID/Env with the ctx scope values and
	// normalizes Route; sync them back to the DTO to preserve the pre-close-out
	// entry behavior (callers can read the effective scope).
	req.GameID, req.Env, req.Route = shared.GameID, shared.Env, shared.Route
	if err != nil {
		// Dispatch failure still returns a traceable empty response (matching
		// pre-close-out behavior).
		if resp != nil {
			return functionInvokeResponseFromUnified(resp), err
		}
		return nil, err
	}
	return functionInvokeResponseFromUnified(resp), nil
}

func functioninvokeRequestFromDTO(req *FunctionInvokeRequest) *functioninvoke.Request {
	if req == nil {
		return &functioninvoke.Request{}
	}
	return &functioninvoke.Request{
		ID:              req.ID,
		Params:          req.Params,
		Payload:         req.Payload,
		GameID:          req.GameID,
		Env:             req.Env,
		Mode:            req.Mode,
		Route:           req.Route,
		TargetServiceID: req.TargetServiceID,
		HashKey:         req.HashKey,
		TimeoutMs:       req.TimeoutMs,
		Metadata:        req.Metadata,
		OperationID:     req.OperationID,
	}
}

func functionInvokeResponseFromUnified(resp *functioninvoke.Response) *FunctionInvokeResponse {
	if resp == nil {
		return nil
	}
	out := &FunctionInvokeResponse{
		TaskId:            resp.TaskId,
		Result:            resp.Result,
		ApprovalID:        resp.ApprovalID,
		ApprovalRequired:  resp.ApprovalRequired,
		ApprovalWorkflow:  resp.ApprovalWorkflow,
		TraceID:           resp.TraceID,
		ExecutionMetadata: resp.ExecutionMetadata,
		OperationID:       resp.OperationID,
		Outcome:           resp.Outcome,
	}
	if resp.Broadcast != nil {
		items := make([]BroadcastAgentItem, 0, len(resp.Broadcast.Results))
		for _, item := range resp.Broadcast.Results {
			items = append(items, BroadcastAgentItem{AgentID: item.AgentID, Result: item.Result, Error: item.Error})
		}
		out.Broadcast = &BroadcastResult{
			Total:   resp.Broadcast.Total,
			Success: resp.Broadcast.Success,
			Failure: resp.Broadcast.Failure,
			Results: items,
		}
	}
	return out
}

func validateInvokeInput(ctx context.Context, svcCtx *svc.ServiceContext, req *FunctionInvokeRequest, payload []byte) error {
	return functioninvoke.ValidateInvokeInput(ctx, svcCtx, functioninvokeRequestFromDTO(req), payload)
}

func validateInvokeRoute(req *FunctionInvokeRequest) error {
	if req == nil {
		return errorx.NewBadRequest("invoke request is required")
	}
	shared := functioninvokeRequestFromDTO(req)
	if err := functioninvoke.ValidateInvokeRoute(shared); err != nil {
		return err
	}
	req.Route = shared.Route
	return nil
}

func cloneMetadata(metadata map[string]string) map[string]string {
	return functioninvoke.CloneMetadata(metadata)
}

func isApprovedContinuation(metadata map[string]string) bool {
	return functioninvoke.IsApprovedContinuation(metadata)
}

func isPageSnapshotGoverned(metadata map[string]string) bool {
	return functioninvoke.IsPageSnapshotGoverned(metadata)
}

func effectiveTimeoutMs(ctx context.Context, svcCtx *svc.ServiceContext, req *FunctionInvokeRequest) int {
	return functioninvoke.EffectiveTimeoutMs(ctx, svcCtx, functioninvokeRequestFromDTO(req))
}

func declaredTimeoutMs(ctx context.Context, svcCtx *svc.ServiceContext, req *FunctionInvokeRequest) int {
	return functioninvoke.DeclaredTimeoutMs(ctx, svcCtx, functioninvokeRequestFromDTO(req))
}

func auditFunctionInvoke(ctx context.Context, svcCtx *svc.ServiceContext, functionID string, admin *model.Admin, userRoles []string, functionPolicy *policy.Policy, invokeErr error, durationMs int64) {
	functioninvoke.AuditFunctionInvoke(ctx, svcCtx, functionID, admin, userRoles, functionPolicy, invokeErr, durationMs)
}

func createFunctionApproval(ctx context.Context, svcCtx *svc.ServiceContext, req *FunctionInvokeRequest, payload []byte, admin *model.Admin, functionPolicy *policy.Policy) (string, error) {
	return functioninvoke.CreateFunctionApproval(ctx, svcCtx, functioninvokeRequestFromDTO(req), payload, admin, functionPolicy)
}

func approvalNotifyRecipients(ctx context.Context, svcCtx *svc.ServiceContext) ([]string, error) {
	return functioninvoke.ApprovalNotifyRecipients(ctx, svcCtx)
}

func dedupeStrings(in []string) []string {
	return functioninvoke.DedupeStrings(in)
}

func auditApprovalCreated(ctx context.Context, svcCtx *svc.ServiceContext, functionID string, admin *model.Admin, userRoles []string, approvalID string, functionPolicy *policy.Policy) {
	functioninvoke.AuditApprovalCreated(ctx, svcCtx, functionID, admin, userRoles, approvalID, functionPolicy)
}

func functionPublish(ctx context.Context, svcCtx *svc.ServiceContext, req *FunctionPublishRequest) (*FunctionPublishResponse, error) {
	// Implementation would publish function to approval queue
	return &FunctionPublishResponse{
		ApprovalId: "",
		Published:  true,
	}, nil
}

// Instance management implementations
