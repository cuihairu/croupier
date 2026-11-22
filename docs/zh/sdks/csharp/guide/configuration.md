# 配置

`ClientConfig` 是 C# SDK 的核心配置对象。

## 主要选项

- `AgentAddr`
- `ServiceId`
- `ServiceVersion`
- `GameId`
- `Env`
- `Insecure` — `true`（默认）明文；`false` 时 C# TCP transport **尚未实现** TLS，连接被显式拒绝（`NotSupportedException`），绝不静默降级（AUD-014）
- `TimeoutSeconds`
- `HeartbeatIntervalSeconds`
- `AutoReconnect`
