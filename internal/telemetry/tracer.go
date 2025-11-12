package telemetry

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// GameTracer 游戏事件追踪器
type GameTracer struct {
	tracer  trace.Tracer
	metrics *GameMetrics
	bridge  *AnalyticsBridge
}

// NewGameTracer 创建游戏追踪器
func NewGameTracer(tracer trace.Tracer, metrics *GameMetrics, bridge *AnalyticsBridge) *GameTracer {
	return &GameTracer{
		tracer:  tracer,
		metrics: metrics,
		bridge:  bridge,
	}
}

// === 会话管理 ===

// SessionStartRequest 会话开始请求
type SessionStartRequest struct {
	UserID      string
	SessionID   string
	Platform    string
	Region      string
	GameType    string
	GenreCode   string
	AppVersion  string
	EntryPoint  string
	CampaignID  string
	DeviceID    string
}

// StartUserSession 开始用户会话（顶级 Trace）
func (t *GameTracer) StartUserSession(ctx context.Context, req SessionStartRequest) (context.Context, trace.Span) {
	// 创建新的 Trace（每个游戏会话一个 Trace）
	ctx, span := t.tracer.Start(ctx, EventSessionStart,
		trace.WithSpanKind(trace.SpanKindConsumer),
		trace.WithAttributes(
			GameUserIDKey.String(req.UserID),
			GameSessionIDKey.String(req.SessionID),
			GamePlatformKey.String(req.Platform),
			GameRegionKey.String(req.Region),
			GameTypeKey.String(req.GameType),
			GameGenreKey.String(req.GenreCode),
			GameVersionKey.String(req.AppVersion),
			SessionEntryPointKey.String(req.EntryPoint),
		),
	)

	// 记录会话开始指标
	t.metrics.SessionCounter.Add(ctx, 1, metric.WithAttributes(
		GamePlatformKey.String(req.Platform),
		GameRegionKey.String(req.Region),
		GameTypeKey.String(req.GameType),
	))

	// 记录用户登录指标
	t.metrics.UserLoginCounter.Add(ctx, 1, metric.WithAttributes(
		GamePlatformKey.String(req.Platform),
		GameRegionKey.String(req.Region),
	))

	// 发送Analytics事件
	if t.bridge != nil {
		t.bridge.SendSessionEvent(ctx, EventSessionStart, span, req.UserID, req.SessionID, req.Platform, req.Region, map[string]interface{}{
			"game_type":    req.GameType,
			"genre_code":   req.GenreCode,
			"app_version":  req.AppVersion,
			"entry_point":  req.EntryPoint,
			"campaign_id":  req.CampaignID,
			"device_id":    req.DeviceID,
		})
	}

	return ctx, span
}

// SessionEndRequest 会话结束请求
type SessionEndRequest struct {
	UserID      string
	SessionID   string
	DurationMs  int64
	CauseOfEnd  string // normal/crash/disconnect/quit
}

// EndUserSession 结束用户会话
func (t *GameTracer) EndUserSession(ctx context.Context, req SessionEndRequest) {
	span := trace.SpanFromContext(ctx)
	if span != nil {
		span.SetName(EventSessionEnd)
		span.SetAttributes(
			SessionDurationKey.Int64(req.DurationMs),
			SessionCauseEndKey.String(req.CauseOfEnd),
		)

		// 记录会话时长分布
		t.metrics.SessionDuration.Record(ctx, float64(req.DurationMs), metric.WithAttributes(
			SessionCauseEndKey.String(req.CauseOfEnd),
		))

		// 发送Analytics事件
		if t.bridge != nil {
			t.bridge.SendSessionEvent(ctx, EventSessionEnd, span, req.UserID, req.SessionID, "", "", map[string]interface{}{
				"duration_ms": req.DurationMs,
				"cause_end":   req.CauseOfEnd,
			})
		}

		if req.CauseOfEnd == "normal" {
			span.SetStatus(codes.Ok, "Session ended normally")
		} else {
			span.SetStatus(codes.Error, fmt.Sprintf("Session ended: %s", req.CauseOfEnd))
		}

		span.End()
	}
}

