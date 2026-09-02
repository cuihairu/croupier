---
title: Agent 配置全解
icon: settings
order: 11
category:
  - 运维手册
tag:
  - 配置
  - agent
---

# Agent 配置全解

对应 `configs/agent.yaml`（模板见仓库；环境变量前缀 `CROUPIER_AGENT_*`，CLI 参数 `--xxx`，优先级 CLI > 环境变量 > YAML）。

## 完整键位

### server（上游连接）

| 键                          | 默认    | 说明                                                                       |
| --------------------------- | ------- | -------------------------------------------------------------------------- |
| `server.addr`               | 必填    | Server transport 入口。生产指向 L4 LB（HAProxy/NLB/Service），不指具体实例 |
| `server.insecure`           | `true`  | 明文连接（dev）；生产置 `false` 走 TLS                                     |
| `server.serverName`         | `""`    | TLS SNI/校验名（与证书 CN 不一致时使用）                                   |
| `server.insecureSkipVerify` | `false` | 跳过证书校验（仅排障，禁止生产开启）                                       |

### agent（身份与可达性）

| 键                           | 默认            | 说明                                                                                                |
| ---------------------------- | --------------- | --------------------------------------------------------------------------------------------------- |
| `agent.id`                   | `""`            | 留空自动生成；多 Agent 部署建议显式指定（拓扑页可读性）                                             |
| `agent.gameId` / `agent.env` | `""`            | 作用域绑定；留空由注册的游戏服声明                                                                  |
| `agent.localAddr`            | `0.0.0.0:19091` | 本地 TCP 监听（游戏服函数注册入口）                                                                 |
| `agent.httpAddr`             | 必填            | **Server→Agent 回调地址**，必须是 Server 视角可达的地址（容器网络用服务名，裸机用 IP）              |
| `agent.labels`               | `{}`            | 自定义标签（机房/机型等，节点页过滤用）                                                             |
| `agent.invokeTimeoutMs`      | `15000`         | Agent→游戏服同步调用默认预算（毫秒）；请求 `metadata["timeoutMs"]` 声明更小值时取更小者，上限 60000 |

> `httpAddr` 是最常见的部署错误：写 `0.0.0.0` 或 `localhost` 会导致 Server 回调失败——job 下发/文件传输走不通。

### upstream（连接韧性）

| 键                           | 默认    | 说明                                 |
| ---------------------------- | ------- | ------------------------------------ |
| `upstream.heartbeatInterval` | `30`    | 心跳秒数；Server 侧按 3 倍间隔判失联 |
| `upstream.retryInterval`     | `5`     | 断连重连退避基础间隔                 |
| `upstream.maxRetries`        | `3`     | 单轮重试次数（之后按间隔继续轮询）   |
| `upstream.timeout`           | `10000` | 请求超时（毫秒）                     |

### tls（本地 `:19091` 监听的入站 TLS）

`tls.enabled / certFile / keyFile / caFile / insecureSkipVerify`——游戏服与 Agent 同网段，通常内网明文即可。

### outboundTLS（Agent→Server 链路 TLS）

与 `server.insecure: false` 配套：`enabled / certFile / keyFile / caFile / serverName / insecureSkipVerify`。生产推荐内部 CA 签发，见 [TLS 与证书](./tls-certificates)。

### ops（指标上报）

| 键                     | 默认   | 说明                               |
| ---------------------- | ------ | ---------------------------------- |
| `ops.enabled`          | `true` | 系统指标采集上报                   |
| `ops.metrics_interval` | `30s`  | 采集周期                           |
| `ops.metrics_enabled`  | `true` | 指标开关（关掉采集但保留上报通道） |

## 双 Agent / 多 Agent 部署

同一宿主多 Agent（多游戏或多环境）：复制配置差异点只有 `agent.id`、`agent.gameId/env`、`agent.localAddr`、`agent.httpAddr` 四项（参考 `configs/agent2.yaml`）。上游 `server.addr` 共用同一 LB 入口。

## 环境变量示例

```bash
CROUPIER_AGENT_SERVER_ADDR=haproxy.internal:19090 \
CROUPIER_AGENT_SERVER_INSECURE=false \
CROUPIER_AGENT_OUTBOUNDTLS_ENABLED=true \
CROUPIER_AGENT_OUTBOUNDTLS_CAFILE=/etc/croupier/ca.pem \
CROUPIER_AGENT_AGENT_HTTPADDR=10.0.1.5:19091 \
croupier-agent --config /etc/croupier/agent.yaml
```

## 验证

1. Agent 日志出现 session 建立与注册成功
2. Dashboard「运维中心 → 节点维护」看到 agent 在线、标签正确
3. 从 Dashboard 触发一次该 Agent 名下函数调用，验证回调链路（`httpAddr` 正确性）
