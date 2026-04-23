# Croupier C++ SDK 配置指南

本文档给出 C++ SDK 应遵循的统一配置语义，重点是让 C++ SDK 与其他语言在 session、TLS、重连与 payload 行为上保持一致。

## 配置原则

- 配置面只保留当前明确需要的字段
- 优先表达 session 语义，而不是绑定某种历史 transport 实现
- 不再暴露 `rpc_addr`、`local_listen`、`grpc_target`、`nng_server` 这类过时字段

## 推荐配置模型

建议 C++ SDK 至少支持如下结构：

```json
{
  "address": "127.0.0.1:19091",
  "connectTimeoutMs": 5000,
  "requestTimeoutMs": 30000,
  "heartbeat": {
    "intervalMs": 30000
  },
  "tls": {
    "enabled": false,
    "certFile": "",
    "keyFile": "",
    "caFile": "",
    "serverName": "",
    "insecureSkipVerify": false
  },
  "reconnect": {
    "enabled": true,
    "initialDelayMs": 1000,
    "maxDelayMs": 30000,
    "multiplier": 2.0,
    "jitter": 0.2,
    "steadyStateDelayMs": 30000
  }
}
```

## 字段说明

| 字段 | 说明 |
| --- | --- |
| `address` | Agent 本地 gateway 地址 |
| `connectTimeoutMs` | 建连超时 |
| `requestTimeoutMs` | 单请求超时 |
| `heartbeat.intervalMs` | provider 心跳周期 |
| `tls.enabled` | 是否启用 TLS |
| `tls.certFile` | 客户端证书 |
| `tls.keyFile` | 客户端私钥 |
| `tls.caFile` | CA 文件 |
| `tls.serverName` | 证书校验名 / SNI |
| `tls.insecureSkipVerify` | 仅限开发环境跳过校验 |
| `reconnect.enabled` | 是否自动重连 |
| `reconnect.initialDelayMs` | 初始退避时间 |
| `reconnect.maxDelayMs` | 最大退避时间 |
| `reconnect.multiplier` | 指数退避倍率 |
| `reconnect.jitter` | 抖动比例 |
| `reconnect.steadyStateDelayMs` | 达到上限后的持续廉价重试周期 |

## TLS 规则

默认规则：

- `SDK <-> Agent` 默认可以不开启 TLS
- 跨主机、跨网段或有合规要求时启用 TLS
- 需要双向身份校验时启用 mTLS

注意：

- `tls` 是 `tcp session` 的安全配置
- `tls` 不是新的 transport kind

## 重连规则

推荐默认值：

- `enabled = true`
- `initialDelayMs = 1000`
- `maxDelayMs = 30000`
- `multiplier = 2.0`
- `jitter = 0.2`
- `steadyStateDelayMs = 30000`

设计意图：

- 刚断线时快速恢复
- 避免惊群
- 达到上限后保持低成本持续重试，而不是无限变慢

## payload 与 schema

统一规则：

- 平台协议消息继续使用 protobuf
- 业务 payload 默认固定为 JSON
- `input_schema` / `output_schema` 默认是 JSON Schema

这意味着接入方无需先定义自己的 `.proto` 文件。

## `profile` 术语说明

在配置语境里，`profile` 的专业含义通常是：

- `configuration profile`
- `deployment profile`
- `runtime profile`

它表示“命名配置预设”，而不是“个性化偏好”。

建议：

- 如果区分环境，优先用 `deployment profile`
- 如果区分连接安全与网络边界，优先用 `connection profile`

例如：

- `local-dev`
- `intra-cluster`
- `cross-zone-tls`
- `cross-zone-mtls`

## 推荐 profile 示例

### 本机开发

```json
{
  "profile": "local-dev",
  "address": "127.0.0.1:19091",
  "tls": {
    "enabled": false
  }
}
```

### 同集群内网

```json
{
  "profile": "intra-cluster",
  "address": "agent.service.local:19091",
  "tls": {
    "enabled": false
  }
}
```

### 跨机房 TLS

```json
{
  "profile": "cross-zone-tls",
  "address": "agent.example.com:19091",
  "tls": {
    "enabled": true,
    "caFile": "/etc/croupier/ca.crt",
    "serverName": "agent.example.com"
  }
}
```

### mTLS

```json
{
  "profile": "cross-zone-mtls",
  "address": "agent.example.com:19091",
  "tls": {
    "enabled": true,
    "certFile": "/etc/croupier/client.crt",
    "keyFile": "/etc/croupier/client.key",
    "caFile": "/etc/croupier/ca.crt",
    "serverName": "agent.example.com"
  }
}
```

## 明确禁止的字段

以下字段不应出现在新的配置模型里：

- `rpc_addr`
- `local_listen`
- `listen_port`
- `grpc_addr`
- `grpc_target`
- `nng_server`

原因是这些字段会把 SDK 重新拉回回拨式或本地监听式模型。

## 相关文档

- [C++ SDK 概览](./)
- [SDK 规范](../../sdk/specification.md)
- [SDK Wire Protocol](../../architecture/sdk-wire-protocol.md)
