/**
 * Dashboard Mock API
 *
 * 覆盖 Console、Resource Catalog、Proposal、Contract、Versioning 等 API 端点。
 */

import { Request, Response } from 'express';

// ---------------------------------------------------------------------------
// Mock 数据
// ---------------------------------------------------------------------------

const mockPlayers = [
  { id: '1001', name: '玩家A', level: 10, status: 'active', createdAt: '2024-01-01T00:00:00Z' },
  { id: '1002', name: '玩家B', level: 20, status: 'active', createdAt: '2024-01-02T00:00:00Z' },
  { id: '1003', name: '玩家C', level: 15, status: 'banned', createdAt: '2024-01-03T00:00:00Z' },
];

const mockInventory = [
  { itemId: 'item-001', name: '金币', quantity: 1000, type: 'currency' },
  { itemId: 'item-002', name: '钻石', quantity: 50, type: 'currency' },
  { itemId: 'item-003', name: '铁剑', quantity: 1, type: 'equipment' },
];

// ---------------------------------------------------------------------------
// Helper: build mock page specs
// ---------------------------------------------------------------------------

function buildResourcePage(resourceKey: string) {
  return {
    pageKey: `resource--${resourceKey}`,
    type: 'resource',
    resourceKey,
    title: { 'zh-CN': resourceKey === 'players' ? '玩家列表' : '背包物品' },
    category: { key: resourceKey, labels: { 'zh-CN': resourceKey } },
    resource: {
      listView: {
        columns: [
          { key: 'id', title: { 'zh-CN': 'ID' }, dataType: 'string', visible: true },
          { key: 'name', title: { 'zh-CN': '名称' }, dataType: 'string', visible: true },
          { key: 'level', title: { 'zh-CN': '等级' }, dataType: 'number', visible: true },
          { key: 'status', title: { 'zh-CN': '状态' }, dataType: 'string', visible: true },
        ],
        pagination: { enabled: true, defaultSize: 20 },
        identityKey: 'id',
      },
      actions: [
        { key: 'edit', title: { 'zh-CN': '编辑' }, type: 'link', bindingId: 'update' },
        {
          key: 'delete',
          title: { 'zh-CN': '删除' },
          type: 'danger',
          confirm: true,
          bindingId: 'delete',
        },
      ],
      createForm: {
        jsonSchema: {
          type: 'object',
          properties: {
            name: { type: 'string', title: '名称' },
            level: { type: 'number', title: '等级' },
          },
          required: ['name'],
        },
      },
      deleteAction: {
        title: { 'zh-CN': '删除' },
        description: { 'zh-CN': '确认删除此记录？' },
        confirmText: { 'zh-CN': '确认删除' },
        bindingId: 'delete',
        risk: 'high',
      },
    },
    bindings: [
      {
        id: 'list',
        functionId: `${resourceKey}.list`,
        usage: 'query',
        execution: { mode: 'sync' },
      },
      {
        id: 'create',
        functionId: `${resourceKey}.create`,
        usage: 'action',
        execution: { mode: 'sync' },
      },
      {
        id: 'update',
        functionId: `${resourceKey}.update`,
        usage: 'action',
        execution: { mode: 'sync' },
      },
      {
        id: 'delete',
        functionId: `${resourceKey}.delete`,
        usage: 'action',
        execution: { mode: 'sync' },
      },
    ],
  };
}

function buildOperationPage(functionId: string) {
  return {
    pageKey: `operation--${functionId}`,
    type: 'operation',
    title: { 'zh-CN': '发送邮件' },
    category: { key: 'mail', labels: { 'zh-CN': '邮件系统' } },
    operation: {
      form: {
        jsonSchema: {
          type: 'object',
          properties: {
            to: { type: 'string', title: '收件人' },
            subject: { type: 'string', title: '主题' },
            content: { type: 'string', title: '内容' },
          },
          required: ['to', 'content'],
        },
      },
      confirm: {
        title: { 'zh-CN': '确认发送' },
        description: { 'zh-CN': '确认发送此邮件？' },
        confirmText: { 'zh-CN': '确认' },
        bindingId: 'main',
      },
      resultView: {
        successMessage: { 'zh-CN': '邮件发送成功' },
        errorMessage: { 'zh-CN': '邮件发送失败' },
      },
    },
    bindings: [{ id: 'main', functionId, usage: 'action', execution: { mode: 'sync' } }],
  };
}

