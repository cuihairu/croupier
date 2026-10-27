/**
 * 执行幂等键缓存（D4 安全重试）
 *
 * 一次「执行意图」（用户点一次提交）对应一个 Idempotency-Key，服务端据此保证
 * 同 key 只真实执行一次：已完成的重放落地结果，进行中的判结果未知，键误复用
 * 拒绝。缓存的职责是区分「重试上一次失败的意图」与「提交一次新的意图」：
 *
 * - 上一次同内容提交尚未成功时，再次提交复用同一 key（命中重放或结果未知，
 *   不会重复执行副作用）；
 * - 上一次提交成功后清除该作用域，之后同内容提交视为新的意图，生成新 key
 *   真实执行（同内容不等于同意图）。
 *
 * 缓存是浏览器内存态：刷新/换标签页即清空，届时同内容重试会拿到新 key。保留
 * 窗口（TTL）刻意短于服务端占位窗口——不把一次早先失败的意图在结果已可重跑
 * 之后仍用旧 key 追跑，避免服务端超窗回收被用户误读为重放。容量上限用插入序
 * 淘汰，避免长会话无限增长；淘汰后同一意图的重试会退化为新 key。
 */

import type { BindingExecutionContext } from '@/types/dashboard';

/** 执行意图作用域：页面 + 绑定 + 提交上下文（上下文按 JSON 序列化比对）。 */
export interface ExecuteIntentScope {
  pageKey: string;
  bindingId: string;
  context: BindingExecutionContext;
}

/** 缓存保留窗口：10 分钟，短于服务端占位窗口（15 分钟）。 */
const IDEM_KEY_TTL_MS = 10 * 60 * 1000;

/** 作用域缓存上限：超出按插入序淘汰最旧条目。 */
const IDEM_KEY_CACHE_LIMIT = 50;

/** 键前缀：保证浏览器生成的键与服务端归一字符集 [A-Za-z0-9._:-] 相容。 */
const IDEM_KEY_PREFIX = 'page-exec-';

interface CacheEntry {
  key: string;
  expiresAt: number;
}

const cache = new Map<string, CacheEntry>();

/** 作用域指纹：上下文对象按插入序稳定序列化，同一路径重复提交字节一致。 */
export function executeIntentScopeKey(scope: ExecuteIntentScope): string {
  return `${scope.pageKey}\u0000${scope.bindingId}\u0000${JSON.stringify(scope.context)}`;
}

/** 生成幂等键：时间戳 + 随机段，均为小写 base36 与连字符，长度远小于上限。 */
export function generateExecuteIdempotencyKey(now: number = Date.now()): string {
  const random = Math.random().toString(36).slice(2, 10).padEnd(8, '0');
  return `${IDEM_KEY_PREFIX}${now.toString(36)}-${random}`;
}

/** 清除已过期条目（保留窗口到期：旧意图不再追跑，下次同内容按新意图处理）。 */
function pruneExpired(cacheAt: number): void {
  for (const [scopeKey, entry] of cache) {
    if (entry.expiresAt <= cacheAt) {
      cache.delete(scopeKey);
    }
  }
}

/** 容量上限：按插入序淘汰最旧作用域（Map 迭代序即插入序）。 */
function evictOverLimit(): void {
  while (cache.size > IDEM_KEY_CACHE_LIMIT) {
    const oldest = cache.keys().next().value;
    if (oldest === undefined) break;
    cache.delete(oldest);
  }
}

/**
 * 取本次执行意图的幂等键：同作用域未过期即复用（重试），否则新建。
 * @param scopeKey 由 executeIntentScopeKey 生成的作用域指纹
 * @param now 注入当前时间戳，供用例固定时钟断言过期语义
 */
export function acquireExecuteIdempotencyKey(scopeKey: string, now: number = Date.now()): string {
  const existing = cache.get(scopeKey);
  if (existing && existing.expiresAt > now) {
    return existing.key;
  }
  pruneExpired(now);
  const key = generateExecuteIdempotencyKey(now);
  cache.set(scopeKey, { key, expiresAt: now + IDEM_KEY_TTL_MS });
  evictOverLimit();
  return key;
}

/** 意图已闭环（服务端确认执行成功）：清除作用域，下次同内容提交是新意图。 */
export function releaseExecuteIdempotencyKey(scopeKey: string): void {
  cache.delete(scopeKey);
}

/** 仅用例使用：清空内存缓存。 */
export function clearExecuteIdempotencyCache(): void {
  cache.clear();
}
