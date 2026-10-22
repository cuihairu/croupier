---
title: Runtime Console Navigation and Page Mounting
icon: menu
order: 5
category:
  - Core Concepts
tag:
  - Runtime Console
  - Menu Management
  - Page Mounting
---

# Runtime Console Navigation and Page Mounting

The left-side navigation of the Runtime Console (`/console`) is not decided by route configuration or the function catalog; it has exactly one source: **the menu tree maintained in Menu Management + published pages explicitly mounted to menus**. For a page to appear in the Runtime Console, three conditions must hold at once:

```text
Menu node exists (created in Menu Management) + page is published + page is mounted to that menu
```

All three are required — **publishing alone never puts a page into the console**. For the detailed architecture rules see Runtime Console Dynamic Menu.

## Full Workflow

### Step 1: Create menus in Menu Management

Entry point: sidebar **Functions & Pages → Menu Management** (`/functions/menus`).

> **Default menu categories**: the first time menus are accessed for a brand-new `(gameId, env)` scope (from the Menu Management page or the Runtime Console), the system automatically creates seven categories: Player Workbench / Mail & Compensation / Ban Management / Order Review / Activity Config / Announcement Management / Operation Records (deploy directory `configs/default-menus.json`). Business pages inside these categories only become usable after they are wired to game capabilities, published, and mounted — see [Default Menus and Suggested Capabilities](../../design/default-game-menus.md). Existing menus are not renamed, reordered, or remounted; an old scope that has never been initialized only gets its missing categories filled once; scopes that already carry the initialization marker are not force-reseeded on template upgrades; to disable default categories entirely, remove the file or set it to an empty array.

Click **New Menu** and fill in:

| Field                     | Description                                                                                                                                                                                            |
| ------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| Menu key (`menuKey`)      | Starts with a letter; only letters/digits/hyphens/underscores, length 1–64. **It is the console route segment**: after a page is mounted to the `player` menu, its path is `/console/player/<pageKey>` |
| Menu name (`labels`)      | Localized titles (`zh-CN` / `en-US`), shown in the console sidebar                                                                                                                                     |
| Icon                      | Optional; the sidebar menu icon                                                                                                                                                                        |
| Permission (`permission`) | Optional. When set, only users holding this permission can see the menu (including the entire subtree)                                                                                                 |
| Visible                   | When off, this menu (including its subtree) is excluded from console navigation                                                                                                                        |
| Sort order                | Order within its group; can also be adjusted by dragging in the tree                                                                                                                                   |

Menus support arbitrary-depth nesting: every node in the tree has an **Add submenu** button. Deleting a menu cascades to all of its submenus and automatically unmounts pages mounted below it (the pages themselves and their published state are unaffected).

### Step 2: Publish the page

A page must have an **active published snapshot** to appear in the console. Draft-only pages (saved in the editor but never published) do not appear.

Publish via either path:

- **Proposal inbox**: run `accept-and-publish` on the generated proposal (the publish confirmation dialog also lets you pick the mount menu — see Step 3);
- **Page Studio / version management**: publish an existing draft.

### Step 3: Mount the page to a menu

Admin UI entry points (recommended; requires the `pages:edit` or `admin:all` permission), by scenario:

**Scenario A: one-step mount while publishing a default page (proposal inbox)**

1. Go to **Functions & Pages → Page Studio** (`/functions/pages`), top proposal inbox, **Ready to publish** queue;
2. Click the **Publish** button on the target proposal row;
3. In the confirmation dialog, pick the target menu in the **Mount to menu** dropdown (optional; if skipped, the page is only published and does not yet enter console navigation);
4. Click **Confirm** — publish and mount complete in one action; console navigation updates immediately.

**Scenario B: change the mount of an already published/draft page (Page Studio)**

1. Go to **Functions & Pages → Page Studio** (`/functions/pages`); the **Advanced page management** panel at the bottom is expanded by default — find the row of the target page;
2. Click the row's **Mount menu** icon button (the hierarchy icon next to pencil/preview);
3. Pick the target menu in the dialog's menu tree (same structure as Menu Management, submenus at any depth), then click **Confirm**. **For an unmounted page the first menu is selected by default** — confirming directly completes the mount; clearing the selection and confirming unmounts the page.

Proposal rows of materialized pages can also reach Scenario B via **More (⋯) → Mount menu** (it locates the page and opens the mount dialog automatically).

**Scenario C: inline mount inside the page edit dialog (save/publish on one screen)**

After **Edit** on a Page Studio row opens the "Edit page" dialog, the "Page info" card on the left provides a **Mount menu** selector:

- It shows the current mount when opened; **unchanged values are not submitted with save** (the mount API is not called on every save). **For an unmounted page the first menu is preselected and treated as modified** — saving submits the mount along with it (avoiding the misleading case of showing a default that never takes effect);
- Both "Save draft only" and "Save and publish" can carry the mount in one click (a mount failure does not roll back the save/publish; a hint suggests retrying the mount later);
- Clearing the selection = unmount (takes effect on save); clearing the prefilled default on an unmounted page likewise means no mount.

