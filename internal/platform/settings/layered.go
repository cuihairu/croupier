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
)

// ValidKeys is the L3 whitelist.
var ValidKeys = map[string]struct{}{
	KeySiteName: {}, KeySiteLogoURL: {}, KeySiteFaviconURL: {},
	KeySiteDescription: {}, KeyFooterCopyright: {}, KeyFooterICP: {},
	KeyFooterLinks: {}, KeyDefaultLocale: {},

	KeyFeatureDev: {}, KeyFeatureSupport: {}, KeyFeatureAnalytics: {},
	KeyFeatureOps: {}, KeyFeatureExtensions: {},

	KeyObsAlertmanagerURL: {}, KeyObsGrafanaExploreURL: {}, KeyObsJaegerURL: {},
}

// boolKeys 是布尔语义的 key（PutKey 校验 + GetBool 读取）。
var boolKeys = map[string]struct{}{
	KeyFeatureDev: {}, KeyFeatureSupport: {}, KeyFeatureAnalytics: {},
	KeyFeatureOps: {}, KeyFeatureExtensions: {},
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
