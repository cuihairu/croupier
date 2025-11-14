package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"math/rand"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/cuihairu/croupier/examples/otel-integration/internal/game"
	"github.com/cuihairu/croupier/examples/otel-integration/internal/telemetry"
)

func main() {
	// 创建结构化日志器
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	// 加载 OpenTelemetry 配置
	config := telemetry.LoadConfigFromEnv()
	config.ServiceName = "game-simulator"

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

	// 创建游戏模拟器
	simulator := &GameSimulator{
		gameServer:    gameServer,
		gameTelemetry: gameTelemetry,
		logger:        logger,
	}

	logger.Info("游戏模拟器启动",
		slog.String("service", config.ServiceName),
		slog.String("game_id", config.GameID),
	)

	// 启动多个模拟场景
	var wg sync.WaitGroup

	// 场景1: 正常游戏会话
	wg.Add(1)
	go func() {
		defer wg.Done()
		simulator.SimulateNormalGameplay(ctx, 20) // 20个用户
	}()

	// 场景2: 重度玩家会话（长时间游戏）
	wg.Add(1)
	go func() {
		defer wg.Done()
		simulator.SimulateHeavyGameplay(ctx, 5) // 5个重度玩家
	}()

	// 场景3: 付费用户场景
	wg.Add(1)
	go func() {
		defer wg.Done()
		simulator.SimulatePayingUsers(ctx, 3) // 3个付费用户
	}()

	// 场景4: 问题场景（崩溃、网络问题等）
	wg.Add(1)
	go func() {
		defer wg.Done()
		simulator.SimulateProblematicSessions(ctx, 8) // 8个有问题的会话
	}()

	// 等待中断信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-quit
		logger.Info("正在关闭游戏模拟器...")
		// 这里可以添加优雅关闭逻辑
		os.Exit(0)
	}()

	// 等待所有模拟完成
	wg.Wait()
	logger.Info("所有游戏模拟完成")
}

// GameSimulator 游戏模拟器
type GameSimulator struct {
	gameServer    *game.Server
	gameTelemetry *telemetry.GameTelemetry
	logger        *slog.Logger
}

// SimulateNormalGameplay 模拟正常游戏玩法
func (s *GameSimulator) SimulateNormalGameplay(ctx context.Context, userCount int) {
	s.logger.Info("开始模拟正常游戏玩法", slog.Int("user_count", userCount))

	var wg sync.WaitGroup
	for i := 0; i < userCount; i++ {
		wg.Add(1)
		go func(userIndex int) {
			defer wg.Done()

			userID := fmt.Sprintf("normal_user_%d", userIndex)
			sessionDuration := time.Duration(rand.Intn(300)+60) * time.Second // 1-6分钟

			s.gameServer.SimulateGameplay(ctx, userID, sessionDuration)
		}(i)

		// 错峰启动，避免同时开始
		time.Sleep(time.Duration(rand.Intn(5)+1) * time.Second)
	}

	wg.Wait()
	s.logger.Info("正常游戏玩法模拟完成")
}

// SimulateHeavyGameplay 模拟重度玩家游戏
func (s *GameSimulator) SimulateHeavyGameplay(ctx context.Context, userCount int) {
	s.logger.Info("开始模拟重度玩家游戏", slog.Int("user_count", userCount))

	var wg sync.WaitGroup
	for i := 0; i < userCount; i++ {
		wg.Add(1)
		go func(userIndex int) {
			defer wg.Done()

			userID := fmt.Sprintf("heavy_user_%d", userIndex)
			sessionDuration := time.Duration(rand.Intn(600)+900) * time.Second // 15-25分钟

			s.gameServer.SimulateGameplay(ctx, userID, sessionDuration)
		}(i)

		time.Sleep(time.Duration(rand.Intn(10)+5) * time.Second)
	}

	wg.Wait()
	s.logger.Info("重度玩家游戏模拟完成")
}

// SimulatePayingUsers 模拟付费用户
func (s *GameSimulator) SimulatePayingUsers(ctx context.Context, userCount int) {
	s.logger.Info("开始模拟付费用户", slog.Int("user_count", userCount))

	platforms := []string{"ios", "android", "windows"}
	regions := []string{"cn-north", "us-west", "eu-west"}

	for i := 0; i < userCount; i++ {
		userID := fmt.Sprintf("paying_user_%d", i)
		platform := platforms[rand.Intn(len(platforms))]
		region := regions[rand.Intn(len(regions))]

		// 开始会话
		session := s.gameServer.HandlePlayerLogin(ctx, userID, "premium-game", platform, region)

		// 模拟付费用户的行为模式
		s.simulatePayingUserBehavior(ctx, session.ID)

		// 结束会话
		s.gameServer.HandlePlayerLogout(session.ID, "normal")

		time.Sleep(time.Duration(rand.Intn(30)+10) * time.Second)
	}

	s.logger.Info("付费用户模拟完成")
}

