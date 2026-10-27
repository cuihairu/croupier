/**
 * 执行幂等键缓存语义（D4 安全重试）：
 * 同一提交意图（同页面/绑定/上下文）在未成功前复用同一 key；成功后清除，
 * 同内容再提交是新意图；过期/淘汰后生成新 key（新意图，允许真实重跑）。
 */
import {
  acquireExecuteIdempotencyKey,
  clearExecuteIdempotencyCache,
  executeIntentScopeKey,
  generateExecuteIdempotencyKey,
  releaseExecuteIdempotencyKey,
} from './executeIdempotency';

const KEY_CHARSET = /^[A-Za-z0-9._:-]+$/;

function scope(pageKey = 'player.manage', bindingId = 'player.query', keyword = 'alice') {
  return executeIntentScopeKey({
    pageKey,
    bindingId,
    context: { form: { keyword } },
  });
}

describe('execute idempotency key cache', () => {
  beforeEach(() => clearExecuteIdempotencyCache());

  it('同作用域未成功前复用同一 key，且键符合服务端归一字符集', () => {
    const s = scope();
    const first = acquireExecuteIdempotencyKey(s, 1_000);
    const second = acquireExecuteIdempotencyKey(s, 2_000);
    expect(second).toBe(first);
    expect(first).toMatch(KEY_CHARSET);
    expect(first.length).toBeLessThanOrEqual(128);
  });

  it('不同作用域（页面/绑定/上下文任一不同）得到不同 key', () => {
    const a = acquireExecuteIdempotencyKey(scope('p1', 'b1', 'k'), 1_000);
    const b = acquireExecuteIdempotencyKey(scope('p2', 'b1', 'k'), 1_000);
    const c = acquireExecuteIdempotencyKey(scope('p1', 'b2', 'k'), 1_000);
    const d = acquireExecuteIdempotencyKey(scope('p1', 'b1', 'k2'), 1_000);
    expect(new Set([a, b, c, d]).size).toBe(4);
  });

  it('成功后清除：同内容再次提交是新意图（新 key）', () => {
    const s = scope();
    const first = acquireExecuteIdempotencyKey(s, 1_000);
    releaseExecuteIdempotencyKey(s);
    const next = acquireExecuteIdempotencyKey(s, 2_000);
    expect(next).not.toBe(first);
  });

  it('过期后生成新 key（旧意图不再被追跑）', () => {
    const s = scope();
    const first = acquireExecuteIdempotencyKey(s, 1_000);
    // 10 分钟保留窗口后（客户端刻意短于服务端占位窗口）
    const later = acquireExecuteIdempotencyKey(s, 1_000 + 10 * 60 * 1000 + 1);
    expect(later).not.toBe(first);
  });

  it('容量上限按插入序淘汰最旧作用域', () => {
    // 生成 51 个作用域，第 1 个被淘汰；再取第 1 个得到新 key。
    const firstScope = scope('evict', 'b', 'k');
    const firstKey = acquireExecuteIdempotencyKey(firstScope, 1_000);
    for (let i = 0; i < 50; i += 1) {
      acquireExecuteIdempotencyKey(`scope-${i}\u0000b\u0000{}`, 1_000 + i);
    }
    const reacquired = acquireExecuteIdempotencyKey(firstScope, 2_000);
    expect(reacquired).not.toBe(firstKey);
  });

  it('生成键格式稳定：前缀 + 时间戳 + 随机段，可被服务端归一接受', () => {
    const key = generateExecuteIdempotencyKey(1_700_000_000_000);
    expect(key).toMatch(/^page-exec-[0-9a-z]+-[0-9a-z]{8}$/);
    expect(key).toMatch(KEY_CHARSET);
  });
});
