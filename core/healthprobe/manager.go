package healthprobe

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// Manager 多目标探针装配（M1）：per-agent 声明 probe.targets[] → 多 Probe，
// 每目标按各自 EffectiveInterval 周期探测。窗口事件与结果回调经装配注入
// （上行出口由调用方决定——capture 走本地日志+事件上行，sidecar 走
// report.ProbeUplink）。
//
// 动态目标集（R4）：server 按 db_sources.owner_agent_id 派生的 db 目标经
// Add/Remove 增删——Start 之后 Add 的探针立即起跑（自带 per-probe 派生
// ctx），Remove 按目标名取消对应探针 goroutine。采样策略/窗口状态归探针
// 自身与装配侧管理，增删不动存活目标。
type Manager struct {
	mu        sync.Mutex
	items     []managerItem
	scopeGame string
	scopeEnv  string
	onWindow  WindowListener
	onResult  ResultListener

	// 动态装配记账：StartDynamic 记 ctx/intervalFn 并置 running；此后 Add
	// 的目标立即起跑。关闭（ctx 取消）时按 per-probe cancel 停全部探针。
	ctx        context.Context
	intervalFn func(target string, now time.Time) time.Duration
	running    bool
	wg         sync.WaitGroup
}

type managerItem struct {
	cfg    TargetConfig
	probe  *Probe
	cancel context.CancelFunc // per-probe 取消（Remove/整体关闭用）
}

// newManagerBase 空装配底座：目标经 NewManager 批量声明或 Add 逐个进入。
func newManagerBase(scopeGame, scopeEnv string, onWindow WindowListener, onResult ResultListener) *Manager {
	return &Manager{scopeGame: scopeGame, scopeEnv: scopeEnv, onWindow: onWindow, onResult: onResult}
}

// NewManager 从目标声明构造多探针。任一目标校验失败即整体报错（宁可不起
// 装配，不静默丢弃——与 capture 配置校验同哲学）。
func NewManager(cfgs []TargetConfig, scopeGame, scopeEnv string, onWindow WindowListener, onResult ResultListener) (*Manager, error) {
	if len(cfgs) == 0 {
		return nil, fmt.Errorf("probe manager: no targets configured")
	}
	m := newManagerBase(scopeGame, scopeEnv, onWindow, onResult)
	for _, tc := range cfgs {
		if err := m.addItemLocked(tc, false); err != nil {
			return nil, err
		}
	}
	return m, nil
}

// NewDynamicManager 无本地目标声明的空装配底座（R4：server 派生的 db 目标
// 全部经 Add 进入；本地 probe.targets[] 为空的归属 agent 用此构造）。
func NewDynamicManager(scopeGame, scopeEnv string, onWindow WindowListener, onResult ResultListener) *Manager {
	return newManagerBase(scopeGame, scopeEnv, onWindow, onResult)
}

// Add 追加目标。重名或目标校验失败返回错误且不改动现有装配；manager 已
// Start 时新探针立即起跑，否则由后续 Start 统一拉起。
func (m *Manager) Add(cfg TargetConfig) error {
	if m == nil {
		return fmt.Errorf("probe manager: nil manager")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.addItemLocked(cfg, m.running && m.ctx != nil)
}

// addItemLocked 校验并登记目标；startNow 时立即起跑（调用方持锁）。
func (m *Manager) addItemLocked(cfg TargetConfig, startNow bool) error {
	name := strings.TrimSpace(cfg.Name)
	if name == "" {
		return fmt.Errorf("probe manager: target name is required")
	}
	cfg.Name = name
	for _, it := range m.items {
		if it.cfg.Name == name {
			return fmt.Errorf("probe manager: duplicate target name %q", name)
		}
	}
	p, err := ProbeFromTarget(cfg, m.scopeGame, m.scopeEnv, m.onWindow, m.onResult)
	if err != nil {
		return err
	}
	m.items = append(m.items, managerItem{cfg: cfg, probe: p})
	if startNow {
		m.startPendingLocked()
	}
	return nil
}

// startPendingLocked 拉起所有尚未起跑的目标（per-probe 派生 ctx 写回 items，
// Remove/关闭据此取消）。调用方持锁；manager 已置 running 且 ctx 就绪。
func (m *Manager) startPendingLocked() {
	for i := range m.items {
		if m.items[i].cancel != nil {
			continue
		}
		cctx, cancel := context.WithCancel(m.ctx)
		m.items[i].cancel = cancel
		fn := m.intervalFn
		m.wg.Add(1)
		go func(it managerItem) {
			defer m.wg.Done()
			it.probe.RunDynamic(cctx, managerIntervalFunc(fn, it.cfg))
		}(m.items[i])
	}
}

// managerIntervalFunc 单目标的间隔函数：intervalFn 为 nil 时按目标声明的
// EffectiveInterval 驱动。
func managerIntervalFunc(intervalFn func(target string, now time.Time) time.Duration, cfg TargetConfig) func(time.Time) time.Duration {
	return func(now time.Time) time.Duration {
		if intervalFn == nil {
			return cfg.EffectiveInterval()
		}
		return intervalFn(cfg.Name, now)
	}
}

// Remove 按目标名移除并停止对应探针 goroutine；未知名返回错误（装配侧
// 幂等场景自行忽略）。
func (m *Manager) Remove(name string) error {
	if m == nil {
		return fmt.Errorf("probe manager: nil manager")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, it := range m.items {
		if it.cfg.Name != name {
			continue
		}
		if it.cancel != nil {
			it.cancel()
		}
		m.items = append(m.items[:i], m.items[i+1:]...)
		return nil
	}
	return fmt.Errorf("probe manager: target %q not found", name)
}

// Probes 返回装配出的探针（只读视图）。
func (m *Manager) Probes() []*Probe {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]*Probe, 0, len(m.items))
	for _, it := range m.items {
		out = append(out, it.probe)
	}
	return out
}

// Start 启动全部探针周期探测；ctx 取消即全部停止（含此后 Add 进来的）。
// 每目标一个 goroutine，各自按 EffectiveInterval 驱动（默认 30s）。阻塞至
// ctx 取消且全部探针退出。
func (m *Manager) Start(ctx context.Context) {
	m.StartDynamic(ctx, nil)
}

// StartDynamic 动态间隔驱动全部探针：intervalFn 按目标名取下一间隔（采样
// 策略快照的读侧——热更即换快照，探针下轮自动跟随；异常态提频由调用方在
// fn 内结合探针自身状态叠加）。fn 为 nil 时等价 Start。Start 后 Add 的目标
// 立即按同一 fn 起跑。
func (m *Manager) StartDynamic(ctx context.Context, intervalFn func(target string, now time.Time) time.Duration) {
	m.mu.Lock()
	m.ctx = ctx
	m.intervalFn = intervalFn
	m.running = true
	m.startPendingLocked()
	m.mu.Unlock()

	<-ctx.Done()
	m.stopAll()
	m.wg.Wait()
}

// stopAll 取消全部 per-probe ctx 并退出运行态（之后 Add 不再自动起跑）。
func (m *Manager) stopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.running = false
	for i := range m.items {
		if m.items[i].cancel != nil {
			m.items[i].cancel()
		}
	}
}
