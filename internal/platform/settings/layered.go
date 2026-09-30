// Package settings implements the layered platform configuration read model
// (docs/architecture/config-layering.md):
//
//	L1 code default ← L2 config file/env ← L3 database override (highest)
//
// All platform configuration reads must go through Layered — direct reads of
// config.Config or the settings table bypass the layering contract.
package settings

import (
	"context"
	"encoding/json"
	"github.com/cuihairu/croupier/internal/config"
	"log/slog"
	"math"
	"strconv"
	"strings"
	"sync"

	"github.com/cuihairu/croupier/internal/model"
)

// L3 keys (whitelist). Anything not listed here must not be stored in the
// database; bootstrap-class configuration (DSN/ports/JWT secret) is
// L2-only by design and must never appear in this list
// (docs/architecture/config-layering.md §4).
const (
	KeySiteName        = "site.name"
	KeySiteLogoURL     = "site.logoUrl"
	KeySiteFaviconURL  = "site.faviconUrl"
	KeySiteDescription = "site.description"
	KeyFooterCopyright = "footer.copyright"
	KeyFooterICP       = "footer.icp"
	KeyFooterLinks     = "footer.links" // JSON array [{key,title,url}]
	KeyDefaultLocale   = "site.defaultLocale"

	// 系统信息扩展键（OPEN-ISSUES #49）：对外地址、文档链接与协议全文。
	// serverUrl/taskPublicUrl 为展示与外链拼接预留（消费面见 #52/#53），
	// 协议/首页内容为纯文本，经公开站点快照下发给匿名访客。
	KeySiteServerURL     = "site.serverUrl"     // 对外访问地址（域名）
	KeySiteTaskPublicURL = "site.taskPublicUrl" // 异步任务对外地址
	KeySiteDocsURL       = "site.docsUrl"       // 文档链接（关于）
	KeySiteHomeContent   = "site.homeContent"   // 首页内容（登录页欢迎区展示）
	KeySiteUserAgreement = "site.userAgreement" // 用户协议全文
	KeySitePrivacyPolicy = "site.privacyPolicy" // 隐私政策全文

	// features.* 是五域功能开关的 L3 运行时覆盖（P2）：只能关闭 L2 已启用
	// 的域，不能开启 L2 裁剪掉的域（合成语义 L2 ∧ L3，见 FeatureEnabled）。
	KeyFeatureDev        = "features.dev"
	KeyFeatureSupport    = "features.support"
	KeyFeatureAnalytics  = "features.analytics"
	KeyFeatureOps        = "features.ops"
	KeyFeatureExtensions = "features.extensions"

	// obs.* 是观测集成 URL（从 OpsStateStore 内存态迁入，重启不再丢失）。
	KeyObsAlertmanagerURL   = "obs.alertmanagerUrl"
	KeyObsGrafanaExploreURL = "obs.grafanaExploreUrl"
	KeyObsJaegerURL         = "obs.jaegerUrl"

	// notification.* 是审批/告警通知渠道配置（设置中心通知 Tab）。
	// SMTP 为未配置时邮件渠道静默跳过（与 EmailSender no-op 语义一致）。
	// SMTP 传输细节（#55）：encryption 空串 = 自动（465 隐式 TLS，否则
	// STARTTLS-if-advertised）；authType 默认 plain；insecureSkipVerify 默认 false。
	KeyNotifyEmailEnabled           = "notification.emailEnabled"           // bool
	KeyNotifySMTPHost               = "notification.smtpHost"               // string
	KeyNotifySMTPPort               = "notification.smtpPort"               // int
	KeyNotifySMTPUser               = "notification.smtpUser"               // string
	KeyNotifySMTPPassword           = "notification.smtpPassword"           // string（写入后读取接口脱敏）
	KeyNotifySMTPFrom               = "notification.smtpFrom"               // string
	KeyNotifySMTPEncryption         = "notification.smtpEncryption"         // string: ""|none|ssl|starttls
	KeyNotifySMTPAuthType           = "notification.smtpAuthType"           // string: plain|login
	KeyNotifySMTPInsecureSkipVerify = "notification.smtpInsecureSkipVerify" // bool
	KeyNotifyDingtalkURL            = "notification.dingtalkUrl"            // string
	KeyNotifyDingtalkSecret         = "notification.dingtalkSecret"         // string
	KeyNotifyWebhookURL             = "notification.webhookUrl"             // string
	KeyNotifyWebhookSecret          = "notification.webhookSecret"          // string
	KeyNotifyWecomURL               = "notification.wecomUrl"               // string
	KeyNotifyFeishuURL              = "notification.feishuUrl"              // string
	KeyNotifyFeishuSecret           = "notification.feishuSecret"           // string
	KeyNotifyInAppEnabled           = "notification.inAppEnabled"           // bool

	// 性能参数（L3 运行时配置，默认均为 0/false 表示沿用内置基线或无限制）。
	KeyPerfMaxCpuPct      = "perf.maxCpuPct"      // CPU 阈值%（0 = 不过滤）
	KeyPerfMaxMemoryPct   = "perf.maxMemoryPct"   // 内存阈值%（0 = 不过滤）
	KeyPerfMaxDiskPct     = "perf.maxDiskPct"     // 磁盘阈值%（0 = 不过滤）
	KeyPerfMaxConcurrent  = "perf.maxConcurrent"  // 并发请求上限（0 = 无限制）
	KeyPerfMaxThreadCount = "perf.maxThreadCount" // 线程数上限（0 = 无限制）
	KeyPerfCacheSize      = "perf.cacheSize"      // 内存缓存大小（字节，0 = 沿用默认）

	// 日志维护（L3 运行时配置，默认均为 0 表示沿用内置基线）。
	// retentionDays 本批接线（覆盖周期清理保留期，0 = 跟随配置文件）；
	// cleanupCron/copierDir/copierKeep 为占位未接线（诚实边界见 OPEN-ISSUES #54）。
	KeyLogRetentionDays = "log.retentionDays" // 保留天数（0 = 跟随配置文件）
	KeyLogCleanupCron   = "log.cleanupCron"   // 定时清理表达式（占位，未接线）
	KeyLogCopierDir     = "log.copierDir"     // 复制器日志目录（占位，未接线）
	KeyLogCopierKeep    = "log.copierKeep"    // 复制器日志保留数（占位，未接线）

	// 登录方式（外部身份源，L3 运行时配置——Harbor 模式：yaml 仅作
	// bootstrap 初始值，UI 配置热生效；凭据键脱敏回显）
	KeyAuthLocalEnabled       = "auth.local.enabled"       // bool：账号密码登录开关（默认 true）
	KeyAuthLdapEnabled        = "auth.ldap.enabled"        // bool
	KeyAuthLdapAddr           = "auth.ldap.addr"           // string
	KeyAuthLdapBaseDn         = "auth.ldap.baseDn"         // string
	KeyAuthLdapBindDn         = "auth.ldap.bindDn"         // string
	KeyAuthLdapBindPassword   = "auth.ldap.bindPassword"   // string (secret)
	KeyAuthLdapUserFilter     = "auth.ldap.userFilter"     // string
	KeyAuthLdapStartTLS       = "auth.ldap.startTls"       // bool
	KeyAuthLdapDefaultRoles   = "auth.ldap.defaultRoles"   // string (逗号分隔)
	KeyAuthOidcEnabled        = "auth.oidc.enabled"        // bool
	KeyAuthOidcIssuer         = "auth.oidc.issuer"         // string
	KeyAuthOidcClientId       = "auth.oidc.clientId"       // string
	KeyAuthOidcClientSecret   = "auth.oidc.clientSecret"   // string (secret)
	KeyAuthOidcRedirectUrl    = "auth.oidc.redirectUrl"    // string
	KeyAuthOidcDefaultRoles   = "auth.oidc.defaultRoles"   // string (逗号分隔)
	KeyAuthGitHubEnabled      = "auth.github.enabled"      // bool
	KeyAuthGitHubClientId     = "auth.github.clientId"     // string
	KeyAuthGitHubClientSecret = "auth.github.clientSecret" // string (secret)
	KeyAuthGitHubRedirectUrl  = "auth.github.redirectUrl"  // string
	KeyAuthGitHubDefaultRoles = "auth.github.defaultRoles" // string (逗号分隔)
	KeyAuthGitHubSuccessURL   = "auth.github.successUrl"   // string

	// 微信开放平台扫码登录（OPEN-ISSUES #51 第三批）：openid 为影子账号主键。
	KeyAuthWeChatEnabled      = "auth.wechat.enabled"      // bool
	KeyAuthWeChatAppId        = "auth.wechat.appId"        // string
	KeyAuthWeChatAppSecret    = "auth.wechat.appSecret"    // string (secret)
	KeyAuthWeChatRedirectUrl  = "auth.wechat.redirectUrl"  // string
	KeyAuthWeChatDefaultRoles = "auth.wechat.defaultRoles" // string (逗号分隔)
	KeyAuthWeChatSuccessURL   = "auth.wechat.successUrl"   // string

	// 自定义 OAuth2 身份源（OPEN-ISSUES #51 第三批）：三端点 + UserInfo 属性映射。
	KeyAuthGenericOAuthEnabled       = "auth.genericoauth.enabled"       // bool
	KeyAuthGenericOAuthClientId      = "auth.genericoauth.clientId"      // string
	KeyAuthGenericOAuthClientSecret  = "auth.genericoauth.clientSecret"  // string (secret)
	KeyAuthGenericOAuthRedirectUrl   = "auth.genericoauth.redirectUrl"   // string
	KeyAuthGenericOAuthAuthUrl       = "auth.genericoauth.authUrl"       // string
	KeyAuthGenericOAuthTokenUrl      = "auth.genericoauth.tokenUrl"      // string
	KeyAuthGenericOAuthUserInfoUrl   = "auth.genericoauth.userInfoUrl"   // string
	KeyAuthGenericOAuthScopes        = "auth.genericoauth.scopes"        // string (逗号分隔)
	KeyAuthGenericOAuthUsernameField = "auth.genericoauth.usernameField" // string
	KeyAuthGenericOAuthNicknameField = "auth.genericoauth.nicknameField" // string
	KeyAuthGenericOAuthEmailField    = "auth.genericoauth.emailField"    // string
	KeyAuthGenericOAuthDefaultRoles  = "auth.genericoauth.defaultRoles"  // string (逗号分隔)
	KeyAuthGenericOAuthSuccessURL    = "auth.genericoauth.successUrl"    // string

	// 自助注册（OPEN-ISSUES #51b）：默认关闭；DefaultRoles 留空则不赋角色。
	KeyAuthRegisterEnabled      = "auth.register.enabled"      // bool
	KeyAuthRegisterDefaultRoles = "auth.register.defaultRoles" // string (逗号分隔)

	// 注册邮箱策略（OPEN-ISSUES #51c）
	KeyAuthEmailDomainWhitelist      = "auth.email.domainWhitelist"      // string：注册邮箱域后缀白名单（空 = 不限）
	KeyAuthEmailAliasRestriction     = "auth.email.aliasRestriction"     // bool：拒绝 + 别名，local 去点归一查重
	KeyAuthEmailVerificationRequired = "auth.email.verificationRequired" // bool：注册后须邮箱验证才能登录（#51c 第二批）

	// 账号安全策略（L3 运行时配置，全部默认关闭——关闭即维持内置基线：
	// 密码 8-128 位 + 弱密码表 + 至少 2/4 字符类；不限期；TOTP 自助不强制）
	KeySecurityMFARequired            = "security.mfaRequired"              // bool：强制 local 账号启用 TOTP
	KeySecurityPasswordMinLength      = "security.passwordMinLength"        // int：0 = 沿用内置 8
	KeySecurityPasswordRequireUpper   = "security.passwordRequireUppercase" // bool：必须含大写字母
	KeySecurityPasswordRequireSpecial = "security.passwordRequireSpecial"   // bool：必须含特殊字符
	KeySecurityPasswordMaxAgeDays     = "security.passwordMaxAgeDays"       // int：0 = 永不过期

	// 系统维护（OPEN-ISSUES #52）：更新检查源 URL。返回 JSON 版本清单
	// （识别 version/tagName/tag_name/latestVersion 任意一键，兼容 GitHub
	// releases/latest 的 tag_name）；空 = 未配置，检查更新仅回版本注记。
	KeySystemUpdateCheckURL = "system.updateCheckUrl" // string

	// 安全与限制（OPEN-ISSUES #56，语义见 internal/security/secguard）
	KeySecAllowPorts     = "sec.allowPorts"     // string：允许的端口
	KeySecAllowIPs       = "sec.allowIPs"       // string：允许的 IP
	KeySecDomainFilter   = "sec.domainFilter"   // string：域名过滤
	KeySecSSRFProtection = "sec.ssrfProtection" // bool：SSRF 保护

	// 出站调用策略（OPEN-ISSUES #57，第三方服务健康探针与会话/重试）
	KeyNetRequestTimeoutMs = "net.requestTimeoutMs" // int：外呼单请求超时 ms（0=沿用调用方缺省）
	KeyNetMaxRetries       = "net.maxRetries"       // int：5xx/网络错误重试次数（0=不重试）
	KeyNetRetryBackoffMs   = "net.retryBackoffMs"   // int：重试指数退避基数 ms（0=500ms 缺省）
)

