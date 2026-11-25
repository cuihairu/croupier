package cluster

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	internaltcp "github.com/cuihairu/croupier/internal/transport/tcp"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func handshakedCtx() context.Context {
	ctx := internaltcp.WithConnectionSession(context.Background(), internaltcp.NewConnectionSession())
	internaltcp.ConnectionSessionFromContext(ctx).Set(internaltcp.ConnectionHandshakeKey, true)
	return ctx
}

func newDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open(t.TempDir()+"/cluster.db"), &gorm.Config{})
	require.NoError(t, err)
	return db
}

// ---------- membership ----------

func TestMembership_RegisterRenewListResign(t *testing.T) {
	db := newDB(t)
	m := NewDBMembership(db, 2*time.Second)
	require.NoError(t, m.EnsureTable(context.Background()))

	// New instance: epoch=1.
	epoch, err := m.Register(context.Background(), PeerInfo{
		InstanceID: "a", AdvertiseAddr: "10.0.0.1:8444",
	})
	require.NoError(t, err)
	assert.EqualValues(t, 1, epoch)

	_, err = m.Register(context.Background(), PeerInfo{
		InstanceID: "b", AdvertiseAddr: "10.0.0.2:8444",
	})
	require.NoError(t, err)

	alive, err := m.ListAlive(context.Background())
	require.NoError(t, err)
	assert.Len(t, alive, 2)

	// Restart reusing the same ID: epoch increments and the address updates.
	epoch2, err := m.Register(context.Background(), PeerInfo{
		InstanceID: "a", AdvertiseAddr: "10.0.0.1:9444",
	})
	require.NoError(t, err)
	assert.EqualValues(t, 2, epoch2)
	alive, _ = m.ListAlive(context.Background())
	for _, p := range alive {
		if p.InstanceID == "a" {
			assert.Equal(t, "10.0.0.1:9444", p.AdvertiseAddr)
			assert.EqualValues(t, 2, p.Epoch)
		}
	}

	// Renew lease.
	require.NoError(t, m.Renew(context.Background(), "a"))

	// After lease expiry the instance is no longer alive.
	time.Sleep(2100 * time.Millisecond)
	alive, err = m.ListAlive(context.Background())
	require.NoError(t, err)
	assert.Empty(t, alive)

	// Renewing after expiry = partition recovery (the record survives and can
	// come back).
	require.NoError(t, m.Renew(context.Background(), "a"))
	alive, _ = m.ListAlive(context.Background())
	assert.Len(t, alive, 1)

	// Graceful exit.
	require.NoError(t, m.Resign(context.Background(), "a"))
	alive, _ = m.ListAlive(context.Background())
	assert.Empty(t, alive)
}

func TestMembership_RenewUnknownInstance(t *testing.T) {
	db := newDB(t)
	m := NewDBMembership(db, time.Second)
	require.NoError(t, m.EnsureTable(context.Background()))
	require.Error(t, m.Renew(context.Background(), "ghost"))
}

// ---------- owner resolution ----------

type memMembership struct {
	peers []PeerInfo
}

func (m *memMembership) Register(ctx context.Context, info PeerInfo) (uint64, error) { return 1, nil }
func (m *memMembership) Renew(ctx context.Context, id string) error                  { return nil }
func (m *memMembership) ListAlive(ctx context.Context) ([]PeerInfo, error)           { return m.peers, nil }
func (m *memMembership) Resign(ctx context.Context, id string) error                 { return nil }

func setupOwner(t *testing.T) (*DBOwnerResolver, *MeshInterconnect) {
	t.Helper()
	db := newDB(t)
	resolver := NewDBOwnerResolver(db, 3*time.Minute)
	require.NoError(t, resolver.EnsureTable(context.Background()))
	mesh := NewMeshInterconnect(PeerInfo{InstanceID: "self", AdvertiseAddr: "1.1.1.1:8444"}, resolver,
		&memMembership{peers: []PeerInfo{{InstanceID: "peer-a", AdvertiseAddr: "2.2.2.2:8444", Epoch: 1}}})
	resolver.SetMesh(mesh)
	mesh.RefreshPeers(context.Background())
	return resolver, mesh
}

