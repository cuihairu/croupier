---
title: API 参考
---

# Ingestion API

鉴权

- 头部：`X-Timestamp`（秒）、`X-Nonce`（随机串）、`X-Signature`（Base64(HMAC-SHA256(secret, `${ts}\n${nonce}\n${sha256(body)}`))）
- 异常码：401/403（签名）、429（限流）、400（格式错误）

POST /api/ingest/events

- 请求体：事件数组，每条至少包含 `event`、`ts`，推荐加 `uid`、`game_id`、`env`

```json
[
  {
    "event": "session.start",
    "ts": 1731700000000,
    "attrs": { "uid": "u1", "game_id": "demo", "env": "dev" }
  }
]
```

- 返回：`{"ok":true}` 或错误详情

POST /api/ingest/payments

- 请求体：支付事件数组（字段同上，业务字段根据需要扩展）

# OTel Collector（服务端）

- 推荐直接接入 OTLP（HTTP/gRPC），采集 traces/metrics/logs
- 参考: ./opentelemetry-integration.md

# Server 端分析 API（/api/v1/analytics/*）

鉴权：同其他 Server REST 端点（Bearer JWT）；游戏范围通过查询参数 `gameId`/`env` 过滤（前端由全局 scope 自动注入）。响应遵循统一契约：成功直接返回业务 JSON，错误返回 `{error, message, details}`。

## 数据仓库（ClickHouse 聚合表，链路 A/C 出口）

GET /api/v1/analytics/warehouse/dau

- 参数：`gameId`、`env`（可选）、`days`（默认 14，最大 90）
- 返回：`{"points":[{"date":"2026-08-24","gameId":"demo","env":"prod","dau":120,"newUsers":12}]}`
- 未配置 `CLICKHOUSE_DSN` 时返回 503 `{"error":"service_unavailable","message":"分析仓库未启用"}`

GET /api/v1/analytics/warehouse/online

- 参数：`gameId`、`env`（可选）、`minutes`（默认 60，最大 1440）
- 返回：`{"points":[{"minute":"2026-08-24T06:00:00Z","online":7}]}`（分钟级 HLL 在线，SUM 归并）

GET /api/v1/analytics/warehouse/revenue

- 参数：`gameId`、`env`（可选）、`days`（默认 14，最大 90）
- 返回：`{"points":[{"date":"2026-08-24","revenueCents":9900,"refundsCents":500,"failed":2}]}`

## 函数调用分析（audit_records 聚合）

GET /api/v1/analytics/invocations/summary

- 参数：`gameId`、`env`、`hours`（默认 24，最大 720）
- 返回：`{"total":13,"failed":3,"successRate":0.769,"avgDurationMs":2.5,"p95DurationMs":8,"topFunctions":[{"functionId":"player.list","total":8,"failed":3,"avgDurationMs":2.1}]}`

GET /api/v1/analytics/invocations/trend

- 参数：`gameId`、`env`、`interval`（`hour` 默认/`day`）
- 返回：`{"points":[{"bucket":"2026-08-23","total":9,"failed":2}]}`

GET /api/v1/analytics/invocations

- 参数：`gameId`、`env`、`functionId`、`outcome`（success/failure）、`page`、`pageSize`（≤100）
- 返回：`{"items":[{"timestamp":"...","functionId":"player.list","actor":"admin","outcome":"success","durationMs":2,"traceId":"..."}],"total":8,"page":1,"pageSize":20}`

## 概览与实时（链路 B）

- GET `/api/v1/analytics/overview` — KPI 概览（DAU/MAU/收入/ARPU 等）
- GET `/api/v1/analytics/realtime` — 实时大屏（SSE，text/event-stream）
- GET `/api/v1/analytics/realtime/series` — 实时曲线序列
- POST `/api/v1/analytics/ingest` — Server 侧事件写入（内部）
- GET/PUT `/api/v1/analytics/filters` — 采样控制（读取/保存过滤器）

## 行为分析（链路 B）

- GET `/api/v1/analytics/behavior/` — 行为事件聚合
- GET `/api/v1/analytics/behavior/events` — 事件明细（分页）
- GET `/api/v1/analytics/behavior/paths` — Top N 路径
- POST `/api/v1/analytics/behavior/funnel` — 漏斗分析
- GET `/api/v1/analytics/behavior/adoption` — 功能采用率
- GET `/api/v1/analytics/behavior/adoption/breakdown` — 采用率分维度下钻

## 支付分析（链路 B）

- GET `/api/v1/analytics/payments/` — 支付汇总
- GET `/api/v1/analytics/payments/summary` — 按日汇总（收入/交易数/用户数）
- GET `/api/v1/analytics/payments/product-trend` — 商品收入趋势
- GET `/api/v1/analytics/payments/transactions` — 交易明细（分页）
- POST `/api/v1/analytics/payments/ingest` — 支付事件写入（内部）

## 留存与关卡（链路 B）

- GET `/api/v1/analytics/retention` — 留存（cohort）
- GET `/api/v1/analytics/levels` — 关卡漏斗/通过率
- GET `/api/v1/analytics/levels/episodes` — 章节指标
- GET `/api/v1/analytics/levels/maps` — 地图热力/死亡点
