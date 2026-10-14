package agent

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cuihairu/croupier/core/healthprobe"
	"github.com/cuihairu/croupier/core/report"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
)

// ProbeManager agent 侧探测装配（R2：M1 多探针 + M2 上行接线）。职责：
//
//	配置 probe.targets[] → core/healthprobe.Manager（多 Probe 周期探测）
//	                     → 逐目标 report.ProbeUplink（不对称上行 §13.1）
//	                     → DrainProbeStats 折进 MetricsReport 批量面
//
// 硬约束（R2 验收）：窗口事件消费者**只做上行**，绝不触发进程重启/恢复动作——
// 探测是观测面，恢复是 supervisor 面；把两者耦合会让「发现故障」直接变成
// 「对故障动手」。本文件因此不引用任何 supervisor/ops 装配。

// ProbeManagerOptions 装配参数。Send 为 nil 时只统计不上行（server 不消费
// 或通道未就绪时 agent 照跑，§13.1 M2 可回退语义）。
type ProbeManagerOptions struct {
	AgentID string
	GameID  string
	Env     string
	// Send 单发 AlertEvent 正文（经 UpstreamClient.AlertEventSender 注入）。
	Send func(ctx context.Context, body []byte) error
	// MaxDetailsPerWindow 每故障窗口每目标的失败明细条数上限（<=0 用默认）。
	MaxDetailsPerWindow int
	Logger              *slog.Logger
}

// ProbeConfig 探测面配置契约（agent 配置 `probe:` 段，键一律小驼峰）。
// Targets 只声明「探什么」，阈值/语义/间隔/上行口径由本包给显式默认。
type ProbeConfig struct {
	Targets []healthprobe.TargetConfig `json:"targets" yaml:"targets"`
	// MaxDetailsPerWindow 每故障窗口每目标的失败明细条数上限（<=0 用默认 10）。
	MaxDetailsPerWindow int `json:"maxDetailsPerWindow,omitempty" yaml:"maxDetailsPerWindow,omitempty"`
}

// ProbeManager 多目标探针 + 上行桥的组合体。
type ProbeManager struct {
	manager *healthprobe.Manager
	uplinks map[string]*report.ProbeUplink
	order   []string
	logger  *slog.Logger
	opts    ProbeManagerOptions // 供 R4 动态目标的 uplink 后补装配复用

	// mu 守 uplinks/unavailable/localInterval/order/dynamic：静态装配写一次
	// 后由 ApplyTargets（R4 下行热更）与探针 goroutine（读回调路由）并发访问。
	mu sync.RWMutex

	// policy 采样策略热更快照（probe.policy 下行）；nil = 用本地配置间隔。
	// 读侧按需取值，热更无需重建探针（采样策略不是探针结构变更）。
	policy atomic.Pointer[healthprobe.ProbePolicy]
	// unavailable 每目标当前是否开着故障窗口（窗口开着 = 异常态，用于提频）。
	unavailable map[string]*atomic.Bool
	// localInterval 每目标本地配置的基础间隔（策略未覆盖/未下发时回落）。
	localInterval map[string]time.Duration
	// dynamic R4：server 按 db_sources.owner_agent_id 派生的 db 目标集（本地
	// probe.targets[] 之外的动态增删，键为目标名）。
	dynamic map[string]healthprobe.TargetConfig
}

// NewProbeManager 从目标声明装配探测面。任一目标非法（含重名）整体报错——
// 重名会让上行路由静默串台，宁可不起装配。cfgs 为空时装配「空静态 + 动态
// 可增删」的探测面（R4：归属 agent 可不声明本地目标，db 目标由 server 下发；
// agent 配置 `probe:` 段存在即参与探测面，targets 为空是合法声明）。
func NewProbeManager(cfgs []healthprobe.TargetConfig, opts ProbeManagerOptions) (*ProbeManager, error) {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}

	seen := make(map[string]bool, len(cfgs))
	uplinks := make(map[string]*report.ProbeUplink, len(cfgs))
	order := make([]string, 0, len(cfgs))
	unavailable := make(map[string]*atomic.Bool, len(cfgs))
	localInterval := make(map[string]time.Duration, len(cfgs))
	for _, tc := range cfgs {
		name := strings.TrimSpace(tc.Name)
		if seen[name] {
			return nil, fmt.Errorf("probe manager: duplicate target name %q", name)
		}
		seen[name] = true

		sem := tc.Semantics
		if sem == "" {
			sem = healthprobe.SemanticsReadiness
		}
		uplinks[name] = report.NewProbeUplink(report.ProbeUplinkConfig{
			AgentID:             opts.AgentID,
			GameID:              opts.GameID,
			Env:                 opts.Env,
			Target:              name,
			Semantics:           sem,
			MaxDetailsPerWindow: opts.MaxDetailsPerWindow,
		}, opts.Send)
		unavailable[name] = &atomic.Bool{}
		localInterval[name] = tc.EffectiveInterval()
		order = append(order, name)
	}

	pm := &ProbeManager{uplinks: uplinks, order: order, logger: logger,
		unavailable: unavailable, localInterval: localInterval,
		dynamic: map[string]healthprobe.TargetConfig{}, opts: opts}

	// 单一消费者路由器：core Manager 给的是共享回调，逐目标分流到各自上行桥。
	routeWindow := func(ev healthprobe.WindowEvent) {
		pm.mu.RLock()
		flag, ok := pm.unavailable[ev.Window.Target]
		up, upOK := pm.uplinks[ev.Window.Target]
		pm.mu.RUnlock()
		if ok {
			// 窗口开 = 不可用（异常态提频的触发信号，与采样策略解耦）。
			flag.Store(ev.Kind == healthprobe.EventUnavailable)
		}
		if !upOK {
			return
		}
		up.OnWindow(ev)
	}
	routeResult := func(res healthprobe.Result) {
		pm.mu.RLock()
		up, ok := pm.uplinks[res.Target]
		pm.mu.RUnlock()
		if !ok {
			return
		}
		up.OnResult(res)
	}

	if len(cfgs) == 0 {
		pm.manager = healthprobe.NewDynamicManager(opts.GameID, opts.Env, routeWindow, routeResult)
		return pm, nil
	}
	mgr, err := healthprobe.NewManager(cfgs, opts.GameID, opts.Env, routeWindow, routeResult)
	if err != nil {
		return nil, err
	}
	pm.manager = mgr
	return pm, nil
}

