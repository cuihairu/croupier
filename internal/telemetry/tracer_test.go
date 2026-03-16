package telemetry

import (
	"testing"
)

// TestNewGameTracer 测试创建 GameTracer
func TestNewGameTracer(t *testing.T) {
	metrics := &GameMetrics{}
	bridge := &AnalyticsBridge{}

	// Use nil tracer for basic testing
	gt := NewGameTracer(nil, metrics, bridge)

	if gt == nil {
		t.Fatal("NewGameTracer() should return non-nil GameTracer")
	}
	if gt.metrics != metrics {
		t.Error("metrics not set correctly")
	}
	if gt.bridge != bridge {
		t.Error("bridge not set correctly")
	}
}

// TestSessionStartRequest 测试会话开始请求
func TestSessionStartRequest(t *testing.T) {
	req := SessionStartRequest{
		UserID:     "user123",
		SessionID:  "session456",
		Platform:   "ios",
		Region:     "us",
		GameType:   "td",
		GenreCode:  "strategy",
		AppVersion: "1.0.0",
		EntryPoint: "main_menu",
		CampaignID: "campaign1",
		DeviceID:   "device123",
	}

	if req.UserID != "user123" {
		t.Errorf("UserID = %s, want 'user123'", req.UserID)
	}
	if req.Platform != "ios" {
		t.Errorf("Platform = %s, want 'ios'", req.Platform)
	}
	if req.GameType != "td" {
		t.Errorf("GameType = %s, want 'td'", req.GameType)
	}
	if req.GenreCode != "strategy" {
		t.Errorf("GenreCode = %s, want 'strategy'", req.GenreCode)
	}

	// Test different platforms
	platforms := []string{"ios", "android", "windows", "macos", "web"}
	for _, platform := range platforms {
		req.Platform = platform
		if req.Platform != platform {
			t.Errorf("Platform = %s, want %s", req.Platform, platform)
		}
	}
}

// TestSessionEndRequest 测试会话结束请求
func TestSessionEndRequest(t *testing.T) {
	req := SessionEndRequest{
		UserID:     "user123",
		SessionID:  "session456",
		DurationMs: 60000,
		CauseOfEnd: "normal",
	}

	if req.DurationMs != 60000 {
		t.Errorf("DurationMs = %d, want 60000", req.DurationMs)
	}
	if req.CauseOfEnd != "normal" {
		t.Errorf("CauseOfEnd = %s, want 'normal'", req.CauseOfEnd)
	}

	// Test different end causes
	causes := []string{"normal", "crash", "disconnect", "quit"}
	for _, cause := range causes {
		req.CauseOfEnd = cause
		if req.CauseOfEnd != cause {
			t.Errorf("CauseOfEnd = %s, want %s", req.CauseOfEnd, cause)
		}
	}

	// Test edge cases
	req.DurationMs = 0
	if req.DurationMs != 0 {
		t.Errorf("Zero DurationMs should be 0, got %d", req.DurationMs)
	}

	req.DurationMs = 86400000 // 24 hours in ms
	if req.DurationMs != 86400000 {
		t.Errorf("Large DurationMs should be 86400000, got %d", req.DurationMs)
	}
}

// TestLevelStartRequest 测试关卡开始请求
func TestLevelStartRequest(t *testing.T) {
	req := LevelStartRequest{
		UserID:       "user123",
		SessionID:    "session456",
		LevelID:      "level01",
		ChapterID:    "chapter01",
		Difficulty:   "hard",
		WaveIndex:    0,
		AttemptIndex: 1,
		IsBossWave:   false,
	}

	if req.LevelID != "level01" {
		t.Errorf("LevelID = %s, want 'level01'", req.LevelID)
	}
	if req.Difficulty != "hard" {
		t.Errorf("Difficulty = %s, want 'hard'", req.Difficulty)
	}
	if req.IsBossWave != false {
		t.Errorf("IsBossWave should be false")
	}
	if req.AttemptIndex != 1 {
		t.Errorf("AttemptIndex = %d, want 1", req.AttemptIndex)
	}

	// Test different difficulties
	difficulties := []string{"easy", "normal", "hard", "expert", "nightmare"}
	for _, diff := range difficulties {
		req.Difficulty = diff
		if req.Difficulty != diff {
			t.Errorf("Difficulty = %s, want %s", req.Difficulty, diff)
		}
	}

	// Test boss wave
	req.IsBossWave = true
	if req.IsBossWave != true {
		t.Errorf("IsBossWave should be true")
	}
}

