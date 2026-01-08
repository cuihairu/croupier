# 事件驱动架构设计

## 概述

本文档描述 Croupier 系统的事件驱动架构，该架构支持数据分析、活动系统和未来扩展的事件订阅者。

## 设计目标

1. **统一事件采集** - 客户端/游戏服务器只需发送一次事件
2. **解耦订阅者** - 数据分析、活动系统等独立订阅，互不影响
3. **可扩展性** - 支持新增订阅者无需修改现有代码
4. **高可用性** - 支持事件重试、死信队列、故障恢复
5. **性能优化** - 批量处理、异步处理、检查点机制

## 架构图

```
┌─────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┐
│                                                                  事件驱动架构                                                       │
├─────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
│                                                                                                                             │
│  ┌──────────────┐     ┌──────────────┐     ┌──────────────┐                                                                │
│  │ Game Client  │     │ Game Server  │     │  Admin API   │                                                                │
│  │   (SDK)      │     │   (Backend)  │     │   (手动触发)   │                                                                │
│  └──────┬───────┘     └──────┬───────┘     └──────┬───────┘                                                                │
│         │                    │                     │                                                                       │
│         └────────────────────┼─────────────────────┘                                                                       │
│                              │                                                                                              │
│                              ▼                                                                                              │
│  ┌─────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┐   │
│  │                                                       Event Gateway Service                                          │   │
│  │                                                   (事件网关服务 - 独立进程)                                            │   │
│  │  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐              │   │
│  │  │  HTTP Collector │  │  gRPC Collector│  │  Event Validator│  │  Event Enricher │  │  Rate Limiter   │              │   │
│  │  │   /api/events   │  │   EventService │  │   (Schema)      │  │  (Context)      │  │                 │              │   │
│  │  └────────┬────────┘  └────────┬────────┘  └────────┬────────┘  └────────┬────────┘  └─────────────────┘              │   │
│  └───────────┼────────────────────┼───────────────────┼───────────────────┼────────────────────────────────────────────────┘   │
│              │                    │                   │                   │                                                   │
│  ┌───────────┴────────────────────┴───────────────────┴───────────────────┴────────────────────────────────────────────────┐   │
│  │                                                      Event Bus / Message Queue                                          │   │
│  │                                                                                                                     Redis   │
│  │  ┌───────────────────────────────────────────────────────────────────────────────────────────────────────────────┐   │   │
│  │  │                                          events (主事件流)                                                       │   │   │
│  │  │  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐                 │   │   │
│  │  │  │  event  │ │  event  │ │  event  │ │  event  │ │  event  │ │  event  │ │  event  │ │  event  │  ...          │   │   │
│  │  │  └─────────┘ └─────────┘ └─────────┘ └─────────┘ └─────────┘ └─────────┘ └─────────┘ └─────────┘                 │   │   │
│  │  └───────────────────────────────────────────────────────────────────────────────────────────────────────────────┘   │   │
│  │                                                                                                                      │   │
│  │  ┌───────────────────────────────────────────────────────────────────────────────────────────────────────────────┐   │   │
│  │  │                                       events:high_priority (高优先级/实时)                                        │   │   │
│  │  └───────────────────────────────────────────────────────────────────────────────────────────────────────────────┘   │   │
│  │                                                                                                                      │   │
│  │  ┌───────────────────────────────────────────────────────────────────────────────────────────────────────────────┐   │   │
│  │  │                                        events:dlq (死信队列)                                                     │   │   │
│  │  └───────────────────────────────────────────────────────────────────────────────────────────────────────────────┘   │   │
│  └─────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┘   │
│                                                                                                                             │
│                              │                    │                    │                                                   │
│              ┌───────────────┴────────────────────┴────────────────────┴───────────────────────────────────────┐         │
│              │                                     │                                      │                    │         │
│              ▼                                     ▼                                      ▼                    ▼         │
│  ┌───────────────────────┐           ┌───────────────────────┐           ┌───────────────────────┐   ┌──────────────────┐  │
│  │  Analytics Worker     │           │   Campaign Worker     │           │   (Future) Worker     │   │   Other Services  │  │
│  │                       │           │                       │           │                       │   │                  │  │
│  │  - ClickHouse 写入    │           │  - 触发器匹配         │           │  - 实时通知           │   │  - Webhook       │  │
│  │  - 聚合计算 (DAU/MAU) │           │  - 条件评估           │           │  - 风控检测           │   │  - 第三方集成     │  │
│  │  - 指标刷新           │           │  - 动作执行           │           │  - 日志分析           │   │                  │  │
│  │                       │           │  - 奖励发放           │           │                       │   │                  │  │
│  └───────────────────────┘           └───────────────────────┘           └───────────────────────┘   └──────────────────┘  │
│                                                                                                                             │
└─────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┘
```

