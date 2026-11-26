/**
 * @jest-environment node
 *
 * http 模块 SSR 守卫分支：node 环境下 window 未定义——getToken 返回
 * undefined（fetchJSON 不带 Authorization）、URL origin 回退
 * http://localhost（buildDownloadUrl / createEventSource）。
 * jsdom 侧行为由各页面测试覆盖。
 */
import { buildDownloadUrl, createEventSource, fetchJSON } from '../http';

beforeAll(() => {
  // node 环境无业务 fetch 依赖：替换为桩，只验证请求构造与分支走向
  globalThis.fetch = jest.fn(
    async () => new Response('{"ok":1}', { status: 200 }),
  ) as unknown as typeof fetch;
});

/** 驱动 SSEStream.readStream 走给定 chunk 序列，按事件类型收集 data。
 * fetch 桩永不 settle，仅靠 close() 的 abort 退出，connect 的重连不干扰。 */
async function driveSSE(chunks: string[]): Promise<Record<string, string[]>> {
  globalThis.fetch = jest.fn(
    (_url: RequestInfo | URL, init?: RequestInit) =>
      new Promise<Response>((_resolve, reject) => {
        init?.signal?.addEventListener('abort', () => reject(new Error('aborted')));
      }),
  ) as unknown as typeof fetch;

  const stream = createEventSource('/api/stream', { attachToken: false });
  const types = ['message', 'update', 'e1', 'e2', 'a', 'b'];
  const collected: Record<string, string[]> = {};
  for (const type of types) collected[type] = [];
  const record = (type: string) => (e: MessageEvent) => {
    collected[type].push(e.data);
  };
  for (const type of types) {
    stream.addEventListener(type, record(type));
  }

  const pending = [...chunks];
  const reader = {
    async read(): Promise<ReadableStreamReadResult<Uint8Array>> {
      const chunk = pending.shift();
      if (chunk === undefined) return { done: true, value: undefined };
      return { done: false, value: new TextEncoder().encode(chunk) };
    },
  };
  const resp = { body: { getReader: () => reader } } as unknown as Response;
  await (stream as unknown as { readStream(r: Response): Promise<void> }).readStream(resp);
  stream.close();
  return collected;
}

describe('services/core/http（SSR：无 window 守卫分支）', () => {
  it('node 环境前置校验：window 未定义', () => {
    expect(typeof window).toBe('undefined');
  });

  it('fetchJSON：无 window 时跳过 token，请求不带 Authorization', async () => {
    const data = await fetchJSON('/health', { skipScopeHeaders: true });
    expect(data).toEqual({ ok: 1 });
    const fetchMock = globalThis.fetch as jest.Mock;
    const [, init] = fetchMock.mock.calls[0] as [string, RequestInit];
    const headers = new Headers(init.headers);
    expect(headers.get('Authorization')).toBeNull();
    expect(headers.get('Accept')).toBe('application/json');
  });

  it('buildDownloadUrl：origin 回退 http://localhost，空参数被过滤', () => {
    const url = buildDownloadUrl('/api/exports/logs', { page: 2, since: undefined });
    expect(url).toBe('http://localhost/api/v1/exports/logs?page=2');
  });

  it('createEventSource：无 fetch 时同步抛错（SSR 守卫，C-004）', () => {
    const original = globalThis.fetch;
    Reflect.deleteProperty(globalThis, 'fetch');
    try {
      expect(() => createEventSource('/api/stream')).toThrow(/fetch is not available/);
    } finally {
      globalThis.fetch = original;
    }
  });

  it('readStream：CRLF 跨 chunk 边界不误切帧（AUD-012）', async () => {
    // 服务端把 `event: update\r\ndata: hello\r\n\r\n` 的 \r\n 从中间拆开：
    // chunk1 以 \r 结尾、chunk2 以 \n 开头。旧实现先把悬垂 \r 归一化成 \n，
    // 下一 chunk 的 \n 拼上即成假空行 \n\n，把 event 行单独切帧（无 data 不
    // 发出），data 行落到下一帧变默认 message——update 事件就丢了。
    const { update, message } = await driveSSE(['event: update\r', '\ndata: hello\r\n\r\n']);
    expect(update).toEqual(['hello']);
    expect(message).toEqual([]);
  });

  it('readStream：跨 chunk 分帧边界与纯 CR 行尾（AUD-012 补充）', async () => {
    // 纯 CR 行尾且跨 chunk（\r 不是 CRLF 前半，是完整行尾）：
    // `data: a\r` + `\r`（空行分隔）= 事件 a；`data: b\r\n\r\n` = 事件 b。
    const cr = await driveSSE(['data: a\r', '\rdata: b\r\n\r\n']);
    expect(cr.message).toEqual(['a', 'b']);

    // LF 单块完整事件（无回归）。
    const lf = await driveSSE(['event: e1\ndata: 1\n\n', 'event: e2\ndata: 2\n\n']);
    expect(lf.e1).toEqual(['1']);
    expect(lf.e2).toEqual(['2']);

    // CRLF 边界恰在两个事件之间（帧分隔完整，不粘连）。
    const between = await driveSSE([
      'event: a\r\ndata: 1\r\n\r',
      '\n\r\nevent: b\r\ndata: 2\r\n\r\n',
    ]);
    expect(between.a).toEqual(['1']);
    expect(between.b).toEqual(['2']);
    expect(between.message).toEqual([]);
  });

  it('readStream：多行 data 的同一事件字节任意分块结果一致（AUD-012 验收）', async () => {
    // 同一逻辑字节流 `event: e1\r\ndata: l1\r\ndata: l2\r\n\r\n`：
    // 整块送达、CRLF 中腰拆开、逐字节送达必须产出完全一致的事件——
    // 事件名 e1、多行 data 以 \n 连接为 "l1\nl2"，不丢不串。
    const wire = 'event: e1\r\ndata: l1\r\ndata: l2\r\n\r\n';
    const whole = await driveSSE([wire]);
    const torn = await driveSSE(['event: e1\r', '\ndata: l1\r\n', 'data: l2\r', '\n\r\n']);
    const byteAtATime = await driveSSE(wire.split(''));
    for (const got of [whole, torn, byteAtATime]) {
      expect(got.e1).toEqual(['l1\nl2']);
      expect(got.message).toEqual([]);
    }
  });

  it('createEventSource：fetch 可用时返回 SSEStream，origin 回退且 URL 不含 token', () => {
    globalThis.fetch = jest.fn(
      (_url: RequestInfo | URL, init?: RequestInit) =>
        new Promise<Response>((_resolve, reject) => {
          init?.signal?.addEventListener('abort', () => reject(new Error('aborted')));
        }),
    ) as unknown as typeof fetch;

    const stream = createEventSource('/api/stream');
    const fetchMock = globalThis.fetch as jest.Mock;
    const url = String(fetchMock.mock.calls[0][0]);
    expect(url).toMatch(/^http:\/\/localhost\/api\/v1\/stream/);
    expect(new URL(url).searchParams.has('token')).toBe(false);
    expect(typeof stream.addEventListener).toBe('function');
    expect(typeof stream.close).toBe('function');
    stream.close();
  });
});
