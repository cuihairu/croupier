/*
 * Assignments/columns.tsx 三个 build 纯函数专项测试
 *
 * buildAssignmentColumns：七类 listColumn key 分派（id Badge 二态/
 * name 结构/version Tag 回退/status 三臂含未知值 fallback/capability
 * 资源与操作二态/assignedAt 日期格式化/actions 兜底列）+ 行操作矩阵
 * （write 权限过滤、visibleWhen 二态、enable/disable/detail 三回调）；
 * buildCategoryColumns：resource/count/activeCount/activeRate 计算
 * （空组 0、100% success）+ 批量启停回调；buildRouteColumns：
 * id/name/capability 结构 + 查看函数回调。render 经 RTL 真渲染断言 DOM。
 */
import React from 'react';
import { fireEvent, render, screen, within } from '@testing-library/react';
import { buildAssignmentColumns, buildCategoryColumns, buildRouteColumns } from '../columns.tsx';
import type { AssignmentPageSchema } from '../pageSchema';
import type { AssignmentGroup, AssignmentItem } from '../types';

jest.mock('@umijs/max', () => ({
  FormattedMessage: ({ defaultMessage }: { id: string; defaultMessage?: string }) => (
    <>{defaultMessage ?? ''}</>
  ),
  getIntl: () => ({ formatMessage: (o: { defaultMessage?: string }) => o.defaultMessage ?? '' }),
}));

const intl = {
  formatMessage: (d: { id: string; defaultMessage: string }) => d.defaultMessage,
};

const ITEM = (over: Partial<AssignmentItem> = {}): AssignmentItem => ({
  id: 'fn.a',
  name: '函数A',
  version: '1.0.0',
  resource: 'player',
  operation: 'list',
  status: 'active',
  assignedAt: '2026-09-30T08:00:00Z',
  ...over,
});

const GROUP = (over: Partial<AssignmentGroup> = {}): AssignmentGroup => ({
  resource: 'player',
  items: [ITEM(), ITEM({ id: 'fn.b', status: 'disabled' })],
  activeCount: 1,
  ...over,
});

const ROW_ACTIONS: AssignmentPageSchema['rowActions'] = [
  { key: 'enable', tooltip: '启用', icon: 'check', permission: 'write', visibleWhen: 'isActive' },
  { key: 'disable', tooltip: '禁用', icon: 'delete', danger: true, visibleWhen: 'notActive' },
  { key: 'detail', tooltip: '详情', icon: 'setting' },
];

/** 直接执行列 render 函数并渲染为 DOM（ProColumns render 由 pro-table 在单元格调用）。 */
const renderCell = (node: React.ReactNode) => render(<>{node}</>);

const colOf = <T,>(cols: Array<{ title?: string; render?: unknown }>, title: string): T => {
  const col = cols.find((c) => c.title === title);
  expect(col).toBeDefined();
  return col as T;
};

type ListCol = typeof buildAssignmentColumns extends (o: infer O) => Array<infer C> ? C : never;
type Render = (
  text: unknown,
  record: AssignmentItem,
  index: number,
  action: 'edit',
) => React.ReactNode;

const buildList = (overrides: Partial<Parameters<typeof buildAssignmentColumns>[0]> = {}) =>
  buildAssignmentColumns({
    intl,
    canWrite: true,
    selected: [],
    setSelected: jest.fn(),
    listColumns: [
      { key: 'id', title: '函数ID', width: 200, copyable: true },
      { key: 'name', title: '名称', width: 180 },
      { key: 'version', title: '版本', width: 90 },
      { key: 'status', title: '状态', width: 100 },
      { key: 'capability', title: '能力', width: 160 },
      { key: 'assignedAt', title: '分配时间', width: 170 },
      { key: 'actions', title: '操作', width: 140 },
    ],
    rowActions: ROW_ACTIONS,
    onOpenDetail: jest.fn(),
    ...overrides,
  });

