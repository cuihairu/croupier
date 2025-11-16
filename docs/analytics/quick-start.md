---
title: 快速开始
---
# 5 分钟上手

本节帮助你快速启动基础依赖、启用采集入口，并通过 HTTP 上报一条事件。

前置要求
- Docker 或本地 ClickHouse/Redis
- curl 或任意 HTTP 客户端

1) 启动基础依赖
```bash
docker compose up -d clickhouse redis
```

2) 启动 Ingestion 与 Worker（示例）
```bash
# 构建二进制
make dev   # 生成到 bin/

# 启动服务（示意参数，按需调整）
bin/server --http_addr :8080
bin/analytics-ingest --http_addr :18080 --redis_url redis://localhost:6379/0 --secret your-secret
bin/analytics-worker --redis_url redis://localhost:6379/0 --clickhouse_url http://localhost:8123
```

3) 客户端上报一条事件
```bash
BODY='[{"event":"session.start","ts":'$(date +%s)'000,"attrs":{"uid":"u1","game_id":"demo"}}]'
TS=$(date +%s)
NONCE=$(openssl rand -hex 8)
SIG=$(printf "%s\n%s\n%s" "$TS" "$NONCE" "$(printf "%s" "$BODY" | shasum -a 256 | awk '{print $1}')" | \
  openssl dgst -sha256 -hmac "your-secret" -binary | base64)
curl -sS -X POST "http://localhost:18080/api/ingest/events" \
  -H "Content-Type: application/json" \
  -H "X-Timestamp: $TS" \
  -H "X-Nonce: $NONCE" \
  -H "X-Signature: $SIG" \
  --data "$BODY"
```

4) 在 ClickHouse 中查看
```sql
-- 连接 CH 后执行：
SELECT event, ts, attrs FROM analytics.events ORDER BY ts DESC LIMIT 10;
```

5) 接入 Grafana（可选）
- 添加 ClickHouse/Prometheus 数据源
- 导入内置看板（在 packs/analytics/*.json 中提供示例）

小结
- 客户端事件 → Ingestion → Redis Stream → Worker → ClickHouse
- 服务端 Traces/Metrics 建议直接走 OTel Collector → ClickHouse/Prometheus

参考
- 指标全景图: ./game-metrics-overview.md
- 采集架构: ./data-collection-architecture.md
