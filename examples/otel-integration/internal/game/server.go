package game

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/cuihairu/croupier/examples/otel-integration/internal/telemetry"
)

// Server 游戏服务器
type Server struct {
	gameTelemetry *telemetry.GameTelemetry
	activeSessions map[string]*telemetry.GameSession
}

// NewServer 创建游戏服务器
func NewServer(gt *telemetry.GameTelemetry) *Server {
	return &Server{
		gameTelemetry: gt,
		activeSessions: make(map[string]*telemetry.GameSession),
	}
}

// HandlePlayerLogin 处理玩家登录
func (s *Server) HandlePlayerLogin(ctx context.Context, userID, gameID, platform, region string) *telemetry.GameSession {
	session := s.gameTelemetry.StartSession(ctx, userID, gameID, platform, region)
	s.activeSessions[session.ID] = session
	return session
}

// HandlePlayerLogout 处理玩家登出
func (s *Server) HandlePlayerLogout(sessionID, reason string) {
	if session, exists := s.activeSessions[sessionID]; exists {
		session.EndSession(reason)
		delete(s.activeSessions, sessionID)
	}
}

// HandleLevelStart 处理关卡开始
func (s *Server) HandleLevelStart(sessionID, levelID string, difficulty int) (context.Context, error) {
	session, exists := s.activeSessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	ctx := session.StartLevel(levelID, difficulty)
	return ctx, nil
}

// HandleLevelComplete 处理关卡完成
func (s *Server) HandleLevelComplete(ctx context.Context, sessionID, levelID string, score, earnedCoins int) error {
	session, exists := s.activeSessions[sessionID]
	if !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	session.CompleteLevel(ctx, levelID, score, earnedCoins)
	return nil
}

// HandleLevelFail 处理关卡失败
func (s *Server) HandleLevelFail(ctx context.Context, sessionID, levelID, failReason string) error {
	session, exists := s.activeSessions[sessionID]
	if !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	session.FailLevel(ctx, levelID, failReason)
	return nil
}

// HandlePurchase 处理购买
func (s *Server) HandlePurchase(sessionID, sku string, priceUSD float64) error {
	session, exists := s.activeSessions[sessionID]
	if !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	session.RecordPurchase(sku, priceUSD)
	return nil
}

// HandleClientMetrics 处理客户端指标
func (s *Server) HandleClientMetrics(sessionID string, fps, latency float64) error {
	session, exists := s.activeSessions[sessionID]
	if !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	session.RecordClientFPS(fps)
	session.RecordNetworkLatency(latency)
	return nil
}

// HandleCrash 处理崩溃事件
func (s *Server) HandleCrash(sessionID, crashReason string) error {
	session, exists := s.activeSessions[sessionID]
	if !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	session.RecordCrash(crashReason)
	// 崩溃后自动结束会话
	session.EndSession("crash")
	delete(s.activeSessions, sessionID)
	return nil
}

// SimulateGameplay 模拟游戏玩法（用于演示）
func (s *Server) SimulateGameplay(ctx context.Context, userID string, duration time.Duration) {
	// 随机选择平台和地区
	platforms := []string{"ios", "android", "windows", "web"}
	regions := []string{"cn-north", "cn-south", "us-west", "us-east", "eu-west", "ap-southeast"}

	platform := platforms[rand.Intn(len(platforms))]
	region := regions[rand.Intn(len(regions))]

	// 开始会话
	session := s.HandlePlayerLogin(ctx, userID, "example-game", platform, region)

	endTime := time.Now().Add(duration)
	levelID := 1

	for time.Now().Before(endTime) {
		// 开始关卡
		levelIDStr := fmt.Sprintf("level_%d", levelID)
		difficulty := rand.Intn(5) + 1

		levelCtx, err := s.HandleLevelStart(session.ID, levelIDStr, difficulty)
		if err != nil {
			fmt.Printf("Error starting level: %v\n", err)
			continue
		}

		// 模拟关卡游玩时间
		playTime := time.Duration(rand.Intn(30)+10) * time.Second
		time.Sleep(playTime)

		// 记录客户端指标
		fps := rand.Float64()*60 + 30  // 30-90 FPS
		latency := rand.Float64()*100 + 20  // 20-120ms 延迟
		s.HandleClientMetrics(session.ID, fps, latency)

		// 随机关卡结果
		if rand.Float64() < 0.8 { // 80% 完成率
			score := rand.Intn(1000) + 500
			coins := rand.Intn(100) + 50
			s.HandleLevelComplete(levelCtx, session.ID, levelIDStr, score, coins)
			levelID++
		} else {
			failReasons := []string{"timeout", "enemy_defeat", "player_quit", "network_error"}
			reason := failReasons[rand.Intn(len(failReasons))]
			s.HandleLevelFail(levelCtx, session.ID, levelIDStr, reason)
		}

		// 随机购买行为（5% 概率）
		if rand.Float64() < 0.05 {
			skus := []string{"coin_pack_small", "coin_pack_large", "premium_character", "remove_ads"}
			prices := []float64{0.99, 4.99, 9.99, 2.99}
			idx := rand.Intn(len(skus))
			s.HandlePurchase(session.ID, skus[idx], prices[idx])
		}

		// 随机崩溃（1% 概率）
		if rand.Float64() < 0.01 {
			crashReasons := []string{"out_of_memory", "null_pointer", "network_timeout", "graphics_error"}
			reason := crashReasons[rand.Intn(len(crashReasons))]
			s.HandleCrash(session.ID, reason)
			return // 崩溃后结束模拟
		}

		// 随机等待
		time.Sleep(time.Duration(rand.Intn(5)+1) * time.Second)
	}

	// 正常结束会话
	s.HandlePlayerLogout(session.ID, "normal")
}