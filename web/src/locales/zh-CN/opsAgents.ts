// pages.opsAgents.* — Ops/Agents devops 会话面板（DevopsSessionsPanel）
export default {
  // 面板与状态
  'pages.opsAgents.sessions.title': '会话',
  'pages.opsAgents.sessions.refresh': '刷新',
  'pages.opsAgents.sessions.open': '开会话',
  'pages.opsAgents.sessions.empty': '当前 scope 下没有进行中的 SSH 会话。',
  'pages.opsAgents.sessions.status.connecting': '连接中',
  'pages.opsAgents.sessions.status.connected': '已连接',
  'pages.opsAgents.sessions.status.reconnecting': '重连中',
  'pages.opsAgents.sessions.status.ended': '已结束',
  'pages.opsAgents.sessions.status.error': '连接异常',
  'pages.opsAgents.sessions.openFailed': '会话打开失败',
  'pages.opsAgents.sessions.closeSuccess': '会话已关闭',
  // 会话列表列
  'pages.opsAgents.sessions.column.sessionId': '会话 ID',
  'pages.opsAgents.sessions.column.agentId': 'Agent ID',
  'pages.opsAgents.sessions.column.operator': '操作者',
  'pages.opsAgents.sessions.column.protocol': '协议',
  'pages.opsAgents.sessions.column.target': '目标',
  'pages.opsAgents.sessions.column.channel': '通道',
  'pages.opsAgents.sessions.column.createdAt': '创建时间',
  'pages.opsAgents.sessions.column.actions': '操作',
  // 行内动作
  'pages.opsAgents.sessions.action.attach': '接入',
  'pages.opsAgents.sessions.action.close': '关闭',
  'pages.opsAgents.sessions.action.closeSession': '关闭会话',
  // 开会话表单
  'pages.opsAgents.sessions.modal.title': '开 SSH 会话',
  'pages.opsAgents.sessions.modal.open': '打开',
  'pages.opsAgents.sessions.modal.cancel': '取消',
  'pages.opsAgents.sessions.modal.agentId.required': '请选择 devops agent',
  'pages.opsAgents.sessions.modal.agentId.extra':
    '仅当前 scope 内的在线 agent；打开后由服务端做属主 scope 校验。',
  'pages.opsAgents.sessions.modal.agentId.placeholder': '选择 devops agent',
  'pages.opsAgents.sessions.modal.agentId.empty': '暂无可用 devops agent',
  'pages.opsAgents.sessions.modal.host.label': '目标主机',
  'pages.opsAgents.sessions.modal.host.required': '请输入主机',
  'pages.opsAgents.sessions.modal.host.placeholder': '例如 10.0.0.11',
  'pages.opsAgents.sessions.modal.port.label': '端口',
  'pages.opsAgents.sessions.modal.port.required': '请输入端口',
  'pages.opsAgents.sessions.modal.cols.label': '列',
  'pages.opsAgents.sessions.modal.rows.label': '行',
  'pages.opsAgents.sessions.modal.protocolNote':
    '本批只交付 SSH；RDP/VNC 服务端显式拒绝，不提供选项。',
  // 关闭确认
  'pages.opsAgents.sessions.closeConfirm.title': '关闭该 SSH 会话？',
  'pages.opsAgents.sessions.closeConfirm.content':
    '关闭会结束远端会话并在审计留痕（devops.session_close）。',
  'pages.opsAgents.sessions.closeConfirm.ok': '关闭会话',
  // 会话终端抽屉
  'pages.opsAgents.sessions.drawer.title': '会话终端',
  'pages.opsAgents.sessions.drawer.footerNote':
    '关闭抽屉只断开本地视图，不结束远端会话；点「关闭会话」才真正断开（审计 devops.session_close）。接入时先补最近一屏回放；历史存档在服务端无取回面，暂不可回放。',
  // agent 下拉选项
  'pages.opsAgents.sessions.agent.offline': '{id}（离线）',
};
