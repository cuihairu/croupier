package telemetry

import (
	"fmt"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"reflect"
)

// 游戏业务 Semantic Conventions（基于 events.yaml 和 metrics.yaml）
const (
	// 基础游戏属性
	GameIDKey        = attribute.Key("game.id")
	GameUserIDKey    = attribute.Key("game.user_id") // pseudonymous
	GameSessionIDKey = attribute.Key("game.session_id")
	GamePlatformKey  = attribute.Key("game.platform") // ios/android/windows...
	GameRegionKey    = attribute.Key("game.region")
	GameTypeKey      = attribute.Key("game.type")       // 对应 game_types.yaml
	GameGenreKey     = attribute.Key("game.genre_code") // 对应 taxonomy.yaml
	GameVersionKey   = attribute.Key("game.app_version")

	// 会话相关
	SessionEntryPointKey = attribute.Key("session.entry_point")
	SessionCauseEndKey   = attribute.Key("session.cause_end") // normal/crash/disconnect/quit
	SessionDurationKey   = attribute.Key("session.duration_ms")

	// 关卡/进度相关
	ProgressionLevelIDKey    = attribute.Key("progression.level_id")
	ProgressionChapterIDKey  = attribute.Key("progression.chapter_id")
	ProgressionWaveKey       = attribute.Key("progression.wave_index")
	ProgressionDifficultyKey = attribute.Key("progression.difficulty")
	ProgressionStarsKey      = attribute.Key("progression.stars")
	ProgressionRetriesKey    = attribute.Key("progression.retries")

	// 对战相关
	MatchIDKey        = attribute.Key("match.id")
	MatchModeKey      = attribute.Key("match.mode")       // pve/pvp/ranked...
	MatchResultKey    = attribute.Key("match.result")     // win/lose/draw/abandon
	MatchQueueTypeKey = attribute.Key("match.queue_type") // solo/duo/squad...
	MatchMapIDKey     = attribute.Key("match.map_id")
	MatchDurationKey  = attribute.Key("match.duration_ms")

	// 经济系统
	EconomyCurrencyKey     = attribute.Key("economy.currency")
	EconomyCurrencyKindKey = attribute.Key("economy.currency_kind") // soft/hard/real
	EconomyAmountKey       = attribute.Key("economy.amount")
	EconomySourceKey       = attribute.Key("economy.source") // kill_enemy/wave_bonus/quest/ad_reward
	EconomySinkKey         = attribute.Key("economy.sink")   // tower_build/tower_upgrade/ability

	// 变现相关
	MonetizationOrderIDKey  = attribute.Key("monetization.order_id")
	MonetizationSKUKey      = attribute.Key("monetization.sku_id")
	MonetizationPriceKey    = attribute.Key("monetization.price_usd")
	MonetizationProviderKey = attribute.Key("monetization.provider")

	// 广告系统
	AdNetworkKey   = attribute.Key("ad.network")
	AdPlacementKey = attribute.Key("ad.placement_id")
	AdFormatKey    = attribute.Key("ad.format") // rewarded/interstitial/banner
	AdRevenueKey   = attribute.Key("ad.revenue_usd")
	AdEcpmKey      = attribute.Key("ad.ecpm_usd")

	// 性能相关
	PerformanceFPSKey    = attribute.Key("performance.fps")
	PerformanceMemoryKey = attribute.Key("performance.memory_mb")
	PerformanceCPUKey    = attribute.Key("performance.cpu_load")
	NetworkRTTKey        = attribute.Key("network.rtt_ms")
	NetworkJitterKey     = attribute.Key("network.jitter_ms")
	NetworkPacketLossKey = attribute.Key("network.packet_loss")

	// 错误相关
	ErrorStackHashKey = attribute.Key("error.stack_hash")
	ErrorSceneKey     = attribute.Key("error.scene")
	ErrorSignalKey    = attribute.Key("error.signal_code")

	// 塔防 (TD) 特有属性
	TDTowerIDKey    = attribute.Key("td.tower_id")
	TDTowerTypeKey  = attribute.Key("td.tower_type")
	TDTowerPosXKey  = attribute.Key("td.pos_x")
	TDTowerPosYKey  = attribute.Key("td.pos_y")
	TDTowerCostKey  = attribute.Key("td.cost")
	TDTowerLevelKey = attribute.Key("td.tower_level")

	// 卡牌游戏
	CardDeckIDKey    = attribute.Key("card.deck_id")
	CardArchetypeKey = attribute.Key("card.deck_archetype")
	CardIDKey        = attribute.Key("card.id")

	// 抽卡系统
	GachaPoolIDKey      = attribute.Key("gacha.pool_id")
	GachaPullsKey       = attribute.Key("gacha.pulls")
	GachaRarityKey      = attribute.Key("gacha.rarity")
	GachaPityCounterKey = attribute.Key("gacha.pity_counter")
)

