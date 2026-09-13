/** CanvasNode（画布节点）覆盖：spanOf 四路（缺省 24/非数字/<4/>24 归 24/
 * 合法原值）、标题回退链（title→content→def.name→type）、sectionKey Tag
 * （空串不显）、button 动作 Tag 两态与点击绑定、autoRun·dialog Tag、
 * 复制/删除/点击选中/手柄 stopPropagation、右键菜单六项（disabled 态与
 * 回调派发）、container 子节点渲染（选中边框/删除/右键 up·down 含边界
 * disabled/未注册子节点 null）、editHint 条件（depth 与类型）、右缘调宽
 * （px→24 栅格·clamp·tabs 无手柄）、未注册类型 null。 */
import React from 'react';
import { fireEvent, render, screen } from '@testing-library/react';
import { CanvasNode } from '../CanvasNode';
import { getComponent, resetRegistryForTest } from '../registry';
import { registerBuiltinComponents } from '../components/builtin';
import type { PageNode } from '../model';

beforeAll(() => {
  resetRegistryForTest();
  registerBuiltinComponents();
});

const dragHandleProps = { 'data-testid': 'drag-handle' } as React.HTMLAttributes<HTMLElement>;

const canvasEl = document.createElement('div');
const canvasWidthRef = { current: canvasEl } as React.RefObject<HTMLDivElement | null>;

interface RenderOptions {
  node?: PageNode;
  fn?: undefined;
  selected?: boolean;
  depth?: number;
  selectedChildId?: string | null;
  handlers?: Partial<
    Record<
      | 'onSelect'
      | 'onDelete'
      | 'onDuplicate'
      | 'onSpanChange'
      | 'onChildSelect'
      | 'onChildDelete'
      | 'onChildMove'
      | 'onMoveUp'
      | 'onMoveDown'
      | 'onSelectParent'
      | 'onSaveAsComponent',
      jest.Mock
    >
  >;
}

function renderNode(options: RenderOptions = {}) {
  const h = options.handlers ?? {};
  const props = {
    node: options.node ?? { id: 'n1', type: 'text', props: { content: '说明文字' } },
    fn: options.fn,
    selected: options.selected ?? false,
    depth: options.depth ?? 0,
    onSelect: h.onSelect ?? jest.fn(),
    onDelete: h.onDelete ?? jest.fn(),
    onDuplicate: h.onDuplicate ?? jest.fn(),
    onSpanChange: h.onSpanChange ?? jest.fn(),
    onSelectParent: h.onSelectParent,
    onMoveUp: h.onMoveUp,
    onMoveDown: h.onMoveDown,
    onSaveAsComponent: h.onSaveAsComponent,
    selectedChildId: options.selectedChildId as string | null | undefined,
    onChildSelect: h.onChildSelect,
    onChildDelete: h.onChildDelete,
    onChildMove: h.onChildMove,
    dragHandleProps,
    canvasWidthRef,
  };
  return render(<CanvasNode {...props} />);
}

/** 右缘调宽手柄（col-resize 定位）。 */
const resizeHandle = (c: HTMLElement) => c.querySelector('[style*="col-resize"]') as HTMLElement;

/** jsdom 无 PointerEvent：fireEvent.pointerDown 会丢 clientX，
 * 用 MouseEvent 构造器派发（事件名任意，React 合成事件读 nativeEvent.clientX）。 */
const pointer = (el: Element | Window, type: string, x?: number) =>
  el.dispatchEvent(new MouseEvent(type, { clientX: x, bubbles: true }));

/** 打开右键菜单并点菜单项。 */
const clickMenuItem = (label: string) => {
  const item = screen
    .getAllByText(label)
    .find((el) => el.closest('.ant-dropdown-menu-item')) as HTMLElement;
  fireEvent.click(item);
};

