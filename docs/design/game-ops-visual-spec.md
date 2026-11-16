---
title: Game Ops Console Visual and Interaction Spec (GUX-016)
---

# Game Ops Console Visual and Interaction Spec (GUX-016)

Status: design deliverable for todo GUX-016 (P2, design), based on
`docs/design/game-operations-ux-review-2026-11-16.md`. It codifies the visual and
interaction rules the operator console already follows (or must follow going
forward) across titles, spacing, tables, forms, drawers, semantic colors, and
interaction states, covering dark/light themes, Chinese/English copy, and
keyboard focus. It changes no runtime behavior by itself; enforcement gaps are
listed in §12, not silently assumed.

Normative language: **must** = required for new/changed UI; **should** = do it
unless there is a recorded reason; **may** = allowed.

## 1. Scope and sources

- Applies to the web operator console (`web/`, Umi + antd 6 + pro-components).
- Sources of truth this spec codifies:
  - Theme architecture: `web/src/global.less`, `web/src/utils/antdThemeConfig.ts`,
    `web/src/utils/themeMode.ts`, `web/config/config.ts` (theme token block).
  - Page rhythm: `DASHBOARD_PAGE_TOKENS` (`web/src/components/StandardPage/index.tsx`).
  - Delivered behavior baselines: GUX-005~007 shared semantics, GUX-008~013
    benchmark flow, GUX-014 empty states, GUX-015 copy convergence (evidence under
    `docs/review/evidence/2026-11-17/`).
  - Form contract baselines: `docs/design/form-component-reuse.md`.

## 2. Theme architecture

Two theme presets × light/dark/system:

- Presets: `blue` (factory default, antd 6 seed `#1677ff`) and `inkpink`
  (brand magenta, seed `#93394d`). Preference persisted in localStorage
  (`croupier-theme-preset`); mode (`light`/`dark`/`system`) in `croupier-theme`.
- Effective state is written to `<html data-preset="blue|inkpink">` ×
  `<html data-theme="light|dark">` by `utils/themeMode.ts`; antd algorithm/token
  follow via `<ThemeSync>` and the `antd` runtime export in `app.tsx`.

Rules:

1. Components **must** consume antd tokens (`theme.useToken()`) or the `--brand-*`
   / `--app-*` CSS variables. New raw hex literals in component code are review
   failures.
2. `global.less` (CSS variables) and `utils/antdThemeConfig.ts` (antd token
   seeds) are **co-maintained single sources** — a color change touches both, in
   the same change.
3. antd 6 re-derives palette colors from hex seeds per algorithm. Contrast
   **must** be computed on the **rendered** value, not the seed: e.g. the seed
   `#4096ff` renders as `#3983dc` under the dark algorithm. Dark-mode contrast
   checks **must** be done separately per preset (same seed, two dark
   backgrounds: one passes at 4.78:1 while failing at 4.24:1 in the other).

## 3. Titles

| Level               | Token/style                  | Use                            |
| ------------------- | ---------------------------- | ------------------------------ |
| Page title          | `PageContainer title`        | One per page                   |
| Page subtitle       | `PageContainer subTitle`     | One-line positioning statement |
| Card/section head   | `Typography.Title level={4}` | `margin: 0` inside cards       |
| Sub-block head      | `Typography.Text strong`     | Checklist/help blocks, drawers |
| Auxiliary text      | `Typography.Text secondary`  | Descriptions under heads       |
| Muted labels (12px) | `mutedTextSize` token        | Card footnotes, helper rows    |

Internal identifiers (menu/category keys, internal table names) **must not**
appear as visible titles or badges (GUX-015); raw-key fallback is allowed only
as a last-resort identity fallback when no localized title exists.

## 4. Spacing and surfaces

`DASHBOARD_PAGE_TOKENS` is the spacing vocabulary; new pages **must** import it
instead of inventing values:

| Token                | Value                 | Use                    |
| -------------------- | --------------------- | ---------------------- |
| `cardPadding`        | 20                    | Standard card body     |
| `compactCardPadding` | 12                    | Nested/small cards     |
| `heroCardPadding`    | 24                    | Hero/summary cards     |
| `sectionGap`         | 16                    | Between sections/cards |
| `itemGap`            | 10                    | Inside a section       |
| `mutedTextSize`      | 12                    | Auxiliary text         |
| `softSurface`        | `rgba(0,0,0,0.03)`    | Chip/stat containers   |
| `filterSurface`      | `rgba(0,0,0,0.025)`   | Filter bars            |
| `infoGradient`       | brand-tinted gradient | Summary card wash      |

Radii come from the antd config only: `borderRadius: 8` (controls) and
`borderRadiusLG: 12` (surfaces). Do not introduce additional radius values.

Black-alpha overlays (`softSurface`/`filterSurface`) are dark-mode safe by
construction; any new overlay **must** be alpha-based, not a fixed light hex.

## 5. Tables (ProTable)

Delivered baseline (GUX-008~009, generated resource pages; hand-written pages
follow the same rules):

- Query toolbar: `{ labelWidth: 'auto', defaultCollapsed: true }`; search items
  derive from `valueType`/`valueEnum` (`search: filterable ? undefined : false`);
  high-frequency columns (e.g. player id/nickname) come first.