// 事件类型常量（基于 events.yaml）
const (
	EventSessionStart                = "session.start"
	EventSessionEnd                  = "session.end"
	EventUserRegister                = "user.register"
	EventUserLogin                   = "user.login"
	EventProgressionStart            = "progression.start"
	EventProgressionComplete         = "progression.complete"
	EventProgressionFail             = "progression.fail"
	EventMatchStart                  = "match.start"
	EventMatchEnd                    = "match.end"
	EventEconomyEarn                 = "economy.earn"
	EventEconomySpend                = "economy.spend"
	EventMonetizationPurchaseAttempt = "monetization.purchase_attempt"
	EventMonetizationPurchaseSuccess = "monetization.purchase_success"
	EventMonetizationPurchaseFail    = "monetization.purchase_fail"
	EventAdImpression                = "ad.impression"
	EventAdClick                     = "ad.click"
	EventAdReward                    = "ad.reward"
	EventGachaPull                   = "gacha.pull"
	EventErrorCrash                  = "error.crash"
	EventErrorANR                    = "error.anr"
	EventTDTowerBuild                = "td.tower.build"
	EventTDTowerUpgrade              = "td.tower.upgrade"
)

// GameMetrics 游戏业务指标集合（基于 metrics.yaml）
type GameMetrics struct {
	// === 用户活跃指标 ===
	DAU metric.Int64ObservableGauge // 日活跃用户数
	WAU metric.Int64ObservableGauge // 周活跃用户数
	MAU metric.Int64ObservableGauge // 月活跃用户数

	UserLoginCounter    metric.Int64Counter // 登录次数
	UserRegisterCounter metric.Int64Counter // 注册次数

	// === 留存指标 ===
	RetentionD1  metric.Float64ObservableGauge // 次日留存率
	RetentionD7  metric.Float64ObservableGauge // 7日留存率
	RetentionD30 metric.Float64ObservableGauge // 30日留存率

	// === 会话指标 ===
	SessionDuration metric.Float64Histogram // 会话时长分布
	SessionCounter  metric.Int64Counter     // 会话计数

	// === 变现指标 ===
	RevenueTotal metric.Float64Counter         // 总收入
	ARPU         metric.Float64ObservableGauge // 每用户平均收入
	ARPPU        metric.Float64ObservableGauge // 每付费用户收入
	PaymentRate  metric.Float64ObservableGauge // 付费率

	// 广告收入
	AdRevenue     metric.Float64Counter         // 广告收入
	AdImpressions metric.Int64Counter           // 广告曝光
	AdARPU        metric.Float64ObservableGauge // 广告ARPU

	// === 游戏玩法指标 ===
	// 关卡/进度
	LevelStartCounter    metric.Int64Counter           // 关卡开始
	LevelCompleteCounter metric.Int64Counter           // 关卡完成
	LevelFailCounter     metric.Int64Counter           // 关卡失败
	LevelCompletionRate  metric.Float64ObservableGauge // 关卡完成率
	LevelRetries         metric.Float64Histogram       // 重试次数分布

	// 对战系统
	MatchStartCounter metric.Int64Counter           // 对局开始
	MatchEndCounter   metric.Int64Counter           // 对局结束
	WinRate           metric.Float64ObservableGauge // 胜率
	MatchDuration     metric.Float64Histogram       // 对局时长
	QueueTime         metric.Float64Histogram       // 匹配等待时间

	// 经济系统
	CurrencyEarn   metric.Float64Counter         // 货币获得
	CurrencySpend  metric.Float64Counter         // 货币消费
	EconomyBalance metric.Float64ObservableGauge // 产消比

	// === 技术指标 ===
	ClientFPS      metric.Float64Histogram // 客户端帧率
	NetworkLatency metric.Float64Histogram // 网络延迟
	MemoryUsage    metric.Float64Histogram // 内存使用

	// 稳定性指标
	CrashCounter       metric.Int64Counter           // 崩溃计数
	ANRCounter         metric.Int64Counter           // ANR计数
	CrashRate          metric.Float64ObservableGauge // 崩溃率
	CrashFreeUsersRate metric.Float64ObservableGauge // 无崩溃用户率

	// === 游戏类型特有指标 ===
	// 塔防 (TD)
	TDTowerBuildCounter   metric.Int64Counter           // 塔建造次数
	TDTowerUpgradeCounter metric.Int64Counter           // 塔升级次数
	TDTowerUsageRate      metric.Float64ObservableGauge // 塔型使用率
	TDUpgradeRate         metric.Float64ObservableGauge // 塔升级率

	// 卡牌游戏
	CardUsageRate      metric.Float64ObservableGauge // 卡牌使用率
	CardWinRate        metric.Float64ObservableGauge // 卡牌胜率
	DeckArchetypeShare metric.Float64ObservableGauge // 卡组类型分布

	// 抽卡系统
	GachaPullCounter metric.Int64Counter           // 抽卡次数
	GachaPityCounter metric.Float64ObservableGauge // 保底计数
}

