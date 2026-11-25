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

// Login authenticates a user (local accounts first, then cascading to external
// password identity sources).
func (s *Service) Login(ctx context.Context, req *LoginRequest) (*LoginResponse, error) {
	username := strings.TrimSpace(req.Username)
	if username == "" {
		return nil, errors.New("用户名不能为空")
	}

	password := strings.TrimSpace(req.Password)
	if password == "" {
		s.recordLoginAudit(ctx, username, "auth.login_failed", "failed", req, "password_required", "")
		return nil, errors.New("密码不能为空")
	}

	// The local-account lockout check runs before any password verification:
	// during the lock window not even a bcrypt comparison is attempted, so the
	// lockout cannot be used to probe password correctness or burn CPU. When the
	// account does not exist, flow continues as usual (a missing account gets the
	// same "wrong username or password" response, to prevent account
	// enumeration).
	if admin, err := s.adminModel.FindByUsername(ctx, username); err == nil && admin != nil {
		if admin.LockedUntil != nil && admin.LockedUntil.After(time.Now()) {
			s.recordLoginAudit(ctx, username, "auth.login_locked", "failed", req, "account_locked", identity.KindLocal)
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
		s.recordLoginAudit(ctx, username, "auth.login_failed", "failed", req, reason, "")
		// Only the local provider counts toward failure attempts: failures at
		// external identity sources happen on the IdP side, whose lockout policy
		// is the IdP's own responsibility. When infraErr is set the local
		// database may be unavailable, so counting is skipped to avoid locking
		// accounts by mistake.
		if infraErr == nil {
			s.recordLocalLoginFailure(ctx, username, req)
		}
		return nil, errors.New(msg)
	}

	admin, err := s.resolveAdminForIdentity(ctx, ident)
	if err != nil {
		s.recordLoginAudit(ctx, username, "auth.login_failed", "failed", req, "provision_failed", ident.Provider)
		return nil, errors.New("登录失败")
	}

	// Email verification gate (batch 2 of #51c): local provider only + the
	// switch enabled + the account has an email that is not yet verified. This
	// runs after the password and before MFA — proving email ownership is a
	// prerequisite for activating the account, while the second factor is the
	// second step of login: activate first, then verify.
	if ident.Provider == identity.KindLocal && emailVerificationRequired() && emailVerificationBlocksLogin(admin) {
		s.recordLoginAudit(ctx, username, "auth.login_blocked", "failed", req, "email_not_verified", ident.Provider)
		return nil, ErrEmailNotVerified
	}

	// MFA only applies to the local provider: the second factor for LDAP/OIDC is
	// the IdP's responsibility (the OIDC flow itself happens on the IdP side;
	// bare-LDAP deployments that want MFA should carry it on a local account).
	// A local account with TOTP enabled must present a valid totpCode, or an
	// unused recovery code (the fallback when the authenticator app is lost).
	if ident.Provider == identity.KindLocal && admin.OTPEnabled {
		if s.verifySecondFactor(ctx, admin, req) {
			// Passed: either a valid TOTP code or recovery code is sufficient.
		} else {
			if strings.TrimSpace(req.TOTPCode) == "" {
				s.recordLoginAudit(ctx, username, "auth.mfa_required", "failed", req, "totp_required", ident.Provider)
				return nil, ErrMFARequired
			}
			s.recordLoginAudit(ctx, username, "auth.mfa_failed", "failed", req, "invalid_totp", ident.Provider)
			return nil, errors.New("二次验证码错误")
		}
	}

	// Local account password verified: reset the consecutive-failure count and
	// any leftover lockout.
	if ident.Provider == identity.KindLocal {
		if err := s.adminModel.ResetLoginFailures(ctx, admin.ID); err != nil {
			slog.Default().Warn("reset login failures failed", "username", username, "error", err)
		}
	}

	return s.issueLogin(ctx, admin, ident, req)
}

// recordLocalLoginFailure records one password failure for a local account;
// when the threshold is reached it writes the lockout deadline and audits
// auth.login_locked. A missing account (pure enumeration probing) or a storage
// failure is skipped silently — lockout is a protective measure and must not in
// turn block the normal error response.
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
		s.recordLoginAudit(ctx, username, "auth.login_locked", "failed", req, "threshold_reached", identity.KindLocal)
	}
}

// authenticatePassword tries password providers one by one. A nil identity
// means every provider rejected the credentials; infraErr records the last
// infrastructure error (directory/network failure) so callers can distinguish
// "bad credentials" from "authentication service unavailable".
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
		// Infrastructure failure: keep trying later providers, but remember the error.
		infraErr = err
		slog.Default().Warn("identity provider failure", "provider", p.Kind(), "error", err)
	}
	return nil, infraErr
}

