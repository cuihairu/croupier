package dispatch

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	apperrors "github.com/cuihairu/croupier/internal/errors"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/telemetry"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/protobuf/proto"
)

func (d *Dispatcher) StartTask(ctx context.Context, functionID string, payload []byte) (string, error) {
	resp, err := d.StartTaskRequest(ctx, &sdkv1.InvokeRequest{
		FunctionId: functionID,
		Payload:    payload,
	})
	if err != nil {
		return "", err
	}
	return resp.GetTaskId(), nil
}

// StartTaskRequest forwards a structured InvokeRequest to the agent StartTask RPC.
func (d *Dispatcher) StartTaskRequest(ctx context.Context, req *sdkv1.InvokeRequest) (*sdkv1.StartTaskResponse, error) {
	if req == nil || req.GetFunctionId() == "" {
		return nil, fmt.Errorf("function id is required")
	}
	ctx = telemetry.ExtractContext(ctx, req.Metadata)
	ctx, span := dispatchTracer().Start(ctx, "function.dispatch.task",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(attribute.String("function.id", req.GetFunctionId())),
	)
	defer span.End()

	// 原始 metadata：每轮重试克隆重注入（与 InvokeRequest 同款），避免
	// 上一轮的 agentId/taskId/trace 泄漏到下一候选（转发链路按原始面重建）。
	origMeta := map[string]string{}
	for k, v := range req.GetMetadata() {
		origMeta[k] = v
	}
	// targeted/hash 路由语义上指定了唯一目标，失败换候选会破坏粘性——
	// 只有默认 lb 路径才有 failover。键名与 API 层写方一致。
	routePinned := strings.TrimSpace(origMeta["targetServiceId"]) != "" ||
		strings.TrimSpace(origMeta["hashKey"]) != ""

	tried := map[string]bool{}
	maxAttempts := 1
	if !routePinned {
		maxAttempts = 3
	}
	d.mu.RLock()
	writer := d.taskRunWriter
	d.mu.RUnlock()

	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		agent, err := d.pickAgentWithRouting(ctx, req.GetFunctionId(), origMeta, tried)
		if err != nil {
			if lastErr != nil {
				// failover 耗尽 = 无可用 agent 的可重试状态 → 503
				// （与 InvokeRequest 语义一致）；cause 链保留
				// errAgentUnreachable 可换候选语义。
				return nil, apperrors.Newf(apperrors.ErrCodeServiceUnavailable, "start_task",
					fmt.Errorf("%w: failover exhausted", lastErr),
					"failover exhausted: %v (last error: %v)", err, lastErr)
			}
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return nil, err
		}
		tried[agent.AgentID] = true
		span.SetAttributes(attribute.String("agent.id", agent.AgentID))

		if d.healthTracker != nil {
			d.healthTracker.IncrementConnections(agent.AgentID)
		}
		resp, taskID, err := d.startTaskOnPickedAgent(ctx, span, agent, req, origMeta, writer)
		if d.healthTracker != nil {
			d.healthTracker.DecrementConnections(agent.AgentID)
		}
		if err == nil {
			if d.healthTracker != nil {
				d.healthTracker.RecordSuccess(agent.AgentID)
			}
			if rt := resp.GetTaskId(); rt != "" {
				span.SetAttributes(attribute.String("task.id", rt))
				d.registerTask(rt, agent.AgentID)
			}
			span.SetStatus(codes.Ok, "")
			return resp, nil
		}
		lastErr = err
		if d.healthTracker != nil {
			d.healthTracker.RecordFailure(agent.AgentID)
		}
		if !errors.Is(err, errAgentUnreachable) {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return nil, err
		}
		// 该 agent 不可达（本地无连接且转发失败/无归属）→ 换下一候选。
		// 失败尝试的 task_runs 行标 failed：投递未到 agent，不会有事件
		// 回流推进状态（best-effort，标记失败不放大故障）。
		if writer != nil && taskID != "" {
			_ = writer.MarkRunFailed(ctx, taskID, err.Error())
		}
	}
	span.RecordError(lastErr)
	span.SetStatus(codes.Error, lastErr.Error())
	return nil, lastErr
}

