# Croupier Go SDK Full Flow (Server -> Agent -> Dashboard -> SDK Callback)

## 1. One-click startup (VSCode)

Use launch config:

- `Full Flow: Server + Agent + SDK basic + Dashboard`

This starts:

- Server: `http://localhost:18780` (TCP `19090`)
- Agent: `http://localhost:18888` (local TCP `19091`)
- SDK example (`examples/basic`): local provider `127.0.0.1:19101`
- Dashboard: `http://localhost:8000` (proxy `/api` -> `18780`)

## 2. What success looks like

### SDK log (examples/basic)

Should contain:

- `Successfully connected and registered with Agent`
- `Local service address: tcp://127.0.0.1:19101`
- `Registered functions: 2`

### Agent log

Should contain:

- `[agentlocal] Registered functions ... count=2`
- `[upstream] syncing functions ... function_count=2`

### Server API

Login:

```bash
POST http://localhost:18780/api/v1/auth/login
{"username":"admin","password":"admin123"}
```

List functions:

```bash
GET http://localhost:18780/api/v1/functions
Authorization: Bearer <token>
```

Should include:

- `player.ban`
- `item.create`

## 3. UI callback verification (Dashboard)

1. Open `http://localhost:8000`
2. Login with `admin / admin123`
3. Go to function list page, find `player.ban`
4. Trigger invoke for `player.ban`
5. Request payload example:

```json
{
  "gameId": "tower_defense",
  "env": "development",
  "payload": {
    "player_id": "p1001",
    "reason": "ui-test"
  }
}
```

Expected response contains:

- `status: success`
- `action: ban`
- `message: Player banned successfully`

## 4. Notes

- If SDK bind fails with port conflict, change:
  - `CROUPIER_LOCAL_LISTEN` in launch config (for example `127.0.0.1:19111`)
- If invoke returns `no live agent for function ...`, wait a few seconds for agent upstream sync, then retry.
