# Analytics Ingest Signing Guide

The ingest service (`croupier-ingest`) rejects every request unless it carries a
valid HMAC signature. Each upload should set the following headers:

| Header        | Description                                                                    |
|---------------|--------------------------------------------------------------------------------|
| `X-Game-Id`   | Game identifier used to select the shared secret (per-tenant mapping).         |
| `X-Timestamp` | Unix timestamp (seconds). Replay protection fails if it drifts beyond the skew window (`ANALYTICS_INGEST_SKEW`, default 300s). |
| `X-Nonce`     | Random nonce string. The server stores `(game_id, nonce)` in Redis for the dedupe TTL to block replays. |
| `X-Signature` | `base64(HMAC_SHA256(secret, ts + "\n" + nonce + "\n" + sha256(body)))`         |

Example (TypeScript) helper:

```ts
import crypto from 'crypto';

export function buildIngestHeaders(secret: string, gameId: string, payload: string) {
  const ts = `${Math.floor(Date.now() / 1000)}`;
  const nonce = crypto.randomUUID();
  const bodyHash = crypto.createHash('sha256').update(payload).digest('hex');
  const msg = `${ts}\n${nonce}\n${bodyHash}`;
  const signature = crypto.createHmac('sha256', secret).update(msg).digest('base64');

  return {
    'X-Game-Id': gameId,
    'X-Timestamp': ts,
    'X-Nonce': nonce,
    'X-Signature': signature,
    'Content-Type': 'application/json',
  };
}
```

Secrets are configured via `ANALYTICS_INGEST_SECRETS` (JSON map) or the legacy
`--secret` flag for single-tenant setups. The ingest service also validates that
each event/payment payload includes `game_id`, `env`, `ts`, and other required
fields before enqueueing them.
