---
title: Default Game Operations Menus and Suggested Business Capabilities
---

# Default Game Operations Menus and Suggested Business Capabilities

On 2026-10-17 the user confirmed the Player Workbench, Mail & Compensation, Ban Management, Order Review, and Activity Config as default menu categories with suggested implementations; combined with the existing Announcements and Audit categories, seven top-level categories are provided by default. The menu template has been adjusted; the business subpages in the table still require game capability wiring, page publishing, and mounting — they are not implemented yet.

## 1. Default Categories and Suggested Pages

| Order | Category            | Stable menuKey | Suggested mounted pages                                                              | Business loop requirements                                                                                                        |
| ----- | ------------------- | -------------- | ------------------------------------------------------------------------------------ | --------------------------------------------------------------------------------------------------------------------------------- |
| 10    | Player Workbench    | `player`       | Player/character query, player details, asset view, login history, operation history | Clear account/character/server identity, game data sources and update times; asset writes go through game capabilities            |
| 20    | Mail & Compensation | `mail`         | Mail tasks, compensation tasks, delivery/claim records, failure handling             | Object snapshots, previewed counts and totals, approval, per-object execution credentials, idempotency and failure retry          |
| 30    | Ban Management      | `ban`          | Ban list, ban/unban requests, disposition history                                    | Object/server, reason, duration, approval, game execution results and expiry handling                                             |
| 40    | Order Review        | `payment`      | Order query, credit check, compensation request, reconciliation diffs                | Payment credentials and game credit credentials recorded separately; idempotent compensation, approval, and post-execution review |
| 50    | Activity Config     | `operation`    | Activity list, config versions, publish plans, activation and rollback records       | Config validation, target scope, game time zone, activation time, publish approval and rollback limits                            |
| 60    | Announcements       | `announcement` | Announcement list, publish plans, delivery records                                   | Language/channel/server scope, scheduled activation and withdrawal, actual delivery results                                       |
| 70    | Operation records   | `audit`        | Execution records, approval records, batch task results, change history              | Traceable by operator/target/time/result, chaining operationId, approvals, and game-side credentials                              |

The first five categories are the priority game-operations workflows; the last two keep existing common entries. Complete the end-to-end chain of a single-player query and one compensation operation first, then expand to batch operations.

Suggested subpages are not empty menu nodes created by default, avoiding overly deep navigation or many empty entries; real pages can be combined per game, preferring the two-level navigation of "category → published page → in-page tabs".

## 2. Default Initialization and Customization Rules

- The single template is `configs/default-menus.json`, read by the server from `bootstrapData.baseDir`; it keeps the `MenuSeeder`'s lazy creation on the first menu read per `(gameId, env)` and does not pre-create all environment data when a game is registered.
- The first visit to Menu Management or the Runtime Console for a new scope automatically creates the seven top-level categories, with labels in zh-CN/en-US and icons from the existing allowlist.
- The existing `player/operation/payment/announcement/audit` keys are kept and `mail/ban` are new; existing categories' titles, order, visibility, permissions, and page mounts are never overwritten; the old `operation` is a stable key — existing users are not asked to rename "Operations" to "Activity Config".
- Existing scopes keep the one-time gap-fill plus persistent marker semantics: unprocessed old scopes only get missing keys filled; scopes that already carry the gap-fill marker do not automatically gain new categories on template upgrade; administrators create mail/ban manually per this page — the marker is never deleted to force an upgrade.
- Users can rename, reorder, hide, add subcategories, or delete categories; partially deleted categories do not resurrect on restart; deleting everything and restarting rebuilds per the empty-scope rule; to disable default categories completely, remove the seed file or set it to an empty array.
- Categories and page mounts strictly belong to the current gameId/env; the same menu row cannot be shared across environments; cross-scope config copying requires explicitly creating new menus and page mounts in the target scope.

## 3. Relationship Between Categories, Pages, and Capabilities

