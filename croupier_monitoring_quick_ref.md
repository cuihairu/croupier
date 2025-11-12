# Croupier 监控系统快速参考

## 核心文件位置

```
项目结构：
├── internal/
│   ├── analytics/                    # 分析系统核心
│   │   ├── mq/                       # 消息队列层
│   │   │   ├── queue.go              # 队列接口定义
│   │   │   ├── factory.go            # MQ 工厂模式
│   │   │   ├── redis_pub.go          # Redis Streams 实现
│   │   │   ├── kafka_pub.go          # Kafka 实现
│   │   │   └── noop.go               # 无操作实现
│   │   └── worker/
│   │       └── worker.go             # 事件处理 Worker
│   ├── app/server/http/
│   │   ├── server.go                 # HTTP 服务器主文件
│   │   ├── analytics_routes.go       # /api/analytics/* 路由
│   │   ├── analytics_mq.go           # MQ 初始化
│   │   └── di_providers.go           # DI 配置
│   ├── platform/
│   │   └── monitoring/
│   │       └── certificates/
│   │           └── certificates.go   # 证书监控
│   └── cli/common/
│       └── logging.go                # 日志配置与计数
├── configs/analytics/
│   ├── events.yaml                   # 事件定义
│   ├── metrics.yaml                  # 指标定义
│   ├── game_types.yaml               # 游戏类型分类
│   └── taxonomy.yaml                 # 游戏流派分类
├── cmd/
│   ├── agent/main.go                 # Agent 主程序
│   └── edge/main.go                  # Edge 代理
├── tools/adapters/
│   ├── prom/main.go                  # Prometheus 适配器
│   └── http/main.go                  # HTTP 适配器
└── go.mod                            # 依赖声明
```

## 关键环境变量

### Analytics MQ 配置

```bash
# 选择 MQ 类型
ANALYTICS_MQ_TYPE=redis|kafka|noop    # 默认: noop

# Redis 配置
REDIS_URL=redis://host:6379/0
ANALYTICS_REDIS_STREAM_EVENTS=analytics:events
ANALYTICS_REDIS_STREAM_PAYMENTS=analytics:payments
ANALYTICS_REDIS_MAXLEN=1000000
ANALYTICS_REDIS_MAXLEN_APPROX=true

# Kafka 配置
KAFKA_BROKERS=broker1:9092,broker2:9092
ANALYTICS_KAFKA_TOPIC_EVENTS=analytics.events
ANALYTICS_KAFKA_TOPIC_PAYMENTS=analytics.payments

# ClickHouse 配置
CLICKHOUSE_DSN=clickhouse://host:9000/analytics
```

### Worker 配置

```bash
REDIS_URL=redis://localhost:6379/0
ANALYTICS_REDIS_STREAM_EVENTS=analytics:events
ANALYTICS_REDIS_STREAM_PAYMENTS=analytics:payments
WORKER_GROUP=analytics-worker
WORKER_CONSUMER=worker-1
CLICKHOUSE_DSN=clickhouse://localhost:9000/analytics
```

### 日志配置

```bash
LOG_LEVEL=debug|info|warn|error
LOG_FORMAT=console|json
LOG_FILE=logs/server.log
LOG_OUTPUT=stdout|stderr
CROUPIER_LOG_OUTPUT=stdout|stderr
```

## 事件上报流程

```
1. 客户端发送事件
   POST /api/events/track
   {
     "user_id": "...",
     "event": "session.start",
     "event_time": "2025-11-12T10:30:00Z",
     "props": { ... }
   }

2. Server 处理
   ├─ 验证授权 (Authorization header)
   ├─ 提取 game_id, env (X-Game-ID, X-Env headers)
   ├─ 生成 Trace ID (randHex(8))
   ├─ 掩码敏感字段
   └─ 发布到 MQ

3. MQ 转发
   ├─ Redis: analytics:events stream
   └─ Kafka: analytics.events topic

4. Worker 消费
   ├─ XReadGroup 读取消息
   ├─ 解析 JSON 和提取字段
   ├─ 更新 HyperLogLog 计数
   ├─ 插入 ClickHouse
   └─ XAck 确认

5. 数据写入 ClickHouse
   ├─ analytics.events (原始事件)
   ├─ analytics.payments (支付)
   ├─ analytics.minute_online (分钟在线)
   ├─ analytics.daily_users (日活)
   └─ analytics.daily_revenue (日收入)
```

