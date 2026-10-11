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
  'pages.opsCapture.validate': 'Validate',
  'pages.opsCapture.validate.pass':
    'Validation passed (same implementation the capture-agent applies): reconcile {reconcile} · off-hours {cepOffHours} · burst {cepBurst} · probing {cepProbing} · lineage {lineage} · event filter {eventOps}',
  'pages.opsCapture.validate.failed': 'Validation failed',
  'pages.opsCapture.error.loadFailed': 'Failed to load gate rules',
  'pages.opsCapture.error.publishFailed': 'Publish failed',
  'pages.opsCapture.gates.keyLabel': 'Config key',
  'pages.opsCapture.gates.unpublished': 'Unpublished (agent uses file rules)',
  'pages.opsCapture.gates.version': 'Version {version}',
  'pages.opsCapture.gates.boundary':
    'The "Validate" button runs a semantic pre-check with the same implementation the capture-agent applies (lint + table routing uniqueness/thresholds/event filter via config.ValidateGates); publishing itself still only performs structural checks, and semantic issues in unvalidated publishes are caught by agents keeping the previous version. Published does not mean applied — the reconciliation row above is agent-reported (30s poll + re-register lag); the agent log "capture gates version applied" remains the per-apply source of truth.',
  'pages.opsCapture.gates.placeholder': 'capture.gates JSON (reconcile / cep / lineage sections)',
  'pages.opsCapture.gates.help':
    'Publishing stores to the game database under the topbar game/env scope (config_versions); capture-agents pull via the upstream channel on a 30s version poll and hot-swap atomically. On disconnect or validation failure they keep running on the previous rules.',
  'pages.opsCapture.apply.summary': 'Apply reconciliation (agent-reported): ',
  'pages.opsCapture.apply.noAgents': 'No online capture-agent in scope',
  'pages.opsCapture.apply.allApplied': '{count}/{total} applied',
  'pages.opsCapture.apply.partial': '{count}/{total} applied (others not yet reported)',
  'pages.opsCapture.apply.unknown': 'not reported',
};
