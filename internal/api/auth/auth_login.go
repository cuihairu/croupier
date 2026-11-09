package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/settings"
	"github.com/cuihairu/croupier/internal/security/identity"
	"github.com/cuihairu/croupier/internal/security/jwtutil"
)

// Login 用户登录（本地账号优先，外部密码身份源级联）。
func (s *Service) Login(ctx context.Context, req *LoginRequest) (*LoginResponse, error) {
	username := strings.TrimSpace(req.Username)
	if username == "" {
		return nil, errors.New("用户名不能为空")
	}

	password := strings.TrimSpace(req.Password)
	if password == "" {
		s.recordLoginAudit(username, "auth.login_failed", "failed", req, "password_required", "")
		return nil, errors.New("密码不能为空")
	}

	// 本地账号锁定检查先于一切密码校验：锁定期内连 bcrypt 比对都不做，
	// 避免锁定期间仍可探测密码正确性/消耗 CPU。账号不存在时照常走
	// 后续流程（对不存在的账号保持同样的"用户名或密码错误"响应，
	// 防止账号枚举）。
	if admin, err := s.adminModel.FindByUsername(ctx, username); err == nil && admin != nil {
		if admin.LockedUntil != nil && admin.LockedUntil.After(time.Now()) {
			s.recordLoginAudit(username, "auth.login_locked", "failed", req, "account_locked", identity.KindLocal)
			return nil, fmt.Errorf("账号已锁定，请 %d 分钟后重试", int(time.Until(*admin.LockedUntil).Minutes())+1)
		}
	}

	ident, infraErr := s.authenticatePassword(ctx, username, password)
	if ident == nil {
		reason := "invalid_credentials"
		msg := "用户名或密码错误"
		if infraErr != nil {
			reason = "provider_error"
			msg = "认证服务暂时不可用，请稍后重试"
		}
		s.recordLoginAudit(username, "auth.login_failed", "failed", req, reason, "")
		// 仅 local provider 计入失败次数：外部身份源的失败发生在 IdP
		// 侧，其锁定策略由 IdP 自己负责。infraErr 非空时本地库可能
		// 不可用，跳过计数以免误锁。
		if infraErr == nil {
			s.recordLocalLoginFailure(ctx, username, req)
		}
		return nil, errors.New(msg)
	}

	admin, err := s.resolveAdminForIdentity(ctx, ident)
	if err != nil {
		s.recordLoginAudit(username, "auth.login_failed", "failed", req, "provision_failed", ident.Provider)
		return nil, errors.New("登录失败")
	}

	// 邮箱验证门（#51c 第二批）：仅 local provider + 开关开启 + 账号填过
	// 邮箱且未验证。密码通过后、MFA 之前——邮箱所有权是账号激活的前置，
	// 二次验证是登录的第二因子，顺序先激活后验真。
	if ident.Provider == identity.KindLocal && emailVerificationRequired() && emailVerificationBlocksLogin(admin) {
		s.recordLoginAudit(username, "auth.login_blocked", "failed", req, "email_not_verified", ident.Provider)
		return nil, ErrEmailNotVerified
	}

	// MFA 仅对 local provider 生效：LDAP/OIDC 的二次验证是 IdP 的职责
	//（OIDC 流本身发生在 IdP 侧；裸 LDAP 部署若需要 MFA 应使用 local
	// 账号承载）。已启用 TOTP 的本地账号必须携带有效 totpCode，
	// 或一个未使用过的备用恢复码（丢失验证器 App 时的兜底）。
	if ident.Provider == identity.KindLocal && admin.OTPEnabled {
		if s.verifySecondFactor(ctx, admin, req) {
			// 通过：TOTP 码或恢复码任一有效即可
		} else {
			if strings.TrimSpace(req.TOTPCode) == "" {
				s.recordLoginAudit(username, "auth.mfa_required", "failed", req, "totp_required", ident.Provider)
				return nil, ErrMFARequired
			}
			s.recordLoginAudit(username, "auth.mfa_failed", "failed", req, "invalid_totp", ident.Provider)
			return nil, errors.New("二次验证码错误")
		}
	}

	// 本地账号密码通过：清零连续失败计数与残留锁定。
	if ident.Provider == identity.KindLocal {
		if err := s.adminModel.ResetLoginFailures(ctx, admin.ID); err != nil {
			slog.Default().Warn("reset login failures failed", "username", username, "error", err)
		}
	}

	return s.issueLogin(ctx, admin, ident, req)
}

