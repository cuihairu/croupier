package auth

import (
	"net/http"

	"errors"
	"sync"
	"time"

	"github.com/cuihairu/croupier/internal/common/errorx"

	"github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/security/identity"
	permissionservice "github.com/cuihairu/croupier/internal/service/permission"
	"github.com/cuihairu/croupier/internal/svc"
	"gorm.io/gorm"
)

// ErrMFARequired indicates the local account has TOTP enabled but the login request
// lacks the second-factor code. CodeError carries 401 + StableCode=mfa_required: through
// the unified response.Error output, the frontend branches on the error code to show the
// second-factor code input and never reads message.
var ErrMFARequired = &errorx.CodeError{
	Code:       http.StatusUnauthorized,
	Message:    "该账号已启用二次验证，请输入动态验证码",
	StableCode: "mfa_required",
}

type Service struct {
	adminModel *model.AdminModel
	roleModel  *model.RoleModel
	gameModel  *model.GameModel
	permSvc    *permissionservice.PermissionService
	opsStore   *svc.OpsStateStore
	auditSvc   *audit.AuditService
	jwtSecret  string

	// otpRecoveryModel persists MFA backup recovery codes (may be nil; tests omit it
	// when constructing the Service).
	otpRecoveryModel *model.AdminOTPRecoveryCodeModel
	// recoveryDB is the DB used to lazily build the model when otpRecoveryModel is
	// not injected (may be nil); also the lazy-build source for refreshTokenModel
	// (AUD-052) when it is not injected.
	recoveryDB *gorm.DB
	// refreshTokens persists refresh token rotation state (AUD-052; may be nil: with
	// nil, login opt-in falls back to the legacy single token and the exchange
	// endpoint reports an internal error). Production lazily builds it from
	// recoveryDB; tests can inject it explicitly via WithRefreshTokenModel.
	refreshTokens *model.AdminRefreshTokenModel
	// verificationModel persists registration email verification tokens (#51c batch 2;
	// may be nil: with nil, the verification chain degrades to neither blocking nor
	// sending).
	verificationModel *model.EmailVerificationModel

	// passwordProviders is the password-type identity provider cascade, tried in
	// order; the first element is always the local admins table.
	passwordProviders []identity.PasswordProvider
	// oidc is the redirect-based authorization provider (optional).
	oidc identity.OAuthProvider
	// oidcSuccessURL, when non-empty, is the frontend address the OIDC callback
	// redirects to with the token after success.
	oidcSuccessURL string
	// github is the GitHub OAuth2 provider (nil = disabled); successURL semantics
	// same as OIDC.
	github           identity.OAuthProvider
	githubSuccessURL string
	// wechat / genericOAuth are extended external OAuth providers (#51 batch 3;
	// nil = disabled).
	wechat                 identity.OAuthProvider
	wechatSuccessURL       string
	genericOAuth           identity.OAuthProvider
	genericOAuthSuccessURL string
	// localDisabled = auth.local.enabled explicitly off: the local account+password
	// cascade is removed and login must go through external identity sources; the
	// Providers endpoint uses this to hide the credential form on the frontend.
	localDisabled bool
	// registerEnabled/registerRoles are the self-service registration toggle
	// (#51b, off by default) and the default role names granted at account creation
	// (configured via auth.register.defaultRoles; empty means no roles assigned).
	registerEnabled bool
	registerRoles   []string
	// providerDefaultRoles records the default role names for JIT account creation
	// per Provider Kind.
	providerDefaultRoles map[string][]string

	// providersMu guards runtime hot refresh of identity providers (saving the
	// site settings "login methods" takes effect immediately - Harbor mode: L3 DB
	// config overrides yaml, no restart needed).
	providersMu sync.RWMutex

	// lockoutThreshold / lockoutDuration are the temporary lockout policy after
	// consecutive local password failures (only effective for the local provider;
	// external identity sources count failures on the IdP side).
	lockoutThreshold int
	lockoutDuration  time.Duration
}

// WithAdminModel swaps the admins model. Exists so tests can drive
// storage-failure branches per-dependency (e.g. recovery codes persist but
// enabling the flag fails) without a second full ServiceContext.
func (s *Service) WithAdminModel(m *model.AdminModel) *Service {
	s.adminModel = m
	return s
}

// WithOTPRecoveryModel wires the store for MFA backup recovery codes.
// Optional: without it, recovery-code features report zero remaining codes and
// recovery-code login falls through to the TOTP path.
func (s *Service) WithOTPRecoveryModel(m *model.AdminOTPRecoveryCodeModel) *Service {
	s.otpRecoveryModel = m
	return s
}

