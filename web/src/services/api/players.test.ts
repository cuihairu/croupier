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
const mockedGetItem = localStorage.getItem as unknown as jest.Mock;

describe('players API adapters', () => {
  beforeEach(() => {
    mockedRequest.mockReset();
    mockedGetItem.mockReset().mockReturnValue('tok-players');
  });

  it('lists players with query params and bearer token', async () => {
    mockedRequest.mockResolvedValue({ items: [], total: 0, page: 1, pageSize: 20 });

    await listPlayers({ page: 2, pageSize: 50, gameId: 'demo', search: 'alice', status: 1 });

    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/players', {
      method: 'GET',
      params: { page: 2, pageSize: 50, gameId: 'demo', search: 'alice', status: 1 },
      headers: { Authorization: 'Bearer tok-players' },
    });
  });

  it('omits the Authorization header when no token is stored', async () => {
    mockedGetItem.mockReturnValue(undefined);
    mockedRequest.mockResolvedValue({ items: [], total: 0, page: 1, pageSize: 10 });

    await listPlayers({});

    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/players', {
      method: 'GET',
      params: {},
      headers: undefined,
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
      headers: { Authorization: 'Bearer tok-players' },
    });
  });

  it('fetches a single player via GET', async () => {
    mockedRequest.mockResolvedValue(undefined);

    await getPlayer('p-1');

    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/players/p-1', {
      method: 'GET',
      headers: { Authorization: 'Bearer tok-players' },
    });
  });

  it('updates a player via PUT with the partial body', async () => {
    mockedRequest.mockResolvedValue(undefined);

    await updatePlayer('p-1', { status: 0, level: 3 });

    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/players/p-1', {
      method: 'PUT',
      data: { status: 0, level: 3 },
      headers: { Authorization: 'Bearer tok-players' },
    });
  });

  it('deletes a player via DELETE', async () => {
    mockedRequest.mockResolvedValue(undefined);

    await deletePlayer('p-1');

    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/players/p-1', {
      method: 'DELETE',
      headers: { Authorization: 'Bearer tok-players' },
    });
  });

  it('adjusts player balance via POST with amount and reason', async () => {
    mockedRequest.mockResolvedValue(undefined);

    await adjustPlayerBalance('p-1', { amount: -50, reason: 'refund' });

    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/players/p-1/balance', {
      method: 'POST',
      data: { amount: -50, reason: 'refund' },
      headers: { Authorization: 'Bearer tok-players' },
    });
  });

  it('drops the header for mutation calls too when token is absent', async () => {
    mockedGetItem.mockReturnValue(undefined);
    mockedRequest.mockResolvedValue(undefined);

    await adjustPlayerBalance('p-2', { amount: 10, reason: 'gift' });

    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/players/p-2/balance', {
      method: 'POST',
      data: { amount: 10, reason: 'gift' },
      headers: undefined,
    });
  });

  // 不可达分支说明：每个函数内 `typeof window !== 'undefined' ? ... : ''` 的
  // false 路径是 SSR 防御守卫。jsdom 环境中 globalThis.window 为
  // non-configurable（Object.defineProperty 重定义抛 "Cannot redefine
  // property: window"），无法在单测中置为 undefined，故该分支不可达。
});
