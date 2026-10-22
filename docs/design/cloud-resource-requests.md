---
title: Cloud Resource Request and Provisioning Design (the request-approve-execute-recycle loop)
---

# Cloud Resource Request and Provisioning Design

## Status

- **Design sheet (user, 2026-10-15 — document first, no code)**. This round delivers this design document only; no implementation is touched. Implementation follows at project start as an independent batch: numbered migration + RBAC permission points + three console pages.
- User's words: "ops can control each cloud provider's server creation; developers file requests, and after approval ops execute" and "the requestable resources need to be listed too".
- Linkage to existing capabilities is annotated with file:line from this repo's reality (§7) — no second resource-management system is started.

## 0. Three Roles and the Hard Boundary

Three roles and responsibilities (user's wording, unchanged):

| Role          | Responsibility                                                  |
| ------------- | --------------------------------------------------------------- |
| **Developer** | Files resource requests (type/spec/purpose/duration)            |
| **Approver**  | Approves requests (approve/reject + reason)                     |
| **Ops**       | Actually executes creation/recycling on the cloud provider side |

**Design red line (this feature's core security constraint): developer and ops permissions must be separated.**

1. Developers **can only file requests** and **can never call cloud APIs to create resources directly** — the sole trigger of cloud-side create/recycle actions is ops.
2. The red line binds three layers at once: **orthogonal RBAC permission points** (`cloud.request.create` does not include `cloud.request.execute`, see §6), **separated frontend entries** (developers see only requests and "my requests", never the execution console), and **API-level interception** (calling cloud execution endpoints without the execute permission is always 403 — the server enforces and never trusts frontend hiding).
3. **Credential isolation**: cloud provider secrets live only on the ops side; the developer and approval chains never touch any cloud credentials (§5).

## 1. Resource Catalog (configurable, maintained by ops)

Requestable resources form a **data-driven catalog**, never hard-coded: the catalog is a table (ops CRUD), the request UI pulls from the catalog API, and an empty catalog = nothing requestable on the developer side (an honest empty state, never a disguised option).

### 1.1 Category Closed Set

`compute | database | storage | network | other` (closed enum; a new category = change the enum + migrate, never free text):

| Category | Items (catalog entry examples, not code)                            |
| -------- | ------------------------------------------------------------------- |
| compute  | Cloud servers ECS/CVM/EC2 (spec families, CPU/memory tiers, images) |
| database | MySQL / PostgreSQL / Redis / MongoDB managed instances              |
| storage  | Object storage buckets, cloud disks                                 |
| network  | Public IPs/bandwidth, VPCs, subnets, security groups                |
| other    | CDN, message queues, logging services (added/removed as needed)     |

### 1.2 Catalog Entry Fields

| Field            | Description                                                                                                                                                                                      |
| ---------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| name             | Catalog item name (unique)                                                                                                                                                                       |
| category         | Category closed set (above)                                                                                                                                                                      |
| provider         | Cloud provider (aliyun/tencent/aws/…, set at ops configuration)                                                                                                                                  |
| specOptions      | Spec range: a preset tier list + boundary notes (e.g. 2c4g/4c8g/8c16g); requesters pick within the range                                                                                         |
| defaultDuration  | Default request duration (for temporary resources; ignored for permanent)                                                                                                                        |
| approvalRequired | Whether approval is required (low-risk items may skip — skipping still leaves a trail, see §4 audit)                                                                                             |
| costEstimate     | Billing estimate basis (a static monthly cost value or a short note; estimates are approval references only, cloud bills are authoritative — formulaic billing is undecided, open question §8.5) |
| project          | Owning project                                                                                                                                                                                   |
| lifecycle        | **temporary / permanent** marker                                                                                                                                                                 |

### 1.3 Temporary/Permanent and the atlas Alignment

The `lifecycle` marker aligns with atlas's registered-lifecycle concept (atlas manages the user's own server states/maintenance windows; this repo consumes it via the serverstatus Provider, `internal/platform/serverstatus/atlas.go:45`):

- **temporary**: expected duration required at request; the execution receipt carries `expiresAt` (§3); expiry enters the remind→recycle flow (§4). This corresponds to atlas's "registered service entries with a decommission time".
- **permanent**: no expiry; recycling must go through an explicit recycle request (executed by ops) — silent disappearance is never allowed. Corresponds to atlas's "resident service entries".
- Linkage direction (v2 evaluation, §8.6): fulfilled/recycled receipts **write one-way into the atlas server list** (register/decommission); this repo never consumes atlas in reverse as a catalog source — the catalog's authority is here, and atlas only consumes results.

## 2. Request Model

A new domain document (not stuffed into the existing approvals' function semantics — reasoning in §7.1):

```
CloudResourceRequest
  requestNo        business number (readable, unique)
  requester        requester (admin account)
  catalogItemId    catalog item reference
  catalogItemName  catalog item name snapshot (later renames never affect historical documents)
  provider         cloud provider (catalog snapshot)
  spec             spec selection (JSON: tiers/quantity/region etc., constrained by the catalog's specOptions)
  purpose          purpose (required; the basis for approval and audit)
  duration         expected duration (required for temporary)
  project          owning project (catalog snapshot; changing it at request needs approval confirmation)
  gameId / env     owning game domain (the resource view enters game scope, see §8.4 open question)
  refTicket        linked ticket/project (optional)
  state            state machine (below)
  rejectReason     rejection reason (approver-required)
  createdAt / updatedAt
```

### 2.1 State Machine

```
pending ──approve──> approved ──execute(ops receipt)──> fulfilled ──(temporary expiry)──> expiring ──recycle──> recycled
   │
   └──reject──> rejected ──(modified and resubmitted)──> pending (the resubmission keeps the original number + revision+1; the rejection chain is retained)
```

- `rejected` allows **resubmission after modification** (the same document's revision increments; history is never deleted — rejection reasons and change trails are fully retained).
- After `fulfilled`, temporary resources enter `expiring` driven by the expiry time; a completed recycle lands `recycled` (with recycle time and executor backfilled).
- An `approved` request not executed within the timeout (configurable, default 7 days) reminds ops but is never auto-canceled (ops execution is an offline action; reminders never seize the authority).

### 2.2 Approval Flow

- **Approver determination**: the RBAC permission point `cloud.request.approve` (role-configured, §6).
- **Levels**: v1 is single-level; two-level approval stays a config item (`approvalLevel` catalog/global config — not implemented until decided, §8.3).
- **Self-approval interception**: requester = approver is refused directly (reusing the existing self-approval interception precedent, `internal/api/approval/self_approval_test.go`).
- Approval-free items (`approvalRequired=false`) go directly `pending → approved`, but still flow through the same document (approved by=system, with a trail).

## 3. Execution and Receipts (the ops execution console)

**v1 execution = manual**: ops creates in the cloud console and returns to this repo's execution console to **file a receipt** — this repo does not call cloud APIs (cloud API automation is a later optional batch, §8.7; if built, the credential and execution-machine constraints of §5 apply).

The execution console (visible only with `cloud.request.execute`) lists `approved` pending executions: requested spec, purpose, expected duration, project. The execution receipt form (submitting sets `fulfilled`):

| Receipt field        | Description                                                                                            |
| -------------------- | ------------------------------------------------------------------------------------------------------ |
| provider / region    | The actually provisioned cloud and region (may differ from the request; the receipt is the live truth) |
| instanceId           | The cloud-side instance/resource ID                                                                    |
| privateIp / publicIp | Network info (optional)                                                                                |
| expiresAt            | Expiry (required for temporary; drives §4's expiry chain)                                              |
| costEstimateActual   | Actual monthly cost estimate (optional)                                                                |
| operatorNote         | Ops note (delivery notes, e.g. "joined security group sg-xxx")                                         |

Receipts close the document loop: request (developer) → approval (approver) → execution (ops), each stage leaving its own operator/time trail on the document.

## 4. Recycling and Expiry

- **Temporary resource expiry reminders**: reminders generated at T-7d / T-1d before `expiresAt` (reusing the Alert lifecycle surface, `internal/model/alert.go:11`, source=cloud_resource, carrying the requestNo; no new reminder system). Visible to both requester and ops.
- **Recycle action**: still the ops execution domain (the same permission point + the same execution console as creation), with the receipt recording recycle time and executor. **Expiry never auto-deletes** — this repo does not know the blast radius of cloud-side deletion; expiry only escalates the reminder (the escalated state may mark human handling, reusing the Alert handleStatus workflow).
- **Second confirmation**: recycle execution submissions get a frontend second confirmation (the Popconfirm precedent); **permanent resource** recycling must go through an explicit recycle request (ops can never mark recycled without a document — the document is the authorization).
- After recycling, the document's terminal state is `recycled`; the corresponding atlas server entry is decommissioned (v2 one-way write, §1.3).

## 5. Credential Security

- **v1 (manual execution)**: cloud console credentials never enter this repo — the credential surface is naturally isolated.
- **If cloud API automation comes later** (optional batch, §8.7):
  1. Cloud provider secrets live only on the ops side, referenced via the password-vault `vault://` scheme (the `internal/platform/vault/vault.go:44` Provider interface, the `:83` VaultRefPrefix machine-scannable precedent); secrets never land in the database or in config plaintext;
  2. The execution endpoint is reachable only with `cloud.request.execute` (ops); the server holds only the vault reference, never plaintext, and credentials are consumed on the **ops-designated execution machine** (agent side);
  3. Developer and approver roles' cloud-credential read/write permission points **do not exist** (not disabled — the permission point simply is not there).
- In every shape, credential values never appear in requests/approvals/receipts/audit payloads — audit records "who executed", never "with what".

## 6. RBAC Permission Points (the red line's technical expression)

Four orthogonal permission points; none contains another:

| Permission point        | Role      | Capability                                  |
| ----------------------- | --------- | ------------------------------------------- |
| `cloud.catalog.manage`  | Ops       | Catalog CRUD                                |
| `cloud.request.create`  | Developer | File requests, resubmit, view own documents |
| `cloud.request.approve` | Approver  | Approve/reject (self-approval intercepted)  |
| `cloud.request.execute` | Ops       | File execution receipts, execute recycling  |

- Role assembly uses the existing RBAC role system (the approval-role determination precedent at `internal/security/rbac/unified.go:286`).
- Server-side enforcement: the execute and catalog-management endpoints check permissions at the handler entry, returning 403 with the unified error contract (`{error, message, details}`, the CLAUDE.md API response contract).
- The frontend renders entries by permission point but **never relies on frontend hiding for security** (§0 red line item 2).

## 7. Linkage with Existing Managed Resources (no second system)

| Linkage point                                   | Location                                                                                                                        | Relationship                                                                                                                                                                                                                                                                                    |
| ----------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Auto-adopting new machines (agent registration) | `internal/server/control_handler.go:1041` (handleRegisterRequest)                                                               | A cloud-created machine installing an agent and registering outbound lands in the Ops/Nodes view; receipt instanceId/IP and the node view cross-query by IP/host (panel-linked jumps), never copying node data                                                                                  |
| Managed processes                               | `internal/app/agent/ops_config.go:89` (ManagedProcessConfig, kind: program/probe-adapter)                                       | Game processes/sidecars on new machines use the existing supervisor lifecycle (start/stop/supervision/snapshots/scheduling/updates); the resource domain never re-invents process management                                                                                                    |
| Health probe targets                            | `core/healthprobe/targets.go:24` (TargetConfig) + `internal/app/agent/probe_manager.go:304` (ApplyTargets dynamic distribution) | New machines'/dependencies' probe targets use the existing probe.targets channel; the server-side derived-dynamic-target chain is live, so appending probe targets after a receipt is a config action, not a new mechanism                                                                      |
| Maintenance window status source                | `internal/platform/serverstatus/provider.go:18` (Provider) + `internal/platform/serverstatus/atlas.go:45` (AtlasProvider)       | Temporary resource expiry/recycling aligns with atlas's server-entry lifecycle concept (§1.3); the maintenance-window gate (`internal/svc/serverstatus_gate.go`) keeps consuming per the existing rules                                                                                         |
| Approval domain                                 | `internal/platform/approvals/store.go:16` (Approval, two-person record)                                                         | **Pattern reuse, not data reuse**: resource requests are domain documents (catalog references/specs/receipts/expiry), a different domain from function-invocation approvals (FunctionID/Route/HashKey semantics); behavioral rules like self-approval interception copy the approvals precedent |
| Audit chain                                     | `internal/audit/audit.go:20` (ErrInvalidChain hash chain verification) + `internal/audit/chain/writer.go`                       | All resource-domain events go through the existing hash chain audit — no new audit storage                                                                                                                                                                                                      |
| Expiry reminders                                | `internal/model/alert.go:11` (the Alert lifecycle firing/resolved + the handleStatus human-handling surface)                    | Expiry reminders/escalations reuse the Alert surface (§4); no new reminder system                                                                                                                                                                                                               |

### 7.1 Why the request does not go into the approvals table

The approvals platform's document semantics are "two-step authorization of function-invocation executions" (store.go Approval: FunctionID/Route/TargetServiceID/HashKey/Payload), with a lifecycle ending at approved/rejected + the execution result. A resource request carries a **catalog reference, specs, an execution receipt (instanceId/IP/expiresAt), and the expiry→recycle long-tail state machine** — forcing it in would grow ten nullable columns on approvals and pollute two state machines with each other. The design takes: **the same approval behavior rules (two-person, self-approval interception, rejection reasons), different domain tables**; if a third similar domain appears, extracting a generic approval kernel will be evaluated then (never pre-built).

## 8. Data Model and Migration / Open Decisions

### 8.1 Tables (landed by the implementation batch, with the numbered-migration three-way sync)

| Table                     | Content                                                           |
| ------------------------- | ----------------------------------------------------------------- |
| `cloud_resource_catalog`  | §1.2 catalog entries (ops-maintained)                             |
| `cloud_resource_requests` | §2 request documents (with state machine/receipt fields/revision) |
| Audit                     | Reuses the existing audit chain — no new audit table              |

- Model columns/tables must come with the `internal/svc/migrations.go` numbered migration + the `internal/db/migrate/migrate.go` version bump + migration tests, all three in sync (the CLAUDE.md migration contract; existing databases never run AutoMigrate).
- Whether game-scope fields (gameId/env) become columns: §8.4.

### 8.2 Frontend (implementation batch)

Three pages: resource catalog (ops-maintained), my requests (developers), approval and execution consoles (approvers/ops). Column layout matches the Ops drawer style; the LocalizedText contract, the `any` ban, and the API response contract follow repo-wide rules.

### 8.3 Open Decisions (must be settled before implementation — never guessed)

1. **Catalog management permission placement**: a standalone `cloud.catalog.manage` (recommended) or merged into the existing ops management surface.
2. **Approver role source**: a new RBAC role (recommended) or a fixed admin group.
3. **Two-level approval**: v1 single-level (recommended), with two-level left as a config slot.
4. **Game scope**: whether requests carry gameId/env (recommended yes — the resource view entering the game dimension matches the repo-wide single-company-multi-game model).
5. **Billing estimate basis**: static monthly cost values in the catalog (recommended for v1) or formulaic (needs a spec→price table; heavy engineering).
6. **One-way atlas write-back**: whether fulfilled/recycled auto-writes the atlas list (recommend v2 evaluation; manual atlas maintenance first).
7. **Cloud API automation**: v1 manual receipts (recommended); automation is a separate project with §5's credential/execution-machine constraints.

## 9. Acceptance Criteria (referenced by the implementation batch)

- Developer account: can request, resubmit, and view own documents; catalog management and execution endpoints return 403; no page/API ever shows cloud credentials.
- Approver account: can approve/reject (self-approval intercepted); cannot execute, cannot edit the catalog.
- Ops account: can file execution/recycle receipts (permanent recycling requires a document); cannot file requests on a developer's behalf (execute does not include create).
- Approval-free items: pass straight to approved, with the trail approved by=system.
- The expiry chain: temporary fulfilled with expiresAt → T-7/T-1 reminder Alerts generated → recycle lands recycled, fully trailed.
- Empty catalog: the request page shows an empty state — never a clickable form bound to fail.
