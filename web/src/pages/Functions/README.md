# Functions Pages Quick Map

## Directory (函数管理)

- Entry: `Directory/index.tsx`
- Logic: `Directory/useDirectoryPage.ts`
- UI schema: `Directory/schema.ts`
- Table rendering: `Directory/columns.tsx`

## Detail (函数详情)

- Entry: `Detail.tsx`
  - Owns page-level layout and tab route sync.
- Page hook: `useFunctionDetailPage.ts`
  - Owns data loading, save actions, permissions, route config mutations.
- Page schema: `detailSchema.ts`
  - Main tabs and header actions metadata for Detail page.
  - Supports `loadingWhen` / `disabledWhen` flags for action behavior.
- Config tab: `DetailConfigTab.tsx`
  - 集中展示函数契约 JSON、能力语义、路由与诊断信息。
- UI sections: `DetailSections.tsx`
  - Basic info tab / permissions tab / JSON viewer.
- Async tab blocks: `DetailTabs.tsx`
  - History / Analytics / Warnings are isolated here.
- Invocation sub-features:
  - Function invoke uses `SchemaFormRenderer` to render input JSON Schema.
  - Function detail never owns page/menu/form layout overrides.

Rule:

- Change table/action copy in schema files first.
- Keep data-fetch side effects in hook/tab modules.
- Keep page entry focused on composition and route state sync.
