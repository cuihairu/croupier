package svc

// server 本机探针装配（agent-health-recovery-review §4.2/§4.3，R3 server 侧
// 内核装配）：探测「只有 server 视角才是运维真值」的目标——server 自身依赖
//（server 库/内部 API）与只有 server 能访问的第三方平台 API。实现层与三
// agent 共用 core/healthprobe（零第二套），实例层独立装配；窗口/明细事件经
// 上行桥组包后回调入站（与 agent 上行同一条 Alert 生命周期，视角标注=server）。

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/cuihairu/croupier/core/healthprobe"
	"github.com/cuihairu/croupier/core/report"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	"google.golang.org/protobuf/proto"
)

// ProbeEventSend server 本机探针事件入站回调（生产=ControlService 的探测
// 事件入口：诊断环 + 窗口→Alert 生命周期；测试注入）。本机消费无重传通道，
// 失败语义只记日志，故无错误返回。
type ProbeEventSend func(ctx context.Context, ev *opsv1.AlertEvent)

// ServerProbeAgentID server 视角常量（真值在 core/report：上行桥组包侧与
// server 入站回退共用）。
const ServerProbeAgentID = report.ServerProbeAgentID

// ServerProbeManager server 本机探针面：多目标探针 + 上行桥。空目标集不
// 装配（nil 返回），与现状行为零差异。
type ServerProbeManager struct {
	manager     *healthprobe.Manager
	uplinks     map[string]*report.ProbeUplink
	order       []string
	semByTarget map[string]string
	// typeByTarget/protocolByTarget 目标类型与游戏协议适配器名（配置期静态
	// 声明，随 Statuses 带出——消费方不必解析 last_reason 文本即可区分协议
	// 探测与网络/存储探测，§5.5.3 协议四态的标注面）。
	typeByTarget     map[string]string
	protocolByTarget map[string]string
	// expectedByTarget 目标协议期望声明（§5.5.3 reachable-unverified 触发位）：
	// 纯配置事实，随 Statuses 带出让面板标「可达≠可用 / 未接入协议探测」。
	expectedByTarget map[string]bool
	// phaseMu 守 phaseByTarget：routeResult 由探针 goroutine 并发回调，与
	// Statuses 读侧并发；agent 侧同语义在 ProbeUplink.lastPhase。
	phaseMu       sync.Mutex
	phaseByTarget map[string]string
	logger        *slog.Logger
}

// NewServerProbeManager 装配 server 本机探针面。目标非法（重名/类型未知/
// db Checker 未注册驱动）整体报错——配置配错时启动失败，与 agent 探针面
// 同口径（观测面配错静默不探比报错危险）。目标集为空返回 (nil, nil)。
func NewServerProbeManager(cfgs []healthprobe.TargetConfig, send ProbeEventSend, maxDetailsPerWindow int, logger *slog.Logger) (*ServerProbeManager, error) {
	if len(cfgs) == 0 {
		return nil, nil
	}
	if logger == nil {
		logger = slog.Default()
	}
	if send == nil {
		send = func(context.Context, *opsv1.AlertEvent) {}
	}

	// 上行桥的 send 收的是 proto 序列化正文（与 agent 经 wire 上行同构），
	// 此处解包后走本机回调——组包与消费两侧与 agent 路径完全同构，无第二套。
	// 解包失败只吞（记日志会刷屏，组包侧同进程不可能产出坏包）。
	wrappedSend := func(ctx context.Context, body []byte) error {
		ev := &opsv1.AlertEvent{}
		if err := proto.Unmarshal(body, ev); err != nil {
			return nil
		}
		send(ctx, ev)
		return nil
	}

	uplinks := make(map[string]*report.ProbeUplink, len(cfgs))
	semByTarget := make(map[string]string, len(cfgs))
	typeByTarget := make(map[string]string, len(cfgs))
	protocolByTarget := make(map[string]string, len(cfgs))
	expectedByTarget := make(map[string]bool, len(cfgs))
	order := make([]string, 0, len(cfgs))
	for _, tc := range cfgs {
		name := strings.TrimSpace(tc.Name)
		if _, dup := uplinks[name]; dup {
			return nil, fmt.Errorf("server probe target %q duplicated", name)
		}
		sem := tc.Semantics
		if sem == "" {
			sem = healthprobe.SemanticsReadiness
		}
		uplinks[name] = report.NewProbeUplink(report.ProbeUplinkConfig{
			AgentID:             report.ServerProbeAgentID,
			Target:              name,
			Semantics:           sem,
			ProbeType:           string(tc.Type),
			Protocol:            tc.Protocol,
			ProtocolExpected:    tc.ProtocolExpected,
			MaxDetailsPerWindow: maxDetailsPerWindow,
		}, wrappedSend)
		semByTarget[name] = string(sem)
		typeByTarget[name] = string(tc.Type)
		protocolByTarget[name] = tc.Protocol
		expectedByTarget[name] = tc.ProtocolExpected
		order = append(order, name)
	}

	m := &ServerProbeManager{uplinks: uplinks, order: order, semByTarget: semByTarget,
		typeByTarget: typeByTarget, protocolByTarget: protocolByTarget,
		expectedByTarget: expectedByTarget,
		phaseByTarget:    map[string]string{}, logger: logger}
	manager, err := healthprobe.NewManager(cfgs, "", "", m.routeWindow, m.routeResult)
	if err != nil {
		return nil, err
	}
	m.manager = manager
	return m, nil
}

