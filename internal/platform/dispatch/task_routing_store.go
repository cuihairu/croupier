package dispatch

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type TaskRouting struct {
	TaskID    string    `json:"taskId"`
	AgentID   string    `json:"agentId"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type TaskRoutingStore interface {
	Get(taskID string) (*TaskRouting, error)
	Set(taskID, agentID string) error
	Delete(taskID string) error
	List() ([]*TaskRouting, error)
	Cleanup(ttl time.Duration) error
	Close() error
}

type FileTaskRoutingStore struct {
	filepath string
	mu       sync.RWMutex
	routings map[string]*TaskRouting
}

func NewFileTaskRoutingStore(dataDir string) (*FileTaskRoutingStore, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	fp := filepath.Join(dataDir, "task_routing.json")
	store := &FileTaskRoutingStore{
		filepath: fp,
		routings: make(map[string]*TaskRouting),
	}

	if err := store.load(); err != nil {
		return nil, fmt.Errorf("failed to load task routing data: %w", err)
	}

	return store, nil
}

func (s *FileTaskRoutingStore) Get(taskID string) (*TaskRouting, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	routing, exists := s.routings[taskID]
	if !exists {
		return nil, fmt.Errorf("task routing not found")
	}

	return routing, nil
}

func (s *FileTaskRoutingStore) Set(taskID, agentID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	routing := &TaskRouting{
		TaskID:    taskID,
		AgentID:   agentID,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if existing, exists := s.routings[taskID]; exists {
		routing.CreatedAt = existing.CreatedAt
	}

	s.routings[taskID] = routing
	return s.save()
}

func (s *FileTaskRoutingStore) Delete(taskID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.routings, taskID)
	return s.save()
}

func (s *FileTaskRoutingStore) List() ([]*TaskRouting, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	list := make([]*TaskRouting, 0, len(s.routings))
	for _, routing := range s.routings {
		list = append(list, routing)
	}
	return list, nil
}

func (s *FileTaskRoutingStore) Cleanup(ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := time.Now().Add(-ttl)
	deleted := 0

	for taskID, routing := range s.routings {
		if routing.UpdatedAt.Before(cutoff) {
			delete(s.routings, taskID)
			deleted++
		}
	}

	if deleted > 0 {
		return s.save()
	}
	return nil
}

func (s *FileTaskRoutingStore) Close() error {
	return nil
}

func (s *FileTaskRoutingStore) load() error {
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

	var routings map[string]*TaskRouting
	if err := json.Unmarshal(data, &routings); err != nil {
		return err
	}

	s.routings = routings
	return nil
}

func (s *FileTaskRoutingStore) save() error {
	data, err := json.MarshalIndent(s.routings, "", "  ")
	if err != nil {
		return err
	}

	tmpfile := s.filepath + ".tmp"
	if err := os.WriteFile(tmpfile, data, 0644); err != nil {
		return err
	}

	return os.Rename(tmpfile, s.filepath)
}

type MemoryTaskRoutingStore struct {
	mu       sync.RWMutex
	routings map[string]*TaskRouting
}

func NewMemoryTaskRoutingStore() *MemoryTaskRoutingStore {
	return &MemoryTaskRoutingStore{
		routings: make(map[string]*TaskRouting),
	}
}

func (s *MemoryTaskRoutingStore) Get(taskID string) (*TaskRouting, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	routing, exists := s.routings[taskID]
	if !exists {
		return nil, fmt.Errorf("task routing not found")
	}
	return routing, nil
}

func (s *MemoryTaskRoutingStore) Set(taskID, agentID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	routing := &TaskRouting{
		TaskID:    taskID,
		AgentID:   agentID,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if existing, exists := s.routings[taskID]; exists {
		routing.CreatedAt = existing.CreatedAt
	}

	s.routings[taskID] = routing
	return nil
}

func (s *MemoryTaskRoutingStore) Delete(taskID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.routings, taskID)
	return nil
}

func (s *MemoryTaskRoutingStore) List() ([]*TaskRouting, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	list := make([]*TaskRouting, 0, len(s.routings))
	for _, routing := range s.routings {
		list = append(list, routing)
	}
	return list, nil
}

func (s *MemoryTaskRoutingStore) Cleanup(ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := time.Now().Add(-ttl)
	for taskID, routing := range s.routings {
		if routing.UpdatedAt.Before(cutoff) {
			delete(s.routings, taskID)
		}
	}
	return nil
}

func (s *MemoryTaskRoutingStore) Close() error {
	return nil
}