// ValidKeys is the L3 whitelist.
var ValidKeys = map[string]struct{}{
	KeySiteName: {}, KeySiteLogoURL: {}, KeySiteFaviconURL: {},
	KeySiteDescription: {}, KeyFooterCopyright: {}, KeyFooterICP: {},
	KeyFooterLinks: {}, KeyDefaultLocale: {},
	KeySiteServerURL: {}, KeySiteTaskPublicURL: {}, KeySiteDocsURL: {},
	KeySiteHomeContent: {}, KeySiteUserAgreement: {}, KeySitePrivacyPolicy: {},

	KeyFeatureDev: {}, KeyFeatureSupport: {}, KeyFeatureAnalytics: {},
	KeyFeatureOps: {}, KeyFeatureExtensions: {},

	KeyObsAlertmanagerURL: {}, KeyObsGrafanaExploreURL: {}, KeyObsJaegerURL: {},

	KeyNotifyEmailEnabled: {}, KeyNotifySMTPHost: {}, KeyNotifySMTPPort: {},
	KeyNotifySMTPUser: {}, KeyNotifySMTPPassword: {}, KeyNotifySMTPFrom: {},
	KeyNotifySMTPEncryption: {}, KeyNotifySMTPAuthType: {},
	KeyNotifySMTPInsecureSkipVerify: {},
	KeyNotifyDingtalkURL:            {}, KeyNotifyDingtalkSecret: {},
	KeyNotifyWebhookURL: {}, KeyNotifyWebhookSecret: {}, KeyNotifyInAppEnabled: {},
	KeyNotifyWecomURL: {}, KeyNotifyFeishuURL: {}, KeyNotifyFeishuSecret: {},

	KeyAuthLocalEnabled: {},
	KeyAuthLdapEnabled:  {}, KeyAuthLdapAddr: {}, KeyAuthLdapBaseDn: {},
	KeyAuthLdapBindDn: {}, KeyAuthLdapBindPassword: {}, KeyAuthLdapUserFilter: {},
	KeyAuthLdapStartTLS: {}, KeyAuthLdapDefaultRoles: {},
	KeyAuthOidcEnabled: {}, KeyAuthOidcIssuer: {}, KeyAuthOidcClientId: {},
	KeyAuthOidcClientSecret: {}, KeyAuthOidcRedirectUrl: {}, KeyAuthOidcDefaultRoles: {},
	KeyAuthGitHubEnabled: {}, KeyAuthGitHubClientId: {}, KeyAuthGitHubClientSecret: {},
	KeyAuthGitHubRedirectUrl: {}, KeyAuthGitHubDefaultRoles: {}, KeyAuthGitHubSuccessURL: {},
	KeyAuthWeChatEnabled: {}, KeyAuthWeChatAppId: {}, KeyAuthWeChatAppSecret: {},
	KeyAuthWeChatRedirectUrl: {}, KeyAuthWeChatDefaultRoles: {}, KeyAuthWeChatSuccessURL: {},
	KeyAuthGenericOAuthEnabled: {}, KeyAuthGenericOAuthClientId: {}, KeyAuthGenericOAuthClientSecret: {},
	KeyAuthGenericOAuthRedirectUrl: {}, KeyAuthGenericOAuthAuthUrl: {}, KeyAuthGenericOAuthTokenUrl: {},
	KeyAuthGenericOAuthUserInfoUrl: {}, KeyAuthGenericOAuthScopes: {}, KeyAuthGenericOAuthUsernameField: {},
	KeyAuthGenericOAuthNicknameField: {}, KeyAuthGenericOAuthEmailField: {}, KeyAuthGenericOAuthDefaultRoles: {},
	KeyAuthGenericOAuthSuccessURL: {},
	KeyAuthRegisterEnabled:        {}, KeyAuthRegisterDefaultRoles: {},
	KeyAuthEmailDomainWhitelist: {}, KeyAuthEmailAliasRestriction: {},
	KeyAuthEmailVerificationRequired: {},
	KeyPerfMaxCpuPct:                 {},
	KeyPerfMaxMemoryPct:              {},
	KeyPerfMaxDiskPct:                {},
	KeyPerfMaxConcurrent:             {},
	KeyPerfMaxThreadCount:            {},
	KeyPerfCacheSize:                 {},
	KeyLogRetentionDays:              {},
	KeyLogCleanupCron:                {},
	KeyLogCopierDir:                  {},
	KeyLogCopierKeep:                 {},
	KeySecAllowPorts:                 {},
	KeySecAllowIPs:                   {},
	KeySecDomainFilter:               {},
	KeySecSSRFProtection:             {},
	KeyNetRequestTimeoutMs:           {},
	KeyNetMaxRetries:                 {},
	KeyNetRetryBackoffMs:             {},

	KeySecurityMFARequired: {}, KeySecurityPasswordMinLength: {},
	KeySecurityPasswordRequireUpper: {}, KeySecurityPasswordRequireSpecial: {},
	KeySecurityPasswordMaxAgeDays: {},

	KeySystemUpdateCheckURL: {},
}

