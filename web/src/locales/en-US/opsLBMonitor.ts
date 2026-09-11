// pages.opsLBMonitor.* — Ops/LBMonitor load balancing monitor page
export default {
  'pages.opsLBMonitor.alert.unhealthyBackends':
    'Unhealthy backends: {backends}—TCP sessions may still exist (half-open); reconcile against the ownership status on /ops/nodes.',
  'pages.opsLBMonitor.card.reconciliation': 'Ownership vs LB Reconciliation (Zombie Detection)',
  'pages.opsLBMonitor.card.reconciliationHint':
    'A long-term mismatch between the ownership table agent count and LB sessions (connection alive, heartbeat stopped) signals half-open connections; locate them via the "agent reported" column on /ops/nodes.',
  'pages.opsLBMonitor.card.sessionsByBackend': 'Session Distribution by Backend (current_sessions)',
  'pages.opsLBMonitor.empty.noPrometheus':
    'Prometheus not configured (ops.lbPrometheusUrl); LB monitoring unavailable',
  'pages.opsLBMonitor.empty.noSeriesData':
    'No data yet (make sure Prometheus scrapes haproxy /metrics)',
  'pages.opsLBMonitor.error.loadFailed': 'Failed to load LB monitoring',
  'pages.opsLBMonitor.filter.allBackends': 'All backends',
  'pages.opsLBMonitor.gauge.ownershipRatio': 'Owned {nodes} / LB backends {backends}',
  'pages.opsLBMonitor.header.pipeline':
    'LB Monitoring (Prometheus pipeline: haproxy exporter → prometheus → platform proxy) · auto refresh every 30s',
  'pages.opsLBMonitor.statistic.agentNodes': 'Agent Nodes (Ownership Table)',
  'pages.opsLBMonitor.statistic.backendTotal': 'Total Backends',
  'pages.opsLBMonitor.statistic.totalSessions': 'Total LB Sessions',
  'pages.opsLBMonitor.statistic.unhealthyBackends': 'Unhealthy Backends',
};
