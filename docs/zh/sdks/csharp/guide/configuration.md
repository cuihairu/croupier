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

## JSON 文件配置

`JsonFileConfigProvider` 用 `System.Text.Json` 按标准 JSON 解析配置文件（AUD-104）：

- 字段名对 `ClientConfig` 属性不区分大小写匹配；原有 PascalCase 文件写法继续有效，camelCase 也可用。
- 支持嵌套对象——`Headers` 按 JSON 对象解析进 `Dictionary<string, string>`。
- 字符串值保留逗号、转义序列（`\"`、`\\`、`\uXXXX`）与空字符串。
- 显式值生效：`"Insecure": false`、`"AutoReconnect": false`、`"ReconnectMaxAttempts": 0` 按书写值应用；未出现的字段保留 `ClientConfig` 默认值。
- 语法与类型错误抛 `FormatException`，消息携带文件路径、行列定位与出错 token 的 JSON 路径，不再静默归一坏值。

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
