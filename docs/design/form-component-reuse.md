---
title: Generated Form Component Reuse — Contract Baselines and RJSF Migration Evaluation
---

# Generated Form Component Reuse — Contract Baselines and RJSF Migration Evaluation

Date: 2026-10-27. Status: **evaluation record / first slice of D1 component reuse** (the parent task is the interface-generation chain closure; `todo.md` parent line stays unchecked and records this slice as evidence).

## 1. Scope and Mandate

The interface-generation chain is to reuse Ant Design/ProComponents' table, form, detail, dialog and status components, and keep custom work focused on business semantics, function binding and execution governance. This slice covers the **form reuse** part of that mandate (D1):

1. **Contract test baselines** for the ProComponents form containers `ModalForm`, `DrawerForm` and `StepsForm`. These are the reuse targets for generated forms; the baselines lock their render, data and validation contract so a later migration or a ProComponents upgrade cannot silently break existing generated forms.
2. **First-slice evaluation** of migrating the current JSON-Schema (RJSF) generated forms to those containers. The RJSF surface is large; per the mandate ("if the surface is too large, record the evaluation and do not hard-migrate"), this slice only records the assessment and does not migrate the existing forms.

No existing page/renderer behavior is changed in this slice.

## 2. Current Generated Form Implementation

Generated business pages render forms through `web/src/components/SchemaFormRenderer` (an `@rjsf/antd` wrapper). The contract is `FormPresentationSpec` (`jsonSchema` + `layout` + presentation fields); the RJSF `uiSchema` is derived in memory, never persisted.

Current consumers of the RJSF generated forms:

- `web/src/components/PageRenderer/ResourcePageRenderer.tsx` — create, edit and row-action forms inside an Ant Design `Modal` (`SchemaFormRenderer` with `hideSubmit`; the Modal's `onOk` validates and reads values via the renderer ref).
- `web/src/components/PageRenderer/{CompositeRenderer,OperationPageRenderer,ReportPageRenderer,TaskPageRenderer}.tsx` — section/detail/operation forms.
- `web/src/pages/PageStudio/CompositeEditor/PreviewNode.tsx` — the `ModalForm` wrapper (a local component that reuses `SchemaFormRenderer`, not the ProComponents `ModalForm`) and `StaticFormLive`.
- `web/src/pages/Assignments/CloneModal.tsx`, `web/src/pages/Functions/Invoke/RequestBodyEditor.tsx`, `web/src/pages/PageStudio/CompositeEditor/PropsPanel.tsx` — miscellaneous schema forms.

The concrete template examples are `web/src/components/PageRenderer/templates/*.tsx` `createForm`/`updateForm` and per-action `form` (e.g. `banManagement`, `mailCompensation`, `playerManage`).

## 3. Reuse Targets and Contract Baselines

`ModalForm`, `DrawerForm` and `StepsForm` are `@ant-design/pro-components` form containers. They combine the container (modal/drawer/step) with a `ProForm` submitter and an `onFinish` value contract.

New contract tests: `web/src/components/FormContract/__tests__/formContainers.contract.test.tsx` plus snapshots lock:

- **Render contract** — container title, declared fields, and submit/cancel buttons are present (snapshot per container).
- **Data contract** — `onFinish` receives the form values; `ModalForm` closes after a successful finish (`onOpenChange(false)`); `StepsForm` preserves earlier-step values across steps and submits the merged values on the last step.
- **Validation contract** — a required field left empty blocks submit and does not call `onFinish`.

These are the first jest snapshots in the web package; the snapshot serializer strips the generated `css-var-_r_*` hash so a rebuild of the style token hash inside the same dependency set does not spuriously fail the regression gate while still catching real structure changes.

Gate: the file passes under `web` jest and the full `web` lint stays green.

## 4. RJSF Schema Migration Evaluation

### 4.1 Candidate Mapping

The RJSF `SchemaFormRenderer` is a self-contained `<Form>` (own state, own submitter, AJV validation, `uiSchema`-driven widgets). The ProComponents containers expect `ProForm` children whose values feed a single `onFinish`. Simple generated forms (flat object schema, `string`/`number`/`enum` fields, `required`) map cleanly:

- `Modal` + `SchemaFormRenderer(hideSubmit)` → `ModalForm` with `ProFormText`/`ProFormSelect` children.
- `Drawer` detail/update → `DrawerForm`.
- multi-step/task forms → `StepsForm` with one `StepForm` per step.

ProComponents also ships `BetaSchemaForm` (JSON schema → ProForm fields), which is the closest automated bridge. It is explicitly **beta** ("API may change").

### 4.2 Impedance and Gaps

The migration surface is larger than the happy path because the current renderer provides features that a plain `ProForm`/`BetaSchemaForm` does not cover today:

- **Self-contained submit/ref contract** — `SchemaFormRendererHandle` (`submit`/`validate`/`getValues`) is used by `Modal.onOk`/action forms. Moving to `ModalForm` moves submit ownership to the container, which changes the `ResourcePageRenderer` call sites.
- **Custom widgets** — `web/src/components/SchemaFormRenderer/widgets.tsx` and `widgets-upload.tsx` (upload, remote-option selects, presentation adapters); these are not expressible as `ProFormField` props without a widget registry.
- **Conditional visibility** (`visible-when`, `sectionCondition.ts`) and **hidden/ordered fields** derived into `uiSchema`.
- **External field errors** (`extraErrors`) from server validation, and the custom **error boundary** with retry — must be preserved on the container wrapper.
- **Localized AJV errors** and bilingual labels (`locales/*/schemaFormRenderer.ts`).
- **Read-only/disabled** states and template-driven form structures.

`BetaSchemaForm` does not cover the above today, and its beta status means a migration would pin a moving API. Nesting the RJSF form inside a `ModalForm`/`DrawerForm` would create two form instances with two submit paths, which is not a reuse win.

### 4.3 Conclusion

The RJSF schema form is a behavior-rich, self-contained runtime; the ProComponents containers are shell+submit contracts. Converting the existing generated forms now would require either duplicating the renderer features into ProForm field props or pinning a beta bridge — both are larger than this slice and risk regressing the already-green generated page behavior (5 template renderers + PageStudio preview + composite renderer + invoke editor).

**Decision**: keep the current RJSF generated forms as the runtime, establish the above contract baselines as the migration guard, and defer the hard migration to a later slice that either (a) builds a thin `FormPresentationSpec → ProForm fields` adapter for the covered subset, or (b) upgrades the bridge when `BetaSchemaForm` matures. Do not rewrite the 12+ `SchemaFormRenderer` call sites in this slice.

## 5. Evidence

- Contract tests + snapshots: `web/src/components/FormContract/__tests__/formContainers.contract.test.tsx` (ModalForm/DrawerForm/StepsForm render + data + validation contract).
- This evaluation record: `docs/design/form-component-reuse.md`.
- Commit: see `git log` for the slice.
- Verification: `web` jest on the new suite plus the full web jest + `web` lint gates.
