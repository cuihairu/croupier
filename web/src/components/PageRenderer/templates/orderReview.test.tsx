/** orderReview 模板覆盖：PageSpec 结构 + demo 执行器主要分支。 */
import React from 'react';
import { render, screen } from '@testing-library/react';
import OrderReviewTemplate, {
  createOrderReviewDemoExecute,
  orderReviewPageSpec,
} from './orderReview';
import type { BindingExecutionContext, JSONValue } from '@/types/dashboard';

jest.mock('@ant-design/pro-components', () => ({
  PageContainer: ({ title, children }: { title?: React.ReactNode; children?: React.ReactNode }) => (
    <div>
      <h1>{title}</h1>
      {children}
    </div>
  ),
}));
jest.mock('@/components/PageRenderer', () => ({
  __esModule: true,
  default: () => <div data-testid="page-renderer" />,
}));

const run = (
  execute: ReturnType<typeof createOrderReviewDemoExecute>,
  bindingId: string,
  context: BindingExecutionContext = {},
) => execute(bindingId, context) as Promise<{ kind: string; data: JSONValue }>;

describe('orderReview PageSpec', () => {
  it('归属 payment 分类，详情含支付/发货凭据分列，补发绑定在案', () => {
    expect(orderReviewPageSpec.category.key).toBe('payment');
    if (orderReviewPageSpec.type !== 'resource') return;
    const detailKeys = orderReviewPageSpec.resource.detailView?.fields.map((f) => f.key) ?? [];
    expect(detailKeys).toContain('payTransactionId');
    expect(detailKeys).toContain('gameCreditRef');
    expect(orderReviewPageSpec.bindings.some((b) => b.id === 'action.recredit')).toBe(true);
  });
});

describe('orderReview demo 执行器', () => {
  it('list：分页/渠道/对账筛选与空结果', async () => {
    const execute = createOrderReviewDemoExecute();
    const page2 = await run(execute, 'list', { form: { current: 2, pageSize: 20 } });
    expect(page2.data).toMatchObject({ total: 30 });
    // D5 身份列前置查询：订单号与玩家 ID 均精确命中一条
    const byOrderNo = await run(execute, 'list', {
      form: { current: 1, pageSize: 10, orderNo: '2026102301' },
    });
    expect(byOrderNo.data).toMatchObject({ total: 1 });
    const byPlayerId = await run(execute, 'list', {
      form: { current: 1, pageSize: 10, playerId: '20001' },
    });
    expect(byPlayerId.data).toMatchObject({ total: 1 });
    const byChannel = await run(execute, 'list', {
      form: { current: 1, pageSize: 10, channel: 'ios' },
    });
    expect(byChannel.data).toMatchObject({ total: 6 });
    const missing = await run(execute, 'list', {
      form: { current: 1, pageSize: 10, reconciliation: 'missing_credit' },
    });
    expect(missing.data).toMatchObject({ total: 2 });
    const empty = await run(execute, 'list', {
      form: { current: 1, pageSize: 10, nickname: '不存在的昵称' },
    });
    expect(empty.data).toMatchObject({ items: [], total: 0 });
  });

  it('detail：命中带双凭据，未命中返回 null', async () => {
    const execute = createOrderReviewDemoExecute();
    const hit = await run(execute, 'detail', { row: { orderNo: '2026102301' } });
    expect(hit.data).toMatchObject({ payTransactionId: 'pay-9001' });
    const miss = await run(execute, 'detail', { row: { orderNo: 'nope' } });
    expect(miss.data).toBeNull();
  });

  it('recredit：已发货/无需发货报错，缺发货补发后对账转一致且不可重复补发', async () => {
    const execute = createOrderReviewDemoExecute();
    const missing = await run(execute, 'list', {
      form: { current: 1, pageSize: 10, reconciliation: 'missing_credit' },
    });
    const orderNo = (missing.data as { items: Array<{ orderNo: string }> }).items[0].orderNo;
    const result = await run(execute, 'action.recredit', {
      row: { orderNo },
      form: { reason: '客诉核实' },
    });
    expect(result.data).toMatchObject({ orderNo });
    const detail = await run(execute, 'detail', { row: { orderNo } });
    expect(detail.data).toMatchObject({ gameCredit: 'credited', reconciliation: 'match' });
    await expect(
      run(execute, 'action.recredit', { row: { orderNo }, form: { reason: 'x' } }),
    ).rejects.toThrow('already_credited');
    const all = await run(execute, 'list', { form: { current: 1, pageSize: 50 } });
    const notNeeded = (
      all.data as {
        items: Array<{ orderNo: string; gameCredit: string }>;
      }
    ).items.find((item) => item.gameCredit === 'not_needed');
    await expect(
      run(execute, 'action.recredit', {
        row: { orderNo: notNeeded?.orderNo },
        form: { reason: 'x' },
      }),
    ).rejects.toThrow('credit_not_applicable');
    await expect(
      run(execute, 'action.recredit', { row: { orderNo: 'nope' }, form: {} }),
    ).rejects.toThrow('order_not_found');
  });

  it('未知 binding 走默认成功分支', async () => {
    const execute = createOrderReviewDemoExecute();
    const result = await run(execute, 'unknown');
    expect(result.data).toMatchObject({ success: true });
  });
});

describe('OrderReviewTemplate 组件', () => {
  it('挂载渲染标题与 PageRenderer', () => {
    render(<OrderReviewTemplate />);
    expect(screen.getByRole('heading', { name: '订单核查' })).toBeInTheDocument();
    expect(screen.getByTestId('page-renderer')).toBeInTheDocument();
  });
});
