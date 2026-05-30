# Assignments Page Quick Map

If you want to understand this page quickly, read in this order:

1. `index.tsx`
   - Only composes the page container + renderer + modals.
2. `useAssignmentsPage.ts`
   - All state, data loading, mutations, and action handlers.
3. `pageSchema.ts`
   - All page-level UI configuration:
     - columns
     - toolbar actions
     - row actions
     - category actions
     - tabs and stats
     - action flags (`disabledWhen` / `loadingWhen`)
4. `PageRenderer.tsx`
   - Turns schema + context into concrete UI blocks.
5. `columns.tsx`
   - Column-level render strategy keyed by schema.
6. `schemas.ts`
   - Formily schemas for Canary/Clone modals.

## Rule of thumb

- Change copy/layout/action visibility first in `pageSchema.ts`.
- Change API/state behavior in `useAssignmentsPage.ts`.
- Avoid adding heavy logic back into `index.tsx`.
