---
title: Permissions Notes (RBAC, Auth, Sensitive Operations, and the Bastion Status)
---

# Permissions Notes

For security and operations roles: an inventory of this repo's real permission-control implementation paths — the roles-and-permissions model, the auth chain, sensitive-operation controls, and the current state of bastion/jump access. Each item carries file:line evidence; unimplemented or skeleton-only parts are labeled honestly. Facts verified as of 2026-10-20; line numbers may drift with later evolution, so treat the verification date as authoritative.

> Scope boundary: `docs/zh/guide/concepts/permissions.md` and `docs/zh/operations/security.md` are user-facing concept and configuration docs; this page is written against the code implementation. Where they disagree, the differences noted here (see "Known doc-vs-code discrepancies" at the end) are authoritative. CI/CD is not covered here — that is in the DevOps notes.

## Roles and Permissions Model (the production RBAC path)

### Decision Entry Points

- **There is exactly one production decision path**: `EnforceAnyPermission(subject, granted, required...)` at `internal/security/rbac/logical_permissions.go:40` — driven by the Casbin "equality model" (`logicalPermissionModel`, `:10`), supporting `resource:action` splitting and `*` wildcards (`:96` onward). The server injects it as a variable: `internal/service/permission/service.go:35` (`enforceAnyPermission = rbac.EnforceAnyPermission`).
- **CheckPermission flow**: from `internal/service/permission/service.go:37` — load admin roles (`super_admin` passes directly), generate candidate permission strings via `permissionCandidates(resource, action)` (`:215`, including `*` and `resource:*` forms), and pass on any hit.
- **Roles and permissions data model**: `roles` (`internal/model/role.go:6`, table `roles`), `permissions` (`internal/model/permission.go:10`, table `permissions`), the `role_permissions` join, and the `admin_roles` join. **Roles are global and carry no game dimension** — game/env isolation lives in admin-level scope tables: `admin_game_scopes` and `admin_game_env_scopes` (`internal/model/admin_scope.go:6`/`:18`).
- **game/env scope decisions**: `CheckGameScope` (`service.go:82`) and `CheckGameEnvScope` (`:99`) query those two scope tables; on the HTTP side the `X-Game-ID`/`X-Env` headers are parsed into the scope context and validated per request (`internal/service/permission/middleware.go`). That is: **multi-game isolation relies on admin scope tables, not on role duplication**.

### Seed Data and External Identity

- **Seed roles/permissions**: `configs/roles.json` with 41 roles and `configs/permissions.json` with 39 permissions (measured from the JSON entries on 2026-10-20), persisted at startup by `seedBootstrapRoles` (`internal/svc/service_context.go:1100`, call site `:576`).
- **Identity provider abstraction**: `internal/security/identity/provider.go:1-9` states the boundary — the Provider only answers "who you are" (`KindLocal`/`KindLDAP`/`KindOIDC`/`KindGitHub` constants `:19-22`); "what you can do" is always decided by local RBAC (admins/roles tables); external identity logins JIT-provision a local shadow account as needed (`assignDefaultRoles` at `internal/api/auth/service.go:506`, implemented at `:513`).
- **Gap (model not wired)**: `internal/security/rbac/` also contains `rbac.go` (file policy), `casbin.go` (Casbin enforcer wrapper), `unified.go` (UnifiedPolicyEngine), and `enhanced.go` (ABAC expression engine) — **none of these engines has a production caller**: the file-policy engine's only non-test entry is the CLI config validation `internal/cli/common/validate.go:86-89` (checking that the `rbac_config` file is readable); runtime authorization never goes through them. Any doc text describing an "ABAC policy engine" capability reflects an unwired state.

## Auth Chain

### Token Issuance and Verification

