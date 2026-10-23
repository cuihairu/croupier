/**
 * orderReview - 订单核查业务模板（默认菜单 payment 类）
 *
 * 覆盖业务闭环：订单列表（支付渠道/状态/游戏发货状态/对账结论）→ 详情
 * （支付凭据与游戏发货凭据分列）→ 补发奖励（行操作，仅缺发货订单）。
 *
 * 内置 demo 执行器（内存数据），接入真实环境时将 onExecute 替换为
 * `executePageBinding` 即可。
 *
 * @module components/PageRenderer/templates/orderReview
 */

import React, { useMemo } from 'react';
import { useIntl } from '@umijs/max';
import { PageContainer } from '@ant-design/pro-components';
import PageRenderer from '@/components/PageRenderer';
import type { PageSpec, PageExecuteFn, PageExecutionResult } from '@/types/dashboard';
import {
  asRecord,
  createGameOpsDemoStore,
  createRequestIdFactory,
  type GameOpsDemoStore,
} from './gameOpsDemoData';

/** 订单核查页面规格 */
export const orderReviewPageSpec: PageSpec = {
  pageKey: 'template--order-review',
  type: 'resource',
  resourceKey: 'orders',
  title: { 'zh-CN': '订单核查', 'en-US': 'Order Review' },
  description: {
    'zh-CN': '订单查询、发货核查、对账差异与补发（模板示例）',
    'en-US': 'Order query, credit check, reconciliation diffs and re-credit (template demo)',
  },
  category: {
    key: 'payment',
  },
  resource: {
    listView: {
      identityKey: 'orderNo',
      columns: [
        {
          key: 'orderNo',
          title: { 'zh-CN': '订单号', 'en-US': 'Order No.' },
          dataType: 'string',
          width: 140,
          fixed: 'left',
          render: 'copy',
        },
        {
          key: 'playerId',
          title: { 'zh-CN': '玩家 ID', 'en-US': 'Player ID' },
          dataType: 'string',
          width: 110,
          render: 'copy',
        },
        {
          key: 'nickname',
          title: { 'zh-CN': '昵称', 'en-US': 'Nickname' },
          dataType: 'string',
          filterable: true,
        },
        {
          key: 'amount',
          title: { 'zh-CN': '金额', 'en-US': 'Amount' },
          dataType: 'number',
          width: 90,
          sortable: true,
        },
        {
          key: 'currency',
          title: { 'zh-CN': '币种', 'en-US': 'Currency' },
          dataType: 'enum',
          width: 80,
          enum: [
            { value: 'CNY', label: { 'zh-CN': '人民币', 'en-US': 'CNY' } },
            { value: 'USD', label: { 'zh-CN': '美元', 'en-US': 'USD' } },
          ],
        },
        {
          key: 'channel',
          title: { 'zh-CN': '支付渠道', 'en-US': 'Channel' },
          dataType: 'enum',
          width: 100,
          filterable: true,
          render: 'tag',
          enum: [
            { value: 'official', label: { 'zh-CN': '官方', 'en-US': 'Official' }, color: 'blue' },
            { value: 'ios', label: { 'zh-CN': 'iOS', 'en-US': 'iOS' }, color: 'default' },
            { value: 'android', label: { 'zh-CN': 'Android', 'en-US': 'Android' }, color: 'green' },
            {
              value: 'wechat',
              label: { 'zh-CN': '微信支付', 'en-US': 'WeChat Pay' },
              color: 'cyan',
            },
            { value: 'alipay', label: { 'zh-CN': '支付宝', 'en-US': 'Alipay' }, color: 'geekblue' },
          ],
        },
        {
          key: 'status',
          title: { 'zh-CN': '支付状态', 'en-US': 'Pay Status' },
          dataType: 'enum',
          width: 100,
          filterable: true,
          render: 'tag',
          enum: [
            { value: 'paid', label: { 'zh-CN': '已支付', 'en-US': 'Paid' }, color: 'blue' },
            {
              value: 'credited',
              label: { 'zh-CN': '已发货', 'en-US': 'Credited' },
              color: 'green',
            },
            { value: 'failed', label: { 'zh-CN': '支付失败', 'en-US': 'Failed' }, color: 'red' },
            {
              value: 'refunded',
              label: { 'zh-CN': '已退款', 'en-US': 'Refunded' },
              color: 'default',
            },
          ],
        },
        {
          key: 'gameCredit',
          title: { 'zh-CN': '游戏发货', 'en-US': 'Game Credit' },
          dataType: 'enum',
          width: 110,
          filterable: true,
          render: 'tag',
          enum: [
            {
              value: 'credited',
              label: { 'zh-CN': '已发货', 'en-US': 'Credited' },
              color: 'green',
            },
            { value: 'pending', label: { 'zh-CN': '发货中', 'en-US': 'Pending' }, color: 'blue' },
            { value: 'missing', label: { 'zh-CN': '缺发货', 'en-US': 'Missing' }, color: 'red' },
            {
              value: 'not_needed',
              label: { 'zh-CN': '无需发货', 'en-US': 'Not Needed' },
              color: 'default',
            },
          ],
        },
        {
          key: 'reconciliation',
          title: { 'zh-CN': '对账结论', 'en-US': 'Reconciliation' },
          dataType: 'enum',
          width: 110,
          filterable: true,
          render: 'tag',
          enum: [
            { value: 'match', label: { 'zh-CN': '一致', 'en-US': 'Match' }, color: 'green' },
            {
              value: 'missing_credit',
              label: { 'zh-CN': '缺发货', 'en-US': 'Missing Credit' },
              color: 'red',
            },
            {
              value: 'missing_order',
              label: { 'zh-CN': '缺订单', 'en-US': 'Missing Order' },
              color: 'volcano',
            },
          ],
        },
        {
          key: 'paidAt',
          title: { 'zh-CN': '支付时间（UTC）', 'en-US': 'Paid At (UTC)' },
          dataType: 'datetime',
          width: 180,
          sortable: true,
        },
      ],
      pagination: { enabled: true, defaultSize: 10, pageSizes: [10, 20, 50] },
      rowActions: [
        {
          key: 'recredit',
          title: { 'zh-CN': '补发奖励', 'en-US': 'Re-credit' },
          type: 'primary',
          form: {
            jsonSchema: {
              type: 'object',
              properties: {
                reason: { type: 'string', title: '补发原因' },
              },
              required: ['reason'],
            },
            layout: 'vertical',
          },
          bindingId: 'action.recredit',
        },
      ],
    },
    detailView: {
      layout: 'horizontal',
      fields: [
        { key: 'orderNo', title: { 'zh-CN': '订单号', 'en-US': 'Order No.' }, dataType: 'string' },
        {
          key: 'playerId',
          title: { 'zh-CN': '玩家 ID', 'en-US': 'Player ID' },
          dataType: 'string',
        },
        { key: 'nickname', title: { 'zh-CN': '昵称', 'en-US': 'Nickname' }, dataType: 'string' },
        { key: 'amount', title: { 'zh-CN': '金额', 'en-US': 'Amount' }, dataType: 'number' },
        { key: 'currency', title: { 'zh-CN': '币种', 'en-US': 'Currency' }, dataType: 'string' },
        { key: 'channel', title: { 'zh-CN': '支付渠道', 'en-US': 'Channel' }, dataType: 'string' },
        {
          key: 'status',
          title: { 'zh-CN': '支付状态', 'en-US': 'Pay Status' },
          dataType: 'string',
        },
        {
          key: 'gameCredit',
          title: { 'zh-CN': '游戏发货状态', 'en-US': 'Game Credit Status' },
          dataType: 'string',
        },
        {
          key: 'payTransactionId',
          title: { 'zh-CN': '支付凭据号', 'en-US': 'Pay Transaction ID' },
          dataType: 'string',
        },
        {
          key: 'gameCreditRef',
          title: { 'zh-CN': '游戏发货凭据号', 'en-US': 'Game Credit Ref' },
          dataType: 'string',
        },
        {
          key: 'reconciliation',
          title: { 'zh-CN': '对账结论', 'en-US': 'Reconciliation' },
          dataType: 'string',
        },
        {
          key: 'paidAt',
          title: { 'zh-CN': '支付时间（UTC）', 'en-US': 'Paid At (UTC)' },
          dataType: 'datetime',
        },
        {
          key: 'creditedAt',
          title: { 'zh-CN': '发货时间（UTC）', 'en-US': 'Credited At (UTC)' },
          dataType: 'datetime',
        },
      ],
    },
  },
  bindings: [
    {
      id: 'list',
      functionId: 'order.list',
      usage: 'query',
      selectors: {
        input: { assignments: [] },
        output: [
          { stateKey: 'items', source: '/items', shape: 'collection' },
          { stateKey: 'total', source: '/total', shape: 'scalar' },
        ],
      },
      execution: { mode: 'sync' },
    },
    {
      id: 'detail',
      functionId: 'order.get',
      usage: 'detail',
      selectors: {
        input: { assignments: [{ target: '/orderNo', source: { kind: 'row', path: '/orderNo' } }] },
        output: [{ stateKey: 'detail', source: '', shape: 'object' }],
      },
      execution: { mode: 'sync' },
    },
    {
      id: 'action.recredit',
      functionId: 'order.recredit',
      usage: 'action',
      selectors: {
        input: {
          assignments: [
            { target: '/orderNo', source: { kind: 'row', path: '/orderNo' } },
            { target: '/reason', source: { kind: 'form', path: '/reason' } },
          ],
        },
        output: [{ stateKey: 'result', source: '', shape: 'object' }],
      },
      execution: { mode: 'sync', requireConfirm: false },
    },
  ],
};