describe('渲染', () => {
  it('未注册类型：不渲染', () => {
    const { container } = renderNode({
      node: { id: 'x', type: 'nope' as PageNode['type'], props: {} },
    });
    expect(container).toBeEmptyDOMElement();
  });

  it('标题回退链：title 优先 → 无 title 用 content → 再无用组件名', () => {
    renderNode({ node: { id: 'a', type: 'text', props: { title: '标题优先', content: '内容' } } });
    expect(screen.getByText('标题优先')).toBeInTheDocument();

    const { unmount } = renderNode({ node: { id: 'b', type: 'text', props: {} } });
    // text 无 title 无 content → def.name（Preview 空态也显示组件名，多元素并存）
    const textDef = getComponent('text');
    expect(screen.getAllByText(String(textDef?.name ?? '')).length).toBeGreaterThanOrEqual(1);
    unmount();
  });

  it('sectionKey Tag：非空显示、空串不显示', () => {
    const { unmount } = renderNode({
      node: { id: 'a', type: 'text', props: { content: 'x', sectionKey: 'players' } },
    });
    expect(screen.getByText('⌗players')).toBeInTheDocument();
    unmount();

    renderNode({ node: { id: 'b', type: 'text', props: { content: 'x', sectionKey: '' } } });
    expect(screen.queryByText(/^⌗/)).not.toBeInTheDocument();
  });

  it('button 动作 Tag 两态；点击绑定派发 onSelect', () => {
    const onSelect = jest.fn();
    const { unmount } = renderNode({
      node: { id: 'b1', type: 'button', props: { title: '按钮' } },
      handlers: { onSelect },
    });
    fireEvent.click(screen.getByText('点击绑定动作 →'));
    expect(onSelect).toHaveBeenCalledTimes(1);
    unmount();

    renderNode({
      node: {
        id: 'b2',
        type: 'button',
        props: { title: '按钮', onClick: { kind: 'closeModal', target: '', params: {} } },
      },
    });
    expect(screen.getByText('已绑定动作')).toBeInTheDocument();
  });

  it('fnForm dialog 形态：弹窗 Tag', () => {
    renderNode({
      node: { id: 'f1', type: 'fnForm', props: { title: '弹窗表单', display: 'dialog' } },
    });
    expect(screen.getByText('弹窗')).toBeInTheDocument();
  });

  it('autoRun 与 editHint 按类型/深度条件', () => {
    const { unmount } = renderNode({
      node: { id: 't1', type: 'fnTable', props: { autoRun: true } },
    });
    expect(screen.getByText('自动')).toBeInTheDocument();
    expect(screen.getByText('拖手柄排序 · 拖右缘调宽 · 点击配置')).toBeInTheDocument();
    unmount();

    // modal/text/tabs 不显示 editHint
    const { unmount: u2 } = renderNode({ node: { id: 't2', type: 'text', props: {} } });
    expect(screen.queryByText('拖手柄排序 · 拖右缘调宽 · 点击配置')).not.toBeInTheDocument();
    u2();
    const { unmount: u3 } = renderNode({
      node: { id: 't3', type: 'modal', props: {}, children: [] },
    });
    expect(screen.queryByText('拖手柄排序 · 拖右缘调宽 · 点击配置')).not.toBeInTheDocument();
    u3();
    // depth>0 也不显示
    const { unmount: u4 } = renderNode({
      node: { id: 't4', type: 'fnTable', props: {} },
      depth: 1,
    });
    expect(screen.queryByText('拖手柄排序 · 拖右缘调宽 · 点击配置')).not.toBeInTheDocument();
    u4();
  });

  it('选中态：卡片边框样式区分', () => {
    const { container, unmount } = renderNode({ selected: true });
    const card = container.querySelector('.ant-card') as HTMLElement;
    expect(card.style.borderColor).toBe('rgb(22, 119, 255)');
    unmount();
    const { container: c2 } = renderNode({ selected: false });
    const card2 = c2.querySelector('.ant-card') as HTMLElement;
    expect(card2.style.borderColor).toBe('');
  });
});

