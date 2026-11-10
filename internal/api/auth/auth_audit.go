package auth

import (
	"context"
	"strings"

	"github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/ipgeo"
)

func (s *Service) validLastScope(ctx context.Context, adminID uint, roles []string, gameID, env string) (string, string) {
	gameID = strings.TrimSpace(gameID)
	env = strings.TrimSpace(env)
	if gameID == "" || env == "" || s.gameModel == nil {
		return "", ""
	}

	game, err := s.gameModel.FindByGameIDString(ctx, gameID)
	if err != nil || game == nil {
		return "", ""
	}
	bound, err := s.gameModel.HasEnvBinding(ctx, gameID, env)
	if err != nil || !bound {
		return "", ""
	}
	for _, role := range roles {
		name := strings.ToLower(strings.TrimSpace(role))
		if name == "admin" || name == "super_admin" {
			return gameID, env
		}
	}
	scopes, err := s.adminModel.GetAdminEnvScopes(ctx, adminID)
	if err != nil {
		return "", ""
	}
	for _, scope := range scopes {
		if scope.GameID == game.ID && strings.EqualFold(strings.TrimSpace(scope.Env), env) {
			return gameID, env
		}
	}
	return "", ""
}

// recordLoginAudit persists the login audit event to audit_records via the
// AuditService (hash-chained, survives restarts). The legacy in-memory
// OpsStateStore audit trail was removed: audit history must outlive the
// process, and the ops store is for transient state only.
func (s *Service) recordLoginAudit(ctx context.Context, username, action, result string, req *LoginRequest, reason string, provider string) {
	if s == nil || s.auditSvc == nil {
		return
	}

	metadata := map[string]interface{}{}
	ip, ua := "", ""
	if req != nil {
		ip = strings.TrimSpace(req.ClientIP)
		ua = strings.TrimSpace(req.UserAgent)
		if ip != "" {
			metadata["ip"] = ip
			if region := ipgeo.Region(ip); region != "" {
				metadata["ipRegion"] = region
			}
		}
		if ua != "" {
			metadata["userAgent"] = ua
		}
	}
	if reason != "" {
		metadata["reason"] = reason
	}
	if provider != "" {
		metadata["provider"] = provider
	}

	eventType := audit.EventLogin
	outcome := "success"
	if action != string(audit.EventLogin) {
		eventType = audit.EventLoginFailed
		outcome = "failure"
	}
	// 审计必须与请求生命周期解绑（客户端断开不丢登录审计），
	// WithoutCancel 从入参派生并保留值（CR-FIX-22）。
	_, _ = s.auditSvc.Log(context.WithoutCancel(ctx), eventType,
		audit.WithActorID(strings.TrimSpace(username), "user", strings.TrimSpace(username)),
		audit.WithIPAddress(ip, ua),
		audit.WithDetails(metadata),
		audit.WithOutcome(outcome, reason),
	)
}
