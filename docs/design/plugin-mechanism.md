---
title: Plugin Mechanism Formal Design — Extension-Point Panorama, External-Service Integration Spec, and herald Integration (design profile, approved by delegated decision 2026-10-16)
---

# Plugin Mechanism Formal Design (Plugin × Integration × Alert Outlets)

## Status

- Status: **approved by delegated decision (2026-10-16, FW1016D patrol ticket clause 2 authorizing delegated scheduling and decisions)**; originally filed by the 2026-10-10 design-supplement order (zero code review), merged the same day with LAN asset integration — §3.8; §8's phases are executing under this approval (M1–M4 delivered, M5 awaiting its trigger).
  - **Delegated scope**: the 12 decisions are finalized per the "decision summary" recommendations; the M4 integration template moved from Draft to final. **The user may overturn any item afterward**; an overturn rolls the corresponding § back accordingly.
  - **Delegation rationale**: ① all 12 items already carried recommended options and trigger conditions — the delegation introduces no new technical judgment; ② decisions ①②④⑥⑪ (default no-op with zero outbound / one mechanism, no second registry / no dynamic code loading / detector v1 closed to third-party rules / destructive infrastructure actions forced through approval + audit) are all **tightening moves** — tightening is safer than loosening; ③ leaving them pending would freeze the M5 and detector-pluginization trigger evaluations too — the freeze costs more than deciding.
