/** D3 组合页表格服务端分页覆盖。
 *
 * 数据适配层语义：请求携带 page/pageSize 并由渲染器派生 offset=(page-1)*pageSize
 * ——不把偏移量当页码传、也不把页码当偏移量传；结果给 total 则原样使用，缺 total
 * 时按「本页满则可能有下一页」估算上界（不满页即收口），避免缺 total 被误判为
 * 只剩当前页。翻页/改每页条数只变分页参数；上游筛选/查询条件变化回首页；操作
 * 成功后的刷新（onSuccessRefresh）保持当前页码与筛选。未启用分页保持原客户端
 * 分页（请求不带分页参数、翻页为前端切片）。
 */
import React from 'react';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { App } from 'antd';
import { CompositeRenderer, estimateTotal, paginationRequestParams } from '../CompositeRenderer';
import type { CompositeSection } from '@/types/dashboard';

jest.mock('@/services/api/functions', () => ({ invokeFunction: jest.fn() }));
// SchemaFormRenderer 替身：渲染提交按钮直接触发 onFinish（onSuccessRefresh 路径）
jest.mock('@/components/SchemaFormRenderer', () => ({
  __esModule: true,
  default: ({ onFinish }: { onFinish?: (values: Record<string, unknown>) => void }) => (
    <button type="button" data-testid="form-submit" onClick={() => onFinish?.({ name: '测试' })}>
      form-submit
    </button>
  ),
}));

// 重 DOM 套件在 coverage instrumentation 负载下撞默认 5s 用例预算
jest.setTimeout(20000);

type ExecForm = { form?: Record<string, unknown> };
type ExecResult = { data?: Record<string, unknown> };

const ROWS25 = Array.from({ length: 25 }, (_, i) => ({
  name: `r${String(i + 1).padStart(2, '0')}`,
}));

const col = { key: 'name', title: { 'zh-CN': '昵称' }, dataType: 'string' as const };

/** 执行器替身：按 page/offset/pageSize 真实切片（不忽略分页参数），
 * 记录每次请求入参供断言；withTotal=false 时不返回 total（无总数边界）。 */
const pagedExecutor = (
  all: Record<string, unknown>[],
  calls: Array<Record<string, unknown>>,
  withTotal = true,
) => {
  return async (_bindingId: string, context: ExecForm): Promise<ExecResult> => {
    const form = context.form ?? {};
    calls.push({ ...form });
    const page = Number(form.page) || 1;
    const size = Number(form.pageSize) || 20;
    const offset = Number(form.offset ?? (page - 1) * size);
    return {
      data: {
        items: all.slice(offset, offset + size),
        ...(withTotal ? { total: all.length } : {}),
      },
    };
  };
};

const renderComposite = (sections: CompositeSection[], onExecute: unknown): void => {
  render(
    <App>
      <CompositeRenderer sections={sections} bindings={[]} onExecute={onExecute as never} />
    </App>,
  );
};

describe('D3 分页数据适配层', () => {
  it('offset 由 (page-1)*pageSize 派生，不把页码当偏移量', () => {
    expect(paginationRequestParams(1, 20)).toEqual({ page: 1, pageSize: 20, offset: 0 });
    expect(paginationRequestParams(2, 20)).toEqual({ page: 2, pageSize: 20, offset: 20 });
    expect(paginationRequestParams(3, 10)).toEqual({ page: 3, pageSize: 10, offset: 20 });
    expect(paginationRequestParams(0, 20)).toEqual({ page: 1, pageSize: 20, offset: 0 });
  });

  it('缺 total 时按「本页满则可能有下一页」估算，不满页即收口', () => {
    expect(estimateTotal(1, 20, 20, undefined)).toBe(21); // 满页 → 可能有下一页
    expect(estimateTotal(2, 20, 20, undefined)).toBe(41);
    expect(estimateTotal(2, 20, 5, undefined)).toBe(25); // 不满页 → 收口
    expect(estimateTotal(1, 20, 0, undefined)).toBe(0);
    expect(estimateTotal(2, 20, 3, 30)).toBe(30); // 有 total 原样使用
    expect(estimateTotal(2, 20, 3, '30')).toBe(23); // 非数字按缺省估算
  });
});

