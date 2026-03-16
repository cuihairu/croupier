package telemetry

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

// TestNewGameMetrics 测试创建游戏指标
func TestNewGameMetrics(t *testing.T) {
	// 使用 noop meter provider 创建 meter
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	meter := provider.Meter("test")

	metrics, err := NewGameMetrics(meter)
	if err != nil {
		t.Fatalf("NewGameMetrics() error = %v", err)
	}

	if metrics == nil {
		t.Fatal("NewGameMetrics() should return non-nil metrics")
	}

	// 验证用户活跃指标
	if metrics.DAU == nil {
		t.Error("DAU should be initialized")
	}
	if metrics.WAU == nil {
		t.Error("WAU should be initialized")
	}
	if metrics.MAU == nil {
		t.Error("MAU should be initialized")
	}

	// 验证登录指标
	if metrics.UserLoginCounter == nil {
		t.Error("UserLoginCounter should be initialized")
	}
	if metrics.UserRegisterCounter == nil {
		t.Error("UserRegisterCounter should be initialized")
	}

	// 验证留存指标
	if metrics.RetentionD1 == nil {
		t.Error("RetentionD1 should be initialized")
	}
	if metrics.RetentionD7 == nil {
		t.Error("RetentionD7 should be initialized")
	}
	if metrics.RetentionD30 == nil {
		t.Error("RetentionD30 should be initialized")
	}

	// 验证会话指标
	if metrics.SessionDuration == nil {
		t.Error("SessionDuration should be initialized")
	}
	if metrics.SessionCounter == nil {
		t.Error("SessionCounter should be initialized")
	}

	// 清理
	provider.Shutdown(context.Background())
}

// TestGameMetrics_Counters 测试计数器指标
func TestGameMetrics_Counters(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	meter := provider.Meter("test")
	metrics, _ := NewGameMetrics(meter)
	defer provider.Shutdown(context.Background())

	ctx := context.Background()

	// 测试 UserLoginCounter - 不应该 panic
	if metrics.UserLoginCounter != nil {
		metrics.UserLoginCounter.Add(ctx, 1, metric.WithAttributes(
			GamePlatformKey.String("ios"),
			GameRegionKey.String("us"),
		))
	}

	// 测试 UserRegisterCounter
	if metrics.UserRegisterCounter != nil {
		metrics.UserRegisterCounter.Add(ctx, 1, metric.WithAttributes(
			GamePlatformKey.String("android"),
		))
	}

	// 测试 SessionCounter
	if metrics.SessionCounter != nil {
		metrics.SessionCounter.Add(ctx, 1, metric.WithAttributes())
	}
}

// TestGameMetrics_Float64Counters 测试浮点计数器
func TestGameMetrics_Float64Counters(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	meter := provider.Meter("test")
	metrics, _ := NewGameMetrics(meter)
	defer provider.Shutdown(context.Background())

	ctx := context.Background()

	// 测试 CurrencyEarn
	if metrics.CurrencyEarn != nil {
		metrics.CurrencyEarn.Add(ctx, 100.0, metric.WithAttributes(
			EconomyCurrencyKey.String("gold"),
			EconomySourceKey.String("wave_bonus"),
			EconomyCurrencyKindKey.String("soft"),
		))
	}

	// 测试 CurrencySpend
	if metrics.CurrencySpend != nil {
		metrics.CurrencySpend.Add(ctx, 50.0, metric.WithAttributes(
			EconomyCurrencyKey.String("gems"),
			EconomySinkKey.String("tower_upgrade"),
			EconomyCurrencyKindKey.String("hard"),
		))
	}
}