// secretKeys 是读取时必须脱敏的 key（读取接口只回显尾 4 位）。
var secretKeys = map[string]struct{}{
	KeyNotifySMTPPassword:   {},
	KeyNotifyDingtalkSecret: {},
	KeyNotifyWebhookSecret:  {},
	// 飞书群机器人的签名密钥同样是凭据。此前它被登记在 ValidKeys 里却漏在
	// secretKeys 之外：GET /api/v1/site/notification 快照虽有专门的
	// feishuSecretMasked 字段（一直有掩码），但 PutKey 的「掩码回存保护」分支
	// 只认 IsSecretKey——管理端把快照原样回存时，"****+尾4" 会被当成真值
	// 覆盖入库，通知发送从此静默失败（docs/BUGS.md BUG-017）。
	KeyNotifyFeishuSecret:           {},
	KeyAuthLdapBindPassword:         {},
	KeyAuthOidcClientSecret:         {},
	KeyAuthGitHubClientSecret:       {},
	KeyAuthWeChatAppSecret:          {},
	KeyAuthGenericOAuthClientSecret: {},
}

// IsSecretKey reports whether the key holds a credential that must be masked
// on read.
func IsSecretKey(key string) bool {
	_, ok := secretKeys[key]
	return ok
}

// intKeys 是整数语义的 key。
var intKeys = map[string]struct{}{
	KeyNotifySMTPPort:             {},
	KeySecurityPasswordMinLength:  {},
	KeySecurityPasswordMaxAgeDays: {},
	KeyPerfMaxCpuPct:              {},
	KeyPerfMaxMemoryPct:           {},
	KeyPerfMaxDiskPct:             {},
	KeyPerfMaxConcurrent:          {},
	KeyPerfMaxThreadCount:         {},
	KeyPerfCacheSize:              {},
	KeyLogRetentionDays:           {},
	KeyNetRequestTimeoutMs:        {},
	KeyNetMaxRetries:              {},
	KeyNetRetryBackoffMs:          {},
}

// IsIntKey reports whether the key carries a JSON number value.
func IsIntKey(key string) bool {
	_, ok := intKeys[key]
	return ok
}

// boolKeys 是布尔语义的 key（PutKey 校验 + GetBool 读取）。
var boolKeys = map[string]struct{}{
	KeyFeatureDev: {}, KeyFeatureSupport: {}, KeyFeatureAnalytics: {},
	KeyFeatureOps: {}, KeyFeatureExtensions: {},
	KeyNotifyEmailEnabled: {}, KeyNotifyInAppEnabled: {}, KeyNotifySMTPInsecureSkipVerify: {},
	KeyAuthLocalEnabled: {},
	KeyAuthLdapEnabled:  {}, KeyAuthLdapStartTLS: {}, KeyAuthOidcEnabled: {},
	KeyAuthGitHubEnabled:       {},
	KeyAuthWeChatEnabled:       {},
	KeyAuthGenericOAuthEnabled: {},
	KeyAuthRegisterEnabled:     {},
	KeySecurityMFARequired:     {}, KeySecurityPasswordRequireUpper: {},
	KeySecurityPasswordRequireSpecial: {},
	KeySecSSRFProtection:              {},
	KeyAuthEmailAliasRestriction:      {},
	KeyAuthEmailVerificationRequired:  {},
}

// IsBoolKey reports whether the key carries a JSON boolean value.
func IsBoolKey(key string) bool {
	_, ok := boolKeys[key]
	return ok
}

// IsValidKey reports whether key may be overridden at L3.
func IsValidKey(key string) bool {
	_, ok := ValidKeys[key]
	return ok
}

// Layered resolves configuration values across the three layers.
// The DB store loads asynchronously; until it loads, reads fail open to L2
// (same philosophy as feature flags).
type Layered struct {
	mu       sync.RWMutex
	l2Values map[string]json.RawMessage // resolved from config.Config at boot
	l3Loaded bool
	l3       map[string]json.RawMessage
}

var (
	layeredOnce sync.Once
	layered     *Layered
)

// InitLayered wires the singleton with L2 values from the parsed config and
// starts async L3 loading. Call once at server bootstrap.
func InitLayered(ctx context.Context, cfg *ConfigInput, store *model.PlatformSettingModel) *Layered {
	layeredOnce.Do(func() {
		layered = &Layered{l2Values: resolveL2(cfg)}
		if store != nil {
			// Synchronous first load: the table is tiny and this removes the
			// race between boot-time load and early consumers.
			if overrides, err := store.List(ctx); err != nil {
				slog.Warn("settings: load L3 overrides failed; staying on L2 until reload", "err", err)
			} else {
				layered.l3 = overrides
				layered.l3Loaded = true
				slog.Info("settings: L3 overrides loaded", "count", len(overrides))
			}
		}
	})
	return layered
}

// Current returns the initialized layered instance (nil before Init).
func Current() *Layered { return layered }

// ConfigInput carries the L2 values the platform recognizes. Built once from
// config.Config at bootstrap so the settings package does not depend on it.
type ConfigInput struct {
	SiteName      string
	DefaultLocale string

	// FeatureFlags 是 L2 的五域开关（config.FeatureFlagsConfig）。
	FeatureFlags map[string]bool
	// ObsURLs 是 L2 的观测集成兜底值（env var 解析结果），可空。
	ObsAlertmanagerURL   string
	ObsGrafanaExploreURL string
	ObsJaegerURL         string
}

