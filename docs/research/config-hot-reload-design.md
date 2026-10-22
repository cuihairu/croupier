---
title: Game Config Hot-Reload Survey and Design (Excel / Config Center / Redis / Code Generation)
---

# Game Config Hot-Reload Survey and Design

## Status

Proposed (survey complete, design finalized). The P0 reuse of the ConfigVersion model + the versioned distribution API has landed; the Excel pipeline and push channels are phased.

## 1. Problem

Game configuration (numeric tables / switches / event windows / redemption codes / IAP store) changes far more often than code, and must **take effect without restarts**. Two fundamentally different routes:

- **Compile-time route**: Excel → export source/assets → ship via **client resource packs** (the release system);
- **Runtime route**: config lives outside the package; the server (and client) **pull/push at runtime**.

Confusing the two is a common design mistake: putting numeric tables on the runtime route bloats the client and tears versions; putting switches on the compile-time route defeats the point of hot updates.

## 2. Full Comparison of Market Options

### 2.1 Compile-time (Excel [artifacts])

| Option                                                                   | Artifact       | Toolchain                                               | Hot-update path                                             | Assessment                                                                                                                   |
| ------------------------------------------------------------------------ | -------------- | ------------------------------------------------------- | ----------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------- |
| **Excel → source files** (C++ header / Go struct / Java class)           | Type-safe code | In-house exporter + Excel/WPS + SVN/Git                 | Ships with server builds                                    | A long-standing MMO staple; highest type safety; but **every numeric change requires build + deploy**, no runtime hot update |
| **Excel → binary/JSON/Lua/tables (protobuf/flatbuffers/json/lua table)** | Data files     | ExcelExporter-type tools (e.g. xlsx2lua, Tabtoy, Luban) | Server: restart or in-house reload; client: hot-update pack | Mainstream for mobile games (**Luban** is the de facto standard in China: Excel → json/lua/bin + multi-language schema)      |
| **Google Sheets + CI export**                                            | Same as above  | Sheets API + CI                                         | Same as above                                               | Designer-collaboration friendly (real-time co-editing + revision history); unstable access from China                        |
| **Airtable/Notion databases as config source**                           | JSON API       | In-house sync                                           | Runtime pull                                                | Fine for lightweight back-office config; unsuitable for game numeric tables (weak columns/types)                             |

### 2.2 Runtime (config centers)

| Option                                                            | Mechanism                                              | Push                            | Gradual rollout                  | Game fit | Assessment                                                                                                                                                                                                                                           |
| ----------------------------------------------------------------- | ------------------------------------------------------ | ------------------------------- | -------------------------------- | -------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **etcd**                                                          | KV + watch                                             | Long-lived watch                | Per key prefix                   | ★★★      | Ops standard component; game servers watch once per node; self-hosted                                                                                                                                                                                |
| **Nacos**                                                         | Config center + registry                               | Long polling / UDP              | Beta releases (IP-based canary!) | ★★★★     | Common at Chinese game companies; built-in canary release and rollback, Chinese console                                                                                                                                                              |
| **Apollo (Ctrip)**                                                | Config center                                          | Long polling + real-time        | Per cluster/namespace canary     | ★★★★     | Mature permissions/audit; one-click release records/rollback                                                                                                                                                                                         |
| **Consul KV**                                                     | KV + watch                                             | Long-lived connection           | None built in                    | ★★       | Usually adopted alongside service discovery                                                                                                                                                                                                          |
| **Spring Cloud Config**                                           | git backend                                            | Bus refresh                     | None                             | ★        | Bound to the JVM stack                                                                                                                                                                                                                               |
| **Redis as a config bus** (incl. the skynet convention)           | Pub/sub `__keyspace@0__:notify` or version-key polling | pub/sub (unreliable) or polling | In-house                         | ★★★      | **skynet community convention**: config written to Redis, logic servers subscribe to keyspace notifications or poll the version key; zero extra components since game servers already run Redis; no versioning/audit/canary — must be built in-house |
| **Croupier's own ConfigVersion** (an embryo already in this repo) | DB version table + REST                                | Pull (SSE push can be added)    | Reuses assignment/game scope     | ★★★★     | Already in the model inventory; lacks the distribution/subscription protocol — this document fills that in                                                                                                                                           |
| Firebase Remote Config / LaunchDarkly                             | SaaS                                                   | SDK polling                     | Targeting by user attributes     | ★★       | Unreachable from China / expensive; client-oriented                                                                                                                                                                                                  |

### 2.3 Selection Conclusion

Layered mixing, **one channel per config class**:

