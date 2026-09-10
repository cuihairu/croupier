// pages.opsAnalyticsFilters.* — Ops/AnalyticsFilters 筛选器
export default {
  'pages.opsAnalyticsFilters.action.load': 'Load',
  'pages.opsAnalyticsFilters.action.save': 'Save',
  'pages.opsAnalyticsFilters.card.title': 'Sampling Control (Server Side)',
  'pages.opsAnalyticsFilters.error.loadFailed': 'Failed to load',
  'pages.opsAnalyticsFilters.error.saveFailed':
    'Failed to save (analytics:manage permission required)',
  'pages.opsAnalyticsFilters.events.allAllowed': 'All allowed',
  'pages.opsAnalyticsFilters.events.label': 'Event allowlist: ',
  'pages.opsAnalyticsFilters.events.placeholder': 'Enter allowed event names; empty = allow all',
  'pages.opsAnalyticsFilters.hint.explainTag': 'Info',
  'pages.opsAnalyticsFilters.hint.sampleRule':
    'Note: an empty event list allows all events; clearing the list (deleting all tags) drops everything. The agent drops events based on the allowlist and the sampling rate; payment reporting has its own switch.',
  'pages.opsAnalyticsFilters.hint.sampleTooltip':
    'Random sampling by percentage: 100 keeps everything, 0 drops everything.',
  'pages.opsAnalyticsFilters.payments.disabled': 'Disabled (all dropped)',
  'pages.opsAnalyticsFilters.payments.enabled': 'Reporting allowed',
  'pages.opsAnalyticsFilters.payments.label': 'Payment events: ',
  'pages.opsAnalyticsFilters.payments.stateDisabled': 'Disabled',
  'pages.opsAnalyticsFilters.payments.stateEnabled': 'Enabled',
  'pages.opsAnalyticsFilters.sample.label': 'Global sampling: ',
  'pages.opsAnalyticsFilters.select.envPlaceholder': 'Environment',
  'pages.opsAnalyticsFilters.select.gamePlaceholder': 'Game',
  'pages.opsAnalyticsFilters.success.saved': 'Saved',
  'pages.opsAnalyticsFilters.summary.current': 'Current: ',
  'pages.opsAnalyticsFilters.summary.eventsCount': '· Events',
  'pages.opsAnalyticsFilters.summary.payments': '· Payments',
  'pages.opsAnalyticsFilters.summary.sample': '· Sampling',
  'pages.opsAnalyticsFilters.warning.selectGameEnv': 'Please select a game and environment',
};
