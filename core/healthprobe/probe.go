package healthprobe

import (
	"sync"
	"time"
)

// 状态二值与窗口事件类型。
const (
	StatusAvailable   = "available"
	StatusUnavailable = "unavailable"
	EventUnavailable  = "unavailable"
	EventRecovered    = "recovered"
)

// Window 故障窗口：一次 unavailable→recovered 的完整时间段（按 system 角色
// 建模，用户令 2026-10-09）。EndTS=0 表示未恢复（进行中）；排查时直接回答
// 「什么时间段服务不可用」。双通道留存：本地时间线文件（真值）+ listener
// 上报捎带（窗口开/合作为事件上行）。
type Window struct {
	Target     string `json:"target"`
	Semantics  string `json:"semantics"`
	StartTS    int64  `json:"start_ts_unix_ms"`
	EndTS      int64  `json:"end_ts_unix_ms,omitempty"` // 0 = 未恢复
	DurationMS int64  `json:"duration_ms,omitempty"`
	LastReason string `json:"last_reason,omitempty"` // 达阈值时的最后一次失败原因
	GameID     string `json:"game_id,omitempty"`
	Env        string `json:"env,omitempty"`
}

// WindowEvent 窗口开/合事件；herald 出口（probe.unavailable/probe.recovered）
// 与 server 上报在此接入。
type WindowEvent struct {
	Kind   string `json:"kind"` // unavailable | recovered
	Window Window `json:"window"`
}

// WindowListener 窗口事件回调。
type WindowListener func(WindowEvent)

// Result 单次探测结果。
type Result struct {
	OK                  bool
	Err                 string
	At                  time.Time
	Available           bool
	ConsecutiveFailures int
	WindowOpen          bool
}

// Probe 单目标探针：连续失败/恢复阈值驱动的两态机（available ⇄ unavailable）。
// 初始视为 available；连续失败达 failThr 开窗，连续成功达 okThr 闭窗。
type Probe struct {
	target     string
	sem        Semantics
	check      Checker
	failThr    int
	okThr      int
	scopeGame  string
	scopeEnv   string
	listener   WindowListener
	timeline   TimelineWriter
	maxHistory int

	mu         sync.Mutex
	available  bool
	failStreak int
	okStreak   int
	current    *Window
	history    []Window
}

// Option 探针可选参数。
type Option func(*Probe)

// WithThresholds 覆盖失败/恢复阈值（默认 fail=3、recover=1，对标 k8s
// failureThreshold=3 / successThreshold=1）。
func WithThresholds(fail, recover int) Option {
	return func(p *Probe) {
		if fail > 0 {
			p.failThr = fail
		}
		if recover > 0 {
			p.okThr = recover
		}
	}
}

// WithScope 附加 game_id/env（强制随窗口记录，注册标签同源）。
func WithScope(gameID, env string) Option {
	return func(p *Probe) { p.scopeGame, p.scopeEnv = gameID, env }
}

// OnWindow 注册窗口事件回调（上报/herald 出口）。
func OnWindow(fn WindowListener) Option {
	return func(p *Probe) { p.listener = fn }
}

// WithTimeline 挂接本地时间线文件（真值通道）。
func WithTimeline(w TimelineWriter) Option { return func(p *Probe) { p.timeline = w } }

// WithHistoryLimit 覆盖内存历史环长度（默认 100）。
func WithHistoryLimit(n int) Option {
	return func(p *Probe) {
		if n > 0 {
			p.maxHistory = n
		}
	}
}

// New 构造探针（不做首次探测；周期探测用 Run，单次用 ProbeOnce）。
func New(target string, sem Semantics, check Checker, opts ...Option) *Probe {
	p := &Probe{
		target:     target,
		sem:        sem,
		check:      check,
		failThr:    3,
		okThr:      1,
		maxHistory: 100,
		available:  true,
	}
	for _, o := range opts {
		o(p)
	}
	return p
}
