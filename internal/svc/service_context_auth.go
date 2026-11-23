package svc

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"log/slog"

	objstore "github.com/cuihairu/croupier/internal/platform/objstore"
	"github.com/cuihairu/croupier/internal/platform/settings"
	jwtutil "github.com/cuihairu/croupier/internal/security/jwtutil"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// NewAuthMiddleware 创建认证中间件
func NewAuthMiddleware(svcCtx *ServiceContext) gin.HandlerFunc {
	return NewAuthMiddlewareImpl(svcCtx).Handle
}

// DBHealth 数据库健康检查
type DBHealth struct {
	svcCtx *ServiceContext
}

// NewDBHealth 创建数据库健康检查实例
func NewDBHealth(svcCtx *ServiceContext) *DBHealth {
	return &DBHealth{
		svcCtx: svcCtx,
	}
}

// Check 检查数据库连接
func (h *DBHealth) Check(ctx context.Context) error {
	if h.svcCtx == nil || h.svcCtx.AdminModel == nil {
		return fmt.Errorf("数据库模型未初始化")
	}

	// 执行简单的查询来检查数据库连接
	queryCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	// 尝试查询一个管理员（不关心结果）
	_, err := h.svcCtx.AdminModel.FindOne(queryCtx, 1)
	if err != nil && err != sql.ErrNoRows {
		slog.ErrorContext(queryCtx, "Database health check failed", "error", err)
		return fmt.Errorf("数据库连接检查失败")
	}

	return nil
}

// Ping 简单的 ping 检查
func (h *DBHealth) Ping() error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return h.Check(ctx)
}

// AuthMiddleware 认证中间件实现
type AuthMiddleware struct {
	svcCtx             *ServiceContext
	allowPaths         map[string]struct{}
	allowPref          []string
	publicReadPrefixes []string

	// tokenVersions 缓存 adminID → (tokenVersion, 缓存时刻)，TTL 内不再打库；
	// 撤销操作（登出/改密/禁用）最多延迟 TTL 生效，换取每请求零额外查询。
	tokenVersions sync.Map

	// otpEnabledCache 缓存 adminID → (OTPEnabled, 缓存时刻)，供 MFA 强制
	// gate 查询（与 tokenVersions 同形同 TTL）；绑定确认后最多延迟一个
	// TTL 生效。
	otpEnabledCache sync.Map

	// machinePathPattern 匹配受限机器 token（ARCH-021）允许访问的 Agent
	// 同步端点：/api/v1/agents/<id>/extensions 与 .../extensions/sync。
	// 机器 token 出现在其它任何路径即 403（受限身份不可用于管理面）。
	machinePathPattern *regexp.Regexp
}

// tokenVersionCacheTTL 是 token 版本缓存的生存期。撤销生效延迟上界。
const tokenVersionCacheTTL = 30 * time.Second

type tokenVersionEntry struct {
	version   int
	fetchedAt time.Time
}

// NewAuthMiddlewareImpl 创建认证中间件实例
func NewAuthMiddlewareImpl(svcCtx *ServiceContext) *AuthMiddleware {
	m := &AuthMiddleware{
		svcCtx: svcCtx,
		allowPaths: map[string]struct{}{
			"/healthz":                   {},
			"/api/v1/auth/login":         {},
			"/api/v1/auth/logout":        {},
			"/api/v1/auth/providers":     {}, // 登录页渲染 SSO/LDAP 入口（只回 bool）
			"/api/v1/monitoring/health":  {},
			"/api/v1/monitoring/healthz": {},
		},
		allowPref: []string{
			"/api/v1/auth/login",
			"/api/v1/auth/oidc/", // SSO 跳转 + 身份源回调（state 校验防 CSRF）
		},
		publicReadPrefixes: []string{
			"/api/v1/registry", // 公开访问：注册中心（agents、functions）
			// 客户端公开端点（游戏内 SDK / 玩家侧，无管理台 token）
			"/api/v1/public/", // 配置拉取 + 玩家客服
			"/api/v1/releases/check",
			// file 存储驱动下的头像静态目录（见 registerUploadStaticRoute）。
			// 必须免鉴权：<img src> 请求不会携带 Authorization 头，走鉴权必然
			// 401，头像永远加载不出来。只放行 avatars 子树，其余上传物不对外。
			string(objstore.AvatarPublicPrefix),
			// 游戏图标静态目录（registerIconStaticRoute）：同头像逻辑，<img>
			// 直载免鉴权；内容寻址命名 + CSP 响应头兜 SVG XSS。
			string(objstore.IconPublicPrefix),
		},
	}
	// Prometheus 抓取器不带 JWT：端点开启时加入免认证白名单。
	if svcCtx != nil && svcCtx.Config.Telemetry.Prometheus.Enabled {
		m.allowPaths[svcCtx.Config.Telemetry.Prometheus.PrometheusPath()] = struct{}{}
	}
	// ARCH-021：受限机器 token 只放行 Agent 扩展同步端点（GET 与 POST sync）。
	m.machinePathPattern = regexp.MustCompile(`^/api/v1/agents/[^/]+/extensions(/sync)?$`)
	return m
}

