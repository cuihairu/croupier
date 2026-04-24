# Functions Directory Quick Map

Read this page in this order:

1. `index.tsx`
   - Page composition only (table + drawer layout).
2. `useDirectoryPage.ts`
   - Data loading, table actions, drawer action wiring.
3. `schema.ts`
   - Header actions, drawer actions, row actions, columns metadata.
   - Supports action flags (`disabledWhen` / `loadingWhen`).
4. `columns.tsx`
   - Column render logic mapped by schema keys.
5. `types.ts`
   - Summary/detail row types for table and drawer.

Rule:

- Change copy, actions, and visibility in `schema.ts` first.
- Keep API/state behavior in `useDirectoryPage.ts`.
- Keep `index.tsx` as a thin composition layer.
- Use action flags in schema:
  - `disabledWhen`: disable by context flags (e.g. `noSelection`, `loading`)
  - `loadingWhen`: show action loading state by context flags
