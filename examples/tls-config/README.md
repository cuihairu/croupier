# TLS/mTLS 配置指南

本指南介绍如何为 Croupier 系统配置 TLS/mTLS 加密通信。

## 迁移说明

旧版 `CROUPIER_*TLS_ENABLED` 环境变量直读方式已移除；请通过各服务的 YAML 配置（并可配合 `conf.UseEnv()` 做环境变量覆盖）进行设置。

## 配置文件（YAML）配置（推荐）

### Agent 连接 Server（ControlService 上行）

`services/agent/etc/agent.yaml`：

```yaml
Server:
  Addr: "server:8443"
  Insecure: false
  TLSCertFile: "/path/to/client.crt"
  TLSKeyFile: "/path/to/client.key"
  CAFile: "/path/to/ca.crt"
  ServerName: "croupier-server.example.com"
  InsecureSkipVerify: false
```

### Agent gRPC core（下行：Server/Edge -> Agent）

`services/agent/etc/agent.yaml`：

```yaml
TLS:
  Enabled: true
  CertFile: "/path/to/agent-server.crt"
  KeyFile: "/path/to/agent-server.key"
  CAFile: "/path/to/ca.crt" # 非空则要求 client cert（mTLS）
```

### Dispatcher 连接 Agent（Server/Edge -> Agent）

`services/server/etc/server.yaml` / `services/edge/etc/edge.yaml`：

```yaml
Dispatch:
  AgentTLS:
    Enabled: true
    CertFile: "/path/to/client.crt"
    KeyFile: "/path/to/client.key"
    CAFile: "/path/to/ca.crt"
    ServerName: "agent.example.com"
    InsecureSkipVerify: false
```

### Agent 连接 Game Server（出站：Agent -> Game）

`services/agent/etc/agent.yaml`：

```yaml
OutboundTLS:
  Enabled: true
  CertFile: "/path/to/client.crt"
  KeyFile: "/path/to/client.key"
  CAFile: "/path/to/ca.crt"
  ServerName: "gameserver.example.com"
  InsecureSkipVerify: false
```

## 证书生成示例

### 1. 生成 CA 证书

```bash
# 生成 CA 私钥
openssl genrsa -out ca.key 4096

# 生成 CA 证书
openssl req -new -x509 -days 365 -key ca.key -out ca.crt \
  -subj "/C=CN/ST=Beijing/L=Beijing/O=Croupier/CN=Croupier CA"
```

### 2. 生成服务器证书

```bash
# 生成服务器私钥
openssl genrsa -out server.key 4096

# 生成服务器 CSR
openssl req -new -key server.key -out server.csr \
  -subj "/C=CN/ST=Beijing/L=Beijing/O=Croupier/CN=croupier-server.example.com"

# 签发服务器证书
openssl x509 -req -days 365 -in server.csr -CA ca.crt -CAkey ca.key \
  -CAcreateserial -out server.crt -extensions v3_req \
  -extfile <(cat <<EOF
[v3_req]
basicConstraints = CA:FALSE
keyUsage = nonRepudiation, digitalSignature, keyEncipherment
subjectAltName = @alt_names

[alt_names]
DNS.1 = croupier-server.example.com
DNS.2 = localhost
IP.1 = 127.0.0.1
EOF
)
```

### 3. 生成客户端证书

```bash
# 生成客户端私钥
openssl genrsa -out client.key 4096

# 生成客户端 CSR
openssl req -new -key client.key -out client.csr \
  -subj "/C=CN/ST=Beijing/L=Beijing/O=Croupier/CN=croupier-client"

# 签发客户端证书
openssl x509 -req -days 365 -in client.csr -CA ca.crt -CAkey ca.key \
  -CAcreateserial -out client.crt
```

## 部署场景

### 开发环境

使用自签名证书：

```bash
# 生成开发证书
./scripts/dev-certs.sh

# 然后把 ./certs/{client.crt,client.key,ca.crt} 写入对应服务的 YAML（见上方示例块）
```

### 测试环境

使用内部 CA：

```bash
# 使用测试 CA 签发的证书，并将路径写入对应服务的 YAML（见上方示例块）
```

### 生产环境

使用公共 CA 证书：

```bash
# 使用 Lets Encrypt 或其他公共 CA，并将路径写入对应服务的 YAML（见上方示例块）
```

## 验证配置

### 1. 检查证书有效性

```bash
# 验证服务器证书
openssl verify -CAfile ca.crt server.crt

# 验证客户端证书
openssl verify -CAfile ca.crt client.crt
```

### 2. 测试 TLS 连接

```bash
# 测试 gRPC over TLS
grpcurl -cert client.crt -key client.key -ca ca.crt \
  croupier-server.example.com:8443 list

# 测试普通 TLS
openssl s_client -connect croupier-server.example.com:8443 \
  -cert client.crt -key client.key -CAfile ca.crt
```

## 安全最佳实践

1. **证书管理**
   - 定期轮换证书（建议 90 天）
   - 使用强加密算法（RSA 4096+ 或 ECDSA）
   - 限制私钥访问权限（600）

2. **配置安全**
   - 禁用旧版本 TLS（仅支持 1.2+）
   - 启用证书吊销检查（CRL/OCSP）
   - 设置合理的超时时间

3. **监控告警**
   - 监控证书过期时间
   - 记录 TLS 握手失败
   - 监控加密套件使用

## 故障排查

### 常见错误

1. **证书验证失败**
   ```
   x509: certificate signed by unknown authority
   ```
   - 检查 CA 证书路径
   - 验证证书链完整性

2. **主机名不匹配**
   ```
   x509: certificate is not valid for any names
   ```
   - 检查 SNI 配置
   - 验证证书 SAN 字段

3. **私钥格式错误**
   ```
   tls: failed to parse private key
   ```
   - 确保私钥格式为 PEM
   - 检查私钥权限

### 调试命令

```bash
# 查看证书详情
openssl x509 -in server.crt -text -noout

# 验证证书链
openssl verify -CAfile ca.crt -untrusted intermediate.crt server.crt

# 测试 TLS 连接细节
openssl s_client -connect server:8443 -showcerts
```
