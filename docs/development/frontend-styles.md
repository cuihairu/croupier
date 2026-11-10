# Frontend style system boundaries

Scope: `web/` (Umi Max + React + antd 6). This page defines which styling
mechanism owns which concern, so new code has a single decision rule instead
of four competing ones. Reviewed 2026-11-10 against the actual tree.

## The four layers and what each owns

| Layer                     | Entry point                                                                                                          | Owns                                                                       | Current usage                                        |
| ------------------------- | -------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------- | ---------------------------------------------------- |
| antd theme token          | `web/src/utils/antdThemeConfig.ts` (runtime truth) + `web/config/config.ts` `antd.theme.token` (build-time fallback) | Look and feel of antd components: color palette, radius, spacing semantics | Active; two presets × light/dark                     |
| antd-style `createStyles` | `web/src/**` component files                                                                                         | Component-local styles for non-antd building blocks                        | Active; 6 production files (+1 test)                 |
| `web/src/global.less`     | Global stylesheet                                                                                                    | App shell (body/topbar/sider/cards), theme CSS variables                   | Active; two presets × light/dark, four variable sets |
| Tailwind CSS 4            | `package.json` devDependency + umi `@tailwindcss/postcss` generator                                                  | Nothing                                                                    | **Installed but not activated**                      |

## antd theme token — the only way to restyle antd components

- The runtime source of truth is `web/src/utils/antdThemeConfig.ts`: four
  `ThemeConfig` objects (blue/inkpink × light/dark). Consumers are `app.tsx`
  (the umi antd plugin's runtime export, so the first frame is correct) and
  `<ThemeSync>` (`web/src/components/ThemeSync/`, pushes on toggle).
- `web/config/config.ts` only pins build-time fallbacks (`borderRadius: 8`,
  `borderRadiusLG: 12`, `colorPrimary: '#1677ff'`). It must not grow a
  parallel palette; full token sets live in `antdThemeConfig.ts`.
- Contrast-driven token choices carry their WCAG rationale inline (for
  example `colorTextSecondary` lifted from antd default `#8c8c8c` at 3.3:1
  to `#595959` at ≥7:1). Keep that pattern: every intentional deviation from
  an antd default gets a comment with the measured ratio.
- Do not override antd component internals from `global.less` or ad-hoc
  CSS. If a component looks wrong, fix the token (component-scoped
  `components:` block in `antdThemeConfig.ts` when it is one component,
  global `token:` when it is systemic).

## antd-style `createStyles` — component-local styles

- Use `createStyles(({ token, css }) => ...)` from `antd-style` for styles
  that belong to one non-antd component (list stand-ins, dropdown wrappers,
  page-local layout).
- Colors must derive from `token` (`token.colorBgContainer`,
  `token.colorText`, ...), not from hard-coded hex values, so preset and
  dark-mode switches propagate automatically. Cross-reference the theme
  variables from `global.less` only when the color genuinely belongs to the
  app shell palette rather than the antd token set.
- Precedent: `web/src/components/SimpleList/index.tsx` (the antd 6 List
  replacement), `web/src/components/HeaderDropdown/`,
  `web/src/components/RightContent/AvatarDropdown.tsx`.

## `global.less` — app shell and theme variables

- Owns: the shell surfaces (body, topbar, sider, cards) and the four CSS
  variable sets (`--app-*`, `--brand-*`) keyed by
  `<html data-preset="blue|inkpink">` × `<html data-theme="light|dark">`.
  `web/src/utils/themeMode` writes the attributes; `<ThemeSync>` keeps antd
  tokens in step.
- Consumers: ~32 `.tsx` files read `var(--app-*)` / `var(--brand-*)`
  inline, concentrated in PageStudio / CompositeEditor canvas code where
  values must react to the preset switch without a React re-render.
- Twin-source contract: `global.less` variable blocks and
  `utils/antdThemeConfig.ts` token blocks describe the same two presets and
  are maintained in parallel on purpose (antd tokens cannot express the
  shell variables; CSS variables cannot feed antd algorithms). Any palette
  change must update both sides plus `docs/.vitepress/theme/custom.css`
  (docs site), in the same PR.
- Do not add per-component overrides here. It is a shell/variable sheet,
  not a component stylesheet.

## Tailwind 4 — installed but not activated

`tailwindcss` 4.x is a devDependency and umi's tailwind generator is wired
(`@tailwindcss/postcss` appears in the umi app data), but no stylesheet
imports `tailwindcss`, no `@tailwind`/`@theme` directive exists, and no
utility classes are used in `web/src`. Effective status: **dormant**.

Decision rule until this changes:

- Do not start using utility classes opportunistically. Activating Tailwind
  is a deliberate architecture decision because its preflight reset and its
  utility vocabulary overlap with both the antd token system and the
  `global.less` variable system.
- If a future need justifies activation, first extend this page with: the
  activation point (which stylesheet imports it), preflight handling (disable
  or scope it so antd baseline styles are untouched), and which of the four
  layers it replaces or complements. Then land the activation in the same PR
  as the documentation update.

## Decision rule for new styles

1. Is it how an antd component should look? → `antdThemeConfig.ts` token
   (or `components:` block for a single component).
2. Is it a style belonging to one custom component? → `createStyles` with
   `token`-derived colors.
3. Is it shell surface or a theme-dependent shared color? → `global.less`
   variable (and check whether the same color needs an antd token twin).
4. Everything else (truly one-off, positional): inline style is tolerated,
   but read colors from `var(--app-*)` / `var(--brand-*)` or `token`, never
   hard-coded hex.

## Guardrails in the tree

- `web/tests/antd6Deprecations.test.ts` bans deprecated antd components
  (for example antd `List`) at the component-import level; replacement
  stand-ins live under `web/src/components/`.
- There is no stylelint. Layer boundaries are enforced by review against
  this page, not by a lint rule. If violations accumulate, the follow-up is
  a stylelint rule set, not silent tolerance.

## Related

- UI audit and contrast baseline: `docs/review/2026-10-31-ui-audit-impeccable.md`
- antd 6 deprecation ledger: `docs/BUGS.md` (BUG-015, BUG-022)
- Versioning/dependency policy that governs the antd/antd-style/tailwind
  package versions: [Version and dependency policy](./versioning.md)
