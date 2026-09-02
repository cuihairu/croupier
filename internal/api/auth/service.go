package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/ipgeo"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/security/identity"
	"github.com/cuihairu/croupier/internal/security/jwtutil"
	"github.com/cuihairu/croupier/internal/security/otp"
	permissionservice "github.com/cuihairu/croupier/internal/service/permission"
	"github.com/cuihairu/croupier/internal/svc"
)

// ErrMFARequired 表示本地账号已启用 TOTP，登录请求缺少二次验证码。
// handler 据此返回 401 + error=mfa_required，前端展示二次输入。
var ErrMFARequired = errors.New("mfa_required")

type Service struct {
	adminModel *model.AdminModel
	roleModel  *model.RoleModel
	gameModel  *model.GameModel
	permSvc    *permissionservice.PermissionService
	opsStore   *svc.OpsStateStore
	auditSvc   *audit.AuditService
	jwtSecret  string

	// passwordProviders 是密码型身份提供方级联，按顺序尝试；
	// 首个元素始终是本地 admins 表。
	passwordProviders []identity.PasswordProvider
	// oidc 是重定向授权型提供方（可选）。
	oidc identity.OAuthProvider
	// oidcSuccessURL 非空时，OIDC 回调成功后携带 token 跳转到该前端地址。
	oidcSuccessURL string
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

// RefreshIdentityProviders 用给定配置重建外部身份提供方（本地账号
// 始终保留为级联首位）。无效配置返回错误且不改变现状（保存端可回滚）。
func (s *Service) RefreshIdentityProviders(cfg config.AuthProvidersConfig) error {
	ip, err := buildIdentityProviders(cfg)
	if err != nil {
		return err
	}
	s.providersMu.Lock()
	defer s.providersMu.Unlock()
	// 级联首位固定是本地 admins 表
	s.passwordProviders = append([]identity.PasswordProvider{identity.NewLocalProvider(s.adminModel)}, ip.ldap)
	s.oidc = ip.oidc
	s.oidcSuccessURL = ip.oidcURL
	if ip.ldapRoles != nil {
		s.providerDefaultRoles[identity.KindLDAP] = ip.ldapRoles
	}
	if ip.oidc != nil {
		s.providerDefaultRoles[identity.KindOIDC] = ip.oidcRoles
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

	// MFA 仅对 local provider 生效：LDAP/OIDC 的二次验证是 IdP 的职责
	//（OIDC 流本身发生在 IdP 侧；裸 LDAP 部署若需要 MFA 应使用 local
	// 账号承载）。已启用 TOTP 的本地账号必须携带有效 totpCode。
	if ident.Provider == identity.KindLocal && admin.OTPEnabled {
		code := strings.TrimSpace(req.TOTPCode)
		if code == "" {
			s.recordLoginAudit(username, "auth.mfa_required", "failed", req, "totp_required", ident.Provider)
			return nil, ErrMFARequired
		}
		if !otp.VerifyTOTP(admin.OTPSecret, code, 1) {
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
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return nil, fmt.Errorf("generate shadow password: %w", err)
	}
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
		LastGameID: lastGameID,
		LastEnv:    lastEnv,
	}, nil
}

// OIDCAuthCodeURL 生成跳转到身份源的授权 URL，state 内含 HMAC 签名与时间戳。
func (s *Service) OIDCAuthCodeURL() (string, error) {
	_, oidc, _, _ := s.snapshotProviders()
	if oidc == nil {
		return "", errors.New("OIDC 登录未启用")
	}
	state, err := s.newOIDCState()
	if err != nil {
		return "", err
	}
	return oidc.AuthCodeURL(state), nil
}

// OIDCLoginCallback 处理回调：校验 state，用授权码换取身份，JIT 解析本地
// 账号后签发平台 JWT。
func (s *Service) OIDCLoginCallback(ctx context.Context, code, state string, req *LoginRequest) (*LoginResponse, error) {
	_, oidc, _, _ := s.snapshotProviders()
	if oidc == nil {
		return nil, errors.New("OIDC 登录未启用")
	}
	if !s.verifyOIDCState(state) {
		s.recordLoginAudit("", "auth.login_failed", "failed", req, "invalid_state", identity.KindOIDC)
		return nil, errors.New("登录状态校验失败，请重新发起登录")
	}
	if strings.TrimSpace(code) == "" {
		return nil, errors.New("缺少授权码")
	}

	ident, err := oidc.Exchange(ctx, code)
	if err != nil {
		s.recordLoginAudit("", "auth.login_failed", "failed", req, "oidc_exchange_failed", identity.KindOIDC)
		return nil, errors.New("OIDC 登录失败")
	}

	admin, err := s.resolveAdminForIdentity(ctx, ident)
	if err != nil {
		s.recordLoginAudit(ident.Username, "auth.login_failed", "failed", req, "provision_failed", ident.Provider)
		return nil, errors.New("登录失败")
	}
	return s.issueLogin(ctx, admin, ident, req)
}

// OIDCSuccessURL 返回配置的登录成功跳转地址（可为空）。
func (s *Service) OIDCSuccessURL() string {
	_, _, url, _ := s.snapshotProviders()
	return url
}

// newOIDCState 生成 "payload.signature" 形式的 state；
// payload 为 base64url("nonce.timestamp")，签名为 HMAC-SHA256(jwtSecret, payload)。
func (s *Service) newOIDCState() (string, error) {
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("generate state nonce: %w", err)
	}
	payload := fmt.Sprintf("%s.%d", hex.EncodeToString(nonce), time.Now().Unix())
	sig := s.signState(payload)
	return base64.RawURLEncoding.EncodeToString([]byte(payload)) + "." + sig, nil
}

func (s *Service) verifyOIDCState(state string) bool {
	parts := strings.Split(state, ".")
	if len(parts) != 2 {
		return false
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return false
	}
	if !hmac.Equal([]byte(parts[1]), []byte(s.signState(string(payload)))) {
		return false
	}
	// 有效期 10 分钟。
	fields := strings.Split(string(payload), ".")
	if len(fields) != 2 {
		return false
	}
	var ts int64
	if _, err := fmt.Sscanf(fields[1], "%d", &ts); err != nil {
		return false
	}
	age := time.Since(time.Unix(ts, 0))
	return age >= 0 && age <= 10*time.Minute
}

func (s *Service) signState(payload string) string {
	mac := hmac.New(sha256.New, []byte(s.jwtSecret))
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

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
func (s *Service) recordLoginAudit(username, action, result string, req *LoginRequest, reason string, provider string) {
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
	_, _ = s.auditSvc.Log(context.Background(), eventType,
		audit.WithActorID(strings.TrimSpace(username), "user", strings.TrimSpace(username)),
		audit.WithIPAddress(ip, ua),
		audit.WithDetails(metadata),
		audit.WithOutcome(outcome, reason),
	)
}

// Logout 用户登出
func (s *Service) Logout(ctx context.Context, req *LogoutRequest) (*LogoutResponse, error) {
	// 递增 token_version 使该账号所有已签发 token 立即失效（含调用方
	// 当前使用的这一个）。中间件缓存意味着最长 30s 后全网生效。
	if req != nil && req.Username != "" {
		if admin, err := s.adminModel.FindByUsername(ctx, req.Username); err == nil && admin != nil {
			if err := s.adminModel.BumpTokenVersion(ctx, admin.ID); err != nil {
				slog.Default().Warn("logout token revocation failed", "username", req.Username, "error", err)
			}
		}
	}
	return &LogoutResponse{}, nil
}

func (s *Service) Check(ctx context.Context, username string, req *CheckRequest) (*CheckResponse, error) {
	admin, err := s.adminModel.FindByUsername(ctx, username)
	if err != nil {
		return nil, errors.New("用户不存在")
	}

	allowed, err := s.permSvc.CheckPermission(ctx, admin.ID, strings.TrimSpace(req.Resource), strings.TrimSpace(req.Action))
	if err != nil {
		return &CheckResponse{Allowed: false, Reason: err.Error()}, nil
	}
	if !allowed {
		return &CheckResponse{Allowed: false, Reason: "permission denied"}, nil
	}
	return &CheckResponse{Allowed: true}, nil
}

func (s *Service) BatchCheck(ctx context.Context, username string, req *BatchCheckRequest) (*BatchCheckResponse, error) {
	results := make([]CheckResponse, 0, len(req.Checks))
	for _, check := range req.Checks {
		resp, err := s.Check(ctx, username, &check)
		if err != nil {
			return nil, err
		}
		results = append(results, *resp)
	}
	return &BatchCheckResponse{Results: results}, nil
}
