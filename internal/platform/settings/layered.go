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
	KeyNotifyEmailEnabled   = "notification.emailEnabled"   // bool
	KeyNotifySMTPHost       = "notification.smtpHost"       // string
	KeyNotifySMTPPort       = "notification.smtpPort"       // int
	KeyNotifySMTPUser       = "notification.smtpUser"       // string
	KeyNotifySMTPPassword   = "notification.smtpPassword"   // string（写入后读取接口脱敏）
	KeyNotifySMTPFrom       = "notification.smtpFrom"       // string
	KeyNotifyDingtalkURL    = "notification.dingtalkUrl"    // string
	KeyNotifyDingtalkSecret = "notification.dingtalkSecret" // string
	KeyNotifyWebhookURL     = "notification.webhookUrl"     // string
	KeyNotifyWebhookSecret  = "notification.webhookSecret"  // string
	KeyNotifyWecomURL       = "notification.wecomUrl"       // string
	KeyNotifyFeishuURL      = "notification.feishuUrl"      // string
	KeyNotifyFeishuSecret   = "notification.feishuSecret"   // string
	KeyNotifyInAppEnabled   = "notification.inAppEnabled"   // bool

	// 登录方式（外部身份源，L3 运行时配置——Harbor 模式：yaml 仅作
	// bootstrap 初始值，UI 配置热生效；凭据键脱敏回显）
	KeyAuthLdapEnabled      = "auth.ldap.enabled"      // bool
	KeyAuthLdapAddr         = "auth.ldap.addr"         // string
	KeyAuthLdapBaseDn       = "auth.ldap.baseDn"       // string
	KeyAuthLdapBindDn       = "auth.ldap.bindDn"       // string
	KeyAuthLdapBindPassword = "auth.ldap.bindPassword" // string (secret)
	KeyAuthLdapUserFilter   = "auth.ldap.userFilter"   // string
	KeyAuthLdapStartTLS     = "auth.ldap.startTls"     // bool
	KeyAuthLdapDefaultRoles = "auth.ldap.defaultRoles" // string (逗号分隔)
	KeyAuthOidcEnabled      = "auth.oidc.enabled"      // bool
	KeyAuthOidcIssuer       = "auth.oidc.issuer"       // string
	KeyAuthOidcClientId     = "auth.oidc.clientId"     // string
	KeyAuthOidcClientSecret = "auth.oidc.clientSecret" // string (secret)
	KeyAuthOidcRedirectUrl  = "auth.oidc.redirectUrl"  // string
	KeyAuthOidcDefaultRoles = "auth.oidc.defaultRoles" // string (逗号分隔)
)

// ValidKeys is the L3 whitelist.
var ValidKeys = map[string]struct{}{
	KeySiteName: {}, KeySiteLogoURL: {}, KeySiteFaviconURL: {},
	KeySiteDescription: {}, KeyFooterCopyright: {}, KeyFooterICP: {},
	KeyFooterLinks: {}, KeyDefaultLocale: {},

	KeyFeatureDev: {}, KeyFeatureSupport: {}, KeyFeatureAnalytics: {},
	KeyFeatureOps: {}, KeyFeatureExtensions: {},

	KeyObsAlertmanagerURL: {}, KeyObsGrafanaExploreURL: {}, KeyObsJaegerURL: {},

	KeyNotifyEmailEnabled: {}, KeyNotifySMTPHost: {}, KeyNotifySMTPPort: {},
	KeyNotifySMTPUser: {}, KeyNotifySMTPPassword: {}, KeyNotifySMTPFrom: {},
	KeyNotifyDingtalkURL: {}, KeyNotifyDingtalkSecret: {},
	KeyNotifyWebhookURL: {}, KeyNotifyWebhookSecret: {}, KeyNotifyInAppEnabled: {},
	KeyNotifyWecomURL: {}, KeyNotifyFeishuURL: {}, KeyNotifyFeishuSecret: {},

	KeyAuthLdapEnabled: {}, KeyAuthLdapAddr: {}, KeyAuthLdapBaseDn: {},
	KeyAuthLdapBindDn: {}, KeyAuthLdapBindPassword: {}, KeyAuthLdapUserFilter: {},
	KeyAuthLdapStartTLS: {}, KeyAuthLdapDefaultRoles: {},
	KeyAuthOidcEnabled: {}, KeyAuthOidcIssuer: {}, KeyAuthOidcClientId: {},
	KeyAuthOidcClientSecret: {}, KeyAuthOidcRedirectUrl: {}, KeyAuthOidcDefaultRoles: {},
}

