// pages.opsLBMonitor.* — Ops/LBMonitor 负载均衡监控页
export default {
  'pages.opsLBMonitor.alert.unhealthyBackends':
    '不健康后端：{backends}——TCP 会话可能仍在（半开），注意与 /ops/nodes 的归属状态对账。',
  'pages.opsLBMonitor.card.reconciliation': '归属 vs LB 对账（僵尸探测）',
  'pages.opsLBMonitor.card.reconciliationHint':
    '归属表 agent 数与 LB 会话数长期不一致（连接在、心跳停）= 半开连接信号， 结合 /ops/nodes 的「agent 自报」列定位。',
  'pages.opsLBMonitor.card.sessionsByBackend': '各后端会话分布（current_sessions）',
  'pages.opsLBMonitor.empty.noPrometheus':
    '未配置 Prometheus（ops.lbPrometheusUrl），LB 监控不可用',
  'pages.opsLBMonitor.empty.noSeriesData': '暂无数据（确认 prometheus 已抓取 haproxy /metrics）',
  'pages.opsLBMonitor.error.loadFailed': '加载 LB 监控失败',
  'pages.opsLBMonitor.filter.allBackends': '全部后端',
  'pages.opsLBMonitor.gauge.ownershipRatio': '归属 {nodes} / LB 后端 {backends}',
  'pages.opsLBMonitor.header.pipeline':
    'LB 监控（Prometheus 管道：haproxy exporter → prometheus → 平台代理） · 30s 自动刷新',
  'pages.opsLBMonitor.statistic.agentNodes': 'agent 节点（归属表）',
  'pages.opsLBMonitor.statistic.backendTotal': '后端总数',
  'pages.opsLBMonitor.statistic.totalSessions': 'LB 会话总数',
  'pages.opsLBMonitor.statistic.unhealthyBackends': '不健康后端',
};