## 核心指标查询

### DAU（日活跃用户）

```sql
-- 从事件表查询
SELECT uniqExact(user_id) 
FROM analytics.events 
WHERE toDate(event_time) = toDate('2025-11-12')
  AND event IN ('login', 'session_start')
  AND game_id = 'game1'
  AND env = 'prod'

-- 从预聚合表查询（更快）
SELECT dau 
FROM analytics.daily_users 
WHERE d = '2025-11-12'
  AND game_id = 'game1'
  AND env = 'prod'
```

### 次日留存率

```sql
-- D0 注册用户数
WITH d0_users AS (
  SELECT DISTINCT user_id 
  FROM analytics.events 
  WHERE toDate(event_time) = '2025-11-11'
    AND event IN ('register', 'first_active')
    AND game_id = 'game1'
)
-- D1 活跃用户数
SELECT 
  count(DISTINCT e.user_id) * 100.0 / count(DISTINCT d0_users.user_id) AS retention_d1
FROM d0_users
LEFT JOIN analytics.events e ON e.user_id = d0_users.user_id
  AND toDate(e.event_time) = '2025-11-12'
  AND e.game_id = 'game1'
```

### 付费相关

```sql
SELECT 
  uniqExact(user_id) AS payers,
  sum(amount_cents) AS revenue_cents,
  sum(amount_cents) / 100.0 AS revenue_usd,
  uniqExact(user_id) * 1.0 / (SELECT dau FROM analytics.daily_users WHERE d = '2025-11-12') AS pay_rate
FROM analytics.payments 
WHERE toDate(time) = '2025-11-12'
  AND status = 'success'
  AND game_id = 'game1'
  AND env = 'prod'
```

## API 端点

### Analytics 概览

```http
GET /api/analytics/overview
  ?game_id=game1
  &env=prod
  &start=2025-11-05T00:00:00Z
  &end=2025-11-12T23:59:59Z

Authorization: Bearer <token>
X-Game-ID: game1
X-Env: prod
```

**响应**:
```json
{
  "dau": 12345,
  "wau": 45678,
  "mau": 89012,
  "new_users": 1234,
  "revenue_cents": 567890,
  "pay_rate": 5.6,
  "arpu": 46.1,
  "arppu": 820.5,
  "d1": 42.3,
  "d7": 18.5,
  "d30": 8.2,
  "series": {
    "new_users": [...],
    "peak_online": [...],
    "revenue_cents": [...]
  }
}
```

## 开发快速启动

### 最小化配置（开发环境）

```bash
# 1. 启动依赖
docker run -d --name redis redis:7-alpine
docker run -d --name clickhouse \
  -p 8123:8123 -p 9000:9000 \
  clickhouse/clickhouse-server:latest

# 2. 配置环境变量
export ANALYTICS_MQ_TYPE=redis
export REDIS_URL=redis://localhost:6379/0
export CLICKHOUSE_DSN=clickhouse://localhost:9000/analytics

# 3. 运行 Server
make build
./bin/server

# 4. 运行 Worker (另一个终端)
cd cmd/worker
go run main.go
```

### 测试事件上报

```bash
# 发送事件
curl -X POST http://localhost:8080/api/events/track \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer dev-token" \
  -H "X-Game-ID: game1" \
  -H "X-Env: dev" \
  -d '{
    "user_id": "user123",
    "session_id": "sess456",
    "event": "session.start",
    "event_time": "2025-11-12T10:30:00Z",
    "platform": "ios",
    "props": {}
  }'

# 查询分析数据
curl -X GET "http://localhost:8080/api/analytics/overview?game_id=game1&env=dev" \
  -H "Authorization: Bearer dev-token"
```

