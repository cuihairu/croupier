package dispatch

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// JobRouting represents a job routing entry
type JobRouting struct {
	JobID     string    `json:"job_id"`
	AgentAddr string    `json:"agent_addr"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// JobRoutingStore defines the interface for job routing persistence
type JobRoutingStore interface {
	// Get retrieves job routing by job ID
	Get(jobID string) (*JobRouting, error)

	// Set stores or updates job routing
	Set(jobID, agentAddr string) error

	// Delete removes job routing
	Delete(jobID string) error

	// List returns all job routings
	List() ([]*JobRouting, error)

	// Cleanup removes old entries (older than ttl)
	Cleanup(ttl time.Duration) error

	// Close closes the store
	Close() error
}

// FileJobRoutingStore implements JobRoutingStore using file-based persistence
type FileJobRoutingStore struct {
	filepath string
	mu       sync.RWMutex
	routings map[string]*JobRouting
}

// NewFileJobRoutingStore creates a new file-based job routing store
func NewFileJobRoutingStore(dataDir string) (*FileJobRoutingStore, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	fp := filepath.Join(dataDir, "job_routing.json")
	store := &FileJobRoutingStore{
		filepath: fp,
		routings: make(map[string]*JobRouting),
	}

	// Load existing data
	if err := store.load(); err != nil {
		return nil, fmt.Errorf("failed to load job routing data: %w", err)
	}

	return store, nil
}

func (s *FileJobRoutingStore) Get(jobID string) (*JobRouting, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	routing, exists := s.routings[jobID]
	if !exists {
		return nil, fmt.Errorf("job routing not found")
	}

	return routing, nil
}

func (s *FileJobRoutingStore) Set(jobID, agentAddr string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	routing := &JobRouting{
		JobID:     jobID,
		AgentAddr: agentAddr,
		CreatedAt: now,
		UpdatedAt: now,
	}

	// Preserve created time if already exists
	if existing, exists := s.routings[jobID]; exists {
		routing.CreatedAt = existing.CreatedAt
	}

	s.routings[jobID] = routing

	return s.save()
}

func (s *FileJobRoutingStore) Delete(jobID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.routings, jobID)

	return s.save()
}

func (s *FileJobRoutingStore) List() ([]*JobRouting, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	list := make([]*JobRouting, 0, len(s.routings))
	for _, routing := range s.routings {
		list = append(list, routing)
	}

	return list, nil
}

func (s *FileJobRoutingStore) Cleanup(ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := time.Now().Add(-ttl)
	deleted := 0

	for jobID, routing := range s.routings {
		if routing.UpdatedAt.Before(cutoff) {
			delete(s.routings, jobID)
			deleted++
		}
	}

	if deleted > 0 {
		return s.save()
	}

	return nil
}

func (s *FileJobRoutingStore) Close() error {
	// File-based store doesn't need explicit closing
	return nil
}

func (s *FileJobRoutingStore) load() error {
	// If file doesn't exist, start with empty store
	if _, err := os.Stat(s.filepath); os.IsNotExist(err) {
		return nil
	}

	data, err := os.ReadFile(s.filepath)
	if err != nil {
		return err
	}

	if len(data) == 0 {
		return nil
	}

	var routings map[string]*JobRouting
	if err := json.Unmarshal(data, &routings); err != nil {
		return err
	}

	s.routings = routings
	return nil
}

func (s *FileJobRoutingStore) save() error {
	data, err := json.MarshalIndent(s.routings, "", "  ")
	if err != nil {
		return err
	}

	// Write to temporary file first, then rename to avoid corruption
	tmpfile := s.filepath + ".tmp"
	if err := os.WriteFile(tmpfile, data, 0644); err != nil {
		return err
	}

	return os.Rename(tmpfile, s.filepath)
}

// MemoryJobRoutingStore implements JobRoutingStore using in-memory storage
type MemoryJobRoutingStore struct {
	mu       sync.RWMutex
	routings map[string]*JobRouting
}

// NewMemoryJobRoutingStore creates a new in-memory job routing store
func NewMemoryJobRoutingStore() *MemoryJobRoutingStore {
	return &MemoryJobRoutingStore{
		routings: make(map[string]*JobRouting),
	}
}

func (s *MemoryJobRoutingStore) Get(jobID string) (*JobRouting, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	routing, exists := s.routings[jobID]
	if !exists {
		return nil, fmt.Errorf("job routing not found")
	}

	return routing, nil
}

func (s *MemoryJobRoutingStore) Set(jobID, agentAddr string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	routing := &JobRouting{
		JobID:     jobID,
		AgentAddr: agentAddr,
		CreatedAt: now,
		UpdatedAt: now,
	}

	// Preserve created time if already exists
	if existing, exists := s.routings[jobID]; exists {
		routing.CreatedAt = existing.CreatedAt
	}

	s.routings[jobID] = routing
	return nil
}

func (s *MemoryJobRoutingStore) Delete(jobID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.routings, jobID)
	return nil
}

func (s *MemoryJobRoutingStore) List() ([]*JobRouting, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	list := make([]*JobRouting, 0, len(s.routings))
	for _, routing := range s.routings {
		list = append(list, routing)
	}

	return list, nil
}

func (s *MemoryJobRoutingStore) Cleanup(ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := time.Now().Add(-ttl)

	for jobID, routing := range s.routings {
		if routing.UpdatedAt.Before(cutoff) {
			delete(s.routings, jobID)
		}
	}

	return nil
}

func (s *MemoryJobRoutingStore) Close() error {
	// Memory store doesn't need explicit closing
	return nil
}