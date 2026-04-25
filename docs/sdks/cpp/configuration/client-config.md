# Client Config

推荐配置字段：

- `address`
- `connectTimeoutMs`
- `requestTimeoutMs`
- `heartbeat.intervalMs`
- `tls.*`
- `reconnect.*`

## 建议

- 保持字段最小化
- 不再暴露 `rpc_addr`、`local_listen`、`grpc_target` 等过时字段