| Config type                                                         | Example                                       | Channel                                                                                                                   | Reason                                                                                                                          |
| ------------------------------------------------------------------- | --------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------- |
| Numeric tables (large, strong schema, shared by both ends)          | Character/item/level numbers                  | **Excel → Luban export → versioned object storage**; server-side adapter reload + client via release hot-update packs     | Type-safe; both-ends consistency comes from the same export artifact; the platform only handles artifact hosting and versioning |
| Runtime switches/rules (small, server-only, high-frequency changes) | Feature switches / risk rules / event windows | **croupier ConfigVersion (strengthened in this document)**: versioned + per game/env scope + approval + subscription push | Exactly the GM platform's job; reuses assignment canary and audit                                                               |
| Client dynamic copy/switches                                        | Announcements / gift-pack switches            | Same channel + client SDK pull                                                                                            | Lightweight tier that needs no Lua hot update                                                                                   |

**Do not build a general-purpose config center** (etcd/Nacos already cover the hyperscale case): in a single-company multi-game shape, croupier's DB version table + subscription push suffices within ~a thousand nodes; if some game exceeds a thousand nodes, introduce Nacos as a bypass — the two do not conflict (croupier remains the approval and audit entry).

## 3. Design (croupier ConfigVersion strengthened)

### 3.1 Current State and Gaps to Fill

The `ConfigVersion` model (game-scoped) already exists. Three things to add: **controlled namespaces, a version subscription protocol, and the handoff to the Excel pipeline**.

### 3.2 Data Model Extension

```
ConfigVersion (existing, strengthened)
├── GameID/Env (existing)
├── Namespace   controlled tiers: gameplay(numeric) | runtime(switches) | activity | iap | ops
├── Key         e.g. runtime/login.captchaEnabled
├── Value       JSON (validated by SchemaRoutes — a schema service already exists!)
├── Version     monotonically increasing (per namespace+key)
├── Status      draft → published → rolled_back (approval reuses approvals)
├── GrayTarget  optional canary: a set of server (区服) labels (reuses assignment selectors)
└── Audit       operator / source (manual|pipeline) / approval ticket number
```

### 3.3 Distribution and Subscription Protocol

```
Game server / agent:
  GET /api/v1/configs?namespace=runtime&version>=N   (incremental pull; the configs route base already exists)
  SSE /api/v1/configs/watch?namespaces=...            (reuses the existing SSE infrastructure; changes push immediately)
Client:
  GET /api/v1/public/configs?ns=iap&clientVersion=..  (public read-only endpoint, styled after the release public layer)
```

Game-server-side consumption patterns (framework-agnostic):

- **Pull mode**: full fetch at startup + periodic (30s) version comparison;
- **Push mode**: SSE subscription to change notifications → triggers a pull (loss-proof: notifications carry only version numbers; data always comes from a pull);
- Application callbacks are registered by the game side (`onConfigChange(ns, key)`); croupier does not care about the framework.

### 3.4 Excel Pipeline Handoff (the numeric-table channel)

```
Designers' Excel (Git/SVN)
  → CI export (Luban recommended: Excel → lua/json/bin + schema)
  → artifact upload: POST /api/v1/configs/artifact (reuses objstore+checksum, same channel as the release artifact)
  → registered platform-side as a ConfigVersion with namespace=gameplay (value={manifest, downloadUrl, checksum})
  → game server gets notified → pulls the manifest → downloads on demand → framework reload
     (skynet: re-require the config module; KBE: re-read tables; Node: clear require.cache)
     — reload details belong to the adapter system in hot-patch-design.md
```

The platform does not parse Excel (exporting is CI's job); it only does artifact hosting, versioning, per-server canary, audit, and notification.

### 3.5 Boundaries with Related Systems

| System             | Boundary                                                                                                                                             |
| ------------------ | ---------------------------------------------------------------------------------------------------------------------------------------------------- |
| release-management | Client resource packs (including client config tables) go through release; server config goes through this document                                  |
| hot-patch          | When "applying" config requires a reload, the hot-patch adapter performs it; this document owns only the data channel                                |
| feature-flags      | Switches in the platform's own domain go through featureFlags (deployment-level); in-game business switches go through ConfigVersion (runtime-level) |
| SchemaRoutes       | ConfigVersion value validation reuses the schema service (JSON Schema already exists)                                                                |

## 4. Phases

| Phase           | Content                                                                                                                                                                      |
| --------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **P0 (landed)** | Namespace/canary fields + version subscription SSE watch + public client pull endpoint + tests                                                                               |
| P1              | Excel artifact channel (artifact upload → ConfigVersion registration → manifest distribution); agent-side pull + callback SDK (Go)                                           |
| P2              | Per-server-label canary (reusing assignment); config diff preview and one-click rollback UI                                                                                  |
| P3              | Multi-language SDK (lua/python/js) subscription libraries; config change impact analysis (the list of functions referencing a key, linked to function registration metadata) |

## 5. Review Checklist

- The namespace is a controlled enum (frontend grouping and validation depend on it); new values must be synchronized front and back;
- value must pass JSON Schema validation (reusing the schemas service); keys without a registered schema are refused publication;
- subscription notifications carry only version numbers, never data (notifications may drop; data must be pulled);
- rollback is not deletion: it produces a new version pointing at the old value (versions stay monotonic);
- public endpoints must not expose draft/rolled_back version contents.
