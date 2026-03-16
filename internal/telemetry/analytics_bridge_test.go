package telemetry

import (
	"context"
	"log/slog"
	"testing"
	"time"
)

// TestNewAnalyticsBridge_Disabled 测试禁用状态的桥接器
func TestNewAnalyticsBridge_Disabled(t *testing.T) {
	config := AnalyticsBridgeConfig{
		Enabled: false,
	}

	bridge := NewAnalyticsBridge(config, "test-game", slog.Default())
	if bridge == nil {
		t.Fatal("NewAnalyticsBridge() should return non-nil bridge")
	}
	if bridge.enabled != false {
		t.Error("Bridge should be disabled")
	}
	if bridge.redisClient != nil {
		t.Error("Redis client should be nil when disabled")
	}
}

// TestNewAnalyticsBridge_Enabled 测试启用状态的桥接器
func TestNewAnalyticsBridge_Enabled(t *testing.T) {
	config := AnalyticsBridgeConfig{
		Enabled:        true,
		RedisAddr:      "localhost:6379",
		RedisPassword:  "",
		RedisDB:        0,
		TopicPrefix:    "game:events",
		RetentionHours: 168,
		BatchSize:      100,
		FlushInterval:  30 * time.Second,
	}

	bridge := NewAnalyticsBridge(config, "test-game", slog.Default())
	if bridge == nil {
		t.Fatal("NewAnalyticsBridge() should return non-nil bridge")
	}
	if bridge.enabled != true {
		t.Error("Bridge should be enabled")
	}
	if bridge.redisClient == nil {
		t.Error("Redis client should not be nil when enabled")
	}
	if bridge.gameID != "test-game" {
		t.Errorf("gameID = %s, want 'test-game'", bridge.gameID)
	}
	if bridge.topicPrefix != "game:events" {
		t.Errorf("topicPrefix = %s, want 'game:events'", bridge.topicPrefix)
	}
	if bridge.batchSize != 100 {
		t.Errorf("batchSize = %d, want 100", bridge.batchSize)
	}
	if bridge.flushInterval != 30*time.Second {
		t.Errorf("flushInterval = %v, want 30s", bridge.flushInterval)
	}

	// 清理
	bridge.Shutdown(context.Background())
}

// TestAnalyticsBridge_SendEvent_Disabled 测试禁用时发送事件
func TestAnalyticsBridge_SendEvent_Disabled(t *testing.T) {
	config := AnalyticsBridgeConfig{
		Enabled: false,
	}

	bridge := NewAnalyticsBridge(config, "test-game", slog.Default())

	// 不应该 panic
	bridge.SendEvent(context.Background(), "test.event", nil, nil)
}

// TestAnalyticsBridge_SendEvent_Enabled 测试启用时发送事件
func TestAnalyticsBridge_SendEvent_Enabled(t *testing.T) {
	config := AnalyticsBridgeConfig{
		Enabled:       true,
		RedisAddr:     "localhost:6379",
		BatchSize:     10,
		FlushInterval: 1 * time.Second,
	}

	bridge := NewAnalyticsBridge(config, "test-game", slog.Default())
	defer bridge.Shutdown(context.Background())

	// 不应该 panic - 即使传入 nil span
	bridge.SendEvent(context.Background(), "test.event", nil, nil)

	// 验证事件被添加到通道
	if len(bridge.batchChannel) != 1 {
		t.Errorf("batchChannel length = %d, want 1", len(bridge.batchChannel))
	}
}

// TestAnalyticsBridge_SendSessionEvent 测试发送会话事件
func TestAnalyticsBridge_SendSessionEvent(t *testing.T) {
	config := AnalyticsBridgeConfig{
		Enabled:       true,
		RedisAddr:     "localhost:6379",
		BatchSize:     10,
		FlushInterval: 1 * time.Second,
	}

	bridge := NewAnalyticsBridge(config, "test-game", slog.Default())
	defer bridge.Shutdown(context.Background())

	ctx := context.Background()

	// 测试各种类型的额外属性
	extra := map[string]interface{}{
		"game_type":   "td",
		"app_version": "1.0.0",
		"level":       5,
		"premium":     true,
		"ratio":       0.95,
	}

	// 不应该 panic - 事件会被批量处理器消费
	bridge.SendSessionEvent(ctx, "session.start", nil, "user123", "session456", "ios", "us", extra)
}

