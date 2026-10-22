---
title: Four Agent-Type Back-Office Pages and Game/Environment Ownership Design
---

# Four Agent-Type Back-Office Pages and Game/Environment Ownership Design

Status: **Target / design organized, pending implementation acceptance**. On 2026-10-17 the user made clear that all four agent types must carry `gameId`, `env`, and have their own back-office pages; this document replaces the earlier three-page draft, and every proposed page, endpoint, and field here is a non-delivered statement.

Scope: the four types sidecar, devops, capture, synthetics; desktop web, keyboard and mouse operation, readable and operable on narrow screens; reusing the existing Umi/Ant Design stack, the global scope selector, and the list + detail-drawer patterns. The users are mainly operations, developer-integration, and ops staff; the goal is to first confirm the game environment, then locate instances, check duty status, and execute authorized actions.

Division of authority: the [Agent inventory](agents-inventory.md) owns classification and duties, [game/environment scope](../architecture/game-environment-scope.md) §13/§14.7 owns the scope contract, and this document owns navigation, pages, interactions, data needs, and implementation acceptance; plugin and OTel default integration, runtime disabling, and build trimming are in the [go-plugin survey](../research/synthetics-agent-go-plugin.md).

## 1. Design Principles and Current-State Baseline

1. Every agent instance binds a unique `gameId + env`, uniformly across the four types; when one machine serves different scopes it runs multiple instances, with identity, directories, credentials, and plugin processes isolated separately.
2. The game environment is part of identity and must be shown explicitly on pages; one page per type, sharing a page skeleton, with duty content independent per type.
3. All lists, summaries, details, actions, downloads, and replays are scope-validated server-side; hiding frontend rows is not permission isolation.
4. Existing capabilities reuse the same APIs and components; planned capabilities show their real status — module unit tests, metadata, or the existence of a button must never be presented as a delivered closed loop.
5. Agent connection status, business health, and probe/plugin status are displayed separately; missing data is not treated as success or zero failures.

| Area                | Current code facts                                                              | Implementation gap                                                                                                                            |
| ------------------- | ------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------- |
| Page entry          | `web/config/routes.ts` has `/ops/nodes`, `/ops/capture`, etc.                   | No four-type independent Agent pages from this design yet; the old Nodes/Capture cannot be counted as the new pages                           |
| Registration config | sidecar startup requires gameId/env; devops/capture already carry both fields   | The latter two's Validate does not yet enforce non-empty env; synthetics has no independent entry; validation must be unified across the four |
| Agent list          | `internal/api/ops/helpers.go` has cluster/local query paths                     | The cluster branch has scope filtering; the local and fallback branches need the same filtering; the DTO has no type filter                   |
| Type labeling       | devops/capture register labels.role; sidecar is the legacy incumbent            | Establish an explicit four-type mapping, persistence, and legacy-version recognition rules; never guess all unknown types into sidecar        |
| Duty capabilities   | supervisor, capture rules and alerts, and probe summaries have existing modules | devops protocol sessions, the synthetics plugin runtime, new data surfaces, and sidecar production wiring still have gaps                     |
| Data ownership      | HTTP ResolvedScope and the agent session's single-scope norm exist              | Verify item by item: events, persistent caches, offline records, config distribution, log downloads, and the action chain                     |

## 2. Navigation, Routes, and Entry

Ops gets an "Agents" group; the four routes are the intended targets:

| Menu                 | Frontend route           | Page body                                | Detail tabs                                                                     |
| -------------------- | ------------------------ | ---------------------------------------- | ------------------------------------------------------------------------------- |
| Game agent           | `/ops/agents/sidecar`    | Instances, processes, functions          | Overview / Processes / Functions & adapter plugins / Probes / Executions / Logs |
| Ops agent            | `/ops/agents/devops`     | Allowlisted hosts, sessions, audit       | Overview / Hosts / Sessions & replay / Executions / Logs                        |
| Data capture agent   | `/ops/agents/capture`    | Sources, positions, rules, alerts        | Overview / Capture status / Policy versions / Hit alerts / Executions / Logs    |
| External probe agent | `/ops/agents/synthetics` | Probe points, targets, developer plugins | Overview / Probe targets / Probe runs & windows / Plugins / Executions / Logs   |

