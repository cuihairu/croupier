import { request } from '@umijs/max';
import {
  clearSiteSetting,
  fetchAuthSnapshot,
  fetchFeatureSettings,
  fetchLoginProviders,
  fetchNotificationSettings,
  fetchObservabilitySettings,
  fetchPlatformSettings,
  fetchSiteConfig,
  setSiteSetting,
  testAuthConnection,
} from './sites';

jest.mock('@umijs/max', () => ({ request: jest.fn() }));

const mockedRequest = request as jest.MockedFunction<typeof request>;

// 表格驱动：[名称, 调用, 期望 URL, 期望 options]
type Case = {
  name: string;
  call: () => Promise<unknown>;
  url: string;
  options: Record<string, unknown>;
};

const cases: Case[] = [
  {
    name: 'fetchSiteConfig (public snapshot)',
    call: () => fetchSiteConfig(),
    url: '/api/v1/public/site',
    options: { skipErrorHandler: true },
  },
  {
    name: 'fetchPlatformSettings (raw snapshot with provenance)',
    call: () => fetchPlatformSettings(),
    url: '/api/v1/public/site',
    options: { skipErrorHandler: true },
  },
  {
    name: 'setSiteSetting writes one L3 override',
    call: () => setSiteSetting('site.name', 'GM 后台'),
    url: '/api/v1/site/site.name',
    options: { method: 'PUT', data: { value: 'GM 后台' } },
  },
  {
    name: 'setSiteSetting URL-encodes keys with reserved chars',
    call: () => setSiteSetting('features/dev', true),
    url: '/api/v1/site/features%2Fdev',
    options: { method: 'PUT', data: { value: true } },
  },
  {
    name: 'setSiteSetting accepts array values (footer.links)',
    call: () => setSiteSetting('footer.links', [{ key: 'docs', title: '文档', url: 'https://x' }]),
    url: '/api/v1/site/footer.links',
    options: {
      method: 'PUT',
      data: { value: [{ key: 'docs', title: '文档', url: 'https://x' }] },
    },
  },
  {
    name: 'clearSiteSetting removes the L3 override',
    call: () => clearSiteSetting('site.logoUrl'),
    url: '/api/v1/site/site.logoUrl',
    options: { method: 'DELETE' },
  },
  {
    name: 'fetchFeatureSettings (L2∧L3 composed snapshot)',
    call: () => fetchFeatureSettings(),
    url: '/api/v1/site/features',
    options: { skipErrorHandler: true },
  },
  {
    name: 'fetchObservabilitySettings',
    call: () => fetchObservabilitySettings(),
    url: '/api/v1/site/observability',
    options: { skipErrorHandler: true },
  },
  {
    name: 'fetchNotificationSettings',
    call: () => fetchNotificationSettings(),
    url: '/api/v1/site/notification',
    options: { skipErrorHandler: true },
  },
  {
    name: 'fetchAuthSnapshot (masked credentials)',
    call: () => fetchAuthSnapshot(),
    url: '/api/v1/site/auth',
    options: { skipErrorHandler: true },
  },
  {
    name: 'testAuthConnection for ldap',
    call: () => testAuthConnection('ldap'),
    url: '/api/v1/site/auth/test',
    options: { method: 'POST', data: { kind: 'ldap' } },
  },
  {
    name: 'testAuthConnection for oidc',
    call: () => testAuthConnection('oidc'),
    url: '/api/v1/site/auth/test',
    options: { method: 'POST', data: { kind: 'oidc' } },
  },
  {
    name: 'fetchLoginProviders (public, pre-auth)',
    call: () => fetchLoginProviders(),
    url: '/api/v1/auth/providers',
    options: { skipErrorHandler: true },
  },
];

describe('site settings & auth provider API adapters', () => {
  beforeEach(() => mockedRequest.mockReset().mockResolvedValue(undefined));

  it.each(cases)('$name hits the right URL and options', async ({ call, url, options }) => {
    await call();

    expect(mockedRequest).toHaveBeenCalledTimes(1);
    const [calledUrl, calledOptions] = mockedRequest.mock.calls[0];
    expect(calledUrl).toBe(url);
    expect(calledOptions).toEqual(options);
  });

  it('returns the public site snapshot untouched', async () => {
    const snapshot = {
      siteName: 'GM 后台',
      logoUrl: '/logo.png',
      footerLinks: [{ key: 'docs', title: '文档', url: 'https://x' }],
      defaultLocale: 'zh-CN',
    };
    mockedRequest.mockResolvedValue(snapshot);

    await expect(fetchSiteConfig()).resolves.toEqual(snapshot);
  });

  it('returns the feature snapshot with per-domain state untouched', async () => {
    const snapshot = {
      domains: {
        dev: { enabled: true, trimmedByConfig: false, overridden: true },
        support: { enabled: false, trimmedByConfig: true, overridden: false },
        analytics: { enabled: true, trimmedByConfig: false, overridden: false },
        ops: { enabled: true, trimmedByConfig: false, overridden: false },
        extensions: { enabled: false, trimmedByConfig: false, overridden: true },
      },
    };
    mockedRequest.mockResolvedValue(snapshot);

    await expect(fetchFeatureSettings()).resolves.toEqual(snapshot);
  });
});
