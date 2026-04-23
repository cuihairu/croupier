# ClientConfig API 详解

`ClientConfig` 结构定义了 Croupier 客户端的所有配置选项。

## 结构定义

```cpp
namespace croupier::sdk {

struct ClientConfig {
    // 基础连接配置
    std::string agent_addr = "127.0.0.1:19090";
    std::string game_id;
    std::string env = "development";
    std::string service_id = "cpp-service";

    // 超时配置
    int timeout_seconds = 30;

    // TLS 配置
    bool insecure = true;
    struct Security {
        std::string cert_file;
        std::string key_file;
        std::string ca_file;
        std::string server_name;
    } security;

    // 重连配置
    bool auto_reconnect = true;
    int reconnect_interval_seconds = 5;
    int reconnect_max_attempts = 0;  // 0 = 无限重试

    // 认证配置
    struct Auth {
        std::string token;
        std::map<std::string, std::string> headers;
    } auth;
};

} // namespace croupier::sdk
```

## 配置字段详解

### 基础连接配置

#### agent_addr

Agent 服务器地址，格式为 `host:port`。

```cpp
config.agent_addr = "127.0.0.1:19090";           // 本地 Agent
config.agent_addr = "croupier-agent:19090";      // Kubernetes 服务
config.agent_addr = "10.0.0.5:19090";            // IP 地址
```

#### game_id

游戏/项目唯一标识符。

```cpp
config.game_id = "my-mmorpg";
config.game_id = "production-game-2024";
```

#### env

部署环境标识。

```cpp
config.env = "development";  // 开发环境
config.env = "staging";      // 预发布环境
config.env = "production";   // 生产环境
```

#### service_id

服务实例标识符，用于区分同一服务的多个实例。

```cpp
config.service_id = "game-server-01";
config.service_id = "backend-service-" + std::to_string(GetProcessID());
```

---

### 超时配置

#### timeout_seconds

网络操作超时时间（秒）。

```cpp
config.timeout_seconds = 30;   // 默认 30 秒
config.timeout_seconds = 60;   // 1 分钟
config.timeout_seconds = 300;  // 5 分钟（长操作）
```

---

### TLS 配置

#### insecure

是否跳过 TLS 验证（仅用于开发）。

```cpp
config.insecure = true;   // 开发环境：跳过验证
config.insecure = false;  // 生产环境：启用 TLS
```

#### security.cert_file

客户端证书文件路径。

```cpp
config.security.cert_file = "/etc/tls/client.crt";
config.security.cert_file = "./certs/client-cert.pem";
```

#### security.key_file

客户端私钥文件路径。

```cpp
config.security.key_file = "/etc/tls/client.key";
config.security.key_file = "./certs/client-key.pem";
```

#### security.ca_file

CA 证书文件路径。

```cpp
config.security.ca_file = "/etc/tls/ca.crt";
config.security.ca_file = "./certs/ca-bundle.crt";
```

#### security.server_name

TLS 服务器名称验证（SNI）。

```cpp
config.security.server_name = "croupier.internal";
config.security.server_name = "agent.production.com";
```

---

### 重连配置

#### auto_reconnect

是否自动重连。

```cpp
config.auto_reconnect = true;   // 连接断开时自动重连
config.auto_reconnect = false;  // 连接断开时退出
```

#### reconnect_interval_seconds

重连尝试间隔（秒）。

```cpp
config.reconnect_interval_seconds = 5;   // 每 5 秒重试一次
config.reconnect_interval_seconds = 30;  // 每 30 秒重试一次
```

#### reconnect_max_attempts

最大重连次数（0 表示无限重试）。

```cpp
config.reconnect_max_attempts = 0;    // 无限重试
config.reconnect_max_attempts = 10;   // 最多尝试 10 次
config.reconnect_max_attempts = -1;   // 不重连
```

---

### 认证配置

#### auth.token

JWT 认证令牌。

```cpp
config.auth.token = "Bearer eyJhbGciOiJIUzI1NiIs...";
```

#### auth.headers

额外的 HTTP 头部。

```cpp
config.auth.headers = {
    {"X-Game-Version", "0.1.0"},
    {"X-Service-Region", "us-west-2"},
    {"X-Cluster", "production-01"}
};
```

## 配置加载

### 从文件加载

```cpp
#include "croupier/sdk/config/client_config_loader.h"

ClientConfigLoader loader;
ClientConfig config = loader.LoadFromFile("./config.json");
```

### 从环境变量覆盖

```cpp
ClientConfig config = loader.LoadWithEnvironmentOverrides(
    "./config.json",
    "CROUPIER_"  // 环境变量前缀
);

// 环境变量:
// CROUPIER_GAME_ID=override-game
// CROUPIER_ENV=staging
// CROUPIER_AGENT_ADDR=10.0.0.5:19090
```

## 配置验证

```cpp
ClientConfigLoader loader;
auto errors = loader.ValidateConfig(config);

if (!errors.empty()) {
    for (const auto& error : errors) {
        std::cerr << "配置错误: " << error << std::endl;
    }
    return 1;
}
```

## 配置示例

### 开发环境

```json
{
  "game_id": "dev-game",
  "env": "development",
  "service_id": "dev-backend",
  "agent_addr": "127.0.0.1:19090",
  "insecure": true,
  "timeout_seconds": 30,
  "auto_reconnect": true
}
```

### 生产环境

```json
{
  "game_id": "prod-game",
  "env": "production",
  "service_id": "prod-backend-01",
  "agent_addr": "croupier-agent.internal:19090",
  "insecure": false,
  "timeout_seconds": 60,
  "security": {
    "cert_file": "/etc/tls/client.crt",
    "key_file": "/etc/tls/client.key",
    "ca_file": "/etc/tls/ca.crt",
    "server_name": "croupier.internal"
  },
  "auth": {
    "token": "Bearer ${JWT_TOKEN}",
    "headers": {
      "X-Game-Version": "2.1.0",
      "X-Service-Region": "us-west-2"
    }
  },
  "auto_reconnect": true,
  "reconnect_interval_seconds": 10,
  "reconnect_max_attempts": 0
}
```

### Kubernetes 环境

```json
{
  "game_id": "${GAME_ID}",
  "env": "production",
  "service_id": "${HOSTNAME}",
  "agent_addr": "croupier-agent:19090",
  "insecure": false,
  "security": {
    "cert_file": "/etc/tls/client.crt",
    "key_file": "/etc/tls/client.key",
    "ca_file": "/etc/tls/ca.crt"
  },
  "auth": {
    "token": "Bearer ${JWT_TOKEN}"
  }
}
```
