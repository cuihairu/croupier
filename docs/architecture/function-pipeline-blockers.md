---
title: Function pipeline blockers
icon: triangle-exclamation
order: 6
category:
  - System architecture
tag:
  - Function registration
  - UI generation
  - Execution
  - Known issues
---

# Function pipeline blockers (registration → UI generation → execution)

> **Nature**: a full-pipeline inventory snapshot from 2026-09-22. Each entry gives the symptom, evidence (file:line as of the inventory), blast radius, and disposition status.
> **Disposition markers**: ✅ fixed (with fix notes)｜🚧 being fixed in this pass｜📋 planned deferral (a clear plan exists, scheduled)｜⚖️ needs a design decision (the plan affects protocol/product semantics; decide first, build second).

## 0. Structural conclusions

1. **Multi-database mode (`multiGame: true`) is a neglected second-class citizen**: stale-heal, component templates, sweep budgets, and several other subsystems were validated only in single-database mode and break the moment multi-database is switched on (U1 fixed, U2 pending a decision). Production is single-database for now so it is not triggered — the highest-priority structural debt.
2. **The execution chain has a "silent agent death" degradation chain**: E1 (no failover) + E6 (no circuit breaker) + E3 (no watchdog) + E7 (cancel swallows errors) together amplify one network partition into "a synchronous call wedged for 15s → no breaker removes the candidate → async tasks ghost-running → cancel hangs forever".
3. **Three protocol-level capabilities are "field exists but chain broken"**: the idempotency key (E5), cancel propagation (E4), and permission rule-level scope (verified to be a dead field, see §3 E0) — the shared lesson is that a new protocol field must ship with the full "API receives → stored → used" chain in the same PR, otherwise it is born a silently dead field.
4. The grace mechanism for transient registration removals (`function_contracts.removal_pending_at`, migration 0031) is well designed in itself; this inventory found two defects in it (R3/R4), both fixed in the same pass.

## 1. Registration chain (SDK → agent → ControlService → registry → contracts/default pages)

### R1 [Same-named functions across registrants: latest-wins, no conflict detection (high]

When two providers/agents register identically named functions in the same scope, each registration rebuilds every function contract of that session wholesale, and on the `(game_id, env, function_id)` unique index the last writer wins; the schema is compared only against "the same agent's previous registration" (`PreviousFunctionSchema` restricted to the same agent), so cross-registrant schema drift has no alarm and no gate. With multi-language or canary coexistence, contracts flip back and forth → published-page digest mismatch → whole-page `input_schema_stale` (root cause of the 2026-09-20 incident; 5a0f60869 added a parity guard only for demo contracts — the server has no cross-registrant protection).

Evidence: `internal/platform/registry/store.go:525-530` (wholesale rebuild), `internal/model/function_contract.go:17-19` (unique index), `internal/server/control_handler.go:622`, `agentlocal/store.go:165-185` (schema explicitly "last registrant wins").

**Disposition (needs a design decision)**: candidates: a) reject-the-new-keep-the-old plus a registration alarm when cross-registrant schemas are incompatible; b) mark conflicts stale and route them to the alert center for human adjudication; c) provider-dimension contract isolation (multiple version rows per function_id). First decide whether "same-named function from multiple providers" is a supported scenario.

### R2 [Under HA, survivingFunctionMeta is an instance-local in-memory view (medium]

An agent on instance 1 removes function f, but f is provided by agent B on instance 2 → instance 1's memory holds no B session → mis-marks pending → 10 minutes later the sweep truly deletes a still-serving contract. Mitigation: the cluster backfills remote sessions every 30s, triggering `ClearRemovalPending` (`cmd/server/cluster.go:392-445`), but a `ListAliveOwners` failure abandons the backfill outright (`:393-397`). Sustained ownership-table failure plus cross-instance overlap can mis-delete (self-heals once the agent re-registers).

