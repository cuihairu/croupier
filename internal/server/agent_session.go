// Package server implements Server-side session management for connected Agents.
package server

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cuihairu/croupier/internal/transport"
	tcptr "github.com/cuihairu/croupier/internal/transport/tcp"
)

// AgentSession represents an established session with a connected Agent.
// Server dispatches Invoke/StartTask/CancelTask requests through this session.
type AgentSession struct {
	// conn is the bidirectional multiplexed connection.
	conn *tcptr.MuxConn

	// AgentID is the unique identifier of the connected Agent.
	AgentID string

	// SessionID is the unique session identifier assigned on registration.
	SessionID string

	// GameID is the game this agent belongs to.
	GameID string

	// Env is the deployment environment (e.g., "dev", "prod").
	Env string

	// Version is the agent's reported version.
	Version string

	// ConnectedAt is the time the session was established.
	ConnectedAt time.Time

	// LastSeen is the most recent heartbeat or activity time.
	LastSeen atomic.Int64 // Unix timestamp
}

// Conn returns the underlying MuxConn for sending requests to this Agent.
func (s *AgentSession) Conn() *tcptr.MuxConn {
	return s.conn
}

// Addr returns the remote address of the live TCP session. This replaces the
// legacy rpc_addr mirror — the agent's reachable address is the TCP session it
// established, not a self-published string.
func (s *AgentSession) Addr() string {
	if s == nil || s.conn == nil {
		return ""
	}
	return s.conn.RemoteAddr()
}

// UpdateLastSeen updates the LastSeen timestamp to now.
func (s *AgentSession) UpdateLastSeen() {
	s.LastSeen.Store(time.Now().Unix())
}

// GetLastSeen returns the LastSeen time.
func (s *AgentSession) GetLastSeen() time.Time {
	return time.Unix(s.LastSeen.Load(), 0)
}

// Close closes the underlying connection.
func (s *AgentSession) Close() error {
	if s.conn != nil {
		return s.conn.Close()
	}
	return nil
}

// AgentSessionStore manages active Agent sessions.
// It provides thread-safe CRUD operations indexed by AgentID.
type AgentSessionStore struct {
	mu       sync.RWMutex
	sessions map[string]*AgentSession // key: AgentID
}

// NewAgentSessionStore creates a new AgentSessionStore.
func NewAgentSessionStore() *AgentSessionStore {
	return &AgentSessionStore{
		sessions: make(map[string]*AgentSession),
	}
}

// Add registers a new Agent session.
// Returns an error if a session with the same AgentID already exists.
func (store *AgentSessionStore) Add(sess *AgentSession) error {
	store.mu.Lock()
	defer store.mu.Unlock()

	if _, exists := store.sessions[sess.AgentID]; exists {
		return fmt.Errorf("agent session already exists: %s", sess.AgentID)
	}

	sess.UpdateLastSeen()
	store.sessions[sess.AgentID] = sess
	return nil
}

// Upsert adds or replaces an Agent session.
func (store *AgentSessionStore) Upsert(sess *AgentSession) {
	store.mu.Lock()
	defer store.mu.Unlock()

	// Close existing connection if replacing
	if existing, exists := store.sessions[sess.AgentID]; exists && existing.conn != nil {
		// Best-effort close; the old connection is stale
		_ = existing.conn.Close()
	}

	sess.UpdateLastSeen()
	store.sessions[sess.AgentID] = sess
}

// Get returns the Agent session by AgentID.
func (store *AgentSessionStore) Get(agentID string) (*AgentSession, bool) {
	store.mu.RLock()
	defer store.mu.RUnlock()

	sess, ok := store.sessions[agentID]
	return sess, ok
}

// Remove removes an Agent session by AgentID and closes its connection.
// Deprecated: Use RemoveSession which compares sessionID to avoid deleting a
// newer session when an old connection disconnects after a reconnect.
func (store *AgentSessionStore) Remove(agentID string) {
	store.mu.Lock()
	defer store.mu.Unlock()

	if sess, ok := store.sessions[agentID]; ok {
		_ = sess.Close()
		delete(store.sessions, agentID)
	}
}

// RemoveSession removes the Agent session matching the given (agentID, sessionID)
// pair. It only deletes the session if the stored session's SessionID matches
// the provided sessionID. This prevents a stale connection's cleanup from
// deleting a newer session that replaced it during a reconnect.
//
// Returns true if a session was removed.
func (store *AgentSessionStore) RemoveSession(agentID, sessionID string) bool {
	store.mu.Lock()
	defer store.mu.Unlock()

	sess, ok := store.sessions[agentID]
	if !ok {
		return false
	}
	// Only remove if the stored session is the same one that is disconnecting.
	// If sessionID is empty, fall back to the old behavior for backward compat.
	if sessionID != "" && sess.SessionID != sessionID {
		return false
	}
	_ = sess.Close()
	delete(store.sessions, agentID)
	return true
}

// List returns all active Agent sessions.
func (store *AgentSessionStore) List() []*AgentSession {
	store.mu.RLock()
	defer store.mu.RUnlock()

	result := make([]*AgentSession, 0, len(store.sessions))
	for _, sess := range store.sessions {
		result = append(result, sess)
	}
	return result
}

// Count returns the number of active sessions.
func (store *AgentSessionStore) Count() int {
	store.mu.RLock()
	defer store.mu.RUnlock()
	return len(store.sessions)
}

// PruneStale removes sessions that haven't been seen within the given TTL.
func (store *AgentSessionStore) PruneStale(ttl time.Duration) int {
	store.mu.Lock()
	defer store.mu.Unlock()

	now := time.Now()
	pruned := 0
	for id, sess := range store.sessions {
		if now.Sub(sess.GetLastSeen()) > ttl {
			_ = sess.Close()
			delete(store.sessions, id)
			pruned++
		}
	}
	return pruned
}

// ResolveAgentConn returns the MuxConn for an agent's active TCP session.
// This implements the dispatch.AgentSessionResolver interface.
// Returns the MuxConn (which has Call()) or false if no session exists.
func (store *AgentSessionStore) ResolveAgentConn(agentID string) (*tcptr.MuxConn, bool) {
	store.mu.RLock()
	defer store.mu.RUnlock()

	sess, ok := store.sessions[agentID]
	if !ok || sess.conn == nil {
		return nil, false
	}
	return sess.conn, true
}

// ResolveSessionCaller returns a SessionCaller for dispatch.AgentSessionResolver.
func (store *AgentSessionStore) ResolveSessionCaller(agentID string) (transport.SessionCaller, bool) {
	store.mu.RLock()
	defer store.mu.RUnlock()

	sess, ok := store.sessions[agentID]
	if !ok || sess.conn == nil {
		return nil, false
	}
	return sess.conn, true
}

// SessionResolverAdapter adapts AgentSessionStore to dispatch.AgentSessionResolver.
type SessionResolverAdapter struct {
	store *AgentSessionStore
}

// NewSessionResolverAdapter creates a new adapter.
func NewSessionResolverAdapter(store *AgentSessionStore) *SessionResolverAdapter {
	return &SessionResolverAdapter{store: store}
}

// ResolveAgentConn implements dispatch.AgentSessionResolver.
func (a *SessionResolverAdapter) ResolveAgentConn(agentID string) (transport.SessionCaller, bool) {
	return a.store.ResolveSessionCaller(agentID)
}
