// Package session provides shared session runtime abstractions for Croupier.
// This package extracts common session management logic used in both
// Server-Agent and Agent-SDK communication paths.
package session

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cuihairu/croupier/internal/transport/tcp"
)

// Conn is the minimal interface for session connections.
// Both MuxConn and other transport types implement this.
type Conn interface {
	Call(ctx context.Context, msgID uint32, reqBody []byte) (respMsgID uint32, respBody []byte, err error)
	Close() error
	RemoteAddr() string
}

// Session is implemented by BaseSession and other session types.
type Session interface {
	SessionID() string
	UpdateLastSeen()
	GetLastSeen() time.Time
	IsStale(ttl time.Duration) bool
	Close() error
}

// BaseSession provides common session state and behavior.
// It handles connection management, heartbeat tracking, and lifecycle.
type BaseSession struct {
	// conn is the bidirectional multiplexed connection.
	conn *tcp.MuxConn

	// sessionID is the unique session identifier.
	sessionID string

	// Version is the peer's reported version.
	Version string

	// ConnectedAt is the time the session was established.
	ConnectedAt time.Time

	// lastSeen tracks the most recent heartbeat or activity time.
	lastSeen atomic.Int64 // Unix timestamp
}

// NewBaseSession creates a new BaseSession.
func NewBaseSession(conn *tcp.MuxConn, sessionID, version string) *BaseSession {
	s := &BaseSession{
		conn:        conn,
		sessionID:   sessionID,
		Version:     version,
		ConnectedAt: time.Now(),
	}
	s.lastSeen.Store(time.Now().Unix())
	return s
}

// SessionID returns the unique session identifier.
func (s *BaseSession) SessionID() string {
	return s.sessionID
}

// Conn returns the underlying MuxConn for sending requests.
func (s *BaseSession) Conn() *tcp.MuxConn {
	return s.conn
}

// UpdateLastSeen updates the LastSeen timestamp to now.
func (s *BaseSession) UpdateLastSeen() {
	s.lastSeen.Store(time.Now().Unix())
}

// GetLastSeen returns the LastSeen time.
func (s *BaseSession) GetLastSeen() time.Time {
	return time.Unix(s.lastSeen.Load(), 0)
}

// Close closes the underlying connection.
func (s *BaseSession) Close() error {
	if s.conn != nil {
		return s.conn.Close()
	}
	return nil
}

// IsStale returns true if the session hasn't been seen within the given TTL.
func (s *BaseSession) IsStale(ttl time.Duration) bool {
	return time.Since(s.GetLastSeen()) > ttl
}

// RemoteAddr returns the remote peer address.
func (s *BaseSession) RemoteAddr() string {
	if s.conn != nil {
		return s.conn.RemoteAddr()
	}
	return ""
}

// LocalAddr returns the local bound address.
func (s *BaseSession) LocalAddr() string {
	if s.conn != nil {
		return s.conn.LocalAddr()
	}
	return ""
}

// SessionKey identifies a session in a store.
type SessionKey struct {
	// ID is the unique identifier (e.g., AgentID, ServiceID, SessionID).
	ID string

	// Type is the session type for namespacing (e.g., "agent", "provider").
	Type string
}

// String returns a string representation of the key.
func (k SessionKey) String() string {
	return k.Type + ":" + k.ID
}

// StaleHandler is called when a stale session is pruned.
type StaleHandler func(session any) // session is *AgentSession or *ProviderSession

// HeartbeatManager manages periodic heartbeat checks and session pruning.
type HeartbeatManager struct {
	store    SessionStore
	ttl      time.Duration
	interval time.Duration
	handler  StaleHandler

	stopCh chan struct{}
	wg     sync.WaitGroup
	mu     sync.Mutex
}

// NewHeartbeatManager creates a new HeartbeatManager.
func NewHeartbeatManager(store SessionStore, ttl, interval time.Duration, handler StaleHandler) *HeartbeatManager {
	return &HeartbeatManager{
		store:    store,
		ttl:      ttl,
		interval: interval,
		handler:  handler,
		stopCh:   make(chan struct{}),
	}
}

// Start begins the heartbeat check loop.
func (m *HeartbeatManager) Start() {
	m.mu.Lock()
	defer m.mu.Unlock()

	select {
	case <-m.stopCh:
		// Already stopped
		return
	default:
	}

	m.wg.Add(1)
	go m.run()
}

// Stop stops the heartbeat check loop.
func (m *HeartbeatManager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	select {
	case <-m.stopCh:
		// Already stopped
		return
	default:
	}

	close(m.stopCh)
	m.wg.Wait()
	m.stopCh = make(chan struct{})
}

func (m *HeartbeatManager) run() {
	defer m.wg.Done()

	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.pruneStale()
		}
	}
}

func (m *HeartbeatManager) pruneStale() {
	pruned := m.store.PruneStale(m.ttl)
	if pruned > 0 && m.handler != nil {
		// Handler is called per-session in the store implementation
	}
}