func resolveL2(cfg *ConfigInput) map[string]json.RawMessage {
	out := map[string]json.RawMessage{}
	if cfg == nil {
		return out
	}
	if cfg.SiteName != "" {
		out[KeySiteName], _ = marshalString(cfg.SiteName)
	}
	if cfg.DefaultLocale != "" {
		out[KeyDefaultLocale], _ = marshalString(cfg.DefaultLocale)
	}
	// 五域开关：L2 显式 false 时写入 false；未设置（默认启用）不写，
	// FeatureEnabled 对缺失值 fail-open 到 true。
	for _, name := range featureFlagNames {
		if v, ok := cfg.FeatureFlags[name]; ok {
			out[featureKey(name)], _ = json.Marshal(v)
		}
	}
	if cfg.ObsAlertmanagerURL != "" {
		out[KeyObsAlertmanagerURL], _ = marshalString(cfg.ObsAlertmanagerURL)
	}
	if cfg.ObsGrafanaExploreURL != "" {
		out[KeyObsGrafanaExploreURL], _ = marshalString(cfg.ObsGrafanaExploreURL)
	}
	if cfg.ObsJaegerURL != "" {
		out[KeyObsJaegerURL], _ = marshalString(cfg.ObsJaegerURL)
	}
	return out
}

// featureFlagNames 与 config.Flag* / web access.ts 保持同步。
var featureFlagNames = []string{"dev", "support", "analytics", "ops", "extensions"}

func featureKey(name string) string { return "features." + name }

func marshalString(s string) ([]byte, bool) {
	b, err := json.Marshal(s)
	return b, err == nil
}

// GetString resolves a string setting through the layers.
// Returns (value, source, found).
func (l *Layered) GetString(ctx context.Context, key string) (string, string, bool) {
	if !IsValidKey(key) {
		return "", "invalid", false
	}
	if l == nil {
		return "", "default", false
	}
	l.mu.RLock()
	defer l.mu.RUnlock()
	if raw, ok := l.l3[key]; ok && l.l3Loaded {
		var v string
		if err := json.Unmarshal(raw, &v); err == nil {
			return v, "database", true
		}
	}
	if raw, ok := l.l2Values[key]; ok {
		var v string
		if err := json.Unmarshal(raw, &v); err == nil {
			return v, "config", true
		}
	}
	return "", "default", false
}

// GetBool resolves a boolean setting through the layers.
// 未找到时返回 def（fail-open 由调用方决定）。
func (l *Layered) GetBool(key string, def bool) bool {
	if !IsValidKey(key) {
		return def
	}
	if l == nil {
		return def
	}
	l.mu.RLock()
	defer l.mu.RUnlock()
	read := func(raw json.RawMessage) (bool, bool) {
		var v bool
		if err := json.Unmarshal(raw, &v); err == nil {
			return v, true
		}
		return false, false
	}
	if raw, ok := l.l3[key]; ok && l.l3Loaded {
		if v, ok := read(raw); ok {
			return v
		}
	}
	if raw, ok := l.l2Values[key]; ok {
		if v, ok := read(raw); ok {
			return v
		}
	}
	return def
}

// getBoolWithSource resolves a boolean setting through the layers and reports
// which layer provided it（database/config）and whether any did。bool 原始值
// 经 GetString 读不出（string unmarshal 失败），探测覆盖必须走本方法。
func (l *Layered) getBoolWithSource(key string, def bool) (bool, string, bool) {
	if !IsValidKey(key) || l == nil {
		return def, "default", false
	}
	l.mu.RLock()
	defer l.mu.RUnlock()
	read := func(raw json.RawMessage) (bool, bool) {
		var v bool
		if err := json.Unmarshal(raw, &v); err == nil {
			return v, true
		}
		return false, false
	}
	if raw, ok := l.l3[key]; ok && l.l3Loaded {
		if v, ok := read(raw); ok {
			return v, "database", true
		}
	}
	if raw, ok := l.l2Values[key]; ok {
		if v, ok := read(raw); ok {
			return v, "config", true
		}
	}
	return def, "default", false
}

// GetInt resolves an integer setting through the layers.
func (l *Layered) GetInt(key string, def int) int {
	if !IsValidKey(key) {
		return def
	}
	if l == nil {
		return def
	}
	l.mu.RLock()
	defer l.mu.RUnlock()
	read := func(raw json.RawMessage) (int, bool) {
		var v int
		if err := json.Unmarshal(raw, &v); err == nil {
			return v, true
		}
		// 容错：数值被存成字符串。
		var s string
		if err := json.Unmarshal(raw, &s); err == nil {
			if n, err := strconv.Atoi(strings.TrimSpace(s)); err == nil {
				return n, true
			}
		}
		return 0, false
	}
	if raw, ok := l.l3[key]; ok && l.l3Loaded {
		if v, ok := read(raw); ok {
			return v
		}
	}
	if raw, ok := l.l2Values[key]; ok {
		if v, ok := read(raw); ok {
			return v
		}
	}
	return def
}

// FeatureEnabled 合成五域开关：L2（部署级物理裁剪）∧ L3（运营级软开关）。
//
//   - L2 显式 false → 恒 false（L3 无法开启被部署裁剪的域）
//   - L2 未设置（默认启用）→ 跟随 L3；L3 也未设置 → true（fail-open）
//
// name 取 config.Flag* 常量值。
func (l *Layered) FeatureEnabled(name string) bool {
	l2 := true
	if l != nil {
		l.mu.RLock()
		if raw, ok := l.l2Values[featureKey(name)]; ok {
			var v bool
			if err := json.Unmarshal(raw, &v); err == nil {
				l2 = v
			}
		}
		l.mu.RUnlock()
	}
	if !l2 {
		return false
	}
	return l.GetBool(featureKey(name), true)
}

// ObsSnapshot 是观测集成 URL 的解析结果（含来源诊断）。
type ObsSnapshot struct {
	AlertmanagerURL   string            `json:"alertmanagerUrl"`
	GrafanaExploreURL string            `json:"grafanaExploreUrl"`
	JaegerURL         string            `json:"jaegerUrl"`
	Sources           map[string]string `json:"sources"`
}

// ObsSnapshot resolves the observability integration URLs.
func (l *Layered) ObsSnapshot() ObsSnapshot {
	snap := ObsSnapshot{Sources: map[string]string{}}
	for key, dst := range map[string]*string{
		KeyObsAlertmanagerURL:   &snap.AlertmanagerURL,
		KeyObsGrafanaExploreURL: &snap.GrafanaExploreURL,
		KeyObsJaegerURL:         &snap.JaegerURL,
	} {
		v, src, ok := l.GetString(context.Background(), key)
		if ok {
			*dst = v
			snap.Sources[key] = src
		} else {
			snap.Sources[key] = "default"
		}
	}
	return snap
}

// NotificationSnapshot 是通知渠道配置的读取视图。
// 密钥字段已脱敏（secretMasked），仅用于回显"已配置"状态。
type NotificationSnapshot struct {
	EmailEnabled           bool              `json:"emailEnabled"`
	SMTPHost               string            `json:"smtpHost"`
	SMTPPort               int               `json:"smtpPort"`
	SMTPUser               string            `json:"smtpUser"`
	SMTPFrom               string            `json:"smtpFrom"`
	SMTPPasswordSet        bool              `json:"smtpPasswordSet"`
	SMTPPasswordMasked     string            `json:"smtpPasswordMasked,omitempty"`
	SMTPEncryption         string            `json:"smtpEncryption"`
	SMTPAuthType           string            `json:"smtpAuthType"`
	SMTPInsecureSkipVerify bool              `json:"smtpInsecureSkipVerify"`
	DingtalkURL            string            `json:"dingtalkUrl"`
	DingtalkSecretSet      bool              `json:"dingtalkSecretSet"`
	DingtalkSecretMasked   string            `json:"dingtalkSecretMasked,omitempty"`
	WebhookURL             string            `json:"webhookUrl"`
	WebhookSecretSet       bool              `json:"webhookSecretSet"`
	WebhookSecretMasked    string            `json:"webhookSecretMasked,omitempty"`
	WecomURL               string            `json:"wecomUrl"`
	FeishuURL              string            `json:"feishuUrl"`
	FeishuSecretSet        bool              `json:"feishuSecretSet"`
	FeishuSecretMasked     string            `json:"feishuSecretMasked,omitempty"`
	InAppEnabled           bool              `json:"inAppEnabled"`
	Sources                map[string]string `json:"sources"`
}

