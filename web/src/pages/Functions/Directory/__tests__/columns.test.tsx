/** 函数目录列构建（buildDirectoryColumns）：各列 formatter 空态回退、
 * 摘要 50 字截断、标签 >3 折叠、版本/状态过滤器、行操作回调路由。
 * 按列子集渲染（真实宿主为 ProTable，resource 列 filters:true 仅 ProTable 识别）。 */
import React from 'react';
import { ProTable } from '@ant-design/pro-components';
import type { ProColumns } from '@ant-design/pro-components';
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { buildDirectoryColumns } from '../columns';
import type { DirectoryPageSchema } from '../schema';
import type { SummaryRow } from '../types';

// minVersion 列（#26）内嵌 VersionFloorSelect，其 useIntl 走 @umijs/max——
// 真实模块无 IntlProvider 会抛错，按 defaultMessage 透传即可
jest.mock('@umijs/max', () => ({
  __esModule: true,
  useIntl: () => ({
    formatMessage: ({ defaultMessage }: { defaultMessage: string }) => defaultMessage,
  }),
}));

const intl = {
  formatMessage: ({ defaultMessage }: { id: string; defaultMessage: string }): string =>
    defaultMessage,
};

type ColumnKey = DirectoryPageSchema['columns'][number]['key'];

const ALL_COLUMN_DEFS: DirectoryPageSchema['columns'] = [
  { key: 'id', title: '函数ID', width: 250, copyable: true },
  { key: 'version', title: '版本', width: 110 },
  { key: 'minVersion', title: '最低函数版本', width: 130 },
  { key: 'displayName', title: '函数名称', width: 200 },
  { key: 'summary', title: '函数摘要', width: 300 },
  { key: 'resource', title: '资源', width: 160 },
  { key: 'operation', title: '操作', width: 120 },
  { key: 'tags', title: '标签', width: 200 },
  { key: 'enabled', title: '状态', width: 80 },
  { key: 'actions', title: '操作', width: 200 },
];

function defs(...keys: ColumnKey[]): DirectoryPageSchema['columns'] {
  return keys.map((key) => {
    const found = ALL_COLUMN_DEFS.find((col) => col.key === key);
    if (!found) throw new Error(`未知列: ${key}`);
    return found;
  });
}

const rowActions: DirectoryPageSchema['rowActions'] = [
  { key: 'detail', tooltip: '查看详情', icon: 'info' },
  { key: 'schema', tooltip: '契约 Schema', icon: 'code' },
  { key: 'invoke', tooltip: '调用函数', icon: 'play' },
];

type Handlers = {
  onOpenDetail: jest.Mock;
  onOpenSchema: jest.Mock;
  onInvoke: jest.Mock;
  onFloorChange: jest.Mock;
};

function makeHandlers(): Handlers {
  return {
    onOpenDetail: jest.fn(),
    onOpenSchema: jest.fn(),
    onInvoke: jest.fn(),
    onFloorChange: jest.fn(),
  };
}

function buildColumns(
  columnDefs: DirectoryPageSchema['columns'],
  handlers: Handlers,
  versions = ['1.0.0', '2.1.0'],
  versionIndex: Record<string, string[]> = {},
) {
  return buildDirectoryColumns({
    intl,
    columns: columnDefs,
    rowActions,
    versions,
    versionIndex,
    ...handlers,
  });
}

function renderTableWithVersionIndex(
  rows: SummaryRow[],
  columnDefs: DirectoryPageSchema['columns'],
  versionIndex: Record<string, string[]>,
  handlers = makeHandlers(),
) {
  return {
    handlers,
    ...render(
      <ProTable<SummaryRow>
        columns={
          buildColumns(
            columnDefs,
            handlers,
            ['1.0.0', '2.1.0'],
            versionIndex,
          ) as ProColumns<SummaryRow>[]
        }
        dataSource={rows}
        rowKey="id"
        search={false}
        toolBarRender={false}
        pagination={false}
        options={false}
      />,
    ),
  };
}

function renderTable(
  rows: SummaryRow[],
  columnDefs: DirectoryPageSchema['columns'],
  handlers = makeHandlers(),
) {
  return {
    handlers,
    ...render(
      <ProTable<SummaryRow>
        columns={buildColumns(columnDefs, handlers) as ProColumns<SummaryRow>[]}
        dataSource={rows}
        rowKey="id"
        search={false}
        toolBarRender={false}
        pagination={false}
        options={false}
      />,
    ),
  };
}

/** ProColumns.onFilter 可能是 true/false/函数：取函数后断言 */
function filterFn(
  col: ReturnType<typeof buildColumns>[number] | undefined,
): (value: unknown, record: SummaryRow) => boolean {
  const onFilter = col?.onFilter;
  if (typeof onFilter !== 'function') {
    throw new Error('onFilter 应为函数');
  }
  return onFilter as (value: unknown, record: SummaryRow) => boolean;
}

const fullRow: SummaryRow = {
  id: 'player.kick',
  enabled: true,
  displayName: { 'zh-CN': '踢出玩家' },
  summary: { 'zh-CN': '将玩家踢出服务器并断开当前会话连接' },
  resource: 'player',
  operation: 'kick',
  tags: ['t1', 't2', 't3', 't4', 't5'],
  version: '2.1.0',
  minVersion: '1.4.0',
};

const emptyRow: SummaryRow = { id: 'bare.fn' };

