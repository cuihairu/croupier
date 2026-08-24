package telemetry

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
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
	topicPrefix    string
	retentionHours int
	batchSize      int
	flushInterval  time.Duration

	// 批量发送缓冲区
	eventBatch   []AnalyticsEvent
	batchChannel chan AnalyticsEvent

	// stopCh 结束 batchProcessor；此前该 goroutine 无退出机制（泄漏），
	// 且 Shutdown 在调用方 goroutine 直接 flushBatch，与 processor 并发
	// 读写 eventBatch（-race 必报）。
	stopCh          chan struct{}
	processorExited chan struct{}
}

// AnalyticsEvent 标准化的游戏分析事件
type AnalyticsEvent struct {
	EventType  string                 `json:"eventType"`
	GameID     string                 `json:"gameId"`
	UserID     string                 `json:"userId"`
	SessionID  string                 `json:"sessionId"`
	Platform   string                 `json:"platform"`
	Region     string                 `json:"region"`
	Timestamp  int64                  `json:"timestamp"`
	Attributes map[string]interface{} `json:"attributes"`
	TraceID    string                 `json:"traceId,omitempty"`
	SpanID     string                 `json:"spanId,omitempty"`
}

// analyticsEventsStream is the single stream consumed by cmd/analytics-worker.
// The previous per-event-type streams (game:events:<type>) had no consumer.
const analyticsEventsStream = "analytics:events"

// workerPayloadFromEvent converts a bridge AnalyticsEvent into the snake_case
// envelope the analytics worker parses (worker.asString keys). ts uses
// RFC3339 which both worker.parseEventTime (aggregation) and
// worker.normalizeTimestamp (detail insert) accept.
func workerPayloadFromEvent(e AnalyticsEvent) map[string]interface{} {
	ts := time.Unix(e.Timestamp, 0).UTC().Format(time.RFC3339)
	payload := map[string]interface{}{
		"event":      e.EventType,
		"game_id":    e.GameID,
		"user_id":    e.UserID,
		"session_id": e.SessionID,
		"platform":   e.Platform,
		"ts":         ts,
	}
	props := map[string]interface{}{}
	for k, v := range e.Attributes {
		props[k] = v
	}
	if e.TraceID != "" {
		props["trace_id"] = e.TraceID
	}
	if e.SpanID != "" {
		props["span_id"] = e.SpanID
	}
	if e.Region != "" {
		payload["country"] = normalizeCountry(e.Region)
	}
	if len(props) > 0 {
		payload["props"] = props
	}
	return payload
}

// normalizeCountry coerces region/country hints into the ISO-2 form the
// ClickHouse FixedString(2) country column expects; unknown values are
// dropped rather than written as an invalid fixed-width string.
func normalizeCountry(region string) string {
	trimmed := strings.TrimSpace(region)
	if len(trimmed) == 2 {
		return strings.ToUpper(trimmed)
	}
	// Common full names seen from platform hints
	switch strings.ToLower(trimmed) {
	case "cn", "china":
		return "CN"
	case "us", "usa", "united states":
		return "US"
	case "jp", "japan":
		return "JP"
	case "kr", "korea":
		return "KR"
	case "de", "germany":
		return "DE"
	case "gb", "uk", "united kingdom":
		return "GB"
	}
	return ""
}

// AnalyticsBridgeConfig Analytics桥接配置
type AnalyticsBridgeConfig struct {
	Enabled        bool          `yaml:"enabled"`
	RedisAddr      string        `yaml:"redis_addr"`
	RedisPassword  string        `yaml:"redis_password"`
	RedisDB        int           `yaml:"redis_db"`
	TopicPrefix    string        `yaml:"topic_prefix"`
	RetentionHours int           `yaml:"retention_hours"`
	BatchSize      int           `yaml:"batch_size"`
	FlushInterval  time.Duration `yaml:"flush_interval"`
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

	if logger == nil {
		logger = slog.Default()
	}

	bridge := &AnalyticsBridge{
		redisClient:     rdb,
		logger:          logger,
		gameID:          gameID,
		enabled:         true,
		topicPrefix:     config.TopicPrefix,
		retentionHours:  config.RetentionHours,
		batchSize:       config.BatchSize,
		flushInterval:   config.FlushInterval,
		batchChannel:    make(chan AnalyticsEvent, config.BatchSize*2),
		stopCh:          make(chan struct{}),
		processorExited: make(chan struct{}),
	}

	// 启动批量处理协程
	go func() {
		defer close(bridge.processorExited)
		bridge.batchProcessor()
	}()

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

		case <-b.stopCh:
			// 退出前清空缓冲与批次，事件不丢
			for {
				select {
				case event := <-b.batchChannel:
					b.eventBatch = append(b.eventBatch, event)
				default:
					b.flushBatch()
					return
				}
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

	// All bridge events are behavioral: they flow into the single stream the
	// analytics worker consumes (analytics:events). The worker normalizes
	// dotted OTel event names (session.start etc.) via its alias table, so
	// the bridge keeps OTel semantic naming untouched.
	for _, event := range b.eventBatch {
		data, err := json.Marshal(workerPayloadFromEvent(event))
		if err != nil {
			b.logger.Error("Failed to marshal analytics event",
				"error", err, "event_type", event.EventType)
			continue
		}

		pipe.XAdd(ctx, &redis.XAddArgs{
			Stream: analyticsEventsStream,
			Values: map[string]interface{}{
				"data": string(data),
			},
		})
	}

	// The consumer (cmd/analytics-worker) owns stream trimming via
	// ANALYTICS_REDIS_MAXLEN on its publisher side; retention here only
	// guards against an idle worker leaving the stream to grow.
	if b.retentionHours > 0 {
		pipe.Expire(ctx, analyticsEventsStream, time.Duration(b.retentionHours)*time.Hour)
	}

	// 执行批量操作
	if _, err := pipe.Exec(ctx); err != nil {
		b.logger.Error("Failed to send analytics events to Redis",
			"error", err, "batch_size", len(b.eventBatch))
	} else {
		b.logger.Debug("Analytics events sent to Redis",
			"stream", analyticsEventsStream, "batch_size", len(b.eventBatch))
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

// Shutdown 优雅关闭：停止批量协程（其退出路径负责最终 flush），再关 Redis。
// 此前直接在调用方 goroutine flush，与处理器并发读写 eventBatch（数据竞争）。
func (b *AnalyticsBridge) Shutdown(ctx context.Context) error {
	if !b.enabled {
		return nil
	}
	// 手工构造（测试）可能没有配套协程；nil channel 的 select 永远阻塞，
	// 这里对未初始化字段做幂等保护。
	if b.stopCh != nil {
		select {
		case <-b.stopCh:
		default:
			close(b.stopCh)
		}
	}
	if b.processorExited != nil {
		<-b.processorExited
	}
	// processor 退出路径已 flush；手工构造（无协程）场景在此兜底。
	b.flushBatch()

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