Entries come from the sidebar, Nodes instance links, and alert target links; details restore from `agentId` and `tab` in the URL, and returning to the list keeps the current scope's filters and page number. Instance links themselves carry no scope parameters that could override the current request identity — ownership is still decided by the verified ResolvedScope before access; a direct link under a wrong scope shows "no such instance in the current environment".

The four pages ship in batches with their corresponding data surfaces; a page without implemented backend capability does not enter the formal menu, and publishing an empty shell with fake data is forbidden. When a delivered page's current scope has no instances, it shows "no agents of this type in the current game environment" plus an entry to deployment config docs. The connector remains a roadmap item; no new page.

## 3. Common Page Layout

```text
Global top bar: game selector / environment selector
Page title: Game agent (or another type)  [gameId: demo] [env: prod]
Summary: instances | online | offline | business-abnormal (counted for the current scope+type only)
Filters: agent ID / host / connection status / duty status / version  [Refresh]
Instance table: identity & scope | connection | duty health | host | version/fingerprint | last report | actions
Click instance → detail drawer: fixed identity header + categorized tabs + data time/source
```

- The title area always shows the raw `gameId` and `env` IDs, optionally supplemented by game/environment names; every row keeps these two columns too, and the drawer, action confirmations, downloads, and replay headers show them likewise.
- `env` is never conveyed by color alone; prod is emphasized in text, other environments use a unified environment mapping, and unknown environments never fall back to prod/dev.
- Lists use server-side pagination/filtering; the summary aggregates before pagination; page/pageSize, type, filters, and sorting never change the request scope; during refresh the current scope's data is kept with a loading indicator.
- The initial list and summary load in parallel; details and expensive logs/replays load on demand; periodic refresh pauses when the page is hidden. The default 15-second refresh is a UI policy — agent heartbeat and telemetry sampling periods keep their own configs.
- The detail drawer always shows `agent ID / type / gameId / env / host / version / binarySha256`, connection status, and last collection time; a missing fingerprint is labeled "not reported", with copy-full-text allowed.
- Beyond 1000 rows, never download everything at once; tables paginate and details lazy-load, with no per-row real-time RPC. Acceptance reference: in a test environment with a 200ms-cache API, a 50-row first screen is interactive within 2 seconds — not a real-environment SLA.

## 4. Per-Type Duty Data and Actions

### 4.1 sidecar: Game agent

- Process table: name, the four run/management/schedule/update states, PID, uptime, restart count, CPU/RSS, run fingerprint, recent events.
- Function table: functionId, service/provider, schema/version, authorization and approval requirements, linking to the existing function catalog/invocation pages; external function-adapter plugins and SDK sources are labeled separately, and management actions appear only when the plugin runtime is implemented and permitted.
- Reuse the supervisor drawer and the process start/stop/restart/update plus schedule pause/resume endpoints; the update form keeps stagedPath/expectedSha256 validation with result feedback.
- Auto-supervision, scheduling, and update capabilities may be marked available only after being wired from the real config entry; process-action permission is not generic remote shell permission, and the page adds no generic shell.

### 4.2 devops: Ops agent

- Host table: authorized hosts, explicit port/protocol, connectivity and last probe; the allowlist and credential references are managed under the current scope.
- Session table: operator, target, protocol, start/end, status, audit/archive status; the connect entry appears only when a real protocol channel, authorization, and the audit preconditions are available.
- When the sessions switch is off, show "sessions disabled" — the agent may still be online; when the channel is unimplemented or unsupported by the agent version, show a capability-missing note; entering a noop channel by clicking is not allowed.
- Replay requires actual content archives and an authorized data surface; with only start/end metadata, show "session metadata only" and do not label it replayable.

### 4.3 capture: Data capture agent

- Capture status: source type/identity, replication connection status, lastEventAt, read/processed/persisted positions and their semantics, rule hit counts, queue backlog/drops, last error; nonexistent metrics show "not reported" — no fabricated values by inference.
- Policy status: desiredRevision, appliedRevision, last sync/error; publish, history, and rollback link to the existing capture/risk policy pages, all within the same scope.
- Hit alerts: rule, severity, table/record key, time, handling status, and an alert-detail link; credentials and sensitive rows are masked per permissions.
- The raw change-event stream currently has no complete persisted-query promise; the first version shows no fake real-time event list. Event detail needs sampling, retention, permissions, and an interface defined first.
- Position repair/reset carries data risk; this design does not make direct checkpoint editing an ordinary page action — only diagnostics and the existing controlled entries are shown.

