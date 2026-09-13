/** useCanvasDnd（画布拖拽 hook）覆盖：handleDragStart 四态、模板拖入
 * （缺依赖警告/带参弹窗/空模板静默/blocked 三形/modal·container·链式三
 * plan/requiredFunctions 登记/悬空引用上报）、面板 basic 拖入（tabs 脚手架
 * /根级插入/modal-drop 仅 fnForm/编辑中弹窗两态）、容器与页签落点
 * （接受装页·不接受回退兄弟插入·无页回退）、画布内重排（根级跳过/列表外
 * 跳过/modal children 重排）。 */
import React, { type Dispatch, type RefObject, type SetStateAction } from 'react';
import { act, renderHook } from '@testing-library/react';
import { App } from 'antd';
import type { MessageInstance } from 'antd/es/message/interface';
import type { DragEndEvent, DragStartEvent } from '@dnd-kit/core';
import { useCanvasDnd } from '../useCanvasDnd';
import { resetRegistryForTest } from '../registry';
import { registerBuiltinComponents } from '../components/builtin';
import type { PageNode } from '../model';
import type { FunctionDescriptor } from '@/services/api/functions';
import type { ComponentTemplateDTO } from '../ComponentLibrary';

beforeAll(() => {
  resetRegistryForTest();
  registerBuiltinComponents();
});

const banFn: FunctionDescriptor = {
  id: 'player.ban',
  operation: 'update',
  resource: 'player',
  inputSchema: { type: 'object', properties: { playerId: { type: 'string' } } },
};

const fnFormNode = (id: string): PageNode => ({
  id,
  type: 'fnForm',
  props: { functionId: 'player.ban' },
});
const textNode = (id: string): PageNode => ({ id, type: 'text', props: { content: id } });

/** tabs 节点（自带 2 页） */
const tabsNode = (id: string, pages = 2): PageNode => ({
  id,
  type: 'tabs',
  props: {},
  children: Array.from({ length: pages }, (_, i) => ({
    id: `${id}-p${i}`,
    type: 'container' as const,
    props: { title: `页签 ${i + 1}` },
    children: [],
  })),
});

const containerNode = (id: string): PageNode => ({
  id,
  type: 'container',
  props: { title: '容器' },
  children: [],
});

const tpl = (
  tree: PageNode[],
  extra: Partial<ComponentTemplateDTO> = {},
): ComponentTemplateDTO => ({
  key: 'tpl.x',
  name: { 'zh-CN': '测试模板' },
  tree,
  builtin: false,
  ...extra,
});

interface SetupOptions {
  tree?: PageNode[];
  editingModalId?: string | null;
}

function setup(options: SetupOptions = {}) {
  const calls = {
    addChild: jest.fn(),
    registerFn: jest.fn(),
    setSelectedId: jest.fn(),
    setInsertTpl: jest.fn(),
    onTemplateUsed: jest.fn(),
    onDanglingRefs: jest.fn(),
  };
  const treeRef: RefObject<PageNode[]> = { current: options.tree ?? [] };
  const editingModalRef: RefObject<string | null> = { current: options.editingModalId ?? null };
  let realTree = options.tree ?? [];
  const setTree = jest.fn(((action: SetStateAction<PageNode[]>) => {
    realTree = typeof action === 'function' ? action(realTree) : action;
    treeRef.current = realTree;
  }) as Dispatch<SetStateAction<PageNode[]>>);
  const shell: { message?: MessageInstance } = {};

  // App.useApp() 须在 <App> 内才能拿到真实实例：capture 层包在 App 内，
  // hook 组件作为其 children 与 capture 共享同一 message 实例
  const Capture = ({ children }: { children: React.ReactNode }) => {
    shell.message = App.useApp().message;
    return <>{children}</>;
  };
  const CaptureApp = ({ children }: { children: React.ReactNode }) => (
    <App>
      <Capture>{children}</Capture>
    </App>
  );

  const { result } = renderHook(
    () =>
      useCanvasDnd({
        treeRef,
        editingModalRef,
        allFns: [banFn],
        addChild: calls.addChild,
        registerFn: calls.registerFn,
        setTree,
        setSelectedId: calls.setSelectedId,
        setInsertTpl: calls.setInsertTpl,
        onTemplateUsed: calls.onTemplateUsed,
        onDanglingRefs: calls.onDanglingRefs,
      }),
    { wrapper: CaptureApp },
  );

  const warnSpy = () => jest.spyOn(shell.message!, 'warning');

  const dragEnd = (data: unknown, activeId: string, over: string | null) =>
    act(() => {
      result.current.handleDragEnd({
        active: { id: activeId, data: { current: data } },
        over: over === null ? null : { id: over },
      } as unknown as DragEndEvent);
    });

  const dragStart = (data: unknown) =>
    act(() => {
      result.current.handleDragStart({
        active: { data: { current: data } },
      } as unknown as DragStartEvent);
    });

  return { ...calls, result, dragStart, dragEnd, warnSpy, tree: () => realTree, setTree };
}

