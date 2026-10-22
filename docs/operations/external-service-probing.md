---
title: External Service Probing Notes
---

# External Service Probing Notes

This repository has two classes of "external services": runtime dependencies (identity sources, SMTP, webhooks, update sources, maintenance status sources) and cross-network game/infrastructure targets (Agent probes). Existing liveness capability is mostly manual testing; only `core/healthprobe` configured targets run periodically. The platform has no unified background third-party health patrol, consecutive-failure thresholds, alerting or degradation orchestration.

## External Service Inventory

### File-configured inventory

`GET /api/v1/site/external-services` returns a read-only inventory, currently covering:

- **herald** (outbound alerting): Active only when enabled, `baseUrl` and the token environment variable are all present.
- **alertInbound.alertmanager / alertInbound.generic** (inbound webhooks): Active only when the source is configured and the secret environment variable exists.
- **serverStatus.\*** (outbound maintenance status sources): Active only when enabled, the provider is registered, it is not noop, and `baseUrl` plus the optional token are present.

Evidence: `internal/api/sitesettings/external_services.go:57` mounts the endpoint, `internal/api/sitesettings/external_services.go:63` returns the inventory classes; herald gating is at `internal/api/sitesettings/external_services.go:75`, inbound webhooks at `internal/api/sitesettings/external_services.go:98`, status sources at `internal/api/sitesettings/external_services.go:121`. The response only names environment variables and whether they are set — values are never echoed (`internal/api/sitesettings/external_services.go:5`).

This endpoint checks configuration completeness only; it performs no network probing.

### Identity sources

| Dependency                     | Probed during normal use                | Manual test                                                          | Failure handling                                |
| ------------------------------ | --------------------------------------- | -------------------------------------------------------------------- | ----------------------------------------------- |
| LDAP                           | Session establishment and bind at login | TCP dial, 5 seconds                                                  | Config test returns failure; no periodic patrol |
| OIDC                           | Code exchange and UserInfo              | GET `/.well-known/openid-configuration`, 5 seconds, must be HTTP 200 | Config test returns failure; no periodic patrol |
| GitHub / WeChat / custom OAuth | Code exchange and user info             | Config connectivity test not implemented                             | Login request fails; no periodic patrol         |

The LDAP 5-second dial is at `internal/api/sitesettings/auth_test_conn.go:69`; the OIDC 5-second discovery check at `internal/api/sitesettings/auth_test_conn.go:90`, status decision at `internal/api/sitesettings/auth_test_conn.go:100`. The LDAP session implementation is in `internal/security/identity/ldap.go:43`; the custom OAuth token/userinfo flow is in `internal/security/identity/genericoauth.go:98`, where `internal/security/identity/genericoauth.go:133` uses a default `http.Client{}` with no explicit timeout.

**Not currently implemented**: OAuth/GitHub/WeChat connectivity tests, periodic identity-source patrol, failure counting and automatic disabling. **Plan**: unify an identity-source Probe contract — LDAP bind, OIDC discovery, OAuth read-only endpoint or HEAD; failures enter alerting but automatic login degradation must be explicitly configured.

### SMTP

Two kinds of operation:

- Side-effect-free liveness: TCP dial, SMTP greeting, EHLO, NOOP, QUIT — no authentication, no mail; 10-second default. `internal/security/secguard/retry_probe.go:104` defines the flow, `internal/security/secguard/retry_probe.go:113` sets the dial timeout, and with SSRF protection on, `internal/security/secguard/retry_probe.go:114` attaches dial validation.
- Real test mail: sends per the L3 SMTP config with a 10-second dial default; `internal/api/sitesettings/testemail.go:31` is the entry, `internal/api/sitesettings/testemail.go:47` refuses fake success when the host is unconfigured, actual sending is at `internal/platform/approvals/notification.go:432` with the 10-second dial at `internal/platform/approvals/notification.go:449`.

The manual liveness entry is `POST /api/v1/ops/probes/smtp` (`internal/api/ops/probe.go:24`). SMTP transport/auth failures only fail the current call; no periodic probing, no retry, no degradation.

### Notification webhooks and update sources

The manual probe supports six channels: DingTalk, WeCom, Feishu, generic webhook, update source and SMTP. `internal/api/ops/probe.go:14` defines the channels; HTTP targets use GET and treat status codes below 400 as healthy; `internal/security/secguard/retry_probe.go:70` caps redirects at 3 hops and disconnects after reading 1KB; `internal/security/secguard/retry_probe.go:80` uses the `secguard` timeout or a 10-second default. Unconfigured channels return `configured=false` (`internal/api/ops/probe.go:25`); unknown channels 404.

Real outbound calls differ:

- **Notification webhooks**: POST JSON with an HMAC signature (`internal/platform/approvals/notification.go:731`); the overall budget defaults to 10 seconds or `net.requestTimeoutMs`, retried with exponential backoff via `secguard.DoWithRetry` on 5xx/network errors (`internal/platform/approvals/notification.go:756`). 4xx is never retried (`internal/security/secguard/retry_probe.go:43`).
- **Update sources**: GET is idempotent and retryable; 5-second default or `net.requestTimeoutMs`, retried on network errors/5xx (`internal/api/ops/systeminfo.go:116`). Unconfigured sources make no outbound call (`internal/api/ops/systeminfo.go:89`); failure only annotates the result and never blocks operation.

Frequency and alerting: **not currently implemented** — no periodic probes, failure thresholds, recovery windows or alerts. **Plan**: register the six channels as external probe targets, reusing the healthprobe window state machine and alert lifecycle, with per-channel interval, failure threshold and recovery threshold; periodic webhook probing must still use side-effect-free read-only endpoints, never the business POST notification.

### Outbound security and call policy

- `sec.*` parsing: `internal/security/secguard/secguard.go:34` holds allowed ports/IPs/domains, the SSRF switch and the three `net.*` keys; `internal/security/secguard/secguard.go:48` parses from layered settings.
- Timeout: a positive `net.requestTimeoutMs` overrides caller defaults (`internal/security/secguard/secguard.go:184`).
- Retry: `net.maxRetries` range 0–10 (`internal/security/secguard/secguard.go:192`); `net.retryBackoffMs` defaults to 500 milliseconds (`internal/security/secguard/secguard.go:207`). Exponential backoff is at `internal/security/secguard/retry_probe.go:54`, sharing the caller's context budget.
- Note: probes and update sources are idempotent GETs and retryable; notification POSTs are at-least-once and the receiver must dedupe itself.

### Maintenance status sources

Status sources exist for maintenance-window suppression of alert outlets, not as periodic health probes.

Timeout: `serverStatus.providers.<provider>.timeoutMs`, 3-second default (`internal/config/config.go:146`, `internal/platform/serverstatus/rest.go:34`). The cache only caches successful results, TTL defaults to 60 seconds, errors are never cached (`internal/platform/serverstatus/cache.go:8`); config defaults at `internal/config/config.go:133`. On query failure, timeout or missing mapping the gate fails open: alerting continues with `source_unknown` marked — alerts are never silently swallowed (`internal/platform/serverstatus/gate.go:49`); assembly config at `internal/svc/serverstatus_gate.go:63`.

Failure degradation is simply "alert as usual plus a source-unknown mark"; there is no automatic removal or window extension on status-source failure.

## Game/Infrastructure Target Probing

`probe.targets[]` is the only generic periodic probing model, executed by the Agent or the server:

- Interval defaults to 30 seconds (`core/healthprobe/targets.go:10`); a window opens after 3 consecutive failures and closes after 1 consecutive success (`core/healthprobe/probe.go:95`).
- HTTP defaults to 5 seconds, accepting 2xx/3xx (`core/healthprobe/healthprobe.go:42`); TCP defaults to 2 seconds (`core/healthprobe/targets.go:74`); DB defaults to 5 seconds with Ping plus a read-only query (`core/healthprobe/db.go:17`); protocol allows 5 seconds per step and 2 seconds to connect (`core/healthprobe/protocol.go:24`).
- The anomaly sampling floor is 5 seconds (`core/healthprobe/policy.go:29`); policy computes the interval from target, time of day and window state (`core/healthprobe/policy.go:150`, `core/healthprobe/policy.go:192`).
- Window events go upstream immediately; failure details default to 10 per window (`internal/app/agent/probe_manager.go:42`). Normal statistics ride the 30-second `MetricsReport` (`internal/app/agent/upstream.go:586`).

Failure actions are limited to the state machine opening a window, alert persistence and notification outlets; probes never restart processes, shift traffic or modify config. That boundary is stated at `internal/app/agent/probe_manager.go:25`.

### Game protocol checkers

`type: protocol` supports game protocol adapters in addition to declarative private-protocol handshakes (`steps`): an adapter implements the `healthprobe.Checker` interface (`core/healthprobe/probe.go:45`, with optional `Detailing` stage details at `core/healthprobe/probe.go:47`) and registers through `RegisterGameProtocol` (`core/healthprobe/protocol.go:317`; double registration is an assembly error). The built-in adapter today is Minecraft Java SLP: `core/healthprobe/games/minecraft/slp.go:50` defines `SLPChecker`, and `core/healthprobe/games/minecraft/slp.go:69` registers it as the `minecraft` protocol in the package init; the registration is wired into Agent assembly by the blank import at `internal/app/agent/probe_protocols.go:8`. `core/healthprobe/targets.go:123` rejects unregistered protocol names at config-validation time and lists the registered set.

