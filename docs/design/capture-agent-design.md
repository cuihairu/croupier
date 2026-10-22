---
title: capture-agent Design Profile — Game-DB Change Capture and the Item Anti-Cheat Three Gates (Charter Pending)
---

# capture-agent Design Profile (Item Anti-Cheat Monitoring)

## Status

- Status: **C2 delivered (2026-10-11)**. The C1 skeleton (2026-10-10: registration/heartbeat/event stream/positions) + the three gates reconcile/cep/lineage (§8.1–8.3) + the alert uplink chain (§8.4) + gate-rule downlink pull (§8.5) + the panel editing surface (§8.6, closing the downlink loop) + coarse event-filter wiring (§8.7, CDC platformization batch phase two underway) + the appliedRevision report chain (§8.8, phase three's first item) + the rollback API (§8.9) + optimistic locking (§8.10, policy assetization v1) + moving business rules out of YAML (§8.11, making the fallback semantics explicit) + the dry-run validation endpoint (§8.12, pre-publish checks; validation truth unified with agent apply) have landed (`agents/capture/{source,gate,config,runner}` + `cmd/capture-agent`; `capture.enabled` defaults off). Later evolution (the filter-policy system / multi-database Connectors / the risk panel) is in the [CDC filter policy system](cdc-filter-policy-design.md), the CDC Connector architecture, and the risk-panel design.
- Naming (user decision, 2026-10-09): this agent is named **`capture-agent`** (the C of Change Data Capture); the old candidate cdc-agent is retired. The three agents are finalized as sidecar-agent / devops-agent / capture-agent; the panorama is in the [Agent inventory](agents-inventory.md).
- Charter terms (user directive, 2026-10-09):
  1. CDC (Change Data Capture) reads the game business DB's change log, **switchable**;
  2. Three gates: reconcile / CEP (Complex Event Processing) / lineage; anomalies enter croupier's existing alert page with rule-hit details;
  3. capture-agent joins the **existing agent registration chain**, its process supervised by the supervisor (process supervision);
  4. **A pluggable source-layer Provider**: MySQL (binlog) + PostgreSQL (logical replication) provided by default, with SQLite/SQL Server/MongoDB extension slots; the three gates' rule layer is decoupled from sources, and every source emits the **unified change-event stream**;
  5. **Minimalist**: rule allowlists/thresholds are all panel-configured, never hardcoded; the rule DSL picks the simplest option.
- Related: the [Agent inventory](agents-inventory.md) (three-agent naming/duty boundaries/panel pages/scope ownership) · the [agent-core common capability profile](agent-core-design.md) (capture-agent = core + business plugins as the carrier) · the [Agent Supervisor profile](agent-supervisor-design.md) (core is the supervised object; one protocol) · the database monitoring design (a sibling deployed against the same DBs: that one owns metric health, this one owns change compliance) · the Provider plugin design (the same pluggability principle) · the [common CDC core library and replay design](cdc-library-design.md) (the boundary between generic capture/filter/replay and Croupier business adaptation)

## 0. Purpose (Agent-inventory framing; read this section first)

| Dimension                  | Content                                                                                                                                                                                                                                                                                               |
| -------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| One-line duty              | **Prevent unauthorized DB changes**: read the game DB's change log and alert the moment a change happens — who, how, changed what, and whether business records back it                                                                                                                               |
| Deployment position        | Inside the game network, close to the game business DB (MySQL/PostgreSQL), co-located with or adjacent to sidecar-agent                                                                                                                                                                               |
| Monitored objects          | Row-level changes (insert/update/delete) to item-related tables (currency/items/equipment)                                                                                                                                                                                                            |
| Inputs                     | The source DB's change log (binlog / WAL logical replication) + panel-distributed rules (allowlists/thresholds, game-scoped config)                                                                                                                                                                   |
| Outputs                    | Alert events (into the existing alert page, with rule-hit details) + position/hit statistics reporting                                                                                                                                                                                                |
| Boundary with other agents | Owns **change compliance only**: no DB metric health (the db-monitoring sibling's surface), no function registration/invocation (sidecar-agent exclusive), no internal-network remote access / bastion (devops-agent exclusive; reframed 2026-10-11). On anomalies it **only alerts, never disposes** |
| Scope ownership            | Registration must carry `game_id`+`env` labels (unlabeled agents never reach the panel); alerts and change events are always scope-filtered, never crossing environments (see the Agent inventory §scope)                                                                                             |

## 1. The Problem

Abnormal granting of items (currency/items/equipment) is the most frequent money-grade incident in game operations: direct DB edits, manual compensation bypassing business records, bulk farming outside authorized windows, small repeated probes crawling under thresholds. Post-hoc audits rely on binlog replay and discovery lags by days. What is needed is not post-hoc forensics but **alerting the moment a change happens**: who (the connecting account), how (application writes vs manual SQL), changed what (db.table.row, before→after), and whether business records back it.

## 2. Positioning and Deployment Shape

```
Game business DB (MySQL/PostgreSQL)
   │ binlog / logical replication (WAL)
   ▼
capture-agent (a separate process, inside the game network, co-located with or adjacent to sidecar-agent)
   │  the unified change-event stream
   ├─▶ three gates (reconcile/CEP/lineage) ──anomaly──▶ croupier server alert page (the existing system)
   │
   └─ position persistence (GTID/LSN bookmark, crash-safe)
```

- capture-agent is a **standard agent**: it uses the existing registration chain (AgentRegister/heartbeat/task channel), registers with `game_id`+`env` labels, and is visible and start/stoppable on the panel.
- Process supervision reuses the supervisor: `managedProcesses` configures capture-agent itself, and crash auto-restart/backoff/circuit breaking/the event ring are all reused from S2 — **no new supervision is built**. Its own liveness/dependency-readiness probing uses `core/healthprobe` (Liveness/Readiness; DB connectivity is the Readiness semantic).
- Relationship with sidecar-agent: it can be a separate binary (`agents/capture/cmd/capture-agent`) launched and hosted by sidecar-agent, or co-located in one process (to be decided in a later phase; default = separate process + supervisor hosting — crash-domain isolation, so a CDC hang does not drag the function channel).

## 3. The Source Layer: Pluggable Providers (The Unified Change-Event Stream)

### 3.1 The Unified Event Contract (The Only Thing the Rule Layer Knows)

```proto
message ChangeEvent {
  string source_type = 1;      // "mysql" | "postgres" (closed set; expands with sources)
  string database = 2;         // database (PG uses the database+schema concatenation convention)
  string table = 3;
  string op = 4;               // insert/update/delete (closed set)
  bytes  before_json = 5;      // before image (update/delete have it; PG needs REPLICA IDENTITY FULL)
  bytes  after_json = 6;       // after image (insert/update have it)
  string gtid = 7;             // the source-native position identifier: MySQL GTID / PG LSN (part of the idempotency key)
  string account = 8;          // the connecting account (the lineage gate's core field; see §3.3's known boundary for PG)
  string client_addr = 9;      // connection origin (best-effort)
  int64  ts_unix_ms = 10;      // event time (milliseconds)
  string server_id = 11;       // the source instance identifier (multi-instance fan-out)
}
```

- Idempotency key = `gtid + table + primary key + op`: under at-least-once semantics, the gate/alert layers dedup by this key.
- The position bookmark is persisted (a local file + reported to the server, double-written); after a crash restart, reading resumes from the bookmark — **no lost events, idempotent dedup**, fitting naturally with the supervisor's auto-restart.

### 3.2 The Source Provider Interface

```go
// 每个 Provider 只做一件事：把源日志翻译成 ChangeEvent 流。
type ChangeSource interface {
    Open(ctx, SourceConfig, bookmark) (eventCh <-chan ChangeEvent, err)
    Bookmark() []byte        // 当前位点（崩溃前落盘）
    Close(ctx)
}
```

- Switch: `capture.enabled: false` defaults off; when enabled, subscription is further narrowed per database by the `tables` allowlist (subscribing only to item-related tables — controlling both noise and bandwidth).

### 3.3 Source Maturity and Integration Cost (Documented; the Selection Basis)

| Source     | Mechanism                               | Library                                           | Maturity                                                                                                 | Preconditions                                                                                       | Known boundaries                                                                                                                                                                                                                |
| ---------- | --------------------------------------- | ------------------------------------------------- | -------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| MySQL      | binlog ROW mode                         | github.com/go-mysql (siddontang; canal's lineage) | **High** (the de facto standard in China's game industry; large community)                               | `binlog_format=ROW`, `binlog_row_image=FULL`, REPLICATION SLAVE privilege, a dedicated server_id    | **binlog events carry no session account** (verified in C1: go-mysql replication events have no session account field) — the account field is always empty in C1 and the lineage gate degrades per §3.3's boundary; GTID native |
| PostgreSQL | Logical replication (pgoutput)          | github.com/cockroachdb/pglogrepl + jackc/pgx      | **Medium-high** (pglogrepl is the logical-replication protocol implementation maintained by CockroachDB) | `wal_level=logical`, a publication, a replication slot, `REPLICA IDENTITY FULL` (for before images) | **WAL carries no session account** — the lineage gate's account field has no native value on the PG side; table-level consistent snapshots use a temporary slot's exported snapshot; `schemaHistory` unimplemented, see below   |
| SQLite     | (No log stream) trigger-mirrored tables | In-house trigger→change table polling             | Low                                                                                                      | An AFTER trigger on every monitored table                                                           | Schema-invasive; kept as an extension slot, not in the default two sources                                                                                                                                                      |
| SQL Server | Built-in CDC / Change Tracking          | The official sqlserver driver                     | Medium                                                                                                   | Version/licensing dependencies                                                                      | Extension slot                                                                                                                                                                                                                  |
| MongoDB    | Change Streams                          | mongo-driver                                      | Medium                                                                                                   | A replica set                                                                                       | Extension slot                                                                                                                                                                                                                  |

**The PG lineage boundary (honestly labeled)**: PG logical decoding output does not include the session role that executed the transaction. Optional completion paths: ① a pgaudit/`log_statement` side-channel join (deployment cost +); ② on-DB triggers writing an audit table (invasive); ③ the lineage gate degrades on the PG source to "connection origin unknown; only reconcile + CEP active". The default lands as ③ with the degraded state marked on the panel; ①② stay as future enhancements — **never silently pretending the account field exists**.

## 4. The Three Gates (The Rule Layer, Decoupled from Sources)

All gates consume the same event stream, evaluating per event and emitting a "hit detail" structure into alerts.

### 4.1 Gate One: Reconcile (Item Changes ↔ Business Records)

- **Semantics**: item-table changes must JOIN to corresponding business records (orders / mail attachments / GM compensation tickets, etc.) within a time window; no record = anomaly.
- **Config surface (panel)**: the item-table list + each table's "record-association rule" — the record table, JOIN keys (e.g. `item.change_reason_id ↔ ledger.trace_id`), the time window (default ±5 minutes), and an exemption list (e.g. the offline settlement batch account).
- **Simplest implementation**: events enter a (table, join_key) pending-reconcile queue; the record side is likewise subscribable (the record table is in the subscription list), matched bilaterally within the window and written off; un-written-off past the window = alert. **No reconcile engine is introduced — a ring window + a map suffices.**

### 4.2 Gate Two: CEP (Temporal Patterns; Three Built-in Patterns to Start)

- **Off-allowlist time windows**: table/operation × time-window allowlist (e.g. item tables allow bulk changes only 02:00-04:00; changes at other times alert immediately).
- **Volume spikes**: a single event's delta, or a sliding window's cumulative delta, over the threshold (absolute or percentage baselines configured per table/field).
- **Continuous probing**: N small changes (each below the spike threshold) from the same (account, table) within a sliding window — the low-frequency slow-drain shape, implemented with a pure counter.
- **Simplest implementation**: fixed pattern templates + parameters; **no CEP engine** (Flink/Esper-class options all rejected). The sliding window uses ring-bucket counting, memory O(configured cardinality).

### 4.3 Gate Three: Lineage (Who Is Writing)

- **Semantics**: distinguish "application writes" from "manual SQL": a non-allowlisted account writing to a monitored table = anomaly (manual direct DB edits bypassing the approval flow).
- **Config surface (panel)**: an account allowlist (at db/table granularity), with application accounts and ops-exempt accounts listed separately.
- On both MySQL and PG, `account` is not natively obtainable (binlog/WAL events carry no session account — verified in C1); the lineage gate's account field is always empty in C1, and both sides degrade per §3.3's boundary.

### 4.4 Hit Details (The Alert Payload)

```
rule: lineage.whitelist_violation | reconcile.missing_ledger | cep.burst_delta | cep.off_hours | cep.probing
severity: warn | critical (panel-configured per rule)
event: the ChangeEvent's key fields (table/row primary key/delta/before→after summary)
context: account, GTID, summaries of adjacent events in the window, a snapshot of the rule parameters
```

## 5. The Rule DSL Picks the Simplest (User Directive: Pick the Simplest)

**Conclusion: v1 builds no expression language; it builds "parameterized rule templates".**

- All rule shapes of the three gates converge into a **fixed template set** (the five built-in patterns above); each rule = `{template, table list, params(json), exemption list, severity, enabled}`.
- The template set is code (the gates' implementation); rules are pure data — no parser, no expression evaluation, no script-injection surface, no DSL version drift.
- Panel editing = structured forms (the template determines the fields), not writing code in a text box.
- Reserved: if complex boolean combinations are genuinely needed, phase two evaluates a restricted expression language (google/cel-go class), but it **does not enter by default** — this repo's history (openapi derivation, the migration DSL) shows the first thing expressiveness buys is always an escape surface and debugging cost.

## 6. Config and Distribution (Allowlists/Thresholds Never Hardcoded)

- Rule and source-subscription config is stored as **server-side game-scoped config** (reusing the existing game DB config resource mechanism), CRUD from the panel.
- Distribution reuses the agent's existing **config-sync pull channel** (the extension sync puller family): rule version polling → pull on change → hot effect; capture-agent caches the last version locally and keeps running on the old rules while disconnected (degraded, never interrupted).
- Rule hit statistics (trigger counts per rule) ride regular metrics reporting; the panel shows "which rules are firing".

## 7. Alert Integration (The Existing System)

- capture-agent adds **alert event reporting** over the existing agent→server uplink channel (a new message type; a same-family wire protocol extension); the server lands it in the existing alert storage/alert page, reusing silence/claim/escalation — **no separate alert surface**.
- The alert page shows: the source agent, rule-hit details (§4.4), the event summary, and jump parameters.
- The panel's **capture-agent page** (see the [Agent inventory](agents-inventory.md) §panel): instance list + the live change-event stream and hit alerts, all filtered by `game_id`+`env` scope; the log entry reuses the supervisor's log download chain.

## 8. Phases

| Phase | Content                                                                                                                                                                                               | Dependency                      |
| ----- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------- |
| C1    | The agent-core skeleton + the capture-agent skeleton (registration/heartbeat/supervision hookup) + the MySQL source + the unified event stream + position persistence; `capture.enabled` defaults off | The agent-core profile approved |
| C2    | Gate three lineage + gate two CEP (off_hours/burst_delta/probing) + rule templates and the panel config surface + alert reporting into the alert page                                                 | C1                              |
| C3    | Gate one reconcile (record subscription + window write-off) + the PG source (including the lineage degraded-state label)                                                                              | C2                              |
| C4    | Extension-slot evaluation (SQLite trigger mirror / SQL Server / Mongo) + the pgaudit lineage enhancement evaluation                                                                                   | As needed                       |

**Phase ordering (decided 2026-10-11)**: the table above is the original design batch order; actual progression follows the user's gate order **reconcile → cep → lineage** (reconcile moves from C3 to the front of C2; lineage last). The regrouping changes no gate's dependencies — only the progression and acceptance order.

### 8.1 C2 Delivery Record: Gate One · Reconcile (2026-10-11)

Code surface: `agents/capture/gate` (the rule evaluation layer, decoupled from sources) + `agents/capture/config`'s `capture.gates.reconcile.rules` (file config, pure data) + `agents/capture/runner` wiring (`Evaluate` inlined in consume, `Sweep` in the position tick, hits → `slog warn` + `Stats.GateHits`). Rule ID `reconcile.missing_ledger`, severity defaulting `warn`.

Implementation differences from the original design (honest list, each with its reason):

1. **Key shape**: §4.1 expresses JOIN keys as field names (`item.change_reason_id ↔ ledger.trace_id`); C1 row images are positional JSON arrays with no column-name mapping (§9's last item), so the `itemKey`/`ledgerKey` spec is an **integer string = the row-image position index** (e.g. `"1"`). The field-name shape stays forward-compatible: on array row images a value that cannot be fetched is treated as "key missing"; once column-name mapping (information_schema ingestion) arrives, the same spec string switches to field names seamlessly — no config change needed.
2. **Bilateral write-off**: the item side enters the debit queue; the record side first writes off the earliest same-key debit, or hangs as a credit if none is pending (the ordering where records arrive before item changes); when an item change arrives it likewise first writes off a same-key credit. Credits past the window are **silently cleared** — a record without a corresponding item change is not an anomaly (e.g. refund reversals, GM recalls).
3. **Missing-key degradation**: when the item side cannot fetch a key (a NULL join key, or a key spec mismatched to the row image's column count), it enters the window as a never-match sentinel and alerts at window expiry (an empty hit key is that signal) — better false positives than misses; when the record side cannot fetch a key it is skipped and counted in `SkippedNoKey` (the write-off target cannot be located; usually a misconfigured spec, for ops to fix the config).
4. **Exemption degradation**: §4.1's exemptions are per account (the offline settlement batch account); C1 `account` is always empty (§3.3), so this batch's exemptions are **exact join-key value exemptions** (`exemptKeys`). Account-dimension exemptions return once a lineage data source is solved (including the pgaudit / MySQL general-log side channels), with the panel config surface synced.
5. **Capacity boundary**: a single rule's in-window entries cap at `maxPending` (default 10000); past the cap, eviction is FIFO by entry order — a debit eviction produces a hit (`detail` marked `cap exceeded`), a credit eviction is silent; memory stays bounded and the stream is never blocked.
6. **Hit outlet**: §7's hits-into-the-alert-page is outside this batch — this batch's hits land only in the agent log (fields `rule`/`severity`/`table`/`key`/`detail`/`gtid`/`tsUnixMs`) + the `Stats.GateHits` counter. The server-side outlet kind `capture.rule_hit` closed set is ready (`internal/platform/outlet`); the alert-report wire message is a separate increment (§7 wiring).
7. **Validation surface**: rules are fully validated at startup by `config.Load` (a process that refuses to start beats runtime misjudgment): tables must be two-segment `db.table`; the same table must not be reused across rules (the item/record role routing must be unambiguous); `itemTable ≠ ledgerTable`; `severity ∈ {warn, critical}`; `windowSec`/`maxPending ≥ 0`. §6's panel config surface is not yet wired; this batch is file config only.
8. **Concurrency contract**: the gate's internal state is lock-free, relying on the runner's single-goroutine consumption loop (`Evaluate` on the event branch, `Sweep` on the position-tick branch — serialized in one loop).
9. **Time semantics**: the window-entry moment = `max(event TsUnixMs, now)` — a binlog replay backfill (a position-invalidation rescan) does not instantly expire the window and produce mass false positives; expiry is judged as `now - ts >= windowSec`.

Tests: `agents/capture/gate/reconcile_test.go` 13 cases (bilateral matching, window-expiry hits, window boundaries, exemptions, both sides' missing-key degradation, insert/update/delete row-image selection, replay backfill, capacity eviction, multi-rule isolation, deterministic hit ordering, the validation error set) + `runner_gate_test.go` 3 cases (wiring, pass-through when unconfigured, config-validation interception).

### 8.2 C2 Delivery Record: Gate Two · CEP (2026-10-11)

Code surface: `agents/capture/gate/cep.go` with three templates (`cep.off_hours` / `cep.burst_delta` / `cep.probing`) + the `capture.gates.cep.{offHours,burst,probing}` file config + runner wiring (evaluate-on-arrival, hit immediately; the position-tick Sweep only does bucket hygiene). Unlike the reconcile gate, **the same table may carry multiple instances** (time windows + spikes coexisting is normal) with no role-routing uniqueness constraint.

The three templates' semantics and differences:

1. **off_hours**: `windows` are per-day minute-offset segments (`startMin > endMin` = crossing midnight; left-closed right-open); `tzOffsetMin` is explicit config (0=UTC; UTC+8 is 480) — no IANA timezone library is introduced, keeping the agent binary free of a tzdata dependency. Judgment uses **event time** (the source's ground truth): a position-invalidation rescan will re-alert on historical windows (a rescan is itself an abnormal path; acceptable); hit details carry the local clock and tz offset for human verification.
2. **burst_delta**: `mode=single` judges a single event's `|delta|`; `mode=window` judges a sliding window's cumulative `|delta|` (spikes in both directions count). Delta semantics: insert = the after-image value (the full new amount), delete = the before-image value (the full cleared amount), update = after − before. **A hit zeroes the accumulator and restarts the timer** (one hit per window — no per-event spam). The accumulator buckets are pruned by event time + an `maxEvents` capacity (default 10000, FIFO dropping the oldest).
3. **probing**: the key degrades to **(row key, table)** — the design's original (account, table) is unobtainable because C1 `account` is always empty (§3.3). Repeated small changes to the same row hit; a large change interrupts that key's sequence (large amounts belong to burst); a hit resets that key's window (one hit per key per window). `maxTracked` key capacity (default 10000, deterministic FIFO eviction).
4. **The amount field**: `deltaField`'s key shape matches the reconcile gate (an integer string = the position index); a non-numeric value → skip and count `SkippedNoDelta` (a misconfigured-spec signal). float64 precision: integer amounts < 2^53 are safe; currency volumes above that (>9e15) lose precision — games storing money in cents are unaffected.
5. **Percentage baselines** (mentioned in §4.2) are not done in v1: a baseline needs a historical-distribution source (analytics/panel side); the event stream cannot supply baseline values. Absolute thresholds go first; baselines are evaluated with the panel config surface.
6. **Ordering boundary**: CEP hits assume events arrive in order (binlog guarantees a single stream's order); an agent restart loses bucket state (§9's existing boundary — probing/burst windows restart from zero).

Tests: `agents/capture/gate/cep_test.go` 14 cases (inside/outside windows, crossing midnight, tz offsets, operation filters, severity; single vs cumulative spikes; insert/delete full-amount deltas; hit-zeroing; over-window pruning; probing counts and resets; key isolation and large-amount interruption; Sweep bucket hygiene; key-capacity FIFO eviction; the validation error set; multiple instances on one table) + 2 runner wiring cases.

### 8.3 C2 Delivery Record: Gate Three · Lineage (2026-10-11)

Code surface: `agents/capture/gate/lineage.go` (`lineage.whitelist_violation`) + the `capture.gates.lineage.rules` file config + runner wiring. Semantics:

1. **The account allowlist**: `allowAccounts` is required non-empty (it must include at least the application write account — a lineage rule with an empty allowlist = alerting on every write, a config error blocked at startup).
2. **The C1 degraded state (the main difference)**: binlog/WAL events carry no session account; `account` is always empty (§3.3) — empty-account events are **skipped and counted as `Degraded`** (no false positives: an alert with no attributable subject is pure noise; nothing missed: once a source-side account channel appears, this gate takes effect with zero changes). When the gate is enabled, the runner's startup log explicitly marks `degraded: source has no session account`; the panel's "degraded state" label comes with the panel-config batch.
3. **Degraded ≠ gate off**: rules and validation are delivered in the with-account shape; events carrying an account are judged against the allowlist immediately — once the general log / audit plugin side channels (C4 evaluation items) are connected, the gate needs no change.
4. Operation filters and table routing match CEP (an empty `ops` list = all changes; non-subscribed tables pass through and do not count as degraded — the degraded counter reflects only real evaluation volume).

Tests: `agents/capture/gate/lineage_test.go` 5 cases (hits outside the allowlist, pass-through inside it, the empty-account degraded count, operation filters and table routing not miscounted, the validation error set).

### 8.4 C2 Delivery Record: The Alert Report Chain (2026-10-11)

Code surface: the wire pair `MsgAlertEventRequest/Response = 0x040119/0x04011A` (the next free pair in the Ops family; odd request/even response, satisfying the `IsRequest`/`GetResponseMsgID` contract); the proto message pair `opsv1.CaptureRuleHitReport` (agent/game/env/rule/severity/table/key/detail/gtid/event_op/event_ts_unix_ms) + `CaptureRuleHitAck` (accepted/duplicate); the agent's `controlconn.SendAlertReport` + the runner's async report loop (`startHitSender`: `hitCh` buffered 512, non-blocking enqueue dropping when full, a 5s per-send timeout, enqueued send failures retried at 500ms up to 3 times then dropped, the connection following Dial's reconnection); the server's `ControlService.handleAlertEvent` (`SetAlertModel`/`SetOutletManager` injected, assembled in cmd/server/root.go). Semantics and boundaries:

1. **Idempotency key**: `AlertID = "capture." + sha256(agent|rule|gtid|table|key|ts)`, first 16 hex — safe for `model.Alert.AlertID` (a `size:64` unique index); reconnect replays fold server-side into `duplicate` (reflected in the ack, not re-stored).
2. **Level normalization**: the gate layer's severity closed set `{warn, critical}` maps to `outlet.SeverityWarning/Critical`; unknown values are defensively treated as warning (never inventing a new level).
3. **game/env go only into Details**: the `Scope` triple rides the event to the outlet (the panel filters by scope), but the alerts table stores no game identity columns (the table is company-wide single-DB) — the same shape as `alert.inbound` (scope in Details).
4. **Persistence without backpressure**: a hit is first atomically written to the local alert spool (idempotency key = `agent|rule|gtid|table|key|event_ts`, capacity 4096), then pushed into the in-memory fast channel; a full queue does not drop — the spool replay loop reports in order. Only undumpable disk/capacity, or retries exhausted after successful persistence, count as `AlertDropped`. An ACK must have `err=nil` and `accepted=true` (a duplicate ACK must also be accepted); records are deleted after ACK. Without a valid ACK after sending, or across shutdown/restart, records are kept and replayed in order after restart, with the server folding duplicate business alerts by the same idempotency key. The checkpoint advances only after event-consumption confirmation; the alert spool is retained independently, unaffected by checkpoint cleanup.
5. **Mixed-version compatibility**: an old server returns an unknown message type error for the new msgID, and the agent drops it as a send failure (as above); a new server + old agents are unaffected (old agents never send it). No config switch — the gates off means no hits and no noise; gates on means reporting is enabled.
6. **The server's dual-storage semantics**: `alertModel` nil → an explicit error (same as taskStore); `outletManager` nil → store only, no dispatch (degraded when the outlet chain is not assembled).

Tests: server `internal/server/control_handler_alert_test.go` 7 cases (storage + idempotent folding, severity mapping and normalization, validation rejections, nil-model rejection, the occurredAt default, outlet dispatch with scope/dedup, the msgID pair contract); runner `runner_alert_test.go` 5 cases (payload completeness, send-failure counting, retry-after-failure success, the duplicate ack, dropping when the queue is full).

### 8.5 C2 Delivery Record: Gate-Rule Downlink Pull (The Panel Config Surface's Wire Half, 2026-10-11)

Code surface: the wire pair `MsgCaptureConfigRequest/Response = 0x04011B/0x04011C` (continuing the Ops family); proto `opsv1.CaptureGatesConfigRequest` (agent_id) / `CaptureGatesConfigResponse` (version/gates_json); the server's `ControlService.handleCaptureConfig` (`SetConfigVersionModel` + `SetGameDBResolver` injected, assembled in root.go); the agent's `controlconn.PullCaptureGates` + the runner's gate hot-update (the `gateSet` snapshot swapped wholesale via `atomic.Pointer`; consume/sweep read the pointer in the single goroutine while the puller goroutine writes — lock-free, preserving §8.1's concurrency contract) + the `core/configsync.Puller` version poll (30s default). Semantics and differences:

1. **Channel choice (REST rejected)**: sidecar's `GET /api/v1/agents/{id}/extensions` sits behind the JWT Authority + `requireReadPermission`, and agents have no JWT — the downlink is a **wire pull over the registered TCP session** (registration is authentication; the same channel as the alert uplink).
2. **Scope from the session, not the request**: the server resolves the agent's ownership from `AgentSession{GameID,Env}` (`req.agent_id` only addresses the session); config_versions is read by `FindLatestByScope("capture.gates", gameID, env)` — an agent cannot pull another scope's rules; an unregistered session or missing game is rejected outright.
3. **gates_json is opaque to the server**: the server only stores and forwards (the `config_versions` key `capture.gates`, namespace=ops; no schema change); the gate-layer structure is fully validated by the agent at apply time (the validator of last resort) — the same validation function set as the config file (`gate.Validate*Rules`); any illegal gate **rejects the whole payload** (keep last-good, never a partial swap).
4. **Hot-update semantics**: `buildGateSet` validates first, then builds, then swaps the pointer wholesale; on apply failure the version cursor does not advance (retried next round); an empty version (nothing published server-side) = a no-op keeping the status quo; when the connection is not ready it stays silent at Debug (registration incomplete / reconnect windows are normal), while real read errors warn.
5. **multiGame DB split**: the server resolves the game DB per (game,env) via `GameDBResolver` (structurally satisfying `*router.Router`; the control package does not import internal/db) and injects it via dbctx — rules live in the game vendor's own DB; in single-DB mode nothing is injected and the default connection is used. A split-DB resolution failure / rule read failure uniformly returns an empty version (= the agent keeps the status quo); the pull is periodic polling, and one round's read anomaly is never amplified into a channel error.
6. **The gate layer snapshot-ized (behavior unchanged)**: the runner's three gate fields converge into the `gateSet` snapshot; startup still = file rules; a panel publish hot-swaps them — this batch is §6's distribution channel landed agent-side; the panel editing surface (the web card + the configs API writing config_versions) follows in the next batch.

Tests: server `control_handler_config_test.go` 7 cases (latest-version passthrough, unpublished returns empty, scope isolation, unknown agent/nil model rejection, split-DB override in effect, split-DB resolution failure returns empty, the msgID pair contract); runner `runner_config_test.go` 3 cases (hot snapshot swap, illegal rules keep last-good, and the puller's version skip/empty payload/error/illegal rule/new-version full path).

### 8.6 C2 Delivery Record: The Panel Editing Surface (Closing the Downlink Loop, 2026-10-11)

Code surface: REST `GET/PUT /api/v1/ops/capture/gates` (the ops group, scoped + Authority) reading/writing the `config_versions` key `capture.gates` (namespace=ops, `CreateWithMeta` incrementing the optimistic lock); the server does **structure lint only** (the reconcile/cep/lineage three-section object + array shapes; a lint failure is 422 `errorx.CodeError`); the web `/ops/capture` page (a v1 JSON textarea + insert-template + a publish Popconfirm + unpublished/version tags + a "publish ≠ effective" boundary Alert) + `services/api/opsCapture`; Ops/Nodes gained a "Capture" column (the registration label `captureLineage=degraded` → a gold "lineage degraded" badge, reusing §8.3's existing labels — no new API). Semantics and differences:

1. **The server structure-lint / agent semantic-validation layering is unchanged**: the server checks only shapes (objects/arrays), never interpreting rule content — semantic truth remains the agent apply's full validation (the validator of last resort, continuing §8.5 item 3); a successful panel publish ≠ the agent has applied; the truth is the agent log's `capture gates version applied`.
2. **Publishing requires a complete game+env scope**: an empty-scope publish would be pulled by every agent (FindLatestByScope requires the triple non-empty) with no attributable owner — rejected outright with 422.
3. **The degraded badge needs zero new APIs**: the lineage degraded state rides the registration-labels chain (OpsAgentInfo already exposes it); the panel column only maps it for display.
4. **The v1 editor is a JSON textarea**: the structured form (template + params + tests) belongs to the risk-panel design's phase four; this document delivers only the minimal editing surface needed to close the downlink loop.
5. **appliedRevision is not yet reported**: the panel has no "pending effect" visibility; the uplink channel comes with the [CDC filter policy system](cdc-filter-policy-design.md) phase three.

Tests: ops `capture_gates_test.go` 4 cases (incrementing versions + metadata, unpublished returns empty, scope isolation, 5 lint-failure shapes + missing scope + no storage after lint failure); the web page 7 cases (published/unpublished rendering, template insertion, the publish chain, publish failure, load failure, reload) + 3 service cases (URL/method/param shapes); the Nodes 75-case regression green.

### 8.7 Delivery Record: Coarse Event-Filter Wiring (CDC Platformization Batch Phase Two Underway, 2026-10-11)

Code surface: the new contract package `agents/capture/filter` (`policy.go` unified policy header + `match.go` db/table authorization + `event.go` EventFilter + the `connector` package's ChangeEvent/SourceConnector/capability declaration, phase one); runner wiring (`config.Gates` gains the `event.ops` section → `gateSet.event` → coarse filtering before gates in consume); Stats gains `FilteredNoMatch` / `FilteredError` / `Evaluated` / `PolicyApplyErrors` / `RuleEvalErrors` (the filter-policy profile §8 definitions, completed at phase two's close on 2026-10-13); the panel lint recognizes the `event.ops` array shape. Semantics and differences:

1. **EventFilter v1 does operation-type allowlisting only**: db/table scope belongs to SourceFilter (whose authorization semantics default-deny — see the [CDC filter policy system](cdc-filter-policy-design.md) §1.1/§1.2); event filtering is allow semantics (an empty allowlist = allow all). The two semantics differ and share no matcher, preventing boundaries like "an exclude-only config being misdenied by default".
2. **A missing section = allow all** (backward compatible): existing capture.gates JSON and file configs without an `event` section give gateSet.event=nil — pass-through at zero cost, zero behavior change.
3. **Failure semantics**: a legal operation not on the allowlist = dropped as FilteredNoMatch; an unknown operation (unjudgeable shape) = dropped as FilteredError — drops are observable via counters, never silent (profile §5). The red line that gate-evaluation failures retain the event is unchanged, and detection-failure counting has entered the unified surface: `Stats.RuleEvalErrors` (aggregating the CEP gate's SkippedNoDelta increments per sweep cycle; a hot-swapped new gate instance resets its baseline without inflation).
4. **Atomic switching for free**: the event filter hangs inside the gateSet snapshot, so §8.5's wholesale hot-swap/keep-last-good semantics apply to the filter layer too — no new concurrency surface.

Tests: the filter package 3 groups (policy header / authorization match / event filter) + the connector package 7 groups; runner 4 cases (wiring and counting, unknown-operation Error counting, illegal sections keep last-good, buildGateSet closed-set rejection); ops lint 2 new cases (an illegal event shape 422, a legal one passing).

### 8.8 Delivery Record: The appliedRevision Report Chain (CDC Platformization Batch Phase Three's First Item, 2026-10-11)

Code surface: the runner gains `appliedGatesRevision` (an atomic.Pointer[string], written by the puller's `WithOnApply` callback) + `reg` (a register.Client handle, assembled in Run); after taking effect, an async `SyncOnce` re-registers, with `buildPayload` carrying the applied version in the `captureGatesRevision` label — the server session labels are fully refreshed through the existing register/UpsertAgent path: zero server changes, zero proto changes; the panel's `/ops/capture` gained an "effect reconciliation" row (`listOpsNodes` labels reconciling desired vs applied: all applied green / partially applied gold tag + tooltip naming them / no agent gray). Semantics and differences:

1. **Channel choice (reusing the registration surface, no new wire pair)**: appliedRevision is a low-frequency, low-rate state (once per publish taking effect), and the registration-labels surface already carries such data (the captureLineage precedent); the heartbeat proto has no labels and extending HeartbeatRequest is a bigger surface — a re-registration refreshes it, semantically equivalent with the smallest implementation surface.
2. **Best-effort, no rollback**: a failed re-registration does not roll back applied rules (the hot swap already happened); the next heartbeat's disconnect-reregister naturally catches up; the panel reconciliation tolerates lag (30s pull + re-registration) and is not a strong-consistency read.
3. **Apply failure does not advance**: when apply fails (illegal rules keep last-good) the onApply callback does not fire and appliedGatesRevision keeps the last applied value — the panel never shows an unapplied version as applied.
4. **Empty semantics**: before any panel version has taken effect, no label is carried (panel-side = file rules; the reconciliation row renders only when desired>0).

Tests: runner_revision_test 3 cases (the label following the puller's advance / apply failure keeping the last applied value / fetch failure not recording + tolerating a missing reg); the web page 4 new cases (all applied / partially applied / reconciliation failure degradation / no reconciliation row when unpublished).

### 8.9 Delivery Record: Rollback (Republishing a Historical Version, CDC Platformization Batch Phase Three, 2026-10-11)

Code surface: REST `POST /api/v1/ops/capture/gates/rollback` (body `{version, message?}`) — `FindByScope` fetches the target version's content → re-runs the current structure lint → `publishCaptureGates` lands it as a new version (the message auto-prefixed `rollback from version N` + an optional reason); PUT and rollback share the publish path (lint + scope check + CreateWithMeta converged in one place). Semantics and differences:

1. **The version log is immutable**: rollback rewrites no history and deletes no version; the target version's content is copied and published under a new version number — the agent side keeps its usual 30s version-poll hot effect, introducing no third downlink shape (no "rollback command" wire pair).
2. **Historical content re-passes the current lint**: lint rules evolve server-side; rolling back to content the current rules would reject is an explicit 422 — never silently publishing a bad version (a unit test locks the legacy-bad-row direct-insert scenario: a failed rollback lands no new version).
3. **Scope isolation as usual**: a cross-game/env rollback = 404 (not found means nonexistent; never leaking another scope's version existence).
4. **Rolling back the current version is legal**: republishing identical content = an explicit re-application (the version keeps incrementing; the effect-reconciliation row runs its applied→re-reported chain).

Tests: ops `capture_gates_test.go` 3 new cases (historical content republish + the message prefix + history intact + rolling back the current version increments; an illegal version 422 / nonexistent 404 / cross-scope 404 / missing scope 422; lint-evolution rejection + failure stores nothing). Boundary: the panel-side rollback button is not done (belongs to the risk policy page, phase four; the REST is ready).

### 8.10 Delivery Record: Optimistic Lock Wired (Policy Assetization v1, CDC Platformization Batch Phase Three, 2026-10-11)

Code surface: `CaptureGatesPutRequest` gains `baseVersion` (the latest version seen when the editor loaded) → `publishCaptureGates(..., baseVersion)` → `ConfigVersionPayload.BaseVersion` optimistic lock; `model` extracts the sentinel `ErrConfigVersionConflict` (the error text unchanged; judged by `errors.Is`) → the service layer maps it to 409; the panel's publish sends `state.version` as the baseline. Semantics and differences:

1. **Baseline 0 = lock disabled** (backward compatible with existing callers); the rollback endpoint does not use the lock — its target is an explicit version number with content taken from server history rather than a stale editing state; concurrent rollbacks take effect in version-chain order.
2. **Conflict = 409 + the message passed through**: when two people publish from the same old content, the latter fails; reload and re-edit (no auto-merge — merging whole-content semantics is meaningless).
3. **The prior-version chain made explicit**: version = baseVersion+1; a conflict breaks the chain; the version log remains append-only immutable (rollback records in §8.9).

Decision: policy assetization continues on `config_versions` + `namespace=ops` (no new table, no model change, zero goose migrations) — the storage-side upgrade of "a unified policy header + per-typed bodies" waits until keys are split per type (source/event/detection/delivery each with their own key), when policy records need to carry their own policy header as independent deliverables; embedding the header now would create a second truth for revision/scope — not done.

Tests: ops `TestCaptureGatesPutOptimisticLock` added (stale baseline 409 / a correct baseline increments / the default baseline stays backward compatible / an empty chain with a baseline 409 / a conflict stores no version); the web page +1 case (conflict message passthrough), the service +1 case (the baseVersion shape); tsc 0 errors, the targeted suite's 16 cases green.

### 8.11 Delivery Record: Business Rules Moved Out of YAML (Fallback Semantics Made Explicit, CDC Platformization Batch Phase Three, 2026-10-11)

Code surface: behavior unchanged — the file `capture.gates.*` was always the fallback ("effective at startup, wholly overridden once a central version is pulled and applied"; `fetchConfig` on an empty version → nil, silently skipped, with file rules as the backstop); this delivery makes that precedence chain explicit: the runner's startup log `capture file rules loaded as fallback` (with each gate's rule count, preventing file rules being misread as the standing truth) + a `config.Gates` doc comment + the precedence chain locked by unit tests. Semantics and differences:

1. **No config key rename**: the `capture.gates.*` keys stay in place (compatible with existing deployments); "distinguishing the local connection from central distribution" is achieved through fallback semantics rather than relocating keys.
2. **The precedence chain has tests**: file rules (event.ops=delete) effective at startup → the central version (event.ops=insert) applied by PullOnce wholly overrides + appliedRevision follows; unpublished (empty version) → no action, file rules keep backing, revision not recorded.
3. **Phase three closes here**: assetization v1 (§8.10's optimistic lock) + rollback (§8.9) + appliedRevision (§8.8) + the position decision (filter profile §4) + this item; the rest (per-type keys / header embedding / capability-aware lint) goes to phase four and the Connector batch.

Tests: `runner_precedence_test.go` 2 new cases (central overriding file + revision following / unpublished backstop retained). The whole capture package regression green.

### 8.12 Delivery Record: The Dry-Run Validation Endpoint (Pre-Publish Checks, CDC Platformization Batch Phase Four, 2026-10-12)

Code surface: REST `POST /api/v1/ops/capture/gates/validate` (body `{content}`, no scope dependency) — shallow lint + **`config.ValidateGates`** (the new shared validation truth: the static Validate* + construction-time New* (e.g. reconcile table-routing uniqueness) + event-coarse-filter legality; it validates only, never constructing runtime gate objects); `runner.buildGateSet`'s validation sequence was refactored to call the same function (behavior unchanged). On success it returns `{valid, reconcile, cepOffHours, cepBurst, cepProbing, lineage, eventOps}` rule-count previews; on failure 422, with error text identical to what agent apply would report. The web `/ops/capture` page gained a "Validate" button: results render in place as an Alert (success green with per-gate counts / failure red passthrough), and any edit invalidates the verdict. Decisions and semantics:

1. **Validation truth unified**: dry-run and agent apply share `config.ValidateGates` (a single implementation; `runner_gateset_pinning_test.go` locks the mirror case-by-case by error text — changing either side alone explodes); the server's old "never interprets rule semantics" stance (§8.6 item 1) is succeeded by "explicitly importing the agents/capture/gate validation composition" — semantic truth moves from the agent apply moment to an optional pre-check before publish.
2. **The publish path is unchanged (since tightened by §8.13)**: PUT originally did structure lint only; after §8.13's server-side hardening the publish path also runs semantic validation and dependency/capability-aware lint (the pre-check remains an explicit editor action; the semantic interception point tightened from "optional pre-check" to "mandatory at publish").
3. **What it does not solve**: runtime facts such as table existence / account validity are not promised by dry-run; a passing validation ≠ the agent has applied (effect reconciliation still reads appliedRevision).

Tests: `agents/capture/config/gates_validate_test.go` 7 cases (all gates legal / static rejections / duplicate-routing rejections / cep / lineage / event.ops / an empty set legal) + `runner_gateset_pinning_test.go` 7 mirror-locking cases + ops `TestCaptureGatesValidate` (legal counts / lint failure / deep failure / duplicate-routing failure / empty content 422 / dry-run stores nothing) + 3 web-page cases / 1 service case. The full chain regression green.

### 8.13 Delivery Record: Publish Validation Server-Side (Dependency + Capability-Aware Lint, CDC Platformization Batch Phase Three Close-Out, 2026-10-13)

Code surface: `publishCaptureGates`'s publish path adds three checks atop structure lint — `config.ValidateGates` (§8.12's semantic truth tightened from optional pre-check to mandatory at publish; the rollback shares the path and is equally blocked) + `lintCaptureGatesAgainstAgent` (dependency-aware: the reconcile's two tables and the CEP spike tables must all be inside the capture scope — out-of-scope tables reject the publish naming them; capability-aware: reconcile/CEP read row images for keys and numeric deltas, requiring `afterImage`, with `connector.Requires` naming the missing item). The agent's `buildPayload` self-reports two new labels: `captureTables` (the capture allowlist, comma-joined, lexicographic) and `captureConnectorCapabilities` (capability keys comma-joined) — sourced from `source.MySQLCapabilities()` (honestly: afterImage/GTID/resumable positions declared; beforeImage not declared, strictly, because it depends on binlog_row_image; snapshot/pushdown unimplemented). Decisions and semantics:

1. **The capability-set channel choice (decided 2026-10-13: labels, not extending the registration response)**: the same channel as captureGatesRevision — zero proto, zero wire changes; publish validation must fail-soft on lag anyway, and the strong consistency a registration-response extension buys is not worth a proto change. Server-side `captureAgentFacts` reads same-scope connected role=capture sessions: tables as a union, capabilities as an intersection (the weakest connector bounds the publishable surface); contributors=0 (no connected agent / labels missing / unknown capability keys) = skip the dependency/capability lint (never penalizing existing fleets and upgrade windows), while structure + semantic validation still run in full.
2. **The v1 capability mapping**: reconcile/CEP spikes → `afterImage` (row-image key/numeric-delta reads); offHours/probing/lineage/event filtering have no capability dependencies for now (the lineage table dependency follows the lineage rules' capture-surface expansion in another batch). Capability-key parsing is as strict as `ParseCapabilities`: unknown keys are rejected (never silently dropped — a silent drop would make extended capabilities vanish from the report surface and mislead publish validation about compatibility).
3. **The compatibility impact of mandatory publish checks**: existing content that "passes structure lint but fails semantic validation" now 422s at publish (previously publishable, with agent apply refusing and last-good lingering) — the tightening is this project's point (moving the interception earlier); there is no behavior-regression risk.

Tests: ops `capture_gates_agentlint_test.go` 7 cases (an out-of-scope reconcile table rejected / in-scope passing / missing afterImage rejected including the CEP branch / no facts skipping including missing labels and unknown capability keys / a burst out-of-scope rejection / union-intersection merging / strict capability-label parsing / the publish path's semantic backstop) + runner `runner_labels_test.go` 4 cases (label content / multi-table lexicographic / omitted when capture disabled / the capability self-report composition self-check).

### 8.14 Delivery Record: The Panel Editing Surface's Structured Form (CDC Platformization Batch Phase Four, 2026-10-13)

`/ops/capture`'s bare JSON textarea upgraded to a form/JSON dual mode (a Segmented switch), with the rule shape isomorphic to the agent-side `config.Gates`. Key stance: **the content string remains the sole truth — the form is only a JSON producer**. The data layer `gatesForm.ts` (pure functions, no React): `parseGatesForm` is fault-tolerant (legacy content missing sections / drifted types fall back to empty values; bad JSON throws and the caller blocks it outside the mode switch); `serializeGatesForm` always outputs all four sections (empty arrays = capture-all/unrestricted, matching the agent config file's semantics); the `GATE_OPS`/`GATE_SEVERITIES` closed sets share the constraints of `gate.checkOps`/`filter.NewEventFilter`. The form surface `GatesFormEditor`: six sections (reconcile / off-hours / spike / probing / lineage / event filter), row add/remove + nested offHours window editing, with control aria-labels `<section>[<row>].<field>` for unit-test targeting. The publish/validate chain is unchanged: form edits serialize back into content immediately; the "Validate" button and the "Publish" Popconfirm reuse the existing content channel and the validate endpoint (§8.12); JSON mode is retained as the authoritative editing channel (the escape hatch for existing hand-written content).

### 8.15 Delivery Record: MySQL/MariaDB Dual-Dialect Support (CDC Platformization Batch Phase Four, 2026-10-13)

`MySQLConfig` gained the `flavor` key (`"mysql"` | `"mariadb"`, default mysql; an unknown value errors before Open — never a silent fallback). MySQLSource branches by dialect across the chain; the difference list:

| Surface                         | MySQL 8.x                                                                        | MariaDB 11.x                                                                                 | Code location                                           |
| ------------------------------- | -------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------- | ------------------------------------------------------- |
| GTID position variable          | `@@GLOBAL.gtid_executed`                                                         | `@@GLOBAL.gtid_current_pos` (no gtid_executed variable; the query errors directly)           | the `mysql_probe.go` dialect branch                     |
| GTID format                     | UUID:interval sets (`3E11FA47-…:1-5`)                                            | domain-server-seq (`0-1-5`)                                                                  | go-mysql `ParseGTIDSet(flavor,…)`                       |
| GTID switch                     | Requires explicit `gtid-mode=ON` (otherwise probing reports gtid_executed empty) | GTID always on                                                                               | A source-side precondition; CI/local container flags    |
| binlog default                  | On by default in 8.x                                                             | Requires explicit `--log-bin`                                                                | A source-side precondition                              |
| Position resume / event parsing | go-mysql `BinlogSyncerConfig.Flavor` + ROW event parsing                         | The same library and protocol (MariaDB is MySQL-replication compatible); zero code branching | `mysql.go` passing flavor at one point                  |
| JSON type                       | Native binary JSON                                                               | A LONGTEXT alias (no binary JSON)                                                            | Row images all land as strings; the agent is unaffected |

The capability surface is identical on both sides (`MySQLCapabilities` does not branch by dialect): GTID metadata, after images, and position resume are all present; beforeImage stays strictly false, depending on the source's `binlog_row_image=FULL`. Bookmarks are written per dialect: a mysql-format bookmark fed to the mariadb flavor (or the reverse) errors at `ParseGTIDSet` — mismatches surface early.

Verification: `source_flavor_integration_test.go` (integration-tagged, gated by the `TEST_MYSQL_DSN` / `TEST_MARIADB_DSN` env vars, skipping when absent) walks the full chain for both flavors — create DB/table/seed rows → Open with an empty bookmark (probing the dialect starting point) → append marker rows → events received through the allowlist filter → Bookmark parsed back via the dialect's `ParseGTIDSet`. CI gained the `capture-matrix` job (docker run mysql:8.4 GTID-on + mariadb:11 log-bin-on, the same commands as local verification); the unit-test step also added `./agents/...` (previously the agents packages were entirely outside CI's test surface).

### 8.16 Delivery Record: Runner Multi-Connector Assembly and the Unified Consumption Surface (CDC Platformization Batch Phase Four, 2026-10-13)

The runner's consumption surface switched wholesale to the `connector.SourceConnector` / `connector.ChangeEvent` contracts (the runtime-side record is in the connector architecture doc §8.4; here are the differences from the agent profile's perspective):

- **The data-source config surface**: `capture.sourceType` (`"mysql" | "postgres"`, a closed set)
  - `capture.dsnEnv` (the connection-string env-var name — the credential red line: a plaintext DSN never enters config files or logs; error messages name only the variable) + `capture.options` (non-secret params, lowerCamelCase, `tables` required). The §2 legacy `capture.mysql` (credentials in YAML) is tolerated only by the compatibility-parsing code and is no longer written into any example/doc.
- **Assembly dispatch**: canonical postgres → `pg.PostgresConnector`; canonical mysql → `source.MySQLConnector`; legacy → the `ProviderConnector` bridge (a generic Provider→contract adaptation; both event forms isomorphic). `runSourceOnce`: position load (the `kind:value` contract form used directly; legacy bare values fall back to a kind by source type) → Open → Poll → flush-then-close on stream end/shutdown.
- **The three gates' consumption surface**: `Evaluate` all read `connector.ChangeEvent` (the `Operation` closed set, `Before`/`After` position-index-keyed maps, `Position`); §4's gate-layer semantics change zero — the row-image key shape matches the existing "fetch by position index" spec.
- **The supporting surface**: position files are per-source (`<sourceType>.json`); the Readiness probe address resolves the non-credential surface from DSNEnv (unix sockets skip probing); registration-label capability self-reports dispatch by source type (`MySQLCapabilities`/`PostgresCapabilities`).

Verification: the config contract surface (canonical/legacy shapes, `Tables()` ordering, `SourceConfig` credential channels, Validate closed set/required fields, BookmarkPath), `bridgeEvent`'s lossless mapping and guards, `AddressFromDSN` for both dialects, the runner's `loadPosition` legacy fallback, and the runner's full-chain fakeConnector consume/position/reopen/alert/filter-statistics cases green with the package. Boundary: the live-stack full-chain verification (rules→panel) is not done; it belongs to 4306's closing increments.

## 9. Known Boundaries (Honest List)

- The lineage account is unobtainable on both sides (§3.3): neither MySQL binlog nor PG WAL events carry a session account; the account field is always empty in C1, running degraded by default with a panel label.
- The reconcile gate requires the record table to be subscribable in the same DB; cross-DB records (sharded databases/tables) are not promised in C3 and need routing customized to the game vendor's topology.
- When the binlog/WAL retention is shorter than capture-agent's downtime, positions invalidate and rescanning starts from the earliest available point (the reconcile gate's window absorbs false positives); position lag is monitored and alerted.
- Gate evaluation is best-effort (agent-side in-memory state): an agent restart loses sliding-window state; lineage/reconcile are unaffected (purely per-event).
- Before images depend on `binlog_row_image=FULL` / `REPLICA IDENTITY FULL`; unsatisfied, an update's before image is empty and reconcile/spikes degrade to after images.
- Alert reporting still has boundaries: a full queue drops immediately; an enqueued send failure retries at most 3 times; exhausted retries, shutdown interruptions, or agent restarts all lose the alerts records. The hit truth is in the agent's `slog warn` logs (`capture gate hit`) for reconciliation, with drops counted in `Stats.AlertDropped`; a cross-restart persistent queue and post-ACK cleanup are pending.
- Alert reporting still degrades against old servers: a server not recognizing the msgID → enqueued reports drop after at most 3 retries (no reporting during mixed-version deployments; local logs only).
- C1 row images are positional-array JSON (the bare binlog ROW event form, no column-name mapping) — the gates' lineage/reconcile can align by position, but panel-summary readability is limited; column-name mapping (information_schema ingestion) comes with C2.
