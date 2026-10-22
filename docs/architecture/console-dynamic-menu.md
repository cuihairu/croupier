---
title: Runtime console dynamic menu
icon: menu
order: 9
category:
  - System architecture
tag:
  - Console
  - Dynamic menu
  - menu_items
---

# Runtime console dynamic menu

> **Status**: Current — the runtime console menu is driven **exclusively** by menu management (`menu_items`): menu nodes form the navigation tree, and only pages mounted to a menu and published appear in the console. For the detailed model see the [Dashboard resource/page model](./dashboard-page-model.md); for the operation walkthrough (create menu → publish → mount → it appears in the console) see Runtime console navigation and page mounting. Implementation index: menu generation `internal/api/console/` (`generateMenuFromMenuItems`, `GET /api/v1/console/menu`), menu tree and permission filtering `internal/api/menu/` (`AccessibleTree`), menu model `internal/model/menu.go`, mount field `internal/model/page_spec.go` (`MenuID`), frontend routes `web/config/routes.ts`, side-menu assembly `web/src/utils/consoleMenu.ts` (`buildMenuFromConsoleSpec`), canonical arbitration `resolveConsolePageCanonicalPath`.

## Conclusion

The runtime console's left-hand menu is not a static route configuration, not a direct projection of the function catalog, and not an aggregation of PageSpec `category` values.

The menu has exactly one source:

```text
menu_items (menu tree structure) + page_specs.menu_id (mount mapping) + latest published snapshot (page content)
    -> ConsoleMenuSpec
```

The frontend must not modify `web/src/locales/*/menu.ts` for dynamic categories. Menu titles come from the menu system's `menu_items.labels` (maintained on the menu management page); page titles come from the published PageSpec's `title`.

## Mounting and filtering rules

Whether a page appears in the runtime console is decided jointly by two facts:

1. **Mounted**: the draft table's `page_specs.menu_id` points at a menu in the same scope. The mount relation is read from the draft table — mount changes take effect immediately, no republish needed.
2. **Published**: an active publish snapshot exists for that `pageKey` (`published_page_specs`). Draft-only pages never reach the console.

Assembly rules:

- The menu tree structure (hierarchy, ordering, icons, visibility, permissions) is decided entirely by `menu_items`: `menu.AccessibleTree` prunes by the current user's permissions (invisible entries are cut, menus whose permission the user does not hold are cut, an unreachable parent cuts the whole branch, orphans are promoted to roots).
- Mounted pages mix with submenus as a menu node's children, ordered by `order → localized title → key` (compared among siblings within the group).
- Page content (title/icon/order) comes from the latest publish snapshot; published pages that are not mounted do not appear in console navigation, but a direct URL still renders.
- With no menus at all, `items` is empty and the console home shows a "go to menu management" guided empty state.

## Current state of PageSpec.category

A PageSpec's `category.key` is kept as page-side metadata (workbench grouping display) and **no longer drives the runtime console menu**. Its derivation rules (explicit declaration > `resourceKey` prefix > primary binding `functionId` prefix > `pageKey` prefix; details in ProComponents page generation and runtime) still provide defaults for new page drafts, but only affect workbench-side grouping — nothing to do with console navigation.

## Multiple languages

Dynamic menu display names have two sources of truth:

```json
// menu_items (maintained on the menu management page; the sole source for menu titles)
{
  "menuKey": "support",
  "labels": {
    "zh-CN": "客服",
    "en-US": "Support"
  }
}
```

```json
// PublishedPageSpec (the page side carries only its own title)
{
  "title": {
    "zh-CN": "封禁玩家",
    "en-US": "Ban Player"
  }
}
```

Rules:

- `menu_items.labels` is used for menu titles; when labels is empty the display falls back to `menuKey`.
- The published PageSpec's `title` is used for the page's menu title.
- Static locale data is used only for fixed system menus, such as "Runtime console".
- Dynamic menu items must set `locale: false`.
- Publishing fails when a page's `title` lacks the system default language (menu labels are outside this validation).
- Menu labels are maintained on the menu management page, decoupled from the page publish chain; renaming a menu requires no republish.

## Routing

The runtime console keeps fixed-parameter routes to carry the dynamic menu:

