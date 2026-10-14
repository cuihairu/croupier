package healthprobe

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Manager 多目标探针装配（M1）：per-agent 声明 probe.targets[] → 多 Probe，
// 每目标按各自 EffectiveInterval 周期探测。窗口事件与结果回调经装配注入
// （上行出口由调用方决定——capture 走本地日志+事件上行，sidecar 走
// report.ProbeUplink）。
type Manager struct {
	items []managerItem
}

type managerItem struct {
	cfg   TargetConfig
	probe *Probe
}

// NewManager 从目标声明构造多探针。任一目标校验失败即整体报错（宁可不起
// 装配，不静默丢弃——与 capture 配置校验同哲学）。
func NewManager(cfgs []TargetConfig, scopeGame, scopeEnv string, onWindow WindowListener, onResult ResultListener) (*Manager, error) {
	if len(cfgs) == 0 {
		return nil, fmt.Errorf("probe manager: no targets configured")
	}
	m := &Manager{}
	for _, tc := range cfgs {
		p, err := ProbeFromTarget(tc, scopeGame, scopeEnv, onWindow, onResult)
		if err != nil {
			return nil, err
		}
		m.items = append(m.items, managerItem{cfg: tc, probe: p})
	}
	return m, nil
}

// Probes 返回装配出的探针（只读视图）。
func (m *Manager) Probes() []*Probe {
	out := make([]*Probe, 0, len(m.items))
	for _, it := range m.items {
		out = append(out, it.probe)
	}
	return out
}

// Start 启动全部探针周期探测；ctx 取消即全部停止。每目标一个 goroutine，
// 各自按 EffectiveInterval 驱动（默认 30s）。阻塞至 ctx 取消。
func (m *Manager) Start(ctx context.Context) {
	m.StartDynamic(ctx, nil)
}

// StartDynamic 动态间隔驱动全部探针：intervalFn 按目标名取下一间隔（采样
// 策略快照的读侧——热更即换快照，探针下轮自动跟随；异常态提频由调用方在
// fn 内结合探针自身状态叠加）。fn 为 nil 时等价 Start。
func (m *Manager) StartDynamic(ctx context.Context, intervalFn func(target string, now time.Time) time.Duration) {
	var wg sync.WaitGroup
	for _, it := range m.items {
		wg.Add(1)
		go func(it managerItem) {
			defer wg.Done()
			it.probe.RunDynamic(ctx, func(now time.Time) time.Duration {
				if intervalFn == nil {
					return it.cfg.EffectiveInterval()
				}
				return intervalFn(it.cfg.Name, now)
			})
		}(it)
	}
	wg.Wait()
}