// startTaskOnPickedAgent executes one async dispatch attempt against the
// selected agent. Returns the response plus the server-side task ID of this
// attempt (empty when the run row could not be created), so the caller can
// mark the failed attempt's row.
func (d *Dispatcher) startTaskOnPickedAgent(ctx context.Context, span trace.Span, agent *reg.AgentSession, req *sdkv1.InvokeRequest, origMeta map[string]string, writer TaskRunWriter) (*sdkv1.StartTaskResponse, string, error) {
	// 原地重置再注入：保持 req.Metadata 的 map 身份不变——调用方
	// （console/审计）在调用后仍读同一 map 拿 agent_id/task_id，换新实例
	// 会让它们读到空值。
	if req.Metadata == nil {
		req.Metadata = map[string]string{}
	}
	meta := req.Metadata
	for k := range meta {
		delete(meta, k)
	}
	for k, v := range origMeta {
		meta[k] = v
	}
	meta["agentId"] = agent.AgentID
	meta = telemetry.InjectContext(ctx, meta)

	// Generate a server-side task ID so events flowing back from the agent
	// can be matched to a task_runs row. This closes the feedback loop.
	// 每轮尝试独立 ID：失败轮的行由调用方标 failed，成功轮的 ID 随帧
	// 到 owner 沿用（caller/owner 共用同一 ID，事件才能匹配）。
	taskID := ""
	if writer != nil {
		taskID = generateTaskID()
		meta["taskId"] = taskID
		span.SetAttributes(attribute.String("task.id", taskID))
		gameID := origMeta["gameId"]
		env := origMeta["env"]
		actor := origMeta["actor"]
		addr := agent.Addr
		traceID := telemetry.TraceIDFromMetadata(origMeta)
		// ARCH-026：task_runs 主记录必须持久化成功才可派发。写失败返回可
		// 定位错误并不得发送 StartTask——否则 agent 侧执行照跑而账本缺失，
		// 事件回流找不到主记录、任务无法取消（不可追踪的孤儿）。DB 故障与
		// agent 无关，错误非 errAgentUnreachable，调用方不会 failover 重试。
		if err := writer.CreateRunWithMeta(ctx, taskID, req.GetFunctionId(), agent.AgentID, gameID, env, "dispatching", actor, addr, traceID, req.GetPayload()); err != nil {
			err = fmt.Errorf("persist task_runs row for task %s before dispatch: %w", taskID, err)
			span.RecordError(err)
			return nil, taskID, err
		}
	}
	req.Metadata = meta

	reqBytes, err := proto.Marshal(req)
	if err != nil {
		err = fmt.Errorf("marshal request: %w", err)
		span.RecordError(err)
		return nil, taskID, err
	}

	respBytes, err := d.callAgentRouted(ctx, agent.AgentID, protocol.MsgStartTaskRequest, req, reqBytes)
	if err != nil {
		span.RecordError(err)
		return nil, taskID, err
	}

	resp := &sdkv1.StartTaskResponse{}
	if err := proto.Unmarshal(respBytes, resp); err != nil {
		err = fmt.Errorf("unmarshal response: %w", err)
		span.RecordError(err)
		return nil, taskID, err
	}
	return resp, taskID, nil
}

func (d *Dispatcher) CancelTask(ctx context.Context, taskID string) error {
	agentID, err := d.taskAgentID(ctx, taskID)
	if err != nil {
		return err
	}

	req := &sdkv1.CancelTaskRequest{TaskId: taskID}
	reqBytes, err := proto.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	_, err = d.callAgentRoutedCancel(ctx, agentID, taskID, reqBytes)
	if err == nil {
		d.unregisterTask(taskID)
	}

	return err
}