// recordLocalLoginFailure 对本地账号记录一次密码失败；达到阈值时写入
// 锁定截止时间并审计 auth.login_locked。账号不存在（纯枚举探测）或
// 存储故障时静默跳过——锁定是保护措施，不应反过来阻断正常错误响应。
func (s *Service) recordLocalLoginFailure(ctx context.Context, username string, req *LoginRequest) {
	admin, err := s.adminModel.FindByUsername(ctx, username)
	if err != nil || admin == nil {
		return
	}
	_, lockedUntil, err := s.adminModel.RecordLoginFailure(ctx, admin.ID, s.lockoutThreshold, s.lockoutDuration)
	if err != nil {
		slog.Default().Warn("record login failure failed", "username", username, "error", err)
		return
	}
	if lockedUntil != nil {
		s.recordLoginAudit(username, "auth.login_locked", "failed", req, "threshold_reached", identity.KindLocal)
	}
}

// authenticatePassword 逐个尝试密码提供方。返回 nil 表示所有提供方均拒绝；
// infraErr 记录最后一个基础设施错误（目录/网络故障），用于区分
// "凭证错误"与"认证服务不可用"。
func (s *Service) authenticatePassword(ctx context.Context, username, password string) (*identity.Identity, error) {
	var infraErr error
	providers, _, _, _ := s.snapshotProviders()
	for _, p := range providers {
		ident, err := p.Authenticate(ctx, username, password)
		if err == nil {
			return ident, nil
		}
		if errors.Is(err, identity.ErrInvalidCredentials) {
			continue
		}
		// 基础设施故障：继续尝试后续提供方，但记住错误。
		infraErr = err
		slog.Default().Warn("identity provider failure", "provider", p.Kind(), "error", err)
	}
	return nil, infraErr
}

// resolveAdminForIdentity 将认证通过的身份解析为本地 admin 记录。
// 本地提供方直接查表；外部身份源（LDAP/OIDC）在首次登录时 JIT 创建
// 影子账号并赋予默认角色。并发首登的重复创建由 username 唯一索引兜底。
func (s *Service) resolveAdminForIdentity(ctx context.Context, ident *identity.Identity) (*model.Admin, error) {
	admin, err := s.adminModel.FindByUsername(ctx, ident.Username)
	if err == nil && admin != nil {
		s.backfillProfile(ctx, admin, ident)
		return admin, nil
	}
	if ident.Provider == identity.KindLocal {
		// 本地提供方校验通过后记录又消失，视为凭证失效。
		return nil, identity.ErrInvalidCredentials
	}
	return s.provisionShadowAdmin(ctx, ident)
}

// backfillProfile 补全外部身份源带来的展示信息（仅本地为空时写入）。
func (s *Service) backfillProfile(ctx context.Context, admin *model.Admin, ident *identity.Identity) {
	if ident.Provider == identity.KindLocal {
		return
	}
	updates := map[string]interface{}{}
	if admin.Nickname == "" && ident.Nickname != "" {
		updates["nickname"] = ident.Nickname
		admin.Nickname = ident.Nickname
	}
	if admin.Email == "" && ident.Email != "" {
		updates["email"] = ident.Email
		admin.Email = ident.Email
	}
	if len(updates) == 0 {
		return
	}
	if err := s.adminModel.Update(ctx, admin.ID, updates); err != nil {
		slog.Default().Warn("backfill admin profile failed", "username", ident.Username, "error", err)
	}
}

// provisionShadowAdmin 为外部身份源首次登录创建本地影子账号。
// 密码字段写入随机值（不可用于本地登录），实际认证始终走外部提供方。
func (s *Service) provisionShadowAdmin(ctx context.Context, ident *identity.Identity) (*model.Admin, error) {
	// crypto/rand.Read 自 Go 1.24 起永不返回错误（失败即进程内 fatal），
	// error 分支不可达，已删除。
	buf := make([]byte, 32)
	_, _ = rand.Read(buf)
	nickname := ident.Nickname
	if nickname == "" {
		nickname = ident.Username
	}
	admin := &model.Admin{
		Username: ident.Username,
		Nickname: nickname,
		Email:    ident.Email,
		Status:   1,
	}
	if err := s.adminModel.Create(ctx, admin, hex.EncodeToString(buf)); err != nil {
		// 并发首登：唯一索引冲突后回落到已存在记录。
		existing, findErr := s.adminModel.FindByUsername(ctx, ident.Username)
		if findErr == nil && existing != nil {
			return existing, nil
		}
		return nil, fmt.Errorf("provision shadow admin: %w", err)
	}
	s.assignDefaultRoles(ctx, admin.ID, ident.Provider)
	slog.Default().Info("provisioned shadow admin from external identity",
		"username", ident.Username, "provider", ident.Provider)
	return admin, nil
}

