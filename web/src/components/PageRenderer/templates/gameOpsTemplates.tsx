/**
 * gameOpsTemplates - 游戏运营业务模板注册表（界面生成链「业务模板为主入口」骨架）
 *
 * 单一目录把五个默认游戏业务分类（docs/design/default-game-menus.md 的
 * player/mail/ban/payment/operation）映射到对应业务模板页面：
 * 每个条目携带稳定的模板 key、默认菜单分类 key、双语文案、页面组件与
 * PageSpec 常量。控制台模板路由（/console/templates/<key>）与本注册表
 * 一一对应；后续模板 → 提案 → 发布 → 挂载默认菜单的链路复用同一份
 * PageSpec 常量，不再各自维护页面副本。
 *
 * 边界（与 todo L7 口径一致）：
 * - 仅面向游戏运营业务页面，不是通用低代码平台；
 * - 模板页使用内置 demo 执行器做样式与交互联调，正式页面走
 *   PageSpec 发布链与 executePageBinding；
 * - 新业务页面先在此注册，再补路由与菜单挂载，避免散落定义。
 *
 * @module components/PageRenderer/templates/gameOpsTemplates
 */

import type { ComponentType } from 'react';
import type { PageSpec } from '@/types/dashboard';
import PlayerManageTemplate, { playerManagePageSpec } from './playerManage';
import MailCompensationTemplate, { mailCompensationPageSpec } from './mailCompensation';
import BanManagementTemplate, { banManagementPageSpec } from './banManagement';
import OrderReviewTemplate, { orderReviewPageSpec } from './orderReview';
import ActivityConfigTemplate, { activityConfigPageSpec } from './activityConfig';

/** 游戏运营业务模板条目 */
export interface GameOpsTemplateEntry {
  /** 模板 key，同时是控制台路由段（/console/templates/<key>） */
  key: string;
  /** 对应默认菜单分类稳定 key（configs/default-menus.json） */
  menuKey: string;
  title: { 'zh-CN': string; 'en-US': string };
  description: { 'zh-CN': string; 'en-US': string };
  /** 模板预览页组件（内置 demo 执行器） */
  component: ComponentType;
  /** 模板 PageSpec 常量（发布链消费同一份定义） */
  pageSpec: PageSpec;
}

/** 五个默认游戏业务模板（顺序即默认菜单 order 10-50） */
export const gameOpsTemplates: GameOpsTemplateEntry[] = [
  {
    key: 'player-manage',
    menuKey: 'player',
    title: { 'zh-CN': '玩家工作台', 'en-US': 'Player Workbench' },
    description: {
      'zh-CN': '玩家列表、详情、封禁、充值与邮件触达',
      'en-US': 'Player list, detail, ban, recharge and mail',
    },
    component: PlayerManageTemplate,
    pageSpec: playerManagePageSpec,
  },
  {
    key: 'mail-compensation',
    menuKey: 'mail',
    title: { 'zh-CN': '邮件与补偿', 'en-US': 'Mail & Compensation' },
    description: {
      'zh-CN': '邮件任务、补偿任务、目标快照、审批与失败处理',
      'en-US': 'Mail tasks, compensation tasks, target snapshots, approvals and retries',
    },
    component: MailCompensationTemplate,
    pageSpec: mailCompensationPageSpec,
  },
  {
    key: 'ban-management',
    menuKey: 'ban',
    title: { 'zh-CN': '封禁管理', 'en-US': 'Ban Management' },
    description: {
      'zh-CN': '封禁列表、封禁/解封申请与处置历史',
      'en-US': 'Ban list, ban/unban requests and disposition history',
    },
    component: BanManagementTemplate,
    pageSpec: banManagementPageSpec,
  },
  {
    key: 'order-review',
    menuKey: 'payment',
    title: { 'zh-CN': '订单核查', 'en-US': 'Order Review' },
    description: {
      'zh-CN': '订单查询、发货核查、对账差异与补发',
      'en-US': 'Order query, credit check, reconciliation diffs and re-credit',
    },
    component: OrderReviewTemplate,
    pageSpec: orderReviewPageSpec,
  },
  {
    key: 'activity-config',
    menuKey: 'operation',
    title: { 'zh-CN': '活动配置', 'en-US': 'Activity Config' },
    description: {
      'zh-CN': '活动列表、配置版本、发布计划与回滚',
      'en-US': 'Activity list, config versions, publish plans and rollback',
    },
    component: ActivityConfigTemplate,
    pageSpec: activityConfigPageSpec,
  },
];

/** 默认菜单分类稳定 key（与 configs/default-menus.json 前五类一致） */
export const gameOpsDefaultMenuKeys = ['player', 'mail', 'ban', 'payment', 'operation'] as const;

/** 按模板 key 查找条目 */
export function findGameOpsTemplate(key: string): GameOpsTemplateEntry | undefined {
  return gameOpsTemplates.find((entry) => entry.key === key);
}

/** 控制台模板预览路由前缀 */
export const gameOpsTemplateRoutePrefix = '/console/templates';