describe('基础交互', () => {
  it('点击卡片选中（透传事件）；拖拽手柄点击不触发选中', () => {
    const onSelect = jest.fn();
    const { container } = renderNode({ handlers: { onSelect } });
    fireEvent.click(container.querySelector('.ant-card') as HTMLElement);
    expect(onSelect).toHaveBeenCalledTimes(1);

    fireEvent.click(screen.getByTestId('drag-handle'));
    expect(onSelect).toHaveBeenCalledTimes(1);
  });

  it('extra 操作：复制与删除', () => {
    const onDuplicate = jest.fn();
    const onDelete = jest.fn();
    const { container } = renderNode({ handlers: { onDuplicate, onDelete } });
    const extra = container.querySelector('.ant-card-extra') as HTMLElement;
    const buttons = extra.querySelectorAll('button');
    fireEvent.click(buttons[0]);
    expect(onDuplicate).toHaveBeenCalledTimes(1);
    fireEvent.click(buttons[1]);
    expect(onDelete).toHaveBeenCalledTimes(1);
  });

  it('右键菜单：上移/下移/父容器/保存组件禁用态（未传回调）', () => {
    const { container } = renderNode({
      handlers: { onMoveUp: jest.fn(), onMoveDown: jest.fn() },
    });
    fireEvent.contextMenu(container.querySelector('.ant-card') as HTMLElement);
    const disabled = screen
      .getAllByText('选择父容器')
      .filter((el) => el.closest('.ant-dropdown-menu-item'));
    const parentItem = disabled[0].closest('.ant-dropdown-menu-item') as HTMLElement;
    expect(parentItem.className).toContain('disabled');
    const saveItem = screen
      .getAllByText('保存为组件')[0]
      .closest('.ant-dropdown-menu-item') as HTMLElement;
    expect(saveItem.className).toContain('disabled');
  });

  it('右键菜单：六项回调全派发（stopPropagation 不冒泡）', () => {
    const h = {
      onSelectParent: jest.fn(),
      onSaveAsComponent: jest.fn(),
      onDuplicate: jest.fn(),
      onDelete: jest.fn(),
      onMoveUp: jest.fn(),
      onMoveDown: jest.fn(),
    };
    const { container } = renderNode({ handlers: h });
    fireEvent.contextMenu(container.querySelector('.ant-card') as HTMLElement);
    clickMenuItem('上移');
    expect(h.onMoveUp).toHaveBeenCalledTimes(1);
    fireEvent.contextMenu(container.querySelector('.ant-card') as HTMLElement);
    clickMenuItem('下移');
    expect(h.onMoveDown).toHaveBeenCalledTimes(1);
    fireEvent.contextMenu(container.querySelector('.ant-card') as HTMLElement);
    clickMenuItem('选择父容器');
    expect(h.onSelectParent).toHaveBeenCalledTimes(1);
    fireEvent.contextMenu(container.querySelector('.ant-card') as HTMLElement);
    clickMenuItem('保存为组件');
    expect(h.onSaveAsComponent).toHaveBeenCalledTimes(1);
    fireEvent.contextMenu(container.querySelector('.ant-card') as HTMLElement);
    clickMenuItem('复制');
    expect(h.onDuplicate).toHaveBeenCalledTimes(1);
    fireEvent.contextMenu(container.querySelector('.ant-card') as HTMLElement);
    clickMenuItem('删除');
    expect(h.onDelete).toHaveBeenCalledTimes(1);
  });
});

