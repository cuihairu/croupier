/**
 * gameOpsDemoData - 游戏运营业务模板共享示例数据（内存，demo 专用）
 *
 * 面向五个默认游戏业务模板（玩家工作台/邮件与补偿/封禁管理/订单核查/活动配置）
 * 的相互关联示例数据：玩家/区服/角色、邮件任务、补偿任务、封禁记录、订单、
 * 活动配置与操作记录相互引用，数据量足够验证分页/筛选/批量选择。
 *
 * 边界（与 todo E1 口径一致）：
 * - 数据是模板预览/联调用示例，不写真实 scope、不冒充真实游戏数据；
 * - 覆盖空结果筛选、长昵称、多种状态、时区标注（ISO UTC 存储 + 列标题标注）、
 *   数量/金额带单位字段等边界；
 * - 接入真实环境时 demo 执行器整体替换为 executePageBinding，本文件不参与。
 *
 * @module components/PageRenderer/templates/gameOpsDemoData
 */

import type { JSONValue } from '@/types/dashboard';

/** 独立演示游戏环境（仅浏览器内存 demo；不映射真实租户 scope） */
export interface GameOpsDemoEnvironment {
  gameId: string;
  environment: string;
  accountId: string;
  roleName: string;
  permissions: string[];
  persistence: 'memory-only';
}

/** 统一演示 scope 与账号；seed 永远只服务该独立 demo 环境 */
export const gameOpsDemoEnvironment: GameOpsDemoEnvironment = {
  gameId: 'croupier-demo',
  environment: 'local-demo',
  accountId: 'gm-demo',
  roleName: 'Demo GameOps Operator',
  permissions: ['console.read', 'gameops.demo.read', 'gameops.demo.write'],
  persistence: 'memory-only',
};

/** 区服 */
export interface GameOpsServer {
  [key: string]: JSONValue;
  id: string;
  name: string;
  status: 'healthy' | 'busy' | 'maintenance';
}

/** 玩家（含角色与区服归属） */
export interface GameOpsPlayer {
  [key: string]: JSONValue;
  id: string;
  nickname: string;
  character: string;
  serverId: string;
  level: number;
  vip: number;
  status: 'active' | 'banned';
  /** 金币余额（单位：金） */
  balanceGold: number;
  lastLoginAt: string;
  createdAt: string;
}

/** 邮件/补偿运营任务（两类共用一张任务列表，type 区分） */
export interface GameOpsTask {
  [key: string]: JSONValue;
  id: string;
  type: 'mail' | 'compensation';
  title: string;
  /** 邮件正文或补偿原因 */
  detail: string;
  receiverRange: 'all' | 'server' | 'players';
  /** 区服级任务的真实区服引用 */
  targetServerId: string | null;
  /** 玩家级任务的真实玩家引用 */
  targetPlayerId: string | null;
  /** 充值补偿指向的真实订单号 */
  targetOrderNo: string | null;
  /** 目标人数快照 */
  targetCount: number;
  /** 补偿每人数量（单位随 currency；邮件任务为 null） */
  perAmount: number | null;
  /** 补偿总量 = perAmount * targetCount（邮件任务为 null） */
  totalAmount: number | null;
  currency: string | null;
  status:
    | 'pending_approval'
    | 'sending'
    | 'sent'
    | 'partial_failed'
    | 'executing'
    | 'completed'
    | 'failed'
    | 'cancelled';
  sentCount: number;
  failedCount: number;
  operator: string;
  approvalId: string | null;
  createdAt: string;
}

/** 封禁记录 */
export interface GameOpsBanRecord {
  [key: string]: JSONValue;
  id: string;
  playerId: string;
  nickname: string;
  serverId: string;
  approvalId: string;
  reason: 'cheating' | 'fraud' | 'abuse' | 'risk';
  durationHours: number;
  bannedAt: string;
  expiresAt: string;
  status: 'active' | 'expired' | 'lifted';
  operator: string;
  approver: string;
}

