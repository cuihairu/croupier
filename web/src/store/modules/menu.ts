import { useSyncExternalStore } from 'react';
import { listAccessibleMenus, type MenuItem } from '@/services/api/menu';

/**
 * 登录态菜单树的全局状态（T-M7）：登录/切换 scope 后拉取
 * GET /api/v1/menus/accessible，缓存供侧边栏（app.tsx menu.request）与
 * 其他消费方共享。单例 store + subscribe + useSyncExternalStore，
 * 不引入新状态库（与仓库 initialState/useModel 习惯互补）。
 */

let cached: MenuItem[] | null = null;
let inflight: Promise<MenuItem[]> | null = null;
const listeners = new Set<() => void>();

function emit(): void {
  for (const listener of listeners) {
    listener();
  }
}

/** 订阅缓存变化；返回取消订阅函数。 */
export function subscribeAccessibleMenus(listener: () => void): () => void {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

/** 读取缓存快照（未加载过时为 null）。 */
export function getCachedAccessibleMenus(): MenuItem[] | null {
  return cached;
}

/**
 * 拉取可访问菜单树。并发调用共享同一次请求；force=true 绕过缓存
 * （scope 切换 / 菜单事件 / 登录后均应强刷）。
 */
export async function refreshAccessibleMenus(force = false): Promise<MenuItem[]> {
  if (!force && cached) return cached;
  if (inflight) return inflight;
  inflight = listAccessibleMenus()
    .then((menus) => {
      cached = menus;
      emit();
      return menus;
    })
    .finally(() => {
      inflight = null;
    });
  return inflight;
}

/** 清空缓存（登出 / 身份失效时调用）。 */
export function resetAccessibleMenus(): void {
  cached = null;
  inflight = null;
  emit();
}

export interface AccessibleMenusState {
  menus: MenuItem[] | null;
  loaded: boolean;
  refresh: () => Promise<MenuItem[]>;
}

/** React hook：订阅可访问菜单缓存。 */
export function useAccessibleMenus(): AccessibleMenusState {
  const menus = useSyncExternalStore(subscribeAccessibleMenus, getCachedAccessibleMenus);
  return {
    menus,
    loaded: menus !== null,
    refresh: () => refreshAccessibleMenus(true),
  };
}
