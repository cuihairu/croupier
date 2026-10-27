/** D2 组合页表格行选择（批量目标核对）覆盖。
 *
 * 渲染面（真实 antd Table）：multiple=true 时 checkbox 多选 + 选择栏（已选
 * 计数/本页可见/目标清单/清空/筛选变更核对），跨页保留选择时计数与可见数
 * 同口径更新，rowSelected 事件仍以首行触发（与单选同一运行时写入路径）。
 * 未启用（缺省）保持既有单选 radio、无选择栏——rowSelected 语义不变。
 */
import React from 'react';
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { App } from 'antd';
import { CompositeRenderer } from '../CompositeRenderer';
import type { CompositeSection } from '@/types/dashboard';

jest.mock('@/services/api/functions', () => ({ invokeFunction: jest.fn() }));

// 重 DOM 套件在 coverage instrumentation 负载下撞默认 5s 用例预算
jest.setTimeout(20000);

type ExecForm = { form?: Record<string, unknown> };
type ExecResult = { data?: Record<string, unknown> };

const ROWS25 = Array.from({ length: 25 }, (_, i) => ({
  uid: `u${String(i + 1).padStart(2, '0')}`,
  name: `r${String(i + 1).padStart(2, '0')}`,
}));

const col = { key: 'uid', title: { 'zh-CN': 'UID' }, dataType: 'string' as const };
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
    columns: [col, nameCol],
    selection: { multiple: true, identityKey: 'uid' },
    pagination: { enabled: true, defaultSize: 20, pageSizes: [10, 20, 50] },
    ...tableOverrides,
  } as CompositeSection['table'],
  ...overrides,
});

/** 执行器替身：表格按 page/offset 真实切片；下游事件表单返回空成功。 */
const makeExecutor =
  (calls: Array<Record<string, unknown>>) =>
  async (bindingId: string, context: ExecForm): Promise<ExecResult> => {
    if (bindingId !== 'b-table') return { data: { ok: true } };
    const form = context.form ?? {};
    calls.push({ ...form });
    const page = Number(form.page) || 1;
    const size = Number(form.pageSize) || 20;
    const offset = Number(form.offset ?? (page - 1) * size);
    return { data: { items: ROWS25.slice(offset, offset + size), total: ROWS25.length } };
  };

const renderComposite = (sections: CompositeSection[], onExecute: unknown): void => {
  render(
    <App>
      <CompositeRenderer sections={sections} bindings={[]} onExecute={onExecute as never} />
    </App>,
  );
};

/** 行复选框：按行内单元格文本定位（antd 原生 checkbox input）。 */
const rowCheckbox = (name: string): HTMLInputElement => {
  const cell = screen.getByText(name).closest('tr');
  return cell!.querySelector('input[type="checkbox"]') as HTMLInputElement;
};

describe('D2 组合页表格多选与选择栏', () => {
  it('勾选行后显示已选计数；跨页保留选择时显示本页可见数', async () => {
    const calls: Array<Record<string, unknown>> = [];
    renderComposite([tableSection()], makeExecutor(calls));
    await waitFor(() => expect(calls.length).toBe(1));
    await screen.findByText('r01');

    fireEvent.click(rowCheckbox('r01'));
    await waitFor(() =>
      expect(screen.getByTestId('selection-count')).toHaveTextContent('已选择 1 项'),
    );
    // 同页选择：不出现本页可见数（无跨页无需提示）
    expect(screen.queryByTestId('selection-visible')).not.toBeInTheDocument();

    fireEvent.click(screen.getByText('2'));
    await screen.findByText('r21');
    // 第 2 页：u01 不在本页结果集 → 总数保持、本页可见 0
    expect(screen.getByTestId('selection-count')).toHaveTextContent('已选择 1 项');
    expect(screen.getByTestId('selection-visible')).toHaveTextContent('本页可见 0 项');

    fireEvent.click(rowCheckbox('r21'));
    await waitFor(() =>
      expect(screen.getByTestId('selection-count')).toHaveTextContent('已选择 2 项'),
    );
    expect(screen.getByTestId('selection-visible')).toHaveTextContent('本页可见 1 项');
  });

  it('rowSelected 事件以首行触发（与单选同一运行时写入路径）', async () => {
    const calls: Array<Record<string, unknown>> = [];
    const onExecute = jest.fn().mockImplementation(makeExecutor(calls));
    const mailForm: CompositeSection = {
      key: 'mailForm',
      bindingId: 'b-form',
      view: 'form',
      title: { 'zh-CN': '发邮件' },
      form: { jsonSchema: { type: 'object', properties: { uid: { type: 'string' } } } },
    };
    renderComposite(
      [
        tableSection({
          events: [
            {
              event: 'rowSelected',
              action: {
                kind: 'runBinding',
                target: 'mailForm',
                params: { uid: '{{list.selectedRow.uid}}' },
              },
            },
          ],
        }),
        mailForm,
      ],
      onExecute,
    );
    await screen.findByText('r01');

    fireEvent.click(rowCheckbox('r01'));
    await waitFor(() => expect(onExecute).toHaveBeenCalledWith('b-form', { form: { uid: 'u01' } }));
  });

  it('目标清单：列出全部已选对象，身份列置前', async () => {
    const calls: Array<Record<string, unknown>> = [];
    renderComposite([tableSection()], makeExecutor(calls));
    await screen.findByText('r01');

    fireEvent.click(rowCheckbox('r01'));
    fireEvent.click(rowCheckbox('r02'));
    await waitFor(() =>
      expect(screen.getByTestId('selection-count')).toHaveTextContent('已选择 2 项'),
    );

    fireEvent.click(screen.getByRole('button', { name: '查看目标清单' }));
    await screen.findByText('已选目标清单（2 项）');
    const modal = within(document.querySelector('.ant-modal') as HTMLElement);
    expect(modal.getByText('u01')).toBeInTheDocument();
    expect(modal.getByText('u02')).toBeInTheDocument();
    // 身份列在前（UID 在昵称之前）
    expect(modal.getAllByRole('columnheader')[0]).toHaveTextContent('UID');
  });

  it('取消选择：清空计数并隐藏选择栏', async () => {
    const calls: Array<Record<string, unknown>> = [];
    renderComposite([tableSection()], makeExecutor(calls));
    await screen.findByText('r01');

    fireEvent.click(rowCheckbox('r01'));
    await waitFor(() =>
      expect(screen.getByTestId('selection-count')).toHaveTextContent('已选择 1 项'),
    );

    fireEvent.click(screen.getByRole('button', { name: '取消选择' }));
    await waitFor(() => expect(screen.queryByTestId('selection-count')).not.toBeInTheDocument());
    expect(screen.queryByTestId('selection-visible')).not.toBeInTheDocument();
    // 行复选框回到未选中
    await waitFor(() => expect(rowCheckbox('r01').checked).toBe(false));
  });
});

