---
title: Mobile Threat Model
icon: shield-alert
order: 60
category:
  - Security
tag:
  - Mobile
  - Threat Model
  - Companion App
---

# Mobile Threat Model

Threat model for the mobile companion app (`mobile/`, Flutter). It is a
**companion client**: it calls the same control-plane REST API as the web
dashboard, with no game-side agent or SDK role. Scope and asset boundaries
are documented in
[the mobile companion design](../archive/legacy-en/design/mobile-companion-design.md).

The model uses a STRIDE-style enumeration per asset. Assets are rated by
impact (High/Medium/Low). Every entry states the **current mitigation**
(what the code already does) and the **accepted residual risk** (what we
knowingly do not defend, with rationale).

## Assets

| Asset                                    | Where                                                          | Impact |
| ---------------------------------------- | -------------------------------------------------------------- | ------ |
| Session token (Bearer JWT)               | `core/storage/session_store.dart` via `flutter_secure_storage` | High   |
| Server URL (endpoint identity)           | same store (survives logout)                                   | Medium |
| Scope selection (`game_id`/`env`)        | same store; sent as `X-Game-ID`/`X-Env`                        | Medium |
| API responses (audit, approvals, alerts) | in-memory only, per-page controllers                           | Medium |
| Device identity / probe result           | `core/server/server_probe.dart`                                | Low    |

## Threat table

| #   | Threat                                            | Asset           | STRIDE                 | Impact | Current mitigation                                                                                                                                                                      | Accepted residual                                                                                                                                                          |
| --- | ------------------------------------------------- | --------------- | ---------------------- | ------ | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| M-1 | Token exfiltrated from app storage                | Session token   | Information disclosure | High   | Token lives in `flutter_secure_storage` (keystore / Keychain), never in `SharedPreferences` or plaintext files; `SessionData` documents "no password, no TOTP seed, no biometric data". | On a rooted / jailbroken device the OS keystore is weaker; accepted because the companion targets operator phones, not adversarial hardware.                               |
| M-2 | Token replayed against a different endpoint       | Session token   | Spoofing               | High   | Token is scoped by the server (audience/expiry) and the app only sends it to the configured `serverUrl`; `ApiClient` injects `Authorization: Bearer` on every request.                  | A captured token can be replayed until expiry if the server's TLS is downgraded; that is a server-side boundary (see `docs/operations/security.md`), not a mobile control. |
| M-3 | Scope header injection via request options        | Scope selection | Tampering              | Medium | `ApiClient` strips any caller-supplied `X-Game-ID`/`X-Env` before injecting the stored scope (mirrors the web interceptor semantics).                                                   | None at app layer; defense is on the server (validates the scope pair atomically).                                                                                         |
| M-4 | 401 session clearing leaves a half-configured app | Session token   | Denial of service      | Low    | `onUnauthorized` clears the session but **keeps `serverUrl`** so the login page prefills it (URL is configuration, not a credential).                                                   | An operator must re-enter credentials; no security impact.                                                                                                                 |
| M-5 | Sensitive data persisted on disk                  | API responses   | Information disclosure | Medium | Responses live in Riverpod controller state (in-memory) only; nothing writes audit/approval payloads to storage.                                                                        | Screenshots / OS snapshot could capture rendered data; accepted for an operator tool with a short-lived session.                                                           |
| M-6 | Server URL spoofing via deep link / QR            | Server URL      | Spoofing               | Medium | `ServerProbe` validates the endpoint is reachable and returns a probe result before allowing login; the URL is entered by the operator.                                                 | No pinning on the mobile side (standard `dio` TLS validation); the design treats the dashboard host as trusted infrastructure.                                             |
| M-7 | Logging leaks the token                           | Session token   | Information disclosure | Medium | No `debugPrint` of the token in `ApiClient`/`SessionStore`; error normalization uses `ApiError` (status + message) without echoing headers.                                             | None found; a future change that logs request options must redact `Authorization`.                                                                                         |

## Verification

- `mobile/test/core/storage/session_store_test.dart` and
  `session_store_secure_test.dart` cover the secure-store contract and the
  "serverUrl survives logout" behavior (M-4).
- `mobile/test/core/api/api_client_test.dart` and `api_error_test.dart`
  cover token injection, scope-header stripping (M-3) and the 401 → clear →
  redirect flow (M-4).
- Coverage gate in `.github/workflows/ci-mobile.yml` (80% floor) keeps the
  security-relevant paths exercised.

## Known boundaries

- This model covers the **companion app** only. The server, agent, SDK and
  wire-protocol trust boundaries are in `docs/operations/security.md` and
  `docs/architecture/transport-glossary.md`.
- No biometric/PIN app lock exists on the companion app (design scope M2);
  a found unlocked device exposes an active session until the server token
  expires. Accepted as a product decision for the operator-facing tool.
