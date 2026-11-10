# Contributing to Croupier

Thanks for your interest in contributing! This document covers the build
workflow, coding standards, and review gates for this repository.

## Build and Test

```bash
make proto        # Generate protobuf code (local protoc; buf lint only)
make build        # Build server / agent binaries to ./bin
make test         # Unit tests with race detection
go test ./internal/... ./core/... ./agents/... ./pkg/... ./cmd/...   # CI test scope
gofmt -w .        # Format all Go files before committing
```

Frontend (dashboard): `pnpm --dir web run tsc` must pass with 0 errors and
`pnpm --dir web test` must be green. Documentation site: `cd docs && pnpm build`.

Integration examples live in `examples/`; config templates in `configs/`.

## Commit Messages

- English only, short and concrete: `feat: add provider handshake auth`,
  `fix: null check on login`.
- No vague one-word summaries for behavior changes ("优化展示"-style messages
  are rejected); say what actually changed.
- Large refactors should be split so each commit builds and tests green.

## Pull Requests

Before requesting review, make sure:

1. `go build ./...`, `gofmt -l .` (empty output), and the test scope above are
   green; new features come with new test cases.
2. TypeScript changes: no `any` types; `pnpm --dir web run tsc` and the web
   test suite pass.
3. Model changes (`internal/model/`) ship their numbered migration in the same
   PR — see the Database Schema Migration Contract in `CLAUDE.md`.
4. Localized text flows through `normalizeLocalizedText` and
   `localizedText(value, locale, fallback)`; no ad-hoc `value['zh-CN']`
   chains in components.
5. HTTP handlers follow the unified API response contract (success payloads
   unwrapped; errors use `{ error, message, details }`).
6. Contract/SDK field names are `lowerCamelCase` at every JSON/SDK surface.
7. Docs sync: model changes update `docs/architecture/dashboard-page-model.md`,
   wire changes update `docs/architecture/pagespec-protocol.md` /
   `sdk-wire-protocol.md`, user-facing behavior updates the matching
   operations page — English and the `docs/zh/**` mirror together.

A reviewer (see `.github/CODEOWNERS`) must approve before merge. CI must be
green; environment-flake reds are re-run, not waived silently.

## Code Style

- Go: gofmt-formatted; package import boundaries documented in `CLAUDE.md`
  must hold (e.g. `internal/platform/registry` must not import
  `internal/model`).
- TypeScript/React: strict typing only — `unknown` plus type guards instead of
  `any`; typed library imports instead of loose signatures.
- YAML config keys are `lowerCamelCase` everywhere (see the Configuration
  Contract in `CLAUDE.md`).
- Proto changes go through `make proto`; generated `pkg/pb` code is committed.

## Reporting Issues

- Bugs: use the bug report template; include repro steps, config snippets
  (secrets redacted), and logs.
- Security vulnerabilities: do **not** open a public issue — follow
  [SECURITY.md](SECURITY.md).

## Licensing

By contributing, you agree that your contributions are licensed under the
Apache License 2.0 that covers this repository.
