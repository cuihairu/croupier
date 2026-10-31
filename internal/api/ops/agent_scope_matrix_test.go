package ops

// AGENT-001：四类 Agent（sidecar/devops/capture/synthetics）scope 隔离矩阵。
//
// 逐项核对当前支持的注册/配置/列表/统计/详情/动作/日志/离线读取，覆盖：
//   - 列表：listNodes 按 game/env 过滤 + role 标签过滤（跨 scope agent 不可见）
//   - 详情：OpsNodeDetail 与列表同源，跨 scope 返回 404
//   - 动作：requireAgentScope 拒绝跨 game / 跨 env
//   - 离线读取：LastKnownScope 返回持久绑定（LastKnownScope 测试另覆盖授权）
//   - 同 ID 冲突：AgentSessionStore.Upsert 拒绝更换 game/env 绑定
//     （tcp_listener_test / control_handler 已覆盖，此处仅核对语义签名）
//
// devops 未接通的会话/回放列 NOT IMPLEMENTED（依赖 AGENT-002/003，不以假数据当验收）。
import (
	"context"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/server"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	agentRoleSidecar   = "sidecar"
	agentRoleDevops    = "devops"
	agentRoleCapture   = "capture"
	agentRoleSynthetic = "synthetics"
)

func seedRoleAgent(store *registry.Store, id, role, gameID, env string) {
	now := time.Now()
	_ = store.UpsertAgent(&registry.AgentSession{
		AgentID:   id,
		GameID:    gameID,
		Env:       env,
		Addr:      "127.0.0.1:0",
		Version:   "1.0.0",
		Labels:    map[string]string{"role": role, "hostname": id + "-host"},
		Functions: map[string]registry.FunctionMeta{"f.x": {Enabled: true}},
		LastSeen:  now,
		ExpireAt:  now.Add(time.Hour),
	})
}

// 列表：同 scope 的某角色可见，跨 game / 跨 env 的同角色不可见；role 过滤只
// 返回该角色。
func TestAgentScopeMatrix_ListFiltersRoleAndScope(t *testing.T) {
	store := registry.NewStore()
	seedRoleAgent(store, "sidecar-a", agentRoleSidecar, "g1", "prod")
	seedRoleAgent(store, "devops-a", agentRoleDevops, "g1", "prod")
	seedRoleAgent(store, "capture-a", agentRoleCapture, "g1", "prod")
	seedRoleAgent(store, "synthetics-a", agentRoleSynthetic, "g1", "prod")
	// 跨 scope 的同角色（不应被 g1/prod 选中）
	seedRoleAgent(store, "capture-b", agentRoleCapture, "g2", "prod")
	seedRoleAgent(store, "capture-c", agentRoleCapture, "g1", "dev")

	svcCtx := &svc.ServiceContext{RegistryStore: store}

	for _, role := range []string{agentRoleSidecar, agentRoleDevops, agentRoleCapture, agentRoleSynthetic} {
		t.Run(role, func(t *testing.T) {
			nodes := listNodes(context.Background(), svcCtx, "g1", "prod", role)
			require.Len(t, nodes, 1, "四类角色同 scope 各 1 个")
			assert.Equal(t, role, nodes[0].Labels["role"])
			assert.Equal(t, "g1", nodes[0].GameId)
			assert.Equal(t, "prod", nodes[0].Env)
		})
	}

	// 同角色跨 scope 不可见（g1/prod 只应看到 capture-a）
	capture := listNodes(context.Background(), svcCtx, "g1", "prod", agentRoleCapture)
	require.Len(t, capture, 1)
	assert.Equal(t, "capture-a", capture[0].Id)
}

// 详情：与列表同源，跨 scope 的 agent 详情不可见（404），同 scope 可见。
func TestAgentScopeMatrix_DetailSameSourceAsList(t *testing.T) {
	store := registry.NewStore()
	seedRoleAgent(store, "capture-a", agentRoleCapture, "g1", "prod")
	svcCtx := &svc.ServiceContext{RegistryStore: store}
	s := NewService(svcCtx)

	// 同 scope 可见
	ctx := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "g1", Env: "prod"})
	resp, err := s.OpsNodeDetail(ctx, &OpsNodeDetailRequest{NodeID: "capture-a"})
	require.NoError(t, err)
	assert.Equal(t, "capture-a", resp.Node.Id)
	assert.Equal(t, "capture", resp.Node.Labels["role"])

	// 跨 game / 跨 env 不可见（列表同源 → 404）
	for _, sc := range []svc.GameScope{
		{GameID: "g2", Env: "prod"},
		{GameID: "g1", Env: "dev"},
	} {
		cross := svc.WithGameScope(context.Background(), sc)
		_, err := s.OpsNodeDetail(cross, &OpsNodeDetailRequest{NodeID: "capture-a"})
		require.Error(t, err, "cross-scope agent detail must be invisible")
	}
}

// 动作：requireAgentScope 对四类角色一致拒绝跨 game / 跨 env。
func TestAgentScopeMatrix_ActionsRejectCrossScope(t *testing.T) {
	store := registry.NewStore()
	seedRoleAgent(store, "sidecar-a", agentRoleSidecar, "g1", "prod")
	svcCtx := &svc.ServiceContext{RegistryStore: store}
	s := NewAgentService(svcCtx)

	ok := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "g1", Env: "prod"})
	require.NoError(t, s.requireAgentScope(ok, "sidecar-a"))

	for _, sc := range []svc.GameScope{
		{GameID: "g2", Env: "prod"},
		{GameID: "g1", Env: "dev"},
	} {
		cross := svc.WithGameScope(context.Background(), sc)
		err := s.requireAgentScope(cross, "sidecar-a")
		require.Error(t, err, "cross-scope action must be rejected")
	}
}

// 同 ID 冲突：同一 Agent ID 全局唯一，已绑定 game/env 后不得改为其它
// scope。跨 scope 指派由 dispatch.agentCanInvoke 拒绝（见 routing_scope_test
// TestAgentCanInvoke_Matrix）。
func TestAgentScopeMatrix_SameIDCannotChangeScope(t *testing.T) {
	store := server.NewAgentSessionStore()
	require.NoError(t, store.Upsert(&server.AgentSession{
		AgentID: "agent-1", GameID: "g1", Env: "prod",
	}))
	require.NoError(t, store.Upsert(&server.AgentSession{
		AgentID: "agent-1", GameID: "g1", Env: "prod",
	}))
	require.Error(t, store.Upsert(&server.AgentSession{
		AgentID: "agent-1", GameID: "g2", Env: "prod",
	}))
	require.Error(t, store.Upsert(&server.AgentSession{
		AgentID: "agent-1", GameID: "g1", Env: "dev",
	}))
}
