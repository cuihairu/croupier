---
title: 仓库规范
---

# Repository Guidelines

## Project Structure & Module Organization

- Go monorepo: binaries in `cmd/`, core implementation in `internal/`, stable exported helpers in `pkg/`.
- Frontend UI: `dashboard/`.
- Configs & assets: `configs/`, `descriptors/`, `schemas/`, `scripts/`, `docs/`, runtime data in `data/`.
- Protocol/IDL: `proto/`, generated stubs in `pkg/pb`.

## Build, Test, and Development Commands

- Build all server-side binaries: `make build`
- Build a specific binary: `make server`, `make agent`, `make worker`, `make ingest`
- Generate protobuf code: `make proto`
- Run tests: `make test`
- Build docs: `cd docs && pnpm install && pnpm run build`
- Build dashboard: `cd dashboard && npm ci && npm run build`

## Coding Style & Naming Conventions

- Go: `gofmt`/`goimports`; packages lowercase; exported ids `CamelCase`; use `context.Context` first; structured logs.
- New entrypoints go under `cmd/`; do not reintroduce `services/*` layout or stale document references.
- TypeScript/React: Prettier + ESLint; 2-space indent; components `PascalCase`; hooks `useX`.
- Commits: Conventional Commits (`feat(scope): ...`, `fix`, `chore`, `docs`).

## Testing Guidelines

- Go unit tests co-locate as `*_test.go`; prefer table-driven tests.
- Frontend: `cd dashboard && npm run test` or `npm run test:coverage`.
- Add tests when touching RBAC, APIs, routing, analytics processing, or descriptor resolution.

## Commit & Pull Request Guidelines

- PR should include: what changed, why, and how it was verified.
- When adding APIs or permissions, update the matching files under `configs/`.
- Keep diffs focused; ensure `make test` passes before merge.

## Security & Configuration Tips

- Secrets go through environment variables, not hardcoded YAML.
- Example local run: `./bin/croupier-server --config configs/server.yaml`.
