import { request } from '@umijs/max';
import {
  deleteGame,
  getGame,
  listGamesMeta,
  listMyGames,
  updateGame,
  uploadGameIcon,
  upsertGame,
} from './games';

jest.mock('@umijs/max', () => ({
  request: jest.fn(),
  getIntl: () => ({
    formatMessage: ({ defaultMessage }: { defaultMessage: string }) => defaultMessage,
  }),
}));

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
    it('POSTs the full game payload including config and icon', async () => {
      mockedRequest.mockResolvedValue({ game: { id: 1 } });

      await upsertGame({
        name: 'demo',
        aliasName: '演示',
        description: '描述',
        config: '{"a":1}',
        icon: 'https://cdn.example.com/game.png',
      });

      expect(mockedRequest).toHaveBeenCalledWith('/api/v1/games', {
        method: 'POST',
        data: {
          name: 'demo',
          aliasName: '演示',
          icon: 'https://cdn.example.com/game.png',
          description: '描述',
          config: '{"a":1}',
        },
      });
    });

    it('tolerates a void (204) response and optional fields', async () => {
      mockedRequest.mockResolvedValue(undefined);

      await expect(
        upsertGame({ name: 'demo', aliasName: undefined, description: undefined }),
      ).resolves.toBeUndefined();

      expect(mockedRequest).toHaveBeenCalledWith('/api/v1/games', {
        method: 'POST',
        data: {
          name: 'demo',
          aliasName: undefined,
          icon: undefined,
          description: undefined,
          config: undefined,
        },
      });
    });
  });

  it('deleteGame issues DELETE by numeric id', async () => {
    mockedRequest.mockResolvedValue(undefined);

    await deleteGame(7);

    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/games/7', { method: 'DELETE' });
  });

  it('getGame fetches game detail by id', async () => {
    mockedRequest.mockResolvedValue({ game: { id: 3, name: 'demo', aliasName: '演示' } });

    await expect(getGame(3)).resolves.toEqual({
      game: { id: 3, name: 'demo', aliasName: '演示' },
    });
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/games/3');
  });

  it('updateGame PUTs aliasName and icon (empty string clears icon)', async () => {
    mockedRequest.mockResolvedValue({ game: { id: 3 } });

    await updateGame(3, { aliasName: '新名', icon: '' });

    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/games/3', {
      method: 'PUT',
      data: { aliasName: '新名', icon: '' },
    });
  });
});

// ---- uploadGameIcon：Fake XHR 拦截（同 storage.test.ts 先例）----
class IconFakeXHR {
  onload: (() => void) | null = null;
  onerror: (() => void) | null = null;
  upload: { onprogress: ((event: ProgressEvent) => void) | null } = { onprogress: null };
  status = 0;
  responseText = '';
  method = '';
  url = '';
  headers: Record<string, string> = {};
  body: FormData | undefined;

  open(method: string, url: string) {
    this.method = method;
    this.url = url;
  }

  setRequestHeader(key: string, value: string) {
    this.headers[key] = value;
  }

  send(body?: FormData) {
    this.body = body;
  }
}

let lastIconXHR: IconFakeXHR;

class IconFakeXHRTracked extends IconFakeXHR {
  constructor() {
    super();
    lastIconXHR = this;
  }
}

describe('uploadGameIcon', () => {
  let originalXHR: typeof global.XMLHttpRequest;

  beforeAll(() => {
    originalXHR = global.XMLHttpRequest;
    Object.defineProperty(global, 'XMLHttpRequest', { value: IconFakeXHRTracked, writable: true });
  });

  afterAll(() => {
    Object.defineProperty(global, 'XMLHttpRequest', { value: originalXHR, writable: true });
  });

  beforeEach(() => {
    (localStorage.getItem as unknown as jest.Mock).mockReturnValue('tok-1');
  });

  it('POSTs multipart file with bearer token and resolves key/url', async () => {
    const pending = uploadGameIcon(new File(['icon'], 'logo.png'));
    await Promise.resolve();
    expect(lastIconXHR.method).toBe('POST');
    expect(lastIconXHR.url).toBe('/api/v1/games/icons');
    expect(lastIconXHR.headers.Authorization).toBe('Bearer tok-1');
    expect(lastIconXHR.body).toBeInstanceOf(FormData);
    expect(lastIconXHR.body?.get('file')).toBeInstanceOf(File);

    lastIconXHR.status = 200;
    lastIconXHR.responseText = JSON.stringify({
      key: 'icons/games/4d5e562f37cd6576.png',
      url: '/uploads/icons/games/4d5e562f37cd6576.png',
    });
    lastIconXHR.onload?.();

    await expect(pending).resolves.toEqual({
      key: 'icons/games/4d5e562f37cd6576.png',
      url: '/uploads/icons/games/4d5e562f37cd6576.png',
    });
  });

  it('reports upload progress percentages when computable', async () => {
    const seen: number[] = [];
    const pending = uploadGameIcon(new File(['icon'], 'logo.png'), (p) => seen.push(p));
    await Promise.resolve();

    lastIconXHR.upload.onprogress?.({
      lengthComputable: true,
      loaded: 25,
      total: 100,
    } as ProgressEvent);
    // 不可计算时不汇报
    lastIconXHR.upload.onprogress?.({
      lengthComputable: false,
      loaded: 50,
      total: 100,
    } as ProgressEvent);
    lastIconXHR.status = 200;
    lastIconXHR.responseText = JSON.stringify({ key: 'k', url: 'u' });
    lastIconXHR.onload?.();

    await pending;
    expect(seen).toEqual([25]);
  });

  it('rejects with backend message on HTTP error status', async () => {
    const pending = uploadGameIcon(new File(['icon'], 'logo.png'));
    await Promise.resolve();

    lastIconXHR.status = 400;
    lastIconXHR.responseText = JSON.stringify({ error: 'x', message: '仅支持 png/jpg/webp/svg' });
    lastIconXHR.onload?.();

    await expect(pending).rejects.toThrow('仅支持 png/jpg/webp/svg');
  });

  it('rejects with fallback message on malformed response body', async () => {
    const pending = uploadGameIcon(new File(['icon'], 'logo.png'));
    await Promise.resolve();

    lastIconXHR.status = 200;
    lastIconXHR.responseText = 'not-json';
    lastIconXHR.onload?.();

    await expect(pending).rejects.toThrow('上传响应解析失败');
  });

  it('rejects with fallback message on success body missing url', async () => {
    const pending = uploadGameIcon(new File(['icon'], 'logo.png'));
    await Promise.resolve();

    lastIconXHR.status = 200;
    lastIconXHR.responseText = JSON.stringify({ key: 'k' });
    lastIconXHR.onload?.();

    await expect(pending).rejects.toThrow('上传响应缺少图标地址');
  });

  it('rejects on network error', async () => {
    const pending = uploadGameIcon(new File(['icon'], 'logo.png'));
    await Promise.resolve();

    lastIconXHR.onerror?.();

    await expect(pending).rejects.toThrow('图标上传失败');
  });
});
