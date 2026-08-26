import { request } from '@umijs/max';

// Source: internal/platform/settings/layered.go SiteSnapshot
export type SiteConfig = {
  siteName: string;
  logoUrl?: string;
  faviconUrl?: string;
  description?: string;
  footerCopyright?: string;
  footerIcp?: string;
  footerLinks?: Array<{ key: string; title: string; url: string }>;
  defaultLocale?: string;
};

// Public snapshot (login page and pre-auth also need it).
export async function fetchSiteConfig(): Promise<SiteConfig> {
  return request<SiteConfig>('/api/v1/public/site', { skipErrorHandler: true });
}

export type SettingSource = 'default' | 'config' | 'database';

export type PlatformSettings = SiteConfig & {
  sources?: Record<string, SettingSource>;
};

// Admin: read the raw snapshot with per-key provenance.
export async function fetchPlatformSettings(): Promise<PlatformSettings> {
  return request<PlatformSettings>('/api/v1/public/site', { skipErrorHandler: true });
}

// Admin: write one L3 override. Value is the JSON value for the key:
// string for site.*/obs.*, boolean for features.*, array for footer.links.
export async function setSiteSetting(key: string, value: unknown): Promise<void> {
  await request(`/api/v1/site/${encodeURIComponent(key)}`, {
    method: 'PUT',
    data: { value },
  });
}

// Admin: clear the L3 override = follow the config file again.
export async function clearSiteSetting(key: string): Promise<void> {
  await request(`/api/v1/site/${encodeURIComponent(key)}`, {
    method: 'DELETE',
  });
}

// ---- 功能开关（features.*，L3 运行时软开关） ----

export type FeatureDomain = 'dev' | 'support' | 'analytics' | 'ops' | 'extensions';

export type FeatureDomainState = {
  /** 合成值（L2 ∧ L3） */
  enabled: boolean;
  /** 部署配置（L2）物理裁剪：路由未注册，L3 无法开启 */
  trimmedByConfig: boolean;
  /** 存在 L3 数据库覆盖 */
  overridden: boolean;
};

export type FeatureSnapshot = {
  domains: Record<FeatureDomain, FeatureDomainState>;
};

// Admin: per-domain feature state (composed L2∧L3, trim info, override presence).
export async function fetchFeatureSettings(): Promise<FeatureSnapshot> {
  return request<FeatureSnapshot>('/api/v1/site/features', { skipErrorHandler: true });
}

// ---- 观测集成（obs.*） ----

export type ObservabilitySettings = {
  alertmanagerUrl: string;
  grafanaExploreUrl: string;
  jaegerUrl: string;
  sources: Record<string, SettingSource>;
};

// Admin: observability integration URLs with provenance.
export async function fetchObservabilitySettings(): Promise<ObservabilitySettings> {
  return request<ObservabilitySettings>('/api/v1/site/observability', {
    skipErrorHandler: true,
  });
}
