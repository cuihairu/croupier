package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cuihairu/croupier/examples/otel-integration/internal/game"
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
	config.ServiceName = "game-server"

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

	// 创建游戏遥测服务
	gameTelemetry, err := telemetry.NewGameTelemetry(logger)
	if err != nil {
		log.Fatalf("Failed to create game telemetry: %v", err)
	}

	// 创建游戏服务器
	gameServer := game.NewServer(gameTelemetry)

	// 创建 HTTP 服务器来处理 API 请求
	mux := http.NewServeMux()

	// 健康检查端点
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		tracer := otel.Tracer("game-server")
		ctx, span := tracer.Start(r.Context(), "health_check")
		defer span.End()

		span.SetAttributes(
			attribute.String("http.method", r.Method),
			attribute.String("http.url", r.URL.String()),
		)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"healthy","service":"game-server"}`))

		logger.InfoContext(ctx, "健康检查请求",
			slog.String("method", r.Method),
			slog.String("url", r.URL.String()),
		)
	})

	// 玩家登录端点
	mux.HandleFunc("/api/login", func(w http.ResponseWriter, r *http.Request) {
		tracer := otel.Tracer("game-server")
		ctx, span := tracer.Start(r.Context(), "player_login",
			trace.WithAttributes(
				attribute.String("http.method", r.Method),
				attribute.String("http.endpoint", "/api/login"),
			),
		)
		defer span.End()

		start := time.Now()

		userID := r.URL.Query().Get("user_id")
		if userID == "" {
			userID = fmt.Sprintf("user_%d", time.Now().UnixNano()%10000)
		}

		platform := r.URL.Query().Get("platform")
		if platform == "" {
			platform = "web"
		}

		region := r.URL.Query().Get("region")
		if region == "" {
			region = "cn-north"
		}

		session := gameServer.HandlePlayerLogin(ctx, userID, config.GameID, platform, region)

		duration := time.Since(start)
		statusCode := http.StatusOK

		// 记录 API 指标
		gameTelemetry.RecordAPICall(ctx, r.Method, "/api/login", float64(duration.Milliseconds()), statusCode)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		response := fmt.Sprintf(`{"session_id":"%s","user_id":"%s","status":"success"}`, session.ID, userID)
		w.Write([]byte(response))

		logger.InfoContext(ctx, "玩家登录",
			slog.String("session_id", session.ID),
			slog.String("user_id", userID),
			slog.String("platform", platform),
			slog.String("region", region),
			slog.Duration("duration", duration),
		)
	})

	// 关卡开始端点
	mux.HandleFunc("/api/level/start", func(w http.ResponseWriter, r *http.Request) {
		tracer := otel.Tracer("game-server")
		ctx, span := tracer.Start(r.Context(), "level_start",
			trace.WithAttributes(
				attribute.String("http.method", r.Method),
				attribute.String("http.endpoint", "/api/level/start"),
			),
		)
		defer span.End()

		start := time.Now()

		sessionID := r.URL.Query().Get("session_id")
		levelID := r.URL.Query().Get("level_id")
		if levelID == "" {
			levelID = "level_1"
		}

		levelCtx, err := gameServer.HandleLevelStart(sessionID, levelID, 1)
		if err != nil {
			statusCode := http.StatusBadRequest
			gameTelemetry.RecordAPICall(ctx, r.Method, "/api/level/start", float64(time.Since(start).Milliseconds()), statusCode)

			http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), statusCode)
			return
		}

		duration := time.Since(start)
		statusCode := http.StatusOK

		gameTelemetry.RecordAPICall(ctx, r.Method, "/api/level/start", float64(duration.Milliseconds()), statusCode)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		response := fmt.Sprintf(`{"level_id":"%s","status":"started"}`, levelID)
		w.Write([]byte(response))

		logger.InfoContext(levelCtx, "关卡开始",
			slog.String("session_id", sessionID),
			slog.String("level_id", levelID),
		)
	})

	// 指标端点
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("# 指标数据通过 OpenTelemetry 导出到 Collector\n"))
	})

	// 启动 HTTP 服务器
	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	// 启动后台游戏模拟器
	go func() {
		time.Sleep(5 * time.Second) // 等待服务器启动

		logger.Info("开始游戏模拟")
		for i := 0; i < 5; i++ {
			userID := fmt.Sprintf("simulated_user_%d", i+1)
			go gameServer.SimulateGameplay(ctx, userID, 2*time.Minute)
		}
	}()

	// 优雅关闭处理
	go func() {
		logger.Info("游戏服务器启动", slog.String("addr", server.Addr))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("服务器启动失败", "error", err)
		}
	}()

	// 等待中断信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("正在关闭游戏服务器...")

	// 优雅关闭 HTTP 服务器
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("服务器关闭失败", "error", err)
	}

	logger.Info("游戏服务器已关闭")
}