// NotificationSnapshot builds the notification channel view (masked).
func (l *Layered) NotificationSnapshot() NotificationSnapshot {
	snap := NotificationSnapshot{
		EmailEnabled: l.GetBool(KeyNotifyEmailEnabled, false),
		SMTPHost:     stringOrEmpty(l.GetString(context.Background(), KeyNotifySMTPHost)),
		SMTPPort:     l.GetInt(KeyNotifySMTPPort, 0),
		SMTPUser:     stringOrEmpty(l.GetString(context.Background(), KeyNotifySMTPUser)),
		SMTPFrom:     stringOrEmpty(l.GetString(context.Background(), KeyNotifySMTPFrom)),
		DingtalkURL:  stringOrEmpty(l.GetString(context.Background(), KeyNotifyDingtalkURL)),
		WebhookURL:   stringOrEmpty(l.GetString(context.Background(), KeyNotifyWebhookURL)),
		WecomURL:     stringOrEmpty(l.GetString(context.Background(), KeyNotifyWecomURL)),
		FeishuURL:    stringOrEmpty(l.GetString(context.Background(), KeyNotifyFeishuURL)),
		InAppEnabled: l.GetBool(KeyNotifyInAppEnabled, true),
		Sources:      map[string]string{},
	}
	// #55 SMTP 传输细节回显（encryption/authType 空串 = 未覆盖走自动/默认）
	snap.SMTPEncryption = stringOrEmpty(l.GetString(context.Background(), KeyNotifySMTPEncryption))
	snap.SMTPAuthType = stringOrEmpty(l.GetString(context.Background(), KeyNotifySMTPAuthType))
	snap.SMTPInsecureSkipVerify = l.GetBool(KeyNotifySMTPInsecureSkipVerify, false)
	mask := func(key string) (set bool, masked string) {
		v, src, ok := l.GetString(context.Background(), key)
		if !ok || v == "" {
			return false, ""
		}
		snap.Sources[key] = src
		if len(v) <= 4 {
			return true, "****"
		}
		return true, "****" + v[len(v)-4:]
	}
	snap.SMTPPasswordSet, snap.SMTPPasswordMasked = mask(KeyNotifySMTPPassword)
	snap.DingtalkSecretSet, snap.DingtalkSecretMasked = mask(KeyNotifyDingtalkSecret)
	snap.WebhookSecretSet, snap.WebhookSecretMasked = mask(KeyNotifyWebhookSecret)
	snap.FeishuSecretSet, snap.FeishuSecretMasked = mask(KeyNotifyFeishuSecret)
	return snap
}

// NotifySMTPConfig 供通知服务构建 EmailSender 的原始配置（未脱敏）。
type NotifySMTPConfig struct {
	Enabled  bool
	Host     string
	Port     int
	User     string
	Password string
	From     string
	// Encryption 传输加密：""|none|ssl|starttls（空串 = 自动，见键注释）
	Encryption string
	// AuthType 认证方式：plain|login（默认 plain）
	AuthType string
	// InsecureSkipVerify 跳过 TLS 证书校验（自签证书场景，默认 false）
	InsecureSkipVerify bool
}

// NotifySMTP resolves the raw SMTP configuration (unmasked, internal use).
func (l *Layered) NotifySMTP() NotifySMTPConfig {
	return NotifySMTPConfig{
		Enabled:            l.GetBool(KeyNotifyEmailEnabled, false),
		Host:               stringOrEmpty(l.GetString(context.Background(), KeyNotifySMTPHost)),
		Port:               l.GetInt(KeyNotifySMTPPort, 0),
		User:               stringOrEmpty(l.GetString(context.Background(), KeyNotifySMTPUser)),
		Password:           stringOrEmpty(l.GetString(context.Background(), KeyNotifySMTPPassword)),
		From:               stringOrEmpty(l.GetString(context.Background(), KeyNotifySMTPFrom)),
		Encryption:         stringOrEmpty(l.GetString(context.Background(), KeyNotifySMTPEncryption)),
		AuthType:           stringOrEmpty(l.GetString(context.Background(), KeyNotifySMTPAuthType)),
		InsecureSkipVerify: l.GetBool(KeyNotifySMTPInsecureSkipVerify, false),
	}
}

// NotifyChannelsResolved 是外部渠道的解析结果（未脱敏，内部使用）。
type NotifyChannelsResolved struct {
	DingtalkURL    string
	DingtalkSecret string
	WebhookURL     string
	WebhookSecret  string
	WecomURL       string
	FeishuURL      string
	FeishuSecret   string
	InAppEnabled   bool
	EmailEnabled   bool
}

// NotifyChannels resolves the external channel endpoints (unmasked).
func (l *Layered) NotifyChannels() NotifyChannelsResolved {
	return NotifyChannelsResolved{
		DingtalkURL:    stringOrEmpty(l.GetString(context.Background(), KeyNotifyDingtalkURL)),
		DingtalkSecret: stringOrEmpty(l.GetString(context.Background(), KeyNotifyDingtalkSecret)),
		WebhookURL:     stringOrEmpty(l.GetString(context.Background(), KeyNotifyWebhookURL)),
		WebhookSecret:  stringOrEmpty(l.GetString(context.Background(), KeyNotifyWebhookSecret)),
		WecomURL:       stringOrEmpty(l.GetString(context.Background(), KeyNotifyWecomURL)),
		FeishuURL:      stringOrEmpty(l.GetString(context.Background(), KeyNotifyFeishuURL)),
		FeishuSecret:   stringOrEmpty(l.GetString(context.Background(), KeyNotifyFeishuSecret)),
		InAppEnabled:   l.GetBool(KeyNotifyInAppEnabled, true),
		EmailEnabled:   l.GetBool(KeyNotifyEmailEnabled, false),
	}
}

// stringOrEmpty 丢弃 GetString 的 found 标志，便于快照组装。
//
//nolint:revive // 参数名 _ 保持调用点可读
func stringOrEmpty(v string, _ string, _ bool) string { return v }

// FeatureDomainState 是单个功能域的开关诊断信息。
type FeatureDomainState struct {
	// Enabled 是合成值（L2 ∧ L3）。
	Enabled bool `json:"enabled"`
	// TrimmedByConfig 表示部署配置（L2）物理裁剪了该域：路由未注册，
	// L3 覆盖无法开启。
	TrimmedByConfig bool `json:"trimmedByConfig"`
	// Overridden 表示存在 L3 数据库覆盖。
	Overridden bool `json:"overridden"`
}

// FeatureSnapshot 是五域开关的管理视图（GET /api/v1/site/features）。
type FeatureSnapshot struct {
	Domains map[string]FeatureDomainState `json:"domains"`
}