### 4.4 synthetics: External probe agent

- Probe points: name, region/network segment/observation perspective, owning agent and scope; the physical target address carries no env semantics.
- Target table: explicit target category `targetCategory` (game / third-party service), protocol/plugin capability, enabled, period, timeout, last result, failure phase, latency, and failure window.
- "Port reachable", "protocol verified", "business verified", "not connected", "target failure", "plugin error", "timeout", and "cancelled" are displayed separately; status maps from server-side result semantics — a living plugin does not mean a usable target.
- Plugin table: ID/version/fingerprint, supported capabilities, process status, last error, concurrency/backlog; enable/disable and manual probes execute under authorization with audit, and a manual probe never implicitly alters automatic scheduling or probe-window state.
- Game login and third-party checks are implemented by go-plugin external programs; host permissions and game control are never delegated; the target and credential scope must equal the agent's bound scope.

#### Target Categories (clarified by the user, 2026-10-17)

External probing covers two kinds of objects; creating a target requires choosing the independent field `targetCategory`, with this closed set:

| Value         | Page name           | Typical objects                                                              |
| ------------- | ------------------- | ---------------------------------------------------------------------------- |
| `game`        | Game                | Game gateway, handshake, login, and game business availability               |
| `third_party` | Third-party service | Payment, account, messaging, storage, and other external dependency services |

- This field is modeled separately from the agent type `synthetics`, the protocol `http/tcp/private protocol`, and plugin capabilities; an HTTP target may also belong to a game — the category is never guessed from protocol, URL, or plugin name, and one plugin may support both categories.
- The page offers an "All / Game / Third-party service" filter; the list keeps a category column, and the create/edit form, details, probe runs, failure windows, and alerts all display the category; summaries aggregate server-side per current scope and category, and cache keys include the category filter.
- The category flows through target config, distribution, the Check request, results, persistent records, and alerts; the server treats the target config as authoritative, plugin return values cannot change the category, and historical records keep the category snapshot from execution time.
- Both categories keep `gameId/env`; a third-party service never automatically becomes a global target. A shared physical service is declared as separate targets per scope with independent authorization; cross-scope reading of results or credentials is never opened.
- New configs with a missing or unknown category are refused publication; legacy data migration must classify explicitly — defaulting everything to game is not allowed; unclassified history is available only for authorized diagnostics and is not counted in either category's normal statistics.

### 4.5 Capabilities and Switches Shared by All Four Types

The overview shows the plugin host's and OTel's compiled/enabled states with reasons, distinguishing "not compiled", "disabled", "enabled", and "unknown (legacy version)"; default-build integration with runtime disabling is the rule, and a trimmed build must never masquerade as "capability off but can be turned on". Config uses real distribution/apply confirmation; switch buttons with no corresponding write endpoint are not provided for now.

Agent online, duty ready, target business health, plugin availability, and telemetry export status are displayed separately; OTel being off must not make the agent heartbeat or business metrics read as offline.

## 5. Scope Contract and Interactions

### 5.1 Identity, Naming, and Data Flow

All four types use the same required ownership block; the example shows only the common fields — each binary keeps its own server/business config structure:

```yaml
agent:
  id: demo-prod-sidecar-01
  gameId: demo
  env: prod
```

sidecar/devops/capture/synthetics use independent IDs, e.g. `demo-prod-devops-01`, `demo-prod-capture-01`, `demo-prod-synthetics-01`; ID naming only aids recognition — the server must never parse IDs to derive scope. The config-generation/deployment-doc pages show the game environment, carry explicit values before copying config, and use credential references rather than embedding secrets in examples.

