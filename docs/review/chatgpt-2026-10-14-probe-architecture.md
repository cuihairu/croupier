# Probe Architecture Correction Recommendations (External Review · Pending Verification)

- **Source**: https://chatgpt.com/s/t_6aceecf2932481918f642489c4a19b19
- **Fetched**: 2026-10-14
- **Nature**: **External advice, not a statement of fact about this repo**. Every item in this file must be verified against this repo's code (cite file:line) before being acted on; nothing here may be taken as evidence that a feature is implemented or a TODO is done. The original also says of itself: "This is currently an architecture correction proposal; it cannot be taken as proof that the game-simulation probing feature in the repository is already implemented."

## Core Claims

Probing should be unified into **one reusable probe kernel** rather than the Agent and the Server each implementing their own health-check logic. Three layers:

1. **Shared probe kernel**: unified probe interface, scheduling, timeouts, consecutive-failure thresholds, recovery determination, state machine, and event model.
2. **Probes extended per protocol**: HTTP, TCP, databases, and **game-client-simulation protocol probes**, each implementing its own probe logic.
3. **Execution location determined by the probe target**: both the Agent and the Server can run the same kernel, but where it runs depends on **whose perspective** the probe result represents and whether network position affects the conclusion.

Stated explicitly: **shared probe code ≠ all probes executed from one end, and ≠ the Server and the Agent probing the same targets.**

### Why Game-Protocol Probing Cannot Be Replaced by TCP

A TCP port being reachable does not mean the game server can actually accept client login, game entry, or key protocol interactions. So it is **not on the same level** as ordinary HTTP/TCP probes: TCP success only proves a connection was established; a simulated client can go on to verify protocol handshake, login responses, character entry, and other business flows. **How far a probe goes is defined by the game protocol and the scenario; forcing all games through one identical flow is wrong.**

Three easily confused levels: game server process alive ≠ game service usable ≠ reachable from the player's network.

## 1. Code Placement Recommendations

The original explicitly says "do not create a second health-probe engine; extend the existing common capabilities", with shared packages holding only generic mechanisms.

| Code responsibility                                              | Recommended placement                              | Notes                                                                         |
| ---------------------------------------------------------------- | -------------------------------------------------- | ----------------------------------------------------------------------------- |
| `Checker`, timeouts, scheduling                                  | `core/healthprobe`                                 | Shared by Agent and Server                                                    |
| Consecutive-failure threshold, recovery threshold, state machine | `core/healthprobe`                                 | Avoid two sets of state-judgment logic                                        |
| Failure windows, recovery events, timelines                      | `core/healthprobe`                                 | Unified event semantics                                                       |
| HTTP, TCP probes                                                 | Common probe package                               | Whoever needs it instantiates it                                              |
| Database probes                                                  | Common probe package or standalone adapter package | Whether it runs depends on deployment location                                |
| Game-client simulation probes                                    | Standalone game-protocol adapter module            | Reuse the common interface; keep game-protocol dependencies out of the kernel |
| Probe target config and distribution                             | Server                                             | Manages config, versions, authorization, execution location                   |
| Probe execution, result collection                               | Agent or Server runtime                            | Run per the target's network perspective                                      |

The game-simulation probe module **must implement the existing `Checker` contract**, returning only probe results; it must not re-implement failure thresholds, state machines, or failure windows itself.

## 2. Agent / Server Division of Labor Table

Decision criterion: **what question the probe result must answer** comes first, then network reachability decides the execution location.

| Probe target                                                    | Default execution location                                              | Reason                                                                     |
| --------------------------------------------------------------- | ----------------------------------------------------------------------- | -------------------------------------------------------------------------- |
| Is the game process alive                                       | Agent on the node hosting the game                                      | Local process state is most accurate                                       |
| Are game ports / internal services reachable                    | An Agent that can reach that internal network                           | The Server may not be able to enter the game's internal network            |
| Simulated real client login, handshake, game entry              | A dedicated probe Agent, or an Agent with the matching network position | Must verify the protocol chain, not just the port                          |
| Player-facing cross-network game reachability                   | Probe Agents deployed at representative network positions               | Probing from a single data center cannot represent the player-side network |
| The Server's own databases, Redis, internal APIs                | Server                                                                  | Verifies the platform's own operating conditions                           |
| Third-party platform APIs only the Server can reach             | Server                                                                  | The Agent cannot represent the Server's actual access path                 |
| Third-party services only the game's internal network can reach | The corresponding internal-network Agent                                | The Server's probe results are not representative                          |

## 3. Five Design Constraints (to be written into the design doc)

1. **Shared engine, not shared execution location** — the Agent and the Server call the same kernel, but every probe target must declare its intended network perspective and execution location.
2. **Probing separated from recovery** — the probe only observes and returns results; whether to restart a process or run a recovery script is decided by an independent recovery policy; **a failed external-service probe must not trigger a local process restart by default**.
3. **Config separated from execution** — the Server owns config, authorization, and distribution; actual probing runs on the selected Agent or Server, and reported results must carry target identity, execution location, time, and probe type.
4. **Game-simulation probing evolves independently** — no concrete game protocol goes into `core/healthprobe`; each game provides its own protocol adapter while sharing probe lifecycle and result semantics.
5. **Avoid duplicate probing and duplicate alerting** — one target may have multiple probe locations, but each location's results must be distinguishable and aggregated by the Server; **a single Agent's failure must not be declared a game-wide outage**.

## 4. Five Correction Recommendations for This Repo's Existing Design

