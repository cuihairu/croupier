package settings

import (
	"context"
	"encoding/json"
)

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
		Password:           l.secretStr(context.Background(), KeyNotifySMTPPassword),
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
		DingtalkSecret: l.secretStr(context.Background(), KeyNotifyDingtalkSecret),
		WebhookURL:     stringOrEmpty(l.GetString(context.Background(), KeyNotifyWebhookURL)),
		WebhookSecret:  l.secretStr(context.Background(), KeyNotifyWebhookSecret),
		WecomURL:       stringOrEmpty(l.GetString(context.Background(), KeyNotifyWecomURL)),
		FeishuURL:      stringOrEmpty(l.GetString(context.Background(), KeyNotifyFeishuURL)),
		FeishuSecret:   l.secretStr(context.Background(), KeyNotifyFeishuSecret),
		InAppEnabled:   l.GetBool(KeyNotifyInAppEnabled, true),
		EmailEnabled:   l.GetBool(KeyNotifyEmailEnabled, false),
	}
}

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
