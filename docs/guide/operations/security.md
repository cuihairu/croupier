---
title: 安全配置
icon: shield-halved
order: 2
category:
  - 运维指南
tag:
  - 安全
  - TLS
  - 认证
---

# 安全配置

本文档介绍 Croupier 的安全配置和最佳实践。

## 目录

[[toc]]

## 安全架构

```mermaid
graph TB
    subgraph "客户端"
        UI[Dashboard<br/>HTTPS]
    end

    subgraph "Server"
        TLS[mTLS 验证]
        Auth[认证授权]
        Audit[审计日志]
    end

    subgraph "通信层"
        mTLS[Server ↔ Agent<br/>mTLS]
    end

    subgraph "数据层"
        Encrypt[(数据加密)]
    end

    UI -->|HTTPS| TLS
    TLS --> Auth
    Auth --> Audit
    TLS --> mTLS
```

## TLS/mTLS 配置

### 证书结构（统一使用一个 CA）

```
                            Root CA (ca.crt)
                                 |
                +----------------+----------------+
                |                                 |
        Server ← ← ← ← ← ← ← ← Agent ← ← ← ← ← ← ← ← ← ← ← ← ← ← ← ← ← ← ← ← ← ← ← ← ← ←
        |               |                 |               |
   Server.crt         Agent1.crt       Agent2.crt       Agent3.crt
```

**本地开发环境（自动生成证书）**：
- 证书目录：`etc/certs/`
- Server 和 Agent 共享同一个 CA：`ca.crt`
- 自动生成时检查证书是否过期，过期则重新生成

**生产环境**：
- 使用 Let's Encrypt 或手动签名的证书
- 配置文件中指定证书路径

### 生成 CA

```bash
# 生成根 CA
openssl genrsa -out ca.key 4096
openssl req -new -x509 -days 3650 \
  -key ca.key -out ca.crt \
  -subj "/CN=Croupier Root CA/O=Croupier/C=CN"
```

### 生成服务器证书

```bash
# 生成私钥
openssl genrsa -out server.key 4096

# 生成 CSR
openssl req -new -key server.key -out server.csr \
  -subj "/CN=server.example.com/O=Croupier/C=CN"

# 签发证书
openssl x509 -req -days 365 \
  -in server.csr -CA ca.crt -CAkey ca.key -CAcreateserial \
  -out server.crt \
  -extfile <(echo "subjectAltName=DNS:server.example.com,DNS:*.server.example.com")
```

### 生成 Agent 证书

```bash
# 生成私钥
openssl genrsa -out agent.key 4096

# 生成 CSR
openssl req -new -key agent.key -out agent.csr \
  -subj "/CN=agent-1/O=Croupier/C=CN"

# 签发证书
openssl x509 -req -days 365 \
  -in agent.csr -CA ca.crt -CAkey ca.key \
  -out agent.crt
```

### Server 配置

当前服务端控制面 TLS 配置位于 `control` 节点，而不是旧的 `GRPC` 节点。

**本地开发（自动生成证书）**：

```yaml
# server.yaml
control:
  addr: ":19090"
  cert: ""      # 空=自动生成证书
  key: ""       # 空=自动生成密钥
  ca: ""        # 空=不要求客户端证书（启用 mTLS 时设置 CA 路径）
```

**生产环境（手动配置证书）**：

```yaml
# server.yaml
control:
  addr: ":19090"
  cert: "/path/to/server.crt"     # 手动配置证书
  key: "/path/to/server.key"      # 手动配置密钥
  ca: "/path/to/ca.crt"           # 配置 CA 时启用 mTLS
```

### Agent 配置

**本地开发（跳过证书验证）**：

```yaml
# agent.yaml
server:
  addr: localhost:19090
  insecure: false                # 使用 TLS
  caFile: "etc/certs/ca.crt"     # CA 证书
  insecureSkipVerify: true       # 跳过验证（本地开发）
```

**生产环境（严格 mTLS）**：

```yaml
# agent.yaml
server:
  addr: server.example.com:19090
  insecure: false
  caFile: "/path/to/ca.crt"          # CA 证书
  tlsCertFile: "/path/to/agent.crt"  # 客户端证书
  tlsKeyFile: "/path/to/agent.key"   # 客户端密钥
  serverName: "server.example.com"   # Server SNI
  InsecureSkipVerify: false      # 严格验证

### Agent 配置

```yaml
agent:
  tls:
    ca_file: "data/ca.crt"
    cert_file: "data/agent.crt"
    key_file: "data/agent.key"
    server_name: "server.example.com"
    min_version: "TLS1.2"
```

## 认证配置

### JWT 配置

```yaml
server:
  auth:
    jwt_secret: "${JWT_SECRET}"  # 至少 32 字符
    jwt_expiry: "24h"
    jwt_refresh_expiry: "168h"  # 7 天
    issuer: "croupier"