Evidence: `store.go:531-546`, `:803-825` (iterates this instance's `s.agents`).

**Disposition (needs a design decision)**: skip the marking round when the backfill fails (a fail-open conservative posture), or make the pending determination read a shared ownership table instead of local memory. Not triggered in single-active deployments.

### R3 [Finalize does not regenerate page proposals for surviving resources (medium]

`FinalizeExpiredContractRemovals`, for the "resource still has other live contracts" case, rebuilds only the capability aggregation and not the resource page proposals → the `resource:<key>` proposal's PageSpec keeps bindings/sections of the deleted function until that resource's next registration or a manual rebuild. The "same net effect as the immediate-removal path" claim does not hold for proposal content (the old path regenerates via `plan.Resources → RebuildProposalsForResource`). Publishing on that basis produces bindings doomed to `binding_function_missing`.

**Fix**: when finalize hits and the resource still has live contracts, append `RebuildProposalsForResource` after `RebuildResourceCapability`; append one `RegenerateContractTemplates` at the close of the whole finalize round. After the fix, proposals and templates match the immediate-removal path's net effect.

### R4 [Removal marking/unmarking touches updated_at, creating digest ghost churn (medium]

`MarkRemovalPending` / `ClearRemovalPending` use gorm `Update` (which auto-touches `updated_at`), while `computeDigest` serializes the whole row (`UpdatedAt` lacks `json:"-"`) → the first `RebuildProposalsForResource` inside the grace window judges the proposal "changed": accepted proposals are knocked back to pending, and extra proposal/semantics version snapshots appear. This directly defeats the change's own "zero version noise" goal (the model comment at `function_contract_model.go:47-53` explicitly lists the updated_at ghost perturbation as the motivation for the skip-write mechanism; adding `json:"-"` missed gorm's auto-touch).

**Fix**: both spots switch to `UpdateColumn(s)` (bypasses gorm's auto-touch, leaves `updated_at` alone); add a "digest unchanged after marking" regression test.

### R5 [Permanent disappearance (agent fully offline) has no contract sweep path (medium]

pending is triggered only by "the same agent's re-registration diff"; when an agent crashes and never re-registers, the session cleanup chain (in-memory ExpireAt cleanup, DB DeleteExpired sessions, RemoveAgentIfStale on disconnect, pruneOrphanSnapshots) never touches contracts. The dead function's bound contracts + proposals + pages persist indefinitely (zero instances, calls always fail), with no manual deletion entry (`FunctionDelete` removes only the legacy functions table).

**Disposition (needs a design decision)**: candidates: a) mark pending on functions the expired session exclusively owned (reusing the grace window); b) add a manual removal entry to the function catalog (via the `RemoveFunctionContract` immediate path).

### R6 [After finalize claims a row, derived-action failures are swallowed with no retry (medium-low]

Post-deletion removed history / proposal cleanup / resource rebuild failures get only slog.Warn; the row is claimed, never processed again, and invisible to operations (contrast: the registration chain surfaces registrationWarnings in the UI).

**Fix**: derived-action failures are aggregated into the return (`errors.Join`, without interrupting the round — remaining candidates keep sweeping) so callers can perceive the incomplete state; the incomplete state can be manually repaired via `RebuildAllProposals` (endpoint already exists). The registrationWarnings-style alarm surface was not built; see E7 for unified treatment with the watchdog.

### R7 [Sweep-loop engineering shortfalls (low]

The candidate query has no LIMIT; a row-level error returns early, deferring the remaining candidates; multi-database sweeps run scope by scope serially.

**Fix**: `ListExpiredPendingRemovals` gains a limit parameter (oldest-first LIMIT truncation; one sweep round batches `removalSweepBatchSize=200`, remaining candidates continue next round; the `idx_function_contracts_removal_pending` index supports the filter+sort); a claim failure (`DeleteIfRemovalPending` erroring) no longer ends the round early — aggregated into the returned error, same treatment as derived-action failures (R6). Boundary: per-scope serial scanning in multi-database mode is kept — at current volume one round fits the 30s budget; parallelism/budget slicing revisited at scale.

### R8 [Restart/recovery wholesale rematerialization amplification (low]

After an instance restarts, `LoadFromDBFiltered` restores the snapshot and the first registration diffs everything as Added → full contract/proposal/template rebuilds of SELECT/UPDATE round trips; the skip-write/digest gating already guarantees content correctness, leaving only write amplification and recovery churn. The historical sqlite transaction-deadlock lesson says this path is sensitive to write locks; incremental reconciliation is needed at scale.

### R9 [Heartbeat self-heal reseed overwrites the DB snapshot with an empty Functions session (low]

The `control_handler.go` heartbeat reseed sends `Functions: map{}` and `writeToDB` writes `agent_sessions.functions` as an empty object → later diff baselines are distorted (everything Added; harmless in most cases); the cross-instance "only upgrade, never downgrade" backfill cannot recover from an empty row. Low-risk corner case, handled with R8.