- **JWT**: `Sign(...)` at `internal/security/jwtutil/token.go:32`, HS256 (`:50`), claims carry `tokenVersion` (`:19`) and `adminID`; TTL 24 hours (`:23`).
- **Secret resolution**: `ResolveSecret` at `internal/security/jwtutil/secret.go:37` — outside dev mode, a missing `CROUPIER_AUTH_JWT_SECRET` / `auth.jwtSecret` is a hard error that refuses startup (`:44`); dev mode falls back to `croupier-dev-secret` (`:12`/`:42`). Production deployments must configure it explicitly.
- **Middleware wiring**: `internal/svc/service_context.go:62` declares `Authority gin.HandlerFunc`, wired at `:626`; the HTTP routes in `internal/handler/routes.go:119-120` attach `serverCtx.Authority` to the whole `protected` group.

### Request-Side Verification

- **Token extraction**: the `Authorization: ***` header, with a `?token=` query fallback for `<img>`/EventSource scenarios such as SSE (`internal/svc/service_context.go:1629-1641`).
- **authenticate**: from `:1736` — after parsing claims it force-compares the current `admins.token_version` (`:1747-1751`, `checkTokenVersion` `:1758`): password change/disable/logout increments the stored version, invalidating old tokens immediately. A short-TTL cache reduces database lookups.
- **mfaGate**: `:1686-1707` — when the site security policy `MFARequired` is on, accounts without TOTP bound get `403 mfa_required` except for whitelisted prefixes (`:1699-1703`).
- **Public endpoint whitelist**: public routes registered at `routes.go:108-116` (registry read-only, OpenAPI read-only, update check, CI webhook, alert push, player support, etc.) bypass Authority; token and HMAC verification for webhook/alert endpoints lives inside each handler (`:110-112` comment). The Prometheus scrape endpoint joins the no-auth whitelist when enabled (`service_context.go:1615`).
- **External tokens (API tokens)**: `HashExternalToken` at `internal/model/external_token.go:18-21` — the plaintext is returned exactly once at creation, the store keeps a sha256 hex (`crypto/sha256`), and verification compares hashes.
- **Frontend guard**: `web/src/app.tsx`'s `onPageChange` redirects unauthenticated users to the login page (`:296-299`); `web/src/access.ts` maps roles to frontend visibility. **The frontend is only the experience layer** — the backend Authority is the enforcement point.

### Known Weaknesses (labeled honestly)

- **`X-Admin-ID` header fallback**: `internal/service/permission/middleware.go:161-171` — when session context is unavailable, the `X-Admin-ID` header's identity claim is trusted directly. This path targets intranet deployment shapes; **internet-exposed deployments must confirm this middleware is not reachable by any bypass chain**.

## Sensitive Operation Controls

### Risk Grading and Approval Policies

- **Four risk levels**: `internal/dbenum/p1_enums.go:226-229` — `safe | warning | high | danger` (`String()` and parsing at `:233-247`). A function contract's `Risk`/`ApprovalRequired` fields are normalized by `function.ResolvePolicyRisk` (explicit contract declaration > default policy, two-level priority).
- **Default policies (hard-coded fallback)**: `internal/policy/manager.go:109-132` — `high`: forced approval (`single_admin` workflow) + audit; `danger`: forced approval (**`two_person` workflow**) + audit + execution restricted to `super_admin`; when `configs/default-policies.yaml` exists it overrides the hard-coded defaults.
- **Approval interception**: function invocations that hit forced approval become approval tickets instead of executing directly; approval-related routes start at `routes.go:229`.

### Two-Person Rule and Step-Up TOTP

- **Where the two-person rule actually lives**: not in the rbac package but in the approval service — `rejectSelfApproval` at `internal/api/approval/service.go:117-136`: an applicant cannot approve their own request, enforced on both the approve (`:217`) and reject (`:295`) paths; `approval.allowSelfApprove=true` disables it (an explicit choice for single-admin deployments), and both the block and the pass write hash-chain audit events (`approval.self_approval_blocked`) for traceability.
- **Step-up verification for high-risk approvals**: `internal/api/approval/stepup_otp.go:16-26` — approvals of governance risk `high`/`danger` require step-up TOTP: enrolled accounts must supply and pass an OTP, unenrolled accounts are rejected outright (`otp_not_enrolled`); silent passes are forbidden.

