package dispatch

import (
	"context"
	"errors"
	"fmt"
	"strconv"
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

func (d *Dispatcher) Invoke(ctx context.Context, functionID string, payload []byte) ([]byte, error) {
	resp, err := d.InvokeRequest(ctx, &sdkv1.InvokeRequest{
		FunctionId: functionID,
		Payload:    payload,
	})
	if err != nil {
		return nil, err
	}
	return resp.GetPayload(), nil
}

// InvokeRequest forwards a fully populated InvokeRequest to a live agent.
func (d *Dispatcher) InvokeRequest(ctx context.Context, req *sdkv1.InvokeRequest) (*sdkv1.InvokeResponse, error) {
	if req == nil || req.GetFunctionId() == "" {
		return nil, fmt.Errorf("function id is required")
	}
	// 调用方声明的同步调用预算（metadata["timeoutMs"]，毫秒）：
	// 生效范围 [1s, invokeTimeout]，越界 clamp。Go context 的 deadline 取
	// min 语义——这里收紧后，内层 callAgent 的 invokeTimeout 只作上限。
	if budget := requestTimeoutBudget(req.GetMetadata()); budget > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, budget)
		defer cancel()
	}
	ctx = telemetry.ExtractContext(ctx, req.Metadata)
	ctx, span := dispatchTracer().Start(ctx, "function.dispatch.invoke",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(attribute.String("function.id", req.GetFunctionId())),
	)
	defer span.End()

	// 原始 metadata：每次重试克隆后注入，避免上一轮的 agent_id/trace
	// 泄漏到下一候选（转发链路按原始面重建）。
	origMeta := map[string]string{}
	for k, v := range req.GetMetadata() {
		origMeta[k] = v
	}
	// targeted/hash 路由语义上指定了唯一目标，失败换候选会破坏粘性——
	// 只有默认 lb 路径才有 failover。键名与写方一致（API 层注入
	// targetServiceId/hashKey；pickAgentWithRouting 读同一对键）。
	routePinned := strings.TrimSpace(origMeta["targetServiceId"]) != "" ||
		strings.TrimSpace(origMeta["hashKey"]) != ""

	tried := map[string]bool{}
	maxAttempts := 1
	if !routePinned {
		maxAttempts = 3
	}
	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		agent, err := d.pickAgentWithRouting(ctx, req.GetFunctionId(), origMeta, tried)
		if err != nil {
			if lastErr != nil {
				// failover 耗尽本质是「无可用 agent」的可重试状态，应映射
				// 503 service_unavailable（与首轮 noLiveAgentError 一致），
				// 而非普通 error 落 500。cause 链保留 errAgentUnreachable
				// 可换候选的语义；message 同时带 lastErr 保住排障信息
				// （session not found / 转发失败原因）。
				return nil, apperrors.Newf(apperrors.ErrCodeServiceUnavailable, "invoke",
					fmt.Errorf("%w: failover exhausted", lastErr),
					"failover exhausted: %v (last error: %v)", err, lastErr)
			}
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return nil, err
		}
		tried[agent.AgentID] = true
		span.SetAttributes(attribute.String("agent.id", agent.AgentID))

		resp, err := d.invokeOnPickedAgent(ctx, span, agent, req, origMeta)
		if err == nil {
			return resp, nil
		}
		lastErr = err
		if !errors.Is(err, errAgentUnreachable) {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return nil, err
		}
		// 该 agent 不可达（本地无连接且转发失败/无路由）→ 换下一候选。
	}
	span.RecordError(lastErr)
	span.SetStatus(codes.Error, lastErr.Error())
	return nil, lastErr
}

