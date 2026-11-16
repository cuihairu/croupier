import { useEffect, useRef, useState } from 'react';
import { getScope, getScopeVersion, subscribeScope, type Scope } from '@/stores/scope';

/**
 * 顶栏游戏/环境切换联动（#32/#35/#38 一族「选了没刷新」问题的统一入口）。
 *
 * 背景：scoped 接口的请求头注入（services/core/scope 的拦截器）保证服务端
 * 按当前 scope 过滤数据，但页面自己持有的列表只在挂载时拉一次——切游戏后
 * 展示的还是旧游戏数据。本 hook 订阅全局 scope store，scopeKey 变化时调用
 * reload 重拉。
 *
 * 语义约定：
 * - 挂载时不触发（页面自身的 mount effect 负责首拉，避免重复请求）；
 * - GameSelector 初始化完成的第一次 setScope 也算一次「变化」→ 会重拉，
 *   保证深链进入（挂载时 scope 尚未就绪、用空 scope/兜底 scope 首拉）的
 *   页面最终以正确 scope 展示；
 * - reload 用 ref 透传，调用方无需给它套 useCallback 稳定引用。
 *
 * 返回 scope/scopeKey：scopeKey 可直接塞进 ProTable 的 params（ProTable 在
 * params 变化时自动重发 request），fetch 型页面用 reload 回调。
 */
/** 只订阅、不触发回调：适用于把 scope 字段用作数据加载依赖（useCallback
 *  deps 含 gameId → 切换自然重建并重拉）或需要渲染 scope 文案的页面。 */
export function useScope(): { scope: Scope; scopeKey: string } {
  const [scope, setScope] = useState<Scope>(() => getScope());
  useEffect(() => subscribeScope((next) => setScope(next)), []);
  return { scope, scopeKey: `${scope.gameId || ''}:${scope.env || ''}` };
}

export function useScopeReload(reload: () => void | Promise<unknown>): {
  scope: Scope;
  scopeKey: string;
} {
  const { scope, scopeKey } = useScope();
  const reloadRef = useRef(reload);
  reloadRef.current = reload;
  const initialKeyRef = useRef(scopeKey);

  useEffect(() => {
    if (scopeKey === initialKeyRef.current) return;
    initialKeyRef.current = scopeKey;
    void reloadRef.current();
  }, [scopeKey]);

  return { scope, scopeKey };
}

/**
 * 作用域在途结果防护（GUX-004）：自定义 fetch 型 loader 用它丢弃「发起时
 * 的 scope 与落地时的 scope 不同」的响应——切游戏/环境后旧 scope 请求晚到
 * 不能覆盖新 scope 数据。ProTable 族（params 带 scopeKey）由 pro-table 内部
 * debounce + abort 保证错序防护，不需要本守卫。
 *
 * 用法：请求发起前 `fresh.begin()`，响应落地 setState 前判
 * `fresh.isStale()` 为 true 即丢弃（配合 mounted 守卫一起用）。
 */
export function useScopeFreshGuard(): { begin: () => void; isStale: () => boolean } {
  const capturedRef = useRef(0);
  return {
    begin: () => {
      capturedRef.current = getScopeVersion();
    },
    isStale: () => capturedRef.current !== getScopeVersion(),
  };
}