## 2. UI generation chain (contract → proposal → draft/publish → rendering)

### U1 [Multi-database mode: stale-heal auto-healing fails entirely (high]

The heal loop enumerates scopes via the meta database's `metaDB.Table("page_specs")`, but in database-per-game mode page_specs / published_page_specs live in the game database (`PageSpec` is in GameModels) and the meta migration set never creates that table → the query errors and returns every round, healing never runs. In multi-database deployments, contract drift on published pages gets no automatic healing and execution is blocked wholesale — everything becomes manual.

**Fix**: scope enumeration branches on deployment shape (`healScopes`) — single-database mode (Router not wired) behaves as before, taking distinct scopes from page_specs; multi-database mode enumerates from the `game_envs` binding table (the meta-side single source of truth, same source as the migration fanout), with page-level queries landing in the right game database via `scopeDBContext` (scope + dbctx game-database override). Regression tests use a real Router file-backed database e2e to verify "drift on a published page inside a bound scope gets synced, rename-semantics syncs drafts only without publishing, second round is idempotent" + the single-database multi-scope enumeration does not regress. Boundary: a scope whose binding exists but whose game database has not been lazily created spins one round on an empty query (a scope with published pages has necessarily created its database; no side effects).

### U2 [Multi-database mode: component template library hits the wrong database (high]

The component-templates REST mounts on the non-scoped route group (`internal/handler/routes.go:111-117`) and the auto-rebuild closure uses `serverCtx.DB` (the meta connection), but `component_templates` is in GameModels (the game database) → in multi-database mode the template API and registration auto-rebuild read/write a nonexistent meta table, and registrationWarnings flood the screen.

**Disposition (needs a design decision)**: is the template library per-game or global? If global, the table should migrate to the meta database (numbered migration); if per-game, the REST and rebuild closure need scoping. Decide the direction first, then build.

### U3 [Composite pages have no contract-change regeneration channel (medium]

`RebuildAllProposals` / `RebuildProposalsForResource` cover only resource/standalone; `composite--` proposals update only when the editor re-saves; after contract drift on a published composite page, sync-selectors stays `manual_required` forever and heal never lets it through → the only way out is reopening and re-saving in the editor.

