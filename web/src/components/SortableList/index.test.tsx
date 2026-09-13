/** SortableList/SortableItem/DragHandle：列表渲染与 render-prop 契约（索引、
 * dragHandleProps 抓手样式）、键盘拖拽重排全链路（isDragging 半透明 + transform）、
 * handleDragEnd 四条守卫（over 为空 / 同 id / active 不在列表 / over 不在列表）、
 * externalDnd 注册到祖先 DndContext。
 *
 * mock 手法：@dnd-kit/core 的 closestCenter 被替换为按 globalThis.__dndOver
 * 决定碰撞结果的桩——undefined 委托真实实现，null 返回空（over 为空），
 * 字符串伪造指定 id 的碰撞，使守卫分支在 jsdom（无真实布局）下也可确定触达。
 * 键盘传感器：激活键 Space 派发在手柄上（走 listeners 的 onKeyDown activator），
 * 移动/落点键在传感器 setTimeout 挂载后派发到 document。 */
import React from 'react';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { DndContext, KeyboardSensor, closestCenter, useSensor, useSensors } from '@dnd-kit/core';
import type { DragEndEvent } from '@dnd-kit/core';
import { sortableKeyboardCoordinates } from '@dnd-kit/sortable';
import { DragHandle, SortableItem, SortableList } from './index';

jest.mock('@dnd-kit/core', () => {
  const actual = jest.requireActual('@dnd-kit/core');
  const override = (...args: Parameters<typeof actual.closestCenter>) => {
    const flag = (globalThis as { __dndOver?: string | null }).__dndOver;
    if (flag === undefined) return actual.closestCenter(...args);
    if (flag === null) return [];
    return [{ id: flag }];
  };
  return { ...actual, closestCenter: override };
});

interface Row {
  key: string;
  label: string;
}

const rows: Row[] = [
  { key: 'a', label: '行A' },
  { key: 'b', label: '行B' },
  { key: 'c', label: '行C' },
];

const rowRenderer = (
  item: Row,
  index: number,
  dragHandleProps: React.HTMLAttributes<HTMLElement>,
) => (
  <div data-testid={`row-${item.key}`}>
    <span data-testid={`handle-${item.key}`} {...dragHandleProps}>
      {item.label}
    </span>
    <span>{`序号${index}`}</span>
  </div>
);

/** 设定伪造的碰撞目标：undefined=真实检测，null=无碰撞，字符串=指定 id。 */
function setOver(over: string | null | undefined): void {
  (globalThis as { __dndOver?: string | null }).__dndOver = over;
}

/** 在手柄上按 Space 激活键盘拖拽（传感器随后异步把移动键监听挂到 document）。 */
async function startDrag(handleTestId: string): Promise<void> {
  fireEvent.keyDown(screen.getByTestId(handleTestId), { key: ' ', code: 'Space' });
  await new Promise((resolve) => setTimeout(resolve, 0));
}

/** 拖拽中按键（移动/落点）。 */
function pressDragKey(key: string, code: string): void {
  fireEvent.keyDown(document, { key, code });
}

/** 完整拖拽流：激活 → ArrowDown 移动一次（触发碰撞计算）→ Space 落点。 */
async function dragAndDrop(handleTestId: string): Promise<void> {
  await startDrag(handleTestId);
  pressDragKey('ArrowDown', 'ArrowDown');
  pressDragKey(' ', 'Space');
}

/** 渲染内部含 DndContext 的标准 SortableList。 */
function renderList(onReorder: jest.Mock, extra?: React.ReactNode) {
  return render(
    <SortableList items={rows} getKey={(r) => r.key} onReorder={onReorder}>
      {(item, index, dragHandleProps) => (
        <React.Fragment>
          {rowRenderer(item, index, dragHandleProps)}
          {item.key === 'c' ? extra : null}
        </React.Fragment>
      )}
    </SortableList>,
  );
}

beforeEach(() => {
  setOver(undefined);
});

afterEach(() => {
  setOver(undefined);
});

