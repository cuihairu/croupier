package function

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/cluster"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// instances 跨实例聚合（照 /ops/nodes 的「归属表全集 + 本地补详情」模式）：
// 本地 registry 只含连到本实例的 agent，归属表（cluster_agent_owners，
// TTL 内即活跃）补出对端实例持有的 agent，明细从共享 agent_sessions
// 快照表读取。单实例（Cluster nil）退化纯本地。

// seedRemoteSession 在共享 DB（agent_sessions 快照表）造一条远端 agent
// 会话行，并返回指向它的归属表 fake。
func seedRemoteSession(t *testing.T, f *invokeFixture, sess *reg.AgentSession, ownerInstance string) {
	t.Helper()
	require.NoError(t, reg.MigrateAgentSessions(f.db))
	require.NoError(t, reg.NewAgentSessionModel(f.db).Upsert(context.Background(), sess))
}

func remoteSession(agentID string) *reg.AgentSession {
	now := time.Now()
	return &reg.AgentSession{
		AgentID: agentID,
		GameID:  "demo",
		Env:     "prod",
		Addr:    "10.0.0.2:40252",
		Functions: map[string]reg.FunctionMeta{
			"demo.remote_fn": {Enabled: true},
		},
		Providers: []reg.ProviderSession{{
			ProviderID:  "svc-" + agentID,
			GameID:      "demo",
			Env:         "prod",
			Addr:        "10.0.0.2:40252",
			Version:     "1.0.0",
			SDKName:     "croupier-java-sdk",
			SDKLanguage: "java",
			SDKVersion:  "0.1.0",
			FunctionIDs: []string{"demo.remote_fn"},
		}},
		ExpireAt: now.Add(5 * time.Minute),
		LastSeen: now,
	}
}

func ownersFake(recs ...cluster.AgentOwnerRecord) func(context.Context) ([]cluster.AgentOwnerRecord, error) {
	return func(context.Context) ([]cluster.AgentOwnerRecord, error) { return recs, nil }
}

func TestFunctionInstancesAll_ClusterNil_LocalOnly(t *testing.T) {
	f := newInvokeFixture(t)
	f.registerAgent(t, "agent-local", "demo.fn")

	resp, err := NewService(f.svcCtx).FunctionInstancesAll(f.ctxFor("opuser"), &FunctionInstancesAllRequest{})
	require.NoError(t, err)
	require.Len(t, resp.Instances, 1)
	assert.Equal(t, "demo.fn", resp.Instances[0].FunctionID)
	assert.Equal(t, "agent-local", resp.Instances[0].AgentID)
	assert.Empty(t, resp.Instances[0].OwnerInstance)
	assert.Equal(t, 1, resp.Total)
}

func TestFunctionInstancesAll_RemoteOwnerAggregated(t *testing.T) {
	f := newInvokeFixture(t)
	f.registerAgent(t, "agent-local", "demo.fn")
	seedRemoteSession(t, f, remoteSession("agent-remote"), "server2")
	f.svcCtx.Cluster = &svc.ClusterRuntime{
		ListAgentOwners: ownersFake(
			cluster.AgentOwnerRecord{AgentID: "agent-local", InstanceID: "self", GameID: "demo", Env: "prod"},
			cluster.AgentOwnerRecord{AgentID: "agent-remote", InstanceID: "server2", GameID: "demo", Env: "prod"},
		),
	}

	resp, err := NewService(f.svcCtx).FunctionInstancesAll(f.ctxFor("opuser"), &FunctionInstancesAllRequest{})
	require.NoError(t, err)
	byAgent := map[string][]FunctionInstanceSummary{}
	for _, in := range resp.Instances {
		byAgent[in.AgentID] = append(byAgent[in.AgentID], in)
	}
	// 本地行不动 + 远端行带 provider 明细与 owner 标注。
	require.Len(t, byAgent["agent-local"], 1)
	require.Len(t, byAgent["agent-remote"], 1)
	remote := byAgent["agent-remote"][0]
	assert.Equal(t, "demo.remote_fn", remote.FunctionID)
	assert.Equal(t, "svc-agent-remote", remote.ServiceID)
	assert.Equal(t, "java", remote.SDKLang)
	assert.Equal(t, "croupier-java-sdk", remote.SDKName)
	assert.Equal(t, "server2", remote.OwnerInstance)
	assert.Equal(t, 2, resp.Total)
}

func TestFunctionInstancesAll_RemoteScopeMismatch_Excluded(t *testing.T) {
	f := newInvokeFixture(t)
	seedRemoteSession(t, f, remoteSession("agent-other-game"), "server2")
	f.svcCtx.Cluster = &svc.ClusterRuntime{
		ListAgentOwners: ownersFake(
			cluster.AgentOwnerRecord{AgentID: "agent-other-game", InstanceID: "server2", GameID: "other-game", Env: "prod"},
		),
	}

	resp, err := NewService(f.svcCtx).FunctionInstancesAll(f.ctxFor("opuser"), &FunctionInstancesAllRequest{})
	require.NoError(t, err)
	assert.Empty(t, resp.Instances)
}

