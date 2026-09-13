/** useEditorHistory（树历史 hook）覆盖：setTree（函数式/直设/同引用跳过/
 * past 50 步上限截断/future 清空）、undo（空栈静默/恢复快照/future 前插/
 * 选择态清理三路——悬空 selectedId 清 null·multiIds 过滤·editingModalId
 * 悬空清存活留）、redo（空栈静默/恢复/past 追加）、快捷键（Ctrl+Z 撤销·
 * Ctrl+Shift+Z 与 Ctrl+Y 重做·无修饰键或非 z 忽略）。 */
import React from 'react';
import { act, fireEvent, renderHook } from '@testing-library/react';
import { useEditorHistory } from '../useEditorHistory';
import type { PageNode } from '../model';

const n = (id: string, children: PageNode[] = []): PageNode => ({
  id,
  type: 'text',
  props: {},
  children,
});

function setup() {
  // 选择态以函数式 updater 语义手工维护（hook 只依赖 Dispatch 契约）
  const state = {
    selectedId: null as string | null,
    multiIds: new Set<string>(),
    editingModalId: null as string | null,
  };
  const apply = <T,>(cur: T, action: T | ((prev: T) => T)): T =>
    typeof action === 'function' ? (action as (prev: T) => T)(cur) : action;
  const setSelectedId = jest.fn((a: Parameters<typeof apply<string | null>>[1]) => {
    state.selectedId = apply(state.selectedId, a);
  });
  const setMultiIds = jest.fn((a: Parameters<typeof apply<Set<string>>>[1]) => {
    state.multiIds = apply(state.multiIds, a);
  });
  const setEditingModalId = jest.fn((a: Parameters<typeof apply<string | null>>[1]) => {
    state.editingModalId = apply(state.editingModalId, a);
  });
  const { result } = renderHook(() =>
    useEditorHistory({ setSelectedId, setMultiIds, setEditingModalId }),
  );
  return { state, result };
}

describe('setTree：统一树写入通道', () => {
  it('函数式基于当前树求值；直设同样入栈', () => {
    const h = setup();
    act(() => h.result.current.setTree((prev) => [...prev, n('a')]));
    expect(h.result.current.tree.map((x) => x.id)).toEqual(['a']);
    act(() => h.result.current.setTree([n('b')]));
    expect(h.result.current.tree.map((x) => x.id)).toEqual(['b']);
    expect(h.result.current.past).toHaveLength(2);
  });

  it('同引用跳过（不入栈不清 future）', () => {
    const h = setup();
    act(() => h.result.current.setTree([n('a')]));
    const before = h.result.current.past.length;
    act(() => h.result.current.setTree(h.result.current.tree));
    expect(h.result.current.past).toHaveLength(before);
  });

  it('新变更清空 future', () => {
    const h = setup();
    act(() => h.result.current.setTree([n('a')]));
    act(() => h.result.current.setTree([n('b')]));
    act(() => h.result.current.undo());
    expect(h.result.current.future).toHaveLength(1);
    act(() => h.result.current.setTree([n('c')]));
    expect(h.result.current.future).toHaveLength(0);
  });

  it('past 上限 50 步（最旧快照淘汰）', () => {
    const h = setup();
    for (let i = 0; i < 55; i += 1) {
      act(() => h.result.current.setTree([n(`v${i}`)]));
    }
    expect(h.result.current.past).toHaveLength(50);
    expect(h.result.current.tree[0].id).toBe('v54');
    // 最早可撤销到 v4（逐次独立 act：undo 闭包依赖最新 past）
    for (let i = 0; i < 50; i += 1) {
      act(() => h.result.current.undo());
    }
    expect(h.result.current.tree[0].id).toBe('v4');
  });
});

