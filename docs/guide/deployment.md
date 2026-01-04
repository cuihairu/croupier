---
title: 部署指南
icon: rocket
order: 4
category:
  - 入门指南
tag:
  - 部署
  - 运维
  - 生产环境
---

# 部署指南

本文档介绍 Croupier 在生产环境中的部署方案和最佳实践。

## 目录

[[toc]]

## 部署架构

### 推荐架构

```mermaid
graph TB
    subgraph "DMZ / 公网区"
        LB[负载均衡器<br/>Nginx/ALB]
        Edge[Edge Proxy<br/>可选]
    end

    subgraph "内网 - 管理区"
        ServerHA[Croupier Server<br/>高可用集群]
        DB[(PostgreSQL<br/>主从)]
        Redis[(Redis<br/>缓存/队列)]
    end

    subgraph "内网 - 游戏区"
        Agent1[Agent 1<br/>游戏服务器 A]
        Agent2[Agent 2<br/>游戏服务器 B]
        Agent3[Agent 3<br/>游戏服务器 C]
    end

    LB --> Edge
    Edge --> ServerHA
    LB --> ServerHA
    ServerHA --> DB
    ServerHA --> Redis
    ServerHA --> Agent1
    ServerHA --> Agent2
    ServerHA --> Agent3
```

## 部署模式

### 单机房部署

适用于小规模或单一区域部署。

```
┌─────────────────────────────────────────┐
│           数据中心                       │
│  ┌─────────────────────────────────┐   │
│  │  管理网段 (内网)                │   │
│  │  Server HA (3 节点)            │   │
│  │         │                       │   │
│  │    PostgreSQL + Redis          │   │
│  └─────────────────────────────────┘   │
│  ┌─────────────────────────────────┐   │
│  │  游戏网段 (内网)                │   │
│  │  Agent + Game Server (多节点)  │   │
│  └─────────────────────────────────┘   │
└─────────────────────────────────────────┘
```

### 多机房部署

适用于跨区域部署，需要 Edge 组件。

```
┌──────────────┐       ┌──────────────┐
│  机房 A       │       │  机房 B       │
│  Server HA   │◄─────►│  Server HA   │
│  │           │       │  │           │
│  Agent       │       │  Agent       │
│  │           │       │  │           │
│  Game Server │       │  Game Server │
└──────────────┘       └──────────────┘
       │                       │
       └───────────┬───────────┘
                   │
              ┌─────────┐
              │  Edge   │
              │  (DMZ)  │
              └─────────┘
```

## Docker 部署

### Server 部署

#### Docker Compose

```yaml
# docker-compose.yml
version: '3.8'

services:
  postgres:
    image: postgres:16
    environment:
      POSTGRES_DB: croupier
      POSTGRES_USER: croupier
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD}
    volumes:
      - postgres_data:/var/lib/postgresql/data
    ports:
      - "5432:5432"

  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"

  server:
    image: croupier-server:latest
    ports:
      - "8443:8443"
      - "8080:8080"
    environment:
      - DATABASE_URL=postgres://croupier:${POSTGRES_PASSWORD}@postgres:5432/croupier?sslmode=disable
      - CROUPIER_SERVER_ADDR=:8443
      - CROUPIER_SERVER_HTTP_ADDR=:8080
    volumes:
      - ./configs:/app/configs
      - ./data:/app/data
    depends_on:
      - postgres
      - redis

volumes:
  postgres_data:
```

#### 单独部署

```bash
# 构建镜像
docker build -t croupier-server:latest -f docker/Dockerfile.server .

# 运行容器
docker run -d \
  --name croupier-server \
  -p 8443:8443 \
  -p 8080:8080 \
  -v $(pwd)/data:/app/data \
  -v $(pwd)/configs:/app/configs \
  -e DATABASE_URL="postgres://croupier:password@postgres:5432/croupier" \
  croupier-server:latest
```

### Agent 部署

```bash
# 运行 Agent 容器
docker run -d \
  --name croupier-agent \
  --network host \
  -v $(pwd)/configs:/app/configs \
  -e CROUPIER_AGENT_SERVER_ADDR="server:8443" \
  -e CROUPIER_AGENT_GAME_ID="my-game" \
  -e CROUPIER_AGENT_ENV="prod" \
  croupier-agent:latest
```

## Kubernetes 部署

### Namespace

