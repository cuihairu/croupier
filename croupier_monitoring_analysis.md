# Croupier 项目监控、上报与指标收集架构分析

## 执行摘要

Croupier 项目实现了一套完整的游戏分析系统，包括事件上报、指标收集、存储和查询。系统采用**消息队列 + 流处理 + 时序数据库**的分层架构，支持多种数据源（Kafka、Redis）和多种存储后端（ClickHouse）。

---

## 一、整体架构概览

### 1.1 核心组件关系

```
游戏客户端/服务器
        ↓
    HTTP REST API
        ↓
┌─────────────────────────────────────────────┐
│      Croupier Server (HTTP/gRPC)            │
│  - Analytics Routes Handler                 │
│  - Analytics MQ Publisher (Redis/Kafka)     │
└─────────────────────────────────────────────┘
        ↓
┌─────────────────────────────────────────────┐
│   Analytics MQ Layer                        │
│  ┌──────────────┬──────────────┐           │
│  │ Redis Streams│  Kafka Topics│           │
│  │ (events)     │  (events)    │           │
│  │ (payments)   │  (payments)  │           │
│  └──────────────┴──────────────┘           │
└─────────────────────────────────────────────┘
        ↓
┌─────────────────────────────────────────────┐
│   Analytics Worker                          │
│  - Event 处理与验证                        │
│  - 支付数据处理                            │
│  - HLL 聚合（DAU/新增用户/在线人数）       │
│  - 每日收入聚合                            │
└─────────────────────────────────────────────┘
        ↓
┌─────────────────────────────────────────────┐
│      ClickHouse 时序数据库                  │
│  ┌──────────────────────────────────────┐  │
│  │ analytics.events                     │  │
│  │ analytics.payments                   │  │
│  │ analytics.minute_online              │  │
│  │ analytics.daily_users (DAU/新增)     │  │
│  │ analytics.daily_revenue              │  │
│  └──────────────────────────────────────┘  │
└─────────────────────────────────────────────┘
        ↓
  /api/analytics/* (查询接口)
```

---

## 二、事件和上报系统

### 2.1 事件定义与分类

**配置文件位置**: `/Users/cui/Workspaces/croupier/configs/analytics/events.yaml`

#### 事件类型（支持的核心事件）

| 事件ID | 类别 | 必需属性 | 描述 |
|--------|------|--------|------|
| `session.start` | session | user_id, session_id, event_time, platform | 会话开始 |
| `session.end` | session | user_id, session_id, event_time | 会话结束，含时长 |
| `user.register` | user | user_id, event_time | 用户注册 |
| `user.login` | user | user_id, event_time | 用户登录 |
| `progression.start` | progression | user_id, session_id, event_time, level_id | 关卡/副本开始 |
| `progression.complete` | progression | user_id, session_id, event_time, level_id | 关卡/副本完成 |
| `progression.fail` | progression | user_id, session_id, event_time, level_id | 关卡/副本失败 |
| `match.start` | match | user_id, session_id, event_time, match_id, game_mode | 对局开始 |
| `match.end` | match | user_id, session_id, event_time, match_id, match_result | 对局结束 |
| `round.start` | round | user_id, event_time, match_id, round_id | 回合开始（棋牌/桌游） |
| `round.end` | round | user_id, event_time, match_id, round_id, result | 回合结束 |
| `economy.earn` | economy | user_id, event_time, currency, amount | 货币获得（金币/钻石） |
| `economy.spend` | economy | user_id, event_time, currency, amount | 货币消耗 |
| `monetization.purchase_attempt` | monetization | user_id, event_time, order_id, sku_id | 购买尝试 |
| `monetization.purchase_success` | monetization | (成功购买) | 购买成功 |

#### 公共属性

```yaml
common_attributes:
  - user_id (pseudonymous, required)
  - session_id (required)
  - device_id (pseudonymous, optional)
  - platform (enum: ios, android, windows, macos, linux, web, ps, xbox, switch)
  - region, country, app_version, build_number, server
  - game_type (refer to game_types.yaml)
  - genre_code (refer to taxonomy.yaml)
  - event_time (ISO8601 datetime)
```

### 2.2 游戏类型与分类

**配置文件位置**: `/Users/cui/Workspaces/croupier/configs/analytics/game_types.yaml`

