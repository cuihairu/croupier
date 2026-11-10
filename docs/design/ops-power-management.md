---
title: Ops Power Management Design
---

# Ops Power Management Design

Date: 2026-11-11. Status: **implemented as designed in this document** (OPS-COMPLETE-001, local mock verification; real BMC/WOL/cloud hardware verification pending environment). This page designs the remote machine power lifecycle capability for the ops plane: power on, power off, reboot, and status query, all behind the existing approval chain.

## 1. Goal and Scope

The ops plane already has provision (cloud provisioning/destroy via terraform-exec), batchops, runbooks, supervisor, and the approval service with OTP step-up. What is missing is the **physical/remote machine power lifecycle**: turning a machine on, off, or rebooting it, and querying its power state. The motivating scenario is automated recovery — supervisor finds a service down because its machine is off; the platform must be able to power the machine on, wait for readiness, and let supervisor bring the service back.

The platform never invents a second approval system. Power operations reuse the approvals store, the OTP step-up, the two-person rule, and the audit chain exactly as provision ticket mode does.

## 2. Current-State Facts This Design Builds On

- **Provision Run pattern** (`internal/ops/provision`): `Run{ID, Request, State, ApprovalID, Error, CreatedAt, UpdatedAt}` with states `pending → applying → waiting_register → adopted / failed` plus a destroy sub-machine `destroy_pending → destroying → destroyed / destroy_failed`. The orchestrator persists the run first, then creates the approval record and links `run.ApprovalID`. Resume re-checks approved state before executing (`ErrRunNotApproved` → 409).
- **Approval gate abstraction** (`internal/svc/provision_wiring.go`): subsystems create approvals via `approvals.Store.Create(&Approval{FunctionID, GameID, Env, Actor, Mode, Route, Metadata})` and read back `State == "approved"`. Provision uses `FunctionID: "ops.provision"`, `Mode: "provision"`, `Route: "/ops/provision"`, `Metadata: {provisionRunId}`. Powerops follows with `FunctionID: "ops.power.*"`, `Mode: "powerops"`, `Metadata: {powerRequestId, targets, action}`.
- **OTP step-up** (`internal/api/approval/stepup_otp.go`): approve-time risk is resolved from the approval's `FunctionID` via `function.ResolvePolicyRisk`; resolution failure is fail-closed to high. High/danger approvals require TOTP when `security.approvalStepUpOtp` is enforced (default on); stable error codes `otp_required` / `otp_invalid` / `otp_not_enrolled`.
- **Two-person rule**: an approver cannot approve their own request (`self_approval_forbidden`), with the explicit single-admin escape hatch `approval.allowSelfApprove`; both outcomes are hash-chain audited.
- **Agent execution channel**: `internal/logic/ops.GetAgentOpsClient().GetClient(ctx, agentID)` yields `ExecuteCommand` / `PushFile` (opsv1 wire), already used by batchops. `POST /ops/nodes/:nodeId/restart` restarts the **agent process** over the control channel (`MsgRestartProcessRequest`) — it is not a host reboot and has no approval gate. Power ops needs both: in-band agent channel for graceful host reboot, out-of-band paths for everything when the agent is down.
- **Machine inventory**: the `nodes` table (`NodeID`, `Meta datatypes.JSONMap`) is the existing machine model. Powerops mounts its per-machine power config under `nodes.meta.power` instead of adding a new table (so no new numbered migration is required for the inventory itself; power **requests** get their own table, see §6).
- **Batch semantics** (`internal/ops/batchop` + `internal/api/ops/batchops.go`): registry-based target selection, dry-run default, per-target result aggregation, and the `context.WithoutCancel` background execution so a disconnected client cannot leave a batch stuck running (CR-FIX-22). Powerops reuses the same aggregation and background execution discipline.

## 3. Capability Matrix

Machine type × power action, with protocol and credential source:

| Machine class                                    | Power on                                  | Graceful power off                                         | Force power off                                    | Reboot                                                            | Status query                                | Protocol / credential                                                                                                                                           |
| ------------------------------------------------ | ----------------------------------------- | ---------------------------------------------------------- | -------------------------------------------------- | ----------------------------------------------------------------- | ------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Physical host with IPMI/BMC                      | ✅ `ipmitool chassis power on`            | ✅ `chassis power soft`                                    | ✅ `chassis power off`                             | ✅ `chassis power cycle` / `reset`                                | ✅ `chassis power status`                   | ipmitool CLI over BMC network; BMC credentials from server config (`powerops.ipmi.*`) with username/password resolved from the credential store when configured |
| LAN machine with Wake-on-LAN                     | ✅ magic packet (UDP broadcast to port 9) | ❌                                                         | ❌                                                 | ❌                                                                | ❌ (infer: agent session online after wake) | WOL is one-directional; wake confirmation = agent reconnects                                                                                                    |
| Cloud instance (provision-managed or registered) | ✅ provider start                         | ✅ provider stop (graceful where the provider supports it) | ⚠️ provider force-stop semantics vary              | ✅ provider reboot                                                | ✅ provider status                          | Provider CLI/API via config-declared command template (`powerops.cloud.*`); cloud credentials stay in server config, never in API payloads                      |
| Agent host (agent session alive)                 | n/a (already on)                          | ✅ graceful shutdown via agent channel                     | ❌ (agent dies mid-shutdown; use BMC path instead) | ✅ graceful reboot via agent channel, then verify agent reconnect | ✅ agent session + system-info liveness     | agent-ops `ExecuteCommand` (e.g. `systemctl reboot` / `shutdown -h now`), subject to agent-side command policy                                                  |

