import { request } from '@umijs/max';
import { getExecutionLog, listExecutionLogs } from './executionLogs';

jest.mock('@umijs/max', () => ({ request: jest.fn() }));

const mockedRequest = request as jest.MockedFunction<typeof request>;

describe('execution log API adapters', () => {
  beforeEach(() => mockedRequest.mockReset().mockResolvedValue(undefined));

  it('lists execution logs with query params on the trailing-slash URL', async () => {
    const body = {
      items: [
        {
          id: 1,
          gameId: 'demo',
          env: 'prod',
          source: 'invoke',
          functionId: 'player.ban',
          actor: 'admin',
          status: 'succeeded',
          durationMs: 12,
          createdAt: '2026-09-01T00:00:00Z',
        },
      ],
      total: 1,
      page: 1,
      size: 20,
    };
    mockedRequest.mockResolvedValue(body);

    const resp = await listExecutionLogs({
      page: 1,
      pageSize: 20,
      status: 'succeeded',
      mine: true,
      gameId: undefined,
    });

    expect(resp).toEqual(body);
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/execution-logs/', {
      params: { page: 1, pageSize: 20, status: 'succeeded', mine: true, gameId: undefined },
    });
  });

  it('fetches one log detail (masked payloads included) by id', async () => {
    const detail = {
      id: 7,
      gameId: 'demo',
      env: 'prod',
      source: 'page',
      functionId: 'player.query',
      pageKey: 'players',
      bindingId: 'list',
      actor: 'admin',
      route: 'lb',
      status: 'succeeded',
      durationMs: 34,
      traceId: 'tr-1',
      truncated: true,
      createdAt: '2026-09-01T00:00:00Z',
      requestPayload: { playerId: 'p-1' },
      responseBody: { total: 1 },
    };
    mockedRequest.mockResolvedValue(detail);

    const resp = await getExecutionLog(7);

    expect(resp).toEqual(detail);
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/execution-logs/7');
  });
});