// === 关卡游玩链路 ===

// LevelStartRequest 关卡开始请求
type LevelStartRequest struct {
	UserID       string
	SessionID    string
	LevelID      string
	ChapterID    string
	StageID      string
	Difficulty   string
	WaveIndex    int
	AttemptIndex int
	IsBossWave   bool
}

// StartLevelPlaythrough 开始关卡游玩链路
func (t *GameTracer) StartLevelPlaythrough(ctx context.Context, req LevelStartRequest) (context.Context, trace.Span) {
	ctx, span := t.tracer.Start(ctx, EventProgressionStart,
		trace.WithAttributes(
			GameUserIDKey.String(req.UserID),
			GameSessionIDKey.String(req.SessionID),
			ProgressionLevelIDKey.String(req.LevelID),
			ProgressionChapterIDKey.String(req.ChapterID),
			ProgressionDifficultyKey.String(req.Difficulty),
			ProgressionWaveKey.Int(req.WaveIndex),
			attribute.Int("progression.attempt_index", req.AttemptIndex),
			attribute.Bool("progression.is_boss_wave", req.IsBossWave),
		),
	)

	t.metrics.LevelStartCounter.Add(ctx, 1, metric.WithAttributes(
		ProgressionLevelIDKey.String(req.LevelID),
		ProgressionDifficultyKey.String(req.Difficulty),
	))

	return ctx, span
}

// LevelCompleteRequest 关卡完成请求
type LevelCompleteRequest struct {
	LevelID         string
	DurationMs      int64
	Stars           int
	Retries         int
	WaveIndex       int
	HeartsRemaining int
	Difficulty      string
}

// CompleteLevelPlaythrough 完成关卡游玩
func (t *GameTracer) CompleteLevelPlaythrough(ctx context.Context, result LevelCompleteRequest) {
	span := trace.SpanFromContext(ctx)
	if span != nil {
		span.SetName(EventProgressionComplete)
		span.SetAttributes(
			ProgressionStarsKey.Int(result.Stars),
			ProgressionRetriesKey.Int(result.Retries),
			attribute.Int64("progression.duration_ms", result.DurationMs),
			ProgressionWaveKey.Int(result.WaveIndex),
			attribute.Int("progression.hearts_remaining", result.HeartsRemaining),
			attribute.Bool("progression.success", true),
		)

		// 记录关卡完成指标
		t.metrics.LevelCompleteCounter.Add(ctx, 1, metric.WithAttributes(
			ProgressionLevelIDKey.String(result.LevelID),
			ProgressionDifficultyKey.String(result.Difficulty),
		))

		// 记录重试次数分布
		t.metrics.LevelRetries.Record(ctx, float64(result.Retries), metric.WithAttributes(
			ProgressionLevelIDKey.String(result.LevelID),
		))

		// 发送Analytics事件
		if t.bridge != nil {
			t.bridge.SendProgressionEvent(ctx, EventProgressionComplete, span, "", "", result.LevelID, map[string]interface{}{
				"duration_ms":       result.DurationMs,
				"stars":             result.Stars,
				"retries":           result.Retries,
				"wave_index":        result.WaveIndex,
				"hearts_remaining":  result.HeartsRemaining,
				"difficulty":        result.Difficulty,
			})
		}

		span.SetStatus(codes.Ok, "Level completed successfully")
		span.End()
	}
}

// LevelFailRequest 关卡失败请求
type LevelFailRequest struct {
	LevelID         string
	DurationMs      int64
	Reason          string
	FailWave        int
	HeartsRemaining int
	Difficulty      string
}