describe('SortableList', () => {
  it('渲染条目：label、序号与抓手样式契约（cursor grab / touchAction none）', () => {
    renderList(jest.fn());
    expect(screen.getByText('行A')).toBeInTheDocument();
    expect(screen.getByText('行B')).toBeInTheDocument();
    expect(screen.getByText('行C')).toBeInTheDocument();
    expect(screen.getByText('序号0')).toBeInTheDocument();
    expect(screen.getByText('序号2')).toBeInTheDocument();
    const handle = screen.getByTestId('handle-b');
    expect(handle.style.cursor).toBe('grab');
    expect(handle.style.touchAction).toBe('none');
  });

  it('DragHandle 导出即 HolderOutlined 图标', () => {
    render(<DragHandle />);
    expect(document.querySelector('.anticon-holder')).toBeInTheDocument();
  });

  it('键盘拖拽重排：拖 A 到 B 上，onReorder 收到重排后的数组', async () => {
    const onReorder = jest.fn();
    renderList(onReorder);
    setOver('b');
    await dragAndDrop('handle-a');
    await waitFor(() =>
      expect(onReorder).toHaveBeenCalledWith([
        { key: 'b', label: '行B' },
        { key: 'a', label: '行A' },
        { key: 'c', label: '行C' },
      ]),
    );
  });

  it('拖拽中：条目半透明 + transform，落点后还原', async () => {
    const onReorder = jest.fn();
    renderList(onReorder);
    const wrapper = screen.getByTestId('row-a').parentElement as HTMLElement;
    expect(wrapper.style.opacity).toBe('1');
    expect(wrapper.style.transform).toBe('');

    setOver('b');
    await startDrag('handle-a');
    pressDragKey('ArrowDown', 'ArrowDown');
    await waitFor(() => expect(wrapper.style.opacity).toBe('0.5'));
    expect(wrapper.style.transform).not.toBe('');

    pressDragKey(' ', 'Space');
    await waitFor(() =>
      expect(onReorder).toHaveBeenCalledWith([
        { key: 'b', label: '行B' },
        { key: 'a', label: '行A' },
        { key: 'c', label: '行C' },
      ]),
    );
  });

  it('守卫：over 为空（碰撞检测无结果）不触发重排', async () => {
    const onReorder = jest.fn();
    renderList(onReorder);
    setOver(null);
    await dragAndDrop('handle-a');
    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(onReorder).not.toHaveBeenCalled();
  });

  it('守卫：over 与 active 同 id 不触发重排', async () => {
    const onReorder = jest.fn();
    renderList(onReorder);
    setOver('a');
    await dragAndDrop('handle-a');
    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(onReorder).not.toHaveBeenCalled();
  });

  it('守卫：拖拽项 key 不在 items（外部条目）不触发重排', async () => {
    const onReorder = jest.fn();
    const extra = (
      <SortableItem id="outside-x">
        {(dragHandleProps) => (
          <div data-testid="row-outside">
            <span data-testid="handle-outside" {...dragHandleProps}>
              外部条目
            </span>
          </div>
        )}
      </SortableItem>
    );
    renderList(onReorder, extra);
    setOver('a');
    await dragAndDrop('handle-outside');
    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(onReorder).not.toHaveBeenCalled();
  });

  it('守卫：落点 id 不在 items（外部条目）不触发重排', async () => {
    const onReorder = jest.fn();
    const extra = (
      <SortableItem id="outside-x">
        {(dragHandleProps) => (
          <div data-testid="row-outside">
            <span data-testid="handle-outside" {...dragHandleProps}>
              外部条目
            </span>
          </div>
        )}
      </SortableItem>
    );
    renderList(onReorder, extra);
    setOver('outside-x');
    await dragAndDrop('handle-a');
    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(onReorder).not.toHaveBeenCalled();
  });

  it('externalDnd：不渲染内部 DndContext，事件注册到祖先上下文', async () => {
    const onDragEnd = jest.fn();
    const Harness = ({ children }: { children: React.ReactNode }) => {
      const sensors = useSensors(
        useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates }),
      );
      return (
        <DndContext sensors={sensors} collisionDetection={closestCenter} onDragEnd={onDragEnd}>
          {children}
        </DndContext>
      );
    };
    render(
      <Harness>
        <SortableList items={rows} getKey={(r) => r.key} onReorder={jest.fn()} externalDnd>
          {rowRenderer}
        </SortableList>
      </Harness>,
    );
    setOver('b');
    await dragAndDrop('handle-a');
    await waitFor(() => expect(onDragEnd).toHaveBeenCalledTimes(1));
    const event = onDragEnd.mock.calls[0][0] as DragEndEvent;
    expect(event.active.id).toBe('a');
    expect(event.over?.id).toBe('b');
  });
});