/** 订单（支付凭据与游戏发货凭据分列） */
export interface GameOpsOrder {
  [key: string]: JSONValue;
  orderNo: string;
  playerId: string;
  nickname: string;
  /** 支付金额（单位随 currency） */
  amount: number;
  currency: 'CNY' | 'USD';
  channel: 'official' | 'ios' | 'android' | 'wechat' | 'alipay';
  status: 'paid' | 'credited' | 'failed' | 'refunded';
  /** 游戏侧发货状态 */
  gameCredit: 'credited' | 'pending' | 'missing' | 'not_needed';
  /** 对账结论 */
  reconciliation: 'match' | 'missing_credit' | 'missing_order';
  payTransactionId: string;
  gameCreditRef: string | null;
  paidAt: string;
  creditedAt: string | null;
}

/** 活动配置版本 */
export interface GameOpsActivityVersion {
  [key: string]: JSONValue;
  version: number;
  note: string;
  publishedAt: string;
  operator: string;
}

/** 活动配置 */
export interface GameOpsActivity {
  [key: string]: JSONValue;
  id: string;
  name: string;
  type: 'login_gift' | 'rank_reward' | 'shop_discount' | 'holiday_event';
  /** 活动时区（显式声明，不从字段名猜测） */
  timezone: 'Asia/Shanghai' | 'UTC' | 'America/Los_Angeles';
  startAt: string;
  endAt: string;
  status: 'draft' | 'configured' | 'published' | 'active' | 'ended' | 'rolled_back';
  currentVersion: number;
  versions: GameOpsActivityVersion[];
}

/** 操作记录（operationId/approvalId 贯穿） */
export interface GameOpsOpRecord {
  [key: string]: JSONValue;
  id: string;
  at: string;
  operator: string;
  action: string;
  target: string;
  result: 'success' | 'pending_approval' | 'failed';
  operationId: string;
  approvalId: string | null;
}

/** 共享示例数据仓库（可变；各模板 demo 执行器在此之上做业务闭环） */
export interface GameOpsDemoStore {
  servers: GameOpsServer[];
  players: GameOpsPlayer[];
  tasks: GameOpsTask[];
  bans: GameOpsBanRecord[];
  orders: GameOpsOrder[];
  activities: GameOpsActivity[];
  opRecords: GameOpsOpRecord[];
  /** 恢复到初始 seed，供模板页重置入口调用 */
  reset: () => void;
  /** 追加一条操作记录并返回 operationId */
  pushOpRecord(entry: {
    operator: string;
    action: string;
    target: string;
    result: GameOpsOpRecord['result'];
    approvalId?: string | null;
  }): string;
}

/** 长昵称边界用例（>40 字符，验证列宽/截断/详情不溢出） */
const LONG_NICKNAME = '风起云涌一区无敌战神丶樱花漫天飞舞的超级无敌可爱小龙女本尊在此';

