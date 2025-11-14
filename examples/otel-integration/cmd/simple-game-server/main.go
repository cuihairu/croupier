package main

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/cuihairu/croupier/examples/otel-integration/internal/telemetry"
)

// 🎮 极简游戏服务器示例 - 5分钟集成Analytics
func main() {
	fmt.Println("🚀 启动极简游戏服务器...")

	// 1. 初始化Analytics（5行代码搞定！）
	telemetry.Init(telemetry.SimpleConfig{
		GameID:    "simple-game-demo",
		ServerURL: "http://localhost:8080", // Croupier Server地址
		BatchSize: 5,                       // 5个事件一批
		FlushSec:  3,                       // 3秒强制刷新
	})

	// 确保程序结束时发送剩余事件
	defer telemetry.Shutdown()

	fmt.Println("✅ Analytics初始化完成")

	// 2. 模拟游戏流程
	simulateGameplay()
}

func simulateGameplay() {
	fmt.Println("🎭 开始模拟游戏会话...")

	// 模拟5个用户的游戏流程
	for i := 1; i <= 5; i++ {
		userID := fmt.Sprintf("player_%d", i)
		sessionID := fmt.Sprintf("session_%d_%d", i, time.Now().Unix())

		// 用户注册/登录
		if rand.Float64() < 0.3 { // 30%新用户
			telemetry.Register(userID, "android", "cn-north", "organic")
			fmt.Printf("👤 新用户注册: %s\n", userID)
		} else {
			telemetry.Login(userID, "android", "cn-north")
			fmt.Printf("🔓 用户登录: %s\n", userID)
		}

		// 会话开始
		sessionStartTime := time.Now()
		fmt.Printf("🎯 会话开始: %s\n", sessionID)

		// 游戏玩法流程
		playGameSession(userID, sessionID)

		// 会话结束
		sessionDuration := int64(time.Since(sessionStartTime).Seconds())
		fmt.Printf("⏰ 会话结束: %s，时长: %d秒\n", sessionID, sessionDuration)

		// 添加随机延迟
		time.Sleep(time.Duration(rand.Intn(2)+1) * time.Second)
	}

	fmt.Println("🏁 游戏模拟完成，等待事件发送...")
	time.Sleep(5 * time.Second) // 等待最后的事件发送
}

func playGameSession(userID, sessionID string) {
	// 游玩3个关卡
	episodes := []string{"tutorial", "forest", "desert"}

	for i, episode := range episodes {
		levelID := fmt.Sprintf("level_%d_%d", i+1, rand.Intn(5)+1)

		// 关卡开始
		telemetry.StartLevel(userID, sessionID, levelID, episode)
		fmt.Printf("  🏁 关卡开始: %s (%s)\n", levelID, episode)

		// 模拟游戏时长
		playDuration := rand.Intn(30) + 10 // 10-40秒
		time.Sleep(time.Duration(playDuration) * time.Millisecond * 10) // 加速模拟

		// 游戏过程中的事件
		simulateGameplayEvents(userID, sessionID, levelID, episode)

		// 关卡结果
		if rand.Float64() < 0.7 { // 70%通过率
			retries := rand.Intn(3)
			score := rand.Intn(1000) + 500
			telemetry.CompleteLevel(userID, sessionID, levelID, int64(playDuration), retries, int64(score))
			fmt.Printf("  ✅ 关卡完成: %s，得分: %d\n", levelID, score)
		} else {
			reason := []string{"timeout", "enemy_defeat", "fall_down"}[rand.Intn(3)]
			progress := rand.Float64() * 0.8 + 0.1 // 10%-90%
			telemetry.FailLevel(userID, sessionID, levelID, reason, progress)
			fmt.Printf("  ❌ 关卡失败: %s，原因: %s\n", levelID, reason)
		}
	}

	// 会话中的其他事件
	simulateSessionEvents(userID, sessionID)
}

