/**
 * useScopeReload：全局 scope 切换联动的统一 hook 回归。
 * 核心契约——挂载不触发（避免与页面首拉重复）、scope 变化必触发、
 * 变化只触发一次（同一 scope 重复 set 不重拉）。
 */
import { act, renderHook } from '@testing-library/react';
import { useScopeReload } from '../useScopeReload';
import { setScope } from '@/stores/scope';

describe('useScopeReload', () => {
  const reload = jest.fn();

  beforeEach(() => {
    jest.clearAllMocks();
    act(() => setScope({ gameId: 'default', env: 'dev' }));
  });

  it('挂载时不触发 reload（页面自身 mount effect 负责首拉）', () => {
    renderHook(() => useScopeReload(reload));
    expect(reload).not.toHaveBeenCalled();
  });

  it('scope 切换触发 reload，并暴露最新 scope/scopeKey', () => {
    const { result } = renderHook(() => useScopeReload(reload));
    act(() => setScope({ gameId: 'game-a', env: 'prod' }));
    expect(reload).toHaveBeenCalledTimes(1);
    expect(result.current.scope).toEqual({ gameId: 'game-a', env: 'prod' });
    expect(result.current.scopeKey).toBe('game-a:prod');
  });

  it('同一 scope 重复 set 不重拉；换回旧 scope 再切换仍重拉', () => {
    renderHook(() => useScopeReload(reload));
    act(() => setScope({ gameId: 'game-a', env: 'prod' }));
    act(() => setScope({ gameId: 'game-a', env: 'prod' }));
    expect(reload).toHaveBeenCalledTimes(1);
    act(() => setScope({ gameId: 'default', env: 'dev' }));
    act(() => setScope({ gameId: 'game-b', env: 'dev' }));
    expect(reload).toHaveBeenCalledTimes(3);
  });

  it('reload 引用每次渲染更换也调用最新版本（ref 透传）', () => {
    const seen: number[] = [];
    const { rerender } = renderHook(({ fn }: { fn: () => void }) => useScopeReload(fn), {
      initialProps: { fn: () => seen.push(1) },
    });
    rerender({ fn: () => seen.push(2) });
    act(() => setScope({ gameId: 'game-c', env: 'dev' }));
    expect(seen).toEqual([2]);
  });
});