/** 构造相互关联的示例数据（每次调用全新实例，测试间互不污染） */
export function createGameOpsDemoStore(): GameOpsDemoStore {
  const now = Date.now();
  const iso = (msAgo: number) => new Date(now - msAgo).toISOString();

  const servers: GameOpsServer[] = [
    { id: 's1', name: '电信一区', status: 'healthy' },
    { id: 's2', name: '电信二区', status: 'busy' },
    { id: 's3', name: '联通一区', status: 'maintenance' },
  ];

  // 26 名玩家：足够多页；#7 封禁态；#13 长昵称边界
  const players: GameOpsPlayer[] = Array.from({ length: 26 }, (_, index) => {
    const seq = index + 1;
    return {
      id: `200${String(seq).padStart(2, '0')}`,
      nickname: seq === 13 ? LONG_NICKNAME : `玩家${seq}号`,
      character: `角色${seq}`,
      serverId: servers[seq % servers.length].id,
      level: 5 + ((seq * 7) % 55),
      vip: seq % 7,
      status: seq % 7 === 0 ? 'banned' : 'active',
      balanceGold: (seq * 977) % 9000,
      lastLoginAt: iso(seq * 3600_000),
      createdAt: iso(seq * 86_400_000 * 3),
    };
  });

  const tasks: GameOpsTask[] = [
    {
      id: 'task-mail-1001',
      type: 'mail',
      title: 'S3 赛季结算奖励',
      detail: '恭喜完成 S3 赛季，奖励已随信发放。',
      receiverRange: 'all',
      targetServerId: null,
      targetPlayerId: null,
      targetOrderNo: null,
      targetCount: 26,
      perAmount: null,
      totalAmount: null,
      currency: null,
      status: 'sent',
      sentCount: 26,
      failedCount: 0,
      operator: 'gm01',
      approvalId: null,
      createdAt: iso(49 * 3600_000),
    },
    {
      id: 'task-mail-1002',
      type: 'mail',
      title: '停机维护补偿',
      detail: '感谢理解，维护补偿见附件。',
      receiverRange: 'server',
      targetServerId: 's1',
      targetPlayerId: null,
      targetOrderNo: null,
      targetCount: 9,
      perAmount: null,
      totalAmount: null,
      currency: null,
      status: 'partial_failed',
      sentCount: 7,
      failedCount: 2,
      operator: 'operator',
      approvalId: null,
      createdAt: iso(20 * 3600_000),
    },
    {
      id: 'task-comp-2001',
      type: 'compensation',
      title: '回档补偿',
      detail: '10-21 回档事故全员补偿',
      receiverRange: 'all',
      targetServerId: null,
      targetPlayerId: null,
      targetOrderNo: null,
      targetCount: 26,
      perAmount: 500,
      totalAmount: 13000,
      currency: '金',
      status: 'pending_approval',
      sentCount: 0,
      failedCount: 0,
      operator: 'operator',
      approvalId: 'approval-comp-2001',
      createdAt: iso(2 * 3600_000),
    },
    {
      id: 'task-comp-2002',
      type: 'compensation',
      title: '充值未到账补偿',
      detail: '订单 20261023001 补发',
      receiverRange: 'players',
      targetServerId: null,
      targetPlayerId: '20008',
      targetOrderNo: '2026102308',
      targetCount: 1,
      perAmount: 648,
      totalAmount: 648,
      currency: '金',
      status: 'completed',
      sentCount: 1,
      failedCount: 0,
      operator: 'gm01',
      approvalId: 'approval-comp-2002',
      createdAt: iso(30 * 3600_000),
    },
  ];

  // 12 条封禁：三种状态齐备，玩家引用真实存在
  const banReasons: GameOpsBanRecord['reason'][] = ['cheating', 'fraud', 'abuse', 'risk'];
  const bans: GameOpsBanRecord[] = Array.from({ length: 12 }, (_, index) => {
    const seq = index + 1;
    const player = players[(seq * 3) % players.length];
    const durationHours = [24, 72, 168, 720][seq % 4];
    return {
      id: `ban-300${String(seq).padStart(2, '0')}`,
      playerId: player.id,
      nickname: player.nickname,
      serverId: player.serverId,
      approvalId: `approval-ban-${String(seq).padStart(4, '0')}`,
      reason: banReasons[seq % banReasons.length],
      durationHours,
      bannedAt: iso((seq + 2) * 86_400_000),
      expiresAt: iso((seq + 2) * 86_400_000 - durationHours * 3600_000),
      status: seq % 3 === 0 ? 'expired' : seq % 5 === 0 ? 'lifted' : 'active',
      operator: seq % 2 === 0 ? 'operator' : 'gm01',
      approver: 'admin',
    };
  });

  // 30 笔订单：对账三类 + 发货四态齐备；#8 缺发货（可演示补发）
  const channels: GameOpsOrder['channel'][] = ['official', 'ios', 'android', 'wechat', 'alipay'];
  const orders: GameOpsOrder[] = Array.from({ length: 30 }, (_, index) => {
    const seq = index + 1;
    const player = players[(seq * 5) % players.length];
    const missingCredit = seq === 8 || seq === 17;
    const missingOrder = seq === 24;
    const failed = seq % 11 === 0;
    const gameCredit: GameOpsOrder['gameCredit'] = failed
      ? 'not_needed'
      : missingCredit
        ? 'missing'
        : seq === 26
          ? 'pending'
          : 'credited';
    const orderNo = `2026102${String(300 + seq)}`;
    return {
      orderNo,
      playerId: player.id,
      nickname: player.nickname,
      amount: [6, 30, 98, 328, 648][seq % 5],
      currency: seq % 9 === 0 ? 'USD' : 'CNY',
      channel: channels[seq % channels.length],
      status: failed ? 'failed' : missingCredit || seq === 26 ? 'paid' : 'credited',
      gameCredit,
      reconciliation: missingCredit ? 'missing_credit' : missingOrder ? 'missing_order' : 'match',
      payTransactionId: `pay-${9000 + seq}`,
      gameCreditRef: gameCredit === 'credited' ? `credit-${7000 + seq}` : null,
      paidAt: iso(seq * 5 * 3600_000),
      creditedAt: gameCredit === 'credited' ? iso(seq * 5 * 3600_000 - 60_000) : null,
    };
  });

  const activityTypes: GameOpsActivity['type'][] = [
    'login_gift',
    'rank_reward',
    'shop_discount',
    'holiday_event',
  ];
  const timezones: GameOpsActivity['timezone'][] = ['Asia/Shanghai', 'UTC', 'America/Los_Angeles'];
  const activities: GameOpsActivity[] = Array.from({ length: 8 }, (_, index) => {
    const seq = index + 1;
    const versionCount = (seq % 3) + 1;
    return {
      id: `act-400${seq}`,
      name: `${['每日登录礼', '巅峰排行榜', '限时折扣商店', '节日庆典'][seq % 4]}·第${seq}期`,
      type: activityTypes[seq % activityTypes.length],
      timezone: timezones[seq % timezones.length],
      startAt: iso(-seq * 7 * 86_400_000),
      endAt: iso(seq * 86_400_000),
      status:
        seq % 4 === 0
          ? 'active'
          : seq % 4 === 1
            ? 'configured'
            : seq % 4 === 2
              ? 'published'
              : 'ended',
      currentVersion: versionCount,
      versions: Array.from({ length: versionCount }, (_, v) => ({
        version: v + 1,
        note: v === 0 ? '初版配置' : `调整奖励数值（v${v + 1}）`,
        publishedAt: iso((versionCount - v) * 86_400_000),
        operator: v % 2 === 0 ? 'gm01' : 'operator',
      })),
    };
  });

  const opRecords: GameOpsOpRecord[] = [
    {
      id: 'op-5001',
      at: iso(26 * 3600_000),
      operator: 'operator',
      action: 'player.ban',
      target: 'ban-30003',
      result: 'success',
      operationId: 'op-5001',
      approvalId: 'approval-ban-0003',
    },
    {
      id: 'op-5002',
      at: iso(2 * 3600_000),
      operator: 'operator',
      action: 'compensation.grant',
      target: 'task-comp-2001',
      result: 'pending_approval',
      operationId: 'op-5002',
      approvalId: 'approval-comp-2001',
    },
    {
      id: 'op-5003',
      at: iso(8 * 3600_000),
      operator: 'gm01',
      action: 'activity.publish',
      target: 'act-4003',
      result: 'success',
      operationId: 'op-5003',
      approvalId: null,
    },
  ];

  let opRecordSeq = 0;
  const store: GameOpsDemoStore = {
    servers,
    players,
    tasks,
    bans,
    orders,
    activities,
    opRecords,
    reset() {
      // 原地替换整组关联 seed，共享 store 的调用方无需重新绑定。
      const fresh = createGameOpsDemoStore();
      store.servers = fresh.servers;
      store.players = fresh.players;
      store.tasks = fresh.tasks;
      store.bans = fresh.bans;
      store.orders = fresh.orders;
      store.activities = fresh.activities;
      store.opRecords = fresh.opRecords;
      opRecordSeq = 0;
    },
    pushOpRecord(entry) {
      opRecordSeq += 1;
      const operationId = `op-demo-${opRecordSeq}`;
      store.opRecords.unshift({
        id: operationId,
        at: new Date().toISOString(),
        operator: entry.operator,
        action: entry.action,
        target: entry.target,
        result: entry.result,
        operationId,
        approvalId: entry.approvalId ?? null,
      });
      return operationId;
    },
  };
  return store;
}

/** 从未知来源读取对象记录（demo 执行器防御） */
export function asRecord(value: JSONValue | undefined): Record<string, JSONValue> {
  return value && typeof value === 'object' && !Array.isArray(value)
    ? (value as Record<string, JSONValue>)
    : {};
}

/** 自增 requestId 工厂（对齐 PageExecutionResult 契约） */
export function createRequestIdFactory(): () => string {
  let seq = 0;
  return () => `gameops-demo-${++seq}`;
}