// FeatureSnapshot builds the admin feature overview.
func (l *Layered) FeatureSnapshot() FeatureSnapshot {
	out := FeatureSnapshot{Domains: map[string]FeatureDomainState{}}
	for _, name := range featureFlagNames {
		key := featureKey(name)
		l2 := true
		if l != nil {
			l.mu.RLock()
			if raw, ok := l.l2Values[key]; ok {
				var v bool
				if err := json.Unmarshal(raw, &v); err == nil {
					l2 = v
				}
			}
			l.mu.RUnlock()
		}
		out.Domains[name] = FeatureDomainState{
			Enabled:         l.FeatureEnabled(name),
			TrimmedByConfig: !l2,
			Overridden:      l != nil && l.l3Loaded && l.hasL3(key),
		}
	}
	return out
}

func (l *Layered) hasL3(key string) bool {
	if l == nil {
		return false
	}
	_, ok := l.l3[key]
	return ok
}

// Reload re-reads L3 after a Set/Clear so consumers converge without restart.
func (l *Layered) Reload(ctx context.Context, store *model.PlatformSettingModel) {
	if l == nil || store == nil {
		return
	}
	overrides, err := store.List(ctx)
	if err != nil {
		slog.Warn("settings: reload L3 failed", "err", err)
		return
	}
	l.mu.Lock()
	l.l3 = overrides
	l.l3Loaded = true
	l.mu.Unlock()
}

// SiteSnapshot is the public site payload served to any visitor
// (GET /api/v1/public/site). Every field is safe to expose.
type SiteSnapshot struct {
	SiteName      string            `json:"siteName"`
	LogoURL       string            `json:"logoUrl,omitempty"`
	FaviconURL    string            `json:"faviconUrl,omitempty"`
	Description   string            `json:"description,omitempty"`
	FooterCopy    string            `json:"footerCopyright,omitempty"`
	FooterICP     string            `json:"footerIcp,omitempty"`
	FooterLinks   []FooterLink      `json:"footerLinks,omitempty"`
	DefaultLocale string            `json:"defaultLocale,omitempty"`
	ServerURL     string            `json:"serverUrl,omitempty"`
	TaskPublicURL string            `json:"taskPublicUrl,omitempty"`
	DocsURL       string            `json:"docsUrl,omitempty"`
	HomeContent   string            `json:"homeContent,omitempty"`
	UserAgreement string            `json:"userAgreement,omitempty"`
	PrivacyPolicy string            `json:"privacyPolicy,omitempty"`
	Sources       map[string]string `json:"sources"` // per-key provenance
}

// FooterLink mirrors the frontend footer link shape.
type FooterLink struct {
	Key   string `json:"key"`
	Title string `json:"title"`
	URL   string `json:"url"`
}

// SiteSnapshot builds the public snapshot through the layers.
func (l *Layered) SiteSnapshot() SiteSnapshot {
	get := func(key string) (string, string, bool) {
		v, src, ok := l.GetString(context.Background(), key)
		return v, src, ok
	}
	snap := SiteSnapshot{
		SiteName: "Croupier", // L1 built-in default
		Sources:  map[string]string{},
		FooterLinks: []FooterLink{
			{Key: "croupier", Title: "Croupier", URL: "https://github.com/cuihairu/croupier"},
		},
	}
	if v, src, ok := get(KeySiteName); ok {
		snap.SiteName, snap.Sources[KeySiteName] = v, src
	} else {
		snap.Sources[KeySiteName] = "default"
	}
	if v, src, ok := get(KeySiteLogoURL); ok {
		snap.LogoURL = v
		snap.Sources[KeySiteLogoURL] = src
	}
	if v, src, ok := get(KeySiteFaviconURL); ok {
		snap.FaviconURL = v
		snap.Sources[KeySiteFaviconURL] = src
	}
	if v, src, ok := get(KeySiteDescription); ok {
		snap.Description = v
		snap.Sources[KeySiteDescription] = src
	}
	if v, src, ok := get(KeyFooterCopyright); ok {
		snap.FooterCopy = v
		snap.Sources[KeyFooterCopyright] = src
	}
	if v, src, ok := get(KeyFooterICP); ok {
		snap.FooterICP = v
		snap.Sources[KeyFooterICP] = src
	}
	if raw, src, ok := get(KeyFooterLinks); ok {
		var links []FooterLink
		if err := json.Unmarshal([]byte(raw), &links); err == nil && len(links) > 0 {
			snap.FooterLinks = links
			snap.Sources[KeyFooterLinks] = src
		}
	}
	if v, src, ok := get(KeyDefaultLocale); ok {
		snap.DefaultLocale = v
		snap.Sources[KeyDefaultLocale] = src
	}
	if v, src, ok := get(KeySiteServerURL); ok {
		snap.ServerURL = v
		snap.Sources[KeySiteServerURL] = src
	}
	if v, src, ok := get(KeySiteTaskPublicURL); ok {
		snap.TaskPublicURL = v
		snap.Sources[KeySiteTaskPublicURL] = src
	}
	if v, src, ok := get(KeySiteDocsURL); ok {
		snap.DocsURL = v
		snap.Sources[KeySiteDocsURL] = src
	}
	if v, src, ok := get(KeySiteHomeContent); ok {
		snap.HomeContent = v
		snap.Sources[KeySiteHomeContent] = src
	}
	if v, src, ok := get(KeySiteUserAgreement); ok {
		snap.UserAgreement = v
		snap.Sources[KeySiteUserAgreement] = src
	}
	if v, src, ok := get(KeySitePrivacyPolicy); ok {
		snap.PrivacyPolicy = v
		snap.Sources[KeySitePrivacyPolicy] = src
	}
	return snap
}

// ResetForTest clears the singleton (test-only hook, exported for
// cross-package handler tests).
func ResetForTest() {
	resetForTest()
}

// resetForTest clears the singleton (test-only).
func resetForTest() {
	layeredOnce = sync.Once{}
	layered = nil
}

// AuthSnapshot 是登录方式（外部身份源）的读视图（凭据脱敏）。
type AuthSnapshot struct {
	Local        LocalAuthSnapshot    `json:"local"`
	GitHub       AuthProviderSnapshot `json:"github"`
	WeChat       AuthProviderSnapshot `json:"wechat"`
	GenericOAuth AuthProviderSnapshot `json:"genericoauth"`
	LDAP         AuthProviderSnapshot `json:"ldap"`
	OIDC         AuthProviderSnapshot `json:"oidc"`
	Register     AuthProviderSnapshot `json:"register"` // 自助注册（#51b）：enabled + defaultRoles
	Email        EmailPolicySnapshot  `json:"email"`    // 注册邮箱策略（#51c）
}

// EmailPolicySnapshot 是注册邮箱策略（OPEN-ISSUES #51c）的读视图：
// 域白名单空串 = 不限；别名限制默认关。语义在 auth service.Register
// 注册链路执行。
type EmailPolicySnapshot struct {
	DomainWhitelist  string `json:"domainWhitelist"`
	AliasRestriction bool   `json:"aliasRestriction"`
	// VerificationRequired：注册后须邮箱验证才能登录（令牌链路见
	// auth service #51c 第二批）。
	VerificationRequired bool              `json:"verificationRequired"`
	Sources              map[string]string `json:"sources"`
}

// EmailPolicy resolves the register email policy.
func (l *Layered) EmailPolicy() EmailPolicySnapshot {
	snap := EmailPolicySnapshot{Sources: map[string]string{}}
	v, src, ok := l.GetString(context.Background(), KeyAuthEmailDomainWhitelist)
	if ok {
		snap.DomainWhitelist = v
		snap.Sources[KeyAuthEmailDomainWhitelist] = src
	} else {
		snap.Sources[KeyAuthEmailDomainWhitelist] = "default"
	}
	snap.AliasRestriction, snap.Sources[KeyAuthEmailAliasRestriction], _ = l.getBoolWithSource(KeyAuthEmailAliasRestriction, false)
	snap.VerificationRequired, snap.Sources[KeyAuthEmailVerificationRequired], _ = l.getBoolWithSource(KeyAuthEmailVerificationRequired, false)
	return snap
}