// TestGameMetrics_Histograms 测试直方图指标
func TestGameMetrics_Histograms(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	meter := provider.Meter("test")
	metrics, _ := NewGameMetrics(meter)
	defer provider.Shutdown(context.Background())

	ctx := context.Background()

	// 测试 SessionDuration
	if metrics.SessionDuration != nil {
		metrics.SessionDuration.Record(ctx, 60000, metric.WithAttributes(
			SessionCauseEndKey.String("normal"),
		))
	}

	// 测试 LevelRetries
	if metrics.LevelRetries != nil {
		metrics.LevelRetries.Record(ctx, 2.0, metric.WithAttributes(
			ProgressionLevelIDKey.String("level01"),
		))
	}

	// 测试 ClientFPS
	if metrics.ClientFPS != nil {
		metrics.ClientFPS.Record(ctx, 60.0, metric.WithAttributes(
			GameUserIDKey.String("user123"),
		))
	}

	// 测试 MemoryUsage
	if metrics.MemoryUsage != nil {
		metrics.MemoryUsage.Record(ctx, 256.0, metric.WithAttributes(
			GameUserIDKey.String("user123"),
		))
	}
}

// TestGameMetrics_Int64Counters 测试整型计数器
func TestGameMetrics_Int64Counters(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	meter := provider.Meter("test")
	metrics, _ := NewGameMetrics(meter)
	defer provider.Shutdown(context.Background())

	ctx := context.Background()

	// 测试 LevelStartCounter
	if metrics.LevelStartCounter != nil {
		metrics.LevelStartCounter.Add(ctx, 1, metric.WithAttributes(
			ProgressionLevelIDKey.String("level01"),
			ProgressionDifficultyKey.String("hard"),
		))
	}

	// 测试 LevelCompleteCounter
	if metrics.LevelCompleteCounter != nil {
		metrics.LevelCompleteCounter.Add(ctx, 1, metric.WithAttributes(
			ProgressionLevelIDKey.String("level01"),
			ProgressionDifficultyKey.String("hard"),
		))
	}

	// 测试 CrashCounter
	if metrics.CrashCounter != nil {
		metrics.CrashCounter.Add(ctx, 1, metric.WithAttributes(
			ErrorSceneKey.String("battle_scene"),
			attribute.String("device_id", "device123"),
		))
	}
}

// TestAttributeKeys 测试属性键常量
func TestAttributeKeys(t *testing.T) {
	tests := []struct {
		key   attribute.Key
		value string
	}{
		{GameIDKey, "game.id"},
		{GameUserIDKey, "game.user_id"},
		{GameSessionIDKey, "game.session_id"},
		{GamePlatformKey, "game.platform"},
		{GameRegionKey, "game.region"},
		{GameTypeKey, "game.type"},
		{GameGenreKey, "game.genre_code"},
		{GameVersionKey, "game.app_version"},
		{SessionEntryPointKey, "session.entry_point"},
		{SessionCauseEndKey, "session.cause_end"},
		{SessionDurationKey, "session.duration_ms"},
		{ProgressionLevelIDKey, "progression.level_id"},
		{ProgressionChapterIDKey, "progression.chapter_id"},
		{ProgressionWaveKey, "progression.wave_index"},
		{ProgressionDifficultyKey, "progression.difficulty"},
		{ProgressionStarsKey, "progression.stars"},
		{ProgressionRetriesKey, "progression.retries"},
		{MatchIDKey, "match.id"},
		{MatchModeKey, "match.mode"},
		{MatchResultKey, "match.result"},
		{MatchQueueTypeKey, "match.queue_type"},
		{MatchMapIDKey, "match.map_id"},
		{MatchDurationKey, "match.duration_ms"},
		{EconomyCurrencyKey, "economy.currency"},
		{EconomyCurrencyKindKey, "economy.currency_kind"},
		{EconomyAmountKey, "economy.amount"},
		{EconomySourceKey, "economy.source"},
		{EconomySinkKey, "economy.sink"},
		{MonetizationOrderIDKey, "monetization.order_id"},
		{MonetizationSKUKey, "monetization.sku_id"},
		{MonetizationPriceKey, "monetization.price_usd"},
		{MonetizationProviderKey, "monetization.provider"},
		{AdNetworkKey, "ad.network"},
		{AdPlacementKey, "ad.placement_id"},
		{AdFormatKey, "ad.format"},
		{AdRevenueKey, "ad.revenue_usd"},
		{AdEcpmKey, "ad.ecpm_usd"},
		{PerformanceFPSKey, "performance.fps"},
		{PerformanceMemoryKey, "performance.memory_mb"},
		{PerformanceCPUKey, "performance.cpu_load"},
		{NetworkRTTKey, "network.rtt_ms"},
		{NetworkJitterKey, "network.jitter_ms"},
		{NetworkPacketLossKey, "network.packet_loss"},
		{ErrorStackHashKey, "error.stack_hash"},
		{ErrorSceneKey, "error.scene"},
		{ErrorSignalKey, "error.signal_code"},
		{TDTowerIDKey, "td.tower_id"},
		{TDTowerTypeKey, "td.tower_type"},
		{TDTowerPosXKey, "td.pos_x"},
		{TDTowerPosYKey, "td.pos_y"},
		{TDTowerCostKey, "td.cost"},
		{TDTowerLevelKey, "td.tower_level"},
		{CardDeckIDKey, "card.deck_id"},
		{CardArchetypeKey, "card.deck_archetype"},
		{CardIDKey, "card.id"},
		{GachaPoolIDKey, "gacha.pool_id"},
		{GachaPullsKey, "gacha.pulls"},
		{GachaRarityKey, "gacha.rarity"},
		{GachaPityCounterKey, "gacha.pity_counter"},
	}

	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			if string(tt.key) != tt.value {
				t.Errorf("Attribute key = %s, want %s", tt.key, tt.value)
			}
		})
	}
}