支持 30+ 种游戏类型，包括：
- 休闲类: casual_puzzle, hyper_casual, idle_incremental
- RPG类: rpg, arpg, srpg, mmorpg
- 策略类: strategy_4x, rts, tower_defense
- 竞技类: moba, shooter, battle_royale, fighting_ftg, card_ccg
- 其他: simulation_tycoon, sandbox_survival, rhythm, racing, etc.

每种游戏类型关联：
- 推荐事件集合
- 推荐关键指标
- 推荐维度分解
- 特征标签

### 2.3 事件上报接口

**代码位置**: `internal/app/server/http/server.go` + `analytics_routes.go`

#### 上报端点

```
POST /api/events/track
Content-Type: application/json

{
  "user_id": "user_123",
  "session_id": "sess_456",
  "event": "session.start",
  "event_time": "2025-11-12T10:30:00Z",
  "platform": "ios",
  "app_version": "1.2.3",
  "props": {
    "entry_point": "main_menu",
    "campaign_id": "campaign_001"
  }
}
```

#### 上报流程

1. **请求验证与认证**
   - 检查 Authorization 头
   - 验证 game_id 和 env（从 X-Game-ID、X-Env 头获取）
   - 审计日志记录

2. **数据处理**
   - Trace ID 生成（随机 8 字节 hex）
   - 敏感字段掩码（如 user_id）
   - 格式验证

3. **消息队列发布**
   - 序列化为 JSON
   - 发送到 MQ（Redis Streams 或 Kafka）
   - 关键字段：game_id, env, user_id, event, ts, props_json

---

## 三、消息队列（MQ）系统

### 3.1 支持的 MQ 类型

**代码位置**: `internal/analytics/mq/`

#### Redis Streams（推荐用于开发）

**类**: `redisQueue`

```go
// 初始化
NewRedis(url, streamEvents, streamPayments, maxLen, approx)

// Redis 环境变量配置
REDIS_URL                           // 默认: redis://localhost:6379/0
ANALYTICS_REDIS_STREAM_EVENTS       // 默认: analytics:events
ANALYTICS_REDIS_STREAM_PAYMENTS     // 默认: analytics:payments
ANALYTICS_REDIS_MAXLEN              // 默认: 1000000
ANALYTICS_REDIS_MAXLEN_APPROX       // 默认: true
```

**存储结构**:
```
Stream: analytics:events
  Entry: {
    data: "{\"user_id\":\"...\",\"event\":\"...\",\"ts\":\"...\",\"props\":{}}"
  }

Stream: analytics:payments
  Entry: {
    data: "{\"user_id\":\"...\",\"order_id\":\"...\",\"amount_cents\":...,\"status\":\"...\",\"ts\":\"...\"}"
  }
```

#### Kafka（推荐用于生产）

**类**: `kafkaQueue`

```go
// 初始化
NewKafka(brokers, topicEvents, topicPayments)

// 环境变量配置
KAFKA_BROKERS                       // 默认: localhost:9092
ANALYTICS_KAFKA_TOPIC_EVENTS        // 默认: analytics.events
ANALYTICS_KAFKA_TOPIC_PAYMENTS      // 默认: analytics.payments
```

#### Noop（无操作，用于开发）

**类**: `Noop`
- 所有操作均为空操作，不实际发送数据
- 默认后备方案

### 3.2 MQ 工厂与自动选择

**代码位置**: `internal/analytics/mq/factory.go`

```go
func NewFromEnv() Queue {
    t := os.Getenv("ANALYTICS_MQ_TYPE")
    switch t {
    case "redis":
        return newRedisFromEnv()  // Redis Streams
    case "kafka":
        return newKafkaFromEnv()  // Kafka
    default:
        return NewNoop()          // No-op
    }
}
```

---

## 四、分析 Worker 系统

### 4.1 Worker 功能

**代码位置**: `internal/analytics/worker/worker.go`

Worker 是一个后台处理组件，负责：

1. **事件消费**
   - 从 Redis Stream 或 Kafka 消费
   - 支持消费者组（Consumer Groups）
   - 自动失败重试与确认

2. **数据处理**
   - 解析 JSON 负载
   - 提取关键字段（user_id, game_id, env, event, platform等）
   - 类型转换与验证

