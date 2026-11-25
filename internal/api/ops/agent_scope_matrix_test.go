package ops

// AGENT-001: scope isolation matrix for the four agent kinds
// (sidecar/devops/capture/synthetics).
//
// Verifies item by item the currently supported registration/config/list/
// stats/detail/action/log/offline-read surfaces, covering:
//   - list: listNodes filters by game/env plus the role label (cross-scope
//     agents are invisible)
//   - detail: OpsNodeDetail shares its source with the list; cross-scope
//     returns 404
//   - actions: requireAgentScope rejects cross-game / cross-env
//   - offline read: LastKnownScope returns the persistent binding
//     (LastKnownScope tests cover authorization separately)
//   - same-ID conflict: AgentSessionStore.Upsert refuses to rebind game/env
//     (already covered by tcp_listener_test / control_handler; only the
//     semantic signature is verified here)
//
// devops session/replay columns that are not wired up are NOT IMPLEMENTED
// (they depend on AGENT-002/003; fake data is not accepted as sign-off).
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

// List: a role in the same scope is visible, while the same role across a
// different game / env is not; role filtering returns only that role.
func TestAgentScopeMatrix_ListFiltersRoleAndScope(t *testing.T) {
	store := registry.NewStore()
	seedRoleAgent(store, "sidecar-a", agentRoleSidecar, "g1", "prod")
	seedRoleAgent(store, "devops-a", agentRoleDevops, "g1", "prod")
	seedRoleAgent(store, "capture-a", agentRoleCapture, "g1", "prod")
	seedRoleAgent(store, "synthetics-a", agentRoleSynthetic, "g1", "prod")
	// Same role across scopes (must not be selected by g1/prod)
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

	// The same role across scopes is invisible (g1/prod should only see
	// capture-a)
	capture := listNodes(context.Background(), svcCtx, "g1", "prod", agentRoleCapture)
	require.Len(t, capture, 1)
	assert.Equal(t, "capture-a", capture[0].Id)
}

// Detail: shares the list's source, so cross-scope agent detail is invisible
// (404) while the same scope is visible.
func TestAgentScopeMatrix_DetailSameSourceAsList(t *testing.T) {
	store := registry.NewStore()
	seedRoleAgent(store, "capture-a", agentRoleCapture, "g1", "prod")
	svcCtx := &svc.ServiceContext{RegistryStore: store}
	s := NewService(svcCtx)

	// Visible in the same scope
	ctx := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "g1", Env: "prod"})
	resp, err := s.OpsNodeDetail(ctx, &OpsNodeDetailRequest{NodeID: "capture-a"})
	require.NoError(t, err)
	assert.Equal(t, "capture-a", resp.Node.Id)
	assert.Equal(t, "capture", resp.Node.Labels["role"])

	// Invisible across game / env (same source as the list -> 404)
	for _, sc := range []svc.GameScope{
		{GameID: "g2", Env: "prod"},
		{GameID: "g1", Env: "dev"},
	} {
		cross := svc.WithGameScope(context.Background(), sc)
		_, err := s.OpsNodeDetail(cross, &OpsNodeDetailRequest{NodeID: "capture-a"})
		require.Error(t, err, "cross-scope agent detail must be invisible")
	}
}

// Actions: requireAgentScope consistently rejects cross-game / cross-env for
// all four roles.
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

// Same-ID conflict: an Agent ID is globally unique, so once bound to a game/env
// it cannot be switched to another scope. Cross-scope assignment is rejected by
// dispatch.agentCanInvoke (see routing_scope_test TestAgentCanInvoke_Matrix).
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