// Handle 处理认证中间件（Gin 风格）
func (m *AuthMiddleware) Handle(c *gin.Context) {
	if m.shouldBypassGin(c) {
		c.Next()
		return
	}

	// ARCH-021：受限机器身份（X-Croupier-Agent-Token 专用头）。机器 token 与
	// 管理员 Bearer JWT 分流：只放行扩展同步端点，路径不符即 403；解析失败
	// 即 401。管理员仍走下方 Authorization 路径（服务端授权不因机器接入而
	// 被取消）。
	if machineToken := c.GetHeader(jwtutil.MachineTokenHeader); machineToken != "" {
		if m.machinePathPattern == nil || !m.machinePathPattern.MatchString(c.Request.URL.Path) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "machine_token_forbidden", "message": "机器身份 token 仅限扩展同步端点"})
			return
		}
		claims, err := jwtutil.ParseMachine(machineToken, jwtutil.GetGlobalSecret())
		if err != nil {
			slog.ErrorContext(c.Request.Context(), "machine token validation failed", "error", err)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "machine_token_invalid", "message": "机器身份认证失败"})
			return
		}
		c.Request = c.Request.WithContext(jwtutil.WithMachineIdentity(c.Request.Context(), claims))
		c.Set("machineIdentity", claims)
		slog.InfoContext(c.Request.Context(), "Authenticated machine", "agentID", claims.AgentID, "gameID", claims.GameID, "env", claims.Env)
		c.Next()
		return
	}

	// AUD-044：全部 protected 端点只认 Authorization: Bearer header。
	// 历史上此处还接受 ?token= query param（SSE 前端经 EventSource 无法
	// 自定义 header），令牌因此落入访问日志/浏览器历史/Referer。C-004 后
	// 前端 SSE 已改走 fetch 流 + Authorization header（web/src/services/
	// core/http.ts 的 createEventSource），query 通道再无仓内消费方，整体
	// 移除：需要流式订阅的客户端同样走 fetch 流（或反向代理注入 header），
	// 不再把 JWT 放进 URL。
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing authorization", "message": "未授权"})
		return
	}
	tokenParts := strings.SplitN(authHeader, " ", 2)
	if len(tokenParts) != 2 || tokenParts[0] != "Bearer" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization header format", "message": "授权头格式错误"})
		return
	}
	token := tokenParts[1]

	if token == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing authorization", "message": "未授权"})
		return
	}

	// 验证 JWT token
	username, roles, adminID, err := m.authenticate(c.Request.Context(), token)
	if err != nil {
		slog.ErrorContext(c.Request.Context(), "authentication failed", "error", err)
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authentication_failed", "message": "认证失败"})
		return
	}

	ctx := context.WithValue(c.Request.Context(), "username", username)
	ctx = context.WithValue(ctx, "roles", roles)
	ctx = context.WithValue(ctx, "adminID", adminID)
	c.Request = c.Request.WithContext(ctx)
	c.Set("username", username)
	c.Set("roles", roles)
	c.Set("adminID", adminID)
	slog.InfoContext(c.Request.Context(), "Authenticated user", "username", username, "roles", roles)

	if !m.mfaGate(c, adminID) {
		return
	}

	c.Next()
}

