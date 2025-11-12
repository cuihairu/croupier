package main

import (
	"context"
	"log"
	"log/slog"
	"math/rand"
	"net/http"
	"os"
	"time"

	"github.com/cuihairu/croupier/internal/telemetry"
	"github.com/gin-gonic/gin"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	// 从环境变量加载配置
	config := telemetry.LoadConfigFromEnv()

	// 创建遥测服务
	telemetryService, err := telemetry.NewGameTelemetryService(config, logger)
	if err != nil {
		log.Fatalf("Failed to create telemetry service: %v", err)
	}
	defer telemetryService.Shutdown(context.Background())

	// 创建Gin应用
	r := gin.Default()

	// 添加OpenTelemetry中间件
	r.Use(telemetryService.GinMiddleware())

	// 模拟游戏API端点
	r.GET("/api/session/start", func(c *gin.Context) {
		ctx := c.Request.Context()

		// 模拟会话开始
		req := telemetry.SessionStartRequest{
			UserID:     "user123",
			SessionID:  "session456",
			Platform:   "ios",
			Region:     "us-east",
			GameType:   "tower_defense",
			GenreCode:  "strategy",
			AppVersion: "1.0.0",
			EntryPoint: "main_menu",
		}

		ctx, span := telemetryService.StartUserSession(ctx, req)
		defer span.End()

		c.JSON(http.StatusOK, gin.H{"status": "session started", "session_id": req.SessionID})
	})

	r.GET("/api/session/end", func(c *gin.Context) {
		ctx := c.Request.Context()

		// 模拟会话结束
		req := telemetry.SessionEndRequest{
			UserID:     "user123",
			SessionID:  "session456",
			DurationMs: int64(rand.Intn(300000) + 60000), // 1-5分钟
			CauseOfEnd: "normal",
		}

		telemetryService.EndUserSession(ctx, req)

		c.JSON(http.StatusOK, gin.H{"status": "session ended"})
	})

	r.GET("/api/level/complete", func(c *gin.Context) {
		ctx := c.Request.Context()

		// 模拟关卡完成
		req := telemetry.LevelCompleteRequest{
			LevelID:         "level_1",
			DurationMs:      int64(rand.Intn(120000) + 30000), // 30秒-2分钟
			Stars:           rand.Intn(3) + 1,                  // 1-3星
			Retries:         rand.Intn(3),                      // 0-2次重试
			WaveIndex:       rand.Intn(10) + 1,                 // 第1-10波
			HeartsRemaining: rand.Intn(3),                      // 剩余生命
			Difficulty:      "normal",
		}

		telemetryService.CompleteLevelPlaythrough(ctx, req)

		c.JSON(http.StatusOK, gin.H{
			"status":   "level completed",
			"level_id": req.LevelID,
			"stars":    req.Stars,
		})
	})

	r.GET("/api/economy/transaction", func(c *gin.Context) {
		ctx := c.Request.Context()

		// 模拟经济交易
		transaction := telemetry.EconomyTransaction{
			UserID:       "user123",
			Currency:     "coins",
			CurrencyKind: "soft",
			Amount:       float64(rand.Intn(1000) + 100),
			Type:         "earn",
			Source:       "level_completion",
			BalanceAfter: float64(rand.Intn(10000) + 1000),
		}

		telemetryService.TrackEconomyTransaction(ctx, transaction)

		c.JSON(http.StatusOK, gin.H{
			"status":   "transaction recorded",
			"amount":   transaction.Amount,
			"currency": transaction.Currency,
		})
	})

	// 健康检查端点
	r.GET("/health", func(c *gin.Context) {
		ctx := c.Request.Context()

		err := telemetryService.Health(ctx)
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unhealthy", "error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})

	// 指标端点（用于Prometheus）
	r.GET("/metrics", func(c *gin.Context) {
		c.String(200, "# Prometheus metrics endpoint (placeholder)")
	})

	// 启动自动事件生成器（演示用）
	go generateDemoEvents(telemetryService)

	logger.Info("Starting Croupier Demo Server", "port", 8080)
	log.Fatal(r.Run(":8080"))
}

// generateDemoEvents 生成演示事件
func generateDemoEvents(service *telemetry.GameTelemetryService) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ctx := context.Background()

			// 随机生成不同类型的事件
			eventType := rand.Intn(4)

			switch eventType {
			case 0: // 会话开始
				req := telemetry.SessionStartRequest{
					UserID:     generateUserID(),
					SessionID:  generateSessionID(),
					Platform:   randomPlatform(),
					Region:     randomRegion(),
					GameType:   "tower_defense",
					GenreCode:  "strategy",
					AppVersion: "1.0.0",
					EntryPoint: randomEntryPoint(),
				}
				_, span := service.StartUserSession(ctx, req)
				span.End()

			case 1: // 关卡完成
				req := telemetry.LevelCompleteRequest{
					LevelID:         randomLevel(),
					DurationMs:      int64(rand.Intn(120000) + 30000),
					Stars:           rand.Intn(3) + 1,
					Retries:         rand.Intn(3),
					WaveIndex:       rand.Intn(10) + 1,
					HeartsRemaining: rand.Intn(3),
					Difficulty:      randomDifficulty(),
				}
				service.CompleteLevelPlaythrough(ctx, req)

			case 2: // 经济交易
				transaction := telemetry.EconomyTransaction{
					UserID:       generateUserID(),
					Currency:     randomCurrency(),
					CurrencyKind: randomCurrencyKind(),
					Amount:       float64(rand.Intn(1000) + 100),
					Type:         randomTransactionType(),
					Source:       randomSource(),
					BalanceAfter: float64(rand.Intn(10000) + 1000),
				}
				service.TrackEconomyTransaction(ctx, transaction)
			}
		}
	}
}

// 辅助函数生成随机数据
func generateUserID() string {
	return "user" + randomString(6)
}

func generateSessionID() string {
	return "session" + randomString(8)
}

func randomPlatform() string {
	platforms := []string{"ios", "android", "windows", "mac", "web"}
	return platforms[rand.Intn(len(platforms))]
}

func randomRegion() string {
	regions := []string{"us-east", "us-west", "eu", "asia", "cn"}
	return regions[rand.Intn(len(regions))]
}

func randomEntryPoint() string {
	entryPoints := []string{"main_menu", "push_notification", "deep_link", "social_share"}
	return entryPoints[rand.Intn(len(entryPoints))]
}

func randomLevel() string {
	levels := []string{"level_1", "level_2", "level_3", "boss_1", "daily_challenge"}
	return levels[rand.Intn(len(levels))]
}

func randomDifficulty() string {
	difficulties := []string{"easy", "normal", "hard", "expert"}
	return difficulties[rand.Intn(len(difficulties))]
}

func randomCurrency() string {
	currencies := []string{"coins", "gems", "energy", "hearts"}
	return currencies[rand.Intn(len(currencies))]
}

func randomCurrencyKind() string {
	kinds := []string{"soft", "hard", "premium"}
	return kinds[rand.Intn(len(kinds))]
}

func randomTransactionType() string {
	types := []string{"earn", "spend"}
	return types[rand.Intn(len(types))]
}

func randomSource() string {
	sources := []string{"level_completion", "daily_login", "achievement", "ad_reward"}
	return sources[rand.Intn(len(sources))]
}

func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}