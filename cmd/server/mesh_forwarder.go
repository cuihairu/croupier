package main

import (
	"context"
	"fmt"

	"github.com/cuihairu/croupier/internal/cluster"
	"github.com/cuihairu/croupier/internal/platform/dispatch"
	"github.com/cuihairu/croupier/internal/telemetry"
	"github.com/cuihairu/croupier/pkg/protocol"
)

// meshForwarder 适配 Dispatcher 的 HA 转发：把对远端实例持有 agent 的
// 调用翻译成 ForwardedInvoke 经互联 mesh 发给 owner。调用方上下文
// （username/roles/adminId/game/env/traceId）透传给 owner 侧审计。
// 三类调用（invoke/start_task/cancel_task）共用同一帧格式，按 MsgID
// 映射 Kind。
type meshForwarder struct {
	mesh *cluster.MeshInterconnect
}

func newMeshForwarder(mesh *cluster.MeshInterconnect) *meshForwarder {
	return &meshForwarder{mesh: mesh}
}

func (f *meshForwarder) Forward(ctx context.Context, call *dispatch.RemoteCall) ([]byte, error) {
	req := &cluster.ForwardedInvoke{
		AgentID:        call.AgentID,
		FunctionID:     call.FunctionID,
		Payload:        call.Payload,
		Metadata:       call.Metadata,
		IdempotencyKey: call.IdempotencyKey,
		TaskID:         call.TaskID,
		Caller:         callerFromContext(ctx, call.Metadata),
	}
	switch call.MsgID {
	case protocol.MsgStartTaskRequest:
		req.Kind = cluster.ForwardKindStartTask
	case protocol.MsgCancelTaskRequest:
		req.Kind = cluster.ForwardKindCancel
	}
	res, err := f.mesh.Forward(ctx, call.AgentID, req)
	if err != nil {
		// 不可达类错误（无路由/拨号失败/一跳超限）标记为可换候选重试。
		return nil, fmt.Errorf("mesh forward: %w", err)
	}
	if res.NotOwner {
		return nil, fmt.Errorf("mesh forward: owner changed: %s", res.Error)
	}
	if !res.OK {
		return nil, fmt.Errorf("remote invoke failed: %s", res.Error)
	}
	return res.Payload, nil
}

// callerFromContext 从调用链 ctx 与调用 metadata 构造 CallerContext。
// HTTP 链路的身份由 svc.AuthMiddleware 注入 request ctx（字符串键
// username/roles/adminID，uint 型 adminID）；scheduler 等后台派发的 ctx
// 无身份 → 字段留空（与本地路径的后台任务审计同语义）。TraceID 取
// telemetry ctx（dispatch 层已 ExtractContext + 开 span）。
func callerFromContext(ctx context.Context, metadata map[string]string) cluster.CallerContext {
	caller := cluster.CallerContext{
		GameID: metadata["gameId"],
		Env:    metadata["env"],
	}
	if ctx == nil {
		return caller
	}
	if v, ok := ctx.Value("username").(string); ok {
		caller.Username = v
	}
	if v, ok := ctx.Value("roles").([]string); ok {
		caller.Roles = v
	}
	if v, ok := ctx.Value("adminID").(uint); ok {
		caller.AdminID = v
	}
	caller.TraceID = telemetry.TraceIDFromContext(ctx)
	return caller
}

var _ dispatch.RemoteForwarder = (*meshForwarder)(nil)