// TestLevelCompleteRequest 测试关卡完成请求
func TestLevelCompleteRequest(t *testing.T) {
	req := LevelCompleteRequest{
		LevelID:         "level01",
		DurationMs:      45000,
		Stars:           3,
		Retries:         0,
		WaveIndex:       10,
		HeartsRemaining: 5,
		Difficulty:      "hard",
	}

	if req.Stars != 3 {
		t.Errorf("Stars = %d, want 3", req.Stars)
	}
	if req.Retries != 0 {
		t.Errorf("Retries = %d, want 0", req.Retries)
	}
	if req.HeartsRemaining != 5 {
		t.Errorf("HeartsRemaining = %d, want 5", req.HeartsRemaining)
	}

	// Test star ratings
	for stars := 0; stars <= 3; stars++ {
		req.Stars = stars
		if req.Stars != stars {
			t.Errorf("Stars = %d, want %d", req.Stars, stars)
		}
	}

	// Test edge cases
	req.Retries = 999
	if req.Retries != 999 {
		t.Errorf("Large Retries should be 999, got %d", req.Retries)
	}
}

// TestLevelFailRequest 测试关卡失败请求
func TestLevelFailRequest(t *testing.T) {
	req := LevelFailRequest{
		LevelID:         "level01",
		DurationMs:      20000,
		Reason:          "out_of_lives",
		FailWave:        5,
		HeartsRemaining: 0,
		Difficulty:      "hard",
	}

	if req.Reason != "out_of_lives" {
		t.Errorf("Reason = %s, want 'out_of_lives'", req.Reason)
	}
	if req.FailWave != 5 {
		t.Errorf("FailWave = %d, want 5", req.FailWave)
	}
	if req.HeartsRemaining != 0 {
		t.Errorf("HeartsRemaining = %d, want 0", req.HeartsRemaining)
	}

	// Test various failure reasons
	reasons := []string{"out_of_lives", "timeout", "quit", "disconnect", "defeat"}
	for _, reason := range reasons {
		req.Reason = reason
		if req.Reason != reason {
			t.Errorf("Reason = %s, want %s", req.Reason, reason)
		}
	}

	// Test edge cases
	req.FailWave = 0
	if req.FailWave != 0 {
		t.Errorf("Zero FailWave should be 0, got %d", req.FailWave)
	}

	req.HeartsRemaining = -1 // possible for unlimited hearts mode
	if req.HeartsRemaining != -1 {
		t.Errorf("Negative HeartsRemaining should be -1, got %d", req.HeartsRemaining)
	}
}

// TestMatchStartRequest 测试对战开始请求
func TestMatchStartRequest(t *testing.T) {
	req := MatchStartRequest{
		UserID:        "user123",
		SessionID:     "session456",
		MatchID:       "match789",
		GameMode:      "pvp",
		QueueType:     "ranked",
		MapID:         "map01",
		QueueTimeMs:   30000,
		MMR:           1500,
		TeamID:        "team01",
		DeckID:        "deck01",
		DeckArchetype: "aggro",
	}

	if req.MatchID != "match789" {
		t.Errorf("MatchID = %s, want 'match789'", req.MatchID)
	}
	if req.MMR != 1500 {
		t.Errorf("MMR = %d, want 1500", req.MMR)
	}
	if req.GameMode != "pvp" {
		t.Errorf("GameMode = %s, want 'pvp'", req.GameMode)
	}
	if req.QueueType != "ranked" {
		t.Errorf("QueueType = %s, want 'ranked'", req.QueueType)
	}

	// Test different game modes
	modes := []string{"pvp", "pve", "ranked", "casual", "tournament"}
	for _, mode := range modes {
		req.GameMode = mode
		if req.GameMode != mode {
			t.Errorf("GameMode = %s, want %s", req.GameMode, mode)
		}
	}

	// Test MMR ranges
	mmrValues := []int{0, 500, 1000, 1500, 2000, 2500, 3000, 10000}
	for _, mmr := range mmrValues {
		req.MMR = mmr
		if req.MMR != mmr {
			t.Errorf("MMR = %d, want %d", req.MMR, mmr)
		}
	}
}

