---
title: Game Operations Template Demo Data
---

# Game Operations Template Demo Data

## 1. Scope and Boundaries

The five default game-operations templates (Player Workbench, Mail & Compensation, Ban Management, Order Review, and Activity Config) share one browser-memory dataset for template previews, interaction walkthroughs, and acceptance demos.

- Isolated demo environment: `gameId=croupier-demo`, `environment=local-demo`.
- Demo operator identity: `accountId=gm-demo`, with logical permissions `console.read`, `gameops.demo.read`, and `gameops.demo.write`.
- This identity only describes the intended operator and permissions in demo pages. It **does not create a real login account, JWT, backend scope, or permission record**. Template routes still use the current console session and existing authorization.
- Demo data does not write real `page_specs`, `gameId/env` menu rows, or game business tables. Reloading the page or clicking Reset restores the initial seed.

## 2. Start and Access

```bash
cd web
pnpm start
```

Open these routes with an existing console session:

| Template            | Route                                  | Demo capabilities                                                                       |
| ------------------- | -------------------------------------- | --------------------------------------------------------------------------------------- |
| Player Workbench    | `/console/templates/player-manage`     | Player query/detail, create/edit/delete, ban, recharge, mail, batch ban/unban           |
| Mail & Compensation | `/console/templates/mail-compensation` | Mail/compensation tasks, approval state, failed retry, per-player and total amounts     |
| Ban Management      | `/console/templates/ban-management`    | Ban records, create, unban, extend, batch unban                                         |
| Order Review        | `/console/templates/order-review`      | Payment and game-credit evidence, reconciliation diffs, idempotent missing-credit retry |
| Activity Config     | `/console/templates/activity-config`   | Activity versions, publish, online/offline, rollback                                    |

The page banner shows the demo `gameId/env/accountId` and default menu category, and provides a Reset Demo Data action.

## 3. Dataset and Edge Cases

- Three servers and 26 players, including a nickname longer than 40 characters.
- Thirty orders covering `credited/pending/missing/not_needed` game-credit states and `match/missing_credit/missing_order` reconciliation results.
- Twelve ban records covering `active/expired/lifted`.
- Mail/compensation tasks cover pending approval, partial failure, executing, completed, and related states; activities cover multiple versions, time zones, and business states.
- Players, orders, bans, tasks, activities, and operation records reference each other through `playerId/serverId/orderNo/approvalId/operationId`, enabling cross-page selection, filtering, pagination, and write-then-read checks.

## 4. Reset and Tests

- UI entry: **Reset Demo Data** in the template-page banner.
- Code entry: `resetGameOpsDemoRuntime()`, which replaces every collection in the shared store in place.
- Regression command: `cd web && npx jest src/components/PageRenderer/templates --runInBand`.
- Real game integration replaces the demo executors entirely. Demo acceptance never substitutes for a real-environment acceptance record.
