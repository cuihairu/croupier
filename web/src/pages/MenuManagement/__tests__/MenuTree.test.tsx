/**
 * MenuTree 交互与拖拽胶水覆盖：
 * 1. buildDropHandler 对 antd Tree drop info 的归一（pos 缺省容错、非法 drop 不回调）；
 * 2. 树节点 switcher 收起/再展开（onExpand 受控回写）；
 * 3. 操作区容器与「加子菜单」按钮阻断树节点选中冒泡（stopPropagation）；
 * 4. 删除 Popconfirm 点取消不触发 onDelete（onCancel 仅阻断冒泡）；
 * 5. 挂载页面排序比较器全分支（order 兜底 0 / pageKey 决胜）、archived 徽标、
 *    sortOrder=0 不渲染排序标签。
 *
 * 分支覆盖登记（v8 coverage）：L279 `nodeDraggable` 的 `node.key ?? ''` nullish
 * 侧结构性不可达——treeData 由本组件构造，key 恒为 String(id) / `page:${pageKey}`
 * 字符串，不存在缺失 key 的节点。
 */
import React from 'react';
import { render, screen, fireEvent, waitFor, within, configure } from '@testing-library/react';
import MenuTree, { buildDropHandler, type MenuMountedPage } from '../MenuTree';
import type { MenuItem } from '@/services/api/menu';

/**
 * 收集 formatMessage 收到的 ICU 占位符插值调用。
 *
 * 真实 react-intl 在 defaultMessage 含 `{order}` 却没有 values 时会抛
 * MISSING_VALUE 解析错误（onError 被调用），并把 `{order}` 原样渲染。此前本文件
 * 的 mock 只回 defaultMessage、丢弃 values，等于替组件把「漏传 values」的错误
 * 藏起来了——组件因此长期停留在 `formatMessage(...).replace('{order}', n)` 的
 * 错误写法上（docs/BUGS.md BUG-004）。这里保留 values 并记录调用，让回归测试
 * 能真正断言插值路径。
 */
const intlCalls: { id: string; defaultMessage: string; values?: Record<string, unknown> }[] = [];

jest.mock('@umijs/max', () => ({
  // localizedText 渲染需要 locale；formatMessage 语义同 tests/setupTests.jsx
  //（按 values 做 {placeholder} 插值）
  useIntl: () => ({
    locale: 'zh-CN',
    formatMessage: (
      descriptor: { id: string; defaultMessage: string },
      values?: Record<string, unknown>,
    ) => {
      intlCalls.push({ id: descriptor.id, defaultMessage: descriptor.defaultMessage, values });
      return Object.entries(values || {}).reduce(
        (msg, [key, val]) => msg.split(`{${key}}`).join(String(val)),
        descriptor.defaultMessage,
      );
    },
  }),
}));

/** 渲染前清空 intl 调用记录。 */
function resetIntlCalls(): void {
  intlCalls.length = 0;
}

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

function renderTree(
  handlers: Partial<TreeHandlers & { pages?: MenuMountedPage[] }> = {},
): TreeHandlers {
  const merged: TreeHandlers = {
    onEdit: jest.fn(),
    onDelete: jest.fn(),
    onMove: jest.fn(),
    ...handlers,
  };
  const { pages, ...treeHandlers } = handlers;
  render(
    <MenuTree
      items={treeItems}
      pages={pages ?? []}
      canManage
      onEdit={merged.onEdit}
      onDelete={merged.onDelete}
      onMove={merged.onMove}
    />,
  );
  return merged;
}

const mountedPages: MenuMountedPage[] = [
  {
    pageKey: 'operation--player.ban',
    type: 'operation',
    menuId: 2,
    title: { 'zh-CN': '封禁玩家' },
    status: 'published',
    order: 3,
  },
  {
    pageKey: 'resource--player',
    type: 'resource',
    menuId: 2,
    title: { 'zh-CN': '玩家列表' },
    status: 'draft',
  },
];

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

