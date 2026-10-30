# Versioned Audit Digest & Write-Failure Policy

> Implementation recheck, 2026-10-30: D1–D3 digest/compatibility changes have local evidence. D4 remains a target: AuditFunctionInvoke/AuditApprovalCreated print errors and recordApprovalAudit ignores them; caller-level 5xx/blocking is not established. ARCH-008 is reopened.

> ARCH-007 · P2 · 方案决策 · 版本化审计摘要与失败策略
>
> Scope: the audit chain in `internal/audit` — `calculateHash` / `hashBytes` /
> `ValidateChain` (audit.go), the chain columns on `audit_records`
> (`chain_hash` / `chain_prev_hash` / `chain_sequence` / `chain_signer_id` /
> `chain_signature`, store.go), the optional `AuditSigner`, and the write-failure
> path of `Log` (3 `ErrChainSequenceConflict` retries, then error). Deliberately
> out of scope: the audit record schema beyond chain fields, the UI, retention /
> export, and multi-signer quorum.
>
> Status: **DECIDED** (2026-10-29). D1–D5 settled: canonical hash field set with
> a round-trip-stable encoding, SHA-256 replacing DJB2, an explicit
> `chain_version` column (migration 0049, backfill existing rows = 1, never
> rewrite existing hashes), a strict-by-default write-failure policy with an
> explicit opt-in best-effort mode, and the external signature kept as an
> independent, separately-verified concern. Drives ARCH-008. This document
> changes no runtime behavior. Landed in ARCH-008 (2026-10-29).

## 1. Problem

The audit chain does not detect tampering on most of the record, uses a
non-cryptographic hash, and is not stable across a database round-trip. The
write-failure policy is implicit and un-decided.

Evidence:

- **Incomplete field set.** `calculateHash` (audit.go) hashes only
  `timestamp`, `event_type`, `actor_id`, `resource` (ID), `outcome`,
  `prev_hash`, `sequence`, and `details`. A tamper that flips
  `category`, `severity`, `action`, `changes`, `context`, `errorMessage`,
  `resource.gameId` / `resource.environment`, or any `actor` / `resource`
  metadata field is invisible to the chain: the record recomputes to the same
  hash.
- **Non-crypto hash.** `hashBytes` (audit.go) is DJB2 — a 32-bit
  `uint32` accumulation formatted `%08x`, trivially collidable, not a
  cryptographic digest. The code comment above it literally reads "Using SHA256
  for hashing / In production, you'd use crypto/sha256", an intent-versus-
  implementation mismatch. The existing `chain_hash varchar(64)` column already
  fits a SHA-256 hex digest (64 chars) exactly — no width change is needed.
- **Not round-trip stable.** The hash input uses `record.Timestamp.UnixNano()`
  (nanosecond precision), but postgres `timestamptz` persists at microsecond
  precision and MySQL `datetime(3)` at millisecond precision. `Log` hashes the
  full-precision time; `ValidateChain` recomputes on the DB-truncated time, so
  records with sub-column-precision timestamps report a false
  `hash_mismatch`. Separately, `Details` / `Changes` numeric values re-encode
  as `float64` after a JSON round-trip, so large `int64` values can diverge
  between the Log-time and Validate-time inputs.
- **No version field.** There is no way to evolve the field set or algorithm
  without breaking the validation of existing rows; the algorithm and field set
  are implicit in the code and cannot be told apart per row.
- **Implicit write-failure policy.** `Log` retries 3× on
  `ErrChainSequenceConflict`, then returns an error. This is fail-closed for
  every event class with no explicit decision, no per-category distinction, and
  no operator-facing signal beyond the propagated 5xx.

The external signature (`AuditSigner` / `ChainInfo.Signature` / `SignerID`,
columns `chain_signer_id` / `chain_signature`) already exists and is verified
independently in `ValidateChain`. It is a separate concern that this document
must position relative to the hash.

## 2. Decisions

### D1 — Canonical hash field set with a round-trip-stable encoding