/**
 * 创建订单核查 demo 执行器：内存实现 list/detail 与补发奖励。
 * 补发仅允许「缺发货」订单，重复补发与已发货订单直接报错（幂等边界示例）。
 */
export function createOrderReviewDemoExecute(
  store: GameOpsDemoStore = createGameOpsDemoStore(),
): PageExecuteFn {
  let orders = store.orders;
  const nextRequestId = createRequestIdFactory();

  return async (bindingId, context): Promise<PageExecutionResult> => {
    const form = asRecord(context.form);
    const row = asRecord(context.row);

    switch (bindingId) {
      case 'list': {
        const current = Number(form.current) || 1;
        const pageSize = Number(form.pageSize) || 10;
        let filtered = orders;
        if (form.nickname) {
          filtered = filtered.filter((order) => order.nickname.includes(String(form.nickname)));
        }
        if (form.channel) {
          filtered = filtered.filter((order) => order.channel === form.channel);
        }
        if (form.reconciliation) {
          filtered = filtered.filter((order) => order.reconciliation === form.reconciliation);
        }
        const start = (current - 1) * pageSize;
        return {
          kind: 'sync',
          requestId: nextRequestId(),
          data: { items: filtered.slice(start, start + pageSize), total: filtered.length },
        };
      }
      case 'detail': {
        const order = orders.find((item) => item.orderNo === String(row.orderNo));
        return { kind: 'sync', requestId: nextRequestId(), data: order ?? null };
      }
      case 'action.recredit': {
        const order = orders.find((item) => item.orderNo === String(row.orderNo));
        if (!order) {
          throw new Error('order_not_found');
        }
        if (order.gameCredit === 'credited') {
          throw new Error('already_credited');
        }
        if (order.gameCredit !== 'missing') {
          throw new Error('credit_not_applicable');
        }
        orders = orders.map((item) =>
          item.orderNo === order.orderNo
            ? {
                ...item,
                gameCredit: 'credited',
                status: 'credited',
                reconciliation: 'match',
                gameCreditRef: `credit-re-${order.orderNo}`,
                creditedAt: new Date().toISOString(),
              }
            : item,
        );
        store.pushOpRecord({
          operator: 'admin',
          action: 'order.recredit',
          target: order.orderNo,
          result: 'success',
        });
        return {
          kind: 'sync',
          requestId: nextRequestId(),
          data: { success: true, orderNo: order.orderNo },
        };
      }
      default:
        return { kind: 'sync', requestId: nextRequestId(), data: { success: true } };
    }
  };
}

/**
 * 订单核查模板页面：订单列表 + 凭据分列详情 + 缺发货补发。
 */
const OrderReviewTemplate: React.FC = () => {
  const intl = useIntl();
  const execute = useMemo(() => createOrderReviewDemoExecute(), []);
  return (
    <PageContainer
      title={intl.formatMessage({
        id: 'component.pageRenderer.orderReview.pageTitle',
        defaultMessage: '订单核查',
      })}
      subTitle={intl.formatMessage({
        id: 'component.pageRenderer.orderReview.pageSubtitle',
        defaultMessage: '业务模板 · 订单查询 / 发货核查 / 对账差异 / 补发',
      })}
    >
      <PageRenderer pageSpec={orderReviewPageSpec} onExecute={execute} />
    </PageContainer>
  );
};

export default OrderReviewTemplate;
