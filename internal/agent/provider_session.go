// Package agent implements Agent-side session management for connected SDK Providers.
package agent

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	tcptr "github.com/cuihairu/croupier/internal/transport/tcp"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
)

// ProviderSession represents an established session with a connected SDK Provider.
// Agent dispatches Invoke/StartTask/CancelTask requests to the Provider through this session.
type ProviderSession struct {
	// conn is the bidirectional multiplexed connection.
	conn *tcptr.MuxConn

	// SessionID is the unique session identifier assigned on connect.
	SessionID string

	// ServiceID is the Provider's service identifier (e.g., "prom-adapter").
	ServiceID string

	// Version is the Provider's reported version.
	Version string

	// Functions are the function descriptors registered by this Provider.
	Functions []*sdkv1.ProviderFunctionDescriptor

	// ConnectedAt is the time the session was established.
	ConnectedAt time.Time

	// LastSeen is the most recent heartbeat or activity time.
	LastSeen atomic.Int64 // Unix timestamp

	// SDKLanguage is the SDK language (e.g., "go", "java", "python").
	SDKLanguage string

	// SDKVersion is the SDK release version.
	SDKVersion string

	// SDKName is the SDK display name (e.g., "croupier-js-sdk"); user-overridable.
	SDKName string
}

// Conn returns the underlying MuxConn for sending requests to this Provider.
func (s *ProviderSession) Conn() *tcptr.MuxConn {
	return s.conn
}

// UpdateLastSeen updates the LastSeen timestamp to now.
func (s *ProviderSession) UpdateLastSeen() {
	s.LastSeen.Store(time.Now().Unix())
}

// GetLastSeen returns the LastSeen time.
func (s *ProviderSession) GetLastSeen() time.Time {
	return time.Unix(s.LastSeen.Load(), 0)
}

// FunctionIDs returns the list of function IDs registered by this Provider.
func (s *ProviderSession) FunctionIDs() []string {
	ids := make([]string, 0, len(s.Functions))
	for _, f := range s.Functions {
		if f != nil && f.Id != "" {
			ids = append(ids, f.Id)
		}
	}
	return ids
}

// Close closes the underlying connection.
func (s *ProviderSession) Close() error {
	if s.conn != nil {
		return s.conn.Close()
	}
	return nil
}

// ProviderSessionStore manages active Provider sessions.
// It provides thread-safe CRUD operations indexed by SessionID and ServiceID.
type ProviderSessionStore struct {
	mu          sync.RWMutex
	bySessionID map[string]*ProviderSession // key: SessionID
	byServiceID map[string]*ProviderSession // key: ServiceID
}

// NewProviderSessionStore creates a new ProviderSessionStore.
func NewProviderSessionStore() *ProviderSessionStore {
	return &ProviderSessionStore{
		bySessionID: make(map[string]*ProviderSession),
		byServiceID: make(map[string]*ProviderSession),
	}
}

// Add registers a new Provider session.
// Returns an error if a session with the same SessionID already exists.
func (store *ProviderSessionStore) Add(sess *ProviderSession) error {
	store.mu.Lock()
	defer store.mu.Unlock()

	if _, exists := store.bySessionID[sess.SessionID]; exists {
		return fmt.Errorf("provider session already exists: %s", sess.SessionID)
	}

	sess.UpdateLastSeen()
	store.bySessionID[sess.SessionID] = sess
	store.byServiceID[sess.ServiceID] = sess
	return nil
}

// Upsert adds or replaces a Provider session.
func (store *ProviderSessionStore) Upsert(sess *ProviderSession) {
	store.mu.Lock()
	defer store.mu.Unlock()

	// Close existing connection if replacing by service ID
	if existing, exists := store.byServiceID[sess.ServiceID]; exists {
		_ = existing.Close()
		// Also remove old session ID mapping if different
		if existing.SessionID != sess.SessionID {
			delete(store.bySessionID, existing.SessionID)
		}
	}

	sess.UpdateLastSeen()
	store.bySessionID[sess.SessionID] = sess
	store.byServiceID[sess.ServiceID] = sess
}

// GetBySessionID returns the Provider session by SessionID.
func (store *ProviderSessionStore) GetBySessionID(sessionID string) (*ProviderSession, bool) {
	store.mu.RLock()
	defer store.mu.RUnlock()

	sess, ok := store.bySessionID[sessionID]
	return sess, ok
}

// GetByServiceID returns the Provider session by ServiceID.
func (store *ProviderSessionStore) GetByServiceID(serviceID string) (*ProviderSession, bool) {
	store.mu.RLock()
	defer store.mu.RUnlock()

	sess, ok := store.byServiceID[serviceID]
	return sess, ok
}

// Remove removes a Provider session by SessionID and closes its connection.
func (store *ProviderSessionStore) Remove(sessionID string) {
	store.mu.Lock()
	defer store.mu.Unlock()

	if sess, ok := store.bySessionID[sessionID]; ok {
		_ = sess.Close()
		delete(store.byServiceID, sess.ServiceID)
		delete(store.bySessionID, sessionID)
	}
}

// RemoveByServiceID removes a Provider session by ServiceID.
func (store *ProviderSessionStore) RemoveByServiceID(serviceID string) {
	store.mu.Lock()
	defer store.mu.Unlock()

	if sess, ok := store.byServiceID[serviceID]; ok {
		_ = sess.Close()
		delete(store.bySessionID, sess.SessionID)
		delete(store.byServiceID, serviceID)
	}
}

// List returns all active Provider sessions.
func (store *ProviderSessionStore) List() []*ProviderSession {
	store.mu.RLock()
	defer store.mu.RUnlock()

	result := make([]*ProviderSession, 0, len(store.bySessionID))
	for _, sess := range store.bySessionID {
		result = append(result, sess)
	}
	return result
}

// Count returns the number of active sessions.
func (store *ProviderSessionStore) Count() int {
	store.mu.RLock()
	defer store.mu.RUnlock()
	return len(store.bySessionID)
}

// PruneStale removes sessions that haven't been seen within the given TTL.
func (store *ProviderSessionStore) PruneStale(ttl time.Duration) int {
	store.mu.Lock()
	defer store.mu.Unlock()

	now := time.Now()
	pruned := 0
	for id, sess := range store.bySessionID {
		if now.Sub(sess.GetLastSeen()) > ttl {
			_ = sess.Close()
			delete(store.byServiceID, sess.ServiceID)
			delete(store.bySessionID, id)
			pruned++
		}
	}
	return pruned
}