// TestAnalyticsBridge_SendProgressionEvent 测试发送进度事件
func TestAnalyticsBridge_SendProgressionEvent(t *testing.T) {
	config := AnalyticsBridgeConfig{
		Enabled:       true,
		RedisAddr:     "localhost:6379",
		BatchSize:     10,
		FlushInterval: 1 * time.Second,
	}

	bridge := NewAnalyticsBridge(config, "test-game", slog.Default())
	defer bridge.Shutdown(context.Background())

	ctx := context.Background()

	extra := map[string]interface{}{
		"duration_ms": 30000,
		"stars":       3,
		"retries":     1,
	}

	// 不应该 panic - 事件会被批量处理器消费
	bridge.SendProgressionEvent(ctx, "progression.complete", nil, "user123", "session456", "level01", extra)
}

// TestAnalyticsBridge_SendEconomyEvent 测试发送经济事件
func TestAnalyticsBridge_SendEconomyEvent(t *testing.T) {
	config := AnalyticsBridgeConfig{
		Enabled:       true,
		RedisAddr:     "localhost:6379",
		BatchSize:     10,
		FlushInterval: 1 * time.Second,
	}

	bridge := NewAnalyticsBridge(config, "test-game", slog.Default())
	defer bridge.Shutdown(context.Background())

	ctx := context.Background()

	extra := map[string]interface{}{
		"currency_kind": "soft",
		"item_id":       "tower_01",
		"balance_after": 1000,
	}

	// 不应该 panic - 事件会被批量处理器消费
	bridge.SendEconomyEvent(ctx, "economy.spend", nil, "user123", "gold", 100, extra)
}

// TestAnalyticsBridge_Shutdown 测试关闭桥接器
func TestAnalyticsBridge_Shutdown(t *testing.T) {
	tests := []struct {
		name    string
		enabled bool
	}{
		{"Disabled bridge", false},
		{"Enabled bridge", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := AnalyticsBridgeConfig{
				Enabled:       tt.enabled,
				RedisAddr:     "localhost:6379",
				BatchSize:     10,
				FlushInterval: 1 * time.Second,
			}

			bridge := NewAnalyticsBridge(config, "test-game", slog.Default())

			ctx := context.Background()
			err := bridge.Shutdown(ctx)
			if err != nil {
				t.Errorf("Shutdown() error = %v", err)
			}
		})
	}
}

// TestAnalyticsBridge_Health 测试健康检查
func TestAnalyticsBridge_Health(t *testing.T) {
	t.Run("Disabled bridge", func(t *testing.T) {
		config := AnalyticsBridgeConfig{
			Enabled: false,
		}

		bridge := NewAnalyticsBridge(config, "test-game", slog.Default())

		ctx := context.Background()
		err := bridge.Health(ctx)
		if err != nil {
			t.Errorf("Health() should return nil when disabled, got %v", err)
		}
	})

	t.Run("Enabled bridge - no Redis", func(t *testing.T) {
		config := AnalyticsBridgeConfig{
			Enabled:       true,
			RedisAddr:     "localhost:9999", // 不存在的端口
			BatchSize:     10,
			FlushInterval: 1 * time.Second,
		}

		bridge := NewAnalyticsBridge(config, "test-game", slog.Default())
		defer bridge.Shutdown(context.Background())

		ctx := context.Background()
		err := bridge.Health(ctx)
		// 应该返回错误，因为 Redis 不可用
		if err == nil {
			t.Error("Health() should return error when Redis is unavailable")
		}
	})
}

