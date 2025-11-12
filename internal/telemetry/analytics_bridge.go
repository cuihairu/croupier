package telemetry

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// AnalyticsBridge 将OpenTelemetry事件桥接到现有Analytics系统
type AnalyticsBridge struct {
	redisClient *redis.Client
	logger      *slog.Logger
	gameID      string
	enabled     bool

	// MQ配置
	topicPrefix     string
	retentionHours  int
	batchSize       int
	flushInterval   time.Duration

	// 批量发送缓冲区
	eventBatch   []AnalyticsEvent
	batchChannel chan AnalyticsEvent
}

// AnalyticsEvent 标准化的游戏分析事件
type AnalyticsEvent struct {
	EventType   string                 `json:"event_type"`
	GameID      string                 `json:"game_id"`
	UserID      string                 `json:"user_id"`
	SessionID   string                 `json:"session_id"`
	Platform    string                 `json:"platform"`
	Region      string                 `json:"region"`
	Timestamp   int64                  `json:"timestamp"`
	Attributes  map[string]interface{} `json:"attributes"`
	TraceID     string                 `json:"trace_id,omitempty"`
	SpanID      string                 `json:"span_id,omitempty"`
}

// AnalyticsBridgeConfig Analytics桥接配置
type AnalyticsBridgeConfig struct {
	Enabled         bool          `yaml:"enabled"`
	RedisAddr       string        `yaml:"redis_addr"`
	RedisPassword   string        `yaml:"redis_password"`
	RedisDB         int           `yaml:"redis_db"`
	TopicPrefix     string        `yaml:"topic_prefix"`
	RetentionHours  int           `yaml:"retention_hours"`
	BatchSize       int           `yaml:"batch_size"`
	FlushInterval   time.Duration `yaml:"flush_interval"`
}

// NewAnalyticsBridge 创建Analytics桥接器
func NewAnalyticsBridge(config AnalyticsBridgeConfig, gameID string, logger *slog.Logger) *AnalyticsBridge {
	if !config.Enabled {
		return &AnalyticsBridge{enabled: false}
	}

	// 创建Redis客户端
	rdb := redis.NewClient(&redis.Options{
		Addr:     config.RedisAddr,
		Password: config.RedisPassword,
		DB:       config.RedisDB,
	})

	bridge := &AnalyticsBridge{
		redisClient:   rdb,
		logger:        logger,
		gameID:        gameID,
		enabled:       true,
		topicPrefix:   config.TopicPrefix,
		retentionHours: config.RetentionHours,
		batchSize:     config.BatchSize,
		flushInterval: config.FlushInterval,
		batchChannel:  make(chan AnalyticsEvent, config.BatchSize*2),
	}

	// 启动批量处理协程
	go bridge.batchProcessor()

	return bridge
}

// 启动批量事件处理器
func (b *AnalyticsBridge) batchProcessor() {
	if !b.enabled {
		return
	}

	ticker := time.NewTicker(b.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case event := <-b.batchChannel:
			b.eventBatch = append(b.eventBatch, event)
			if len(b.eventBatch) >= b.batchSize {
				b.flushBatch()
			}

		case <-ticker.C:
			if len(b.eventBatch) > 0 {
				b.flushBatch()
			}
		}
	}
}

// 刷新批量事件到Redis
func (b *AnalyticsBridge) flushBatch() {
	if len(b.eventBatch) == 0 {
		return
	}

	ctx := context.Background()
	pipe := b.redisClient.Pipeline()

	// 按事件类型分组发送
	eventGroups := make(map[string][]AnalyticsEvent)
	for _, event := range b.eventBatch {
		eventGroups[event.EventType] = append(eventGroups[event.EventType], event)
	}

	for eventType, events := range eventGroups {
		topic := fmt.Sprintf("%s:%s", b.topicPrefix, eventType)

		for _, event := range events {
			data, err := json.Marshal(event)
			if err != nil {
				b.logger.Error("Failed to marshal analytics event",
					"error", err, "event_type", eventType)
				continue
			}

			// 发送到Redis Stream
			pipe.XAdd(ctx, &redis.XAddArgs{
				Stream: topic,
				Values: map[string]interface{}{
					"data": string(data),
					"game_id": event.GameID,
					"user_id": event.UserID,
					"timestamp": event.Timestamp,
				},
			})
		}

		// 设置Stream过期时间
		if b.retentionHours > 0 {
			pipe.Expire(ctx, topic, time.Duration(b.retentionHours)*time.Hour)
		}
	}

	// 执行批量操作
	if _, err := pipe.Exec(ctx); err != nil {
		b.logger.Error("Failed to send analytics events to Redis",
			"error", err, "batch_size", len(b.eventBatch))
	} else {
		b.logger.Debug("Analytics events sent to Redis",
			"batch_size", len(b.eventBatch), "event_types", len(eventGroups))
	}

	// 清空批次
	b.eventBatch = b.eventBatch[:0]
}

