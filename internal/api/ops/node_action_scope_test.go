package ops

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// AGENT-001: the live actions and the meta path of /ops/nodes/:nodeId
// (opsNodeDrain/opsNodeRestart/opsNodeUndrain/opsNodeMeta in helpers.go) had no
// scope gate — list/detail were already filtered by X-Game-ID/X-Env
// (listNodes), yet actions resolved the connection by nodeId alone, so an
// admin of scope A could act on a node in scope B. The assertions below are
// fail-closed: an action/read must be rejected when the node does not belong to
// the request scope, and allowed within the same scope.

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

	// same scope → reaches session resolution (no resolver → an explicit session
	// resolver unavailable, not a scope rejection; proving scope checks run
	// before the action).
	ctx := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "demo", Env: "prod"})
	_, err := opsNodeDrain(ctx, svcCtx, &OpsNodeCommandsRequest{NodeId: "node-1"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "session resolver unavailable")

	// cross game → fail-closed.
	crossGame := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "other", Env: "prod"})
	_, err = opsNodeDrain(crossGame, svcCtx, &OpsNodeCommandsRequest{NodeId: "node-1"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not belong")

	// cross env → fail-closed.
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

	// same scope → the read is allowed.
	ctx := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "demo", Env: "prod"})
	resp, err := opsNodeMeta(ctx, svcCtx, &OpsNodeMetaRequest{NodeID: "node-1"})
	require.NoError(t, err)
	assert.Equal(t, "us-east-1", resp.Labels["zone"])
	assert.Equal(t, "capture", resp.Labels["role"])

	// cross game/env → fail-closed.
	crossGame := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "other", Env: "prod"})
	_, err = opsNodeMeta(crossGame, svcCtx, &OpsNodeMetaRequest{NodeID: "node-1"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not belong")

	crossEnv := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "demo", Env: "dev"})
	_, err = opsNodeMeta(crossEnv, svcCtx, &OpsNodeMetaRequest{NodeID: "node-1"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not belong")

	// nonexistent node → the 404 contract holds (not 500).
	_, err = opsNodeMeta(crossGame, svcCtx, &OpsNodeMetaRequest{NodeID: "ghost"})
	require.Error(t, err)
}
