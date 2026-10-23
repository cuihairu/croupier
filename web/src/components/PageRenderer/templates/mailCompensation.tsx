/**
 * mailCompensation - 邮件与补偿业务模板（默认菜单 mail 类）
 *
 * 覆盖业务闭环：任务列表（邮件/补偿两类共用，type 区分）→ 详情
 * （目标人数快照、每人数量与总量、审批号）→ 新建邮件/补偿任务（create，
 * 补偿进入待审批态）→ 失败重试 / 撤回（行操作）。
 *
 * 内置 demo 执行器（内存数据，gameOpsDemoData 共享仓库），接入真实环境时
 * 将 onExecute 替换为 `executePageBinding` 即可。
 *
 * @module components/PageRenderer/templates/mailCompensation
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

/** 邮件与补偿任务页面规格 */
export const mailCompensationPageSpec: PageSpec = {
  pageKey: 'template--mail-compensation',
  type: 'resource',
  resourceKey: 'ops_tasks',
  title: { 'zh-CN': '邮件与补偿', 'en-US': 'Mail & Compensation' },
  description: {
    'zh-CN': '邮件任务、补偿任务、目标快照与失败处理（模板示例）',
    'en-US':
      'Mail tasks, compensation tasks, target snapshots and failure handling (template demo)',
  },
  category: {
    key: 'mail',
  },
  resource: {
    listView: {
      identityKey: 'id',
      columns: [
        {
          key: 'id',
          title: { 'zh-CN': '任务 ID', 'en-US': 'Task ID' },
          dataType: 'string',
          width: 150,
          fixed: 'left',
          render: 'copy',
        },
        {
          key: 'type',
          title: { 'zh-CN': '类型', 'en-US': 'Type' },
          dataType: 'enum',
          width: 100,
          filterable: true,
          render: 'tag',
          enum: [
            { value: 'mail', label: { 'zh-CN': '邮件', 'en-US': 'Mail' }, color: 'blue' },
            {
              value: 'compensation',
              label: { 'zh-CN': '补偿', 'en-US': 'Compensation' },
              color: 'purple',
            },
          ],
        },
        {
          key: 'title',
          title: { 'zh-CN': '标题/事由', 'en-US': 'Title / Reason' },
          dataType: 'string',
        },
        {
          key: 'receiverRange',
          title: { 'zh-CN': '发放范围', 'en-US': 'Receiver Range' },
          dataType: 'enum',
          width: 110,
          render: 'tag',
          enum: [
            { value: 'all', label: { 'zh-CN': '全服', 'en-US': 'All' }, color: 'cyan' },
            {
              value: 'server',
              label: { 'zh-CN': '指定区服', 'en-US': 'Server' },
              color: 'geekblue',
            },
            {
              value: 'players',
              label: { 'zh-CN': '指定玩家', 'en-US': 'Players' },
              color: 'default',
            },
          ],
        },
        {
          key: 'targetCount',
          title: { 'zh-CN': '目标人数', 'en-US': 'Targets' },
          dataType: 'number',
          width: 100,
          sortable: true,
        },
        {
          key: 'perAmount',
          title: { 'zh-CN': '每人数量', 'en-US': 'Per Target' },
          dataType: 'number',
          width: 100,
        },
        {
          key: 'totalAmount',
          title: { 'zh-CN': '总量（金）', 'en-US': 'Total (gold)' },
          dataType: 'number',
          width: 120,
        },
        {
          key: 'status',
          title: { 'zh-CN': '状态', 'en-US': 'Status' },
          dataType: 'enum',
          width: 120,
          filterable: true,
          render: 'tag',
          enum: [
            {
              value: 'pending_approval',
              label: { 'zh-CN': '待审批', 'en-US': 'Pending Approval' },
              color: 'orange',
            },
            { value: 'sending', label: { 'zh-CN': '发送中', 'en-US': 'Sending' }, color: 'blue' },
            { value: 'sent', label: { 'zh-CN': '已发送', 'en-US': 'Sent' }, color: 'green' },
            {
              value: 'partial_failed',
              label: { 'zh-CN': '部分失败', 'en-US': 'Partial Failed' },
              color: 'volcano',
            },
            {
              value: 'executing',
              label: { 'zh-CN': '执行中', 'en-US': 'Executing' },
              color: 'blue',
            },
            {
              value: 'completed',
              label: { 'zh-CN': '已完成', 'en-US': 'Completed' },
              color: 'green',
            },
            { value: 'failed', label: { 'zh-CN': '失败', 'en-US': 'Failed' }, color: 'red' },
            {
              value: 'cancelled',
              label: { 'zh-CN': '已撤回', 'en-US': 'Cancelled' },
              color: 'default',
            },
          ],
        },
        {
          key: 'sentCount',
          title: { 'zh-CN': '已送达/人', 'en-US': 'Delivered' },
          dataType: 'number',
          width: 110,
        },
        {
          key: 'failedCount',
          title: { 'zh-CN': '失败/人', 'en-US': 'Failed' },
          dataType: 'number',
          width: 90,
        },
        {
          key: 'operator',
          title: { 'zh-CN': '操作人', 'en-US': 'Operator' },
          dataType: 'string',
          width: 100,
        },
        {
          key: 'createdAt',
          title: { 'zh-CN': '创建时间（UTC）', 'en-US': 'Created At (UTC)' },
          dataType: 'datetime',
          width: 180,
        },
      ],
      pagination: { enabled: true, defaultSize: 10, pageSizes: [10, 20, 50] },
      rowActions: [
        {
          key: 'retryFailed',
          title: { 'zh-CN': '重试失败', 'en-US': 'Retry Failed' },
          type: 'primary',
          confirm: true,
          confirmTitle: { 'zh-CN': '重试失败投递', 'en-US': 'Retry Failed Deliveries' },
          confirmDescription: {
            'zh-CN': '仅对失败目标重新投递，已送达目标不会重复收到。',
            'en-US':
              'Only failed targets are retried; delivered targets will not receive duplicates.',
          },
          bindingId: 'action.retryFailed',
        },
        {
          key: 'cancelTask',
          title: { 'zh-CN': '撤回', 'en-US': 'Withdraw' },
          type: 'danger',
          confirm: true,
          confirmTitle: { 'zh-CN': '撤回任务', 'en-US': 'Withdraw Task' },
          confirmDescription: {
            'zh-CN': '撤回后未送达目标不再收到，已送达部分不回收。',
            'en-US':
              'Undelivered targets will no longer receive it; delivered parts are not recalled.',
          },
          bindingId: 'action.cancelTask',
        },
        {
          key: 'edit',
          title: { 'zh-CN': '编辑', 'en-US': 'Edit' },
          type: 'link',
          bindingId: 'update',
        },
      ],
    },
    detailView: {
      layout: 'horizontal',
      fields: [
        { key: 'id', title: { 'zh-CN': '任务 ID', 'en-US': 'Task ID' }, dataType: 'string' },
        { key: 'type', title: { 'zh-CN': '类型', 'en-US': 'Type' }, dataType: 'string' },
        {
          key: 'title',
          title: { 'zh-CN': '标题/事由', 'en-US': 'Title / Reason' },
          dataType: 'string',
        },
        {
          key: 'detail',
          title: { 'zh-CN': '正文/原因', 'en-US': 'Content / Reason' },
          dataType: 'string',
        },
        {
          key: 'receiverRange',
          title: { 'zh-CN': '发放范围', 'en-US': 'Receiver Range' },
          dataType: 'string',
        },
        {
          key: 'targetCount',
          title: { 'zh-CN': '目标人数快照', 'en-US': 'Target Snapshot' },
          dataType: 'number',
        },
        {
          key: 'perAmount',
          title: { 'zh-CN': '每人数量', 'en-US': 'Per Target' },
          dataType: 'number',
        },
        {
          key: 'totalAmount',
          title: { 'zh-CN': '总量（金）', 'en-US': 'Total (gold)' },
          dataType: 'number',
        },
        {
          key: 'approvalId',
          title: { 'zh-CN': '审批单号', 'en-US': 'Approval ID' },
          dataType: 'string',
        },
        { key: 'status', title: { 'zh-CN': '状态', 'en-US': 'Status' }, dataType: 'string' },
        {
          key: 'sentCount',
          title: { 'zh-CN': '已送达/人', 'en-US': 'Delivered' },
          dataType: 'number',
        },
        {
          key: 'failedCount',
          title: { 'zh-CN': '失败/人', 'en-US': 'Failed' },
          dataType: 'number',
        },
        { key: 'operator', title: { 'zh-CN': '操作人', 'en-US': 'Operator' }, dataType: 'string' },
        {
          key: 'createdAt',
          title: { 'zh-CN': '创建时间（UTC）', 'en-US': 'Created At (UTC)' },
          dataType: 'datetime',
        },
      ],
    },
    createForm: {
      jsonSchema: {
        type: 'object',
        properties: {
          type: {
            type: 'string',
            title: '任务类型',
            enum: ['mail', 'compensation'],
            enumNames: ['邮件', '补偿'],
          },
          title: { type: 'string', title: '邮件标题/补偿事由' },
          detail: { type: 'string', title: '邮件正文/补偿原因' },
          receiverRange: {
            type: 'string',
            title: '发放范围',
            enum: ['all', 'server', 'players'],
            enumNames: ['全服', '指定区服', '指定玩家'],
          },
          perAmount: { type: 'number', title: '补偿每人数量（金，邮件留空）' },
        },
        required: ['type', 'title'],
      },
      layout: 'vertical',
    },
    updateForm: {
      jsonSchema: {
        type: 'object',
        properties: {
          title: { type: 'string', title: '邮件标题/补偿事由' },
          detail: { type: 'string', title: '邮件正文/补偿原因' },
        },
        required: ['title'],
      },
      layout: 'vertical',
    },
  },
  bindings: [
    {
      id: 'list',
      functionId: 'mail_task.list',
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
      functionId: 'mail_task.get',
      usage: 'detail',
      selectors: {
        input: { assignments: [{ target: '/id', source: { kind: 'row', path: '/id' } }] },
        output: [{ stateKey: 'detail', source: '', shape: 'object' }],
      },
      execution: { mode: 'sync' },
    },
    {
      id: 'create',
      functionId: 'mail_task.create',
      usage: 'action',
      execution: { mode: 'sync' },
    },
    {
      id: 'update',
      functionId: 'mail_task.update',
      usage: 'action',
      execution: { mode: 'sync' },
    },
    {
      id: 'action.retryFailed',
      functionId: 'mail_task.retry_failed',
      usage: 'action',
      selectors: {
        input: {
          assignments: [{ target: '/id', source: { kind: 'row', path: '/id' } }],
        },
        output: [{ stateKey: 'result', source: '', shape: 'object' }],
      },
      execution: { mode: 'sync', requireConfirm: true },
    },
    {
      id: 'action.cancelTask',
      functionId: 'mail_task.cancel',
      usage: 'action',
      selectors: {
        input: {
          assignments: [{ target: '/id', source: { kind: 'row', path: '/id' } }],
        },
        output: [{ stateKey: 'result', source: '', shape: 'object' }],
      },
      execution: { mode: 'sync', requireConfirm: true },
    },
  ],
};