// FailLevelPlaythrough 关卡失败
func (t *GameTracer) FailLevelPlaythrough(ctx context.Context, result LevelFailRequest) {
	span := trace.SpanFromContext(ctx)
	if span != nil {
		span.SetName(EventProgressionFail)
		span.SetAttributes(
			attribute.String("progression.fail_reason", result.Reason),
			ProgressionWaveKey.Int(result.FailWave),
			attribute.Int64("progression.duration_ms", result.DurationMs),
			attribute.Int("progression.hearts_remaining", result.HeartsRemaining),
			attribute.Bool("progression.success", false),
		)

		t.metrics.LevelFailCounter.Add(ctx, 1, metric.WithAttributes(
			ProgressionLevelIDKey.String(result.LevelID),
			attribute.String("fail_reason", result.Reason),
			ProgressionDifficultyKey.String(result.Difficulty),
		))

		span.SetStatus(codes.Error, fmt.Sprintf("Level failed: %s", result.Reason))
		span.End()
	}
}

// === 对战系统链路 ===

// MatchStartRequest 对战开始请求
type MatchStartRequest struct {
	UserID       string
	SessionID    string
	MatchID      string
	GameMode     string
	QueueType    string
	MapID        string
	QueueTimeMs  int
	MMR          int
	TeamID       string
	DeckID       string
	DeckArchetype string
}

// StartMatch 开始对战
func (t *GameTracer) StartMatch(ctx context.Context, req MatchStartRequest) (context.Context, trace.Span) {
	ctx, span := t.tracer.Start(ctx, EventMatchStart,
		trace.WithAttributes(
			GameUserIDKey.String(req.UserID),
			GameSessionIDKey.String(req.SessionID),
			MatchIDKey.String(req.MatchID),
			MatchModeKey.String(req.GameMode),
			MatchQueueTypeKey.String(req.QueueType),
			MatchMapIDKey.String(req.MapID),
			attribute.Int("match.queue_time_ms", req.QueueTimeMs),
			attribute.Int("match.mmr", req.MMR),
			CardDeckIDKey.String(req.DeckID),
			CardArchetypeKey.String(req.DeckArchetype),
		),
	)

	// 记录匹配等待时间
	t.metrics.QueueTime.Record(ctx, float64(req.QueueTimeMs), metric.WithAttributes(
		MatchQueueTypeKey.String(req.QueueType),
		MatchModeKey.String(req.GameMode),
	))

	t.metrics.MatchStartCounter.Add(ctx, 1, metric.WithAttributes(
		MatchModeKey.String(req.GameMode),
	))

	return ctx, span
}

// MatchEndRequest 对战结束请求
type MatchEndRequest struct {
	MatchID       string
	MatchResult   string // win/lose/draw/abandon
	DurationMs    int64
	GameMode      string
	Kills         int
	Deaths        int
	Assists       int
	DamageDone    int
	DamageTaken   int
	Surrender     bool
	DeckID        string
	DeckArchetype string
}

// EndMatch 结束对战
func (t *GameTracer) EndMatch(ctx context.Context, result MatchEndRequest) {
	span := trace.SpanFromContext(ctx)
	if span != nil {
		span.SetName(EventMatchEnd)
		span.SetAttributes(
			MatchResultKey.String(result.MatchResult),
			MatchDurationKey.Int64(result.DurationMs),
			attribute.Int("match.kills", result.Kills),
			attribute.Int("match.deaths", result.Deaths),
			attribute.Int("match.assists", result.Assists),
			attribute.Int("match.damage_done", result.DamageDone),
			attribute.Int("match.damage_taken", result.DamageTaken),
			attribute.Bool("match.surrender", result.Surrender),
			CardDeckIDKey.String(result.DeckID),
			CardArchetypeKey.String(result.DeckArchetype),
		)

		// 记录对局时长
		t.metrics.MatchDuration.Record(ctx, float64(result.DurationMs), metric.WithAttributes(
			MatchModeKey.String(result.GameMode),
			MatchResultKey.String(result.MatchResult),
		))

		t.metrics.MatchEndCounter.Add(ctx, 1, metric.WithAttributes(
			MatchResultKey.String(result.MatchResult),
			MatchModeKey.String(result.GameMode),
		))

		if result.MatchResult == "win" {
			span.SetStatus(codes.Ok, "Match won")
		} else {
			span.SetStatus(codes.Ok, fmt.Sprintf("Match result: %s", result.MatchResult))
		}

		span.End()
	}
}

