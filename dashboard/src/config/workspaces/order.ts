/**
 * 示例 Workspace 配置 - 订单管理
 */

import type { WorkspaceConfig } from '@/types/workspace';

export const orderWorkspaceConfig: WorkspaceConfig = {
  objectKey: 'order',
  title: '订单管理',
  description: '订单查询、处理和统计',
  icon: 'ShoppingOutlined',
  layout: {
    type: 'tabs',
    tabs: [
      {
        key: 'list',
        title: '订单列表',
        icon: 'UnorderedListOutlined',
        functions: ['order.list'],
        layout: {
          type: 'list',
          listFunction: 'order.list',
          columns: [
            { key: 'orderId', title: '订单号', width: 180, fixed: 'left' },
            { key: 'playerId', title: '玩家ID', width: 150 },
            { key: 'productName', title: '商品名称', width: 200 },
            { key: 'amount', title: '金额', width: 120, render: 'money', sortable: true },
            {
              key: 'status',
              title: '状态',
              width: 100,
              render: 'status',
              renderOptions: {
                statusMap: {
                  pending: { text: '待支付', status: 'default' },
                  paid: { text: '已支付', status: 'success' },
                  refunded: { text: '已退款', status: 'warning' },
                  cancelled: { text: '已取消', status: 'error' },
                },
              },
            },
            { key: 'createdAt', title: '创建时间', width: 180, render: 'datetime', sortable: true },
          ],
          rowActions: [
            {
              key: 'view',
              label: '查看详情',
              function: 'order.getInfo',
            },
            {
              key: 'refund',
              label: '退款',
              function: 'order.refund',
              permissions: ['order.refund'],
            },
          ],
          toolbarActions: [
            {
              key: 'export',
              label: '导出',
              function: 'order.export',
              permissions: ['order.export'],
            },
          ],
        },
      },
      {
        key: 'detail',
        title: '订单详情',
        icon: 'FileTextOutlined',
        functions: ['order.getInfo'],
        layout: {
          type: 'form-detail',
          queryFunction: 'order.getInfo',
          queryFields: [
            {
              key: 'orderId',
              label: '订单号',
              type: 'input',
              required: true,
              placeholder: '请输入订单号',
            },
          ],
          detailSections: [
            {
              title: '订单信息',
              fields: [
                { key: 'orderId', label: '订单号' },
                { key: 'playerId', label: '玩家ID' },
                { key: 'productId', label: '商品ID' },
                { key: 'productName', label: '商品名称' },
                { key: 'amount', label: '金额', render: 'money' },
                {
                  key: 'status',
                  label: '状态',
                  render: 'status',
                  renderOptions: {
                    statusMap: {
                      pending: { text: '待支付', status: 'default' },
                      paid: { text: '已支付', status: 'success' },
                      refunded: { text: '已退款', status: 'warning' },
                      cancelled: { text: '已取消', status: 'error' },
                    },
                  },
                },
              ],
            },
            {
              title: '支付信息',
              fields: [
                { key: 'paymentMethod', label: '支付方式' },
                { key: 'paymentChannel', label: '支付渠道' },
                { key: 'transactionId', label: '交易号' },
                { key: 'paidAt', label: '支付时间', render: 'datetime' },
              ],
            },
            {
              title: '时间信息',
              fields: [
                { key: 'createdAt', label: '创建时间', render: 'datetime' },
                { key: 'updatedAt', label: '更新时间', render: 'datetime' },
              ],
            },
          ],
          actions: [
            {
              key: 'refund',
              label: '退款',
              function: 'order.refund',
              type: 'popconfirm',
              buttonType: 'default',
              danger: true,
              permissions: ['order.refund'],
            },
          ],
        },
      },
    ],
  },
  permissions: ['order.view'],
  published: true,
  publishedAt: '2026-03-06T08:00:00Z',
  publishedBy: 'admin',
  menuOrder: 2,
  meta: {
    createdAt: '2026-03-06T08:00:00Z',
    updatedAt: '2026-03-06T08:00:00Z',
    createdBy: 'admin',
    version: 1,
  },
};

export default orderWorkspaceConfig;
