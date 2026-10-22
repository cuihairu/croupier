---
title: 故障排查
icon: wrench
order: 31
category:
  - 运维指南
tag:
  - 故障排查
  - 问题定位
  - FAQ
---

# 故障排查指南

本文聚焦当前会话架构下最常见的排查路径。

## 关键端口

- `18780`：Server REST API
- `19090`：Server 会话/控制入口
- `19091`：Agent 本地网关

## 快速诊断

```mermaid
graph TD
    A[Problem appears] --> B{Which link is abnormal}
    B -->|Server itself| C[Check 18780 health status]
    B -->|Agent <-> Server| D[Check 19090 connectivity and TLS]
    B -->|SDK <-> Agent| E[Check 19091 connectivity and first-frame protocol]
    C --> F[Check configuration and logs]
    D --> F
    E --> F
```

## Server 启动失败

### 检查配置

```bash
python3 -c "import yaml; yaml.safe_load(open('configs/server.yaml'))"
```

### 检查端口

```bash
lsof -i :18780
lsof -i :19090
```

### 检查健康状态

```bash
curl http://localhost:18780/healthz
```

## Agent 连不上 Server

### 检查 Server 是否在线

```bash
curl http://server:18780/healthz
nc -zv server 19090
```

### 检查 TLS

```bash
openssl s_client -connect server:19090 \
  -cert agent.crt \
  -key agent.key \
  -CAfile ca.crt
```

### 常见原因

| 症状                 | 原因                                 |
| -------------------- | ------------------------------------ |
| `connection refused` | Server 会话/控制端口未监听           |
| TLS 握手失败         | 证书、CA、SNI 或 mTLS 配置错误       |
| 反复重连             | 目标地址错误、首帧校验失败、心跳超时 |

## SDK 连不上 Agent

### 检查本地网关

```bash
nc -zv 127.0.0.1 19091
```

### 关键检查

- SDK 连接的是 `19091`，不是 `19090`
- SDK 首帧必须是 `ProviderConnectRequest`
- SDK 不应要求本地监听端口
- SDK 不应配置 `rpc_addr`

### 常见原因

| 症状           | 原因                                               |
| -------------- | -------------------------------------------------- |
| 连上立即断开   | 首帧不是 `ProviderConnectRequest` 或 body 解码失败 |
| 无法触达 Agent | 连错端口、Agent 未启动或 TLS 不匹配                |
| 断线后无法恢复 | 重连策略未启用或退避配置错误                       |

## 调用失败

### 检查函数是否已注册

```bash
curl http://localhost:18780/api/v1/functions
curl http://localhost:18780/api/v1/functions/descriptors
```

### 检查调用超时

按顺序排查：

- SDK handler 是否阻塞
- `requestTimeoutMs` 是否过短
- 会话是否处于 draining 状态
- Agent / Server 是否在施加背压

## 常见 TLS 问题

### 证书不受信任

```text
x509: certificate signed by unknown authority
```

检查：

- `caFile` 是否指向正确的 CA
- 客户端与服务端是否使用同一信任链

### 主机名不匹配

```text
x509: certificate is not valid for any names
```

检查：

- `serverName` 是否与证书 SAN / CN 匹配
- 是否误把 IP 地址当作证书名

## 旧概念检查

如果在日志、配置或文档里看到以下概念，先怀疑撞上了旧路径：

- `gRPC`
- `legacy REQ/REP`
- `LocalControlService`
- `RegisterLocal`
- `rpc_addr`
- SDK 本地监听

这些概念都不应再作为新问题排查的基准。