## 核心组件

### 1. Event Gateway Service (事件网关服务)

**职责**: 统一的事件采集入口，接收所有客户端/服务器上报的事件。

**功能**:
- 接收 HTTP/gRPC 事件上报
- 事件验证 (Schema Validation)
- 事件丰富化 (Enrichment - 添加 IP、时间、地理位置等)
- 限流保护 (Rate Limiting)
- 发布到消息队列

**接口定义**:

```protobuf
// proto/event/v1/event_gateway.proto

syntax = "proto3";

package event.v1;

import "google/protobuf/timestamp.proto";
import "google/protobuf/struct.proto";

// 事件网关服务
service EventGateway {
  // 上报单个事件
  rpc PublishEvent(Event) returns (PublishResponse);

  // 批量上报事件
  rpc PublishEvents(EventBatch) returns (PublishBatchResponse);

  // 获取事件 Schema
  rpc GetEventSchema(GetSchemaRequest) returns (EventSchema);

  // 获取上报状态
  rpc GetPublishStatus(GetStatusRequest) returns (PublishStatus);
}

// 事件定义
message Event {
  // 必填字段
  string event_id = 1;           // 事件唯一 ID (可选，系统自动生成)
  string event_type = 2;         // 事件类型 (见 EventType 枚举)
  string player_id = 3;          // 玩家 ID
  string game_id = 4;            // 游戏 ID
  string env = 5;                // 环境 (dev/staging/prod)

  // 时间信息
  google.protobuf.Timestamp event_time = 10;     // 事件发生时间
  google.protobuf.Timestamp receive_time = 11;   // 接收时间 (服务端填充)

  // 事件属性 (动态结构)
  google.protobuf.Struct properties = 20;

  // 上下文信息 (客户端填充)
  EventContext context = 30;
}

// 事件上下文
message EventContext {
  string session_id = 1;         // 会话 ID
  string server_id = 2;          // 服务器 ID
  string channel = 3;            // 渠道
  string platform = 4;           // 平台 (ios/android/web/pc)
  string app_version = 5;        // 应用版本
  string device_id = 6;          // 设备 ID
  string ip = 7;                 // IP 地址
  string country = 8;            // 国家
  string region = 9;             // 地区
  map<string, string> custom = 100;  // 自定义字段
}

// 批量事件
message EventBatch {
  repeated Event events = 1;
  bool require_all = 2;          // 是否要求全部成功
}

// 发布响应
message PublishResponse {
  bool success = 1;
  string event_id = 2;
  string error_message = 3;
  string trace_id = 4;           // 追踪 ID
}

// 批量发布响应
message PublishBatchResponse {
  int32 success_count = 1;
  int32 failure_count = 2;
  repeated PublishResponse results = 3;
}

// Schema 请求
message GetSchemaRequest {
  string event_type = 1;
}

// 事件 Schema
message EventSchema {
  string event_type = 1;
  string description = 2;
  string version = 3;
  google.protobuf.Struct properties_schema = 4;  // JSON Schema 格式
  repeated string required_fields = 5;
}

// 状态查询
message GetStatusRequest {
  string trace_id = 1;
  string event_id = 2;
}

// 发布状态
message PublishStatus {
  string status = 1;             // pending/processed/failed
  string error_message = 2;
  repeated string subscribers = 3;  // 已处理的订阅者
}
```

### 2. Event Bus (事件总线)

**实现**: Redis Streams / Kafka (可配置切换)

**Stream 定义**:

| Stream 名称 | 用途 | 优先级 | TTL |
|------------|------|-------|-----|
| `events` | 主事件流 | 普通 | 7天 |
| `events:high_priority` | 高优先级事件 (如支付) | 高 | 30天 |
| `events:dlq` | 死信队列 | - | 90天 |

### 3. 事件类型定义

