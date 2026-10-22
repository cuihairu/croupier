# UI Bug Verification and Regression Ledger

> Scope: issues found reproducing/verifying frontend interaction paths on the local stack (`croupier-server` + `pnpm dev`), with root causes, fixes, and regression tests.
>
> Acceptance bar: **every BUG must come with a test that genuinely distinguishes fixed from regressed** — "red before the fix, green after". Assertions that merely check "it rendered", or mocks weakened until they can never fail, do not count as regression tests (this document hit that trap repeatedly; each affected entry says so in its "regression test" section).
>
> Related: `todo.md` (T1–T13 complete), `CLAUDE.md`, `web/package.json` (antd 6).

## Verification Methods

| Method                       | Location                                           | Purpose                                                                                                                                                                                                                                              |
| ---------------------------- | -------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Console audit script         | `web/scripts/console-audit.mjs`                    | Logs into the real stack, walks all **52** routes, collects console warnings/errors/pageerrors, and emits a JSON report. antd's runtime deprecation warnings, React duplicate keys, and missing locale keys are only catchable this way in dev mode. |
| Deprecated-prop static guard | `web/tests/antd6Deprecations.test.ts`              | Derives the deprecation list from the **runtime warning tables** in `node_modules/antd`, scans `src/**/*.tsx`, and fails immediately with file:line on any newly introduced deprecated prop.                                                         |
| Unit tests                   | `pnpm --dir web test` (jest, 251 suites)           | Component behavior regression.                                                                                                                                                                                                                       |
| Go tests                     | `make test` (`go test -short ./...`, 169 packages) | Backend regression.                                                                                                                                                                                                                                  |

How to run the audit script (requires the local stack to be up):

```bash
cd web && node --import tsx scripts/console-audit.mjs http://127.0.0.1:8000
# 报告写入 web/test-results/console-audit.json
```

**Final verdict**: across the **52** routes' console output, antd deprecation warnings **0**, other warnings/errors/pageerrors **0** (before the fixes for BUG-005/007/008/010/011 they were 19, 4, 2, 4, and 14 respectively).

The only residue is 3 × `Failed to load resource: 503` on `/analytics/warehouse` — the backend returns `{"error":"service_unavailable","message":"分析仓库未启用"}`, i.e. the local ClickHouse analytics warehouse is not enabled (`services/api/analytics.ts:484` already notes "503 when the warehouse is not enabled"). This is an **expected backend state**, not a UI defect; the browser logs this resource error for any 503 and the frontend cannot suppress it.

---

## BUG-001 `TestNewTCPListener_NilConfigDefaults` Grabs the Production Default Port; the Local Stack Fails on Startup

**Severity**: high (blocks "go test all green")

**Symptom**

Running `make test` while the local croupier stack is up always fails in `internal/server`:

```
--- FAIL: TestNewTCPListener_NilConfigDefaults (0.00s)
    listen tcp :19090: bind: address already in use
```

**Root cause**

The test calls `NewTCPListener(nil, nil, nil, nil)` directly, and a `nil` config falls back to the production default address `:19090` (the `control.addr` default in `configs/*.yaml` is also `:19090`). So the test really binds the **business port**. Any developer running the local stack, or any parallel test holding that port, gets hit — an environment coupling wholly unrelated to the logic under test. The other cases in the same package's `tcp_listener_test.go` had long switched to the ephemeral port `:0`; only this one slipped through.

**Fix**

Split "config defaulting" out of "listener construction" so it is testable without occupying a port:

