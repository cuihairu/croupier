package function

// 覆盖目标（组 B）：
//  1. validateInvokeInput 的三处透传分支——空白 functionID、descriptor
//     Input 为空 map、required 数组中出现非字符串/空字符串项。
//  2. remoteAgentSnapshots 的快照表读取失败分支（agent_sessions 表缺失，
//     归属表仍活跃）→ 静默回落本地视图。

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/cluster"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
)

// 空白 functionID：不在本层拦截（透传给下游路由层处理）。
func TestV10ValidateInvokeInput_BlankFunctionIDPassthrough(t *testing.T) {
	f := newInvokeFixture(t)
	upsertInputContract(t, f, "demo.grant", datatypes.JSONMap{
		"required": []interface{}{"playerId"},
	})

	require.NoError(t, validateInvokeInput(context.Background(), f.svcCtx,
		&FunctionInvokeRequest{ID: "   "}, []byte(`{}`)))
	require.NoError(t, validateInvokeInput(context.Background(), f.svcCtx,
		&FunctionInvokeRequest{}, []byte(`{}`)))
}

// descriptor 存在但 Input 为空 map：透传。
func TestV10ValidateInvokeInput_EmptyInputPassthrough(t *testing.T) {
	f := newInvokeFixture(t)
	upsertInputContract(t, f, "demo.empty-input", datatypes.JSONMap{})

	require.NoError(t, validateInvokeInput(context.Background(), f.svcCtx,
		&FunctionInvokeRequest{ID: "demo.empty-input"}, []byte(`{}`)))
}

// required 数组含非字符串项（数字）与空字符串项：跳过这些项，
// 只对合法字符串项做缺失判定。
func TestV10ValidateInvokeInput_RequiredNonStringEntriesSkipped(t *testing.T) {
	f := newInvokeFixture(t)
	upsertInputContract(t, f, "demo.mixed-required", datatypes.JSONMap{
		"type":     "object",
		"required": []interface{}{123, "", "playerId"},
	})

	// playerId 缺失：非字符串项与空串项不产出「缺少必填参数」条目。
	err := validateInvokeInput(context.Background(), f.svcCtx,
		&FunctionInvokeRequest{ID: "demo.mixed-required"}, []byte(`{}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "playerId")
	assert.NotContains(t, err.Error(), ", ,", "blank/numeric entries must not join the message")

	// playerId 提供后：无缺失 → 透传。
	require.NoError(t, validateInvokeInput(context.Background(), f.svcCtx,
		&FunctionInvokeRequest{ID: "demo.mixed-required"}, []byte(`{"playerId":"p-1"}`)))
}

// 归属表活跃（agent 不在本地 registry）但共享 agent_sessions 快照表不可读
// （表缺失模拟 DB 故障）→ 返回 nil 静默回落本地视图，不升级成错误。
func TestV10RemoteAgentSnapshots_SnapshotLoadFailureFallsBack(t *testing.T) {
	f := newInvokeFixture(t)
	require.NoError(t, reg.MigrateAgentSessions(f.db))
	require.NoError(t, f.db.Migrator().DropTable("agent_sessions"))
	f.svcCtx.Cluster = &svc.ClusterRuntime{
		ListAgentOwners: ownersFake(
			cluster.AgentOwnerRecord{AgentID: "agent-remote-down", InstanceID: "server2", GameID: "demo", Env: "prod"},
		),
	}

	got := remoteAgentSnapshots(context.Background(), f.svcCtx,
		svc.GameScope{GameID: "demo", Env: "prod"},
		map[string]struct{}{},
		[]cluster.AgentOwnerRecord{
			{AgentID: "agent-remote-down", InstanceID: "server2", GameID: "demo", Env: "prod"},
		})
	assert.Nil(t, got, "snapshot load failure must fall back to nil, not error out")
}
