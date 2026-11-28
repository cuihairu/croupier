# C-005 Auth Gate: JWT localStorage → httpOnly Cookie Migration Review

Date: 2026-11-28. Status: **proposal awaiting owner decision**. Deliverable
for C-005 (P2 security, deferred 2026-11-03 pending "design sign-off").
**Nothing is implemented here.** The proposal narrows the deferred design
decisions to one signature; the code facts below were re-verified today —
several premises from the deferral have changed (C-004 fixed, AUD-044
removed the `?token=` query channel, refresh-token machinery landed).

## 1. Grounding (code facts, 2026-11-28)

| Fact                                                                                                                                                                                          | Source                                                                                       |
| --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------- |
| Login response carries `token` (+ `refreshToken`); web stores **only** the access token (`localStorage.setItem('token', …)`)                                                                  | `web/src/pages/User/Login/index.tsx:173`; no `refreshToken` consumer exists in `web/src` yet |
| Middleware authenticates **only** `Authorization: Bearer` — the `?token=` query channel was removed after C-004 (SSE moved to fetch-stream + header), explicitly so tokens never live in URLs | `internal/svc/service_context_auth.go:162-173` (AUD-044 comment)                             |
| OAuth callbacks 302 to the frontend **with the token in the redirect URL** (`q.Set("token", resp.Token)`) on four isomorphic paths (OIDC/GitHub/WeChat/custom)                                | `internal/api/auth/handler.go:353-372` (`oauthCallback`)                                     |
| Logout bumps `token_version` — every token for the account dies fleet-wide within ≤30s (middleware cache)                                                                                     | `internal/api/auth/auth_check.go:13-25`                                                      |
| MFA / mustChangePassword are **response-gated states** (401 + stable code), not separate intermediate tokens                                                                                  | `internal/api/auth/dto.go:38-44`, `service.go:22-28`                                         |
| Web 401 handling and boot path read/remove `localStorage.token` in 7 places                                                                                                                   | `web/src/requestErrorConfig.ts:214,232,318,325,338,401`; `web/src/app.tsx:118,150`           |
| Same-origin deployments: dev proxy + prod nginx L7 — browser→API is same-origin, so cookies ride fetch by default (`credentials: 'same-origin'`)                                              | memory + deploy topology (deploy-inventory §2)                                               |
| Non-browser clients (mobile Flutter, SDKs, CLI) authenticate with the same JWT via Bearer                                                                                                     | wire contract                                                                                |

Interpretation: after AUD-044 removed the URL surface, **localStorage is
the last XSS-readable token surface** in the browser path. This migration
completes that direction rather than opening a new one.

## 2. D1 — transport end-state

| Option                                                                           | Shape                                                                                                                                                                                                             | Assessment                                                                                                                                                   |
| -------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| (a) **httpOnly cookie for browsers, Bearer stays for non-browser** (recommended) | Login/callback `Set-Cookie` (httpOnly) alongside the body token; middleware reads Bearer first, cookie as fallback for browser requests; web stops writing/reading localStorage entirely and stops sending Bearer | Browser can no longer read the token at all (XSS gains nothing persistent); non-browser clients unaffected; aligns with the AUD-044 single-surface direction |
| (b) Web keeps token memory-only + Bearer                                         | Token held in a JS variable, re-minted via refresh on reload                                                                                                                                                      | XSS still reads it at every page load; refresh-on-reload adds an opaque startup dependency; strictly worse than (a)                                          |
| (c) Server-side session store + opaque cookie                                    | Replace stateless JWT for browsers                                                                                                                                                                                | Larger redesign (session store, fan-out invalidation), contradicts "no new undesigned services"; not needed to fix the finding                               |

## 3. D2 — cookie attributes

Recommended: two cookies —

- `croupier_session` = access JWT: `HttpOnly; SameSite=Strict; Path=/; Max-Age=<access TTL>`, `Secure` **config-gated** (on in prod https, off when `server.mode: dev` serves plain http);
- `croupier_refresh` = refresh JWT: same flags plus `Path=/api/v1/auth/refresh` so it never rides other requests.

Names/flags are constants in one package (no per-handler drift); no `Domain`
(host-only). Dev-over-http without `Secure` is a documented dev-only boundary.

## 4. D3 — CSRF posture