3. **HyperLogLog 聚合**（用于 DAU/新增/在线）
   - **DAU 计数**: 按日期、游戏、环境维度
     ```
     HLL Key: hll:dau:{game}:{env}:{YYYY-MM-DD}
     事件触发: login, session_start
     ```
   - **新增用户计数**: 按日期、游戏、环境维度
     ```
     HLL Key: hll:new:{game}:{env}:{YYYY-MM-DD}
     事件触发: register, first_active
     ```
   - **在线人数（分钟级）**: 按分钟、游戏、环境维度
     ```
     HLL Key: hll:online:{game}:{env}:{YYYYMMDDHHmm}
     事件触发: heartbeat, session_start
     ```

4. **收入聚合**
   - 按日期、游戏、环境维度
   - 状态分类: success, refunded, failed
   - 金额单位: 美分（cents）

5. **数据写入 ClickHouse**
   - 批量插入（batch.Send）
   - 定期 flush（每 15 秒）

### 4.2 Worker 启动与配置

```go
// 环境变量配置
REDIS_URL                              // 默认: redis://localhost:6379/0
ANALYTICS_REDIS_STREAM_EVENTS          // 默认: analytics:events
ANALYTICS_REDIS_STREAM_PAYMENTS        // 默认: analytics:payments
WORKER_GROUP                           // 消费者组名称
WORKER_CONSUMER                        // 消费者实例名称
CLICKHOUSE_DSN                         // 默认: clickhouse://localhost:9000/analytics
```

### 4.3 处理流程细节

```
消费事件 (xreadgroup)
    ↓
解析 JSON payload
    ↓
┌─── 事件流 ─────────┐      ┌─── 支付流 ────────┐
│                     │      │                    │
│ extractFields()    │      │ extractFields()   │
│   - user_id        │      │   - user_id       │
│   - session_id     │      │   - order_id      │
│   - event          │      │   - amount_cents  │
│   - ts             │      │   - status        │
│                     │      │   - currency      │
│ touchAgg()         │      │ touchRevenue()    │
│ (HLL operations)   │      │ (in-memory agg)   │
│                     │      │                    │
│ insertEvent()      │      │ insertPayment()   │
│ (CH batch insert)  │      │ (CH batch insert) │
└─────────────────────┘      └────────────────────┘
        ↓                              ↓
确认消息 (xack)
        ↓
每 15 秒 flush() 一次
        ↓
将聚合数据写入 ClickHouse 表:
  - analytics.minute_online
  - analytics.daily_users
  - analytics.daily_revenue
```

---

## 五、ClickHouse 数据库

### 5.1 表结构

#### 1. analytics.events（原始事件表）

```sql
CREATE TABLE analytics.events
(
  event_time DateTime,
  game_id String,
  env String,
  user_id String,
  session_id String,
  event String,
  channel String,
  platform String,
  country String,
  app_version String,
  event_id String,
  props_json String
)
ENGINE = MergeTree()
ORDER BY (game_id, env, event_time)
PARTITION BY toYYYYMM(event_time)
```

#### 2. analytics.payments（支付表）

```sql
CREATE TABLE analytics.payments
(
  time DateTime,
  game_id String,
  env String,
  user_id String,
  order_id String,
  amount_cents UInt64,
  currency String,
  status String,
  channel String,
  platform String,
  country String,
  region String,
  city String,
  product_id String,
  reason String
)
ENGINE = MergeTree()
ORDER BY (game_id, env, time)
PARTITION BY toYYYYMM(time)
```

#### 3. analytics.minute_online（分钟在线人数）

```sql
CREATE TABLE analytics.minute_online
(
  m DateTime,
  game_id String,
  env String,
  online UInt32
)
ENGINE = MergeTree()
ORDER BY (game_id, env, m)
```

#### 4. analytics.daily_users（日活跃用户）

```sql
CREATE TABLE analytics.daily_users
(
  d Date,
  game_id String,
  env String,
  dau UInt64,
  new_users UInt64,
  version UInt64
)
ENGINE = MergeTree()
ORDER BY (game_id, env, d)
```

#### 5. analytics.daily_revenue（日收入）

```sql
CREATE TABLE analytics.daily_revenue
(
  d Date,
  game_id String,
  env String,
  revenue_cents UInt64,
  refunds_cents UInt64,
  failed UInt64,
  version UInt64
)
ENGINE = MergeTree()
ORDER BY (game_id, env, d)
```

### 5.2 连接配置