// secretKeys 是读取时必须脱敏的 key（读取接口只回显尾 4 位）。
var secretKeys = map[string]struct{}{
	KeyNotifySMTPPassword:   {},
	KeyNotifyDingtalkSecret: {},
	KeyNotifyWebhookSecret:  {},
	KeyAuthLdapBindPassword: {},
	KeyAuthOidcClientSecret: {},
}

// IsSecretKey reports whether the key holds a credential that must be masked
// on read.
func IsSecretKey(key string) bool {
	_, ok := secretKeys[key]
	return ok
}

// intKeys 是整数语义的 key。
var intKeys = map[string]struct{}{
	KeyNotifySMTPPort: {},
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
	KeyNotifyEmailEnabled: {}, KeyNotifyInAppEnabled: {},
	KeyAuthLdapEnabled: {}, KeyAuthLdapStartTLS: {}, KeyAuthOidcEnabled: {},
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

type source struct {
	name   string // diagnostics: "default"|"config"|"database"
	values map[string]json.RawMessage
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
	AlertmanagerURL   string
	GrafanaExploreURL string
	JaegerURL         string
	Sources           map[string]string
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
	EmailEnabled         bool              `json:"emailEnabled"`
	SMTPHost             string            `json:"smtpHost"`
	SMTPPort             int               `json:"smtpPort"`
	SMTPUser             string            `json:"smtpUser"`
	SMTPFrom             string            `json:"smtpFrom"`
	SMTPPasswordSet      bool              `json:"smtpPasswordSet"`
	SMTPPasswordMasked   string            `json:"smtpPasswordMasked,omitempty"`
	DingtalkURL          string            `json:"dingtalkUrl"`
	DingtalkSecretSet    bool              `json:"dingtalkSecretSet"`
	DingtalkSecretMasked string            `json:"dingtalkSecretMasked,omitempty"`
	WebhookURL           string            `json:"webhookUrl"`
	WebhookSecretSet     bool              `json:"webhookSecretSet"`
	WebhookSecretMasked  string            `json:"webhookSecretMasked,omitempty"`
	WecomURL             string            `json:"wecomUrl"`
	FeishuURL            string            `json:"feishuUrl"`
	FeishuSecretSet      bool              `json:"feishuSecretSet"`
	FeishuSecretMasked   string            `json:"feishuSecretMasked,omitempty"`
	InAppEnabled         bool              `json:"inAppEnabled"`
	Sources              map[string]string `json:"sources"`
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
}

// NotifySMTP resolves the raw SMTP configuration (unmasked, internal use).
func (l *Layered) NotifySMTP() NotifySMTPConfig {
	return NotifySMTPConfig{
		Enabled:  l.GetBool(KeyNotifyEmailEnabled, false),
		Host:     stringOrEmpty(l.GetString(context.Background(), KeyNotifySMTPHost)),
		Port:     l.GetInt(KeyNotifySMTPPort, 0),
		User:     stringOrEmpty(l.GetString(context.Background(), KeyNotifySMTPUser)),
		Password: stringOrEmpty(l.GetString(context.Background(), KeyNotifySMTPPassword)),
		From:     stringOrEmpty(l.GetString(context.Background(), KeyNotifySMTPFrom)),
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
	LDAP AuthProviderSnapshot `json:"ldap"`
	OIDC AuthProviderSnapshot `json:"oidc"`
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
	return AuthSnapshot{
		LDAP: l.authProviderSnapshot("ldap"),
		OIDC: l.authProviderSnapshot("oidc"),
	}
}

func (l *Layered) authProviderSnapshot(kind string) AuthProviderSnapshot {
	ctx := context.Background()
	prefix := "auth." + kind + "."
	snap := AuthProviderSnapshot{Fields: map[string]string{}, Sources: map[string]string{}}
	snap.Enabled = l.GetBool(settingsKey(prefix+"enabled"), false)
	for _, f := range []string{"addr", "baseDn", "bindDn", "userFilter", "issuer", "clientId", "redirectUrl", "defaultRoles", "startTls"} {
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
	if kind == "ldap" {
		secretKey = KeyAuthLdapBindPassword
	} else {
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
	}
}
