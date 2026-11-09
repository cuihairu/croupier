package server

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/tasks"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
	"google.golang.org/protobuf/proto"
)

func (s *ControlService) handleTaskEvent(ctx context.Context, data []byte) ([]byte, error) {
	req := &sdkv1.TaskEvent{}
	if err := proto.Unmarshal(data, req); err != nil {
		return nil, fmt.Errorf("unmarshal TaskEvent: %w", err)
	}

	s.mu.RLock()
	taskStore := s.taskStore
	s.mu.RUnlock()
	if taskStore == nil {
		return nil, fmt.Errorf("task store not configured")
	}

	taskID := strings.TrimSpace(req.GetTaskId())
	if taskID == "" {
		return nil, fmt.Errorf("task_id is required")
	}

	now := time.Now()
	blockedStatuses := tasks.TerminalStatuses()
	updates := map[string]interface{}{
		"progress": req.GetProgress(),
		"message":  req.GetMessage(),
	}

	switch strings.ToLower(strings.TrimSpace(req.GetType())) {
	case string(tasks.EventStarted):
		blockedStatuses = append(blockedStatuses, tasks.StatusCancelRequested)
		updates["status"] = tasks.StatusRunning
		updates["started_at"] = &now
	case string(tasks.EventProgress), string(tasks.EventLog):
		blockedStatuses = append(blockedStatuses, tasks.StatusCancelRequested)
		updates["status"] = tasks.StatusRunning
	case string(tasks.EventCompleted):
		updates["status"] = tasks.StatusSucceeded
		updates["progress"] = int32(100)
		updates["finished_at"] = &now
		updates["result_payload"] = model.EncodeTaskPayload(req.GetPayload())
	case string(tasks.EventFailed):
		updates["status"] = tasks.StatusFailed
		updates["finished_at"] = &now
		updates["error_message"] = req.GetMessage()
	case string(tasks.EventCancelRequested):
		blockedStatuses = append(blockedStatuses, tasks.StatusCancelRequested)
		updates["status"] = tasks.StatusCancelRequested
		updates["cancel_requested_at"] = &now
	case string(tasks.EventCancelled):
		updates["status"] = tasks.StatusCancelled
		updates["finished_at"] = &now
	default:
		blockedStatuses = append(blockedStatuses, tasks.StatusCancelRequested)
		updates["status"] = tasks.StatusRunning
	}

	// R30-03（ARCH-015）：状态更新与事件追加在单一事务内完成（RecordTaskEvent），
	// 任一失败整体回滚——不再有「task_runs 已终态但 task_events 缺失」的部分
	// 写入被终态 guard 静默 ACK。终态 guard 拦截时（updated=false）不追加事件，
	// 终态任务保持终态，任务流在终态事件后不再暴露误导性事件。
	if _, err := taskStore.RecordTaskEvent(ctx, taskID, blockedStatuses, updates, tasks.EventType(req.GetType()), req.GetProgress(), req.GetMessage(), req.GetPayload()); err != nil {
		return nil, fmt.Errorf("record task event: %w", err)
	}
	return nil, nil
}

// handleMetricEvent accepts a pushed MetricsReport snapshot from an agent.
// The report is stored in the MetricsStore for later retrieval by the ops API.
func (s *ControlService) handleMetricEvent(ctx context.Context, data []byte) ([]byte, error) {
	req := &opsv1.MetricsReport{}
	if err := proto.Unmarshal(data, req); err != nil {
		return nil, fmt.Errorf("unmarshal MetricsReport: %w", err)
	}

	agentID := strings.TrimSpace(req.GetAgentId())
	if agentID == "" {
		return nil, fmt.Errorf("agent_id is required")
	}

	// Store metrics for later retrieval
	if s.metricsStore != nil {
		s.metricsStore.Add(agentID, req)
	}
	if s.logStore != nil {
		s.logStore.Ingest(agentID, req.GetLogs())
	}

	if s.logger != nil {
		s.logger.Debug("received metrics report",
			"agent_id", agentID,
			"cpu_percent", req.GetCpu().GetUsagePercent(),
			"mem_used_bytes", req.GetMemory().GetUsedBytes(),
			"disk_count", len(req.GetDisks()),
			"network_count", len(req.GetNetworks()),
		)
	}
	return nil, nil
}

func (s *ControlService) normalizeMetricEvent(data []byte, identity *AgentIdentity) (*opsv1.MetricsReport, error) {
	req := &opsv1.MetricsReport{}
	if err := proto.Unmarshal(data, req); err != nil {
		return nil, fmt.Errorf("unmarshal MetricsReport: %w", err)
	}
	req.AgentId = identity.AgentID
	return req, nil
}

func (s *ControlService) handleMetricEventAsAgent(ctx context.Context, data []byte, identity *AgentIdentity) ([]byte, error) {
	req, err := s.normalizeMetricEvent(data, identity)
	if err != nil {
		return nil, err
	}
	normalized, err := proto.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal MetricsReport: %w", err)
	}
	return s.handleMetricEvent(ctx, normalized)
}
