---
title: TLS 与证书
icon: lock
order: 12
category:
  - 运维手册
tag:
  - 安全
  - 配置
---

# TLS 与证书

平台三条内部链路各自独立配置 TLS，互不影响：

| 链路              | 方向                      | Server 侧配置                | Agent 侧配置                               |
| ----------------- | ------------------------- | ---------------------------- | ------------------------------------------ |
| control transport | Agent → Server `:19090`   | `control.cert / key / ca`    | `outboundTLS.*` + `server.insecure: false` |
| dispatch 回调     | Server → Agent `httpAddr` | `agentDispatch.toAgentTLS.*` | `tls.*`（本地监听）                        |
| 本地注册          | 游戏服 → Agent `:19091`   | —                            | `tls.*`（是否加密由游戏侧要求决定）        |

Dashboard 的 HTTP `:18780` 建议挂在前置 nginx/Ingress 上统一终结 TLS（平台自身不做 HTTPS）。

## 开发证书

```bash
./scripts/dev-certs.sh   # 生成自签证书到本地（仅开发/联调）
```

## 生产建议

1. **内部 CA 签发**：为 Server transport 与 Agent 各签服务证书，CA 证书分发到对端 `caFile`
2. **双向校验**：Server `control.ca` 校验 Agent 客户端证书；Agent `outboundTLS.caFile` 校验 Server——双端 mTLS
3. **证书轮换**：证书文件路径热加载不生效，轮换后需重启对应进程；建议滚动的窗口期内新旧 CA 并存（`caFile` 指向 CA bundle）
4. **`insecureSkipVerify` 禁止生产开启**——它绕过的是身份校验，不是加密协商失败兜底

## Server 侧配置示例

```yaml
control:
  addr: ":19090"
  cert: /etc/croupier/tls/server.pem
  key: /etc/croupier/tls/server-key.pem
  ca: /etc/croupier/tls/ca.pem # 校验 Agent 客户端证书；空 = 单向 TLS

agentDispatch:
  toAgentTLS:
    enabled: true
    certFile: /etc/croupier/tls/server.pem
    keyFile: /etc/croupier/tls/server-key.pem
    caFile: /etc/croupier/tls/ca.pem
```

## Agent 侧配置示例

```yaml
server:
  addr: "haproxy.internal:19090"
  insecure: false
  serverName: "croupier-transport.internal" # 证书 SAN 不等于 LB 地址时指定

outboundTLS:
  enabled: true
  certFile: /etc/croupier/tls/agent.pem
  keyFile: /etc/croupier/tls/agent-key.pem
  caFile: /etc/croupier/tls/ca.pem

tls: # :19091 本地监听（游戏服 → Agent）
  enabled: false # 与游戏服同网段时通常内网明文；有合规要求再开
```

## 证书有效期管理

- 平台不内置过期告警前，用外部巡检（Prometheus blackbox / cert-exporter）监控证书 `notAfter`
- 「运维中心 → 证书监控」页面展示平台纳管证书的状态与到期时间（ops 域能力，见 [API 参考](/api/) certificates 一节）
- 高危操作（审批双签）走平台审批流，证书更换属于低风险操作可直接执行

## 排障

| 症状                                            | 常见原因                                                                  |
| ----------------------------------------------- | ------------------------------------------------------------------------- |
| Agent 反复重连 `connection closed`              | 证书 SAN 不含 LB 地址 → 配 `server.serverName` 或重签证书                 |
| `x509: certificate signed by unknown authority` | 对端 `caFile` 未更新为新 CA                                               |
| 握手成功但 job 下发失败                         | TLS 无关——检查 `agent.httpAddr` 可达性（见 [Agent 配置](./config-agent)） |
| 明文连到 TLS 端口                               | `server.insecure` 与端口实际形态不一致                                    |