// mfaBypassPaths 是 MFA 强制 gate 的放行前缀：绑定/解绑流程自身、个人
// 资料与改密（账号恢复通道）、玩家侧公开端点不受 gate 限制。logout 在
// 免认证白名单，本就不经过此处。
var mfaBypassPrefixes = []string{
	"/api/v1/auth/mfa",
	"/api/v1/profile",
}

// mfaGate 账号安全策略强制 TOTP：security.mfaRequired 开启时，未绑定
// TOTP 的 local 账号（PasswordHash 非空判定，与 MFA 管理语义一致）只能
// 访问白名单，其余 API 一律 403 mfa_required——与登录响应
// mfaSetupRequired 同构，前端据此锁定路由强制引导绑定。
// 返回 false 表示已写响应、请求终止。
func (m *AuthMiddleware) mfaGate(c *gin.Context, adminID uint) bool {
	if m.svcCtx == nil || m.svcCtx.AdminModel == nil || adminID == 0 {
		return true
	}
	if !settings.Current().SecurityPolicy().MFARequired {
		return true
	}
	path := c.Request.URL.Path
	for _, prefix := range mfaBypassPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	if m.cachedOTPEnabled(c.Request.Context(), adminID) {
		return true
	}
	c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
		"error":   "mfa_required",
		"message": "账号安全策略要求先绑定二次验证",
	})
	return false
}

// cachedOTPEnabled 查询（带 30s 进程内缓存）账号是否已启用 TOTP。
// 模型查询失败时按 productionFailClosed 分流（AUD-045）：生产信号在场则
// 视为「未启用」并交给 mfaGate 拦截——存储故障不能成为强制双因素的绕过口；
// 测试/精简部署（无生产信号）保持旧的可用性优先放行。
func (m *AuthMiddleware) cachedOTPEnabled(ctx context.Context, adminID uint) bool {
	if v, ok := m.otpEnabledCache.Load(adminID); ok {
		entry := v.(tokenVersionEntry)
		if time.Since(entry.fetchedAt) < tokenVersionCacheTTL {
			return entry.version == 1
		}
	}
	admin, err := m.svcCtx.AdminModel.FindOne(ctx, adminID)
	if err != nil {
		if m.productionFailClosed() {
			slog.ErrorContext(ctx, "otp enabled check failed: failing closed", "adminID", adminID, "error", err)
			return false
		}
		slog.WarnContext(ctx, "otp enabled check skipped", "adminID", adminID, "error", err)
		return true
	}
	enabled := admin.OTPEnabled
	store := 0
	if enabled {
		store = 1
	}
	m.otpEnabledCache.Store(adminID, tokenVersionEntry{version: store, fetchedAt: time.Now()})
	return enabled
}

func (m *AuthMiddleware) shouldBypassGin(c *gin.Context) bool {
	return m.shouldBypass(c.Request)
}

func (m *AuthMiddleware) authenticate(ctx context.Context, token string) (string, []string, uint, error) {
	secret := jwtutil.GetGlobalSecret()
	if strings.TrimSpace(secret) == "" {
		return "", nil, 0, errors.New("jwt secret not initialized")
	}

	claims, err := jwtutil.Parse(token, secret)
	if err != nil {
		return "", nil, 0, fmt.Errorf("invalid token: %w", err)
	}

	// ARCH-021 两型严格分流：受限机器 token 与管理员 JWT 共用同一 HMAC
	// secret，但管理员 Claims 无 tokenType 字段，golang-jwt/v4 反序列化
	// 忽略未知字段——把机器 token 放进 Authorization: Bearer 时 Parse 会
	// 以 admin Claims 成功解析（Username 空、AdminID=0），checkTokenVersion
	// 又会在 AdminID==0 时跳过校验并放行，使机器 token 被当作「空用户名
	// 管理员身份」通过认证。真实管理员 JWT 恒有非空 Username 与非零
	// AdminID（issueLogin 从 admins 行签发），此处拒绝退化 claims。
	if claims.AdminID == 0 || claims.Username == "" {
		return "", nil, 0, errors.New("token missing admin identity")
	}

	// token 版本比对：改密码/禁用/登出后 admins.token_version 已递增，
	// 旧 claims 版本落后即拒绝。版本查询走 30s 进程内缓存（见结构体注释）。
	if err := m.checkTokenVersion(ctx, claims); err != nil {
		return "", nil, 0, err
	}

	return claims.Username, claims.Roles, claims.AdminID, nil
}