describe('buildAssignmentColumns 列分派', () => {
  it('id 列：copyable/width 结构 + Badge 二态（active success / disabled default）', () => {
    const cols = buildList();
    const col = colOf<ListCol>(cols, '函数ID');
    expect((col as { dataIndex?: string }).dataIndex).toBe('id');
    expect((col as { copyable?: boolean }).copyable).toBe(true);

    const render = col.render as Render;
    const active = render('v', ITEM(), 0, 'edit');
    const { container } = renderCell(active);
    // antd 6：Badge 状态色渲染在内部 dot（.ant-badge-status-success）。
    expect(container.querySelector('.ant-badge-status-success')).not.toBeNull();
    expect(within(container).getByText('fn.a')).toBeInTheDocument();

    const disabled = renderCell(render('v', ITEM({ status: 'disabled' }), 0, 'edit'));
    expect(disabled.container.querySelector('.ant-badge-status-default')).not.toBeNull();
  });

  it('name 列：ellipsis 结构无 render；version 列：蓝 Tag + 空值 "-"', () => {
    const cols = buildList();
    const name = colOf<ListCol>(cols, '名称');
    expect((name as { ellipsis?: boolean }).ellipsis).toBe(true);
    expect((name as { render?: unknown }).render).toBeUndefined();

    const version = colOf<ListCol>(cols, '版本');
    const render = version.render as (t: unknown) => React.ReactNode;
    const { container } = renderCell(render('1.2.3'));
    expect(container.querySelector('.ant-tag-blue')).not.toBeNull();
    expect(within(container).getByText('1.2.3')).toBeInTheDocument();

    const empty = renderCell(render(''));
    expect(within(empty.container).getByText('-')).toBeInTheDocument();
  });

  it('status 列：active/disabled/未知值三臂', () => {
    const cols = buildList();
    const render = colOf<ListCol>(cols, '状态').render as Render;

    // 每次 render 都向 document.body 追加新容器，断言须用 within 限定最新容器。
    let { container } = renderCell(render('active', ITEM(), 0, 'edit'));
    expect(within(container).getByText('已启用')).toBeInTheDocument();
    expect(container.querySelector('.ant-tag-success')).not.toBeNull();

    ({ container } = renderCell(render('disabled', ITEM(), 0, 'edit')));
    expect(within(container).getByText('未启用')).toBeInTheDocument();
    expect(container.querySelector('.ant-tag-default')).not.toBeNull();

    // 未知 status（如 canary）→ config[text] 落空回退 disabled 臂。
    ({ container } = renderCell(render('canary', ITEM({ status: 'canary' }), 0, 'edit')));
    expect(within(container).getByText('未启用')).toBeInTheDocument();
    expect(container.querySelector('.ant-tag-default')).not.toBeNull();
  });

  it('capability 列：资源有无二态 + 操作有无二态', () => {
    const cols = buildList();
    const render = colOf<ListCol>(cols, '能力').render as Render;

    let { container } = renderCell(
      render(null, ITEM({ resource: 'player', operation: 'grant' }), 0, 'edit'),
    );
    expect(screen.getByText('player')).toBeInTheDocument();
    expect(container.querySelector('.ant-tag-blue')).not.toBeNull();
    expect(screen.getByText('grant')).toBeInTheDocument();
    expect(container.querySelector('.ant-tag-purple')).not.toBeNull();

    screen.getAllByText('player').forEach((el) => el.remove());
    screen.getAllByText('grant').forEach((el) => el.remove());
    ({ container } = renderCell(
      render(null, ITEM({ resource: '', operation: undefined }), 0, 'edit'),
    ));
    expect(screen.getByText('未声明')).toBeInTheDocument();
    expect(container.querySelector('.ant-tag-default')).not.toBeNull();
    expect(container.querySelector('.ant-tag-purple')).toBeNull();
  });

  it('assignedAt 列：formatDateTime 透传（合法时间 → zh-CN 本地化串）', () => {
    const cols = buildList();
    const render = colOf<ListCol>(cols, '分配时间').render as Render;
    renderCell(render('2026-09-30T08:00:00Z', ITEM(), 0, 'edit'));
    // toLocaleString('zh-CN') 时区相关，只断日期前缀（UTC 与 UTC+8 同日）。
    expect(screen.getByText(/2026\/9\/30/)).toBeInTheDocument();
  });

  it('未知 key → actions 兜底列（fixed right）', () => {
    const cols = buildList({
      listColumns: [{ key: 'actions', title: '操作' }],
    });
    expect(cols).toHaveLength(1);
    expect((cols[0] as { fixed?: string }).fixed).toBe('right');
    expect((cols[0] as { render?: unknown }).render).toBeDefined();
  });
});