func TestOwnerResolver_ClaimResolveRelease(t *testing.T) {
	resolver, mesh := setupOwner(t)
	ctx := context.Background()

	// Held by self.
	require.NoError(t, resolver.ClaimOwner(ctx, "agent-1", "demo", "prod", "self", 1, nil))
	owner, err := resolver.ResolveOwner(ctx, "agent-1")
	require.NoError(t, err)
	require.NotNil(t, owner)
	assert.Equal(t, "self", owner.InstanceID)

	// Held by a peer (present in peers).
	require.NoError(t, resolver.ClaimOwner(ctx, "agent-2", "demo", "prod", "peer-a", 1, nil))
	owner, _ = resolver.ResolveOwner(ctx, "agent-2")
	require.NotNil(t, owner)
	assert.Equal(t, "peer-a", owner.InstanceID)

	// Held by a peer that is not in last-known peers (instance dead) → no
	// owner.
	require.NoError(t, resolver.ClaimOwner(ctx, "agent-3", "demo", "prod", "peer-z", 1, nil))
	owner, _ = resolver.ResolveOwner(ctx, "agent-3")
	assert.Nil(t, owner)

	// Release.
	require.NoError(t, resolver.Release(ctx, "agent-1"))
	owner, _ = resolver.ResolveOwner(ctx, "agent-1")
	assert.Nil(t, owner)

	// Unknown agent.
	owner, _ = resolver.ResolveOwner(ctx, "ghost")
	assert.Nil(t, owner)
	_ = mesh
}

func TestOwnerResolver_TTLExpiry(t *testing.T) {
	db := newDB(t)
	// Generous TTL + explicitly back-dating last_seen_at: avoids the "Resolve
	// first, then Sleep" pattern turning flaky on slow disks / under load when
	// the Claim→Resolve gap exceeds a short TTL (the first assertion would
	// fail).
	resolver := NewDBOwnerResolver(db, 2*time.Second)
	require.NoError(t, resolver.EnsureTable(context.Background()))
	ctx := context.Background()
	require.NoError(t, resolver.ClaimOwner(ctx, "agent-1", "g", "e", "self", 1, nil))

	owner, _ := resolver.ResolveOwner(ctx, "agent-1")
	require.NotNil(t, owner)

	require.NoError(t, db.WithContext(ctx).Model(&AgentOwnerRecord{}).
		Where("agent_id = ?", "agent-1").
		Update("last_seen_at", time.Now().UTC().Add(-3*time.Second)).Error)
	owner, _ = resolver.ResolveOwner(ctx, "agent-1")
	assert.Nil(t, owner, "owner 记录过期后应视为无 owner")
}

// ---------- forwarding: one-hop limit + fencing ----------

func TestForward_HopLimit(t *testing.T) {
	mesh := NewMeshInterconnect(PeerInfo{InstanceID: "b"}, nil, nil)
	_, err := mesh.Forward(context.Background(), "agent-1", &ForwardedInvoke{Forwarded: true})
	assert.ErrorIs(t, err, ErrHopLimit)
}

func TestForward_NoRoute(t *testing.T) {
	mesh := NewMeshInterconnect(PeerInfo{InstanceID: "b"}, nil, nil)
	_, err := mesh.Forward(context.Background(), "agent-1", &ForwardedInvoke{})
	assert.ErrorIs(t, err, ErrNoRoute)
}

