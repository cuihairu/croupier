# Dashboard Bootstrap (Contracts + Adapter Skeleton)

This guide bootstraps extension contract outputs and adapter skeleton files into a dashboard repo.

## Input

- OpenAPI: `docs/contracts/extensions-openapi-v1.yaml`
- Templates: `docs/contracts/templates/dashboard/**`

## Output in dashboard repo

- `src/services/contracts/extensions.ts` (generated)
- `src/services/generated/extensions-client/**` (generated)
- `src/services/errors/codes.ts` (template)
- `src/services/errors/mapper.ts` (template)
- `src/services/adapters/extensions.ts` (template)
- `src/services/api/extensions.ts` (template)
- `.github/workflows/extensions-regression.yml` (template)

## Linux/macOS

```bash
bash scripts/contracts/bootstrap-dashboard-contracts.sh /path/to/dashboard
```

## Windows PowerShell

```powershell
powershell -File scripts/contracts/bootstrap-dashboard-contracts.ps1 -DashboardRoot C:\path\to\dashboard
```

## Notes

1. Existing files are not overwritten by default.
2. Add `-Force` / `--force` to overwrite template files.
3. Generated files will always be regenerated.
