/**
 * OPS-011 运维大屏 —— 健康判定纯逻辑。
 *
 * 纯函数，无 React 依赖，便于单测（含「20 节点模拟」验收：1 离线 2 过载
 * 着色正确）。阈值三档可配（默认 60/85，网络级可配登记为边界：当前取默认
 * 常量，接入网络级配置见 docs/operations/ops-dashboard.md 已知边界）。
 */
import type { OpsNode } from '@/services/api/ops';

/** 健康档位：normal 正常 / pressure 压力 / overload 过载 / offline 离线 / unknown 无数据。 */
export type HealthTier = 'normal' | 'pressure' | 'overload' | 'offline' | 'unknown';

/** 三色阈值（CPU/内存各一套，任一越过即升档）。 */
export type HealthThresholds = {
  pressureCpu: number;
  pressureMem: number;
  overloadCpu: number;
  overloadMem: number;
};

export const DEFAULT_HEALTH_THRESHOLDS: HealthThresholds = {
  pressureCpu: 60,
  pressureMem: 60,
  overloadCpu: 85,
  overloadMem: 85,
};

/** 离线/陈旧的节点（灰态置底）。 */
export function isOfflineNode(node: OpsNode): boolean {
  const s = (node.status || '').toLowerCase();
  return s === 'offline' || s === 'stale';
}

/** 无指标样本（cpu/mem 缺失）→ 未知，不误判（与 OPS-007 闲置判定同口径）。 */
export function hasMetrics(node: OpsNode): boolean {
  return node.cpu?.usagePercent != null && node.memory?.usagePercent != null;
}

/**
 * 从 CPU/内存使用率判定原始健康档（不含防抖）。overload 优先，其次 pressure。
 * probeFail 为真直接判过载（探针失败即过载信号，见交付与验收）。
 */
export function tierFromMetrics(
  cpu?: number,
  mem?: number,
  probeFail = false,
  t: HealthThresholds = DEFAULT_HEALTH_THRESHOLDS,
): HealthTier {
  if (probeFail) return 'overload';
  if (cpu == null || mem == null) return 'unknown';
  if (cpu >= t.overloadCpu || mem >= t.overloadMem) return 'overload';
  if (cpu >= t.pressureCpu || mem >= t.pressureMem) return 'pressure';
  return 'normal';
}

/** 节点原始健康档（不防抖）：离线优先，其次指标。 */
export function tierOfNode(
  node: OpsNode,
  probeFail = false,
  t: HealthThresholds = DEFAULT_HEALTH_THRESHOLDS,
): HealthTier {
  if (isOfflineNode(node)) return 'offline';
  return tierFromMetrics(node.cpu?.usagePercent, node.memory?.usagePercent, probeFail, t);
}

/**
 * 防抖：越界档位（pressure/overload）需连续 required 个周期才变色；
 * normal/offline/unknown 立即恢复并清零连续计数。
 *
 * @param target 本轮原始档位
 * @param displayed 当前显示档位（上次调用返回值）
 * @param streak 连续越界周期数
 * @returns 新显示档位与连续计数
 */
export function debounceTier(
  target: HealthTier,
  displayed: HealthTier,
  streak: number,
  required = 3,
): { tier: HealthTier; streak: number } {
  if (target === 'normal' || target === 'offline' || target === 'unknown') {
    return { tier: target, streak: 0 };
  }
  if (target === displayed) {
    return { tier: displayed, streak: streak + 1 };
  }
  const next = streak + 1;
  if (next >= required) {
    return { tier: target, streak: 0 };
  }
  return { tier: displayed, streak: next };
}

/** 排序：离线置底；其余按档位（过载→压力→正常→未知）再按主机名。 */
export function sortNodesForWall(nodes: OpsNode[], tierOf: (n: OpsNode) => HealthTier): OpsNode[] {
  const rank: Record<HealthTier, number> = {
    overload: 0,
    pressure: 1,
    normal: 2,
    unknown: 3,
    offline: 4,
  };
  return [...nodes].sort((a, b) => {
    const ra = rank[tierOf(a)] ?? 3;
    const rb = rank[tierOf(b)] ?? 3;
    if (ra !== rb) return ra - rb;
    const na = (a.hostname || a.id || '').toLowerCase();
    const nb = (b.hostname || b.id || '').toLowerCase();
    return na.localeCompare(nb);
  });
}

/** 聚合带数据。 */
export type FleetSummary = {
  totalNodes: number;
  onlineCount: number;
  onlineRate: number; // 0-100 百分比
  offlineCount: number;
  pressureCount: number;
  overloadCount: number;
  avgCpu: number | null;
  avgMem: number | null;
  activeAlerts: number;
  todayProvisionCount: number;
};

/** 计算聚合带（在线率/平均负载只计有指标且在线节点）。 */
export function buildFleetSummary(
  nodes: OpsNode[],
  tierOf: (n: OpsNode) => HealthTier,
  activeAlerts: number,
  todayProvisionCount: number,
): FleetSummary {
  let online = 0;
  let offline = 0;
  let pressure = 0;
  let overload = 0;
  let cpuSum = 0;
  let memSum = 0;
  let metrified = 0;
  for (const n of nodes) {
    const tier = tierOf(n);
    if (tier === 'offline') offline += 1;
    else online += 1;
    if (tier === 'pressure') pressure += 1;
    if (tier === 'overload') overload += 1;
    if (tier !== 'offline' && hasMetrics(n)) {
      cpuSum += n.cpu?.usagePercent ?? 0;
      memSum += n.memory?.usagePercent ?? 0;
      metrified += 1;
    }
  }
  const total = nodes.length;
  return {
    totalNodes: total,
    onlineCount: online,
    onlineRate: total > 0 ? Math.round((online / total) * 100) : 0,
    offlineCount: offline,
    pressureCount: pressure,
    overloadCount: overload,
    avgCpu: metrified > 0 ? Number((cpuSum / metrified).toFixed(1)) : null,
    avgMem: metrified > 0 ? Number((memSum / metrified).toFixed(1)) : null,
    activeAlerts,
    todayProvisionCount,
  };
}

/** 数据是否过期（最后成功刷新距今 > staleMs）。 */
export function isStale(lastUpdated: number | null, now: number, staleMs = 30_000): boolean {
  if (lastUpdated == null) return true;
  return now - lastUpdated > staleMs;
}

/** 今日（本地时区）创建计数。 */
export function countCreatedToday(createdAt?: string, now = new Date()): boolean {
  if (!createdAt) return false;
  const d = new Date(createdAt);
  if (Number.isNaN(d.getTime())) return false;
  return (
    d.getFullYear() === now.getFullYear() &&
    d.getMonth() === now.getMonth() &&
    d.getDate() === now.getDate()
  );
}
