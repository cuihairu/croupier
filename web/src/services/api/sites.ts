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

// Admin: write one L3 override (value is a JSON string for string keys).
export async function setSiteSetting(key: string, value: unknown): Promise<void> {
  await request(`/api/v1/site/${encodeURIComponent(key)}`, {
    method: 'PUT',
    data: { value },
  });
}

// Admin: clear the L3 override = follow the config file again.
export async function clearSiteSetting(key: string): Promise<void> {
  await request(`/api/v1/site/${encodeURIComponent(key)}`, { method: 'DELETE' });
}