// WithRecoveryDB lets the service build the recovery-code model lazily from a
// DB handle. Used by tests and by wiring paths that only have *gorm.DB.
func (s *Service) WithRecoveryDB(db *gorm.DB) *Service {
	s.recoveryDB = db
	return s
}

// WithVerificationModel injects the email verification token model (#51c batch 2).
// With nil, the verification chain degrades wholesale: no email is sent and login is
// not blocked (toggle semantics are still decided by settings).
func (s *Service) WithVerificationModel(m *model.EmailVerificationModel) *Service {
	s.verificationModel = m
	return s
}

// WithGameModel enables validation of persisted scope before login returns it
// to the frontend. It is optional for legacy callers that do not manage games.
func (s *Service) WithGameModel(gameModel *model.GameModel) *Service {
	s.gameModel = gameModel
	return s
}

// WithAuditService enables persistent login auditing (audit_records table).
// Without it login audit stays memory-only (OpsStateStore) and is lost on
// restart.
func (s *Service) WithAuditService(auditSvc *audit.AuditService) *Service {
	s.auditSvc = auditSvc
	return s
}

// WithRoleModel enables role resolution for JIT provisioning (external
// identity sources get default local roles on first login).
func (s *Service) WithRoleModel(roleModel *model.RoleModel) *Service {
	s.roleModel = roleModel
	return s
}

// WithPasswordProvider appends an external password identity provider
// (e.g. LDAP) to the cascade after the built-in local provider.
func (s *Service) WithPasswordProvider(p identity.PasswordProvider) *Service {
	if p != nil {
		s.passwordProviders = append(s.passwordProviders, p)
	}
	return s
}

// WithOIDCProvider enables the redirect-based OIDC login flow.
// defaultRoles are assigned to shadow accounts on first login; successURL,
// when non-empty, receives the issued token as a query param on callback.
func (s *Service) WithOIDCProvider(p identity.OAuthProvider, defaultRoles []string, successURL string) *Service {
	s.oidc = p
	s.providerDefaultRoles[identity.KindOIDC] = defaultRoles
	s.oidcSuccessURL = successURL
	return s
}

// WithGitHubProvider enables the redirect-based GitHub OAuth2 login flow
// (semantics same as OIDC: JIT provisioning + default roles + success redirect).
func (s *Service) WithGitHubProvider(p identity.OAuthProvider, defaultRoles []string, successURL string) *Service {
	s.github = p
	s.providerDefaultRoles[identity.KindGitHub] = defaultRoles
	s.githubSuccessURL = successURL
	return s
}

// WithWeChatProvider enables the WeChat Open Platform QR login flow
// (semantics same as OIDC: JIT provisioning + default roles + success redirect).
func (s *Service) WithWeChatProvider(p identity.OAuthProvider, defaultRoles []string, successURL string) *Service {
	s.wechat = p
	s.providerDefaultRoles[identity.KindWeChat] = defaultRoles
	s.wechatSuccessURL = successURL
	return s
}

// WithGenericOAuthProvider enables a custom OAuth2 (authorization code) login
// flow (semantics same as OIDC: JIT provisioning + default roles + success redirect).
func (s *Service) WithGenericOAuthProvider(p identity.OAuthProvider, defaultRoles []string, successURL string) *Service {
	s.genericOAuth = p
	s.providerDefaultRoles[identity.KindGenericOAuth] = defaultRoles
	s.genericOAuthSuccessURL = successURL
	return s
}

// WithLocalEnabled sets the account+password login toggle (false = disable the local cascade).
func (s *Service) WithLocalEnabled(enabled bool) *Service {
	s.localDisabled = !enabled
	return s
}

// WithRegister configures the self-service registration toggle and default roles (OPEN-ISSUES #51b).
func (s *Service) WithRegister(enabled bool, defaultRoles []string) *Service {
	s.registerEnabled = enabled
	s.registerRoles = defaultRoles
	return s
}

// WithProviderDefaultRoles sets JIT default roles for a provider kind.
func (s *Service) WithProviderDefaultRoles(kind string, roles []string) *Service {
	s.providerDefaultRoles[kind] = roles
	return s
}

// snapshotProviders returns the current snapshot of identity providers (a safe read
// under hot refresh).
func (s *Service) snapshotProviders() ([]identity.PasswordProvider, identity.OAuthProvider, string, map[string][]string) {
	s.providersMu.RLock()
	defer s.providersMu.RUnlock()
	roles := make(map[string][]string, len(s.providerDefaultRoles))
	for k, v := range s.providerDefaultRoles {
		roles[k] = v
	}
	return s.passwordProviders, s.oidc, s.oidcSuccessURL, roles
}

