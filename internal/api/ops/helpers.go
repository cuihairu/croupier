package ops

import (
	"context"
	"fmt"
	"strings"

	"github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/svc"
)

// recordOpsAudit persists an ops audit event (node drain/restart, maintenance
// changes, ...) to the audit_records table via the AuditService. This
// replaces the legacy in-memory OpsStateStore audit trail, which was lost on
// every restart.
func recordOpsAudit(ctx context.Context, svcCtx *svc.ServiceContext, eventType audit.AuditEventType, target, outcome string, details map[string]interface{}) {
	if svcCtx == nil || svcCtx.AuditService == nil {
		return
	}
	actor := "system"
	if username, err := utils.CurrentUsername(ctx); err == nil && strings.TrimSpace(username) != "" {
		actor = strings.TrimSpace(username)
	}
	_, err := svcCtx.AuditService.Log(ctx, eventType,
		audit.WithActorID(actor, "user", actor),
		audit.WithResourceID("node", strings.TrimSpace(target)),
		audit.WithDetails(details),
		audit.WithOutcome(outcome, ""),
	)
	if err != nil {
		svcCtx.AuditService.ReportWriteGap(ctx, eventType, err)
	}
}

// enforceRequestScope 把请求体携带的 gameId/env 与认证 scope 比对。
// 非空请求 scope 且与认证 scope 冲突时返回 403（作用域受限用户不得向越权
// 游戏/环境提交）；认证 scope 为空（平台级）时不限制。
func enforceRequestScope(ctx context.Context, gameID, env string) error {
	scope := svc.GameScopeFromContext(ctx)
	if scope.GameID != "" && gameID != "" && gameID != scope.GameID {
		return errorx.NewForbiddenWithCode("scope_mismatch",
			fmt.Sprintf("game %q not in authenticated scope %q", gameID, scope.GameID), nil)
	}
	if scope.Env != "" && env != "" && env != scope.Env {
		return errorx.NewForbiddenWithCode("scope_mismatch",
			fmt.Sprintf("env %q not in authenticated scope %q", env, scope.Env), nil)
	}
	return nil
}

// safeObjectPathPart 校验对象键/本地路径片段不含路径分隔符或遍历序列。
// role/name/version 参与 deploy 对象键与 agent 暂存路径拼接，须拒绝
// '/', '\\', ".." 与空值，防任意路径写/对象键逃逸。
func safeObjectPathPart(value string) (string, error) {
	v := strings.TrimSpace(value)
	if v == "" {
		return "", fmt.Errorf("must not be empty")
	}
	if strings.ContainsAny(v, "/\\") || v == "." || v == ".." || strings.Contains(v, "..") {
		return "", fmt.Errorf("must not contain path separators or traversal: %q", value)
	}
	return v, nil
}
