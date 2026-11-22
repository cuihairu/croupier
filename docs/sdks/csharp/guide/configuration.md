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
