package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/cuihairu/croupier/internal/common/errorx"

	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/settings"
)

// LocalEnabled reports whether username/password login is enabled
// (auth.local.enabled, enabled by default).
func (s *Service) LocalEnabled() bool {
	s.providersMu.RLock()
	defer s.providersMu.RUnlock()
	return !s.localDisabled
}

// RegisterEnabled reports whether self-service registration is on
// (auth.register.enabled, off by default).
func (s *Service) RegisterEnabled() bool {
	s.providersMu.RLock()
	defer s.providersMu.RUnlock()
	return s.registerEnabled
}

// RegisterRequest is the self-service registration input (POST
// /api/v1/auth/register, accessible anonymously).
type RegisterRequest struct {
	Username  string `json:"username"`
	Password  string `json:"password"`
	Nickname  string `json:"nickname"`
	Email     string `json:"email"`
	ClientIP  string `json:"-"`
	UserAgent string `json:"-"`
}

// Register self-service registers a local account (OPEN-ISSUES #51b, off by
// default). The password is checked by security.* policy (the same chain as
// account creation/password change); default roles are granted on success. No
// token is issued — after registering, the user logs in normally
// (mustChangePassword/MFA gates still apply as usual).
func (s *Service) Register(ctx context.Context, req *RegisterRequest) (*model.Admin, error) {
	if !s.RegisterEnabled() {
		return nil, errors.New("自助注册未开启")
	}
	username := strings.TrimSpace(req.Username)
	if !isValidRegisterUsername(username) {
		return nil, errors.New("用户名须为 3-32 位字母、数字、下划线或连字符")
	}
	if err := utils.ValidatePasswordForUser(req.Password, username); err != nil {
		return nil, err
	}
	if existing, err := s.adminModel.FindByUsername(ctx, username); err == nil && existing != nil {
		return nil, fmt.Errorf("用户名已存在")
	}
	email := strings.TrimSpace(req.Email)
	if emailVerificationRequired() && email == "" {
		// Once required is on, email goes from optional to mandatory: without a
		// delivery address there is nothing to verify.
		return nil, errorx.NewBadRequest("已开启注册邮箱验证，注册必须填写邮箱")
	}
	if email != "" {
		if err := s.checkRegisterEmailPolicy(ctx, email); err != nil {
			return nil, err
		}
	}
	nickname := strings.TrimSpace(req.Nickname)
	if nickname == "" {
		nickname = username
	}
	admin := &model.Admin{
		Username: username,
		Nickname: nickname,
		Email:    email,
		Status:   1,
	}
	if err := s.adminModel.Create(ctx, admin, req.Password); err != nil {
		// Concurrent duplicate name: the unique index turns this into a failure.
		s.recordLoginAudit(ctx, username, "auth.register_failed", "failed",
			&LoginRequest{ClientIP: req.ClientIP, UserAgent: req.UserAgent}, "create_failed", "")
		return nil, fmt.Errorf("注册失败：用户名可能已存在")
	}
	s.assignRegisterRoles(ctx, admin.ID)
	// Batch 2 of #51c: required on and an email provided → issue a verification
	// token and send the mail. Failure is only logged (the token is already
	// persisted and the user can use the resend endpoint), leaving the
	// registration result unaffected.
	s.maybeSendVerificationOnRegister(ctx, admin)
	s.recordLoginAudit(ctx, username, "auth.register", "success",
		&LoginRequest{ClientIP: req.ClientIP, UserAgent: req.UserAgent}, "", "")
	slog.Default().Info("self-registered admin account", "username", username)
	return admin, nil
}

// checkRegisterEmailPolicy is the registration email policy (OPEN-ISSUES
// #51c). When email is non-empty it runs in order: (1) basic shape validation
// (exactly one @, the domain contains a dot); (2) domain-suffix whitelist
// (auth.email.domainWhitelist, empty = unrestricted); (3) alias restriction
// (auth.email.aliasRestriction): reject + alias forms, and compare against
// existing accounts in the same domain after normalizing the local part by
// dropping dots and lowercasing (u.s@example.com counts as us@example.com).
// L3 is read live at each registration — saving takes effect immediately, with
// no need to wire into the hot-refresh assembly chain.
func (s *Service) checkRegisterEmailPolicy(ctx context.Context, email string) error {
	local, domain, hasAt := strings.Cut(email, "@")
	if !hasAt || local == "" || !strings.Contains(domain, ".") || strings.ContainsAny(email, " \t") {
		return fmt.Errorf("邮箱格式无效")
	}
	cur := settings.Current()
	if cur == nil {
		return nil
	}
	if wl, _, ok := cur.GetString(ctx, settings.KeyAuthEmailDomainWhitelist); ok && strings.TrimSpace(wl) != "" {
		domain = strings.ToLower(strings.TrimSuffix(domain, "."))
		allowed := false
		for _, entry := range strings.Split(wl, ",") {
			entry = strings.ToLower(strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(entry), ".")))
			if entry == "" {
				continue
			}
			if domain == entry || strings.HasSuffix(domain, "."+entry) {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("邮箱域名 %s 不在允许清单内", domain)
		}
	}
	if cur.GetBool(settings.KeyAuthEmailAliasRestriction, false) {
		if strings.Contains(local, "+") {
			return fmt.Errorf("邮箱别名（+tag 形态）不允许用于注册")
		}
		canonical := strings.ReplaceAll(strings.ToLower(local), ".", "")
		existing, err := s.adminModel.FindEmailsByDomain(ctx, domain)
		if err == nil {
			for _, e := range existing {
				el, ed, ok2 := strings.Cut(strings.TrimSpace(e), "@")
				if !ok2 {
					continue
				}
				if strings.EqualFold(ed, domain) && strings.ReplaceAll(strings.ToLower(el), ".", "") == canonical {
					return fmt.Errorf("该邮箱（忽略别名与点号后）已被其他账号使用")
				}
			}
		}
	}
	return nil
}

// isValidRegisterUsername validates the registration username shape
// (3-32 chars of letters/digits/_/-).
func isValidRegisterUsername(u string) bool {
	if len(u) < 3 || len(u) > 32 {
		return false
	}
	for _, r := range u {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_', r == '-':
		default:
			return false
		}
	}
	return true
}

// assignRegisterRoles grants the registration default roles; a missing role
// name is skipped with a warning (same policy as JIT provisioning).
func (s *Service) assignRegisterRoles(ctx context.Context, adminID uint) {
	s.providersMu.RLock()
	names := s.registerRoles
	s.providersMu.RUnlock()
	for _, name := range names {
		role, ok := s.findRoleByName(ctx, name)
		if !ok {
			slog.Default().Warn("register default role not found", "role", name)
			continue
		}
		if err := s.adminModel.AssignRole(ctx, adminID, role.ID); err != nil {
			slog.Default().Warn("assign register default role failed",
				"role", name, "adminID", adminID, "error", err)
		}
	}
}