Legend: ✅ first-class support; ⚠️ supported with caveats stated; ❌ protocol cannot express the action.

Notes the matrix implies:

- WOL and IPMI are **out-of-band**: they work when the target's agent is off — that is the whole point of the auto power-on scenario.
- Agent-channel graceful reboot is **in-band**: it dies with the machine, so its verification step is "agent session reconnects within N seconds", and its use is confined to planned maintenance where the machine is expected to come back.
- Cloud force-off is provider-specific; powerops reports the provider's own result rather than pretending uniform semantics.

## 4. Requirement Scenarios

1. **Auto power-on recovery** (primary): supervisor/healthprobe observes a service down; diagnosis says the machine is off (agent session absent + BMC status off). Operator (or runbook step) submits `POST /ops/power` with action `power_on`; after approval the BMC/WOL provider executes; verification polls BMC status / agent reconnect; done state records who/when/how.
2. **Maintenance window batch shutdown**: operator selects N machines and action `power_off` (graceful). Batch semantics: one request, per-machine result rows, partial failure does not hide successes; machines with live agents get in-band graceful shutdown, headless ones get BMC soft-off.
3. **Exception reboot**: a hung machine gets `reboot`; risk-tiered approval per §5; verification = machine returns (BMC reachable / agent reconnects) within the configured window, otherwise the request lands `failed` with the last observed state — it never lies "done" on a machine that did not come back.

## 5. Approval Policy Matrix

Actions map to risk levels; the request's `FunctionID` (`ops.power.power_on` / `ops.power.power_off` / `ops.power.force_off` / `ops.power.reboot` / `ops.power.status`) is what the approval step-up resolves:

| Action                                       | Risk                                                            | Approval flow                                                                                                                                                                                                  | OTP                                           |
| -------------------------------------------- | --------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------- |
| `power_on`                                   | low                                                             | single approval, default workflow                                                                                                                                                                              | not required by step-up (audit only)          |
| `reboot` (graceful)                          | medium                                                          | single approval                                                                                                                                                                                                | per step-up policy (medium is not OTP-forced) |
| `power_off` (graceful)                       | medium                                                          | single approval                                                                                                                                                                                                | per step-up policy                            |
| `force_off`                                  | high                                                            | single approval                                                                                                                                                                                                | **OTP enforced** (high ⇒ step-up)             |
| `reboot`/`power_off` on batch (multi-target) | max risk of member actions; batch declared in approval metadata | one approval covering the declared target list (targets are snapshotted into the approval metadata at request time — the approved scope cannot silently change); per-machine results land in the power request | OTP if any member is high                     |

Who requests / who approves: exactly the existing approval model — requester is the authenticated admin (recorded as `Actor`), approver is any admin other than the requester (two-person rule), with the same `approval.allowSelfApprove` escape hatch and audit trail. `power.status` is a query: it needs authentication + agent/machine scope but no approval.

Fail-closed alignment: if the policy manager cannot resolve risk for a `ops.power.*` function ID, the approval step-up treats it as high (existing behavior), so an unconfigured deployment never loses the OTP gate on dangerous actions.

## 6. Request State Machine

One power request covers one action and a target machine list. States follow the provision Run pattern:

```text
pending ──(approved)──▶ executing ──▶ verifying ──▶ done
   │                      │             │
   └──(rejected)──▶ rejected          ├──▶ failed        (any machine failed verification)
                                      └──▶ partial      (some machines done, some failed)
```