The hash input is a fixed, version-stable set of **all** semantic fields —
never an omission-prone subset. Empty and zero values are included explicitly
(no `omitempty`), so the field set is closed: any future change to it is a
version bump (D3), not a silent drift.

Canonical input (version 2):

```
{
  "id":           record.ID,
  "timestamp":    record.Timestamp.UTC().UnixMilli(),
  "event_type":   record.EventType,
  "category":     record.Category,
  "severity":     record.Severity,
  "actor":        { id, type, name, email, roles, ipAddress, userAgent,
                    sessionId, mfaUsed, delegatedBy, metadata },
  "action":       record.Action,
  "resource":     { type, id, name, gameId, environment, metadata },
  "outcome":      record.Outcome,
  "error_message":record.ErrorMessage,
  "changes":      { before, after, diffFields } or null,
  "context":      { requestId, traceId, correlationId, service, version,
                    environment, tags },
  "details":      record.Details or {},
  "prev_hash":    record.ChainInfo.PrevHash,
  "sequence":     record.ChainInfo.Sequence
}
```

Encoding rules, chosen so the Log-time input and the Validate-time input are
byte-identical:

1. **Timestamp at millisecond precision.** `Log` truncates
   `record.Timestamp` to `time.Millisecond` before hashing (and before storing),
   so the hash sees the same value a postgres/MySQL column round-trip returns.
2. **JSON round-trip the maps.** `Details` and `Changes.before`/`.after` are
   passed through `json.Marshal` → `json.Unmarshal` once at Log time, so their
   numeric values are the DB-normalized `float64` form before hashing. This
   removes the large-`int64` divergence.
3. **Deterministic serialization.** `json.Marshal` of the map sorts keys
   alphabetically; nested maps sort the same way. No `omitempty`, no
   locale-dependent formatting, UTC-normalized time.

**Decision:** integrity means "any field change is detected", not "the 7-field
subset is chained". A record whose hash is valid is bit-identical on every
semantic field to what the signer/hasher committed.

### D2 — SHA-256 replaces DJB2

`hashBytes` switches to `crypto/sha256.Sum256` over the canonical input bytes,
formatted as lowercase hex (64 chars — exactly `chain_hash varchar(64)`).

- DJB2 is retained **only** as the legacy algorithm used to validate version-1
  rows (D3). New rows never use it.
- No column-width change; `chain_hash` stays `varchar(64)`.

### D3 — Explicit `chain_version` column

A numbered goose migration **0049** (next after the current 48) adds
`chain_version smallint NOT NULL DEFAULT 1` to `audit_records` and backfills
existing rows to `1`. New records are written with version `2`.

- `ValidateChain` dispatches on the stored version: `v1` → legacy field set +
  DJB2, `v2` → canonical field set + SHA-256.
- **Existing hashes are never rewritten.** `prev_hash` of every later row
  references the previous hash _value_; rewriting v1 values would break the
  chain. v1 rows remain verifiable with the legacy algorithm, and the chain
  continues seamlessly into v2 rows.
- Version is stored, not inferred from hash length. Length inference is
  ambiguous once two versions share a digest length, and a future algorithm may
  reuse a length; an explicit column is unambiguous and survives both.
- Migration follows the repository rules: idempotent (`HasColumn` guard before
  `AddColumn`), registered in `registerSvcMigrations`, `MinimumRequiredVersion`
  bumped 48 → 49, `migrate_test.go` synthetic list updated.

### D4 — Write-failure policy: strict by default, explicit opt-in best-effort

The default remains **fail-closed** (`strict`): after the 3 chain-conflict
retries, an audit write failure returns an error and the caller surfaces it
(e.g. HTTP 5xx). Rationale: an audit system must not silently proceed past a
trail gap; the operator must learn the trail broke. This is a required caller contract; current callers do not all comply (2026-10-30 recheck).

