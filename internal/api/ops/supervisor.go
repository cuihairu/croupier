package ops

import (
	"context"
	"errors"
	"time"

	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"

	"github.com/cuihairu/croupier/internal/logic/ops"
	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
)

// defaultSupervisorLogMaxBytes 日志下载缺省 tail 上限（512KB）。
const defaultSupervisorLogMaxBytes = 512 * 1024

// supervisorStateString 把 proto ProcessState 映射为面板契约的小写状态串。
// 闭集映射，未知值显式落 unknown（proto 加新枚举时此处同步）。
func supervisorStateString(s opsv1.ProcessState) string {
	switch s {
	case opsv1.ProcessState_PROCESS_STATE_RUNNING:
		return "running"
	case opsv1.ProcessState_PROCESS_STATE_STOPPED:
		return "stopped"
	case opsv1.ProcessState_PROCESS_STATE_FAILED:
		return "failed"
	case opsv1.ProcessState_PROCESS_STATE_STARTING:
		return "starting"
	case opsv1.ProcessState_PROCESS_STATE_STOPPING:
		return "stopping"
	case opsv1.ProcessState_PROCESS_STATE_BACKOFF:
		return "backoff"
	case opsv1.ProcessState_PROCESS_STATE_BROKEN:
		return "broken"
	default:
		return "unknown"
	}
}

// summarizeSupervised 聚合状态灯：任一 FAILED/BROKEN → error；其余存在非
// RUNNING 或带超限标记 → warn；空集/全 RUNNING 无标记 → ok。
func summarizeSupervised(procs []*opsv1.SupervisedProcessSnapshot) OpsSupervisorSummary {
	summary := OpsSupervisorSummary{Status: "ok", Total: len(procs)}
	if len(procs) == 0 {
		return summary
	}
	for _, p := range procs {
		if p == nil {
			continue
		}
		if p.State == opsv1.ProcessState_PROCESS_STATE_FAILED || p.State == opsv1.ProcessState_PROCESS_STATE_BROKEN {
			summary.Status = "error"
			break
		}
		if p.State == opsv1.ProcessState_PROCESS_STATE_RUNNING {
			summary.Running++
		}
	}
	if summary.Status == "ok" {
		for _, p := range procs {
			if p != nil && (p.State != opsv1.ProcessState_PROCESS_STATE_RUNNING || len(p.Flags) > 0) {
				summary.Status = "warn"
				break
			}
		}
	}
	return summary
}

// supervisedProcessFromSnapshot 把 wire 快照转为面板 DTO（flags 拷贝，防别名）。
func supervisedProcessFromSnapshot(snap *opsv1.SupervisedProcessSnapshot) OpsSupervisedProcess {
	out := OpsSupervisedProcess{
		Name:              snap.Name,
		Pid:               snap.Pid,
		State:             supervisorStateString(snap.State),
		UptimeSeconds:     snap.UptimeSeconds,
		RestartCount:      snap.RestartCount,
		RssBytes:          snap.RssBytes,
		CpuPercent:        snap.CpuPercent,
		LastEventUnix:     snap.LastEventUnix,
		NextRestartAtUnix: snap.NextRestartAtUnix,
	}
	if len(snap.Flags) > 0 {
		out.Flags = make([]string, len(snap.Flags))
		copy(out.Flags, snap.Flags)
	} else {
		out.Flags = []string{}
	}
	return out
}

// agentSupervisorFromEntry 从最新 metrics 上报构建 supervisor 视图。
// agent 从未上报（无 entry）时返回空 processes + ok 汇总——面板以
// timestamp 缺失/陈旧判断数据新鲜度，不在此处硬 404。
func agentSupervisorFromEntry(agentID string, entry *registry.MetricsEntry) *OpsAgentSupervisorResponse {
	resp := &OpsAgentSupervisorResponse{
		AgentID:   agentID,
		Processes: []OpsSupervisedProcess{},
		Summary:   OpsSupervisorSummary{Status: "ok"},
	}
	if entry == nil || entry.Report == nil {
		return resp
	}
	resp.Timestamp = utils.FormatTimestamp(entry.Received)
	for _, snap := range entry.Report.SupervisedProcesses {
		if snap == nil {
			continue
		}
		resp.Processes = append(resp.Processes, supervisedProcessFromSnapshot(snap))
	}
	resp.Summary = summarizeSupervised(entry.Report.SupervisedProcesses)
	return resp
}

