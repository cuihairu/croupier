package session

import (
	"sync"
	"time"
)

// SessionStore defines the interface for session storage and management.
type SessionStore interface {
	// PruneStale removes sessions that haven't been seen within the given TTL.
	// Returns the number of sessions pruned.
	PruneStale(ttl time.Duration) int

	// Count returns the number of active sessions.
	Count() int
}

// BaseStore provides a generic thread-safe session store implementation.
// It manages sessions indexed by a string key with automatic stale pruning.
type BaseStore struct {
	mu       sync.RWMutex
	sessions map[string]*sessionWrapper
}

type sessionWrapper struct {
	session    Session
	lastPruned time.Time
}

// NewBaseStore creates a new BaseStore.
func NewBaseStore() *BaseStore {
	return &BaseStore{
		sessions: make(map[string]*sessionWrapper),
	}
}

// Add registers a new session.
// Returns an error if a session with the same key already exists.
func (s *BaseStore) Add(key string, session Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.sessions[key]; exists {
		return &SessionExistsError{Key: key}
	}

	session.UpdateLastSeen()
	s.sessions[key] = &sessionWrapper{session: session}
	return nil
}

// Upsert adds or replaces a session.
func (s *BaseStore) Upsert(key string, session Session, closeExisting bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	wrapper, exists := s.sessions[key]
	if exists && closeExisting && wrapper.session != nil {
		_ = wrapper.session.Close()
	}

	session.UpdateLastSeen()
	s.sessions[key] = &sessionWrapper{session: session}
}

// Get returns the session by key.
func (s *BaseStore) Get(key string) (Session, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	wrapper, ok := s.sessions[key]
	if !ok {
		return nil, false
	}
	return wrapper.session, true
}

// Remove removes a session by key and closes it.
func (s *BaseStore) Remove(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if wrapper, ok := s.sessions[key]; ok {
		if wrapper.session != nil {
			_ = wrapper.session.Close()
		}
		delete(s.sessions, key)
	}
}

// List returns all active sessions.
func (s *BaseStore) List() []Session {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]Session, 0, len(s.sessions))
	for _, wrapper := range s.sessions {
		if wrapper.session != nil {
			result = append(result, wrapper.session)
		}
	}
	return result
}

// Count returns the number of active sessions.
func (s *BaseStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.sessions)
}

// PruneStale removes sessions that haven't been seen within the given TTL.
func (s *BaseStore) PruneStale(ttl time.Duration) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	pruned := 0
	for key, wrapper := range s.sessions {
		if wrapper.session == nil {
			delete(s.sessions, key)
			pruned++
			continue
		}
		if now.Sub(wrapper.session.GetLastSeen()) > ttl {
			_ = wrapper.session.Close()
			delete(s.sessions, key)
			pruned++
		}
	}
	return pruned
}

// SessionExistsError is returned when adding a duplicate session.
type SessionExistsError struct {
	Key string
}

func (e *SessionExistsError) Error() string {
	return "session already exists: " + e.Key
}

// IsSessionExists returns true if err is a SessionExistsError.
func IsSessionExists(err error) bool {
	_, ok := err.(*SessionExistsError)
	return ok
}
