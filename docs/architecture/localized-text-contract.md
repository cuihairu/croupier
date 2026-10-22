# The LocalizedText contract

Status: mandatory (enforced in CI by `scripts/localized_text_guard.sh`)

## Background

Localized fields once existed in three shapes at the same time: the BCP47 keys sent down by the backend (`{ "zh-CN": ..., "en-US": ... }`), short keys invented by two frontend services (`{ zh, en }`), and bare strings. Eight-plus copy-pasted value-extraction chains lived in components with mutually contradictory fallback orders, which ended with a calling page crashing as it rendered a `{en-US, zh-CN}` object directly as a React child. This contract ends that drift.

## The contract

1. **One definition**: `spec.LocalizedText = map[BCP47-locale]string` (Go,
   `internal/dashboard/spec/types.go`) and the `LocalizedText` type in
   `web/src/types/dashboard.ts`. Keys must be valid BCP47 locales (`"zh-CN"` is
   the system default language and first in the render fallback chain, `"en-US"`
   is the second fallback; other BCP47 keys such as `"ja-JP"` are valid).
   No module may declare a second localized-text type or invent short keys
   (`zh` / `en` / `zh_cn`).

2. **One normalization layer**: service boundaries go through
   `normalizeLocalizedText` (`web/src/services/api/functions-enhanced.ts`).
   Every input shape (BCP47, legacy short keys, bare strings) is normalized to
   BCP47 keys and then **passed through as-is**; a dual-key output is not
   forced (D7): a bare string normalizes to a single system-default `"zh-CN"`
   key; other BCP47 keys (e.g. `"ja-JP"`) are preserved as-is and never dropped.
   Legacy short keys may only be read as a fallback inside that one function
   (mapped to the canonical key without overwriting existing values); producing
   them at any output boundary is not allowed.

3. **One render path**: components must call
   `localizedText(value, locale, fallback)` from
   `web/src/utils/localizedText.ts`.
   Inlining value-extraction chains such as `value['zh-CN'] || ...` or
   `value?.zh || value?.en` inside components is forbidden.

## Enforcement

`scripts/localized_text_guard.sh` runs in CI (CI - Dashboard / CI - Core); any hit turns the build red:

- redefining the `LocalizedText` type/interface (`import type` re-exports are fine)
- inline locale value-extraction chains inside components (the contract util and the normalization layer itself are exempt)
- fabricating short keys at service outputs

## Review checklist

Localized fields on new APIs/DTOs are checked against the three rules above. A second LocalizedText definition, a value-extraction chain inside a component, or a new normalize implementation all count as review failures.