```text
Default menu template → current game environment menu categories
Game SDK / external function adapter plugins → function contracts
Function contract + page design → PageSpec publish → mount to menu → usable business pages
```

Default initialization only creates categories: it installs no business extensions, creates no business tables, registers no pseudo functions, auto-approves no pages, and grants no operation permissions. Empty categories reuse the console's empty-category page and page-mount guidance; only published and mounted pages enter their category — no mock data and no action buttons without an execution chain.

Business pages can be carried by official extensions, game SDK functions, or external function adapter plugins, all reusing authorization, approval, tasks, audit, and published snapshots; menu visibility never replaces backend permissions. The authoritative ownership of third-party payment credentials, game assets, and character state is declared by the actual integrator — the existence of default categories never makes a Croupier local copy the game's truth.

**Business template registry (2026-10-27).** `web/src/components/PageRenderer/templates/gameOpsTemplates.tsx` is the single catalog that maps the menu categories to business template pages: template key = console route segment (`/console/templates/<key>`), bilingual title and description, a template preview component (in-memory demo executor), and the PageSpec constant the publish chain consumes. Each entry declares, beyond `menuKey`:

- `categories` — the capability axis that organizes template selection: `player-query` / `resource-management` / `compensation` / `task-monitoring`. An entry may belong to more than one axis; the mail-and-compensation template carries both `compensation` and `task-monitoring`, since `mail_task.retry_failed` and `mail_task.cancel` are the task-monitoring surface.
- `functions` — the game function contracts the template binds, each tagged with a role (`list` / `get` / `create` / `update` / `delete` / `action`). This array is the dependency declaration the template → proposal → publish chain carries, and it stays in one-to-one correspondence with the entry's `pageSpec.bindings.functionId`: `gameOpsTemplateCapabilities.test.ts` asserts set equality, so the registry cannot drift from the spec.
- `capabilitiesNote` — an explicit CRUD-coverage statement for the capability gaps: order review is read-only (orders are written by payment channels, only re-credit is offered), ban management mutates through extend/unban rather than edit/delete, activity config evolves through `publish_version`/`rollback` rather than in-place edit/delete, and mail compensation ends tasks through cancel rather than delete.

The unknown-template fallback list groups entries by the four capability categories and shows the bound functions plus the missing CRUD roles, so a capability gap is visible before a template is adopted.

## 4. Relationship to Platform Fixed Menus

This template serves the dynamic Runtime Console's game business navigation. The four agent page sets, monitoring alerts, resource/function catalogs, Page Studio, menu management, user permissions, and system settings keep using the existing platform navigation; they are not recreated as business categories in every game environment.

Support tickets, data analysis, server management, etc. can be added later or have existing capabilities mounted, as the game actually needs — the first version's default empty categories will not keep expanding.

## 5. Verification and Boundaries

- The config must be parseable by the real `LoadSeedMenus`: the seven keys unique, order increasing, labels BCP47-valid, icons renderable.
- The seeder's regression cases carry over: scope independence, idempotent repeated visits, no overwriting of existing categories, and no resurrection of user-deleted categories.
- PageSpec wire, publishing, and menu-mount contracts are unchanged, and no live data migration is involved; running services need the updated bootstrap file and a restart to load the template.
- Delivering the default categories is not the delivery of the business subpages; the five business loops are tracked separately in todo and accepted against real game integrations.
- The CRUD loop (query → detail → create/edit/delete confirm → refresh) is accepted against real game contracts, not asserted from generated nodes or demo data: the registry declares which CRUD roles each template provides, and a template that lacks a role states why instead of implying coverage.

Related: [Console dynamic menus](../architecture/console-dynamic-menu.md), [Page publish and mount wizard](../guide/concepts/console-navigation.md), [Agent console pages](agent-dashboard-design.md).

For the carrying and acceptance of business data truth, execution contracts, batch compensation, and integration/recovery design, see the [Game console business design backlog](game-operations-design-plan.md); this menu template does not define those shared contracts itself.
