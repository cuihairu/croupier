import { request } from '@umijs/max';
import { deleteGame, listGamesMeta, listMyGames, updateGame, upsertGame } from './games';

jest.mock('@umijs/max', () => ({ request: jest.fn() }));

const mockedRequest = request as jest.MockedFunction<typeof request>;

const EMPTY_GAME = {
  id: undefined,
  name: undefined,
  aliasName: undefined,
  envs: undefined,
  envMeta: undefined,
};

describe('games API adapters', () => {
  beforeEach(() => mockedRequest.mockReset());

  describe('listGamesMeta', () => {
    it('normalizes raw rows: alias fallbacks, env derivation, dirty envMeta, null rows', async () => {
      mockedRequest.mockResolvedValue({
        games: [
          {
            id: 1,
            gameId: 'demo',
            aliasName: '演示',
            envs: ['dev', 'prod'],
            envMeta: [{ env: 'dev' }, { env: 'prod' }],
          },
          {
            name: 'by-name',
            displayName: '显示名',
            envs: [],
            envMeta: [{ env: 'staging' }, null, { description: 'missing env' }],
          },
          { gameName: 'legacy-game-name' },
          { envs: [] },
          null,
        ],
      });

      const { games } = await listGamesMeta();

      // 1) gameId/aliasName 直取 + 非空 envs 原样保留
      expect(games[0]).toEqual({
        id: 1,
        name: 'demo',
        aliasName: '演示',
        envs: ['dev', 'prod'],
        envMeta: [{ env: 'dev' }, { env: 'prod' }],
      });
      // 2) name 走 raw.name 兜底、aliasName 走 displayName 兜底；
      //    envs 为空数组时从 envMeta 派生，envMeta 内 null 行/缺 env 行被剔除
      expect(games[1]).toEqual({
        id: undefined,
        name: 'by-name',
        aliasName: '显示名',
        envs: ['staging'],
        envMeta: [{ env: 'staging' }],
      });
      // 3) aliasName 最后兜底 gameName；无 envs/envMeta → undefined
      expect(games[2]).toEqual({ ...EMPTY_GAME, aliasName: 'legacy-game-name' });
      // 4) envs 为空数组且无 envMeta → envs 归一 undefined
      expect(games[3]).toEqual(EMPTY_GAME);
      // 5) null 行整体归一为空形态
      expect(games[4]).toEqual(EMPTY_GAME);

      expect(mockedRequest).toHaveBeenCalledWith('/api/v1/games');
    });

    it('returns an empty list when games is missing or not an array', async () => {
      mockedRequest.mockResolvedValue({});
      await expect(listGamesMeta()).resolves.toEqual({ games: [] });

      mockedRequest.mockResolvedValue({ games: 'nope' });
      await expect(listGamesMeta()).resolves.toEqual({ games: [] });
    });

    it('returns an empty list when the response body is empty', async () => {
      mockedRequest.mockResolvedValue(undefined);

      await expect(listGamesMeta()).resolves.toEqual({ games: [] });
    });
  });

  describe('listMyGames', () => {
    it('normalizes the current user games from /api/v1/profile/games', async () => {
      mockedRequest.mockResolvedValue({ games: [{ gameId: 'demo', envs: ['prod'] }] });

      await expect(listMyGames()).resolves.toEqual({
        games: [
          {
            id: undefined,
            name: 'demo',
            aliasName: undefined,
            envs: ['prod'],
            envMeta: undefined,
          },
        ],
      });

      expect(mockedRequest).toHaveBeenCalledWith('/api/v1/profile/games');
    });

    it('returns an empty list on an empty response body', async () => {
      mockedRequest.mockResolvedValue(undefined);

      await expect(listMyGames()).resolves.toEqual({ games: [] });
    });
  });

  describe('upsertGame', () => {
    it('POSTs the full game payload including config', async () => {
      mockedRequest.mockResolvedValue({ game: { id: 1 } });

      await upsertGame({ name: 'demo', aliasName: '演示', description: '描述', config: '{"a":1}' });

      expect(mockedRequest).toHaveBeenCalledWith('/api/v1/games', {
        method: 'POST',
        data: { name: 'demo', aliasName: '演示', description: '描述', config: '{"a":1}' },
      });
    });

    it('tolerates a void (204) response and optional fields', async () => {
      mockedRequest.mockResolvedValue(undefined);

      await expect(
        upsertGame({ name: 'demo', aliasName: undefined, description: undefined }),
      ).resolves.toBeUndefined();

      expect(mockedRequest).toHaveBeenCalledWith('/api/v1/games', {
        method: 'POST',
        data: { name: 'demo', aliasName: undefined, description: undefined, config: undefined },
      });
    });
  });

  it('deleteGame issues DELETE by numeric id', async () => {
    mockedRequest.mockResolvedValue(undefined);

    await deleteGame(7);

    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/games/7', { method: 'DELETE' });
  });

  it('updateGame issues PUT with name/aliasName/description', async () => {
    mockedRequest.mockResolvedValue(undefined);

    await updateGame(3, { name: 'n', aliasName: 'a', description: 'd' });

    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/games/3', {
      method: 'PUT',
      data: { name: 'n', aliasName: 'a', description: 'd' },
    });
  });
});
