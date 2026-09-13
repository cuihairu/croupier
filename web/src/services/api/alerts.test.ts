import { request } from '@umijs/max';
import { deleteAlertSilence, listAlertSilences, listAlerts, silenceAlert } from './alerts';

jest.mock('@umijs/max', () => ({ request: jest.fn() }));

const mockedRequest = request as jest.MockedFunction<typeof request>;
// tests/setupTests.jsx 把 window/global.localStorage 换成了 jest.fn() mock
const mockedGetItem = jest.mocked(localStorage.getItem);

// 表格驱动：[名称, 调用, 期望 URL, 期望 options（不含 Authorization）]
type Case = {
  name: string;
  call: () => Promise<unknown>;
  url: string;
  options: Record<string, unknown>;
};

const cases: Case[] = [
  {
    name: 'listAlerts with filters',
    call: () => listAlerts({ page: 2, pageSize: 20, level: 'critical', status: 'firing' }),
    url: '/api/v1/alerts',
    options: {
      method: 'GET',
      params: { page: 2, pageSize: 20, level: 'critical', status: 'firing' },
    },
  },
  {
    name: 'listAlerts without filters',
    call: () => listAlerts(),
    url: '/api/v1/alerts',
    options: { method: 'GET', params: undefined },
  },
  {
    name: 'silenceAlert with reason',
    call: () => silenceAlert('al-1', 300, '误报'),
    url: '/api/v1/alerts/al-1/silence',
    options: { method: 'POST', data: { duration: 300, reason: '误报' } },
  },
  {
    name: 'silenceAlert without reason',
    call: () => silenceAlert('al-1', 600),
    url: '/api/v1/alerts/al-1/silence',
    options: { method: 'POST', data: { duration: 600, reason: undefined } },
  },
  {
    name: 'listAlertSilences',
    call: () => listAlertSilences(),
    url: '/api/v1/alerts/silences',
    options: { method: 'GET' },
  },
  {
    name: 'deleteAlertSilence',
    call: () => deleteAlertSilence('sil-1'),
    url: '/api/v1/alerts/silences/sil-1',
    options: { method: 'DELETE' },
  },
];

describe('alerts API adapters', () => {
  beforeEach(() => {
    mockedRequest.mockReset().mockResolvedValue(undefined);
    mockedGetItem.mockReset().mockReturnValue('tok-abc');
  });

  it.each(cases)(
    '$name attaches the bearer token from localStorage',
    async ({ call, url, options }) => {
      await call();

      expect(mockedRequest).toHaveBeenCalledTimes(1);
      const [calledUrl, calledOptions] = mockedRequest.mock.calls[0];
      expect(calledUrl).toBe(url);
      expect(calledOptions).toEqual({
        ...options,
        headers: { Authorization: 'Bearer tok-abc' },
      });
    },
  );

  it.each(cases)(
    '$name omits the Authorization header without a stored token',
    async ({ call, url, options }) => {
      mockedGetItem.mockReturnValue(null);

      await call();

      const [calledUrl, calledOptions] = mockedRequest.mock.calls[0];
      expect(calledUrl).toBe(url);
      expect(calledOptions).toEqual({ ...options, headers: undefined });
    },
  );

  it('returns the alerts list response untouched', async () => {
    mockedRequest.mockResolvedValue({
      items: [
        {
          id: 'al-1',
          type: 'player.report',
          level: 'critical',
          message: 'boom',
          source: 'rule',
          status: 'firing',
          createdAt: '2026-09-01T00:00:00Z',
        },
      ],
      total: 1,
      page: 1,
      pageSize: 20,
    });

    const resp = await listAlerts();

    expect(resp.total).toBe(1);
    expect(resp.items[0].id).toBe('al-1');
    // jsdom 下 window 恒定义，SSR 防御分支（typeof window === 'undefined' → ''）
    // 在测试环境不可达，此处断言 token 路径已生效
    expect(mockedGetItem).toHaveBeenCalledWith('token');
  });
});