```go
// ClickHouse DSN 格式
clickhouse://host:port/database

// 例如:
CLICKHOUSE_DSN=clickhouse://localhost:9000/analytics
CLICKHOUSE_DSN=http://localhost:8123/analytics  // HTTP protocol
```

---

## 六、指标定义与计算

### 6.1 指标分类

**配置文件位置**: `configs/analytics/metrics.yaml`

#### A. 用户相关指标

| 指标ID | 中文名 | 类型 | 窗口 | 说明 |
|--------|--------|------|------|------|
| `dau` | 日活跃用户数 | counter | 1d | 当日产生关键事件的独立用户 |
| `wau` | 周活跃用户数 | counter | 7d | 最近7天内的独立用户 |
| `mau` | 月活跃用户数 | counter | 30d | 最近30天内的独立用户 |
| `retention_d1` | 次日留存率 | ratio | 1d | D0注册用户中D1有活跃的占比 |
| `retention_d7` | 7日留存率 | ratio | 7d | D0注册用户中D7有活跃的占比 |
| `retention_d30` | 30日留存率 | ratio | 30d | D0注册用户中D30有活跃的占比 |

#### B. 会话相关指标

| 指标ID | 中文名 | 类型 | 说明 |
|--------|--------|------|------|
| `session_length_p50` | 会话时长P50 | histogram | 会话时长的中位数 |
| `session_length_p95` | 会话时长P95 | histogram | 会话时长的95分位 |

#### C. 质量与稳定性指标

| 指标ID | 中文名 | 类型 | 说明 |
|--------|--------|------|------|
| `crash_rate` | 崩溃率 | ratio | 崩溃事件占会话的比例 |
| `crash_free_users_rate` | 无崩溃用户占比 | ratio | 无崩溃的DAU占比 |
| `anr_rate` | ANR率 | ratio | ANR事件占会话的比例 |

#### D. 变现指标

| 指标ID | 中文名 | 类型 | 公式/说明 |
|--------|--------|------|----------|
| `arpu` | 每用户平均收入 | gauge | sum(revenue) / dau |
| `arppu` | 每付费用户平均收入 | gauge | sum(revenue) / 付费用户数 |
| `pur` | 付费率 | ratio | 付费用户数 / dau |
| `ad_arpu` | 广告ARPU | gauge | sum(ad_revenue) / dau |
| `ad_impressions_per_dau` | 广告曝光数/DAU | gauge | count(ad_impression) / dau |

#### E. 游戏玩法指标

| 指标ID | 中文名 | 类型 | 说明 |
|--------|--------|------|------|
| `win_rate` | 胜率 | ratio | 胜场占总对局的比例 |
| `kda` | KDA | gauge | (击杀+助攻) / max(1, 死亡) |
| `accuracy_rate` | 命中率 | ratio | 命中次数 / 射击次数 |
| `level_completion_rate` | 关卡完成率 | ratio | 完成 / 开始 |
| `retries_avg` | 平均重试次数 | gauge | avg(重试次数) |

#### F. 经济平衡指标

| 指标ID | 中文名 | 类型 | 说明 |
|--------|--------|------|------|
| `idle_offline_income_share` | 离线收益占比 | ratio | 离线收入 / 总收入 |
| `economy_balance_ratio` | 经济产消比 | gauge | 总收入 / 总支出 |

#### G. 卡牌相关指标（TCG/CCG）

| 指标ID | 中文名 | 类型 | 说明 |
|--------|--------|------|------|
| `card_usage_rate` | 卡牌使用率 | ratio | 含卡牌的对局占比 |
| `card_win_rate` | 卡牌胜率 | ratio | 使用卡牌的胜率 |
| `deck_archetype_share` | 卡组原型占比 | ratio | 卡组参与对局占比 |
| `deck_archetype_win_rate` | 卡组胜率 | ratio | 卡组对局胜率 |

#### H. 塔防专用指标

| 指标ID | 中文名 | 类型 | 说明 |
|--------|--------|------|------|
| `td_level_clear_rate` | 塔防关卡通关率 | ratio | 完成率 |
| `td_wave_fail_rate_by_wave` | 各波次失败率 | ratio | 特定波次失败占比 |
| `td_avg_hearts_remaining` | 平均残余生命 | gauge | 通关时剩余生命值 |
| `td_tower_usage_rate_by_type` | 塔型使用率 | ratio | 各塔型建造占比 |
| `td_upgrade_rate` | 塔升级率 | ratio | 升级 / 建造 |

