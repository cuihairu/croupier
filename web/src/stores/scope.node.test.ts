/**
 * @jest-environment node
 *
 * 覆盖 scope.ts 的 SSR 分支：jsdom 下 window.window 为
 * [LegacyUnforgeable]，无法遮蔽为 undefined，只能在 node 环境触达
 * `typeof window === 'undefined'` 各分支。
 */
import { getScope, isScopeReady, markScopeReady, setScope, subscribeScope } from './scope';

describe('stores/scope（SSR：无 window 环境）', () => {
  it('模块加载时不读 localStorage、不标记就绪', () => {
    expect(isScopeReady()).toBe(false);
    expect(getScope()).toEqual({});
  });

  it('setScope 跳过持久化与 window 事件派发，但仍通知订阅者', () => {
    const listener = jest.fn();
    const unsub = subscribeScope(listener);

    const next = setScope({ gameId: 'g', env: 'e' });

    expect(next).toEqual({ gameId: 'g', env: 'e' });
    expect(getScope()).toEqual({ gameId: 'g', env: 'e' });
    expect(listener).toHaveBeenCalledTimes(1);
    expect(listener).toHaveBeenCalledWith({ gameId: 'g', env: 'e' });
    unsub();
  });

  it('markScopeReady 无 window 时仍标记就绪并广播', () => {
    const listener = jest.fn();
    const unsub = subscribeScope(listener);

    markScopeReady();

    expect(isScopeReady()).toBe(true);
    expect(listener).toHaveBeenCalledTimes(1);
    unsub();
  });
});
