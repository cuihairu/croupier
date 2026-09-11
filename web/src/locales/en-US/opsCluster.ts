// pages.opsCluster.* — Ops/Cluster server member topology page
export default {
  'pages.opsCluster.badge.offline': 'Offline',
  'pages.opsCluster.badge.online': 'Online',
  'pages.opsCluster.button.refresh': 'Refresh',
  'pages.opsCluster.card.instanceList': 'Instances',
  'pages.opsCluster.column.advertiseAddr': 'Interconnect Address',
  'pages.opsCluster.column.agentConnections': 'Agent Connections',
  'pages.opsCluster.column.instance': 'Instance',
  'pages.opsCluster.column.startedAt': 'Started At',
  'pages.opsCluster.column.status': 'Status',
  'pages.opsCluster.error.loadFailed': 'Failed to load cluster info',
  'pages.opsCluster.hint.singleInstance':
    'Currently deployed as a single instance (cluster.enabled=false). See docs/architecture/server-ha-multi-instance.md to enable multi-instance HA.',
  'pages.opsCluster.statistic.agentConnections': 'Agent Connection Distribution',
  'pages.opsCluster.statistic.agentUnit': '',
  'pages.opsCluster.statistic.instanceTotal': 'Total Instances',
  'pages.opsCluster.statistic.onlineInstances': 'Online Instances',
  'pages.opsCluster.subTitle': 'Server multi-instance member topology (auto refresh every 10s)',
  'pages.opsCluster.tag.currentInstance': 'Current Instance',
  'pages.opsCluster.tooltip.leaseExpired':
    'Lease expired—the instance is down or partitioned; its agents will reconnect to a live instance automatically',
};