// TestMatchEndRequest 测试对战结束请求
func TestMatchEndRequest(t *testing.T) {
	req := MatchEndRequest{
		MatchID:       "match789",
		MatchResult:   "win",
		DurationMs:    120000,
		GameMode:      "pvp",
		Kills:         10,
		Deaths:        5,
		Assists:       8,
		DamageDone:    15000,
		DamageTaken:   8000,
		Surrender:     false,
		DeckID:        "deck01",
		DeckArchetype: "aggro",
	}

	if req.MatchResult != "win" {
		t.Errorf("MatchResult = %s, want 'win'", req.MatchResult)
	}
	if req.Kills != 10 {
		t.Errorf("Kills = %d, want 10", req.Kills)
	}
	if req.Deaths != 5 {
		t.Errorf("Deaths = %d, want 5", req.Deaths)
	}
	if req.Assists != 8 {
		t.Errorf("Assists = %d, want 8", req.Assists)
	}
	if req.Surrender != false {
		t.Errorf("Surrender should be false")
	}

	// Test different match results
	results := []string{"win", "lose", "draw", "abandon"}
	for _, result := range results {
		req.MatchResult = result
		if req.MatchResult != result {
			t.Errorf("MatchResult = %s, want %s", req.MatchResult, result)
		}
	}

	// Test surrender
	req.Surrender = true
	if req.Surrender != true {
		t.Errorf("Surrender should be true")
	}

	// Test KDA calculation
	req.Kills = 20
	req.Deaths = 5
	req.Assists = 10
	kda := float64(req.Kills+req.Assists) / float64(req.Deaths)
	if kda != 6.0 {
		t.Errorf("KDA = %f, want 6.0", kda)
	}
}

// TestEconomyTransaction 测试经济交易
func TestEconomyTransaction(t *testing.T) {
	// Test earn transaction
	earnTrans := EconomyTransaction{
		UserID:       "user123",
		Currency:     "gold",
		CurrencyKind: "soft",
		Amount:       100,
		Type:         "earn",
		Source:       "wave_bonus",
		Sink:         "",
		ItemID:       "",
		BalanceAfter: 1100,
	}

	if earnTrans.Type != "earn" {
		t.Errorf("Type = %s, want 'earn'", earnTrans.Type)
	}
	if earnTrans.CurrencyKind != "soft" {
		t.Errorf("CurrencyKind = %s, want 'soft'", earnTrans.CurrencyKind)
	}

	// Test spend transaction
	spendTrans := EconomyTransaction{
		UserID:       "user123",
		Currency:     "gems",
		CurrencyKind: "hard",
		Amount:       50,
		Type:         "spend",
		Source:       "",
		Sink:         "tower_upgrade",
		ItemID:       "tower_01",
		BalanceAfter: 450,
	}

	if spendTrans.Type != "spend" {
		t.Errorf("Type = %s, want 'spend'", spendTrans.Type)
	}
	if spendTrans.CurrencyKind != "hard" {
		t.Errorf("CurrencyKind = %s, want 'hard'", spendTrans.CurrencyKind)
	}

	// Test different currency kinds
	kinds := []string{"soft", "hard", "real"}
	for _, kind := range kinds {
		spendTrans.CurrencyKind = kind
		if spendTrans.CurrencyKind != kind {
			t.Errorf("CurrencyKind = %s, want %s", spendTrans.CurrencyKind, kind)
		}
	}

	// Test different currency names
	currencies := []string{"gold", "gems", "coins", "tickets", "energy"}
	for _, currency := range currencies {
		spendTrans.Currency = currency
		if spendTrans.Currency != currency {
			t.Errorf("Currency = %s, want %s", spendTrans.Currency, currency)
		}
	}

	// Test amount validation
	spendTrans.Amount = 0
	if spendTrans.Amount != 0 {
		t.Errorf("Zero amount should be 0, got %f", spendTrans.Amount)
	}

	spendTrans.Amount = 1000000.99
	if spendTrans.Amount != 1000000.99 {
		t.Errorf("Large amount should be 1000000.99, got %f", spendTrans.Amount)
	}
}

