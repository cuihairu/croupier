# Client Config

本文是 C++ SDK 配置语义的 canonical 入口。配置只表达当前统一 session 模型，不再绑定历史本地监听、回拨或特定 transport 实现。

## 推荐配置模型

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
| `tls.serverName` | 证书校验名或 SNI |
| `tls.insecureSkipVerify` | 仅限开发环境跳过校验 |
| `reconnect.enabled` | 是否自动重连 |
| `reconnect.initialDelayMs` | 初始退避时间 |
| `reconnect.maxDelayMs` | 最大退避时间 |
| `reconnect.multiplier` | 指数退避倍率 |
| `reconnect.jitter` | 抖动比例 |
| `reconnect.steadyStateDelayMs` | 达到上限后的持续廉价重试周期 |

## 配置原则

- 保持字段最小化，只保留当前明确需要的能力。
- `SDK <-> Agent` 默认连接 Agent 本地 gateway，默认可以不开启 TLS。
- 跨主机、跨网段或有合规要求时启用 TLS；需要双向身份校验时启用 mTLS。
- `tls` 是 TCP session 的安全配置，不是新的 transport kind。
- 重连应默认开启，避免 Agent 短暂重启导致业务进程长期不可用。

## 禁止项

以下字段属于历史回拨或本地监听模型，不应再出现在新配置中：

- `rpc_addr`
- `local_listen`
- `grpc_target`
- `grpc_addr`
- `nng_server`
- `callback_addr`
