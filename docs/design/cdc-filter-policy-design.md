---
title: The CDC Filter Policy System — Unified Policy Contract, Layered Execution, and Distribution Ack (organized per user order 2026-10-11)
---

# The CDC Filter Policy System (Filter Policy)

## Status

- Status: **the contract is coded, and the execution layer's first gate is wired (phases one + two delivered 2026-10-11)**. This page is the write-up ordered by the user: the layered filter model, the four rule types, the unified policy contract, the distribution-ack chain, the position relationship, failure semantics, and observability. The contract surface (the unified policy header/database-table authorization matching with default-deny/dependency checks/the four-outcome classification) has landed in `agents/capture/filter/` (pure logic, zero dependencies, locked by unit tests); the EventFilter (operation allowlist) is wired into the runner pipeline before the gates (phase two, default = allow-all for backward compatibility); the position-policy linkage is decided (§4). Task breakdown in `todo.md`'s "CDC platformization batch"; cross-linked with the [capture-agent brief](capture-agent-design.md) (the current three-gate implementation), the CDC connector architecture (multi-database ingest), and the risk-control panel design (panel ownership).
- **Positioning (user order, 2026-10-11)**: the CDC Agent is not the final management center of filter rules — it is the capture-and-transport component deployed next to the data source, responsible for discarding unneeded data as early as possible; unified rule management, version control, and filter semantics belong to the control plane. **What is unified is policy definition and management, not forcing all events back to the server for filtering.**

## 0. Purpose (read this first)

It solves three problems:

1. **Performance**: discard unneeded data as close to the source as possible (not shipped to the server for judgment).
2. **Consistency**: different agents never implement conflicting filter rule sets (semantics unified in the control plane).
3. **Operability**: rules configured, distributed, audited, and rolled back uniformly — never per-agent hand edits.

**The four-layer filtering view** (one `game.items` change passes through in order):

| Layer                    | Judgment                                                        | Executed at                |
| ------------------------ | --------------------------------------------------------------- | -------------------------- |
| ① Capture-scope filter   | Should this table be monitored at all?                          | Agent (next to the source) |
| ② Event filter           | Should this INSERT/UPDATE/DELETE enter the processing pipeline? | Agent                      |
| ③ Business rule judgment | Is this change anomalous?                                       | Agent (the three gates)    |
| ④ Output filter          | Which downstream subscribers receive the event/alert?           | Server                     |

Without separating these four layers, adding PostgreSQL, more sources, or new business rules later would mix capture logic, business judgment, and push logic together.

## 1. The Four Rule Types (one policy model, typed bodies)

**Principle: a unified policy model ≠ stuffing four rule types into one giant JSON.** A unified policy header (identity/scope/revision/lifecycle) + typed config bodies:

| Rule type            | Responsibility                                                                     | Default execution location | Panel ownership                  |
| -------------------- | ---------------------------------------------------------------------------------- | -------------------------- | -------------------------------- |
| `SourceFilterPolicy` | Which databases/schemas/tables/operations to capture; column capture and masking   | Data source / Agent        | Data source (capture scope)      |
| `EventFilterPolicy`  | Which change events enter the processing pipeline (cheap deterministic conditions) | Agent                      | Data source / capture policy     |
| `DetectionPolicy`    | How to identify anomalous changes (the reconcile / CEP / lineage three gates)      | Agent                      | Risk control (detection rules)   |
| `DeliveryPolicy`     | Which events/alerts go to which subscribers (scope/permissions/masking/routing)    | Server                     | Messaging and data-access policy |

Conceptual example (not a current API):

```yaml
apiVersion: croupier.io/v1
kind: FilterPolicy
metadata:
  id: game-item-monitor
  revision: 12
scope:
  gameId: game-a
  env: prod
  sourceId: mysql-main
source:
  tables:
    include: [game.items, game.player_wallet, game.ledger]
  operations:
    include: [insert, update, delete]
event:
  operations:
    include: [insert, update, delete]
detection:
  reconcile: { enabled: true }
  cep: { enabled: true }
  lineage: { enabled: true }
```

### 1.1 SourceFilter (capture scope)

- Database/schema/table include/exclude; operation types; column capture and masking policy; pushed down to the connector/log subscription mechanism when the source's capability allows.
- Current anchor: `MySQLConfig.Tables` in `agents/capture/source` is this layer's first implementation (local YAML) and must be promoted to centrally managed policy. The contract surface is coded: `filter.TableMatcher` (include/exclude + default deny + exclude wins + conflicts are construction errors) + the `MissingDependencies` dependency check.
- **Important limit**: never drop business ledger tables to save event volume — when a reconcile rule depends on `game.items` + `game.ledger`, `ledger` must be captured even if it never alerts on its own (rule dependency validation happens at publish, see §3).

