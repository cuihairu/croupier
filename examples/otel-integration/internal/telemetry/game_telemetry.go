package telemetry

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// GameTelemetry 游戏遥测服务
type GameTelemetry struct {
	tracer       trace.Tracer
	meter        metric.Meter
	logger       *slog.Logger

	// 业务指标
	sessionCounter       metric.Int64Counter
	sessionDuration      metric.Float64Histogram
	playerCounter        metric.Int64UpDownCounter
	levelStartCounter    metric.Int64Counter
	levelCompleteCounter metric.Int64Counter
	levelFailCounter     metric.Int64Counter
	revenueCounter       metric.Float64Counter
	crashCounter         metric.Int64Counter

	// 技术指标
	requestDuration      metric.Float64Histogram
	requestCounter       metric.Int64Counter
	errorCounter         metric.Int64Counter
	networkLatency       metric.Float64Histogram
	clientFPS           metric.Float64Histogram
}

// NewGameTelemetry 创建游戏遥测服务
func NewGameTelemetry(logger *slog.Logger) (*GameTelemetry, error) {
	gt := &GameTelemetry{
		tracer: otel.Tracer("game.example"),
		meter:  otel.Meter("game.example"),
		logger: logger,
	}

	var err error

	// 业务指标初始化
	gt.sessionCounter, err = gt.meter.Int64Counter("game.session.total",
		metric.WithDescription("Total game sessions started"),
		metric.WithUnit("{sessions}"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create session counter: %w", err)
	}

	gt.sessionDuration, err = gt.meter.Float64Histogram("game.session.duration",
		metric.WithDescription("Game session duration"),
		metric.WithUnit("ms"),
		metric.WithExplicitBucketBoundaries(
			1000, 30000, 60000, 300000, 600000, 1800000, 3600000, 7200000,
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create session duration histogram: %w", err)
	}

	gt.playerCounter, err = gt.meter.Int64UpDownCounter("game.players.online",
		metric.WithDescription("Current online players"),
		metric.WithUnit("{players}"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create player counter: %w", err)
	}

	gt.levelStartCounter, err = gt.meter.Int64Counter("game.level.start.total",
		metric.WithDescription("Total level attempts"),
		metric.WithUnit("{attempts}"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create level start counter: %w", err)
	}

	gt.levelCompleteCounter, err = gt.meter.Int64Counter("game.level.complete.total",
		metric.WithDescription("Total level completions"),
		metric.WithUnit("{completions}"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create level complete counter: %w", err)
	}

	gt.levelFailCounter, err = gt.meter.Int64Counter("game.level.fail.total",
		metric.WithDescription("Total level failures"),
		metric.WithUnit("{failures}"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create level fail counter: %w", err)
	}

	gt.revenueCounter, err = gt.meter.Float64Counter("game.revenue.total",
		metric.WithDescription("Total revenue in USD"),
		metric.WithUnit("USD"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create revenue counter: %w", err)
	}

	gt.crashCounter, err = gt.meter.Int64Counter("game.crash.total",
		metric.WithDescription("Total client crashes"),
		metric.WithUnit("{crashes}"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create crash counter: %w", err)
	}

	// 技术指标初始化
	gt.requestDuration, err = gt.meter.Float64Histogram("game.request.duration",
		metric.WithDescription("Request duration"),
		metric.WithUnit("ms"),
		metric.WithExplicitBucketBoundaries(1, 5, 10, 25, 50, 75, 100, 250, 500, 750, 1000, 2500, 5000, 7500, 10000),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create request duration histogram: %w", err)
	}

	gt.requestCounter, err = gt.meter.Int64Counter("game.request.total",
		metric.WithDescription("Total requests"),
		metric.WithUnit("{requests}"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create request counter: %w", err)
	}

	gt.errorCounter, err = gt.meter.Int64Counter("game.error.total",
		metric.WithDescription("Total errors"),
		metric.WithUnit("{errors}"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create error counter: %w", err)
	}

	gt.networkLatency, err = gt.meter.Float64Histogram("game.network.latency",
		metric.WithDescription("Network latency"),
		metric.WithUnit("ms"),
		metric.WithExplicitBucketBoundaries(10, 25, 50, 100, 150, 200, 300, 500, 1000, 2000),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create network latency histogram: %w", err)
	}

	gt.clientFPS, err = gt.meter.Float64Histogram("game.client.fps",
		metric.WithDescription("Client frames per second"),
		metric.WithUnit("fps"),
		metric.WithExplicitBucketBoundaries(10, 15, 20, 30, 45, 60, 75, 90, 120, 144, 240),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create client FPS histogram: %w", err)
	}

	return gt, nil
}

// GameSession 表示游戏会话
type GameSession struct {
	ID           string
	UserID       string
	GameID       string
	Platform     string
	Region       string
	StartTime    time.Time
	Level        string
	gt          *GameTelemetry
	span         trace.Span
	ctx          context.Context
}

// StartSession 开始游戏会话
func (gt *GameTelemetry) StartSession(ctx context.Context, userID, gameID, platform, region string) *GameSession {
	sessionID := fmt.Sprintf("session_%d_%s", time.Now().UnixNano(), userID[:8])

	// 创建分布式追踪 span
	ctx, span := gt.tracer.Start(ctx, "game.session",
		trace.WithAttributes(
			attribute.String("game.id", gameID),
			attribute.String("game.user_id", hashUserID(userID)), // 脱敏处理
			attribute.String("game.session_id", sessionID),
			attribute.String("game.platform", platform),
			attribute.String("game.region", region),
			attribute.String("session.entry_point", "main_menu"),
		),
	)

	// 记录会话开始指标
	attrs := metric.WithAttributes(
		attribute.String("game.id", gameID),
		attribute.String("game.platform", platform),
		attribute.String("game.region", region),
	)

	gt.sessionCounter.Add(ctx, 1, attrs)
	gt.playerCounter.Add(ctx, 1, attrs)

	// 结构化日志
	gt.logger.InfoContext(ctx, "游戏会话开始",
		slog.String("session_id", sessionID),
		slog.String("user_id", hashUserID(userID)),
		slog.String("game_id", gameID),
		slog.String("platform", platform),
		slog.String("region", region),
		slog.String("trace_id", span.SpanContext().TraceID().String()),
	)

	span.AddEvent("session.start", trace.WithAttributes(
		attribute.String("entry_point", "main_menu"),
	))

	return &GameSession{
		ID:        sessionID,
		UserID:    userID,
		GameID:    gameID,
		Platform:  platform,
		Region:    region,
		StartTime: time.Now(),
		gt:        gt,
		span:      span,
		ctx:       ctx,
	}
}

// StartLevel 开始关卡
func (gs *GameSession) StartLevel(levelID string, difficulty int) context.Context {
	gs.Level = levelID

	// 创建关卡追踪 span
	ctx, span := gs.gt.tracer.Start(gs.ctx, "game.level.attempt",
		trace.WithAttributes(
			attribute.String("game.id", gs.GameID),
			attribute.String("game.session_id", gs.ID),
			attribute.String("level.id", levelID),
			attribute.Int("level.difficulty", difficulty),
		),
	)

	// 记录关卡开始指标
	attrs := metric.WithAttributes(
		attribute.String("game.id", gs.GameID),
		attribute.String("level.id", levelID),
		attribute.Int("level.difficulty", difficulty),
		attribute.String("game.platform", gs.Platform),
	)

	gs.gt.levelStartCounter.Add(ctx, 1, attrs)

	gs.gt.logger.InfoContext(ctx, "关卡开始",
		slog.String("session_id", gs.ID),
		slog.String("level_id", levelID),
		slog.Int("difficulty", difficulty),
		slog.String("trace_id", span.SpanContext().TraceID().String()),
	)

	span.AddEvent("level.loading.start")

	// 模拟关卡加载延迟
	time.Sleep(time.Duration(rand.Intn(100)+50) * time.Millisecond)
	span.AddEvent("level.loading.complete")

	return ctx
}

// CompleteLevel 完成关卡
func (gs *GameSession) CompleteLevel(ctx context.Context, levelID string, score int, earnedCoins int) {
	span := trace.SpanFromContext(ctx)

	// 记录关卡完成
	attrs := metric.WithAttributes(
		attribute.String("game.id", gs.GameID),
		attribute.String("level.id", levelID),
		attribute.String("game.platform", gs.Platform),
	)

	gs.gt.levelCompleteCounter.Add(ctx, 1, attrs)

	// 记录经济收益
	economyAttrs := metric.WithAttributes(
		attribute.String("game.id", gs.GameID),
		attribute.String("economy.currency", "coins"),
		attribute.String("economy.source", "level_complete"),
	)

	span.SetAttributes(
		attribute.Int("level.score", score),
		attribute.Int("economy.coins_earned", earnedCoins),
		attribute.String("level.result", "complete"),
	)

	gs.gt.logger.InfoContext(ctx, "关卡完成",
		slog.String("session_id", gs.ID),
		slog.String("level_id", levelID),
		slog.Int("score", score),
		slog.Int("earned_coins", earnedCoins),
	)

	span.AddEvent("level.complete", trace.WithAttributes(
		attribute.Int("score", score),
		attribute.Int("earned_coins", earnedCoins),
	))

	span.End()
}

// FailLevel 关卡失败
func (gs *GameSession) FailLevel(ctx context.Context, levelID string, failReason string) {
	span := trace.SpanFromContext(ctx)

	attrs := metric.WithAttributes(
		attribute.String("game.id", gs.GameID),
		attribute.String("level.id", levelID),
		attribute.String("fail_reason", failReason),
		attribute.String("game.platform", gs.Platform),
	)

	gs.gt.levelFailCounter.Add(ctx, 1, attrs)

	span.SetAttributes(
		attribute.String("level.result", "fail"),
		attribute.String("level.fail_reason", failReason),
	)

	gs.gt.logger.WarnContext(ctx, "关卡失败",
		slog.String("session_id", gs.ID),
		slog.String("level_id", levelID),
		slog.String("fail_reason", failReason),
	)

	span.AddEvent("level.fail", trace.WithAttributes(
		attribute.String("fail_reason", failReason),
	))

	span.End()
}

// RecordPurchase 记录购买
func (gs *GameSession) RecordPurchase(sku string, priceUSD float64) {
	ctx, span := gs.gt.tracer.Start(gs.ctx, "game.monetization.purchase",
		trace.WithAttributes(
			attribute.String("game.id", gs.GameID),
			attribute.String("monetization.sku", sku),
			attribute.Float64("monetization.price_usd", priceUSD),
		),
	)
	defer span.End()

	// 记录收入
	revenueAttrs := metric.WithAttributes(
		attribute.String("game.id", gs.GameID),
		attribute.String("monetization.sku", sku),
		attribute.String("game.platform", gs.Platform),
	)

	gs.gt.revenueCounter.Add(ctx, priceUSD, revenueAttrs)

	gs.gt.logger.InfoContext(ctx, "购买完成",
		slog.String("session_id", gs.ID),
		slog.String("sku", sku),
		slog.Float64("price_usd", priceUSD),
	)

	span.AddEvent("purchase.complete")
}

// RecordNetworkLatency 记录网络延迟
func (gs *GameSession) RecordNetworkLatency(latencyMs float64) {
	attrs := metric.WithAttributes(
		attribute.String("game.id", gs.GameID),
		attribute.String("game.platform", gs.Platform),
	)

	gs.gt.networkLatency.Record(gs.ctx, latencyMs, attrs)
}

// RecordClientFPS 记录客户端帧率
func (gs *GameSession) RecordClientFPS(fps float64) {
	attrs := metric.WithAttributes(
		attribute.String("game.id", gs.GameID),
		attribute.String("game.platform", gs.Platform),
	)

	gs.gt.clientFPS.Record(gs.ctx, fps, attrs)
}

// RecordCrash 记录崩溃
func (gs *GameSession) RecordCrash(crashReason string) {
	ctx, span := gs.gt.tracer.Start(gs.ctx, "game.error.crash",
		trace.WithAttributes(
			attribute.String("game.id", gs.GameID),
			attribute.String("error.crash_reason", crashReason),
			attribute.String("game.platform", gs.Platform),
		),
	)
	defer span.End()

	attrs := metric.WithAttributes(
		attribute.String("game.id", gs.GameID),
		attribute.String("crash_reason", crashReason),
		attribute.String("game.platform", gs.Platform),
	)

	gs.gt.crashCounter.Add(ctx, 1, attrs)

	gs.gt.logger.ErrorContext(ctx, "游戏崩溃",
		slog.String("session_id", gs.ID),
		slog.String("crash_reason", crashReason),
	)

	span.AddEvent("client.crash")
}

// EndSession 结束游戏会话
func (gs *GameSession) EndSession(causeEnd string) {
	duration := time.Since(gs.StartTime)

	// 记录会话时长
	durationAttrs := metric.WithAttributes(
		attribute.String("game.id", gs.GameID),
		attribute.String("game.platform", gs.Platform),
		attribute.String("session.cause_end", causeEnd),
	)

	gs.gt.sessionDuration.Record(gs.ctx, float64(duration.Milliseconds()), durationAttrs)
	gs.gt.playerCounter.Add(gs.ctx, -1, metric.WithAttributes(
		attribute.String("game.id", gs.GameID),
		attribute.String("game.platform", gs.Platform),
	))

	gs.span.SetAttributes(
		attribute.String("session.cause_end", causeEnd),
		attribute.Int64("session.duration_ms", duration.Milliseconds()),
	)

	gs.gt.logger.InfoContext(gs.ctx, "游戏会话结束",
		slog.String("session_id", gs.ID),
		slog.String("cause_end", causeEnd),
		slog.Duration("duration", duration),
	)

	gs.span.AddEvent("session.end", trace.WithAttributes(
		attribute.String("cause_end", causeEnd),
	))

	gs.span.End()
}

// RecordAPICall 记录API调用
func (gt *GameTelemetry) RecordAPICall(ctx context.Context, method, endpoint string, durationMs float64, statusCode int) {
	attrs := metric.WithAttributes(
		attribute.String("http.method", method),
		attribute.String("http.endpoint", endpoint),
		attribute.Int("http.status_code", statusCode),
	)

	gt.requestDuration.Record(ctx, durationMs, attrs)
	gt.requestCounter.Add(ctx, 1, attrs)

	if statusCode >= 400 {
		gt.errorCounter.Add(ctx, 1, attrs)
	}
}

// hashUserID 对用户ID进行脱敏处理
func hashUserID(userID string) string {
	// 简单的哈希处理，实际应用中应使用更安全的哈希算法
	return fmt.Sprintf("user_%x", len(userID)*37+int(userID[0]))
}