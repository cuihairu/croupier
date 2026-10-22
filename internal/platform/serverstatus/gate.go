package serverstatus

import (
	"context"
	"log/slog"
	"sync/atomic"
)

// Verdict gate 裁决结果：投递与否 + 事件标注位（设计 §5「事件标注」落点，
// S2 接线时写进 AlertEvent metadata / 信封扩展位）。
type Verdict struct {
	// Suppressed=true 抑制投递（出口链整体静默；告警/事件落库不变）。
	Suppressed bool
	// SourceUnknown=true 状态源不可达/无映射——照报（Suppressed 必为
	// false）+ 事件标 source_unknown（宁误报不漏报，用户令原文「不许静默」）。
	SourceUnknown bool
	// Source 参与裁决的状态源名（"atlas"…；未知分支为空）。
	Source string
}

// Gate 维护状态 gate（S1：判定内核；S2 接线到出口链入口）。provider 为 nil
// 或 ref 无法定位服务器时直通——gate 只对能定位服务器且启用了状态源的事件
// 生效（宁多报不漏报）。
type Gate struct {
	provider Provider
	cache    *Cache
	logger   *slog.Logger

	suppressed atomic.Int64
	unknown    atomic.Int64
	queries    atomic.Int64
}

// NewGate 创建 gate。provider 为 nil 时整体旁路（等价 enabled=false）；
// cache 为 nil 时新建默认 TTL 缓存。
func NewGate(provider Provider, cache *Cache, logger *slog.Logger) *Gate {
	if cache == nil {
		cache = NewCache(0)
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Gate{provider: provider, cache: cache, logger: logger}
}

// Enabled 状态源是否已布线（svc 装配层据此决定挂不挂 gate）。
func (g *Gate) Enabled() bool { return g != nil && g.provider != nil }

// Evaluate 三分支裁决：
//  1. InMaintenance=true       → Suppressed（抑制投递，计数+日志，不静默丢）
//  2. InMaintenance=false      → 照常投递
//  3. 查询失败/超时/无映射/无定位 → 照常投递 + SourceUnknown 标注
//
// 缓存只命中成功结果（错误分支每次回源）；TTL 内命中不回源。
func (g *Gate) Evaluate(ctx context.Context, ref ServerRef) Verdict {
	if !g.Enabled() || !ref.Valid() {
		return Verdict{}
	}
	if st, ok := g.cache.Get(ref); ok {
		return g.decide(st, nil)
	}
	g.queries.Add(1)
	st, err := g.provider.GetServerStatus(ctx, ref)
	if err == nil {
		g.cache.Set(ref, st)
	}
	return g.decide(st, err)
}

// decide 纯判定（缓存命中与回源共用）。
func (g *Gate) decide(st ServerStatus, err error) Verdict {
	if err != nil {
		g.unknown.Add(1)
		g.logger.Warn("serverstatus: provider query failed, fail-open to alerting",
			"source", st.Source, "error", err)
		return Verdict{SourceUnknown: true}
	}
	if st.InMaintenance {
		g.suppressed.Add(1)
		g.logger.Info("serverstatus: alert suppressed inside maintenance window",
			"source", st.Source)
		return Verdict{Suppressed: true, Source: st.Source}
	}
	return Verdict{Source: st.Source}
}

// Counters 观测计数（抑制/未知/回源次数；S2 接指标，先留日志口径——
// 抑制不是丢弃，计数可查）。
func (g *Gate) Counters() (suppressed, unknown, queries int64) {
	return g.suppressed.Load(), g.unknown.Load(), g.queries.Load()
}