describe('D3 组合页表格服务端分页', () => {
  it('页 2 每页 20 的请求 offset=20，且只渲染当前页', async () => {
    const calls: Array<Record<string, unknown>> = [];
    renderComposite(
      [
        {
          key: 'list',
          bindingId: 'b-list',
          view: 'table',
          title: { 'zh-CN': '玩家' },
          autoRun: true,
          table: {
            columns: [col],
            pagination: { enabled: true, defaultSize: 20, pageSizes: [10, 20, 50] },
          },
        },
      ],
      pagedExecutor(ROWS25, calls),
    );

    await waitFor(() => expect(calls.length).toBe(1));
    expect(calls[0]).toMatchObject({ page: 1, pageSize: 20, offset: 0 });
    expect(screen.getByText('r20')).toBeInTheDocument();
    expect(screen.queryByText('r21')).not.toBeInTheDocument();

    fireEvent.click(screen.getByText('2'));
    await waitFor(() => expect(calls.length).toBe(2));
    expect(calls[1]).toMatchObject({ page: 2, pageSize: 20, offset: 20 });
    expect(screen.getByText('r21')).toBeInTheDocument();
    expect(screen.getByText('r25')).toBeInTheDocument();
    expect(screen.queryByText('r20')).not.toBeInTheDocument();
  });

  it('筛选变化回首页：上游条件变化时页码重置为 1', async () => {
    const tableCalls: Array<Record<string, unknown>> = [];
    const upstreamResults = [{ q: '昵称甲' }, { q: '昵称乙' }];
    let upstreamIndex = 0;
    const onExecute = async (bindingId: string, context: ExecForm): Promise<ExecResult> => {
      const form = context.form ?? {};
      if (bindingId === 'b-up') {
        return { data: upstreamResults[upstreamIndex++] };
      }
      tableCalls.push({ ...form });
      const page = Number(form.page) || 1;
      const size = Number(form.pageSize) || 20;
      const offset = Number(form.offset ?? (page - 1) * size);
      return { data: { items: ROWS25.slice(offset, offset + size), total: ROWS25.length } };
    };
    renderComposite(
      [
        { key: 'upstream', bindingId: 'b-up', view: 'fields', title: { 'zh-CN': '查询条件' } },
        {
          key: 'list',
          bindingId: 'b-list',
          view: 'table',
          title: { 'zh-CN': '结果' },
          refreshOn: ['upstream'],
          table: {
            columns: [col],
            pagination: { enabled: true, defaultSize: 20, pageSizes: [10, 20, 50] },
          },
        },
      ],
      onExecute,
    );

    const buttons = await screen.findAllByRole('button', { name: '执行' });
    fireEvent.click(buttons[0]); // 上游第一次 → 表格按新条件重跑（首页）
    await waitFor(() => expect(tableCalls.length).toBe(1));
    expect(tableCalls[0]).toMatchObject({ q: '昵称甲', page: 1, pageSize: 20, offset: 0 });

    fireEvent.click(screen.getByText('2')); // 翻到第 2 页
    await waitFor(() => expect(tableCalls.length).toBe(2));
    expect(tableCalls[1]).toMatchObject({ q: '昵称甲', page: 2, pageSize: 20, offset: 20 });

    fireEvent.click(buttons[0]); // 上游条件变化 → 必须回到首页
    await waitFor(() => expect(tableCalls.length).toBe(3));
    expect(tableCalls[2]).toMatchObject({ q: '昵称乙', page: 1, pageSize: 20, offset: 0 });
  });

  it('操作成功后刷新：保持当前页码与筛选，不重置为首页', async () => {
    const calls: Array<Record<string, unknown>> = [];
    const onExecute = async (bindingId: string, context: ExecForm): Promise<ExecResult> => {
      if (bindingId !== 'b-list') return { data: { ok: true } };
      const form = context.form ?? {};
      calls.push({ ...form });
      const page = Number(form.page) || 1;
      const size = Number(form.pageSize) || 20;
      const offset = Number(form.offset ?? (page - 1) * size);
      return { data: { items: ROWS25.slice(offset, offset + size), total: ROWS25.length } };
    };
    renderComposite(
      [
        {
          key: 'list',
          bindingId: 'b-list',
          view: 'table',
          title: { 'zh-CN': '玩家' },
          autoRun: true,
          table: { columns: [col], pagination: { enabled: true, defaultSize: 20 } },
        },
        {
          key: 'act',
          bindingId: 'b-act',
          view: 'form',
          title: { 'zh-CN': '操作' },
          form: { jsonSchema: { type: 'object', properties: { name: { type: 'string' } } } },
          onSuccessRefresh: ['list'],
        },
      ],
      onExecute,
    );

    await waitFor(() => expect(calls.length).toBe(1));
    expect(calls[0]).toMatchObject({ page: 1, pageSize: 20, offset: 0 });
    fireEvent.click(screen.getByText('2'));
    await waitFor(() => expect(calls.length).toBe(2));
    expect(calls[1]).toMatchObject({ page: 2, pageSize: 20, offset: 20 });

    fireEvent.click(screen.getByTestId('form-submit')); // 操作成功 → 刷新列表
    await waitFor(() => expect(calls.length).toBe(3));
    expect(calls[2]).toMatchObject({ page: 2, pageSize: 20, offset: 20 });
  });

  it('无总数边界：缺 total 时按满页推进、不满页收口，不多报页数', async () => {
    const calls: Array<Record<string, unknown>> = [];
    renderComposite(
      [
        {
          key: 'list',
          bindingId: 'b-list',
          view: 'table',
          title: { 'zh-CN': '玩家' },
          autoRun: true,
          table: { columns: [col], pagination: { enabled: true, defaultSize: 20 } },
        },
      ],
      pagedExecutor(ROWS25, calls, false),
    );

    await waitFor(() => expect(calls.length).toBe(1));
    expect(screen.getByText('2')).toBeInTheDocument(); // 满页 → 有下一页
    expect(screen.queryByText('3')).not.toBeInTheDocument();

    fireEvent.click(screen.getByText('2'));
    await waitFor(() => expect(calls.length).toBe(2));
    expect(screen.getByText('共 25 条')).toBeInTheDocument();
    expect(screen.queryByText('3')).not.toBeInTheDocument(); // 不满页 → 收口
  });

  it('未启用分页：请求不带分页参数，翻页为前端切片不重跑', async () => {
    const calls: Array<Record<string, unknown>> = [];
    renderComposite(
      [
        {
          key: 'list',
          bindingId: 'b-list',
          view: 'table',
          title: { 'zh-CN': '玩家' },
          autoRun: true,
          table: { columns: [col] },
        },
      ],
      pagedExecutor(ROWS25, calls),
    );

    await waitFor(() => expect(calls.length).toBe(1));
    expect(calls[0]).toEqual({}); // 不分页请求：全量拉取
    expect(screen.getByText('r10')).toBeInTheDocument(); // 客户端 pageSize 10
    expect(screen.queryByText('r11')).not.toBeInTheDocument();

    fireEvent.click(screen.getByText('2'));
    await screen.findByText('r11');
    expect(calls.length).toBe(1); // 翻页不重跑
  });
});
