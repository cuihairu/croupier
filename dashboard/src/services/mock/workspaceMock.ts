/**
 * Mock 数据服务
 *
 * 提供 Workspace 配置和函数调用的 mock 数据
 */

import type { WorkspaceConfig } from '@/types/workspace';
import playerWorkspaceConfig from '@/config/workspaces/player';
import orderWorkspaceConfig from '@/config/workspaces/order';
import itemWorkspaceConfig from '@/config/workspaces/item';

/**
 * Mock Workspace 配置数据
 */
const mockWorkspaceConfigs: Record<string, WorkspaceConfig> = {
  player: playerWorkspaceConfig,
  order: orderWorkspaceConfig,
  item: itemWorkspaceConfig,
};

/**
 * Mock 玩家数据
 */
const mockPlayers = [
  {
    playerId: 'P10001',
    nickname: '张三',
    level: 50,
    exp: 125000,
    gold: 1000000,
    diamond: 5000,
    vipLevel: 5,
    vipExpireAt: '2026-12-31T23:59:59Z',
    status: '1',
    createdAt: '2026-01-01T00:00:00Z',
    lastLoginAt: '2026-03-06T08:00:00Z',
  },
  {
    playerId: 'P10002',
    nickname: '李四',
    level: 45,
    exp: 98000,
    gold: 800000,
    diamond: 3000,
    vipLevel: 3,
    vipExpireAt: '2026-06-30T23:59:59Z',
    status: '1',
    createdAt: '2026-01-15T00:00:00Z',
    lastLoginAt: '2026-03-05T20:00:00Z',
  },
  {
    playerId: 'P10003',
    nickname: '王五',
    level: 60,
    exp: 180000,
    gold: 1500000,
    diamond: 8000,
    vipLevel: 8,
    vipExpireAt: '2027-03-31T23:59:59Z',
    status: '1',
    createdAt: '2025-12-01T00:00:00Z',
    lastLoginAt: '2026-03-06T07:30:00Z',
  },
];

/**
 * Mock 订单数据
 */
const mockOrders = [
  {
    orderId: 'ORD20260306001',
    playerId: 'P10001',
    productId: 'PROD001',
    productName: '月卡',
    amount: 30.0,
    status: 'paid',
    paymentMethod: 'wechat',
    paymentChannel: '微信支付',
    transactionId: 'WX20260306001',
    paidAt: '2026-03-06T08:00:00Z',
    createdAt: '2026-03-06T07:55:00Z',
    updatedAt: '2026-03-06T08:00:00Z',
  },
  {
    orderId: 'ORD20260306002',
    playerId: 'P10002',
    productId: 'PROD002',
    productName: '钻石礼包',
    amount: 98.0,
    status: 'paid',
    paymentMethod: 'alipay',
    paymentChannel: '支付宝',
    transactionId: 'ALI20260306001',
    paidAt: '2026-03-06T09:00:00Z',
    createdAt: '2026-03-06T08:55:00Z',
    updatedAt: '2026-03-06T09:00:00Z',
  },
  {
    orderId: 'ORD20260306003',
    playerId: 'P10003',
    productId: 'PROD003',
    productName: '超值礼包',
    amount: 198.0,
    status: 'pending',
    paymentMethod: '',
    paymentChannel: '',
    transactionId: '',
    paidAt: null,
    createdAt: '2026-03-06T09:30:00Z',
    updatedAt: '2026-03-06T09:30:00Z',
  },
];

/**
 * Mock 道具数据
 */
const mockItems = [
  {
    itemId: 'ITEM001',
    itemName: '金币',
    itemType: 'currency',
    quality: '普通',
    description: '游戏通用货币',
    stackable: '是',
  },
  {
    itemId: 'ITEM002',
    itemName: '钻石',
    itemType: 'currency',
    quality: '稀有',
    description: '高级货币',
    stackable: '是',
  },
  {
    itemId: 'ITEM003',
    itemName: '经验药水',
    itemType: 'consumable',
    quality: '普通',
    description: '使用后获得1000经验',
    stackable: '是',
  },
  {
    itemId: 'ITEM004',
    itemName: '传说武器',
    itemType: 'equipment',
    quality: '传说',
    description: '攻击力+500',
    stackable: '否',
  },
];

/**
 * Mock 函数调用处理
 */
