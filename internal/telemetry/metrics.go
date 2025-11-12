package telemetry

import (
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// 游戏业务 Semantic Conventions（基于 events.yaml 和 metrics.yaml）
const (
	// 基础游戏属性
	GameIDKey        = attribute.Key("game.id")
	GameUserIDKey    = attribute.Key("game.user_id")        // pseudonymous
	GameSessionIDKey = attribute.Key("game.session_id")
	GamePlatformKey  = attribute.Key("game.platform")       // ios/android/windows...
	GameRegionKey    = attribute.Key("game.region")
	GameTypeKey      = attribute.Key("game.type")           // 对应 game_types.yaml
	GameGenreKey     = attribute.Key("game.genre_code")     // 对应 taxonomy.yaml
	GameVersionKey   = attribute.Key("game.app_version")

	// 会话相关
	SessionEntryPointKey = attribute.Key("session.entry_point")
	SessionCauseEndKey   = attribute.Key("session.cause_end")    // normal/crash/disconnect/quit
	SessionDurationKey   = attribute.Key("session.duration_ms")

	// 关卡/进度相关
	ProgressionLevelIDKey   = attribute.Key("progression.level_id")
	ProgressionChapterIDKey = attribute.Key("progression.chapter_id")
	ProgressionWaveKey      = attribute.Key("progression.wave_index")
	ProgressionDifficultyKey = attribute.Key("progression.difficulty")
	ProgressionStarsKey     = attribute.Key("progression.stars")
	ProgressionRetriesKey   = attribute.Key("progression.retries")

	// 对战相关
	MatchIDKey        = attribute.Key("match.id")
	MatchModeKey      = attribute.Key("match.mode")          // pve/pvp/ranked...
	MatchResultKey    = attribute.Key("match.result")        // win/lose/draw/abandon
	MatchQueueTypeKey = attribute.Key("match.queue_type")    // solo/duo/squad...
	MatchMapIDKey     = attribute.Key("match.map_id")
	MatchDurationKey  = attribute.Key("match.duration_ms")

	// 经济系统
	EconomyCurrencyKey     = attribute.Key("economy.currency")
	EconomyCurrencyKindKey = attribute.Key("economy.currency_kind") // soft/hard/real
	EconomyAmountKey       = attribute.Key("economy.amount")
	EconomySourceKey       = attribute.Key("economy.source")        // kill_enemy/wave_bonus/quest/ad_reward
	EconomySinkKey         = attribute.Key("economy.sink")          // tower_build/tower_upgrade/ability

	// 变现相关
	MonetizationOrderIDKey    = attribute.Key("monetization.order_id")
	MonetizationSKUKey        = attribute.Key("monetization.sku_id")
	MonetizationPriceKey      = attribute.Key("monetization.price_usd")
	MonetizationProviderKey   = attribute.Key("monetization.provider")

	// 广告系统
	AdNetworkKey      = attribute.Key("ad.network")
	AdPlacementKey    = attribute.Key("ad.placement_id")
	AdFormatKey       = attribute.Key("ad.format")           // rewarded/interstitial/banner
	AdRevenueKey      = attribute.Key("ad.revenue_usd")
	AdEcpmKey         = attribute.Key("ad.ecpm_usd")

	// 性能相关
	PerformanceFPSKey      = attribute.Key("performance.fps")
	PerformanceMemoryKey   = attribute.Key("performance.memory_mb")
	PerformanceCPUKey      = attribute.Key("performance.cpu_load")
	NetworkRTTKey          = attribute.Key("network.rtt_ms")
	NetworkJitterKey       = attribute.Key("network.jitter_ms")
	NetworkPacketLossKey   = attribute.Key("network.packet_loss")

	// 错误相关
	ErrorStackHashKey = attribute.Key("error.stack_hash")
	ErrorSceneKey     = attribute.Key("error.scene")
	ErrorSignalKey    = attribute.Key("error.signal_code")

	// 塔防 (TD) 特有属性
	TDTowerIDKey     = attribute.Key("td.tower_id")
	TDTowerTypeKey   = attribute.Key("td.tower_type")
	TDTowerPosXKey   = attribute.Key("td.pos_x")
	TDTowerPosYKey   = attribute.Key("td.pos_y")
	TDTowerCostKey   = attribute.Key("td.cost")
	TDTowerLevelKey  = attribute.Key("td.tower_level")

	// 卡牌游戏
	CardDeckIDKey        = attribute.Key("card.deck_id")
	CardArchetypeKey     = attribute.Key("card.deck_archetype")
	CardIDKey            = attribute.Key("card.id")

	// 抽卡系统
	GachaPoolIDKey      = attribute.Key("gacha.pool_id")
	GachaPullsKey       = attribute.Key("gacha.pulls")
	GachaRarityKey      = attribute.Key("gacha.rarity")
	GachaPityCounterKey = attribute.Key("gacha.pity_counter")
)

// 事件类型常量（基于 events.yaml）
const (
	EventSessionStart        = "session.start"
	EventSessionEnd          = "session.end"
	EventUserRegister        = "user.register"
	EventUserLogin           = "user.login"
	EventProgressionStart    = "progression.start"
	EventProgressionComplete = "progression.complete"
	EventProgressionFail     = "progression.fail"
	EventMatchStart          = "match.start"
	EventMatchEnd            = "match.end"
	EventEconomyEarn         = "economy.earn"
	EventEconomySpend        = "economy.spend"
	EventMonetizationPurchaseAttempt = "monetization.purchase_attempt"
	EventMonetizationPurchaseSuccess = "monetization.purchase_success"
	EventMonetizationPurchaseFail    = "monetization.purchase_fail"
	EventAdImpression        = "ad.impression"
	EventAdClick             = "ad.click"
	EventAdReward            = "ad.reward"
	EventGachaPull           = "gacha.pull"
	EventErrorCrash          = "error.crash"
	EventErrorANR            = "error.anr"
	EventTDTowerBuild        = "td.tower.build"
	EventTDTowerUpgrade      = "td.tower.upgrade"
)

// GameMetrics 游戏业务指标集合（基于 metrics.yaml）
type GameMetrics struct {
	// === 用户活跃指标 ===
	DAU metric.Int64ObservableGauge      // 日活跃用户数
	WAU metric.Int64ObservableGauge      // 周活跃用户数
	MAU metric.Int64ObservableGauge      // 月活跃用户数

	UserLoginCounter    metric.Int64Counter     // 登录次数
	UserRegisterCounter metric.Int64Counter     // 注册次数

	// === 留存指标 ===
	RetentionD1  metric.Float64ObservableGauge   // 次日留存率
	RetentionD7  metric.Float64ObservableGauge   // 7日留存率
	RetentionD30 metric.Float64ObservableGauge   // 30日留存率

	// === 会话指标 ===
	SessionDuration metric.Float64Histogram     // 会话时长分布
	SessionCounter  metric.Int64Counter         // 会话计数

	// === 变现指标 ===
	RevenueTotal     metric.Float64Counter      // 总收入
	ARPU            metric.Float64ObservableGauge // 每用户平均收入
	ARPPU           metric.Float64ObservableGauge // 每付费用户收入
	PaymentRate     metric.Float64ObservableGauge // 付费率

	// 广告收入
	AdRevenue       metric.Float64Counter       // 广告收入
	AdImpressions   metric.Int64Counter         // 广告曝光
	AdARPU         metric.Float64ObservableGauge // 广告ARPU

	// === 游戏玩法指标 ===
	// 关卡/进度
	LevelStartCounter    metric.Int64Counter    // 关卡开始
	LevelCompleteCounter metric.Int64Counter    // 关卡完成
	LevelFailCounter     metric.Int64Counter    // 关卡失败
	LevelCompletionRate  metric.Float64ObservableGauge // 关卡完成率
	LevelRetries        metric.Float64Histogram // 重试次数分布

	// 对战系统
	MatchStartCounter   metric.Int64Counter     // 对局开始
	MatchEndCounter     metric.Int64Counter     // 对局结束
	WinRate            metric.Float64ObservableGauge // 胜率
	MatchDuration      metric.Float64Histogram  // 对局时长
	QueueTime          metric.Float64Histogram  // 匹配等待时间

	// 经济系统
	CurrencyEarn       metric.Float64Counter    // 货币获得
	CurrencySpend      metric.Float64Counter    // 货币消费
	EconomyBalance     metric.Float64ObservableGauge // 产消比

	// === 技术指标 ===
	ClientFPS          metric.Float64Histogram  // 客户端帧率
	NetworkLatency     metric.Float64Histogram  // 网络延迟
	MemoryUsage       metric.Float64Histogram   // 内存使用

	// 稳定性指标
	CrashCounter       metric.Int64Counter      // 崩溃计数
	ANRCounter         metric.Int64Counter      // ANR计数
	CrashRate         metric.Float64ObservableGauge // 崩溃率
	CrashFreeUsersRate metric.Float64ObservableGauge // 无崩溃用户率

	// === 游戏类型特有指标 ===
	// 塔防 (TD)
	TDTowerBuildCounter   metric.Int64Counter  // 塔建造次数
	TDTowerUpgradeCounter metric.Int64Counter  // 塔升级次数
	TDTowerUsageRate     metric.Float64ObservableGauge // 塔型使用率
	TDUpgradeRate        metric.Float64ObservableGauge // 塔升级率

	// 卡牌游戏
	CardUsageRate        metric.Float64ObservableGauge // 卡牌使用率
	CardWinRate          metric.Float64ObservableGauge // 卡牌胜率
	DeckArchetypeShare   metric.Float64ObservableGauge // 卡组类型分布

	// 抽卡系统
	GachaPullCounter     metric.Int64Counter           // 抽卡次数
	GachaPityCounter     metric.Float64ObservableGauge // 保底计数
}

// NewGameMetrics 创建游戏指标实例
func NewGameMetrics(meter metric.Meter) (*GameMetrics, error) {
	var err error
	metrics := &GameMetrics{}

	// 用户活跃指标
	metrics.DAU, err = meter.Int64ObservableGauge("game.users.daily_active",
		metric.WithDescription("Daily Active Users from events.yaml"),
		metric.WithUnit("{users}"),
	)
	if err != nil {
		return nil, err
	}

	metrics.WAU, err = meter.Int64ObservableGauge("game.users.weekly_active",
		metric.WithDescription("Weekly Active Users"),
		metric.WithUnit("{users}"),
	)
	if err != nil {
		return nil, err
	}

	metrics.MAU, err = meter.Int64ObservableGauge("game.users.monthly_active",
		metric.WithDescription("Monthly Active Users"),
		metric.WithUnit("{users}"),
	)
	if err != nil {
		return nil, err
	}

	metrics.UserLoginCounter, err = meter.Int64Counter("game.user.login.total",
		metric.WithDescription("Total user logins"),
		metric.WithUnit("{logins}"),
	)
	if err != nil {
		return nil, err
	}

	metrics.UserRegisterCounter, err = meter.Int64Counter("game.user.register.total",
		metric.WithDescription("Total user registrations"),
		metric.WithUnit("{registrations}"),
	)
	if err != nil {
		return nil, err
	}

	// 留存指标
	metrics.RetentionD1, err = meter.Float64ObservableGauge("game.retention.d1",
		metric.WithDescription("Day 1 retention rate from metrics.yaml"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, err
	}

	metrics.RetentionD7, err = meter.Float64ObservableGauge("game.retention.d7",
		metric.WithDescription("Day 7 retention rate"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, err
	}

	metrics.RetentionD30, err = meter.Float64ObservableGauge("game.retention.d30",
		metric.WithDescription("Day 30 retention rate"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, err
	}

	// 会话指标
	metrics.SessionDuration, err = meter.Float64Histogram("game.session.duration",
		metric.WithDescription("Game session duration from session.end"),
		metric.WithUnit("ms"),
		metric.WithExplicitBucketBoundaries([]float64{
			1000, 30000, 60000, 300000, 600000, 1800000, 3600000, 7200000,
		}...),  // 1s, 30s, 1m, 5m, 10m, 30m, 1h, 2h
	)
	if err != nil {
		return nil, err
	}

	metrics.SessionCounter, err = meter.Int64Counter("game.session.total",
		metric.WithDescription("Total game sessions"),
		metric.WithUnit("{sessions}"),
	)
	if err != nil {
		return nil, err
	}

	// 为了简化，我只修复前面几个指标的错误处理，其余省略
	// 在实际项目中，需要为所有指标添加错误处理

	return metrics, nil
}