1. Bring the game-client simulation probe Agent into the official architecture, config model, and TODO.
2. Define `ProbeTarget`'s execution location, probe perspective, probe type, protocol adapter, and timeout policy.
3. Decide how probe results reported from Agents to the Server enter the unified health state, alerting, and recovery flows.
4. Clarify which existing Server-side probes (databases etc.) stay on the Server and which move to Agents for network-reachability or perspective reasons.
5. Check the cross-platform accuracy of existing implementations such as Windows process probing, so that unifying the interface does not leave wrong behavior underneath.

---

## Verification Conclusions (measured in this repo, 2026-10-14)

| External claim                                                                 | Verdict                                                      | Evidence                                                                                                                                                                                                                                                                                                                                                                                            |
| ------------------------------------------------------------------------------ | ------------------------------------------------------------ | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `core/healthprobe` already has a common `Checker`, thresholds, failure windows | **Holds**                                                    | Package comments and `Probe`/`Result`/`Window`/`HeartbeatFile`, `process_unix.go`/`process_windows.go`                                                                                                                                                                                                                                                                                              |
| Do not build a second probe engine                                             | **Holds**                                                    | Consistent with the user's directive "probing is a server-and-agent shared feature; the implementation converges in a single location"; now upgraded to a hard constraint — dbmon's SQL battery converges into the core `DBChecker` (`database/sql` stdlib, drivers registered at assembly), see profile §6                                                                                         |
| "The previously designed game-client simulation probing"                       | **Not found in docs, but the design did exist**              | Nothing under `docs/`, `grep ProbeTarget` has zero hits — but the **user clarified on 2026-10-14**: the design existed, the user decided to **defer implementation**, and it was **never written down**. The correct description is "a legacy requirement decided as deferred and never documented", **not a brand-new requirement**; when registering the TODO this origin must be stated honestly |
| Preconditions for landing "no second engine"                                   | **Verified**                                                 | This repo has **no** reusable game-protocol client/protocol stack: `sdks/` is the game-server→agent registration surface, and `examples/cmd/e2e-agent-probe` is Croupier's own wire mock — game protocol adapters must be written fresh per game (profile §5.4; the engineering cost is the basis for the P2 rating)                                                                                |
| Three-layer structure (kernel / protocol probes / execution location)          | **Partially in place**                                       | Kernel + http/tcp/process probes exist (`healthprobe.go:32/43/74`, `process_unix.go`); the db probe is a second server-side implementation pending convergence (`internal/api/dbmon/probe.go`); the execution-location (assembly) layer does not exist yet (M1) — profile §5.1                                                                                                                      |
| Agent/Server division-of-labor table                                           | **Mergeable, merged**                                        | The external 7-row table matches this repo's "whoever's perspective matters does the probing" principle; merged into a single authoritative table (the external perspective rows added: player-network reachability adopted, game-process-alive = sidecar same-machine semantics); profile §4.2 is the sole authoritative source                                                                    |
| Five design constraints                                                        | **All adopted / already satisfied**                          | "Probe failure does not trigger local process restart" already has structural isolation: the recovery loop's sole trigger is the process-exit event (`ops_server.go:426-439`) plus the permission double gate (`:443-448`); the probe's `OnWindow` only logs — item-by-item verdicts in profile §5.2                                                                                                |
| Windows process-probe accuracy                                                 | **"Unified interface, degraded underneath" partially holds** | `process_windows.go:9-27`: FindProcess always succeeds, so the PID-reuse window reports alive falsely (missed-alert direction, not false kills); keep the degradation plus an honest comment, and `x/sys/windows` remains core's only potential exceptional dependency — to be decided separately then — profile §5.3                                                                               |

## Landing Tracker

- [x] The probe kernel's three-layer structure written into `docs/design/agent-health-recovery-review.md` (§5.1: kernel exists / probes partially exist pending convergence / execution-location layer is new) — this commit (2026-10-14 docs consolidation batch)
- [x] The five design constraints entered into the design doc one by one (§5.2: constraint 2 already has structural isolation evidence at `ops_server.go:426-448`; constraint 5 lands as alertID perspective tagging) — this commit
- [x] Agent / Server division-of-labor table merged into a single canonical version (profile §4.2 marked as the sole authoritative source; the external table is no longer cited separately) — this commit
- [x] Game-client simulation probing registered in `todo.md` (origin = a legacy requirement decided as deferred and never documented; P2; player perspective mandatory) — this commit
- [x] `ProbeTarget` naming ruling: **new term not adopted** — keep the existing `Probe`/`Checker`/`Window`/`Window.Target` (`probe.go:21`), with the config surface using the `probe.targets` section — this commit
- [x] Windows process-probe accuracy check (§5.3: unified interface with degraded underneath is accurate — FindProcess always succeeds + PID reuse reports alive falsely; keep the degradation plus boundary note; `x/sys/windows` to be decided separately later) — this commit
- [ ] Execution-location and representative network placement plan for player-side probing (delivered when game-client simulation probing is chartered as P2)
- [ ] R2–R4 code batches each with commit hash + test count (start accumulating when the R2 assembly batch begins)

Note: this batch is docs-only (the docs check script passes; no test changes); the verification evidence for the checked items above is the file:line citations (see the verification conclusions table).

## The User's Settled Position (2026-10-14, authoritative; overrides any contrary statement in the external advice)

1. The design **existed but was never documented**; at the time it was decided to **defer implementation** because of "added complexity and clients scattered everywhere".
2. **Server-to-server mutual probing is rejected**: mutual probing from the server side was once considered and the user said explicitly "**that would lose most of its point**" — not accepted as an alternative.
3. **Implementation stance**: **if it is done, it is done from the player's client**. The player-side perspective is mandatory, not an optional enhancement; no "make do with server-to-server probing first" interim plan is accepted.
4. Therefore the external review's division-of-labor row "player-facing cross-network game reachability → probe Agents deployed at representative network positions" is consistent with this position and is adopted.
