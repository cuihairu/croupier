export type PreviewKanbanColumn = {
  id: string;
  title: string;
  color?: string;
  cards: Array<{
    id: string;
    title: string;
    description?: string;
    priority?: 'high' | 'medium' | 'low';
    tags?: string[];
    assignee?: { id: string; name: string };
    dueDate?: string;
  }>;
};

export type PreviewTimelineEvent = {
  id: string;
  type: 'created' | 'updated' | 'deleted' | 'approved' | 'rejected' | 'custom';
  title: string;
  description?: string;
  timestamp: string;
  actor?: { id: string; name: string };
  status?: 'success' | 'pending' | 'error' | 'warning';
  tags?: string[];
};

function ago(hours: number): string {
  return new Date(Date.now() - hours * 3600 * 1000).toISOString();
}

export function buildGamePreviewKanbanColumns(): PreviewKanbanColumn[] {
  return [
    {
      id: 'todo',
      title: '待处理',
      color: '#1677ff',
      cards: [
        {
          id: 'todo-1',
          title: 'S12 服务器补偿审核',
          description: '确认补偿道具与发放范围',
          priority: 'high',
          tags: ['补偿', 'S12'],
          assignee: { id: 'gm_01', name: 'GM-林' },
          dueDate: ago(4),
        },
      ],
    },
    {
      id: 'in-progress',
      title: '进行中',
      color: '#faad14',
      cards: [
        {
          id: 'doing-1',
          title: '春节活动奖池配置',
          description: '核对 SSR 权重与保底逻辑',
          priority: 'medium',
          tags: ['活动', '奖池'],
          assignee: { id: 'op_11', name: '运营-周' },
          dueDate: ago(2),
        },
      ],
    },
    {
      id: 'done',
      title: '已完成',
      color: '#52c41a',
      cards: [
        {
          id: 'done-1',
          title: '玩家标签批量重算',
          description: '已执行并校验样本',
          priority: 'low',
          tags: ['标签', '批处理'],
          assignee: { id: 'dev_07', name: '后端-陈' },
          dueDate: ago(1),
        },
      ],
    },
  ];
}

export function buildGamePreviewTimelineEvents(): PreviewTimelineEvent[] {
  return [
    {
      id: 'evt-1',
      type: 'created',
      title: '创建限时活动配置',
      description: '活动 ID: act_2026_spring',
      timestamp: ago(6),
      actor: { id: 'op_11', name: '运营-周' },
      status: 'success',
      tags: ['活动'],
    },
    {
      id: 'evt-2',
      type: 'updated',
      title: '调整奖池权重',
      description: 'SSR 权重由 1.2% 调整至 1.5%',
      timestamp: ago(3),
      actor: { id: 'op_11', name: '运营-周' },
      status: 'warning',
      tags: ['奖池', '配置'],
    },
    {
      id: 'evt-3',
      type: 'approved',
      title: '活动审批通过',
      description: '发布窗口: 20:00 - 23:59',
      timestamp: ago(1),
      actor: { id: 'admin_01', name: '管理员-王' },
      status: 'success',
      tags: ['审批'],
    },
  ];
}

export function buildGamePreviewDashboardStats(): Array<{
  key: string;
  title: string;
  value: number;
  suffix?: string;
}> {
  return [
    { key: 'online', title: '在线人数', value: 1280 },
    { key: 'dau', title: 'DAU', value: 56000 },
    { key: 'payRate', title: '付费率', value: 12.8, suffix: '%' },
    { key: 'arppu', title: 'ARPPU', value: 186 },
  ];
}

export function buildPreviewListConfig(listFunction = ''): Record<string, any> {
  return {
    listFunction,
    columns: [
      { key: 'playerId', title: '玩家ID' },
      { key: 'nickname', title: '昵称' },
      { key: 'level', title: '等级' },
      { key: 'status', title: '状态' },
      { key: 'updatedAt', title: '更新时间', render: 'datetime' },
    ],
  };
}

export function buildPreviewDetailConfig(detailFunction = ''): Record<string, any> {
  return {
    detailFunction,
    sections: [
      {
        title: '基础信息',
        column: 2,
        fields: [
          { key: 'playerId', label: '玩家ID' },
          { key: 'nickname', label: '昵称' },
          { key: 'level', label: '等级' },
          { key: 'vip', label: 'VIP' },
          { key: 'status', label: '状态' },
          { key: 'updatedAt', label: '更新时间' },
        ],
      },
    ],
  };
}

export function buildPreviewFormConfig(submitFunction = ''): Record<string, any> {
  return {
    submitFunction,
    fields: [
      { key: 'nickname', label: '昵称', type: 'input', required: true },
      { key: 'level', label: '等级', type: 'number', required: true },
      {
        key: 'status',
        label: '状态',
        type: 'select',
        options: [
          { label: '启用', value: 'active' },
          { label: '禁用', value: 'disabled' },
        ],
      },
      { key: 'remark', label: '备注', type: 'textarea' },
    ],
  };
}
