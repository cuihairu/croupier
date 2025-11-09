# Repository Guidelines

## Project Structure & Module Organization
- Go monorepo: `cmd/{server,agent,edge,analytics-worker}` (binaries), core in `internal/`, shared libs in `pkg/`.
- Frontend UI: `web/` (Umi Max + Ant Design 5).
- Configs & assets: `configs/` (RBAC, users, dev certs), `packs/`, `scripts/`, `docs/`, ephemeral data in `data/`.
- Protocol/IDL: `proto/` (+ `buf.gen.yaml`), generated code under `gen/`.

## Build, Test, and Development Commands
- Build all: `make dev` (proto + binaries to `bin/`). Targets: `make server`/`agent`/`edge`.
- Backend tests: `make test` (Go), lint: `make lint`.
- Web dev: `cd web && npm run start` (or `pnpm start`), build: `npm run build`.
- Docker Compose quickstart:
  - Core DBs only: `docker compose up -d redis clickhouse`.
  - Full stack (server/edge/web + DBs): `docker compose up -d`.
  - Optional profiles: tools (`--profile tools`), stream (`--profile stream`).

## Coding Style & Naming Conventions
- Go: `gofmt`/`goimports`; packages lowercase; exported ids `CamelCase`; use `context.Context` first; structured logs.
- TypeScript/React: Prettier + ESLint; 2‑space indent; components `PascalCase`; hooks `useX`; pages live in `web/src/pages/*/index.tsx`.
- Commits: Conventional Commits (`feat(scope): ...`, `fix`, `chore`, `docs`).

## Testing Guidelines
- Go unit tests co‑locate as `*_test.go`; prefer table‑driven tests; cover auth/routing/validation.
- Frontend: `cd web && npm run test` or `npm run test:coverage`.
- Aim for meaningful assertions and stable snapshots; add tests when touching RBAC, APIs, or analytics logic.

## Commit & Pull Request Guidelines
- PR must include: what/why, test plan, screenshots for UI, and docs/config updates if applicable.
- When adding APIs/permissions, update `configs/{permissions,roles,rbac}.json` and note any migrations.
- Keep diffs small and focused; link issues; ensure `make lint && make test && npm -C web run build` pass.

## Security & Configuration Tips
- Dev mTLS certs in `configs/dev/`; env via flags or YAML; secrets through env vars.
- Example run: `docker compose up -d redis clickhouse && bin/server --http_addr :8080 ...`.
