package cluster

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOwnerHooks_ClaimTouchRelease(t *testing.T) {
	db := newDB(t)
	resolver := NewDBOwnerResolver(db, 3*time.Minute)
	require.NoError(t, resolver.EnsureTable(context.Background()))
	// selfInstanceID depends on the mesh being injected (startCluster does it
	// in production).
	resolver.SetMesh(NewMeshInterconnect(PeerInfo{InstanceID: "self"}, nil, nil))
	hooks := NewOwnerHooks(resolver, "self", 3)
	ctx := context.Background()

	hooks.OnAgentRegistered(ctx, "agent-1", "demo", "prod", nil)

	// Touch requires the ownership record to belong to this instance.
	hooks.OnAgentHeartbeat(ctx, "agent-1")

	hooks.OnAgentDisconnected(ctx, "agent-1")
	owner, _ := resolver.ResolveOwner(ctx, "agent-1")
	assert.Nil(t, owner)
}

func TestOwnerHooks_NilSafety(t *testing.T) {
	var h *OwnerHooks
	assert.NotPanics(t, func() {
		h.OnAgentRegistered(context.Background(), "a", "g", "e", nil)
		h.OnAgentHeartbeat(context.Background(), "a")
		h.OnAgentDisconnected(context.Background(), "a")
	})
}

func TestCountAgentsByOwner(t *testing.T) {
	db := newDB(t)
	resolver := NewDBOwnerResolver(db, time.Minute)
	require.NoError(t, resolver.EnsureTable(context.Background()))
	ctx := context.Background()

	require.NoError(t, resolver.ClaimOwner(ctx, "a1", "g", "e", "self", 1, nil))
	require.NoError(t, resolver.ClaimOwner(ctx, "a2", "g", "e", "self", 1, nil))
	require.NoError(t, resolver.ClaimOwner(ctx, "a3", "g", "e", "peer", 1, nil))

	counts, err := resolver.CountAgentsByOwner(ctx)
	require.NoError(t, err)
	assert.EqualValues(t, 2, counts["self"])
	assert.EqualValues(t, 1, counts["peer"])
}
