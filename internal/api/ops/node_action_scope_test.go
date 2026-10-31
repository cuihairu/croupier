package ops

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// AGENT-001：/ops/nodes/:nodeId 的 live 动作与 meta 路径（helpers.go 的
// opsNodeDrain/opsNodeRestart/opsNodeUndrain/opsNodeMeta）此前无 scope 门——
// 列表/详情已按 X-Game-ID/X-Env 过滤（listNodes），动作却只按 nodeId 解析
// 连接，A scope 管理员可对 B scope 节点执行动作。以下断言 fail-closed：
// 节点不属于当前请求 scope 时动作/读必须拒绝，同 scope 放行。

func opsNodeActionFixture(t *testing.T) (*svc.ServiceContext, *registry.Store) {
	t.Helper()
	store := registry.NewStore()
	require.NoError(t, store.UpsertAgent(&registry.AgentSession{
		AgentID: "node-1", GameID: "demo", Env: "prod",
	}))
	svcCtx := &svc.ServiceContext{RegistryStore: store}
	return svcCtx, store
}

func TestOpsNodeDrain_CrossScopeRejected(t *testing.T) {
	svcCtx, _ := opsNodeActionFixture(t)

	// 同 scope → 到达会话解析（无 resolver → 明确报 session resolver unavailable，
	// 而非 scope 拒绝；证明 scope 校验先于动作）。
	ctx := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "demo", Env: "prod"})
	_, err := opsNodeDrain(ctx, svcCtx, &OpsNodeCommandsRequest{NodeId: "node-1"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "session resolver unavailable")

	// 跨 game → fail-closed。
	crossGame := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "other", Env: "prod"})
	_, err = opsNodeDrain(crossGame, svcCtx, &OpsNodeCommandsRequest{NodeId: "node-1"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not belong")

	// 跨 env → fail-closed。
	crossEnv := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "demo", Env: "dev"})
	_, err = opsNodeDrain(crossEnv, svcCtx, &OpsNodeCommandsRequest{NodeId: "node-1"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not belong")
}

func TestOpsNodeRestartUndrain_CrossScopeRejected(t *testing.T) {
	svcCtx, _ := opsNodeActionFixture(t)
	crossGame := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "other", Env: "prod"})

	_, err := opsNodeRestart(crossGame, svcCtx, &OpsNodeCommandsRequest{NodeId: "node-1"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not belong")

	_, err = opsNodeUndrain(crossGame, svcCtx, &OpsNodeCommandsRequest{NodeId: "node-1"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not belong")
}

func TestOpsNodeMeta_CrossScopeRejected(t *testing.T) {
	svcCtx, store := opsNodeActionFixture(t)
	_ = store.UpsertAgent(&registry.AgentSession{
		AgentID: "node-1", GameID: "demo", Env: "prod",
		Labels: map[string]string{"zone": "us-east-1", "role": "capture"},
	})

	// 同 scope → 放行读。
	ctx := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "demo", Env: "prod"})
	resp, err := opsNodeMeta(ctx, svcCtx, &OpsNodeMetaRequest{NodeID: "node-1"})
	require.NoError(t, err)
	assert.Equal(t, "us-east-1", resp.Labels["zone"])
	assert.Equal(t, "capture", resp.Labels["role"])

	// 跨 game/env → fail-closed。
	crossGame := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "other", Env: "prod"})
	_, err = opsNodeMeta(crossGame, svcCtx, &OpsNodeMetaRequest{NodeID: "node-1"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not belong")

	crossEnv := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "demo", Env: "dev"})
	_, err = opsNodeMeta(crossEnv, svcCtx, &OpsNodeMetaRequest{NodeID: "node-1"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not belong")

	// 不存在节点 → 404 契约保持（非 500）。
	_, err = opsNodeMeta(crossGame, svcCtx, &OpsNodeMetaRequest{NodeID: "ghost"})
	require.Error(t, err)
}