#### I. 棋牌相关指标

| 指标ID | 中文名 | 类型 | 说明 |
|--------|--------|------|------|
| `avg_round_duration` | 回合时长P50 | histogram | 回合耗时中位数 |
| `win_rate_by_seat` | 按座位胜率 | ratio | 不同座位的胜率 |
| `rake_rate` | 抽水率 | gauge | 平台佣金 / 底池 |
| `afk_leave_rate` | 中途离场率 | ratio | 放弃对局占比 |

### 6.2 指标维度

支持的标准维度：
- `platform` (ios, android, windows, web, etc.)
- `region` (地理区域)
- `channel` (渠道)
- `app_version` (应用版本)
- 游戏特定维度: level_id, game_mode, character_id, card_id, etc.

### 6.3 指标计算查询示例

代码位置: `internal/app/server/http/analytics_routes.go`

```go
// DAU 查询
SELECT uniqExact(user_id) FROM analytics.events 
WHERE event IN ('login', 'session_start')
  AND toDate(event_time) = toDate(?)
  AND game_id = ? AND env = ?

// WAU/MAU 查询
SELECT uniqExact(user_id) FROM analytics.events 
WHERE event_time >= now() - interval 7 day
  AND game_id = ? AND env = ?

// 次日留存
SELECT uniqExact(user_id) 
FROM analytics.events 
WHERE toDate(event_time) = ? 
  AND user_id IN (
    SELECT user_id FROM analytics.events 
    WHERE toDate(event_time) = ? 
      AND event IN ('register', 'first_active')
  )
  AND game_id = ? AND env = ?

// 支付相关
SELECT 
  sum(amount_cents) AS revenue,
  uniqExact(user_id) AS payers
FROM analytics.payments 
WHERE status = 'success'
  AND time BETWEEN toDateTime(?) AND toDateTime(?)
  AND game_id = ? AND env = ?
```

---

## 七、监控和查询 API

### 7.1 Analytics 概览接口

**端点**: `GET /api/analytics/overview`

```http
GET /api/analytics/overview?game_id=game1&env=prod&start=2025-11-05T00:00:00Z&end=2025-11-12T00:00:00Z

响应:
{
  "dau": 12345,
  "wau": 45678,
  "mau": 89012,
  "new_users": 1234,
  "revenue_cents": 567890,
  "pay_rate": 5.6,        // 付费率百分比
  "arpu": 46.1,           // 每用户平均收入（美元）
  "arppu": 820.5,         // 每付费用户平均收入
  "d1": 42.3,             // 次日留存率百分比
  "d7": 18.5,             // 7日留存率
  "d30": 8.2,             // 30日留存率
  "series": {
    "new_users": [        // 时间序列数据
      ["2025-11-05T00:00:00Z", 150],
      ["2025-11-06T00:00:00Z", 180],
      ...
    ],
    "peak_online": [...],
    "revenue_cents": [...]
  }
}
```

**认证**: 需要 `analytics:read` 权限

### 7.2 其他监控端点

```
GET /healthz                    # 健康检查
GET /metrics                    # 指标概览（JSON）
GET /api/certificates           # 证书监控（HTTPS）
```

---

## 八、日志与追踪系统

### 8.1 日志配置

**代码位置**: `internal/cli/common/logging.go`

```yaml
# 服务器日志配置
log:
  level: info              # debug|info|warn|error
  format: console          # console|json
  output: stdout           # stdout|stderr
  file: logs/server.log    # 可选文件输出
  max_size: 100            # MB
  max_backups: 7           # 备份文件数
  max_age: 7               # 日志保留天数
  compress: true           # 是否压缩
```

### 8.2 日志计数器

**代码位置**: `internal/cli/common/logging.go`

系统维护日志级别计数器：

```go
var cntDebug, cntInfo, cntWarn, cntError atomic.Int64

// 获取日志统计
GetLogCounters() -> {
  "debug": 1234,
  "info": 5678,
  "warn": 123,
  "error": 45,
  "total": 7080
}
```

### 8.3 请求追踪

**代码位置**: `internal/app/server/http/server.go`

