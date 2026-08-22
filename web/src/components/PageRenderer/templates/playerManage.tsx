/**
 * PlayerManageTemplate - 玩家管理 CRUD 模板页面
 *
 * 覆盖完整资源页面形态：列表（筛选/排序/分页/列设置）、详情抽屉、
 * 新建/编辑/删除、行操作（封禁/充值/邮件）与批量操作（批量封禁/解封）。
 *
 * 内置 demo 执行器（内存数据），可直接渲染联调样式；接入真实环境时
 * 将 onExecute 替换为 `executePageBinding` 即可。
 *
 * @module components/PageRenderer/templates/playerManage
 */

import React, { useMemo } from 'react';
import { PageContainer } from '@ant-design/pro-components';
import PageRenderer from '@/components/PageRenderer';
import type {
  PageSpec,
  PageExecuteFn,
  PageExecutionResult,
  FormValues,
  JSONValue,
} from '@/types/dashboard';

// ---------------------------------------------------------------------------
// PageSpec
// ---------------------------------------------------------------------------

/** 玩家管理资源页面规格 */
export const playerManagePageSpec: PageSpec = {
  pageKey: 'template--player-manage',
  type: 'resource',
  resourceKey: 'players',
  title: { 'zh-CN': '玩家管理', 'en-US': 'Player Management' },
  description: {
    'zh-CN': '玩家列表、详情、封禁、充值与邮件触达',
    'en-US': 'Player list, detail, ban, recharge and mail',
  },
  category: {
    key: 'players',
    labels: { 'zh-CN': '玩家管理', 'en-US': 'Players' },
  },
  resource: {
    listView: {
      identityKey: 'id',
      columns: [
        {
          key: 'id',
          title: { 'zh-CN': '玩家 ID', 'en-US': 'Player ID' },
          dataType: 'string',
          width: 120,
          fixed: 'left',
          render: 'copy',
        },
        {
          key: 'nickname',
          title: { 'zh-CN': '昵称', 'en-US': 'Nickname' },
          dataType: 'string',
          filterable: true,
        },
        {
          key: 'level',
          title: { 'zh-CN': '等级', 'en-US': 'Level' },
          dataType: 'number',
          width: 90,
          sortable: true,
        },
        {
          key: 'vip',
          title: { 'zh-CN': 'VIP', 'en-US': 'VIP' },
          dataType: 'number',
          width: 80,
        },
        {
          key: 'status',
          title: { 'zh-CN': '状态', 'en-US': 'Status' },
          dataType: 'enum',
          width: 100,
          filterable: true,
          render: 'tag',
          enum: [
            { value: 'active', label: { 'zh-CN': '正常', 'en-US': 'Active' }, color: 'green' },
            { value: 'banned', label: { 'zh-CN': '已封禁', 'en-US': 'Banned' }, color: 'red' },
          ],
        },
        {
          key: 'balance',
          title: { 'zh-CN': '余额', 'en-US': 'Balance' },
          dataType: 'number',
          width: 110,
        },
        {
          key: 'lastLoginAt',
          title: { 'zh-CN': '最近登录', 'en-US': 'Last Login' },
          dataType: 'datetime',
          width: 180,
        },
      ],
      pagination: { enabled: true, defaultSize: 10, pageSizes: [10, 20, 50] },
      rowActions: [
        {
          key: 'ban',
          title: { 'zh-CN': '封禁', 'en-US': 'Ban' },
          type: 'danger',
          form: {
            jsonSchema: {
              type: 'object',
              properties: {
                reason: { type: 'string', title: '封禁原因' },
                duration: { type: 'number', title: '封禁时长（小时）' },
              },
              required: ['reason'],
            },
            layout: 'vertical',
          },
          bindingId: 'action.ban',
        },
        {
          key: 'recharge',
          title: { 'zh-CN': '充值', 'en-US': 'Recharge' },
          type: 'primary',
          form: {
            jsonSchema: {
              type: 'object',
              properties: {
                amount: { type: 'number', title: '充值金额' },
                channel: {
                  type: 'string',
                  title: '支付渠道',
                  enum: ['official', 'ios', 'android'],
                  enumNames: ['官方', 'iOS', 'Android'],
                },
              },
              required: ['amount'],
            },
            layout: 'vertical',
          },
          bindingId: 'action.recharge',
        },
        {
          key: 'mail',
          title: { 'zh-CN': '邮件', 'en-US': 'Mail' },
          form: {
            jsonSchema: {
              type: 'object',
              properties: {
                title: { type: 'string', title: '邮件标题' },
                content: { type: 'string', title: '邮件内容' },
              },
              required: ['title', 'content'],
            },
            layout: 'vertical',
          },
          bindingId: 'action.mail',
        },
        {
          key: 'edit',
          title: { 'zh-CN': '编辑', 'en-US': 'Edit' },
          type: 'link',
          bindingId: 'update',
        },
      ],
      batchActions: [
        {
          key: 'batchBan',
          title: { 'zh-CN': '批量封禁', 'en-US': 'Batch Ban' },
          type: 'danger',
          confirm: true,
          confirmTitle: { 'zh-CN': '批量封禁', 'en-US': 'Batch Ban' },
          confirmDescription: {
            'zh-CN': '确认封禁选中的玩家？',
            'en-US': 'Ban the selected players?',
          },
          bindingId: 'action.batchBan',
        },
        {
          key: 'batchUnban',
          title: { 'zh-CN': '批量解封', 'en-US': 'Batch Unban' },
          confirm: true,
          confirmTitle: { 'zh-CN': '批量解封', 'en-US': 'Batch Unban' },
          confirmDescription: {
            'zh-CN': '确认解封选中的玩家？',
            'en-US': 'Unban the selected players?',
          },
          bindingId: 'action.batchUnban',
        },
      ],
    },
    detailView: {
      layout: 'horizontal',
      fields: [
        { key: 'id', title: { 'zh-CN': '玩家 ID', 'en-US': 'Player ID' }, dataType: 'string' },
        { key: 'nickname', title: { 'zh-CN': '昵称', 'en-US': 'Nickname' }, dataType: 'string' },
        { key: 'level', title: { 'zh-CN': '等级', 'en-US': 'Level' }, dataType: 'number' },
        { key: 'vip', title: { 'zh-CN': 'VIP', 'en-US': 'VIP' }, dataType: 'number' },
        { key: 'status', title: { 'zh-CN': '状态', 'en-US': 'Status' }, dataType: 'string' },
        { key: 'balance', title: { 'zh-CN': '余额', 'en-US': 'Balance' }, dataType: 'number' },
        {
          key: 'lastLoginAt',
          title: { 'zh-CN': '最近登录', 'en-US': 'Last Login' },
          dataType: 'datetime',
        },
        {
          key: 'createdAt',
          title: { 'zh-CN': '注册时间', 'en-US': 'Created At' },
          dataType: 'datetime',
        },
      ],
    },
    createForm: {
      jsonSchema: {
        type: 'object',
        properties: {
          nickname: { type: 'string', title: '昵称' },
          level: { type: 'number', title: '等级' },
          vip: { type: 'number', title: 'VIP 等级' },
        },
        required: ['nickname'],
      },
      layout: 'vertical',
    },
    updateForm: {
      jsonSchema: {
        type: 'object',
        properties: {
          nickname: { type: 'string', title: '昵称' },
          level: { type: 'number', title: '等级' },
          vip: { type: 'number', title: 'VIP 等级' },
        },
        required: ['nickname'],
      },
      layout: 'vertical',
    },
    deleteAction: {
      title: { 'zh-CN': '删除玩家', 'en-US': 'Delete Player' },
      description: {
        'zh-CN': '删除后不可恢复，确认删除该玩家？',
        'en-US': 'This cannot be undone. Delete this player?',
      },
      confirmText: { 'zh-CN': '确认删除', 'en-US': 'Delete' },
      cancelText: { 'zh-CN': '取消', 'en-US': 'Cancel' },
      bindingId: 'delete',
      risk: 'high',
    },
  },
  bindings: [
    {
      id: 'list',
      functionId: 'player.list',
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
      functionId: 'player.get',
      usage: 'detail',
      selectors: {
        input: { assignments: [{ target: '/id', source: { kind: 'row', path: '/id' } }] },
        output: [{ stateKey: 'detail', source: '', shape: 'object' }],
      },
      execution: { mode: 'sync' },
    },
    {
      id: 'create',
      functionId: 'player.create',
      usage: 'action',
      execution: { mode: 'sync' },
    },
    {
      id: 'update',
      functionId: 'player.update',
      usage: 'action',
      execution: { mode: 'sync' },
    },
    {
      id: 'delete',
      functionId: 'player.delete',
      usage: 'action',
      execution: { mode: 'sync' },
    },
    {
      id: 'action.ban',
      functionId: 'player.ban',
      usage: 'action',
      selectors: {
        input: {
          assignments: [
            { target: '/playerId', source: { kind: 'row', path: '/id' } },
            { target: '/reason', source: { kind: 'form', path: '/reason' } },
            { target: '/duration', source: { kind: 'form', path: '/duration' } },
          ],
        },
        output: [{ stateKey: 'result', source: '', shape: 'object' }],
      },
      execution: { mode: 'sync' },
    },
    {
      id: 'action.recharge',
      functionId: 'player.recharge',
      usage: 'action',
      selectors: {
        input: {
          assignments: [
            { target: '/playerId', source: { kind: 'row', path: '/id' } },
            { target: '/amount', source: { kind: 'form', path: '/amount' } },
            { target: '/channel', source: { kind: 'form', path: '/channel' } },
          ],
        },
        output: [{ stateKey: 'result', source: '', shape: 'object' }],
      },
      execution: { mode: 'sync', requireConfirm: false },
    },
    {
      id: 'action.mail',
      functionId: 'player.mail',
      usage: 'action',
      selectors: {
        input: {
          assignments: [
            { target: '/playerId', source: { kind: 'row', path: '/id' } },
            { target: '/title', source: { kind: 'form', path: '/title' } },
            { target: '/content', source: { kind: 'form', path: '/content' } },
          ],
        },
        output: [{ stateKey: 'result', source: '', shape: 'object' }],
      },
      execution: { mode: 'sync' },
    },
    {
      id: 'action.batchBan',
      functionId: 'player.batchBan',
      usage: 'action',
      selectors: {
        input: {
          assignments: [{ target: '/playerIds', source: { kind: 'selection', path: '/id' } }],
        },
        output: [{ stateKey: 'result', source: '', shape: 'object' }],
      },
      execution: { mode: 'sync', requireConfirm: true },
    },
    {
      id: 'action.batchUnban',
      functionId: 'player.batchUnban',
      usage: 'action',
      selectors: {
        input: {
          assignments: [{ target: '/playerIds', source: { kind: 'selection', path: '/id' } }],
        },
        output: [{ stateKey: 'result', source: '', shape: 'object' }],
      },
      execution: { mode: 'sync', requireConfirm: true },
    },
  ],
};

