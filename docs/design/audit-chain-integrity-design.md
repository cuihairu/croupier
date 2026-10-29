# Audit Chain Integrity: Versioned Digest & Failure Policy

> ARCH-007 · P2 · 方案决策 · 版本化审计摘要与失败策略
>
> Scope: the audit chain in `internal/audit/audit.go` (`AuditService.Log` →
> `buildChainInfo` → `calculateHash`/`hashBytes`, `ValidateChain`) and its
> persistence (`internal/audit/store.go` `AuditModel`: `audit_records`).
> The audit **retention/export** (`Archive`, `Export`) and the notifier
> (`AuditNotifier`) are out of scope here.
>
> Status: **DECIDED** (2026-10-29). D1–D7 settled: canonical full-payload hash,
> SHA-256 upgrade, `hash_version` for old-chain compatibility, scope/action/
> changes coverage, fail-open-with-surfacing write policy plus a fail-closed
> security allowlist, and confirmation that the external signature already owns
> its dedicated columns. Drives ARCH-008 (落地审计完整性规则). This document
> does not establish production readiness.
> Prerequisite for: ARCH-008.

## 1. Problem

The audit chain is meant to make tampering of audit records evident, but the
current digest covers only a narrow summary of each record and uses a
non-cryptographic 32-bit function, so most of the record's meaning — and most
tampering — is outside the integrity guarantee.

Evidence (all in `internal/audit/audit.go`):

- **Narrow hash fields** (`calculateHash`, line 628): the digest input is only
  `timestamp`, `event_type`, `actor_id` (just `Actor.ID`), `resource` (just
  `Resource.ID`), `outcome`, `prev_hash`, `sequence`, and `details` (when set).
  **Not hashed**: `action`, `category`, `severity`, `error_message`,
  `resource.type/name/gameId/environment`, the whole `changes`
  (before/after/diffFields), the whole `context`, and all of `actor`
  (type/name/roles/ip/userAgent/sessionId/mfaUsed/delegatedBy/metadata). A
  tamper that rewrites `changes.before/after`, `action`, or the scope
  (`resource.gameId`/`resource.environment`) therefore passes chain validation
  unchanged.
- **Non-cryptographic digest** (`hashBytes`, line 807): djb2, 32-bit, `%08x`.
  A 32-bit non-keyed hash is not a commitment — collision forgery is cheap
  (~2^16 records for a birthday collision) and there is no preimage resistance
  for an attacker who can recompute. The code comment even says "in production,
  you'd use crypto/sha256", but the implementation never changed.
- **No versioning**: there is no `hash_version` column. Any change to the hash
  field set or algorithm would silently invalidate every pre-existing record's
  digest; `ValidateChain` would report `hash_mismatch` for the whole history.
- **External signature is dormant**: production wires the signer as `nil`
  (`internal/svc/service_context.go:354` `NewAuditService(auditStore, nil)`).
  The columns `chain_signer_id`/`chain_signature` exist and are always empty.
- **Write-failure handling is inconsistent**: `Log` returns an error, but call
  sites split between `_, err := ...Log(...)` (may propagate) and
  `_, _ = ...Log(...)` (silently dropped) — e.g. `internal/api/approval/
stepup_otp.go:149` and `service.go:142/371`, `internal/api/configexplorer/
service.go:84/171` drop it; `internal/platform/functioninvoke/approval.go:50`,
  `internal/api/console/service.go:1197`, `internal/api/page/service.go:1848`,
  `internal/api/openapi/service.go:2072` capture it. There is no single policy
  and no observable signal (metric/header) when an audit row is lost.
- **Number round-trip hazard**: `ToRecord` decodes the JSON columns with
  `json.Unmarshal` (default → `float64`). A `details`/`changes` value that is an
  integer above 2^53 (e.g. an id `9007199254740993`) is written as `int64` and
  hashed as `"9007199254740993"`, then read back as `float64` and re-marshaled
  as `"9007199254740992"` — a **false `hash_mismatch`** on a perfectly intact
  chain. The digest must be reproducible from the stored representation, not the
  in-memory one.

