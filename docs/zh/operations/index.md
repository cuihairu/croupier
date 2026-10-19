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

本栏目面向值班与 SRE 角色，回答如何部署、升级、配置、观测和排障。设计动机看[架构栏目](/zh/architecture/)，端点语义看 [API 概览](/zh/api/)。

## 内容地图

| 分组     | 内容                                                                                                           |
| -------- | -------------------------------------------------------------------------------------------------------------- |
| 部署     | 部署形态总览、Docker、二进制与 systemd、Kubernetes、升级回滚、负载均衡、集群操作。                             |
| 配置     | [Server 配置](/operations/config-server)、[Agent 配置](/operations/agent-config)、TLS 证书、告警、通知与限流。 |
| 可观测性 | 指标、健康监控、数据库监控、外部服务探测。                                                                     |
| 可靠性   | 备份恢复、故障排除、升级回滚、Server HA 与多实例。                                                             |
| 安全     | [安全基线](/zh/operations/security)、[风控](/zh/operations/risk-control)、证书与传输安全。                     |
| Agent    | [Agent 对照](/zh/operations/agent-comparison)、默认 sidecar、devops 与 capture 的部署边界。                    |
| 发布     | SDK 包发布流程和发布前置检查。                                                                                 |

## 部署形态

| 形态             | 适合场景                       | 关键要求                                           |
| ---------------- | ------------------------------ | -------------------------------------------------- |
| Docker Compose   | 本地开发、联调和小规模试点。   | 统一管理 Redis、ClickHouse、数据库和 Server 依赖。 |
| 二进制 + systemd | 固定主机和受控内网。           | 配置目录、权限、日志轮转、开机启动和健康检查。     |
| Kubernetes       | 需要弹性、滚动发布和集中调度。 | Secret、探针、资源限额、存储和 ingress/L4 策略。   |

无论哪种形态，Agent 都主动连接 Server 控制入口；Server 不反向拨号 Agent。

## 端口与端点速查

| 端口 / 路径                      | 归属    | 用途                                               |
| -------------------------------- | ------- | -------------------------------------------------- |
| `:18780`                         | Server  | REST API 与 Dashboard 后端。                       |
| `:19090`                         | Server  | 控制与会话入口，Agent 主动连接，前置 L4 负载均衡。 |
| `:19091`                         | Agent   | 本地 TCP gateway，游戏进程或 SDK 接入。            |
| `:8404`                          | HAProxy | 连接分布统计页。                                   |
| `GET /healthz`                   | Server  | 存活探针。                                         |
| `GET /api/v1/monitoring/healthz` | Server  | 监控健康端点。                                     |
| `GET /api/v1/monitoring/metrics` | Server  | Prometheus 指标，需要认证。                        |
| `GET /api/v1`                    | Server  | 版本与功能域元信息。                               |

## 值班检查顺序

1. 检查 Server `healthz`、数据库组件状态和最近错误日志。
2. 检查 Agent 注册数量、心跳、重连与连接分布。
3. 检查核心 API 是否可用，页面执行是否出现权限、审批或绑定过期错误。
4. 检查数据库、Redis、ClickHouse、对象存储和外部通知通道。
5. 处理证书、限流、备份与磁盘空间告警。
6. 升级前确认备份、迁移路径、回滚版本和配置兼容性。

## 阅读入口

新部署从[部署总览](/zh/operations/deploy-docker)或现有平台对应的形态页开始。排障从[故障排除](/operations/troubleshooting)开始；改动安全边界前先读[安全基线](/zh/operations/security)和[风控](/zh/operations/risk-control)。