// LocalAuthSnapshot 账号密码登录开关读视图（默认启用，显式覆盖 false 才停用；
// 停用前置约束见 auth.RefreshIdentityProviders 防锁死守卫）。
type LocalAuthSnapshot struct {
	Enabled    bool   `json:"enabled"`
	Overridden bool   `json:"overridden"`
	Source     string `json:"source,omitempty"` // database/config
}

// AuthProviderSnapshot 单一身份源的生效配置（secret 只回 set+尾4）。
type AuthProviderSnapshot struct {
	Enabled      bool              `json:"enabled"`
	Fields       map[string]string `json:"fields"`       // 非凭据字段的生效值
	SecretSet    bool              `json:"secretSet"`    // 凭据是否已设置
	SecretMasked string            `json:"secretMasked"` // ****+尾4
	Sources      map[string]string `json:"sources"`      // 每键来源（database/yaml/default）
}

// AuthSnapshot resolves the identity provider settings (masked).
func (l *Layered) AuthSnapshot() AuthSnapshot {
	local := LocalAuthSnapshot{Enabled: true} // L1 默认启用
	if v, src, ok := l.getBoolWithSource(KeyAuthLocalEnabled, true); ok {
		local.Overridden = true
		local.Source = src
		local.Enabled = v
	}
	return AuthSnapshot{
		Local:        local,
		GitHub:       l.authProviderSnapshot("github"),
		WeChat:       l.authProviderSnapshot("wechat"),
		GenericOAuth: l.authProviderSnapshot("genericoauth"),
		LDAP:         l.authProviderSnapshot("ldap"),
		OIDC:         l.authProviderSnapshot("oidc"),
		Register:     l.authProviderSnapshot("register"),
		Email:        l.EmailPolicy(),
	}
}

// SecurityPolicySnapshot 是账号安全策略的读视图（登录/改密/建号校验链
// 与设置页共用；全零值 = 关闭，维持内置基线）。
type SecurityPolicySnapshot struct {
	MFARequired              bool `json:"mfaRequired"`
	PasswordMinLength        int  `json:"passwordMinLength"`
	PasswordRequireUppercase bool `json:"passwordRequireUppercase"`
	PasswordRequireSpecial   bool `json:"passwordRequireSpecial"`
	PasswordMaxAgeDays       int  `json:"passwordMaxAgeDays"`
}

// SecurityPolicy resolves the account security policy (defaults off).
func (l *Layered) SecurityPolicy() SecurityPolicySnapshot {
	return SecurityPolicySnapshot{
		MFARequired:              l.GetBool(KeySecurityMFARequired, false),
		PasswordMinLength:        l.GetInt(KeySecurityPasswordMinLength, 0),
		PasswordRequireUppercase: l.GetBool(KeySecurityPasswordRequireUpper, false),
		PasswordRequireSpecial:   l.GetBool(KeySecurityPasswordRequireSpecial, false),
		PasswordMaxAgeDays:       l.GetInt(KeySecurityPasswordMaxAgeDays, 0),
	}
}

// OutboundSnapshot 是出站安全与限制（sec.*，OPEN-ISSUES #56）与出站调用
// 策略（net.*，OPEN-ISSUES #57）的读视图：清单为空串 = 不限；
// ssrfProtection false = 不拦截；超时/重试 0 = 沿用调用方缺省。语义实现
// 在 internal/security/secguard（CheckURL 静态校验 + 拨号 Control 钩子 +
// DoWithRetry/Probe）。
type OutboundSnapshot struct {
	AllowPorts       string            `json:"allowPorts"`
	AllowIPs         string            `json:"allowIPs"`
	DomainFilter     string            `json:"domainFilter"`
	SSRFProtection   bool              `json:"ssrfProtection"`
	RequestTimeoutMs int               `json:"requestTimeoutMs"`
	MaxRetries       int               `json:"maxRetries"`
	RetryBackoffMs   int               `json:"retryBackoffMs"`
	Sources          map[string]string `json:"sources"`
}

// OutboundSnapshot resolves the outbound guard settings.
func (l *Layered) OutboundSnapshot() OutboundSnapshot {
	snap := OutboundSnapshot{Sources: map[string]string{}}
	for key, dst := range map[string]*string{
		KeySecAllowPorts:   &snap.AllowPorts,
		KeySecAllowIPs:     &snap.AllowIPs,
		KeySecDomainFilter: &snap.DomainFilter,
	} {
		v, src, ok := l.GetString(context.Background(), key)
		if ok {
			*dst = v
			snap.Sources[key] = src
		} else {
			snap.Sources[key] = "default"
		}
	}
	snap.SSRFProtection, snap.Sources[KeySecSSRFProtection], _ = l.getBoolWithSource(KeySecSSRFProtection, false)
	for key, dst := range map[string]*int{
		KeyNetRequestTimeoutMs: &snap.RequestTimeoutMs,
		KeyNetMaxRetries:       &snap.MaxRetries,
		KeyNetRetryBackoffMs:   &snap.RetryBackoffMs,
	} {
		v, src := l.getIntWithSource(key, 0)
		*dst = intSafe(v, 0)
		snap.Sources[key] = src
	}
	return snap
}

// PerformanceSettingsSnapshot 是性能参数（perf.*）的读视图：阈值/上限为
// 0 表示「不启用该限制」，来源逐键标注（default/config/yaml/database）。
type PerformanceSettingsSnapshot struct {
	MaxCpuPct      int               `json:"maxCpuPct"`
	MaxMemoryPct   int               `json:"maxMemoryPct"`
	MaxDiskPct     int               `json:"maxDiskPct"`
	MaxConcurrent  int               `json:"maxConcurrent"`
	MaxThreadCount int               `json:"maxThreadCount"`
	CacheSize      int64             `json:"cacheSize"`
	Sources        map[string]string `json:"sources"`
}

// intSafe 将 int64 配置值收窄到 int：超出 int32 可表示范围时回退默认值。
// 32 位平台（GOARCH=386/arm）上 int 为 32 位，直接 int(v) 会静默截断
// （CodeQL go/incorrect-integer-conversion）；统一按 int32 边界收口，
// 跨平台行为一致，配置值合理域远小于该边界。
func intSafe(v, def int64) int {
	if v < math.MinInt32 || v > math.MaxInt32 {
		return int(def)
	}
	return int(v)
}

// getIntWithSource 带来源读整型（database/config/default），层逻辑同 GetInt
// （含数值被存成字符串的容错）。GetString 只解字符串、GetInt 不带来源，
// perf.* 快照两者都要，故单独成 helper（同 getBoolWithSource 先例）。
func (l *Layered) getIntWithSource(key string, def int64) (int64, string) {
	if !IsValidKey(key) || l == nil {
		return def, "default"
	}
	l.mu.RLock()
	defer l.mu.RUnlock()
	read := func(raw json.RawMessage) (int64, bool) {
		var v int64
		if err := json.Unmarshal(raw, &v); err == nil {
			return v, true
		}
		var s string
		if err := json.Unmarshal(raw, &s); err == nil {
			if n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64); err == nil {
				return n, true
			}
		}
		return 0, false
	}
	if raw, ok := l.l3[key]; ok && l.l3Loaded {
		if v, ok := read(raw); ok {
			return v, "database"
		}
	}
	if raw, ok := l.l2Values[key]; ok {
		if v, ok := read(raw); ok {
			return v, "config"
		}
	}
	return def, "default"
}