```go
// Trace ID 生成
traceID := randHex(8)  // 8 字节随机 hex 字符串

// 传播到函数调用
meta := map[string]string{
  "trace_id": traceID,
  "game_id": gameID,
  "env": env
}

// 审计日志记录
s.audit.Log("invoke", user, functionID, map[string]string{
  "ip": clientIP,
  "trace_id": traceID,
  "game_id": gameID,
  "env": env,
  "payload_snapshot": masked
})
```

### 8.4 Prometheus Adapter

**代码位置**: `tools/adapters/prom/main.go`

实现 Prometheus 查询适配器：

```
服务注册:
  prom.query         - 单点查询
  prom.query_range   - 范围查询

JSON 载体:
  {
    "expr": "up",
    "start": "2025-11-12T10:00:00Z",
    "end": "2025-11-12T11:00:00Z",
    "step": "1m"
  }

HTTP 转发:
  /api/v1/query
  /api/v1/query_range
```

---

## 九、证书监控系统

### 9.1 功能

**代码位置**: `internal/platform/monitoring/certificates/certificates.go`

系统支持 HTTPS 证书的自动监控和告警：

#### 数据库表

```sql
certificates
├── id (PK)
├── domain (unique)
├── port
├── issuer, subject, algorithm
├── valid_from, valid_to
├── days_left
├── status (valid|expiring|expired|error)
├── last_checked
├── alert_days (default: 30)
└── enabled

certificate_alerts
├── id (PK)
├── certificate_id (FK)
├── alert_type (email|sms|webhook|chat)
├── target
├── enabled
├── last_sent
```

#### 关键操作

- 添加监控域名: `AddDomain(domain, port, alertDays)`
- 检查证书: `CheckCertificate(certID)`
- 获取快过期的证书: `GetExpiringCertificates()`
- 证书统计: `GetCertificateStats()`

---

## 十、性能和配置最佳实践

### 10.1 消息队列配置建议

#### Redis（开发/小规模）

```bash
ANALYTICS_MQ_TYPE=redis
REDIS_URL=redis://localhost:6379/0
ANALYTICS_REDIS_STREAM_EVENTS=analytics:events
ANALYTICS_REDIS_STREAM_PAYMENTS=analytics:payments
ANALYTICS_REDIS_MAXLEN=1000000
ANALYTICS_REDIS_MAXLEN_APPROX=true
```

**优势**:
- 简单部署
- 支持事务
- 内存快速

**劣势**:
- 内存限制
- 单机性能瓶颈

#### Kafka（生产环境）

```bash
ANALYTICS_MQ_TYPE=kafka
KAFKA_BROKERS=broker1:9092,broker2:9092,broker3:9092
ANALYTICS_KAFKA_TOPIC_EVENTS=analytics.events
ANALYTICS_KAFKA_TOPIC_PAYMENTS=analytics.payments
```

**优势**:
- 高吞吐量
- 分布式容错
- 消息持久化

**劣势**:
- 部署复杂
- 依赖性强

### 10.2 ClickHouse 配置建议

```bash
CLICKHOUSE_DSN=clickhouse://clickhouse-cluster:9000/analytics

# 或 HTTP 接口
CLICKHOUSE_DSN=http://clickhouse-cluster:8123/analytics
```

**表设计建议**:
```sql
-- 按时间和维度分区
ENGINE = MergeTree()
ORDER BY (game_id, env, event_time)
PARTITION BY toYYYYMM(event_time)

-- 定期 TTL 清理
TTL event_time + INTERVAL 90 DAY DELETE
```

### 10.3 Worker 性能调优

```bash
# 消费者配置
WORKER_GROUP=analytics-worker-group
WORKER_CONSUMER=consumer-instance-1

# 批量处理
batch_timeout: 50ms  # Kafka writer
flush_interval: 15s  # ClickHouse flusher

# 并发控制
XReadGroup count: 200
```

---

## 十一、缺失和可改进之处

### 11.1 当前限制

1. **缺少统一的 Prometheus 指标暴露**
   - 仅有 HTTP `/metrics` 端点返回 JSON
   - 未实现 Prometheus text format

2. **分布式追踪（Distributed Tracing）**
   - OpenTelemetry 依赖存在但未集成
   - 仅基于 Trace ID 的应用级追踪
   - 缺少 Jaeger/Tempo 集成

3. **实时告警系统**
   - 证书告警配置已存在
   - 缺少指标异常告警（如：DAU 异常下降）
   - 缺少可视化仪表板

