package cluster

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type errOwnerResolverV9 struct{}

func (errOwnerResolverV9) ResolveOwner(context.Context, string) (*PeerInfo, error) {
	return nil, errors.New("v9: resolve boom")
}

type errMembershipV9 struct {
	registerErr error
	listErr     error
}

func (m *errMembershipV9) Register(ctx context.Context, info PeerInfo) (uint64, error) {
	return 0, m.registerErr
}
func (m *errMembershipV9) Renew(ctx context.Context, instanceID string) error { return nil }
func (m *errMembershipV9) ListAlive(ctx context.Context) ([]PeerInfo, error) {
	return nil, m.listErr
}
func (m *errMembershipV9) Resign(ctx context.Context, instanceID string) error { return nil }

func waitForV9(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("v9: condition not met within deadline")
}

func TestOwnerHooks_WriteFailuresV9(t *testing.T) {
	db := newDB(t)
	resolver := NewDBOwnerResolver(db, time.Minute)
	require.NoError(t, resolver.EnsureTable(context.Background()))
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	hooks := NewOwnerHooks(resolver, "self", 1)
	ctx := context.Background()
	assert.NotPanics(t, func() {
		hooks.OnAgentRegistered(ctx, "agent-1", "g", "e")
		hooks.OnAgentHeartbeat(ctx, "agent-1")
		hooks.OnAgentDisconnected(ctx, "agent-1")
	})
}

