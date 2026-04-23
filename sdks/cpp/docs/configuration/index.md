# 配置指南

Croupier C++ SDK 支持灵活的配置系统，包括文件配置、环境变量覆盖和配置验证。

## 目录

- [客户端配置](./client-config.md) - ClientConfig 完整参考
- [环境配置](./environments.md) - 多环境配置管理
- [安全配置](./security.md) - TLS 和认证配置

## 快速开始

### 基础配置文件

```json
{
  "game_id": "demo-game",
  "env": "development",
  "service_id": "demo-service",
  "agent_addr": "127.0.0.1:19090",
  "insecure": true
}
```

### 加载配置

```cpp
#include "croupier/sdk/config/client_config_loader.h"

ClientConfigLoader loader;
ClientConfig config = loader.LoadFromFile("./config.json");
```

### 环境变量覆盖

```bash
export CROUPIER_GAME_ID=production-game
export CROUPIER_ENV=production
export CROUPIER_AGENT_ADDR=croupier-agent:19090

./your-app
```

## 配置加载器

### ClientConfigLoader 方法

| 方法 | 说明 |
|------|------|
| `LoadFromFile(path)` | 从 JSON 文件加载 |
| `LoadFromJson(json)` | 从 JSON 字符串加载 |
| `LoadWithEnvironmentOverrides(path, prefix)` | 加载并应用环境变量 |
| `LoadProfile(dir, profile)` | 加载配置 profile |
| `ValidateConfig(config)` | 验证配置 |

### 示例

```cpp
ClientConfigLoader loader;

// 文件加载
ClientConfig config = loader.LoadFromFile("./production.json");

// 环境变量覆盖
ClientConfig config = loader.LoadWithEnvironmentOverrides(
    "./production.json",
    "CROUPIER_"
);

// 验证
auto errors = loader.ValidateConfig(config);
if (!errors.empty()) {
    for (const auto& error : errors) {
        std::cerr << error << std::endl;
    }
}
```

## 环境变量

所有配置字段都可以通过环境变量覆盖，使用 `CROUPIER_` 前缀：

| 配置字段 | 环境变量 |
|----------|----------|
| `game_id` | `CROUPIER_GAME_ID` |
| `env` | `CROUPIER_ENV` |
| `agent_addr` | `CROUPIER_AGENT_ADDR` |
| `service_id` | `CROUPIER_SERVICE_ID` |
| `insecure` | `CROUPIER_INSECURE` |
| `timeout_seconds` | `CROUPIER_TIMEOUT_SECONDS` |
| `auto_reconnect` | `CROUPIER_AUTO_RECONNECT` |
| `reconnect_interval_seconds` | `CROUPIER_RECONNECT_INTERVAL_SECONDS` |

### 嵌套字段

```bash
# 安全配置
export CROUPIER_CERT_FILE="/path/to/cert.pem"
export CROUPIER_KEY_FILE="/path/to/key.pem"
export CROUPIER_CA_FILE="/path/to/ca.pem"

# 认证配置
export CROUPIER_AUTH_TOKEN="Bearer token123"
```

## 配置文件位置

SDK 按以下顺序查找配置文件：

1. 当前目录：`./config.json`
2. 用户目录：`~/.croupier/config.json`
3. 系统目录：`/etc/croupier/config.json`

可以通过命令行或代码指定：

```cpp
// 指定配置文件
ClientConfig config = loader.LoadFromFile("/path/to/config.json");
```

## 配置验证

配置加载器会自动验证：

- ✅ 必需字段存在
- ✅ 网络地址格式正确
- ✅ 文件路径存在
- ✅ 环境名称有效
- ✅ 数值范围合理

### 自定义验证

```cpp
class CustomValidator {
public:
    std::vector<std::string> Validate(const ClientConfig& config) {
        std::vector<std::string> errors;

        if (config.game_id.empty()) {
            errors.push_back("game_id 不能为空");
        }

        if (config.env != "production" && config.env != "staging") {
            errors.push_back("env 必须是 production 或 staging");
        }

        if (config.timeout_seconds < 1 || config.timeout_seconds > 300) {
            errors.push_back("timeout_seconds 必须在 1-300 之间");
        }

        return errors;
    }
};
```

## 下一步

- [客户端配置详解](./client-config.md)
- [多环境配置](./environments.md)
- [安全配置](./security.md)