export const mockFunctionInvoke = async (functionId: string, params: any): Promise<any> => {
  // 模拟网络延迟
  await new Promise((resolve) => setTimeout(resolve, 300));

  // 根据函数 ID 返回不同的数据
  switch (functionId) {
    // 玩家相关
    case 'player.getInfo':
      return mockPlayers.find((p) => p.playerId === params.playerId) || null;

    case 'player.list':
      return {
        data: mockPlayers,
        total: mockPlayers.length,
        page: params.page || 1,
        pageSize: params.pageSize || 10,
      };

    case 'player.updateLevel':
      return { success: true, message: '等级更新成功' };

    case 'player.ban':
      return { success: true, message: '封禁成功' };

    case 'player.unban':
      return { success: true, message: '解封成功' };

    case 'player.export':
      return { success: true, message: '导出成功', url: '/download/players.xlsx' };

    // 订单相关
    case 'order.getInfo':
      return mockOrders.find((o) => o.orderId === params.orderId) || null;

    case 'order.list':
      return {
        data: mockOrders,
        total: mockOrders.length,
        page: params.page || 1,
        pageSize: params.pageSize || 10,
      };

    case 'order.refund':
      return { success: true, message: '退款成功' };

    case 'order.export':
      return { success: true, message: '导出成功', url: '/download/orders.xlsx' };

    // 道具相关
    case 'item.list':
      return {
        data: mockItems,
        total: mockItems.length,
        page: params.page || 1,
        pageSize: params.pageSize || 10,
      };

    case 'item.grant':
      return {
        success: true,
        message: `成功发放 ${params.count} 个道具 ${params.itemId} 给玩家 ${params.playerId}`,
      };

    case 'item.recycle':
      return {
        success: true,
        message: `成功回收玩家 ${params.playerId} 的 ${params.count} 个道具 ${params.itemId}`,
      };

    default:
      throw new Error(`未知的函数: ${functionId}`);
  }
};

/**
 * Mock Workspace 配置加载
 */
export const mockLoadWorkspaceConfig = async (
  objectKey: string,
): Promise<WorkspaceConfig | null> => {
  // 模拟网络延迟
  await new Promise((resolve) => setTimeout(resolve, 200));

  return mockWorkspaceConfigs[objectKey] || null;
};

/**
 * Mock Workspace 配置保存
 */
export const mockSaveWorkspaceConfig = async (
  config: WorkspaceConfig,
): Promise<WorkspaceConfig> => {
  // 模拟网络延迟
  await new Promise((resolve) => setTimeout(resolve, 200));

  // 更新缓存
  mockWorkspaceConfigs[config.objectKey] = {
    ...config,
    meta: {
      ...config.meta,
      updatedAt: new Date().toISOString(),
    },
  };

  return mockWorkspaceConfigs[config.objectKey];
};

/**
 * Mock Workspace 配置列表
 */
export const mockListWorkspaceConfigs = async (): Promise<WorkspaceConfig[]> => {
  // 模拟网络延迟
  await new Promise((resolve) => setTimeout(resolve, 200));

  return Object.values(mockWorkspaceConfigs);
};

/**
 * Mock Workspace 配置删除
 */
export const mockDeleteWorkspaceConfig = async (objectKey: string): Promise<void> => {
  // 模拟网络延迟
  await new Promise((resolve) => setTimeout(resolve, 200));

  delete mockWorkspaceConfigs[objectKey];
};

/**
 * Mock Workspace 发布
 */
export const mockPublishWorkspaceConfig = async (objectKey: string): Promise<WorkspaceConfig> => {
  await new Promise((resolve) => setTimeout(resolve, 200));

  const config = mockWorkspaceConfigs[objectKey];
  if (!config) throw new Error(`配置不存在: ${objectKey}`);

  mockWorkspaceConfigs[objectKey] = {
    ...config,
    published: true,
    publishedAt: new Date().toISOString(),
    publishedBy: 'admin',
  };
  return mockWorkspaceConfigs[objectKey];
};

/**
 * Mock Workspace 取消发布
 */
export const mockUnpublishWorkspaceConfig = async (objectKey: string): Promise<WorkspaceConfig> => {
  await new Promise((resolve) => setTimeout(resolve, 200));

  const config = mockWorkspaceConfigs[objectKey];
  if (!config) throw new Error(`配置不存在: ${objectKey}`);

  mockWorkspaceConfigs[objectKey] = {
    ...config,
    published: false,
    publishedAt: undefined,
    publishedBy: undefined,
  };
  return mockWorkspaceConfigs[objectKey];
};

/**
 * Mock 已发布 Workspace 列表
 */
export const mockListPublishedWorkspaceConfigs = async (): Promise<WorkspaceConfig[]> => {
  await new Promise((resolve) => setTimeout(resolve, 200));

  return Object.values(mockWorkspaceConfigs)
    .filter((c) => c.published)
    .sort((a, b) => (a.menuOrder ?? 99) - (b.menuOrder ?? 99));
};
