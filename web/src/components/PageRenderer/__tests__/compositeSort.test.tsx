/** D3 组合页表格列排序覆盖。
 *
 * 渲染面（真实 antd Table）：可排序列（generator 按 number/datetime 数据类型
 * 机械标注 sortable）给表头排序 UI。分页未启用时 antd 本地排序（compare 函数，
 * number 按数值、其余按字符串码点序、缺失值排末尾）；启用服务端分页时列用
 * sorter=true（仅表头 UI），排序状态经受控 sortOrder 下推请求（sortBy/sortOrder）
 * 并回首页重跑——排序只改变结果集顺序、不改变成员，因此不属于筛选指纹（不写入
 * sectionInputs），已选行不因排序被判过期；不可排序列无排序 UI。
 */
import React from 'react';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { App } from 'antd';
import { CompositeRenderer } from '../CompositeRenderer';
import { sortRequestParams } from '../compositeRuntime';
import type { CompositeSection } from '@/types/dashboard';

jest.mock('@/services/api/functions', () => ({ invokeFunction: jest.fn() }));

// 重 DOM 套件在 coverage instrumentation 负载下撞默认 5s 用例预算
jest.setTimeout(20000);

type ExecForm = { form?: Record<string, unknown> };
type ExecResult = { data?: Record<string, unknown> };

const ROWS = [
  { uid: 'u01', name: 'r01', level: 30 },
  { uid: 'u02', name: 'r02', level: 10 },
  { uid: 'u03', name: 'r03', level: 20 },
];

const ROWS25 = Array.from({ length: 25 }, (_, i) => ({
  uid: `u${String(i + 1).padStart(2, '0')}`,
  name: `r${String(i + 1).padStart(2, '0')}`,
  level: 25 - i,
}));

const levelCol = {
  key: 'level',
  title: { 'zh-CN': '等级' },
  dataType: 'number' as const,
  sortable: true,
};
const uidCol = { key: 'uid', title: { 'zh-CN': 'UID' }, dataType: 'string' as const };
const nameCol = { key: 'name', title: { 'zh-CN': '昵称' }, dataType: 'string' as const };

const tableSection = (
  overrides: Partial<CompositeSection> = {},
  tableOverrides: Record<string, unknown> = {},
): CompositeSection => ({
  key: 'list',
  bindingId: 'b-table',
  view: 'table',
  title: { 'zh-CN': '玩家' },
  autoRun: true,
  table: {
    columns: [levelCol, uidCol, nameCol],
    selection: { multiple: true, identityKey: 'uid' },
    pagination: { enabled: false },
    ...tableOverrides,
  } as CompositeSection['table'],
  ...overrides,
});

/** 执行器替身：表格按 page/offset 真实切片并记录每次请求入参。 */
const makeExecutor =
  (calls: Array<Record<string, unknown>>, rows: Array<Record<string, unknown>> = ROWS25) =>
  async (bindingId: string, context: ExecForm): Promise<ExecResult> => {
    if (bindingId !== 'b-table') return { data: { ok: true } };
    const form = context.form ?? {};
    calls.push({ ...form });
    const page = Number(form.page) || 1;
    const size = Number(form.pageSize) || 20;
    const offset = Number(form.offset ?? (page - 1) * size);
    return { data: { items: rows.slice(offset, offset + size), total: rows.length } };
  };

const renderComposite = (sections: CompositeSection[], onExecute: unknown): void => {
  render(
    <App>
      <CompositeRenderer sections={sections} bindings={[]} onExecute={onExecute as never} />
    </App>,
  );
};

/** 行顺序：antd Table 行带 data-row-key，取各行内名字按 DOM 顺序排列。 */
const rowOrder = (names: string[]): string[] => {
  const rows = Array.from(document.querySelectorAll('tbody tr[data-row-key]'));
  return rows
    .map((tr) => names.find((n) => (tr.textContent ?? '').includes(n)) ?? '')
    .filter(Boolean);
};

/** 点击表头触发排序（antd 的排序点击落在列头 .ant-table-column-sorters）。 */
const clickSort = (title: string): void => {
  fireEvent.click(
    screen.getByText(title).closest('th')!.querySelector('.ant-table-column-sorters')!,
  );
};

describe('D3 排序请求参数', () => {
  it('无排序状态不下发排序键；asc/desc 归一', () => {
    expect(sortRequestParams(undefined)).toEqual({});
    expect(sortRequestParams({ field: 'level', order: 'asc' })).toEqual({
      sortBy: 'level',
      sortOrder: 'asc',
    });
    expect(sortRequestParams({ field: 'level', order: 'desc' })).toEqual({
      sortBy: 'level',
      sortOrder: 'desc',
    });
    expect(sortRequestParams({ field: '', order: 'asc' })).toEqual({});
  });
});

