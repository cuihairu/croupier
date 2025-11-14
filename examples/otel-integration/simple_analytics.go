package telemetry

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// SimpleAnalytics 极简游戏分析SDK - 5分钟集成
type SimpleAnalytics struct {
	gameID      string
	serverURL   string
	httpClient  *http.Client
	batchQueue  []map[string]interface{}
	batchMutex  sync.Mutex
	flushTicker *time.Ticker
	stopChannel chan struct{}
}

// SimpleConfig 简化配置
type SimpleConfig struct {
	GameID    string `json:"game_id"`    // 游戏ID，必填
	ServerURL string `json:"server_url"` // Croupier Server地址，如 "http://localhost:8080"
	BatchSize int    `json:"batch_size"` // 批量大小，默认10
	FlushSec  int    `json:"flush_sec"`  // 刷新间隔秒数，默认5
}

// NewSimpleAnalytics 创建极简分析实例
func NewSimpleAnalytics(config SimpleConfig) *SimpleAnalytics {
	if config.BatchSize == 0 {
		config.BatchSize = 10
	}
	if config.FlushSec == 0 {
		config.FlushSec = 5
	}

	analytics := &SimpleAnalytics{
		gameID:    config.GameID,
		serverURL: config.ServerURL,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
		batchQueue:  make([]map[string]interface{}, 0, config.BatchSize),
		stopChannel: make(chan struct{}),
	}

	// 启动定时刷新
	analytics.flushTicker = time.NewTicker(time.Duration(config.FlushSec) * time.Second)
	go analytics.flushRoutine()

	return analytics
}

// 定时刷新协程
func (s *SimpleAnalytics) flushRoutine() {
	for {
		select {
		case <-s.flushTicker.C:
			s.flush()
		case <-s.stopChannel:
			s.flush() // 最后一次刷新
			return
		}
	}
}

// TrackEvent 追踪游戏事件 - 核心方法
func (s *SimpleAnalytics) TrackEvent(userID string, event string, properties map[string]interface{}) {
	if properties == nil {
		properties = make(map[string]interface{})
	}

	eventData := map[string]interface{}{
		"game_id":  s.gameID,
		"user_id":  userID,
		"event":    event,
		"ts":       time.Now().Format(time.RFC3339),
		"props":    properties,
	}

	s.batchMutex.Lock()
	s.batchQueue = append(s.batchQueue, eventData)
	shouldFlush := len(s.batchQueue) >= cap(s.batchQueue)
	s.batchMutex.Unlock()

	if shouldFlush {
		s.flush()
	}
}

// 便捷方法集合 - 游戏常用事件

// UserLogin 用户登录
func (s *SimpleAnalytics) UserLogin(userID string, platform string, region string) {
	s.TrackEvent(userID, "login", map[string]interface{}{
		"platform": platform,
		"region":   region,
	})
}

// UserRegister 用户注册
func (s *SimpleAnalytics) UserRegister(userID string, platform string, region string, channel string) {
	s.TrackEvent(userID, "register", map[string]interface{}{
		"platform": platform,
		"region":   region,
		"channel":  channel,
	})
}

// SessionStart 会话开始
func (s *SimpleAnalytics) SessionStart(userID string, sessionID string, platform string) {
	s.TrackEvent(userID, "session_start", map[string]interface{}{
		"session_id": sessionID,
		"platform":   platform,
	})
}

// SessionEnd 会话结束
func (s *SimpleAnalytics) SessionEnd(userID string, sessionID string, duration int64) {
	s.TrackEvent(userID, "session_end", map[string]interface{}{
		"session_id":    sessionID,
		"duration_sec":  duration,
	})
}

// LevelStart 关卡开始
func (s *SimpleAnalytics) LevelStart(userID string, sessionID string, level string, episode string) {
	s.TrackEvent(userID, "level_start", map[string]interface{}{
		"session_id": sessionID,
		"level":      level,
		"episode":    episode,
	})
}

// LevelComplete 关卡完成
func (s *SimpleAnalytics) LevelComplete(userID string, sessionID string, level string, duration int64, retries int, score int64) {
	s.TrackEvent(userID, "level_complete", map[string]interface{}{
		"session_id":   sessionID,
		"level":        level,
		"duration_sec": duration,
		"retries":      retries,
		"score":        score,
	})
}

// LevelFail 关卡失败
func (s *SimpleAnalytics) LevelFail(userID string, sessionID string, level string, reason string, progress float64) {
	s.TrackEvent(userID, "level_fail", map[string]interface{}{
		"session_id": sessionID,
		"level":      level,
		"reason":     reason,
		"progress":   progress,
	})
}

// Purchase 内购事件
func (s *SimpleAnalytics) Purchase(userID string, orderID string, productID string, amount float64, currency string, success bool) {
	s.TrackEvent(userID, "purchase", map[string]interface{}{
		"order_id":   orderID,
		"product_id": productID,
		"amount":     amount,
		"currency":   currency,
		"success":    success,
	})
}

// AdImpression 广告展示
func (s *SimpleAnalytics) AdImpression(userID string, adID string, adType string, placement string, revenue float64) {
	s.TrackEvent(userID, "ad_impression", map[string]interface{}{
		"ad_id":     adID,
		"ad_type":   adType,
		"placement": placement,
		"revenue":   revenue,
	})
}