## Standalone synthetics Agent

`synthetics-agent` is a standalone probing process for cross-network targets, deployed separately from the sidecar Agent: the entry `cmd/synthetics-agent/main.go:15` reads `configs/synthetics-agent.yaml` by default (see `configs/synthetics-agent.example.yaml`), and `:27` hosts the runner under `supervisable.Main` with a heartbeat file.

### Configuration and targets

- Identity and uplink: `agents/synthetics/config/config.go:28` defines `id/serverAddr/gameId/env/heartbeatFile`; the probe cycle `synthetics.interval` must be positive (`agents/synthetics/config/config.go:260`).
- Target model: the `Target` at `agents/synthetics/config/config.go:179` supports only the two native types `http` and `icmp` (`agents/synthetics/config/config.go:334`; http without `url` is rejected at `:305`, icmp without `address` at `:331`). http targets support `method`, `expectedStatus` and `bodyContains` assertions; icmp targets require `count` successful echoes (`agents/synthetics/config/config.go:202`).
- Ownership marking: `targetCategory` (game / third_party) is a required ownership class that travels with every durable result so operators never guess who a target belongs to (`agents/synthetics/config/config.go:206`).

### Plugin probing and degradation

- The plugin host only hosts external probe programs: the `PluginConfig` at `agents/synthetics/config/config.go:47` requires commands to be declared explicitly in config and referenced by name from a target before anything launches (`:48` — nothing is launched otherwise).
- Three layers of switches and backpressure: per-plugin `Enabled` (false keeps the plugin process from ever launching and degrades plugin targets to the native HTTP probe with an explicit `plugin_disabled` cause); the runtime master switch `pluginsEnabled` (`agents/synthetics/config/config.go:107`); `pluginMaxConcurrent` caps concurrency and callers queue at most `pluginQueueWindow` before rejection (`agents/synthetics/config/config.go:111`); plugin crashes back off exponentially via `pluginBackoff/pluginBackoffMax`, and `pluginBreakAfter` consecutive failures open a `pluginOpenWindow` circuit that degrades without touching the subprocess (`agents/synthetics/config/config.go:113`).
- Routing: targets with a `plugin` field go through the plugin runtime (`Check` at `agents/synthetics/pluginhost/controller.go:86`); the HTTP fields stay mandatory — they are the fallback probe parameters when the plugin side degrades. The native probe and the fallback are at `agents/synthetics/runner/runner.go:129` and `agents/synthetics/runner/runner.go:168`.

### Result uplink

Each probe result is packed into an `opsv1.AlertEvent` (`agents/synthetics/runner/runner.go:190`) and sent up the control link; the server dispatches by Agent identity (`internal/server/control_handler.go:449`, `internal/server/control_handler.go:451`). As with probe.targets: failure actions are only alert persistence and notification outlets — no automatic disposition.

## Gaps and Plans

| Gap                             | Current state                                                                                                                                       | Plan                                                                                      |
| ------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------- |
| Third-party periodic probing    | synthetics-agent already covers http/icmp periodic probing; identity sources and notification channels are still not registered as periodic targets | Categorize targets and wire into healthprobe and the alert lifecycle                      |
| Identity-source health          | Only manual LDAP/OIDC tests                                                                                                                         | Unified Probe contract, failure alerting, optional degradation switch                     |
| OAuth outbound timeout          | Custom OAuth userinfo has no explicit timeout                                                                                                       | Converge on the guarded HTTP client and a unified timeout                                 |
| Service catalog / status page   | The file-configured inventory lacks Redis, object storage, herald connectivity                                                                      | Extend inventory items and bind probe targets                                             |
| Alert/degradation orchestration | Third-party failures only return the call result                                                                                                    | Consecutive-failure windows, alerts; auto-degradation requires approval and observability |

## Minimal Operating Advice

1. Enable `sec.ssrfProtection` in production and configure `sec.allowDomains`/`sec.allowIPs`/`sec.allowPorts` explicitly instead of running the guard fully open.
2. Set a business cap for `net.requestTimeoutMs`; keep `net.maxRetries` small, especially never amplifying against non-idempotent webhook endpoints.
3. Before wiring periodic webhook probing, provide a `/health` or dedicated read-only probe on the receiver side — never use the business POST endpoint as a health check.
4. When deploying status sources, confirm `timeoutMs`, `cacheTtlSeconds` and the token environment variable; the fail-open semantics mean a broken status source increases false positives instead of suppressing alerts.
5. Record the failure reason for every manual probe call; the response carries no history window and cannot substitute for alerting.
