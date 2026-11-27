# Configuration

`ClientConfig` is the core configuration object of the C# SDK.

## Main Options

- `AgentAddr`
- `ServiceId`
- `ServiceVersion`
- `GameId`
- `Env`
- `Insecure` — plaintext when `true` (default); when `false` the C# TCP transport does **not** implement TLS and the connection is explicitly rejected (`NotSupportedException`), never silently downgraded (AUD-014)
- `TimeoutSeconds`
- `HeartbeatIntervalSeconds`
- `AutoReconnect`

## JSON File Configuration

`JsonFileConfigProvider` parses the file with `System.Text.Json` as standard JSON (AUD-104):

- Field names are matched case-insensitively against `ClientConfig` properties; the original PascalCase file layout keeps working, and camelCase is accepted too.
- Nested objects are supported — `Headers` is parsed as a JSON object into `Dictionary<string, string>`.
- Strings keep commas, escapes (`\"`, `\\`, `\uXXXX`), and empty values intact.
- Explicit values win: `"Insecure": false`, `"AutoReconnect": false`, and `"ReconnectMaxAttempts": 0` are applied as written; omitted fields keep their `ClientConfig` defaults.
- Syntax and type errors throw a `FormatException` whose message carries the file path plus line/position and the JSON path of the offending token, instead of silently normalizing bad values.

```json
{
  "AgentAddr": "127.0.0.1:19091",
  "ServiceId": "game-backend-01",
  "GameId": "demo",
  "Env": "development",
  "Insecure": false,
  "Headers": { "X-Game-Version": "1.0.0" },
  "TimeoutSeconds": 30,
  "AutoReconnect": true,
  "ReconnectMaxAttempts": 0
}
```