- Row actions are link-state buttons. Toolbar-level one-shot actions (e.g. bulk
  operations) may keep `type="primary"`. More than 4 entries in an operation
  column collapse into a "more" dropdown; destructive entries keep their
  danger-confirmed semantics inside the dropdown.
- Numeric and time columns right-align, derived mechanically from `dataType`.
- Cell rendering goes through the shared semantic layer (`renderFieldSemantic`,
  §8) and follows the UI locale.
- Empty/loading/error follow the GUX-014 state model: distinguish no-data /
  not-mounted / no-permission / load-failure; a merged empty card replaces
  per-row repetition; next steps are role-aware.

## 6. Forms

- Generated forms render via `SchemaFormRenderer` (RJSF antd wrapper) against
  `FormPresentationSpec`; uiSchema is derived in memory, never persisted.
- Container reuse targets are `ModalForm` / `DrawerForm` / `StepsForm` with
  contract baselines in
  `web/src/components/FormContract/__tests__/formContainers.contract.test.tsx`
  (render / data / validation contracts). A ProComponents upgrade must keep
  those baselines green.
- Required marks and validation messages use antd defaults; blocking validation
  must prevent `onFinish` (locked by the contract tests).
- Submit buttons: one primary submit per form; cancel is default-state.
  Destructive confirmations use `modal.confirm` with `danger` ok-button props.

## 7. Drawers and modals

- Detail drawers: right-side `Drawer`, `size={640}` in the resource renderer
  (600 in hand-written pages — new pages **should** use 640). Detail opens with
  the row identity anchor (`detail-identity`) at top when the spec declares
  `identityKey`.
- Row-action forms with more than a few fields use `DrawerForm`; short forms
  use `ModalForm`.
- Closing behavior stays on antd defaults (Esc/mask). Unsaved-data guards are a
  known follow-up, not silently assumed.

## 8. Semantic colors and status

- Status text/tag vocabulary comes **only** from declared enum options; their
  colors may come from the contract (`opt.color`) and default to `default`.
- An undeclared enum value renders as 未知（{value}）with a neutral tag — never
  guessed semantics, never color-only signaling. The same not-color-only rule
  applies to booleans (是/否) and invalid datetimes (无效时间（{value}）).
- Status colors use antd semantic tokens (`success`/`processing`/`error`/
  `default` tag names, `colorSuccess` etc. tokens). Brand accents use
  `--brand-1/2/3` only.
- Money/count/identity semantics (unit following UI locale, no invented units,
  copyable identities) follow GUX-005~007; lists and details must show the same
  value with the same meaning (shared semantic layer).

## 9. Interaction states

The full state set every interactive surface **must** define or inherit:

1. Hover / active — antd primitives provide them; custom ones must define.
2. Keyboard focus — antd 6 primitives ship a visible focus ring. Any custom
   interactive element **must** either be a real `button`/`a` (inheriting the
   ring) or add an explicit `:focus-visible` outline; a click handler on a
   non-focusable `div` is a review failure.
3. Disabled — antd disabled styles; also disable during in-flight submit
   (`loading` on submit buttons).
4. Loading — table `loading`, `Spin` for page-level, `Skeleton` for detail
   drawers.
5. Empty / error — GUX-014 model; error surfaces show the extracted message
   (`extractErrorMessage`) with a retry where a retry is meaningful.
6. Partial success — receipts show only real returned fields (GUX-013
   whitelist); unknown stays unknown, no invented rollbacks.

## 10. Dark/light and Chinese/English coverage

- Visual changes **must** be verified in both light and dark, per preset when
  brand colors are involved (§2 rule 3).
- All user-visible copy goes through intl keys (`useIntl`/`FormattedMessage`)
  or `localizedText` for LocalizedText contract values; component
  `defaultMessage`s are zh and must stay in sync with locale files (zh-CN/en-US
  key parity is tested by `web/src/locales/__tests__/featureDictParity.test.ts`).
- Copy convergence follows GUX-015: guidance is on-demand (collapsed help);
  template/demo strings and internal jargon do not appear in default views.

## 11. Review checklist

- [ ] No raw hex / invented spacing in the diff (tokens or CSS vars only).
- [ ] Status meaning never color-only; undeclared values render 未知（原值）.
- [ ] Custom interactive elements are focusable with a visible focus ring.
- [ ] Dark + light verified (per preset when brand colors are touched).
- [ ] zh-CN/en-US key parity; no hardcoded locale strings in new copy.
- [ ] Empty/loading/error/partial-success states covered by tests.
- [ ] Table/form/drawer conventions match §5–7 (link actions, collapsed search,
      640 drawers, contract-tested form containers).

## 12. Known boundaries (honest)

1. This is a design deliverable: no automated guard enforces §11 yet (the
   dashboard_vnext_guard does not scan for raw hex or focus styles). Codifying a
   guard is a follow-up, not a claim.
2. Legacy deviations exist: e.g. some analytics/detail pages still carry hero
   gradients or hand-rolled spacing from before the tokens; they are not
   retro-fitted by this spec. New work must comply; touching a legacy surface
   means bringing that surface over.
3. `docs/design/index.md` registration for this document is deferred: the file
   is another in-flight edit in a parallel session; registering here would
   sweep foreign lines into this commit.
4. The unsaved-data guard for drawers/modals (§7) and a full keyboard-only
   audit of legacy pages are not done; antd primitives are covered by the
   framework, custom surfaces were not exhaustively audited.