// PerformanceSettings resolves the performance knobs (defaults all 0 = off).
func (l *Layered) PerformanceSettings() PerformanceSettingsSnapshot {
	snap := PerformanceSettingsSnapshot{Sources: map[string]string{}}
	cpuPct, cpuSrc := l.getIntWithSource(KeyPerfMaxCpuPct, 0)
	memPct, memSrc := l.getIntWithSource(KeyPerfMaxMemoryPct, 0)
	diskPct, diskSrc := l.getIntWithSource(KeyPerfMaxDiskPct, 0)
	maxConc, concSrc := l.getIntWithSource(KeyPerfMaxConcurrent, 0)
	maxThr, thrSrc := l.getIntWithSource(KeyPerfMaxThreadCount, 0)
	snap.MaxCpuPct, snap.Sources["maxCpuPct"] = intSafe(cpuPct, 0), cpuSrc
	snap.MaxMemoryPct, snap.Sources["maxMemoryPct"] = intSafe(memPct, 0), memSrc
	snap.MaxDiskPct, snap.Sources["maxDiskPct"] = intSafe(diskPct, 0), diskSrc
	snap.MaxConcurrent, snap.Sources["maxConcurrent"] = intSafe(maxConc, 0), concSrc
	snap.MaxThreadCount, snap.Sources["maxThreadCount"] = intSafe(maxThr, 0), thrSrc
	snap.CacheSize, snap.Sources["cacheSize"] = l.getIntWithSource(KeyPerfCacheSize, 0)
	return snap
}

// LogsSettingsSnapshot 日志维护 L3 快照（OPEN-ISSUES #54）。
type LogsSettingsSnapshot struct {
	// RetentionDays log.retentionDays 覆盖值；0 = 跟随配置文件
	// （executionLog.retentionDays / taskLog.retentionDays 各自生效，缺省 7）。
	RetentionDays int
	Sources       map[string]string
}

// LogsSettings resolves the log maintenance knobs.
func (l *Layered) LogsSettings() LogsSettingsSnapshot {
	snap := LogsSettingsSnapshot{Sources: map[string]string{}}
	days, src := l.getIntWithSource(KeyLogRetentionDays, 0)
	snap.RetentionDays, snap.Sources["retentionDays"] = intSafe(days, 0), src
	return snap
}

func (l *Layered) authProviderSnapshot(kind string) AuthProviderSnapshot {
	ctx := context.Background()
	prefix := "auth." + kind + "."
	snap := AuthProviderSnapshot{Fields: map[string]string{}, Sources: map[string]string{}}
	snap.Enabled = l.GetBool(settingsKey(prefix+"enabled"), false)
	for _, f := range []string{"addr", "baseDn", "bindDn", "userFilter", "issuer", "clientId", "redirectUrl", "defaultRoles", "startTls", "successUrl", "appId", "authUrl", "tokenUrl", "userInfoUrl", "scopes", "usernameField", "nicknameField", "emailField"} {
		if v, src, ok := l.GetString(ctx, settingsKey(prefix+f)); ok && v != "" {
			snap.Fields[f] = v
			snap.Sources[f] = src
		}
	}
	// startTls 是 bool，放进 Fields 的字符串视图
	if l.GetBool(settingsKey(prefix+"startTls"), false) {
		snap.Fields["startTls"] = "true"
	}
	var secretKey string
	switch kind {
	case "ldap":
		secretKey = KeyAuthLdapBindPassword
	case "github":
		secretKey = KeyAuthGitHubClientSecret
	case "wechat":
		secretKey = KeyAuthWeChatAppSecret
	case "genericoauth":
		secretKey = KeyAuthGenericOAuthClientSecret
	default:
		secretKey = KeyAuthOidcClientSecret
	}
	if v, src, ok := l.GetString(ctx, secretKey); ok && v != "" {
		snap.SecretSet = true
		snap.Sources["secret"] = src
		if len(v) > 4 {
			snap.SecretMasked = "****" + v[len(v)-4:]
		} else {
			snap.SecretMasked = "****"
		}
	}
	return snap
}

func settingsKey(k string) string { return k }

// AuthProviderConfig 从分层配置解析出当前生效的外部身份源配置
// （L3 DB 覆盖 yaml；供登录方式热刷新与 Test Connection 使用）。
func (l *Layered) AuthProviderConfig() config.AuthProvidersConfig {
	ctx := context.Background()
	str := func(key string) string {
		v, _, ok := l.GetString(ctx, key)
		if !ok {
			return ""
		}
		return v
	}
	roles := func(key string) []string {
		raw := str(key)
		if raw == "" {
			return nil
		}
		parts := strings.Split(raw, ",")
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			if p = strings.TrimSpace(p); p != "" {
				out = append(out, p)
			}
		}
		return out
	}
	return config.AuthProvidersConfig{
		Local: config.LocalProviderConfig{
			// 默认启用（L3 未覆盖时 true）；显式 false 才停用账号密码登录
			Enabled: boolPtr(l.GetBool(KeyAuthLocalEnabled, true)),
		},
		LDAP: config.LDAPProviderConfig{
			Enabled:        l.GetBool(KeyAuthLdapEnabled, false),
			Addr:           str(KeyAuthLdapAddr),
			BaseDN:         str(KeyAuthLdapBaseDn),
			BindDN:         str(KeyAuthLdapBindDn),
			BindPassword:   str(KeyAuthLdapBindPassword),
			UserFilter:     str(KeyAuthLdapUserFilter),
			StartTLS:       l.GetBool(KeyAuthLdapStartTLS, false),
			UserDNTemplate: str(KeyAuthLdapUserFilter), // 兼容：filter 未配时回退模板语义由 build 侧处理
			DefaultRoles:   roles(KeyAuthLdapDefaultRoles),
		},
		OIDC: config.OIDCProviderConfig{
			Enabled:      l.GetBool(KeyAuthOidcEnabled, false),
			Issuer:       str(KeyAuthOidcIssuer),
			ClientID:     str(KeyAuthOidcClientId),
			ClientSecret: str(KeyAuthOidcClientSecret),
			RedirectURL:  str(KeyAuthOidcRedirectUrl),
			DefaultRoles: roles(KeyAuthOidcDefaultRoles),
		},
		GitHub: config.GitHubProviderConfig{
			Enabled:         l.GetBool(KeyAuthGitHubEnabled, false),
			ClientID:        str(KeyAuthGitHubClientId),
			ClientSecret:    str(KeyAuthGitHubClientSecret),
			RedirectURL:     str(KeyAuthGitHubRedirectUrl),
			DefaultRoles:    roles(KeyAuthGitHubDefaultRoles),
			LoginSuccessURL: str(KeyAuthGitHubSuccessURL),
		},
		WeChat: config.WeChatProviderConfig{
			Enabled:         l.GetBool(KeyAuthWeChatEnabled, false),
			AppID:           str(KeyAuthWeChatAppId),
			AppSecret:       str(KeyAuthWeChatAppSecret),
			RedirectURL:     str(KeyAuthWeChatRedirectUrl),
			DefaultRoles:    roles(KeyAuthWeChatDefaultRoles),
			LoginSuccessURL: str(KeyAuthWeChatSuccessURL),
		},
		GenericOAuth: config.GenericOAuthProviderConfig{
			Enabled:       l.GetBool(KeyAuthGenericOAuthEnabled, false),
			ClientID:      str(KeyAuthGenericOAuthClientId),
			ClientSecret:  str(KeyAuthGenericOAuthClientSecret),
			RedirectURL:   str(KeyAuthGenericOAuthRedirectUrl),
			AuthURL:       str(KeyAuthGenericOAuthAuthUrl),
			TokenURL:      str(KeyAuthGenericOAuthTokenUrl),
			UserInfoURL:   str(KeyAuthGenericOAuthUserInfoUrl),
			Scopes:        str(KeyAuthGenericOAuthScopes),
			UsernameField: str(KeyAuthGenericOAuthUsernameField),
			NicknameField: str(KeyAuthGenericOAuthNicknameField),
			EmailField:    str(KeyAuthGenericOAuthEmailField),
			DefaultRoles:  roles(KeyAuthGenericOAuthDefaultRoles),
		},
		Register: config.RegisterConfig{
			Enabled:      l.GetBool(KeyAuthRegisterEnabled, false),
			DefaultRoles: roles(KeyAuthRegisterDefaultRoles),
		},
	}
}

func boolPtr(v bool) *bool { return &v }
