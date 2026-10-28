/** D1 第二片：玩家/订单列表与详情复用 ProComponents 原生能力契约。
 *
 * 真实渲染 ProTable / Drawer / ProDescriptions（不 mock pro-components），
 * 直接用 gameOpsTemplates 注册表的 player-manage / order-review PageSpec
 * 常量驱动 ResourcePageRenderer，锁定三类复用契约：
 * - 列定义与筛选语义：filterable 列进 ProTable 查询表单，非 filterable 列不进；
 * - 分页语义：ProTable 原生请求链进入数据适配层后统一 page/pageSize/offset；
 * - 详情复用：真实 Drawer + ProDescriptions 按 detailView.fields 渲染。
 */
import React from 'react';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { App } from 'antd';
import ResourcePageRenderer from '@/components/PageRenderer/ResourcePageRenderer';
import { playerManagePageSpec, createPlayerManageDemoExecute } from './playerManage';
import { orderReviewPageSpec, createOrderReviewDemoExecute } from './orderReview';
import { createGameOpsDemoStore } from './gameOpsDemoData';
import type { PageExecuteFn } from '@/types/dashboard';

jest.mock('@/components/PageRenderer', () => ({
  __esModule: true,
  default: () => null,
}));

jest.mock('@/components/SchemaFormRenderer', () => {
  const Stub = React.forwardRef<unknown, Record<string, unknown>>((_props, ref) => {
    React.useImperativeHandle(ref, () => ({ validate: () => true, getValues: () => ({}) }));
    return <div data-testid="schema-form-stub" />;
  });
  Stub.displayName = 'SchemaFormRendererStub';
  return { __esModule: true, default: Stub };
});

const renderTemplateSpec = (
  pageSpec: typeof playerManagePageSpec | typeof orderReviewPageSpec,
  execute: PageExecuteFn,
) => {
  const onExecute = jest.fn(execute);
  render(
    <App>
      <ResourcePageRenderer
        spec={pageSpec.resource!}
        bindings={pageSpec.bindings}
        onExecute={onExecute as unknown as PageExecuteFn}
      />
    </App>,
  );
  return onExecute;
};

describe('D1 ProComponents 列表与详情复用契约', () => {
  it('玩家列表：filterable 语义由 ProTable 原生 search 承载，分页走原生请求链', async () => {
    const onExecute = renderTemplateSpec(
      playerManagePageSpec,
      createPlayerManageDemoExecute(createGameOpsDemoStore()),
    );

    // 真实 ProTable 渲染行，26 条数据只出第一页 10 行
    await waitFor(() => expect(screen.getByText('玩家1号')).toBeInTheDocument());
    expect(screen.getByText('玩家10号')).toBeInTheDocument();
    expect(screen.queryByText('玩家11号')).not.toBeInTheDocument();

    // 列定义→原生查询表单：filterable 列进入查询，非 filterable 列不进入
    // D5 模板默认呈现：玩家/角色身份列标为可查询前置条件，filterable 进原生查询
    expect(screen.getByLabelText('玩家 ID')).toBeInTheDocument();
    expect(screen.getByLabelText('昵称')).toBeInTheDocument();
    expect(screen.getByLabelText('状态')).toBeInTheDocument();
    // 非 filterable 列不进查询表单（level/vip/balanceGold 仅表格列）
    expect(screen.queryByLabelText('等级')).not.toBeInTheDocument();
    expect(screen.queryByLabelText('金币余额（金）')).not.toBeInTheDocument();

    // 分页语义由数据适配层承载：current 只作 UI 页码，绑定收到 page/offset
    const listCalls = onExecute.mock.calls.filter(([bindingId]) => bindingId === 'list');
    expect(listCalls.length).toBeGreaterThanOrEqual(1);
    expect(listCalls[0][1]).toMatchObject({ form: { page: 1, pageSize: 10, offset: 0 } });

    // 原生分页 total 与翻页语义：真实 Table 请求第 2 页
    expect(screen.getByText('共 26 条')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('listitem', { name: '2' }));
    await waitFor(() => expect(screen.getByText('玩家11号')).toBeInTheDocument());
    const pagedCall = onExecute.mock.calls.find(
      ([, context]) => context.form?.page === 2 && context.form?.pageSize === 10,
    );
    expect(pagedCall).toBeTruthy();
    expect(pagedCall?.[1]).toMatchObject({ form: { offset: 10 } });
  }, 90000);

  it('订单列表：channel/reconciliation 枚举筛选走原生 valueEnum 与 request 语义', async () => {
    const onExecute = renderTemplateSpec(
      orderReviewPageSpec,
      createOrderReviewDemoExecute(createGameOpsDemoStore()),
    );

    await waitFor(() => expect(screen.getByText('2026102301')).toBeInTheDocument());
    // 订单号/玩家 ID 身份列均为可查询前置条件
    expect(screen.getByLabelText('订单号')).toBeInTheDocument();
    expect(screen.getByLabelText('支付渠道')).toBeInTheDocument();
    expect(screen.getByLabelText('对账结论')).toBeInTheDocument();
    // 非 filterable 列（币种）不进入查询表单
    expect(screen.queryByLabelText('币种')).not.toBeInTheDocument();

    // 枚举列在表格内的原生筛选语义：ProTable 查询表单有对应字段控件
    expect(screen.getByLabelText('支付渠道')).toHaveAttribute('id', 'channel');
    expect(screen.getByLabelText('对账结论')).toHaveAttribute('id', 'reconciliation');

    // 分页 defaultSize 与 offset 语义由原生 request 链传入
    const listCalls = onExecute.mock.calls.filter(([bindingId]) => bindingId === 'list');
    expect(listCalls[0][1]).toMatchObject({ form: { page: 1, pageSize: 10, offset: 0 } });
    expect(screen.getByText('共 30 条')).toBeInTheDocument();
  }, 90000);

  it('玩家详情：真实 Drawer + ProDescriptions 按 detailView.fields 渲染', async () => {
    renderTemplateSpec(
      playerManagePageSpec,
      createPlayerManageDemoExecute(createGameOpsDemoStore()),
    );

    await waitFor(() => expect(screen.getByText('玩家1号')).toBeInTheDocument());
    fireEvent.click(screen.getAllByRole('button', { name: /查\s*看/ })[0]);
    await waitFor(() => expect(screen.getAllByText('详情').length).toBeGreaterThan(0));
    await waitFor(() => expect(screen.getAllByText('玩家1号').length).toBeGreaterThanOrEqual(2));

    // Drawer 存在且 ProDescriptions 渲染 detailView 字段标签
    expect(document.querySelector('.ant-drawer')).toBeInTheDocument();
    expect(screen.getAllByText('玩家 ID').length).toBeGreaterThan(0);
    expect(screen.getAllByText('昵称').length).toBeGreaterThan(0);
    expect(screen.getAllByText('注册时间').length).toBeGreaterThan(0);
    expect(document.querySelector('.ant-descriptions')).toBeInTheDocument();
  }, 90000);
});