// Start 启动全部探针；阻塞至 ctx 取消。间隔按每轮向采样策略快照现取——
// probe.policy 下行热更后下轮即跟随，无需重启探针。
func (m *ProbeManager) Start(ctx context.Context) {
	if m == nil || m.manager == nil {
		return
	}
	m.manager.StartDynamic(ctx, m.IntervalFor)
}

// SetPolicy 热更采样策略快照（probe.policy 下行，评审简档 §13.3 C）。
// nil = 清除策略，回落本地 probe.targets 声明的间隔。
func (m *ProbeManager) SetPolicy(p *healthprobe.ProbePolicy) {
	if m == nil {
		return
	}
	m.policy.Store(p)
	if m.logger != nil {
		m.logger.Info("probe sampling policy applied",
			"targets", len(m.order), "has_policy", p != nil)
	}
}

// Policy 返回当前采样策略快照（未下发时 nil）。
func (m *ProbeManager) Policy() *healthprobe.ProbePolicy {
	if m == nil {
		return nil
	}
	return m.policy.Load()
}

// IntervalFor 目标 target 在 now 的生效采样间隔：策略快照已下发即以策略为
// 唯一真值（§13.3 C「频率是 server 下发的策略，agent 不本地写死」——不做
// 「本地值兜底」隐藏层，未声明即继承策略基础间隔）；未下发时回落本地
// probe.targets 声明的间隔。目标开着故障窗口且策略允许时按 C#5 提频
// （减半、下限 5s、绝不慢于原值）。
func (m *ProbeManager) IntervalFor(target string, now time.Time) time.Duration {
	policy := m.Policy()
	if policy == nil {
		if m == nil {
			return healthprobe.DefaultInterval
		}
		m.mu.RLock()
		d, ok := m.localInterval[target]
		m.mu.RUnlock()
		if ok {
			return d
		}
		return healthprobe.DefaultInterval
	}
	unavailable := false
	if m != nil {
		m.mu.RLock()
		flag, ok := m.unavailable[target]
		m.mu.RUnlock()
		if ok {
			unavailable = flag.Load()
		}
	}
	return policy.EffectiveIntervalFor(target, now, unavailable)
}

// DrainProbeStats 供 metrics 采样器折叠（正常态聚合面）。目标顺序稳定，便于
// 断言与排查。
func (m *ProbeManager) DrainProbeStats() []*opsv1.ProbeStats {
	if m == nil {
		return nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*opsv1.ProbeStats, 0, len(m.uplinks))
	names := append([]string(nil), m.order...)
	sort.Strings(names)
	for _, name := range names {
		if st := m.uplinks[name].Drain(); st != nil && st.Checked > 0 {
			out = append(out, st)
		}
	}
	return out
}

// Probes 返回装配出的探针（只读视图，供内部诊断/测试）。
func (m *ProbeManager) Probes() []*healthprobe.Probe {
	if m == nil || m.manager == nil {
		return nil
	}
	return m.manager.Probes()
}

// currentWindow 汇报当前所有开着的故障窗口（启动日志与排障用）。
func (m *ProbeManager) currentWindow() []healthprobe.Window {
	var out []healthprobe.Window
	for _, p := range m.Probes() {
		if w, ok := p.CurrentWindow(); ok {
			out = append(out, w)
		}
	}
	return out
}

// ProbeManagerOnce 启动探针并在首次探测后立即返回（单次诊断用途；周期探测
// 用 Start）。窗口事件消费者同样只做上行。
func (m *ProbeManager) ProbeManagerOnce(ctx context.Context) {
	if m == nil {
		return
	}
	var wg sync.WaitGroup
	probes := m.Probes()
	m.mu.RLock()
	orderLen := len(m.order)
	m.mu.RUnlock()
	for i, p := range probes {
		if i >= orderLen {
			break
		}
		wg.Add(1)
		go func(p *healthprobe.Probe) {
			defer wg.Done()
			p.ProbeOnce(ctx)
		}(p)
	}
	wg.Wait()
	if open := m.currentWindow(); len(open) > 0 && m.logger != nil {
		m.logger.Info("probe diagnostic run finished", "open_windows", len(open))
	}
}