- The supplement order's three items in one document: ① the plugin mechanism's formal design (§2/§3/§6/§7); ② the external-service integration spec (§4 + Appendix A); ③ the herald integration spec (§5 + Appendix B); plus ④ the devops-agent LAN asset integration approach (§3.8, cockpit-first + provider fallback).
- Design inputs: the [plugin architecture survey](../research/plugin-architecture-survey.md) (Grafana/VS Code/WordPress/VitePress, six-question rubric), the Provider plugin design (#66, P0 landed), the extension installation model (five tables, four states), the official-extension unified pattern, the herald integration profile, and the [Agent inventory](agents-inventory.md).
- Stance order (2026-10-10, outlet Providerization + notification strengthening, merged into this design): ① alert outlets become Providers — the `notify(event) -> {delivered, channel, retry}` contract + registry + a config-driven degradation order (the primary outlet failing slides to the next; the gateway fallback is isomorphic); herald is only the default external outlet option, never hard-wired; ② **the default no-op first principle** — all external-service Providers (alert outlets / status sources / vendor calls) default to a **real no-op implementation in the registry** (same interface, not if-statements scattered through business code); zero config = zero outbound (the data-exfiltration-disabled-by-default red line); herald/atlas are both options that take effect only when explicitly added and enabled in config, addable/removable/swappable, with a switch = changing a registry entry and zero business-code change; ③ the alert default = in-site notification (built-in and on by default, reachable with zero config, zero outbound, riding the existing notification system's unread/read/persistence); external outlets are an optional Provider chain that degrades back in-site on failure; ④ in-site notification strengthening — level(info|warn|critical)/type/source/ref fields + urgency ordering + repeat-trigger escalation with dedup (§5.1).
- Decision summary (finalized per the recommended options, for post-decision review):
  1. **The default no-op first principle (zero config = zero outbound)**: all external-service Providers (alert outlets / status sources / vendor calls) default to a **real no-op implementation in the registry** (same interface, not if-checks scattered in business code) — zero config means zero outbound, honoring the data-exfiltration-disabled-by-default red line. herald/atlas are both **options**: they take effect only when explicitly added and enabled in config, and can be added, removed, or swapped; a switch = changing a registry entry with zero business-code change; the config page lists "enabled external services", each with an enable switch.
  2. **One mechanism, not two**: the plugin mechanism = the existing extension five tables (catalog/release/installation/binding/events) + the #66 driver layer; this design only does panorama placement and incremental contracts — **no new registry, no second manifest format**.
  3. **Four trust tiers** (§2): compile-time built-in → official extension packs → third-party extension packs → external services (zero code). Untrusted third-party code **never enters the process** — it integrates as an external service, with no dynamic code loading (following #66 ruling ①).
  4. **No subprocess/gRPC sandbox**: in-process + three-tier RBAC capabilities + the secguard outbound guard + secrets references are the isolation boundary (survey §7.3's threat model does not hold here).
  5. **Seven closed-set extension point classes to start** (§3): external platform calls / CDC data sources / detector rules / alert outlets / alert inlets / dashboard pages & cards / agent capabilities; a new extension point must pass §7.2's "six questions" checklist before entering the table.
  6. **Detector v1 is built-in parameterized; third-party rule plugins are not opened** (decision ⑥): the capture three gates / devops rules are template + parameters; rule-engine pluginization keeps a reserved slot (§3.3).
  7. **The alert-outlet Provider chain** (§5.1): in-site notification = the built-in default outlet (always present, reachable with zero config, zero outbound, riding the existing notification system's unread/read/persistence) → external outlets = an optional Provider chain (the notify contract + registry + degradation order: the primary outlet failing slides to the next; herald is the default external option, with generic webhook/email joining through the same interface). The three Provider classes (alert outlet / status source / vendor) are **isomorphic under one mechanism** — not three separate patterns.
  8. **Alert-inlet webhook v1 closed set, two formats** (§5.3): Alertmanager-compatible + generic JSON; HMAC signatures + timestamp anti-replay + `eventId` idempotency, landing in the existing alert page (silence/claim/escalation fully reused).
  9. **Config hot-reload on dual tracks** (§6.4): server-side installation config PUT + reconcile rebuilding instances; agent-side core/configsync version polling (the extension_sync_puller precedent).
  10. **LAN asset integration is cockpit-first** (§3.8): for devops-agent to operate LAN PVE/BMC/vSphere assets, cockpit is preferred as the external-service adaptation (an L4 config instance over unified REST; croupier is only a client and does not re-wrap bmclib/bpg-proxmox-api/govmomi); when cockpit is undeployed/unreachable, fall back to the provider-plugin slot with a local thin wrapper of the same libraries for direct connection (an L1 driver, designed as same-lineage code decoupling — a single source of truth for the operation contract).
  11. **All destructive infrastructure actions go through approval + audit** (§3.8): BMC power-on, PVE snapshot rollback, test-environment cloning, etc. force a second confirmation (reusing the approval flow) + execlog audit.
  12. **Interface naming aligns with the cockpit design profile** (§3.8): the operation closed set is `<asset>.<action>` (e.g. `bmc.power_on` / `pve.snapshot_rollback` / `vsphere.vm_clone`); croupier does not invent a second naming scheme.

## 1. Positioning: A Repo-Wide Definition of "Plugin"

**Current-state facts** (as of 2026-10-10): the extension system already runs (five tables + the `official.<domain>` unified pattern + pack import + agent sync); #66's driver layer P0 has landed (the `internal/drivers` contract + the Registry disposition + the manifest provider block finalized); the cicd/identity built-in factory registries exist; the herald outlet profile is filed. **What is missing is not a mechanism but a panorama map and an integration spec** — this design supplies both and constrains future extension points from each inventing their own.

| Term            | Definition                                                                                                           |
| --------------- | -------------------------------------------------------------------------------------------------------------------- |
| Host            | The process that loads plugins: the croupier server / sidecar-agent / capture-agent                                  |
| Extension point | A pluggable slot the host leaves: a stable contract + a registration method + failure semantics (§3's seven classes) |
| Plugin          | A capability unit attached to an extension point, in one of four forms (§2's tiers)                                  |
| Driver          | A compile-time built-in implementation of one invocation protocol (openapi/webhook, #66 §3.1)                        |
| Provider        | A configured instance of a driver (carried by installation + runtime binding)                                        |
| Binding         | An `extension_runtime_bindings` row: a runtime mount produced by installation (function/provider/page/…)             |

## 2. The Four-Tier Plugin Taxonomy (Built-in vs Third-Party)

The criterion is the **trust boundary**, not technical flair — the trust tier determines the admission route and contract strength:

| Tier                           | Form                                               | Admission                              | Contract                                                                       | Isolation boundary                                                                 | Version negotiation                                                             | Precedent                                                                |
| ------------------------------ | -------------------------------------------------- | -------------------------------------- | ------------------------------------------------------------------------------ | ---------------------------------------------------------------------------------- | ------------------------------------------------------------------------------- | ------------------------------------------------------------------------ |
| L1 compile-time built-in       | Go code in the binary                              | Code review (PR)                       | Go interface + `init()` self-registering factory table                         | Same process; RBAC gates the invocation surface                                    | Ships with platform releases (the driver closed set has no independent version) | `internal/cicd`, `internal/drivers`, `internal/security/identity`        |
| L2 official extension packs    | `official.*` seeds (manifest into the five tables) | In-repo seed review                    | Release manifest + the unified pattern's three-tier permissions + configSchema | In-process + secguard + RBAC                                                       | `min_core_version` + the dependency graph                                       | notification/alerting/approval/backup-advanced                           |
| L3 third-party extension packs | pack `.tgz` import                                 | Admin import + installation approval   | The L2 contract (isomorphic validation, zero exemptions)                       | Same as L2; checksum + release dedup                                               | Same as L2                                                                      | The pack import chain (contract baseline §7.3)                           |
| L4 external service            | Zero code; instantiated by config                  | Admin configuration (capability-gated) | §4's integration surface (endpoint/credentials/protocol closed set)            | Network boundary: secguard guards outbound + secrets references never in plaintext | The service may release autonomously (the contract = HTTP semantics)            | Provider config instances (#66 §3.1, isomorphic to a Grafana datasource) |

**Core ruling**: plugin = "configured instance" over "executable package". Untrusted third-party code has no channel into the process (no `.so`/subprocess/gRPC plugin loading); capability extension relies on L1 platform releases + L2/L3 declarative extensions + L4 external services. Compared with Grafana backend plugins (subprocess gRPC) and VS Code (a shared host process): we take WordPress's trust model ("what an administrator can install" is the boundary) + Grafana's contract discipline (capability declaration + version negotiation), and take neither's runtime loading.

## 3. Extension-Point Panorama (What Can Plug In)

Every extension point has five elements: **contract surface / registration method / lifecycle ownership / config hot-reload / failure disposition**. Seven classes to start:

### 3.1 External Platform Calls (driver/provider) — #66's Territory

- Contract surface: `drivers.Driver` (Kind/Call/Close, JSON passthrough) + the manifest provider block (type/operations/permissions, finalized in P0).
- Registration: compile-time `drivers.Register` + runtime bindings (the provider kind). Lifecycle/hot-reload/failure semantics all follow #66 §6; this design does not redefine them.
- Status: **P0 landed**; the openapi driver moved in with P1.

### 3.2 CDC Data Sources (capture source)

- Contract surface: `agents/capture/source.Provider` (Open/Bookmark/Close) + the unified ChangeEvent contract (11 fields).
- Registration: compile-time built-in (L1) — MySQL landed; PG arrives with C3; the source closed set expands with capture batches. **Third-party source plugins are not opened**: CDC sources need REPLICATION privileges and position semantics, where a misconfiguration silently drops events — not admitted to L3/L4.
- Hot-reload: a table-allowlist change = reopen the source (position resumption loses nothing); a connection-parameter change = process restart (supervisor-launched).
- Failure disposition: connection failures back off and reopen; corrupted positions fail-safe-exit and hand off to ops (implemented).
- Status: MySQL landed (the capture batch); PG with C3; the MongoDB CDC connector delivered with #4310 (2026-10-16); the source closed set stays closed to third-party source plugins.

### 3.3 Detectors / Rules (the capture three gates, devops rules)

- Contract surface (fixed at C2's landing): a rule = a template closed set (`lineage.*`/`reconcile.*`/`cep.*`) + a parameter surface (panel-configured); evaluated per event, hit details feeding the alert pipeline.
- **v1 is built-in parameterized; third-party rule plugins are not opened** (decision ⑥): rule correctness directly determines the false-positive rate, and third-party rules have no audit or testing handle; engine pluginization (e.g. allowing custom CEP expressions) keeps a reserved slot — trigger = ≥2 game vendors demanding to bring their own rules.
- Status: design finalized (capture profile §4); implementation with C2.

### 3.4 Alert Outlets

- Contract surface: the `Outlet` interface slot (§5.1) — `Deliver(ctx, AlertEvent) (DeliveryOutcome, error)`, with AlertEvent as §5.2's unified envelope; the receipt `{delivered, channel, retry}` feeds degradation-chain decisions and failure counters.
- Registration: a compile-time factory registry + server-side config selecting enabled outlets and the degradation order; **the default no-op first principle** (decision ①): with no external outlet configured, the chain = in-site only (the built-in default outlet, zero outbound); herald is the default external option, with generic webhook/email joining through the same interface.
- Status: **landed (2026-10-10, M2)** — `internal/platform/outlet` (Outlet + Manager + HeraldOutlet); the event-source hook is `MetricsStore.SetOnSupervisorEvent`; the config section is `herald:` (the herald profile §8 lists implementation-vs-profile differences); **Providerization + the degradation chain + notification tiering are increments of the 2026-10-10 stance order** (§5.1 revised; implementation batch §8 M6).

### 3.5 Alert Inlets (webhook form)

- Contract surface: §5.3's inbound webhook contract (HMAC + idempotency + a mapping table).
- Registration: config-driven source mapping (the v1 closed set: Alertmanager + generic JSON), landing in the existing alert page.
- Status: **landed (2026-10-10, M3)** — the `internal/api/alert` inbound handler (`POST /api/v1/alerts/inbound/{source}`, a public route following the cicd webhook precedent; HMAC-SHA256 + a ±5-minute anti-replay window + eventId idempotent upsert into the `alerts` table) + the `alertInbound:` config section (source closed set × secretEnv environment-variable references × config-driven label mapping). Implementation-vs-design differences in the todo.md M3 delivery record.

### 3.6 Dashboard Pages and Cards

- Contract surface: **the existing system; no new mechanism** — `binding_type=page/capability` (runtime binding) + the PageSpec/ComponentTemplate publishing chain (dashboard-page-model). An extension page = a page binding derived at installation; a card component = a ComponentTemplate entry.
- Boundary: third-party packs provide page declarations (declarative PageSpec), **not executable frontend code** — the page rendering host is the croupier web, and the component closed set is bounded by the editor's supported surface (composite-editor v3's capability domain).
- Status: landed (the five-batch chain); this design only places it.

### 3.7 Agent Capabilities (the function surface / managed processes)

- Contract surface: the function registration chain (FunctionDescriptor + pack) + the `managedProcesses` config (supervisor S1/S2 delivered).
- Boundary: core-family agents (devops/capture) **lacking a function surface is positioning, not a defect** (agent-core §9); the function surface belongs to the game-server SDK ecosystem carried by sidecar-agent.
- Status: landed; placement noted.

### 3.8 LAN Infrastructure Assets (cockpit as an external service + provider fallback)

The devops-agent must be able to operate LAN PVE/BMC/vSphere assets (the infrastructure action steps of deployment pipelines). Two integration tiers, **cockpit first, provider fallback**:

- **① Preferred: cockpit as the external-service adaptation** (an L4 config instance, zero code). The capability provider is cockpit's hypervisor/BMC provider layer (unified REST); croupier is only a client plugin (openapi driver + provider binding) and **does not re-wrap** bmclib / bpg-proxmox-api / govmomi — the maintenance of those libraries (versions, credential rotation, BMC firmware variance) belongs to cockpit. Zero new dependencies on the croupier side.
- **② Circuit-breaker fallback: a local thin wrapper at the provider-plugin slot** (an L1 driver). When cockpit is undeployed/unreachable, connect to the assets directly: the driver is a thin wrapper over the same libraries (bmclib/govmomi/proxmox-api), designed as same-lineage code decoupled from the cockpit client — the two implementations evolve independently but share a **single source of truth for the operation contract** (the same operations closed set, below); when cockpit is reachable, config switches back to the L4 path with identical behavior.
- **③ Scenarios and destructive discipline**: action steps = physical-host BMC power (`bmc.*`), PVE snapshot rollback (`pve.*`), test-environment cloning (`vsphere.*`/`pve.*`). All destructive operations **force a second confirmation** (reusing the approval flow, the approval batch precedent) + **execlog audit** (records through the existing execlog chain: operator/time/params/result in full).
- **④ Interface naming alignment**: operation naming follows the cockpit design profile, with the closed set `<asset>.<action>` (starting `bmc.power_on` / `bmc.power_off` / `bmc.power_cycle` / `pve.snapshot_rollback` / `pve.vm_clone` / `vsphere.vm_clone`, etc.); the croupier side (manifest provider block operations, panel buttons, approval tickets) references this naming uniformly — no second scheme is invented.
- Contract surface: §4.1's standard integration surface (endpoint = the cockpit REST baseURL, credentials = a cockpit token secret reference, protocol = http-rest, permissions = `infrastructure.operate` to start).
- Status: this design finalizes the integration approach and the two-tier path; the cockpit profile's filing and the fallback driver are implementation batches (§8).
- Boundary: the cockpit design profile **is not yet filed in this repo** (parallel/pending charter) — §3.8's naming alignment is subject to two-way calibration once that profile lands (§9).

## 4. External-Service Integration Spec (The Vendor Self-Service Integration Surface)

### 4.1 The Standard Integration Surface's Five Elements

External services (SMS gateways / risk APIs / data platforms / ticketing systems…) integrated by croupier all answer the same table:

| Element         | Contract                                                                                                                  | Notes                                                                           |
| --------------- | ------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------- |
| Endpoint        | `baseURL` (http/https, inside the secguard outbound-guard allowlist)                                                      | One base per service; operation paths derived by the driver protocol            |
| Credentials     | `token` → a secret_refs reference (**never in plaintext**; the existing config/secrets split model)                       | Rotation = change the secret, not the config                                    |
| Protocol        | A closed set of three: `http-rest` (the openapi driver) / `webhook-out` (outbound push) / `webhook-in` (inbound receipt)  | A custom protocol = write an L1 driver (platform release), not a config surface |
| Timeout & retry | Driver-unified defaults (connect 5s / send-receive 30s), overridable per provider; idempotency keys carried by the caller | Standardized failure semantics (§6.6)                                           |
| Permissions     | The three-tier capabilities `<domain>.read/operate/admin` + the invocation surface `external.<provider>.<method>`         | Management surface and invocation surface separated (#66 §3.3's current state)  |

### 4.2 Adapting the Three Protocol Shapes

1. **http-rest**: a vendor with a REST API → the openapi driver. The vendor only supplies an "operation list" (method names + paths + request/response JSON shapes); the manifest provider block declares operations, and the binding config carries endpoint and credentials. **The vendor writes no code at all.** (Spec example: LAN infrastructure assets integrated over cockpit's unified REST — §3.8 ①; its circuit-breaker fallback is this spec's "custom protocol = L1 driver" in its asset form — §3.8 ②)
2. **webhook-out** (croupier → vendor, the alert-outlet class): §5 Outlet's generic-webhook implementation — a fixed envelope (Appendix B) POSTed to the vendor's URL with an HMAC signature header; the vendor routes on envelope fields.
3. **webhook-in** (vendor → croupier, the alert-inlet class): §5.3's inbound contract — the vendor maps its own alert fields onto the croupier envelope per the mapping template.

### 4.3 The Integration Document Template (Appendix A in Full)

`docs/templates/` carries a self-service checklist (M4): a vendor completes integration by filling in 10 steps — capability list → protocol shape → the manifest provider block JSON → binding config → secrets registration → capability declaration → timeout/retry parameters → connection test → panel-visibility acceptance → upgrade/decommission conventions. Revised during the first real vendor integration walkthrough.

## 5. herald Integration Spec (Alert Outlets, Both Directions)

### 5.1 The Outlet Abstraction (Providerized: Registry + Degradation Chain + In-Site Default)

**First principle: default no-op (zero config = zero outbound)**. All external-service Providers (alert outlets / status sources / vendor calls) default to a **real no-op implementation in the registry** (same interface, not if-checks scattered in business code) — zero config means zero outbound, honoring the data-exfiltration-disabled-by-default red line. herald/atlas are both **options**: they take effect only when explicitly added and enabled in config, and can be added, removed, or swapped; a switch = changing a registry entry with zero business-code change. The config page lists "enabled external services", each with an enable switch.

- **The `Outlet` interface slot** (server-side, a small interface):

```go
// Deliver delivers one alert event; the receipt feeds degradation decisions and failure counts.
// Implementations must be non-blocking-friendly (internal queues or fast returns).
type Outlet interface {
    Name() string // the registry key ("internal" / "noop" / "herald" / "webhook" / "email")
    Deliver(ctx context.Context, ev AlertEvent) (DeliveryOutcome, error)
}

// DeliveryOutcome is the outlet receipt: the basis for degradation-chain and failure-count decisions.
type DeliveryOutcome struct {
    Delivered bool   // whether delivered (receipt semantics defined by the implementation)
    Channel   string // the actual delivery channel (a herald category / a webhook URL identifier / …)
    Retryable bool   // whether the failure is retryable (the Permanent short-circuit classification from M2 carries over)
}
```

- **The outlet chain (config-driven degradation order)**: `outlets: [{type, enabled, config}…]`, an ordered list; delivery tries in order, sliding to the next when the primary outlet fails (Retryable); failure at the chain tail = a failure counter + logs (§6.6 — silent drops are forbidden). **The in-site outlet (`internal`) is the built-in head of the chain — always present, not removable, zero outbound**: in-site notification is reachable under any config; when all external outlets fail, delivery naturally degrades back in-site (the user's stance: alerts default to in-site notification).
- **The registry (compile-time factory)**: `internal` (in-site notification; built-in default; writes the messages table with level/type/source/ref tiering, below) / `noop` (the external default: the silent implementation when no external outlet is configured — **a real registry entry**) / `herald` (the default external option, delivered in M2) / `webhook` (generic, §4.2 shape 2) / `email` (reserved). **A new outlet = one new Outlet implementation (L1 built-in) + one registry line**; the event vocabulary and the alert pipeline change zero.
- **In-site notification strengthening (the 2026-10-10 stance order)**: the messages table gains `level` (an `info|warn|critical` enum, default info, with history backfilled as info) / `type` (notification type: incident/alert/report/system…, reusing the existing column) / `source` (which agent/status source) / `ref_type`+`ref_id` (linked object: incident/server, filterable) / `scope` (**visibility range, a mandatory field**, below); **ordering** = urgency critical>warn>info, time-descending within a level (the list/panel default); **visual tiering** = badge color/icon per level, with in-site notifications mapped by default from event severity (critical/warning/info→critical/warn/info); **escalation** = the same event (dedupKey) re-triggering raises the level (info→warn→critical, repeated warn→critical) with dedup (update the same row, never create); report pushes tier by the same rules (see [incident-reports](incident-reports.md) §6).
- **Notification visibility scope (scope, the 2026-10-10 supplementary order)**: **notifications are not broadcast to everyone**. messages.scope records the category context (e.g. `{category: "client"}`); visibility resolves at read time through a "category → department/role" mapping (stored in incident_categories.audience `{roles, users}`, editable on the settings page): visible to that category's audience (department roles + explicit accounts) plus that category's leader; **administrators see everything**; reports/leaderboards/owner reports follow the same rule — **the total aggregate goes only to the administrator, leaders receive their own category's shard, not one copy each**; notifications without a category fall to "ops" or the configured fallback recipient group — **never lost**; the external API/webhook likewise limits visibility by the token's scope (incident-reports §9).
- Event sources (capture rule hits / devops rules / supervisor breaker / healthprobe windows) only produce §5.2 envelopes and **know nothing of outlets**; the outlet manager delivers per the configured chain (M2's Manager fan-out/retry/Permanent short-circuit semantics are retained, extended into chained degradation).
- **The generic webhook = the same-interface implementation** (§4.2 shape 2): the same envelope POSTed out, with the vendor building its own mapping; deployments without herald use the webhook outlet to reach DingTalk/Feishu/WeCom bots.

### 5.2 The Unified Alert Event Envelope (Outbound Wire)

The **event vocabulary** shared across outlets (the closed set expands by batch; new kinds go through the corresponding profile):

| Field              | Type               | Notes                                                                                                                                                            |
| ------------------ | ------------------ | ---------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `kind`             | string, closed set | `capture.rule_hit` / `devops.build_failed` / `supervisor.breaker_tripped` / `probe.unavailable` / `probe.recovered` / `alert.inbound` (the inlet-to-outlet path) |
| `severity`         | closed set         | `critical` / `warning` / `info` (outlets map urgency themselves)                                                                                                 |
| `eventId`          | string             | croupier's own event primary key (an alert id / an event seq / `{target}:{window_start_ts}`), feeds idempotency                                                  |
| `dedupKey`         | string             | The outlet-side dedup key (same key + same state folds; a state flip re-sends)                                                                                   |
| `title` / `body`   | string             | Summary text (v1 delivers directly, no templating)                                                                                                               |
| `occurredAtUnixMs` | int64              | Event time                                                                                                                                                       |
| `scope`            | object             | `gameId` / `env` / `agentId` (a nullable scope triple)                                                                                                           |

The herald mapping (kind→category, severity→urgency, event_id→idempotency) follows the herald profile's §3 table; the webhook outlet passes the envelope through directly + `X-Croupier-Signature` (HMAC-SHA256, a shared-secret config reference).

### 5.3 Alert Inlets (Third-Party Alert Systems → croupier, Webhook Form)

- **Endpoint**: `POST /api/v1/alerts/inbound/{source}` (source = a registered source name; the closed set starts as `alertmanager` / `generic`; gated by the `alerts.operate` capability + a per-source shared secret).
- **Security**: `X-Croupier-Signature` (HMAC-SHA256 over the raw body) + `X-Croupier-Timestamp` (a ±5-minute anti-replay window); secrets via secret_refs.
- **The payload contract (generic)**: §5.2's envelope is used directly (`kind` fixed to `alert.inbound`, with `labels.source` carrying the source semantics); **Alertmanager-compatible**: accepts `{alerts:[{status,labels,annotations,startsAt,endsAt,generatorURL}]}`, with mapping config projecting labels keys onto envelope fields (config-driven — vendor field names are never hardcoded).
- **Landing**: inbound alerts land in the **existing alert storage/alert page** — the silence/claim/escalation chains are fully reused (the same bucket as capture alerts; stored per gameId/env scope); `eventId` idempotent dedup (a re-push with the same id does not duplicate rows).
- **What is not done**: croupier does not build a cross-system alert convergence/noise-reduction engine (that is outlet-side/herald-side duty); the inlet does not write acknowledgment states back to source systems (v1 is one-way).

### 5.4 Boundaries (Honest List)

- Outlet failures never block the alert chain: async delivery + bounded retries + chained degradation (the primary outlet failing slides to the next; a chain-tail failure counts and logs; the in-site outlet is always present and unaffected by degradation — the Provider-chain version of the herald profile §6 stance, universal across all outlets).
- No external outlet configured = in-site only (the external default no-op), zero outbound; the config page must list "enabled external services" with an enable switch each (decision ①).
- Inlet signature-verification failure = 400 + an audit event; nothing is stored (anti-spam).
- Silence/claim semantics are croupier-side disposition states and are not pushed to outlets to stop delivery (the herald profile §6 boundary, universal across outlets).
- Templated payloads for the generic webhook outlet (DingTalk/Feishu format adaptation) are reserved; v1 delivers only the standard envelope.

## 6. Plugin Interface Contracts (The Mechanism Surface, Common to All Extension Points)

### 6.1 Registration and Discovery: Choosing Between the Two Tables

| Registration method                            | Applicability criterion                                                      | Precedent                                            |
| ---------------------------------------------- | ---------------------------------------------------------------------------- | ---------------------------------------------------- |
| Compile-time `init()` self-registering factory | Code in the binary (L1); single-instance semantics; no multi-scope need      | `internal/drivers`, `internal/cicd`, capture sources |
| Runtime five-table binding                     | Needs an install lifecycle / multi-instance / multi-scope / panel visibility | The whole extension system                           |

The criterion in one line: **needing an install surface goes to the five tables; not needing one goes compile-time**; the two are bridged by the manifest provider block (#66 §3.2's isomorphism constraint).

### 6.2 The Manifest Contract

The only manifest = the release `manifest_json` (existing fields + the provider block finalized in P0 + the unified pattern's three-tier permissions + configSchema). **Inventing** a second manifest format or a second declaration file for new extension points is forbidden; a new extension point's declarations always arrive as new manifest blocks via the trio "field closed set + guard test + binding isomorphism" (the provider-block precedent).

### 6.3 Lifecycle

- Five-table items: the four-state sync state machine (installed/enabled/disabled/uninstalled, §4's installation model as actually landed) + reconcile replacing bindings in full; upgrade = a version switch + config validated against the new schema; rollback = upgrading back to the old version.
- Compile-time built-in items (L1): no install surface; lifecycle = the config `enabled` switch + process restart (drivers instances starting/stopping with installation state is the sole P1-batch exception — a provider is an L4 config instance driven through the five tables).

### 6.4 Config Hot-Reload (Dual Tracks)

| Host   | Mechanism                                 | Semantics                                                                                                                                                                                                    |
| ------ | ----------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| server | Installation config `PUT` + reconcile     | A config change → the instance rebuilds (driver Close + a new config construction); the hot-reload granularity = the provider instance, never the process                                                    |
| agent  | The `core/configsync.Puller` version poll | A version advance → apply takes effect hot; a failed apply does not advance the cursor (the old config keeps running); disconnection degrades to the last-known config (the extension_sync_puller precedent) |

The hot-reload semantics table must be declared explicitly (every extension point's configSchema annotated `hotReload: true|false`; the default false = requires restart — false is the safe default).

### 6.5 Isolation and Permissions

- In-process execution, no subprocess/gRPC sandbox (decision ③); the isolation boundary = three-tier RBAC capabilities (management surface) + the invocation surface `external.<provider>.<method>` + the secguard outbound guard (SSRF/allowlist) + secrets references.
- Failure isolation: panic recovery on every extension point's invocation surface (the driver Call wraps recover→error) + timeout caps + consecutive-failure counting (reaching the threshold flags the binding failed, visible on the panel; automatic circuit breaking reuses the supervisor's breaker semantics, reserved).

### 6.6 Failure Disposition (Per-Extension-Point Degradation Semantics; Silent Drops Forbidden)

| Extension point      | Failure semantics                                                                                          |
| -------------------- | ---------------------------------------------------------------------------------------------------------- |
| driver/provider call | The error passes through to the caller (HTTP semantics); the binding counts toward failed                  |
| CDC source           | Backoff and reopen; corrupted positions fail-safe-exit (§3.2)                                              |
| Detector             | A single rule's panic does not take down the event stream (recover + per-rule disable + event bookkeeping) |
| Alert outlet         | Async retry + failure counting; alert-page storage unaffected                                              |
| Alert inlet          | 400 + audit; nothing stored                                                                                |
| Dashboard binding    | A component placeholder + error state; the rest of the page stays usable                                   |
| Agent capability     | Supervisor backoff restart / breaker (S2 delivered)                                                        |

### 6.7 Version Negotiation

`min_core_version` (existing) + the driver closed set ships with platform releases (no independent version) + the manifest grows no separate version field (#66 ruling ⑤). The signing/trusted-distribution layer stays a long-term reservation (survey §7.3: a single-company self-hosted deployment; low priority).

## 7. Relationship to #66 (One Unified Placement, Not Two Sets)

### 7.1 Mapping Table

| #66 item                                         | Its place in this design                                                                     |
| ------------------------------------------------ | -------------------------------------------------------------------------------------------- |
| The three-tier model (extension/driver/provider) | Referenced in §1's glossary; not redefined                                                   |
| The driver interface + registry (P0 landed)      | §3.1, extension-point class 1                                                                |
| The manifest provider block (P0 finalized)       | §6.2 — the first "new block" precedent of the sole manifest                                  |
| The Registry archival disposition (P0 landed)    | §6.1 "choosing between the two tables" counter-example footnote (the double-registry lesson) |
| The P1 external-platform migration               | A scheduled batch within §3.1; unaffected by this design                                     |
| The P2 pack descriptor / webhook driver          | Reserved in §3.1; trigger conditions unchanged                                               |
| The P3 out-of-process connector                  | A §2 core-ruling long-term re-review item; no commitment                                     |

### 7.2 The New-Extension-Point Admission "Six Questions" Checklist (The Pre-Gate for Entering the §3 Table)

① What is the contract surface (an interface / a manifest block)? ② Compile-time or five-table registration (§6.1's criterion)? ③ Lifecycle ownership (the four states or an enabled switch)? ④ Config hot-reload semantics (the hotReload annotation)? ⑤ The permission surface (three-tier capabilities + invocation-surface naming)? ⑥ Failure disposition (a §6.6 table row)? — an extension point that cannot answer all six may not enter the table, preventing extension points from each inventing contracts.

## 8. Phased Plan (Approved by Delegated Decision 2026-10-16; Executing As Approved)

| Phase                                            | Content                                                                                                                                                                                                                                                                                                                         | Acceptance gate                                                                                                         |
| ------------------------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------- |
| M1 placement annotation (zero code)              | This design filed; §3's extension points each get a status anchor (§3.1–§3.8, eight status lines; §3.2 completed 2026-10-16); #66's P0 outputs placed                                                                                                                                                                           | Three-layer docs sync + docs build ✅                                                                                   |
| M2 the Outlet interface slot + the herald outlet | The Outlet small interface + the herald adapter (the herald profile §7's original landing, with the K3 alert-channel batch) — **delivered (2026-10-10)**; implementation-vs-profile differences in the herald profile §8                                                                                                        | Adapter unit tests (mapping/retry classification) + a local heraldd smoke test + `herald.enabled` default off ✅        |
| M3 the alert-inlet webhook                       | The inbound endpoint + Alertmanager/generic mapping + HMAC/anti-replay/idempotency — **delivered (2026-10-10)**                                                                                                                                                                                                                 | Unit tests (signature/replay/mapping/idempotency) + alert-page storage regression ✅                                    |
| M4 the integration template                      | `docs/templates/external-service-integration-template.md` + revision during the first real vendor walkthrough — **delivered as Draft (2026-10-10)**: the template's 10 steps + path criteria + herald/Alertmanager dual walkthrough records + 4 revision items; the http-rest branch awaits backfill from the first REST vendor | Template reviewed (**finalized by delegated decision 2026-10-16**, FW1016D clause 2 authority) + walkthrough records ✅ |
| M5 the cockpit integration batch                 | After the cockpit profile is filed: the L4 provider config instance connects to cockpit REST (naming aligned per §3.8 ④); on trigger, evaluate ② the provider fallback driver (a thin wrapper over the same libraries)                                                                                                          | End-to-end: a panel-initiated BMC power-on → approval → execlog record → the asset action completes                     |
| Long-term reservations                           | Detector pluginization, multi-outlet fan-out, webhook outlet templating (DingTalk/Feishu), signed distribution                                                                                                                                                                                                                  | Trigger conditions per §3.3/§5.4                                                                                        |

## 9. Known Boundaries (Honest List)

- This design is **placement and specification**, bringing no immediate new capability: everything beyond M2–M4 stays reserved with explicit triggers (§8).
- Detectors/sources closed to third-party plugins is **positioning**: rule correctness and data integrity are not worth trading for ecosystem reach; if game vendors make a strong future demand, re-review through the "six questions" (§7.2).
- The inbound-alert v1 closed set is two formats: a new vendor format = new mapping config (ideal) or a new mapping template (an L1 change riding a batch); **zero-code arbitrary-format integration is not promised**.
- Multi-outlet fan-out and outlet-side payload templating are not done (§5.4); alert convergence/noise reduction stays out of croupier.
- Third-party panel pages are bounded by the component closed set (§3.6): only what PageSpec can express may plug in; arbitrary custom rendering is out of the capability domain.
- The cockpit design profile **is not yet filed in this repo** (§3.8): §3.8's `<asset>.<action>` naming is the croupier-side proposed shape, final per two-way calibration once the cockpit profile lands; M5 does not start before cockpit is ready.
- hotReload defaults false: un-annotated config changes always require a restart — docs must never exaggerate them into hot reloads.