describe('buildAssignmentColumns 行操作矩阵', () => {
  /** 按钮为 icon-only + Tooltip（title 未 hover 不在 DOM）：以图标类名定位并点击。 */
  const clickIcon = (container: HTMLElement, iconClass: string) => {
    const icon = container.querySelector(iconClass);
    expect(icon).not.toBeNull();
    fireEvent.click((icon as HTMLElement).closest('button') as HTMLElement);
  };

  it('canWrite=false：permission=write 的 enable 过滤，read 的 detail 保留', () => {
    const cols = buildList({ canWrite: false });
    const render = colOf<ListCol>(cols, '操作').render as Render;
    const { container } = renderCell(render(null, ITEM({ status: 'active' }), 0, 'edit'));

    // enable（write + isActive）被过滤；detail（无 permission）保留 → 仅 1 按钮。
    expect(container.querySelectorAll('button')).toHaveLength(1);
    expect(container.querySelector('.anticon-setting')).not.toBeNull();
    expect(container.querySelector('.anticon-check-circle')).toBeNull();
  });

  it('visibleWhen：isActive 行见 enable、notActive 行见 disable（danger）', () => {
    const cols = buildList();
    const render = colOf<ListCol>(cols, '操作').render as Render;

    const { container: activeBox } = renderCell(
      render(null, ITEM({ status: 'active' }), 0, 'edit'),
    );
    expect(activeBox.querySelector('.anticon-check-circle')).not.toBeNull();
    expect(activeBox.querySelector('.anticon-delete')).toBeNull();
    expect(activeBox.querySelector('.anticon-setting')).not.toBeNull();

    const { container: disabledBox } = renderCell(
      render(null, ITEM({ status: 'disabled' }), 0, 'edit'),
    );
    expect(disabledBox.querySelector('.anticon-delete')).not.toBeNull();
    expect(disabledBox.querySelector('.anticon-check-circle')).toBeNull();
    expect(
      (disabledBox.querySelector('.anticon-delete') as HTMLElement).closest('button'),
    ).toHaveClass('ant-btn-dangerous');
  });

  it('runAction 三臂：enable 追加 selected、disable 过滤 selected、detail 回调', () => {
    const setSelected = jest.fn();
    const onOpenDetail = jest.fn();
    const cols = buildList({ selected: ['fn.x'], setSelected, onOpenDetail });
    const render = colOf<ListCol>(cols, '操作').render as Render;

    const { container: activeBox } = renderCell(
      render(null, ITEM({ status: 'active' }), 0, 'edit'),
    );
    clickIcon(activeBox, '.anticon-check-circle');
    expect(setSelected).toHaveBeenLastCalledWith(['fn.x', 'fn.a']);

    const { container: disabledBox } = renderCell(
      render(null, ITEM({ status: 'disabled' }), 0, 'edit'),
    );
    clickIcon(disabledBox, '.anticon-delete');
    expect(setSelected).toHaveBeenLastCalledWith(['fn.x']);

    const { container: detailBox } = renderCell(
      render(null, ITEM({ status: 'active' }), 0, 'edit'),
    );
    clickIcon(detailBox, '.anticon-setting');
    expect(onOpenDetail).toHaveBeenCalledWith('fn.a');
  });

  it('rowActions 为空 → 空 Space 无按钮', () => {
    const cols = buildList({ rowActions: [] });
    const render = colOf<ListCol>(cols, '操作').render as Render;
    const { container } = renderCell(render(null, ITEM(), 0, 'edit'));
    expect(container.querySelectorAll('button')).toHaveLength(0);
  });
});