func TestForward_ToOwnerAndResultDecode(t *testing.T) {
	db := newDB(t)
	resolver := NewDBOwnerResolver(db, time.Minute)
	require.NoError(t, resolver.EnsureTable(context.Background()))

	var gotReq *ForwardedInvoke
	owner := PeerInfo{InstanceID: "peer-a", AdvertiseAddr: "10.0.0.2:8444", Epoch: 7}
	mesh := NewMeshInterconnect(PeerInfo{InstanceID: "self"}, resolver, nil)
	mesh.dial = func(ctx context.Context, addr string, self PeerInfo) (*peerConn, error) {
		return &peerConn{
			addr:  addr,
			epoch: 7,
			send: func(ctx context.Context, msgID uint32, body []byte) ([]byte, error) {
				var req ForwardedInvoke
				if err := json.Unmarshal(body, &req); err != nil {
					return nil, err
				}
				gotReq = &req
				return json.Marshal(ForwardedResult{OK: true, Payload: json.RawMessage(`{"done":true}`)})
			},
			close: func() {},
		}, nil
	}
	// Seed the owner record.
	require.NoError(t, resolver.ClaimOwner(context.Background(), "agent-9", "demo", "prod", owner.InstanceID, owner.Epoch, nil))
	// Seed the owner into the peers cache (needed for cross-validation).
	mesh.peersMu.Lock()
	mesh.peersCache = []PeerInfo{owner, {InstanceID: "self"}}
	mesh.peersMu.Unlock()

	result, err := mesh.Forward(context.Background(), "agent-9", &ForwardedInvoke{
		FunctionID: "player.ban", Payload: []byte(`{}`),
		Caller: CallerContext{Username: "alice"},
	})
	require.NoError(t, err)
	assert.True(t, result.OK)

	// Request side: one-hop flag set + epoch passed through.
	require.NotNil(t, gotReq)
	assert.True(t, gotReq.Forwarded)
	assert.EqualValues(t, 7, gotReq.CallerEpoch)
	assert.Equal(t, "player.ban", gotReq.FunctionID)
	assert.Equal(t, "alice", gotReq.Caller.Username)
}

// ---------- owner-side handler: fencing and hop checks ----------

func TestServeForwardHandler_FencingAndFlags(t *testing.T) {
	localCalled := false
	handler := ServeForwardHandler(42, func(ctx context.Context, req *ForwardedInvoke) (*ForwardedResult, error) {
		localCalled = true
		return &ForwardedResult{OK: true}, nil
	})

	// Epoch mismatch (zombie owner) → NotOwner, no local execution.
	var result ForwardedResult
	require.NoError(t, json.Unmarshal(handler(handshakedCtx(), mustJSON(t, &ForwardedInvoke{Forwarded: true, CallerEpoch: 41})), &result))
	assert.False(t, result.OK)
	assert.True(t, result.NotOwner)
	assert.False(t, localCalled)

	// Not marked forwarded (unexpected call) → refused.
	require.NoError(t, json.Unmarshal(handler(handshakedCtx(), mustJSON(t, &ForwardedInvoke{Forwarded: false, CallerEpoch: 42})), &result))
	assert.False(t, result.OK)
	assert.False(t, localCalled)

	// Normal path → executed locally.
	require.NoError(t, json.Unmarshal(handler(handshakedCtx(), mustJSON(t, &ForwardedInvoke{Forwarded: true, CallerEpoch: 42})), &result))
	assert.True(t, result.OK)
	assert.True(t, localCalled)
}

func TestServeHelloHandler(t *testing.T) {
	handler := ServeHelloHandler(PeerInfo{InstanceID: "me"}, 9)

	// Correct role.
	var resp helloResponse
	require.NoError(t, json.Unmarshal(handler(context.Background(), mustJSON(t, &helloRequest{Role: "server", InstanceID: "peer"})), &resp))
	assert.Equal(t, "me", resp.InstanceID)
	assert.EqualValues(t, 9, resp.Epoch)

	// Non-server role → empty response (refused).
	require.NoError(t, json.Unmarshal(handler(context.Background(), mustJSON(t, &helloRequest{Role: "agent"})), &resp))
	assert.Empty(t, resp.InstanceID)
}

func TestMeshPeers_ExcludesSelf(t *testing.T) {
	mesh := NewMeshInterconnect(PeerInfo{InstanceID: "self"}, nil,
		&memMembership{peers: []PeerInfo{
			{InstanceID: "self"}, {InstanceID: "peer-a"},
		}})
	mesh.RefreshPeers(context.Background())
	peers := mesh.Peers()
	require.Len(t, peers, 1)
	assert.Equal(t, "peer-a", peers[0].InstanceID)
}

func mustJSON(t *testing.T, v interface{}) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return b
}