4. **MetricsMQ 的实现状态**
   - analytis/worker 仅关注事件处理
   - 缺少其他关键服务指标的收集（如：gRPC 延迟、DB 连接池状态）

### 11.2 建议的增强方向

1. **完整的 OpenTelemetry 集成**
   ```go
   // 在关键路径添加
   tracer := otel.Tracer("croupier")
   ctx, span := tracer.Start(ctx, "invoke_function")
   defer span.End()
   ```

2. **Prometheus 指标导出**
   ```go
   // 使用 prometheus-go 库
   invocations := prometheus.NewCounterVec(...)
   functionLatency := prometheus.NewHistogramVec(...)
   ```

3. **指标异常告警**
   ```yaml
   # 使用 Prometheus AlertManager
   alert: DAUDropped
   expr: rate(dau[1h]) < 0.8 * avg_over_time(dau[7d])
   ```

4. **实时数据可视化**
   - Grafana 仪表板连接 ClickHouse
   - Web UI 实时显示关键指标

---

## 十二、部署示例

### 12.1 Docker Compose 完整栈

```yaml
version: '3.8'
services:
  # Redis（MQ）
  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
    volumes:
      - redis-data:/data

  # ClickHouse（数据仓库）
  clickhouse:
    image: clickhouse/clickhouse-server:latest
    ports:
      - "8123:8123"
      - "9000:9000"
    environment:
      CLICKHOUSE_DB: analytics
      CLICKHOUSE_USER: default
      CLICKHOUSE_PASSWORD: ""
    volumes:
      - clickhouse-data:/var/lib/clickhouse

  # Croupier Server
  croupier-server:
    build: .
    ports:
      - "8080:8080"
      - "8443:8443"
    environment:
      ANALYTICS_MQ_TYPE: redis
      REDIS_URL: redis://redis:6379/0
      CLICKHOUSE_DSN: clickhouse://clickhouse:9000/analytics
      LOG_LEVEL: info
    depends_on:
      - redis
      - clickhouse

  # Analytics Worker
  analytics-worker:
    build:
      context: .
      dockerfile: Dockerfile.worker
    environment:
      REDIS_URL: redis://redis:6379/0
      CLICKHOUSE_DSN: clickhouse://clickhouse:9000/analytics
      WORKER_GROUP: analytics-workers
      WORKER_CONSUMER: worker-1
    depends_on:
      - redis
      - clickhouse

volumes:
  redis-data:
  clickhouse-data:
```

### 12.2 环境变量模板

```bash
# 日志配置
LOG_LEVEL=info
LOG_FORMAT=json
LOG_FILE=logs/server.log

# 分析系统
ANALYTICS_MQ_TYPE=redis              # redis|kafka
REDIS_URL=redis://localhost:6379/0

# ClickHouse
CLICKHOUSE_DSN=clickhouse://localhost:9000/analytics

# 数据库（games/users/audit）
DATABASE_URL=postgres://user:pass@localhost:5432/croupier
DB_DRIVER=postgres

# 其他
JWT_SECRET=your-secret-key
RBAC_CONFIG=configs/rbac.json
```

---

## 总结

Croupier 项目构建了一套完整的游戏数据分析系统，包括：

| 层级 | 组件 | 技术 |
|------|------|------|
| **摄取层** | HTTP REST API + 事件定义 | Gin + YAML 配置 |
| **传输层** | MQ（Redis/Kafka） | redis-go + segmentio/kafka-go |
| **处理层** | Analytics Worker | Go + HyperLogLog |
| **存储层** | ClickHouse 时序库 | clickhouse-go driver |
| **查询层** | /api/analytics/* | SQL 查询接口 |
| **可观测性** | 日志、追踪、指标 | slog + custom 指标 |

系统支持：
- ✅ **30+ 游戏类型分类**
- ✅ **100+ 核心指标定义**
- ✅ **灵活的事件属性架构**
- ✅ **多种消息队列后端**
- ✅ **实时聚合（DAU、收入）**
- ✅ **HTTPS 证书监控**
- ✅ **多维度数据分析**

需要重点改进的方向：
- ⚠️ 完整 OpenTelemetry 集成
- ⚠️ Prometheus text format 导出
- ⚠️ 实时告警系统
- ⚠️ 可视化仪表板