## 数据流追踪

### Redis Streams 查看

```bash
redis-cli

# 查看事件流中的最新消息
XREVRANGE analytics:events + COUNT 5

# 查看消费者组状态
XINFO GROUPS analytics:events

# 查看待确认消息
XPENDING analytics:events analytics-worker
```

### ClickHouse 查看

```bash
clickhouse-client

-- 查看最新事件
SELECT * FROM analytics.events ORDER BY event_time DESC LIMIT 10;

-- 查看表大小
SELECT 
  table,
  sum(bytes) as size
FROM system.parts
WHERE database = 'analytics'
GROUP BY table;

-- 查看分区
SELECT partition, count() 
FROM analytics.events 
GROUP BY partition;
```

## 监控指标概览

### 支持的指标类型

| 类型 | 示例 | 计算方式 |
|------|------|--------|
| counter | dau, wau, mau | uniqExact() |
| ratio | retention_d1, crash_rate | sum(分子) / sum(分母) |
| gauge | arpu, arppu | sum(值) / count(项) |
| histogram | session_length_p50 | quantile(0.5) |

### 指标维度

常见维度：
- platform (ios, android, web, windows, macos)
- region (CN, US, JP, EU, etc.)
- channel (app_store, google_play, web, etc.)
- app_version (1.0.0, 1.1.0, etc.)
- 游戏特定: level_id, game_mode, character_id, card_id

## 故障排查

### Worker 未处理事件

```bash
# 检查 MQ 连接
redis-cli ping  # 应返回 PONG

# 检查 ClickHouse 连接
clickhouse-client -q "SELECT 1"

# 检查消费者组
redis-cli XINFO GROUPS analytics:events

# 查看待确认消息
redis-cli XPENDING analytics:events analytics-worker
```

### 数据延迟

```bash
# 1. 检查 Worker flush 间隔（应为 15s）
# 2. 检查 ClickHouse 写入性能
SELECT 
  table,
  sum(rows),
  sum(bytes)
FROM system.parts
WHERE database = 'analytics'
GROUP BY table;

# 3. 检查网络延迟
curl -I http://clickhouse:8123/
```

### 丢失消息

```bash
# 检查 Redis Streams 大小限制
redis-cli XINFO STREAM analytics:events

# 检查 Kafka 副本因子和 retention
kafka-topics.sh --describe --topic analytics.events

# 恢复消费者组
redis-cli XGROUP SETID analytics:events analytics-worker 0
```

## 性能优化建议

### Redis Streams 优化

```bash
# 增加流大小限制（支持更多消息积压）
ANALYTICS_REDIS_MAXLEN=10000000

# 启用近似清理（牺牲精度换取性能）
ANALYTICS_REDIS_MAXLEN_APPROX=true

# 增加消费批次大小（对应 Worker 中的 Count: 200）
```

### Kafka 优化

```bash
# 增加分区数提高并发
kafka-topics.sh --alter --topic analytics.events --partitions 12

# 增加消费者实例
WORKER_CONSUMER=worker-2,worker-3

# 调整 batch 超时
kafka_batch_timeout: 100ms
```

### ClickHouse 优化

```bash
# 增加 merge 线程
<max_part_insertion_threads>8</max_part_insertion_threads>

# 调整批插入大小
# Worker 代码中的 batch.Send() 频率
```

## 参考链接

- 配置文件：`/Users/cui/Workspaces/croupier/configs/analytics/`
- MQ 代码：`/Users/cui/Workspaces/croupier/internal/analytics/mq/`
- Worker 代码：`/Users/cui/Workspaces/croupier/internal/analytics/worker/`
- Routes 代码：`/Users/cui/Workspaces/croupier/internal/app/server/http/analytics_routes.go`