```go
// internal/event/types/event_type.go

package types

// EventType 事件类型枚举
type EventType string

const (
    // ========== 账户事件 ==========
    EventTypePlayerRegister   EventType = "player.register"   // 玩家注册
    EventTypePlayerLogin      EventType = "player.login"      // 玩家登录
    EventTypePlayerLogout     EventType = "player.logout"     // 玩家登出
    EventTypePlayerSessionEnd EventType = "player.session_end" // 会话结束

    // ========== 经济事件 ==========
    EventTypePaymentStart     EventType = "payment.start"     // 支付开始
    EventTypePaymentSuccess   EventType = "payment.success"   // 支付成功
    EventTypePaymentFail      EventType = "payment.fail"      // 支付失败
    EventTypePaymentRefund    EventType = "payment.refund"    // 退款
    EventTypeCurrencyConsume  EventType = "currency.consume"  // 货币消耗
    EventTypeCurrencyEarn     EventType = "currency.earn"     // 货币获得
    EventTypeItemConsume      EventType = "item.consume"      // 物品消耗
    EventTypeItemEarn         EventType = "item.earn"         // 物品获得

    // ========== 游戏行为 ==========
    EventTypeQuestAccept      EventType = "quest.accept"      // 接取任务
    EventTypeQuestComplete    EventType = "quest.complete"    // 完成任务
    EventTypeQuestFail        EventType = "quest.fail"        // 任务失败
    EventTypeLevelUp          EventType = "player.level_up"   // 升级
    EventTypeAchievementUnlock EventType = "achievement.unlock" // 成就解锁
    EventTypeBossKill         EventType = "boss.kill"         // 击杀 BOSS
    EventTypePVPBattle        EventType = "pvp.battle"        // PVP 战斗
    EventTypePVPWin           EventType = "pvp.win"           // PVP 胜利
    EventTypePVPLoss          EventType = "pvp.loss"          // PVP 失败

    // ========== 社交事件 ==========
    EventTypeGuildJoin        EventType = "guild.join"        // 加入公会
    EventTypeGuildLeave       EventType = "guild.leave"       // 离开公会
    EventTypeFriendAdd        EventType = "friend.add"        // 添加好友
    EventTypeChatSend         EventType = "chat.send"         // 发送聊天
    EventTypeGiftSend         EventType = "gift.send"         // 发送礼物

    // ========== 系统事件 ==========
    EventTypeSystemMaintenance EventType = "system.maintenance" // 系统维护
    EventTypeSystemAnnouncement EventType = "system.announcement" // 系统公告
)

// EventPriority 事件优先级
type EventPriority string

const (
    PriorityLow      EventPriority = "low"
    PriorityNormal   EventPriority = "normal"
    PriorityHigh     EventPriority = "high"
    PriorityCritical EventPriority = "critical"
)

// GetPriority 获取事件优先级
func (et EventType) GetPriority() EventPriority {
    switch et {
    case EventTypePaymentSuccess, EventTypePaymentRefund:
        return PriorityCritical
    case EventTypePaymentStart, EventTypePaymentFail:
        return PriorityHigh
    case EventTypePlayerLogin, EventTypePlayerRegister:
        return PriorityHigh
    default:
        return PriorityNormal
    }
}
```

### 4. Worker 订阅者基类

