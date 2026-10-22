# Agent Lifecycle and Desired-State Coordination Recommendations (External Review · Pending Verification)

- **Source**: https://chatgpt.com/s/t_6acef63fad008191b11dbaece3056aab
- **Fetched**: 2026-10-14
- **Nature**: **External advice, not a statement of fact about this repo**. The original text also says of itself: "**The Reconciler is a proposed new architectural component and does not mean Croupier has already implemented a complete desired-state coordination mechanism**" — before acting, everything must be verified against the code (file:line).
- **Builds on**: [probe-architecture](chatgpt-2026-10-14-probe-architecture.md) (three-layer structure) and probe-adapter (external adapter contract). This document unifies the **managed objects** mentioned in the previous two documents into a single model.

## 1. Overall Architecture: Desired-State Coordination (Reconciler)

The Server declares the state programs **should** be in → the Agent checks the **actual local** state → then progressively performs start, stop, update, and config publishing.

Call chain: `Server → Desired → Task → Agent → Reconciler → {Scheduler, Supervisor, Updater, Config, Health} → Supervisor → Runtime → {Probe, GameTool}`

The `Reconciler` is a **proposed new** component; this repo does not implement it today.

## 2. Which Lifecycles the Agent Should Manage

| Capability                           | Owned                   | Notes                                                                      |
| ------------------------------------ | ----------------------- | -------------------------------------------------------------------------- |
| Install, start, stop, restart        | Yes                     | Unified process management                                                 |
| Boot autostart and crash recovery    | Yes                     | **Distinguish program crashes from deliberate stops**                      |
| Scheduled start, stop, restart       | Yes                     | Use the unified scheduler                                                  |
| Update binaries                      | Yes                     | Download, verify, switch, roll back                                        |
| Update config files                  | Yes                     | Versioning, validation, atomic publish                                     |
| **Modify probe frequency**           | Yes                     | As **desired config** of the managed program                               |
| Pause and resume                     | Yes                     | **State explicitly whether scheduling, the program, or probing is paused** |
| Health probing                       | Yes                     | **Reuse `core/healthprobe`**                                               |
| Failure recovery policy              | Yes                     | Restart, backoff, circuit breaking, manual intervention                    |
| View logs, versions, run state       | Yes                     | Report uniformly to the Server                                             |
| **Arbitrary remote shell execution** | **Not open by default** | Separate authorization, allowlist, audit                                   |

**Key boundary**: updating binaries and configs **≠** permission to run arbitrary remote commands. Program operations should be modeled as **explicit, authorized actions**, not a generic command-execution entry point.

## 3. The Unified Managed-Program Model `ManagedProgram`

It can represent both **game helper programs** and the **external probe adapters** from the previous two reviews — merging the two classes of managed objects into one model.

Structural sketch: `Schedule ScheduleSpec` / `Recovery RecoverySpec` / `type ProgramDesiredState string ...`

## 4. Four States That Must Be Kept Apart

**Run state / management state / scheduling state / update state** — conflating them leads to misjudgments such as treating "updating" as "stopped".

## 5. How to Update Binaries

- Binaries are distributed from **trusted sources**, verified by **content hash and signature**.
- **Atomically swapping a binary alone does not make an update safe**: you must also consider a running process / file locks, database or config compatibility, and **recovery when the new version fails to start**.
- **The Agent's own upgrade must be designed separately**; it cannot simply reuse the ordinary managed-program update logic.

## 6. How Scheduled Jobs and Health Recovery Cooperate

Recommendation: **a unified scheduling model, but do not fuse scheduled jobs and health recovery into one execution logic**.

## 7. Impact on This Repo's Existing Architecture (per the original text)

The original explicitly frames this as **proposed new** work, and notes it would touch Scheduler / Supervisor / Updater / Config / Health.

---

## Verification and Adoption Verdicts (this repo, 2026-10-14)

| Claim                                              | Verdict                      | Notes                                                                                                                                                                                                                                      |
| -------------------------------------------------- | ---------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| Introduce a `Reconciler`                           | **Pending, leaning adopt**   | This repo has no complete desired-state coordination today; the responsibility boundary between the existing `Supervisor` and dispatch must be verified first to avoid duplicating existing mechanisms                                     |
| `ManagedProgram` as the unified managed object     | **Adopt**                    | Directly resolves the split between the "game helper program" and "external probe adapter" models; the relationship with probe-adapter's `ProbeAdapter` inventory **must be spelled out — three coexisting config models are not allowed** |
| "Modify probe frequency" as desired config         | **Adopt**                    | Aligns exactly with the user's 2026-10-14 directive "the server controls sampling frequency / time windows / special time points, defaults when unset"; it is the higher-level model of the same request                                   |
| Distinguish crashes from deliberate stops          | **Adopt**                    | Otherwise deliberate shutdowns also trigger recovery, causing spurious restarts                                                                                                                                                            |
| Pausing must name what is paused                   | **Adopt**                    | Pausing scheduling ≠ pausing the program ≠ pausing probing                                                                                                                                                                                 |
| Update ≠ remote shell                              | **Adopt as a hard boundary** | Consistent with this repo's permission-convergence direction; a generic command-execution entry point is a security anti-pattern                                                                                                           |
| Atomic binary swap alone is not enough             | **Adopt**                    | Must cover running processes, file locks, data/config compatibility, and rollback when the new version fails to start                                                                                                                      |
| Agent self-upgrade designed separately             | **Adopt**                    | A failed self-upgrade cuts the Agent off entirely — a different risk surface from ordinary program updates                                                                                                                                 |
| Scheduled jobs and recovery implemented separately | **Adopt**                    | Unified scheduling model, but the execution logic stays unmerged                                                                                                                                                                           |

## Landing Tracker

- [ ] Verify the existing responsibility boundaries of this repo's `Supervisor` / dispatch and decide whether `Reconciler` is new or a completion of existing mechanisms (cite file:line)
- [ ] Spell out the relationship between `ManagedProgram` and `ProbeAdapter` (probe-adapter review): merge into one config or layered references — **three coexisting config models are not allowed**
- [ ] Define the four state classes (run / management / scheduling / update) in the design doc
- [ ] Update flow must cover: trusted sources, hash and signature verification, running processes and file locks, data/config compatibility, rollback on failed start
- [ ] Wire "modify probe frequency" into the desired-state model (merged with the user's 2026-10-14 sampling-frequency requirement — one design for one request)
- [ ] Remote shell explicitly not open by default; program operations modeled as authorized actions
- [ ] Dedicated design section for Agent self-upgrade
- [ ] Each item above checked off with commit hash + test count