// productionFailClosed 报告认证关键查询在存储故障时是否必须 fail-closed。
// 只有「明确的生产信号」（server.mode 或 CROUPIER_MODE/CROUPIER_ENV）才
// 收紧；无生产信号的测试/精简部署保留显式兼容（AUD-045）。svcCtx 缺失
// 或未注入 Config 时无法判定环境，按兼容处理——此时 AdminModel 通常也
// 未装配，两个校验本就整段跳过。
func (m *AuthMiddleware) productionFailClosed() bool {
	if m.svcCtx == nil {
		return false
	}
	return !isDevelopmentConfig(m.svcCtx.Config)
}

// checkTokenVersion 校验 claims.TokenVersion 与库中当前版本一致。
// AdminModel 不可用（单测/精简部署）时跳过校验，保持向后兼容。
func (m *AuthMiddleware) checkTokenVersion(ctx context.Context, claims *jwtutil.Claims) error {
	if m.svcCtx == nil || m.svcCtx.AdminModel == nil || claims.AdminID == 0 {
		return nil
	}
	current, err := m.currentTokenVersion(ctx, claims.AdminID)
	if err != nil {
		// AUD-010：账号不存在（硬删/软删，First 无行即 ErrRecordNotFound）
		// 时旧 JWT 必须拒绝——此前误当存储故障放行，是「删除账号后旧 token
		// 仍被接受」的安全缺口。返回错误与「token revoked」同语义（认证失败）。
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("account not found")
		}
		// 真存储故障：生产 fail-closed（AUD-045）。令牌版本无法核验时放行
		// 等于撤销（登出/改密/禁用）整体失效，且攻击者可用「打挂数据库」
		// 换一张永久通行证；测试/精简部署（无生产信号）保留旧的可用性优先
		// 兼容，避免单测与精简形态全站 401。
		if m.productionFailClosed() {
			slog.ErrorContext(ctx, "token version check failed: failing closed", "adminID", claims.AdminID, "error", err)
			return errors.New("token version unavailable")
		}
		slog.WarnContext(ctx, "token version check skipped", "adminID", claims.AdminID, "error", err)
		return nil
	}
	if claims.TokenVersion != current {
		return errors.New("token revoked")
	}
	return nil
}

func (m *AuthMiddleware) currentTokenVersion(ctx context.Context, adminID uint) (int, error) {
	if v, ok := m.tokenVersions.Load(adminID); ok {
		entry := v.(tokenVersionEntry)
		if time.Since(entry.fetchedAt) < tokenVersionCacheTTL {
			return entry.version, nil
		}
	}
	version, err := m.svcCtx.AdminModel.GetTokenVersion(ctx, adminID)
	if err != nil {
		return 0, err
	}
	m.tokenVersions.Store(adminID, tokenVersionEntry{version: version, fetchedAt: time.Now()})
	return version, nil
}

func (m *AuthMiddleware) shouldBypass(r *http.Request) bool {
	path := r.URL.Path
	if _, ok := m.allowPaths[path]; ok {
		return true
	}
	for _, prefix := range m.allowPref {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	if r.Method == http.MethodGet {
		// 前缀匹配
		for _, prefix := range m.publicReadPrefixes {
			if strings.HasPrefix(path, prefix) {
				return true
			}
		}
	}
	return false
}