/**
 * 创建邮件与补偿 demo 执行器：内存实现 list/detail/create/update 与
 * 失败重试、撤回。补偿创建进入待审批态并落审批号，邮件直接进入发送完成。
 */
export function createMailCompensationDemoExecute(
  store: GameOpsDemoStore = createGameOpsDemoStore(),
): PageExecuteFn {
  let tasks = store.tasks;
  let seq = tasks.length + 1;
  const nextId = () => `task-demo-${++seq}`;
  const nextRequestId = createRequestIdFactory();

  return async (bindingId, context): Promise<PageExecutionResult> => {
    const form = asRecord(context.form);
    const row = asRecord(context.row);

    switch (bindingId) {
      case 'list': {
        const current = Number(form.current) || 1;
        const pageSize = Number(form.pageSize) || 10;
        let filtered = tasks;
        if (form.type) {
          filtered = filtered.filter((task) => task.type === form.type);
        }
        if (form.status) {
          filtered = filtered.filter((task) => task.status === form.status);
        }
        const start = (current - 1) * pageSize;
        return {
          kind: 'sync',
          requestId: nextRequestId(),
          data: { items: filtered.slice(start, start + pageSize), total: filtered.length },
        };
      }
      case 'detail': {
        const task = tasks.find((item) => item.id === String(row.id));
        return { kind: 'sync', requestId: nextRequestId(), data: task ?? null };
      }
      case 'create': {
        const type = String(form.type) === 'compensation' ? 'compensation' : 'mail';
        const targetCount = type === 'mail' ? 26 : 1;
        const perAmount = type === 'compensation' ? Number(form.perAmount) || 0 : null;
        const approvalId = type === 'compensation' ? `approval-${nextId()}` : null;
        tasks = [
          {
            id: nextId(),
            type,
            title: String(form.title ?? '未命名任务'),
            detail: String(form.detail ?? ''),
            receiverRange:
              form.receiverRange === 'server' || form.receiverRange === 'players'
                ? form.receiverRange
                : 'all',
            targetCount,
            perAmount,
            totalAmount: perAmount === null ? null : perAmount * targetCount,
            currency: type === 'compensation' ? '金' : null,
            status: type === 'compensation' ? 'pending_approval' : 'sent',
            sentCount: type === 'mail' ? targetCount : 0,
            failedCount: 0,
            operator: 'admin',
            approvalId,
            createdAt: new Date().toISOString(),
          },
          ...tasks,
        ];
        if (approvalId) {
          store.pushOpRecord({
            operator: 'admin',
            action: 'compensation.grant',
            target: tasks[0].id,
            result: 'pending_approval',
            approvalId,
          });
        }
        return { kind: 'sync', requestId: nextRequestId(), data: { id: tasks[0].id, approvalId } };
      }
      case 'update': {
        tasks = tasks.map((task) =>
          task.id === String(row.id)
            ? {
                ...task,
                title: String(form.title ?? task.title),
                detail: String(form.detail ?? task.detail),
              }
            : task,
        );
        return { kind: 'sync', requestId: nextRequestId(), data: { success: true } };
      }
      case 'action.retryFailed': {
        const task = tasks.find((item) => item.id === String(row.id));
        if (!task) {
          throw new Error('task_not_found');
        }
        if (task.failedCount === 0) {
          throw new Error('no_failed_targets');
        }
        tasks = tasks.map((item) =>
          item.id === task.id
            ? {
                ...item,
                sentCount: item.sentCount + item.failedCount,
                failedCount: 0,
                status: 'sent',
              }
            : item,
        );
        store.pushOpRecord({
          operator: 'admin',
          action: 'mail_task.retry_failed',
          target: task.id,
          result: 'success',
        });
        return {
          kind: 'sync',
          requestId: nextRequestId(),
          data: { success: true, retried: task.failedCount },
        };
      }
      case 'action.cancelTask': {
        const task = tasks.find((item) => item.id === String(row.id));
        if (!task) {
          throw new Error('task_not_found');
        }
        if (task.status === 'sent' || task.status === 'completed') {
          throw new Error('task_already_finished');
        }
        tasks = tasks.map((item) =>
          item.id === task.id ? { ...item, status: 'cancelled' } : item,
        );
        store.pushOpRecord({
          operator: 'admin',
          action: 'mail_task.cancel',
          target: task.id,
          result: 'success',
        });
        return { kind: 'sync', requestId: nextRequestId(), data: { success: true } };
      }
      default:
        return { kind: 'sync', requestId: nextRequestId(), data: { success: true } };
    }
  };
}

/**
 * 邮件与补偿模板页面：任务列表 + 详情 + 新建（邮件即时发送/补偿进审批）+
 * 失败重试 + 撤回。
 */
const MailCompensationTemplate: React.FC = () => {
  const intl = useIntl();
  const execute = useMemo(() => createMailCompensationDemoExecute(), []);
  return (
    <PageContainer
      title={intl.formatMessage({
        id: 'component.pageRenderer.mailCompensation.pageTitle',
        defaultMessage: '邮件与补偿',
      })}
      subTitle={intl.formatMessage({
        id: 'component.pageRenderer.mailCompensation.pageSubtitle',
        defaultMessage: '业务模板 · 任务列表 / 目标快照 / 审批 / 失败重试',
      })}
    >
      <PageRenderer pageSpec={mailCompensationPageSpec} onExecute={execute} />
    </PageContainer>
  );
};

export default MailCompensationTemplate;
