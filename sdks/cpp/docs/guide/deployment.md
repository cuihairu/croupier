# 部署指南

本指南介绍如何部署使用 Croupier C++ SDK 的应用程序。

## 部署模式

### 模式对比

| 模式 | 复杂度 | 性能 | 适用场景 |
|------|--------|------|----------|
| 独立进程 | 低 | 中 | 小规模游戏 |
| 容器化 | 中 | 高 | 微服务架构 |
| 多实例 | 高 | 最高 | 大规模游戏 |

## 独立进程部署

### 目录结构

```
/opt/game-server/
├── bin/
│   ├── game-server          # 主程序
│   └── croupier-example     # 示例程序
├── lib/
│   └── libcroupier_sdk.so   # SDK 动态库
├── configs/
│   ├── development.json
│   └── production.json
└── scripts/
    ├── start.sh
    └── stop.sh
```

### 启动脚本

```bash
#!/bin/bash
# start.sh

export LD_LIBRARY_PATH=/opt/game-server/lib:$LD_LIBRARY_PATH
export CROUPIER_ENV=production
export CROUPIER_AGENT_ADDR=croupier-agent.internal:19090

/opt/game-server/bin/game-server
```

### 停止脚本

```bash
#!/bin/bash
# stop.sh

pkill -f game-server
```

### 系统服务（systemd）

```ini
[Unit]
Description=Game Server with Croupier SDK
After=network.target

[Service]
Type=simple
User=gameserver
WorkingDirectory=/opt/game-server
Environment="LD_LIBRARY_PATH=/opt/game-server/lib"
Environment="CROUPIER_ENV=production"
Environment="CROUPIER_AGENT_ADDR=croupier-agent.internal:19090"
ExecStart=/opt/game-server/bin/game-server
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

安装服务：
```bash
sudo cp game-server.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable game-server
sudo systemctl start game-server
```

## 容器化部署

### Dockerfile

```dockerfile
FROM ubuntu:22.04 AS base

# 安装运行时依赖
RUN apt-get update && apt-get install -y \
    libgrpc++1.54 \
    libprotobuf32 \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# 构建阶段
FROM base AS builder

RUN apt-get update && apt-get install -y \
    cmake \
    g++ \
    git \
    pkg-config \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /build
COPY . .

RUN cmake -B build \
    -DCMAKE_BUILD_TYPE=Release \
    -DBUILD_EXAMPLES=OFF \
    -DBUILD_TESTING=OFF
RUN cmake --build build --parallel
RUN cmake --install build --prefix /usr/local

# 运行阶段
FROM base

# 复制构建产物
COPY --from=builder /usr/local/lib /usr/local/lib
COPY --from=builder /usr/local/bin /usr/local/bin

# 复制配置
COPY configs/ /etc/croupier/
COPY bin/game-server /app/

ENV LD_LIBRARY_PATH=/usr/local/lib
ENV CROUPIER_ENV=production
ENV CROUPIER_AGENT_ADDR=croupier-agent:19090

WORKDIR /app
EXPOSE 8080

CMD ["game-server"]
```

### 构建镜像

```bash
docker build -t game-server:latest .
```

### Docker Compose

```yaml
version: '3.8'

services:
  croupier-agent:
    image: croupier/agent:latest
    ports:
      - "19090:19090"
    environment:
      - CROUPIER_SERVER_ADDR=croupier-server:8443

  game-server:
    build: .
    depends_on:
      - croupier-agent
    environment:
      - CROUPIER_ENV=production
      - CROUPIER_AGENT_ADDR=croupier-agent:19090
      - CROUPIER_GAME_ID=production-game
    restart: unless-stopped
