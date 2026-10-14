import { request } from '@umijs/max';
import { getOpsProbeSummary, type ProbeSummaryResult } from './opsProbe';

jest.mock('@umijs/max', () => ({ request: jest.fn() }));

const mockedRequest = request as jest.MockedFunction<typeof request>;

// Authorization 与顶栏 game/env scope 头都走全局请求拦截器，适配层只负责 URL。
describe('opsProbe service', () => {
  beforeEach(() => {
    mockedRequest.mockReset();
  });

  it('getOpsProbeSummary: GET /api/v1/ops/probe/summary', async () => {
    const payload: ProbeSummaryResult = {
      onlineDataIntegrated: false,
      rows: [
        {
          agentId: 'agent-a',
          target: 'redis',
          semantics: 'readiness',
          gameId: 'demo',
          env: 'prod',
          available: false,
          windowOpen: true,
          windowDurationMs: 4200,
          checked: 10,
          failed: 2,
          successRate: 0.8,
          lastReason: 'dial tcp: i/o timeout',
          lastReportAt: '2026-10-14T12:00:00Z',
        },
      ],
    };
    mockedRequest.mockResolvedValueOnce(payload);

    await expect(getOpsProbeSummary()).resolves.toEqual(payload);
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/ops/probe/summary', {
      skipErrorHandler: true,
    });
  });

  it('getOpsProbeSummary: 在线趋势未接入时仍回空行集（灰态一等状态）', async () => {
    mockedRequest.mockResolvedValueOnce({ onlineDataIntegrated: false, rows: [] });
    await expect(getOpsProbeSummary()).resolves.toEqual({
      onlineDataIntegrated: false,
      rows: [],
    });
  });
});