| Option                                               | Assessment                                                                                                                                                                                                                                       |
| ---------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| (a) **SameSite=Strict alone** (recommended)          | Cross-site requests (fetch or top-level navigation POST) never carry the cookie → CSRF writes are dead without any CSRF token; same-origin fetches unaffected. Simplest, no token rotation machinery                                             |
| (b) Lax + CSRF double-submit token                   | Only needed if some top-level cross-site navigation must carry the session — none exists in this app (SPA boot is a same-origin fetch)                                                                                                           |
| (c) Strict + custom-header check as defense-in-depth | Optional hardening: cookie fallback additionally requires a non-CORS-safelisted custom header (e.g. `X-Requested-With: croupier`) which cross-site fetch cannot set. One middleware line; recommended as a cheap second lock, decided with D3(a) |

## 5. D4 — OAuth callback token delivery

Replace the `?token=` redirect query with `Set-Cookie` on the **302 response
itself** (the callback runs on the API origin, so the cookie is set directly);
the redirect URL carries no token. All four isomorphic callback paths
(OIDC/GitHub/WeChat/custom) change in the one shared `oauthCallback` exit.
This also kills the token-in-Referer/browser-history residue the AUD-044
comment already flagged as the reason the query channel died.

## 6. D5 — refresh token placement

- (a) **Second scoped httpOnly cookie** (recommended): browser refresh calls
  carry it automatically; it cannot ride any other request (Path-scoped).
- (b) Body-only refresh token: re-creates the same XSS-readable long-lived
  secret problem for browsers the moment web integrates refresh
  (`web/src` has no refresh consumer yet — (a) decided now avoids retrofit).

## 7. D6 — logout

`Logout` already revokes server-side (`token_version` bump, ≤30s fleet-wide).
Add: both cookies cleared (`Max-Age=0`, same flags) on the logout response;
web drops its 7 localStorage touchpoints (`requestErrorConfig.ts`,
`app.tsx`, `Login/index.tsx`).

## 8. D7 — rollout shape

| Option                                          | Assessment                                                                                                                                                                                                    |
| ----------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| (a) **Single release, both ends** (recommended) | The repo owns web + server in one deploy; body token stays (non-browser clients keep working); web switches to cookie-only in the same release. One migration, no dual-mode middleware branch to remove later |
| (b) Two releases (dual-write then strip)        | Only pays off if external deployments pin web and server versions apart — the compose deploy ships them as one stack                                                                                          |

Body token remains in responses for non-browser clients; web code simply
never persists or sends it. A unit test asserts `localStorage` receives
nothing on login/callback/logout flows (acceptance A2).

## 9. Acceptance mapping

| #   | Scenario                                                                                                         | Maps to     |
| --- | ---------------------------------------------------------------------------------------------------------------- | ----------- |
| A1  | Login response sets `croupier_session` (+`croupier_refresh`) with the D2 flags                                   | D1(a)/D2/D5 |
| A2  | Login / OAuth callback / logout leave `localStorage` untouched (new unit test, both EN/zh flows)                 | D7          |
| A3  | Browser request with no Bearer authenticates via cookie fallback; Bearer path unchanged for non-browser clients  | D1(a)       |
| A4  | Cross-site POST with stolen nothing: cookie not attached (Strict), request 401                                   | D3(a)       |
| A5  | OAuth callback 302: URL carries no token; cookie set on the 302; frontend lands clean                            | D4          |
| A6  | Refresh call succeeds with only the scoped refresh cookie                                                        | D5(a)       |
| A7  | Logout: server revocation unchanged (≤30s) + both cookies cleared                                                | D6          |
| A8  | Existing auth suites (login state machine, MFA gates, callback tests) all green with the cookie assertions added | all         |

## 10. Decisions needed from the owner

- [ ] D1: approve (a) httpOnly cookie for browsers / Bearer for non-browser.
- [ ] D2: approve the two-cookie naming/flags and the dev-http `Secure` gating.
- [ ] D3: approve Strict-only CSRF posture (optionally + custom-header check).
- [ ] D4: approve OAuth 302 Set-Cookie (URL token removed on all four paths).
- [ ] D5: approve scoped refresh cookie.
- [ ] D6/D7: approve logout clearing + single-release rollout.

## 11. Boundary

- No token format, TTL, or crypto change; no server-side session store —
  the stateless JWT contract is untouched; only the browser transport
  surface moves.
- Mobile/SDK/CLI continue on Bearer + body token, unchanged.
- The deferral-era premise "middleware reads Authorization/?token=" is
  obsolete (AUD-044); this review re-grounds on the current single-header
  middleware.
