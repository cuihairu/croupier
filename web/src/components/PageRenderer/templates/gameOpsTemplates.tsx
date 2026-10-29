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

/**
 * 模板能力分类（界面生成链 C1：按玩家查询 / 资源管理 / 补偿操作 / 任务监控
 * 四类组织模板选择）。一个模板可属于多类——邮件与补偿模板同时承载补偿操作与
 * 任务监控（mail_task.* 的 retry_failed / cancel 就是任务监控面）。
 */
export type GameOpsTemplateCategory =
  'player-query' | 'resource-management' | 'compensation' | 'task-monitoring';

/** 四类能力分类的稳定 key 与双语文案（模板选择按此分组展示） */
export const gameOpsTemplateCategories: Array<{
  key: GameOpsTemplateCategory;
  label: { 'zh-CN': string; 'en-US': string };
}> = [
  { key: 'player-query', label: { 'zh-CN': '玩家查询', 'en-US': 'Player Query' } },
  {
    key: 'resource-management',
    label: { 'zh-CN': '资源管理', 'en-US': 'Resource Management' },
  },
  { key: 'compensation', label: { 'zh-CN': '补偿操作', 'en-US': 'Compensation' } },
  { key: 'task-monitoring', label: { 'zh-CN': '任务监控', 'en-US': 'Task Monitoring' } },
];

/** 模板函数绑定在模板中的角色（CRUD 能力轴） */
export type GameOpsTemplateFunctionRole =
  'list' | 'get' | 'create' | 'update' | 'delete' | 'action';

/** 模板函数绑定：游戏函数契约 id + 角色，是模板 → 提案 → 发布链的依赖声明 */
export interface GameOpsTemplateFunction {
  /** 游戏函数契约 id */
  id: string;
  /** 该函数在模板中的角色 */
  role: GameOpsTemplateFunctionRole;
}

/** CRUD 能力轴（C1 验收口径：查询 → 详情 → 新建/编辑/删除确认 → 成功刷新） */
export const gameOpsTemplateCrudRoles: GameOpsTemplateFunctionRole[] = [
  'create',
  'update',
  'delete',
];