// invokeOnPickedAgent executes one invoke attempt against the selected agent.
func (d *Dispatcher) invokeOnPickedAgent(ctx context.Context, span trace.Span, agent *reg.AgentSession, req *sdkv1.InvokeRequest, origMeta map[string]string) (*sdkv1.InvokeResponse, error) {
	if d.healthTracker != nil {
		d.healthTracker.IncrementConnections(agent.AgentID)
		defer d.healthTracker.DecrementConnections(agent.AgentID)
	}

	// 原地重置再注入：保持 req.Metadata 的 map 身份不变——调用方
	// （console/审计）在调用后仍读同一 map 拿 agent_id，换新实例会
	// 让它们读到空值。
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
	// telemetry.InjectContext 对非 nil 入参（上方已保证 meta 非 nil）原地
	// 注入并原样返回，恒不返回 nil，nil 回退分支为死代码已删。
	req.Metadata = telemetry.InjectContext(ctx, meta)

	reqBytes, err := proto.Marshal(req)
	if err != nil {
		if d.healthTracker != nil {
			d.healthTracker.RecordFailure(agent.AgentID)
		}
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	respBytes, err := d.callAgentRouted(ctx, agent.AgentID, protocol.MsgInvokeRequest, req, reqBytes)
	if err != nil {
		if d.healthTracker != nil {
			d.healthTracker.RecordFailure(agent.AgentID)
		}
		return nil, err
	}

	resp := &sdkv1.InvokeResponse{}
	if err := proto.Unmarshal(respBytes, resp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	if d.healthTracker != nil {
		d.healthTracker.RecordSuccess(agent.AgentID)
	}
	return resp, nil
}

// callAgent sends a request to an Agent via its established TCP session.
func (d *Dispatcher) callAgent(ctx context.Context, agentID string, msgID uint32, reqBody []byte) ([]byte, error) {
	if d.sessionResolver == nil {
		return nil, fmt.Errorf("session resolver not configured: %w", errAgentUnreachable)
	}

	caller, ok := d.sessionResolver.ResolveAgentConn(agentID)
	if !ok {
		return nil, fmt.Errorf("agent %s session not found: %w", agentID, errAgentUnreachable)
	}

	callCtx := ctx
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		// 无调用方/契约声明预算时才套全局默认——已声明预算（ctx deadline，
		// 由 requestTimeoutBudget clamp 到 [1s,60s]）是权威值，不被默认值
		// 收紧（声明 30s 契约不应被 15s 默认截断）。
		var cancel context.CancelFunc
		callCtx, cancel = context.WithTimeout(ctx, d.invokeTimeout)
		defer cancel()
	}

	_, respBody, err := caller.Call(callCtx, msgID, reqBody)
	return respBody, err
}

// requestTimeoutBudget 解析 metadata["timeoutMs"]（调用方声明的一次同步
// 调用预算，毫秒）。有效范围 [1s, 60s]：小于 1s 提到 1s（防误传 0/负值瞬断），
// 大于 60s 截到 60s（同步通道上限，更长操作应走异步任务）。垃圾值/缺失
// 返回 0（= 不收紧，沿用 invokeTimeout 全局默认）。
func requestTimeoutBudget(meta map[string]string) time.Duration {
	raw := strings.TrimSpace(meta["timeoutMs"])
	if raw == "" {
		return 0
	}
	ms, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || ms <= 0 {
		return 0
	}
	if ms < 1000 {
		ms = 1000
	}
	if ms > 60000 {
		ms = 60000
	}
	return time.Duration(ms) * time.Millisecond
}

// callAgentRouted 在本地调用之上叠加 HA 转发：本地无该 agent 的 session
// 时，经 remoteForwarder 转发到持有连接的实例（一跳限制在 mesh 侧保证，
// owner 侧 InvokeRequestOnAgent 不走本方法避免环路）。invoke 与 start_task
// 共用（按 msgID 区分，req 里携带 function/payload/metadata 全量语义）。
func (d *Dispatcher) callAgentRouted(ctx context.Context, agentID string, msgID uint32, req *sdkv1.InvokeRequest, reqBytes []byte) ([]byte, error) {
	respBytes, err := d.callAgent(ctx, agentID, msgID, reqBytes)
	if err == nil || !errors.Is(err, errAgentUnreachable) {
		return respBytes, err
	}
	d.mu.RLock()
	fwd := d.remoteForwarder
	d.mu.RUnlock()
	if fwd == nil {
		return nil, err
	}
	fwdMeta := map[string]string{}
	for k, v := range req.GetMetadata() {
		fwdMeta[k] = v
	}
	// 转发链路自己重建路由/追踪上下文，本地注入的标记不带过去。
	// taskId 保留在 metadata 里随帧到 owner——caller 已创建 task_runs 行，
	// owner 侧 StartTaskOnAgent 重建 InvokeRequest 时沿用同一 ID。
	delete(fwdMeta, "agentId")
	respBytes, ferr := fwd.Forward(ctx, &RemoteCall{
		MsgID:          msgID,
		AgentID:        agentID,
		FunctionID:     req.GetFunctionId(),
		Payload:        req.GetPayload(),
		Metadata:       fwdMeta,
		IdempotencyKey: req.GetIdempotencyKey(),
	})
	if ferr != nil {
		return nil, fmt.Errorf("forward to owner of %s: %v: %w", agentID, ferr, errAgentUnreachable)
	}
	return respBytes, nil
}

// InvokeRequestOnAgent 定向调用指定 Agent（集群转发 owner 路径使用）。
//
// 与 InvokeRequest 的差异：跳过 agent 选择（转发请求已指定目标——
// 发起方按共享目录解析的 owner 就是我们），直接走本地连接。
// 返回原始响应字节（转发协议原样回传）。
func (d *Dispatcher) InvokeRequestOnAgent(ctx context.Context, agentID string, req *sdkv1.InvokeRequest) ([]byte, error) {
	if req == nil || strings.TrimSpace(agentID) == "" {
		return nil, fmt.Errorf("agent id is required")
	}
	reqBytes, err := proto.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	if d.healthTracker != nil {
		d.healthTracker.IncrementConnections(agentID)
		defer d.healthTracker.DecrementConnections(agentID)
	}
	respBytes, err := d.callAgent(ctx, agentID, protocol.MsgInvokeRequest, reqBytes)
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
