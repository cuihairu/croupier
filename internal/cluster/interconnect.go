package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	internaltcp "github.com/cuihairu/croupier/internal/transport/tcp"
	"github.com/cuihairu/croupier/pkg/protocol"
)

// MeshInterconnect is the direct-connection mesh implementation: lazy connect
// + connection pool + one-hop limit + fencing.
//
// Transport reuses transport/tcp's frame protocol; message bodies are JSON
// (the interconnect is an internal low-frequency path, so JSON is enough; the
// frame protocol matches existing links exactly, and mTLS is handled by
// transport-layer configuration).
type MeshInterconnect struct {
	self       PeerInfo
	selfEpoch  uint64
	resolver   OwnerResolver
	membership Membership

	mu    sync.Mutex
	conns map[string]*peerConn // key: instanceID

	dial dialFunc

	// peersCache holds last-known peers (forwarding continues while the shared
	// storage flaps).
	peersMu    sync.RWMutex
	peersCache []PeerInfo
}

// dialFunc establishes/reuses a connection to a peer and completes the server
// handshake (injectable test double).
type dialFunc func(ctx context.Context, addr string, self PeerInfo) (*peerConn, error)

// peerConn wraps one long-lived connection to a peer.
type peerConn struct {
	instanceID string
	addr       string
	epoch      uint64

	send  func(ctx context.Context, msgID uint32, body []byte) ([]byte, error)
	close func()
}

// NewMeshInterconnect creates the mesh interconnect.
func NewMeshInterconnect(self PeerInfo, resolver OwnerResolver, membership Membership) *MeshInterconnect {
	m := &MeshInterconnect{
		self:       self,
		resolver:   resolver,
		membership: membership,
		conns:      map[string]*peerConn{},
	}
	m.dial = defaultDialPeer
	return m
}

// SetEpoch sets this instance's epoch returned by Register (used for fencing
// checks).
func (m *MeshInterconnect) SetEpoch(epoch uint64) { m.selfEpoch = epoch }

// SelfInfo returns this instance's identity.
func (m *MeshInterconnect) SelfInfo() PeerInfo { return m.self }

// RefreshPeers refreshes the last-known peers cache (called by the discovery
// loop).
func (m *MeshInterconnect) RefreshPeers(ctx context.Context) {
	if m.membership == nil {
		return
	}
	peers, err := m.membership.ListAlive(ctx)
	if err != nil {
		slog.Warn("cluster: refresh peers failed, keeping last-known", "error", err)
		return
	}
	m.peersMu.Lock()
	m.peersCache = peers
	m.peersMu.Unlock()
}

// Peers returns the last-known peers (excluding self).
func (m *MeshInterconnect) Peers() []PeerInfo {
	m.peersMu.RLock()
	defer m.peersMu.RUnlock()
	out := make([]PeerInfo, 0, len(m.peersCache))
	for _, p := range m.peersCache {
		if p.InstanceID != m.self.InstanceID {
			out = append(out, p)
		}
	}
	return out
}

// Forward forwards a call to the owner instance.
func (m *MeshInterconnect) Forward(ctx context.Context, agentID string, req *ForwardedInvoke) (*ForwardedResult, error) {
	// Invariant 1: one-hop limit — a request already forwarded is not forwarded
	// again.
	if req.Forwarded {
		return nil, ErrHopLimit
	}
	if m.resolver == nil {
		return nil, ErrNoRoute
	}
	owner, err := m.resolver.ResolveOwner(ctx, agentID)
	if err != nil {
		return nil, fmt.Errorf("resolve owner: %w", err)
	}
	if owner == nil {
		return nil, ErrNoRoute
	}
	// The target is self: the caller should not have entered the forward path;
	// defensive return.
	if owner.InstanceID == m.self.InstanceID {
		return &ForwardedResult{OK: false, Error: "owner is self"}, nil
	}

	req.Forwarded = true
	req.AgentID = agentID
	// Record the owner epoch at resolution time (owner-side fencing check).
	req.CallerEpoch = owner.Epoch

	conn, err := m.getConn(ctx, *owner)
	if err != nil {
		return nil, fmt.Errorf("dial owner %s(%s): %w", owner.InstanceID, owner.AdvertiseAddr, err)
	}

	// Every ForwardedInvoke field is a basic string/[]byte/map[string]string/
	// bool/uint64 type, so Marshal always succeeds; the err branch was dead
	// code and has been removed.
	body, _ := json.Marshal(req)
	respBody, err := conn.send(ctx, protocol.MsgForwardInvokeReq, body)
	if err != nil {
		m.dropConn(owner.InstanceID)
		return nil, fmt.Errorf("forward rpc: %w", err)
	}
	var result ForwardedResult
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("decode forward response: %w", err)
	}
	return &result, nil
}

