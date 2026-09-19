/**
 * MenuTree 交互与拖拽胶水覆盖：
 * 1. buildDropHandler 对 antd Tree drop info 的归一（pos 缺省容错、非法 drop 不回调）；
 * 2. 树节点 switcher 收起/再展开（onExpand 受控回写）；
 * 3. 操作区容器与「加子菜单」按钮阻断树节点选中冒泡（stopPropagation）；
 * 4. 删除 Popconfirm 点取消不触发 onDelete（onCancel 仅阻断冒泡）。
 */
import React from 'react';
import { render, screen, fireEvent, waitFor, within, configure } from '@testing-library/react';
import MenuTree, { buildDropHandler } from '../MenuTree';
import type { MenuItem } from '@/services/api/menu';

jest.mock('@umijs/max', () => ({
  // localizedText 渲染需要 locale；formatMessage 语义同 tests/setupTests.jsx
  useIntl: () => ({
    locale: 'zh-CN',
    formatMessage: ({ defaultMessage }: { defaultMessage: string }) => defaultMessage,
  }),
}));

// coverage instrumentation 下树收起动画（rc-motion deadline）较慢：放宽等待
configure({ asyncUtilTimeout: 5000 });
jest.setTimeout(20000);

/** 构造 MenuItem（缺省：顶级、可见、无子节点、排序 1）。 */
function makeNode(
  overrides: Partial<MenuItem> & Pick<MenuItem, 'id' | 'menuKey' | 'labels'>,
): MenuItem {
  return { parentId: null, sortOrder: 1, isVisible: true, children: [], ...overrides };
}

const treeItems: MenuItem[] = [
  makeNode({
    id: 1,
    menuKey: 'resource',
    labels: { 'zh-CN': '资源管理' },
    sortOrder: 1,
    children: [
      makeNode({
        id: 2,
        menuKey: 'player',
        labels: { 'zh-CN': '玩家管理' },
        parentId: 1,
        sortOrder: 1,
      }),
    ],
  }),
  makeNode({ id: 3, menuKey: 'secret', labels: { 'zh-CN': '机密' }, sortOrder: 2 }),
];

interface TreeHandlers {
  onEdit: jest.Mock;
  onDelete: jest.Mock;
  onMove: jest.Mock;
}

function renderTree(handlers: Partial<TreeHandlers> = {}): TreeHandlers {
  const merged: TreeHandlers = {
    onEdit: jest.fn(),
    onDelete: jest.fn(),
    onMove: jest.fn(),
    ...handlers,
  };
  render(
    <MenuTree
      items={treeItems}
      canManage
      onEdit={merged.onEdit}
      onDelete={merged.onDelete}
      onMove={merged.onMove}
    />,
  );
  return merged;
}

/** 取当前「资源管理」行的展开开关（受控展开下收起/展开状态由 class 表达）。 */
const getResourceSwitcher = (): HTMLElement => {
  const row = screen.getByText('资源管理').closest('.ant-tree-treenode');
  const switcher = row?.querySelector('.ant-tree-switcher');
  expect(switcher).not.toBeNull();
  return switcher as HTMLElement;
};

describe('buildDropHandler 拖拽胶水', () => {
  it('drop info 缺 pos 时容错按顶级定位，inside 放置回调 onMove', () => {
    const onMove = jest.fn();
    const handler = buildDropHandler(treeItems, onMove);

    // pos 缺省（TreeDropInfo 契约上可选）：'' → [''] → 视作顶级第 0 位，relative=0；
    // dropToGap=false → position='inside'：机密(3) 拖入 资源管理(1) 内部
    handler({ node: { key: 1 }, dragNode: { key: 3 }, dropPosition: 0, dropToGap: false });

    expect(onMove).toHaveBeenCalledTimes(1);
    expect(onMove).toHaveBeenCalledWith([{ id: 3, parentId: 1, sortOrder: 2 }]);
  });

  it('无效 drop（拖到自身）计算不出更新时不回调 onMove', () => {
    const onMove = jest.fn();
    const handler = buildDropHandler(treeItems, onMove);

    handler({ node: { key: 1, pos: '0' }, dragNode: { key: 1 }, dropPosition: 1, dropToGap: true });

    expect(onMove).not.toHaveBeenCalled();
  });
});

describe('MenuTree 节点交互', () => {
  it('点击 switcher 收起子级，再次点击恢复（onExpand 回写受控展开）', async () => {
    renderTree();
    expect(screen.getByText('玩家管理')).toBeInTheDocument();

    // 收起 资源管理：子级 玩家管理 随之卸载
    fireEvent.click(getResourceSwitcher());
    await waitFor(() => expect(getResourceSwitcher()).toHaveClass('ant-tree-switcher_close'));
    await waitFor(() => expect(screen.queryByText('玩家管理')).not.toBeInTheDocument());

    // 再次点击恢复展开
    fireEvent.click(getResourceSwitcher());
    await waitFor(() => expect(screen.getByText('玩家管理')).toBeInTheDocument());
  });

  it('点击操作区容器空白不选中树节点（Space onClick stopPropagation）', () => {
    renderTree();
    const addChild = screen.getAllByRole('button', { name: /加子菜单/ })[0];
    // 按钮所在 Space(size=0) 根节点（onClick 绑定处）
    const spaceRoot = addChild.closest('.ant-space-item')?.parentElement;
    expect(spaceRoot).not.toBeNull();

    fireEvent.click(spaceRoot as HTMLElement);
    // 冒泡被阻断：树节点不进入选中态（selected 类挂在 content wrapper 上）
    expect(document.querySelector('.ant-tree-node-selected')).toBeNull();

    // 对照组：直接点击标题区域会选中该节点
    fireEvent.click(screen.getByText('资源管理'));
    expect(document.querySelector('.ant-tree-node-selected')).not.toBeNull();
  });

  it('加子菜单按钮以 createChild 模式回调 onEdit，且不触发节点选中', () => {
    const handlers = renderTree();
    fireEvent.click(screen.getAllByRole('button', { name: /加子菜单/ })[0]);
    expect(handlers.onEdit).toHaveBeenCalledWith(
      expect.objectContaining({ id: 1, menuKey: 'resource' }),
      'createChild',
    );
    // 按钮 onClick 内 stopPropagation：不冒泡成节点选中
    expect(document.querySelector('.ant-tree-treenode.ant-tree-node-selected')).toBeNull();
  });

  it('删除 Popconfirm 点取消不触发删除（onCancel 仅阻断冒泡）', async () => {
    const handlers = renderTree();
    fireEvent.click(screen.getAllByRole('button', { name: /删\s*除/ })[0]);

    const popover = document.querySelector('.ant-popover');
    expect(popover).not.toBeNull();
    const cancel = within(popover as HTMLElement).getByRole('button', { name: /取\s*消|Cancel/ });
    fireEvent.click(cancel);

    expect(handlers.onDelete).not.toHaveBeenCalled();
    // 取消后确认气泡关闭（leave 动画下节点以 hidden 类残留 DOM，断言无可见气泡）
    await waitFor(() =>
      expect(document.querySelector('.ant-popover:not(.ant-popover-hidden)')).toBeNull(),
    );
  });
});
