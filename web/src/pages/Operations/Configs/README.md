# Operations Configs Quick Map

Read this page in this order:

1. `index.tsx`
   - Pure UI composition (filters, table, edit/version/diff modals).
2. `useConfigsPage.ts`
   - All query state, loading, validation/save/version/rollback handlers.
3. `schema.ts`
   - Toolbar filters/actions + format options + table columns metadata.
4. `diff.tsx`
   - Diff helper view and format-to-language resolver.

Rule:

- Keep CRUD and version flow in `useConfigsPage.ts`.
- Keep modal layout and table columns in `index.tsx`.
- Reuse `langOf` for editor language mapping to avoid duplicated conditionals.