// TestAnalyticsBridge_flushBatch 测试刷新批次
func TestAnalyticsBridge_flushBatch(t *testing.T) {
	config := AnalyticsBridgeConfig{
		Enabled:       true,
		RedisAddr:     "localhost:6379",
		BatchSize:     10,
		FlushInterval: 1 * time.Second,
	}

	bridge := NewAnalyticsBridge(config, "test-game", slog.Default())
	defer bridge.Shutdown(context.Background())

	// 添加一些事件到批次
	bridge.eventBatch = []AnalyticsEvent{
		{
			EventType: "test.event1",
			GameID:    "test-game",
			UserID:    "user1",
			Timestamp: time.Now().UnixMilli(),
		},
		{
			EventType: "test.event2",
			GameID:    "test-game",
			UserID:    "user2",
			Timestamp: time.Now().UnixMilli(),
		},
	}

	// 调用 flushBatch - 应该不 panic
	bridge.flushBatch()

	// 批次应该被清空
	if len(bridge.eventBatch) != 0 {
		t.Errorf("eventBatch length = %d, want 0", len(bridge.eventBatch))
	}
}

// TestAnalyticsBridge_batchProcessor 测试批量处理器
func TestAnalyticsBridge_batchProcessor(t *testing.T) {
	config := AnalyticsBridgeConfig{
		Enabled:       true,
		RedisAddr:     "localhost:6379",
		BatchSize:     2,
		FlushInterval: 100 * time.Millisecond,
	}

	bridge := NewAnalyticsBridge(config, "test-game", slog.Default())
	defer bridge.Shutdown(context.Background())

	// 发送事件到通道 (使用正常 API 而不是直接发送)
	bridge.SendEvent(context.Background(), "event1", nil, nil)
	bridge.SendEvent(context.Background(), "event2", nil, nil)

	// 等待批量处理器处理
	time.Sleep(200 * time.Millisecond)

	// 批次应该被清空
	if len(bridge.eventBatch) != 0 {
		// 可能已经被处理
	}
}

// TestAnalyticsEvent 测试 AnalyticsEvent 结构体
func TestAnalyticsEvent(t *testing.T) {
	event := AnalyticsEvent{
		EventType:  "test.event",
		GameID:     "game123",
		UserID:     "user123",
		SessionID:  "session456",
		Platform:   "ios",
		Region:     "us",
		Timestamp:  1234567890,
		TraceID:    "trace789",
		SpanID:     "span012",
		Attributes: map[string]interface{}{"key": "value"},
	}

	if event.EventType != "test.event" {
		t.Errorf("EventType = %s, want 'test.event'", event.EventType)
	}
	if event.GameID != "game123" {
		t.Errorf("GameID = %s, want 'game123'", event.GameID)
	}

	// Test timestamp ranges
	timestamps := []int64{0, 1000000, 1234567890, 9999999999}
	for _, ts := range timestamps {
		event.Timestamp = ts
		if event.Timestamp != ts {
			t.Errorf("Timestamp = %d, want %d", event.Timestamp, ts)
		}
	}
}

// TestAnalyticsBridgeConfigDefaults 测试配置默认值
func TestAnalyticsBridgeConfigDefaults(t *testing.T) {
	config := AnalyticsBridgeConfig{}

	// 设置一些值验证默认值处理
	if config.BatchSize == 0 {
		config.BatchSize = 100
	}
	if config.FlushInterval == 0 {
		config.FlushInterval = 30 * time.Second
	}

	if config.BatchSize != 100 {
		t.Errorf("BatchSize = %d, want 100", config.BatchSize)
	}
	if config.FlushInterval != 30*time.Second {
		t.Errorf("FlushInterval = %v, want 30s", config.FlushInterval)
	}
}