func (d *Dispatcher) StreamTask(ctx context.Context, taskID string) ([]*sdkv1.TaskEvent, bool, error) {
	return d.StreamTaskAfterSeq(ctx, taskID, 0)
}

// StreamTaskAfterSeq streams task events after a given sequence number.
// Returns events, whether the task is done, and any error.
func (d *Dispatcher) StreamTaskAfterSeq(ctx context.Context, taskID string, afterSeq int64) ([]*sdkv1.TaskEvent, bool, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil, false, fmt.Errorf("task id is required")
	}

	d.mu.RLock()
	query := d.taskEventQuery
	d.mu.RUnlock()

	if query == nil {
		return nil, false, fmt.Errorf("task event query not configured")
	}

	records, err := query.ListEvents(ctx, taskID, afterSeq)
	if err != nil {
		return nil, false, fmt.Errorf("query events: %w", err)
	}
	events := taskEventsFromRecords(records)

	run, err := query.GetRun(ctx, taskID)
	if err != nil {
		if !errors.Is(err, ErrTaskRunNotFound) {
			return nil, false, fmt.Errorf("query task run: %w", err)
		}
		return events, false, nil
	}

	done := isTaskRunDone(run.Status)
	return events, done, nil
}

// StreamTaskRealtime forwards task events to the provided callback.
func (d *Dispatcher) StreamTaskRealtime(ctx context.Context, taskID string, fn func(*sdkv1.TaskEvent) bool) (bool, error) {
	return d.StreamTaskRealtimeAfterSeq(ctx, taskID, 0, fn)
}

// StreamTaskRealtimeAfterSeq streams task events to the callback after a given sequence number.
// Returns whether the task is done and any error.
func (d *Dispatcher) StreamTaskRealtimeAfterSeq(ctx context.Context, taskID string, afterSeq int64, fn func(*sdkv1.TaskEvent) bool) (bool, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return false, fmt.Errorf("task id is required")
	}

	d.mu.RLock()
	query := d.taskEventQuery
	d.mu.RUnlock()

	if query == nil {
		return false, fmt.Errorf("task event query not configured")
	}

	for {
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		default:
		}

		records, err := query.ListEvents(ctx, taskID, afterSeq)
		if err != nil {
			return false, fmt.Errorf("query events: %w", err)
		}

		done := false
		run, err := query.GetRun(ctx, taskID)
		if err != nil {
			if !errors.Is(err, ErrTaskRunNotFound) {
				return false, fmt.Errorf("query task run: %w", err)
			}
		} else {
			done = isTaskRunDone(run.Status)
		}

		for _, record := range records {
			if record.Seq > afterSeq {
				afterSeq = record.Seq
			}
			evt := record.Event
			if evt == nil {
				continue
			}
			if !fn(evt) {
				return done, nil // Callback stopped streaming
			}
		}

		if done {
			return done, nil
		}

		// Wait before next poll
		time.Sleep(taskStreamPollInterval)
	}
}

var taskStreamPollInterval = 500 * time.Millisecond

func taskEventsFromRecords(records []TaskEventRecord) []*sdkv1.TaskEvent {
	events := make([]*sdkv1.TaskEvent, 0, len(records))
	for _, record := range records {
		if record.Event != nil {
			events = append(events, record.Event)
		}
	}
	return events
}

// isTaskRunDone checks if a task run status indicates completion.
func isTaskRunDone(status string) bool {
	switch strings.ToLower(status) {
	case "succeeded", "success", "done", "completed":
		return true
	case "failed", "error":
		return true
	case "cancelled", "canceled":
		return true
	case "timed_out", "timeout":
		return true
	default:
		return false
	}
}

// isTaskEventTypeDone checks if a task event type indicates completion.
func isTaskEventTypeDone(eventType string) bool {
	switch strings.ToLower(eventType) {
	case "completed", "success", "succeeded":
		return true
	case "failed", "error":
		return true
	case "cancelled", "canceled":
		return true
	default:
		return false
	}
}

