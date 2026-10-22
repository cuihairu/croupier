export default {
  'pages.riskControl.title': 'Risk Strategies',
  'pages.riskControl.reload': 'Reload',
  'pages.riskControl.rollback': 'Rollback',
  'pages.riskControl.rollback.confirm': 'Republish this historical version as a new revision?',
  'pages.riskControl.rollback.success':
    'New revision published from version {version}; capture-agents will hot-apply within the poll cycle',
  'pages.riskControl.error.loadFailed': 'Failed to load strategy versions',
  'pages.riskControl.error.rollbackFailed': 'Rollback failed',
  'pages.riskControl.column.version': 'Version',
  'pages.riskControl.column.message': 'Message',
  'pages.riskControl.column.author': 'Published by',
  'pages.riskControl.column.time': 'Published at',
  'pages.riskControl.column.action': 'Actions',
  'pages.riskControl.current': 'Current',
  'pages.riskControl.unpublished': 'Unpublished (agent uses file rules)',
  'pages.riskControl.empty': 'No strategy version published in this scope',
  'pages.riskControl.boundary':
    'Rollback never rewrites history: the selected version content is copied and published as a new revision, hot-applied by the regular poll. Published does not mean applied — the reconciliation row above is agent-reported (30s poll + re-register lag); the agent log "capture gates version applied" remains the per-apply source of truth.',
};