```yaml
# namespace.yaml
apiVersion: v1
kind: Namespace
metadata:
  name: croupier
```

### ConfigMap

```yaml
# configmap.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: croupier-config
  namespace: croupier
data:
  server.yaml: |
    server:
      addr: ":8443"
      http_addr: ":8080"
      db:
        driver: postgres
      log:
        level: info
        format: json
```

### Secret

```yaml
# secret.yaml
apiVersion: v1
kind: Secret
metadata:
  name: croupier-secret
  namespace: croupier
type: Opaque
stringData:
  database-url: "postgres://croupier:password@postgres:5432/croupier"
  jwt-secret: "your-jwt-secret"
```

### Server Deployment

```yaml
# server-deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: croupier-server
  namespace: croupier
spec:
  replicas: 3
  selector:
    matchLabels:
      app: croupier-server
  template:
    metadata:
      labels:
        app: croupier-server
    spec:
      containers:
      - name: server
        image: croupier-server:latest
        ports:
        - containerPort: 8443
          name: grpc
        - containerPort: 8080
          name: http
        env:
        - name: DATABASE_URL
          valueFrom:
            secretKeyRef:
              name: croupier-secret
              key: database-url
        - name: JWT_SECRET
          valueFrom:
            secretKeyRef:
              name: croupier-secret
              key: jwt-secret
        volumeMounts:
        - name: config
          mountPath: /app/configs
        - name: data
          mountPath: /app/data
        resources:
          requests:
            memory: "256Mi"
            cpu: "500m"
          limits:
            memory: "1Gi"
            cpu: "2000m"
        livenessProbe:
          httpGet:
            path: /healthz
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 30
        readinessProbe:
          httpGet:
            path: /readyz
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 10
      volumes:
      - name: config
        configMap:
          name: croupier-config
      - name: data
        persistentVolumeClaim:
          claimName: croupier-data
```

### Service

```yaml
# service.yaml
apiVersion: v1
kind: Service
metadata:
  name: croupier-server
  namespace: croupier
spec:
  selector:
    app: croupier-server
  ports:
  - name: grpc
    port: 8443
    targetPort: 8443
  - name: http
    port: 8080
    targetPort: 8080
  type: ClusterIP
```

### Ingress

```yaml
# ingress.yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: croupier-ingress
  namespace: croupier
  annotations:
    nginx.ingress.kubernetes.io/grpc-backend: "true"
    nginx.ingress.kubernetes.io/ssl-redirect: "true"
    cert-manager.io/cluster-issuer: "letsencrypt-prod"
spec:
  ingressClassName: nginx
  tls:
  - hosts:
    - croupier.example.com
    secretName: croupier-tls
  rules:
  - host: croupier.example.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: croupier-server
            port:
              number: 8080
```

## 二进制部署

### 系统服务配置

#### systemd (Linux)

```ini
# /etc/systemd/system/croupier-server.service
[Unit]
Description=Croupier Server
After=network.target postgresql.service

[Service]
Type=simple
User=croupier
Group=croupier
WorkingDirectory=/opt/croupier
ExecStart=/opt/croupier/bin/croupier-server --config /etc/croupier/server.yaml
Restart=always
RestartSec=5

Environment="DATABASE_URL=postgres://croupier:password@localhost:5432/croupier"

# 安全加固
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/log/croupier /var/lib/croupier

[Install]
WantedBy=multi-user.target
```

启动服务：

```bash
# 重载配置
sudo systemctl daemon-reload

# 启用并启动服务
sudo systemctl enable --now croupier-server

# 查看状态
sudo systemctl status croupier-server

# 查看日志
sudo journalctl -u croupier-server -f
```

## 高可用配置

### 数据库高可用

#### PostgreSQL 主从复制

```bash
# 主库配置 (postgresql.conf)
wal_level = replica
max_wal_senders = 5
wal_keep_size = 1GB

# pg_hba.conf
host    replication     replicator      0.0.0.0/0      md5
```

#### 连接池 (PgBouncer)

```ini
[databases]
croupier = host=localhost port=5432 dbname=croupier

[pgbouncer]
listen_addr = 0.0.0.0
listen_port = 6432
auth_type = md5
auth_file = /etc/pgbouncer/userlist.txt
pool_mode = transaction
max_client_conn = 1000
default_pool_size = 50
```

### Redis 高可用

#### Redis Sentinel