func TestFunctionInstancesAll_OwnerWithoutSnapshotRow_Skipped(t *testing.T) {
	f := newInvokeFixture(t)
	f.registerAgent(t, "agent-local", "demo.fn")
	// 归属表活跃但快照表无行（owner 已 Release 的竞态）：跳过该 agent，
	// 不造空行也不影响本地视图。
	f.svcCtx.Cluster = &svc.ClusterRuntime{
		ListAgentOwners: ownersFake(
			cluster.AgentOwnerRecord{AgentID: "agent-ghost", InstanceID: "server2", GameID: "demo", Env: "prod"},
		),
	}

	resp, err := NewService(f.svcCtx).FunctionInstancesAll(f.ctxFor("opuser"), &FunctionInstancesAllRequest{})
	require.NoError(t, err)
	require.Len(t, resp.Instances, 1)
	assert.Equal(t, "agent-local", resp.Instances[0].AgentID)
}

func TestFunctionInstancesAll_OwnerTableError_FallsBackToLocal(t *testing.T) {
	f := newInvokeFixture(t)
	f.registerAgent(t, "agent-local", "demo.fn")
	f.svcCtx.Cluster = &svc.ClusterRuntime{
		ListAgentOwners: func(context.Context) ([]cluster.AgentOwnerRecord, error) {
			return nil, errors.New("owner table down")
		},
	}

	resp, err := NewService(f.svcCtx).FunctionInstancesAll(f.ctxFor("opuser"), &FunctionInstancesAllRequest{})
	require.NoError(t, err)
	require.Len(t, resp.Instances, 1)
	assert.Equal(t, "agent-local", resp.Instances[0].AgentID)
}

func TestFunctionInstancesAll_LocalAgentInOwnerTable_NotDuplicated(t *testing.T) {
	f := newInvokeFixture(t)
	f.registerAgent(t, "agent-local", "demo.fn")
	seedRemoteSession(t, f, remoteSession("agent-remote"), "server2")
	f.svcCtx.Cluster = &svc.ClusterRuntime{
		ListAgentOwners: ownersFake(
			cluster.AgentOwnerRecord{AgentID: "agent-local", InstanceID: "self", GameID: "demo", Env: "prod"},
			cluster.AgentOwnerRecord{AgentID: "agent-remote", InstanceID: "server2", GameID: "demo", Env: "prod"},
		),
	}

	resp, err := NewService(f.svcCtx).FunctionInstancesAll(f.ctxFor("opuser"), &FunctionInstancesAllRequest{})
	require.NoError(t, err)
	counts := map[string]int{}
	for _, in := range resp.Instances {
		counts[in.AgentID]++
	}
	assert.Equal(t, 1, counts["agent-local"], "本地 agent 不因归属表重复产出")
	assert.Equal(t, 1, counts["agent-remote"])
}

// 本地 registry 里的 DB 快照（refreshRemoteSnapshots 回灌）实为对端持有：
// 条目必须标真归属，否则前端把远端快照渲染成「本实例」误导排障。
func TestFunctionInstancesAll_LocalSnapshotOwnedByPeer_Annotated(t *testing.T) {
	f := newInvokeFixture(t)
	f.registerAgent(t, "agent-snap", "demo.fn")
	f.svcCtx.Cluster = &svc.ClusterRuntime{
		InstanceID: "self",
		ListAgentOwners: ownersFake(
			cluster.AgentOwnerRecord{AgentID: "agent-snap", InstanceID: "server2", GameID: "demo", Env: "prod"},
		),
	}

	resp, err := NewService(f.svcCtx).FunctionInstancesAll(f.ctxFor("opuser"), &FunctionInstancesAllRequest{})
	require.NoError(t, err)
	require.Len(t, resp.Instances, 1)
	assert.Equal(t, "server2", resp.Instances[0].OwnerInstance, "对端持有的本地快照要标注归属实例")
}

// 自持 agent（owner=本实例）保持空标注（前端渲染「本实例」）。
func TestFunctionInstancesAll_SelfOwnedLocal_EmptyAnnotation(t *testing.T) {
	f := newInvokeFixture(t)
	f.registerAgent(t, "agent-local", "demo.fn")
	f.svcCtx.Cluster = &svc.ClusterRuntime{
		InstanceID: "self",
		ListAgentOwners: ownersFake(
			cluster.AgentOwnerRecord{AgentID: "agent-local", InstanceID: "self", GameID: "demo", Env: "prod"},
		),
	}

	resp, err := NewService(f.svcCtx).FunctionInstancesAll(f.ctxFor("opuser"), &FunctionInstancesAllRequest{})
	require.NoError(t, err)
	require.Len(t, resp.Instances, 1)
	assert.Empty(t, resp.Instances[0].OwnerInstance)
}

func TestFunctionInstances_RemoteOwnerIncluded(t *testing.T) {
	f := newInvokeFixture(t)
	seedRemoteSession(t, f, remoteSession("agent-remote"), "server2")
	f.svcCtx.Cluster = &svc.ClusterRuntime{
		ListAgentOwners: ownersFake(
			cluster.AgentOwnerRecord{AgentID: "agent-remote", InstanceID: "server2", GameID: "demo", Env: "prod"},
		),
	}

	resp, err := NewService(f.svcCtx).FunctionInstances(f.ctxFor("opuser"), &FunctionInstancesRequest{ID: "demo.remote_fn"})
	require.NoError(t, err)
	require.Len(t, resp.Items, 1)
	assert.Equal(t, "agent-remote", resp.Items[0].AgentId)
	assert.Equal(t, "server2", resp.Items[0].OwnerInstance)

	// 本地未持有该函数、远端也没有的函数 ID：空列表。
	resp2, err := NewService(f.svcCtx).FunctionInstances(f.ctxFor("opuser"), &FunctionInstancesRequest{ID: "demo.nope"})
	require.NoError(t, err)
	assert.Empty(t, resp2.Items)
}