An explicit opt-in `best_effort` mode (config knob
`audit.writeFailurePolicy: strict | best_effort`, default `strict`) changes only
the failure branch: `Log` records the gap at WARN, fires a **rate-limited**
alert through the existing notifier, and returns the record (carrying a gap
marker in `ChainInfo`) instead of erroring. It is opt-in because silently
proceeding on an audit-gap is a security anti-pattern; an operator choosing it
accepts the surfaced, alerted gap in exchange for control-plane availability
during an audit-store outage.

**Decision:** integrity wins by default; availability is a deliberate, explicit,
alerted opt-in. There is no silent fail-open.

### D5 — External signature is an independent, optional concern

The `AuditSigner` signature is **not** folded into the hash field set and is
**not** recomputed when the hash algorithm or version changes. It signs the
same canonical payload bytes as D1 (or, if a deployment prefers, the resulting
hash — the deployer's choice, kept out of the algorithm path), and
`ValidateChain` verifies it independently, exactly as today.

- A version bump (D2/D3) therefore does not invalidate existing signatures, and
  existing signatures need no re-issue.
- The single signature column pair (`chain_signer_id` / `chain_signature`) is
  retained; multi-signer quorum is out of scope.
- Hash and signature are two independent integrity layers; neither replaces the
  other.

## 3. Impact / review table

| Aspect                                                                         | Before (current)                           | After (ARCH-007 decision)                                                  |
| ------------------------------------------------------------------------------ | ------------------------------------------ | -------------------------------------------------------------------------- |
| Tamper on category/severity/action/changes/context/errorMessage/scope/metadata | not detected                               | detected (D1)                                                              |
| Hash algorithm                                                                 | DJB2 32-bit non-crypto                     | SHA-256, 64-hex, fits `chain_hash varchar(64)` (D2)                        |
| DB round-trip (timestamp precision, JSON number re-encode)                     | false `hash_mismatch`                      | byte-stable, no false mismatch (D1)                                        |
| Algorithm / field-set evolution                                                | implicit, un-versioned, breaks old rows    | explicit `chain_version`, v1 legacy path kept, hashes never rewritten (D3) |
| Audit-store outage                                                             | Log returns error; some callers discard it | strict by default; best_effort opt-in with WARN + rate-limited alert (D4)  |
| External signature                                                             | optional, independently verified           | unchanged; independent of hash version (D5)                                |
| Old (v1) rows                                                                  | —                                          | still verifiable with legacy algorithm (D3)                                |

## 4. Implementation notes for ARCH-008

1. Migration 0049: add `chain_version smallint NOT NULL DEFAULT 1` with a
   `HasColumn` guard; backfill existing rows to 1; register in
   `registerSvcMigrations`; bump `MinimumRequiredVersion` 48 → 49; update the
   synthetic migration list in `migrate_test.go`.
2. Extract `canonicalHashInput(record, version) ([]byte, error)`: v1 returns the
   current legacy input + DJB2; v2 returns the D1 canonical input + SHA-256.
   `calculateHash` and `ValidateChain` both dispatch on the record's version.
3. In `Log`, truncate `record.Timestamp` to `time.Millisecond` before
   `buildChainInfo` (and before storage); JSON round-trip `Details` and
   `Changes.before`/`.after` before hashing.
4. Write the new-record version (`2`) through the model (`chain_version`
   column) so `ValidateChain` can dispatch per row.
5. On write failure after retries, branch on the policy: `strict` returns the
   error (current behavior); `best_effort` logs WARN + rate-limited notifier
   alert + returns the record with a gap marker.
6. Tests: per-field tamper detection (each semantic field flipped → hash
   mismatch), round-trip stability (write → read → `ValidateChain` with no
   false mismatch), a legacy v1 row validates with DJB2 while a v2 row validates
   with SHA-256, multi-instance `ErrChainSequenceConflict` still resolved,
   `best_effort` logs + alerts + returns without error, `strict` returns the
   error.

This document changes no runtime behavior. Acceptance (tamper detection,
old-chain verifiability, multi-instance write consistency, approved
failure-policy handling) is the ARCH-008 implementation gate.