// ---------------------------------------------------------------------------
// Demo 执行器（内存数据，仅供模板预览/联调）
// ---------------------------------------------------------------------------

interface DemoPlayer {
  [key: string]: JSONValue;
  id: string;
  nickname: string;
  level: number;
  vip: number;
  status: 'active' | 'banned';
  balance: number;
  lastLoginAt: string;
  createdAt: string;
}

function seedPlayers(): DemoPlayer[] {
  const now = Date.now();
  return Array.from({ length: 23 }, (_, index) => {
    const seq = index + 1;
    return {
      id: `100${String(seq).padStart(2, '0')}`,
      nickname: `玩家${seq}号`,
      level: 5 + ((seq * 7) % 55),
      vip: seq % 6,
      status: seq % 7 === 0 ? 'banned' : 'active',
      balance: (seq * 137) % 5000,
      lastLoginAt: new Date(now - seq * 3600_000).toISOString(),
      createdAt: new Date(now - seq * 86400_000 * 3).toISOString(),
    };
  });
}

function asRecord(value: JSONValue | undefined): FormValues {
  return value && typeof value === 'object' && !Array.isArray(value) ? (value as FormValues) : {};
}

/**
 * 创建玩家管理 demo 执行器：内存实现 list/detail/create/update/delete
 * 与封禁/充值/邮件/批量操作，返回结构对齐 PageSpec selectors。
 */
