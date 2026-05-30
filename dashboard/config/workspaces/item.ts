/**
 * 示例 Workspace 配置 - 道具管理
 */

import type { WorkspaceConfig } from '@/types/workspace';

export const itemWorkspaceConfig: WorkspaceConfig = {
  objectKey: 'item',
  title: '道具管理',
  description: '道具查询、发放和回收',
  icon: 'GiftOutlined',
  layout: {
    type: 'tabs',
    tabs: [
      {
        key: 'list',
        title: '道具列表',
        icon: 'UnorderedListOutlined',
        functions: ['item.list'],
        layout: {
          type: 'list',
          listFunction: 'item.list',
          columns: [
            { key: 'itemId', title: '道具ID', width: 120, fixed: 'left' },
            { key: 'itemName', title: '道具名称', width: 200 },
            { key: 'itemType', title: '类型', width: 120 },
            { key: 'quality', title: '品质', width: 100, render: 'tag' },
            { key: 'description', title: '描述', width: 300 },
            { key: 'stackable', title: '可堆叠', width: 100, render: 'tag' },
          ],
        },
      },
      {
        key: 'grant',
        title: '发放道具',
        icon: 'PlusOutlined',
        functions: ['item.grant'],
        layout: {
          type: 'form',
          submitFunction: 'item.grant',
          formLayout: 'horizontal',
          fields: [
            {
              key: 'playerId',
              label: '玩家ID',
              type: 'input',
              required: true,
              placeholder: '请输入玩家ID',
            },
            {
              key: 'itemId',
              label: '道具ID',
              type: 'input',
              required: true,
              placeholder: '请输入道具ID',
            },
            {
              key: 'count',
              label: '数量',
              type: 'number',
              required: true,
              defaultValue: 1,
              rules: [{ type: 'number', min: 1, max: 9999, message: '数量必须在1-9999之间' }],
            },
            {
              key: 'reason',
              label: '发放原因',
              type: 'textarea',
              required: true,
              placeholder: '请输入发放原因',
            },
          ],
          showReset: true,
        },
      },
      {
        key: 'recycle',
        title: '回收道具',
        icon: 'DeleteOutlined',
        functions: ['item.recycle'],
        layout: {
          type: 'form',
          submitFunction: 'item.recycle',
          formLayout: 'horizontal',
          fields: [
            {
              key: 'playerId',
              label: '玩家ID',
              type: 'input',
              required: true,
              placeholder: '请输入玩家ID',
            },
            {
              key: 'itemId',
              label: '道具ID',
              type: 'input',
              required: true,
              placeholder: '请输入道具ID',
            },
            {
              key: 'count',
              label: '数量',
              type: 'number',
              required: true,
              defaultValue: 1,
              rules: [{ type: 'number', min: 1, max: 9999, message: '数量必须在1-9999之间' }],
            },
            {
              key: 'reason',
              label: '回收原因',
              type: 'textarea',
              required: true,
              placeholder: '请输入回收原因',
            },
          ],
          showReset: true,
        },
      },
    ],
  },
  permissions: ['item.view'],
  meta: {
    createdAt: '2026-03-06T08:00:00Z',
    updatedAt: '2026-03-06T08:00:00Z',
    createdBy: 'admin',
    version: 1,
  },
};

export default itemWorkspaceConfig;
