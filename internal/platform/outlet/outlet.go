// Package outlet 实现告警出口抽象（plugin-mechanism 设计 §5.1）：事件源
// 只产统一信封（AlertEvent），出口管理器按注册的 Outlet 扇出；出口失败
// 不阻塞告警链（异步投递 + 有限重试 + 失败计数）。herald = 第一内置出口
// （herald.go）；generic webhook 出口留位（设计 §5.4，按需批）。
package outlet

import (
	"context"
	"errors"
	"log/slog"
	"sync/atomic"
	"time"
)

// 告警事件 kind 闭集（plugin-mechanism §5.2；新增 kind 随对应 agent 简档走）。
const (
	// KindCaptureRuleHit capture 规则命中（capture-agent 简档 §4.4）。
	KindCaptureRuleHit = "capture.rule_hit"
	// KindDevopsBuildFailed devops 构建失败（devops-agent 规则）。
	KindDevopsBuildFailed = "devops.build_failed"
	// KindSupervisorBreaker supervisor 熔断触发（agent supervisor S2）。
	KindSupervisorBreaker = "supervisor.breaker_tripped"
	// KindProbeUnavailable healthprobe 故障窗口开始。
	KindProbeUnavailable = "probe.unavailable"
	// KindProbeRecovered healthprobe 故障窗口恢复。
	KindProbeRecovered = "probe.recovered"
	// KindAlertInbound 告警入口转出口链路（M3 inbound webhook）。
	KindAlertInbound = "alert.inbound"
)

// 告警严重度闭集（三档；出口自行映射紧急度）。
const (
	SeverityCritical = "critical"
	SeverityWarning  = "warning"
	SeverityInfo     = "info"
)

// 投递重试参数：有限重试防出口拖垮告警链；间隔固定短退避即可（出口
// 不可达是部署级事件，指数退避收益有限）。
const (
	dispatchAttempts   = 3
	dispatchRetryDelay = 2 * time.Second
	dispatchTimeout    = 15 * time.Second
)

// Scope 是可空的 scope 三元组（出口按 scope 分流受众时启用；v1 事件源
// 至少带 agentId）。
type Scope struct {
	GameID  string
	Env     string
	AgentID string
}

// AlertEvent 是出口间共享的统一告警事件信封（plugin-mechanism §5.2）。
type AlertEvent struct {
	Kind             string // kind 闭集
	Severity         string // severity 闭集
	EventID          string // croupier 自有事件主键（进幂等）
	DedupKey         string // 出口侧去重键（同键同态折叠，状态翻转重发）
	Title            string // 摘要文本（v1 直投，不用模板）
	Body             string
	OccurredAtUnixMs int64 // 事件时间
	Scope            Scope // 可空三元组
}

// Outlet 是出口接口位（供应商可插拔：新增出口 = 新增一个实现 + 配置
// 一行，事件词汇与告警管线零改动）。Deliver 返回错误即投递失败（重试
// 由管理器负责）；实现必须非阻塞友好（内部队列或快速返回）。
type Outlet interface {
	// Name 是配置选择键（"herald" / "webhook"）。
	Name() string
	Deliver(ctx context.Context, ev AlertEvent) error
}

// permanentError 标记不可重试错误（拒绝类：凭据/契约/4xx 语义——重试
// 无意义，管理器立即放弃）。
type permanentError struct{ err error }

func (e *permanentError) Error() string { return e.err.Error() }
func (e *permanentError) Unwrap() error { return e.err }

// Permanent 把 err 标记为不可重试；nil 原样返回。
func Permanent(err error) error {
	if err == nil {
		return nil
	}
	return &permanentError{err: err}
}

func isPermanent(err error) bool {
	var p *permanentError
	return errors.As(err, &p)
}

// Manager 按注册的出口扇出信封。v1 配置只启 herald，接口位支持多出口
// 并发扇出（多出口扇出全量开放留后续批次）。
type Manager struct {
	mu       chan struct{} // 序列化 outlets 切片变更（Register 与快照读）
	outlets  []Outlet
	failures atomic.Int64 // 投递失败累计（面板/日志观测用）
	sleep    func(time.Duration)
}

// NewManager creates an empty manager. 出口经 Register 注册后生效。
func NewManager() *Manager {
	return &Manager{mu: make(chan struct{}, 1), sleep: time.Sleep}
}

// Register adds one outlet；nil 忽略。v1 单出口语义下重复注册同一出口
// 不去重（管理器不感知实现），配置侧保证只启一个。
func (m *Manager) Register(o Outlet) {
	if o == nil {
		return
	}
	m.mu <- struct{}{}
	m.outlets = append(m.outlets, o)
	<-m.mu
}

// Names returns registered outlet names（诊断用，快照序）。
func (m *Manager) Names() []string {
	m.mu <- struct{}{}
	defer func() { <-m.mu }()
	out := make([]string, 0, len(m.outlets))
	for _, o := range m.outlets {
		out = append(out, o.Name())
	}
	return out
}

// Failures returns the cumulative delivery failure count.
func (m *Manager) Failures() int64 { return m.failures.Load() }

// Dispatch 异步扇出一条信封：每出口独立 goroutine 带有限重试，panic
// recover 兜底——出口不可用绝不阻塞告警链，失败只计数与记日志。
func (m *Manager) Dispatch(ev AlertEvent) {
	m.mu <- struct{}{}
	snapshot := make([]Outlet, len(m.outlets))
	copy(snapshot, m.outlets)
	<-m.mu
	for _, o := range snapshot {
		go m.deliver(o, ev)
	}
}

func (m *Manager) deliver(o Outlet, ev AlertEvent) {
	defer func() { _ = recover() }()
	var lastErr error
	for attempt := 0; attempt < dispatchAttempts; attempt++ {
		if attempt > 0 {
			m.sleep(dispatchRetryDelay)
		}
		ctx, cancel := context.WithTimeout(context.Background(), dispatchTimeout)
		err := o.Deliver(ctx, ev)
		cancel()
		if err == nil {
			return
		}
		lastErr = err
		if isPermanent(err) {
			break
		}
	}
	m.failures.Add(1)
	slog.WarnContext(context.Background(), "outlet: deliver failed after retries",
		"outlet", o.Name(), "kind", ev.Kind, "eventId", ev.EventID, "error", lastErr)
}
