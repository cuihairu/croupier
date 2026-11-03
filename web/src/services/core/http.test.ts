import { TextDecoder as NodeTextDecoder, TextEncoder as NodeTextEncoder } from 'util';
import { applyScopeHeaders, needsResolvedScope, waitForResolvedScope } from './scope';
import { buildDownloadUrl, createEventSource, fetchJSON } from './http';

jest.mock('./scope', () => ({
  applyScopeHeaders: jest.fn(),
  needsResolvedScope: jest.fn().mockReturnValue(false),
  waitForResolvedScope: jest.fn().mockResolvedValue(undefined),
}));

const mockedNeedsScope = needsResolvedScope as jest.MockedFunction<typeof needsResolvedScope>;
const mockedWaitFor = waitForResolvedScope as jest.MockedFunction<typeof waitForResolvedScope>;
const mockedApply = applyScopeHeaders as jest.MockedFunction<typeof applyScopeHeaders>;

const jsonResponse = (body: unknown, status = 200) =>
  ({
    ok: status >= 200 && status < 300,
    status,
    text: async () => (typeof body === 'string' ? body : JSON.stringify(body)),
    // 与真实 Response 一致：json 走真解析，畸形体抛 SyntaxError
    json: async () => JSON.parse(typeof body === 'string' ? body : JSON.stringify(body)),
  }) as Response;

describe('fetchJSON', () => {
  let fetchSpy: jest.SpyInstance;

  beforeAll(() => {
    // jsdom 无 global.fetch：补占位实现后再 spy（spyOn 需要属性已存在）
    if (typeof global.fetch !== 'function') {
      Object.defineProperty(global, 'fetch', { value: async () => ({}), writable: true });
    }
  });

  beforeEach(() => {
    fetchSpy = jest.spyOn(global, 'fetch');
    (localStorage.getItem as unknown as jest.Mock).mockReturnValue('tok-http');
    mockedNeedsScope.mockReturnValue(false);
    mockedWaitFor.mockClear();
    mockedApply.mockClear();
  });

  afterEach(() => {
    fetchSpy?.mockRestore();
  });

  it('upgrades legacy paths, attaches bearer token and Accept, and parses JSON', async () => {
    fetchSpy.mockResolvedValueOnce(jsonResponse({ items: [1] }));

    const data = await fetchJSON<{ items: number[] }>('/api/things');

    expect(data).toEqual({ items: [1] });
    const [url, init] = fetchSpy.mock.calls[0];
    expect(url).toBe('/api/v1/things');
    expect(init.headers.get('Authorization')).toBe('Bearer tok-http');
    expect(init.headers.get('Accept')).toBe('application/json');
  });

  it('sets Content-Type only for string bodies and keeps caller headers', async () => {
    fetchSpy.mockResolvedValueOnce(jsonResponse({}));
    await fetchJSON('/api/v1/a', { method: 'POST', body: '{"x":1}' });
    expect(fetchSpy.mock.calls[0][1].headers.get('Content-Type')).toBe('application/json');

    fetchSpy.mockResolvedValueOnce(jsonResponse({}));
    await fetchJSON('/api/v1/b', {
      method: 'POST',
      body: new FormData(),
      headers: { 'Content-Type': 'text/csv', Accept: 'text/csv' },
    });
    const headers = fetchSpy.mock.calls[1][1].headers;
    expect(headers.get('Content-Type')).toBe('text/csv');
    expect(headers.get('Accept')).toBe('text/csv');
  });

  it('skips the Authorization header with skipAuth and when no token exists', async () => {
    fetchSpy.mockResolvedValueOnce(jsonResponse({}));
    await fetchJSON('/api/v1/open', { skipAuth: true });
    expect(fetchSpy.mock.calls[0][1].headers.get('Authorization')).toBeNull();

    (localStorage.getItem as unknown as jest.Mock).mockReturnValue(null);
    fetchSpy.mockResolvedValueOnce(jsonResponse({}));
    await fetchJSON('/api/v1/open');
    expect(fetchSpy.mock.calls[1][1].headers.get('Authorization')).toBeNull();
  });

  it('localStorage access throwing (blocked storage) degrades to no Authorization header', async () => {
    // 隐私模式/存储被策略禁用：getItem 抛错不能中断请求
    (localStorage.getItem as unknown as jest.Mock).mockImplementationOnce(() => {
      throw new Error('SecurityError');
    });
    fetchSpy.mockResolvedValueOnce(jsonResponse({}));

    await expect(fetchJSON('/api/v1/open')).resolves.toEqual({});
    expect(fetchSpy.mock.calls[0][1].headers.get('Authorization')).toBeNull();
  });

  it('waits for scope resolution and applies scope headers for scope-needing URLs', async () => {
    mockedNeedsScope.mockReturnValue(true);
    fetchSpy.mockResolvedValueOnce(jsonResponse({ ok: 1 }));

    await fetchJSON('/api/v1/pages');

    expect(mockedWaitFor).toHaveBeenCalledWith('/api/v1/pages');
    expect(mockedApply).toHaveBeenCalledTimes(1);
    // scope header 由 applyScopeHeaders 假体注入的调用形参体现（Headers 实例）
    expect(mockedApply.mock.calls[0][0]).toBeInstanceOf(Headers);
  });

  it('skips scope waiting entirely with skipScopeHeaders', async () => {
    mockedNeedsScope.mockReturnValue(true);
    fetchSpy.mockResolvedValueOnce(jsonResponse({}));

    await fetchJSON('/api/v1/pages', { skipScopeHeaders: true });

    expect(mockedWaitFor).not.toHaveBeenCalled();
    expect(mockedApply).not.toHaveBeenCalled();
  });

  it('throws HttpError with status and responseText on non-2xx, falling back to a status message', async () => {
    fetchSpy.mockResolvedValueOnce(jsonResponse('boom-body', 409));

    const err = await fetchJSON('/api/v1/x').catch((e: Error & { status?: number }) => e);
    expect(err).toBeInstanceOf(Error);
    expect(err.message).toBe('boom-body');
    expect(err.status).toBe(409);
    expect((err as { responseText?: string }).responseText).toBe('boom-body');

    fetchSpy.mockResolvedValueOnce({
      ok: false,
      status: 502,
      text: async () => '',
    } as unknown as Response);
    const err2 = await fetchJSON('/api/v1/y').catch((e: Error) => e);
    expect(err2.message).toBe('Request failed: 502');
  });

  it('returns undefined for 204 and for malformed JSON bodies', async () => {
    fetchSpy.mockResolvedValueOnce({ ok: true, status: 204 } as Response);
    await expect(fetchJSON('/api/v1/n')).resolves.toBeUndefined();

    fetchSpy.mockResolvedValueOnce(jsonResponse('not-json', 200));
    await expect(fetchJSON('/api/v1/m')).resolves.toBeUndefined();
  });
});

