package main

import (
	"context"
	"fmt"

	"github.com/cuihairu/croupier/internal/cluster"
	"github.com/cuihairu/croupier/internal/platform/dispatch"
)

// meshForwarder 适配 Dispatcher 的 HA 转发：把对远端实例持有 agent 的
// 调用翻译成 ForwardedInvoke 经互联 mesh 发给 owner。调用方上下文
// （username/game/env）透传给 owner 侧审计。
type meshForwarder struct {
	mesh *cluster.MeshInterconnect
}

func newMeshForwarder(mesh *cluster.MeshInterconnect) *meshForwarder {
	return &meshForwarder{mesh: mesh}
}

func (f *meshForwarder) ForwardInvoke(ctx context.Context, agentID, functionID string, payload []byte, metadata map[string]string, idempotencyKey string) ([]byte, error) {
	caller := cluster.CallerContext{
		GameID: metadata["game_id"],
		Env:    metadata["env"],
	}
	if v, ok := ctx.Value("username").(string); ok {
		caller.Username = v
	}
	req := &cluster.ForwardedInvoke{
		AgentID:        agentID,
		FunctionID:     functionID,
		Payload:        payload,
		Metadata:       metadata,
		IdempotencyKey: idempotencyKey,
		Caller:         caller,
	}
	res, err := f.mesh.Forward(ctx, agentID, req)
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

var _ dispatch.RemoteForwarder = (*meshForwarder)(nil)
