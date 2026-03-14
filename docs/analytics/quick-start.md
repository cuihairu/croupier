---
title: 快速开始
---

# 5 分钟上手

本节帮助你快速启动基础依赖、启用采集入口，并通过 HTTP 上报一条事件。

前置要求：

- Docker 或本地 ClickHouse/Redis
- curl 或任意 HTTP 客户端

## 1. 启动基础依赖

```bash
docker compose up -d clickhouse redis
```

## 2. 启动服务

```bash
make build
./bin/croupier-server --config configs/server.yaml
ANALYTICS_INGEST_SECRET=your-secret ./bin/ingest --addr :18080 --secret your-secret
REDIS_URL=redis://localhost:6379/0 ./bin/analytics-worker
```

## 3. 上报一条事件

```bash
BODY='[{"game_id":"demo","env":"dev","event":"session.start","ts":'$(date +%s)'000}]'
TS=$(date +%s)
NONCE=$(openssl rand -hex 8)
SIG=$(printf "%s\n%s\n%s" "$TS" "$NONCE" "$(printf "%s" "$BODY" | shasum -a 256 | awk '{print $1}')" | \
  openssl dgst -sha256 -hmac "your-secret" -binary | base64)

curl -sS -X POST "http://localhost:18080/api/ingest/events" \
  -H "Content-Type: application/json" \
  -H "X-Game-Id: demo" \
  -H "X-Timestamp: $TS" \
  -H "X-Nonce: $NONCE" \
  -H "X-Signature: $SIG" \
  --data "$BODY"
```

## 4. 健康检查

```bash
curl http://localhost:18080/healthz
```
