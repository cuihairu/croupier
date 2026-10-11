export default {
  'menu.Ops.Capture': 'Capture Rules',

  'pages.opsCapture.gates.title': 'Gate Rules Publishing',
  'pages.opsCapture.reload': 'Reload',
  'pages.opsCapture.template': 'Insert Template',
  'pages.opsCapture.publish': 'Publish',
  'pages.opsCapture.publish.confirm':
    'Publish as a new full version and hot-deliver to capture-agents in this scope?',
  'pages.opsCapture.publish.success':
    'Version {version} published; capture-agents will hot-apply within the poll cycle',
  'pages.opsCapture.error.loadFailed': 'Failed to load gate rules',
  'pages.opsCapture.error.publishFailed': 'Publish failed',
  'pages.opsCapture.gates.keyLabel': 'Config key',
  'pages.opsCapture.gates.unpublished': 'Unpublished (agent uses file rules)',
  'pages.opsCapture.gates.version': 'Version {version}',
  'pages.opsCapture.gates.boundary':
    'The server only performs structural checks; semantic validation (table routing/thresholds) runs in full on the capture-agent at apply time, and invalid rules keep the previous version. Published does not mean applied — the source of truth is the agent log "capture gates version applied".',
  'pages.opsCapture.gates.placeholder': 'capture.gates JSON (reconcile / cep / lineage sections)',
  'pages.opsCapture.gates.help':
    'Publishing stores to the game database under the topbar game/env scope (config_versions); capture-agents pull via the upstream channel on a 30s version poll and hot-swap atomically. On disconnect or validation failure they keep running on the previous rules.',
};
