package functioninvoke

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/approvals"
	"github.com/cuihairu/croupier/internal/policy"
	"github.com/cuihairu/croupier/internal/service/notify"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/cuihairu/croupier/internal/telemetry"
)

// AuditFunctionInvoke logs function invocation to audit service
func AuditFunctionInvoke(ctx context.Context, svcCtx *svc.ServiceContext, functionID string, admin *model.Admin, userRoles []string, functionPolicy *policy.Policy, invokeErr error, durationMs int64) {
	username := ""
	if admin != nil {
		username = admin.Username
	}

	outcome := "success"
	errorMsg := ""
	if invokeErr != nil {
		outcome = "failure"
		errorMsg = invokeErr.Error()
	}

	// Build audit details
	details := map[string]interface{}{
		"function_id":   functionID,
		"risk_level":    functionPolicy.DefaultRiskLevel,
		"require_audit": functionPolicy.RequireAudit,
		"is_override":   functionPolicy.IsOverride,
		"policy_source": functionPolicy.Source,
		"user_roles":    userRoles,
		"allowed_roles": functionPolicy.AllowedRoles,
		// Duration and trace id make audit records a data source for
		// invocation analytics (volume / success rate / latency / Jaeger hop).
		"duration_ms": durationMs,
		"traceId":     telemetry.TraceIDFromContext(ctx),
		"gameId":      svc.ResolveGameID(ctx, ""),
		"env":         svc.ResolveEnv(ctx, ""),
	}

	// Log the audit event
	_, err := svcCtx.AuditService.Log(ctx, audit.EventFunctionInvoke,
		audit.WithActorID(username, "user", username),
		audit.WithResourceID("function", functionID),
		audit.WithDetails(details),
		audit.WithOutcome(outcome, errorMsg),
		audit.WithIPAddress("", ""), // Could extract from request context if needed
	)
	if err != nil {
		// 副作用已执行（函数调用完成）：不能返回 5xx 暗示「业务未执行」
		// 或引导盲重试。呈现缺口（WARN + rate-limited 告警），operator 可见。
		svcCtx.AuditService.ReportWriteGap(ctx, audit.EventFunctionInvoke, err)
	}
}

// CreateFunctionApproval creates an approval request for function invocation
func CreateFunctionApproval(ctx context.Context, svcCtx *svc.ServiceContext, req *Request, payload []byte, admin *model.Admin, functionPolicy *policy.Policy) (string, error) {
	username := "system"
	if admin != nil {
		username = admin.Username
	}
	functionID := strings.TrimSpace(req.ID)

	// Generate approval ID
	approvalID := fmt.Sprintf("func_%s_%d", functionID, time.Now().UnixNano())

	// Create approval record
	approval := &approvals.Approval{
		ID:         approvalID,
		State:      "pending",
		FunctionID: functionID,
		GameID:     strings.TrimSpace(req.GameID),
		Env:        strings.TrimSpace(req.Env),
		Actor:      username,
		Mode:       strings.TrimSpace(req.Mode),
		// 空路由默认 lb（ValidateInvokeRoute 同一语义），审批存储需可读路由。
		Route: func() string {
			route := strings.TrimSpace(req.Route)
			if route == "" {
				return "lb"
			}
			return route
		}(),
		TargetServiceID: strings.TrimSpace(req.TargetServiceID),
		HashKey:         strings.TrimSpace(req.HashKey),
		Payload:         payload,
		Metadata:        CloneMetadata(req.Metadata),
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	// Store approval
	_, err := svcCtx.ApprovalsStore.Create(approval)
	if err != nil {
		return "", fmt.Errorf("failed to create approval: %w", err)
	}

	// 通知有审批权限的管理员（站内信/钉钉/webhook，未配置渠道静默跳过）。
	// 申请人同步接收提交通知，与完成事件（approval.approved/rejected）对齐。
	if svcCtx.NotifyService != nil {
		recipients, rErr := ApprovalNotifyRecipients(ctx, svcCtx)
		if rErr != nil {
			recipients = nil
		}
		if approval.Actor != "" {
			recipients = DedupeStrings(append(recipients, approval.Actor))
		}
		svcCtx.NotifyService.Dispatch(ctx, notify.Event{
			Type:       "approval.created",
			Title:      "新的审批请求: " + approval.FunctionID,
			Message:    "用户 " + approval.Actor + " 发起了 " + approval.FunctionID + " 调用审批，等待处理。",
			Recipients: recipients,
			Priority:   "high",
			Data: map[string]interface{}{
				"approvalId": approvalID,
				"functionId": approval.FunctionID,
				"gameId":     approval.GameID,
				"env":        approval.Env,
			},
		})
	}

	return approvalID, nil
}

// ApprovalNotifyRecipients 列出审批通知接收人（admin 角色用户名）。
func ApprovalNotifyRecipients(ctx context.Context, svcCtx *svc.ServiceContext) ([]string, error) {
	if svcCtx.AdminModel == nil {
		return nil, nil
	}
	active := 1
	admins, _, err := svcCtx.AdminModel.List(ctx, model.ListAdminsOptions{Role: "admin", Status: &active, Page: 1, PageSize: 200})
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(admins))
	for i := range admins {
		if admins[i].Username != "" {
			out = append(out, admins[i].Username)
		}
	}
	return out, nil
}

// DedupeStrings 去除空串与重复接收人。
func DedupeStrings(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, v := range in {
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

// AuditApprovalCreated logs approval creation to audit service
func AuditApprovalCreated(ctx context.Context, svcCtx *svc.ServiceContext, functionID string, admin *model.Admin, userRoles []string, approvalID string, functionPolicy *policy.Policy) {
	username := ""
	if admin != nil {
		username = admin.Username
	}

	// Build audit details
	details := map[string]interface{}{
		"function_id":       functionID,
		"approvalId":        approvalID,
		"approval_workflow": functionPolicy.ApprovalWorkflow,
		"risk_level":        functionPolicy.DefaultRiskLevel,
		"user_roles":        userRoles,
		"allowed_roles":     functionPolicy.AllowedRoles,
	}

	// Log the audit event
	_, err := svcCtx.AuditService.Log(ctx, audit.EventApprovalCreated,
		audit.WithActorID(username, "user", username),
		audit.WithResourceID("approval", approvalID),
		audit.WithDetails(details),
		audit.WithOutcome("success", ""),
	)
	if err != nil {
		// 副作用已执行（审批请求已创建）：呈现缺口而非静默/5xx。
		svcCtx.AuditService.ReportWriteGap(ctx, audit.EventApprovalCreated, err)
	}
}