// resolveAdminForIdentity resolves an authenticated identity into a local admin
// record. The local provider looks the record up directly; external identity
// sources (LDAP/OIDC) JIT-provision a shadow account with default roles on
// first login. Duplicate creation from concurrent first logins is covered by
// the username unique index.
func (s *Service) resolveAdminForIdentity(ctx context.Context, ident *identity.Identity) (*model.Admin, error) {
	admin, err := s.adminModel.FindByUsername(ctx, ident.Username)
	if err == nil && admin != nil {
		s.backfillProfile(ctx, admin, ident)
		return admin, nil
	}
	if ident.Provider == identity.KindLocal {
		// The record vanished after the local provider validated the credentials:
		// treat as invalid credentials.
		return nil, identity.ErrInvalidCredentials
	}
	return s.provisionShadowAdmin(ctx, ident)
}

// backfillProfile fills in display information carried by external identity
// sources (written only when the local value is empty).
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

// provisionShadowAdmin creates a local shadow account on first login from an
// external identity source. The password field is written with a random value
// (unusable for local login); actual authentication always goes through the
// external provider.
func (s *Service) provisionShadowAdmin(ctx context.Context, ident *identity.Identity) (*model.Admin, error) {
	// crypto/rand.Read never returns an error since Go 1.24 (failure is fatal
	// in-process), so the error branch is unreachable and was removed.
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
		// Concurrent first login: on unique-index conflict, fall back to the
		// existing record.
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

// assignDefaultRoles grants local roles named by the provider's default role
// list; a missing role name is skipped with a warning.
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

// issueLogin finishes role loading, JWT issuance and audit; it is the shared
// exit for password login and the OIDC callback.
func (s *Service) issueLogin(ctx context.Context, admin *model.Admin, ident *identity.Identity, req *LoginRequest) (*LoginResponse, error) {
	roleModels, err := s.adminModel.GetAdminRoles(ctx, admin.ID)
	if err != nil {
		s.recordLoginAudit(ctx, admin.Username, "auth.login_failed", "failed", req, "load_roles_failed", ident.Provider)
		return nil, errors.New("获取用户角色失败")
	}

	roles := make([]string, 0, len(roleModels))
	for _, role := range roleModels {
		roles = append(roles, role.Name)
	}

	issuedAt := time.Now()
	lastGameID, lastEnv := s.validLastScope(ctx, admin.ID, roles, admin.LastGameID, admin.LastEnv)

	// AUD-052: when the request explicitly sets useRefresh=true, go through
	// refresh-token rotation (2h access + 7d one-time refresh). If the store is
	// unavailable (test assembly / database failure), fall back to the legacy
	// single token so login is not blocked by rotation infrastructure failure;
	// a hard error (issuance failure) fails the whole login.
	if req != nil && req.UseRefresh {
		resp, handled, err := s.issueRefreshableLogin(ctx, admin, ident, req, roles, issuedAt, lastGameID, lastEnv)
		if handled || err != nil {
			return resp, err
		}
		slog.Default().Warn("refresh token store unavailable; issuing legacy token", "username", admin.Username)
	}

	token, err := jwtutil.Sign(s.jwtSecret, admin.Username, roles, admin.ID, admin.TokenVersion, issuedAt)
	if err != nil {
		s.recordLoginAudit(ctx, admin.Username, "auth.login_failed", "failed", req, "token_generation_failed", ident.Provider)
		return nil, errors.New("生成 token 失败")
	}

	s.recordLoginAudit(ctx, admin.Username, "auth.login", "success", req, "", ident.Provider)

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

// mfaSetupRequired reports whether TOTP must be enrolled after this login: the
// account security policy security.mfaRequired is on and this account has not
// yet confirmed enrollment. Applies only to the local provider (the second
// factor for LDAP/OIDC is the IdP's responsibility, matching the existing MFA
// semantics).
func mfaSetupRequired(provider string, otpEnabled bool) bool {
	if provider != identity.KindLocal || otpEnabled {
		return false
	}
	return settings.Current().SecurityPolicy().MFARequired
}

// adminNeedsPasswordChange reports whether this account must change its
// password right after login: either explicitly flagged must_change_password,
// or the password has expired (password_expires_at) — both are delivered via
// mustChangePassword=true in the login response, and the frontend forces the
// password-change flow (#20).
func adminNeedsPasswordChange(admin *model.Admin) bool {
	if admin.MustChangePassword {
		return true
	}
	return admin.PasswordExpiresAt != nil && admin.PasswordExpiresAt.Before(time.Now())
}