describe('buildCategoryColumns', () => {
  const buildCat = (overrides: Partial<Parameters<typeof buildCategoryColumns>[0]> = {}) =>
    buildCategoryColumns({
      resourceColumns: [
        { key: 'resource', title: '资源', width: 140 },
        { key: 'count', title: '总数', width: 80 },
        { key: 'activeCount', title: '已启用', width: 90 },
        { key: 'activeRate', title: '启用率', width: 160 },
        { key: 'actions', title: '批量操作', width: 180 },
      ],
      onBatchAssign: jest.fn(),
      ...overrides,
    });

  it('resource 列 dataIndex / count 列 items.length / activeCount 绿 Tag', () => {
    const cols = buildCat();
    const resource = cols.find((c) => c.title === '资源') as { dataIndex?: string };
    expect(resource.dataIndex).toBe('resource');

    const count = cols.find((c) => c.title === '总数') as {
      render?: (v: unknown, r: AssignmentGroup) => React.ReactNode;
    };
    expect(count.render?.(null, GROUP())).toBe(2);

    const active = cols.find((c) => c.title === '已启用') as {
      render?: (v: unknown, r: AssignmentGroup) => React.ReactNode;
    };
    const { container } = renderCell(active.render?.(2, GROUP()));
    expect(screen.getByText('2')).toBeInTheDocument();
    expect(container.querySelector('.ant-tag-green')).not.toBeNull();
  });

  it('activeRate：比例计算、空组 0%、100% success 态', () => {
    const cols = buildCat();
    const rate = cols.find((c) => c.title === '启用率') as {
      render?: (v: unknown, r: AssignmentGroup) => React.ReactNode;
    };
    const render = rate.render as NonNullable<typeof rate.render>;

    // antd 6：Progress 文本渲染在 .ant-progress-indicator（title 属性 + 文本）。
    let { container } = renderCell(
      render(null, GROUP({ items: [ITEM(), ITEM(), ITEM(), ITEM()], activeCount: 3 })),
    );
    const indicator75 = container.querySelector('.ant-progress-indicator');
    expect(indicator75).not.toBeNull();
    expect(indicator75).toHaveTextContent('75%');
    expect(indicator75).toHaveAttribute('title', '75%');

    ({ container } = renderCell(render(null, GROUP({ items: [], activeCount: 0 }))));
    expect(container.querySelector('.ant-progress-indicator')).toHaveTextContent('0%');

    ({ container } = renderCell(render(null, GROUP({ items: [ITEM()], activeCount: 1 }))));
    expect(container.querySelector('.ant-progress-status-success')).not.toBeNull();
  });

  it('actions 兜底列：全部启用/全部禁用回调 onBatchAssign', () => {
    const onBatchAssign = jest.fn();
    const cols = buildCat({ onBatchAssign });
    const actions = cols.find((c) => c.title === '批量操作');
    expect(actions).toBeDefined();

    renderCell(
      (actions as { render?: (v: unknown, r: AssignmentGroup) => React.ReactNode }).render?.(
        null,
        GROUP(),
      ),
    );
    fireEvent.click(screen.getByText('全部启用'));
    expect(onBatchAssign).toHaveBeenLastCalledWith('player', true);
    fireEvent.click(screen.getByText('全部禁用'));
    expect(onBatchAssign).toHaveBeenLastCalledWith('player', false);
  });
});

describe('buildRouteColumns', () => {
  const buildRoute = (overrides: Partial<Parameters<typeof buildRouteColumns>[0]> = {}) =>
    buildRouteColumns({
      intl,
      capabilityColumns: [
        { key: 'id', title: '函数ID', width: 200, copyable: true },
        { key: 'name', title: '名称', width: 180 },
        { key: 'capability', title: '能力', width: 160 },
        { key: 'actions', title: '操作', width: 140 },
      ],
      onOpenDetail: jest.fn(),
      ...overrides,
    });

  it('id 列 copyable / name 列 ellipsis 结构', () => {
    const cols = buildRoute();
    expect((cols[0] as { copyable?: boolean }).copyable).toBe(true);
    expect((cols[1] as { ellipsis?: boolean }).ellipsis).toBe(true);
    expect((cols[1] as { render?: unknown }).render).toBeUndefined();
  });

  it('capability 列：资源有无 + 操作有无二态', () => {
    const cols = buildRoute();
    const render = (cols[2] as { render?: (v: unknown, r: AssignmentItem) => React.ReactNode })
      .render as NonNullable<(typeof cols)[2]['render']>;

    let { container } = renderCell(render(null, ITEM({ resource: 'mission' })));
    expect(screen.getByText('mission')).toBeInTheDocument();
    expect(container.querySelector('.ant-tag-purple')).not.toBeNull();

    screen.getAllByText('mission').forEach((el) => el.remove());
    ({ container } = renderCell(render(null, ITEM({ resource: '', operation: undefined }))));
    expect(screen.getByText('未声明')).toBeInTheDocument();
    expect(container.querySelector('.ant-tag-default')).not.toBeNull();
  });

  it('actions 兜底列：查看函数按钮 → onOpenDetail', () => {
    const onOpenDetail = jest.fn();
    const cols = buildRoute({ onOpenDetail });
    const actions = cols[3] as {
      render?: (v: unknown, r: AssignmentItem) => React.ReactNode;
    };
    renderCell(actions.render?.(null, ITEM({ id: 'fn.route' })));
    fireEvent.click(screen.getByText('查看函数'));
    expect(onOpenDetail).toHaveBeenCalledWith('fn.route');
  });
});