// TestEventTypeConstants 测试事件类型常量
func TestEventTypeConstants(t *testing.T) {
	constants := []string{
		EventSessionStart,
		EventSessionEnd,
		EventUserRegister,
		EventUserLogin,
		EventProgressionStart,
		EventProgressionComplete,
		EventProgressionFail,
		EventMatchStart,
		EventMatchEnd,
		EventEconomyEarn,
		EventEconomySpend,
		EventMonetizationPurchaseAttempt,
		EventMonetizationPurchaseSuccess,
		EventMonetizationPurchaseFail,
		EventAdImpression,
		EventAdClick,
		EventAdReward,
		EventGachaPull,
		EventErrorCrash,
		EventErrorANR,
		EventTDTowerBuild,
		EventTDTowerUpgrade,
	}

	expectedValues := []string{
		"session.start",
		"session.end",
		"user.register",
		"user.login",
		"progression.start",
		"progression.complete",
		"progression.fail",
		"match.start",
		"match.end",
		"economy.earn",
		"economy.spend",
		"monetization.purchase_attempt",
		"monetization.purchase_success",
		"monetization.purchase_fail",
		"ad.impression",
		"ad.click",
		"ad.reward",
		"gacha.pull",
		"error.crash",
		"error.anr",
		"td.tower.build",
		"td.tower.upgrade",
	}

	for i, c := range constants {
		if c != expectedValues[i] {
			t.Errorf("Event constant = %s, want %s", c, expectedValues[i])
		}
	}
}

// TestGameMetrics_ObservableInstruments 测试可观测仪器
func TestGameMetrics_ObservableInstruments(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	meter := provider.Meter("test")
	metrics, _ := NewGameMetrics(meter)
	defer provider.Shutdown(context.Background())

	// 验证初始化的可观测仪器
	if metrics.DAU == nil {
		t.Error("DAU should be initialized")
	}
	if metrics.WAU == nil {
		t.Error("WAU should be initialized")
	}
	if metrics.MAU == nil {
		t.Error("MAU should be initialized")
	}
	if metrics.RetentionD1 == nil {
		t.Error("RetentionD1 should be initialized")
	}
	if metrics.RetentionD7 == nil {
		t.Error("RetentionD7 should be initialized")
	}
	if metrics.RetentionD30 == nil {
		t.Error("RetentionD30 should be initialized")
	}
}