// SendEvent 发送分析事件
func (b *AnalyticsBridge) SendEvent(ctx context.Context, eventType string, span trace.Span, attrs []attribute.KeyValue) {
	if !b.enabled {
		return
	}

	// 构建标准化事件
	event := AnalyticsEvent{
		EventType:  eventType,
		GameID:     b.gameID,
		Timestamp:  time.Now().UnixMilli(),
		Attributes: make(map[string]interface{}),
	}

	// 提取span上下文
	if span != nil {
		spanCtx := span.SpanContext()
		if spanCtx.HasTraceID() {
			event.TraceID = spanCtx.TraceID().String()
		}
		if spanCtx.HasSpanID() {
			event.SpanID = spanCtx.SpanID().String()
		}
	}

	// 转换属性
	for _, attr := range attrs {
		key := string(attr.Key)
		value := attr.Value.AsInterface()

		switch key {
		case "game.user_id":
			event.UserID = value.(string)
		case "game.session_id":
			event.SessionID = value.(string)
		case "game.platform":
			event.Platform = value.(string)
		case "game.region":
			event.Region = value.(string)
		default:
			event.Attributes[key] = value
		}
	}

	// 异步发送事件
	select {
	case b.batchChannel <- event:
	default:
		b.logger.Warn("Analytics event channel full, dropping event",
			"event_type", eventType, "user_id", event.UserID)
	}
}

// SendSessionEvent 发送会话相关事件
func (b *AnalyticsBridge) SendSessionEvent(ctx context.Context, eventType string, span trace.Span, userID, sessionID, platform, region string, extra map[string]interface{}) {
	attrs := []attribute.KeyValue{
		GameUserIDKey.String(userID),
		GameSessionIDKey.String(sessionID),
		GamePlatformKey.String(platform),
		GameRegionKey.String(region),
	}

	// 添加额外属性
	for k, v := range extra {
		switch value := v.(type) {
		case string:
			attrs = append(attrs, attribute.String(k, value))
		case int:
			attrs = append(attrs, attribute.Int(k, value))
		case int64:
			attrs = append(attrs, attribute.Int64(k, value))
		case float64:
			attrs = append(attrs, attribute.Float64(k, value))
		case bool:
			attrs = append(attrs, attribute.Bool(k, value))
		}
	}

	b.SendEvent(ctx, eventType, span, attrs)
}

// SendProgressionEvent 发送进度相关事件
func (b *AnalyticsBridge) SendProgressionEvent(ctx context.Context, eventType string, span trace.Span, userID, sessionID, levelID string, extra map[string]interface{}) {
	attrs := []attribute.KeyValue{
		GameUserIDKey.String(userID),
		GameSessionIDKey.String(sessionID),
		ProgressionLevelIDKey.String(levelID),
	}

	for k, v := range extra {
		switch value := v.(type) {
		case string:
			attrs = append(attrs, attribute.String(k, value))
		case int:
			attrs = append(attrs, attribute.Int(k, value))
		case int64:
			attrs = append(attrs, attribute.Int64(k, value))
		case float64:
			attrs = append(attrs, attribute.Float64(k, value))
		case bool:
			attrs = append(attrs, attribute.Bool(k, value))
		}
	}

	b.SendEvent(ctx, eventType, span, attrs)
}

// SendEconomyEvent 发送经济相关事件
func (b *AnalyticsBridge) SendEconomyEvent(ctx context.Context, eventType string, span trace.Span, userID, currency string, amount float64, extra map[string]interface{}) {
	attrs := []attribute.KeyValue{
		GameUserIDKey.String(userID),
		EconomyCurrencyKey.String(currency),
		EconomyAmountKey.Float64(amount),
	}

	for k, v := range extra {
		switch value := v.(type) {
		case string:
			attrs = append(attrs, attribute.String(k, value))
		case int:
			attrs = append(attrs, attribute.Int(k, value))
		case int64:
			attrs = append(attrs, attribute.Int64(k, value))
		case float64:
			attrs = append(attrs, attribute.Float64(k, value))
		case bool:
			attrs = append(attrs, attribute.Bool(k, value))
		}
	}

	b.SendEvent(ctx, eventType, span, attrs)
}

// Shutdown 优雅关闭
func (b *AnalyticsBridge) Shutdown(ctx context.Context) error {
	if !b.enabled {
		return nil
	}

	// 刷新剩余事件
	b.flushBatch()

	// 关闭Redis连接
	if b.redisClient != nil {
		return b.redisClient.Close()
	}

	return nil
}

// Health 健康检查
func (b *AnalyticsBridge) Health(ctx context.Context) error {
	if !b.enabled {
		return nil
	}

	return b.redisClient.Ping(ctx).Err()
}