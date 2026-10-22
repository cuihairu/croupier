# New Business Admission Rules (Extension-First)

Updated: 2026-03-15  
Status: In effect

## The Boundary in One Sentence

The core only provides the stable foundation (auth, permissions, audit, function and extension runtimes); new business must land as extension installs by default.

## Admission Rules

1. New business APIs go into extension domains by default, not directly into the core API.
2. New business pages are provided by default through extension page bindings (`page/ui/navigation` binding).
3. New business configuration goes into installation `config_json/secret_refs_json` by default, without adding a YAML main path.
4. New business capabilities are exposed by default through capability / operation, without exposing underlying implementation details.
5. If something truly must enter the core, all of the following must hold:
   - It is a foundational capability every extension would reuse;
   - It can be governed uniformly by permissions and audit;
   - It has a clearly stable interface and is not bound to specific business semantics.

## Review Checklist (required on PRs)

- Could this requirement be implemented as an extension? If not, what is the reason?
- Was a new YAML main-config entry added? If so, is it migration-compatibility only, with a decommission plan?
- Were the capability / page / function binding designs completed?
- Were the install, start/stop, upgrade, and event-observability paths completed?

## Prohibitions

- Piling further heavy business modules into the `internal/api/*` core groups.
- Adding "only this module can use it" core-coupled code.
- Bypassing the extension-install model by hard-coding runtime configuration paths.