```text
/console/home
/console/:categoryKey
/console/:categoryKey/:pageKey
```

In `/console/:categoryKey` the `categoryKey` segment is the **mounted menu key** (`menuKey`) and shows the pages mounted under that menu (submenu groups included).

`/console/:categoryKey/:pageKey` renders a concrete PageSpec. Canonical arbitration takes the menu tree as the sole source of truth: `resolveConsolePageCanonicalPath` recursively searches the `ConsoleMenuSpec` tree for the mount path of `pageKey`; when the URL differs from the canonical path the frontend redirects to it. A page mounted to no menu (empty-string canonical) does not redirect; the direct URL renders normally. The URL is not a scope: pages, menus, and executions all query against the global `game_id + env` context; the same `pageKey` can exist in different scopes.

## Default menu seed (T-M10)

The menu read path (`GET /menus`, `GET /menus/accessible`, including the `menu.AccessibleTree` shared with console navigation assembly) triggers lazy seeding on a scope's first access, importing the top-level menu skeleton from `default-menus.json` in the bootstrap directory (`bootstrapData.baseDir`, same directory as `admins.json`). Implementation: `internal/svc/menu_seeder.go` (`MenuSeeder.EnsureSeeded`).

Design boundaries (intentional behavior):

- A present, non-empty file enables seeding; a missing file or empty array disables it (a zero-configuration switch); any invalid entry (non-compliant menuKey, all-empty labels) disables the whole batch with an Error log — no half-installed skeleton is left behind.
- **Empty scope → seed everything**: never updates/overwrites existing keys; a user who deletes every menu and restarts gets one reseed. To disable seeding completely, delete the seed file.
- **Non-empty scope → a single "fill missing menuKeys" backfill** (a persistent flag `menuSeedBackfill/<gameId>/<env>` in `platform_settings`, executed once per scope): fixes old scopes that had data before T-M10 shipped and would otherwise lack the skeleton forever (the old behavior "non-empty scope skips entirely"). The flag is written after the first seeding pass (including a full seed into an empty scope), so default categories the user deleted are **not** re-added after restart; a flag-write failure only degrades to one more attempt after restart (key idempotency prevents duplicate creation).
- Seeding is system behavior and writes no user audit (same semantics as AdminManager's default admin).
- The default skeleton (`configs/default-menus.json`, updated 2026-10-17): seven categories — player workbench / mail and compensation / ban management / order review / campaign configuration / announcement management / operation records. It is only a skeleton of mount points — the three-element rule (menu + published + mounted) is unchanged; empty groups render in the console as a single link to `/console/<menuKey>`. For recommended subpages, game capability requirements, and the upgrade boundary for existing scopes, see [Default game-operations menus](https://cuihairu.github.io/croupier/zh/design/default-game-menus).

## Boundaries

Forbidden:

- Maintaining a hardcoded category table.
- Adding static i18n keys for dynamic categories.
- Re-implementing menu inference in frontend pages.
- Generating the runtime console menu directly from the function catalog or PageSpec `category` aggregation.
- Showing unpublished PageSpecs (draft-only) or function registration drafts in the runtime console.
- Putting menu copy into PageSpecs, frontend components, or static dictionaries (menu names belong to the menu system alone).

Allowed:

- Generating a `category.key` default at PageSpec save time per the derivation rules (workbench-side grouping only).
- The Server generating PageSpec proposals from function capability contracts; proposals are not the menu source of truth.
- The user overriding page title, icon, and ordering in the page workbench; maintaining tree structure, labels, icons, ordering, permissions, and visibility in menu management.

## Acceptance rules

- Adding a menu requires no frontend code change.
- After switching languages, menu titles come from `menu_items.labels` and page titles from the published PageSpec `title`.
- Published but unmounted pages do not appear in console navigation; once mounted they appear with no republish.
- A mounted page that is unpublished (unpublish) disappears from the console; unmounting also makes it disappear (the menu group remains as an empty group).
- Menus the user has no permission for (`menu_items.permission` not held, or not visible) do not appear in the console.
- A page without the default-language `title` fails to publish.
- No second menu logic exists between the function catalog, Page Studio drafts, and the runtime console menu.
- After switching the global game/env, the menu shows only the new scope's menu tree and mounted pages.
