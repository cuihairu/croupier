import { request } from '@umijs/max';
import {
  adjustPlayerBalance,
  createPlayer,
  deletePlayer,
  getPlayer,
  listPlayers,
  updatePlayer,
} from './players';

jest.mock('@umijs/max', () => ({ request: jest.fn() }));

const mockedRequest = request as jest.MockedFunction<typeof request>;

// Authorization 由 requestErrorConfig 的请求拦截器统一注入
// （含无 token 时的省略），适配层只负责 URL/method/参数形状。
describe('players API adapters', () => {
  beforeEach(() => {
    mockedRequest.mockReset();
  });

  it('lists players with query params', async () => {
    mockedRequest.mockResolvedValue({ items: [], total: 0, page: 1, pageSize: 20 });

    await listPlayers({ page: 2, pageSize: 50, gameId: 'demo', search: 'alice', status: 1 });

    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/players', {
      method: 'GET',
      params: { page: 2, pageSize: 50, gameId: 'demo', search: 'alice', status: 1 },
    });
  });

  it('creates a player via POST with the body payload', async () => {
    mockedRequest.mockResolvedValue(undefined);

    await createPlayer({
      username: 'alice',
      password: 'secret',
      nickname: 'Alice',
      gameId: 'demo',
    });

    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/players', {
      method: 'POST',
      data: { username: 'alice', password: 'secret', nickname: 'Alice', gameId: 'demo' },
    });
  });

  it('fetches a single player via GET', async () => {
    mockedRequest.mockResolvedValue(undefined);

    await getPlayer('p-1');

    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/players/p-1', {
      method: 'GET',
    });
  });

  it('updates a player via PUT with the partial body', async () => {
    mockedRequest.mockResolvedValue(undefined);

    await updatePlayer('p-1', { status: 0, level: 3 });

    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/players/p-1', {
      method: 'PUT',
      data: { status: 0, level: 3 },
    });
  });

  it('deletes a player via DELETE', async () => {
    mockedRequest.mockResolvedValue(undefined);

    await deletePlayer('p-1');

    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/players/p-1', {
      method: 'DELETE',
    });
  });

  it('adjusts player balance via POST with amount and reason', async () => {
    mockedRequest.mockResolvedValue(undefined);

    await adjustPlayerBalance('p-1', { amount: -50, reason: 'refund' });

    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/players/p-1/balance', {
      method: 'POST',
      data: { amount: -50, reason: 'refund' },
    });
  });

  it('适配层不注入 Authorization/headers 键（拦截器职责）', async () => {
    mockedRequest.mockResolvedValue(undefined);

    await getPlayer('p-2');
    await adjustPlayerBalance('p-2', { amount: 10, reason: 'gift' });

    for (const call of mockedRequest.mock.calls) {
      expect(call[1]).not.toHaveProperty('headers');
    }
  });
});
