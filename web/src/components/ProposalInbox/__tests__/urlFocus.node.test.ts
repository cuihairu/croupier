/**
 * @jest-environment node
 *
 * urlFocus SSR 守卫分支：node 环境下 window 未定义，三个读 URL 工具
 * 安全退化为 no-op（读参数返回空串、清参/导航不触碰全局对象）。
 * jsdom 侧行为见 urlFocus.test.ts（此前 jsdom 无法重定义 window，
 * 该守卫分支在浏览器环境测试中不可达）。
 */
import {
  clearProposalKeyParam,
  currentProposalKey,
  currentResourceKey,
  navigateTo,
} from '../urlFocus';

describe('urlFocus（SSR：无 window 守卫分支）', () => {
  it('node 环境前置校验：window 未定义', () => {
    expect(typeof window).toBe('undefined');
  });

  it('currentResourceKey / currentProposalKey 返回空串', () => {
    expect(currentResourceKey()).toBe('');
    expect(currentProposalKey()).toBe('');
  });

  it('clearProposalKeyParam 安全 no-op（不触碰 history）', () => {
    expect(() => clearProposalKeyParam()).not.toThrow();
  });

  it('navigateTo 安全 no-op（不触发导航、不抛错）', () => {
    expect(() => navigateTo('/functions/pages?focus=k')).not.toThrow();
  });
});