// DynamicTargets 返回当前动态目标名（升序快照，R4 下行装配的诊断/测试面）。
func (m *ProbeManager) DynamicTargets() []string {
	if m == nil {
		return nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]string, 0, len(m.dynamic))
	for name := range m.dynamic {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// ApplyTargets 应用 server 下发的 db 目标集（R4：db_sources.owner_agent_id
// 归属派生，version 未变时调用方不会进来）。按目标名 diff：
//
//   - 新增 → 装配探针与上行桥并立即起跑（manager 已 Start 时）；
//   - 缺失 → 停探针、摘上行桥（卸载语义：解除归属/源被删）；
//   - 名字在且配置未变 → 保留原探针（窗口状态不清零）；
//   - 名字在但配置变了 → 重建探针（DSN/驱动等变了，旧窗口状态不再成立）。
//
// 重名、与本地 probe.targets[] 撞名、或任一目标非法一律整体拒绝——返回错误
// 且不改动现有装配（宁可拒绝，不静默少探；调用方版本游标不推进，下轮重试）。
func (m *ProbeManager) ApplyTargets(cfgs []healthprobe.TargetConfig) error {
	if m == nil || m.manager == nil {
		return fmt.Errorf("probe manager: not initialized")
	}
	want := make(map[string]healthprobe.TargetConfig, len(cfgs))
	for _, tc := range cfgs {
		name := strings.TrimSpace(tc.Name)
		if name == "" {
			return fmt.Errorf("probe targets: empty target name")
		}
		if _, dup := want[name]; dup {
			return fmt.Errorf("probe targets: duplicate target name %q", name)
		}
		tc.Name = name
		want[name] = tc
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	// 先整体校验再动装配：撞本地静态目标名（上行路由按名分流，撞名静默串台）
	// 或 checker 构造失败都拒绝，避免半装配。
	for name := range want {
		if _, occupied := m.uplinks[name]; occupied {
			if _, isDynamic := m.dynamic[name]; !isDynamic {
				return fmt.Errorf("probe targets: name %q collides with local probe.targets", name)
			}
		}
	}
	for _, tc := range want {
		if _, err := healthprobe.NewChecker(tc); err != nil {
			return err
		}
	}

	// 卸：不在新集合内的动态目标。
	for name := range m.dynamic {
		if _, keep := want[name]; !keep {
			m.removeDynamicLocked(name)
		}
	}
	// 装/换：新增与配置变更的目标。
	for name, tc := range want {
		if old, ok := m.dynamic[name]; ok {
			if old == tc {
				continue // 配置未变，保留原探针（窗口状态不清零）
			}
			m.removeDynamicLocked(name)
		}
		if err := m.addDynamicLocked(tc); err != nil {
			return err
		}
	}
	return nil
}

// removeDynamicLocked 摘除一个动态目标：停探针 goroutine、删上行桥与记账
// （调用方持写锁）。
func (m *ProbeManager) removeDynamicLocked(name string) {
	// manager.Remove 未知名报错属幂等场景，忽略——记账面（dynamic map）才是
	// 增删真值。
	_ = m.manager.Remove(name)
	delete(m.uplinks, name)
	delete(m.unavailable, name)
	delete(m.localInterval, name)
	delete(m.dynamic, name)
	for i, n := range m.order {
		if n == name {
			m.order = append(m.order[:i], m.order[i+1:]...)
			break
		}
	}
}

// addDynamicLocked 装配一个动态目标：上行桥 + 提频标志 + 本地间隔 + 探针。
// manager.Add 失败即回滚本目标全部记账（调用方持写锁）。
func (m *ProbeManager) addDynamicLocked(tc healthprobe.TargetConfig) error {
	name := tc.Name
	sem := tc.Semantics
	if sem == "" {
		sem = healthprobe.SemanticsReadiness
	}
	m.uplinks[name] = report.NewProbeUplink(report.ProbeUplinkConfig{
		AgentID:             m.opts.AgentID,
		GameID:              m.opts.GameID,
		Env:                 m.opts.Env,
		Target:              name,
		Semantics:           sem,
		MaxDetailsPerWindow: m.opts.MaxDetailsPerWindow,
	}, m.opts.Send)
	m.unavailable[name] = &atomic.Bool{}
	m.localInterval[name] = tc.EffectiveInterval()
	m.order = append(m.order, name)
	if err := m.manager.Add(tc); err != nil {
		delete(m.uplinks, name)
		delete(m.unavailable, name)
		delete(m.localInterval, name)
		for i, n := range m.order {
			if n == name {
				m.order = append(m.order[:i], m.order[i+1:]...)
				break
			}
		}
		return err
	}
	m.dynamic[name] = tc
	return nil
}
