/**
 * 控制台执行服务幂等接线（D4 安全重试）：请求带 Idempotency-Key 头；
 * 失败重试复用同一 key（命中服务端重放或结果未知），成功清除该意图；
 * 草稿预览执行与已发布执行同口径。
 */
import { request } from '@umijs/max';
import { clearExecuteIdempotencyCache, executeIntentScopeKey } from '@/services/executeIdempotency';
import { executeDraftCompositeBinding, executePageBinding } from './console';

jest.mock('@umijs/max', () => ({ request: jest.fn() }));

const mockRequest = jest.mocked(request);

/** request(url, config) 的第二个参数即配置体（含 headers/data）。 */
function configOf(call: unknown): {
  data?: Record<string, unknown>;
  headers?: Record<string, unknown>;
} {
  const args = call as unknown[];
  return ((Array.isArray(args) ? args[1] : {}) ?? {}) as {
    data?: Record<string, unknown>;
    headers?: Record<string, unknown>;
  };
}

function keyOf(call: unknown): string {
  return (configOf(call).headers ?? {})['Idempotency-Key'] as string;
}

describe('console execute idempotency wiring', () => {
  beforeEach(() => {
    clearExecuteIdempotencyCache();
    mockRequest.mockReset();
  });

  it('已发布执行：请求带 Idempotency-Key，失败重试复用同一 key', async () => {
    mockRequest.mockResolvedValueOnce(new Error('boom') as unknown);
    mockRequest.mockResolvedValueOnce({
      result: { kind: 'sync', requestId: 'r2', replayed: true },
      replayed: true,
    } as unknown);

    await expect(
      executePageBinding('player.manage', 'player.query', { form: {} }),
    ).rejects.toBeInstanceOf(Error);
    const second = await executePageBinding('player.manage', 'player.query', { form: {} });

    expect(second.replayed).toBe(true);
    expect(second.requestId).toBe('r2');
    expect(keyOf(mockRequest.mock.calls[0])).toBe(keyOf(mockRequest.mock.calls[1]));
    expect(keyOf(mockRequest.mock.calls[0])).toMatch(/^page-exec-/);
  });

  it('已发布执行错误保留给页面反馈，不走全局错误处理', async () => {
    const rawError = new Error('permission denied');
    mockRequest.mockRejectedValueOnce(rawError);

    await expect(executePageBinding('player.manage', 'player.query', { form: {} })).rejects.toBe(
      rawError,
    );
    expect(configOf(mockRequest.mock.calls[0]).skipErrorHandler).toBe(true);
  });

  it('草稿预览执行错误保留给页面反馈，不走全局错误处理', async () => {
    const rawError = new Error('permission denied');
    mockRequest.mockRejectedValueOnce(rawError);

    await expect(
      executeDraftCompositeBinding('player.manage', 'player.query', [], { form: {} }),
    ).rejects.toBe(rawError);
    expect(configOf(mockRequest.mock.calls[0]).skipErrorHandler).toBe(true);
  });

  it('成功后清除意图：同内容再次提交是新意图（新 key）', async () => {
    const body = { result: { kind: 'sync', requestId: 'r1' } };
    mockRequest.mockResolvedValueOnce(body as unknown);
    mockRequest.mockResolvedValueOnce(body as unknown);

    await executePageBinding('player.manage', 'player.query', { form: {} });
    const second = await executePageBinding('player.manage', 'player.query', { form: {} });

    expect(second.replayed).toBeUndefined();
    expect(keyOf(mockRequest.mock.calls[0])).not.toBe(keyOf(mockRequest.mock.calls[1]));
  });

  it('请求体不含幂等键：key 只走请求头，不落业务 payload', async () => {
    mockRequest.mockResolvedValue({ result: { kind: 'sync', requestId: 'r' } } as unknown);
    await executePageBinding('player.manage', 'player.query', { form: { a: 1 } });

    const call = mockRequest.mock.calls[0][1] as { data: Record<string, unknown> };
    expect(Object.keys(call.data)).toEqual(['context']);
    expect(call.data.context).toEqual({ form: { a: 1 } });
  });

  it('返回空结果也保持意图未闭环（同 key 重试）', async () => {
    mockRequest.mockResolvedValueOnce({} as unknown);
    mockRequest.mockResolvedValueOnce({ result: { kind: 'sync', requestId: 'r2' } } as unknown);

    await expect(
      executePageBinding('player.manage', 'player.query', { form: {} }),
    ).rejects.toThrow();
    const second = await executePageBinding('player.manage', 'player.query', { form: {} });

    expect(second.requestId).toBe('r2');
    expect(keyOf(mockRequest.mock.calls[0])).toBe(keyOf(mockRequest.mock.calls[1]));
  });

  it('草稿预览执行：同口径幂等（审批/失败场景不重复落单依赖服务端同 key 判定）', async () => {
    mockRequest.mockResolvedValueOnce({
      result: { kind: 'approval', requestId: 'r', approvalId: 'a-1' },
    } as unknown);
    mockRequest.mockResolvedValueOnce({
      result: { kind: 'approval', requestId: 'r', approvalId: 'a-1' },
      replayed: true,
    } as unknown);

    const sections = [{ key: 's1' }];
    await executeDraftCompositeBinding('draft--idem', 'player.list', sections, { form: {} });
    const second = await executeDraftCompositeBinding('draft--idem', 'player.list', sections, {
      form: {},
    });

    expect(second.replayed).toBe(true);
    expect(second.approvalId).toBe('a-1');
    expect(keyOf(mockRequest.mock.calls[0])).toBe(keyOf(mockRequest.mock.calls[1]));
  });

  it('作用域含上下文字节：上下文变化即新意图', async () => {
    mockRequest.mockResolvedValue({ result: { kind: 'sync', requestId: 'r' } } as unknown);
    const s1 = executeIntentScopeKey({ pageKey: 'p', bindingId: 'b', context: { form: { k: 1 } } });
    const s2 = executeIntentScopeKey({ pageKey: 'p', bindingId: 'b', context: { form: { k: 2 } } });
    expect(s1).not.toBe(s2);
  });
});
