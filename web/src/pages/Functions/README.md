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
  - JSON/函数表单/路由配置子页签集中管理。
- UI sections: `DetailSections.tsx`
  - Basic info tab / permissions tab / JSON viewer.
- Async tab blocks: `DetailTabs.tsx`
  - History / Analytics / Warnings are isolated here.
- Function Form sub-features:
  - `config` tab includes JSON / Function Form / Route sub-tabs.
  - Function Form sub-tab is rendered by `@/components/FunctionFormManager`.

Rule:

- Change table/action copy in schema files first.
- Keep data-fetch side effects in hook/tab modules.
- Keep page entry focused on composition and route state sync.
