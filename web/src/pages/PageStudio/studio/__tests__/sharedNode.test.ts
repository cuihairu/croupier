/**
 * @jest-environment node
 *
 * studio/shared SSR 守卫分支：node 环境下 window 未定义，currentFocusPageKey/
 * currentMountFlag/clearMountParam 安全退化（读参数空串/false、清参 no-op）。
 * jsdom 侧行为见 draftColumns.test.tsx。
 */
import { clearMountParam, currentFocusPageKey, currentMountFlag } from '../shared';

describe('studio/shared（SSR：无 window 守卫分支）', () => {
  it('node 环境前置校验：window 未定义', () => {
    expect(typeof window).toBe('undefined');
  });

  it('currentFocusPageKey 返回空串、currentMountFlag 返回 false', () => {
    expect(currentFocusPageKey()).toBe('');
    expect(currentMountFlag()).toBe(false);
  });

  it('clearMountParam 安全 no-op（不抛错）', () => {
    expect(() => clearMountParam()).not.toThrow();
  });
});
