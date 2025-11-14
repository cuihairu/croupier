package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"math/rand"
	"net/http"
	"os"
	"time"

	"github.com/cuihairu/croupier/examples/otel-integration/internal/telemetry"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

func main() {
	// 创建结构化日志器
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	// 加载 OpenTelemetry 配置
	config := telemetry.LoadConfigFromEnv()
	config.ServiceName = "game-client"

	ctx := context.Background()

	// 初始化 OpenTelemetry
	provider, err := telemetry.NewProvider(ctx, config)
	if err != nil {
		log.Fatalf("Failed to initialize OpenTelemetry: %v", err)
	}

	// 确保在程序结束时清理资源
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := provider.Shutdown(shutdownCtx); err != nil {
			logger.Error("Failed to shutdown OpenTelemetry provider", "error", err)
		}
	}()

	// 创建客户端分析器
	clientAnalytics, err := telemetry.NewClientAnalytics()
	if err != nil {
		log.Fatalf("Failed to create client analytics: %v", err)
	}

	// 模拟客户端行为
	client := &GameClient{
		serverURL: "http://localhost:8080",
		logger:    logger,
		tracer:    otel.Tracer("game-client"),
		analytics: clientAnalytics,
		deviceID:  fmt.Sprintf("device_%d", time.Now().Unix()),
	}

	// 模拟应用启动
	client.simulateAppStartup(ctx)

	// 运行游戏会话
	userID := fmt.Sprintf("client_user_%d", time.Now().Unix())
	client.RunGameSession(ctx, userID)
}

// GameClient 游戏客户端
type GameClient struct {
	serverURL string
	logger    *slog.Logger
	tracer    trace.Tracer
	analytics *telemetry.ClientAnalytics
	deviceID  string
}

// simulateAppStartup 模拟应用启动过程
func (c *GameClient) simulateAppStartup(ctx context.Context) {
	ctx, span := c.tracer.Start(ctx, "client.app.startup",
		trace.WithAttributes(
			attribute.String("device.id", c.deviceID),
		),
	)
	defer span.End()

	c.logger.InfoContext(ctx, "应用启动中...", slog.String("device_id", c.deviceID))

	// 记录启动时间
	c.analytics.RecordLoadingMetrics(ctx, "app_start")

	// 模拟一些启动时可能发生的事件
	if rand.Float64() < 0.02 { // 2% 概率启动崩溃
		c.analytics.RecordStabilityEvent(ctx, "crash", "startup_crash")
		c.logger.ErrorContext(ctx, "应用启动崩溃")
		return
	}

	// 初始化性能监控
	go c.startPerformanceMonitoring(ctx)

	c.logger.InfoContext(ctx, "应用启动完成")
}

// startPerformanceMonitoring 启动性能监控
func (c *GameClient) startPerformanceMonitoring(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second) // 每5秒记录一次性能数据
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.analytics.RecordPerformanceMetrics(ctx, c.deviceID)
			c.analytics.RecordNetworkMetrics(ctx, "session_monitoring")

			// 模拟偶尔的稳定性问题
			if rand.Float64() < 0.001 { // 0.1% 概率ANR
				c.analytics.RecordStabilityEvent(ctx, "anr", "main_thread_blocked")
			}
			if rand.Float64() < 0.0005 { // 0.05% 概率卡顿
				c.analytics.RecordStabilityEvent(ctx, "freeze", "frame_drop")
			}
		}
	}
}