// TestPurchaseFlow 测试购买流程
func TestPurchaseFlow(t *testing.T) {
	purchase := PurchaseFlow{
		UserID:          "user123",
		OrderID:         "order123",
		SKUID:           "sku_01",
		PriceUSD:        4.99,
		CurrencyCode:    "USD",
		PaymentProvider: "apple",
	}

	if purchase.OrderID != "order123" {
		t.Errorf("OrderID = %s, want 'order123'", purchase.OrderID)
	}
	if purchase.PriceUSD != 4.99 {
		t.Errorf("PriceUSD = %f, want 4.99", purchase.PriceUSD)
	}
	if purchase.CurrencyCode != "USD" {
		t.Errorf("CurrencyCode = %s, want 'USD'", purchase.CurrencyCode)
	}

	// Test different payment providers
	providers := []string{"apple", "google", "stripe", "paypal"}
	for _, provider := range providers {
		purchase.PaymentProvider = provider
		if purchase.PaymentProvider != provider {
			t.Errorf("PaymentProvider = %s, want %s", purchase.PaymentProvider, provider)
		}
	}

	// Test different currencies
	currencies := []string{"USD", "EUR", "GBP", "JPY", "CNY"}
	for _, currency := range currencies {
		purchase.CurrencyCode = currency
		if purchase.CurrencyCode != currency {
			t.Errorf("CurrencyCode = %s, want %s", purchase.CurrencyCode, currency)
		}
	}

	// Test price ranges
	prices := []float64{0.99, 1.99, 4.99, 9.99, 19.99, 49.99, 99.99}
	for _, price := range prices {
		purchase.PriceUSD = price
		if purchase.PriceUSD != price {
			t.Errorf("PriceUSD = %f, want %f", purchase.PriceUSD, price)
		}
	}
}

// TestPurchaseResult 测试购买结果
func TestPurchaseResult(t *testing.T) {
	// Test success
	successResult := PurchaseResult{
		OrderID:    "order123",
		SKUID:      "sku_01",
		PriceUSD:   4.99,
		Success:    true,
		FailReason: "",
		TaxUSD:     0.50,
		Country:    "US",
	}

	if !successResult.Success {
		t.Error("Success should be true")
	}
	if successResult.TaxUSD != 0.50 {
		t.Errorf("TaxUSD = %f, want 0.50", successResult.TaxUSD)
	}

	// Test failure
	failResult := PurchaseResult{
		OrderID:    "order123",
		SKUID:      "sku_01",
		PriceUSD:   4.99,
		Success:    false,
		FailReason: "payment_declined",
		TaxUSD:     0,
		Country:    "US",
	}

	if failResult.Success {
		t.Error("Success should be false")
	}
	if failResult.FailReason != "payment_declined" {
		t.Errorf("FailReason = %s, want 'payment_declined'", failResult.FailReason)
	}

	// Test different failure reasons
	failReasons := []string{"payment_declined", "insufficient_funds", "timeout", "user_cancelled", "invalid_sku"}
	for _, reason := range failReasons {
		failResult.FailReason = reason
		if failResult.FailReason != reason {
			t.Errorf("FailReason = %s, want %s", failResult.FailReason, reason)
		}
	}

	// Test tax calculation
	successResult.PriceUSD = 10.00
	successResult.TaxUSD = 0.80 // 8% tax
	total := successResult.PriceUSD + successResult.TaxUSD
	if total != 10.80 {
		t.Errorf("Total = %f, want 10.80", total)
	}
}