### Audit Chain and Honest Boundaries

- **Audit records and chain**: `AuditRecord` at `internal/audit/audit.go:151-166` (including `ChainInfo`); sensitive-field masking `maskSensitiveData` (`:549`) + the unified sensitive-field list (`:564`).
- **Honest boundary 1 (chain hash strength)**: the chain hash `hashBytes` (`:807-815`) is actually DJB2 32-bit (the in-function comment claims "Using SHA256" but the implementation iterates a uint32) — **fine as an integrity hint, insufficient as tamper-proofing strength**; a known hardening item.
- **Honest boundary 2 (verification has no production entry)**: `ValidateChain` (`:667-668`) exists but has no production caller and no exposed endpoint (a repo-wide grep hits only definitions and tests).
- **Honest boundary 3 (approval storage defaults to memory)**: the approval store defaults to `approvals.NewMemStore()` (`internal/svc/service_context.go:342`) — **approval tickets are lost on restart**; a SQL store implementation exists but is not wired at this assembly point.
- **Honest boundary 4 (signer is nil)**: production wiring is `audit.NewAuditService(auditStore, nil)` (`:350`) — the audit outbound signer is nil; records are stored but not signed.

## Bastion/Jump Access (not implemented, skeleton)

The remote session channel of `agents/devops` (devops-agent) is **not implemented**; the current state is a skeleton batch:

- **Evidence**: `agents/devops/session/session.go:64-73` `noopTarget` — a skeleton placeholder Provider whose `Open` always returns `session channel %s not implemented yet (skeleton)` (`:72`); the assembly point `agents/devops/runner/runner.go:92` is exactly `session.NewNoopTarget()`. The whole directory has no `x/crypto/ssh` and no guacamole dependency; `guacdAddr` at `config.go:64-66` is a **reserved field whose comment states the skeleton batch does not wire it**.
- **What has landed** (reusable once a channel exists):
  - **Allowlist**: `Check(host, port, protocol)` at `agents/devops/session/allowlist.go:31` — a host/port/protocol triple not on the allowlist is rejected; config validation requires the allowlist to be non-empty and host/ports set when `devops.enabled=true` (`agents/devops/config/config.go:98-118`).
  - **Broker orchestration**: `OpenSession` at `agents/devops/session/broker.go:63` — allowlist first, audit first (failures leave a record too), then the Provider.
  - **Metadata-level recording**: `SessionMeta` at `agents/devops/session/recorder.go:16-27` — session start/end, protocol, target, and operator written to disk as JSONL (no screen content).
- **Plan pointer**: the real session channel (guacd main channel + local SSH fallback) belongs to the devops real-session entry in `todo.md`; design background in `docs/zh/design/agent-core-design.md`. **Until then, this repo has no usable bastion/jump capability, and any claim of "jumping to intranet hosts through Croupier" is false.**

## Known Doc-vs-Code Discrepancies

| Claim                                            | What the docs say                                                                 | What the code does                                                                                                                                                                               |
| ------------------------------------------------ | --------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `game:{gameId}:{permission}` permission format   | Listed as a permission naming form in `docs/zh/guide/concepts/permissions.md:132` | No implementation of any `game:`-prefixed permission string (a repo-wide grep only hits an unrelated telemetry topic prefix); game isolation actually goes through the `admin_game_scopes` table |
| ABAC / UnifiedPolicyEngine / risk policy engines | The concept docs describe these capabilities                                      | The engine code sits in `internal/security/rbac/` but has no production caller (see "model not wired" above)                                                                                     |
| Where the two-person rule lives                  | Often understood as an RBAC capability                                            | Implemented in the approval service's `rejectSelfApproval` (`internal/api/approval/service.go:117`); the RBAC layer only decides roles/permissions/scopes                                        |
| Bastion remote sessions                          | Some design docs describe a guacd/SSH scheme                                      | Skeleton only (`noopTarget` always rejects); the scheme has not landed                                                                                                                           |

## Related pages
