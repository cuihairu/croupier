# 配置

## ClientConfig 选项

`ClientConfig` 是 SDK 的核心配置类：

```csharp
public class ClientConfig
{
    public string AgentAddr { get; set; } = "127.0.0.1:19090";
    public string ServiceId { get; set; } = "csharp-service";
    public string ServiceVersion { get; set; } = "1.0.0";
    public string GameId { get; set; } = "default-game";
    public string Env { get; set; } = "dev";
    public string LocalAddr { get; set; } = "0.0.0.0:0";
    public bool Insecure { get; set; }
    public string? CertFile { get; set; }
    public string? KeyFile { get; set; }
    public string? CaFile { get; set; }
    public string? ServerName { get; set; }
    public int TimeoutSeconds { get; set; } = 30;
    public int HeartbeatIntervalSeconds { get; set; } = 30;
    public bool AutoReconnect { get; set; } = true;
    public int ReconnectIntervalSeconds { get; set; } = 5;
    public int MaxConcurrentMessages { get; set; } = 100;
    public int MaxMessageSize { get; set; } = 4 * 1024 * 1024;
}
```

## 环境变量

SDK 支持通过环境变量配置：

| 变量名 | 说明 | 默认值 |
|--------|------|--------|
| `CROUPIER_AGENT_ADDR` | Agent 地址 | 127.0.0.1:19090 |
| `CROUPIER_SERVICE_ID` | 服务 ID | csharp-service |
| `CROUPIER_SERVICE_VERSION` | 服务版本 | 1.0.0 |
| `CROUPIER_GAME_ID` | 游戏 ID | default-game |
| `CROUPIER_ENV` | 环境 | dev |
| `CROUPIER_LOCAL_ADDR` | 本地监听地址 | 0.0.0.0:0 |
| `CROUPIER_INSECURE` | 跳过 TLS | false |
| `CROUPIER_CERT_FILE` | 客户端证书 | - |
| `CROUPIER_KEY_FILE` | 客户端私钥 | - |
| `CROUPIER_CA_FILE` | CA 证书 | - |
| `CROUPIER_SERVER_NAME` | SNI 服务器名 | - |
| `CROUPIER_TIMEOUT_SECONDS` | 连接超时 | 30 |
| `CROUPIER_AUTO_RECONNECT` | 自动重连 | true |
| `CROUPIER_RECONNECT_INTERVAL_SECONDS` | 重连间隔 | 5 |

## 从环境变量加载

```csharp
using Croupier.Sdk.Configuration;

var configProvider = new EnvironmentConfigProvider();
var config = configProvider.GetConfig();
var client = new CroupierClient(config);
```

## JSON 文件配置

创建 `appsettings.json`：

```json
{
  "Croupier": {
    "AgentAddr": "127.0.0.1:19090",
    "ServiceId": "my-service",
    "GameId": "my-game",
    "Env": "production",
    "Insecure": false,
    "AutoReconnect": true
  }
}
```

使用 `IOptions<T>` 读取：

```csharp
builder.Services.Configure<ClientConfig>(
    builder.Configuration.GetSection("Croupier"));
```
