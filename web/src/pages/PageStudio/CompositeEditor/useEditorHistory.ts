import {
  useCallback,
  useEffect,
  useRef,
  useState,
  type Dispatch,
  type SetStateAction,
} from 'react';
import type { PageNode } from './model';

/** 编辑器树历史 hook：撤销/重做（50 步快照）+ 树变更统一入口。
 * - setTree 是 history-aware 的唯一树写入通道（past/future 维护在此），
 *   函数式 action 基于 treeRef 求值，避免 setState updater 内副作用
 *   在 StrictMode 双调用下重复入栈。
 * - 撤销/重做后按存活节点清理选择状态（防悬空选中/多选/弹窗编辑态）——
 *   选择状态的 setter 由调用方注入（选择语义仍属编辑器主页）。
 * - Ctrl/Cmd+Z 撤销、Ctrl/Cmd+Shift+Z / Ctrl+Y 重做。 */
export function useEditorHistory({
  setSelectedId,
  setMultiIds,
  setEditingModalId,
}: {
  setSelectedId: Dispatch<SetStateAction<string | null>>;
  setMultiIds: Dispatch<SetStateAction<Set<string>>>;
  setEditingModalId: Dispatch<SetStateAction<string | null>>;
}) {
  const [tree, setTreeState] = useState<PageNode[]>([]);
  // 撤销/重做历史（快照栈，最多 50 步）
  const [past, setPast] = useState<PageNode[][]>([]);
  const [future, setFuture] = useState<PageNode[][]>([]);
  const treeRef = useRef(tree);
  treeRef.current = tree;

  /** history-aware setTree：所有树变更统一入口（撤销/重做安全网）。
   * 函数式 action 基于 treeRef 求值（避免 setState updater 内副作用
   * 在 StrictMode 双调用下重复入栈）。 */
  const setTree = useCallback((action: SetStateAction<PageNode[]>) => {
    const next =
      typeof action === 'function'
        ? (action as (prev: PageNode[]) => PageNode[])(treeRef.current)
        : action;
    if (next === treeRef.current) return;
    setPast((p) => [...p.slice(-49), treeRef.current]);
    setFuture(() => []);
    setTreeState(next);
    treeRef.current = next;
  }, []);

  /** 撤销/重做后按存活节点集清理选择状态（防悬空选中/多选/弹窗编辑态）。 */
  const pruneSelection = useCallback((nodes: PageNode[]) => {
    const alive = new Set<string>();
    const walkIds = (list: PageNode[]) => {
      for (const n of list) {
        alive.add(n.id);
        if (n.children) walkIds(n.children);
      }
    };
    walkIds(nodes);
    setSelectedId((cur) => (cur && alive.has(cur) ? cur : null));
    setMultiIds((prev) => {
      const next = new Set([...prev].filter((id) => alive.has(id)));
      return next.size === prev.size ? prev : next;
    });
    setEditingModalId((cur) => (cur && !alive.has(cur) ? null : cur));
  }, []);

  const undo = useCallback(() => {
    if (past.length === 0) return;
    const prev = past[past.length - 1];
    setPast((p) => p.slice(0, -1));
    setFuture((f) => [treeRef.current, ...f]);
    setTreeState(prev);
    treeRef.current = prev;
    pruneSelection(prev);
  }, [past.length, past, pruneSelection]);

  const redo = useCallback(() => {
    if (future.length === 0) return;
    const next = future[0];
    setFuture((f) => f.slice(1));
    setPast((p) => [...p, treeRef.current]);
    setTreeState(next);
    treeRef.current = next;
    pruneSelection(next);
  }, [future.length, future, pruneSelection]);

  // 快捷键：Ctrl/Cmd+Z 撤销、Ctrl/Cmd+Shift+Z / Ctrl+Y 重做
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (!(e.ctrlKey || e.metaKey) || e.key.toLowerCase() !== 'z') return;
      e.preventDefault();
      if (e.shiftKey) redo();
      else undo();
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [undo, redo]);

  return { tree, setTree, treeRef, undo, redo, past, future };
}
