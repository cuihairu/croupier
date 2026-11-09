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
	"log/slog"
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

	// 审批 step-up TOTP 总开关（OPEN-ISSUES #61）：high/danger 审批是否强制
	// 二次验证。默认 true（fail-safe，与 #75 落地语义一致）；关闭后高危审批
	// 降级为中低风险语义（otp 可选、提供了就校验），审计记 disabled 旁路档。
	// 移动端定位是便利，强制门槛必须可由服务端一键降级。
	KeySecurityApprovalStepUpOTP = "security.approvalStepUpOtp" // bool

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
	KeySecurityPasswordMaxAgeDays: {}, KeySecurityApprovalStepUpOTP: {},

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
	KeySecurityPasswordRequireSpecial: {}, KeySecurityApprovalStepUpOTP: {},
	KeySecSSRFProtection:             {},
	KeyAuthEmailAliasRestriction:     {},
	KeyAuthEmailVerificationRequired: {},
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
