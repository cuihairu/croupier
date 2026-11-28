# Minecraft Online-Mode Login: Probe Adaptation Boundary Survey

Date: 2026-11-28. Deliverable for PROBE-003 (资料调查, MC half). Collects the
Java Edition online-mode login protocol chain and the account/licensing
conditions it implies, and states what each probe level can and cannot prove.
The 自家游戏 (own-game) half of PROBE-003 is explicitly out of scope here — it
awaits the game's own protocol/auth documentation and remains user-gated.

Grounding in this repo:

- **SLP adapter is fully implemented** (`core/healthprobe/games/minecraft/slp.go`):
  layer-3 game-protocol checker, self-written from the public Server List Ping
  spec, standard library only, registered as game protocol `minecraft`
  (`internal/app/agent/probe_protocols.go` blank import). Its own header states
  the boundary this survey confirms: SLP never touches the login/encryption
  chain, so it verifies any Java server with **zero accounts** — and therefore
  is **status evidence only, never login evidence** (PROBE-003's binding
  clause).
- Live outbound evidence (2026-11-28, this host):
  `sessionserver.mojang.com/session/minecraft/hasJoined` → **204** (alive;
  204 is the documented "not joined" response, so the endpoint behaves per
  spec), `api.mojang.com` → **404 at root** (host alive), legacy
  `authserver.mojang.com` → **unreachable** (legacy Yggdrasil auth is retired
  after the Microsoft-account migration).

## 1. The online-mode login chain (public spec, wiki.vg)

```
C→S  Handshake (intent=login, protocol version)
C→S  Login Start (player name; recent versions also carry the client UUID)
S→C  Encryption Request (server public key DER, random verify token, should-authenticate flag)
     ── client-side account auth, before or interleaved with the next step ──
C→Microsoft  device-code OAuth → XBL user token (user.auth.xboxlive.com)
             → XSTS token (xsts.auth.xboxlive.com)
             → minecraftservices.com authentication/login_with_xbox → access_token
C→Mojang     session join (sessionserver.mojang.com/session/minecraft/join):
             accessToken + selectedProfile + serverId hash
             (weird-hex sha1 of serverId + sharedSecret + serverPublicKey)
C→S  Encryption Response (shared secret + verify token, RSA-encrypted with the server key)
S→Mojang  hasJoined?username&serverId → 200 profile JSON = authenticated; 204 = reject
     ── both sides switch to AES-CFB8 stream cipher; compression threshold follows ──
S→C/C→S  (1.20.2+) Login Acknowledged → configuration state → play state
```

Related facts that matter for probing:

- **Offline-mode** servers skip encryption/auth entirely; the UUID is derived
  as `nameUUIDFromBytes("OfflinePlayer:" + name)` (MD5-based v3). Login-path
  probing against an offline-mode server needs no account at all.
- **Proxy forwarding** (relevant when the platform probes through an
  ops-facing proxy): BungeeCord IP forwarding trusts handshake data (must be
  firewall-isolated); Velocity modern signs forwarded player info with a
  shared secret over a plugin channel.
- The server performs one `hasJoined` outbound call **per online-mode login** —
  login-level probing consumes Mojang-side rate budget (community-documented
  IP rate limiting in the hundreds per 10 minutes).

## 2. Account and licensing conditions

| Side                      | Requirement                                                                                                                                                                                                |
| ------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Player client             | Microsoft account **with a Java Edition entitlement** (purchase or Game Pass). Token chain needs an OAuth flow to MSA (device code is the headless-friendly one) and the XBL→XSTS→minecraftservices chain. |
| Game server (online-mode) | **No account of its own.** Needs outbound reachability to `sessionserver.mojang.com` (per-login `hasJoined`) and DNS/TLS to Microsoft/Mojang hosts.                                                        |
| Probe actor               | See levels below — only level C needs a real licensed account, which is the licensing decision the user has reserved (许可证变更另行授权).                                                                 |

## 3. Probe levels: what each can prove

| Level                                          | Needs                                                                | Proves                                                                            | Cannot prove                                                                                                              |
| ---------------------------------------------- | -------------------------------------------------------------------- | --------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------- |
| **A. SLP** (implemented, `minecraft` protocol) | nothing                                                              | server alive; version; player counts; motd/icon                                   | anything about login/auth — binding clause: **SLP is not login evidence**                                                 |
| **B. Login-path reachability** (no account)    | nothing                                                              | handshake→Login Start→Encryption Request path; server-side TLS/key exchange alive | a _successful_ login: an online-mode server drops the synthetic client at `hasJoined` (unknown profile) before play state |
| **C. Real login probe**                        | **licensed account + user authorization** + MSA token secret storage | end-to-end player login exactly as a player would do it                           | — (this is the full-evidence level; gated on the license decision)                                                        |
| **D. Auth-dependency health**                  | nothing                                                              | reachability/latency of `sessionserver`/`api.mojang.com` from the server host     | game-side login behavior                                                                                                  |

Level B nuance: against an **offline-mode** target, the synthetic login can
complete to play state with zero accounts — full login-path evidence, but only
proves the unauthenticated path. Against an **online-mode** target it stops at
the encryption boundary — which is itself a meaningful "server login machinery
alive" signal.

## 4. Conclusion and remaining gates

- The MC half of PROBE-003's collection deliverable is this document plus the
  already-implemented SLP adapter; level B/D probe work is implementable with
  zero accounts whenever scheduled (they are not part of PROBE-003 itself —
  PROBE-003 is the survey).
- Remaining gates for PROBE-003 full closure, both user decisions:
  1. **自家游戏 login protocol/auth doc** — cannot be surveyed from the
     outside; the game must provide it.
  2. **Licensed-account authorization** (level C) — the reserved licensing
     decision; without it, login evidence for online-mode targets stops at
     level B's encryption boundary.
- Binding clause restated for any future implementation: SLP status checks
  must never be recorded as player-login evidence.