// === 经济系统 ===

// EconomyTransaction 经济交易
type EconomyTransaction struct {
	UserID       string
	Currency     string
	CurrencyKind string // soft/hard/real
	Amount       float64
	Type         string // earn/spend
	Source       string // for earn: kill_enemy/wave_bonus/quest/ad_reward/offline
	Sink         string // for spend: tower_build/tower_upgrade/ability/unlock/boost
	ItemID       string
	BalanceAfter float64
}

// TrackEconomyTransaction 追踪经济交易
func (t *GameTracer) TrackEconomyTransaction(ctx context.Context, transaction EconomyTransaction) {
	var eventName string
	var attributes []attribute.KeyValue

	baseAttrs := []attribute.KeyValue{
		GameUserIDKey.String(transaction.UserID),
		EconomyCurrencyKey.String(transaction.Currency),
		EconomyCurrencyKindKey.String(transaction.CurrencyKind),
		EconomyAmountKey.Float64(transaction.Amount),
		attribute.String("economy.item_id", transaction.ItemID),
		attribute.Float64("economy.balance_after", transaction.BalanceAfter),
	}

	if transaction.Type == "earn" {
		eventName = EventEconomyEarn
		attributes = append(baseAttrs, EconomySourceKey.String(transaction.Source))

		t.metrics.CurrencyEarn.Add(ctx, transaction.Amount, metric.WithAttributes(
			EconomyCurrencyKey.String(transaction.Currency),
			EconomySourceKey.String(transaction.Source),
			EconomyCurrencyKindKey.String(transaction.CurrencyKind),
		))
	} else {
		eventName = EventEconomySpend
		attributes = append(baseAttrs, EconomySinkKey.String(transaction.Sink))

		t.metrics.CurrencySpend.Add(ctx, transaction.Amount, metric.WithAttributes(
			EconomyCurrencyKey.String(transaction.Currency),
			EconomySinkKey.String(transaction.Sink),
			EconomyCurrencyKindKey.String(transaction.CurrencyKind),
		))
	}

	ctx, span := t.tracer.Start(ctx, eventName, trace.WithAttributes(attributes...))
	span.End()

	// 发送Analytics事件
	if t.bridge != nil {
		t.bridge.SendEconomyEvent(ctx, eventName, span, transaction.UserID, transaction.Currency, transaction.Amount, map[string]interface{}{
			"currency_kind":  transaction.CurrencyKind,
			"item_id":        transaction.ItemID,
			"balance_after":  transaction.BalanceAfter,
			"source":         transaction.Source,
			"sink":           transaction.Sink,
		})
	}
}

// === 变现系统 ===

// PurchaseFlow 付费流程
type PurchaseFlow struct {
	UserID         string
	OrderID        string
	SKUID          string
	PriceUSD       float64
	CurrencyCode   string
	PaymentProvider string
}

// StartPurchase 开始付费流程
func (t *GameTracer) StartPurchase(ctx context.Context, purchase PurchaseFlow) (context.Context, trace.Span) {
	ctx, span := t.tracer.Start(ctx, EventMonetizationPurchaseAttempt,
		trace.WithAttributes(
			GameUserIDKey.String(purchase.UserID),
			MonetizationOrderIDKey.String(purchase.OrderID),
			MonetizationSKUKey.String(purchase.SKUID),
			MonetizationPriceKey.Float64(purchase.PriceUSD),
			attribute.String("monetization.currency_code", purchase.CurrencyCode),
			MonetizationProviderKey.String(purchase.PaymentProvider),
		),
	)

	return ctx, span
}

// PurchaseResult 付费结果
type PurchaseResult struct {
	OrderID    string
	SKUID      string
	PriceUSD   float64
	Success    bool
	FailReason string
	TaxUSD     float64
	Country    string
}