export function createPlayerManageDemoExecute(): PageExecuteFn {
  let players = seedPlayers();
  let requestSeq = 0;
  const nextRequestId = () => `demo-${++requestSeq}`;

  return async (bindingId, context): Promise<PageExecutionResult> => {
    const form = asRecord(context.form);
    const row = asRecord(context.row);
    const selection = Array.isArray(context.selection) ? context.selection.map(String) : [];

    switch (bindingId) {
      case 'list': {
        const current = Number(form.current) || 1;
        const pageSize = Number(form.pageSize) || 10;
        let filtered = players;
        if (form.nickname) {
          filtered = filtered.filter((player) => player.nickname.includes(String(form.nickname)));
        }
        if (form.status) {
          filtered = filtered.filter((player) => player.status === form.status);
        }
        const start = (current - 1) * pageSize;
        return {
          kind: 'sync',
          requestId: nextRequestId(),
          data: { items: filtered.slice(start, start + pageSize), total: filtered.length },
        };
      }
      case 'detail': {
        const player = players.find((item) => item.id === String(row.id));
        return { kind: 'sync', requestId: nextRequestId(), data: player ?? null };
      }
      case 'create': {
        const id = String(Math.max(...players.map((p) => Number(p.id)), 10000) + 1);
        players = [
          {
            id,
            nickname: String(form.nickname ?? `玩家${id}`),
            level: Number(form.level) || 1,
            vip: Number(form.vip) || 0,
            status: 'active',
            balance: 0,
            lastLoginAt: new Date().toISOString(),
            createdAt: new Date().toISOString(),
          },
          ...players,
        ];
        return { kind: 'sync', requestId: nextRequestId(), data: { id } };
      }
      case 'update': {
        players = players.map((player) =>
          player.id === String(row.id)
            ? {
                ...player,
                nickname: String(form.nickname ?? player.nickname),
                level: Number(form.level ?? player.level),
                vip: Number(form.vip ?? player.vip),
              }
            : player,
        );
        return { kind: 'sync', requestId: nextRequestId(), data: { success: true } };
      }
      case 'delete': {
        players = players.filter((player) => player.id !== String(row.id));
        return { kind: 'sync', requestId: nextRequestId(), data: { success: true } };
      }
      case 'action.ban': {
        players = players.map((player) =>
          player.id === String(row.id) ? { ...player, status: 'banned' } : player,
        );
        return { kind: 'sync', requestId: nextRequestId(), data: { success: true } };
      }
      case 'action.recharge': {
        const amount = Number(form.amount) || 0;
        players = players.map((player) =>
          player.id === String(row.id) ? { ...player, balance: player.balance + amount } : player,
        );
        return { kind: 'sync', requestId: nextRequestId(), data: { success: true, amount } };
      }
      case 'action.mail':
        return { kind: 'sync', requestId: nextRequestId(), data: { success: true } };
      case 'action.batchBan': {
        players = players.map((player) =>
          selection.includes(player.id) ? { ...player, status: 'banned' } : player,
        );
        return {
          kind: 'sync',
          requestId: nextRequestId(),
          data: { success: true, count: selection.length },
        };
      }
      case 'action.batchUnban': {
        players = players.map((player) =>
          selection.includes(player.id) ? { ...player, status: 'active' } : player,
        );
        return {
          kind: 'sync',
          requestId: nextRequestId(),
          data: { success: true, count: selection.length },
        };
      }
      default:
        return { kind: 'sync', requestId: nextRequestId(), data: { success: true } };
    }
  };
}

// ---------------------------------------------------------------------------
// 模板页面组件
// ---------------------------------------------------------------------------

/**
 * 玩家管理 CRUD 模板页面：直接可渲染的完整示例，
 * 列表 + 详情 + 封禁 + 充值 + 邮件 + 批量操作。
 */
const PlayerManageTemplate: React.FC = () => {
  const execute = useMemo(() => createPlayerManageDemoExecute(), []);
  return (
    <PageContainer title="玩家管理" subTitle="CRUD 模板 · 列表 / 详情 / 封禁 / 充值 / 邮件">
      <PageRenderer pageSpec={playerManagePageSpec} onExecute={execute} />
    </PageContainer>
  );
};

export default PlayerManageTemplate;
