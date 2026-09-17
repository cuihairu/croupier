import { request } from '@umijs/max';
import { deleteAlertSilence, listAlertSilences, listAlerts, silenceAlert } from './alerts';

jest.mock('@umijs/max', () => ({ request: jest.fn() }));

const mockedRequest = request as jest.MockedFunction<typeof request>;

// Authorization 由 requestErrorConfig 的请求拦截器统一注入
// （含无 token 时的省略），适配层只负责 URL/method/参数形状。
// 表格驱动：[名称, 调用, 期望 URL, 期望 options]
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
  });

  it.each(cases)('$name hits the right URL, method and payload', async ({ call, url, options }) => {
    await call();

    expect(mockedRequest).toHaveBeenCalledTimes(1);
    const [calledUrl, calledOptions] = mockedRequest.mock.calls[0];
    expect(calledUrl).toBe(url);
    expect(calledOptions).toEqual(options);
    // Authorization 由 requestErrorConfig 拦截器统一注入，适配层不携带 headers
    expect(calledOptions).not.toHaveProperty('headers');
  });

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
  });
});