```go
// internal/event/worker/worker.go

package worker

import (
    "context"
    "encoding/json"
    "log/slog"
    "time"

    redis "github.com/redis/go-redis/v9"
)

// Event 事件结构
type Event map[string]any

// WorkerConfig Worker 配置
type WorkerConfig struct {
    // Redis 配置
    RedisURL        string
    StreamEvents    string
    StreamHighPrio  string
    StreamDLQ       string

    // 消费者组配置
    ConsumerGroup   string
    ConsumerName    string

    // 批处理配置
    BatchSize       int
    BlockTime       time.Duration
    FlushInterval   time.Duration

    // 重试配置
    MaxRetries      int
    RetryInterval   time.Duration
}

// EventHandler 事件处理器接口
type EventHandler interface {
    // GetEventTypes 返回要处理的事件类型，空表示处理所有
    GetEventTypes() []string

    // Handle 处理单个事件
    Handle(ctx context.Context, event Event) error

    // HandleBatch 批量处理事件 (可选，优化性能)
    HandleBatch(ctx context.Context, events []Event) error

    // GetName 获取处理器名称
    GetName() string
}

// Worker 事件处理 Worker
type Worker struct {
    config   WorkerConfig
    client   *redis.Client
    handlers []EventHandler
    ctx      context.Context
    cancel   context.CancelFunc
}

// NewWorker 创建新 Worker
func NewWorker(config WorkerConfig) *Worker {
    return &Worker{
        config:   config,
        handlers: make([]EventHandler, 0),
    }
}

// RegisterHandler 注册事件处理器
func (w *Worker) RegisterHandler(handler EventHandler) {
    w.handlers = append(w.handlers, handler)
}

// Start 启动 Worker
func (w *Worker) Start(ctx context.Context) error {
    w.ctx, w.cancel = context.WithCancel(ctx)

    // 初始化 Redis
    opt, err := redis.ParseURL(w.config.RedisURL)
    if err != nil {
        return err
    }
    w.client = redis.NewClient(opt)

    // 确保消费组存在
    w.ensureConsumerGroup()

    // 启动处理循环
    go w.processLoop()
    go w.flushLoop()

    // 启动死信处理
    go w.processDLQ()

    return nil
}

// Stop 停止 Worker
func (w *Worker) Stop() {
    if w.cancel != nil {
        w.cancel()
    }
    if w.client != nil {
        w.client.Close()
    }
}

// ensureConsumerGroup 确保消费组存在
func (w *Worker) ensureConsumerGroup() {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    // 创建主消费组
    w.client.XGroupCreateMkStream(ctx, w.config.StreamEvents, w.config.ConsumerGroup, "$")
    w.client.XGroupCreateMkStream(ctx, w.config.StreamHighPrio, w.config.ConsumerGroup, "$")
}

// processLoop 主处理循环
func (w *Worker) processLoop() {
    for {
        select {
        case <-w.ctx.Done():
            return
        default:
            w.processMessages()
        }
    }
}

// processMessages 处理消息
func (w *Worker) processMessages() {
    // 优先处理高优先级队列
    w.processStream(w.config.StreamHighPrio)
    // 然后处理普通队列
    w.processStream(w.config.StreamEvents)
}

// processStream 处理单个 Stream
func (w *Worker) processStream(stream string) {
    ctx, cancel := context.WithTimeout(w.ctx, w.config.BlockTime)
    defer cancel()

    // 读取消息
    result, err := w.client.XReadGroup(ctx, &redis.XReadGroupArgs{
        Group:    w.config.ConsumerGroup,
        Consumer: w.config.ConsumerName,
        Streams:  []string{stream, ">"},
        Count:    int64(w.config.BatchSize),
    }).Result()

    if err != nil || len(result) == 0 {
        return
    }

    for _, stream := range result {
        for _, msg := range stream.Messages {
            w.handleMessage(ctx, stream.Stream, msg)
        }
    }
}

// handleMessage 处理单条消息
func (w *Worker) handleMessage(ctx context.Context, stream string, msg redis.XMessage) error {
    // 解析事件
    var event Event
    data, ok := msg.Values["data"]
    if !ok {
        return w.ack(ctx, stream, msg.ID)
    }

    if err := json.Unmarshal([]byte(data.(string)), &event); err != nil {
        w.sendToDLQ(stream, msg, "invalid_json", err.Error())
        return w.ack(ctx, stream, msg.ID)
    }

    // 分发给处理器
    var handlerErr error
    for _, handler := range w.handlers {
        if w.shouldHandle(handler, event) {
            if err := handler.Handle(ctx, event); err != nil {
                slog.Warn("handler failed",
                    "handler", handler.GetName(),
                    "event_id", event["event_id"],
                    "error", err)
                handlerErr = err
            }
        }
    }

    // 处理失败，重试
    if handlerErr != nil {
        retryCount, _ := event["retry_count"].(float64)
        if int(retryCount) >= w.config.MaxRetries {
            w.sendToDLQ(stream, msg, "max_retries", handlerErr.Error())
        } else {
            event["retry_count"] = int(retryCount) + 1
            w.requeue(stream, event)
        }
    }

    return w.ack(ctx, stream, msg.ID)
}

// shouldHandle 判断处理器是否应该处理此事件
func (w *Worker) shouldHandle(handler EventHandler, event Event) bool {
    eventTypes := handler.GetEventTypes()
    if len(eventTypes) == 0 {
        return true // 处理所有事件
    }
    eventType, _ := event["event_type"].(string)
    for _, et := range eventTypes {
        if et == eventType {
            return true
        }
    }
    return false
}

// ack 确认消息
func (w *Worker) ack(ctx context.Context, stream, id string) error {
    return w.client.XAck(ctx, stream, w.config.ConsumerGroup, id).Err()
}

// requeue 重新入队
func (w *Worker) requeue(stream string, event Event) {
    data, _ := json.Marshal(event)
    w.client.XAdd(context.Background(), &redis.XAddArgs{
        Stream: stream,
        Values: map[string]interface{}{"data": string(data)},
    })
}

// sendToDLQ 发送到死信队列
func (w *Worker) sendToDLQ(stream string, msg redis.XMessage, reason, details string) {
    deadEntry := map[string]interface{}{
        "original_stream": stream,
        "original_id":     msg.ID,
        "reason":          reason,
        "details":         details,
        "failed_at":       time.Now().Unix(),
        "original_data":   msg.Values["data"],
    }
    w.client.XAdd(context.Background(), &redis.XAddArgs{
        Stream: w.config.StreamDLQ,
        Values: deadEntry,
    })
}

// processDLQ 处理死信队列 (后台任务)
func (w *Worker) processDLQ() {
    ticker := time.NewTicker(5 * time.Minute)
    defer ticker.Stop()

    for {
        select {
        case <-w.ctx.Done():
            return
        case <-ticker.C:
            // 扫描死信队列，可以进行告警或人工处理
            slog.Info("checking DLQ", "stream", w.config.StreamDLQ)
        }
    }
}

// flushLoop 定期刷新 (用于批处理优化)
func (w *Worker) flushLoop() {
    ticker := time.NewTicker(w.config.FlushInterval)
    defer ticker.Stop()

    for {
        select {
        case <-w.ctx.Done():
            return
        case <-ticker.C:
            for _, handler := range w.handlers {
                if flusher, ok := handler.(Flusher); ok {
                    flusher.Flush(context.Background())
                }
            }
        }
    }
}

// Flusher 刷新器接口 (可选)
type Flusher interface {
    Flush(ctx context.Context) error
}
```