## 2. Decisions

### D1 — Canonical full-payload digest

The digest input becomes a **canonical JSON object covering every
integrity-relevant field**, replacing the current narrow list:

```json
{
  "id":          "<audit_id>",
  "timestamp":   <unix_nano>,
  "event_type":  "<string>",
  "category":    "<string>",
  "severity":    "<string>",
  "action":      "<string>",
  "outcome":     "<string>",
  "error_message": "<string>",
  "sequence":    <int64>,
  "prev_hash":   "<string>",
  "actor":       <raw actor_json bytes>,
  "resource":    <raw resource_json bytes>,
  "details":     <raw details_json bytes>,
  "changes":     <raw changes_json bytes>,
  "context":     <raw context_json bytes>
}
```

- The JSON sub-objects (`actor`, `resource`, `details`, `changes`, `context`)
  are embedded as their **stored raw column bytes**, not re-decoded values, so
  the write-time and verify-time digests are byte-identical by construction —
  this removes the number-precision hazard entirely. (Alternative: decode with
  `json.Decoder.UseNumber()`; either satisfies "old chains verifiable", but raw
  bytes are the primary choice because they are exact for any JSON value.)
- Typed scalar fields are serialized as scalars; `sequence` and `prev_hash`
  stay **inside** the digest so the linkage itself is tamper-evident.
- Excluded: `chain.hash`, `chain.signature`, `chain.signer_id` (self-referential
  / keyed-cover fields) and `created_at` (store insertion timestamp, not a
  semantic field).
- `action` is always present (defaults to `event_type` when unset,
  `setDefaults`), so it is never a non-deterministic omission.

### D2 — SHA-256 digest function

The digest function becomes `crypto/sha256`, hex-encoded (64 chars). This is
the **v2** algorithm. The v1 djb2 32-bit function is retained only to verify
pre-existing records (see D3), never for new ones.

### D3 — `hash_version` column for old-chain compatibility

- Add an `hash_version int` column on `audit_records`.
- New records write `hash_version = 2` (SHA-256, D1 payload).
- A goose migration backfills existing rows with `hash_version = 1` (the legacy
  narrow-field djb2 digest).
- `ValidateChain` dispatches per record: v1 recomputes the legacy digest
  (unchanged algorithm over the unchanged field list — old records stay
  verifiable and **detect the same tampering they did before**); v2 recomputes
  the D1/D2 digest.
- Old records are **not** re-hashed: re-writing `chain_hash` would break the
  `prev_hash` links of every descendant row, and re-signing history is a
  deliberate operation with its own integrity implications. The chain is linear
  and single-global (see D5), so a v1 record is simply the chain head that a v2
  record links onto.

### D4 — Scope, action and changes coverage (the "normalized fields" of the ticket)

D1 already covers them; this decision makes the intent explicit. The previous
gap — `action`, `resource.gameId`/`resource.environment` (the operational
scope), and `changes.before/after` being un-hashed — is closed. A tamper that
rewrites any of these now breaks the chain.

### D5 — Single global chain

The chain is one global sequence (`chain_sequence`), not per-scope/per-game.
Rationale: the audit UI lists records globally; cross-scope tampering
correlation is only visible on a single ordered chain; per-scope chains would
multiply verification and complicate the "which chain" question. Multi-instance
concurrency stays on the existing **unique-constraint + retry** mechanism
(`store.go` `ErrChainSequenceConflict`, `Log` retry loop) — no advisory lock,
no DB sequence.

### D6 — Write-failure policy: fail-open with mandatory surfacing, plus a fail-closed security allowlist

