package agent

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"

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
}

// NewProbeManager 从目标声明装配探测面。任一目标非法（含重名）整体报错——
// 重名会让上行路由静默串台，宁可不起装配。
func NewProbeManager(cfgs []healthprobe.TargetConfig, opts ProbeManagerOptions) (*ProbeManager, error) {
	if len(cfgs) == 0 {
		return nil, fmt.Errorf("probe manager: no targets configured")
	}
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}

	seen := make(map[string]bool, len(cfgs))
	uplinks := make(map[string]*report.ProbeUplink, len(cfgs))
	order := make([]string, 0, len(cfgs))
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
		order = append(order, name)
	}

	pm := &ProbeManager{uplinks: uplinks, order: order, logger: logger}

	// 单一消费者路由器：core Manager 给的是共享回调，逐目标分流到各自上行桥。
	routeWindow := func(ev healthprobe.WindowEvent) {
		up, ok := pm.uplinks[ev.Window.Target]
		if !ok {
			return
		}
		up.OnWindow(ev)
	}
	routeResult := func(res healthprobe.Result) {
		up, ok := pm.uplinks[res.Target]
		if !ok {
			return
		}
		up.OnResult(res)
	}

	mgr, err := healthprobe.NewManager(cfgs, opts.GameID, opts.Env, routeWindow, routeResult)
	if err != nil {
		return nil, err
	}
	pm.manager = mgr
	return pm, nil
}

// Start 启动全部探针；阻塞至 ctx 取消。
func (m *ProbeManager) Start(ctx context.Context) {
	if m == nil || m.manager == nil {
		return
	}
	m.manager.Start(ctx)
}

// DrainProbeStats 供 metrics 采样器折叠（正常态聚合面）。目标顺序稳定，便于
// 断言与排查。
func (m *ProbeManager) DrainProbeStats() []*opsv1.ProbeStats {
	if m == nil {
		return nil
	}
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
	for i, p := range probes {
		if i >= len(m.order) {
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
