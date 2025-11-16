# Assignments (Per Game/Env Function Sets)

This document describes a minimal control-plane for function assignments per game/env. It is an early building block for M5 (pack downlink & hot update).

Concept
- Assignments is a mapping from `game_id|env` to an array of `function_id`s.
- Server persists this mapping to `<packDir>/assignments.json` and exposes HTTP APIs to get/set it.
- Agents can later poll this mapping to decide which adapters/packs to activate (future work).

Server APIs
- GET `/api/assignments?game_id=&env=`
  - Returns `{ assignments: { "<game>|<env>": ["fn1","fn2",...] } }`.
  - Filters are optional; when omitted, returns all entries.
- POST `/api/assignments`
  - Body: `{ "game_id": "<game>", "env": "<env>", "functions": ["fn1","fn2"] }`
  - Overwrites the mapping for the given key; persists to `assignments.json`.
  - Response: `{ ok: true, unknown: ["fnX", ...] }` where `unknown` lists function ids that are not present in the current descriptors and were ignored.

CLI
- List assignments:
```
./bin/croupier assignments list --api http://localhost:8080
./bin/croupier assignments list --game_id mygame --env prod --api http://localhost:8080
```
- Set assignments:
```
./bin/croupier assignments set \
  --game_id mygame --env prod \
  --functions prom.query,prom.query_range \
  --api http://localhost:8080
```

Web UI
- Configure via GM → Assignments: choose game/env and select function ids (empty means allow all). Save to persist on the server.
- GM → Functions will auto-filter the function list by assignments when game/env is selected.

Notes
- This is a minimal, file-backed control-plane intended to unblock end-to-end demos.
- Future work (M5): Agent-side polling & hot (un)load of adapters/packs, Server-side validation & drift detection.
