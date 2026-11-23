// pages.opsAgents.* — Ops/Agents devops session panel (DevopsSessionsPanel)
export default {
  // Panel and status
  'pages.opsAgents.sessions.title': 'Sessions',
  'pages.opsAgents.sessions.refresh': 'Refresh',
  'pages.opsAgents.sessions.open': 'New session',
  'pages.opsAgents.sessions.empty': 'No in-progress SSH sessions in the current scope.',
  'pages.opsAgents.sessions.status.connecting': 'Connecting',
  'pages.opsAgents.sessions.status.connected': 'Connected',
  'pages.opsAgents.sessions.status.reconnecting': 'Reconnecting',
  'pages.opsAgents.sessions.status.ended': 'Ended',
  'pages.opsAgents.sessions.status.error': 'Connection error',
  'pages.opsAgents.sessions.openFailed': 'Failed to open the session',
  'pages.opsAgents.sessions.closeSuccess': 'Session closed',
  // Session list columns
  'pages.opsAgents.sessions.column.sessionId': 'Session ID',
  'pages.opsAgents.sessions.column.agentId': 'Agent ID',
  'pages.opsAgents.sessions.column.operator': 'Operator',
  'pages.opsAgents.sessions.column.protocol': 'Protocol',
  'pages.opsAgents.sessions.column.target': 'Target',
  'pages.opsAgents.sessions.column.channel': 'Channel',
  'pages.opsAgents.sessions.column.createdAt': 'Created at',
  'pages.opsAgents.sessions.column.actions': 'Actions',
  // Row actions
  'pages.opsAgents.sessions.action.attach': 'Attach',
  'pages.opsAgents.sessions.action.close': 'Close',
  'pages.opsAgents.sessions.action.closeSession': 'Close session',
  // Open-session form
  'pages.opsAgents.sessions.modal.title': 'Open SSH session',
  'pages.opsAgents.sessions.modal.open': 'Open',
  'pages.opsAgents.sessions.modal.cancel': 'Cancel',
  'pages.opsAgents.sessions.modal.agentId.required': 'Please select a devops agent',
  'pages.opsAgents.sessions.modal.agentId.extra':
    'Only online agents in the current scope; the server validates ownership scope on open.',
  'pages.opsAgents.sessions.modal.agentId.placeholder': 'Select a devops agent',
  'pages.opsAgents.sessions.modal.agentId.empty': 'No available devops agent',
  'pages.opsAgents.sessions.modal.host.label': 'Target host',
  'pages.opsAgents.sessions.modal.host.required': 'Please enter a host',
  'pages.opsAgents.sessions.modal.host.placeholder': 'e.g. 10.0.0.11',
  'pages.opsAgents.sessions.modal.port.label': 'Port',
  'pages.opsAgents.sessions.modal.port.required': 'Please enter a port',
  'pages.opsAgents.sessions.modal.cols.label': 'Cols',
  'pages.opsAgents.sessions.modal.rows.label': 'Rows',
  'pages.opsAgents.sessions.modal.protocolNote':
    'This batch ships SSH only; RDP/VNC are explicitly rejected by the server and not offered.',
  // Close confirm
  'pages.opsAgents.sessions.closeConfirm.title': 'Close this SSH session?',
  'pages.opsAgents.sessions.closeConfirm.content':
    'Closing ends the remote session and leaves an audit record (devops.session_close).',
  'pages.opsAgents.sessions.closeConfirm.ok': 'Close session',
  // Session terminal drawer
  'pages.opsAgents.sessions.drawer.title': 'Session terminal',
  'pages.opsAgents.sessions.drawer.footerNote':
    'Closing the drawer only detaches the local view and does not end the remote session; use "Close session" to actually disconnect (audit devops.session_close). Attach first loads the latest screen; history archives have no server retrieval surface and are not replayable.',
  // Agent dropdown option
  'pages.opsAgents.sessions.agent.offline': '{id} (offline)',
};
