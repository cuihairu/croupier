package auth

// AUD-052 refresh-token rotation: opt-in clients (login request
// useRefresh=true) get a short-lived access token (2h) plus a one-time refresh
// token (7d, rotated on redemption); legacy clients (default request, including
// the Flutter mobile app) keep receiving the 24h single token, with behavior
// completely unchanged.
//
// The one-time semantics of refresh-token redemption (Refresh) rely on the
// persistent state in admin_refresh_tokens: a non-empty UsedAt means already
// redeemed, so a second appearance is a replay and triggers revocation of every
// refresh token under that account (reuse detection). On the permission side:
// JWTs with typ=refresh are rejected by jwtutil.Parse, so they cannot serve as
// API access credentials; tokens with typ≠refresh are likewise rejected by
// ParseRefresh — closed out in both directions.
//
// The transport layer keeps JSON body + client-side storage and deliberately
// avoids httpOnly cookies — cookification is a deferred C-005 item (an
// architecture-level change to the auth gate that needs the user to confirm the
// direction) and is out of scope here.

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/ipgeo"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/security/identity"
	"github.com/cuihairu/croupier/internal/security/jwtutil"
)

// refreshTokenModel returns the refresh-token store; when not injected, it is
// lazily built from recoveryDB (the business DB wired in by routes assembly).
// When both are nil (a pure unit-test assembly) it returns nil: login falls
// back to the legacy single token and the redemption endpoint reports an
// internal error.
func (s *Service) refreshTokenModel() *model.AdminRefreshTokenModel {
	if s.refreshTokens != nil {
		return s.refreshTokens
	}
	if s.recoveryDB == nil {
		return nil
	}
	s.refreshTokens = model.NewAdminRefreshTokenModel(s.recoveryDB)
	return s.refreshTokens
}

// WithRefreshTokenModel injects the refresh-token store (for tests; production
// lazily builds it from recoveryDB).
func (s *Service) WithRefreshTokenModel(m *model.AdminRefreshTokenModel) *Service {
	s.refreshTokens = m
	return s
}

// issueRefreshableLogin issues the access+refresh pair and persists the
// rotation state (AUD-052).
//   - ok=false and err==nil: the refresh store is unavailable; the caller falls
//     back to the legacy single token (a refresh flow without persistent state
//     cannot do replay detection — rather degrade than issue one).
//   - err!=nil: a hard failure (issuance failed) and the whole login fails.
func (s *Service) issueRefreshableLogin(ctx context.Context, admin *model.Admin, ident *identity.Identity, req *LoginRequest, roles []string, issuedAt time.Time, lastGameID, lastEnv string) (*LoginResponse, bool, error) {
	m := s.refreshTokenModel()
	if m == nil {
		return nil, false, nil
	}
	accessToken, err := jwtutil.SignAccess(s.jwtSecret, admin.Username, roles, admin.ID, admin.TokenVersion, issuedAt)
	if err != nil {
		s.recordLoginAudit(ctx, admin.Username, "auth.login_failed", "failed", req, "token_generation_failed", ident.Provider)
		return nil, true, errors.New("生成 token 失败")
	}
	refreshToken, err := jwtutil.SignRefresh(s.jwtSecret, admin.Username, admin.ID, admin.TokenVersion, issuedAt)
	if err != nil {
		s.recordLoginAudit(ctx, admin.Username, "auth.login_failed", "failed", req, "token_generation_failed", ident.Provider)
		return nil, true, errors.New("生成 token 失败")
	}
	row := &model.AdminRefreshToken{
		AdminID:   admin.ID,
		TokenHash: model.HashRefreshToken(refreshToken),
		ExpiresAt: issuedAt.Add(jwtutil.RefreshTokenTTL),
		ClientIP:  req.ClientIP,
		UserAgent: req.UserAgent,
	}
	if err := m.Create(ctx, row); err != nil {
		// Store unavailable (database failure): fall back to the legacy single
		// token so login stays usable.
		slog.Default().Warn("persist refresh token failed; falling back to legacy token",
			"username", admin.Username, "error", err)
		return nil, false, nil
	}
	s.recordLoginAudit(ctx, admin.Username, "auth.login", "success", req, "", ident.Provider)
	return &LoginResponse{
		Token:        accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int64(jwtutil.AccessTokenTTL / time.Second),
		User: UserInfo{
			Username: admin.Username,
			Nickname: admin.Nickname,
			Roles:    roles,
		},
		LastGameID:         lastGameID,
		LastEnv:            lastEnv,
		MustChangePassword: adminNeedsPasswordChange(admin),
		MFASetupRequired:   mfaSetupRequired(ident.Provider, admin.OTPEnabled),
	}, true, nil
}