- Config/YAML/REST use `gameId`, `env`; proto/DB use `game_id`, `env`; HTTP requests reuse `ResolvedScope`, and optional overrides go only through the paired `X-Game-ID`/`X-Env` headers — the current scope is no longer duplicated in query/body.
- Agent registration must carry both values explicitly non-empty; the server validates that the game environment exists and the registering identity is authorized, then binds the session; heartbeat, reconnect, and HA forwarding keep the same ownership, which is never derived from or overridden by user-defined labels.
- Plugin inputs, tasks, targets, alerts, audit, positions, config, logs, and offline records inherit the agent's scope; same-named fields in plugin results cannot rewrite ownership — conflicts are rejected and logged as contract errors.
- Managed targets belonging to another scope must not be commanded; ownerAgentId alone must never bypass ownership validation; the existing DB-probe ownership-derived cross-scope special case must be closed out before implementation.
- Agent IDs keep the current registry's global-uniqueness requirement; one ID may never have an active connection overriding a different scope; changing scope uses a new instance identity and cleans up old routing; editing ownership directly on the page is not supported.

### 5.2 Page Switch Flow

1. Select the new game/environment and complete the existing scope restore/authorization confirmation; with no valid scope, no business requests are sent.
2. Immediately close the old instance drawer, action confirmations, and log/replay views; cancel old-scope queries, subscriptions, and refresh tasks; clear pending targets and forms.
3. Cache keys include at least `(gameId, env, type, agentId, resource, filters)`; an old request returning late must never write into the new page; the new scope's details are re-queried.
4. Reload the list and summary with the page number reset; same-scope ordinary refreshes keep filters. When permission is revoked, data is cleared and the user must re-select an authorized scope.
5. If a switch happens after an action was submitted, no automatic retry or migration to the new environment; the backend completes the audit under the action's submitting scope, and the user checks execution records back in the original environment.

### 5.3 Backend Validation and Errors

- Order: authentication → ResolvedScope/authorization → resource ownership → action permission → agent capability/local switch → execution and audit.
- Lists, aggregations, details, downloads, replays, and mutations all perform the same ownership checks; the cluster/local/fallback/offline-persisted query paths are consistent, and full access without a scope is forbidden.
- No valid scope returns 400; an unauthorized scope returns 403; a target outside the current legal scope returns 404 with a body that leaks no other environment's existence; a temporarily unreachable agent's action returns a recognizable unavailable error — never wrapping no-response as success.
- Success returns the business object directly; errors are `{error,message,details}` with no new code/data envelope; downloads return a proper Content-Disposition, and file contents remain scope-validated.
- Offline instances use the persisted last-known scope with timestamps; history/log visibility is retained and marked stale, with all real-time controls disabled. Migrated history lacking ownership does not enter the four pages; diagnostics go only through authorized platform logs.

## 6. Data and Interface Needs

The following are all target endpoint lists: existing endpoints get isolation and DTO fixes first, then reuse; new endpoints have their contracts fixed at implementation.

| Data/action                     | Source or endpoint (`/api/v1` prefix)                                                                         | Status and requirements                                                                                                                                       |
| ------------------------------- | ------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Instance list & summary         | `GET /ops/agents?type=sidecar`; proposed `GET /ops/agents/summary?type=sidecar`                               | Existing list extended with type, pagination, filters, and an offline view; summary as independent server-side aggregation                                    |
| Instance detail                 | Proposed `GET /ops/agents/:agentId`                                                                           | Full identity, type, capabilities, connection, and data freshness; no duplicated scope params                                                                 |
| System & supervision            | Existing `.../system-info`, `.../supervisor`, `.../supervisor/events`, `.../supervisor/logs`, `.../processes` | Reuse API/Drawer; add unified scope validation and time source                                                                                                |
| Process actions                 | Existing `.../processes/:name/start                                                                           | stop                                                                                                                                                          | restart | update`and`.../schedule/pause | resume` | Reuse existing permissions and the local double gate; the request target comes from the path, never replaced by a form |
| Functions & adapter plugins     | Existing function catalog/registry; proposed plugin instance & action queries                                 | Filterable by agent but never bypassing function scope/permissions; the adapter runtime is pending                                                            |
| devops hosts & sessions         | Proposed `.../hosts`, `.../sessions`, and authorized session create/close/replay                              | Delivered when a real protocol channel, audit, and archive are ready; create idempotency keys and disconnect/cleanup semantics defined in the protocol design |
| capture status                  | Proposed `.../capture/status`, reusing the capture/risk policy and alert APIs                                 | Stable exposure of positions, counters, versions, alert backlog; no raw-event query guarantee yet                                                             |
| synthetics targets/runs/plugins | Proposed `.../probe/targets`, `.../probe/runs`, `.../plugins` and authorized actions                          | Plugin runtime and scoped config delivered together; declarative built-in probes never presented as external plugins done                                     |
| Failure windows                 | Proposed `.../probe/timeline`, reusing the existing probe summary                                             | Uniquely keyed by windowId+agentId+targetId+scope, with explicit persistence and retention; an in-memory ring must not be labeled as cross-restart history    |
| Execution audit & logs          | Proposed `.../executions`; reuse supervisor logs and the existing executionlog queries                        | operator/action/result/time filters; scope/type/agentId come from server-side identity; log read volume limited                                               |

