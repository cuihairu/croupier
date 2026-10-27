/**
 * banManagement - 封禁管理业务模板（默认菜单 ban 类）
 *
 * 覆盖业务闭环：封禁列表（对象身份/原因/时长/到期/状态）→ 详情
 * （操作人/审批人）→ 新建封禁（create，原因枚举 + 时长小时数）→
 * 解封 / 延长时长（行操作）→ 批量解封（跨页选择核对）。
 *
 * 内置 demo 执行器（内存数据），接入真实环境时将 onExecute 替换为
 * `executePageBinding` 即可。
 *
 * @module components/PageRenderer/templates/banManagement
 */

import React, { useMemo } from 'react';
import { useIntl } from '@umijs/max';
import { PageContainer } from '@ant-design/pro-components';
import PageRenderer from '@/components/PageRenderer';
import type { PageSpec, PageExecuteFn, PageExecutionResult } from '@/types/dashboard';
import {
  asRecord,
  createRequestIdFactory,
  type GameOpsBanRecord,
  type GameOpsDemoStore,
} from './gameOpsDemoData';
import { gameOpsDemoRuntimeStore } from './gameOpsDemoRuntime';

/** 封禁管理页面规格 */
export const banManagementPageSpec: PageSpec = {
  pageKey: 'template--ban-management',
  type: 'resource',
  resourceKey: 'bans',
  title: { 'zh-CN': '封禁管理', 'en-US': 'Ban Management' },
  description: {
    'zh-CN': '封禁列表、封禁/解封申请、处置历史（模板示例）',
    'en-US': 'Ban list, ban/unban requests and disposition history (template demo)',
  },
  category: {
    key: 'ban',
  },
  resource: {
    listView: {
      identityKey: 'id',
      columns: [
        {
          key: 'id',
          title: { 'zh-CN': '记录 ID', 'en-US': 'Record ID' },
          dataType: 'string',
          width: 140,
          fixed: 'left',
          render: 'copy',
          filterable: true,
        },
        {
          key: 'playerId',
          title: { 'zh-CN': '玩家 ID', 'en-US': 'Player ID' },
          dataType: 'string',
          width: 110,
          render: 'copy',
          filterable: true,
        },
        {
          key: 'nickname',
          title: { 'zh-CN': '昵称', 'en-US': 'Nickname' },
          dataType: 'string',
          filterable: true,
        },
        {
          key: 'serverId',
          title: { 'zh-CN': '区服', 'en-US': 'Server' },
          dataType: 'enum',
          width: 110,
          filterable: true,
          enum: [
            { value: 's1', label: { 'zh-CN': '电信一区', 'en-US': 'Telcom 1' } },
            { value: 's2', label: { 'zh-CN': '电信二区', 'en-US': 'Telcom 2' } },
            { value: 's3', label: { 'zh-CN': '联通一区', 'en-US': 'Unicom 1' } },
          ],
        },
        {
          key: 'reason',
          title: { 'zh-CN': '封禁原因', 'en-US': 'Reason' },
          dataType: 'enum',
          width: 110,
          filterable: true,
          render: 'tag',
          enum: [
            {
              value: 'cheating',
              label: { 'zh-CN': '使用外挂', 'en-US': 'Cheating' },
              color: 'red',
            },
            { value: 'fraud', label: { 'zh-CN': '欺诈', 'en-US': 'Fraud' }, color: 'volcano' },
            { value: 'abuse', label: { 'zh-CN': '辱骂他人', 'en-US': 'Abuse' }, color: 'orange' },
            {
              value: 'risk',
              label: { 'zh-CN': '账号风险', 'en-US': 'Account Risk' },
              color: 'gold',
            },
          ],
        },
        {
          key: 'durationHours',
          title: { 'zh-CN': '时长', 'en-US': 'Duration' },
          dataType: 'number',
          width: 120,
          sortable: true,
          unit: { 'zh-CN': '小时', 'en-US': 'hours' },
        },
        {
          key: 'expiresAt',
          title: { 'zh-CN': '到期时间', 'en-US': 'Expires At' },
          dataType: 'datetime',
          width: 180,
          sortable: true,
        },
        {
          key: 'status',
          title: { 'zh-CN': '状态', 'en-US': 'Status' },
          dataType: 'enum',
          width: 100,
          filterable: true,
          render: 'tag',
          enum: [
            { value: 'active', label: { 'zh-CN': '生效中', 'en-US': 'Active' }, color: 'red' },
            {
              value: 'expired',
              label: { 'zh-CN': '已到期', 'en-US': 'Expired' },
              color: 'default',
            },
            { value: 'lifted', label: { 'zh-CN': '已解封', 'en-US': 'Lifted' }, color: 'green' },
          ],
        },
        {
          key: 'operator',
          title: { 'zh-CN': '操作人', 'en-US': 'Operator' },
          dataType: 'string',
          width: 100,
        },
        {
          key: 'approver',
          title: { 'zh-CN': '审批人', 'en-US': 'Approver' },
          dataType: 'string',
          width: 100,
        },
      ],
      pagination: { enabled: true, defaultSize: 10, pageSizes: [10, 20, 50] },
      rowActions: [
        {
          key: 'unban',
          title: { 'zh-CN': '解封', 'en-US': 'Unban' },
          type: 'primary',
          form: {
            jsonSchema: {
              type: 'object',
              properties: {
                reason: { type: 'string', title: '解封原因' },
              },
              required: ['reason'],
            },
            layout: 'vertical',
          },
          bindingId: 'action.unban',
        },
        {
          key: 'extend',
          title: { 'zh-CN': '延长时长', 'en-US': 'Extend' },
          form: {
            jsonSchema: {
              type: 'object',
              properties: {
                extraHours: { type: 'number', title: '延长时长（小时）' },
              },
              required: ['extraHours'],
            },
            layout: 'vertical',
          },
          bindingId: 'action.extend',
        },
      ],
      batchActions: [
        {
          key: 'batchUnban',
          title: { 'zh-CN': '批量解封', 'en-US': 'Batch Unban' },
          confirm: true,
          confirmTitle: { 'zh-CN': '批量解封', 'en-US': 'Batch Unban' },
          confirmDescription: {
            'zh-CN': '确认解封选中的封禁记录？仅生效中的记录会被解封。',
            'en-US': 'Unban the selected records? Only active records will be lifted.',
          },
          bindingId: 'action.batchUnban',
        },
      ],
    },
    detailView: {
      layout: 'horizontal',
      fields: [
        { key: 'id', title: { 'zh-CN': '记录 ID', 'en-US': 'Record ID' }, dataType: 'string' },
        {
          key: 'playerId',
          title: { 'zh-CN': '玩家 ID', 'en-US': 'Player ID' },
          dataType: 'string',
        },
        { key: 'nickname', title: { 'zh-CN': '昵称', 'en-US': 'Nickname' }, dataType: 'string' },
        { key: 'serverId', title: { 'zh-CN': '区服', 'en-US': 'Server' }, dataType: 'string' },
        { key: 'reason', title: { 'zh-CN': '封禁原因', 'en-US': 'Reason' }, dataType: 'string' },
        {
          key: 'durationHours',
          title: { 'zh-CN': '时长', 'en-US': 'Duration' },
          dataType: 'number',
        },
        {
          key: 'bannedAt',
          title: { 'zh-CN': '封禁时间', 'en-US': 'Banned At' },
          dataType: 'datetime',
        },
        {
          key: 'expiresAt',
          title: { 'zh-CN': '到期时间', 'en-US': 'Expires At' },
          dataType: 'datetime',
        },
        { key: 'status', title: { 'zh-CN': '状态', 'en-US': 'Status' }, dataType: 'string' },
        { key: 'operator', title: { 'zh-CN': '操作人', 'en-US': 'Operator' }, dataType: 'string' },
        { key: 'approver', title: { 'zh-CN': '审批人', 'en-US': 'Approver' }, dataType: 'string' },
      ],
    },
    createForm: {
      jsonSchema: {
        type: 'object',
        properties: {
          playerId: { type: 'string', title: '玩家 ID' },
          serverId: {
            type: 'string',
            title: '区服',
            enum: ['s1', 's2', 's3'],
            enumNames: ['电信一区', '电信二区', '联通一区'],
          },
          reason: {
            type: 'string',
            title: '封禁原因',
            enum: ['cheating', 'fraud', 'abuse', 'risk'],
            enumNames: ['使用外挂', '欺诈', '辱骂他人', '账号风险'],
          },
          durationHours: { type: 'number', title: '封禁时长（小时）' },
        },
        required: ['playerId', 'reason', 'durationHours'],
      },
      layout: 'vertical',
    },
  },
  bindings: [
    {
      id: 'list',
      functionId: 'ban.list',
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
      functionId: 'ban.get',
      usage: 'detail',
      selectors: {
        input: { assignments: [{ target: '/id', source: { kind: 'row', path: '/id' } }] },
        output: [{ stateKey: 'detail', source: '', shape: 'object' }],
      },
      execution: { mode: 'sync' },
    },
    {
      id: 'create',
      functionId: 'ban.create',
      usage: 'action',
      execution: { mode: 'sync' },
    },
    {
      id: 'action.unban',
      functionId: 'ban.unban',
      usage: 'action',
      selectors: {
        input: {
          assignments: [
            { target: '/id', source: { kind: 'row', path: '/id' } },
            { target: '/reason', source: { kind: 'form', path: '/reason' } },
          ],
        },
        output: [{ stateKey: 'result', source: '', shape: 'object' }],
      },
      execution: { mode: 'sync', requireConfirm: false },
    },
    {
      id: 'action.extend',
      functionId: 'ban.extend',
      usage: 'action',
      selectors: {
        input: {
          assignments: [
            { target: '/id', source: { kind: 'row', path: '/id' } },
            { target: '/extraHours', source: { kind: 'form', path: '/extraHours' } },
          ],
        },
        output: [{ stateKey: 'result', source: '', shape: 'object' }],
      },
      execution: { mode: 'sync', requireConfirm: false },
    },
    {
      id: 'action.batchUnban',
      functionId: 'ban.batch_unban',
      usage: 'action',
      selectors: {
        input: {
          assignments: [{ target: '/ids', source: { kind: 'selection', path: '/id' } }],
        },
        output: [{ stateKey: 'result', source: '', shape: 'object' }],
      },
      execution: { mode: 'sync', requireConfirm: true },
    },
  ],
};