// CompletePurchase 完成付费
func (t *GameTracer) CompletePurchase(ctx context.Context, result PurchaseResult) {
	span := trace.SpanFromContext(ctx)
	if span != nil {
		if result.Success {
			span.SetName(EventMonetizationPurchaseSuccess)
			span.SetStatus(codes.Ok, "Purchase completed successfully")
			span.SetAttributes(
				attribute.Float64("monetization.tax_usd", result.TaxUSD),
				attribute.String("monetization.country", result.Country),
			)

			// 记录收入
			t.metrics.RevenueTotal.Add(ctx, result.PriceUSD, metric.WithAttributes(
				MonetizationSKUKey.String(result.SKUID),
				attribute.String("country", result.Country),
			))
		} else {
			span.SetName(EventMonetizationPurchaseFail)
			span.SetStatus(codes.Error, fmt.Sprintf("Purchase failed: %s", result.FailReason))
			span.SetAttributes(
				attribute.String("monetization.fail_reason", result.FailReason),
			)
		}

		span.End()
	}
}

// === 广告系统 ===

// AdImpressionRequest 广告曝光请求
type AdImpressionRequest struct {
	UserID       string
	AdNetwork    string
	PlacementID  string
	AdFormat     string // rewarded/interstitial/banner/native
	PlacementType string // between_waves/revive/booster/double_reward
	EcpmUSD      float64
	RevenueUSD   float64
}

// TrackAdImpression 追踪广告曝光
func (t *GameTracer) TrackAdImpression(ctx context.Context, req AdImpressionRequest) {
	ctx, span := t.tracer.Start(ctx, EventAdImpression,
		trace.WithAttributes(
			GameUserIDKey.String(req.UserID),
			AdNetworkKey.String(req.AdNetwork),
			AdPlacementKey.String(req.PlacementID),
			AdFormatKey.String(req.AdFormat),
			attribute.String("ad.placement_type", req.PlacementType),
			AdEcpmKey.Float64(req.EcpmUSD),
			AdRevenueKey.Float64(req.RevenueUSD),
		),
	)
	defer span.End()

	// 记录广告指标
	t.metrics.AdImpressions.Add(ctx, 1, metric.WithAttributes(
		AdNetworkKey.String(req.AdNetwork),
		AdPlacementKey.String(req.PlacementID),
		AdFormatKey.String(req.AdFormat),
	))

	if req.RevenueUSD > 0 {
		t.metrics.AdRevenue.Add(ctx, req.RevenueUSD, metric.WithAttributes(
			AdNetworkKey.String(req.AdNetwork),
			AdPlacementKey.String(req.PlacementID),
		))
	}
}

// === 性能监控 ===

// PerformanceMetrics 性能指标
type PerformanceMetrics struct {
	UserID      string
	FPS         float64
	MemoryMB    float64
	CPULoad     float64
	RTTMs       float64
	JitterMs    float64
	PacketLoss  float64
}

// RecordPerformance 记录性能指标
func (t *GameTracer) RecordPerformance(ctx context.Context, perf PerformanceMetrics) {
	// 记录客户端帧率
	if perf.FPS > 0 {
		t.metrics.ClientFPS.Record(ctx, perf.FPS, metric.WithAttributes(
			GameUserIDKey.String(perf.UserID),
		))
	}

	// 记录内存使用
	if perf.MemoryMB > 0 {
		t.metrics.MemoryUsage.Record(ctx, perf.MemoryMB, metric.WithAttributes(
			GameUserIDKey.String(perf.UserID),
		))
	}

	// 记录网络延迟
	if perf.RTTMs > 0 {
		t.metrics.NetworkLatency.Record(ctx, perf.RTTMs, metric.WithAttributes(
			GameUserIDKey.String(perf.UserID),
		))
	}
}

// === 错误追踪 ===

// CrashEvent 崩溃事件
type CrashEvent struct {
	UserID      string
	SessionID   string
	StackHash   string
	SignalCode  string
	Scene       string
	DeviceID    string
}

