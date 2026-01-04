package agentlocal

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	localv1 "github.com/cuihairu/croupier/pkg/pb/croupier/agent/local/v1"
)

type Instance struct {
	ServiceID string
	Addr      string
	Version   string
	LastSeen  time.Time
}

type LocalStore struct {
	mu sync.RWMutex
	// function_id -> instances
	data map[string][]Instance
	// function_id -> service_id -> function version
	funcVersions map[string]map[string]string
	// job_id -> job result
	jobResults map[string]*JobResult
	// callback for updates
	onUpdate func()
}

// JobResult stores the result of a job
type JobResult struct {
	JobID     string
	State     string // pending, running, completed, failed, cancelled
	Payload   []byte
	Error     string
	CreatedAt time.Time
	UpdatedAt time.Time
}

func NewLocalStore() *LocalStore {
	return &LocalStore{
		data:         map[string][]Instance{},
		funcVersions: map[string]map[string]string{},
		jobResults:   map[string]*JobResult{},
	}
}

// OnUpdate sets a callback to be invoked when the store changes.
func (s *LocalStore) OnUpdate(fn func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	slog.Debug("[agentlocal] OnUpdate callback set")
	s.onUpdate = fn
}

// Register replaces instances for the provided function ids for a service.
func (s *LocalStore) Register(serviceID, addr, version string, funcs []*localv1.LocalFunctionDescriptor) {
	s.mu.Lock()
	defer s.mu.Unlock()
	slog.Debug("[agentlocal] Register called", "service_id", serviceID, "function_count", len(funcs))
	now := time.Now()
	// remove prior instances from this serviceID for all functions
	for fid, arr := range s.data {
		next := arr[:0]
		for _, it := range arr {
			if it.ServiceID != serviceID {
				next = append(next, it)
			}
		}
		if len(next) == 0 {
			delete(s.data, fid)
		} else {
			s.data[fid] = next
		}
	}
	for fid, svc := range s.funcVersions {
		delete(svc, serviceID)
		if len(svc) == 0 {
			delete(s.funcVersions, fid)
		}
	}
	inst := Instance{ServiceID: serviceID, Addr: addr, Version: version, LastSeen: now}
	for _, fn := range funcs {
		if fn == nil || fn.GetId() == "" {
			continue
		}
		fid := fn.GetId()
		s.data[fid] = append(s.data[fid], inst)
		if fn.GetVersion() != "" {
			if s.funcVersions[fid] == nil {
				s.funcVersions[fid] = map[string]string{}
			}
			s.funcVersions[fid][serviceID] = fn.GetVersion()
		}
	}
	if s.onUpdate != nil {
		fmt.Println("DEBUG: Triggering OnUpdate")
		go s.onUpdate()
	} else {
		fmt.Println("DEBUG: OnUpdate is nil")
	}
}

// Heartbeat updates last seen for a service across all functions.
func (s *LocalStore) Heartbeat(serviceID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for fid, arr := range s.data {
		for i := range arr {
			if arr[i].ServiceID == serviceID {
				arr[i].LastSeen = now
			}
		}
		s.data[fid] = arr
	}
}

// List snapshot of functions and instances.
func (s *LocalStore) List() map[string][]Instance {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string][]Instance, len(s.data))
	for fid, arr := range s.data {
		cp := make([]Instance, len(arr))
		copy(cp, arr)
		out[fid] = cp
	}
	return out
}

// Prune removes instances older than maxAge; returns removed count.
func (s *LocalStore) Prune(maxAge time.Duration) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	removed := 0
	for fid, arr := range s.data {
		next := arr[:0]
		for _, it := range arr {
			if now.Sub(it.LastSeen) <= maxAge {
				next = append(next, it)
			} else {
				removed++
			}
		}
		if len(next) == 0 {
			delete(s.data, fid)
		} else {
			s.data[fid] = next
		}
	}
	if removed > 0 && s.onUpdate != nil {
		go s.onUpdate()
	}
	return removed
}

// FunctionVersions returns snapshot of function versions by service.
func (s *LocalStore) FunctionVersions() map[string]map[string]string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]map[string]string, len(s.funcVersions))
	for fid, svcVersions := range s.funcVersions {
		cp := make(map[string]string, len(svcVersions))
		for sid, ver := range svcVersions {
			cp[sid] = ver
		}
		out[fid] = cp
	}
	return out
}

// SetJobResult stores or updates a job result
func (s *LocalStore) SetJobResult(jobID, state string, payload []byte, errorMsg string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	if result, exists := s.jobResults[jobID]; exists {
		result.State = state
		result.Payload = payload
		result.Error = errorMsg
		result.UpdatedAt = now
	} else {
		result := &JobResult{
			JobID:     jobID,
			State:     state,
			Payload:   payload,
			Error:     errorMsg,
			CreatedAt: now,
			UpdatedAt: now,
		}
		s.jobResults[jobID] = result
	}
}

// GetJobResult retrieves a job result
func (s *LocalStore) GetJobResult(jobID string) (*JobResult, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result, exists := s.jobResults[jobID]
	return result, exists
}

// RemoveJobResult removes a job result
func (s *LocalStore) RemoveJobResult(jobID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.jobResults, jobID)
}

// CleanupOldJobResults removes job results older than maxAge
func (s *LocalStore) CleanupOldJobResults(maxAge time.Duration) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	removed := 0

	for jobID, result := range s.jobResults {
		if now.Sub(result.UpdatedAt) > maxAge {
			delete(s.jobResults, jobID)
			removed++
		}
	}

	return removed
}