// RunGameSession 运行游戏会话
func (c *GameClient) RunGameSession(ctx context.Context, userID string) {
	ctx, span := c.tracer.Start(ctx, "game.client.session",
		trace.WithAttributes(
			attribute.String("client.user_id", userID),
			attribute.String("device.id", c.deviceID),
		),
	)
	defer span.End()

	// 1. 登录（模拟加载）
	c.analytics.RecordLoadingMetrics(ctx, "level") // 登录界面加载
	sessionID, err := c.login(ctx, userID, "ios", "cn-north")
	if err != nil {
		c.logger.ErrorContext(ctx, "登录失败", "error", err)
		return
	}

	c.logger.InfoContext(ctx, "登录成功",
		slog.String("session_id", sessionID),
		slog.String("user_id", userID),
	)

	// 记录用户交互（登录操作）
	c.analytics.RecordUserInteraction(ctx, "login")

	// 2. 模拟游戏玩法
	for i := 1; i <= 5; i++ {
		levelID := fmt.Sprintf("level_%d", i)

		// 关卡加载
		c.analytics.RecordLoadingMetrics(ctx, "level")

		// 开始关卡
		err := c.startLevel(ctx, sessionID, levelID)
		if err != nil {
			c.logger.ErrorContext(ctx, "关卡开始失败",
				"level_id", levelID,
				"error", err,
			)
			continue
		}

		// 记录关卡开始的用户交互
		c.analytics.RecordUserInteraction(ctx, "level_start")

		// 模拟游戏玩法
		c.simulateGameplay(ctx, sessionID, levelID, time.Duration(rand.Intn(30)+10)*time.Second)

		c.logger.InfoContext(ctx, "关卡完成",
			slog.String("level_id", levelID),
		)

		// 可能的资源下载（新关卡内容）
		if rand.Float64() < 0.3 { // 30% 概率需要下载新资源
			c.analytics.RecordLoadingMetrics(ctx, "asset_download")
		}

		// 随机等待
		time.Sleep(time.Duration(rand.Intn(3)+1) * time.Second)
	}

	span.AddEvent("game.session.complete")
	c.logger.InfoContext(ctx, "游戏会话完成", slog.String("user_id", userID))
}

// login 登录
func (c *GameClient) login(ctx context.Context, userID, platform, region string) (string, error) {
	ctx, span := c.tracer.Start(ctx, "client.api.login",
		trace.WithAttributes(
			attribute.String("client.user_id", userID),
			attribute.String("game.platform", platform),
			attribute.String("game.region", region),
		),
	)
	defer span.End()

	url := fmt.Sprintf("%s/api/login?user_id=%s&platform=%s&region=%s",
		c.serverURL, userID, platform, region)

	resp, err := http.Get(url)
	if err != nil {
		span.RecordError(err)
		return "", fmt.Errorf("login request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		span.SetAttributes(attribute.Int("http.status_code", resp.StatusCode))
		return "", fmt.Errorf("login failed with status: %d", resp.StatusCode)
	}

	// 简化：从响应中提取 session_id
	sessionID := fmt.Sprintf("session_%d", time.Now().UnixNano())

	span.SetAttributes(
		attribute.String("session_id", sessionID),
		attribute.Int("http.status_code", resp.StatusCode),
	)

	return sessionID, nil
}

// startLevel 开始关卡
func (c *GameClient) startLevel(ctx context.Context, sessionID, levelID string) error {
	ctx, span := c.tracer.Start(ctx, "client.api.level.start",
		trace.WithAttributes(
			attribute.String("session_id", sessionID),
			attribute.String("level.id", levelID),
		),
	)
	defer span.End()

	url := fmt.Sprintf("%s/api/level/start?session_id=%s&level_id=%s",
		c.serverURL, sessionID, levelID)

	resp, err := http.Get(url)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("start level request failed: %w", err)
	}
	defer resp.Body.Close()

	span.SetAttributes(attribute.Int("http.status_code", resp.StatusCode))

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("start level failed with status: %d", resp.StatusCode)
	}

	return nil
}

// simulateGameplay 模拟游戏玩法
func (c *GameClient) simulateGameplay(ctx context.Context, sessionID, levelID string, duration time.Duration) {
	ctx, span := c.tracer.Start(ctx, "client.gameplay",
		trace.WithAttributes(
			attribute.String("session_id", sessionID),
			attribute.String("level.id", levelID),
			attribute.Int64("gameplay.duration_ms", duration.Milliseconds()),
		),
	)
	defer span.End()

	endTime := time.Now().Add(duration)
	tickCount := 0
	interactionCount := 0

	for time.Now().Before(endTime) {
		tickCount++

		// 模拟游戏循环
		if tickCount%10 == 0 { // 每10个tick记录一次客户端指标
			c.recordDetailedClientMetrics(ctx, levelID)
		}

		// 模拟用户交互
		if tickCount%30 == 0 { // 每30个tick模拟一次用户交互
			interactionCount++
			interactionTypes := []string{"tap", "swipe", "pinch", "long_press"}
			interactionType := interactionTypes[rand.Intn(len(interactionTypes))]
			c.analytics.RecordUserInteraction(ctx, interactionType)
		}

		// 模拟渲染间隔（60 FPS目标）
		time.Sleep(16 * time.Millisecond)
	}

	// 模拟关卡完成的交互
	c.analytics.RecordUserInteraction(ctx, "level_complete")

	span.SetAttributes(
		attribute.Int("gameplay.tick_count", tickCount),
		attribute.Int("gameplay.interaction_count", interactionCount),
	)
	span.AddEvent("gameplay.complete")

	c.logger.InfoContext(ctx, "关卡游玩完成",
		slog.String("level_id", levelID),
		slog.Duration("duration", duration),
		slog.Int("tick_count", tickCount),
		slog.Int("interaction_count", interactionCount),
	)
}