describe('挂载页面叶子展示', () => {
  it('已发布/草稿页面作为只读叶子挂在菜单节点下，带状态徽标与排序值', () => {
    renderTree({ pages: mountedPages });
    expect(screen.getByText('封禁玩家')).toBeInTheDocument();
    expect(screen.getByText('玩家列表')).toBeInTheDocument();
    expect(screen.getByText('已发布')).toBeInTheDocument();
    expect(screen.getByText('草稿')).toBeInTheDocument();
    expect(screen.getByText('发布后才会出现在控制台导航')).toBeInTheDocument();
    expect(screen.getByText('排序 3')).toBeInTheDocument();
  });

  /**
   * BUG-004 回归：排序标签必须经 intl values 传 `{order}`。
   *
   * 组件一度写成 `formatMessage({ defaultMessage: '排序 {order}' }).replace(...)`：
   * 真实 react-intl 会因缺 values 抛 MISSING_VALUE 解析错误（console 报错），
   * 再由 .replace 兜住可见文本。断言两条：(1) 占位符经 values 传入；(2) 渲染
   * 结果由 intl 插值而来，页面不再自行 replace。
   */
  it('排序标签的 {order} 经 intl values 传入，不靠组件侧 replace 兜底', () => {
    resetIntlCalls();
    renderTree({ pages: mountedPages });

    const orderCalls = intlCalls.filter((c) => c.id === 'pages.menuManagement.page.order');
    expect(orderCalls.length).toBeGreaterThan(0);
    // 每个含 {order} 的调用都必须带上 order 值——漏传即触发 MISSING_VALUE。
    for (const call of orderCalls) {
      expect(call.defaultMessage).toContain('{order}');
      expect(call.values).toHaveProperty('order');
      expect(typeof call.values?.order).toBe('number');
    }
    // 插值后的可见文本由 intl 产出
    expect(screen.getByText('排序 3')).toBeInTheDocument();
  });

  it('页面叶子不参与拖拽（nodeDraggable 排除 page: 前缀）', () => {
    renderTree({ pages: mountedPages });
    const pageNode = screen.getByText('封禁玩家').closest('.ant-tree-treenode');
    const draggableEl = pageNode?.querySelector('[draggable="true"]');
    expect(draggableEl).toBeNull();
  });

  it('点击「编辑页面」回调 onEditPage(pageKey)', () => {
    const onEditPage = jest.fn();
    // 直接渲染带 onEditPage 的树
    render(
      <MenuTree
        items={treeItems}
        pages={mountedPages}
        canManage
        onEdit={jest.fn()}
        onDelete={jest.fn()}
        onMove={jest.fn()}
        onEditPage={onEditPage}
      />,
    );
    const banRow = screen.getByText('封禁玩家').closest('.ant-tree-treenode');
    fireEvent.click(within(banRow as HTMLElement).getByText('编辑页面'));
    expect(onEditPage).toHaveBeenCalledWith('operation--player.ban');
  });

  it('同菜单多页排序：order 升序、缺省 order 兜底 0、同序按 pageKey 字典序决胜', () => {
    // 四页覆盖比较器全部分支：(5,5)/(缺省,缺省) 命中 || 右侧 localeCompare，
    // (5,缺省)/(缺省,5) 命中两侧 ?? 兜底与 || 左侧短路
    renderTree({
      pages: [
        {
          pageKey: 'zz--late',
          type: 'resource',
          menuId: 1,
          title: { 'zh-CN': '丁页' },
          status: 'draft',
        },
        {
          pageKey: 'aa--early',
          type: 'resource',
          menuId: 1,
          title: { 'zh-CN': '甲页' },
          status: 'draft',
        },
        {
          pageKey: 'mm--mid',
          type: 'operation',
          menuId: 1,
          title: { 'zh-CN': '丙页' },
          status: 'draft',
          order: 5,
        },
        {
          pageKey: 'nn--mid',
          type: 'operation',
          menuId: 1,
          title: { 'zh-CN': '乙页' },
          status: 'draft',
          order: 5,
        },
      ],
    });

    // 期望顺序：aa--early(0) < zz--late(0，pageKey 决胜) < mm--mid(5) < nn--mid(5，决胜)
    const rowOf = (label: string): HTMLElement => {
      const row = screen.getByText(label).closest('.ant-tree-treenode');
      expect(row).not.toBeNull();
      return row as HTMLElement;
    };
    const [jia, ding, bing, yi] = ['甲页', '丁页', '丙页', '乙页'].map(rowOf);
    const follows = (a: HTMLElement, b: HTMLElement) =>
      (a.compareDocumentPosition(b) & Node.DOCUMENT_POSITION_FOLLOWING) !== 0;
    expect(follows(jia, ding)).toBe(true);
    expect(follows(ding, bing)).toBe(true);
    expect(follows(bing, yi)).toBe(true);
  });

  it('archived 状态页面：默认徽标「已下架」，不渲染草稿提示', () => {
    renderTree({
      pages: [
        {
          pageKey: 'legacy--page',
          type: 'resource',
          menuId: 2,
          title: { 'zh-CN': '下架页' },
          status: 'archived',
        },
      ],
    });
    expect(screen.getByText('下架页')).toBeInTheDocument();
    expect(screen.getByText('已下架')).toBeInTheDocument();
    expect(screen.queryByText('发布后才会出现在控制台导航')).not.toBeInTheDocument();
  });

  it('sortOrder 为 0 的菜单节点不渲染排序标签（truthy 判断）', () => {
    render(
      <MenuTree
        items={[
          makeNode({ id: 9, menuKey: 'zero', labels: { 'zh-CN': '零序菜单' }, sortOrder: 0 }),
        ]}
        pages={[]}
        canManage
        onEdit={jest.fn()}
        onDelete={jest.fn()}
        onMove={jest.fn()}
      />,
    );
    expect(screen.getByText('零序菜单')).toBeInTheDocument();
    expect(screen.queryByText('排序 0')).not.toBeInTheDocument();
  });
});
