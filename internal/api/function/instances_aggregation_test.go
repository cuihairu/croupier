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

// instances cross-instance aggregation (following the /ops/nodes
// "ownership table full set + local detail fill-in" pattern): the local
// registry only holds agents connected to this instance, while the ownership
// table (cluster_agent_owners, active within TTL) supplies the agents held by
// the peer instance, with details read from the shared agent_sessions
// snapshot table. Single-instance (Cluster nil) degenerates to pure local.

// seedRemoteSession inserts a remote agent session row into the shared DB
// (agent_sessions snapshot table) and returns the ownership-table fake that
// points at it.
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
		Version: "2.1.0",
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
	// Local rows untouched + the remote row carries provider details and the owner annotation.
	require.Len(t, byAgent["agent-local"], 1)
	require.Len(t, byAgent["agent-remote"], 1)
	remote := byAgent["agent-remote"][0]
	assert.Equal(t, "demo.remote_fn", remote.FunctionID)
	assert.Equal(t, "svc-agent-remote", remote.ServiceID)
	assert.Equal(t, "java", remote.SDKLang)
	assert.Equal(t, "croupier-java-sdk", remote.SDKName)
	assert.Equal(t, "0.1.0", remote.SDKVersion)
	assert.Equal(t, "2.1.0", remote.AgentVersion)
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
	// Ownership table active but no snapshot-table row (the owner-already
	// Released race): skip the agent without fabricating an empty row or
	// perturbing the local view.
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

// A DB snapshot inside the local registry (refilled by refreshRemoteSnapshots)
// is actually held by the peer: the entry must be annotated with its true
// owner, otherwise the frontend renders the remote snapshot as "this instance"
// and misleads troubleshooting.
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

// A self-owned agent (owner = this instance) keeps an empty annotation (the
// frontend renders "this instance").
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

	// A function ID neither held locally nor present remotely: empty list.
	resp2, err := NewService(f.svcCtx).FunctionInstances(f.ctxFor("opuser"), &FunctionInstancesRequest{ID: "demo.nope"})
	require.NoError(t, err)
	assert.Empty(t, resp2.Items)
}
