package telemetry

import (
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// 补充游戏特定指标，对应 configs/analytics 中的特殊指标

// ExtendedGameMetrics 扩展的游戏指标，覆盖 analytics 配置中的特殊指标
type ExtendedGameMetrics struct {
	// === 塔防 (TD) 特有指标 ===
	TDWaveFailByWave     metric.Int64Counter           // 各波次失败计数
	TDHeartsRemaining    metric.Float64ObservableGauge // 平均剩余生命
	TDWaveStartCounter   metric.Int64Counter           // 波次开始计数

	// === 卡牌游戏特有指标 ===
	DeckArchetypeWinRate metric.Float64ObservableGauge // 卡组原型胜率
	RoundDuration        metric.Float64Histogram       // 回合时长
	RoundStartCounter    metric.Int64Counter           // 回合开始计数
	RoundEndCounter      metric.Int64Counter           // 回合结束计数

	// === 经济系统扩展指标 ===
	OfflineIncomeShare   metric.Float64ObservableGauge // 离线收益占比
	EconomyBalanceGauge  metric.Float64ObservableGauge // 经济产消比实时值
	CurrencyBySource     metric.Float64Counter         // 按来源的货币获得
	CurrencyBySink       metric.Float64Counter         // 按去向的货币消费

	// === 抽卡系统扩展指标 ===
	GachaPityCounter     metric.Float64ObservableGauge // 平均保底计数
	GachaRareDropRate    metric.Float64ObservableGauge // 稀有掉落率
	GachaPoolParticipation metric.Int64Counter         // 卡池参与度

	// === 棋牌/桌游特有指标 ===
	WinRateBySeat        metric.Float64ObservableGauge // 按座位胜率
	RakeRate             metric.Float64ObservableGauge // 抽水率
	AFKLeaveRate         metric.Float64ObservableGauge // 中途离场率
	PotSize              metric.Float64Histogram       // 底池大小分布

	// === 战斗系统扩展指标 ===
	AccuracyByWeapon     metric.Float64ObservableGauge // 按武器命中率
	KDAByHero           metric.Float64ObservableGauge // 按英雄KDA
	DamagePerMatch      metric.Float64Histogram       // 单局伤害分布

	// === 社交系统指标 ===
	GuildJoinRate        metric.Float64ObservableGauge // 公会加入率
	GuildActiveRate      metric.Float64ObservableGauge // 公会活跃度
	FriendInteractionRate metric.Float64ObservableGauge // 好友互动率

	// === 性能和体验指标 ===
	SessionQuality       metric.Float64ObservableGauge // 会话质量评分
	LoadingTimeByScreen  metric.Float64Histogram       // 按屏幕加载时间
	UIResponseTime       metric.Float64Histogram       // UI响应时间
}

// NewExtendedGameMetrics 创建扩展游戏指标
func NewExtendedGameMetrics(meter metric.Meter) (*ExtendedGameMetrics, error) {
	var err error
	metrics := &ExtendedGameMetrics{}

	// === 塔防特有指标 ===
	metrics.TDWaveFailByWave, err = meter.Int64Counter("game.td.wave.fail.total",
		metric.WithDescription("Tower Defense wave failure count by wave index"),
		metric.WithUnit("{failures}"),
	)
	if err != nil {
		return nil, err
	}

	metrics.TDHeartsRemaining, err = meter.Float64ObservableGauge("game.td.hearts.remaining.avg",
		metric.WithDescription("Average hearts remaining when completing TD level"),
		metric.WithUnit("{hearts}"),
	)
	if err != nil {
		return nil, err
	}

	metrics.TDWaveStartCounter, err = meter.Int64Counter("game.td.wave.start.total",
		metric.WithDescription("Tower Defense wave start count"),
		metric.WithUnit("{waves}"),
	)
	if err != nil {
		return nil, err
	}

	// === 卡牌游戏指标 ===
	metrics.DeckArchetypeWinRate, err = meter.Float64ObservableGauge("game.card.deck_archetype.win_rate",
		metric.WithDescription("Win rate by deck archetype"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, err
	}

	metrics.RoundDuration, err = meter.Float64Histogram("game.card.round.duration",
		metric.WithDescription("Card game round duration"),
		metric.WithUnit("ms"),
		metric.WithExplicitBucketBoundaries(
			500, 1000, 5000, 10000, 30000, 60000, 120000, 300000,
		), // 0.5s to 5m
	)
	if err != nil {
		return nil, err
	}

	metrics.RoundStartCounter, err = meter.Int64Counter("game.card.round.start.total",
		metric.WithDescription("Card game round start count"),
		metric.WithUnit("{rounds}"),
	)
	if err != nil {
		return nil, err
	}

	// === 经济系统扩展 ===
	metrics.OfflineIncomeShare, err = meter.Float64ObservableGauge("game.economy.offline_income.share",
		metric.WithDescription("Share of offline income vs total income"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, err
	}

	metrics.EconomyBalanceGauge, err = meter.Float64ObservableGauge("game.economy.balance.ratio",
		metric.WithDescription("Economy balance ratio (earn/spend)"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, err
	}

	metrics.CurrencyBySource, err = meter.Float64Counter("game.economy.earn.by_source",
		metric.WithDescription("Currency earned by source type"),
		metric.WithUnit("{currency}"),
	)
	if err != nil {
		return nil, err
	}

	metrics.CurrencyBySink, err = meter.Float64Counter("game.economy.spend.by_sink",
		metric.WithDescription("Currency spent by sink type"),
		metric.WithUnit("{currency}"),
	)
	if err != nil {
		return nil, err
	}

	// === 抽卡系统扩展 ===
	metrics.GachaPityCounter, err = meter.Float64ObservableGauge("game.gacha.pity.counter.avg",
		metric.WithDescription("Average gacha pity counter"),
		metric.WithUnit("{pulls}"),
	)
	if err != nil {
		return nil, err
	}

	metrics.GachaRareDropRate, err = meter.Float64ObservableGauge("game.gacha.rare_drop.rate",
		metric.WithDescription("Rare drop rate in gacha"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, err
	}

	metrics.GachaPoolParticipation, err = meter.Int64Counter("game.gacha.pool.participation.total",
		metric.WithDescription("Gacha pool participation count"),
		metric.WithUnit("{participations}"),
	)
	if err != nil {
		return nil, err
	}

	// === 棋牌/桌游指标 ===
	metrics.WinRateBySeat, err = meter.Float64ObservableGauge("game.board.win_rate.by_seat",
		metric.WithDescription("Win rate by seat position"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, err
	}

	metrics.RakeRate, err = meter.Float64ObservableGauge("game.board.rake.rate",
		metric.WithDescription("Rake rate (commission/pot)"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, err
	}

	metrics.AFKLeaveRate, err = meter.Float64ObservableGauge("game.match.afk_leave.rate",
		metric.WithDescription("AFK/leave rate in matches"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, err
	}

	metrics.PotSize, err = meter.Float64Histogram("game.board.pot.size",
		metric.WithDescription("Pot size distribution"),
		metric.WithUnit("{currency}"),
		metric.WithExplicitBucketBoundaries(
			10, 50, 100, 500, 1000, 5000, 10000, 50000, 100000,
		),
	)
	if err != nil {
		return nil, err
	}

	// === 战斗系统扩展 ===
	metrics.AccuracyByWeapon, err = meter.Float64ObservableGauge("game.combat.accuracy.by_weapon",
		metric.WithDescription("Accuracy rate by weapon type"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, err
	}

	metrics.KDAByHero, err = meter.Float64ObservableGauge("game.combat.kda.by_hero",
		metric.WithDescription("KDA by hero/character"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, err
	}

	metrics.DamagePerMatch, err = meter.Float64Histogram("game.combat.damage.per_match",
		metric.WithDescription("Damage dealt per match"),
		metric.WithUnit("{damage}"),
		metric.WithExplicitBucketBoundaries(
			100, 500, 1000, 2500, 5000, 10000, 25000, 50000, 100000,
		),
	)
	if err != nil {
		return nil, err
	}

	// === 社交系统指标 ===
	metrics.GuildJoinRate, err = meter.Float64ObservableGauge("game.social.guild.join_rate",
		metric.WithDescription("Guild join rate (joins/invites)"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, err
	}

	metrics.GuildActiveRate, err = meter.Float64ObservableGauge("game.social.guild.active_rate",
		metric.WithDescription("Guild active member rate"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, err
	}

	metrics.FriendInteractionRate, err = meter.Float64ObservableGauge("game.social.friend.interaction_rate",
		metric.WithDescription("Friend interaction rate"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, err
	}

	// === 性能和体验指标 ===
	metrics.SessionQuality, err = meter.Float64ObservableGauge("game.session.quality.score",
		metric.WithDescription("Session quality score (0-100)"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, err
	}

	metrics.LoadingTimeByScreen, err = meter.Float64Histogram("game.ui.loading_time.by_screen",
		metric.WithDescription("Loading time by screen type"),
		metric.WithUnit("ms"),
		metric.WithExplicitBucketBoundaries(
			100, 250, 500, 1000, 2500, 5000, 10000, 30000,
		),
	)
	if err != nil {
		return nil, err
	}

	metrics.UIResponseTime, err = meter.Float64Histogram("game.ui.response_time",
		metric.WithDescription("UI response time for user interactions"),
		metric.WithUnit("ms"),
		metric.WithExplicitBucketBoundaries(
			10, 25, 50, 100, 250, 500, 1000, 2500,
		),
	)
	if err != nil {
		return nil, err
	}

	return metrics, nil
}

// === 预定义的属性键，对应 configs/analytics 中的维度 ===

// 塔防属性
var (
	TDWaveIndexKey     = attribute.Key("td.wave_index")
	TDTowerLevelKey    = attribute.Key("td.tower_level")
	TDHeartsLeftKey    = attribute.Key("td.hearts_remaining")
	TDBossWaveKey      = attribute.Key("td.is_boss_wave")
)

// 卡牌游戏属性
var (
	CardDeckArchetypeKey = attribute.Key("card.deck_archetype")
	CardDeckIDKey        = attribute.Key("card.deck_id")
	CardIDKey            = attribute.Key("card.id")
	CardRarityKey        = attribute.Key("card.rarity")
	RoundIDKey           = attribute.Key("round.id")
	GameVariantKey       = attribute.Key("game.variant")
	StakesKey            = attribute.Key("game.stakes")
)

// 经济系统属性
var (
	EconomySourceKey      = attribute.Key("economy.source")       // kill_enemy, wave_bonus, quest, ad_reward, offline
	EconomySinkKey        = attribute.Key("economy.sink")         // tower_build, tower_upgrade, ability, unlock
	CurrencyKindKey       = attribute.Key("economy.currency_kind") // soft, hard, real
	OfflineIndicatorKey   = attribute.Key("economy.is_offline")
)

// 抽卡系统属性
var (
	GachaPoolIDKey      = attribute.Key("gacha.pool_id")
	GachaPityKey        = attribute.Key("gacha.pity_counter")
	GachaRarityKey      = attribute.Key("gacha.rarity")
	GachaPullsKey       = attribute.Key("gacha.pulls")
)

// 棋牌/桌游属性
var (
	SeatIDKey           = attribute.Key("seat.id")
	TableIDKey          = attribute.Key("table.id")
	RakeAmountKey       = attribute.Key("rake.amount")
	PotSizeKey          = attribute.Key("pot.size")
	LeaveReasonKey      = attribute.Key("leave.reason") // normal, afk, disconnect, rage_quit
)

// 战斗系统属性
var (
	WeaponIDKey         = attribute.Key("weapon.id")
	HeroIDKey           = attribute.Key("hero.id")
	DamageTypeKey       = attribute.Key("damage.type")
	TargetTypeKey       = attribute.Key("target.type")
)

// 社交系统属性
var (
	GuildIDKey          = attribute.Key("guild.id")
	GuildLevelKey       = attribute.Key("guild.level")
	FriendIDKey         = attribute.Key("friend.id")
	InteractionTypeKey  = attribute.Key("interaction.type")
)

// 性能和体验属性
var (
	ScreenNameKey       = attribute.Key("screen.name")
	LoadingTypeKey      = attribute.Key("loading.type")
	QualityScoreKey     = attribute.Key("quality.score")
	UIElementKey        = attribute.Key("ui.element")
)