describe('右缘调宽', () => {
  it('px → 24 栅格换算（clientWidth 480：40px = 2 格）', () => {
    const onSpanChange = jest.fn();
    const { container } = renderNode({
      node: { id: 'a', type: 'fnTable', props: { span: 12 } },
      handlers: { onSpanChange },
    });
    Object.defineProperty(canvasEl, 'clientWidth', { value: 480, configurable: true });
    pointer(resizeHandle(container), 'pointerdown', 200);
    pointer(window, 'pointermove', 240);
    pointer(window, 'pointerup');
    expect(onSpanChange).toHaveBeenLastCalledWith(14);
  });

  it('clamp 上限 24（clientWidth 为 0 时 total=1：1px 即满格）', () => {
    const onSpanChange = jest.fn();
    const { container } = renderNode({ handlers: { onSpanChange } });
    Object.defineProperty(canvasEl, 'clientWidth', { value: 0, configurable: true });
    pointer(resizeHandle(container), 'pointerdown', 100);
    pointer(window, 'pointermove', 101);
    expect(onSpanChange).toHaveBeenLastCalledWith(24);
    pointer(window, 'pointerup');
  });

  it('clamp 下限 4；pointerup 后不再响应 move', () => {
    const onSpanChange = jest.fn();
    const { container } = renderNode({
      node: { id: 'a', type: 'fnTable', props: { span: 12 } },
      handlers: { onSpanChange },
    });
    Object.defineProperty(canvasEl, 'clientWidth', { value: 480, configurable: true });
    pointer(resizeHandle(container), 'pointerdown', 200);
    pointer(window, 'pointermove', -2000);
    expect(onSpanChange).toHaveBeenLastCalledWith(4);
    pointer(window, 'pointerup');
    // up 后监听已移除：后续 move 不再派发
    pointer(window, 'pointermove', 5000);
    expect(onSpanChange).toHaveBeenCalledTimes(1);
  });

  it('resize 期间 ref 为空：move 静默跳过', () => {
    const onSpanChange = jest.fn();
    const saved = canvasWidthRef.current;
    const { container } = renderNode({
      node: { id: 'a', type: 'fnTable', props: { span: 12 } },
      handlers: { onSpanChange },
    });
    canvasWidthRef.current = null;
    pointer(resizeHandle(container), 'pointerdown', 200);
    pointer(window, 'pointermove', 400);
    expect(onSpanChange).not.toHaveBeenCalled();
    pointer(window, 'pointerup');
    canvasWidthRef.current = saved;
  });

  it('tabs 无调宽手柄', () => {
    const { container } = renderNode({
      node: { id: 'tb', type: 'tabs', props: { title: '页签' }, children: [] },
    });
    expect(resizeHandle(container)).toBeNull();
  });

  it('手柄点击 stopPropagation：不触发卡片选中', () => {
    const onSelect = jest.fn();
    const { container } = renderNode({ handlers: { onSelect } });
    fireEvent.click(resizeHandle(container));
    expect(onSelect).not.toHaveBeenCalled();
  });
});