// TaskAgentID exposes tracked task routing (primarily for diagnostics).
// Returns the agentID for the task.
func (d *Dispatcher) TaskAgentID(taskID string) (string, bool) {
	d.mu.RLock()
	agentID, ok := d.taskRouting[taskID]
	d.mu.RUnlock()
	return agentID, ok
}

// callAgentRoutedCancel 对 CancelTask 叠加 HA 转发：目标 agent 的 session
// 在对端实例时经 remoteForwarder 转发（kind=cancel_task，TaskID 随帧）。
// owner 侧只投递取消给 agent；路由清理（unregisterTask）留在 caller——
// task routing 是 caller 实例的内存态。
func (d *Dispatcher) callAgentRoutedCancel(ctx context.Context, agentID, taskID string, reqBytes []byte) ([]byte, error) {
	respBytes, err := d.callAgent(ctx, agentID, protocol.MsgCancelTaskRequest, reqBytes)
	if err == nil || !errors.Is(err, errAgentUnreachable) {
		return respBytes, err
	}
	d.mu.RLock()
	fwd := d.remoteForwarder
	d.mu.RUnlock()
	if fwd == nil {
		return nil, err
	}
	respBytes, ferr := fwd.Forward(ctx, &RemoteCall{
		MsgID:   protocol.MsgCancelTaskRequest,
		AgentID: agentID,
		TaskID:  taskID,
	})
	if ferr != nil {
		return nil, fmt.Errorf("forward to owner of %s: %v: %w", agentID, ferr, errAgentUnreachable)
	}
	return respBytes, nil
}

// StartTaskOnAgent 定向把异步任务投递给指定 Agent（集群转发 owner 路径
// 使用）。与 StartTaskRequest 的差异：跳过 agent 选择，也**不**写
// task_runs、**不** registerTask——run 行与服务端 task ID 都由 caller 在
// dispatch 前创建（ID 在 metadata.taskId 里随帧带到，owner 原样透传给
// agent），owner 只负责本地投递。返回原始响应字节（转发协议原样回传）。
func (d *Dispatcher) StartTaskOnAgent(ctx context.Context, agentID string, req *sdkv1.InvokeRequest) ([]byte, error) {
	if req == nil || strings.TrimSpace(agentID) == "" {
		return nil, fmt.Errorf("agent id is required")
	}
	if req.Metadata == nil {
		req.Metadata = map[string]string{}
	}
	// 与本地 StartTaskRequest 对齐：定向投递也注入 agentId 标记。
	req.Metadata["agentId"] = agentID
	reqBytes, err := proto.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	if d.healthTracker != nil {
		d.healthTracker.IncrementConnections(agentID)
		defer d.healthTracker.DecrementConnections(agentID)
	}
	respBytes, err := d.callAgent(ctx, agentID, protocol.MsgStartTaskRequest, reqBytes)
	if err != nil {
		if d.healthTracker != nil {
			d.healthTracker.RecordFailure(agentID)
		}
		return nil, err
	}
	if d.healthTracker != nil {
		d.healthTracker.RecordSuccess(agentID)
	}
	return respBytes, nil
}

// CancelTaskOnAgent 定向取消指定 Agent 上的任务（集群转发 owner 路径）。
// 与 CancelTask 的差异：**不**查/不删本地 task routing——routing 在 caller
// 实例的内存态里，取消成功后由 caller unregisterTask；owner 只投递。
func (d *Dispatcher) CancelTaskOnAgent(ctx context.Context, agentID, taskID string) ([]byte, error) {
	if strings.TrimSpace(agentID) == "" || strings.TrimSpace(taskID) == "" {
		return nil, fmt.Errorf("agent id and task id are required")
	}
	reqBytes, err := proto.Marshal(&sdkv1.CancelTaskRequest{TaskId: taskID})
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	return d.callAgent(ctx, agentID, protocol.MsgCancelTaskRequest, reqBytes)
}
