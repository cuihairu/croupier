---
title: 游戏数据分析系统
icon: chart-line
order: 1
category:
  - 分析系统
tag:
  - 分析
---

# 游戏数据分析系统文档

> **2026-10-29 产品边界与现状纠偏**：内置共用核心运营指标，专业分析接外部系统；已有页面/模型/管道不代表指标已经形成可核对闭环。当前留存生产计算、数据源统一及聚合语义仍有缺口。[范围、代码证据与交付计划](./core-and-external)。以下指标分类是能力目录，不是全部已实现声明。

本节只保留与当前仓库内分析链路实现对应的说明：`cmd/ingest`、`cmd/analytics-worker`、`configs/analytics/*`、Redis、ClickHouse。

## 文档导航

### 核心概念

- [游戏指标全景图](./game-metrics-overview.md)

### 技术实施

### 快速落地

## 数据链路拓扑

三条链路各有分工。浏览器通过 Server 的 Analytics API 查询报表，Server 按端点读取 ClickHouse 或关系模型；浏览器不直连数据库。下图仅展示现有默认 Redis 路径，专业外部分析属于目标方案。

| 链路                   | 数据来源                                           | 存储                                                                                     | 查询出口                                          | 适用场景                                   |
| ---------------------- | -------------------------------------------------- | ---------------------------------------------------------------------------------------- | ------------------------------------------------- | ------------------------------------------ |
| **A：ClickHouse 管道** | 客户端/游戏服 → `cmd/ingest`（HMAC 签名上报）      | Redis Streams → `cmd/analytics-worker` → ClickHouse（events/payments 明细 + 3 张聚合表） | **数据仓库页**（`/api/v1/analytics/warehouse/*`） | 玩家行为、DAU/在线、收入等大规模明细与聚合 |
| **B：Server GORM**     | Server 自身业务模型（玩家、支付、关卡等游戏库表）  | 各 game 库（`internal/model`）                                                           | 概览/留存/行为/支付/关卡页                        | 中低数据量、需与业务实体 join 的统计       |
| **C：OTel bridge**     | Server 进程内遥测事件（OTel 语义命名，点号事件名） | bridge XAdd → `analytics:events` → 同管道 A                                              | 同 A（事件归流后与 A 合并）                       | 服务端自动埋点，无需游戏侧改造             |

<div class="analytics-diagram">

![分析链路实际拓扑：独立 ingest、worker、Server 进程，进程内 bridge 与两类存储](/architecture/analytics-zh.svg)

</div>

[打开完整 SVG 架构图](/architecture/analytics-zh.svg) · [英文 SVG](/architecture/analytics-en.svg)

图中箭头表示写入或查询请求/响应，不是执行时间线。虚线为可选 bridge，交叉线无圆点不相连。Redis 和 ClickHouse 是数据服务，bridge 与 Analytics API 是 Server 进程内模块。仓库查询仍须经过 Server；GORM 与仓库不是同一个数据源。

## 当前链路与代码依据

| 关系                                | 实现依据与边界                                                                                                                                                                                                 |
| ----------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 生产方 → ingest → Redis             | `cmd/ingest/cmd/root.go` 的 `/api/ingest/events`、`/api/ingest/payments` 路由与验签；`internal/analytics/mq/redis_pub.go` 发布 JSON 到 Streams。HTTP 202 不等于 ClickHouse 已落盘。HMAC 密钥应留在受信生产端。 |
| Server bridge → Redis               | `internal/telemetry/analytics_bridge.go` 的进程内缓冲与批次发布，直接写 `analytics:events`，不经过 ingest HTTP。控制面遥测不能直接作为游戏 DAU。                                                               |
| worker ↔ Redis；worker → ClickHouse | `internal/analytics/worker/worker.go` 的消费组读取、pending 回收、重试/死信及明细/聚合批写。ACK 与批次持久化不构成原子事务，ARCH-011/012 仍开放。                                                              |
| 浏览器 → Server → 数据库            | `web/src/services/api/analytics.ts`、`internal/handler/routes.go`、`internal/api/analytics/warehouse.go` 及同目录关系报表实现。仓库端点读 ClickHouse；其他报表按各自实现读关系模型，不自动复用仓库事件。       |

上图不画出未接通的能力：MQ 工厂另有 Kafka/noop 发布选项（`internal/analytics/mq/factory.go`），当前 worker 消费 Redis，不能把 Kafka 画成同一条已接通链路。专业分析集成和核心指标闭环参见 [范围与交付计划](./core-and-external)。

## 启动顺序

```bash
docker compose up -d redis clickhouse
make server
make worker
make ingest
./bin/croupier-server --config configs/server.yaml
ANALYTICS_INGEST_SECRET=dev-secret ./bin/ingest --addr :8088 --secret dev-secret
./bin/analytics-worker
```