describe('handleDragStart：面板拖拽项记录', () => {
  it('panel basic/fn/template 三态；canvas/无 data 置 null', () => {
    const h = setup();
    h.dragStart({ source: 'panel', kind: 'basic', basicType: 'text' });
    expect(h.result.current.dragItem).toEqual({ kind: 'basic', basicType: 'text' });

    h.dragStart({ source: 'panel', kind: 'fn', fn: banFn, componentType: 'fnForm' });
    expect(h.result.current.dragItem).toEqual({ kind: 'fn', fn: banFn, componentType: 'fnForm' });

    const t = tpl([textNode('a')]);
    h.dragStart({ source: 'panel', kind: 'template', tpl: t, missing: [] });
    expect(h.result.current.dragItem).toEqual({ kind: 'template', tpl: t, missing: [] });

    h.dragStart({ source: 'canvas' });
    expect(h.result.current.dragItem).toBeNull();
    h.dragStart(undefined);
    expect(h.result.current.dragItem).toBeNull();
  });
});

describe('handleDragEnd：模板拖入', () => {
  it('无落点：清 dragItem 无副作用', () => {
    const h = setup();
    h.dragStart({ source: 'panel', kind: 'basic', basicType: 'text' });
    h.dragEnd({ source: 'panel', kind: 'basic', basicType: 'text' }, 'draggable-1', null);
    expect(h.result.current.dragItem).toBeNull();
    expect(h.setTree).not.toHaveBeenCalled();
    expect(h.addChild).not.toHaveBeenCalled();
  });

  it('缺依赖函数：warning 并拦截', () => {
    const h = setup();
    const spy = h.warnSpy();
    h.dragEnd(
      { source: 'panel', kind: 'template', tpl: tpl([textNode('a')]), missing: ['f1', 'f2'] },
      'draggable-1',
      'canvas-root',
    );
    expect(spy).toHaveBeenCalledWith('缺少依赖函数：f1, f2');
    expect(h.setTree).not.toHaveBeenCalled();
  });

  it('带参数模板：保留落点走参数弹窗', () => {
    const h = setup();
    const t = tpl([textNode('a')], {
      params: [{ key: 'p', nodeId: 'a', prop: 'content' }],
    });
    h.dragEnd(
      { source: 'panel', kind: 'template', tpl: t, missing: [] },
      'draggable-1',
      'canvas-root',
    );
    expect(h.setInsertTpl).toHaveBeenCalledWith({ tpl: t, overId: 'canvas-root' });
    expect(h.setTree).not.toHaveBeenCalled();
  });

  it('空模板：静默返回（无插入无警告）', () => {
    const h = setup();
    const spy = h.warnSpy();
    h.dragEnd(
      { source: 'panel', kind: 'template', tpl: tpl([]), missing: [] },
      'draggable-1',
      'canvas-root',
    );
    expect(h.setTree).not.toHaveBeenCalled();
    expect(h.addChild).not.toHaveBeenCalled();
    expect(spy).not.toHaveBeenCalled();
  });

  it('弹窗落点但含非 fnForm：blocked 警告', () => {
    const h = setup();
    const spy = h.warnSpy();
    h.dragEnd(
      {
        source: 'panel',
        kind: 'template',
        tpl: tpl([{ id: 'x', type: 'fnTable', props: {} }]),
        missing: [],
      },
      'draggable-1',
      'modal-drop:m1',
    );
    expect(spy).toHaveBeenCalledWith('弹窗内只能放函数表单（V1）');
    expect(h.addChild).not.toHaveBeenCalled();
  });

  it('弹窗落点全 fnForm：装入弹窗并登记快照', () => {
    const h = setup();
    const t = tpl([fnFormNode('f1'), fnFormNode('f2')], { requiredFunctions: ['player.ban'] });
    h.dragEnd(
      { source: 'panel', kind: 'template', tpl: t, missing: [] },
      'draggable-1',
      'modal-drop:m1',
    );
    expect(h.addChild).toHaveBeenCalledTimes(2);
    expect(h.addChild).toHaveBeenCalledWith('m1', expect.objectContaining({ type: 'fnForm' }));
    expect(h.registerFn).toHaveBeenCalledWith(banFn);
    expect(h.setSelectedId).toHaveBeenCalled();
    expect(h.onTemplateUsed).toHaveBeenCalledWith(t);
  });

  it('requiredFunctions 未命中 allFns：不登记', () => {
    const h = setup();
    const t = tpl([fnFormNode('f1')], { requiredFunctions: ['no.such'] });
    h.dragEnd(
      { source: 'panel', kind: 'template', tpl: t, missing: [] },
      'draggable-1',
      'canvas-root',
    );
    expect(h.registerFn).not.toHaveBeenCalled();
  });

  it('容器落点（类型满足契约）：模板装入容器', () => {
    const h = setup({ tree: [containerNode('c1')] });
    const t = tpl([textNode('a'), { id: 'b', type: 'button', props: {} }]);
    h.dragEnd({ source: 'panel', kind: 'template', tpl: t, missing: [] }, 'draggable-1', 'c1');
    expect(h.addChild).toHaveBeenCalledTimes(2);
    expect(h.addChild).toHaveBeenCalledWith('c1', expect.objectContaining({ type: 'text' }));
  });

  it('容器落点含 tabs：blocked 警告（契约不允许）', () => {
    const h = setup({ tree: [containerNode('c1')] });
    const spy = h.warnSpy();
    h.dragEnd(
      { source: 'panel', kind: 'template', tpl: tpl([tabsNode('tt')]), missing: [] },
      'draggable-1',
      'c1',
    );
    expect(spy).toHaveBeenCalledWith(expect.stringContaining('容器不接受「tabs」'));
    expect(h.addChild).not.toHaveBeenCalled();
  });

  it('根级链式插入：保持模板顺序', () => {
    const h = setup();
    const t = tpl([textNode('a'), { id: 'b', type: 'button', props: {} }]);
    h.dragEnd(
      { source: 'panel', kind: 'template', tpl: t, missing: [] },
      'draggable-1',
      'canvas-root',
    );
    const tree = h.tree();
    expect(tree).toHaveLength(2);
    expect(tree[0]).toMatchObject({ type: 'text' });
    expect(tree[1]).toMatchObject({ type: 'button' });
  });

  it('节点后插入：insertAfter 锚点', () => {
    const h = setup({ tree: [textNode('keep1'), textNode('keep2')] });
    const t = tpl([{ id: 'b', type: 'button', props: {} }]);
    h.dragEnd({ source: 'panel', kind: 'template', tpl: t, missing: [] }, 'draggable-1', 'keep1');
    const tree = h.tree();
    expect(tree.map((n) => n.type)).toEqual(['text', 'button', 'text']);
  });

  it('悬空引用：实例化后上报 onDanglingRefs', () => {
    const h = setup();
    const t = tpl([{ id: 'b', type: 'button', props: { onClick: { target: 'gone-node' } } }]);
    h.dragEnd(
      { source: 'panel', kind: 'template', tpl: t, missing: [] },
      'draggable-1',
      'canvas-root',
    );
    expect(h.onDanglingRefs).toHaveBeenCalledWith(
      expect.arrayContaining([
        expect.objectContaining({ prop: 'onClick', kind: 'action', ref: 'gone-node' }),
      ]),
    );
  });
});

