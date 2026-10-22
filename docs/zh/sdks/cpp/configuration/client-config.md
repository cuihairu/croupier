# Client Config

本页是 C++ SDK 配置语义的权威入口。配置只表达当前统一的 session 模型，不再绑定旧版 local-listener、callback 或任何具体传输实现。

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

## 字段参考

| 字段                           | 说明                           |
| ------------------------------ | ------------------------------ |
| `address`                      | Agent 本地网关地址             |
| `connectTimeoutMs`             | 连接超时                       |
| `requestTimeoutMs`             | 单次请求超时                   |
| `heartbeat.intervalMs`         | Provider 心跳间隔              |
| `tls.enabled`                  | 是否启用 TLS                   |
| `tls.certFile`                 | 客户端证书                     |
| `tls.keyFile`                  | 客户端私钥                     |
| `tls.caFile`                   | CA 文件                        |
| `tls.serverName`               | 证书校验名或 SNI               |
| `tls.insecureSkipVerify`       | 跳过校验，仅限开发环境         |
| `reconnect.enabled`            | 是否启用自动重连               |
| `reconnect.initialDelayMs`     | 初始退避延迟                   |
| `reconnect.maxDelayMs`         | 最大退避延迟                   |
| `reconnect.multiplier`         | 指数退避倍数                   |
| `reconnect.jitter`             | 抖动比例                       |
| `reconnect.steadyStateDelayMs` | 达到上限后的常态低成本重试间隔 |

## 配置原则

- 字段集保持最小，只保留当前明确需要的能力。
- `SDK <-> Agent` 默认连 Agent 本地网关，TLS 可默认关闭。
- 跨主机、跨网段或合规敏感场景启用 TLS；需要双向认证时启用 mTLS。
- `tls` 是 TCP session 的安全配置，不是新的传输类型。
- 重连应默认开启，避免 Agent 短暂重启导致业务进程长时间不可用。

## 禁用字段

以下字段属于旧版 callback 或 local-listener 模型，不得出现在新配置中：

- `rpc_addr`
- `local_listen`
- `grpc_target`
- `grpc_addr`
- `nng_server`
- `callback_addr`
