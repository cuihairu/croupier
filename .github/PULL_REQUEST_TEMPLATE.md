## Summary

<!-- What does this PR change, and why? One or two sentences. -->

## Changes

<!-- Bullet list of concrete changes (files/modules touched). -->

## Verification

- [ ] `go build ./...` and `gofmt -l .` (empty output) pass
- [ ] `go test ./internal/... ./core/... ./agents/... ./pkg/... ./cmd/...` green; new tests for new behavior
- [ ] Web changes: `pnpm --dir web run tsc` 0 errors + `pnpm --dir web test` green
- [ ] Docs build: `cd docs && CI=true pnpm build` (docs touched)

## Contract Checklist (tick or state N/A)

- [ ] Model changes ship the numbered migration in this PR (goose chain + `MinimumRequiredVersion` + migration tests)
- [ ] API responses follow the unified contract (no ad-hoc envelopes)
- [ ] Localized text goes through `normalizeLocalizedText` / `localizedText` (no inline locale chains)
- [ ] Contract/SDK fields are `lowerCamelCase`; docs EN + `docs/zh/**` updated
- [ ] Config keys are canonical `lowerCamelCase`; examples updated if config changed
- [ ] Known boundaries stated honestly in the description and docs (no silent gaps)
