// pages.opsCluster.* — Ops/Cluster 集群成员拓扑页
export default {
  'pages.opsCluster.badge.offline': '离线',
  'pages.opsCluster.badge.online': '在线',
  'pages.opsCluster.button.refresh': '刷新',
  'pages.opsCluster.card.instanceList': '实例列表',
  'pages.opsCluster.column.advertiseAddr': '互联地址',
  'pages.opsCluster.column.agentConnections': 'Agent 连接',
  'pages.opsCluster.column.instance': '实例',
  'pages.opsCluster.column.startedAt': '启动时间',
  'pages.opsCluster.column.status': '状态',
  'pages.opsCluster.error.loadFailed': '加载集群信息失败',
  'pages.opsCluster.hint.singleInstance':
    '当前为单实例部署（cluster.enabled=false）。开启多实例高可用见 docs/architecture/server-ha-multi-instance.md。',
  'pages.opsCluster.statistic.agentConnections': 'Agent 连接分布',
  'pages.opsCluster.statistic.agentUnit': '个',
  'pages.opsCluster.statistic.instanceTotal': '实例总数',
  'pages.opsCluster.statistic.onlineInstances': '在线实例',
  'pages.opsCluster.subTitle': 'Server 多实例成员拓扑（10s 自动刷新）',
  'pages.opsCluster.tag.currentInstance': '当前实例',
  'pages.opsCluster.tooltip.leaseExpired':
    '租约过期——实例宕机或网络分区，其名下 Agent 将自动重连到存活实例',
};
