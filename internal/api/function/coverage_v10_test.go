package function

// Coverage targets (group B):
//  1. The three pass-through branches of validateInvokeInput — blank
//     functionID, a descriptor with an empty Input map, and non-string/empty
//     string entries appearing in the required array.
//  2. The snapshot-table read failure branch of remoteAgentSnapshots (the
//     agent_sessions table is missing while the ownership table is still
//     active) -> silently falls back to the local view.

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

// Blank functionID: not intercepted in this layer (passed through to the
// downstream routing layer).
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

// The descriptor exists but Input is an empty map: pass-through.
func TestV10ValidateInvokeInput_EmptyInputPassthrough(t *testing.T) {
	f := newInvokeFixture(t)
	upsertInputContract(t, f, "demo.empty-input", datatypes.JSONMap{})

	require.NoError(t, validateInvokeInput(context.Background(), f.svcCtx,
		&FunctionInvokeRequest{ID: "demo.empty-input"}, []byte(`{}`)))
}

// The required array holds a non-string entry (a number) and an empty string
// entry: those entries are skipped and the missing check only considers
// well-formed string entries.
func TestV10ValidateInvokeInput_RequiredNonStringEntriesSkipped(t *testing.T) {
	f := newInvokeFixture(t)
	upsertInputContract(t, f, "demo.mixed-required", datatypes.JSONMap{
		"type":     "object",
		"required": []interface{}{123, "", "playerId"},
	})

	// playerId missing: the non-string and blank entries produce no
	// "missing required parameter" entry.
	err := validateInvokeInput(context.Background(), f.svcCtx,
		&FunctionInvokeRequest{ID: "demo.mixed-required"}, []byte(`{}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "playerId")
	assert.NotContains(t, err.Error(), ", ,", "blank/numeric entries must not join the message")

	// After playerId is provided: nothing missing -> pass-through.
	require.NoError(t, validateInvokeInput(context.Background(), f.svcCtx,
		&FunctionInvokeRequest{ID: "demo.mixed-required"}, []byte(`{"playerId":"p-1"}`)))
}

// The ownership table is active (the agent is not in the local registry) but
// the shared agent_sessions snapshot table is unreadable (a dropped table
// simulating a DB failure) -> returns nil to silently fall back to the local
// view instead of escalating to an error.
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