func simulateGameplayEvents(userID, sessionID, levelID, episode string) {
	// 游戏内货币获得
	if rand.Float64() < 0.8 {
		coins := float64(rand.Intn(100) + 20)
		telemetry.EarnCurrency(userID, "coins", coins, "level_reward")
		fmt.Printf("  💰 获得金币: %.0f\n", coins)
	}

	// 游戏内货币消费
	if rand.Float64() < 0.4 {
		cost := float64(rand.Intn(50) + 10)
		items := []string{"health_potion", "power_up", "extra_life"}
		item := items[rand.Intn(len(items))]
		telemetry.SpendCurrency(userID, "coins", cost, item, "consumable")
		fmt.Printf("  💸 消费金币: %.0f 购买 %s\n", cost, item)
	}

	// 偶尔的错误事件
	if rand.Float64() < 0.05 { // 5%概率
		errorTypes := []string{"network_timeout", "texture_load_fail", "physics_error"}
		errorType := errorTypes[rand.Intn(len(errorTypes))]
		telemetry.ReportError(userID, errorType, "Simulated game error", "stack_trace_here")
		fmt.Printf("  🐛 错误事件: %s\n", errorType)
	}
}

func simulateSessionEvents(userID, sessionID string) {
	// 内购事件
	if rand.Float64() < 0.15 { // 15%付费率
		orderID := fmt.Sprintf("order_%d", time.Now().UnixNano())
		products := []struct {
			id    string
			price float64
		}{
			{"coin_pack_small", 0.99},
			{"coin_pack_large", 4.99},
			{"premium_pass", 9.99},
			{"character_skin", 2.99},
		}

		product := products[rand.Intn(len(products))]
		success := rand.Float64() < 0.9 // 90%支付成功率

		telemetry.Buy(userID, orderID, product.id, product.price, "USD", success)

		status := "成功"
		if !success {
			status = "失败"
		}
		fmt.Printf("  💳 内购%s: %s ($%.2f)\n", status, product.id, product.price)
	}

	// 广告展示
	if rand.Float64() < 0.6 { // 60%用户看广告
		adID := fmt.Sprintf("ad_%d", time.Now().UnixNano()%10000)
		adTypes := []string{"rewarded", "interstitial", "banner"}
		adType := adTypes[rand.Intn(len(adTypes))]
		placements := []string{"level_complete", "main_menu", "pause_menu"}
		placement := placements[rand.Intn(len(placements))]
		revenue := rand.Float64() * 0.05 // $0-0.05 eCPM

		telemetry.ShowAd(userID, adID, adType, placement, revenue)
		fmt.Printf("  📺 广告展示: %s位置的%s广告\n", placement, adType)
	}

	// 社交分享（偶尔）
	if rand.Float64() < 0.1 { // 10%分享率
		platforms := []string{"wechat", "weibo", "facebook", "twitter"}
		platform := platforms[rand.Intn(len(platforms))]
		content := "刚刚完成了一个超难关卡！"

		telemetry.Track(userID, "social_share", map[string]interface{}{
			"platform": platform,
			"content":  content,
		})
		fmt.Printf("  📱 社交分享到: %s\n", platform)
	}
}

// === 游戏特定事件示例 ===

// 玩家对战事件
func trackPVPMatch(userID, opponentID string, result string, duration int64) {
	telemetry.Track(userID, "pvp_match", map[string]interface{}{
		"opponent_id": opponentID,
		"result":      result, // "win", "lose", "draw"
		"duration":    duration,
		"mode":        "ranked",
	})
}

// 公会事件
func trackGuildActivity(userID, guildID, activity string) {
	telemetry.Track(userID, "guild_activity", map[string]interface{}{
		"guild_id": guildID,
		"activity": activity, // "join", "leave", "donate", "chat"
	})
}

// 道具强化事件
func trackItemUpgrade(userID, itemID string, fromLevel, toLevel int, success bool) {
	telemetry.Track(userID, "item_upgrade", map[string]interface{}{
		"item_id":    itemID,
		"from_level": fromLevel,
		"to_level":   toLevel,
		"success":    success,
	})
}

// 排行榜事件
func trackLeaderboard(userID string, category string, rank int, score int64) {
	telemetry.Track(userID, "leaderboard", map[string]interface{}{
		"category": category, // "global", "weekly", "friends"
		"rank":     rank,
		"score":    score,
	})
}