### 5. Analytics Worker (数据分析订阅者)

```go
// cmd/analytics-worker/main.go

package main

import (
    "context"
    "os"

    "github.com/cuihairu/croupier/internal/event/worker"
    "github.com/cuihairu/croupier/internal/event/handlers/analytics"
)

func main() {
    config := worker.WorkerConfig{
        RedisURL:        getEnv("REDIS_URL", "redis://localhost:6379/0"),
        StreamEvents:    "events",
        StreamHighPrio:  "events:high_priority",
        StreamDLQ:       "events:dlq",
        ConsumerGroup:   "analytics-group",
        ConsumerName:    "analytics-worker",
        BatchSize:       100,
        BlockTime:       2 * time.Second,
        FlushInterval:   15 * time.Second,
        MaxRetries:      3,
    }

    w := worker.NewWorker(config)

    // 注册分析处理器
    w.RegisterHandler(analytics.NewClickHouseHandler())
    w.RegisterHandler(analytics.NewAggregationHandler())

    ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
    defer cancel()

    if err := w.Start(ctx); err != nil {
        slog.Error("start worker", "err", err)
        os.Exit(1)
    }

    <-ctx.Done()
    w.Stop()
}
```

### 6. Campaign Worker (活动系统订阅者)

```go
// cmd/campaign-worker/main.go

package main

import (
    "context"
    "os"

    "github.com/cuihairu/croupier/internal/event/worker"
    "github.com/cuihairu/croupier/internal/event/handlers/campaign"
)

func main() {
    config := worker.WorkerConfig{
        RedisURL:        getEnv("REDIS_URL", "redis://localhost:6379/0"),
        StreamEvents:    "events",
        StreamHighPrio:  "events:high_priority",
        StreamDLQ:       "events:dlq",
        ConsumerGroup:   "campaign-group",  // 不同的消费组
        ConsumerName:    "campaign-worker",
        BatchSize:       50,
        BlockTime:       1 * time.Second,
        FlushInterval:   5 * time.Second,
        MaxRetries:      5,  // 活动系统更多重试
    }

    w := worker.NewWorker(config)

    // 注册活动处理器
    w.RegisterHandler(campaign.NewTriggerMatcher())      // 触发器匹配
    w.RegisterHandler(campaign.NewConditionEvaluator())  // 条件评估
    w.RegisterHandler(campaign.NewActionExecutor())      // 动作执行

    ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
    defer cancel()

    if err := w.Start(ctx); err != nil {
        slog.Error("start worker", "err", err)
        os.Exit(1)
    }

    <-ctx.Done()
    w.Stop()
}
```

## 目录结构

