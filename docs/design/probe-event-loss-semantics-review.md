---
title: Probe Event Loss Semantics Review
---

# Probe Event Loss Semantics Review

Date: 2026-11-28. Status: **proposal awaiting owner decision**. This page is
the PROBE-005 方案决策 deliverable. It defines which probe results may be lost
silently and which must remain verifiable, and proposes the minimal mechanism
gap — reusing the existing report/alert surfaces. No code is changed by this
page — implementation is gated on the owner's approval.

## 1. Current semantics (code facts)

Agent side (`agents/synthetics/runner/runner.go` `execute`):

- Every probe event — periodic samples **and** failure/recovery transitions
  alike — is sent fire-and-forget: `SendProbeEvent` with a 5s timeout.
- Session unavailable → event dropped; an OTel counter (`session_unavailable`)
  and a warn log record the loss.
- Send failure → event dropped; counter (`report_failed`) + warn log.
- **No buffer, no retry, no resend.** A disconnect-window failure edge is
  unobservable to the server unless the next successful sample implies it.

Server side (`internal/server/`):

- `handleProbeEvent` → durable `probe_events` table (`ProbeEventModel`,
  optional via `SetProbeEventModel`) + in-memory diagnostic ring
  (`probeEventSink`: per-agent 500 entries, 30-minute retention, no dedup —
  "重复比缺失好排查") + the `probe_lifecycle` window→Alert path that drives
  the alert center.

So today: **everything is droppable, and nothing marks the gap.** The server
sees the last state before a disconnect and silently extrapolates it across
the missing window — a target that went down and recovered mid-disconnect
looks continuously healthy.

## 2. Proposed semantics: two classes

| Class                               | Examples                       | Loss tolerance                                                                                                                                       |
| ----------------------------------- | ------------------------------ | ---------------------------------------------------------------------------------------------------------------------------------------------------- |
| **C1 periodic samples**             | every-N-seconds ok/status rows | Droppable. Loss is bounded and visible via staleness; no per-sample persistence obligation (acceptance wording)                                      |
| **C2 failure/recovery transitions** | ok→fail edge, fail→ok edge     | Must be **verifiable**: either the edge reaches the server (possibly late), or the window is explicitly marked unknown — never silently extrapolated |

The acceptance wording "区分可丢的周期采样与必须可核对的故障/恢复跃迁" lands
as this two-class table, enforced at the agent send path and the server
freshness path.

## 3. Mechanism proposal (reuse-first)

### 3.1 Agent side — bounded transition buffer + resend on reconnect

- A small per-agent ring holds **C2 edges only** (C1 samples are never
  buffered). Edges are low-rate by construction: at most 2 per target per
  state flip, so the ring's capacity bound is a safety net, not a working-set
  assumption (capacity budget §3.3).
- On reconnection (the existing configsync reconnect cadence already resumes
  pulls — the same session-available moment), buffered edges are resent oldest
  first, each still carrying its original timestamp.
- Ring overflow policy: drop **oldest C1-adjacent material first is N/A**
  (samples aren't buffered); for edges, overflow drops the oldest edge and
  increments an explicit `transition_buffer_overflow` counter — the loss is
  counted and observable, never silent.

### 3.2 Server side — freshness window and 缺测 (no-data) state

- Each target gets a freshness window (derived from its effective interval,
  e.g. ≥3× interval, config-floor bounded). When no sample arrives within the
  window, the target's server-side view becomes **unknown/no-data** — not
  "last state still holds".
- `probe_lifecycle` consumes this: a transition into `unknown` is a lifecycle
  event (alert-visible, distinct from ok/fail); a stale-fail must not
  auto-resolve as ok without a real fail→ok edge.
- Existing durable surfaces unchanged: `probe_events` keeps storing what
  arrives; the diagnostic ring keeps its 500/30min semantics.

### 3.3 Capacity budget (explicit, not implicit)

| Buffer                     | Bound                                                                                                              | Overflow behavior                        |
| -------------------------- | ------------------------------------------------------------------------------------------------------------------ | ---------------------------------------- |
| Agent transition ring      | Fixed per-agent cap (supervisor event ring precedent: 500 entries; the proposal's working set is target-count × 2) | Drop-oldest edge + explicit counter      |
| Send backlog per reconnect | Drain with per-send timeout (existing 5s send timeout), bounded by ring size                                       | Remaining edges stay for the next window |
| Server ring                | Unchanged (500 / 30 min, per agent)                                                                                | Unchanged (drop-oldest)                  |
| Server durable table       | Unchanged (retention owned by `probe_events` policy)                                                               | Unchanged                                |

No per-sample persistence is introduced anywhere — C1 remains fire-and-forget.

## 4. Reuse map (what is NOT new)

| Existing surface                                         | Role in this proposal                                    |
| -------------------------------------------------------- | -------------------------------------------------------- |
| `core/configsync` reconnect cadence                      | The resend trigger moment (no new session hook)          |
| OTel counters (`session_unavailable`/`report_failed`)    | Loss observability, unchanged + one new overflow counter |
| `probe_events` durable table + diagnostic ring           | Server landing surfaces, unchanged semantics             |
| `probe_lifecycle` window→Alert path                      | Consumes `unknown` as a new lifecycle state              |
| Supervisor event ring bound (500, `registry/metrics.go`) | The capacity precedent for the agent ring                |

## 5. Acceptance mapping (design → verification scenarios)

| #   | Scenario                                                          | Expected                                                                                                 |
| --- | ----------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------- |
| A1  | Disconnect window contains a ok→fail edge                         | Edge buffered; on reconnect it reaches the server with its original timestamp (bounded by ring capacity) |
| A2  | Disconnect window contains down-then-recovered                    | Both edges arrive late; server timeline shows the true outage, not continuous health                     |
| A3  | Disconnect longer than the freshness window with no buffered edge | Server marks the target `unknown` (缺测), lifecycle-visible; no silent extrapolation                     |
| A4  | Buffer overflow (pathological edge rate)                          | Oldest edge dropped with `transition_buffer_overflow` counter; loss is observable                        |
| A5  | Steady-state healthy probing                                      | Behavior identical to today: no buffering, no extra persistence, C1 remains droppable                    |

## 6. Decisions needed from the owner

- [ ] **D1** two-class semantics — approve the C1 droppable / C2 verifiable split (§2) as the governing contract
- [ ] **D2** agent transition buffer — approve the bounded edge ring + resend-on-reconnect mechanism (§3.1)
- [ ] **D3** freshness/缺测 state — approve the server-side `unknown` state and its probe_lifecycle integration (§3.2)
- [ ] **D4** capacity budget — approve the explicit bounds table (§3.3), supervisor-ring-aligned
- [ ] Acceptance mapping §5 (A1–A5) — confirm as the implementation's verification bar