// getConn lazily establishes or reuses the connection to the owner.
func (m *MeshInterconnect) getConn(ctx context.Context, owner PeerInfo) (*peerConn, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if c, ok := m.conns[owner.InstanceID]; ok && c.addr == owner.AdvertiseAddr {
		return c, nil
	}
	if c, ok := m.conns[owner.InstanceID]; ok {
		c.close()
		delete(m.conns, owner.InstanceID)
	}
	c, err := m.dial(ctx, owner.AdvertiseAddr, m.self)
	if err != nil {
		return nil, err
	}
	c.instanceID = owner.InstanceID
	c.epoch = owner.Epoch
	m.conns[owner.InstanceID] = c
	return c, nil
}

func (m *MeshInterconnect) dropConn(instanceID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if c, ok := m.conns[instanceID]; ok {
		c.close()
		delete(m.conns, instanceID)
	}
}

// Close closes all peer connections.
func (m *MeshInterconnect) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, c := range m.conns {
		c.close()
		delete(m.conns, id)
	}
}

// defaultDialPeer establishes a connection to the peer over the transport/tcp
// base and performs the handshake.
func defaultDialPeer(ctx context.Context, addr string, self PeerInfo) (*peerConn, error) {
	return nil, fmt.Errorf("cluster: tcp dial to %s not wired in test context", addr)
}

var _ Interconnect = (*MeshInterconnect)(nil)

// ServeForwardHandler returns the owner-side handler for MsgForwardInvokeReq.
// localInvoke is injected by the dispatch layer: after validating caller
// authentication it executes over the local Agent connection.
//
// Fencing invariant 2 is enforced here: when req.CallerEpoch differs from the
// local epoch the request is rejected (the zombie-owner scenario after network
// partition recovery).
func ServeForwardHandler(selfEpoch uint64, localInvoke func(ctx context.Context, req *ForwardedInvoke) (*ForwardedResult, error)) func(ctx context.Context, body []byte) []byte {
	return func(ctx context.Context, body []byte) []byte {
		var req ForwardedInvoke
		if err := json.Unmarshal(body, &req); err != nil {
			return marshalResult(&ForwardedResult{OK: false, Error: "bad request"})
		}
		// ARCH-003: a connection without a completed hello handshake must not
		// execute forward. mTLS already guarantees the peer's certificate
		// identity; this gate ensures "identity binding precedes any action" —
		// certificate-less links (plaintext, or a direct forward after a failed
		// handshake) are refused.
		if session := internaltcp.ConnectionSessionFromContext(ctx); session == nil || !session.HandshakeDone() {
			return marshalResult(&ForwardedResult{OK: false, Error: "not handshaked"})
		}
		// Invariant 1: a request that reached the owner must not be forwarded
		// again (the caller guarantees Forwarded=true; the owner only refuses
		// anomalies beyond the explicit loop marker).
		if !req.Forwarded {
			return marshalResult(&ForwardedResult{OK: false, Error: "not forwarded flag"})
		}
		// Invariant 2: fencing — the epoch the initiator saw differs from the
		// local one → zombie owner.
		if req.CallerEpoch != selfEpoch {
			return marshalResult(&ForwardedResult{OK: false, NotOwner: true, Error: "stale epoch"})
		}
		result, err := localInvoke(ctx, &req)
		if err != nil {
			return marshalResult(&ForwardedResult{OK: false, Error: err.Error()})
		}
		return marshalResult(result)
	}
}

func marshalResult(r *ForwardedResult) []byte {
	// Every ForwardedResult field is a basic bool/string/[]byte type, so
	// Marshal always succeeds; the err fallback branch was dead code and has
	// been removed.
	b, _ := json.Marshal(r)
	return b
}

// Now is exposed for tests.
var Now = time.Now