// RefreshIdentityProviders rebuilds the external identity providers from the given
// config (the local account stays as the head of the cascade by default; when
// auth.local.enabled is explicitly off, the local cascade is removed wholesale).
// Invalid config returns an error and leaves the current state untouched (the saving
// side can roll back).
func (s *Service) RefreshIdentityProviders(cfg config.AuthProvidersConfig) error {
	ip, err := buildIdentityProviders(cfg)
	if err != nil {
		return err
	}
	// Lockout prevention: if local password login and all external identity sources
	// become unavailable at once -> refuse to apply; the saving side (PutKey
	// onAuthChange) rolls back the key accordingly.
	githubUp := ip.github != nil
	wechatUp := ip.wechat != nil
	genericUp := ip.genericOAuth != nil
	if !ip.localEnabled && !cfg.LDAP.Enabled && !cfg.OIDC.Enabled && !githubUp && !wechatUp && !genericUp {
		return errors.New("不能停用所有登录方式：账号密码登录关闭时至少需启用 LDAP/OIDC/GitHub/WeChat/自定义 OAuth 之一")
	}
	s.providersMu.Lock()
	defer s.providersMu.Unlock()
	if ip.localEnabled {
		// The head of the cascade is always the local admins table
		s.passwordProviders = append([]identity.PasswordProvider{identity.NewLocalProvider(s.adminModel)}, ip.ldap)
	} else {
		// Local disabled: the cascade keeps only LDAP (possibly empty - login relies
		// entirely on external OAuth flows; with an empty password cascade
		// authenticatePassword rejects directly)
		s.passwordProviders = nil
		if ip.ldap != nil {
			s.passwordProviders = []identity.PasswordProvider{ip.ldap}
		}
	}
	s.localDisabled = !ip.localEnabled
	s.registerEnabled = ip.registerEnabled
	s.registerRoles = ip.registerRoles
	s.oidc = ip.oidc
	s.oidcSuccessURL = ip.oidcURL
	s.github = ip.github
	s.githubSuccessURL = ip.githubURL
	s.wechat = ip.wechat
	s.wechatSuccessURL = ip.wechatURL
	s.genericOAuth = ip.genericOAuth
	s.genericOAuthSuccessURL = ip.genericOAuthURL
	if ip.ldapRoles != nil {
		s.providerDefaultRoles[identity.KindLDAP] = ip.ldapRoles
	}
	if ip.oidc != nil {
		s.providerDefaultRoles[identity.KindOIDC] = ip.oidcRoles
	}
	if ip.github != nil {
		s.providerDefaultRoles[identity.KindGitHub] = ip.githubRoles
	}
	if ip.wechat != nil {
		s.providerDefaultRoles[identity.KindWeChat] = ip.wechatRoles
	}
	if ip.genericOAuth != nil {
		s.providerDefaultRoles[identity.KindGenericOAuth] = ip.genericOAuthRoles
	}
	return nil
}

// OIDCEnabled reports whether the OIDC login flow is wired.
func (s *Service) OIDCEnabled() bool { return s.oidc != nil }

// LDAPEnabled reports whether an LDAP provider is wired in the cascade.
func (s *Service) LDAPEnabled() bool {
	for _, p := range s.passwordProviders {
		if p.Kind() == identity.KindLDAP {
			return true
		}
	}
	return false
}

func NewService(adminModel *model.AdminModel, permSvc *permissionservice.PermissionService, jwtSecret string, opsStore ...*svc.OpsStateStore) *Service {
	var store *svc.OpsStateStore
	if len(opsStore) > 0 {
		store = opsStore[0]
	}
	threshold, duration := config.LoginLockoutConfig{}.LoginLockoutDefaults()
	return &Service{
		adminModel: adminModel,
		permSvc:    permSvc,
		opsStore:   store,
		jwtSecret:  jwtSecret,
		passwordProviders: []identity.PasswordProvider{
			identity.NewLocalProvider(adminModel),
		},
		providerDefaultRoles: map[string][]string{},
		lockoutThreshold:     threshold,
		lockoutDuration:      duration,
	}
}

// WithLoginLockout overrides the default local-account login failure lockout policy
// (threshold / lockout duration).
func (s *Service) WithLoginLockout(cfg config.LoginLockoutConfig) *Service {
	threshold, duration := cfg.LoginLockoutDefaults()
	s.lockoutThreshold = threshold
	s.lockoutDuration = duration
	return s
}