describe('container 子节点', () => {
  const containerNode = (): PageNode => ({
    id: 'c1',
    type: 'container',
    props: { title: '容器' },
    children: [
      { id: 'k1', type: 'text', props: { content: '子一' } },
      { id: 'k2', type: 'text', props: { content: '子二' } },
      { id: 'bad', type: 'nope2' as PageNode['type'], props: {} },
    ],
  });

  it('渲染子节点（未注册子跳过）；spanOf 作用于 Col', () => {
    const { container } = renderNode({ node: containerNode() });
    // Preview 与装饰层可能同文本并存，断言存在即可
    expect(screen.getAllByText('子一').length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByText('子二').length).toBeGreaterThanOrEqual(1);
    const cols = container.querySelectorAll('.ant-col');
    expect(cols).toHaveLength(2);
  });

  it('子节点 Col span：非法/越界归 24，合法原值', () => {
    const node = containerNode();
    (node.children as PageNode[])[0] = { id: 'k1', type: 'text', props: { content: 'x', span: 2 } };
    (node.children as PageNode[])[1] = {
      id: 'k2',
      type: 'text',
      props: { content: 'y', span: '8' },
    };
    const { container } = renderNode({ node });
    const cols = container.querySelectorAll('.ant-col');
    expect(cols[0].className).toContain('ant-col-24');
    expect(cols[1].className).toContain('ant-col-8');
  });

  /** 子节点交互块（padding:6px 的 dashed div；Container.Preview 空态占位
   * 也是 dashed 但无 padding——按文本定位会命中两处，块本身才有交互）。 */
  const childBlock = (c: HTMLElement, idx: number) =>
    c.querySelectorAll('[style*="padding: 6px"]')[idx] as HTMLElement;

  /** 找子节点右键菜单项（多次 contextMenu 的菜单 portal 共存，取最新一个）。 */
  const lastMenuItem = (label: string) => {
    const items = screen.getAllByText(label).filter((el) => el.closest('.ant-dropdown-menu-item'));
    return items[items.length - 1] as HTMLElement;
  };

  /** 点子节点右键菜单项。 */
  const clickChildMenuItem = (label: string) => {
    fireEvent.click(lastMenuItem(label));
  };

  it('子节点点击选中与选中边框', () => {
    const onChildSelect = jest.fn();
    const { container, unmount } = renderNode({
      node: containerNode(),
      handlers: { onChildSelect },
    });
    fireEvent.click(childBlock(container, 0));
    expect(onChildSelect).toHaveBeenCalledWith('k1');
    unmount();

    const { container: c2 } = renderNode({ node: containerNode() });
    // 未选中子块为虚线边框
    expect(c2.querySelectorAll('[style*="dashed"]').length).toBeGreaterThan(0);

    // selectedChildId 命中：实线蓝框
    const { container: c3 } = renderNode({
      node: containerNode(),
      selectedChildId: 'k1',
    });
    const solid = c3.querySelector('[style*="1px solid rgb(22, 119, 255)"]');
    expect(solid).not.toBeNull();
  });

  it('子节点删除按钮（行内 + 右键菜单）', () => {
    const onChildDelete = jest.fn();
    const { container } = renderNode({ node: containerNode(), handlers: { onChildDelete } });
    // 子块内的行内删除按钮（块内唯一「删除」文本按钮）
    const del = [...childBlock(container, 1).querySelectorAll('button')].find(
      (btn) => btn.textContent === '删除',
    ) as HTMLElement;
    fireEvent.click(del);
    expect(onChildDelete).toHaveBeenCalledWith('k2');

    // 右键菜单删除
    fireEvent.contextMenu(childBlock(container, 0));
    clickChildMenuItem('删除');
    expect(onChildDelete).toHaveBeenCalledWith('k1');
  });

  it('子节点右键菜单：上移/下移 + 首末 disabled + 派发方向', () => {
    const onChildMove = jest.fn();
    // disabled 判定按原始 children 索引：本用例不含未注册占位节点。
    // 子菜单非受控且互不关闭：每个断言独立渲染，避免上一菜单残留干扰点击。
    const node = (): PageNode => {
      const n = containerNode();
      (n.children as PageNode[]).pop();
      return n;
    };
    const openMenu = (blockIdx: number) => {
      const utils = renderNode({ node: node(), handlers: { onChildMove } });
      fireEvent.contextMenu(childBlock(utils.container, blockIdx));
      return utils;
    };

    // 首子节点上移禁用（点击不派发）
    let utils = openMenu(0);
    expect(
      (lastMenuItem('上移').closest('.ant-dropdown-menu-item') as HTMLElement).className,
    ).toContain('disabled');
    fireEvent.click(lastMenuItem('上移'));
    expect(onChildMove).not.toHaveBeenCalled();
    utils.unmount();
    // 末子节点下移禁用
    utils = openMenu(1);
    expect(
      (lastMenuItem('下移').closest('.ant-dropdown-menu-item') as HTMLElement).className,
    ).toContain('disabled');
    utils.unmount();
    // 子一向下移动
    utils = openMenu(0);
    clickChildMenuItem('下移');
    expect(onChildMove).toHaveBeenCalledWith('k1', 1);
    utils.unmount();
    // 子二向上移动
    utils = openMenu(1);
    clickChildMenuItem('上移');
    expect(onChildMove).toHaveBeenCalledWith('k2', -1);
    utils.unmount();
  });

  it('空 children / 无 children：不渲染子区块', () => {
    const { unmount } = renderNode({
      node: { id: 'c', type: 'container', props: {}, children: [] },
    });
    expect(screen.queryByText('删除')).not.toBeInTheDocument();
    unmount();
    renderNode({ node: { id: 'c2', type: 'container', props: {} } });
    expect(screen.queryByText('删除')).not.toBeInTheDocument();
  });

  it('子块包装层点击 stopPropagation：选中与主节点点击均不触发', () => {
    const onSelect = jest.fn();
    const onChildSelect = jest.fn();
    const { container } = renderNode({
      node: containerNode(),
      handlers: { onSelect, onChildSelect },
    });
    fireEvent.click(childBlock(container, 0).parentElement as HTMLElement);
    expect(onChildSelect).not.toHaveBeenCalled();
    expect(onSelect).not.toHaveBeenCalled();
  });
});