describe('createEventSource', () => {
  let sseFetchSpy: jest.SpyInstance;

  beforeAll(() => {
    // jsdom 无 global.fetch：SSE 流走 fetch，先补占位实现再 spy
    if (typeof global.fetch !== 'function') {
      Object.defineProperty(global, 'fetch', { value: async () => ({}), writable: true });
    }
    // jsdom 未实现 TextEncoder/TextDecoder（帧解析依赖）：以 Node util 补齐
    if (typeof global.TextDecoder === 'undefined') {
      Object.defineProperty(global, 'TextDecoder', { value: NodeTextDecoder, writable: true });
    }
    if (typeof global.TextEncoder === 'undefined') {
      Object.defineProperty(global, 'TextEncoder', { value: NodeTextEncoder, writable: true });
    }
  });

  beforeEach(() => {
    sseFetchSpy = jest.spyOn(global, 'fetch');
    (localStorage.getItem as unknown as jest.Mock).mockReturnValue('tok-http');
  });

  afterEach(() => {
    sseFetchSpy?.mockRestore();
  });

  // C-004：token 走 Authorization 头，绝不进 URL/日志/Referer。
  const pendingFetch = () => (_url: unknown, init?: RequestInit) =>
    new Promise<Response>((_resolve, reject) => {
      init?.signal?.addEventListener('abort', () => reject(new Error('aborted')));
    });

  it('sends the bearer header instead of a token query param', () => {
    sseFetchSpy.mockImplementationOnce(pendingFetch());

    const stream = createEventSource('/api/v1/stream', {
      params: { gameId: 'demo', env: undefined, page: 2 },
    });

    const [url, init] = sseFetchSpy.mock.calls[0] as [string, RequestInit];
    const parsed = new URL(url);
    expect(parsed.pathname).toBe('/api/v1/stream');
    expect(parsed.searchParams.get('gameId')).toBe('demo');
    expect(parsed.searchParams.get('page')).toBe('2');
    expect(parsed.searchParams.has('env')).toBe(false);
    expect(parsed.searchParams.has('token')).toBe(false);
    expect(new Headers(init.headers).get('Authorization')).toBe('Bearer tok-http');
    stream.close();
  });

  it('omits the Authorization header with attachToken=false and upgrades legacy paths', () => {
    sseFetchSpy.mockImplementationOnce(pendingFetch());

    const stream = createEventSource('/api/stream', { attachToken: false });

    const [url, init] = sseFetchSpy.mock.calls[0] as [string, RequestInit];
    const parsed = new URL(url);
    expect(parsed.pathname).toBe('/api/v1/stream');
    expect(new Headers(init.headers).get('Authorization')).toBeNull();
    stream.close();
  });

  it('parses SSE frames and dispatches named events', async () => {
    const chunks = [
      'event: connected\ndata: {"status":"connected"}\n\n',
      'event: message\ndata: {"online":1}\n\n',
    ];
    let idx = 0;
    let finish: (() => void) | undefined;
    const reader = {
      read: async (): Promise<{ done: boolean; value?: Uint8Array }> => {
        if (idx < chunks.length) {
          const value = new TextEncoder().encode(chunks[idx]);
          idx += 1;
          return { done: false, value };
        }
        await new Promise<void>((r) => {
          finish = r;
        });
        return { done: true };
      },
    };
    sseFetchSpy.mockResolvedValueOnce({
      ok: true,
      status: 200,
      body: { getReader: () => reader },
    } as unknown as Response);

    const stream = createEventSource('/api/v1/stream');
    const seen: Array<{ type: string; data: string }> = [];
    stream.addEventListener('connected', (e) => seen.push({ type: 'connected', data: e.data }));
    stream.addEventListener('message', (e) => seen.push({ type: 'message', data: e.data }));
    const opened = jest.fn();
    stream.onopen = opened;

    await new Promise((r) => setTimeout(r, 0));
    expect(opened).toHaveBeenCalled();
    expect(seen).toEqual([
      { type: 'connected', data: '{"status":"connected"}' },
      { type: 'message', data: '{"online":1}' },
    ]);
    finish?.();
    stream.close();
  });

  it('stops without reconnecting on non-200 (fatal, EventSource semantics)', async () => {
    sseFetchSpy.mockResolvedValueOnce({ ok: false, status: 401 } as unknown as Response);

    const stream = createEventSource('/api/v1/stream');
    const onerror = jest.fn();
    stream.onerror = onerror;

    await new Promise((r) => setTimeout(r, 0));
    expect(onerror).toHaveBeenCalledTimes(1);
    stream.close();
    await new Promise((r) => setTimeout(r, 0));
    expect(sseFetchSpy).toHaveBeenCalledTimes(1);
  });
});

describe('buildDownloadUrl', () => {
  it('drops nullish params and resolves against the window origin', () => {
    const url = buildDownloadUrl('/api/v1/audit/export', { gameId: 'demo', env: null, page: 3 });
    const parsed = new URL(url);
    expect(parsed.origin).toBe(window.location.origin);
    expect(parsed.pathname).toBe('/api/v1/audit/export');
    expect(parsed.searchParams.get('gameId')).toBe('demo');
    expect(parsed.searchParams.get('page')).toBe('3');
    expect(parsed.searchParams.has('env')).toBe(false);
  });

  it('keeps the path untouched when no params are given', () => {
    expect(buildDownloadUrl('/api/v1/files/x.csv')).toBe(
      `${window.location.origin}/api/v1/files/x.csv`,
    );
  });
});
