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

### 批次校验与重试语义

- ingest 对整批先校验后发布（DATA-016）：任一条目非法返回 `400` 并携带 `details.index`，全批入队数为 0。
- 发布中途失败返回 `500 queue_write_failed`，`details.enqueued` / `details.failed_index` / `details.total` 显式披露已入队前缀（DATA-017），不再是裸的批次级错误。
- 重试契约：以相同内容整批重发。生产方未携带 `event_id` 的事件由 worker 按载荷指纹派生确定性 UUIDv5（`internal/analytics/worker/worker_insert.go` 的 `deriveEventID`），重复提交与 worker 重投收敛到同一 ClickHouse 自然键，不再产生随机新 id；生产方提供的合法 UUID `event_id` 原样沿用。
- 金额精度（DATA-018）：`amount_cents` 只接受 uint64 值域内的正整数。ingest 以 `UseNumber` 解码并逐位解析——分值小数、负数与超范围值拒绝 `400` 并携带条目位置；worker 同样逐位解析，坏行带原因进死信，不再经 float64 截断。
- 重试保留（DATA-019）：失败消息重投回**源流时不带任何自有保留上限**——源流裁剪（如 `ANALYTICS_REDIS_MAXLEN`）由发布方配置掌控。重试副本不再会在积压超过旧 1000 条上限时驱逐未读条目；重试总量由 `retry_count` 收敛（超过 `maxPendingRetries` 进死信）。
- 聚合恢复（DATA-020）：每个触碰的分钟/日在 Redis 中留有**持久化刷新标记**（`ANALYTICS_AGG_DIRTY_SET`），因此原始事件 ACK 后、聚合行落库前杀进程，重启后仍有刷新义务。HLL 尽力写入失败时另置重建标记（`ANALYTICS_AGG_REBUILD_SET`），下次刷新从持久化的 `analytics.events` 行重建该 HLL（HLL 并集幂等）。重建失败保留标记且不写入虚假的零；HLL 正常写入不触发重建查询。
- 管道告警恢复（DATA-022）：`event_stream_stalled` / `event_volume_drop` 告警只在**正向证据**下关闭——本轮实际读到的该作用域数据行满足恢复规则。作用域从发现窗口消失（完全静默恰是告警中的断流本身）、查询失败或只读到部分结果都按未知处理并保持告警（`events_query_failed` 单独标记未知轮次）。近期速率与基线速率各除以自己的窗口长度，非默认窗口组合不会使比较失真。
- 消费组初始化（DATA-023）：新建组从 `WORKER_GROUP_START` 声明的位点起步——默认 `0`，晚于首批入站启动的 worker 会消费保留积压；`$` 表示只消费新消息。**既有组启动时绝不复复位**：Redis 原生位点即消费位置，跨重启保持，旧应用 checkpoint 不会回放已 ACK 数据。持久化 checkpoint 只在两类场景生效：组不存在时以其建组（Redis 丢组后的续传恢复），或显式 `WORKER_GROUP_RESET=true` 运维复位。建组失败必须记录日志，不再静默吞掉；与其他 worker 的 `BUSYGROUP` 竞态保持对方游标。
- 行为分析全窗口聚合（DATA-024）：摘要、热图、采用明细、漏斗、路径及关卡/剧集/地图报表通过**逐页翻读全部匹配事件**覆盖完整查询窗口——以模型返回的权威 `count` 判定单类型何时读完（请求页大小会被 `ListEvents` 重新归一）。超过聚合上限（`maxWindowEvents`，50000 条）的窗口返回有界样本并在响应 `sample` 对象中披露（`totalEvents`/`scannedEvents`/`sampled`/`coverage`），部分读取绝不冒充完整结果；分页事件端点保留自身页大小契约。
- 实时指标来源（DATA-025）：`onlineUsers` 统计 15 分钟实时窗口内出现 `heartbeat`、`session_start` 或 OTel 点号别名 `session.start` 事件的**去重用户数**；`activeSessions` 统计同窗口内 `session_start`/`session.start` 事件数。业务事件（`login`、`purchase` 等）只计入吞吐（`eventsPerMinute`，全部行为事件 / 15 分钟），不再虚增在线与会话数。`/analytics/realtime` 响应新增 `source` 对象披露每项指标的来源与窗口长度，实时在线曲线使用同一活性事件集合。边界：系统无玩家会话/心跳表，「在线」仅是由分析事件推导的活性代理，并非实时 socket 连接数；窗口内无活性事件的用户按此口径视为不在线，即使连接仍存活。
- 实时趋势分桶（DATA-026）：`/analytics/realtime/series` 用**一次**有界查询（`maxRealtimeBucketRows`，100000）读取整个请求窗口，在 Go 中完成分桶；数据库往返次数与桶数解耦（旧实现每桶两次查询，在最长支持窗口下可达 2880 次）。所有桶都会返回（包括空桶），恰好落在桶边界的事件只计入一个桶。超过行数上限时会通过响应 `sample` 对象披露（`scannedEvents`/`sampled`/`coverage`），绝不把截断读取冒充完整窗口。
- 边界：payments 表无去重列，相同支付行重发仍会追加第二条原始行，依赖按 `order_id` 的读取口径对账（DATA-004/051/056 跟踪）。

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
