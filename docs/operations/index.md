---
title: 运维手册
icon: tool
order: 1
category:
  - 运维手册
tag:
  - 运维
  - 部署
  - SRE
---

# 运维手册

面向平台值班与 SRE 角色的操作文档：回答「怎么部署、怎么升级、怎么配置、怎么排障」——设计动机见[架构栏目](/architecture/)，端点语义见 [API 参考](/api/)。

## 内容地图

| 分组                                | 覆盖                                                                                                                                                                                                                                                 |
| ----------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| [部署](/operations/deploy-overview) | 形态选型总览、Docker Compose（含 HA 编排）、二进制 + systemd、Kubernetes、版本升级与回滚、负载均衡                                                                                                                                                   |
| 配置                                | [Server 配置全解](/operations/config-server)、[Agent 配置全解](/operations/config-agent)、[TLS 与证书](/operations/tls-certificates)；分层模型与热更新见架构栏（[配置分层](/architecture/config-layering)、[功能开关](/architecture/feature-flags)） |
| [可观测性](/operations/monitoring)  | 指标、健康检查、Prometheus 接入                                                                                                                                                                                                                      |
| 可靠性                              | [备份与恢复](/operations/backup-restore)、[故障排除](/operations/troubleshooting)、[数据库迁移策略](/architecture/database-migration-strategy)                                                                                                       |
| [安全](/operations/security)        | TLS、密钥、RBAC 审计基线                                                                                                                                                                                                                             |

## 端口与端点速查

| 端口/路径      | 归属    | 用途                                                    |
| -------------- | ------- | ------------------------------------------------------- |
| `:18780`       | Server  | HTTP API + Dashboard 后端                               |
| `:19090`       | Server  | 自研 transport，Agent/SDK 接入入口（经 L4 LB）          |
| `:19091`       | Agent   | 本地 TCP，游戏服函数注册                                |
| `:8404`        | HAProxy | stats 页（Agent 连接分布排查）                          |
| `GET /healthz` | Server  | 存活探针（根路径与 `/api/v1/monitoring/healthz` 均可）  |
| `GET /metrics` | Server  | Prometheus 指标（需认证，`/api/v1/monitoring/metrics`） |
| `GET /api/v1`  | Server  | 版本与功能域元信息（前端菜单裁剪依据）                  |
