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

	// 模拟客户端行为
	client := &GameClient{
		serverURL: "http://localhost:8080",
		logger:    logger,
		tracer:    otel.Tracer("game-client"),
	}

	// 运行游戏会话
	userID := fmt.Sprintf("client_user_%d", time.Now().Unix())
	client.RunGameSession(ctx, userID)
}

// GameClient 游戏客户端
type GameClient struct {
	serverURL string
	logger    *slog.Logger
	tracer    trace.Tracer
}

// RunGameSession 运行游戏会话
func (c *GameClient) RunGameSession(ctx context.Context, userID string) {
	ctx, span := c.tracer.Start(ctx, "game.client.session",
		trace.WithAttributes(
			attribute.String("client.user_id", userID),
		),
	)
	defer span.End()

	// 1. 登录
	sessionID, err := c.login(ctx, userID, "ios", "cn-north")
	if err != nil {
		c.logger.ErrorContext(ctx, "登录失败", "error", err)
		return
	}

	c.logger.InfoContext(ctx, "登录成功",
		slog.String("session_id", sessionID),
		slog.String("user_id", userID),
	)

	// 2. 模拟游戏玩法
	for i := 1; i <= 5; i++ {
		levelID := fmt.Sprintf("level_%d", i)

		// 开始关卡
		err := c.startLevel(ctx, sessionID, levelID)
		if err != nil {
			c.logger.ErrorContext(ctx, "关卡开始失败",
				"level_id", levelID,
				"error", err,
			)
			continue
		}

		// 模拟游戏玩法
		c.simulateGameplay(ctx, levelID, time.Duration(rand.Intn(30)+10)*time.Second)

		c.logger.InfoContext(ctx, "关卡完成",
			slog.String("level_id", levelID),
		)

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
func (c *GameClient) simulateGameplay(ctx context.Context, levelID string, duration time.Duration) {
	ctx, span := c.tracer.Start(ctx, "client.gameplay",
		trace.WithAttributes(
			attribute.String("level.id", levelID),
			attribute.Int64("gameplay.duration_ms", duration.Milliseconds()),
		),
	)
	defer span.End()

	endTime := time.Now().Add(duration)
	tickCount := 0

	for time.Now().Before(endTime) {
		tickCount++

		// 模拟游戏循环
		if tickCount%10 == 0 { // 每10个tick记录一次指标
			c.recordClientMetrics(ctx, levelID)
		}

		// 模拟渲染间隔（60 FPS）
		time.Sleep(16 * time.Millisecond)
	}

	span.SetAttributes(attribute.Int("gameplay.tick_count", tickCount))
	span.AddEvent("gameplay.complete")

	c.logger.InfoContext(ctx, "关卡游玩完成",
		slog.String("level_id", levelID),
		slog.Duration("duration", duration),
		slog.Int("tick_count", tickCount),
	)
}

// recordClientMetrics 记录客户端指标
func (c *GameClient) recordClientMetrics(ctx context.Context, levelID string) {
	ctx, span := c.tracer.Start(ctx, "client.metrics",
		trace.WithAttributes(
			attribute.String("level.id", levelID),
		),
	)
	defer span.End()

	// 模拟客户端指标
	fps := rand.Float64()*30 + 45  // 45-75 FPS
	memoryMB := rand.Float64()*200 + 100  // 100-300 MB
	networkLatency := rand.Float64()*50 + 20  // 20-70 ms
	cpuUsage := rand.Float64()*40 + 30  // 30-70%

	span.SetAttributes(
		attribute.Float64("client.fps", fps),
		attribute.Float64("client.memory_mb", memoryMB),
		attribute.Float64("client.network_latency_ms", networkLatency),
		attribute.Float64("client.cpu_usage_percent", cpuUsage),
	)

	// 模拟偶尔的性能问题
	if rand.Float64() < 0.1 { // 10% 概率
		if fps < 50 {
			span.AddEvent("performance.low_fps", trace.WithAttributes(
				attribute.Float64("fps", fps),
			))
		}
		if networkLatency > 100 {
			span.AddEvent("network.high_latency", trace.WithAttributes(
				attribute.Float64("latency_ms", networkLatency),
			))
		}
	}
}