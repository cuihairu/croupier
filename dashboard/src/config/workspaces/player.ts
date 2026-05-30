/**
 * 示例 Workspace 配置 - 玩家管理
 */

import type { WorkspaceConfig } from '@/types/workspace';

export const playerWorkspaceConfig: WorkspaceConfig = {
  objectKey: 'player',
  title: '玩家管理',
  description: '玩家信息查询、管理和操作',
  icon: 'UserOutlined',
  layout: {
    type: 'tabs',
    tabs: [
      {
        key: 'info',
        title: '玩家信息',
        icon: 'InfoCircleOutlined',
        functions: ['player.getInfo', 'player.updateLevel', 'player.ban', 'player.unban'],
        layout: {
          type: 'form-detail',
          queryFunction: 'player.getInfo',
          queryFields: [
            {
              key: 'playerId',
              label: '玩家ID',
              type: 'input',
              required: true,
              placeholder: '请输入玩家ID',
            },
          ],
          detailSections: [
            {
              title: '基本信息',
              fields: [
                { key: 'playerId', label: '玩家ID' },
                { key: 'nickname', label: '昵称' },
                { key: 'level', label: '等级' },
                { key: 'exp', label: '经验值' },
                { key: 'gold', label: '金币', render: 'money' },
                { key: 'diamond', label: '钻石' },
              ],
            },
            {
              title: 'VIP 信息',
              fields: [
                { key: 'vipLevel', label: 'VIP 等级' },
                { key: 'vipExpireAt', label: 'VIP 到期时间', render: 'datetime' },
              ],
            },
            {
              title: '状态信息',
              fields: [
                {
                  key: 'status',
                  label: '账号状态',
                  render: 'status',
                  renderOptions: {
                    statusMap: {
                      '1': { text: '正常', status: 'success' },
                      '2': { text: '封禁', status: 'error' },
                      '3': { text: '冻结', status: 'warning' },
                    },
                  },
                },
                { key: 'createdAt', label: '注册时间', render: 'datetime' },
                { key: 'lastLoginAt', label: '最后登录', render: 'datetime' },
              ],
            },
          ],
          actions: [
            {
              key: 'updateLevel',
              label: '更新等级',
              function: 'player.updateLevel',
              type: 'modal',
              buttonType: 'primary',
            },
            {
              key: 'ban',
              label: '封禁',
              function: 'player.ban',
              type: 'popconfirm',
              buttonType: 'default',
              danger: true,
              permissions: ['player.ban'],
            },
            {
              key: 'unban',
              label: '解封',
              function: 'player.unban',
              type: 'popconfirm',
              buttonType: 'default',
              permissions: ['player.unban'],
            },
          ],
        },
      },
      {
        key: 'list',
        title: '玩家列表',
        icon: 'UnorderedListOutlined',
        functions: ['player.list'],
        layout: {
          type: 'list',
          listFunction: 'player.list',
          columns: [
            { key: 'playerId', title: '玩家ID', width: 150, fixed: 'left' },
            { key: 'nickname', title: '昵称', width: 150 },
            { key: 'level', title: '等级', width: 100, sortable: true },
            { key: 'gold', title: '金币', width: 120, render: 'money', sortable: true },
            { key: 'vipLevel', title: 'VIP', width: 100 },
            {
              key: 'status',
              title: '状态',
              width: 100,
              render: 'status',
              renderOptions: {
                statusMap: {
                  '1': { text: '正常', status: 'success' },
                  '2': { text: '封禁', status: 'error' },
                  '3': { text: '冻结', status: 'warning' },
                },
              },
            },
            { key: 'lastLoginAt', title: '最后登录', width: 180, render: 'datetime' },
          ],
          rowActions: [
            {
              key: 'view',
              label: '查看',
              function: 'player.getInfo',
              type: 'drawer',
            },
            {
              key: 'ban',
              label: '封禁',
              function: 'player.ban',
              permissions: ['player.ban'],
            },
          ],
          toolbarActions: [
            {
              key: 'export',
              label: '导出',
              function: 'player.export',
              permissions: ['player.export'],
            },
          ],
        },
      },
    ],
  },
  permissions: ['player.view'],
  published: true,
  publishedAt: '2026-03-06T08:00:00Z',
  publishedBy: 'admin',
  menuOrder: 1,
  meta: {
    createdAt: '2026-03-06T08:00:00Z',
    updatedAt: '2026-03-06T08:00:00Z',
    createdBy: 'admin',
    version: 1,
  },
};

export default playerWorkspaceConfig;
