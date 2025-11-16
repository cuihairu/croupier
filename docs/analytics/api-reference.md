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
  {"event":"session.start","ts":1731700000000,"attrs":{"uid":"u1","game_id":"demo","env":"dev"}}
]
```
- 返回：`{"ok":true}` 或错误详情

POST /api/ingest/payments
- 请求体：支付事件数组（字段同上，业务字段根据需要扩展）

# OTel Collector（服务端）
- 推荐直接接入 OTLP（HTTP/gRPC），采集 traces/metrics/logs
- 参考: ./opentelemetry-integration.md
