# TLS / mTLS 配置指南

本指南说明当前 session 设计下的 TLS 策略与配置方式。

## 当前默认策略

### `Agent <-> Server`

- 默认应启用 TLS
- 生产环境推荐 mTLS
- 首帧仍是 session 子协议消息，TLS 只是其外层安全封装

### `SDK <-> Agent`

- 默认可不启用 TLS
- 内网同机、同机房场景下明文通常足够
- 跨主机、跨网段、零信任网络或合规场景时启用 TLS / mTLS

## 证书生成示例

### 1. 生成 CA

```bash
openssl genrsa -out ca.key 4096
openssl req -new -x509 -days 3650 \
  -key ca.key -out ca.crt \
  -subj "/C=CN/ST=Beijing/L=Beijing/O=Croupier/CN=Croupier CA"
```

### 2. 生成 Server 证书

```bash
openssl genrsa -out server.key 4096
openssl req -new -key server.key -out server.csr \
  -subj "/C=CN/ST=Beijing/L=Beijing/O=Croupier/CN=croupier-server.example.com"
openssl x509 -req -days 365 -in server.csr -CA ca.crt -CAkey ca.key \
  -CAcreateserial -out server.crt
```

### 3. 生成 Agent / Client 证书

```bash
openssl genrsa -out agent.key 4096
openssl req -new -key agent.key -out agent.csr \
  -subj "/C=CN/ST=Beijing/L=Beijing/O=Croupier/CN=croupier-agent"
openssl x509 -req -days 365 -in agent.csr -CA ca.crt -CAkey ca.key \
  -CAcreateserial -out agent.crt
```

## `Agent <-> Server` 配置

### Server 侧

`configs/server.yaml`：

```yaml
control:
  addr: ":19090"
  cert: "/path/to/server.crt"
  key: "/path/to/server.key"
  ca: "/path/to/ca.crt"
```

说明：

- `ca` 为空时表示只做单向 TLS
- `ca` 非空时表示要求客户端证书，可实现 mTLS

### Agent 上行客户端

`configs/agent.yaml`：

```yaml
server:
  addr: "croupier-server.example.com:19090"
  insecure: false
  serverName: "croupier-server.example.com"
  insecureSkipVerify: false
  tlsCertFile: "/path/to/agent.crt"
  tlsKeyFile: "/path/to/agent.key"
  caFile: "/path/to/ca.crt"
```

## `SDK <-> Agent` 配置

如果需要给 Agent 本地 gateway 启用 TLS，可在 Agent 配置中打开本地 listener TLS：

`configs/agent.yaml`：

```yaml
tls:
  enabled: true
  certFile: "/path/to/agent.crt"
  keyFile: "/path/to/agent.key"
  caFile: "/path/to/ca.crt"
  insecureSkipVerify: false
```

SDK 侧则需要提供：

- `tls.enabled = true`
- `tls.caFile`
- 如需 mTLS，再提供 `tls.certFile` / `tls.keyFile`

## 验证方式

### 测试 `Agent <-> Server` TLS

```bash
openssl s_client -connect croupier-server.example.com:19090 \
  -cert agent.crt \
  -key agent.key \
  -CAfile ca.crt
```

### 测试 `SDK <-> Agent` TLS

```bash
openssl s_client -connect 127.0.0.1:19091 \
  -CAfile ca.crt
```

## 关键说明

- TLS 负责加密与认证，不改变 `subprotocol`
- `sdk-agent subprotocol` 与 `agent-server subprotocol` 仍按首帧消息识别
- v1 不需要为 TLS 再单独设计新的协议头

## 常见建议

1. 本机开发优先明文，先把 session 逻辑跑通
2. 生产环境的 `Agent <-> Server` 默认启用 mTLS
3. `SDK <-> Agent` 是否启用 TLS 取决于部署边界，而不是强制统一