```conf
# sentinel.conf
port 26379
sentinel monitor mymaster 127.0.0.1 6379 2
sentinel down-after-milliseconds mymaster 5000
sentinel parallel-syncs mymaster 1
sentinel failover-timeout mymaster 10000
```

## 负载均衡

### Nginx 配置

```nginx
upstream croupier_grpc {
    least_conn;
    server server1:8443 max_fails=3 fail_timeout=30s;
    server server2:8443 max_fails=3 fail_timeout=30s;
    server server3:8443 max_fails=3 fail_timeout=30s;
}

upstream croupier_http {
    least_conn;
    server server1:8080 max_fails=3 fail_timeout=30s;
    server server2:8080 max_fails=3 fail_timeout=30s;
    server server3:8080 max_fails=3 fail_timeout=30s;
}

# gRPC 代理
server {
    listen 8443 ssl http2;
    server_name croupier.example.com;

    ssl_certificate /etc/nginx/ssl/server.crt;
    ssl_certificate_key /etc/nginx/ssl/server.key;

    location / {
        grpc_pass grpc://croupier_grpc;
        grpc_set_header X-Real-IP $remote_addr;
    }
}

# HTTP 代理
server {
    listen 8080 ssl;
    server_name croupier.example.com;

    ssl_certificate /etc/nginx/ssl/server.crt;
    ssl_certificate_key /etc/nginx/ssl/server.key;

    location / {
        proxy_pass http://croupier_http;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    }
}
```

## 监控配置

### Prometheus

```yaml
# prometheus.yml
scrape_configs:
  - job_name: 'croupier-server'
    static_configs:
      - targets: ['server1:9090', 'server2:9090', 'server3:9090']
```

### Grafana Dashboard

导入 Croupier 提供的 Grafana 面板配置：

- Server 指标面板
- Agent 连接面板
- 函数调用面板
- 错误率面板

## 备份策略

### 数据库备份

```bash
# 每日全量备份
0 2 * * * pg_dump -U croupier croupier | gzip > /backup/croupier_$(date +\%Y\%m\%d).sql.gz

# 保留 30 天
find /backup -name "croupier_*.sql.gz" -mtime +30 -delete
```

### 配置备份

```bash
# 备份配置文件
rsync -av /etc/croupier/ /backup/configs/
```

## 安全加固

### TLS 证书

```bash
# 生成 CA
openssl genrsa -out ca.key 4096
openssl req -new -x509 -days 3650 -key ca.key -out ca.crt

# 生成服务器证书
openssl genrsa -out server.key 4096
openssl req -new -key server.key -out server.csr
openssl x509 -req -days 365 -in server.csr -CA ca.crt -CAkey ca.key -CAcreateserial -out server.crt

# 生成 Agent 证书
openssl genrsa -out agent.key 4096
openssl req -new -key agent.key -out agent.csr
openssl x509 -req -days 365 -in agent.csr -CA ca.crt -CAkey ca.key -out agent.crt
```

### 防火墙

```bash
# Server
ufw allow 22/tcp      # SSH
ufw allow 8080/tcp    # HTTP
ufw allow 8443/tcp    # gRPC
ufw enable

# Agent (出站连接即可)
ufw allow out 8443/tcp
```

## 健康检查

```bash
# HTTP 健康检查
curl http://localhost:8080/healthz

# 预期输出
# {"status":"ok"}

# 就绪检查
curl http://localhost:8080/readyz

# 版本信息
curl http://localhost:8080/version
```

## 故障排查

### 常见问题

| 问题 | 可能原因 | 解决方法 |
|------|----------|----------|
| 连接拒绝 | 端口未监听 | 检查配置和防火墙 |
| TLS 握手失败 | 证书过期/不匹配 | 更新证书 |
| 数据库连接失败 | 网络或认证问题 | 检查 DATABASE_URL |
| Agent 无法注册 | Server 地址错误 | 检查 server_addr |

### 日志查看

```bash
# Server 日志
journalctl -u croupier-server -n 100 -f

# Agent 日志
journalctl -u croupier-agent -n 100 -f

# Docker 日志
docker logs -f croupier-server

# Kubernetes 日志
kubectl logs -f deployment/croupier-server -n croupier
```

## 下一步

- [监控指南](./operations/monitoring.md) - 监控和告警配置
- [安全配置](./operations/security.md) - 安全加固指南
- [故障排查](./operations/troubleshooting.md) - 常见问题解决
