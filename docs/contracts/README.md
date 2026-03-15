# API Contracts

This directory stores versioned API contract artifacts used by frontend and integration clients.

Current baseline:

- `extensions-openapi-v1.yaml`: Extension domain API baseline (catalog, installations, runtime operations).
- `codegen.md`: TypeScript type/client generation guide.
- `bootstrap-dashboard.md`: One-command bootstrap for dashboard contracts and adapter skeleton.
- `frontend-error-mapping-v1.json`: Frontend error-code mapping baseline for extension domain.
- `templates/dashboard/.github/workflows/extensions-regression.yml`: Dashboard CI gate template.

Rules:

1. Keep contracts backward compatible within the same major version.
2. Additive fields are allowed; breaking changes require a new versioned file.
3. Frontend adapters should bind to these contracts instead of raw backend response internals.
