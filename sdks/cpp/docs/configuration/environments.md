# 多环境配置

管理不同环境（开发、测试、生产）的配置。

## 目录结构

```
configs/
├── base.json           # 基础配置
├── development.json    # 开发环境
├── staging.json        # 预发布环境
└── production.json     # 生产环境
```

## 基础配置 (base.json)

所有环境共享的配置：

```json
{
  "service_id": "croupier-game-server",
  "timeout_seconds": 30,
  "auto_reconnect": true,
  "reconnect_interval_seconds": 5,
  "reconnect_max_attempts": 0
}
```

## 开发环境 (development.json)

```json
{
  "extends": "base.json",
  "game_id": "dev-game",
  "env": "development",
  "agent_addr": "127.0.0.1:19090",
  "insecure": true,
  "timeout_seconds": 30
}
```

## 预发布环境 (staging.json)

```json
{
  "extends": "base.json",
  "game_id": "staging-game",
  "env": "staging",
  "agent_addr": "staging-agent.internal:19090",
  "insecure": false,
  "timeout_seconds": 60,
  "security": {
    "cert_file": "/etc/tls/staging/client.crt",
    "key_file": "/etc/tls/staging/client.key",
    "ca_file": "/etc/tls/staging/ca.crt",
    "server_name": "staging-agent.internal"
  }
}
```

## 生产环境 (production.json)

```json
{
  "extends": "base.json",
  "game_id": "production-game",
  "env": "production",
  "agent_addr": "croupier-agent.internal:19090",
  "insecure": false,
  "timeout_seconds": 60,
  "security": {
    "cert_file": "/etc/tls/production/client.crt",
    "key_file": "/etc/tls/production/client.key",
    "ca_file": "/etc/tls/production/ca.crt",
    "server_name": "croupier-agent.internal"
  },
  "auth": {
    "token": "Bearer ${JWT_TOKEN}",
    "headers": {
      "X-Game-Version": "0.1.0",
      "X-Service-Region": "us-west-2"
    }
  }
}
```

## 加载配置

### 指定环境

```cpp
#include "croupier/sdk/config/client_config_loader.h"

int main(int argc, char* argv[]) {
    // 默认开发环境
    std::string env = "development";
    if (argc > 1) {
        env = argv[1];  // 从命令行获取
    }

    ClientConfigLoader loader;
    ClientConfig config = loader.LoadProfile("./configs", env);

    CroupierClient client(config);
    // ...
}
```

### 使用环境变量

```cpp
// 从环境变量读取
const char* env = std::getenv("CROUPIER_ENV");
if (!env) env = "development";

ClientConfig config = loader.LoadProfile("./configs", env);
```

### 程序运行时切换

```bash
# 开发环境
./game-server development

# 预发布环境
./game-server staging

# 生产环境
./game-server production
```

## 配置合并

当加载 profile 时，配置按以下顺序合并：

1. 加载 `base.json`（如果存在）
2. 加载 `{env}.json`
3. 应用环境变量覆盖

### 示例

```cpp
ClientConfigLoader loader;
ClientConfig config = loader.LoadProfile("./configs", "production");

// 等价于：
// 1. 加载 base.json
// 2. 加载 production.json（覆盖 base.json 中的值）
// 3. 应用 CROUPIER_* 环境变量（再覆盖）
```

## 环境特定覆盖

### 开发环境特有

```json
{
  "extends": "base.json",
  "game_id": "dev-game",
  "env": "development",
  "debug": true,
  "log_level": "debug",
  "mock_mode": true
}
```

### 生产环境特有

```json
{
  "extends": "base.json",
  "game_id": "prod-game",
  "env": "production",
  "debug": false,
  "log_level": "info",
  "metrics_enabled": true
}
```

## Kubernetes ConfigMap

### 创建 ConfigMap

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: game-config
data:
  production.json: |
    {
      "extends": "base.json",
      "game_id": "prod-game",
      "env": "production",
      "agent_addr": "croupier-agent:19090",
      "insecure": false
    }
```

### 挂载到 Pod

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: game-server
spec:
  containers:
  - name: game-server
    image: game-server:latest
    volumeMounts:
    - name: config
      mountPath: /etc/croupier/configs
  volumes:
  - name: config
    configMap:
      name: game-config
```

## Docker 环境

### Dockerfile

```dockerfile
FROM ubuntu:22.04

COPY configs/ /etc/croupier/configs/
ENV CROUPIER_ENV=production

ENTRYPOINT ["game-server"]
```

### docker-compose.yml

```yaml
version: '3.8'

services:
  game-server:
    image: game-server:latest
    environment:
      - CROUPIER_ENV=development
      - CROUPIER_AGENT_ADDR=croupier-agent:19090
    volumes:
      - ./configs:/etc/croupier/configs
```

## 最佳实践

### 1. 敏感信息不写入配置文件

```json
// 不要这样做
{
  "auth": {
    "token": "hardcoded-token"
  }
}

// 使用环境变量
{
  "auth": {
    "token": "Bearer ${JWT_TOKEN}"
  }
}
```

### 2. 使用配置验证

```cpp
auto errors = loader.ValidateConfig(config);
if (!errors.empty()) {
    std::cerr << "配置错误:" << std::endl;
    for (const auto& error : errors) {
        std::cerr << "  - " << error << std::endl;
    }
    exit(1);
}
```

### 3. 配置文件版本控制

```
configs/
├── base.json           # 提交到 Git
├── development.json    # 提交到 Git
├── production.example.json  # 提交到 Git（示例）
└── production.json     # 不提交（包含真实密钥）
```

### 4. 使用 .gitignore

```
configs/production.json
configs/staging.json
*.secret.json
```
