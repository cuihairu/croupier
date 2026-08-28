import { request } from '@umijs/max';

// LB 监控（docs/operations/load-balancing.md「LB 监控」）
// 数据源：ops.prometheusUrl / CROUPIER_LB_PROMETHEUS_URL；未配置时集群
// 响应不带 lbStats，本页隐藏。

export type ClusterLbStatsInfo = {
  enabled: boolean;
  queryUrl: string;
};

export type LbStatsQueryRequest = {
  query: string;
};

// Prometheus /api/v1/query 裁剪结果
export type LbStatsQueryResult = {
  status: string;
  data: {
    resultType: string;
    result: {
      metric: Record<string, string>;
      value: [number, string];
    }[];
  };
};

export async function queryLbStats(data: LbStatsQueryRequest) {
  return request<LbStatsQueryResult>('/api/v1/ops/cluster/lb-stats', {
    method: 'POST',
    data,
  });
}
