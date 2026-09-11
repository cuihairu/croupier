package main

import (
	"context"
	"fmt"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/dispatch"
)

// taskRunAgentLookup 实现 dispatch.TaskAgentLookup：从共享 task_runs 行
// 解析任务所在 agent。跨实例取消兜底用——task routing（内存 map + 本地
// taskStore）是 per-instance 的，取消请求落到非发起实例时两者都 miss，
// 共享库行是唯一可靠来源（发起实例 dispatch 时写入 AgentID）。
type taskRunAgentLookup struct {
	m *model.TaskRunModel
}

func (l *taskRunAgentLookup) AgentForTask(ctx context.Context, taskID string) (string, error) {
	if l == nil || l.m == nil {
		return "", fmt.Errorf("task run model unavailable")
	}
	run, err := l.m.FindByTaskID(ctx, taskID)
	if err != nil {
		return "", err
	}
	if run == nil || run.AgentID == "" {
		return "", fmt.Errorf("task run %s has no agent", taskID)
	}
	return run.AgentID, nil
}

var _ dispatch.TaskAgentLookup = (*taskRunAgentLookup)(nil)
