import { request } from '@umijs/max';
import { fetchRegistry } from './registry';

jest.mock('@umijs/max', () => ({ request: jest.fn() }));

const mockedRequest = request as jest.MockedFunction<typeof request>;

describe('registry API adapter', () => {
  beforeEach(() => mockedRequest.mockReset());

  it('fetches the registry view as-is (no client-side normalization)', async () => {
    const payload = {
      agents: [
        {
          agentId: 'a-1',
          gameId: 'demo',
          env: 'prod',
          addr: '10.0.0.2:19091',
          functions: 3,
          healthy: true,
          expiresInSec: 30,
        },
      ],
      functions: [{ gameId: 'demo', id: 'player.ban', agents: ['a-1'] }],
      coverage: [
        {
          gameEnv: 'demo/prod',
          functions: { 'player.ban': { total: 2, healthy: 1 } },
          uncovered: ['mail.send'],
        },
      ],
    };
    mockedRequest.mockResolvedValue(payload);

    await expect(fetchRegistry()).resolves.toEqual(payload);
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/registry');
  });
});