// NewGameMetrics 创建游戏指标实例
func NewGameMetrics(meter metric.Meter) (*GameMetrics, error) {
	m := &GameMetrics{}

	type gaugeSpec struct {
		field string
		name  string
		desc  string
		unit  string
	}
	intGauges := []gaugeSpec{
		{"DAU", "game.users.daily_active", "Daily Active Users from events.yaml", "{users}"},
		{"WAU", "game.users.weekly_active", "Weekly Active Users", "{users}"},
		{"MAU", "game.users.monthly_active", "Monthly Active Users", "{users}"},
	}
	floatGauges := []gaugeSpec{
		{"RetentionD1", "game.users.retention_d1", "D1 retention", "1"},
		{"RetentionD7", "game.users.retention_d7", "D7 retention", "1"},
		{"RetentionD30", "game.users.retention_d30", "D30 retention", "1"},
		{"ARPU", "game.revenue.arpu", "Average revenue per user", "$"},
		{"ARPPU", "game.revenue.arppu", "Average revenue per paying user", "$"},
		{"PaymentRate", "game.revenue.payment_rate", "Paying user rate", "1"},
		{"AdARPU", "game.ad.arpu", "Ad revenue per user", "$"},
		{"LevelCompletionRate", "game.level.completion_rate", "Level completion rate", "1"},
		{"WinRate", "game.match.win_rate", "Match win rate", "1"},
		{"EconomyBalance", "game.economy.balance", "Economy earn/spend balance", "1"},
		{"CrashRate", "game.stability.crash_rate", "Crash rate", "1"},
		{"CrashFreeUsersRate", "game.stability.crash_free_users_rate", "Crash-free users rate", "1"},
		{"TDTowerUsageRate", "game.td.tower_usage_rate", "TD tower usage rate", "1"},
		{"TDUpgradeRate", "game.td.upgrade_rate", "TD tower upgrade rate", "1"},
		{"CardUsageRate", "game.card.usage_rate", "Card usage rate", "1"},
		{"CardWinRate", "game.card.win_rate", "Card win rate", "1"},
		{"DeckArchetypeShare", "game.card.deck_archetype_share", "Deck archetype share", "1"},
		{"GachaPityCounter", "game.gacha.pity_count", "Gacha pity count", "1"},
	}
	intCounters := []gaugeSpec{
		{"UserLoginCounter", "game.user.login.total", "Total user logins", "{logins}"},
		{"UserRegisterCounter", "game.user.register.total", "Total user registrations", "{users}"},
		{"SessionCounter", "game.session.total", "Total game sessions", "{sessions}"},
		{"AdImpressions", "game.ad.impressions.total", "Ad impressions", "{impressions}"},
		{"LevelStartCounter", "game.level.start.total", "Level starts", "{levels}"},
		{"LevelCompleteCounter", "game.level.complete.total", "Level completions", "{levels}"},
		{"LevelFailCounter", "game.level.fail.total", "Level failures", "{levels}"},
		{"MatchStartCounter", "game.match.start.total", "Matches started", "{matches}"},
		{"MatchEndCounter", "game.match.end.total", "Matches ended", "{matches}"},
		{"CrashCounter", "game.stability.crash.total", "Crashes", "{crashes}"},
		{"ANRCounter", "game.stability.anr.total", "Application not responding events", "{anrs}"},
		{"TDTowerBuildCounter", "game.td.tower_build.total", "TD towers built", "{towers}"},
		{"TDTowerUpgradeCounter", "game.td.tower_upgrade.total", "TD towers upgraded", "{towers}"},
		{"GachaPullCounter", "game.gacha.pull.total", "Gacha pulls", "{pulls}"},
	}
	floatCounters := []gaugeSpec{
		{"RevenueTotal", "game.revenue.total", "Total revenue", "$"},
		{"AdRevenue", "game.ad.revenue.total", "Ad revenue", "$"},
		{"CurrencyEarn", "game.economy.currency_earned", "Currency earned", "{currency}"},
		{"CurrencySpend", "game.economy.currency_spent", "Currency spent", "{currency}"},
	}
	histograms := []struct {
		field  string
		name   string
		desc   string
		unit   string
		bounds []float64
	}{
		{"SessionDuration", "game.session.duration", "Session duration distribution", "s", []float64{10, 60, 300, 600, 1800, 3600, 7200, 14400}},
		{"LevelRetries", "game.level.retries", "Level retry distribution", "{retries}", []float64{1, 2, 3, 5, 10, 20}},
		{"MatchDuration", "game.match.duration", "Match duration distribution", "s", []float64{30, 60, 180, 300, 600, 1200}},
		{"QueueTime", "game.match.queue_time", "Matchmaking wait distribution", "s", []float64{1, 5, 15, 30, 60, 120, 300}},
		{"ClientFPS", "game.tech.client_fps", "Client FPS distribution", "{fps}", []float64{15, 24, 30, 45, 60, 120, 240}},
		{"NetworkLatency", "game.tech.network_latency", "Network latency distribution", "ms", []float64{10, 50, 100, 200, 500, 1000, 30000, 60000}},
		{"MemoryUsage", "game.tech.memory_usage", "Memory usage distribution", "MB", []float64{128, 256, 512, 1024, 2048, 4096}},
	}

	// 统一错误处理：任一指标创建失败即返回，杜绝部分字段为 nil 接口导致
	// 运行期方法调用 panic（此前只初始化了 10 个指标）。
	for _, spec := range intGauges {
		g, err := meter.Int64ObservableGauge(spec.name,
			metric.WithDescription(spec.desc),
			metric.WithUnit(spec.unit))
		if err != nil {
			return nil, fmt.Errorf("create gauge %s: %w", spec.name, err)
		}
		setMetricField[metric.Int64ObservableGauge](m, spec.field, g)
	}
	for _, spec := range floatGauges {
		g, err := meter.Float64ObservableGauge(spec.name,
			metric.WithDescription(spec.desc),
			metric.WithUnit(spec.unit))
		if err != nil {
			return nil, fmt.Errorf("create gauge %s: %w", spec.name, err)
		}
		setMetricField[metric.Float64ObservableGauge](m, spec.field, g)
	}
	for _, spec := range intCounters {
		c, err := meter.Int64Counter(spec.name,
			metric.WithDescription(spec.desc),
			metric.WithUnit(spec.unit))
		if err != nil {
			return nil, fmt.Errorf("create counter %s: %w", spec.name, err)
		}
		setMetricField[metric.Int64Counter](m, spec.field, c)
	}
	for _, spec := range floatCounters {
		c, err := meter.Float64Counter(spec.name,
			metric.WithDescription(spec.desc),
			metric.WithUnit(spec.unit))
		if err != nil {
			return nil, fmt.Errorf("create counter %s: %w", spec.name, err)
		}
		setMetricField[metric.Float64Counter](m, spec.field, c)
	}
	for _, h := range histograms {
		hist, err := meter.Float64Histogram(h.name,
			metric.WithDescription(h.desc),
			metric.WithUnit(h.unit),
			metric.WithExplicitBucketBoundaries(h.bounds...))
		if err != nil {
			return nil, fmt.Errorf("create histogram %s: %w", h.name, err)
		}
		setMetricField[metric.Float64Histogram](m, h.field, hist)
	}
	return m, nil
}

// setMetricField 按字段名写入指标值。NewGameMetrics 用规格表批量创建，
// 字段反射赋值保持与结构体声明一一对应，避免手写 46 段重复赋值。
func setMetricField[T any](m *GameMetrics, name string, value T) {
	rv := reflect.ValueOf(m).Elem()
	f := rv.FieldByName(name)
	if !f.IsValid() || !f.CanSet() {
		panic(fmt.Sprintf("telemetry: unknown GameMetrics field %q", name))
	}
	vt := reflect.ValueOf(value)
	// noop meter 返回具体实现而非 metric 接口，需要按字段接口类型转换。
	iv := reflect.New(f.Type()).Elem()
	if !vt.Type().AssignableTo(f.Type()) {
		ok := vt.Type().Implements(f.Type())
		if !ok {
			panic(fmt.Sprintf("telemetry: value for field %s does not implement %s", name, f.Type()))
		}
		iv.Set(vt.Convert(f.Type()))
	} else {
		iv.Set(vt)
	}
	f.Set(iv)
}
