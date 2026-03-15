# Extensions Contract Codegen

This guide generates TypeScript types and a fetch client from:

- `docs/contracts/extensions-openapi-v1.yaml`

## Prerequisites

- Node.js 18+
- `npx` available

## Generate TypeScript types

Linux/macOS:

```bash
bash scripts/contracts/gen-extensions-ts-types.sh dashboard/src/services/contracts/extensions.ts
```

Windows PowerShell:

```powershell
powershell -File scripts/contracts/gen-extensions-ts-types.ps1 -OutFile dashboard/src/services/contracts/extensions.ts
```

## Generate TypeScript client

Linux/macOS:

```bash
bash scripts/contracts/gen-extensions-ts-client.sh dashboard/src/services/generated/extensions-client
```

Windows PowerShell:

```powershell
powershell -File scripts/contracts/gen-extensions-ts-client.ps1 -OutDir dashboard/src/services/generated/extensions-client
```

## Recommended integration

1. Keep generated code under `src/services/contracts` and `src/services/generated`.
2. Wrap generated client in `src/services/api/extensions.ts`.
3. Map DTOs to ViewModel in `src/services/adapters/extensions.ts`.
4. Do not use generated DTO fields directly in pages.

## Bootstrap command

If you want generated files + adapter skeleton together:

- See `docs/contracts/bootstrap-dashboard.md`.