func TestForward_ErrorPathsV9(t *testing.T) {
	ctx := context.Background()

	mesh := NewMeshInterconnect(PeerInfo{InstanceID: "self"}, errOwnerResolverV9{}, nil)
	_, err := mesh.Forward(ctx, "agent-1", &ForwardedInvoke{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "resolve owner")

	meshNoOwner := NewMeshInterconnect(PeerInfo{InstanceID: "self"}, &staticOwnerResolver{}, nil)
	_, err = meshNoOwner.Forward(ctx, "agent-1", &ForwardedInvoke{})
	assert.ErrorIs(t, err, ErrNoRoute)

	meshSelf := NewMeshInterconnect(
		PeerInfo{InstanceID: "self"},
		&staticOwnerResolver{owner: &PeerInfo{InstanceID: "self", Epoch: 1}},
		nil,
	)
	res, err := meshSelf.Forward(ctx, "agent-1", &ForwardedInvoke{})
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.False(t, res.OK)
	assert.Equal(t, "owner is self", res.Error)

	meshDialFail := NewMeshInterconnect(
		PeerInfo{InstanceID: "self"},
		&staticOwnerResolver{owner: &PeerInfo{InstanceID: "peer", AdvertiseAddr: "10.0.0.1:1", Epoch: 1}},
		nil,
	)
	meshDialFail.dial = func(ctx context.Context, addr string, self PeerInfo) (*peerConn, error) {
		return nil, errors.New("v9: dial boom")
	}
	_, err = meshDialFail.Forward(ctx, "agent-1", &ForwardedInvoke{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "dial owner peer")
}

func TestForward_SendFailuresV9(t *testing.T) {
	ctx := context.Background()
	owner := &PeerInfo{InstanceID: "peer", AdvertiseAddr: "10.0.0.2:8444", Epoch: 1}

	closed := false
	mesh := NewMeshInterconnect(PeerInfo{InstanceID: "self"}, &staticOwnerResolver{owner: owner}, nil)
	mesh.dial = func(ctx context.Context, addr string, self PeerInfo) (*peerConn, error) {
		return &peerConn{
			addr: addr,
			send: func(ctx context.Context, msgID uint32, body []byte) ([]byte, error) {
				return nil, errors.New("v9: send boom")
			},
			close: func() { closed = true },
		}, nil
	}
	_, err := mesh.Forward(ctx, "agent-1", &ForwardedInvoke{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "forward rpc")
	assert.True(t, closed, "send 失败后连接应被 drop")
	mesh.mu.Lock()
	_, exists := mesh.conns["peer"]
	mesh.mu.Unlock()
	assert.False(t, exists, "失败连接应从池中移除")

	mesh2 := NewMeshInterconnect(PeerInfo{InstanceID: "self"}, &staticOwnerResolver{owner: owner}, nil)
	mesh2.dial = func(ctx context.Context, addr string, self PeerInfo) (*peerConn, error) {
		return &peerConn{
			addr: addr,
			send: func(ctx context.Context, msgID uint32, body []byte) ([]byte, error) {
				return []byte("not-json"), nil
			},
			close: func() {},
		}, nil
	}
	_, err = mesh2.Forward(ctx, "agent-1", &ForwardedInvoke{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "decode forward response")
}

func TestForward_ConnReuseAndAddrChangeV9(t *testing.T) {
	ctx := context.Background()
	owner := &PeerInfo{InstanceID: "peer", AdvertiseAddr: "10.0.0.2:8444", Epoch: 1}
	ownerMoved := &PeerInfo{InstanceID: "peer", AdvertiseAddr: "10.0.0.9:8444", Epoch: 2}

	var (
		dialCount int
		mu        sync.Mutex
		closed    []bool
	)
	newConn := func(addr string) *peerConn {
		mu.Lock()
		closed = append(closed, false)
		idx := len(closed) - 1
		mu.Unlock()
		return &peerConn{
			addr: addr,
			send: func(ctx context.Context, msgID uint32, body []byte) ([]byte, error) {
				return json.Marshal(ForwardedResult{OK: true})
			},
			close: func() {
				mu.Lock()
				closed[idx] = true
				mu.Unlock()
			},
		}
	}

	mesh := NewMeshInterconnect(PeerInfo{InstanceID: "self"}, &staticOwnerResolver{owner: owner}, nil)
	mesh.dial = func(ctx context.Context, addr string, self PeerInfo) (*peerConn, error) {
		mu.Lock()
		dialCount++
		mu.Unlock()
		return newConn(addr), nil
	}

	for i := 0; i < 2; i++ {
		res, err := mesh.Forward(ctx, "agent-1", &ForwardedInvoke{})
		require.NoError(t, err)
		assert.True(t, res.OK)
	}
	mu.Lock()
	assert.Equal(t, 1, dialCount, "同地址应复用连接")
	mu.Unlock()

	mesh.resolver = &staticOwnerResolver{owner: ownerMoved}
	_, err := mesh.Forward(ctx, "agent-1", &ForwardedInvoke{})
	require.NoError(t, err)
	mu.Lock()
	assert.Equal(t, 2, dialCount, "地址变化应重建连接")
	assert.True(t, closed[0], "旧连接应被关闭")
	mu.Unlock()

	mesh.Close()
	mu.Lock()
	assert.True(t, closed[1], "Close 应关闭全部连接")
	mu.Unlock()
	mesh.mu.Lock()
	assert.Empty(t, mesh.conns)
	mesh.mu.Unlock()
}

func TestRefreshPeers_NilMembershipAndErrorV9(t *testing.T) {
	ctx := context.Background()

	meshNil := NewMeshInterconnect(PeerInfo{InstanceID: "self"}, nil, nil)
	assert.NotPanics(t, func() { meshNil.RefreshPeers(ctx) })

	meshErr := NewMeshInterconnect(PeerInfo{InstanceID: "self"}, nil, &errMembershipV9{listErr: errors.New("v9: list boom")})
	meshErr.peersMu.Lock()
	meshErr.peersCache = []PeerInfo{{InstanceID: "peer-a", Epoch: 1}}
	meshErr.peersMu.Unlock()
	meshErr.RefreshPeers(ctx)
	assert.Equal(t, []PeerInfo{{InstanceID: "peer-a", Epoch: 1}}, meshErr.Peers(), "ListAlive 失败应保留 last-known 缓存")
}

func TestServeForwardHandler_BadJSONAndInvokeErrorV9(t *testing.T) {
	ctx := context.Background()
	handler := ServeForwardHandler(7, func(ctx context.Context, req *ForwardedInvoke) (*ForwardedResult, error) {
		return nil, errors.New("v9: local invoke boom")
	})

	var result ForwardedResult
	require.NoError(t, json.Unmarshal(handler(ctx, []byte("{{{bad json")), &result))
	assert.False(t, result.OK)
	assert.Equal(t, "bad request", result.Error)

	require.NoError(t, json.Unmarshal(handler(ctx, mustJSON(t, &ForwardedInvoke{Forwarded: true, CallerEpoch: 7})), &result))
	assert.False(t, result.OK)
	assert.Contains(t, result.Error, "local invoke boom")
}

type v9LifecycleMembership struct {
	mu          sync.Mutex
	renewCalls  int
	listCalls   int
	renewErrFor int
	resignErr   bool
	resigned    bool
}

func (m *v9LifecycleMembership) Register(ctx context.Context, info PeerInfo) (uint64, error) {
	return 3, nil
}

func (m *v9LifecycleMembership) Renew(ctx context.Context, instanceID string) error {
	m.mu.Lock()
	m.renewCalls++
	n := m.renewCalls
	m.mu.Unlock()
	if n <= m.renewErrFor {
		return errors.New("v9: renew boom")
	}
	return nil
}

func (m *v9LifecycleMembership) ListAlive(ctx context.Context) ([]PeerInfo, error) {
	m.mu.Lock()
	m.listCalls++
	m.mu.Unlock()
	return []PeerInfo{{InstanceID: "other", AdvertiseAddr: "10.0.0.2:8444", Epoch: 1}}, nil
}

func (m *v9LifecycleMembership) Resign(ctx context.Context, instanceID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.resigned = true
	if m.resignErr {
		return errors.New("v9: resign boom")
	}
	return nil
}

func TestLifecycle_StartRegisterFailV9(t *testing.T) {
	lc := Start(context.Background(),
		Config{InstanceID: "v9-reg-fail", AdvertiseAddr: "127.0.0.1:19099"},
		&errMembershipV9{registerErr: errors.New("v9: register boom")},
		nil, nil)
	assert.Nil(t, lc)
}

func TestLifecycle_LoopRenewPollStopV9(t *testing.T) {
	mem := &v9LifecycleMembership{renewErrFor: 1, resignErr: true}
	db := newDB(t)
	resolver := NewDBOwnerResolver(db, time.Minute)
	require.NoError(t, resolver.EnsureTable(context.Background()))

	cfg := Config{
		InstanceID:        "v9-lifecycle",
		AdvertiseAddr:     "127.0.0.1:19099",
		HeartbeatInterval: 15 * time.Millisecond,
		LeaseTTL:          200 * time.Millisecond,
		PeerPollInterval:  15 * time.Millisecond,
	}
	lc := Start(context.Background(), cfg, mem, resolver,
		func(ctx context.Context, addr string, self PeerInfo) (*peerConn, error) {
			return nil, errors.New("v9: no dial")
		})
	require.NotNil(t, lc)
	assert.EqualValues(t, 3, lc.Epoch())
	require.NotNil(t, lc.Mesh())

	waitForV9(t, func() bool {
		mem.mu.Lock()
		defer mem.mu.Unlock()
		return mem.renewCalls >= 2 && mem.listCalls >= 1
	})

	lc.Stop(context.Background())
	mem.mu.Lock()
	defer mem.mu.Unlock()
	assert.True(t, mem.resigned)
}

func TestLifecycle_StopNilV9(t *testing.T) {
	var lc *Lifecycle
	assert.NotPanics(t, func() { lc.Stop(context.Background()) })
}
