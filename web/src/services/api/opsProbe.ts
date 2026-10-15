import { request } from '@umijs/max';

// Source: croupier/internal/api/ops/probe_summary.go ProbeSummaryResponse/Row
// 探针在线汇总状态矩阵（agent-health-recovery-review §13.2）：
// 一行 = 目标 × 执行位置；只呈现事实（窗口开合 + 聚合计数 + 最近失败原因），
// 不出单一合成分——探测是事实不是渐变分数。
//
// 两视角并存：agent 视角来自最近一份 MetricsReport 的 probe_stats 快照，
// server 视角（agentId='server'）来自 server 本机探针面当前状态。

// Source: croupier/internal/api/ops/probe_summary.go ProbeSummaryRow
export type ProbeSummaryRow = {
  agentId: string;
  target: string;
  /** 视角声明（readiness / liveness），缺省由内核按目标类型推导。 */
  semantics?: string;
  /** 目标归属 scope；空 = 平台级行。 */
  gameId?: string;
  env?: string;
  /** 探活结果（内核两态）。 */
  available: boolean;
  /** 故障窗口是否开着；开着即 unavailable。 */
  windowOpen: boolean;
  /** 开着的窗口已持续毫秒数。 */
  windowDurationMs?: number;
  checked: number;
  failed: number;
  /** 0-1 比率；checked=0 视为缺失（服务端不给该行造 1）。 */
  successRate: number;
  lastReason?: string;
  /** 数据新鲜度锚点（RFC3339）；server 视角 = 快照时点。 */
  lastReportAt?: string;
  /** checker 类型（http|tcp|db|protocol）；旧版 agent 未上报时缺省。 */
  probeType?: string;
  /** 游戏协议适配器名（仅 type=protocol 且走适配器时非空）。 */
  protocol?: string;
  /** 最近一次失败的阶段标记（协议 Detailing 出口；无 phase 的 checker 为空）。 */
  phase?: string;
};

// Source: croupier/internal/api/ops/probe_summary.go ProbeSummaryResponse
export type ProbeSummaryResult = {
  /** 在线趋势数据是否已接入；false = 灰态「未接入在线数据」。 */
  onlineDataIntegrated: boolean;
  rows: ProbeSummaryRow[];
};

// scope（X-Game-ID/X-Env）由全局请求拦截器注入，与告警列表同口径：
// 带 scope 返回同 scope 行 + 平台级行，不带 scope 返回全部。
export async function getOpsProbeSummary() {
  return request<ProbeSummaryResult>('/api/v1/ops/probe/summary', {
    skipErrorHandler: true,
  });
}