describe('buildDirectoryColumns 列 formatter', () => {
  it('id 列：按启用状态渲染 Badge 并输出函数 ID', () => {
    renderTable([{ ...fullRow, enabled: false }, emptyRow], defs('id'));
    expect(screen.getByText('player.kick')).toBeInTheDocument();
    expect(screen.getByText('bare.fn')).toBeInTheDocument();
    expect(document.querySelectorAll('.ant-badge').length).toBeGreaterThanOrEqual(2);
  });

  it('version 列：有值出蓝色 Tag，缺省回退 -；过滤器来自离散版本', () => {
    const columns = buildColumns(defs('version'), makeHandlers());
    const versionCol = columns.find((col) => col.dataIndex === 'version');
    expect(versionCol?.filters).toEqual([
      { text: 'v1.0.0', value: '1.0.0' },
      { text: 'v2.1.0', value: '2.1.0' },
    ]);
    const versionFilter = filterFn(versionCol);
    expect(versionFilter('2.1.0', fullRow)).toBe(true);
    expect(versionFilter('9.9.9', fullRow)).toBe(false);

    renderTable([fullRow, emptyRow], defs('version'));
    expect(screen.getByText('v2.1.0')).toBeInTheDocument();
    expect(screen.getByText('-')).toBeInTheDocument();
  });

  it('minVersion 列（#26）：行内下拉展示当前门槛选中项，未配置回退占位「未设置」', () => {
    renderTable([fullRow, emptyRow], defs('minVersion'));
    expect(screen.getByTitle('≥ v1.4.0')).toBeInTheDocument();
    expect(screen.getByText('未设置')).toBeInTheDocument();
  });

  it('minVersion 列（#26）：选项来自 versionIndex，选择后回调 (functionId, version)', async () => {
    const { handlers } = renderTableWithVersionIndex([fullRow], defs('minVersion'), {
      'player.kick': ['1.4.0', '2.1.0'],
    });
    fireEvent.mouseDown(screen.getByRole('combobox'));
    await waitFor(() => {
      expect(screen.getByTitle('≥ v2.1.0')).toBeInTheDocument();
    });
    // 1.4.0 已在当前值上；下拉列出全部历史版本供切换
    fireEvent.click(screen.getByTitle('≥ v2.1.0'));
    expect(handlers.onFloorChange).toHaveBeenCalledWith('player.kick', '2.1.0');
  });

  it('displayName 列：取 zh-CN 文案，缺省回退函数 ID', () => {
    renderTable(
      [{ id: 'named.fn', displayName: { 'zh-CN': '裸函数' } }, { id: 'fallback.fn' }],
      defs('displayName'),
    );
    expect(screen.getByText('裸函数')).toBeInTheDocument();
    expect(screen.getByText('fallback.fn')).toBeInTheDocument();
  });

  it('summary 列：空态 -；超过 50 字截断出省略号且完整文案不直接展示', () => {
    const long = '长'.repeat(60);
    renderTable(
      [
        { ...emptyRow, summary: { 'zh-CN': long } },
        { id: 'empty.summary.fn', summary: undefined },
      ],
      defs('summary'),
    );
    expect(screen.getByText(`${'长'.repeat(50)}...`)).toBeInTheDocument();
    expect(screen.queryByText(long)).not.toBeInTheDocument();
    expect(screen.getByText('-')).toBeInTheDocument();
  });

  it('resource/operation 列：未声明回退「未声明」，有值按颜色出 Tag', () => {
    renderTable([fullRow, emptyRow], defs('resource', 'operation'));
    expect(screen.getAllByText('未声明')).toHaveLength(2);
    expect(screen.getByText('player')).toBeInTheDocument();
    expect(screen.getByText('kick')).toBeInTheDocument();
  });

  it('tags 列：最多展示 3 个，超出折叠为 +N', () => {
    renderTable([fullRow], defs('tags'));
    expect(screen.getByText('t1')).toBeInTheDocument();
    expect(screen.getByText('t3')).toBeInTheDocument();
    expect(screen.queryByText('t4')).not.toBeInTheDocument();
    expect(screen.getByText('+2')).toBeInTheDocument();
  });

  it('enabled 列：启用/禁用文案 + 过滤器布尔值', () => {
    const columns = buildColumns(defs('enabled'), makeHandlers());
    const enabledCol = columns.find((col) => col.dataIndex === 'enabled');
    expect(enabledCol?.filters).toEqual([
      { text: '启用', value: true },
      { text: '禁用', value: false },
    ]);
    const enabledFilter = filterFn(enabledCol);
    expect(enabledFilter(true, fullRow)).toBe(true);
    expect(enabledFilter(false, fullRow)).toBe(false);

    renderTable([fullRow, emptyRow], defs('enabled'));
    expect(screen.getByText('启用')).toBeInTheDocument();
    expect(screen.getByText('禁用')).toBeInTheDocument();
  });
});

describe('buildDirectoryColumns 行操作回调', () => {
  it('detail/schema/invoke 三个图标按钮分别路由到对应回调', () => {
    const { handlers } = renderTable([fullRow], defs('id', 'actions'));
    const row = screen.getByText('player.kick').closest('tr');
    const actionCell = row ? within(row).getByText('player.kick').closest('tr') : null;
    const buttons = actionCell ? Array.from(actionCell.querySelectorAll('button')) : [];
    expect(buttons).toHaveLength(3);

    fireEvent.click(buttons[0]);
    expect(handlers.onOpenDetail).toHaveBeenCalledWith(fullRow);

    fireEvent.click(buttons[1]);
    expect(handlers.onOpenSchema).toHaveBeenCalledWith('player.kick');

    fireEvent.click(buttons[2]);
    expect(handlers.onInvoke).toHaveBeenCalledWith(fullRow);
  });
});
