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

  it('createEventSource：origin 回退后因 EventSource 缺失抛错（分支已评估）', () => {
    expect(() => createEventSource('/events')).toThrow();
  });
});