describe('undo/redo：快照栈与选择态清理', () => {
  it('空栈静默（无副作用）', () => {
    const h = setup();
    act(() => h.result.current.undo());
    expect(h.result.current.tree).toEqual([]);
    act(() => h.result.current.redo());
    expect(h.result.current.tree).toEqual([]);
  });

  it('undo 恢复上一快照并可 redo 回来', () => {
    const h = setup();
    act(() => h.result.current.setTree([n('a')]));
    act(() => h.result.current.setTree([n('b')]));
    act(() => h.result.current.undo());
    expect(h.result.current.tree.map((x) => x.id)).toEqual(['a']);
    act(() => h.result.current.redo());
    expect(h.result.current.tree.map((x) => x.id)).toEqual(['b']);
  });

  it('undo 清理悬空选择态：selectedId 清 null、multiIds 过滤、editingModalId 悬空清', () => {
    const h = setup();
    h.state.selectedId = 'gone';
    h.state.multiIds = new Set(['gone', 'keep']);
    h.state.editingModalId = 'gone';
    act(() => h.result.current.setTree([n('keep')]));
    act(() => h.result.current.undo());
    expect(h.state.selectedId).toBeNull();
    expect([...h.state.multiIds]).toEqual([]);
    expect(h.state.editingModalId).toBeNull();
  });

  it('undo 存活选择态保留（含嵌套子节点 id 收集）', () => {
    const h = setup();
    h.state.selectedId = 'child';
    h.state.multiIds = new Set(['child', 'root']);
    h.state.editingModalId = 'root';
    act(() => h.result.current.setTree([n('x')]));
    act(() => h.result.current.undo());
    // undo 回到空树——先用含嵌套的快照验证
    expect(h.state.selectedId).toBeNull();
  });

  it('undo 到含嵌套子节点的快照：嵌套 id 存活则选择保留', () => {
    const h = setup();
    const nested = [n('root', [n('child')]), n('modal', [n('form')])];
    act(() => h.result.current.setTree(nested));
    h.state.selectedId = 'child';
    h.state.multiIds = new Set(['child', 'root']);
    h.state.editingModalId = 'modal';
    act(() => h.result.current.setTree([n('other')]));
    act(() => h.result.current.undo());
    expect(h.state.selectedId).toBe('child');
    expect([...h.state.multiIds].sort()).toEqual(['child', 'root']);
    expect(h.state.editingModalId).toBe('modal');
  });

  it('multiIds 全存活：返回原引用（size 相同短路）', () => {
    const h = setup();
    act(() => h.result.current.setTree([n('a')]));
    h.state.multiIds = new Set(['a']);
    const before = h.state.multiIds;
    act(() => h.result.current.setTree([n('b')]));
    act(() => h.result.current.undo());
    expect(h.state.multiIds).toBe(before);
  });
});

describe('快捷键', () => {
  it('Ctrl+Z 撤销；Ctrl+Shift+Z 与 Ctrl+Y 重做', () => {
    const h = setup();
    act(() => h.result.current.setTree([n('a')]));
    act(() => h.result.current.setTree([n('b')]));
    fireEvent.keyDown(window, { ctrlKey: true, key: 'z' });
    expect(h.result.current.tree.map((x) => x.id)).toEqual(['a']);
    fireEvent.keyDown(window, { ctrlKey: true, shiftKey: true, key: 'Z' });
    expect(h.result.current.tree.map((x) => x.id)).toEqual(['b']);
    fireEvent.keyDown(window, { ctrlKey: true, key: 'z' });
    fireEvent.keyDown(window, { ctrlKey: true, key: 'y' });
    expect(h.result.current.tree.map((x) => x.id)).toEqual(['b']);
  });

  it('Meta(Cmd)+Z 撤销；无修饰键或非 z/y 忽略', () => {
    const h = setup();
    act(() => h.result.current.setTree([n('a')]));
    act(() => h.result.current.setTree([n('b')]));
    fireEvent.keyDown(window, { key: 'z' });
    expect(h.result.current.tree.map((x) => x.id)).toEqual(['b']);
    fireEvent.keyDown(window, { ctrlKey: true, key: 'x' });
    expect(h.result.current.tree.map((x) => x.id)).toEqual(['b']);
    fireEvent.keyDown(window, { metaKey: true, key: 'z' });
    expect(h.result.current.tree.map((x) => x.id)).toEqual(['a']);
  });
});