describe('D2 筛选变更后的旧选择处理', () => {
  it('筛选变化后提示核对，清除选择后提示消失；翻页不计筛选变更', async () => {
    const upstreamResults = [{ q: '甲' }, { q: '乙' }];
    let upstreamIndex = 0;
    const calls: Array<Record<string, unknown>> = [];
    const onExecute = async (bindingId: string, context: ExecForm): Promise<ExecResult> => {
      const form = context.form ?? {};
      if (bindingId === 'b-up') return { data: upstreamResults[upstreamIndex++] };
      calls.push({ ...form });
      const page = Number(form.page) || 1;
      const size = Number(form.pageSize) || 20;
      const offset = Number(form.offset ?? (page - 1) * size);
      return { data: { items: ROWS25.slice(offset, offset + size), total: ROWS25.length } };
    };
    renderComposite(
      [
        { key: 'upstream', bindingId: 'b-up', view: 'fields', title: { 'zh-CN': '查询条件' } },
        tableSection({ refreshOn: ['upstream'], autoRun: false }),
      ],
      onExecute,
    );

    // 上游第一次执行 → 表格带条件重跑
    const buttons = await screen.findAllByRole('button', { name: '执行' });
    fireEvent.click(buttons[0]);
    await waitFor(() => expect(calls.length).toBe(1));
    await screen.findByText('r01');

    fireEvent.click(rowCheckbox('r01'));
    await waitFor(() =>
      expect(screen.getByTestId('selection-count')).toHaveTextContent('已选择 1 项'),
    );
    expect(screen.queryByTestId('selection-stale')).not.toBeInTheDocument();

    // 翻页不算筛选变更：跨页保留不触发核对提示
    fireEvent.click(screen.getByText('2'));
    await screen.findByText('r21');
    expect(screen.queryByTestId('selection-stale')).not.toBeInTheDocument();

    // 上游条件变化 → 表格重跑 → 旧选择提示核对
    fireEvent.click(buttons[0]);
    await waitFor(() => expect(calls.length).toBe(3));
    await waitFor(() => expect(screen.getByTestId('selection-stale')).toBeInTheDocument());
    expect(screen.getByTestId('selection-stale')).toHaveTextContent('筛选条件已变更');

    // 清除选择后提示消失
    fireEvent.click(screen.getByRole('button', { name: '取消选择' }));
    await waitFor(() => expect(screen.queryByTestId('selection-stale')).not.toBeInTheDocument());
  });
});

describe('D2 缺省单选保持', () => {
  it('未配置选择：保持 radio、无选择栏', async () => {
    const calls: Array<Record<string, unknown>> = [];
    renderComposite(
      [tableSection({}, { selection: undefined, pagination: { enabled: false } })],
      makeExecutor(calls),
    );
    await screen.findByText('r01');

    const radios = document.querySelectorAll('input[type="radio"]');
    expect(radios.length).toBeGreaterThan(0);
    expect(document.querySelectorAll('input[type="checkbox"]').length).toBe(0);
    fireEvent.click(radios[0]);
    // 缺省单选不渲染选择栏（目标核对为多选专属能力），
    // rowSelected 事件语义由 compositeSelectedRow 覆盖
    expect(screen.queryByTestId('selection-count')).not.toBeInTheDocument();
  });
});