// TrackCrash 追踪崩溃
func (t *GameTracer) TrackCrash(ctx context.Context, crash CrashEvent) {
	ctx, span := t.tracer.Start(ctx, EventErrorCrash,
		trace.WithAttributes(
			GameUserIDKey.String(crash.UserID),
			GameSessionIDKey.String(crash.SessionID),
			ErrorStackHashKey.String(crash.StackHash),
			ErrorSignalKey.String(crash.SignalCode),
			ErrorSceneKey.String(crash.Scene),
		),
	)
	defer span.End()

	span.SetStatus(codes.Error, "Application crash")

	t.metrics.CrashCounter.Add(ctx, 1, metric.WithAttributes(
		ErrorSceneKey.String(crash.Scene),
		attribute.String("device_id", crash.DeviceID),
	))
}

// === 塔防游戏特有事件 ===

// TowerBuildRequest 塔建造请求
type TowerBuildRequest struct {
	UserID      string
	LevelID     string
	TowerID     string
	TowerType   string
	PosX        int
	PosY        int
	Cost        float64
	WaveIndex   int
}

// TrackTowerBuild 追踪塔建造
func (t *GameTracer) TrackTowerBuild(ctx context.Context, req TowerBuildRequest) {
	ctx, span := t.tracer.Start(ctx, EventTDTowerBuild,
		trace.WithAttributes(
			GameUserIDKey.String(req.UserID),
			ProgressionLevelIDKey.String(req.LevelID),
			TDTowerIDKey.String(req.TowerID),
			TDTowerTypeKey.String(req.TowerType),
			TDTowerPosXKey.Int(req.PosX),
			TDTowerPosYKey.Int(req.PosY),
			TDTowerCostKey.Float64(req.Cost),
			ProgressionWaveKey.Int(req.WaveIndex),
		),
	)
	defer span.End()

	t.metrics.TDTowerBuildCounter.Add(ctx, 1, metric.WithAttributes(
		TDTowerTypeKey.String(req.TowerType),
		ProgressionLevelIDKey.String(req.LevelID),
	))
}

// TowerUpgradeRequest 塔升级请求
type TowerUpgradeRequest struct {
	UserID      string
	LevelID     string
	TowerID     string
	TowerType   string
	FromLevel   int
	ToLevel     int
	Cost        float64
	WaveIndex   int
}

// TrackTowerUpgrade 追踪塔升级
func (t *GameTracer) TrackTowerUpgrade(ctx context.Context, req TowerUpgradeRequest) {
	ctx, span := t.tracer.Start(ctx, EventTDTowerUpgrade,
		trace.WithAttributes(
			GameUserIDKey.String(req.UserID),
			ProgressionLevelIDKey.String(req.LevelID),
			TDTowerIDKey.String(req.TowerID),
			TDTowerTypeKey.String(req.TowerType),
			attribute.Int("td.from_level", req.FromLevel),
			attribute.Int("td.to_level", req.ToLevel),
			TDTowerCostKey.Float64(req.Cost),
			ProgressionWaveKey.Int(req.WaveIndex),
		),
	)
	defer span.End()

	t.metrics.TDTowerUpgradeCounter.Add(ctx, 1, metric.WithAttributes(
		TDTowerTypeKey.String(req.TowerType),
		ProgressionLevelIDKey.String(req.LevelID),
	))
}

// === 抽卡系统 ===

// GachaPullRequest 抽卡请求
type GachaPullRequest struct {
	UserID       string
	PoolID       string
	Pulls        int
	Rarity       string
	PityCounter  int
	ItemIDs      []string
}

// TrackGachaPull 追踪抽卡
func (t *GameTracer) TrackGachaPull(ctx context.Context, req GachaPullRequest) {
	ctx, span := t.tracer.Start(ctx, EventGachaPull,
		trace.WithAttributes(
			GameUserIDKey.String(req.UserID),
			GachaPoolIDKey.String(req.PoolID),
			GachaPullsKey.Int(req.Pulls),
			GachaRarityKey.String(req.Rarity),
			GachaPityCounterKey.Int(req.PityCounter),
		),
	)
	defer span.End()

	t.metrics.GachaPullCounter.Add(ctx, int64(req.Pulls), metric.WithAttributes(
		GachaPoolIDKey.String(req.PoolID),
		GachaRarityKey.String(req.Rarity),
	))
}