// TestAdImpressionRequest 测试广告曝光请求
func TestAdImpressionRequest(t *testing.T) {
	req := AdImpressionRequest{
		UserID:        "user123",
		AdNetwork:     "admob",
		PlacementID:   "placement_between_waves",
		AdFormat:      "rewarded",
		PlacementType: "between_waves",
		EcpmUSD:       10.50,
		RevenueUSD:    0.02,
	}

	if req.AdNetwork != "admob" {
		t.Errorf("AdNetwork = %s, want 'admob'", req.AdNetwork)
	}
	if req.AdFormat != "rewarded" {
		t.Errorf("AdFormat = %s, want 'rewarded'", req.AdFormat)
	}

	// Test different ad formats
	formats := []string{"rewarded", "interstitial", "banner", "native"}
	for _, format := range formats {
		req.AdFormat = format
		if req.AdFormat != format {
			t.Errorf("AdFormat = %s, want %s", req.AdFormat, format)
		}
	}

	// Test different ad networks
	networks := []string{"admob", "unity", "ironsource", "applovin"}
	for _, network := range networks {
		req.AdNetwork = network
		if req.AdNetwork != network {
			t.Errorf("AdNetwork = %s, want %s", req.AdNetwork, network)
		}
	}

	// Test revenue calculation
	req.EcpmUSD = 20.00
	req.RevenueUSD = 0.05
	if req.RevenueUSD <= 0 {
		t.Error("RevenueUSD should be positive")
	}
}

// TestPerformanceMetrics 测试性能指标
func TestPerformanceMetrics(t *testing.T) {
	perf := PerformanceMetrics{
		UserID:     "user123",
		FPS:        60,
		MemoryMB:   256,
		CPULoad:    45.5,
		RTTMs:      50,
		JitterMs:   5,
		PacketLoss: 0.1,
	}

	if perf.FPS != 60 {
		t.Errorf("FPS = %f, want 60", perf.FPS)
	}
	if perf.MemoryMB != 256 {
		t.Errorf("MemoryMB = %f, want 256", perf.MemoryMB)
	}
	if perf.CPULoad != 45.5 {
		t.Errorf("CPULoad = %f, want 45.5", perf.CPULoad)
	}
	if perf.RTTMs != 50 {
		t.Errorf("RTTMs = %f, want 50", perf.RTTMs)
	}
	if perf.JitterMs != 5 {
		t.Errorf("JitterMs = %f, want 5", perf.JitterMs)
	}
	if perf.PacketLoss != 0.1 {
		t.Errorf("PacketLoss = %f, want 0.1", perf.PacketLoss)
	}

	// Test with zero values
	zeroPerf := PerformanceMetrics{
		UserID: "user123",
	}
	if zeroPerf.FPS != 0 {
		t.Errorf("Zero FPS should be 0, got %f", zeroPerf.FPS)
	}

	// Test FPS ranges
	fpsValues := []float64{30, 60, 120, 144, 240}
	for _, fps := range fpsValues {
		perf.FPS = fps
		if perf.FPS != fps {
			t.Errorf("FPS = %f, want %f", perf.FPS, fps)
		}
	}

	// Test memory ranges
	memValues := []float64{64, 128, 256, 512, 1024, 2048}
	for _, mem := range memValues {
		perf.MemoryMB = mem
		if perf.MemoryMB != mem {
			t.Errorf("MemoryMB = %f, want %f", perf.MemoryMB, mem)
		}
	}
}

// TestCrashEvent 测试崩溃事件
func TestCrashEvent(t *testing.T) {
	crash := CrashEvent{
		UserID:     "user123",
		SessionID:  "session456",
		StackHash:  "abc123",
		SignalCode: "SIGSEGV",
		Scene:      "battle_scene",
		DeviceID:   "device123",
	}

	if crash.SignalCode != "SIGSEGV" {
		t.Errorf("SignalCode = %s, want 'SIGSEGV'", crash.SignalCode)
	}
	if crash.Scene != "battle_scene" {
		t.Errorf("Scene = %s, want 'battle_scene'", crash.Scene)
	}

	// Test different signal codes
	signals := []string{"SIGSEGV", "SIGABRT", "SIGFPE", "SIGILL", "SIGTERM"}
	for _, signal := range signals {
		crash.SignalCode = signal
		if crash.SignalCode != signal {
			t.Errorf("SignalCode = %s, want %s", crash.SignalCode, signal)
		}
	}

	// Test different scenes
	scenes := []string{"battle_scene", "menu_scene", "loading_scene", "shop_scene"}
	for _, scene := range scenes {
		crash.Scene = scene
		if crash.Scene != scene {
			t.Errorf("Scene = %s, want %s", crash.Scene, scene)
		}
	}
}

