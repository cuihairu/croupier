import { request } from '@umijs/max';
import { normalizeFunctionSummary } from './functions-enhanced';
import { getFunctionInstances } from './functions-enhanced';

jest.mock('@umijs/max', () => ({ request: jest.fn() }));

const mockedRequest = request as jest.MockedFunction<typeof request>;

describe('normalizeFunctionSummary', () => {
  it('normalizes backend locale keys used by FunctionSpec summaries', () => {
    expect(
      normalizeFunctionSummary({
        id: 'player.list',
        status: 1,
        displayName: { 'zh-CN': '玩家列表', 'en-US': 'Player List' },
        summary: { 'zh-CN': '查询玩家', 'en-US': 'List players' },
      }),
    ).toMatchObject({
      id: 'player.list',
      enabled: true,
      displayName: { 'zh-CN': '玩家列表', 'en-US': 'Player List' },
      summary: { 'zh-CN': '查询玩家', 'en-US': 'List players' },
    });
  });

  it('does not claim an enabled function when status is disabled', () => {
    expect(normalizeFunctionSummary({ id: 'player.delete', status: 0 }).enabled).toBe(false);
  });
});

describe('getFunctionInstances', () => {
  beforeEach(() => mockedRequest.mockReset());

  it('encodes function ids and normalizes the backend envelope', async () => {
    mockedRequest.mockResolvedValue({
      items: [
        {
          functionId: 'player/read',
          agentId: 'agent-1',
          serviceId: 'service-1',
          status: 'active',
        },
      ],
      total: 1,
    });

    await expect(getFunctionInstances({ functionId: 'player/read' })).resolves.toEqual({
      instances: [
        expect.objectContaining({
          functionId: 'player/read',
          agentId: 'agent-1',
          serviceId: 'service-1',
          status: 'running',
        }),
      ],
      total: 1,
    });
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/functions/player%2Fread/instances', {
      params: { gameId: undefined },
    });
  });

  it('propagates scope or permission errors instead of returning fake empty data', async () => {
    mockedRequest.mockRejectedValue(new Error('scope_required'));

    await expect(getFunctionInstances()).rejects.toThrow('scope_required');
  });
});