// routeWindow 窗口事件分流到目标上行桥（manager 给的是共享回调）。
func (m *ServerProbeManager) routeWindow(ev healthprobe.WindowEvent) {
	if up, ok := m.uplinks[ev.Window.Target]; ok {
		up.OnWindow(ev)
	}
}

// routeResult 单次结果分流：失败明细即时（不对称上报 §13.1）。
func (m *ServerProbeManager) routeResult(res healthprobe.Result) {
	if up, ok := m.uplinks[res.Target]; ok {
		up.OnResult(res)
	}
	// 失败阶段标记留存（与 ProbeUplink.lastPhase 同语义）：只在失败时写，
	// 成功不覆盖——口径是「最近一次失败的阶段」，与 lastReason 一致。
	if !res.OK {
		m.phaseMu.Lock()
		m.phaseByTarget[res.Target] = res.Details["phase"]
		m.phaseMu.Unlock()
	}
}

// Targets 已装配的目标名（顺序同配置声明）。
func (m *ServerProbeManager) Targets() []string {
	if m == nil {
		return nil
	}
	return append([]string(nil), m.order...)
}

// ServerProbeStatus 单目标当前状态快照（ops 汇总页消费面）。
type ServerProbeStatus struct {
	Target     string
	Semantics  string
	Available  bool
	WindowOpen bool
	DurationMS int64
	LastReason string
	// ProbeType/Protocol 目标类型与游戏协议适配器名；Phase 最近一次失败的
	// 阶段标记（协议 Detailing 出口，无 phase 的 checker 为空）。三者构成
	// §5.5.3「协议四态」的标注面：不可达 / 握手失败 / 已验证。
	ProbeType string
	Protocol  string
	Phase     string
	// ProtocolExpected 目标声明了协议期望（reachable-unverified 触发位）。
	ProtocolExpected bool
}

// Statuses 全部 server 目标的当前状态快照（读侧不探测——呈现现状不注入
// 新事实，与 agent 侧「读 probe_stats 快照」同口径）。
func (m *ServerProbeManager) Statuses() []ServerProbeStatus {
	if m == nil || m.manager == nil {
		return nil
	}
	out := make([]ServerProbeStatus, 0, len(m.order))
	for _, p := range m.manager.Probes() {
		name := p.Target()
		m.phaseMu.Lock()
		phase := m.phaseByTarget[name]
		m.phaseMu.Unlock()
		st := ServerProbeStatus{
			Target:           name,
			Semantics:        m.semByTarget[name],
			Available:        p.Available(),
			ProbeType:        m.typeByTarget[name],
			Protocol:         m.protocolByTarget[name],
			Phase:            phase,
			ProtocolExpected: m.expectedByTarget[name],
		}
		if w, open := p.CurrentWindow(); open {
			st.WindowOpen = true
			st.DurationMS = w.DurationMS
			st.LastReason = w.LastReason
		}
		out = append(out, st)
	}
	return out
}

// Start 启动全部探针（阻塞至 ctx 取消）。间隔走本地配置声明（probe.policy
// 采样策略只作用于 agent 侧采样面——server 探的是自身依赖，无下发语义）。
func (m *ServerProbeManager) Start(ctx context.Context) {
	if m == nil || m.manager == nil {
		return
	}
	m.manager.Start(ctx)
}

// ProbeOnceByName 手动诊断（§4.2）：对已配置目标即时探一次并返回结果。
// 目标未配置返回 ok=false（404 语义由 API 层落）。探测结果同样过窗口状态
// 机与上行桥——手动诊断与周期探测共用一份事实。
func (m *ServerProbeManager) ProbeOnceByName(target string) (healthprobe.Result, bool) {
	if m == nil || m.manager == nil {
		return healthprobe.Result{}, false
	}
	for _, p := range m.manager.Probes() {
		if p.Target() == target {
			return p.ProbeOnce(context.Background()), true
		}
	}
	return healthprobe.Result{}, false
}
