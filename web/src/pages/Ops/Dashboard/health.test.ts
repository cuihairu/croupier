/**
 * OPS-011 运维大屏健康判定纯逻辑单测。
 *
 * 覆盖：阈值分类（60/85 三档）、探针失败→过载、离线/无数据判定、
 * 防抖（连续 3 周期越界才变色，normal 立即恢复）、离线置底排序、
 * 聚合带（在线率/平均负载只计有指标在线节点）、过期检测、今日计数。
 */
import {
  buildFleetSummary,
  countCreatedToday,
  debounceTier,
  DEFAULT_HEALTH_THRESHOLDS,
  hasMetrics,
  isOfflineNode,
  isStale,
  sortNodesForWall,
  tierFromMetrics,
  tierOfNode,
} from './health';
import type { OpsNode } from '@/services/api/ops';

type PartialNode = {
  id: string;
  hostname?: string;
  status?: string;
  gameId?: string;
  env?: string;
  cpu?: { usagePercent?: number };
  memory?: { usagePercent?: number };
};

function node(partial: PartialNode): OpsNode {
  const cpu = partial.cpu
    ? { usagePercent: partial.cpu.usagePercent ?? 0, cores: 0, load1m: 0, load5m: 0, load15m: 0 }
    : undefined;
  const memory = partial.memory
    ? {
        totalBytes: 0,
        usedBytes: 0,
        availableBytes: 0,
        usagePercent: partial.memory.usagePercent ?? 0,
        swapTotal: 0,
        swapUsed: 0,
      }
    : undefined;
  return {
    id: partial.id,
    hostname: partial.hostname,
    status: partial.status,
    gameId: partial.gameId,
    env: partial.env,
    cpu,
    memory,
  } as OpsNode;
}

describe('tierFromMetrics 阈值分类', () => {
  it('cpu<60 且 mem<60 → normal', () => {
    expect(tierFromMetrics(30, 40)).toBe('normal');
  });
  it('cpu>=60 或 mem>=60 → pressure', () => {
    expect(tierFromMetrics(60, 40)).toBe('pressure');
    expect(tierFromMetrics(40, 84)).toBe('pressure'); // 60-85 区间为压力
    expect(tierFromMetrics(70, 70)).toBe('pressure');
  });
  it('cpu>=85 或 mem>=85 → overload', () => {
    expect(tierFromMetrics(90, 40)).toBe('overload');
    expect(tierFromMetrics(40, 95)).toBe('overload');
  });
  it('无指标 → unknown（不误判）', () => {
    expect(tierFromMetrics(undefined, undefined)).toBe('unknown');
    expect(tierFromMetrics(50, undefined)).toBe('unknown');
  });
  it('探针失败 → overload（优先于指标）', () => {
    expect(tierFromMetrics(20, 20, true)).toBe('overload');
  });
  it('自定义阈值生效', () => {
    const t = { pressureCpu: 70, pressureMem: 70, overloadCpu: 95, overloadMem: 95 };
    expect(tierFromMetrics(75, 40, false, t)).toBe('pressure');
    expect(tierFromMetrics(96, 40, false, t)).toBe('overload');
  });
});

describe('isOfflineNode / hasMetrics', () => {
  it('offline/stale → 离线；其余不判离线', () => {
    expect(isOfflineNode(node({ id: 'a', status: 'offline' }))).toBe(true);
    expect(isOfflineNode(node({ id: 'b', status: 'stale' }))).toBe(true);
    expect(isOfflineNode(node({ id: 'c', status: 'online' }))).toBe(false);
    expect(isOfflineNode(node({ id: 'd' }))).toBe(false);
  });
  it('hasMetrics 需要 cpu+mem 齐备', () => {
    expect(
      hasMetrics(node({ id: 'a', cpu: { usagePercent: 10 }, memory: { usagePercent: 20 } })),
    ).toBe(true);
    expect(hasMetrics(node({ id: 'b' }))).toBe(false);
    expect(hasMetrics(node({ id: 'c', cpu: { usagePercent: 10 } }))).toBe(false);
  });
});

describe('tierOfNode 离线优先', () => {
  it('offline 节点直接判离线（不读指标）', () => {
    const n = node({ id: 'a', status: 'offline', cpu: { usagePercent: 99 } });
    expect(tierOfNode(n)).toBe('offline');
  });
  it('在线节点走指标', () => {
    const n = node({
      id: 'a',
      status: 'online',
      cpu: { usagePercent: 90 },
      memory: { usagePercent: 30 },
    });
    expect(tierOfNode(n)).toBe('overload');
  });
});

describe('debounceTier 连续 3 周期越界才变色', () => {
  it('首次越界不立即变色，保持原档', () => {
    expect(debounceTier('overload', 'normal', 0)).toEqual({ tier: 'normal', streak: 1 });
  });
  it('第二次仍保持', () => {
    expect(debounceTier('overload', 'normal', 1)).toEqual({ tier: 'normal', streak: 2 });
  });
  it('第三次达阈值变色并清零', () => {
    expect(debounceTier('overload', 'normal', 2)).toEqual({ tier: 'overload', streak: 0 });
  });
  it('normal 立即恢复并清零', () => {
    expect(debounceTier('normal', 'overload', 0)).toEqual({ tier: 'normal', streak: 0 });
  });
  it('offline/unknown 立即生效', () => {
    expect(debounceTier('offline', 'normal', 0)).toEqual({ tier: 'offline', streak: 0 });
    expect(debounceTier('unknown', 'overload', 1)).toEqual({ tier: 'unknown', streak: 0 });
  });
  it('required 可配（如 5）', () => {
    expect(debounceTier('overload', 'normal', 0, 5)).toEqual({ tier: 'normal', streak: 1 });
    expect(debounceTier('overload', 'normal', 4, 5)).toEqual({ tier: 'overload', streak: 0 });
  });
});