### 1.2 EventFilter (coarse event filtering)

- Only cheap deterministic judgments: ignore unmonitored operation types, skip events clearly outside the pipeline, filter by event metadata (source/environment/category), and check that metadata required by later rules exists.
- **Coarse filtering must never drop events that could trigger anomaly detection**: an UPDATE without a parseable amount field is not discarded — it may still hit an off-hours rule (the current three-gate implementation already complies: a missing delta counts as `SkippedNoDelta` rather than swallowing the event, and the other gates still evaluate).
- **v1 coded and wired (started in phase two, 2026-10-11)**: `filter.EventFilter` is an operation-type allowlist only (the `event.ops` section; empty default = allow-all for backward compatibility); the runner filters before the gates, counting `FilteredNoMatch`/`FilteredError`; unknown operations = an Error count and drop (the shape is unevaluable — observable, never silent). Table-level filtering does not enter the event filter (allow semantics vs authorization semantics differ, see §1.1).
- Responsibility boundary: it does not replace permission checks; broad rules configured by ordinary users must never override security constraints.

### 1.3 DetectionPolicy (business anomaly detection)

- Reuses the existing three gates (reconcile/cep/lineage in `agents/capture/gate/`), **kept executing on the Agent**: reconcile and CEP need windows/counters/pending-match state; moving everything server-side would add network dependency and make real-time detection a central load.
- The `gate` package remains the rule execution engine (rule sets already hot-swap atomically — see the brief's §8.5 gateSet snapshot); **no generic rule engine is built and no CEL/Drools/Flink introduced** — parameterized templates + deterministic matching suffice.
- Rule configuration, versioning, enable/disable, and audit are managed uniformly by the Server (before this batch carried by the `capture.gates` config_versions key; after policy-ization it migrates into DetectionPolicy — see §7's migration path).

### 1.4 DeliveryPolicy (output and subscription)

- Server side: the same anomalous event — security ops get the full alert, ordinary ops get masked details, analytics gets aggregate statistics, and other games/environments get nothing.
- Involving scope, subscriber permissions, and data masking, **it cannot rely on agent coarse filtering**; reuse the existing alerting system (alerts + outlet) and manage output scope and subscription permissions separately.

## 2. Policy Lifecycle (desiredRevision vs appliedRevision)

Distribution in six steps: **edit → validate → publish (immutable revision, recording publisher/time/previous version) → distribute (only to agents matching gameId+env+sourceId) → apply (agent-local full validation + atomic switch) → acknowledge (report the actually effective version)**.

- Agents cache published policies and run fully offline in normal operation; **they never ask the Server for a filtering judgment per event**.
- Current channel anchors: the downstream pull wire pair `0x04011B/0x04011C` + `core/configsync` version polling (brief §8.5) is the embryo of steps 4/5; **step 6 is live (2026-10-11)**: after taking effect the agent re-registers asynchronously, carrying appliedRevision in the `captureGatesRevision` label, and `/ops/capture`'s "effectiveness reconciliation" row reconciles desired vs applied (lag tolerated); step 3's immutable revision chain is now explicit (optimistic locking on baseVersion, conflicts get 409 and the chain refuses, brief §8.10) — "embedding the unified policy header" waits for the per-type key split.
- **Publish state must distinguish `desiredRevision` (intended) from `appliedRevision` (actually effective)** — the panel must never display "effective" when only desired exists.
- Two special cases of policy change:
  1. Changing CEP thresholds: switch rule instances atomically within the safety boundary; whether the original window state is kept must be explicit (current implementation: the whole gateSet snapshot is replaced, window state is not kept and restarts from zero — the brief's §8.2 timing boundary).
  2. Adding captured tables: define the position from which it takes effect — **updating the allowlist alone is not enough**, otherwise the new table's historical changes could never be backfilled (decided in §4: increments from the current position; history goes through a separate backfill).

## 3. Publish-Time Validation (capability-aware + dependency-aware)

- **Coded (server-side since 2026-10-13, capture-agent brief §8.13)**: the publish path = structural lint + `config.ValidateGates` semantic truth + dependency awareness (reconcile's two tables / CEP's mutated tables ⊆ the capture scope; publishing outside the scope is refused, naming the offending tables) + capability awareness (reconcile/CEP require `afterImage`). Capability sets and capture scope are self-reported via registration labels (`captureTables` / `captureConnectorCapabilities`; decided: the label channel adds zero proto pairs; when facts are unknowable the lint is skipped with lag tolerated).
- include/exclude conflicts are detected at publish time, not left to each agent's interpretation.
- **New capture scope is denied by default**: unauthorized databases/tables are never added automatically.
- Business security rules must not be overridable by ordinary event filter rules.
- Every policy binds gameId+env; production and test never mix.

## 4. Filtering vs CDC Positions (decided 2026-10-11)

Scenario: an agent monitors only `game.items` for a week, and today `game.ledger` is added — the global replication position may already have passed ledger's historical changes; updating the allowlist **does not automatically backfill the past**.

Four decisions (each anchored in existing code paths, promoting the status quo to contract):

1. **Position ownership belongs solely to the event source; the policy layer has no position concept.** Positions are produced and advanced only by `source.Provider.Bookmark()` / `connector.SourceConnector.Checkpoint()` (the runner persists them periodically via `source.Store`); policy hot-swaps go through the gateSet atomic snapshot, and the two paths never touch each other. Policy publish/rollback/retirement at any revision **never moves positions and never triggers replay**.
2. **New rules/new tables take effect from the current position.** Increments become visible after publish confirmation; publishing never implicitly backfills history, and allowlist updates never replay the past.
3. **Needing history = a separate backfill flow, explicitly triggered.** Backfill is a standalone action (a standalone task + a standalone audit), gated on the connector's `snapshot` capability (the contract is settled: snapshot without `resumableCheckpoint` = an illegal combination); **it is never executed as a side effect of a policy publish**.
4. **Switch auditing lands in logs first**: on successful apply, record `capture gates version applied revision=N position=<kind>:<value>` (the position string is non-sensitive); server-side switch auditing extends with phase three's policy assetization, reusing the appliedRevision piggyback pattern without adding wire pairs.

**The bottom line (already satisfied by the code, now contract): filter rule changes never break CDC recoverability.** When a policy is invalid and last-good is kept, the replication stream keeps advancing and events keep being evaluated under the old rules (position advance never stops for a policy failure); policy switches likewise never reset positions. Empty/expired positions still take the existing degradation path (rescan from the earliest available start, binlog retention constraints unchanged).

## 5. Failure Semantics (never a blanket drop)

Policy execution results have four classifications:

| Result        | Semantics                                                                                 |
| ------------- | ----------------------------------------------------------------------------------------- |
| `MATCH`       | A rule hit                                                                                |
| `NO_MATCH`    | The rule executed normally, conditions unmet                                              |
| `ERROR`       | The rule could not execute properly (parse failure/missing field/version incompatibility) |
| `UNSUPPORTED` | The current agent does not support this policy capability                                 |

- SourceFilter and security-related policies: deny unauthorized capture scope by default.
- Detection failure on already-captured events: **prefer keeping the event, record the error, and emit an observable signal — never silently drop** (the current three-gate standard agrees: validation failures reject the whole apply and keep last-good; runtime errors are counted and reported).

## 6. Column Filtering and Sensitive Data

- "Fields used for rule computation" and "fields ultimately outputtable" are separated: rule-needed fields stay inside the controlled agent; alert payloads keep only necessary fields; console/subscribers mask by permission; unneeded sensitive fields are ideally never captured or transmitted.
- **Column filtering must never delete reconcile join keys, primary keys, or before-images** (that would break detection capability).
- Current anchor: the key/gtid/detail in alert Details are already designed to a minimal surface; row images are position-array JSON (brief §9); the column-name mapping (information_schema ingestion) will design the column allowlist together when it lands.

## 7. Landing in the Existing Code (no gate/ rewrite)

| Module                     | Changes                                                                                                                                                                                                                                           |
| -------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `agents/capture/source/`   | Keep source adaptation and capture-scope filtering; position ownership stays with the event source (`Bookmark`/`Checkpoint`), and policy changes never move positions (§4 decided)                                                                |
| `agents/capture/gate/`     | Keep the three gates' execution logic; rule sets already swap instances (the gateSet snapshot)                                                                                                                                                    |
| `agents/capture/config/`   | Local connection config and centrally distributed policy are layered: `capture.gates.*` file rules = fallback (when no center/nothing published; a central version fetch overrides wholesale once effective, noted explicitly in the startup log) |
| `agents/capture/runner/`   | Assemble the filter pipeline, atomic switching, filter statistics, and effective-version reporting (new `agents/capture/filter/` pure-logic package: event-level matching and filtering decisions, no MySQL driver/console API dependencies)      |
| Server agent control chain | Policy distribution acks, version sync, failure states (appliedRevision upstream)                                                                                                                                                                 |
| Console                    | Policy editing, preview validation, publish, rollback, and agent effectiveness status (ownership in the risk-control panel design)                                                                                                                |
| Alerts and subscriptions   | Reuse the existing alerting system; manage output scope and subscription permissions separately (DeliveryPolicy)                                                                                                                                  |

**Current baseline (delivered 2026-10-11)**: the `capture.gates` downstream pull chain (wire 0x04011B/C + configsync 30s + gateSet atomic hot-swap + server-side structural lint panel publish) = DetectionPolicy distribution chain v1; the panel's JSON edit card = policy editor v1. Policy-ization evolves on this baseline without overturning it.

## 8. Observability (filter statistics must be reported)

| Metric                         | Purpose                         |
| ------------------------------ | ------------------------------- |
| `events_received_total`        | Raw change events received      |
| `events_filtered_total`        | Events filtered (by reason)     |
| `events_evaluated_total`       | Events entering the rule engine |
| `policy_apply_errors_total`    | Policy application failures     |
| `rule_evaluation_errors_total` | Rule execution errors           |
| `policy_applied_revision`      | The currently effective version |

Purpose: judge whether filtering is too strict, whether rules have gone stale, and whether agents are missing detections due to config incompatibility. Current anchors: beyond Stats.Total/Insert/Update/Delete/Skipped/GateHits/AlertDropped, `FilteredNoMatch`/`FilteredError` (the first batch of events_filtered_total by reason), `Evaluated` (events_evaluated_total, counted only for gate evaluation after coarse filtering), `PolicyApplyErrors` (policy_apply_errors_total: counted when panel-distributed parse/validation failures keep last-good), and `RuleEvalErrors` (rule_evaluation_errors_total: CEP detection failures keep the event and count at the gate only, sweep-period aggregated, generalizing the SkippedNoDelta standard) are coded (phase two wrap-up, 2026-10-13); aggregate reporting and revision reporting follow in phase three.

## 9. Implementation Order (four phases)

1. **Contract (✅ delivered 2026-10-11)**: the four rule types' boundaries (the `filter.PolicyKind` closed set), the unified event model (`connector.ChangeEvent` with capability-difference fields), include/exclude semantics (exact + database-level wildcards, exclude wins), default deny, and the four-outcome failure classification (`filter.Result`).
2. **Agent execution layer (started 2026-10-11)**: EventFilter v1 coded and wired (operation allowlist + counters); remaining: metadata-presence filtering, gate evaluation error counting, and expanded filter-layer unit tests.
3. **Central policy management**: policy assetization (unified header + typed bodies), validation (capability/dependency-aware), versioning, distribution acks, and rollback; removing the hand-written YAML dependency for business rules.
4. **Panel and verification**: the policy management UI; tests for rule updates, disconnect recovery, position recovery, new-table enablement, filter errors, and cross-game/environment isolation.

Task breakdown in `todo.md`'s "CDC platformization batch".

## 10. Known Boundaries (an honest list)

- Phase one's contract surface is coded (`agents/capture/filter/`: policy.go's unified header + lifecycle + revision-advance semantics, match.go's database/table authorization + default deny + dependency check + four-outcome classification, 7 unit test groups); the remaining "should/recommend" items are not implemented. Contract decisions made while coding: include/exclude conflicts count only when both sides name the same exact table (a database-level exclude overriding an include = a legal whole-database-off gesture; exclude determinism wins); `MissingDependencies` conservatively reports everything missing for a nil matcher (no authorization info).
- The position-policy linkage is decided (§4, 2026-10-11): positions belong to the event source, new tables take effect incrementally from the current position, and history goes through a separate backfill flow; the backfill flow itself is not implemented (gated on the connector's `snapshot` capability, landing with the connector batch), and policy-switch auditing lands in logs first (agent logs; server-side audit extends with assetization).
- The appliedRevision reporting channel is live (2026-10-11, registration label piggyback + the panel's effectiveness reconciliation row); the lag window = 30s polling + re-registration — never treated as a strongly consistent read. Policy assetization (the unified header / immutable revision storage) remains phase-three work.
- DeliveryPolicy's subscription/masking surface has no detailed design (it reuses alerts + outlet's existing permission model; the extension is a later batch).