- `pending`: request persisted (request table row with targets snapshot + action + reason + requester); approval record created and linked (`ApprovalID`). Provision precedent: persist first, then create approval, then link.
- `approved → executing`: approval approved; execution starts. Per-machine attempts run through the provider; each machine row records attempt time, raw result, error.
- `executing → verifying`: for actions with observable effects (power_on, reboot, power_off), poll confirmation — BMC `power status`, cloud provider status, or agent session reconnect — until a deadline (`powerops.verifyTimeout`, default 120s, per-attempt backoff).
- `done` / `partial` / `failed`: terminal states with per-machine outcomes; `partial` exists because batch power is the one place where "some machines came back" must be distinguishable from "all came back".
- Rejection path: approval rejected ⇒ request `rejected`, nothing executes.
- Execution runs on a `context.WithoutCancel`-derived background context (CR-FIX-22 discipline): a disconnected HTTP client must not leave the request stuck in `executing`.

Storage: new table `power_requests` (header) + `power_request_targets` (per-machine rows), added under the numbered-migration contract (`internal/svc/migrations.go` registration + `MinimumRequiredVersion` bump + synthetic-migration test sync, all in the same PR as the model). The machine inventory itself stays on `nodes.meta.power` — no inventory migration.

## 7. Package and API Surface

- `internal/powerops`: `Provider` interface `PowerOn / PowerOff / Reboot / Status` (context + machine + options → result), implementations `ipmi` (ipmitool CLI wrapper; command assembly covered by tests against a fake binary), `wol` (UDP magic packet), `cloud` (config-declared command template), `agent` (in-band graceful reboot via agent-ops client). `RequestStore` + state machine + verify loop live here too, so the HTTP layer stays a DTO mapping shell (provision layering precedent).
- `internal/api/ops/powerops.go`: `POST /ops/power` (targets + action + reason → pending request + approval id), `POST /ops/power/:id/resume` (approval passed → execute, provision-resume precedent), `GET /ops/power/:id` (state + per-machine results), `GET /ops/power` (list). Mounted under the existing `/ops` group with its feature flag; provider not configured ⇒ honest `503 powerops_not_configured`, never a silent no-op.
- Machine registration: `PUT /ops/agents/:agentId/power-config` writes `nodes.meta.power` (provider kind, BMC address, MAC, cloud instance ref) with agent-scope enforcement like every other per-agent endpoint.
- Audit: `powerops.request`, `powerops.execute`, `powerops.verify` events through `recordOpsAudit`, same as provision destroy.

## 8. Dependencies

- **BMC credentials**: `powerops.ipmi.*` server config (address template, username, password, optional `passwordEnv` env reference following the provision bootstrap-token pattern — committed config never carries the secret). No `vault://` integration exists in the repo today; the design names the env-reference pattern as the credential source of record and leaves a vault backend as future work (honest boundary).
- **Execution channel**: in-band actions ride the existing agent-ops client (no new agent plugin required); out-of-band actions run from the **server host** (ipmitool / UDP broadcast / cloud CLI). The design does not require agents to reach BMC networks — that would couple agent deployments to management LANs. If a deployment's server host cannot reach a BMC, that machine is power-unreachable and the request records it as such (honest failure, not a hang).
- **ipmitool binary**: expected on the server host at the configured path; missing binary ⇒ provider construction fails at submit time with `powerops_ipmi_unavailable`, not at approve-then-fail time.

## 9. Non-Goals and Honest Boundaries

- **No real BMC/WOL/cloud verification in this repo's CI**: there is no hardware. All provider behavior is covered by mocks plus a fake ipmitool binary for command assembly; **real BMC/WOL verification awaits an environment with hardware** — this is recorded, not hand-waved.
- **No IPMI network reachability from server ⇒ unreachable machines**: the platform reports `unreachable`/`failed`; it does not pretend, and does not route power through arbitrary agent hops.
- **WOL is wake-only**: no graceful off, no status; confirmation is agent reconnection, which requires the machine's agent to be configured for autostart — outside platform control, stated as a deployment prerequisite.
- **Cloud actions are command-template driven**, not deep provider SDK integrations: `powerops.cloud.*` declares per-action command lines (e.g. provider CLI invocations). Provision's terraform workspace manages create/destroy, not instance start/stop, so powerops does not pretend to reuse it; a future provider-SDK integration would replace the template layer.
- **No automatic unattended power-on loops**: the platform does not auto-approve or auto-retry power actions. The supervisor-driven scenario enters through the same human-approved request; automation may _draft_ the request, never _approve_ it.
- **Force-off on agent hosts is refused** (the channel dies with the host): the API maps such requests to the machine's out-of-band provider when one is registered, or fails honestly when none is.

## 10. Verification Plan (Local)

- Unit: provider mocks (state transitions, verify loop, partial failure aggregation), state machine (pending→rejected on approval reject; resume requires approved), ipmitool command assembly against a fake binary, WOL packet bytes.
- Handler: submit → pending + approval id; unapproved resume → 409; reject path executes nothing; provider unconfigured → 503.
- Real-hardware acceptance (power_on/off/status against a BMC, WOL wake of a suspended machine, cloud start/stop) — **pending environment**, tracked as the known boundary above.