// recordDetailedClientMetrics 记录详细的客户端指标
func (c *GameClient) recordDetailedClientMetrics(ctx context.Context, levelID string) {
	ctx, span := c.tracer.Start(ctx, "client.detailed_metrics",
		trace.WithAttributes(
			attribute.String("level.id", levelID),
			attribute.String("device.id", c.deviceID),
		),
	)
	defer span.End()

	// 模拟各种客户端指标
	fps := simulateRealisticFPS()
	memoryMB := simulateRealisticMemory()
	networkLatency := simulateRealisticNetworkLatency()
	cpuUsage := simulateRealisticCPU()
	batteryDrain := simulateRealisticBatteryDrain()
	temperature := simulateRealisticTemperature()

	span.SetAttributes(
		attribute.Float64("client.fps", fps),
		attribute.Float64("client.memory_mb", memoryMB),
		attribute.Float64("client.network_latency_ms", networkLatency),
		attribute.Float64("client.cpu_usage_percent", cpuUsage),
		attribute.Float64("client.battery_drain_percent_per_hour", batteryDrain),
		attribute.Float64("client.temperature_celsius", temperature),
	)

	// 检测性能问题并记录事件
	if fps < 30 {
		span.AddEvent("performance.low_fps", trace.WithAttributes(
			attribute.Float64("fps", fps),
		))
	}

	if memoryMB > 800 {
		span.AddEvent("performance.high_memory", trace.WithAttributes(
			attribute.Float64("memory_mb", memoryMB),
		))
	}

	if temperature > 70 {
		span.AddEvent("performance.overheat", trace.WithAttributes(
			attribute.Float64("temperature", temperature),
		))
	}

	if networkLatency > 200 {
		span.AddEvent("network.high_latency", trace.WithAttributes(
			attribute.Float64("latency_ms", networkLatency),
		))
	}

	// 模拟稀有事件
	if rand.Float64() < 0.001 { // 0.1% 概率内存不足
		c.analytics.RecordStabilityEvent(ctx, "out_of_memory", "level_assets_too_large")
	}

	// 记录性能指标到专用的客户端分析器
	// （这里简化处理，实际应用中会调用 clientAnalytics 的具体方法）
}

// === 现实的性能数据模拟函数 ===

func simulateRealisticFPS() float64 {
	deviceType := rand.Float64()
	if deviceType < 0.2 { // 20% 低端设备
		return rand.Float64()*20 + 15 // 15-35 FPS
	} else if deviceType < 0.6 { // 40% 中端设备
		return rand.Float64()*30 + 30 // 30-60 FPS
	} else { // 40% 高端设备
		return rand.Float64()*60 + 60 // 60-120 FPS
	}
}

func simulateRealisticMemory() float64 {
	baseMemory := 150.0 // 基础内存使用
	levelMultiplier := rand.Float64()*2 + 0.5 // 关卡复杂度影响
	return baseMemory * levelMultiplier
}

func simulateRealisticNetworkLatency() float64 {
	networkQuality := rand.Float64()
	if networkQuality < 0.1 { // 10% 差网络
		return rand.Float64()*200 + 100 // 100-300ms
	} else if networkQuality < 0.4 { // 30% 一般网络
		return rand.Float64()*80 + 30 // 30-110ms
	} else { // 60% 好网络
		return rand.Float64()*40 + 10 // 10-50ms
	}
}

func simulateRealisticCPU() float64 {
	gameIntensity := rand.Float64()*40 + 20 // 游戏强度 20-60%
	backgroundLoad := rand.Float64()*20 + 5 // 背景负载 5-25%
	return gameIntensity + backgroundLoad
}

func simulateRealisticBatteryDrain() float64 {
	return rand.Float64()*25 + 10 // 10-35% per hour
}

func simulateRealisticTemperature() float64 {
	baseTemp := 35.0 // 基础温度
	heatIncrease := rand.Float64() * 25 // 游戏产生的热量
	return baseTemp + heatIncrease
}