**Disposition (needs a design decision)**: the semantics of composite regeneration (keep the manual layout, replace only the drifted binding's schema) need their own design, connected to the "batch sync writes drafts only" boundary.

### U4 [Versioning "regenerate" misfires semantically on composite pages (medium]

For non-resource pages, `RegenerateProposal` treats a composite page's first action/task/report binding as the "primary function" and generates a single-function Operation proposal, force-changing the PageKey over the original composite proposal → one regenerateDraft/BulkRepublish replaces a composite page with a single-function page (content lost in two steps).

**Disposition**: composite pages should refuse standalone regeneration (400) with the entry grayed out. Small change, fixed together with U3.

### U5 [After an accepted proposal is knocked back to pending, re-accepting always 409s (medium]

For a page whose draft already exists, its proposal resets to pending on contract change and re-enters the inbox; the user clicks accept → `page draft already exists` Conflict, with only reject or destructive regenerate available (contrary to sync-selectors' keep-custom path).

**Disposition (needs a design decision)**: the semantics of accepting when a draft already exists (merge selector changes? point to sync-selectors?) is a product call.

### U6 [Legacy no-schema pages have no render-side fallback (low]

c76f889d1's single-payload fallback exists only in the server generator; previously published empty-form pages / old composite "confirm execution" buttons are not repaired by heal and need a manual republish. Converges naturally with the next full rebuild.

## 3. Execution chain (web → HTTP → dispatcher → agent → SDK)

### E0 [Verified up front) permission rule gameId/env is a dead field]

The `GET/PUT /api/v1/functions/:id/permissions` DTO has no `gameId`/`env`: form-submitted values are silently dropped at JSON binding and GET does not echo them; every rule in production is permanently a global rule. The model columns and `FunctionActionAllowed`'s rule-level scope matching already support both; the break is only at the two REST ends. The dead input boxes have been removed; the three-layer scope relationship and the break record are in `game-environment-scope.md` §12.7; wiring it through is a feature-level change, deferred.

### E1 [Calls on half-open connections do not fail over (high]

`errAgentUnreachable` covers only "session missing / forwarding failed"; timeouts/connection resets on an established session return the whole request as failed. When an agent dies without FIN (power loss / network partition) the candidate stays alive until the 5-minute TTL, and every call during that window eats the full 15s timeout. Combined with the HA circuit breaker not being enabled (wiring hardcodes `NewDispatcherWithHA(..., false, ...)`, `SetHAEnabled` has no callers) there is no removal fallback.

**Disposition (needs a design decision)**: whether timeout-class errors join the failover candidate set (duplicating side effects is a risk, linked to E5's idempotency); also evaluate enabling the HA circuit breaker.

### E2 ["Disable function" does not affect execution (high]

Disable only flips the functions table's `status` (`model.StatusDisabled=0`); nowhere in the execution chain (dispatcher candidates, invoke, /tasks Start) reads it → a disabled function remains callable, semantics inverted from intent.

**Fix**: a shared guard `utils.EnsureFunctionEnabled` (`functionInvoke` after route validation and before telemetry/audit/execution recording; `/tasks` Start after contract validation); disabled returns 409 `function_disabled` (same vocabulary as the page generator's identically named diagnostic). Boundary: no materialized row in the functions table is not intercepted (same truth table as the disable write path); dispatcher candidate selection still ignores status (direct SDK calls are not subject to the guard).

### E3 [Async tasks have no watchdog (high]

Run status advances only via agent events; after an agent disconnects, the run row stays at dispatching/running forever, `timed_out` has no writer, the frontend's 1.5s polling never terminates, and call statistics aggregated by status drift.

**Resolved in ARCH-014/015/018** (see `docs/design/task-retry-recovery-protocol.md`): agent session removal now reaps that agent's non-terminal rows to `unknown` (execution state not established, no auto-rerun), and the D3 watchdog reaper (ARCH-018) transitions rows with no event past the budget (`taskLog.reaperTimeout`, default 5m) to terminal `timed_out` — `timed_out` now has a writer. The terminal-state semantics question was settled in favor of `timed_out` (= "no further evidence", not "failed at the game"); `interrupted` was not introduced. An execution-interrupt fault drill (`cmd/server/fault_drill_r27_test.go`) exercises disconnect / drain / SIGTERM / watchdog with platform-convergence assertions only (the probe fixture is not game-recovery evidence).

### E4 [Cancel propagation reaches only the agent transport layer, never the game function (high]

The agent does not forward cancels to the provider; SDK inbound handlers execute with `context.Background()` → "cancel" = stop waiting, not stop executing; game-side side effects (charging / sending mail) happen anyway with no compensation record. The SDK's startTask/cancelTask/streamTask handlers are dead code in the platform chain.

**Disposition (protocol-level)**: add a provider cancel frame to the wire protocol + thread ctx through SDK handlers; all six language SDKs change in sync. High cost, separately scheduled.

### E5 [Idempotency key: nowhere in the chain is it actually used (high]

The proto field, the `task_runs` column, and the StartRequest field all exist, but the API does not accept it, BuildInvokeRequest does not set it, CreateRunWithMeta neither writes nor queries it, and there is no dedup store → double clicks / network retries = duplicate execution (especially dangerous for async tasks); failover minting a new taskID each round stems from the same gap.

**Disposition (protocol-level)**: designed together with E1 — the API layer accepts `Idempotency-Key` (REST header or body), a task_runs unique index, dedup-window semantics. This is the foundation of execution-chain correctness; recommend scheduling it first.

### E6 [Approval storage is an in-process MemStore (medium]

PG/SQLite implementations exist but are not wired; a restart loses every pending approval; approvals have no TTL, so an old single can be approved indefinitely late, and renewal executions carry `approvalBypass=approved` skipping all policy.

**Disposition (needs a design decision)**: when to wire the PG/SQLite store (involves the approvals table migration and HA semantics — once two instances share an approval pool, the "two people" of a two-person rule must be judged across instances).

### E7 [A grab-bag of silently swallowed errors (medium]

Policy query failure → nil passes (fail-open); audit write failure is only fmt.Printf; task_runs row creation is best-effort (when events have nowhere to land and 0 rows updated, even the event is dropped); REST cancel's agent-forward failure is swallowed by `_ =` → cancel_requested becomes a factual terminal state. Treated uniformly with the E3 watchdog: fail-open items become fail-closed or alarm explicitly.

### E8 [Frontend task polling stops permanently on a single error (medium]

The task event polling stops permanently without retry on a single network error/401; the progress panel appears frozen until the user re-initiates.

**Fix**: `subscribeTaskEvents` retries within 5 consecutive failures (interval stays 1500ms, no exponential backoff), resetting the counter on success; each failure still calls onError (consumers render it as a log line, so transient drops stay visible), stopping only past the limit. Terminal failure vs. "temporarily unreachable" is distinguishable by count in the upper-layer logs.

### E9 [Version-gate blocking has no execution-side explainability (medium]

Functions rejected by the gate never enter the session function set; calls report 503 `no_live_agent`, indistinguishable from "the agent genuinely has no live instance"; after the gate is raised, already-materialized functions remain callable until the next registration.

**Fix**: at the dispatcher's "no candidates" close (`noLiveAgentError` promoted to a method, shared by the sync/broadcast call sites), functions with both a `function_version_below_minimum` registration warning **and** a still-configured gate return the dedicated error code `function_version_below_minimum` (503; the HTTP contract's `error` field carries the code), with a message naming the blocked version and the two ways out (upgrade and re-register / lower the gate). The double condition prevents mis-reporting: the warning is an in-memory surface, deleting the gate does not auto-clear warnings, and with the gate deleted it falls back to the original `no_live_agent` semantics. Companion: the blocking warning gains `FunctionID`/`Version` fields (previously empty, so per-function filtering failed); registering a compliant version clears that function's historical blocking warnings (lifecycle follows the registration act, closing the loop the same way the provider_scope_mismatch fix did), guaranteeing "warning present ⟹ the latest registration was still blocked". Boundary: the original semantics — after a gate raise, already-materialized functions stay callable until the next registration — is unchanged (contract regression is guarded by the registration side); under HA the warning is an instance-local memory surface, and cross-instance ownership-forwarded scenarios are attributed by the peer instance against its own warning surface.

### E10 [Remaining engineering shortfalls (medium-low/low]

The synchronous hot path does 2 DB queries per call with no cache (contract query jitter silently falls back to the 15s timeout budget); the agent side degrades to `arr[0]` routing when no healthy instance exists within the 30s heartbeat window; task_events seq is read-max-then-increment, non-atomic, with no unique index; the HTTP layer has no per-request timeout middleware (WriteTimeout 10min). Absorbed as the corresponding modules iterate.

## 4. Priority recommendations

1. **Fixed in this pass**: R3, R4, R6, R7, E0, E2, E8, E9, U1 + analytics placeholder implementations switched to real aggregations (see §5).
2. **Decide, then build**: R1, R2, R5, U2, U3, U4, U5, E1, E3, E4, E5, E6 (each already has candidate plans).
3. **Absorb as volume grows**: R8, R9, U6, E7, E10.

## 5. Fix log

- 2026-09-22 (second batch): U1 (multi-database stale-heal branches on deployment shape: single-database queries page_specs directly as before, multi-database enumerates scopes from game_envs bindings and lands page queries in the game database via the Router, with real-Router file-database e2e and single-database no-regression cases); E9 (the dispatcher returns the dedicated `function_version_below_minimum` error code for gate-blocked functions, warnings gain FunctionID/Version, compliant registration clears stale warnings); R7 (sweep candidates oldest-first LIMIT, 200/round + claim failures don't interrupt the round, aggregated into the return).

- 2026-09-22 (first batch): E0 (roles dropdown wired to `listRoles`, dead field removed, §12.7 docs); R3 (finalize appends resource proposal regeneration + template close-out); R4 (removal marking/unmarking switched to UpdateColumn eliminating digest churn, with updated_at/digest-unchanged regression cases, 0031 migration adding the column + the sweep candidate index `idx_function_contracts_removal_pending`); R6 (finalize derived failures aggregated into the return via errors.Join); E2 (`EnsureFunctionEnabled` intercepts at both entries, 409 function_disabled); E8 (task event polling retries within 5 consecutive failures, interval unchanged); function analytics switched from "config version count + hardcoded 100%" to real `execution_logs` aggregations (total calls / success rate / average latency / today·week·month, success rate shows "—" on zero data instead of faking it); the console/registry removal tests and the api layer's old analytics-caliber tests adapted to the grace and new statistics semantics in step.