describe('D3 组合页表格客户端排序', () => {
  it('可排序列表头可排序并本地重排；不可排序列无排序 UI', async () => {
    renderComposite(
      [tableSection({}, { pagination: { enabled: false } })],
      makeExecutor([] as Array<Record<string, unknown>>, ROWS),
    );
    await screen.findByText('r01');
    expect(rowOrder(['r01', 'r02', 'r03'])).toEqual(['r01', 'r02', 'r03']);

    const levelTh = screen.getByText('等级').closest('th');
    expect(levelTh).toHaveClass('ant-table-column-has-sorters');
    // UID 列不可排序：无排序 UI
    expect(screen.getByText('UID').closest('th')).not.toHaveClass('ant-table-column-has-sorters');

    // 点击一次 = 升序（数值 10/20/30）
    clickSort('等级');
    await waitFor(() => expect(rowOrder(['r01', 'r02', 'r03'])).toEqual(['r02', 'r03', 'r01']));
    // 再点击 = 降序
    clickSort('等级');
    await waitFor(() => expect(rowOrder(['r01', 'r02', 'r03'])).toEqual(['r01', 'r03', 'r02']));
  });
});

describe('D3 组合页表格服务端排序', () => {
  it('分页启用时点击排序下推 sortBy/sortOrder 并回首页', async () => {
    const calls: Array<Record<string, unknown>> = [];
    renderComposite(
      [
        tableSection(
          {},
          { pagination: { enabled: true, defaultSize: 20, pageSizes: [10, 20, 50] } },
        ),
      ],
      makeExecutor(calls),
    );

    await waitFor(() => expect(calls.length).toBe(1));
    expect(calls[0]).toMatchObject({ page: 1, pageSize: 20 });
    expect(calls[0]).not.toHaveProperty('sortBy');

    // 翻到页 2
    fireEvent.click(screen.getByText('2'));
    await waitFor(() => expect(calls.length).toBe(2));
    expect(calls[1]).toMatchObject({ page: 2 });

    // 排序：回首页 + 下推排序参数
    clickSort('等级');
    await waitFor(() => expect(calls.length).toBe(3));
    expect(calls[2]).toMatchObject({ page: 1, sortBy: 'level', sortOrder: 'asc' });
    await waitFor(() =>
      expect(screen.getByText('等级').closest('th')).toHaveClass('ant-table-column-sort'),
    );

    // 再点击：同列切换降序
    clickSort('等级');
    await waitFor(() => expect(calls.length).toBe(4));
    expect(calls[3]).toMatchObject({ page: 1, sortBy: 'level', sortOrder: 'desc' });
  });

  it('排序不属于筛选指纹：已选行不因排序被判过期', async () => {
    const upstreamResults = [{ q: '甲' }, { q: '乙' }];
    let upstreamIndex = 0;
    const calls: Array<Record<string, unknown>> = [];
    const onExecute = async (bindingId: string, context: ExecForm): Promise<ExecResult> => {
      const form = context.form ?? {};
      if (bindingId === 'b-up') return { data: upstreamResults[upstreamIndex++] };
      return makeExecutor(calls)(bindingId, context);
    };
    renderComposite(
      [
        { key: 'upstream', bindingId: 'b-up', view: 'fields', title: { 'zh-CN': '查询条件' } },
        tableSection(
          { refreshOn: ['upstream'], autoRun: false },
          {
            pagination: { enabled: true, defaultSize: 20, pageSizes: [10, 20, 50] },
          },
        ),
      ],
      onExecute,
    );

    // 上游执行 → 表格带条件重跑
    const buttons = await screen.findAllByRole('button', { name: '执行' });
    fireEvent.click(buttons[0]);
    await waitFor(() => expect(calls.length).toBe(1));
    await screen.findByText('r01');

    fireEvent.click(
      screen.getByText('r01').closest('tr')!.querySelector('input[type="checkbox"]')!,
    );
    await waitFor(() =>
      expect(screen.getByTestId('selection-count')).toHaveTextContent('已选择 1 项'),
    );
    expect(screen.queryByTestId('selection-stale')).not.toBeInTheDocument();

    // 排序重跑：不触发选择核对提示（排序参数不进 sectionInputs/指纹）
    clickSort('等级');
    await waitFor(() => expect(calls.length).toBe(2));
    expect(calls[1]).toMatchObject({ sortBy: 'level', sortOrder: 'asc' });
    expect(screen.queryByTestId('selection-stale')).not.toBeInTheDocument();

    // 对照组：上游条件真实变化才提示核对
    fireEvent.click(buttons[0]);
    await waitFor(() => expect(calls.length).toBe(3));
    await waitFor(() => expect(screen.getByTestId('selection-stale')).toBeInTheDocument());
  });
});
