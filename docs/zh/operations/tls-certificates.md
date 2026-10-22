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

Dashboard 的 HTTP `:18780` 应放在前置 nginx/Ingress 之后统一终结 TLS（平台自身不做 HTTPS）。

## 开发证书

```bash
./scripts/dev-certs.sh   # Generates self-signed certificates locally (development/integration testing only)
```

## 生产建议

1. **内部 CA 签发**：Server transport 与各 Agent 分别签发服务端证书；CA 证书分发到对端 `caFile`
2. **双向校验**：Server 的 `control.ca` 校验 Agent 客户端证书；Agent 的 `outboundTLS.caFile` 校验 Server——两端 mTLS
3. **证书轮换**：证书文件路径的热加载不生效，轮换后必须重启对应进程；滚动窗口期建议新旧 CA 并存（`caFile` 指向 CA bundle）
4. **生产绝不开启 `insecureSkipVerify`**——它绕过身份校验，不是加密协商失败的退路

## Server 侧配置示例

```yaml
control:
  addr: ":19090"
  cert: /etc/croupier/tls/server.pem
  key: /etc/croupier/tls/server-key.pem
  ca: /etc/croupier/tls/ca.pem # Verifies Agent client certificates; empty = one-way TLS

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
  serverName: "croupier-transport.internal" # Specify when the certificate SAN differs from the LB address

outboundTLS:
  enabled: true
  certFile: /etc/croupier/tls/agent.pem
  keyFile: /etc/croupier/tls/agent-key.pem
  caFile: /etc/croupier/tls/ca.pem

tls: # :19091 local listener (game server → Agent)
  enabled: false # Usually plaintext within the game server's subnet; enable only if compliance requires it
```

## 证书有效期管理

- 平台内置到期告警之前，用外部巡检（Prometheus blackbox / cert-exporter）盯证书 `notAfter`
- 「运营中心 → 证书监控」页展示平台托管证书的状态与到期（运营功能域能力，见 [API 参考](/zh/api/) 的 certificates 小节）
- 高危操作（双人签名审批）走平台审批流；证书更换属低风险操作可直接执行

## 故障排查

| 症状                                            | 常见原因                                                  |
| ----------------------------------------------- | --------------------------------------------------------- |
| Agent 反复重连报 `connection closed`            | 证书 SAN 不含 LB 地址 → 设 `server.serverName` 或换发证书 |
| `x509: certificate signed by unknown authority` | 对端 `caFile` 未更新到新 CA                               |
| 握手成功但任务派发失败                          | 与 TLS 无关——查 `agent.httpAddr` 可达性（见 Agent 配置）  |
| 明文连到 TLS 端口                               | `server.insecure` 与端口实际形态不一致                    |