/**
 * 创建封禁管理 demo 执行器：内存实现 list/detail/create 与解封、
 * 延长时长、批量解封。非生效中记录解封/延期时报错（业务校验前置）。
 */
export function createBanManagementDemoExecute(
  store: GameOpsDemoStore = gameOpsDemoRuntimeStore,
): PageExecuteFn {
  let seq = store.bans.length + 1;
  const nextRequestId = createRequestIdFactory();

  const nextId = () => `ban-demo-${++seq}`;

  return async (bindingId, context): Promise<PageExecutionResult> => {
    let bans = store.bans;
    const form = asRecord(context.form);
    const row = asRecord(context.row);
    const selection = Array.isArray(context.selection) ? context.selection : [];

    try {
      switch (bindingId) {
        case 'list': {
          const current = Number(form.current) || 1;
          const pageSize = Number(form.pageSize) || 10;
          let filtered = bans;
          if (form.id) {
            filtered = filtered.filter((ban) => ban.id === String(form.id));
          }
          if (form.playerId) {
            filtered = filtered.filter((ban) => ban.playerId === String(form.playerId));
          }
          if (form.nickname) {
            filtered = filtered.filter((ban) => ban.nickname.includes(String(form.nickname)));
          }
          if (form.status) {
            filtered = filtered.filter((ban) => ban.status === form.status);
          }
          if (form.reason) {
            filtered = filtered.filter((ban) => ban.reason === form.reason);
          }
          const start = (current - 1) * pageSize;
          return {
            kind: 'sync',
            requestId: nextRequestId(),
            data: { items: filtered.slice(start, start + pageSize), total: filtered.length },
          };
        }
        case 'detail': {
          const ban = bans.find((item) => item.id === String(row.id));
          return { kind: 'sync', requestId: nextRequestId(), data: ban ?? null };
        }
        case 'create': {
          const playerId = String(form.playerId ?? '').trim();
          const durationHours = Number(form.durationHours) || 0;
          if (!playerId) {
            throw new Error('player_id_required');
          }
          if (durationHours <= 0) {
            throw new Error('duration_must_be_positive');
          }
          const now = new Date();
          const id = nextId();
          const record: GameOpsBanRecord = {
            id,
            playerId,
            nickname: `玩家${playerId}号`,
            serverId: String(form.serverId ?? 's1'),
            approvalId: `approval-${id}`,
            reason: String(form.reason ?? 'risk') as GameOpsBanRecord['reason'],
            durationHours,
            bannedAt: now.toISOString(),
            expiresAt: new Date(now.getTime() + durationHours * 3600_000).toISOString(),
            status: 'active',
            operator: 'admin',
            approver: 'admin',
          };
          bans = [record, ...bans];
          store.pushOpRecord({
            operator: 'admin',
            action: 'player.ban',
            target: `player ${playerId}`,
            result: 'success',
            approvalId: record.approvalId,
          });
          return { kind: 'sync', requestId: nextRequestId(), data: { id: record.id } };
        }
        case 'action.unban': {
          const ban = bans.find((item) => item.id === String(row.id));
          if (!ban) {
            throw new Error('ban_not_found');
          }
          if (ban.status !== 'active') {
            throw new Error('ban_not_active');
          }
          bans = bans.map((item) => (item.id === ban.id ? { ...item, status: 'lifted' } : item));
          store.pushOpRecord({
            operator: 'admin',
            action: 'player.unban',
            target: `player ${ban.playerId}`,
            result: 'success',
          });
          return { kind: 'sync', requestId: nextRequestId(), data: { success: true } };
        }
        case 'action.extend': {
          const ban = bans.find((item) => item.id === String(row.id));
          if (!ban) {
            throw new Error('ban_not_found');
          }
          if (ban.status !== 'active') {
            throw new Error('ban_not_active');
          }
          const extraHours = Number(form.extraHours) || 0;
          if (extraHours <= 0) {
            throw new Error('duration_must_be_positive');
          }
          bans = bans.map((item) =>
            item.id === ban.id
              ? {
                  ...item,
                  durationHours: item.durationHours + extraHours,
                  expiresAt: new Date(
                    new Date(item.expiresAt).getTime() + extraHours * 3600_000,
                  ).toISOString(),
                }
              : item,
          );
          store.pushOpRecord({
            operator: 'admin',
            action: 'ban.extend',
            target: ban.id,
            result: 'success',
          });
          return { kind: 'sync', requestId: nextRequestId(), data: { success: true, extraHours } };
        }
        case 'action.batchUnban': {
          const ids = selection.map((item) => String(asRecord(item as never).id));
          let lifted = 0;
          bans = bans.map((item) => {
            if (ids.includes(item.id) && item.status === 'active') {
              lifted += 1;
              return { ...item, status: 'lifted' };
            }
            return item;
          });
          if (lifted === 0) {
            throw new Error('no_active_ban_selected');
          }
          store.pushOpRecord({
            operator: 'admin',
            action: 'ban.batch_unban',
            target: `${lifted} records`,
            result: 'success',
          });
          return { kind: 'sync', requestId: nextRequestId(), data: { success: true, lifted } };
        }
        default:
          return { kind: 'sync', requestId: nextRequestId(), data: { success: true } };
      }
    } finally {
      store.bans = bans;
    }
  };
}

/**
 * 封禁管理模板页面：封禁列表 + 详情 + 新建封禁 + 解封/延期 + 批量解封。
 */
const BanManagementTemplate: React.FC = () => {
  const intl = useIntl();
  const execute = useMemo(() => createBanManagementDemoExecute(), []);
  return (
    <PageContainer
      title={intl.formatMessage({
        id: 'component.pageRenderer.banManagement.pageTitle',
        defaultMessage: '封禁管理',
      })}
      subTitle={intl.formatMessage({
        id: 'component.pageRenderer.banManagement.pageSubtitle',
        defaultMessage: '业务模板 · 封禁列表 / 解封 / 延期 / 批量解封',
      })}
    >
      <PageRenderer pageSpec={banManagementPageSpec} onExecute={execute} />
    </PageContainer>
  );
};

export default BanManagementTemplate;
