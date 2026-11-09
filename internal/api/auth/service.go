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

// ErrMFARequired 表示本地账号已启用 TOTP，登录请求缺少二次验证码。
// CodeError 携带 401 + StableCode=mfa_required：经统一 response.Error 输出，
// 前端按 error 码分支展示二次验证码输入，不读 message。
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

	// otpRecoveryModel 持久化 MFA 备用恢复码（可为 nil，测试构造 Service 时省略）。
	otpRecoveryModel *model.AdminOTPRecoveryCodeModel
	// recoveryDB 是 otpRecoveryModel 未注入时兜底建模型用的 DB（可为 nil）。
	recoveryDB *gorm.DB
	// verificationModel 持久化注册邮箱验证令牌（#51c 第二批，可为 nil：
	// nil 时验证链路整体降级为不拦不发）。
	verificationModel *model.EmailVerificationModel

	// passwordProviders 是密码型身份提供方级联，按顺序尝试；
	// 首个元素始终是本地 admins 表。
	passwordProviders []identity.PasswordProvider
	// oidc 是重定向授权型提供方（可选）。
	oidc identity.OAuthProvider
	// oidcSuccessURL 非空时，OIDC 回调成功后携带 token 跳转到该前端地址。
	oidcSuccessURL string
	// github 是 GitHub OAuth2 提供方（nil = 未启用）；successURL 语义同 OIDC。
	github           identity.OAuthProvider
	githubSuccessURL string
	// wechat / genericOAuth 是扩展外部 OAuth 提供方（#51 第三批，nil = 未启用）。
	wechat                 identity.OAuthProvider
	wechatSuccessURL       string
	genericOAuth           identity.OAuthProvider
	genericOAuthSuccessURL string
	// localDisabled = auth.local.enabled 显式关闭：本地账号密码级联被移除，
	// 登录须走外部身份源；Providers 端点据此让前端隐藏账密表单。
	localDisabled bool
	// registerEnabled/registerRoles 是自助注册开关（#51b，默认关）与注册
	// 建号赋予的默认角色名（由 auth.register.defaultRoles 配置；留空则不赋角色）。
	registerEnabled bool
	registerRoles   []string
	// providerDefaultRoles 按 Provider Kind 记录 JIT 建号时的默认角色名。
	providerDefaultRoles map[string][]string

	// providersMu 保护运行时身份提供方热刷新（站点设置「登录方式」
	// 保存即生效——Harbor 模式：L3 DB 配置覆盖 yaml，无需重启）。
	providersMu sync.RWMutex

	// lockoutThreshold / lockoutDuration 是本地账号连续密码失败后的
	// 临时锁定策略（仅 local provider 生效；外部身份源失败计数在 IdP 侧）。
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

// WithVerificationModel 注入邮箱验证令牌模型（#51c 第二批）。nil 时验证
// 链路整体降级：不发信、登录不拦（开关语义仍由 settings 决定）。
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
// （语义同 OIDC：JIT 建号 + 默认角色 + 成功跳转）。
func (s *Service) WithGitHubProvider(p identity.OAuthProvider, defaultRoles []string, successURL string) *Service {
	s.github = p
	s.providerDefaultRoles[identity.KindGitHub] = defaultRoles
	s.githubSuccessURL = successURL
	return s
}

// WithWeChatProvider enables the WeChat Open Platform QR login flow
// （语义同 OIDC：JIT 建号 + 默认角色 + 成功跳转）。
func (s *Service) WithWeChatProvider(p identity.OAuthProvider, defaultRoles []string, successURL string) *Service {
	s.wechat = p
	s.providerDefaultRoles[identity.KindWeChat] = defaultRoles
	s.wechatSuccessURL = successURL
	return s
}

// WithGenericOAuthProvider enables a custom OAuth2 (authorization code) login
// flow（语义同 OIDC：JIT 建号 + 默认角色 + 成功跳转）。
func (s *Service) WithGenericOAuthProvider(p identity.OAuthProvider, defaultRoles []string, successURL string) *Service {
	s.genericOAuth = p
	s.providerDefaultRoles[identity.KindGenericOAuth] = defaultRoles
	s.genericOAuthSuccessURL = successURL
	return s
}

// WithLocalEnabled 设置账号密码登录开关（false = 停用本地级联）。
func (s *Service) WithLocalEnabled(enabled bool) *Service {
	s.localDisabled = !enabled
	return s
}

// WithRegister 配置自助注册开关与默认角色（OPEN-ISSUES #51b）。
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

// snapshotProviders 返回身份提供方的当前快照（热刷新安全读取）。
func (s *Service) snapshotProviders() ([]identity.PasswordProvider, identity.OAuthProvider, string, map[string][]string) {
	s.providersMu.RLock()
	defer s.providersMu.RUnlock()
	roles := make(map[string][]string, len(s.providerDefaultRoles))
	for k, v := range s.providerDefaultRoles {
		roles[k] = v
	}
	return s.passwordProviders, s.oidc, s.oidcSuccessURL, roles
}

// RefreshIdentityProviders 用给定配置重建外部身份提供方（本地账号默认
// 保留为级联首位；auth.local.enabled 显式关闭时本地级联整体移除）。
// 无效配置返回错误且不改变现状（保存端可回滚）。
func (s *Service) RefreshIdentityProviders(cfg config.AuthProvidersConfig) error {
	ip, err := buildIdentityProviders(cfg)
	if err != nil {
		return err
	}
	// 防锁死：本地密码登录与全部外部身份源同时不可用 → 拒绝应用，
	// 保存端（PutKey onAuthChange）据此回滚该键。
	githubUp := ip.github != nil
	wechatUp := ip.wechat != nil
	genericUp := ip.genericOAuth != nil
	if !ip.localEnabled && !cfg.LDAP.Enabled && !cfg.OIDC.Enabled && !githubUp && !wechatUp && !genericUp {
		return errors.New("不能停用所有登录方式：账号密码登录关闭时至少需启用 LDAP/OIDC/GitHub/WeChat/自定义 OAuth 之一")
	}
	s.providersMu.Lock()
	defer s.providersMu.Unlock()
	if ip.localEnabled {
		// 级联首位固定是本地 admins 表
		s.passwordProviders = append([]identity.PasswordProvider{identity.NewLocalProvider(s.adminModel)}, ip.ldap)
	} else {
		// 本地停用：级联只保留 LDAP（可能为空——全靠外部 OAuth 流登录，
		// 密码级联为空时 authenticatePassword 直接拒绝）
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

// WithLoginLockout 覆盖默认的本地账号登录失败锁定策略（阈值/锁定时长）。
func (s *Service) WithLoginLockout(cfg config.LoginLockoutConfig) *Service {
	threshold, duration := cfg.LoginLockoutDefaults()
	s.lockoutThreshold = threshold
	s.lockoutDuration = duration
	return s
}
