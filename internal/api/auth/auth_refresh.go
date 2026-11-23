package auth

// AUD-052 刷新令牌轮换：opt-in（登录请求 useRefresh=true）的客户端获得
// 短寿命 access（2h）+ 一次性刷新令牌（7d，兑换即轮换）；legacy 客户端
// （缺省请求，含移动端 Flutter）继续拿 24h 单 token，行为完全不变。
//
// 刷新令牌兑换（Refresh）的一次性语义依赖 admin_refresh_tokens 持久状态：
// UsedAt 非空 = 已兑换过，再次出现即为重放，触发该账号名下全部刷新令牌
// 吊销（reuse detection）。权限面：typ=refresh 的 JWT 被 jwtutil.Parse 拒绝，
// 无法当 API 访问凭据；typ≠refresh 的令牌也被 ParseRefresh 拒绝，双方向收口。
//
// 传输层沿用 JSON body + 客户端存储，刻意不走 httpOnly cookie——cookie 化
// 属 C-005 延期项（鉴权门架构级改动，需用户确认方向），本任务不碰。

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

// refreshTokenModel 返回刷新令牌存储；未注入时从 recoveryDB（routes 装配
// 传入的业务 DB）懒建。两者皆 nil（纯单元测试装配）返回 nil，登录回退
// legacy 单 token、兑换端点报内部错误。
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

// WithRefreshTokenModel 注入刷新令牌存储（测试用；生产走 recoveryDB 懒建）。
func (s *Service) WithRefreshTokenModel(m *model.AdminRefreshTokenModel) *Service {
	s.refreshTokens = m
	return s
}

// issueRefreshableLogin 签发 access+refresh 对并落轮换状态（AUD-052）。
//   - ok=false 且 err==nil：刷新存储不可用，调用方回退 legacy 单 token
//     （无持久状态的 refresh 无法做重放检测，宁降级不发）。
//   - err!=nil：硬失败（签发失败），登录整体失败。
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
		// 存储不可用（库故障）：回退 legacy 单 token，保持登录可用。
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

// Refresh 兑换刷新令牌：校验 typ/账号状态/token 版本/一次性语义 → 标记
// 旧令牌已兑换（轮换）→ 签发新 access+refresh 对。重放（已兑换/已吊销）
// 吊销该账号全部刷新令牌并审计 auth.token_refresh_reuse。
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
	// 与访问令牌同源校验：改密码/禁用/登出会 bump token_version，旧刷新
	// 令牌族立即失效（含已落库未兑换的行）。
	if admin.TokenVersion != claims.TokenVersion {
		return nil, errors.New("刷新令牌无效或已过期")
	}

	row, err := m.FindByHash(ctx, model.HashRefreshToken(strings.TrimSpace(req.RefreshToken)))
	if err != nil {
		return nil, errors.New("刷新令牌无效或已过期")
	}

	if row.Revoked || row.UsedAt != nil {
		// 重放：一次性令牌第二次出现。吊销该账号名下全部刷新令牌
		// （整族保守吊销），攻击者手里的后续链条一并作废。
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
		// 旧令牌已兑换、新令牌未落库：客户端只能重新登录（fail-closed，
		// 不回退旧令牌——回退会破坏一次性语义）。
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

// recordRefreshAudit 与 recordLoginAudit 同构（audit_records 持久留痕）：
// tokenID 是落库行 ID，供审计回溯兑换链（RotatedFromID）。
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