// EconomyEarn 游戏内货币获得
func (s *SimpleAnalytics) EconomyEarn(userID string, currency string, amount float64, source string) {
	s.TrackEvent(userID, "economy_earn", map[string]interface{}{
		"currency": currency,
		"amount":   amount,
		"source":   source,
	})
}

// EconomySpend 游戏内货币消费
func (s *SimpleAnalytics) EconomySpend(userID string, currency string, amount float64, item string, category string) {
	s.TrackEvent(userID, "economy_spend", map[string]interface{}{
		"currency": currency,
		"amount":   amount,
		"item":     item,
		"category": category,
	})
}

// SocialShare 社交分享
func (s *SimpleAnalytics) SocialShare(userID string, platform string, content string) {
	s.TrackEvent(userID, "social_share", map[string]interface{}{
		"platform": platform,
		"content":  content,
	})
}

// ErrorReport 错误报告
func (s *SimpleAnalytics) ErrorReport(userID string, errorType string, message string, stackTrace string) {
	s.TrackEvent(userID, "error", map[string]interface{}{
		"error_type":   errorType,
		"message":      message,
		"stack_trace":  stackTrace,
	})
}

// 刷新批量事件到服务器
func (s *SimpleAnalytics) flush() {
	s.batchMutex.Lock()
	if len(s.batchQueue) == 0 {
		s.batchMutex.Unlock()
		return
	}

	// 复制当前批次
	batch := make([]map[string]interface{}, len(s.batchQueue))
	copy(batch, s.batchQueue)
	s.batchQueue = s.batchQueue[:0] // 清空队列
	s.batchMutex.Unlock()

	// 发送到服务器
	s.sendBatch(batch)
}

// 发送批次到服务器
func (s *SimpleAnalytics) sendBatch(batch []map[string]interface{}) {
	jsonData, err := json.Marshal(batch)
	if err != nil {
		fmt.Printf("SimpleAnalytics: Failed to marshal events: %v\n", err)
		return
	}

	url := fmt.Sprintf("%s/api/analytics/ingest", s.serverURL)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		fmt.Printf("SimpleAnalytics: Failed to create request: %v\n", err)
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Game-ID", s.gameID)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		fmt.Printf("SimpleAnalytics: Failed to send events: %v\n", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 202 {
		fmt.Printf("SimpleAnalytics: Server returned status %d\n", resp.StatusCode)
	}
}

// Close 关闭分析实例
func (s *SimpleAnalytics) Close() {
	if s.flushTicker != nil {
		s.flushTicker.Stop()
	}
	close(s.stopChannel)
	s.flush() // 最后一次刷新
}

// === 便捷的全局实例模式 ===

var globalAnalytics *SimpleAnalytics

// Init 初始化全局分析实例
func Init(config SimpleConfig) {
	globalAnalytics = NewSimpleAnalytics(config)
}

// Track 使用全局实例追踪事件
func Track(userID string, event string, properties map[string]interface{}) {
	if globalAnalytics != nil {
		globalAnalytics.TrackEvent(userID, event, properties)
	}
}

// 全局便捷方法
func Login(userID, platform, region string) {
	if globalAnalytics != nil {
		globalAnalytics.UserLogin(userID, platform, region)
	}
}

func Register(userID, platform, region, channel string) {
	if globalAnalytics != nil {
		globalAnalytics.UserRegister(userID, platform, region, channel)
	}
}

func StartLevel(userID, sessionID, level, episode string) {
	if globalAnalytics != nil {
		globalAnalytics.LevelStart(userID, sessionID, level, episode)
	}
}

func CompleteLevel(userID, sessionID, level string, duration int64, retries int, score int64) {
	if globalAnalytics != nil {
		globalAnalytics.LevelComplete(userID, sessionID, level, duration, retries, score)
	}
}

func FailLevel(userID, sessionID, level, reason string, progress float64) {
	if globalAnalytics != nil {
		globalAnalytics.LevelFail(userID, sessionID, level, reason, progress)
	}
}

func Buy(userID, orderID, productID string, amount float64, currency string, success bool) {
	if globalAnalytics != nil {
		globalAnalytics.Purchase(userID, orderID, productID, amount, currency, success)
	}
}

func ShowAd(userID, adID, adType, placement string, revenue float64) {
	if globalAnalytics != nil {
		globalAnalytics.AdImpression(userID, adID, adType, placement, revenue)
	}
}

func EarnCurrency(userID, currency string, amount float64, source string) {
	if globalAnalytics != nil {
		globalAnalytics.EconomyEarn(userID, currency, amount, source)
	}
}

func SpendCurrency(userID, currency string, amount float64, item, category string) {
	if globalAnalytics != nil {
		globalAnalytics.EconomySpend(userID, currency, amount, item, category)
	}
}

func ReportError(userID, errorType, message, stackTrace string) {
	if globalAnalytics != nil {
		globalAnalytics.ErrorReport(userID, errorType, message, stackTrace)
	}
}

// Shutdown 关闭全局实例
func Shutdown() {
	if globalAnalytics != nil {
		globalAnalytics.Close()
		globalAnalytics = nil
	}
}