---
title: Game Ops Console Navigation and Role Workspaces (GUX-002)
---

# Game Ops Console Navigation and Role Workspaces (GUX-002)

Status: design deliverable for todo GUX-002, based on
`docs/design/game-operations-ux-review-2026-11-16.md` (产品与导航 section). This document
regroups **existing** capabilities into three task-oriented workspaces; it deletes no route,
adds no backend permission, and changes no role definition.

## 1. Current inventory

Top-level nav groups today (`web/config/routes.ts`, umi routes = menu tree):

| Group             | Path         | Access key                 | Backing permissions (access.ts)                 |
| ----------------- | ------------ | -------------------------- | ----------------------------------------------- |
| Runtime Console   | `/console`   | `canConsoleRead`           | `console:read`, `pages:read`, `function:invoke` |
| Functions & Pages | `/functions` | `canFunctionsAndPagesRead` | functions/pages/resources/OpenAPI read          |
| Approvals         | `/approvals` | `canAuditRead`             | `audit:read`                                    |
| Analytics         | `/analytics` | `canAnalyticsRead`         | `analytics:read` + feature flag                 |
| Development       | `/dev`       | `canDevRead`               | `dev:read`/`bugs:*` + feature flag              |
| Operations & SRE  | `/ops`       | `canOpsRead`               | `ops:read` + feature flag                       |
| Risk Control      | `/risk`      | `canOpsRead`               | same as Ops                                     |
| Support           | `/support`   | `canSupportRead`           | `support:read` + feature flag                   |
| System Management | `/system`    | `canSystemConfigRead`      | `system:config`, games read                     |
| Access & Accounts | `/admin`     | `canPermissionManage` etc. | roles/users read-manage                         |

The problem the review names: function/page-building entries sit side by side with player
service, risk, and ops entries — every role faces the same ten-group tree, so task context
is lost.

## 2. Role task inventory and permission mapping

Roles come from `configs/roles.json` (17+ seeded roles). Representative permission sets:

| Persona (roles)                                     | Tasks                                                                                                                   | Existing permissions                                                                                                                                  | Nav access today                                                                                                   |
| --------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------ |
| 运营 operations_manager / operations_lead           | player lookup, ban/recharge/mail compensation, order review, activity config, approvals, risk alerts, execution records | `function:invoke`, `functions:read`, `games:read`, `registry:read`, `resources:read`, `assignments:read`, `currency:read`, `item:read`, `wallet:read` | Console ✓; Approvals **✗ (lacks `audit:read`)**; Risk **✗ (lacks `ops:read`)**; Execution Logs ✓ (under Functions) |
| 客服 support_manager                                | player lookup, wallet/currency read, tickets                                                                            | `currency:read`, `wallet:read`, `functions:read`, `games:read`, `registry:read`                                                                       | Console ✓; Support **✗ (lacks `support:read`)**                                                                    |
| 开发接入 developer / frontend_developer / tech_lead | function/resource contracts, OpenAPI, page build & publish, invoke debugging, SDK distribution                          | `functions:read`, `pages:edit/read`, `registry:read`, `resources:read`, `uploads:write`, `function:invoke`                                            | Functions & Pages ✓; Console ✓; Dev **partial (feature flag)**                                                     |
| 平台管理与运维 admin / super_admin / ops_manager    | games/envs, accounts & roles, nodes/servers/agents, system config, extensions, certificates                             | `games:manage`, `system:config`, `ops:*`, `certificates:manage`, `roles:*`/`users:*` (admin)                                                          | System ✓; Ops ✓; Admin ✓                                                                                           |

**Permission gaps are recorded, not fixed here.** The role seeds above lack `audit:read`
(approvals), `ops:read` (risk/nodes), `support:read` (tickets) for the operations/support
personas. Changing `configs/roles.json` is a permission-model decision that stays with the
user; GUX-002 only makes the workspace visibility derive from whatever permissions exist.

## 3. Target navigation structure

Three workspace groups wrap the existing ten top-level groups. **All paths are unchanged** —
only the menu tree regroups, so bookmarks, docs links, and per-route access keys stay valid:

```
运营工作台 Operations         (visible if: canConsoleRead || canAuditRead ||
                              canSupportRead || canAnalyticsRead)
  Runtime Console  /console/...              7 menu nodes + templates
  Approvals        /approvals
  Support          /support
  Risk Control     /risk
  Analytics        /analytics
  Execution Logs   /functions/execution-logs (entry copy; same route)
开发与接入 Dev & Integration  (visible if: canFunctionsAndPagesRead || canDevRead)
  Functions & Pages /functions/...           (minus the execution-logs copy)
  Development       /dev/...
  SDK Distribution  /functions/sdk-distribution (entry copy)
平台管理与运维 Platform       (visible if: canSystemConfigRead || canOpsRead ||
                              canPermissionManage || canExtensionsRead)
  System Management /system/...
  Operations & SRE  /ops/...
  Access & Accounts /admin/...
  Extensions        /system/extensions       (already under system; entry stays)
```

Design decisions:

1. **Regroup, don't re-route.** umi's route tree drives the menu; wrapping existing group
   nodes under three workspace parents changes grouping only. A workspace parent renders if
   any child is visible (umi access semantics support this via derived keys).
2. **No new permissions.** Workspace visibility keys are unions of existing access keys
   computed in `web/src/access.ts`, e.g.
   `canOperationsWorkspace = canConsoleRead || canAuditRead || canSupportRead || canAnalyticsRead`.
   Admin keeps everything (existing `*` permissions already grant all keys).
3. **Cross-workspace entries, not page copies.** Execution Logs and SDK Distribution keep
   their single route and render one page; the second workspace gets a menu entry pointing
   at the same path. No duplicated components, no duplicated permissions.
4. **Scope header stays global.** Game/env selector remains in the top bar for all
   workspaces; high-impact submissions re-display the effective scope (already the pattern;
   reinforced in GUX-012).
5. **Landing rule (feeds GUX-003).** After login, land on the first visible workspace's
   home: 运营 → `/console/home`, 开发 → `/functions/catalog`, 平台 → `/system`.
   When none is visible, fall back to the profile page.
6. **Entry points per workspace (feeds GUX-003/008).** Each workspace home shows that
   persona's tasks (todo/anomaly/recent/common entries). Console home stops carrying
   product-building copy (review observation #1); that content moves to the Dev workspace
   where it is context instead of noise.

## 4. Implementation sketch (not part of this deliverable)

- `web/config/routes.ts`: three parent nodes (`operations`, `dev-integration`,
  `platform`) wrapping existing groups; `redirect` stays on `/`.
- `web/src/access.ts`: three derived `can*Workspace` keys.
- Locale files: workspace titles (运营工作台 / 开发与接入 / 平台管理与运维).
- No server change; no `rbac.json`/`roles.json` change.

## 5. Boundaries

1. This is a design deliverable (GUX-002); implementation lands under GUX-003+ and follows
   the DoD.
2. Role permission gaps (§2) are platform decisions — recorded, not resolved.
3. Risk Control is gated by `ops:read` today, which couples it to Operations & SRE. The
   design keeps that coupling (no new permission); decoupling needs a user-approved
   permission model change.
4. Feature flags (`analytics`/`dev`/`support`/`ops`) continue to apply on top of
   permissions exactly as today.