```

### JWT Token 示例

```json
{
  "header": {
    "alg": "HS256",
    "typ": "JWT"
  },
  "payload": {
    "user_id": "user_123",
    "username": "admin",
    "roles": ["admin"],
    "exp": 1733140800,
    "iat": 1733054400,
    "iss": "croupier"
  }
}
```

### OIDC 配置

```yaml
server:
  auth:
    oidc:
      enabled: true
      issuer: "https://accounts.example.com"
      client_id: "${OIDC_CLIENT_ID}"
      client_secret: "${OIDC_CLIENT_SECRET}"
      redirect_url: "https://croupier.example.com/auth/callback"
      scopes:
        - "openid"
        - "profile"
        - "email"
```

### TOTP 双因素认证

```yaml
server:
  auth:
    totp:
      enabled: true
      issuer: "Croupier"
      period: 30
      digits: 6
```

## 权限配置

### RBAC 角色

```json
{
  "role_id": "admin",
  "name": "管理员",
  "permissions": ["*.*"]
}

{
  "role_id": "gm",
  "name": "游戏管理员",
  "permissions": [
    "player.*",
    "item.*",
    "guild.*"
  ]
}

{
  "role_id": "viewer",
  "name": "查看者",
  "permissions": [
    "player.view",
    "item.view",
    "guild.view"
  ]
}
```

### ABAC 策略

```json
{
  "id": "player.ban",
  "auth": {
    "permission": "player.ban",
    "allow_if": "has_role('admin') || (has_role('gm') && env == 'dev')"
  }
}
```

## 审批配置

### 双人规则

```json
{
  "id": "player.ban",
  "auth": {
    "two_person_rule": true,
    "approval": {
      "enabled": true,
      "threshold": 2,
      "approvers": ["admin", "senior_gm"],
      "timeout": "24h"
    }
  }
}
```

### 审批存储

```yaml
server:
  audit:
    approval_storage: "postgres"  # memory | postgres | sqlite
    approval_db:
      dsn: "postgres://user:pass@localhost:5432/croupier"
```

## 审计日志

### 审计配置

```yaml
server:
  audit:
    enabled: true
    # 敏感字段脱敏
    sensitive_fields:
      - "password"
      - "token"
      - "secret"
      - "api_key"
    # 审计保留天数
    retention_days: 365
    # 备份配置
    backup_enabled: true
    backup_location: "s3://audit-logs/"
```

### 审计链防篡改

```go
type AuditLog struct {
    AuditID  string
    Previous string  // 前一条记录的哈希
    Hash     string  // 本条记录的哈希
    Content  []byte
}

func (a *AuditLog) ComputeHash() string {
    h := sha256.New()
    h.Write([]byte(a.Previous))
    h.Write(a.Content)
    return hex.EncodeToString(h.Sum(nil))
}
```

## 网络安全

### 防火墙配置

```bash
# Server
ufw default deny incoming
ufw default allow outgoing
ufw allow 22/tcp      # SSH
ufw allow 443/tcp     # HTTPS
ufw allow 8443/tcp    # gRPC
ufw allow 8080/tcp    # HTTP
ufw enable
```

### DDoS 防护

```yaml
server:
  http:
    rate_limit:
      enabled: true
      requests_per_second: 100
      burst: 200
    ip_whitelist:
      - "10.0.0.0/8"
      - "192.168.0.0/16"
```

## 数据加密

### 数据库加密

```yaml
server:
  db:
    dsn: "postgres://user:pass@localhost:5432/croupier?sslmode=require"
    ssl:
      enabled: true
      cert_file: "data/client.crt"
      key_file: "data/client.key"
      ca_file: "data/ca.crt"
```

### 敏感字段加密

```go
type User struct {
    UserID   string
    Username string
    Password string `encrypt:"true"`
    APIKey   string `encrypt:"true"`
}
```

## 安全检查清单

### 部署前检查

- [ ] 所有组件使用 mTLS
- [ ] JWT Secret 足够复杂
- [ ] 启用了双因素认证
- [ ] 配置了双人规则
- [ ] 审计日志已启用
- [ ] 敏感字段已脱敏
- [ ] 数据库连接加密
- [ ] 防火墙已配置
- [ ] 限流已启用

### 定期检查

- [ ] 证书有效期检查
- [ ] 审计日志完整性检查
- [ ] 权限审查
- [ ] 安全漏洞扫描

## 故障排查

### TLS 握手失败

```bash
# 测试 TLS 连接
openssl s_client -connect server:8443 \
  -cert agent.crt -key agent.key -CAfile ca.crt

# 检查证书
openssl x509 -in server.crt -text -noout
```

### 认证失败

```bash
# 解码 JWT
echo "eyJhbGci..." | jq -R 'split(".") | .[1] | @base64d | fromjson'
```

## 相关文档

- [权限控制](../concepts/permissions.md)
- [配置管理](../configuration.md)
- [监控指南](./monitoring.md)
