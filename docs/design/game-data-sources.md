---
title: Game Protocol Probe Data Source Selection — Web-Gathered (Skeleton-First Adapter)
---

# Game Protocol Probe Data Source Selection (Web-Gathered)

## Status

- Status: **selection finalized (ruled by the user on 2026-10-15)**. The ruling: items ①/② are marked **pending account verification** (never ticked complete, never replaced by mocks); items ③/⑤ — data sources are **gathered from existing open-source projects on the web, not supplied by the user**, with selection decided on the user's behalf and License changes requiring user confirmation; item ④ — architecture decided on the user's behalf (three-layer split, skeleton first + one real game adapter, with "protocol not integrated" kept as a first-class state).
- This round's gate: **land this document only; write no implementation code**.

## 1. Requirement: what data the skeleton-first adapter needs

Under the §5.5 three layers (kernel / protocol adapter skeleton / per-game adapter), for the skeleton to express a real game's player-side login/handshake/liveness probe as declarative steps, the required inputs are:

- The connection establishment sequence (address/port, transport TCP or UDP, initial handshake packet structure);
- Frame formats (length prefix/varint/alignment), compression and encryption negotiation;
- The authentication flow (if any) and the minimal interaction that establishes "alive";
- Version management that tracks per-game version drift.

This repo has no game protocol stack to reuse (`sdks/` is the game-server→agent registration surface), so the data must come from outside. Candidates are gathered from **existing open-source projects** per the user's ruling — no in-house capture/reverse engineering.

## 2. Candidate Data Source Comparison

| Dimension                | PrismarineJS/minecraft-data                                                                                                                                   | Luanti (formerly Minetest)                                                                                              | AzerothCore / TrinityCore                                                             |
| ------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------- |
| Form                     | **Machine-readable** `protocol.json` (per version), npm package + GitHub repo                                                                                 | **Human-readable docs** (docs.luanti.org network-protocol + in-repo `doc/protocol.txt`), engine source in the same repo | **C++ server implementation** (opcodes/packet structs/login encryption chain in code) |
| Game coverage            | Minecraft Java PC (0.30c→latest) + the Bedrock dual protocol family                                                                                           | Single game (sandbox voxel MMO, UDP)                                                                                    | WoW 3.3.5a single version family (commercial MMO prototype)                           |
| Protocol doc quality     | Structured field-level (state machine + packet structure + types), consumed by a multi-client ecosystem (mineflayer/node-minecraft-protocol/bedrock-protocol) | Official first-class docs, complete connection/auth (SRP) sequences                                                     | No standalone docs — read from source; community wikis supplement                     |
| Self-hostable verifiable | ✅ vanilla server (offline-mode, zero accounts)                                                                                                               | ✅ official server (accounts registered locally on the server, zero external accounts)                                  | ✅ full private server (assets belong to Blizzard; intranet testing only)             |
| License                  | **MIT** (repo LICENSE matches npm)                                                                                                                            | LGPL-2.1+ (engine and docs equally licensed)                                                                            | **GPLv2** (AzerothCore reverted from AGPLv3 in 2024-09, TrinityCore-compatible)       |
| Maintenance              | Active (npm 3.117.x keeps releasing)                                                                                                                          | Active (continues after the 2024 rename to Luanti)                                                                      | Active                                                                                |

Auxiliary indexes (not primary): Wireshark game protocol dissectors (GPL, packet-dissection reading reference); minecraft.wiki protocol docs (merged here after wiki.vg shut down on 2024-11-30, **CC BY-SA**); `squi2rel/node-mindustry` (GPL-3.0, a third-party Mindustry protocol implementation, backup reference for the UDP family).

## 3. Single Recommendation and Reasons

**Recommend PrismarineJS/minecraft-data; the first adapter target = a Minecraft Java Edition offline-mode self-hosted server.**

Reasons:

1. **The only MIT**: the only one of the three candidates allowing both commercial use and redistribution (with copyright notice), hence the only source that can use the "direct data consumption" channel; the rest can only use the "factual reference" channel (see §4). The selection decision stays within the delegation and does not touch the License-change confirmation line.
2. **Machine-readable + versioned per version**: `protocol.json` (state machine + packet structure + field types) can be script-converted into the §5.5 declarative steps — no manual protocol extraction from C++/Java source — the lowest-cost path for "data sources gathered from the web", with a ready-made structure for version-drift management.
3. **A real commercial game with representative protocol shapes**: Minecraft is a real commercial game (a self-hosted copy as the test target, **not a mock**); the Java Edition's TCP varint framing + zlib compression + online encryption handshake is the mainstream shape of PC games and long-connection mobile games, while Bedrock (RakNet/UDP) covers the UDP family — two protocol families giving the "all game types" sample depth.
4. **Zero-account E2E**: offline-mode validates no Microsoft account; a docker self-hosted server suffices for a full player-perspective verification chain (login handshake → liveness), satisfying "skeleton first + one real game adapter" without being blocked on accounts; the online encryption chain is separately marked pending account verification (§6).