describe('handleDragEnd：面板 basic/fn 拖入', () => {
  const basic = (basicType: string) => ({ source: 'panel', kind: 'basic', basicType });
  const fnDrag = { source: 'panel', kind: 'fn', fn: banFn, componentType: 'fnForm' };

  it('tabs：自带两个空页签脚手架', () => {
    const h = setup();
    h.dragEnd(basic('tabs'), 'draggable-1', 'canvas-root');
    const [node] = h.tree();
    expect(node.type).toBe('tabs');
    expect(node.children).toHaveLength(2);
    expect(node.children?.[0].type).toBe('container');
  });

  it('text：根级末尾插入并选中', () => {
    const h = setup({ tree: [textNode('keep')] });
    h.dragEnd(basic('text'), 'draggable-1', 'canvas-root');
    expect(h.tree()).toHaveLength(2);
    expect(h.tree()[1].type).toBe('text');
    expect(h.setSelectedId).toHaveBeenCalledWith(expect.stringContaining(''));
  });

  it('fn 组件：登记函数契约并按 componentType 构造', () => {
    const h = setup();
    h.dragEnd(fnDrag, 'draggable-1', 'canvas-root');
    expect(h.registerFn).toHaveBeenCalledWith(banFn);
    expect(h.tree()[0].type).toBe('fnForm');
  });

  it('modal-drop 落 fnForm：装入弹窗', () => {
    const h = setup();
    h.dragEnd(basic('fnForm'), 'draggable-1', 'modal-drop:m1');
    expect(h.addChild).toHaveBeenCalledWith('m1', expect.objectContaining({ type: 'fnForm' }));
  });

  it('modal-drop 落非 fnForm：警告拦截', () => {
    const h = setup();
    const spy = h.warnSpy();
    h.dragEnd(basic('text'), 'draggable-1', 'modal-drop:m1');
    expect(spy).toHaveBeenCalledWith('弹窗内只能放函数表单（V1）');
    expect(h.addChild).not.toHaveBeenCalled();
  });

  it('编辑中弹窗落 fnForm：装入当前弹窗', () => {
    const h = setup({
      tree: [{ id: 'm1', type: 'modal', props: {}, children: [] }],
      editingModalId: 'm1',
    });
    h.dragEnd(basic('fnForm'), 'draggable-1', 'canvas-root');
    expect(h.addChild).toHaveBeenCalledWith('m1', expect.objectContaining({ type: 'fnForm' }));
  });

  it('编辑中弹窗落非 fnForm：警告拦截', () => {
    const h = setup({ editingModalId: 'm1' });
    const spy = h.warnSpy();
    h.dragEnd(basic('text'), 'draggable-1', 'canvas-root');
    expect(spy).toHaveBeenCalledWith('弹窗内只能放函数表单（V1）');
    expect(h.setTree).not.toHaveBeenCalled();
  });

  it('落点 tabs：接受子类型装入激活页', () => {
    const h = setup({ tree: [tabsNode('t1')] });
    h.dragEnd(basic('button'), 'draggable-1', 't1');
    expect(h.addChild).toHaveBeenCalledWith('t1-p0', expect.objectContaining({ type: 'button' }));
  });

  it('落点 tabs：不接受子类型回退为容器之后兄弟插入', () => {
    const h = setup({ tree: [tabsNode('t1')] });
    const spy = h.warnSpy();
    h.dragEnd(basic('tabs'), 'draggable-1', 't1');
    expect(spy).toHaveBeenCalledWith('页签内不接受「tabs」子组件，已放到页签容器之后');
    const tree = h.tree();
    expect(tree).toHaveLength(2);
    expect(tree[0].type).toBe('tabs');
    expect(tree[1].type).toBe('tabs');
  });

  it('落点 tabs 无页：回退兄弟插入', () => {
    const h = setup({ tree: [tabsNode('t1', 0)] });
    h.dragEnd(basic('button'), 'draggable-1', 't1');
    expect(h.addChild).not.toHaveBeenCalled();
    expect(h.tree()).toHaveLength(2);
  });

  it('落点 tabs：activeTab 指定页命中（非首页兜底）', () => {
    const t = tabsNode('t1');
    t.props.activeTab = 't1-p1';
    const h = setup({ tree: [t] });
    h.dragEnd(basic('button'), 'draggable-1', 't1');
    expect(h.addChild).toHaveBeenCalledWith('t1-p1', expect.objectContaining({ type: 'button' }));
  });

  it('落点 tabs 无 children 字段：pages 兜底空走回退兄弟插入', () => {
    // 手写异常形态（children 缺失）：pages ?? [] 空侧 → 无页回退
    const h = setup({ tree: [{ id: 't1', type: 'tabs', props: {} }] });
    h.dragEnd(basic('button'), 'draggable-1', 't1');
    expect(h.addChild).not.toHaveBeenCalled();
    expect(h.tree()).toHaveLength(2);
    expect(h.tree()[1].type).toBe('button');
  });

  it('落点 container：接受装入', () => {
    const h = setup({ tree: [containerNode('c1')] });
    h.dragEnd(basic('text'), 'draggable-1', 'c1');
    expect(h.addChild).toHaveBeenCalledWith('c1', expect.objectContaining({ type: 'text' }));
  });

  it('落点 container：不接受回退兄弟插入', () => {
    const h = setup({ tree: [containerNode('c1')] });
    const spy = h.warnSpy();
    h.dragEnd(basic('tabs'), 'draggable-1', 'c1');
    expect(spy).toHaveBeenCalledWith('容器不接受「tabs」子组件，已放到容器之后');
    const tree = h.tree();
    expect(tree).toHaveLength(2);
    expect(tree[1].type).toBe('tabs');
  });

  it('落点普通节点：插到其后', () => {
    const h = setup({ tree: [textNode('keep1'), textNode('keep2')] });
    h.dragEnd(basic('button'), 'draggable-1', 'keep1');
    expect(h.tree().map((n) => n.type)).toEqual(['text', 'button', 'text']);
  });
});