```
server/
├── cmd/
│   ├── event-gateway/          # 事件网关服务 (新增)
│   │   └── main.go
│   ├── analytics-worker/       # 数据分析 Worker (已有)
│   ├── campaign-worker/        # 活动 Worker (新增)
│   └── server/                 # 主服务器
│
├── internal/
│   └── event/
│       ├── gateway/            # 事件网关实现
│       │   ├── collector.go    # HTTP/gRPC 收集器
│       │   ├── validator.go    # Schema 验证
│       │   ├── enricher.go     # 事件丰富化
│       │   └── publisher.go    # 发布到 MQ
│       │
│       ├── types/              # 事件类型定义
│       │   ├── event_type.go
│       │   └── schema.go
│       │
│       ├── worker/             # Worker 基础框架
│       │   └── worker.go
│       │
│       └── handlers/           # 事件处理器
│           ├── analytics/      # 数据分析处理器
│           │   ├── clickhouse.go
│           │   └── aggregation.go
│           │
│           └── campaign/       # 活动处理器
│               ├── trigger.go
│               ├── condition.go
│               └── action.go
│
├── proto/
│   └── event/
│       └── v1/
│           └── event_gateway.proto
│
└── docs/
    ├── event-driven-architecture.md    # 本文档
    ├── event-types.md                  # 事件类型文档
    └── campaign-system.md              # 活动系统文档
```

## 部署架构

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                                  部署视图                                        │
├─────────────────────────────────────────────────────────────────────────────────┤
│                                                                                 │
│  ┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐            │
│  │   Game Server   │    │   Game Server   │    │   Game Server   │            │
│  │   (Region: CN)  │    │   (Region: US)  │    │   (Region: EU)  │            │
│  └────────┬────────┘    └────────┬────────┘    └────────┬────────┘            │
│           │                      │                      │                       │
│           └──────────────────────┼──────────────────────┘                       │
│                                  │                                               │
│                    ┌─────────────▼──────────────┐                               │
│                    │   Event Gateway (LB)       │                               │
│                    │   3 instances               │                               │
│                    └─────────────┬──────────────┘                               │
│                                  │                                               │
│                    ┌─────────────▼──────────────┐                               │
│                    │   Redis Cluster            │                               │
│                    │   (Event Bus)              │                               │
│                    └─────────────┬──────────────┘                               │
│                                  │                                               │
│           ┌──────────────────────┼──────────────────────┐                      │
│           │                      │                      │                       │
│  ┌────────▼─────────┐  ┌────────▼─────────┐  ┌────────▼─────────┐             │
│  │ Analytics Worker │  │ Campaign Worker  │  │ (Future) Worker  │             │
│  │ 3 instances      │  │ 2 instances      │  │                  │             │
│  └──────────────────┘  └──────────────────┘  └──────────────────┘             │
│           │                      │                                              │
│           ▼                      ▼                                              │
│  ┌──────────────────┐  ┌──────────────────┐                                     │
│  │ ClickHouse       │  │ Game DB          │                                     │
│  └──────────────────┘  └──────────────────┘                                     │
│                                                                                 │
└─────────────────────────────────────────────────────────────────────────────────┘
```

## 迁移计划

### 阶段 1: 创建 Event Gateway (不影响现有)
1. 实现 `cmd/event-gateway`
2. 定义事件 Schema
3. 部署独立服务

### 阶段 2: 重构 Analytics Worker
1. 重构现有 `analytics-worker` 使用新 Worker 基类
2. 验证功能不变

### 阶段 3: 实现 Campaign Worker
1. 创建 `cmd/campaign-worker`
2. 实现触发器、条件、动作处理器
3. 联调测试

### 阶段 4: 客户端迁移 (可选)
1. SDK 更新，支持直接上报到 Event Gateway
2. 游戏服务器继续通过 MQ 上报 (向后兼容)

## 配置示例

```yaml
# configs/event-gateway.yaml

server:
  http_port: 8080
  grpc_port: 9090

redis:
  url: "redis://localhost:6379/0"
  stream_events: "events"
  stream_high_priority: "events:high_priority"
  stream_dlq: "events:dlq"

validation:
  enabled: true
  strict_mode: false  # false=忽略未知字段, true=拒绝

enrichment:
  geo_ip_enabled: true
  user_agent_enabled: true

rate_limit:
  enabled: true
  requests_per_second: 10000
  burst: 20000
```

## 监控指标

| 指标 | 说明 |
|------|------|
| `event_gateway_events_total` | 接收事件总数 |
| `event_gateway_events_by_type` | 按类型统计 |
| `event_gateway_publish_duration` | 发布耗时 |
| `worker_processed_total` | Worker 处理总数 |
| `worker_failed_total` | 处理失败数 |
| `worker_dlq_size` | 死信队列大小 |
