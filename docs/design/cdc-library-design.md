---
title: Shared CDC Core Library and Event Replay Design
---

# Shared CDC Core Library and Event Replay

## Status and Conclusion

- Status: the L1/L2 core, the optional Croupier journal wiring, and the shared recovery coordination contract are implemented; MySQL/Mongo no longer misreport full transaction boundaries, and recovery gates refuse missing capabilities; the PostgreSQL target DML applier and durable audit adapter are implemented, with real isolated-database coverage for transactions, idempotency, conflict rejection, rollback, dry-run, apply, and replay; the PostgreSQL source consistent snapshot is implemented with the exported snapshot of a temporary logical slot; schema history, the MySQL/Mongo sources, and more targets remain to be implemented (2026-10-19).
- Conclusion: the direction is mature and technically feasible. First establish a business-agnostic Go package boundary inside this repo; once the contract has a second real consumer and a stable versioning policy, split it into a standalone Go module / repository.
- The goal is not to capture every table. Every data source must explicitly declare the databases, tables, and operations it cares about; the core library handles generic selection, pushdown where the source supports it, and a second filter before events enter downstream consumers.
- "Replayable" does not mean "any database can be fully PITR-restored". When only filtered events are recorded, only the captured range can be restored; a full point-in-time restore additionally requires consistent snapshots, contiguous positions, transaction boundaries, DDL/schema history, and a target-database applier.

## Code Status and Gaps

The existing multi-database connector already covers MySQL/MariaDB, PostgreSQL, and MongoDB, with the unified event/position/capability contract in `agents/capture/connector`. Data sources use `capture.options.tables` for table allowlisting; `agents/capture/filter` additionally has the Croupier policy header, database/table authorization matching, and an EventFilter that filters by operation type only. The capture runner feeds events into the reconcile/CEP/lineage three gates, alert hits have a persistent spool, and capture positions are resumable.

The current boundary is sufficient for Croupier's detection use, but it is not an extractable shared library: the connector contract lives in the same directory as the capture agent; the policy header requires `gameId/env`, and rule types include detection/delivery; table and operation filtering are scattered across the source connector and the business runner. The runner does not retain all events, so current position recovery only means "continue capturing", not "replay operations already processed".

## Layer Boundaries

```text
Database protocol adapters (MySQL / PostgreSQL / MongoDB / later implementations)
            ↓ SourceConnector + capability declaration
Shared CDC library (event model, filter compilation/execution, positions, optional event journal and replay)
            ↓ ChangeEvent stream
Croupier adapter (game/env scope, config distribution, detection gates, alerting, and console)
```

The shared CDC library may know source IDs, databases, tables, operations, row images, and source positions; it must not depend on Croupier concepts of games, players, risk rules, alerting, permissions, or console pages. Croupier keeps binding `gameId/env` to source configuration and authorization, without writing that scope into the shared event model.

The first phase puts the shared contract in `pkg/cdc` and the filter primitives in `pkg/cdc/filter`. The existing `agents/capture/connector` keeps compatibility aliases, avoiding a one-shot rewrite of every driver. The packages use only the Go standard library; they must not depend on `agents/`, `internal/`, database drivers, or Croupier protobuf. When the module is later split, the migration should be module-path and release-pipeline work, not an API redesign.

## Event Selection and Filtering

Filter rules use explicit include/exclude selectors: source ID/type, database, table, operation. An empty include denies by default, exclude always wins; `*` is a wildcard for one field only — no regex or executable expressions. "No filter configured" and "explicitly allow everything" must be distinguished by the calling layer, so an empty config cannot silently capture an entire database.

Filtering executes in two places:

1. **Source pushdown**: when connector capabilities allow, exact database/table includes are pushed down to reduce reads and transfer; connectors that do not support wildcards/pushdown report their capability explicitly or fall back conservatively.
2. **Shared event filtering**: after connector decoding and before events enter business consumers, the same rules apply again. Pushdown is an optimization and must not be the only correctness guarantee; rule changes must not let already-filtered tables bypass runtime filtering.

The first phase supports source, database, table, and DML operation filtering only. Column projection, field masking, row-value predicates, and DDL filtering are deferred: they require unified missing-column/null/type semantics and may break the primary keys, before-images, or dependent fields needed for restoration. Arbitrary SQL/script predicates do not enter the first version.

## Journal and Replay

When replay is needed, the shared journal appends only events the filter ultimately lets through, storing stable event IDs, source positions, transaction IDs/sequence numbers/end markers, before/after images, and schema versions. Events must be persisted before source positions are acknowledged; replay consumes in journal-sequence or explicit-range order, and the target adapter applies transactions transactionally. Duplicate delivery is at-least-once plus target idempotency — we do not claim distributed exactly-once across sources, journals, and arbitrary target databases.

