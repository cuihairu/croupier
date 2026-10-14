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
	logger      *slog.Logger
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
			MaxDetailsPerWindow: maxDetailsPerWindow,
		}, wrappedSend)
		semByTarget[name] = string(sem)
		order = append(order, name)
	}

	m := &ServerProbeManager{uplinks: uplinks, order: order, semByTarget: semByTarget, logger: logger}
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
}

// Statuses 全部 server 目标的当前状态快照（读侧不探测——呈现现状不注入
// 新事实，与 agent 侧「读 probe_stats 快照」同口径）。
func (m *ServerProbeManager) Statuses() []ServerProbeStatus {
	if m == nil || m.manager == nil {
		return nil
	}
	out := make([]ServerProbeStatus, 0, len(m.order))
	for _, p := range m.manager.Probes() {
		st := ServerProbeStatus{
			Target:    p.Target(),
			Semantics: m.semByTarget[p.Target()],
			Available: p.Available(),
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