```

运行：
```bash
docker-compose up -d
```

## Kubernetes 部署

### Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: game-server
  namespace: game
spec:
  replicas: 3
  selector:
    matchLabels:
      app: game-server
  template:
    metadata:
      labels:
        app: game-server
    spec:
      containers:
      - name: game-server
        image: your-registry/game-server:v1.0.0
        ports:
        - containerPort: 8080
        env:
        - name: CROUPIER_ENV
          value: "production"
        - name: CROUPIER_AGENT_ADDR
          value: "croupier-agent.game.svc.cluster.local:19090"
        - name: CROUPIER_GAME_ID
          valueFrom:
            configMapKeyRef:
              name: game-config
              key: game-id
        - name: CROUPIER_AUTH_TOKEN
          valueFrom:
            secretKeyRef:
              name: croupier-auth
              key: jwt-token
        resources:
          requests:
            memory: "256Mi"
            cpu: "500m"
          limits:
            memory: "512Mi"
            cpu: "1000m"
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /ready
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5
---
apiVersion: v1
kind: Service
metadata:
  name: game-server
  namespace: game
spec:
  selector:
    app: game-server
  ports:
  - port: 80
    targetPort: 8080
  type: ClusterIP
```

### ConfigMap

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: game-config
  namespace: game
data:
  game-id: "production-game"
  env: "production"
  log-level: "info"
```

### Secret

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: croupier-auth
  namespace: game
type: Opaque
data:
  jwt-token: <base64-encoded-token>
```

## 负载均衡

### Nginx 配置

```nginx
upstream game_servers {
    least_conn;
    server game-server-1:8080;
    server game-server-2:8080;
    server game-server-3:8080;
}

server {
    listen 80;
    server_name game.example.com;

    location / {
        proxy_pass http://game_servers;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Game-ID $http_x_game_id;
    }
}
```

## 监控和日志

### 健康检查端点

```cpp
#include <cpprest/http_listener.h>

class HealthCheckServer {
public:
    void Start() {
        http_listener listener("http://0.0.0.0:8080/health");
        listener.support([](http_request request) {
            if (IsHealthy()) {
                request.reply(status_codes::OK, R"({"status": "healthy"})");
            } else {
                request.reply(status_codes::ServiceUnavailable, R"({"status": "unhealthy"})");
            }
        });
        listener.open().wait();
    }

private:
    bool IsHealthy() {
        // 检查 Croupier 连接状态
        return client_.IsConnected();
    }

    CroupierClient& client_;
};
```

### 日志配置

```cpp
#include <spdlog/spdlog.h>

void SetupLogging() {
    // 根据环境设置日志级别
    std::string env = std::getenv("CROUPIER_ENV") ?: "development";

    if (env == "production") {
        spdlog::set_level(spdlog::level::info);
    } else {
        spdlog::set_level(spdlog::level::debug);
    }

    // 输出到文件
    auto logger = spdlog::basic_logger_mt("file_logger", "/var/log/game-server.log");
    spdlog::set_default_logger(logger);
}
```

## 安全加固

### TLS 配置

```json
{
  "game_id": "secure-game",
  "env": "production",
  "agent_addr": "croupier-agent:19090",
  "insecure": false,
  "security": {
    "cert_file": "/etc/tls/client.crt",
    "key_file": "/etc/tls/client.key",
    "ca_file": "/etc/tls/ca.crt",
    "server_name": "croupier.internal"
  }
}
```

### Secret 管理

使用 Kubernetes Secrets 或 Vault 管理敏感信息：

```bash
# 创建 Secret
kubectl create secret generic croupier-secrets \
  --from-literal=jwt-token='your-jwt-token' \
  --from-literal=api-key='your-api-key' \
  -n game
```

## 高可用配置

### 多区域部署

```yaml
# 不同区域的 Agent 配置
{
  "agent_addrs": [
    "croupier-agent-us-east:19090",
    "croupier-agent-us-west:19090",
    "croupier-agent-eu-west:19090"
  ],
  "reconnect_max_attempts": 0,
  "reconnect_interval_seconds": 5
}
```

### 故障转移

```cpp
class FailoverClient : public CroupierClient {
public:
    void ConnectWithFailover(const std::vector<std::string>& agents) {
        for (const auto& agent : agents) {
            config_.agent_addr = agent;
            if (Connect()) {
                SPDLOG_INFO("Connected to {}", agent);
                return;
            }
            SPDLOG_WARN("Failed to connect to {}, trying next...", agent);
        }
        throw std::runtime_error("All agents failed");
    }
};
```