/** 游戏运营业务模板条目 */
export interface GameOpsTemplateEntry {
  /** 模板 key，同时是控制台路由段（/console/templates/<key>） */
  key: string;
  /** 对应默认菜单分类稳定 key（configs/default-menus.json） */
  menuKey: string;
  /** 能力分类（至少一类，按 gameOpsTemplateCategories 顺序排列） */
  categories: GameOpsTemplateCategory[];
  title: { 'zh-CN': string; 'en-US': string };
  description: { 'zh-CN': string; 'en-US': string };
  /** 模板绑定的游戏函数（与 pageSpec.bindings.functionId 一一对应，禁止漂移） */
  functions: GameOpsTemplateFunction[];
  /** CRUD 能力覆盖说明：解释模板为何不提供新建/编辑/删除中的某些角色 */
  capabilitiesNote: { 'zh-CN': string; 'en-US': string };
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
    categories: ['player-query'],
    title: { 'zh-CN': '玩家工作台', 'en-US': 'Player Workbench' },
    description: {
      'zh-CN': '玩家列表、详情、封禁、充值与邮件触达',
      'en-US': 'Player list, detail, ban, recharge and mail',
    },
    functions: [
      { id: 'player.list', role: 'list' },
      { id: 'player.get', role: 'get' },
      { id: 'player.create', role: 'create' },
      { id: 'player.update', role: 'update' },
      { id: 'player.delete', role: 'delete' },
      { id: 'player.ban', role: 'action' },
      { id: 'player.batchBan', role: 'action' },
      { id: 'player.batchUnban', role: 'action' },
      { id: 'player.batchRecharge', role: 'action' },
      { id: 'player.mail', role: 'action' },
      { id: 'player.recharge', role: 'action' },
    ],
    capabilitiesNote: {
      'zh-CN': '新建/编辑/删除齐备；封禁与批量处置走独立 action 角色。',
      'en-US': 'Create/update/delete all covered; bans and batch handling use the action role.',
    },
    component: PlayerManageTemplate,
    pageSpec: playerManagePageSpec,
  },
  {
    key: 'mail-compensation',
    menuKey: 'mail',
    categories: ['compensation', 'task-monitoring'],
    title: { 'zh-CN': '邮件与补偿', 'en-US': 'Mail & Compensation' },
    description: {
      'zh-CN': '邮件任务、补偿任务、目标快照、审批与失败处理',
      'en-US': 'Mail tasks, compensation tasks, target snapshots, approvals and retries',
    },
    functions: [
      { id: 'mail_task.list', role: 'list' },
      { id: 'mail_task.get', role: 'get' },
      { id: 'mail_task.create', role: 'create' },
      { id: 'mail_task.update', role: 'update' },
      { id: 'mail_task.retry_failed', role: 'action' },
      { id: 'mail_task.cancel', role: 'action' },
    ],
    capabilitiesNote: {
      'zh-CN':
        '提供新建与编辑；任务终态不删除而是取消（action.cancel），故无 delete 角色。失败重试为 action 角色。',
      'en-US':
        'Create and update covered; tasks end via cancel (action) rather than delete, so there is no delete role. Retry is an action role.',
    },
    component: MailCompensationTemplate,
    pageSpec: mailCompensationPageSpec,
  },
  {
    key: 'ban-management',
    menuKey: 'ban',
    categories: ['resource-management'],
    title: { 'zh-CN': '封禁管理', 'en-US': 'Ban Management' },
    description: {
      'zh-CN': '封禁列表、封禁/解封申请与处置历史',
      'en-US': 'Ban list, ban/unban requests and disposition history',
    },
    functions: [
      { id: 'ban.list', role: 'list' },
      { id: 'ban.get', role: 'get' },
      { id: 'ban.create', role: 'create' },
      { id: 'ban.extend', role: 'action' },
      { id: 'ban.unban', role: 'action' },
      { id: 'ban.batch_unban', role: 'action' },
    ],
    capabilitiesNote: {
      'zh-CN':
        '封禁记录不提供原地编辑/删除：变更走延长（action.extend）与解除（action.unban / batch_unban），处置历史留痕。',
      'en-US':
        'Ban records are not edited or deleted in place: changes go through extend and unban actions, keeping the disposition history.',
    },
    component: BanManagementTemplate,
    pageSpec: banManagementPageSpec,
  },
  {
    key: 'order-review',
    menuKey: 'payment',
    categories: ['resource-management'],
    title: { 'zh-CN': '订单核查', 'en-US': 'Order Review' },
    description: {
      'zh-CN': '订单查询、发货核查、对账差异与补发',
      'en-US': 'Order query, credit check, reconciliation diffs and re-credit',
    },
    functions: [
      { id: 'order.list', role: 'list' },
      { id: 'order.get', role: 'get' },
      { id: 'order.recredit', role: 'action' },
    ],
    capabilitiesNote: {
      'zh-CN':
        '订单由支付渠道写入，本模板只读：无新建/编辑/删除，仅对缺发货订单补发（action.recredit，仅缺发货可执行）。',
      'en-US':
        'Orders are written by payment channels, so this template is read-only: no create/update/delete, only re-credit for orders missing game credit.',
    },
    component: OrderReviewTemplate,
    pageSpec: orderReviewPageSpec,
  },
  {
    key: 'activity-config',
    menuKey: 'operation',
    categories: ['resource-management'],
    title: { 'zh-CN': '活动配置', 'en-US': 'Activity Config' },
    description: {
      'zh-CN': '活动列表、配置版本、发布计划与回滚',
      'en-US': 'Activity list, config versions, publish plans and rollback',
    },
    functions: [
      { id: 'activity.list', role: 'list' },
      { id: 'activity.get', role: 'get' },
      { id: 'activity.create', role: 'create' },
      { id: 'activity.publish_version', role: 'action' },
      { id: 'activity.activate', role: 'action' },
      { id: 'activity.end', role: 'action' },
      { id: 'activity.rollback', role: 'action' },
    ],
    capabilitiesNote: {
      'zh-CN':
        '活动配置走版本化演进：新建后以发版（action.publish_version）与回滚推进，不提供原地编辑/删除。',
      'en-US':
        'Activity configs evolve through versions: after create, changes go through publish_version and rollback, with no in-place edit or delete.',
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

/** 模板绑定的角色集合（按 gameOpsTemplateCrudRoles 之前的声明顺序去重） */
export function templateFunctionRoles(entry: GameOpsTemplateEntry): GameOpsTemplateFunctionRole[] {
  const roles: GameOpsTemplateFunctionRole[] = [];
  for (const fn of entry.functions) {
    if (!roles.includes(fn.role)) roles.push(fn.role);
  }
  return roles;
}

/**
 * 缺能力的 CRUD 角色（C1：缺能力显式提示，不以生成节点存在替代可用流程）。
 * 返回新建/编辑/删除中模板未声明的角色，调用方据此向使用者提示能力缺口。
 */
export function missingTemplateCrudRoles(
  entry: GameOpsTemplateEntry,
): GameOpsTemplateFunctionRole[] {
  const roles = templateFunctionRoles(entry);
  return gameOpsTemplateCrudRoles.filter((role) => !roles.includes(role));
}

/** 按能力分类分组模板（分类顺序即 gameOpsTemplateCategories 顺序） */
export function gameOpsTemplatesByCategory(): Array<{
  category: GameOpsTemplateCategory;
  label: { 'zh-CN': string; 'en-US': string };
  entries: GameOpsTemplateEntry[];
}> {
  return gameOpsTemplateCategories.map((category) => ({
    category: category.key,
    label: category.label,
    entries: gameOpsTemplates.filter((entry) => entry.categories.includes(category.key)),
  }));
}

/** 控制台模板预览路由前缀 */
export const gameOpsTemplateRoutePrefix = '/console/templates';