function buildTaskPage(functionId: string) {
  return {
    pageKey: `task--${functionId}`,
    type: 'task',
    title: { 'zh-CN': '批量发奖' },
    category: { key: 'reward', labels: { 'zh-CN': '奖励系统' } },
    task: {
      form: {
        jsonSchema: {
          type: 'object',
          properties: {
            playerIds: { type: 'string', title: '玩家ID列表' },
            rewardId: { type: 'string', title: '奖励ID' },
          },
          required: ['playerIds', 'rewardId'],
        },
      },
      taskView: {
        showTimeline: true,
        showProgress: true,
        showEvents: true,
        cancelable: true,
        retryable: true,
      },
      resultView: {
        successMessage: { 'zh-CN': '批量发奖完成' },
        errorMessage: { 'zh-CN': '批量发奖失败' },
      },
    },
    bindings: [{ id: 'start', functionId, usage: 'task', execution: { mode: 'task' } }],
  };
}

function buildReportPage(functionId: string) {
  return {
    pageKey: `report--${functionId}`,
    type: 'report',
    title: { 'zh-CN': '留存分析' },
    category: { key: 'analytics', labels: { 'zh-CN': '数据分析' } },
    report: {
      queryForm: {
        jsonSchema: {
          type: 'object',
          properties: {
            startDate: { type: 'string', title: '开始日期', format: 'date' },
            endDate: { type: 'string', title: '结束日期', format: 'date' },
          },
        },
      },
      dataset: {
        dimensions: [{ key: 'date', title: { 'zh-CN': '日期' }, dataType: 'date' }],
        metrics: [
          { key: 'retention', title: { 'zh-CN': '留存率' }, dataType: 'number', format: 'percent' },
        ],
      },
      charts: [
        { type: 'line', title: { 'zh-CN': '留存趋势' }, xField: 'date', yField: 'retention' },
      ],
      exportable: true,
    },
    bindings: [{ id: 'query', functionId, usage: 'report', execution: { mode: 'sync' } }],
  };
}

function findPage(pageKey: string) {
  const pages = [
    buildResourcePage('players'),
    buildResourcePage('inventory'),
    buildOperationPage('mail.send'),
    buildOperationPage('system.dangerous-op'),
    buildTaskPage('reward.batchGrant'),
    buildReportPage('analytics.retention'),
  ];
  return pages.find((p) => p.pageKey === pageKey);
}

// ---------------------------------------------------------------------------
// Umi Mock 导出格式
// ---------------------------------------------------------------------------