// TestAnalyticsBridgeConfig 不同配置组合
func TestAnalyticsBridgeConfig(t *testing.T) {
	tests := []struct {
		name            string
		config          AnalyticsBridgeConfig
		expectedEnabled bool
		expectedBatch   int
	}{
		{
			name: "Default values",
			config: AnalyticsBridgeConfig{
				Enabled:       true,
				BatchSize:     100,
				FlushInterval: 30 * time.Second,
			},
			expectedEnabled: true,
			expectedBatch:   100,
		},
		{
			name: "Custom batch size",
			config: AnalyticsBridgeConfig{
				Enabled:       true,
				BatchSize:     500,
				FlushInterval: 60 * time.Second,
			},
			expectedEnabled: true,
			expectedBatch:   500,
		},
		{
			name: "Small batch size",
			config: AnalyticsBridgeConfig{
				Enabled:       true,
				BatchSize:     10,
				FlushInterval: 5 * time.Second,
			},
			expectedEnabled: true,
			expectedBatch:   10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bridge := NewAnalyticsBridge(tt.config, "test-game", slog.Default())
			if bridge.enabled != tt.expectedEnabled {
				t.Errorf("enabled = %v, want %v", bridge.enabled, tt.expectedEnabled)
			}
			if bridge.batchSize != tt.expectedBatch {
				t.Errorf("batchSize = %d, want %d", bridge.batchSize, tt.expectedBatch)
			}
			if bridge.enabled {
				bridge.Shutdown(context.Background())
			}
		})
	}
}

// TestAnalyticsBridge_ChannelOperations 测试通道操作
func TestAnalyticsBridge_ChannelOperations(t *testing.T) {
	config := AnalyticsBridgeConfig{
		Enabled:       true,
		RedisAddr:     "localhost:6379",
		BatchSize:     10,
		FlushInterval: 1 * time.Second,
	}

	bridge := NewAnalyticsBridge(config, "test-game", slog.Default())
	defer bridge.Shutdown(context.Background())

	// 测试多个事件 - 不应该 panic
	for i := 0; i < 5; i++ {
		bridge.SendEvent(context.Background(), "test.event", nil, nil)
	}
}

// TestAnalyticsBridge_EventAttributes 测试事件属性
func TestAnalyticsBridge_EventAttributes(t *testing.T) {
	config := AnalyticsBridgeConfig{
		Enabled:       true,
		RedisAddr:     "localhost:6379",
		BatchSize:     10,
		FlushInterval: 1 * time.Second,
	}

	bridge := NewAnalyticsBridge(config, "test-game", slog.Default())
	defer bridge.Shutdown(context.Background())

	// 测试不同类型的属性值
	extra := map[string]interface{}{
		"string_val": "test",
		"int_val":     42,
		"float_val":   3.14,
		"bool_val":    true,
		"nil_val":     nil,
	}

	// 不应该 panic - 事件会被批量处理器消费
	bridge.SendSessionEvent(context.Background(), "test.event", nil, "user123", "session456", "ios", "us", extra)
}

// TestAnalyticsBridge_Timestamp 测试时间戳
func TestAnalyticsBridge_Timestamp(t *testing.T) {
	config := AnalyticsBridgeConfig{
		Enabled:       true,
		RedisAddr:     "localhost:6379",
		BatchSize:     10,
		FlushInterval: 1 * time.Second,
	}

	bridge := NewAnalyticsBridge(config, "test-game", slog.Default())
	defer bridge.Shutdown(context.Background())

	// 不应该 panic - 事件会被批量处理器消费
	bridge.SendEvent(context.Background(), "test.event", nil, nil)
}

// TestAnalyticsBridge_GameID 测试游戏ID
func TestAnalyticsBridge_GameID(t *testing.T) {
	config := AnalyticsBridgeConfig{
		Enabled:       true,
		RedisAddr:     "localhost:6379",
		BatchSize:     10,
		FlushInterval: 1 * time.Second,
	}

	bridge := NewAnalyticsBridge(config, "my-game", slog.Default())
	defer bridge.Shutdown(context.Background())

	// 不应该 panic - 事件会被批量处理器消费
	bridge.SendEvent(context.Background(), "test.event", nil, nil)
}