describe('handleDragEnd：画布内重排', () => {
  const canvas = (id: string) => ({ source: 'canvas', activeId: id });

  it('落点根：跳过', () => {
    const h = setup({ tree: [textNode('a')] });
    h.dragEnd(canvas('a'), 'a', 'canvas-root');
    expect(h.setTree).not.toHaveBeenCalled();
  });

  it('根级重排：移动到目标位置', () => {
    const h = setup({ tree: [textNode('a'), textNode('b'), textNode('c')] });
    h.dragEnd(canvas('a'), 'a', 'c');
    expect(h.tree().map((n) => n.id)).toEqual(['b', 'c', 'a']);
  });

  it('目标不在同级列表：跳过', () => {
    const h = setup({ tree: [textNode('a'), textNode('b')] });
    h.dragEnd(canvas('a'), 'a', 'not-exist');
    expect(h.setTree).not.toHaveBeenCalled();
  });

  it('弹窗内重排：只动 modal children', () => {
    const h = setup({
      tree: [
        textNode('root'),
        { id: 'm1', type: 'modal', props: {}, children: [fnFormNode('x'), fnFormNode('y')] },
      ],
      editingModalId: 'm1',
    });
    h.dragEnd(canvas('x'), 'x', 'y');
    const tree = h.tree();
    expect(tree[0].id).toBe('root');
    expect(tree[1].children?.map((n) => n.id)).toEqual(['y', 'x']);
  });

  it('弹窗内重排到原位：moveNode 原引用直接返回', () => {
    const h = setup({
      tree: [{ id: 'm1', type: 'modal', props: {}, children: [fnFormNode('x'), fnFormNode('y')] }],
      editingModalId: 'm1',
    });
    h.dragEnd(canvas('x'), 'x', 'x');
    expect(h.tree()[0].children?.map((n) => n.id)).toEqual(['x', 'y']);
  });

  it('editingModalId 无效（不在树中）：重排列表兜底空数组跳过', () => {
    const h = setup({ tree: [textNode('a'), textNode('b')], editingModalId: 'gone' });
    h.dragEnd(canvas('a'), 'a', 'b');
    expect(h.setTree).not.toHaveBeenCalled();
    expect(h.tree().map((n) => n.id)).toEqual(['a', 'b']);
  });

  it('根级重排到原位：返回原引用', () => {
    const h = setup({ tree: [textNode('a'), textNode('b')] });
    h.dragEnd(canvas('a'), 'a', 'a');
    expect(h.tree().map((n) => n.id)).toEqual(['a', 'b']);
  });
});