describe('sortNodesForWall 离线置底', () => {
  const tierOf = (n: OpsNode) => tierOfNode(n);
  it('离线节点排最后，其余按档位再按主机名', () => {
    const nodes = [
      node({
        id: 'n1',
        hostname: 'b-host',
        status: 'online',
        cpu: { usagePercent: 30 },
        memory: { usagePercent: 30 },
      }),
      node({
        id: 'n2',
        hostname: 'a-host',
        status: 'online',
        cpu: { usagePercent: 95 },
        memory: { usagePercent: 30 },
      }),
      node({ id: 'n3', hostname: 'c-host', status: 'offline' }),
      node({
        id: 'n4',
        hostname: 'd-host',
        status: 'online',
        cpu: { usagePercent: 70 },
        memory: { usagePercent: 30 },
      }),
    ];
    const sorted = sortNodesForWall(nodes, tierOf);
    expect(sorted.map((n) => n.id)).toEqual(['n2', 'n4', 'n1', 'n3']);
  });
});

describe('buildFleetSummary 聚合带', () => {
  const tierOf = (n: OpsNode) => tierOfNode(n);
  it('20 节点模拟：1 离线 2 过载 1 压力 → 计数与在线率正确', () => {
    const nodes: OpsNode[] = [];
    // 16 正常
    for (let i = 0; i < 16; i++) {
      nodes.push(
        node({
          id: `n${i}`,
          hostname: `h${i}`,
          status: 'online',
          cpu: { usagePercent: 20 },
          memory: { usagePercent: 30 },
        }),
      );
    }
    // 1 离线
    nodes.push(node({ id: 'off', hostname: 'h-off', status: 'offline' }));
    // 2 过载
    nodes.push(
      node({
        id: 'ov1',
        hostname: 'h-ov1',
        status: 'online',
        cpu: { usagePercent: 95 },
        memory: { usagePercent: 40 },
      }),
    );
    nodes.push(
      node({
        id: 'ov2',
        hostname: 'h-ov2',
        status: 'online',
        cpu: { usagePercent: 40 },
        memory: { usagePercent: 92 },
      }),
    );
    // 1 压力
    nodes.push(
      node({
        id: 'pr1',
        hostname: 'h-pr1',
        status: 'online',
        cpu: { usagePercent: 70 },
        memory: { usagePercent: 30 },
      }),
    );
    expect(nodes.length).toBe(20);
    const s = buildFleetSummary(nodes, tierOf, 3, 2);
    expect(s.totalNodes).toBe(20);
    expect(s.offlineCount).toBe(1);
    expect(s.onlineCount).toBe(19);
    expect(s.onlineRate).toBe(95); // 19/20
    expect(s.overloadCount).toBe(2);
    expect(s.pressureCount).toBe(1);
    expect(s.activeAlerts).toBe(3);
    expect(s.todayProvisionCount).toBe(2);
    expect(s.avgCpu).not.toBeNull();
    expect(s.avgMem).not.toBeNull();
  });
  it('空列表 → 零聚合（不除零）', () => {
    const s = buildFleetSummary([], () => 'normal', 0, 0);
    expect(s).toMatchObject({ totalNodes: 0, onlineRate: 0, avgCpu: null, avgMem: null });
  });

  // 审核发现：旧实现只判 hasMetrics 未排除 offline——离线节点的陈旧 cpu/mem
  // 污染平均负载。回归：offline 节点有指标也不计入均值。
  it('offline 节点有陈旧指标 → 不计入 avgCpu/avgMem', () => {
    const nodes: OpsNode[] = [
      node({
        id: 'live',
        hostname: 'h-live',
        status: 'online',
        cpu: { usagePercent: 40 },
        memory: { usagePercent: 50 },
      }),
      node({
        id: 'stale-offline',
        hostname: 'h-stale',
        status: 'offline',
        cpu: { usagePercent: 99 },
        memory: { usagePercent: 99 },
      }),
    ];
    const s = buildFleetSummary(nodes, (n) => tierOfNode(n), 0, 0);
    expect(s.totalNodes).toBe(2);
    expect(s.offlineCount).toBe(1);
    // 旧实现会得 (40+99)/2=69.5；修复后只用在线节点 40/50。
    expect(s.avgCpu).toBe(40);
    expect(s.avgMem).toBe(50);
  });
});

describe('isStale 数据过期', () => {
  it('无时间戳 → 过期', () => {
    expect(isStale(null, 1000)).toBe(true);
  });
  it('>30s 未刷新 → 过期', () => {
    expect(isStale(1000, 31_001)).toBe(true);
  });
  it('<=30s → 新鲜', () => {
    expect(isStale(1000, 31_000)).toBe(false);
  });
});

describe('countCreatedToday 今日计数', () => {
  const now = new Date(2026, 10, 3, 12, 0, 0); // 2026-11-03 12:00 本地
  it('今日 → true', () => {
    expect(countCreatedToday('2026-11-03T08:00:00Z', now)).toBe(true);
  });
  it('非今日 → false', () => {
    expect(countCreatedToday('2026-11-02T08:00:00Z', now)).toBe(false);
  });
  it('非法/空 → false', () => {
    expect(countCreatedToday('', now)).toBe(false);
    expect(countCreatedToday('not-a-date', now)).toBe(false);
    expect(countCreatedToday(undefined, now)).toBe(false);
  });
});

describe('DEFAULT_HEALTH_THRESHOLDS 契约', () => {
  it('默认 60/85', () => {
    expect(DEFAULT_HEALTH_THRESHOLDS.pressureCpu).toBe(60);
    expect(DEFAULT_HEALTH_THRESHOLDS.overloadCpu).toBe(85);
  });
});
