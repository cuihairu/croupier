// Package session provides shared session runtime abstractions for Croupier.
// This example demonstrates how to adapt existing session types to use
// the shared BaseSession and BaseStore.
package session

import (
	"fmt"
	"time"

	"github.com/cuihairu/croupier/internal/transport/tcp"
)

// ExampleAdapter shows how existing session types can embed BaseSession.
//
// BEFORE (duplicated code):
//   type AgentSession struct {
//       conn     *tcp.MuxConn
//       AgentID  string
//       SessionID string
//       Version  string
//       ConnectedAt time.Time
//       LastSeen atomic.Int64
//       GameID   string
//       Env      string
//   }
//
// AFTER (shared runtime):
//   type AgentSession struct {
//       *BaseSession           // embedded for common fields
//       AgentID  string        // agent-specific
//       GameID   string        // agent-specific
//       Env      string        // agent-specific
//   }

// AgentSessionAdapter demonstrates refactoring AgentSession to use BaseSession.
type AgentSessionAdapter struct {
	*BaseSession

	// Agent-specific fields
	AgentID string
	GameID  string
	Env     string
}

// NewAgentSessionAdapter creates a new AgentSession using shared BaseSession.
func NewAgentSessionAdapter(conn *tcp.MuxConn, agentID, sessionID, gameID, env, version string) *AgentSessionAdapter {
	return &AgentSessionAdapter{
		BaseSession: NewBaseSession(conn, sessionID, version),
		AgentID:     agentID,
		GameID:      gameID,
		Env:         env,
	}
}

// AgentSessionStoreAdapter demonstrates using BaseStore for Agent sessions.
type AgentSessionStoreAdapter struct {
	*BaseStore
}

// NewAgentSessionStoreAdapter creates a new Agent session store.
func NewAgentSessionStoreAdapter() *AgentSessionStoreAdapter {
	return &AgentSessionStoreAdapter{
		BaseStore: NewBaseStore(),
	}
}

// Add adds an Agent session indexed by AgentID.
func (s *AgentSessionStoreAdapter) Add(sess *AgentSessionAdapter) error {
	return s.BaseStore.Add(sess.AgentID, sess)
}

// Upsert adds or replaces an Agent session.
func (s *AgentSessionStoreAdapter) Upsert(sess *AgentSessionAdapter) {
	s.BaseStore.Upsert(sess.AgentID, sess, true) // close existing connection
}

// Get returns the Agent session by AgentID.
func (s *AgentSessionStoreAdapter) Get(agentID string) (*AgentSessionAdapter, bool) {
	sess, ok := s.BaseStore.Get(agentID)
	if !ok {
		return nil, false
	}
	return sess.(*AgentSessionAdapter), true
}

// Remove removes an Agent session by AgentID.
func (s *AgentSessionStoreAdapter) Remove(agentID string) {
	s.BaseStore.Remove(agentID)
}

// List returns all active Agent sessions.
func (s *AgentSessionStoreAdapter) List() []*AgentSessionAdapter {
	all := s.BaseStore.List()
	result := make([]*AgentSessionAdapter, 0, len(all))
	for _, sess := range all {
		result = append(result, sess.(*AgentSessionAdapter))
	}
	return result
}

// Example: ProviderSessionAdapter follows the same pattern.
type ProviderSessionAdapter struct {
	*BaseSession

	// Provider-specific fields
	ServiceID   string
	Functions   []string // function IDs
	SDKLanguage string
	SDKVersion  string
}

// NewProviderSessionAdapter creates a new ProviderSession using shared BaseSession.
func NewProviderSessionAdapter(conn *tcp.MuxConn, sessionID, serviceID, version string) *ProviderSessionAdapter {
	return &ProviderSessionAdapter{
		BaseSession: NewBaseSession(conn, sessionID, version),
		ServiceID:   serviceID,
		Functions:   make([]string, 0),
	}
}

// ProviderSessionStoreAdapter demonstrates using BaseStore for Provider sessions.
// It maintains dual indexing by SessionID and ServiceID.
type ProviderSessionStoreAdapter struct {
	bySessionID *BaseStore
	byServiceID *BaseStore
}

// NewProviderSessionStoreAdapter creates a new Provider session store.
func NewProviderSessionStoreAdapter() *ProviderSessionStoreAdapter {
	return &ProviderSessionStoreAdapter{
		bySessionID: NewBaseStore(),
		byServiceID: NewBaseStore(),
	}
}

// Add registers a new Provider session.
func (s *ProviderSessionStoreAdapter) Add(sess *ProviderSessionAdapter) error {
	if err := s.bySessionID.Add(sess.SessionID(), sess); err != nil {
		return err
	}
	s.byServiceID.Upsert(sess.ServiceID, sess, true)
	return nil
}

// GetBySessionID returns the Provider session by SessionID.
func (s *ProviderSessionStoreAdapter) GetBySessionID(sessionID string) (*ProviderSessionAdapter, bool) {
	sess, ok := s.bySessionID.Get(sessionID)
	if !ok {
		return nil, false
	}
	return sess.(*ProviderSessionAdapter), true
}

// GetByServiceID returns the Provider session by ServiceID.
func (s *ProviderSessionStoreAdapter) GetByServiceID(serviceID string) (*ProviderSessionAdapter, bool) {
	sess, ok := s.byServiceID.Get(serviceID)
	if !ok {
		return nil, false
	}
	return sess.(*ProviderSessionAdapter), true
}

// Remove removes a Provider session by SessionID.
func (s *ProviderSessionStoreAdapter) Remove(sessionID string) {
	sess, ok := s.bySessionID.Get(sessionID)
	if ok {
		providerSess := sess.(*ProviderSessionAdapter)
		s.byServiceID.Remove(providerSess.ServiceID)
	}
	s.bySessionID.Remove(sessionID)
}

// Example demonstrates using HeartbeatManager.
func Example() {
	// Create store
	store := NewAgentSessionStoreAdapter()

	// Create heartbeat manager with 30s TTL, checking every 10s
	manager := NewHeartbeatManager(store, 30*time.Second, 10*time.Second,
		func(session any) {
			// Optional: custom handler when a session is pruned
			sess := session.(*AgentSessionAdapter)
			fmt.Printf("Pruned stale session: %s\n", sess.AgentID)
		})

	// Start heartbeat manager in background
	manager.Start()
	defer manager.Stop()

	// Sessions are automatically pruned if not updated within TTL
	// Use session.UpdateLastSeen() on heartbeat/activity
}

// ExampleNewAgentSessionAdapter demonstrates basic session operations.
func ExampleNewAgentSessionAdapter() {
	// In real usage, conn would be from accepted connection
	// var conn *tcp.MuxConn

	// Create session
	// sess := NewAgentSessionAdapter(conn, "agent-1", "sess-123", "game-1", "prod", "1.0.0")

	// Session can send requests
	// respMsgID, respBody, err := sess.Conn().Call(context.Background(), msgID, reqBody)

	// Update heartbeat
	// sess.UpdateLastSeen()

	// Check if stale
	// if sess.IsStale(30 * time.Second) {
	//     sess.Close()
	// }
}