- `internal/server/tcp_listener.go`: add the `defaultControlAddress` constant and `defaultTCPListenerConfig(config)` (a non-nil config is returned as-is, never overriding the caller's explicit values); `NewTCPListener` now calls it. Production behavior unchanged.
- `internal/server/control_handler_test.go`: split into three tests —
  1. `TestNewTCPListener_NilConfigDefaults`: a pure function asserting the default address and `Insecure`;
  2. `TestNewTCPListener_ConfigDefaults_PreservesExplicitValues`: asserts identity return and no flipping of explicit TLS switches;
  3. `TestNewTCPListener_NilDepsUseDefaults`: uses `127.0.0.1:0` to verify nil sessionStore/registry/logger defaulting, and asserts the actual port **does not equal** the production default port.

**Regression test**: the three cases in the same file. Before the fix, case 1 always goes red with the local stack running.

---

## BUG-002 The Bootstrap Admin's nickname/email/phone Are Silently Dropped

**Severity**: medium (data loss, no crash)

**Symptom**

`GET /api/v1/profile` returns `nickname:""`, `email:""`, `phone:""` for `admin`, even though `configs/users.json` explicitly writes `nickname: "系统管理员"` and `email: "admin@croupier.local"`.

**Root cause (two defects compounding)**

1. `AdminManager.loadDefaultAdmins` iterates in `admins.json` → `users.json` order but **returns** as soon as any file has `loadedCount > 0`. In `configs/admins.json`, `admin` has identity but no profile fields, so it "hits" first and `users.json` is **never read**; the same-name dedup silently drops the profile fields.
2. `seedBootstrapAdmins` only syncs `status` for existing DB rows and never backfills empty profile fields — even with correct config, existing rows stay empty forever.

**Fix**

- `loadDefaultAdmins`: no early return; instead "the first file contributing identity wins; later files only enrich profiles (nickname/email/phone fill blanks only, never overwrite) and add no new accounts", preserving the "admins.json first, users.json fallback" semantics.
- `seedBootstrapAdmins`: for existing rows, backfill only empty profile fields, **never overwriting user-set values**; a second seed is idempotent.

**Regression test**: four cases in `internal/svc/admin_manager_test.go` (`..._EnrichesProfileFromLaterConfig` / `..._EnrichDoesNotOverwriteExistingProfile` / `..._LaterFileDoesNotAddIdentity` / `..._SecondFileLoadsWhenFirstEmpty`) + `internal/svc/bootstrap_admin_seed_test.go`'s `TestSeedBootstrapAdminsBackfillsEmptyProfileFields` (including a second-seed idempotency assertion).

---

## BUG-003 The LB Monitoring "Ownership Ratio" Dashboard Never Shows a Value

**Severity**: medium (silent feature failure)

**Symptom**

On `/ops/lb`, in the "ownership vs LB reconciliation (zombie probes)" card, the ownership-ratio gauge is blank; the reading does not reflect the real ratio of `agent node count / LB backend count`.

**Root cause (reproduced locally; three pieces of evidence)**

1. `@ant-design/plots@2.6.8`'s gauge adaptor rewrites the input into an object — running the adaptor, `data: 0.5` always becomes **`{"value": 0.5}`** (`es/core/plots/gauge/adaptor.js`: `params.options.data = { value: data }`).
2. `@antv/g2@5.4.8`'s Gauge mark accepts only a `number` or `{target, total, percent, name, thresholds}` — its `GaugeData` type has **no `value` key**.
3. g2's `getGaugeData` returns non-numbers as-is, and `dataTransform` destructures only `{name, target, total, percent, thresholds}`. So `target/total/percent` are all `undefined`, `_target = percent || target = undefined`, `_total = percent ? 1 : total = undefined`, and the y channel degrades to `undefined` / `NaN`.

I.e. **the percentage is silently discarded**; neither the needle nor the reading moves.

> An early comment said "renders then throws TypeError". Per the data path above, the value is **silently lost to undefined/NaN** rather than necessarily throwing; this document defers to the measured facts.

**Fix**

- `web/src/pages/Ops/LBMonitor/index.tsx`: render the same ratio with antd `Progress type="dashboard"` (no dependency on the canvas chart chain).
- Extract the pure function `ownershipRatioPercent(nodes, backends)`: no agents → 0 (not NaN); no backends → denominator 1 to avoid division by zero; over 100% is clamped.

**Regression test**: `web/src/pages/Ops/LBMonitor/__tests__/reconciliation.test.tsx`, 7 cases — 5 pure-function boundaries (including the full 0–5 × 0–5 combination asserting an integer in 0–100, never NaN/Infinity), and 2 component renders (reading `归属 2 / LB 后端 4` and `50%`; `0%` with no agents).

---

## BUG-004 The Menu-Tree Order Label Bypasses intl values; the Real intl Throws MISSING_VALUE

**Severity**: low (the UI is right, the console errs) — but it **masked a wrong implementation approach**

**Symptom**

Every node with an order value on the menu management page spams intl parse errors (`MISSING_VALUE`) in the console, while the visible text is correct.

**Root cause**

The component was written as:

```ts
fmt("pages.menuManagement.page.order", "排序 {order}").replace(
  "{order}",
  String(n),
);
```

`{order}` is an ICU placeholder. The real `react-intl` routes through `onError` (MISSING_VALUE) and returns the un-interpolated string when `defaultMessage` contains a placeholder but no `values` — the `.replace` only rescues the **visible text**; the error remains. The correct approach is passing the value through `values`.

**Why it went unnoticed so long**: `MenuTree.test.tsx` mocked `@umijs/max` itself:

```ts
formatMessage: ({ defaultMessage }) => defaultMessage; // 丢弃 values
```

That mock swallowed the "missing values" error along with everything else, effectively patching the component and letting the wrong pattern survive.

**Fix**

- `MenuTree.tsx`: add `fmtOrder(order)` using `intl.formatMessage(desc, { order })`.
- `MenuTree.test.tsx`: replace the mock with a **faithful implementation** (interpolates per `values`) that records each call's `id/defaultMessage/values`.

**Regression test** (same file): `排序标签的 {order} 经 intl values 传入，不靠组件侧 replace 兜底` — asserts every call containing `{order}` carries a numeric `order`, and the interpolated text is `排序 3`.

**Mutation check**: reverting `MenuTree.tsx` to the `.replace` pattern —

- the old case `已发布/草稿页面…排序 3` **still passes** (proving it has no discriminating power);
- the new case fails (`values` is `undefined`).

This contrast is the origin of this document's acceptance bar that a test "must be able to discriminate".

---

## BUG-005 142 antd 6 Deprecated Props: TypeScript Says Nothing, the Pages Say Nothing

**Severity**: medium (console pollution, masking real problems, and a mass breakage waiting at v7)

**Symptom**

Every page load in dev mode spams `Warning: [antd: X] \`p\` is deprecated. Please use \`q\` instead.`. The `Drawer height`one comes from the **global layout**`GameSelector` — one spam per page navigation (the audit script measured 18 on a single page).

**Root cause**

The repo is on `antd@^6.4.3` (6.6.0 installed), but the code still holds plenty of antd 5 patterns. antd 6 only **warns and does not change behavior** for these, so `tsc` stays green, pages look fine, and deprecated usage can lurk indefinitely. The real risk is the mass breakage when v7 removes them, plus real problems buried under the warning flood.

The list was not hand-copied from docs but reverse-engineered from the `devUseWarning('<Component>')` **runtime warning tables** in `node_modules/antd/es/**/*.js` — antd actually uses three spellings, and the extractor must recognize all three (or it leaks):

1. Lookup table + `forEach`: `[['message', 'title']].forEach(...)` (most components)
2. Direct call: `warning.deprecated(!tip, 'tip', 'description')` (`Spin`)
3. Object-literal mapping: `const deprecatedProps = { dropdownRender: 'popupRender' }` (`Select`)

> Only **prop-name**-level deprecations are collected. Value-level suggestions like `size="default"` are not JSX props and are excluded.

**Fix (142 → 0)**

| Prop                                                  | Count | Treatment                                                                                                                                                                       |
| ----------------------------------------------------- | ----- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `Alert message` → `title`                             | 100   | Renamed directly                                                                                                                                                                |
| `Drawer width` / `height` → `size`                    | 20    | Renamed directly (normalized through `drawerSize` to consistent values)                                                                                                         |
| `Space direction` → `orientation`                     | 12    | Renamed directly                                                                                                                                                                |
| `Input` / `InputNumber addonBefore` → `Space.Compact` | 6     | **Structural rewrite**: the prefix moves out of the input to become an adjacent sibling inside `Space.Compact` (static forms: PathControls ×3, BindingModal ×2, SourceModal ×1) |
| `Input addonBefore` → `Input.prefix`                  | 3     | **Structural rewrite**, but via `prefix` rather than `Space.Compact` — see BUG-009 (hot-path editors: ActionEditor ×1, ConstantFieldsEditor ×2)                                 |
| `Statistic valueStyle` → `styles.content`             | 1     | A nested path; moved into the `styles` `content` slot (`content: {...}`)                                                                                                        |
| `Spin tip` → `description`                            | 5     | Renamed directly                                                                                                                                                                |

The rewrite ran as a codemod keyed on JSX opening tags, skipping `>` inside `{}` expressions and string literals (otherwise `<Alert title={a > b} />` would be misjudged as tag end mid-expression); it only renamed props, never touching children/object literals/other components with the same prop names.

**Regression test**: `web/tests/antd6Deprecations.test.ts`, 3 cases:

1. The extractor itself is alive (> 50 table entries + sampling hits on `Alert.message` / `Drawer.width` / `Drawer.height` / `Space.direction` / `Statistic.valueStyle` / `Card.bordered` / `Divider.type` / `Spin.tip` / `Select.dropdownRender`) — prevents the guard from becoming an always-green false negative once the list rots;
2. No deprecated JSX prop exists under `src`; on failure it reports each `文件:行号 <X p=…> → 改用 q`;
3. Literal backstop: `addonBefore=` / `addonAfter=` / `valueStyle=` must never appear.

**Mutation check**: reverting `Welcome.tsx`'s `variant="borderless"` to `bordered={false}` makes case 2 fail immediately, naming `src/pages/Welcome.tsx:126 <Card bordered=…> → 改用 variant`.

---

## BUG-006 The Composite Page Editor Menu Lacks a Locale Key

**Severity**: low

**Symptom**: `menu.FunctionsAndPages.CompositeEditor` is registered in neither the `zh-CN` nor the `en-US` `menu.ts`, so the menu item falls back to the raw key.

**Fix**: `web/src/locales/{zh-CN,en-US}/menu.ts` add `组合页编辑器` / `Composite Page Editor`.

---

## BUG-007 An Empty Avatar String Renders `<img src="">`, Making the Browser Re-Request the Current Page

**Severity**: medium (a real extra network request + a React warning)

**Symptom**

The console on `/admin/account/center` (the redirect target of `/account/center`) reports:

```
An empty string ("") was passed to the src attribute. This may cause the
browser to download the whole page again over the network.
```

**Root cause**

`GET /api/v1/profile` returns `"avatar":""` when unset (verified locally). `Profile/index.tsx` and `AvatarModal.tsx` pass it straight into `<Avatar src={...}>`. An empty `src` is interpreted by the browser as "re-request the current page's URL". Note the same code's `icon={!profile?.avatar ? <UserOutlined /> : undefined}` **already** handles emptiness — only `src` slipped.

**Fix**

Extract `normalizeAvatarSrc()` (`web/src/pages/Profile/shared.ts`): non-string / empty / whitespace-only → `undefined`; a valid URL is trimmed and returned. Both call sites use it.

**Regression test**: `web/src/pages/Profile/__tests__/InfoTab.regression.test.tsx`, 5 cases — `normalizeAvatarSrc`'s empty/whitespace/null/undefined/non-string (guarding against backend type drift)/valid-URL branches, plus one source-level lock: both `src={...}` sites must go through the normalizer and never pass through raw (mounting the whole Profile pulls up 7 tabs and a pile of APIs — disproportionate cost, hence the source assertion, with the trade-off documented in the test comment).

---

## BUG-008 The Profile Form Instance Is Unconnected; antd Warns on Every Render

**Severity**: low

**Symptom**: the console on `/admin/account/center` reports

```
Warning: Instance created by `useForm` is not connected to any Form element.
Forget to pass `form` prop?
```

**Root cause**

The form instance is created by the main `Profile` page's `useForm` and reused (`useProfileData` backfills via `form.setFieldsValue` once data arrives, re-backfills on cancel, and calls `form.submit()` on save), but `InfoTab` renders `<Form form={form}>` **only while `editing` is true**. Outside editing, the instance sits "unconnected".

The warning is not raised at mount time but from `FormHook.warningUnhooked` — the **form instance methods** (`setFieldsValue` / `getFieldsValue` / `submit` …) check `formHooked` inside `setTimeout`. So the trigger is "unconnected + a method called"; mounting without calling never warns.

**Fix**: in `InfoTab`, `<Form>` becomes **permanently mounted**, hidden in non-editing states by a `hidden` container (`display:none` is out of layout, visually identical to before).

**Regression test**: 4 cases in the same file. Key points:

- One **control** case: construct "an orphan instance with no `<Form>` mounted" and call `setFieldsValue`, asserting the warning does fire — proving the warning channel works, otherwise the positive assertion is a false negative;
- The regression lock uses a **structural invariant** (in non-editing state the `<form>` element must be in the DOM with a `div[hidden]` ancestor), not "assert no warning".

**Why not "assert no warning"**: rc-util's `warning` dedups by message — the same text fires once per module lifetime. Once the control case has triggered it, any later "no warning" assertion is trivially true. This was **learned by measurement**: reverting `InfoTab` to conditional rendering kept "no warning" green while the structural assertion failed. Now structural; under mutation checking 2 cases flip red.

---

## BUG-009 Migrating `addonBefore` per antd's Official Advice to `Space.Compact` Slowed the Editor 10×

**Severity**: medium (functionality unchanged, but test/interaction performance pushed to timeouts)

**How it was found**

While fixing BUG-005, per antd's official migration advice, the label prefixes in `ActionEditor` / `ConstantFieldsEditor` were wrapped in `Space.Compact`. `tsc` stayed green and the related unit tests passed — until the **full** jest run:

```
Test Suites: 1 failed, 253 passed, 254 total
Tests:       1 failed, 3251 passed, 3252 total
```

And the failing case changed each run, with per-suite times rising from <1s to 5–21s — the classic **timeout** signature, not a logic error.

**Root cause**

antd's `Space.Compact` wraps **every child** in a `CompactItem` context provider:

```js
const CompactItem = (props) => {
  const { children, ...others } = props;
  return (
    <SpaceCompactItemContext.Provider value={useMemo(() => others, [others])}>
      {children}
    </SpaceCompactItemContext.Provider>
  );
};
```

`others` is a fresh object per render, so the `useMemo` deps change every render → **the context value is a new reference every render** → every consuming `Input` (via `useCompactItemContext`) is dragged into re-renders. Row-action editors are instantiated in bulk inside tables/previews, multiplying the cost.

> Note: the `Space.Compact` wrappers around `Select` in these two files are **pre-existing code** and measurably do not trigger the problem (Select's re-render cost is lower than Input's). This entry concerns only the newly added label-prefix wrapper.

**Fix**

The label prefixes switch to `Input`'s own `prefix` prop (not deprecated in antd 6, carries a label equally well), introducing no extra component and therefore none of the context churn above.

- `web/src/pages/PageStudio/CompositeEditor/ActionEditor.tsx`
- `web/src/pages/PageStudio/CompositeEditor/ConstantFieldsEditor.tsx`

The static forms (`PathControls` / `BindingModal` / `SourceModal`) keep the official `Space.Compact` advice — they are not hot paths and have no such problem.

**Regression test**: in `web/tests/antd6Deprecations.test.ts`, `热路径编辑器的标签前缀用 Input.prefix，未被 Space.Compact 包裹` —

- extracts the source fragment around "the element carrying the label", asserting it contains `<Input` + `prefix={<span` and **does not** contain `<Space.Compact` (comments stripped first, otherwise the explanatory comments themselves would match the substring);
- `ConstantFieldsEditor` additionally backtracks from the `titleAddon` / `varNameAddon` anchors to their respective `<Input`s and asserts each.

**Mutation check**: reverting `ActionEditor` to the `Space.Compact` wrapper fails the case immediately.

**Effect**: full jest went from 207s back to ~145s, with two consecutive full green runs (3252/3252).

> One counter-intuitive measurement worth recording: in a targeted benchmark of 40 `ActionEditor`s in one file, `prefix` (838ms mount / 1115ms for 5 re-renders) and `Space.Compact` (863ms / 1070ms) showed **essentially no difference**. In other words, a targeted benchmark cannot detect this problem — only the real load of a full concurrent run exposes it. **Do not expect a single benchmark case to catch a performance regression.**

---

## BUG-010 `app.cancel` Has No Registered Locale Key; the Composite Page Editor Errors on Every Open

**Severity**: low (copy falls back to `defaultMessage`, but the console goes red every time)

**Symptom**: on `/functions/pages/composite-editor`, the console reports 4 in a row:

```
[React Intl] Missing message: "app.cancel" for locale: "zh-CN", using default message as fallback.
```

**Root cause**

Both `DanglingRefsModal.tsx` and `BindingDrawer.tsx` call `intl.formatMessage({ id: 'app.cancel', defaultMessage: '取消' })`, but `app.cancel` was never registered in `locales/{zh-CN,en-US}/app.ts` — that file previously carried only the `app.layout.*` and `app.request.*` groups, with no common-actions group.

`formatMessage` with a `defaultMessage` falls back and renders fine, but the missing key is reported through `onError` — two more console lines per modal open.

**Fix**: add the common-actions key group to both `zh-CN/app.ts` and `en-US/app.ts` (`app.cancel` / `app.confirm` / `app.save` / `app.close`), synchronized in both languages.

> This issue is outside BUG-005's static scan — deprecation warnings and missing locales are two different problem classes. It was found by the **console audit** (runtime evidence), which is exactly why this round keeps both the static guard and the runtime audit paths.

---

## BUG-011 Both Audit-Log Pages Use `hash` as rowKey, Making Every Key on the Page Identical

**Severity**: medium (React duplicate keys; rows may render duplicated/missing)

**Symptom**: `/admin/login-logs` spams a dozen lines per page:

```
Encountered two children with the same key, `%s`. Keys should be unique so that
components maintain their identity across updates.
```

**Root cause**

`LoginLogs.tsx` and `OperationLogs.tsx` both write `rowKey={(r) => r.hash}`, and `/api/v1/audit` in the current deployment **does not backfill `hash`** — measured: the endpoint returned 19 records with the `hash` field **missing in all**:

```
items 19 / unique hash 1 / empty hash count 19
```

`normalizeAuditEvent` further defaults the missing `hash` to `''`, so `r.hash` is always `''` and every row's key is identical. The server does have a unique `id` (shaped `audit_1790375253236648061_bb169c724c610ee5`, all 19 distinct), but normalization **never carried it over**.

**Fix**

- `services/api/audit.ts`: add `id` to `AuditEvent` and pass `item.id` through `normalizeAuditEvent`.
- Add `auditRowKey(event, index?)`: `id` → `hash` → `time|actor|kind|target` → row index, guaranteeing uniqueness within a page.
- Both pages stamp `__rowKey` onto rows at the **slice/fetch stage**, and the Table uses the string `rowKey="__rowKey"`.

**Why not fall back to index inside the rowKey callback**: antd 6 has deprecated that parameter (`index` parameter of `rowKey` function is deprecated). The first implementation wrote `rowKey={(r, i) => auditRowKey(r, i)}` and the runtime audit immediately caught 2 new warnings — a live example of "audit after fixing" adding a layer the static scan lacks.

**Regression tests**

- `web/src/services/api/auditRowKey.test.ts` (7 cases): normalization keeps `id`; each of `auditRowKey`'s four fallback levels hits; **replaying the real response** (19 records, `hash` all empty) yields all-distinct page keys. Mutation check (reverting the implementation to `return event.hash` and dropping `id`) turns 6 cases red.
- `web/src/pages/Admin/LoginLogs.test.tsx`'s `行 key 在 hash/id 全缺时仍互不相同`: page-level, directly asserting the DOM's `data-row-key` values are distinct and non-empty. Mutation check (reverting to `rowKey={(r) => r.hash}`) turns it red.

> This deliberately does **not** use "assert the console has no duplicate-key warning": that React warning is subject to global dedup — whichever case runs first consumes it for the rest, making the assertion trivially true (the same trap as BUG-008). Instead it asserts the tested property — key uniqueness on the DOM — directly.

---

## BUG-012 Four Independent Avatar Defects in the Account Center (Mutual Data Wipes / Dead Links / 404 / a Header That Never Renders)

**Severity**: high (one of them loses data: any profile save wipes the other fields)

**Symptom**: user-reported "the account center avatar is wrong". Triaged into 4 independent defects:

1. **Mutual data wipes**: `ProfileUpdateRequest`'s four fields are bare strings; Go binds missing fields to empty strings, and the service writes all four columns unconditionally — "change only the nickname" wipes avatar/email/phone. Existing tests only asserted `resp.Ok`, never field survival, so nobody noticed.
2. **Dead links**: the upload endpoint's returned `url` is signed and expiring (S3/OSS/COS default 15min TTL); storing it verbatim means storing a time-delayed dead link.
3. **The file driver's 404**: `fileStore.SignedURL` returns the relative `/uploads/<key>` path, but no handler in the repo serves it.
4. **The header always shows a placeholder**: `toCurrentUser` drops the profile's avatar and `getInitialState` never backfills — the header avatar has never worked.

**Fix** (commit `e69db6f`):

- The four fields become `*string`: nil = not carried = keep the old value, an explicit empty string = clear; the two semantics are separated. An all-empty request short-circuits without touching the DB.
- The DB stores only the **bare object key**; a valid URL is computed at read time. `objstore.NormalizeAvatarKey` normalizes uniformly (bare key / file relative path / absolute URL), rejecting anything outside the `avatars/` directory and any path traversal; legacy absolute URLs are returned as-is at read time (concatenating them into `/uploads/https://…` would turn "maybe expired" into "certainly 404"), and they self-heal on the user's next save.
- The file driver mounts a static directory exposing only the `avatars/` subtree (never leaking general uploads such as defect attachments), auth-free (`<img src>` carries no Authorization header), with the dev proxy gaining `/uploads/` forwarding.
- `CurrentUser` passes avatar/nickname through for backfill; a shared `UserAvatar` component is added, with placeholders unified to name initials (Chinese first character / English initials / username / ? in a four-level fallback), and `src` and the placeholder judged from the same normalization result — eliminating the "whitespace avatar renders as an empty circle" fork.

**Regression tests**: `objstore/avatar_test.go` (four normalized input classes, five rejected illegal inputs, traversal protection, re-signing on every read, legacy URL passthrough, static-directory checks); `api/profile/avatar_test.go` (single-field updates preserve the other fields, empty requests write nothing, explicit empty strings clear, the avatar lands as a bare key for four input shapes, non-conventional directories rejected); 11 `UserAvatar` component cases (initial rules / rendering / standalone placeholder). Verified end-to-end on an independent local instance (28780/29090): upload → sign → save → fetch without an Authorization header → 200 with identical bytes.

## BUG-013 MFA Enrollment Is Unusable + No Recovery-Code Fallback

**Severity**: high (losing the authenticator app locks you out of the account)

**Symptom**: on `/admin/account/center?tab=security`, enabling two-step verification offers only the "manually transcribe the base32 secret" path — Microsoft/Google Authenticator provide no "paste an otpauth link" entry, so enrollment is effectively unusable; and once enabled there is **no recovery path at all** — lose the phone, lose the account.

**Root cause**:

1. The `otpauth://` URI was hand-concatenated; the account name was not percent-encoded (usernames with `@`/`/`/non-ASCII break authenticator parsing), and `algorithm/digits/period` were missing, so some apps parsed with their own defaults;
2. The frontend rendered no QR code and had no recovery-code mechanism;
3. `VerifyTOTP` depended on `time.Now()`, so RFC 6238 Appendix B's fixed test vectors could not be reproduced and implementation correctness had no spec-level lock.

**Fix**:

- `otp.OtpauthURI`: spec-compliant label percent-encoding + an explicit issuer parameter + full algorithm/digits/period (Google Authenticator cross-checks the label prefix against the issuer parameter and silently drops mismatches).
- `VerifyTOTPAt`/`CodeAt` expose injectable time points; `DecodeSecret` normalizes the secret shapes exported by different apps (case/spacing/padding).
- Backup recovery codes: enrollment issues 10 codes at once (a 30-character alphabet minus confusable characters, ≈49.5 bits of entropy), stored as SHA-256, plaintext returned exactly once in the confirm response; consumed one-per-code exactly once (`UPDATE ... WHERE used_at IS NULL` naturally guards concurrent reuse); the login page's OTP input accepts recovery codes (`verifySecondFactor` tries either); disabling MFA clears the recovery codes.
- Frontend: bind by QR scan (manual entry kept as fallback), one-time recovery-code display + download, a remaining-count hint (≤2 warns); the login page's totpCode input maxLength 6→12 with a hint that recovery codes work.
- Migration 0032 adds the `admin_otp_recovery_codes` table (idempotent HasTable/CreateTable), `MinimumRequiredVersion` 31→32, with the migrate_test probe synced.

**Regression tests**: `security/otp/rfc6238_test.go` (RFC 6238 Appendix B's official vectors + skew behavior); `otpauth_test.go` (URI normalization/escaping/params); `api/auth/mfa_recovery_test.go` 14 cases (issuance / single consumption / whitespace-input normalization / disable clears / rebind replaces / remaining counts / recovery-code login in three states); `MfaSettings.test.tsx` 8 cases (QR rendering / one-time recovery-code display / download / disable confirmation / the external-account note).

Live-chain verification (after restarting the local stack, full curl flow): setup → confirm (a real TOTP code) → 10 codes returned once → login without a code 401 `mfa_required` → recovery-code login 200 → **replaying the same code 401** → TOTP login 200 → disable (code+password) zeroes the recovery codes in the DB and normal login resumes.

---

## BUG-014 The Health-Score Decay Test Depends on the Wall Clock and Fails Randomly on Busy Machines

**Severity**: medium (an unstable gate: "all green" flips randomly with machine load)

**Symptom**

`internal/platform/dispatch`'s `TestHealthTracker_ScoreDecay` fails intermittently:

    expected score to decay, still at 100.000000

The same code passes repeatedly on an idle machine and fails on a busy one (this occurrence: 45 C++ compile processes on the same box, load average 144). Unrelated to the logic under test.

**Root cause**

The test assumes "the 100ms decay ticker will fire at least once within 150ms":

```go
tracker.Start()
// ...
time.Sleep(150 * time.Millisecond)
if state.HealthScore() >= 100.0 { t.Errorf("expected score to decay, ...") }
```

`scoreDecayLoop` is a background goroutine's `time.NewTicker`. Under scheduler pressure, `ticker.C` may not be consumed once within 150ms, leaving the score untouched → the test fails. Same family as BUG-001: **the test treats environment timing as a premise**.

**Fix**

Poll until "decayed" with a generous ceiling (a 3s deadline), and switch the assertions to invariants independent of the actual tick count:

- the score did drop (otherwise report a timeout with the actual wait, for triage);
- the score lies on the discrete set `100 × 0.5^n` (n≥1) — locking "decays by `ScoreDecayRate`" without depending on how many ticks fired.

**Verification**: after the fix, 5 consecutive green runs; then 8 CPU spinners were started artificially to create contention, and 3 consecutive runs still passed (the original implementation failed under that condition).

---

## BUG-015 antd 6 Deprecates the `List` Component Entirely — the Prop-Level Guard's Blind Spot

**Severity**: medium (console warnings + a whole-component breakage at the v7 upgrade)

**Symptom**: during the interactive walkthrough of `/admin/account/messages` (the message center), the console reports:

```
Warning: [antd: List] The List component is deprecated. And will be removed in next major version.
```

Yet BUG-005's static guard and the earlier 52-route runtime audit both showed **0 warnings** — because the message center, ticket details, and the account center's tabs were not in the original audit route list (dynamic/detail pages were never included).

**Root cause**: `List` is a **component-level** deprecation in antd 6 (`warning(false, 'deprecated', ...)`, attached to no prop). BUG-005's guard extractor reverse-engineers **prop** mappings from `deprecated('p', 'q')` tables and structurally cannot see component-level deprecations — the two guards (prop-level/component-level) are different dimensions and both are required.

**Fix**:

- All 6 real call sites repo-wide (`Profile/GamesTab` / `NotificationsTab` / `AuditList` / `PermissionsTab`, `Assignments/HistoryModal`, `Support/Tickets/Detail`; note `PageRenderer` / `ResourcePageEditor`'s `<ListTab` was a same-prefix false match) use only the read-only `List / List.Item / List.Item.Meta` surface, uniformly migrated to the in-house stand-in `src/components/SimpleList`: generic + `renderItem` + `Item(actions/style/onClick)` + `Item.Meta(title/description)` + `loading/pagination/locale.emptyText`, with createStyles matching antd List's default visual contract (separators/titles/secondary descriptions/footer pagination).
- `web/tests/antd6Deprecations.test.ts` gains a **component-level** guard: first confirm the installed antd still deprecates List (after an upgrade it automatically prompts a re-review), then assert no `<List` JSX under `src` (the negative assertion does not hit `<ListTab`) and no `import { List } from 'antd'`.

**Regression tests**: `SimpleList.test.tsx` 8 cases (item/Meta rendering, emptyText, actions, whole-row onClick, per-row rowKey invocation, the Spin wrapper, pagination callbacks, the generic shape) + the guard's new cases; all existing tests of migrated files green (Profile/Tickets/Permissions 64, Functions DetailSections 19). Runtime re-verification: after re-running the walkthrough, List warnings are 0.

---

## BUG-016 The Security Center's "Login Notification" Is a Fake Switch (Fake State + Fake Interaction)

**Severity**: medium (users are misled into believing they receive SMS login alerts)

**Symptom**: on `/admin/account/center?tab=security`, the "login notification" row renders a green "enabled" tag whenever the user has filled in a phone number; the helper copy says "for login alerts and SMS verification" — while the repo has **no SMS provider at all** (`ChannelSMS` is an enum constant nobody uses: no sender, no config, no UI entry). Users believe they are receiving SMS.

**Root cause**: the frontend infers channel availability from "the user filled in a phone number" (`hasPhone`). A phone number is only a receive target and says nothing about whether a provider is integrated. This is the recurring **fake state** problem of this document, in its notification-domain variant.

**Fix** (a new `GET /api/v1/profile/notification-channels`; availability is decided only by the backend):

- `approvals/sms.go`: pin down the SMS integration point — the `SMSProvider` interface + `ErrSMSNotConfigured`; the default `unconfiguredSMSProvider` makes sending **fail explicitly** rather than silently succeed when unintegrated; `SMSRegistry.Status()` reports available only when "a provider is registered and credentials are complete".
- `api/profile/notification_channels.go`: the three channels' source of truth — in_app (in-site messages work with zero config; platform settings can turn them off), email (when SMTP is unconfigured, EmailSender is a no-op and must not claim availability), sms (registry only). `available` and `userEnabled` are independent dimensions; unavailability always carries a `reason`.
- The frontend `SecurityTab`: state is fully backend-driven. Unreachable → a warning tag + a reason tooltip + the switch disabled. **Close-out correction 1**: the switch renders read-only as `userEnabled` — it currently comes from platform-level settings with no user-side write endpoint, and "clickable but with no effect" fake interaction is exactly what this entry exists to kill. **Close-out correction 2**: the status tag gets a third branch — when a channel is reachable but the platform has it off, display "disabled"; otherwise "SMTP configured but the platform turned off email" would render as a green "enabled" tag next to an unchecked switch, self-contradictory (the original implementation missed this branch).
- The section header copy moves to the new key `profile.channel.section` (the old key `profile.login.notification`'s existing value is "登录通知"; defaultMessage never takes effect, so before the close-out the header actually displayed the old copy).

**Regression tests**: `notification_channels_test.go` 10 cases (unintegrated / incomplete credentials / SMTP present-absent / admin-disabled / missing account / Registry boundaries — including the core invariant "filling in a phone number must not make SMS available"); `SecurityTab.channels.test.tsx` 14 cases (including the three close-out additions "reachable but off → disabled", "in_app turned off → disabled + reason", "the switch is read-only").

---

## BUG-017 The Feishu Signing Secret Missed secretKeys; Masked Round-Trips Overwrite the Real Value

**Severity**: medium (config corruption: notifications fail silently — a corruption, not a leak)

**Symptom** (found in code review, never hit on the network): `notification.feishuSecret` is in `ValidKeys` but not in `secretKeys`.

**Root cause and impact**: the `GET /api/v1/site/notification` snapshot has a dedicated `feishuSecretMasked` field for it (masking always worked), but `PutKey`'s "masked round-trip protection" branch consults only `IsSecretKey` — when the admin console saves the snapshot back verbatim, `****+last4` is treated as the real value and overwrites the store; from then on Feishu notifications **fail silently** with a fake secret. The SMTP password / DingTalk / Webhook secrets of the same kind were all registered; only Feishu slipped through.

**Fix**: `secretKeys` gains `KeyNotifyFeishuSecret`; the original comment claimed "plaintext echo would leak", which on verification (the snapshot mask predates this fix; the raw `FeishuSecret` value only reaches the sender with no HTTP exposure) was a misjudgment — replaced with the real defect above.

**Regression test**: `layered_notify_test.go`'s `TestKeyClassificationHelpers` gains an `IsSecretKey(KeyNotifyFeishuSecret)` assertion (false before the fix).

---

## BUG-018 Game Access Permissions Are Always Empty: Admin Sees a Blank

**Severity**: medium (impossible to tell "all permissions" from "no permissions")

**Symptom**

On `/admin/account/center?tab=games`: the "permissions" area on every one of admin's game cards is permanently empty.

**Root cause**

`internal/api/profile/service.go`'s `GetUserGames` hardcodes, while building `ProfileGame`:

```go
Permissions: []string{},
```

Not "couldn't query" — a **literal**, empty for every user and every game. The frontend `GamesTab` renders `{(game.permissions || []).map(...)}` directly, so an empty array means an empty area. Admin and read-only accounts see exactly the same thing; "has all permissions" is indistinguishable from "has none".

**Fix**: report truthfully and explain the model.

- `GetUserGames` now computes from the permissions the user actually holds (`perGamePermissions` in `internal/api/profile/permissions.go`): holding the wildcard → `["*"]` (rendered by the frontend as a green "all permissions"); otherwise the real held permission ids.
- The DTO gains `accessLevel` (`full` / `scoped` / `none`) and `permissionScope` (fixed `"role"`). Empty permissions thereby become an **explainable state** instead of a blank.
- The frontend `GamesTab` renders three states by level (all permissions / per-item tags / no explicit permissions + an explanation), and states that "permissions are granted by role, not split per game" — avoiding the impression that each game can be authorized individually.

**Model clarification**: RBAC hangs off roles and is **not split along the game dimension**; what is split per game/environment is "visible games and visible environments" (`admin_game_env_scopes`, which `GetUserGames` already uses to filter). Therefore the same user's permission set is inherently identical across all visible games; per-game permissions have no "only on this game can I do X" semantics.

**Regression tests**: `internal/api/profile/permissions_test.go` (`GetUserGames_AdminSeesFullAccessPerGame` asserts `["*"]`+`full`; `GetUserGames_NonAdminWithoutPermissionsIsNone` asserts `none`)

- `web/src/pages/Profile/__tests__/GamesTab.permissions.test.tsx` 5 cases (including the compatibility path "when the backend is un-upgraded and accessLevel is missing, still never a blank").

---

## BUG-019 The Permissions Overview Is Fabricated Data: resource Is Always "role" and Role Names Are Mixed into Permission IDs

**Severity**: high (the "permissions overview" the frontend renders from this has no relation to actual grants)

**Symptom**

In the profile's permissions page/modal, the "held permissions" list is wrong: every resource name is `role`, and each entry's action is a role name.

**Root cause**

Both constructions in `GetPermissions` are fabricated:

```go
permissions = append(permissions, ProfilePermission{
    Resource: "role",              // 字面量
    Actions:  []string{role.Name}, // 把角色名当操作
})
```

And a subtler one:

```go
for _, role := range roles {
    appendPermission(role)                    // ← 角色名进了 permissionIDs
    if role == "admin" || role == "super_admin" {
        appendPermission("admin")             // ← 同样是角色名，不是权限
        appendPermission("*")
    }
}
```

So admin's `permissionIDs` is `["*", "admin", "user:read", ...]`. The frontend diffs `permissionIDs` against the permission catalog to render "granted / not granted"; `"admin"` and `"super_admin"` are not found in `configs/permissions.json`, becoming two permanently "not granted" fake permissions. Meanwhile `permissionIDs` **does not carry** per-resource full-grant information (only the union), so no tree can be built.

**Fix**

- Role names leave `permissionIDs` entirely and stay only in `roles`; `permissions[]` becomes real resource → action groupings (`resolvePermissions`).
- The resource axis comes from **the permission id's prefix** (`pages:write` → resource=pages), not the `permissions` table's `resource` column — that column stores the module (e.g. `dashboard`), which would collapse the 38-entry catalog into 7 values. That was the direct cause of "the tree only has a few nodes".
- The wildcard test tightens to **both dimensions wildcarded** for `fullAccess` (`resource==* && action==*`, i.e. `*` / `admin:all`). `user:*` means "all actions on the user resource only", with no access to other resources; counting it as fullAccess would make the frontend render every entry as "granted" — fake state again. Single-segment ids (no colon) are treated as "the whole string is the resource", never split on `.` by guesswork.
- Wildcard ids generate no fake `*` resource node.
- The split semantics unify into `rbac.SplitLogicalPermission` (exported in this round); `splitLogicalPermission` and the profile side no longer keep two copies.
- Adds `rolePermissions` (per-role grant detail): with only the union, "which role granted this action" is unanswerable and the tree degenerates into a flat list.

**Regression tests**: `permissions_test.go` 21 cases, including `KeepsRoleNamesOut` (role names stay out of permissionIDs), `ResourceFromIDPrefixNotModule` (locking "no collapse into modules"), `PartialWildcardIsNotFullAccess` (`user:*` ≠ all permissions), `SingleSegmentIDBecomesItsOwnResource` (no splitting on `.`), `RoleGrantsIncludeEmptyRoles`. Also rewrites 4 existing cases that had **locked in the old wrong behavior** (`TestService_GetPermissions_WithAdminRole` etc. asserted `"admin" ∈ permissionIDs`) — those assertions were themselves BUG-019 fossilized.

---

## BUG-020 The Permissions Overview Is a Flat List: No Resource/Action Hierarchy and No Granted/Not-Granted Distinction

**Severity**: medium (cannot tell what is held from what is not)

**Symptom**

"Held permissions" is one flat list: no resource→action hierarchy, and no "granted / not granted" distinction whatsoever.

**Root cause**

`PermissionsTab` flattens the backend's `groups` via `SimpleList`, and that data (1) is fabricated (BUG-019) and (2) contains only **granted** items. So the grayed "not granted" actions have no data source at all — the page reads as "everything is granted".

**Fix**: add `permissionTree.ts` + `PermissionTreeView.tsx`, rendering role → resource → action in three layers.

- **The resource/action candidate set comes from the full permission catalog** (`GET /api/v1/permissions`), not the granted ids; if the catalog is missing, it degrades to "granted only" — still never crashing, never fabricating.
- Each item is judged independently per current role: green (`#52c41a`) + `CheckOutlined` = granted; gray (`#bfbfbf`) + `CloseOutlined` = not granted. A second channel beyond color, so it never relies on color vision alone.
- All three layers expand/collapse; "expand/collapse all" cycles between "fully expanded ⇄ role level only".
- With an account-level wildcard, the whole tree renders all-green with an explicit note that "gray only means not explicitly granted by a concrete role; actually usable" — otherwise users read gray as "unusable".
- Roles with zero permissions still appear on the tree, labeled "no explicit permissions".
- The frontend adds one more backstop: `getMyPermissions` normalization strips role names mistakenly stuffed into `permissionIDs`.

**Regression tests**: `PermissionTree.test.tsx` 23 cases (17 pure logic + 6 rendering), where `目录提供未授权项` locks the core gap and `资源级通配 user:* 覆盖该资源全部操作，但不影响别的资源` locks the wildcard granularity. Mutation check: reverting the resource axis to modules turns 7 cases red.

---

## BUG-021 The "Notifications" Tab Carries an Admin Broadcast Entry, and Announcements Have No Entry At All

**Severity**: high (an unauthorized entry) + medium (a missing feature)

**Symptom**

`/admin/account/center?tab=notifications` has a primary "send message" button at the top, opening `BroadcastModal` (bulk in-site messaging). The profile center is a page **every user has** — ordinary users and admins see the same UI.

**Root cause**

- `NotificationsTab` takes `isAdminUser` / `onSendClick` props and renders the "send message" button + mounts `BroadcastModal`. The profile center's semantics are "my profile / my messages"; broadcast is an admin action, misplaced here.
- The real announcements API `/api/v1/admin/announcements` (`internal/api/announcement`, List/Create/Update/Delete + the user-side `/active`) has existed all along with tests, but had **zero frontend references** — announcements had no entry whatsoever. The 38 permissions in `configs/permissions.json` include nothing announcement-related either.

**Fix**

- ① The received-notifications list: keeps unread marks / mark-all-read / details, and adds the previously missing **mark-one-read** (previously one could only open details or mark all at once). Unread rows get a background tint + a left vertical bar, not distinguished by bolding alone.
- ② Notification channel preferences: in-site/email/SMS, with state **entirely from the backend** `GET /api/v1/profile/notification-channels` (BUG-016); unintegrated channels have their switches disabled. Judgment and rendering are extracted from `SecurityTab` into `NotificationChannels.tsx`, shared by both places to prevent drift; switches render read-only (currently a platform-level setting with no user-side write endpoint).
- ③ The broadcast entry is **removed** from the profile center: `NotificationsTab` no longer takes `isAdminUser` / `onSendClick`, and `BroadcastModal.tsx` is deleted. A new admin page `/admin/announcements` (`access: 'canAdmin'`, matching the backend admin route group's authorization) wires List/Create/Update/Delete with audience (everyone/specified roles), popup flag, and effective time range.

**Two defects fixed along the way**

- `SimpleList.Item` did not pass through DOM attributes, silently dropping `data-testid` / `data-*` (antd's `List.Item` passes them through). Added `...rest` passthrough — otherwise tests like "assert unread state by item id" are unwritable.
- The announcements page's `load` depends on `t`, derived from `intl`; the test env's intl mock returns a fresh reference each render → `useEffect([load])` re-fires → one submit pulls the list 9 times. Fixed by forwarding `t` through `useRef` so its reference is stable.
- `submit` never caught `form.validateFields()`'s rejection → clicking "save" with an invalid form produced an unhandled rejection.

**Regression tests**

- `web/src/pages/Profile/__tests__/NotificationsTab.test.tsx` 13 cases: ③ no send/broadcast controls whatsoever; ① unread highlight / mark-one-read / mark-all-read / unread count / details, with `标为已读` not opening details (stopPropagation); ② channel-state rendering and "no 'enabled' may appear before loading completes".
- `web/src/pages/Admin/Announcements/__tests__/index.test.tsx` 10 cases: list / empty state / audience / flags shown truthfully / load failure does not white-screen; create required-field validation; `audience=role` requires role names; list re-pulled after create; edit with id goes through update; delete double-confirms.
- `SecurityTab.channels.test.tsx` (14 cases) now imports `channelAvailability` from `NotificationChannels`, locking the shared-implementation contract in tests.

---

## BUG-022 The Login Page Has No antd App Context: All Login Success/Failure/MFA Messages Are Lost

**Severity**: high (zero feedback on login failure)

**Symptom**

Playwright, three paths measured (before the fix):

1. Opening `/user/login` directly and logging in with the correct password → login succeeds, but **no "login succeeded" toast**;
2. Redirecting from `/` to the login page and logging in → likewise no toast, and the console reports `[antd: Message] You are calling notice in render ...`;
3. **A wrong password → absolutely zero page feedback** (no toast, no inline error; the page just sits there).

**Root cause**

`src/app.tsx` mounted `AppApiRegistrar` (`AntdApp.useApp()` → `setAppApi` registering the global message/notification/modal) inside the **logged-in layout**'s `childrenRender`, while the login page is `layout: false` and never passes through the layout at all. Two message-losing paths:

- Opening the login page directly: `getMessage()` stays `undefined`; every `getMessage()?.xxx()` is silently swallowed by optional chaining;
- Visiting `/` first, then redirecting: `/` briefly mounted the layout's `<AntdApp>`, so `setAppApi` registered an instance whose holder was never mounted (or already unmounted with the layout) and whose effect had no cleanup; the login page calls `message.success` on that dead instance, antd detects `holderRef.current` is empty, warns, **and silently drops** the message.

Additionally the login page's inline-error fallback is dead code: `const [userLoginState] = useState({})` has no setter, so the `status === 'error'` Alert branch can never hold — the toast is the only failure-feedback channel, and losing it means zero feedback.

**Fix**

- `src/app.tsx`: `AppApiRegistrar` is promoted to module level; a new umi runtime `rootContainer` export wraps **the entire app** (including the `layout: false` login page) with `<AntdApp>`; `childrenRender` no longer double-wraps (`ScopeMenuRefresher`/`SettingDrawer` stay put — they depend on the layout's runtime closure).
- The effect gains symmetric cleanup: `utils/antdApp.ts` adds `clearAppApi(api)` (clears only if still pointing at the same instance, never clobbering a later registrant), so unmounting `<AntdApp>` leaves no dead instance behind.
- Incidentally deletes the `userLoginState` dead state (`const [userLoginState] = useState({})` and its never-true `status === 'error'` branch). — Ignore this item if that code has since changed.

**Regression tests**

- `web/tests/appAntdRoot.test.ts` 4 cases: rootContainer exists and `<AntdApp>` + `AppApiRegistrar` wrap `{container}`; the registrar effect contains `setAppApi(inst)` and the `clearAppApi(inst)` cleanup; childrenRender no longer renders `<AntdApp>`; the antdApp module exports `clearAppApi` (guarding both "move it back into childrenRender" and "delete the cleanup" regressions).
- `web/src/utils/antdApp.test.ts` adds 3 `clearAppApi` semantics cases (clears the current instance, does not clobber a later registrant, no-op when unregistered).
- Runtime verification (Playwright, after the fix): the three paths toast "登录成功！" / "登录成功！" / "登录失败，请重试！" respectively, with 0 `notice in render` warnings.

---

## BUG-023 The Announcements Page Registered 26 of 40 Locale Entries: 14 Missing + the Menu Key Missing

**Severity**: medium (the English UI falls back to Chinese; the console spams Missing message)

**Symptom**

On `/admin/announcements` under en-US, form labels/placeholders/validation messages/row actions (edit/delete)/the "announcement updated" toast all display **Chinese defaultMessage**; every render spams `[React Intl] Missing message: "pages.announcements.field.*"` in the console. The sidebar entry lacks `menu.AccessControl.Announcements` in both zh-CN/en-US, spamming once per render (61 lines in one walkthrough).

**Root cause**

When 2c236aa added the announcements page it referenced 40 `pages.announcements.*` ids but the locale catalogs registered only 26 — everything missed was in the **modal form** (labels and validation for title/content/audience/role/popup/active/range), row actions (edit/delete), and update.success — the batch invisible unless you open create/edit. The walkthrough script at the time also covered only the list layer. The menu key: routes got the `name`, but neither `menu.ts` did.

**Fix**

- `src/locales/zh-CN/pages.ts` / `en-US/pages.ts`: fill in the 14 entries (field.title/.placeholder/.required, field.content/.required, field.audience, field.role/.required, field.popup, field.active, field.range, action.edit, action.delete, update.success), values matching the page defaultMessage semantics.
- `src/locales/zh-CN/menu.ts` / `en-US/menu.ts`: add `menu.AccessControl.Announcements` (系统公告 / Announcements).

**Regression tests**

`web/tests/announcementsLocales.test.ts` 5 cases: extracts **from page source** all `pages.announcements.*` id references (≥40, guarding against the guard itself idling) and asserts presence in both zh-CN/en-US catalogs; separately asserts the bilingual menu key. New entries are followed automatically; any "code references an unregistered id" fails. Mutation check: stashing this round's locale changes turns 4/5 red.

Post-fix full-site walkthrough: console error/warn 0 (89 in the same walkthrough before the fix).

---

## Fake-Data Verification for the Approval Module (Background of BUG-024 / 025)

"Everything about the jump-to-approver is wrong" had never had systematic fake data to reproduce it. This round extended the repo's existing dev-seeding convention (`internal/svc/approval_seed.go`, introduced in 2490e1e) with a repeatable approval fake-data set, and verified the whole chain layer by layer along "the approver shown on the list page → the jump route/params → the target page's query → the backend filter → the displayed fields".

### The Fake-Data Seed (Auto-Seeded in Dev Mode; One Command to Rebuild)

- **Location**: `internal/svc/approval_seed.go` (`seedDemoApprovals`, called in `service_context.go`'s startup sequence after `seedBootstrapGames`).
- **Rebuild**: restarting the dev server rebuilds it — the approval store under dev config is `approvals.NewMemStore()` (`service_context.go`), in-memory and never persisted, fully rebuilt from seed on every startup; naturally idempotent, no cleanup command needed.
- **Production isolation**: gated by `isDevelopmentConfig` (mode dev/development/debug, or `CROUPIER_ENV=dev`; only explicit production turns it off) — the production path executes no seeding.
- **Coverage** (8 records in one scope: 4 approved + 2 rejected + 2 pending):
  - Approvers across **3 distinct accounts**: admin (3) / reviewer01 (2) / reviewer02 (1);
  - Requesters operator / gm01 are **fully staggered** from the approvers (cross-role combinations: ops requests, admin reviews; design requests, security reviews; ops requests, security reviews);
  - All three states present, rejected records all carry rejection reasons; 1 approved record carries `metadata.delegatedFrom=reviewer02` expressing a **delegated approval**;
  - Demo accounts: admin / operator / gm01 / reviewer01 / reviewer02 (login passwords same as bootstrap; admin/admin123).
- **The seed-locking test**: `internal/svc/approval_seed_test.go`'s `TestSeedDemoApprovalsInto_StatesAndTwoPerson` asserts the three-state counts, approver coverage `{admin, reviewer01, reviewer02}`, approver ≠ requester, and exactly 1 delegated record.

### Reproduction and Verdict (Measured on the Local Stack)

```bash
go build -o bin/croupier-server ./cmd/server
(bin/croupier-server --config configs/server-sqlite.yaml &)
# 登录后（X-Game-ID: default / X-Env: dev）：
# GET /api/v1/approvals/                → 8 条种子（三态 × 3 审批人）
# GET /api/v1/audit?kind=approval_approve&actor=<审批人>   → 跳转目标
```

- **Before the fix**: `kind=approval_approve` returns nothing for any actor (total=0) — the BUG-024 scene; the frontend also falls back to querying by the requester when approver is missing (BUG-025).
- **After the fix**: after admin approves one pending record, `actor=admin&kind=approval_approve` yields exactly 1 row (userId=approver, gameId/env in their columns, metadata readable for both requester actor=operator and operator=admin); `actor=<requester>&kind=approval_approve` is permanently 0 rows. When continuation fails (no live agent), the record stays approved and the audit still chains — "the approval happened" and "the continuation result" are separable and decidable.

### Dynamic-Route Walkthrough Caveat (The Dev Server's Default Mock Intercepts Login)

When the browser hits `:8000`, `web/mock/user.ts` intercepts `POST /api/v1/auth/login` and returns `mock-jwt-token-*`; every subsequent real API 401s — **a real-stack walkthrough must start the dev server with `MOCK=none`** (after login, localStorage should hold a real JWT, not a `mock-jwt-token-` prefix). Walkthrough script archived: `/tmp/ui-audit/detail-walkthrough.mjs` (login → ticket list/detail → function catalog/detail → console collection).

---

## BUG-024 Approval Actions Never Write the Audit Chain + Notifications Name the Requester as the Approver

**Severity**: high (the backend root cause of the user-reported "everything about the jump-to-approver is wrong")

**Symptom**

Clicking "view audit (approved)" in the approval list/detail jumps to `/admin/operation-logs?actor=<审批人>&kind=approval_approve`, which forever shows "no data" — for any actor at any time. The approval notification copy reads "已由 operator 通过" — operator is the **requester**, not the approver.

**Root cause**

Two layers compounding:

1. `internal/audit/audit.go` defines `EventApprovalApproved/Rejected` (approval.approved / approval.rejected), but **no writer exists anywhere in the repo** — approvals/rejections only write the extension event stream (`recordApprovalEvent` → the approvals extension table) and notifications; `audit_records` never gains a corresponding row. Operation-logs' kind filter (kindAliases: approval_approve → approval.approved) is therefore permanently an empty set.
2. The notification copy interpolates `record.Actor` (the requester) instead of `record.Approver` (the approver), so message-center/in-site notifications misattribute the approval.

**Fix**

- `internal/api/approval/service.go`: `Approve`/`Reject` call the new `recordApprovalAudit` **before continuation**, writing approval.approved/rejected into the hash-chain audit: actor_id=approver, game_id/env recorded from the record, details carrying both the requester (actor) and the approver (operator) plus functionId/reason. Placing it before continuation is deliberate: a failed continuation is still "approved", so the audit must land first. A write failure is swallowed (audit is a side channel; it never blocks the approval main flow). Note `WithResourceID` replaces Resource wholesale and must be called before `WithGameID`, or the scope columns are erased.
- `internal/api/approval/service.go` + `notify.go`: notification copy switches to `record.Approver`, and the event Data gains an `approver` field.

**Regression tests**

`internal/api/approval/audit_chain_test.go` 3 cases: approve/reject each assert "filtering by approver + event type + scope yields exactly 1 row, details.actor=requester, details.operator=approver", and inversely assert that **no approval.approved row may exist under the requester** (the exact wrong data shape produced before the fix); the third asserts the approval main flow is unaffected when AuditService is absent. Live-stack re-verification: pre-fix baseline total=0; after approval, `actor=admin&kind=approval_approve` total=1.

**Known boundary (unfixed, existing behavior)**: on continuation failure the Approve endpoint returns a `service_unavailable` error to the frontend even though the record is already approved and the audit already chained — the approver sees an error, and only refreshing the list reveals the approved state. The record's reason field keeps the full "approved but continuation failed: …" text. Changing that semantics requires synchronized frontend/SDK prompt adjustments; recorded this round, not fixed.

---

## BUG-025 The Frontend's Approver Jump Falls Back to the Requester (When approver Is Missing)

**Severity**: high (the frontend root cause of the user-reported "everything about the jump-to-approver is wrong")

**Symptom**

In the approval detail, the "view audit (approved)/(rejected)" buttons build their actor param with `current.approver || current.actor` as the fallback — whenever a record lacks approver (historical data, imported data, abnormal states), the jump queries operation-logs for approval_approve **under the requester's** name. Correctly, no such row exists under the requester (someone else did the approval), so the result is either empty or this person's unrelated approvals of **other people's** requests — either way, "the jump lands on the wrong person".

**Root cause**

Two audit-jump buttons in `web/src/pages/Approvals/index.tsx` mishandle "the field may be missing" as "query someone else instead". Semantically the approve/reject action has exactly one subject — the approver; the correct behavior when approver is missing is an **empty-string filter** (a definitive no-results), never falling back to the requester. (The "view audit (requester)" button is already actor=current.actor and unaffected.)

**Fix**

Both buttons become `current.approver || ''`, with comments forbidding the fallback.

**Regression test**

`web/src/pages/Approvals/index.test.tsx` adds an "approver missing but actor present" case: asserts the jump is `actor=&kind=approval_approve` and that `actor=gm01&kind=approval_approve` **must not** appear (the pre-fix behavior was exactly the latter). Combined with the existing two cases ("both approver/actor empty jump with an empty string"), all branches are covered.

**Known boundary (recorded, not fixed)**: delegation currently exists only at the platform layer (`internal/platform/approvals/delegation.go`'s DelegationService, wired to no API handler), and the approval DTO does not expose metadata, so the frontend cannot display `delegatedFrom`. The delegated record expressed via `metadata.delegatedFrom` in the seed is invisible at the API/UI layer for now — display connects once a delegation API lands.

---

## BUG-026 In-Site Message "Send" Has No Permission Check: Any Logged-In User Can Deliver to Any Account

**Severity**: high (unauthorized write: recipients cannot tell the source, and it can forge system notifications)

**Symptom and background**

"Receiving notifications" and "sending notifications" must be separated: the inbox (what I received) is visible to everyone; sending is a back-office operations capability requiring its own entry + a permission gate (user feedback verbatim: "发送信息应该单独的页面，有些人没有权限就不应该看到，而信息通知每个人都可以看到").

The UI side of BUG-021 completed the separation (the profile's messages tab is inbox + channel preferences only, with a jest guard "no send-message/broadcast controls exist on the page"; the only send entry is the admin-only `/admin/announcements`, route `access: 'canAdmin'`, with the sidebar menu filtered by the backend's `menu.AccessibleTree` by permission). But the API side left a hole: the handler of `POST /api/v1/messages` (point-to-point single send) has **no permission check at all** — any logged-in user can call it. The bulk `POST /api/v1/messages/broadcast` has the `isBroadcaster` (admin role) check; the single send has none.

**Root cause**

`internal/api/message/handler.go`'s `Send` binds the request body and stores it directly without checking the current user's role; moreover `model.Message` has no sender field, so a message's origin is completely untraceable (who sent it, whether the sender is who they claim).

**Fix**

`Send` reuses the same `isBroadcaster` gate as `Broadcast` (admin role only, including point-to-point sends); the receiving side (List/Detail/Read/UnreadCount/Stream) stays available to all logged-in users. The frontend `services/api/messages.ts`'s `sendMessage/broadcastMessage` are called by no page (the send UI's only entry is the announcements admin page); they are kept as the admin API client, and non-admin calls now hit the backend's 403 backstop. Existing message tests that used an unauthenticated `handler.Send` for seeding/the main path are corrected to the new contract: receiving-side cases seed directly at the model layer (`seedMessage`), send-path and validation cases use an admin login state (`newMessageHandlerWithAdmin` + `withReqUsername`).

**Regression test**

`internal/api/message/send_permission_test.go`: reuses broadcast's admin/ops two-account fixture, asserting unauthenticated 403, ops role 403, admin 200 with the message stored, and that the ops user's inbox (GET /messages) still 200s and reads — locking both sides' semantics at once.

**Known boundary (recorded, not fixed)**: the `messages` table has no sender (from) column; in-site messages record no source account. Adding the column is a model change requiring a numbered migration per the migration contract plus display-layer design — a separate charter. All current legal send paths (announcements/broadcast/system notifications) are admin or system actions; risk is contained.

---

## BUG-027 The Ticket Detail's Descriptions Row Span Overflows, Spamming an antd Warning on Every Open

**Severity**: low (console noise, no functional impact; same family as BUG-005's "TS says nothing, the page says nothing")

**Symptom**

Opening `/support/tickets/:id`, every render spams `Warning: [antd: Descriptions] Sum of column 'span' in a line not match 'column' of Descriptions`.

**Root cause**

In `Detail.tsx`'s `Descriptions column={2}`, "玩家ID" (span=1) is followed by "联系方式" (`span={2}`), so that line's span sum 1+2=3 exceeds the 2 columns — an antd 6 runtime warning TypeScript never flags. Walkthrough confirmed: opening a ticket detail on the real stack (MOCK=none) triggers it.

**Fix**

"联系方式" drops `span={2}` back to span=1, sharing the row with "玩家ID" (the layout intent was always a pair).

**Regression test**

`Detail.test.tsx` adds a "span sums equal the column count" case: spying `console.error` while rendering a full ticket, asserting no `[antd: Descriptions]` warning. Mutation check: the case is red before the fix (the warning captured) and green after (37/37).

**Walkthrough conclusions (this entry's origin, plus the rest of the round's verdicts)**

- `/support/tickets/:id` with a real id: the detail renders fully (title/status/priority/action buttons) ✅; a nonexistent id: graceful "resource not found" + a back button, no white screen ✅;
- `/functions/:id` with an unknown id: graceful "function not found / 当前函数不存在" guiding back to the catalog ✅ (dev has no registered functions; the real-id scenario is covered equivalently by the fallback);
- The 404 request for a nonexistent ticket id logs an ownerless `AxiosError` pageerror (the UI already falls back; console noise only), recorded during this walkthrough, not chartered separately.

---

## BUG-028 The Bootstrap Admin Can Be Deleted (Deleting Breaks the Login Chain) + No Explicit Disable/Unban Entries

**Severity**: high (deleting a self-bootstrap account like `admin` immediately breaks the login chain; a restart re-seeds it, but it is already a destructive act) + a UX gap (disabling an account is only reachable through the "edit" dialog's enabled toggle — no standalone disable/unban actions, and the "disable revokes tokens" semantics is invisible).

**Symptom and background**

The user requires: admin (accounts declared in the bootstrap config `configs/admins.json`) must be undeletable, disableable, and unban-able. Previously `/admin/permissions/users` rendered "delete" unconditionally on every row, the backend's Delete had no protection, and the status semantics (`1=enabled / 0=disabled`, `-1=unchanged`) lived in scattered literals.

**Root cause**

The backend's knowledge of "accounts in admins.json" lives only in `AdminManager`'s in-memory map; neither the REST Delete nor the frontend consumes it — bootstrap accounts are indistinguishable from ordinary ones in the API view.

**Fix**

1. Backend protection: `AdminManager.IsBootstrapAdmin(username)` exposes the bootstrap set; inside `Delete`'s transaction (after FindOne), hitting a bootstrap account returns 403 "引导管理员不可删除，可改为禁用" (transaction rolled back, no half-deleted state); List/Create/Get/Update responses uniformly attach the computed `bootstrap` field via `buildAdminView`, from which the frontend hides the delete entry and renders a "bootstrap" badge.
2. Explicit disable/unban: the action column gains inline actions — an enabled account shows "disable" (danger, with a Popconfirm stating "已签发登录凭证立即失效，之后可随时解封"), a disabled account shows "unban". Submission uses the existing Update `status=0/1` (disabling automatically `BumpTokenVersion`s to revoke all tokens); the delete entry renders only for non-bootstrap accounts.
3. Status constants (user review note: bare literals forbidden): the backend reuses `model.StatusEnabled/StatusDisabled` uniformly; the Update contract's "do not modify" sentinel is named `admin.AdminStatusUnchanged = -1` (Go's int zero value collides with 0, so -1 must be explicit). The frontend exports `ADMIN_STATUS_ACTIVE/DISABLED/UNCHANGED` from `services/api/permissions.ts`, replacing every literal in UsersV2.

**Regression tests**

- Go: `TestService_Delete_BootstrapAdminForbidden` (a temporary admins.json declares `rootadmin`; Delete must 403 with the account intact; mutation check: removing the guard turns it red), `TestService_Delete_NonBootstrapAdminAllowed` (the unprotected control), `TestService_List_BootstrapFlag` (the bootstrap flag is true only on declared accounts), `TestAdminManagerIsBootstrapAdmin` (nil/blank/unknown usernames safe).
- jest: `UsersV2/__tests__/index.test.tsx` 3 cases — a bootstrap row has no "delete" and carries the "bootstrap" badge; an enabled account's "disable" submits `{status: 0}`; a disabled account's "unban" submits `{status: 1}`.

**Honest boundaries**

- A disabled bootstrap account still cannot be deleted (the protection is status-independent); disabling every admin locks out the login surface — that is an ops action, not code-limited.
- The `bootstrap` flag is computed live from the username: after removing the declaration from admins.json and restarting, the account automatically degrades to an ordinary (deletable) account.

---

## BUG-029 Instance Metadata Lost End-to-End: Two Leaks, in the Go SDK Handoff Chain and the Agent's TCP Registration Path

**Severity**: high (the feature is completely unusable — the `sdk-distribution` metadata page and `/api/v1/providers/sdk-stats`'s `metaKey/metaValue` filters are permanently empty for all Go SDK instances; the instance-metadata feature was dead on arrival).

**Symptom and background**

The instance-metadata feature (SDK `InstanceMetadata` → `ProviderConnectRequest.metadata` → agent → server → sdk-stats) was green in every local unit test, but the live end-to-end check showed `meta=None` on every instance: the server's filter logic worked yet never found any user metadata.

**Root cause (two independent loss points, each in a unit-test blind spot)**

1. **The Go SDK handoff chain's hand-copied fields omit one**: `client.Connect()` builds `ManagerConfig` and `NewManager()` copies back `ClientConfig` field by field, and neither carries `InstanceMetadata` — the metadata is swallowed inside the SDK and never reaches the registration frame.
2. **The agent's TCP provider path drops it wholesale**: the Go SDK connects over TCP 19091, hitting `tcp_local_listener.handleConnect` — that path parses `ProviderConnectRequest` but never reads `metadata` (`agent.ProviderSession` has no Metadata field at all), while `app.go`'s `SetOnConnect` callback **hardcodes instance metadata to platform keys only** (sdkLanguage/sdkVersion/sdkName/gameId/env). The existing metadata-merge unit test covers `local_handler.handleProviderConnect`, which is never called in the TCP provider flow — the test is green on a path production never takes.

**Fix**

1. SDK: `ManagerConfig` gains an `InstanceMetadata` field; both handoff points in `Connect()` and `NewManager()` copy it.
2. Agent: `agent.ProviderSession` gains a `Metadata` field; TCP's `handleConnect` parses user KVs through the new shared helper `MergeUserProviderMetadata` (reserved keys dropped with warnings echoed into the response, empty keys skipped — semantics identical to the local_handler path); `app.go onConnect` registers instances via `MergeProviderInstanceMetadata` as "platform keys + session user KVs"; the local_handler path reuses the same helper, converging both connection paths to one source.

**Regression tests**

- Go SDK: `TestNewManagerCarriesInstanceMetadata`, `TestNewTCPManagerCarriesInstanceMetadata` (both handoff points), `TestRegisterFrameCarriesInstanceMetadata` (frame-level: a fake agent captures the 0x050101 frame and decodes the metadata).
- Agent: `TestTCPProviderConnectCarriesUserMetadata` (a real TCP dial → connect → the onConnect session carries user KVs, reserved keys dropped, response warnings annotated), `TestMergeProviderInstanceMetadata` (platform keys + user KVs merged, nil-safe).

**Honest boundaries**

- Older registered SDK versions (< this fix) still send no metadata; reconnecting after an SDK upgrade restores it;
- Reserved-key conflicts are dropped agent-side only and reported in the connect response's warnings; the SDK does not double-check.

**Addendum (same day, a third loss point)**: after go-live, the demo container's session recovered via `reconnectWithBackoff` after an agent restart and instance metadata went empty again — the reconnect path in client.go **hand-builds another** `ManagerConfig` (`Connect()` was fixed; this one was not) and omits `InstanceMetadata`. Fix: extract `buildManagerConfig()` shared by first-connect and reconnect; regression test `TestReconnectCarriesInstanceMetadata` (fake agent, real connection → agent-side cut → the reconnect frame's metadata asserted; red before the fix, green after). Lesson: every additional hand-copied field handoff in a config chain is one more loss point — they must converge into a single constructor.

---

## BUG-030 resource-catalog Category/Search Filters Silently Dead: Query Binding Matches Field Names Exactly

**Severity**: high (the feature is completely unusable — the "choose category" and "search" interactions never do anything; reported repeatedly by users).

**Symptom and background**

On `/functions/resource-catalog`, choosing a category leaves the list unchanged. The frontend wiring is correct (Select onChange → `?category=...` → auto refetch), and the backend `List()`'s category filter logic is itself correct. Live proof: `GET /api/v1/resource-catalog?category=inventory` returns all 5 records, while `?Category=inventory` (capital C) filters correctly — one letter of case.

**Root cause (helper design flaw + the DTO's trigger condition)**

`internal/common/requestbind.BindQueryCompat`'s reflection fallback **runs only when `ShouldBindQuery` returns an error**; but gin's `ShouldBindQuery` matches tagless fields by **exact field name** (case-sensitive), and binding a string-only struct always succeeds:

1. `resourcecatalog.ListRequest`'s four fields have no form tags → gin matches by the field name `Category`; the frontend's contract key `category` never hits;
2. Binding always "succeeds" → the old reflection fallback never executes a single line.

Double failure compounding; the filter params vanish silently. In `handler_coverage_test.go`, the original `?query=player` case seeded a single record and asserted `Total==1` — which is also 1 without filtering. **The test looked like it covered filtering but had zero discriminating power**, so the bug survived.

**Fix**

1. `ListRequest` gains explicit canonical form tags (`form:"gameId"/"env"/"category"/"query"`) — this instance's fix;
2. `BindQueryCompat`'s reflection fallback now **always runs** (binding errors no longer return early); tagless fields fall back through lcFirst → all-lowercase → case-insensitive in three tiers. Go's initialism-style field names (`GameID`) versus contract keys (`gameId`) have different case distributions that no pure string transform can derive — the final tier's `strings.EqualFold` catches it.

**Regression tests (red before the fix, green after)**

- `TestBindQueryCompat_UntaggedFieldsBindLowercaseNames` (a tagless struct + lowercase keys: gameId/env/category/query/id all bind; `form:"-"` does not);
- `TestBindQueryCompat_TaggedFieldsStillWinOverLowercaseFallback` (form-tag keys take precedence);
- `TestHandler_List_CategoryAndQueryFiltersBindLowercaseParams` (with two seeded resources, category/query filters genuinely narrow — the old implementation's total was always 2);
- Rewritten `TestHandler_Upsert_GETMethod` (internal/api/config, a third case of the same disease): the old assertion only checked "not 400", while gin actually failed to bind `key/value` and the service errored 500 on an empty key — the test **went green for the wrong reason**; once the fix made binding real, it exposed the test's service as a half-constructed shell (a nil model panics). Rewritten to the sibling case's binding-direct assertion (the old code's req.Key was always empty — a usable regression probe);
- Rewritten `TestBindQueryCompatJSONFallback`: the original assertion "json tags don't work with query binding" was a characterization test **fossilizing the defect as expected**; it now asserts the json fallback genuinely catches.

**Lesson**

gin's `ShouldBindQuery` "succeeding" ≠ "having bound"; on a string-only DTO it always succeeds. Any fallback path of the "only on error" kind is dead code when the preceding call always succeeds.

---

## BUG-031 functions/pages Proposal Version Flooding: jsonb Key Order Makes the Digest Never Equal

**Severity**: high (~11 meaningless proposal versions online every ~4 minutes; after restarts/agent reconnects, functions/pages sprouts a pile of "needs handling" warnings; one proposal measured at 2927+ accumulated versions).

**Symptom and background**

Every server restart or agent reconnect sprays "page proposal pending confirmation" warnings onto `/functions/pages`. A demo restart changes no fields — the user judged it a false-positive bug. Controlled re-verification: after restarting croupier-agent, `proposal_versions` climbed from 2927 to 2971; the **only** diff between adjacent versions' payloads was UpdatedAt, with `function_digest`/`semantics_digest` columns identical.

**Root cause**

`proposalComparableDigest` hashes the **raw bytes** of PageSpec/Diagnostics' `model.JSON`, but `page_proposals.page_spec`/`diagnostics` are Postgres **jsonb** columns — jsonb reorders object keys and strips whitespace. Freshly marshaled bytes and DB-round-tripped bytes are **never equal** → `generatedProposalChanged` always true → every proposal rebuild judges "content changed" and floods a new version snapshot. The flooding also piles up pending warnings.

**Fix**

New `canonicalJSONBytes` (unmarshal → re-marshal with normalized key order); PageSpec/Diagnostics are canonicalized before entering the digest, and the DB-read side is canonicalized the same way before comparison.

**Regression test (red before the fix, green after)**

`TestGeneratedProposalChanged` gains a jsonb regression at the top: equivalent JSONs with different key order (`{"b":1,"a":{"y":1,"x":[2,3]}}` vs the reordered, whitespace-stripped shape) assert "unchanged"; a value-only change (`"b":2`) still asserts "changed". Before the fix the first assertion was always red (the byte digest judged true).

**Lesson**

Always normalize key order before comparing against jsonb columns; a storage round-trip that is "semantically identical, byte-different" is a hidden bomb in digest/idempotency logic — local sqlite (JSON text stored and read verbatim) cannot expose it; only online postgres takes the jsonb path.

**Addendum (same day, the second pathogen — re-registration timestamp flooding)**: after canonicalJSONBytes shipped, the continuous flooding (every ~4 minutes, +11) stopped and 12 idle minutes produced zero new versions — but an agent restart still added +4 versions per proposal. The only substantive diff between adjacent payloads was `SemanticsDigest` (PageSpec/FunctionDigest identical): `computeDigest(semantics)` fed the entire `CapabilitySemantics` struct into the digest — embedding gorm.Model (ID/CreatedAt/UpdatedAt/DeletedAt) + an explicit UpdatedAt — and every agent re-registration's upsert refreshes timestamps, so unchanged content still judged "changed". Fix: `semanticsComparableDigest` digests only the semantics' **stable content projection** (JSON fields canonicalized too); regression `TestSemanticsComparableDigest_StableAcrossRegistrationRefresh` (the old whole-struct digest is genuinely unstable across identical content/different metadata — a documented root-cause assertion). Boundary: a proposal without resources has `semantics == nil` (no semantics row) and projects to all-zero semantics — the digest stays a non-empty 64 characters (the `PageProposal.SemanticsDigest` column contract) and is indistinguishable from "an empty semantics row", the two having no content difference to compare (the first implementation returned an empty string and was caught red-handed by `TestContractService_RebuildProposalForFunctionWithoutResource`'s 64-length assertion).

**Addendum (same day, the third pathogen — the row's Version increments unconditionally and is fed into the projection)**: after the second fix deployed, online re-verification: idle flooding zeroed, but **each agent restart still added +3 versions per proposal** (the earlier measured +4 was pathogens two and three stacked). Online evidence: the `resource:order` proposal v385→v390 kept `function_digest` constant with only `semantics_digest` changing; the `capability_semantics` row's `version` went 390→393 while **every stable field was column-for-column identical** — the semantics row's `Version` is a row-version counter incremented unconditionally on every upsert, and the second fix's stable-content projection **mistakenly fed Version into the digest** as well: reconnect upsert → Version+1 → digest changes → "semantics changed" → one snapshot version + one semantics-history version flooded per proposal. Why unit tests were green while production was red: the unit test simulated only "timestamp refresh", never the Version-increment path.

Fix (converged into a single model-layer implementation):

1. `CapabilitySemantics.StableContentDigest()` (model layer): the **sole implementation** of the stable content projection, excluding ID/timestamps/UpdatedBy/**Version**; JSON fields normalized through `normalizeJSONContent` (Unmarshal→Marshal) against jsonb byte drift. The service layer's `semanticsComparableDigest` becomes a thin delegate — the model's upsert change judgment and the proposal's SemanticsDigest sharing one source; two projections would inevitably drift.
2. `UpsertSemantics` changes signature to `(bool, error)`: unchanged content **does not touch the row** (neither Version nor UpdatedAt refreshes) and backfills the row identity; only real changes `Version+1` into the DB.
3. `rebuildFromContracts` inserts a `capability_semantic_versions` history row only when `changed` (automatic reconnects no longer flood history). Manual edits/conflict resolutions keep the "explicit save = a version history entry" semantics (createSemanticVersion afterwards runs unconditionally).

Regression tests (red before the fix, green after): `TestCapabilitySemanticsModel_UpsertSkipsBumpWhenContentUnchanged` (the old implementation, same content run twice, Version=2 red; the new implementation changed=false, Version=1, UpdatedAt unrefreshed; a real change still bumps), `TestSemanticsComparableDigest_IgnoresRowVersion` (two semantics differing only in Version must digest equal — the old projection fed Version and was always red).

Lesson: **a "row version number" is lifecycle metadata, not content** — putting it into a content projection wires an auto-incrementing counter into the change detector, amplifying any write path's unconditional bump into "content changed". A projection must have exactly one implementation (model layer); the service layer may only delegate.

**Addendum (same day, the fourth pathogen — Provenance embeds rebuild timestamps)**: after the third fix deployed, online re-verification **FLOOD-REMAIN**: each agent restart still added +3 per proposal (proposals and semantics rows 410→413, and idle observation showed 414 still climbing). `StableContentDigest` was in the running binary (symbol grep confirmed deployment), yet upsert still judged "changed". Forensics: `capability_semantics.provenance` embeds `updatedAt`/`updatedBy` inside every field entry, and `internal/dashboard/normalizer/provenance.go:75/85/103/180` stamps `UpdatedAt: time.Now()` on every track — **rebuilding refreshes Provenance's timestamps**, so after stripping the row-level timestamps, timestamps still hide inside the JSON blob. The fourth layer of the same disease: the projection ingested "when it was written" rather than "what it is".

Fix: `normalizeProvenance` (model layer) = jsonb normalization + recursively stripping `updatedAt`/`updatedBy` keys before the digest; entries' substantive content (field/value/source/status/confidence/sourceDigest) is fully preserved and still participates in change judgment.

Regression tests (red before the fix, green after; the red proven by temporarily reverting `normalizeJSONContent` to pass through): `TestCapabilitySemanticsDigest_ProvenanceVolatileKeysIgnored` (timestamps alone differing must digest equal; a flipped source must still differ), `TestCapabilitySemanticsModel_UpsertIgnoresProvenanceTimestamps` (timestamp-only drift gives changed=false, Version stays 1).

Lesson: a content projection must not only strip **row-level** lifecycle metadata but also recursively audit **inside the JSON blobs** — any blob written with timestamps by the build process (the provenance/diagnostics family) is the same minefield; the acceptance criterion for "what a projection ingests" is: re-run the system verbatim and every field fed to the projection must reproduce bit-for-bit.

**Close-out (2026-09-27, online re-verification passed)**: after the four-pathogen fix deployed (deploy run 36311407559), restarting croupier-agent left the proposals' max version (resource:* = 423) and the semantics row (= 422) bit-for-bit identical across the restart — **zero bump**, the FLOOD stopped (the same scenario previously flooded 410→414). The re-verification script includes a snapshot-row gate (<8 rows judged invalid) against the false green of "two failed queries whose empty outputs diff equal". OPEN-ISSUES #12 moved to closed.

---

## BUG-032 functions/assignments Never Gates Execution: The Data Exists, the Gate Does Not

**Severity**: high (the user measured "calls execute remotely with no assignment made" — the assignments page is fully operable, the overview statistics complete, yet the execution chain never reads the assignment data; pure decoration).

**Symptom and background**

The `/functions/assignments` page maintains "which functions are open" per game/env, with a full feature surface: overview statistics (assigned/unassigned), change history, clone-to-env, and more. Yet the user could remotely execute every function with no assignment made — suspecting "the assignment never takes effect (or its semantics are not an execution gate)".

**Root cause (forensics)**

- Assignment data lands in `data/assignments.json` (`gameID|env → []functionId`); only `internal/api/assignment` (the page CRUD) and the legacy `internal/logic/assignment` read it — **zero consumers in the invoke chain**.
- The execution chain's actual permission surface: the scope middleware (game/env authorization) → `EnsureFunctionEnabled` (disabled interception) → `enforceFunctionPolicy` (AllowedRoles restriction, empty = pass; the approval workflow). Nothing judges "is this function assigned to the current scope".
- The frontend assignments page never explained its semantics to users, and the "assignment overview / assigned / unassigned" wording strongly implies gate semantics — a feature surface with enforcement missing, the same family as BUG-009's "the disabled button does not intercept": "UI promise > actual enforcement" drift.

**Fix (including the default-policy decision; OPEN-ISSUES #36 ①②③)**

- **Open by default**: a scope (gameID|env) with no assignment record → pass (backward compatible; deployments without assignments behave exactly as before go-live; the existing invoke cases are the implicit regression).
- **A record means an allowlist**: a function not in the list → 403 `function_not_assigned` "函数未开放执行权限：当前游戏/环境未分配该函数（可在「函数分配」页添加）"; an empty list = an empty allowlist (deny all), so `Update` with an emptied list **deletes the record** back to open-by-default (otherwise users could never recover through the UI).
- The gate sits at the same two entry points as EnsureFunctionEnabled: `functionInvoke` + `task Start` (preventing the async entry from bypassing — the same reasoning as E2's disabled interception); rejection happens before telemetry/audit — a rejected call is not an execution and must not pollute execution records.
- Scope keys match with EqualFold (consistent with filterAssignments); function IDs match exactly (registry identifiers are case-sensitive). A file-read failure passes the error through (the same truth table as List — never a silent fail-open).
- `errorx` gains `NewForbiddenWithCode` (symmetric with the existing `NewConflictWithCode`).
- The assignments page top gains an "execution gate" semantics note (zh/en locale) — the UI now explains the semantics.

**Boundary (honest)**: the gate covers the dashboard HTTP invocation path; the SDK/agent's TCP direct connection does not pass it (transport-layer auth is independent). Admin does not bypass — consistent with EnsureFunctionEnabled (scope-level admission, no role exemption), with the 403 copy pointing straight to the assignments page.

**Regression tests (red before the fix, green after)**

- 5 gate unit cases (`assignment/gate_test.go`): no record passes / in-list passes / not-in-list 403+stable code / empty list denies all / scope case-insensitive and non-leaking.
- Invoke integration (`invoke_assignment_test.go`) 2 cases: unassigned 403 with **caller.requests empty** (the request never reaches the agent; sync and async share the gate); in-list passes. The pre-fix red was proven by stashing the two gate call sites (both integration cases FAIL).
- Task integration (`task/service_assignment_test.go`) 2 cases: unassigned 403 with `rt.lastReq == nil` (no task dispatched); in-list passes the gate (the chain reaches the login-user check, proving pass rather than reject).
- `TestService_Update_RemoveAll` asserts the update: after save-with-empty, the key must be deleted (the old behavior left an empty list — which the new semantics would treat as an empty allowlist and deny all).
- jest 2 cases (`Assignments/__tests__/gateSemantics.test.tsx`): the page renders the gate-semantics Alert (open by default / allowlist / 403 / empty-restore) + zh/en locale keys present.

**Lesson**

"Has a page, has data, has statistics" ≠ "has enforcement" — when adding an admin surface, the first question is "who consumes this data on the execution path"; if unanswerable, it is a BUG-009/BUG-032-family decorative feature. Also: when adding a gate to decorative data, the default policy (open/closed) and "how to return to the default" must be decided in the same design pass — otherwise the natural act of clearing the list manufactures a lockout state.

---

## BUG-033 Component-Template Preview Renders No Mock Data: The Generation Chain Is Fine, the Consumer Misses/Misreads in Three Places

**Severity**: medium (the preview's core promise — "enable mock data and verify safely" — does not hold; users repeatedly reported "claimed fixed several times, never solved" — every prior fix touched the mock-generation side, while the real lesion sat in the render-consumption side).

**Symptom**

In `/functions/component-templates`' preview modal, "data source" is switched to mock, yet the preview area still shows no fake data (an empty table or a crash state); built-in templates and custom component templates are both affected.

**Root cause (forensics)**

The mock generation chain itself is fine: `generateMockResponse` has fallbacks for missing outputSchema and unregistered functions (`generateFallbackMockData`). The damage is in three consumer-side places:

1. **The column fallback takes top-level schema properties**: when `props.columns` is empty (built-in template generators always emit `columns: []`), `PreviewNode.previewColumns` falls back to `schemaProperties(fn.outputSchema)` — for a list-shaped schema (`{items:{...}, total:{...}}`) that fetches the `items`/`total` container fields, not row fields; the table's `dataSource = itemsOf(payload)` holds items elements, so `record.items` is always undefined → a table with no cell content.
2. **Row extraction only recognizes `payload.items`**: `previewShared.itemsOf` has no fallback — when the schema's array field is not named `items` (e.g. `players`), mock rows can never be fetched (the compiler has an items selector mapping the list field to `pageState.items`; the preview has no equivalent).
3. **An unregistered function yields zero columns**: fnById missing takes the fallback row data (3 rows), but `schemaProperties(undefined) = []` → a 0-column empty table where even the row count is invisible.

**Fix**

- `types.ts` adds `rowFieldsOf(schema)`: row fields = the element properties of the first "array-of-objects" field (same lineage as `generateMockOutput`'s row generation and the compiler's items selector) → top-level scalar properties (objects/arrays excluded — bare rendering is illegal) → empty.
- `PreviewNode.previewColumns`'s fallback chain becomes: declared columns → `rowFieldsOf` → `FALLBACK_ROW_FIELDS` (a new mockData constant, aligned with `generateFallbackMockData`'s row shape); object/array cell values render JSON-serialized (against React's illegal-children crash).
- `previewShared.itemsOf` gains a fallback: when payload.items is missing, take the first "array-of-objects" field.

**Regression tests (red before the fix, green after; 4 new cases in `previewMockData.test.tsx`)**

- The built-in-template shape (`columns:[]` + a list schema + autoRun): mock rows visible (`u-1001`), headers the row fields uid/nickname — before the fix the whole table was empty.
- An unregistered function (empty fnById): the fallback row `mock-1001` visible — before the fix, zero columns.
- An array field not named items (`players`): the row-extraction fallback hits — before the fix, rows unfetchable.
- A row containing a nested object field: columns take row fields and object values render JSON-serialized — before the fix, top-level items columns and an empty table.

**Lesson**

"Data was generated" ≠ "the user can see it": fixing "it doesn't display" requires verifying the whole chain — select → generate → **consume/render**; verifying only the generation side produces "fixed several times, never solved". Field mappings shared between preview and publish semantics (the items selector) must be implemented in pairs; a one-sided implementation is a drift point.

---

## BUG-034 The Mock e2e "One-Click Publish/Unpublish" Cancel Button Times Out: AntdApp Mounted at the Wrong Layer, So the Modal Holder Misses zh_CN

**Severity**: medium-high (the CI Dashboard mock e2e went red for 12+ consecutive runs (from 2026-09-26 18:13), blocking the quality gate; and antd's built-in copy was silently English in every confirm/modal scenario — not just a test problem).

**Symptom**

`page-studio.spec.ts › 一键发布/下架按钮在主视图直接可见`: the confirm modal pops normally, its copy visible, but `getByRole('button', { name: /取 消/ })` times out at 20s. The CI snapshot shows the cancel button's accessible name is **"Cancel"**, while okText "发 布" is fine (our own string).

**Root cause (forensics)**

f56bfa8 (BUG-022) moved `<AntdApp>` from the logged-in layout's childrenRender to app.tsx's `rootContainer`. umi's render chain, outside-in: rootContainer (the plugin-antd theme ConfigProvider; app.tsx's wrapper) → **i18nProvider (plugin-locale injects the antd zh_CN locale here)** → innerProvider → routes. AntdApp's modal/message holders render at AntdApp's React layer; mounted at rootContainer they sit **outside** the zh_CN ConfigProvider — App.useApp().modal's confirm buttons all fall back to antd's default locale (en). Before the BUG-022 fix, AntdApp sat deep inside the layout (within the locale), which is why CI was green before 9-26 05:38; the layer change alone turned it red.

**Fix**

app.tsx: the mount layer `rootContainer` → `innerProvider` (the innermost layer of the render chain, hugging the routes, still covering every route including the `layout:false` login page — BUG-022's semantics unchanged; the holder enters the zh_CN context and updates reactively with language switches).

**Regression (red → green)**

- The jest guard `tests/appAntdRoot.test.ts`: now locks the innerProvider wiring and **adds a "rootContainer must not mount `<AntdApp>`" assertion** — any change moving `<AntdApp>` back to rootContainer goes red here first (this regression's jest-level reproduction).
- The original e2e case: reproduced locally red on a static build (the same CI path, MOCK=all prod build) (`waiting for getByRole('button', { name: /取 消/ })` 15s timeout, 1 failed) → green after the fix (1 passed, 52.7s).

**Lesson**

A "layout hierarchy fix" must be checked against its position in the framework's synthetic Provider chain: holder-type context components like AntdApp have their mount layer determine whether they inherit lower Providers (locale/theme). umi's rootContainer/i18nProvider/innerProvider wrapping order is an implicit contract, and wiring-guard tests should pin down "at which layer", not merely "whether it exists".

**Boundary (honest)**

The same run's dashboard-quality unit-test step was canceled at 21:43 by the GitHub runner ("The operation was canceled"; run_attempt=1, no concurrency group, neither 30min budget touched, cases still PASS when killed; the previous run's same job was success) — judged a one-off infra cancellation, not a case failure or a budget timeout; no config change, re-verified with the new run from this push.

## BUG-035 Assignments Allowlist Fossil null Values Leak to the Frontend: A Legacy Empty-Save Artifact, With No Read-Side Guard

Severity: low-medium (pollutes selection state/statistics; does not block the save chain)

Symptom (live diagnosis, 2026-09-27): the deployment host's `assignments.json` contained `"default|dev": null` — before 861841d (BUG-032), Update wrote `assignments[key] = accepted` directly, so an empty selection saved a null value to disk, and the volume persisted it across deployments into a fossil. GET /api/v1/assignments passed the null through, and useAssignmentsPage's `Object.values(m).flat()` folded it into a `[null]` phantom selection entry.

Root cause: the read-side `loadAssignments` had no normalization for per-key nulls; two isomorphic implementations (internal/api/assignment and internal/logic/assignment) needed the fix in sync.

Fix: both `loadAssignments` implementations drop null keys on read — the product semantics "clearing the list and saving restores open-by-default" (the Assignments page copy) is equivalent to no record; new empty saves go through delete-on-empty and no longer produce fossils.

Regression: `TestLoadAssignmentsScrubsNullFossilValues` (api package), `TestLoadAssignments_ScrubsNullFossilKeyValues` (logic package) — `{"g1|dev": null, "g2|prod": [...]}` reads back with the null key dropped and the normal key kept; both cases red before the fix (proven by stashing the fix and re-running), green after.

Lesson: for direct-to-file storage, the "empty state" representation needs its semantics decided first (empty array vs delete-the-key); a volume persisting across deployments turns shapes written by old versions into fossil inputs for new versions.

## BUG-036 The README's Top Logo Block Vanished: bf76dad's Repo-Wide Prettier Reformat Swallowed the Standalone HTML Block

Severity: low (presentation surface, no functional impact)

Symptom (user report, 2026-10-02): the centered logo at the top of the GitHub README disappeared. It was present at 43df26e (the `<p align="center"><img .../></p>` block above the h1), and vanished after bf76dad's "prettier repo-wide format" — prettier's normalization of standalone HTML blocks in markdown swallowed the block, and a repo-wide diff confirmed only this one block was lost (zero other differences).

Root cause: two factors compounding — ① prettier reorders/discards standalone HTML blocks in markdown; ② the old path `docs/.vitepress/public/logo.png` was itself stale (the logo now lives at `docs/public/logo.png` and `web/public/logo.png`; the docs site's public directory migrated long ago).

Fix: restore the centered logo block at the top of the README (path updated to the current `docs/public/logo.png`, a repo-root-relative path GitHub renders directly); `.prettierignore` gains `README.md` against recurrence (the same family of "prettier breaks a presentation/contract surface" lesson as `sdks/js/examples/game_demo.ts`).

Regression: no code path to test — the render-surface regression rests on a full `diff 43df26e:README.md` comparison (this instance verified only the logo block differed) + `.prettierignore` holding off future reformats.

Lesson: before any repo-wide prettier reformat, files that are "HTML block + machine/platform-parsed surfaces" must go into `.prettierignore` first (the second incident of this family; the first was the sdks/js game_demo.ts contract fixture).

## BUG-037 The v0.1.5 Release Binary's --version Always Says 0.1.4: The VERSION File Lagged the Tag, and make build Injected a Stale Value

Severity: low (a misreported self-version, no functional impact; but it interferes with install-script upgrade checks and user triage)

Symptom (verified 2026-10-02 by walking the one-click install script): after `curl`-installing the v0.1.5 release's linux-amd64 artifact, `croupier-agent --version` prints `0.1.4` (the Windows artifact under the same tag correctly prints `v0.1.5`).

Root cause: release.yml/nightly.yml's Linux/macOS builds go through `make build`, whose LDFLAGS inject the `$(shell cat VERSION)` file value; if cutting the release tag forgets to bump the VERSION file (`git show v0.1.5:VERSION` is in fact `0.1.4`), the linux/darwin binaries self-report one version behind. The Windows build independently injects `git describe --tags --always --dirty` and is therefore correct — the same tag self-reporting inconsistently across platforms is the direct evidence.

Fix: in both workflows, after the Linux/macOS `make build` (keeping the worker/ingest/tools artifacts), server/agent are re-injected and recompiled with git describe (nightly uses nightly-YYYYMMDD), unifying with the Windows build's approach; the release-surface version no longer depends on the VERSION file's moment (local `make build` still uses the VERSION file — the dev semantics unchanged).

Regression: `git show v0.1.5:VERSION` as evidence (the pre-fix artifact) + the next release's artifacts' `--version` should equal the tag name; nightly artifacts on all platforms should consistently read nightly-YYYYMMDD.

## BUG-038 The Python SDK's Synchronous Invoke Swallows Handler Exceptions Without Responding: The Caller Sees a 15s Timeout Mirage

Severity: medium (validation errors amplified into 15s timeouts; the async task path is unaffected)

Symptom (verified 2026-10-02 via screenshot walkthrough, quickstart stack + sdk-demo-python): on the console's function-execution page, submitting a payload missing `templateId` to the python demo's registered `inventory.grant` reports "调用失败：context deadline exceeded" after 15s; the execution record shows status=failed, duration 15008ms. The same validation error on the Go SDK demo returns an error payload immediately. Under the control condition (fields complete), the python demo responds normally (33ms).

Root cause: `sdks/python/croupier/transport/tcp.py:385-391` `_process_inbound`'s `except Exception` branch only does `LOG.error("Handler error for %s: %s", ...)` and **sends no response frame back** — the demo handler's `ValueError("playerId and templateId are required")` is swallowed, and the caller can only wait out its own deadline (15s default). The Go SDK's same path (`sdks/go/pkg/croupier/tcp_manager.go`, the invoke handler) `return nil, err`s and the framework wraps the error into an error-response frame (validation failures reach the caller immediately). Python's async start_task path has TaskEvent error reporting; only the synchronous invoke path is affected.

Fix: two layers aligned to the Go semantics — `CroupierClient._handle_inbound_invoke` catches handler/validation exceptions and replies with an `InvokeResponse{"error":...}` payload frame (the semantic layer, properly typed); the transport's `_process_inbound` except-fallback replies with an empty-body frame guaranteeing "even a failure must answer the Agent" (escaped exceptions such as request-deserialization failures no longer hang the peer). The e2e contract case was rewritten in sync (the old assertion "error swallowed, no reply" was the lesion itself).

Related observation (originally "pending verification, not chartered") has been verified as a general defect and chartered as **BUG-039**, fixed in the same batch — see the next section.

Regression (should be green after the fix): submitting an illegal payload to a python demo function, the synchronous invoke should return an error response within seconds (not a 15s timeout); execution records should no longer show validation-class failures at the 15008ms scale.

## BUG-039 Schema Forms Let Required String Fields Submit Empty Strings (required Only Checks Key Presence; Clearing Then Submitting Reaches the Wire)

**Symptom**: on the function-execution page, submitting `inventory.grant` — `* Template Id` carries the required asterisk, but clearing after typing (or an initial empty string) still submits directly — the empty-string payload goes out to the game side before `playerId and templateId are required` is reported; compounded with BUG-038 it presents as the 15s timeout mirage.

**Root cause**: `SchemaFormRenderer`'s validation source of truth is JSON Schema + AJV, and AJV's `required` only asserts key presence — `""` is a legal string value. The demo descriptor's `templateId` shape is `{"type":"string"}` + `required` with no `minLength`; the required asterisk (rjsf renders it from schema.required) stops "not filled" but not "empty string". The blast radius is every Schema form's required string fields (the Invoke page, page rendering, and the editor preview all share the renderer), not this field specifically.

**Fix**: `deriveRuntimeSchema`, at the required final state (after visibleWhen hidden-field exemptions are stripped), adds `minLength: 1` to fields that are required, `type:string`, and have no explicit `minLength` — "required = non-empty" matches user expectation (the same semantics as antd's required rule). Explicit `minLength` (including `validationRules` min-mapping output) is never overridden; non-string requireds (number/array/object) are untouched. Error localization specializes `minLength limit=1` into "{title} 不能为空" (otherwise it reads "must have at least 1 character"). The derived schema exists only in memory (an ui-generation constraint); no wire/PageSpec change, no migration.

**Regression**: field-validity 4 cases (derivation injected from schema/fields dual sources; an explicit minLength never overridden; empty-string submission intercepted at onFinish, never reaching the wire; hidden exemptions not injected — the first two red before the submission-interception fix) + index localization 1 case (the limit=1 specialized copy) + the upload suite's required empty-string contract rewritten (the old assertion "required preserves an empty-string submission" was the lesion itself). The renderer semantics doc syncs `docs/architecture/pagespec-protocol.md` "renderer effective semantics". Live re-verification (deploy run 36994211146, gitCommit 0ab4878): the dashboard bundle contains the specialized-copy literals ("不能为空" ×8 / "cannot be empty" ×9), with the new derivation logic live via the image; the interception behavior itself is jest-covered as a front/back contract, with no manual browser re-check (honest boundary).

## Summary

| BUG | Location                                                                                                        | Status              | Regression test                                                                                                                                                                        |
| --- | --------------------------------------------------------------------------------------------------------------- | ------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 001 | `internal/server` tests grab `:19090`                                                                           | Fixed               | 3 Go cases                                                                                                                                                                             |
| 002 | Bootstrap admin profile fields dropped                                                                          | Fixed               | 5 Go cases                                                                                                                                                                             |
| 003 | LB ownership-ratio gauge never displays                                                                         | Fixed               | 7 jest cases                                                                                                                                                                           |
| 004 | Order label bypasses intl values                                                                                | Fixed               | 1 jest case (discriminating power verified)                                                                                                                                            |
| 005 | 142 antd 6 deprecated props                                                                                     | Fixed               | 4 jest cases (discriminating power verified)                                                                                                                                           |
| 006 | Composite page editor missing a locale key                                                                      | Fixed               | Covered by the `consoleMenu`-related cases                                                                                                                                             |
| 007 | Empty avatar string triggers extra requests                                                                     | Fixed               | 5 jest cases                                                                                                                                                                           |
| 008 | Form instance unconnected                                                                                       | Fixed               | 4 jest cases (discriminating power verified)                                                                                                                                           |
| 009 | `Space.Compact` migration slows the hot-path editor                                                             | Fixed               | 1 jest case (discriminating power verified)                                                                                                                                            |
| 010 | `app.cancel` unregistered locale key                                                                            | Fixed               | Held by the console-audit guard                                                                                                                                                        |
| 011 | Both audit-log pages' rowKey all identical                                                                      | Fixed               | 8 jest cases (discriminating power verified)                                                                                                                                           |
| 012 | Avatar mutual wipes / dead links / 404 / header always placeholder                                              | Fixed               | 2 Go suites + 11 jest cases + a measured full chain                                                                                                                                    |
| 013 | MFA enrollment unusable + no recovery-code fallback                                                             | Fixed               | RFC vectors + 14 Go cases + 8 jest cases                                                                                                                                               |
| 014 | Health-score decay test depends on the wall clock                                                               | Fixed               | Rewritten to polling + a discrete invariant                                                                                                                                            |
| 015 | antd 6 deprecates the `List` component (component-level guard blind spot)                                       | Fixed               | 8 jest cases + the guard's new cases + walkthrough re-verification                                                                                                                     |
| 016 | Security center "login notification" fake switch (fake state + fake interaction)                                | Fixed               | 10 Go cases + 14 jest cases                                                                                                                                                            |
| 017 | Feishu secret missed secretKeys; masked round-trip overwrites the real value                                    | Fixed               | The `IsSecretKey` assertion (red before the fix)                                                                                                                                       |
| 018 | Game access permissions always empty (admin sees a blank)                                                       | Fixed               | 2 Go cases + 5 jest cases                                                                                                                                                              |
| 019 | Permissions overview is fabricated data (resource="role" / role names mixed in)                                 | Fixed               | 21 Go cases (including 4 rewritten cases that had fossilized the old wrong behavior)                                                                                                   |
| 020 | Permissions overview flat, no granted/not-granted states                                                        | Fixed               | 23 jest cases + mutation check turning 7 red                                                                                                                                           |
| 021 | Profile center carries the broadcast entry + announcements have no entry                                        | Fixed               | 23 jest cases                                                                                                                                                                          |
| 022 | Login page has no antd App context; all login toasts lost                                                       | Fixed               | 4 wiring-guard cases + 3 antdApp cases + Playwright re-verification on three paths                                                                                                     |
| 023 | Announcements page missing 14 bilingual entries + the menu key                                                  | Fixed               | 5 locale-coverage guard cases (stash mutation turns 4/5 red)                                                                                                                           |
| 024 | Approval actions never write the audit chain + notifications name the requester as approver                     | Fixed               | 3 Go cases (audit_chain_test, including the inverse "never under the requester" assertion) + live-stack re-verification                                                                |
| 025 | Approver jump falls back to the requester (approver missing)                                                    | Fixed               | 1 jest case (pre-fix actor=requester turns red)                                                                                                                                        |
| 026 | In-site message "send" has no permission check (anyone can message anyone)                                      | Fixed               | 1 Go case (unauthenticated/ops 403, admin 200, receiving side unaffected)                                                                                                              |
| 027 | Ticket detail Descriptions span overflow spams warnings                                                         | Fixed               | 1 jest case (red before / green after, spying console.error)                                                                                                                           |
| 028 | Bootstrap admin deletable + no disable/unban entries                                                            | Fixed               | 4 Go cases (including the guard mutation turning red) + 3 jest cases                                                                                                                   |
| 029 | Instance metadata lost end-to-end (SDK handoff chain + the agent TCP path, two leaks)                           | Fixed               | 3 Go SDK cases (including frame-level) + 2 agent cases (real TCP connections)                                                                                                          |
| 030 | resource-catalog category/search filters silently dead (query binding matches field names exactly)              | Fixed               | 3 requestbind cases + 1 handler filter-narrowing case (all red before the fix)                                                                                                         |
| 031 | functions/pages proposal version flooding (jsonb key order + the four-pathogen timestamp family)                | Fixed + closed live | The jsonb regression + 3 StableContentDigest/UpsertSemantics cases + 2 Provenance cases (all red before the fix); live: zero bump across agent restarts                                |
| 032 | functions/assignments never gates execution (data exists, the gate does not)                                    | Fixed               | 5 gate unit cases + 4 invoke/task integration cases (integration red before the fix, proven by stashing the call sites) + 2 jest cases                                                 |
| 033 | Component-template preview renders no mock data (generation fine, consumer misses/misreads)                     | Fixed               | 4 jest cases (built-in shape / unregistered / non-items field name / nested object columns; red before the fix, proven by stashing the consumer fix)                                   |
| 034 | Mock e2e cancel button timeout (AntdApp mounted at the wrong layer; the holder misses zh_CN)                    | Fixed               | The jest guard's new rootContainer prohibition + e2e local red→green (static build, the same CI path)                                                                                  |
| 035 | Assignments allowlist fossil null values leak to the frontend (legacy empty-save artifact, no read-side guard)  | Fixed               | 1 read regression each in the api/logic packages (pre-fix red proven by stashing and re-running)                                                                                       |
| 036 | The README's top logo block vanished (prettier's repo-wide reformat swallowed the standalone HTML block)        | Fixed               | A full diff 43df26e comparison confirming only the logo block differed; restored + README.md into .prettierignore against recurrence (family incident #2; #1 was sdks/js game_demo.ts) |
| 037 | The v0.1.5 release binary's --version always 0.1.4 (the VERSION file lagged; make build injected a stale value) | Fixed               | `git show v0.1.5:VERSION` as evidence + release/nightly workflows' Linux/Mac switched to git describe/nightly re-injection (unified with Windows)                                      |
| 038 | The Python SDK's synchronous invoke swallows handler exceptions without responding (a 15s timeout mirage)       | Fixed               | 5 pytest cases (3 red before the fix + 2 success-path controls) + the e2e contract rewritten; 546 passed in full                                                                       |
| 039 | Schema forms let required string fields submit empty strings (AJV required only checks key presence)            | Fixed               | 4 field-validity cases (2 red before the fix) + 1 localization specialization + the upload suite's required empty-string contract rewritten                                            |

### Leftovers / Unfixed

- **The 9 structural rewrites of `Input.addonBefore`** visually swap `addonBefore`'s built-in styling for `Space.Compact` (6 places) or `Input.prefix` (3 places). `prefix` moves the label from the input's **outside** to its **inside**; these spots' label positions differ from before the rewrite. Component-level unit tests lock the interaction and values; the pixel-level differences have had no visual regression pass.
- ~~**BUG-002's legacy data**~~ (verified closed): after rebuilding the binary and restarting the server, `seedBootstrapAdmins` backfills automatically, and `GET /api/v1/profile` for `admin` returns `nickname: "系统管理员"`, `email: "admin@croupier.local"`.
- **`Space.Compact`'s width behavior**: `Space.Compact` sizes to its content; the widths of `Analytics/Behavior`'s `PathControls` and `OpenAPISources`' two modals may differ subtly from before the rewrite. Component-level unit tests lock the interaction and values; pixel-level differences have had no visual regression pass.
- **`@ant-design/charts`' `Line`** is still used in `LBMonitor`; this round handled only `Gauge`. `Line` showed no value-loss problem (`xField/yField/colorField` go through plots' normal adaptor path) and was left untouched.
- **The audit's 52 routes** were assembled from two leads — "components carrying deprecated props" + "route groups never covered before" (the remaining `/admin/*`, `/dev/*`, `/support/*`, `/system/*`, `/ops/*` paths in `config/routes.ts`, etc.) — and are still not an exhaustive enumeration; dynamic routes (e.g. `/functions/:id`, `/support/tickets/:id`, `/console/:categoryKey`) need concrete ids and were not included. The guard case (BUG-005 case 2) is a **full static scan** of `src/**/*.tsx` and does not depend on the route list; the console audit is complementary runtime evidence.
