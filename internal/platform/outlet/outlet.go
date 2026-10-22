// Package outlet 实现告警出口抽象（plugin-mechanism 设计 §5.1）：事件源
// 只产统一信封（AlertEvent），出口管理器按注册链投递——站内出口（internal）
// 链首恒投递、零外发、不受降级影响；外部出口按序尝试，Retryable 失败滑
// 下一个，链尾失败计数+日志（禁静默丢弃）。herald = 默认外部出口选项
// （herald.go）；noop = 外部默认 no-op 注册项（零配置=零外发红线）。
package outlet

import (
	"context"
	"errors"
	"log/slog"
	"sort"
	"strings"
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
	// KindIncidentReport 周期报表推送（incident-reports §6 分发链）。
	KindIncidentReport = "incident-report"
)

// 告警严重度闭集（三档；出口自行映射紧急度）。
const (
	SeverityCritical = "critical"
	SeverityWarning  = "warning"
	SeverityInfo     = "info"
)

// 出口注册表键（§5.1 注册表；Name() 返回值即配置选择键）。
const (
	// OutletInternal 站内出口：链首内建项，默认必有、不可删、零外发。
	OutletInternal = "internal"
	// OutletNoop 外部默认 no-op：无外部出口配置时的静默真实注册项。
	OutletNoop = "noop"
)

// 投递重试参数：有限重试防出口拖垮告警链；间隔固定短退避即可（出口
// 不可达是部署级事件，指数退避收益有限）。
const (
	dispatchAttempts   = 3
	dispatchRetryDelay = 2 * time.Second
	dispatchTimeout    = 15 * time.Second
)

// DefaultStationRecipient 站内出口默认接收组（事件无 Target 时的兜底
// 受众；无归属类别的通知归此组，不落空——incident-reports §6）。
const DefaultStationRecipient = "group:gm-ops"

// Scope 是可空的 scope 三元组（出口按 scope 分流受众时启用；v1 事件源
// 至少带 agentId）。CategorySlug 供报表分片携带类别上下文（站内通知的
// scope 落 messages.Scope，读时按类别受众过滤——incident-reports §6）。
type Scope struct {
	GameID       string
	Env          string
	AgentID      string
	CategorySlug string
}

// 信封标注键（AlertEvent.Metadata；维护状态 gate 注入，server-status
// provider 设计 §5）。
const (
	// MetaSourceUnknown 状态源不可达/映射不到（fail-open 照报的取证标注）。
	MetaSourceUnknown = "source_unknown"
	// MetaMaintenanceSuppressed 事件被维护状态 gate 抑制（整链静默；
	// 抑制事件不落出口，故此键只出现在日志与 gate 计数里）。
	MetaMaintenanceSuppressed = "maintenance_suppressed"
)

// ErrSuppressedByMaintenance 事件被维护状态 gate 抑制（DispatchExternal
// 的收尾回执；调用方按「未投递+可解释原因」处理）。
var ErrSuppressedByMaintenance = errors.New("suppressed by maintenance gate")

// AlertEvent 是出口间共享的统一告警事件信封（plugin-mechanism §5.2）。
type AlertEvent struct {
	Kind             string // kind 闭集
	Severity         string // severity 闭集
	EventID          string // croupier 自有事件主键（进幂等）
	DedupKey         string // 出口侧去重键（同键同态折叠，状态翻转重发）
	Title            string // 摘要文本（v1 直投，不用模板）
	Body             string
	OccurredAtUnixMs int64             // 事件时间
	Scope            Scope             // 可空三元组
	Target           string            // 受众 ref（如 category-leader:<slug> / user:<id>）；空=出口默认受众
	Metadata         map[string]string // 信封扩展位（gate 标注；出口自行渲染）
}

// DeliveryOutcome 出口回执：降级链判定与失败计数的依据（§5.1）。
type DeliveryOutcome struct {
	Delivered bool   // 是否送达（回执语义由实现定义）
	Channel   string // 实际投递渠道（herald 品类 / noop / internal）
	Retryable bool   // 失败是否可重试（Permanent 短路分类沿用 M2）
}