Common DTOs must include `agentId/type/gameId/env`, plus host/version/binarySha256, connectionState, lastSeen, observedAt, stale, capabilities, and desired/applied config versions; inapplicable fields are omitted or null — unknown numbers must never be set to 0. The type closed set is sidecar/devops/capture/synthetics, normalized by the server from fixed role declarations, never guessed by the page; unknown legacy versions stay in Nodes diagnostics for now and enter the proper page after upgrade.

The `items/total` pagination shape versus the existing `agents` response must be synced with adapters/callers and contract tests at implementation — never silently breaking the old Nodes. Default page size 20, optional 50/100; sorting is stable with agentId as the tiebreaker.

Status and metrics come from the registry/cluster owner, persistent AgentSessions, MetricsStore, the supervisor, the capture runner, and probe/plugin runtimes — each its own source of truth; the page owns no background scheduling or workload state.

## 7. Permissions, Interaction States, and Accessibility

- Reads use the existing `canOpsRead`/ops read permissions; management uses `canOpsManage` layered with specific action permissions and the local allow switches; new capabilities such as devops sessions/recordings, plugin start/stop, and target credentials must map to real backend permissions — frontend-only buttons do not control anything.
- High-impact actions reuse the existing confirmation and approval policies; the confirm dialog always shows `gameId/env/agentId/target`; double-submit is blocked while pending; success refreshes same-scope data; failure keeps input and shows the server's reason; reusing an old dialog after a scope switch is forbidden.

| State                                   | Display and allowed behavior                                                                                     |
| --------------------------------------- | ---------------------------------------------------------------------------------------------------------------- |
| No scope selected / unauthorized        | Prompt to select a game environment or show no-authorization; no instance-list request                           |
| Initial load                            | Skeleton/loading text; instance actions disabled                                                                 |
| No instances in this scope              | Empty state with a deployment-config-docs entry; no fallback to other environments                               |
| Type not yet delivered                  | Not in the formal menu; direct access shows a capability note                                                    |
| Offline / history stale                 | Show last-known values and times; real-time controls disabled; authorized history queries allowed                |
| Not compiled / disabled / not connected | Show reason text; never a green "normal" light instead                                                           |
| Business target failure                 | Show failure phase, reason, and failure window, distinguished from agent offline                                 |
| Network/API error                       | Same scope keeps old values marked stale with retry; 403 clears unauthorized data; 404 closes the invalid detail |
| Logs/replay missing                     | State clearly no archive or not connected; never play simulated content                                          |

Keyboard order: scope selection → page filters → refresh → table actions → pagination; drawer/Modal trap focus, Esc closes non-submitting views and restores focus to the trigger button. Status always pairs icon and text; errors are readable and associated with form fields; data refresh never steals focus; reduced-motion is supported, and animation never conveys failure.

All copy enters the existing zh-CN/en-US localization; times display in the user's timezone with the raw timestamp viewable; IDs, versions, and fingerprints are copyable; long fields truncate with full accessible names; narrow screens scroll tables horizontally while keeping identity/action columns.

## 8. Relationship to Existing Pages and Historical Designs