> Historical note: before 2026-09 this selector sat in the dialog footer, pushed out of the viewport by the full-height edit body and effectively invisible; it has moved into the body, and the old "category key" input was removed at the same time (category now only passes through as a protocol field with legacy data and has no edit entry anymore).

Dialog behavior details:

- The input echoes the currently mounted menu; **clearing it and confirming unmounts the page** (the page disappears from the console, but its published state is preserved and the direct URL stays reachable);
- If the page is still a draft, the dialog notes "the mount will be saved, but the page only appears in Runtime Console navigation after publishing" — a draft can be mounted first and enters the console automatically once published;
- After a successful save **console navigation updates immediately**, with no republish or redeploy needed.

The same operation is available via API (for scripts/batch scenarios):

```bash
TOKEN=<login token>
curl -X PUT "https://<server>/api/v1/pages/<pageKey>/menu" \
  -H "Authorization: Bearer ***" \
  -H "X-Game-ID: <gameId>" -H "X-Env: <env>" \
  -H "Content-Type: application/json" \
  -d '{"menuId": <menu id>}'
```

- `menuId` is the numeric id returned when the menu was created in Step 1 (also listed by `GET /api/v1/menus`);
- The menu must belong to the same `(gameId, env)` scope as the page; cross-scope mounts return "menu not found";
- **Mount changes take effect immediately**; republishing the page is not required.

The API form of unmounting (equivalent to clearing the selection in the UI and confirming):

```bash
curl -X PUT "https://<server>/api/v1/pages/<pageKey>/menu" \
  -H "Authorization: Bearer ***" \
  -H "X-Game-ID: <gameId>" -H "X-Env: <env>" \
  -H "Content-Type: application/json" \
  -d '{"menuId": null}'
```

### Step 4: Verify in the Runtime Console

Refresh the Runtime Console and you will see: menu group nodes (including nested submenus) + pages mounted within each group, interleaved by `sort order → title → key`. A page entry's title comes from the `title` of the page's latest published snapshot.

The "Runtime Console" subtree in the sidebar and the in-console navigation render from the same menu data, with identical permission filtering rules (menu `permission`, visibility, pruning when a parent is unreachable).

## Behavior Quick Reference

| Scenario                                         | Console behavior                                                                                                                                            |
| ------------------------------------------------ | ----------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Page published and mounted                       | Appears under its mounted menu                                                                                                                              |
| Page published, not mounted                      | **Does not appear**; the direct URL `/console/<any segment>/<pageKey>` still renders (no redirect)                                                          |
| Page mounted but not published (draft-only)      | Does not appear                                                                                                                                             |
| Empty menu (no pages mounted)                    | The menu group node stays and renders as a single link to `/console/<menuKey>` (no submenu to expand); it becomes a menu group again once pages are mounted |
| Menu carries `permission`, user lacks it         | The whole subtree (including mounted pages) is invisible                                                                                                    |
| Menu invisible (`isVisible=false`)               | The whole subtree is excluded from navigation                                                                                                               |
| URL menu segment does not match the actual mount | Automatic redirect to the canonical path of the mounted menu                                                                                                |
| Mounted menu deleted                             | Subtree cascade-deleted, page mounts are automatically removed; pages and published state are unaffected                                                    |

## Permission Requirements

| Operation                | Required permission                                                          |
| ------------------------ | ---------------------------------------------------------------------------- |
| Create/edit/delete menus | `menu:create` or `admin:all`                                                 |
| Mount/unmount            | `pages:edit` or `admin:all`                                                  |
| View console menus       | `console:read` (the menu tree is filtered by the current user's permissions) |

## FAQ

**Q: The page published successfully — why is it not in the Runtime Console?**
Publishing does not automatically add it to the menu. Check whether Step 3's mount was performed, whether the menu is visible, and whether the current user holds the menu `permission`.

**Q: Mounted to the wrong menu — how do I fix it?**
Reopen the row's **Mount menu** dialog, pick the new menu and confirm (or PUT the target `menuId` again). It takes effect immediately; no unmount-then-remount needed.

**Q: Can users access an unmounted page?**
Yes. The direct URL `/console/<any menu segment>/<pageKey>` renders normally (no redirect); it just does not appear in navigation. The console homepage's "published pages" count also includes unmounted pages. Combine with access control if you need to restrict access.

**Q: Where did the auto-created menus from older versions go?**
That behavior was removed: since the version where menu_items became the sole driver, console navigation only honors the "menu + mount + published" triple.

**Q: I deleted all menus, but after a restart a set of default menus appeared again. Why?**
Default menu seeding performs a full initialization for empty scopes: after deleting every menu and restarting the service, the first access to that scope re-imports the categories. To disable this completely, remove `configs/default-menus.json` from the deploy directory or set it to an empty array. A scope that already wrote its initialization marker will not restore individually deleted categories; an uninitialized old scope still gets one missing-key reseed, and existing category content is never overwritten.