- **Default: fail-open.** On audit-store write failure the business operation
  proceeds. Rationale: the chain is append-only evidence; a lost row makes
  evidence absent, which is observable, whereas blocking the operation converts
  an audit outage into a functional outage with no integrity gain (the row is
  not written either way). The failure must be **surfaced**, never silent:
  `slog.Error("audit write failed", ...)`, a monotonic counter
  `audit_write_failures_total{event_type, category}`, and a best-effort
  `X-Audit-Degraded: true` response header on the calling HTTP handler.
- **Fail-closed allowlist** (irreversible security mutations whose absence of a
  trail is more dangerous than the outage): `admin.user_delete`,
  `admin.user_lock`, `admin.user_unlock`, `admin.password_change`,
  `auth.mfa_enabled`, `auth.mfa_disabled`, `system.backup_restore`. For these,
  `Log` returns a sentinel `ErrAuditWriteFailed` and the caller **must** abort
  the operation (record not created → op not performed). Identity events
  (`auth.login*`, `authz.access_*`) are deliberately fail-open: they cannot be
  fail-closed (there is no identity to authenticate against a down audit store),
  so a split that blocks login is impossible.
- **Single choke point.** All call sites move to one helper
  (`AuditService.Log` keeps its signature; a new
  `AuditService.LogOrDegrade(ctx, opts...)` applies the policy above). This
  removes the current `_, err` / `_, _` split.

### D7 — External signature: dedicated columns already exist (no change)

`chain_signer_id`/`chain_signature` are already dedicated per-record columns,
not embedded in the digest JSON and not a separate table. **Keep that layout.**
The signature is computed over the same canonical bytes the digest covers (so
hash and signature agree), keyed by `signer_id`. Production currently passes
`signer=nil` (columns dormant); when an external signer (HSM/TPM/KMS) is wired,
it populates the existing columns — no schema change. The signer column pair is
not "one column" to add; it is already single per record.

## 3. Review table

| #   | Scenario (current code)                                    | With D1–D7                                                                                   |
| --- | ---------------------------------------------------------- | -------------------------------------------------------------------------------------------- |
| 1   | Tamper `changes.before/after`, `action`, `resource.gameId` | Chain breaks (`hash_mismatch`) — D1/D4                                                       |
| 2   | Tamper `details` with an integer > 2^53                    | Verifies clean today; **false mismatch** on intact chain — fixed by raw-bytes embedding (D1) |
| 3   | Forge a colliding 32-bit digest (djb2)                     | Precluded for new records (SHA-256, D2); old records keep legacy exposure (documented, D3)   |
| 4   | Audit store down during a normal op                        | Op proceeds, failure surfaced (log+counter+header) — D6                                      |
| 5   | Audit store down during `admin.user_delete`                | Op aborted (`ErrAuditWriteFailed`) — D6                                                      |
| 6   | Old v1 record in the middle of a v2 chain                  | Verifiable under v1 digest; children link onto its v1 hash — D3                              |
| 7   | Multi-instance concurrent `Log`                            | Unique-constraint retry; single global chain — D5                                            |
| 8   | External signer introduced                                 | Populates existing `chain_signer_id`/`chain_signature`; no migration — D7                    |

## 4. Integrity floor & boundary (honest limits)

- The hash chain is **tamper-evidence without a key**. It detects accidental
  corruption and casual/retroactive edits (including direct DB writes) that do
  not recompute the chain. An attacker with DB write **and** the ability to
  recompute the digest can forge the whole chain — the keyed signer (D7) is
  what upgrades this to cryptographic non-repudiation. This is a floor, not a
  guarantee, and is stated so ARCH-008's verification expectations are honest.
- Detection depends on **periodic `ValidateChain`** over the full range; there
  is no continuous verification. ARCH-008 decides the cadence and the alert.
- `hash_version` backfill (D3) makes pre-existing rows _verifiable_, not
  _stronger_: a v1 row is still protected by the weak digest. Re-signing history
  is deliberately out of scope (see D3).