// Outlet 是出口接口位（供应商可插拔：新增出口 = 新增一个实现 + 配置
// 一行，事件词汇与告警管线零改动）。Deliver 返回回执+错误：错误非 nil
// 即投递失败（重试由管理器负责）；实现必须非阻塞友好（内部队列或快速
// 返回）。
type Outlet interface {
	// Name 是注册表键（"internal" / "noop" / "herald" / "webhook"）。
	Name() string
	Deliver(ctx context.Context, ev AlertEvent) (DeliveryOutcome, error)
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

// GateFunc 是维护状态 gate 钩子（server-status provider 设计 §5）：出口
// 链入口的投递前裁决。suppress=true → 整链静默（含站内，落库不变）；
// meta 非空 → 合入 AlertEvent.Metadata（source_unknown 等，fail-open 照报
// 时的取证标注）。outlet 包不依赖 serverstatus——策略由 svc 装配层注入。
type GateFunc func(ctx context.Context, ev AlertEvent) (suppress bool, meta map[string]string)

// Manager 按注册链投递信封：站内出口（internal）恒投递且不受降级影响；
// 外部出口按序尝试，Retryable 失败滑下一个，成功即停，Permanent 拒绝
// 终止链。链尾仍失败的计数+日志由 deliverOne 逐出口负责（§5.4 禁静默
// 丢弃）。
type Manager struct {
	mu       chan struct{} // 序列化 outlets 切片变更（Register 与快照读）
	outlets  []Outlet
	failures atomic.Int64             // 投递失败累计（面板/日志观测用）
	gate     atomic.Pointer[GateFunc] // 维护状态 gate 钩子（未挂=直通）
	sleep    func(time.Duration)
}

// NewManager creates an empty manager. 出口经 Register 注册后生效。
func NewManager() *Manager {
	return &Manager{mu: make(chan struct{}, 1), sleep: time.Sleep}
}

// Register adds one outlet；nil 忽略。链序 = 注册序（配置侧保证 internal
// 链首、外部按主→备顺序注册）。
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

// SetGate 挂维护状态 gate 钩子（启动期装配一次；nil 卸载=直通）。
func (m *Manager) SetGate(fn GateFunc) {
	if fn == nil {
		m.gate.Store(nil)
		return
	}
	m.gate.Store(&fn)
}

// applyGate 投递前评估 gate（设计 §1.2）：只对带服务器定位（Scope.AgentID
// 非空）的事件生效——无法定位服务器的事件不过 gate 照常投递（宁多报不
// 漏报）。meta 合入事件时写时复制，不改调用方信封。
func (m *Manager) applyGate(ctx context.Context, ev AlertEvent) (AlertEvent, bool) {
	p := m.gate.Load()
	if p == nil || *p == nil || ev.Scope.AgentID == "" {
		return ev, false
	}
	suppress, meta := (*p)(ctx, ev)
	if len(meta) == 0 {
		return ev, suppress
	}
	out := ev
	out.Metadata = make(map[string]string, len(ev.Metadata)+len(meta))
	for k, v := range ev.Metadata {
		out.Metadata[k] = v
	}
	for k, v := range meta {
		out.Metadata[k] = v
	}
	return out, suppress
}

// Dispatch 异步投递一条信封：整链一个 goroutine（站内恒投递 + 外部链式
// 降级），panic recover 兜底——出口不可用绝不阻塞告警链，失败只计数与
// 记日志。goroutine 内先过维护状态 gate（抑制=整链静默含站内，落库不变；
// gate 已有计数+日志留痕）。
func (m *Manager) Dispatch(ev AlertEvent) {
	m.mu <- struct{}{}
	snapshot := make([]Outlet, len(m.outlets))
	copy(snapshot, m.outlets)
	<-m.mu
	go func() {
		defer func() { _ = recover() }()
		ev, suppress := m.applyGate(context.Background(), ev)
		if suppress {
			return
		}
		m.deliverChain(snapshot, ev)
	}()
}

// DispatchExternal 同步只投外部链（incident-reports §6 分发用）：站内
// 通知由调用方经 MessageSink 直写（报表分片的收件人是 leader 账号，出口
// 链无法感知类别→leader 映射），此路径跳过站内出口——外部链语义同
// deliverChain：成功即停，Retryable 失败滑下一个，Permanent 终止。返回
// 实际收尾出口的回执（报表 PushStatus 落库依据）。带服务器定位的事件先
// 过维护状态 gate：抑制返回 ErrSuppressedByMaintenance（v1 报表事件无
// agentId 不过 gate，本分支为 agent 定位类外部事件预留）。
func (m *Manager) DispatchExternal(ctx context.Context, ev AlertEvent) (DeliveryOutcome, error) {
	m.mu <- struct{}{}
	snapshot := make([]Outlet, len(m.outlets))
	copy(snapshot, m.outlets)
	<-m.mu
	defer func() { _ = recover() }()
	ev, suppress := m.applyGate(ctx, ev)
	if suppress {
		return DeliveryOutcome{}, ErrSuppressedByMaintenance
	}
	var lastOutcome DeliveryOutcome
	var lastErr error
	for _, o := range snapshot {
		if o.Name() == OutletInternal {
			continue
		}
		outcome, err := m.deliverOne(o, ev)
		lastOutcome, lastErr = outcome, err
		if err == nil && outcome.Delivered {
			return outcome, nil
		}
		if err != nil && isPermanent(err) {
			return outcome, err
		}
		// 无错未送达（出口自报未达）或 Retryable 失败：滑下一个。
	}
	// 链耗尽：返回最后一个外部出口的收尾回执（PushStatus 落库依据）。
	return lastOutcome, lastErr
}

// deliverChain 链式降级（§5.1）：站内出口先投（默认必有、零外发、不受
// 降级影响）；外部出口按序尝试——成功即停，Retryable 失败滑下一个，
// Permanent 拒绝终止链（重试无意义）。
func (m *Manager) deliverChain(outlets []Outlet, ev AlertEvent) {
	defer func() { _ = recover() }()
	for _, o := range outlets {
		if o.Name() == OutletInternal {
			m.deliverOne(o, ev)
		}
	}
	for _, o := range outlets {
		if o.Name() == OutletInternal {
			continue
		}
		outcome, err := m.deliverOne(o, ev)
		if err == nil && outcome.Delivered {
			return
		}
		if err != nil && isPermanent(err) {
			return
		}
	}
}

// deliverOne 单出口投递：有限重试 + Permanent 短路；最终失败计数+日志。
func (m *Manager) deliverOne(o Outlet, ev AlertEvent) (DeliveryOutcome, error) {
	defer func() { _ = recover() }()
	var lastErr error
	var lastOutcome DeliveryOutcome
	for attempt := 0; attempt < dispatchAttempts; attempt++ {
		if attempt > 0 {
			m.sleep(dispatchRetryDelay)
		}
		ctx, cancel := context.WithTimeout(context.Background(), dispatchTimeout)
		outcome, err := o.Deliver(ctx, ev)
		cancel()
		lastOutcome, lastErr = outcome, err
		if err == nil {
			return outcome, nil
		}
		if isPermanent(err) {
			break
		}
	}
	m.failures.Add(1)
	slog.WarnContext(context.Background(), "outlet: deliver failed after retries",
		"outlet", o.Name(), "kind", ev.Kind, "eventId", ev.EventID, "error", lastErr)
	return lastOutcome, lastErr
}

// StationNotice 是站内通知的落库载荷（messages 行语义；outlet 包不依赖
// model，经 MessageSink 由 svc 布线注入持久化实现）。
type StationNotice struct {
	To      string // 接收者（user:<id> 去前缀 / 组 ref）
	Type    string // 通知类型：alert|report|system
	Title   string
	Content string
	Level   string         // info|warn|critical（按事件 severity 映射）
	Source  string         // 产生方（agentId / incident_report / system）
	RefType string         // 关联类型：alert|report|event
	RefID   string         // 关联对象 ID（EventID）
	Scope   map[string]any // 可见范围上下文（gameId/env/agentId 非空项）
}

// MessageSink 是站内出口的消息落库面（svc 布线注入，outlet 包零持久化
// 依赖）。
type MessageSink interface {
	Create(ctx context.Context, n StationNotice) error
}

// InternalOutlet 站内出口（§5.1 链首内建项）：信封→StationNotice→
// messages 表，带 level/type/source/ref/scope 全套字段；零外发。
type InternalOutlet struct {
	sink             MessageSink
	defaultRecipient string
}

// NewInternalOutlet builds the station outlet. defaultRecipient 传空取
// DefaultStationRecipient。
func NewInternalOutlet(sink MessageSink, defaultRecipient string) *InternalOutlet {
	if defaultRecipient == "" {
		defaultRecipient = DefaultStationRecipient
	}
	return &InternalOutlet{sink: sink, defaultRecipient: defaultRecipient}
}

// Name implements Outlet.
func (o *InternalOutlet) Name() string { return OutletInternal }

// Deliver maps the envelope onto one station notice. 落库错误按可重试
// 分类（DB 抖动重试有意义）。正文尾随 gate 标注（source_unknown 等）。
func (o *InternalOutlet) Deliver(ctx context.Context, ev AlertEvent) (DeliveryOutcome, error) {
	n := StationNotice{
		To:      stationRecipient(ev.Target, o.defaultRecipient),
		Type:    noticeTypeForKind(ev.Kind),
		Title:   ev.Title,
		Content: ev.Body + metadataSuffix(ev.Metadata),
		Level:   noticeLevelForSeverity(ev.Severity),
		Source:  noticeSource(ev),
		RefType: noticeRefTypeForKind(ev.Kind),
		RefID:   ev.EventID,
		Scope:   scopeMap(ev),
	}
	if err := o.sink.Create(ctx, n); err != nil {
		return DeliveryOutcome{Delivered: false, Channel: OutletInternal, Retryable: true}, err
	}
	return DeliveryOutcome{Delivered: true, Channel: OutletInternal}, nil
}

// stationRecipient 解析 Target：user:<id> / category-leader:<slug> 去前缀
// 直投账号；其余（含空）落默认接收组。
func stationRecipient(target, def string) string {
	for _, prefix := range []string{"user:", "category-leader:"} {
		if len(target) > len(prefix) && target[:len(prefix)] == prefix {
			return target[len(prefix):]
		}
	}
	return def
}

// noticeTypeForKind kind→通知类型（闭集映射；未知 kind 兜底 system）。
func noticeTypeForKind(kind string) string {
	switch kind {
	case KindIncidentReport:
		return "report"
	case KindAlertInbound, KindCaptureRuleHit, KindDevopsBuildFailed,
		KindSupervisorBreaker, KindProbeUnavailable, KindProbeRecovered:
		return "alert"
	default:
		return "system"
	}
}

// noticeRefTypeForKind kind→关联类型。
func noticeRefTypeForKind(kind string) string {
	switch kind {
	case KindIncidentReport:
		return "report"
	case KindAlertInbound:
		return "alert"
	default:
		return "event"
	}
}

// noticeLevelForSeverity 事件 severity→通知 level（§5.1 分级映射）。
func noticeLevelForSeverity(severity string) string {
	switch severity {
	case SeverityCritical:
		return "critical"
	case SeverityWarning:
		return "warn"
	default:
		return "info"
	}
}

// noticeSource 通知产生方：agent 事件带 AgentID；报表推送固定
// incident_report；其余 system。
func noticeSource(ev AlertEvent) string {
	if ev.Scope.AgentID != "" {
		return ev.Scope.AgentID
	}
	if ev.Kind == KindIncidentReport {
		return "incident_report"
	}
	return "system"
}

// metadataSuffix 把信封标注渲染成正文尾注（魗典序，确定性输出）：
// fail-open 照报时告知接收方状态源不可达（source_unknown）——标注到达人
// 的最小面（站内正文尾 + herald 正文尾），不动 messages schema。
func metadataSuffix(meta map[string]string) string {
	if len(meta) == 0 {
		return ""
	}
	keys := make([]string, 0, len(meta))
	for k := range meta {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return " [" + strings.Join(keys, " ") + "]"
}

// scopeMap 信封 scope→可见范围 map（非空项才进）。
func scopeMap(ev AlertEvent) map[string]any {
	out := make(map[string]any, 4)
	if ev.Scope.GameID != "" {
		out["gameId"] = ev.Scope.GameID
	}
	if ev.Scope.Env != "" {
		out["env"] = ev.Scope.Env
	}
	if ev.Scope.AgentID != "" {
		out["agentId"] = ev.Scope.AgentID
	}
	if ev.Scope.CategorySlug != "" {
		out["categorySlug"] = ev.Scope.CategorySlug
	}
	return out
}

// NoopOutlet 外部默认出口（§5.1 第一原则）：无外部出口配置时的真实注册
// 项——零配置=零外发，回执成功（链终止于静默）。
type NoopOutlet struct{}

// NewNoopOutlet builds the no-op outlet.
func NewNoopOutlet() *NoopOutlet { return &NoopOutlet{} }

// Name implements Outlet.
func (NoopOutlet) Name() string { return OutletNoop }

// Deliver implements Outlet.
func (NoopOutlet) Deliver(context.Context, AlertEvent) (DeliveryOutcome, error) {
	return DeliveryOutcome{Delivered: true, Channel: OutletNoop}, nil
}