// assignDefaultRoles 按提供方默认角色名赋予本地角色；角色名不存在时告警跳过。
func (s *Service) assignDefaultRoles(ctx context.Context, adminID uint, providerKind string) {
	if s.roleModel == nil {
		return
	}
	_, _, _, rolesMap := s.snapshotProviders()
	names := rolesMap[providerKind]
	for _, name := range names {
		role, ok := s.findRoleByName(ctx, name)
		if !ok {
			slog.Default().Warn("default role not found for external identity",
				"role", name, "provider", providerKind)
			continue
		}
		if err := s.adminModel.AssignRole(ctx, adminID, role.ID); err != nil {
			slog.Default().Warn("assign default role failed",
				"role", name, "adminID", adminID, "error", err)
		}
	}
}

func (s *Service) findRoleByName(ctx context.Context, name string) (*model.Role, bool) {
	roles, _, err := s.roleModel.List(ctx, model.ListRolesOptions{Search: name, PageSize: 100})
	if err != nil {
		return nil, false
	}
	for i := range roles {
		if roles[i].Name == name {
			return &roles[i], true
		}
	}
	return nil, false
}

// issueLogin 完成角色加载、JWT 签发与审计，是密码登录与 OIDC 回调的公共出口。
func (s *Service) issueLogin(ctx context.Context, admin *model.Admin, ident *identity.Identity, req *LoginRequest) (*LoginResponse, error) {
	roleModels, err := s.adminModel.GetAdminRoles(ctx, admin.ID)
	if err != nil {
		s.recordLoginAudit(admin.Username, "auth.login_failed", "failed", req, "load_roles_failed", ident.Provider)
		return nil, errors.New("获取用户角色失败")
	}

	roles := make([]string, 0, len(roleModels))
	for _, role := range roleModels {
		roles = append(roles, role.Name)
	}

	token, err := jwtutil.Sign(s.jwtSecret, admin.Username, roles, admin.ID, admin.TokenVersion, time.Now())
	if err != nil {
		s.recordLoginAudit(admin.Username, "auth.login_failed", "failed", req, "token_generation_failed", ident.Provider)
		return nil, errors.New("生成 token 失败")
	}

	s.recordLoginAudit(admin.Username, "auth.login", "success", req, "", ident.Provider)

	lastGameID, lastEnv := s.validLastScope(ctx, admin.ID, roles, admin.LastGameID, admin.LastEnv)

	return &LoginResponse{
		Token: token,
		User: UserInfo{
			Username: admin.Username,
			Nickname: admin.Nickname,
			Roles:    roles,
		},
		LastGameID:         lastGameID,
		LastEnv:            lastEnv,
		MustChangePassword: adminNeedsPasswordChange(admin),
		MFASetupRequired:   mfaSetupRequired(ident.Provider, admin.OTPEnabled),
	}, nil
}

// mfaSetupRequired 判断本次登录后是否必须绑定 TOTP：账号安全策略
// security.mfaRequired 开启且该账号尚未确认启用。仅对 local provider
// 生效（LDAP/OIDC 的二次验证由 IdP 负责，与既有 MFA 语义一致）。
func mfaSetupRequired(provider string, otpEnabled bool) bool {
	if provider != identity.KindLocal || otpEnabled {
		return false
	}
	return settings.Current().SecurityPolicy().MFARequired
}

// adminNeedsPasswordChange 判断该账号登录后是否必须先改密：被显式标记
// must_change_password，或密码已过有效期（password_expires_at）——两者都
// 通过登录响应 mustChangePassword=true 下发，前端强制走改密流程（#20）。
func adminNeedsPasswordChange(admin *model.Admin) bool {
	if admin.MustChangePassword {
		return true
	}
	return admin.PasswordExpiresAt != nil && admin.PasswordExpiresAt.Before(time.Now())
}
