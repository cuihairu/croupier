# 安全配置

本指南介绍 TLS、认证和其他安全相关的配置。

## TLS 配置

### 基础 TLS

```cpp
ClientConfig config;
config.insecure = false;

config.security.cert_file = "/etc/tls/client.crt";
config.security.key_file = "/etc/tls/client.key";
config.security.ca_file = "/etc/tls/ca.crt";
config.security.server_name = "croupier.internal";
```

### 配置文件

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

## 证书生成

### 生成 CA 证书

```bash
# 生成 CA 私钥
openssl genrsa -out ca.key 4096

# 生成 CA 证书
openssl req -new -x509 -days 365 -key ca.key -out ca.crt \
  -subj "/C=CN/ST=Beijing/L=Beijing/O=Croupier/CN=CroupierCA"
```

### 生成服务器证书

```bash
# 生成私钥
openssl genrsa -out server.key 4096

# 生成 CSR
openssl req -new -key server.key -out server.csr \
  -subj "/C=CN/ST=Beijing/L=Beijing/O=Croupier/CN=croupier-agent.internal"

# 用 CA 签名
openssl x509 -req -days 365 -in server.csr -CA ca.crt -CAkey ca.key \
  -CAcreateserial -out server.crt
```

### 生成客户端证书

```bash
# 生成私钥
openssl genrsa -out client.key 4096

# 生成 CSR
openssl req -new -key client.key -out client.csr \
  -subj "/C=CN/ST=Beijing/L=Beijing/O=Croupier/CN=game-server"

# 用 CA 签名
openssl x509 -req -days 365 -in client.csr -CA ca.crt -CAkey ca.key \
  -CAcreateserial -out client.crt
```

## 认证配置

### JWT Token

```cpp
config.auth.token = "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...";
```

### 使用环境变量

```bash
export CROUPIER_AUTH_TOKEN="Bearer ${JWT_TOKEN}"
```

### 配置文件

```json
{
  "auth": {
    "token": "Bearer ${JWT_TOKEN}",
    "headers": {
      "X-Game-ID": "${GAME_ID}",
      "X-Service-Region": "us-west-2",
      "X-Cluster": "production-01"
    }
  }
}
```

## Kubernetes Secrets

### 创建 TLS Secret

```bash
kubectl create secret tls croupier-tls \
  --cert=/path/to/client.crt \
  --key=/path/to/client.key \
  -n game
```

### 创建 Token Secret

```bash
kubectl create secret generic croupier-auth \
  --from-literal=jwt-token='your-jwt-token' \
  -n game
```

### Pod 中使用

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: game-server
spec:
  containers:
  - name: game-server
    image: game-server:latest
    env:
    - name: CROUPIER_AUTH_TOKEN
      valueFrom:
        secretKeyRef:
          name: croupier-auth
          key: jwt-token
    volumeMounts:
    - name: tls-certs
      mountPath: /etc/tls
      readOnly: true
  volumes:
  - name: tls-certs
    secret:
      secretName: croupier-tls
```

## 证书验证

### 验证证书文件存在

```cpp
bool ValidateCertFiles(const ClientConfig& config) {
    std::ifstream cert(config.security.cert_file);
    std::ifstream key(config.security.key_file);
    std::ifstream ca(config.security.ca_file);

    if (!cert.good()) {
        std::cerr << "证书文件不存在: " << config.security.cert_file << std::endl;
        return false;
    }
    if (!key.good()) {
        std::cerr << "私钥文件不存在: " << config.security.key_file << std::endl;
        return false;
    }
    if (!ca.good()) {
        std::cerr << "CA 文件不存在: " << config.security.ca_file << std::endl;
        return false;
    }

    return true;
}
```

### 验证证书有效期

```bash
# 检查证书有效期
openssl x509 -in /etc/tls/client.crt -noout -dates

# 输出示例:
# notBefore=Jan  1 00:00:00 2024 GMT
# notAfter=Dec 31 23:59:59 2024 GMT
```

## 最佳实践

### 1. 永不在生产环境使用 `insecure: true`

```cpp
// 开发环境
#ifdef DEBUG
config.insecure = true;
#else
config.insecure = false;
#endif
```

### 2. 使用密钥管理系统

```cpp
// 从 Vault 获取证书
std::string cert = vault::GetSecret("croupier/tls/client.crt");
std::string key = vault::GetSecret("croupier/tls/client.key");

// 写入临时文件
WriteTempFile("/tmp/client.crt", cert);
WriteTempFile("/tmp/client.key", key);

config.security.cert_file = "/tmp/client.crt";
config.security.key_file = "/tmp/client.key";
```

### 3. 定期轮换证书

```bash
# 证书轮换脚本
#!/bin/bash
# rotate-certs.sh

# 1. 生成新证书
# 2. 更新 Kubernetes Secret
# 3. 重启 Pod（逐步滚动更新）

kubectl create secret tls croupier-tls-new \
  --cert=/path/to/new-client.crt \
  --key=/path/to/new-client.key

kubectl rolling-update game-server \
  --image=game-server:new-version
```

### 4. 证书监控

```cpp
// 检查证书是否即将过期
bool CheckCertExpiry(const std::string& cert_file, int days_before = 30) {
    // 读取证书
    FILE* fp = fopen(cert_file.c_str(), "r");
    X509* cert = PEM_read_X509(fp, nullptr, nullptr, nullptr);
    fclose(fp);

    // 检查有效期
    ASN1_TIME* notAfter = X509_get0_notAfter(cert);
    // ... 解析并比较日期

    X509_free(cert);
    return is_expiring_soon;
}
```

## 故障排查

### 证书验证失败

```
错误: SSL verification failed
```

**解决方案：**
1. 检查 CA 文件路径
2. 确认 `server_name` 与证书 CN 匹配
3. 验证证书未过期

### 私钥不匹配

```
错误: Private key does not match certificate
```

**解决方案：**
```bash
# 验证私钥和证书匹配
openssl x509 -noout -modulus -in client.crt | openssl md5
openssl rsa -noout -modulus -in client.key | openssl md5

# 两个输出应该相同
```

### 连接超时

```
错误: Connection timeout
```

**解决方案：**
1. 检查网络连接
2. 确认 Agent 地址正确
3. 检查防火墙规则
4. 验证 `server_name` 可解析