- `/ops/nodes` stays the node-infrastructure view; the four type pages provide duty views. The supervisor and system-info components/data sources are shared, interlinked rather than duplicating backend state.
- `/ops/capture` and `/risk` keep the policy, release, and alert business entries; the capture agent page shows instance execution/capture status with same-scope links.
- The existing scope-neutral message SSE keeps the full-authorization-range semantics of [the scope spec §13.2](../architecture/game-environment-scope.md); the four pages never reuse it as an unvalidated log/data stream channel.
- The old designs' "scope query parameters", "devops not chartered", "the sidecar new page is delivered", "every agent is already wired", and "the raw change-event stream is queryable today" are all superseded by this document's current-state baseline; historical module completion records are kept but do not mean the four pages are done as a whole.

## 9. Implementation Batches and Acceptance

| Batch                    | Deliverables                                                                                                                                  | Gates                                                                                                                 |
| ------------------------ | --------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------- |
| A identity & isolation   | All four type configs carry explicit gameId/env/type; registration validation, same-ID conflicts, scope validation on every query/action/file | Negative cases: empty values, forged ownership, cross-scope, revoked permission, HA and local fallback                |
| B common pages & sidecar | Shared list/drawer/identity header, capability status, Nodes reuse, sidecar production wiring                                                 | The full real chain CLI→register→list→detail→authorized action→audit                                                  |
| C capture page           | Status DTO, position/policy confirmation/alert linkage, reliability labeling                                                                  | Two scopes' sources and rules never cross-contaminate; capture stream interruption/restart and alert query acceptance |
| D synthetics page        | Independent agent, go-plugin, real targets, plugin management, and the four-way result distinction                                            | Go/Python plugins, real game and third-party services, fault injection, scope binding acceptance                      |
| E devops page            | Host config, real SSH sessions, audit preconditions, and content archives                                                                     | Allowlist/identity/ownership triple validation, disconnect cleanup, authorized replay; RDP/VNC later                  |

All acceptance below is pending implementation — none may be checked off merely because this document was written:

- [ ] Startup fails for any of the four type configs missing gameId or env or containing only blanks; the server refuses registration with a missing or unauthorized scope.
- [ ] One agent ID cannot override a different scope; heartbeat, reconnect, and HA-owner recovery keep ownership consistent.
- [ ] At least two scopes constructed per type: list, summary, detail, logs, downloads, replay, actions, and config distribution must never cross domains; cluster, local, and fallback paths all covered.
- [ ] Every page header, instance row, detail, and action confirmation shows gameId/env; multi-tab operations are never crossed by another tab's default selection.
- [ ] When scope A's slow request returns after switching to B, B's page never shows A's data and the old dialog cannot submit into B; submitted actions are still audited under A and not executed twice.
- [ ] In an authorized scope, a target outside the current environment returns 404; an unauthorized scope returns 403; no scope means no full-data fallback.
- [ ] Offline/legacy/unknown type/not-compiled/disabled/no-metrics all have real copy; connection and business health are shown separately.
- [ ] sidecar functions and processes, capture policy and alerts, devops real sessions, and synthetics real targets/plugins each complete their end-to-end chain — noop/mock never stands in for a delivered claim.
- [ ] Default and no_go_plugin/no_otel/double-trimmed build states are presented truthfully; after master-off no subprocess/exporter starts and heartbeats are unaffected.
- [ ] With both game/third_party targets configured in one scope, filters, summaries, details, probe history, and alert categorization stay consistent; missing/illegal categories refuse publication; an HTTP game target is never misfiled as third-party; plugins cannot rewrite the configured category.
- [ ] A 50-row list meets this document's test-environment first-screen target; pagination does no full prefetch; timed refresh never steals focus and pauses on hidden pages.
- [ ] All available actions are keyboard-reachable with focus restore, non-color-only status, working zh-CN/en-US, and a usable narrow-screen layout.
- [ ] Web tsc/Jest, scope backend tests, PageSpec/localization guards, and necessary real-environment E2E pass, with unverified boundaries reported per repo delivery norms.

## 10. Boundaries Needing Refinement Before Implementation

Failure-window/log/recording retention periods and capacity budgets, the raw CDC event-detail scope, new permission keys for sessions and plugins, synthetics' final mutable-config fields, and API pagination compatibility migration are fixed by their implementing batches; none of these open details may alter single-agent-single-scope, the four-type pagination, or server-side isolation requirements. This document adds no storage tables; if implementation needs persisted fields or indexes, follow the existing numbered-migration and model/protocol/usage-docs sync rules.