export default {
  // Console API
  'GET /api/v1/console/menu': (req: Request, res: Response) => {
    res.send({
      categories: [
        {
          key: 'players',
          labels: { 'zh-CN': '玩家管理', en: 'Players' },
          order: 1,
          pages: [{ pageKey: 'resource--players', title: { 'zh-CN': '玩家列表', en: 'Players' } }],
        },
        {
          key: 'inventory',
          labels: { 'zh-CN': '背包管理', en: 'Inventory' },
          order: 2,
          pages: [
            { pageKey: 'resource--inventory', title: { 'zh-CN': '背包物品', en: 'Inventory' } },
          ],
        },
        {
          key: 'mail',
          labels: { 'zh-CN': '邮件系统', en: 'Mail' },
          order: 3,
          pages: [
            { pageKey: 'operation--mail.send', title: { 'zh-CN': '发送邮件', en: 'Send Mail' } },
          ],
        },
        {
          key: 'reward',
          labels: { 'zh-CN': '奖励系统', en: 'Rewards' },
          order: 4,
          pages: [
            {
              pageKey: 'task--reward.batchGrant',
              title: { 'zh-CN': '批量发奖', en: 'Batch Grant' },
            },
          ],
        },
        {
          key: 'analytics',
          labels: { 'zh-CN': '数据分析', en: 'Analytics' },
          order: 5,
          pages: [
            {
              pageKey: 'report--analytics.retention',
              title: { 'zh-CN': '留存分析', en: 'Retention' },
            },
          ],
        },
        {
          key: 'system',
          labels: { 'zh-CN': '系统', en: 'System' },
          order: 6,
          pages: [
            {
              pageKey: 'operation--system.dangerous-op',
              title: { 'zh-CN': '危险操作', en: 'Dangerous Op' },
            },
          ],
        },
      ],
    });
  },

  'GET /api/v1/console/pages': (req: Request, res: Response) => {
    res.send({
      items: [
        buildResourcePage('players'),
        buildResourcePage('inventory'),
        buildOperationPage('mail.send'),
        buildTaskPage('reward.batchGrant'),
        buildReportPage('analytics.retention'),
      ],
    });
  },

  'GET /api/v1/console/pages/:pageKey': (req: Request, res: Response) => {
    const pageKey = req.params.pageKey as string;
    const page = findPage(pageKey);
    if (!page) {
      res.status(404).send({ error: 'not_found', message: `page not found: ${pageKey}` });
      return;
    }
    res.send({ page });
  },

  'POST /api/v1/console/pages/:pageKey/bindings/:bindingId/execute': (
    req: Request,
    res: Response,
  ) => {
    const { pageKey, bindingId } = req.params;
    const { context } = req.body;

    if (pageKey.includes('players')) {
      if (bindingId === 'list') {
        res.send({
          result: {
            kind: 'sync',
            requestId: `req-${Date.now()}`,
            data: { items: mockPlayers, total: mockPlayers.length },
          },
        });
      } else if (bindingId === 'create') {
        res.send({
          result: {
            kind: 'sync',
            requestId: `req-${Date.now()}`,
            data: { id: '1004', ...context?.form, status: 'active' },
          },
        });
      } else {
        res.send({
          result: {
            kind: 'sync',
            requestId: `req-${Date.now()}`,
            data: { success: true },
          },
        });
      }
    } else if (pageKey.includes('mail')) {
      res.send({
        result: {
          kind: 'sync',
          requestId: `req-${Date.now()}`,
          data: { success: true, messageId: `msg-${Date.now()}` },
        },
      });
    } else if (pageKey.includes('reward')) {
      res.send({
        result: {
          kind: 'task',
          requestId: `req-${Date.now()}`,
          taskId: `task-${Date.now()}`,
        },
      });
    } else if (pageKey.includes('analytics')) {
      res.send({
        result: {
          kind: 'sync',
          requestId: `req-${Date.now()}`,
          data: {
            dimensions: ['2024-01-01', '2024-01-02', '2024-01-03'],
            metrics: { retention: [0.8, 0.6, 0.4] },
          },
        },
      });
    } else {
      res.send({
        result: {
          kind: 'sync',
          requestId: `req-${Date.now()}`,
          data: { success: true },
        },
      });
    }
  },

  // Task API
  'GET /api/v1/tasks/:taskId': (req: Request, res: Response) => {
    res.send({
      id: req.params.taskId,
      status: 'running',
      progress: 60,
      message: '正在处理...',
    });
  },

  'GET /api/v1/tasks/:taskId/events': (req: Request, res: Response) => {
    res.send({
      items: [
        { type: 'progress', progress: 20, message: '开始处理', createdAt: '2024-01-01T00:00:01Z' },
        { type: 'progress', progress: 60, message: '处理中...', createdAt: '2024-01-01T00:00:02Z' },
      ],
    });
  },

  'POST /api/v1/tasks/:taskId/cancel': (req: Request, res: Response) => {
    res.send({ success: true });
  },

  // Approval API
  'GET /api/v1/approvals/:approvalId': (req: Request, res: Response) => {
    res.send({
      id: req.params.approvalId,
      state: 'pending',
      functionId: 'system.dangerous-op',
      actor: 'admin',
      updatedAt: new Date().toISOString(),
    });
  },

  // Resource Catalog API
  'GET /api/v1/resource-catalog': (req: Request, res: Response) => {
    res.send({
      items: [
        {
          resourceKey: 'players',
          status: 'identified',
          functions: [
            { functionId: 'players.list', capability: 'collection_query' },
            { functionId: 'players.get', capability: 'item_query' },
            { functionId: 'players.create', capability: 'create' },
            { functionId: 'players.update', capability: 'update' },
            { functionId: 'players.delete', capability: 'delete' },
            { functionId: 'players.ban', capability: 'action' },
          ],
          semantics: {
            identityField: 'id',
            collectionQueryId: 1,
          },
        },
        {
          resourceKey: 'inventory',
          status: 'identified',
          functions: [
            { functionId: 'inventory.list', capability: 'collection_query' },
            { functionId: 'inventory.get', capability: 'item_query' },
          ],
          semantics: {
            identityField: 'itemId',
            collectionQueryId: 1,
          },
        },
      ],
      total: 2,
    });
  },

  'GET /api/v1/resource-catalog/:resourceKey': (req: Request, res: Response) => {
    const { resourceKey } = req.params;
    res.send({
      resourceKey,
      status: 'identified',
      functions: [
        { functionId: `${resourceKey}.list`, capability: 'collection_query' },
        { functionId: `${resourceKey}.get`, capability: 'item_query' },
      ],
      semantics: {
        identityField: 'id',
        collectionQueryId: 1,
      },
    });
  },

  'GET /api/v1/resource-catalog/:resourceKey/conflicts': (req: Request, res: Response) => {
    res.send({
      conflicts: [],
      provenance: [],
    });
  },

  'GET /api/v1/resource-catalog/:resourceKey/semantics/versions': (req: Request, res: Response) => {
    res.send({
      versions: [{ version: 1, source: 'sdk_explicit', createdAt: '2024-01-01T00:00:00Z' }],
    });
  },

  // Contract API
  'GET /api/v1/contracts': (req: Request, res: Response) => {
    res.send({
      items: [
        { functionId: 'players.list', capability: 'collection_query', execution: 'sync' },
        { functionId: 'players.get', capability: 'item_query', execution: 'sync' },
        { functionId: 'players.create', capability: 'create', execution: 'sync' },
        { functionId: 'mail.send', capability: 'action', execution: 'sync' },
        { functionId: 'reward.batchGrant', capability: 'task', execution: 'task' },
        { functionId: 'analytics.retention', capability: 'report', execution: 'sync' },
      ],
    });
  },

  'GET /api/v1/contracts/:functionId': (req: Request, res: Response) => {
    res.send({
      functionId: req.params.functionId,
      capability: 'action',
      execution: 'sync',
      inputSchema: { type: 'object', properties: {} },
    });
  },

  // Proposal API
  'GET /api/v1/proposals': (req: Request, res: Response) => {
    res.send({
      items: [
        {
          proposalKey: 'resource:players',
          pageKey: 'resource--players',
          quality: 'ready',
          status: 'pending',
        },
        {
          proposalKey: 'resource:inventory',
          pageKey: 'resource--inventory',
          quality: 'ready',
          status: 'pending',
        },
        {
          proposalKey: 'operation:mail.send',
          pageKey: 'operation--mail.send',
          quality: 'basic',
          status: 'pending',
        },
        {
          proposalKey: 'task:reward.batchGrant',
          pageKey: 'task--reward.batchGrant',
          quality: 'needs_review',
          status: 'pending',
        },
        {
          proposalKey: 'report:analytics.retention',
          pageKey: 'report--analytics.retention',
          quality: 'needs_review',
          status: 'pending',
        },
      ],
    });
  },

  'GET /api/v1/proposals/inbox': (req: Request, res: Response) => {
    res.send({
      ready: [
        { proposalKey: 'resource:players', pageKey: 'resource--players', quality: 'ready' },
        { proposalKey: 'resource:inventory', pageKey: 'resource--inventory', quality: 'ready' },
      ],
      needsReview: [
        {
          proposalKey: 'task:reward.batchGrant',
          pageKey: 'task--reward.batchGrant',
          quality: 'needs_review',
        },
        {
          proposalKey: 'report:analytics.retention',
          pageKey: 'report--analytics.retention',
          quality: 'needs_review',
        },
      ],
      stale: [],
    });
  },

  'POST /api/v1/proposals/:proposalKey/accept': (req: Request, res: Response) => {
    res.send({ message: 'proposal accepted and published' });
  },

  // Versioning API
  'GET /api/v1/versioning/:resourceKey/chain': (req: Request, res: Response) => {
    res.send({
      resourceKey: req.params.resourceKey,
      items: [
        { type: 'function_update', timestamp: '2024-01-01T00:00:00Z', summary: 'function updated' },
      ],
      current: { functionVersion: '1.0.0', semanticVersion: 1 },
    });
  },

  'POST /api/v1/versioning/:resourceKey/merge': (req: Request, res: Response) => {
    res.send({ merged: 1, conflicts: 0, message: 'auto-merged 1 safe change' });
  },

  'POST /api/v1/versioning/:resourceKey/regenerate': (req: Request, res: Response) => {
    res.send({ message: 'proposal regenerated' });
  },

  'POST /api/v1/versioning/:resourceKey/republish': (req: Request, res: Response) => {
    res.send({ version: 2, message: 'republished successfully' });
  },

  // Messages API (prevent 404)
  'GET /api/v1/messages/unread-count': (req: Request, res: Response) => {
    res.send({ count: 0 });
  },

  // Pages API
  'GET /api/v1/pages': (req: Request, res: Response) => {
    res.send({
      items: [
        { pageKey: 'resource--players', type: 'resource', title: { 'zh-CN': '玩家列表' } },
        { pageKey: 'resource--inventory', type: 'resource', title: { 'zh-CN': '背包物品' } },
      ],
    });
  },

  // Page Specs API (for Page Studio)
  'GET /api/v1/page-specs': (req: Request, res: Response) => {
    res.send({
      items: [
        {
          pageKey: 'resource--players',
          type: 'resource',
          title: { 'zh-CN': '玩家列表' },
          category: { key: 'players', labels: { 'zh-CN': '玩家管理' } },
          status: 'published',
          version: 1,
        },
        {
          pageKey: 'resource--inventory',
          type: 'resource',
          title: { 'zh-CN': '背包物品' },
          category: { key: 'inventory', labels: { 'zh-CN': '背包管理' } },
          status: 'draft',
          version: 1,
        },
        {
          pageKey: 'operation--mail.send',
          type: 'operation',
          title: { 'zh-CN': '发送邮件' },
          category: { key: 'mail', labels: { 'zh-CN': '邮件系统' } },
          status: 'draft',
          version: 1,
        },
      ],
    });
  },

  'GET /api/v1/page-specs/:pageKey': (req: Request, res: Response) => {
    const pageKey = req.params.pageKey as string;
    const page = findPage(pageKey);
    if (!page) {
      res.status(404).send({ error: 'not_found', message: `page not found: ${pageKey}` });
      return;
    }
    res.send({
      ...page,
      status: 'draft',
      version: 1,
      draftRevision: 1,
    });
  },

  // Function API mocks
  'GET /api/v1/functions/descriptors': (req: Request, res: Response) => {
    res.send([
      {
        id: 'player.list',
        name: 'player.list',
        displayName: { 'zh-CN': '查询玩家列表', en: 'List Players' },
        summary: {
          'zh-CN': '查询玩家列表，支持分页和筛选',
          en: 'Query player list with pagination',
        },
        description: 'Query player list with pagination',
        resource: 'player',
        operation: 'list',
        capability: 'collection_query',
        execution: 'sync',
        inputSchema: JSON.stringify({
          type: 'object',
          properties: {
            page: { type: 'integer', title: '页码', default: 1 },
            pageSize: { type: 'integer', title: '每页数量', default: 20 },
            keyword: { type: 'string', title: '搜索关键词' },
          },
        }),
        tags: ['player', 'query'],
      },
      {
        id: 'player.create',
        name: 'player.create',
        displayName: { 'zh-CN': '创建玩家', en: 'Create Player' },
        summary: { 'zh-CN': '创建新玩家账号', en: 'Create a new player account' },
        resource: 'player',
        operation: 'create',
        capability: 'create',
        execution: 'sync',
        inputSchema: JSON.stringify({
          type: 'object',
          properties: {
            name: { type: 'string', title: '玩家名称' },
            level: { type: 'integer', title: '初始等级', default: 1 },
          },
          required: ['name'],
        }),
        tags: ['player', 'create'],
      },
      {
        id: 'mail.send',
        name: 'mail.send',
        displayName: { 'zh-CN': '发送邮件', en: 'Send Mail' },
        summary: { 'zh-CN': '向玩家发送邮件', en: 'Send mail to player' },
        resource: 'mail',
        operation: 'send',
        capability: 'action',
        execution: 'sync',
        inputSchema: JSON.stringify({
          type: 'object',
          properties: {
            to: { type: 'string', title: '收件人' },
            subject: { type: 'string', title: '主题' },
            content: { type: 'string', title: '内容' },
          },
          required: ['to', 'subject', 'content'],
        }),
        tags: ['mail', 'send'],
      },
    ]);
  },

  'POST /api/v1/functions/:id/invoke': (req: Request, res: Response) => {
    const { id } = req.params;
    res.send({
      success: true,
      data: {
        message: `Function ${id} executed successfully`,
        timestamp: new Date().toISOString(),
      },
    });
  },
};