// simulatePayingUserBehavior 模拟付费用户行为
func (s *GameSimulator) simulatePayingUserBehavior(ctx context.Context, sessionID string) {
	// 付费用户通常会：
	// 1. 玩更多关卡
	// 2. 购买道具/角色
	// 3. 会话时间更长
	// 4. 网络和设备性能更好

	for level := 1; level <= 10; level++ {
		levelID := fmt.Sprintf("premium_level_%d", level)
		difficulty := rand.Intn(3) + 3 // 更高难度

		levelCtx, err := s.gameServer.HandleLevelStart(sessionID, levelID, difficulty)
		if err != nil {
			continue
		}

		// 付费用户性能更好
		fps := rand.Float64()*30 + 50  // 50-80 FPS
		latency := rand.Float64()*30 + 10  // 10-40ms
		s.gameServer.HandleClientMetrics(sessionID, fps, latency)

		// 模拟游戏时间
		time.Sleep(time.Duration(rand.Intn(45)+30) * time.Second)

		// 付费用户成功率更高
		if rand.Float64() < 0.95 { // 95% 成功率
			score := rand.Intn(2000) + 1000
			coins := rand.Intn(200) + 100
			s.gameServer.HandleLevelComplete(levelCtx, sessionID, levelID, score, coins)
		} else {
			s.gameServer.HandleLevelFail(levelCtx, sessionID, levelID, "challenging_difficulty")
		}

		// 购买行为（30% 概率）
		if rand.Float64() < 0.3 {
			skus := []string{"premium_character", "legendary_weapon", "xp_booster", "coin_pack_mega"}
			prices := []float64{9.99, 14.99, 4.99, 19.99}
			idx := rand.Intn(len(skus))
			s.gameServer.HandlePurchase(sessionID, skus[idx], prices[idx])
		}

		time.Sleep(time.Duration(rand.Intn(10)+5) * time.Second)
	}
}

// SimulateProblematicSessions 模拟有问题的会话
func (s *GameSimulator) SimulateProblematicSessions(ctx context.Context, sessionCount int) {
	s.logger.Info("开始模拟问题会话", slog.Int("session_count", sessionCount))

	problemTypes := []string{"crash", "network_issues", "low_performance", "early_quit"}

	for i := 0; i < sessionCount; i++ {
		problemType := problemTypes[rand.Intn(len(problemTypes))]
		userID := fmt.Sprintf("problem_user_%s_%d", problemType, i)

		switch problemType {
		case "crash":
			s.simulateCrashSession(ctx, userID)
		case "network_issues":
			s.simulateNetworkIssues(ctx, userID)
		case "low_performance":
			s.simulateLowPerformance(ctx, userID)
		case "early_quit":
			s.simulateEarlyQuit(ctx, userID)
		}

		time.Sleep(time.Duration(rand.Intn(20)+10) * time.Second)
	}

	s.logger.Info("问题会话模拟完成")
}

// simulateCrashSession 模拟崩溃会话
func (s *GameSimulator) simulateCrashSession(ctx context.Context, userID string) {
	session := s.gameServer.HandlePlayerLogin(ctx, userID, "unstable-game", "android", "cn-south")

	// 玩一会儿然后崩溃
	for level := 1; level <= 3; level++ {
		levelID := fmt.Sprintf("level_%d", level)
		levelCtx, err := s.gameServer.HandleLevelStart(session.ID, levelID, 1)
		if err != nil {
			break
		}

		// 模拟游戏时间
		time.Sleep(time.Duration(rand.Intn(30)+10) * time.Second)

		// 随机完成或失败
		if rand.Float64() < 0.5 {
			s.gameServer.HandleLevelComplete(levelCtx, session.ID, levelID, rand.Intn(500), rand.Intn(50))
		} else {
			s.gameServer.HandleLevelFail(levelCtx, session.ID, levelID, "network_timeout")
		}
	}

	// 崩溃
	crashReasons := []string{"out_of_memory", "null_pointer", "graphics_error", "stack_overflow"}
	reason := crashReasons[rand.Intn(len(crashReasons))]
	s.gameServer.HandleCrash(session.ID, reason)
}

// simulateNetworkIssues 模拟网络问题
func (s *GameSimulator) simulateNetworkIssues(ctx context.Context, userID string) {
	session := s.gameServer.HandlePlayerLogin(ctx, userID, "mobile-game", "android", "remote-area")

	// 网络延迟较高
	for i := 0; i < 5; i++ {
		highLatency := rand.Float64()*300 + 200  // 200-500ms 高延迟
		lowFPS := rand.Float64()*20 + 15  // 15-35 FPS 低帧率
		s.gameServer.HandleClientMetrics(session.ID, lowFPS, highLatency)

		time.Sleep(10 * time.Second)
	}

	s.gameServer.HandlePlayerLogout(session.ID, "network_timeout")
}

// simulateLowPerformance 模拟低性能设备
func (s *GameSimulator) simulateLowPerformance(ctx context.Context, userID string) {
	session := s.gameServer.HandlePlayerLogin(ctx, userID, "casual-game", "android", "emerging-market")

	// 低性能表现
	for i := 0; i < 8; i++ {
		lowFPS := rand.Float64()*15 + 10  // 10-25 FPS
		moderateLatency := rand.Float64()*100 + 50  // 50-150ms
		s.gameServer.HandleClientMetrics(session.ID, lowFPS, moderateLatency)

		time.Sleep(15 * time.Second)
	}

	s.gameServer.HandlePlayerLogout(session.ID, "performance_issues")
}

// simulateEarlyQuit 模拟早退会话
func (s *GameSimulator) simulateEarlyQuit(ctx context.Context, userID string) {
	session := s.gameServer.HandlePlayerLogin(ctx, userID, "tutorial-game", "web", "trial-user")

	// 只玩很短时间就退出
	levelCtx, err := s.gameServer.HandleLevelStart(session.ID, "tutorial_level", 1)
	if err == nil {
		time.Sleep(30 * time.Second) // 只玩30秒
		s.gameServer.HandleLevelFail(levelCtx, session.ID, "tutorial_level", "too_difficult")
	}

	s.gameServer.HandlePlayerLogout(session.ID, "user_quit_early")
}