func opsAgentSupervisor(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsAgentSupervisorRequest) (*OpsAgentSupervisorResponse, error) {
	if svcCtx == nil || svcCtx.MetricsStore == nil {
		return nil, errors.New("metrics store unavailable")
	}
	entry, _ := svcCtx.MetricsStore.GetLatest(req.AgentID)
	return agentSupervisorFromEntry(req.AgentID, entry), nil
}

// supervisorEventFromProto 把 wire 事件转为面板 DTO（时间戳走统一格式化，
// 0 值时间输出空串）。
func supervisorEventFromProto(ev *opsv1.SupervisorEvent) OpsSupervisorEvent {
	out := OpsSupervisorEvent{
		Seq:          ev.Seq,
		Process:      ev.Process,
		Event:        ev.Event,
		OldPid:       ev.OldPid,
		NewPid:       ev.NewPid,
		ExitCode:     ev.ExitCode,
		Signal:       ev.Signal,
		RestartCount: ev.RestartCount,
		Message:      ev.Message,
		LastError:    ev.LastError,
		OomSuspect:   ev.OomSuspect,
		LastRssBytes: ev.LastRssBytes,
	}
	if ev.TsUnix > 0 {
		out.Ts = utils.FormatTimestamp(time.Unix(ev.TsUnix, 0))
	}
	if ev.LastHeartbeatUnix > 0 {
		out.LastHeartbeat = utils.FormatTimestamp(time.Unix(ev.LastHeartbeatUnix, 0))
	}
	return out
}

func opsAgentSupervisorEvents(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsAgentSupervisorEventsRequest) (*OpsAgentSupervisorEventsResponse, error) {
	if svcCtx == nil || svcCtx.MetricsStore == nil {
		return nil, errors.New("metrics store unavailable")
	}
	evs := svcCtx.MetricsStore.GetSupervisorEvents(req.AgentID, req.SinceSeq, req.Limit)
	resp := &OpsAgentSupervisorEventsResponse{
		AgentID:   req.AgentID,
		Events:    []OpsSupervisorEvent{},
		LatestSeq: svcCtx.MetricsStore.SupervisorLatestSeq(req.AgentID),
	}
	for _, ev := range evs {
		if ev != nil {
			resp.Events = append(resp.Events, supervisorEventFromProto(ev))
		}
	}
	return resp, nil
}

// opsAgentSupervisorLog 代理拉取 agent 本地事件日志文件（tail 截断）。响应
// 走文件下载而非 JSON envelope（CLAUDE.md API 契约显式豁免类）。agent 会话
// 不存在时返回 404 语义错误。
func opsAgentSupervisorLog(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsAgentSupervisorLogRequest) (*opsv1.GetSupervisorLogResponse, error) {
	if svcCtx == nil {
		return nil, errors.New("service context unavailable")
	}
	client := ops.GetAgentOpsClient()
	wrapper, err := client.GetClient(ctx, req.AgentID)
	if err != nil {
		return nil, errors.New("ops client unavailable: " + err.Error())
	}
	maxBytes := req.MaxBytes
	if maxBytes <= 0 {
		maxBytes = defaultSupervisorLogMaxBytes
	}
	return wrapper.GetSupervisorLog(ctx, &opsv1.GetSupervisorLogRequest{MaxBytes: int32(maxBytes)})
}

// attachSupervisorSummaries 给 agent 列表逐项补 supervisor 聚合灯（零额外
// 请求：数据来自 MetricsStore 最新一报的 SupervisedProcesses）。store 为空
// 或该 agent 无上报时字段保持 nil，前端展示「-」。
func attachSupervisorSummaries(svcCtx *svc.ServiceContext, agents []OpsAgentInfo) {
	if svcCtx == nil || svcCtx.MetricsStore == nil || len(agents) == 0 {
		return
	}
	for i := range agents {
		entry, ok := svcCtx.MetricsStore.GetLatest(agents[i].AgentID)
		if !ok || entry == nil || entry.Report == nil {
			continue
		}
		summary := summarizeSupervised(entry.Report.SupervisedProcesses)
		agents[i].Supervisor = &summary
	}
}