// Refresh redeems a refresh token: verify typ/account status/token
// version/one-time semantics → mark the old token as redeemed (rotation) →
// issue a new access+refresh pair. A replay (already redeemed/revoked) revokes
// every refresh token for the account and audits
// auth.token_refresh_reuse.
func (s *Service) Refresh(ctx context.Context, req *RefreshRequest) (*RefreshResponse, error) {
	m := s.refreshTokenModel()
	if m == nil {
		return nil, errorx.NewInternalError("refresh token store is not initialized")
	}

	claims, err := jwtutil.ParseRefresh(strings.TrimSpace(req.RefreshToken), s.jwtSecret)
	if err != nil {
		return nil, errors.New("刷新令牌无效或已过期")
	}

	admin, err := s.adminModel.FindByUsername(ctx, claims.Username)
	if err != nil || admin == nil || admin.Status != 1 {
		return nil, errors.New("刷新令牌无效或已过期")
	}
	// Same-origin check as the access token: password change/disable/logout
	// bumps token_version, immediately invalidating the old refresh-token family
	// (including rows already persisted but not yet redeemed).
	if admin.TokenVersion != claims.TokenVersion {
		return nil, errors.New("刷新令牌无效或已过期")
	}

	row, err := m.FindByHash(ctx, model.HashRefreshToken(strings.TrimSpace(req.RefreshToken)))
	if err != nil {
		return nil, errors.New("刷新令牌无效或已过期")
	}

	if row.Revoked || row.UsedAt != nil {
		// Replay: a one-time token appearing a second time. Revoke every refresh
		// token under the account (conservative family-wide revocation), so the
		// attacker's downstream chain is voided along with it.
		if err := m.RevokeAllForAdmin(ctx, admin.ID); err != nil {
			slog.Default().Warn("revoke refresh tokens on reuse failed", "username", admin.Username, "error", err)
		}
		s.recordRefreshAudit(ctx, admin.Username, audit.EventTokenRefreshReuse, req, row.ID)
		return nil, errors.New("刷新令牌已失效：检测到重放，该账号全部刷新令牌已吊销，请重新登录")
	}

	now := time.Now()
	if now.After(row.ExpiresAt) {
		return nil, errors.New("刷新令牌无效或已过期")
	}

	if err := m.MarkUsed(ctx, row.ID, now); err != nil {
		return nil, errorx.NewInternalError("refresh rotation failed")
	}

	roleModels, err := s.adminModel.GetAdminRoles(ctx, admin.ID)
	if err != nil {
		return nil, errors.New("获取用户角色失败")
	}
	roles := make([]string, 0, len(roleModels))
	for _, role := range roleModels {
		roles = append(roles, role.Name)
	}

	accessToken, err := jwtutil.SignAccess(s.jwtSecret, admin.Username, roles, admin.ID, admin.TokenVersion, now)
	if err != nil {
		return nil, errors.New("生成 token 失败")
	}
	refreshToken, err := jwtutil.SignRefresh(s.jwtSecret, admin.Username, admin.ID, admin.TokenVersion, now)
	if err != nil {
		return nil, errors.New("生成 token 失败")
	}
	newRow := &model.AdminRefreshToken{
		AdminID:       admin.ID,
		TokenHash:     model.HashRefreshToken(refreshToken),
		RotatedFromID: row.ID,
		ExpiresAt:     now.Add(jwtutil.RefreshTokenTTL),
		ClientIP:      req.ClientIP,
		UserAgent:     req.UserAgent,
	}
	if err := m.Create(ctx, newRow); err != nil {
		// Old token redeemed, new token not persisted: the client can only log
		// in again (fail-closed, no falling back to the old token — falling back
		// would break the one-time semantics).
		slog.Default().Warn("persist rotated refresh token failed", "username", admin.Username, "error", err)
		return nil, errorx.NewInternalError("refresh rotation failed")
	}

	s.recordRefreshAudit(ctx, admin.Username, audit.EventTokenRefresh, req, newRow.ID)
	return &RefreshResponse{
		Token:        accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int64(jwtutil.AccessTokenTTL / time.Second),
		Username:     admin.Username,
	}, nil
}

// recordRefreshAudit mirrors recordLoginAudit (persistent trail in
// audit_records): tokenID is the persisted row ID, letting audit trace the
// redemption chain (RotatedFromID).
func (s *Service) recordRefreshAudit(ctx context.Context, username string, event audit.AuditEventType, req *RefreshRequest, tokenID uint) {
	if s == nil || s.auditSvc == nil {
		return
	}
	ip, ua := "", ""
	if req != nil {
		ip = strings.TrimSpace(req.ClientIP)
		ua = strings.TrimSpace(req.UserAgent)
	}
	metadata := map[string]interface{}{"tokenId": tokenID}
	if ip != "" {
		metadata["ip"] = ip
		if region := ipgeo.Region(ip); region != "" {
			metadata["ipRegion"] = region
		}
	}
	if ua != "" {
		metadata["userAgent"] = ua
	}
	outcome := "success"
	if event == audit.EventTokenRefreshReuse {
		outcome = "failure"
	}
	_, _ = s.auditSvc.Log(context.WithoutCancel(ctx), event,
		audit.WithActorID(strings.TrimSpace(username), "user", strings.TrimSpace(username)),
		audit.WithIPAddress(ip, ua),
		audit.WithDetails(metadata),
		audit.WithOutcome(outcome, ""),
	)
}