Luanti is recorded as the **UDP family's second candidate** (a fully open game, legally cleanest, high-quality official docs), but this round builds only one adapter (already decided), and its docs are human-readable prose rather than machine-readable, making conversion costlier than minecraft-data. AzerothCore/TrinityCore are positioned as **L3 commercial MMO knowledge prototypes** (deep reference for login encryption/packet structures), not the first adapter.

## 4. License Verification Conclusions

| Source                                                                                                                                                                                                                                  | License                                                                                                                                                    | Commercial use            | Redistribution                          | This repo's (Apache-2.0) posture                                                                                                                                                                                                         |
| --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------- | --------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| minecraft-data                                                                                                                                                                                                                          | MIT ([repo](https://github.com/prismarinejs/minecraft-data) LICENSE consistent with [npm](https://www.npmjs.com/package/minecraft-data), verified 2026-10) | ✅                        | ✅ (keep copyright and license notices) | **Data-consumption channel**: direct dependency allowed; but `protocol.json` expresses the Mojang protocol (third-party copyright), so per the red line it is **not distributed with the repo by default and pulled at build time** (§5) |
| [Luanti](https://github.com/luanti-org/luanti) (engine + docs)                                                                                                                                                                          | LGPL-2.1+ ([official docs](https://docs.luanti.org/for-engine-devs/network-protocol) equally licensed)                                                     | ✅                        | ⚠️ derivatives need LGPL                | **Factual-reference channel**: read the docs, write our own steps; no code copying, no file carrying                                                                                                                                     |
| [AzerothCore](https://github.com/azerothcore/azerothcore-wotlk) (2024-09 [license revert announcement](https://github.com/azerothcore/azerothcore-wotlk/discussions/19868)) / [TrinityCore](https://github.com/TrinityCore/TrinityCore) | GPLv2                                                                                                                                                      | ✅ (under GPL compliance) | ⚠️ derivatives need GPL                 | **Design reference only** (the L3 prototype); no code copying, no file carrying, no linking                                                                                                                                              |
| [minecraft.wiki protocol docs](https://minecraft.wiki/w/Minecraft_Wiki:Projects/wiki.vg_merge)                                                                                                                                          | CC BY-SA (inherited from wiki.vg's 2024-11 shutdown merge)                                                                                                 | ⚠️                        | ⚠️ ShareAlike                           | **Link only, never excerpt** (ShareAlike is incompatible with Apache-2.0 docs)                                                                                                                                                           |

**License-change confirmation line** (ruling): if minecraft-data ever moves to a non-MIT license, stop the consumption channel and get user confirmation before acting; a license change in the GPL/AGPL-family sources does not affect this repo (they always use the zero-copy reference channel).

## 5. Integration Approach

**Build our own adapter skeleton using them as reference; minecraft-data goes through the tooling data channel.** The two run in parallel without contradiction:

1. **Skeleton** (Apache-2.0, self-written, §5.5 three layers): a declarative steps DSL + "protocol not integrated" as a first-class state (never disguised as integrated, no empty implementations). **Delivered (2026-10-15)**: `core/healthprobe/protocol.go` (L2) + the minecraft first adapter (L3); delivery records in agent-health-recovery-review.md §5.5.6.
2. **First adapter** (Minecraft Java offline-mode): **actually landed as an SLP (Server List Ping) L3 code adapter, self-written against the public protocol spec** — SLP is only a handshake + two packets and needs no field-level `protocol.json` data; the build-time pull channel is **reserved for login-level L3** (only the login handshake/Login flow needs field-level protocol expression, at which point the original minecraft-data tooling channel applies and the raw json still never enters the repo). Zero-account verification works against both offline/online-mode servers (SLP involves no login encryption); online-mode's **login chain** verification remains pending accounts per §6.
3. **Self-hosted verification**: a docker vanilla server in offline-mode (EULA accepted once, intranet testing only); player-perspective probing closes the loop with zero accounts.
4. **L3 commercial games** (later): open-source server implementations as knowledge prototypes (zero-copy); real protocol integration proceeds along the dual path of "game-side-provided knowledge or player-side account testing", decided per game at project start.

## 6. Pending Account Verification (disposition of ruling items ①/②)

The following are **never ticked complete, never replaced by mocks**; status = "pending account verification" (distinct from "not implemented": implementation and offline verification can close first, with the account only unlocking the online chain):

| Item                                       | Verification content                                                                      | Prerequisites                                                                              |
| ------------------------------------------ | ----------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------ |
| Minecraft **online-mode** encryption chain | Microsoft account login chain (XBL→MSA→session server) + the full AES-encrypted handshake | 1 MC Java account + an internet-capable test environment                                   |
| Player-side probing of the user's own game | Online verification of the first real commercial game's L3 adapter                        | Test accounts and environment for that game (supplied by the user when the project starts) |

## 7. Red Lines Restated (the ruling's original text, landed)

- ✅ Reference open-source project ideas; ❌ copy code or data files for redistribution.
- Third-party copyrighted data (the json expression of the Mojang protocol, wiki text, Blizzard assets) is **not distributed with the repo by default; it is pulled at build time**.
- The selection is decided; **license changes require user confirmation**.
- "Protocol not integrated" stays a first-class state — no empty implementations disguised as integration.
