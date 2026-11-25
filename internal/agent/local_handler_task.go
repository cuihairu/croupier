package agent

import (
	"context"
	"fmt"

	"github.com/cuihairu/croupier/internal/telemetry"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/protobuf/proto"
)

// handleStartTask handles StartTaskRequest.
func (h *LocalHandler) handleStartTask(ctx context.Context, data []byte) ([]byte, error) {
	req := &sdkv1.InvokeRequest{}
	if err := proto.Unmarshal(data, req); err != nil {
		return nil, fmt.Errorf("unmarshal InvokeRequest for StartTask: %w", err)
	}
	req.Metadata = telemetry.InjectContext(telemetry.ExtractContext(ctx, req.GetMetadata()), req.Metadata)

	taskID := h.tasks.Start(req)

	resp := &sdkv1.StartTaskResponse{
		TaskId: taskID,
	}
	return proto.Marshal(resp)
}

// handleCancelTask handles CancelTaskRequest.
func (h *LocalHandler) handleCancelTask(ctx context.Context, data []byte) ([]byte, error) {
	req := &sdkv1.CancelTaskRequest{}
	if err := proto.Unmarshal(data, req); err != nil {
		return nil, fmt.Errorf("unmarshal CancelTaskRequest: %w", err)
	}

	if h.tasks == nil {
		return nil, fmt.Errorf("task tracking not available")
	}

	h.tasks.Cancel(req.GetTaskId())

	resp := &sdkv1.StartTaskResponse{}
	return proto.Marshal(resp)
}

// executeTask is the TaskExecutor seam: it routes a task invocation through
// the same provider-call path as a synchronous invoke. Used by TaskRunner.
func (h *LocalHandler) executeTask(ctx context.Context, req *sdkv1.InvokeRequest) ([]byte, error) {
	ctx = telemetry.ExtractContext(ctx, req.GetMetadata())
	ctx, span := agentTracer().Start(ctx, "agent.task.execute",
		trace.WithSpanKind(trace.SpanKindConsumer),
		trace.WithAttributes(
			attribute.String("function.id", req.GetFunctionId()),
			attribute.String("agent.id", h.agentID),
			attribute.String("task.id", req.GetMetadata()["taskId"]),
		),
	)
	defer span.End()

	respBytes, err := h.handleInvoke(ctx, mustMarshal(req))
	if err != nil {
		recordSpanResult(span, err)
		return nil, err
	}
	// respBytes comes back from handleInvoke, whose success paths all return
	// proto.Marshal output (valid InvokeResponse wire bytes); unmarshaling it
	// again always succeeds, the error branch was unreachable and has been
	// deleted.
	resp := &sdkv1.InvokeResponse{}
	_ = proto.Unmarshal(respBytes, resp)
	if len(resp.GetPayload()) == 0 {
		recordSpanResult(span, nil)
		return []byte("null"), nil
	}
	recordSpanResult(span, nil)
	return resp.GetPayload(), nil
}
