package ops

import (
	"context"
	"errors"

	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"

	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
)

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
		Name:          snap.Name,
		Pid:           snap.Pid,
		State:         supervisorStateString(snap.State),
		UptimeSeconds: snap.UptimeSeconds,
		RestartCount:  snap.RestartCount,
		RssBytes:      snap.RssBytes,
		CpuPercent:    snap.CpuPercent,
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