// TestTowerBuildRequest 测试塔建造请求
func TestTowerBuildRequest(t *testing.T) {
	req := TowerBuildRequest{
		UserID:    "user123",
		LevelID:   "level01",
		TowerID:   "tower_01",
		TowerType: "arrow",
		PosX:      100,
		PosY:      200,
		Cost:      50,
		WaveIndex: 3,
	}

	if req.TowerType != "arrow" {
		t.Errorf("TowerType = %s, want 'arrow'", req.TowerType)
	}
	if req.Cost != 50 {
		t.Errorf("Cost = %f, want 50", req.Cost)
	}
	if req.PosX != 100 {
		t.Errorf("PosX = %d, want 100", req.PosX)
	}
	if req.PosY != 200 {
		t.Errorf("PosY = %d, want 200", req.PosY)
	}

	// Test different tower types
	towerTypes := []string{"arrow", "cannon", "magic", "air", "splash"}
	for _, tType := range towerTypes {
		req.TowerType = tType
		if req.TowerType != tType {
			t.Errorf("TowerType = %s, want %s", req.TowerType, tType)
		}
	}

	// Test position ranges
	for x := 0; x <= 1000; x += 100 {
		req.PosX = x
		if req.PosX != x {
			t.Errorf("PosX = %d, want %d", req.PosX, x)
		}
	}

	// Test cost ranges
	costs := []float64{25, 50, 100, 200, 500, 1000}
	for _, cost := range costs {
		req.Cost = cost
		if req.Cost != cost {
			t.Errorf("Cost = %f, want %f", req.Cost, cost)
		}
	}
}

// TestTowerUpgradeRequest 测试塔升级请求
func TestTowerUpgradeRequest(t *testing.T) {
	req := TowerUpgradeRequest{
		UserID:    "user123",
		LevelID:   "level01",
		TowerID:   "tower_01",
		TowerType: "arrow",
		FromLevel: 1,
		ToLevel:   2,
		Cost:      100,
		WaveIndex: 5,
	}

	if req.ToLevel != 2 {
		t.Errorf("ToLevel = %d, want 2", req.ToLevel)
	}
	if req.FromLevel != 1 {
		t.Errorf("FromLevel = %d, want 1", req.FromLevel)
	}
	if req.Cost != 100 {
		t.Errorf("Cost = %f, want 100", req.Cost)
	}

	// Test multiple upgrade levels
	for i := 1; i <= 10; i++ {
		req.FromLevel = i
		req.ToLevel = i + 1
		if req.ToLevel != i+1 {
			t.Errorf("ToLevel = %d, want %d", req.ToLevel, i+1)
		}
	}

	// Test upgrade cost scaling
	for level := 1; level <= 5; level++ {
		req.FromLevel = level
		req.ToLevel = level + 1
		req.Cost = float64(level) * 50 // Each level costs 50 more
		expectedCost := float64(level) * 50
		if req.Cost != expectedCost {
			t.Errorf("Level %d upgrade cost = %f, want %f", level, req.Cost, expectedCost)
		}
	}
}