Selective capture has a clear cost: if a table or operation is filtered out, we cannot claim the whole database can be restored to a chosen point in time. Full PITR requires an explicit full-scope capture strategy, including snapshots of the relevant tables and DDL/schema changes; otherwise the feature name and UI must say "filtered event replay / partial restore".

`pkg/cdc/journal` already provides a single-writer JSONL append-only log, stable event IDs, crashed-tail truncation, ordered range reads, idempotent duplicate appends, and transactional sink replay. `Capture` runs the shared filter over events first; hits are written to disk and fsynced before the source position is acknowledged, while misses are explicitly discarded before the position is acknowledged. Duplicate delivery retries safely. The journal deliberately does not depend on the Croupier runner, the alert spool, or any database driver.

The Croupier adapter offers an optional `capture.journal` configuration: the filter include compiles from the existing `capture.options.tables` / legacy `capture.mysql.tables`, source ID/type and operations join the shared selectors, and exclude can narrow further; game/env is never read for event filtering. The runner shares one connector stream with the original detection flow: persist selected events first, run local detection, then acknowledge the source position. The journal has a mandatory hard capacity cap; at the cap, the current event is not acknowledged, consumption fails and pauses via the existing reconnect backoff, and capture resumes after expansion/archival. It never rotates or deletes historical logs on its own.

Replay strictly requires transaction boundary metadata: events with a transaction ID must run contiguously from sequence 1 to `End=true`; independent commits without a transaction ID must also carry an explicit `End=true`. The PG decoder already marks transaction-final events; MySQL does not yet produce transaction-end markers, and the MongoDB change stream currently has no reliable transaction-end event. So these two sources may record events under their filter policies, but replay currently refuses their transactions that lack end boundaries — single-row events must not be mistaken for complete-transaction restores. Real database snapshot consistency, MySQL/Mongo transaction boundaries, target-database writes, and the recovery UI still have to be implemented per connector capability, in batches. Connectors missing before-image / snapshot capabilities must refuse recovery plans whose capability requirements are unmet — they must not fake it with empty values.

The recovery coordination contract lives in `pkg/cdc/recovery`, containing no game/env or database-specific SQL. `TargetConfig` only accepts an adapter ID, non-secret options, and DSN environment variable names, rejecting common plaintext credential options; `Plan` requires an explicit principal, source, target, journal range, source position, exact table list, and consistent-snapshot coverage; `Authorizer` lets the product adapter map tenant/game/environment permissions; `SnapshotVerifier` verifies that the snapshot position precedes the recovery start and the schema version; `Applier` declares capabilities, provides read-only validation, and performs transactional application with atomic rollback and idempotency keys; `Auditor` must durably record requests, authorizations, prechecks, every transaction, and the final result. dry-run executes validation only; apply calls the applier only after precheck and audit succeed. The audit stores only metadata, hashed event IDs, and error types — never row images, raw SQL errors, or credentials. When a target call returns an error, the applier must guarantee transaction rollback; when the outcome is unknown, retry by request ID + transaction ID. The capability declaration gains `schemaHistory`, and source adapters must set it true only when they genuinely provide continuous DDL/schema history.

## Phases and Acceptance

| Phase | Scope                                                                                                                                                                                            | Acceptance                                                                                                                                                                                                                 |
| ----- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| L1    | Standalone CDC contract and shared include/exclude filtering                                                                                                                                     | No Croupier imports; empty include denies by default; exclude wins; old connector imports stay compatible; source filtering and runtime filtering agree                                                                    |
| L2    | Optional append-only journal and ordered replay API (implemented)                                                                                                                                | Crashed-tail handling, corruption detection, idempotent appends, range replay, incomplete-transaction rejection, sink failure does not advance the read cursor; journal persisted before acking the source position        |
| L3    | Croupier wiring and ops configuration (optional local wiring implemented)                                                                                                                        | game/env stays in the adapter; table allowlist/operations/exclude compile to the shared filter; the same source stream writes the journal before acking; capacity-full fails closed                                        |
| L4    | Shared recovery coordination contract (implemented)                                                                                                                                              | Exact scope, authorization, snapshot coverage verification, source/target capability gates, dry-run, transactional atomic rollback and idempotency contract, durable audit                                                 |
| L5    | Concrete database snapshots and target adapters (PostgreSQL target DML adapter and audit adapter implemented; PostgreSQL source consistent snapshot implemented; schema history not implemented) | Per source/target, verify snapshot/before/transaction/schema capabilities; real isolated-database dry-run, transactional apply, failure rollback, and audit integration; sources without the capability keep being refused |

L1/L2 are independently reusable library capabilities; L3 is product wiring; L4 is the database-agnostic recovery coordination boundary; only after L5 can a concrete database be called "recoverable".