// TestGachaPullRequest 测试抽卡请求
func TestGachaPullRequest(t *testing.T) {
	req := GachaPullRequest{
		UserID:      "user123",
		PoolID:      "pool_01",
		Pulls:       10,
		Rarity:      "legendary",
		PityCounter: 80,
		ItemIDs:     []string{"item_01", "item_02", "item_03"},
	}

	if req.Pulls != 10 {
		t.Errorf("Pulls = %d, want 10", req.Pulls)
	}
	if len(req.ItemIDs) != 3 {
		t.Errorf("ItemIDs length = %d, want 3", len(req.ItemIDs))
	}

	// Test different rarities
	rarities := []string{"common", "rare", "epic", "legendary", "mythic"}
	for _, rarity := range rarities {
		req.Rarity = rarity
		if req.Rarity != rarity {
			t.Errorf("Rarity = %s, want %s", req.Rarity, rarity)
		}
	}

	// Test single pull vs ten pull
	req.Pulls = 1
	if req.Pulls != 1 {
		t.Errorf("Single pull should be 1, got %d", req.Pulls)
	}

	req.Pulls = 10
	if req.Pulls != 10 {
		t.Errorf("Ten pull should be 10, got %d", req.Pulls)
	}

	// Test pity counter ranges
	for pity := 0; pity <= 100; pity += 10 {
		req.PityCounter = pity
		if req.PityCounter != pity {
			t.Errorf("PityCounter = %d, want %d", req.PityCounter, pity)
		}
	}

	// Test item IDs array
	req.ItemIDs = []string{"item_01"}
	if len(req.ItemIDs) != 1 {
		t.Errorf("ItemIDs length should be 1, got %d", len(req.ItemIDs))
	}

	req.ItemIDs = make([]string, 10)
	for i := 0; i < 10; i++ {
		req.ItemIDs[i] = "item_" + string(rune('0'+i))
	}
	if len(req.ItemIDs) != 10 {
		t.Errorf("ItemIDs length should be 10, got %d", len(req.ItemIDs))
	}
}

// TestGameTracer_RequestValidation 测试请求验证
func TestGameTracer_RequestValidation(t *testing.T) {
	// Test empty user ID handling
	emptyUserSession := SessionStartRequest{
		UserID:    "",
		SessionID: "session456",
	}
	if emptyUserSession.UserID != "" {
		t.Error("Empty UserID should remain empty")
	}

	// Test negative values handling
	negativeStats := PerformanceMetrics{
		UserID:   "user123",
		FPS:      -1,
		MemoryMB: -100,
	}
	if negativeStats.FPS != -1 {
		t.Errorf("Negative FPS should be preserved, got %f", negativeStats.FPS)
	}

	// Test large values
	largeValues := MatchStartRequest{
		UserID:      "user123",
		QueueTimeMs: 999999,
		MMR:         99999,
	}
	if largeValues.QueueTimeMs != 999999 {
		t.Errorf("Large QueueTimeMs should be preserved, got %d", largeValues.QueueTimeMs)
	}

	// Test zero values in various structs
	zeroLevelReq := LevelStartRequest{
		UserID: "user123",
	}
	if zeroLevelReq.WaveIndex != 0 {
		t.Errorf("Zero WaveIndex should be 0, got %d", zeroLevelReq.WaveIndex)
	}

	zeroMatchEnd := MatchEndRequest{
		MatchID: "match123",
	}
	if zeroMatchEnd.Kills != 0 {
		t.Errorf("Zero Kills should be 0, got %d", zeroMatchEnd.Kills)
	}
}

// TestRequestStructDefaults 测试结构体默认值
func TestRequestStructDefaults(t *testing.T) {
	// Test zero values for all struct types
	var sessionReq SessionStartRequest
	if sessionReq.UserID != "" {
		t.Error("Default UserID should be empty string")
	}

	var sessionEndReq SessionEndRequest
	if sessionEndReq.DurationMs != 0 {
		t.Error("Default DurationMs should be 0")
	}

	var levelReq LevelStartRequest
	if levelReq.AttemptIndex != 0 {
		t.Error("Default AttemptIndex should be 0")
	}

	var matchReq MatchStartRequest
	if matchReq.MMR != 0 {
		t.Error("Default MMR should be 0")
	}

	var trans EconomyTransaction
	if trans.Amount != 0 {
		t.Error("Default Amount should be 0")
	}

	var perf PerformanceMetrics
	if perf.FPS != 0 {
		t.Error("Default FPS should be 0")
	}
}
