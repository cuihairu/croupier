package agentlocal

import (
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/cuihairu/croupier/internal/function/converter"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
)

type Instance struct {
	ProviderID string
	Addr       string
	Version    string
	LastSeen   time.Time
}

// ProviderSession represents a registered provider (one OpenAPI file)
type ProviderSession struct {
	ProviderID   string          // Provider unique identifier
	GameID       string          // Game ID for game/environment scoping
	Env          string          // Logical environment (prod/dev/staging)
	Addr         string          // Provider address
	Version      string          // Provider version
	LastSeenUnix int64           // Last seen timestamp (Unix)
	FunctionIDs  []string        // List of function IDs provided
	OpenAPIDoc   json.RawMessage // Complete OpenAPI 3.0.3 document
}

// FunctionMeta stores metadata for a function including OpenAPI schema fields.
// Based on OpenAPI 3.0.3 Operation Object
type FunctionMeta struct {
	ID           string
	Version      string
	Tags         []string
	Summary      string
	Description  string
	OperationID  string
	Deprecated   bool
	InputSchema  string // JSON Schema for request body (OpenAPI 3.0.3)
	OutputSchema string // JSON Schema for response body (OpenAPI 3.0.3)

	// Additional OpenAPI fields
	Category  string // x-category extension
	Risk      string // x-risk extension
	Entity    string // x-entity extension
	Operation string // x-operation extension (create/read/update/delete/custom)

	// Full OpenAPI operation as JSON (optional, for advanced use cases)
	OpenAPIOperation string // Complete OpenAPI 3.0.3 Operation object as JSON string
}

type LocalStore struct {
	mu sync.RWMutex
	// function_id -> instances
	data map[string][]Instance
	// function_id -> service_id -> function version
	funcVersions map[string]map[string]string
	// function_id -> FunctionMeta (stores the latest metadata for each function)
	funcMeta map[string]*FunctionMeta
	// task_id -> task result
	taskResults map[string]*TaskResult
	// callback for updates
	onUpdate func()
}

// TaskResult stores the result of a task.
type TaskResult struct {
	TaskID    string
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
		funcMeta:     map[string]*FunctionMeta{},
		taskResults:  map[string]*TaskResult{},
	}
}

// OnUpdate sets a callback to be invoked when the store changes.
func (s *LocalStore) OnUpdate(fn func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	slog.Debug("[agentlocal] OnUpdate callback set")
	s.onUpdate = fn
}

// Register replaces instances for the provided function ids for a provider.
func (s *LocalStore) Register(providerID, addr, version string, funcs []*sdkv1.LocalFunctionDescriptor) {
	s.mu.Lock()
	defer s.mu.Unlock()
	slog.Debug("[agentlocal] Register called", "provider_id", providerID, "function_count", len(funcs))
	now := time.Now()
	// remove prior instances from this providerID for all functions
	for fid, arr := range s.data {
		next := arr[:0]
		for _, it := range arr {
			if it.ProviderID != providerID {
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
		delete(svc, providerID)
		if len(svc) == 0 {
			delete(s.funcVersions, fid)
		}
	}
	inst := Instance{ProviderID: providerID, Addr: addr, Version: version, LastSeen: now}
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
			s.funcVersions[fid][providerID] = fn.GetVersion()
		}
		// Store function metadata including OpenAPI schema
		meta := &FunctionMeta{
			ID:           fid,
			Version:      fn.GetVersion(),
			Tags:         fn.GetTags(),
			Summary:      fn.GetSummary(),
			Description:  fn.GetDescription(),
			OperationID:  fn.GetOperationId(),
			Deprecated:   fn.GetDeprecated(),
			InputSchema:  fn.GetInputSchema(),
			OutputSchema: fn.GetOutputSchema(),
			Category:     fn.GetCategory(),
			Risk:         fn.GetRisk(),
			Entity:       fn.GetEntity(),
			Operation:    fn.GetOperation(),
		}
		if op, err := converter.ToOpenAPIOperation(converter.LocalFunctionDescriptorDesc{
			ID:           meta.ID,
			Version:      meta.Version,
			Tags:         meta.Tags,
			Summary:      meta.Summary,
			Description:  meta.Description,
			OperationID:  meta.OperationID,
			Deprecated:   meta.Deprecated,
			InputSchema:  meta.InputSchema,
			OutputSchema: meta.OutputSchema,
			Category:     meta.Category,
			Risk:         meta.Risk,
			Entity:       meta.Entity,
			Operation:    meta.Operation,
		}); err == nil {
			if opJSON, marshalErr := json.Marshal(op); marshalErr == nil {
				meta.OpenAPIOperation = string(opJSON)
			}
		} else {
			slog.Warn("[agentlocal] failed to convert function metadata to openapi operation", "function_id", fid, "error", err)
		}
		s.funcMeta[fid] = meta
	}
	slog.Info("[agentlocal] Registered functions", "provider_id", providerID, "count", len(funcs), "store_size", len(s.data))
	if s.onUpdate != nil {
		slog.Debug("[agentlocal] Triggering OnUpdate callback")
		go s.onUpdate()
	} else {
		slog.Debug("[agentlocal] OnUpdate callback is nil, skipping")
	}
}

// Heartbeat updates last seen for a provider across all functions.
func (s *LocalStore) Heartbeat(providerID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for fid, arr := range s.data {
		for i := range arr {
			if arr[i].ProviderID == providerID {
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

// FunctionMetadata returns snapshot of function metadata including schema.
func (s *LocalStore) FunctionMetadata() map[string]*FunctionMeta {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]*FunctionMeta, len(s.funcMeta))
	for fid, meta := range s.funcMeta {
		if meta == nil {
			continue
		}
		// Copy to avoid data races
		cp := &FunctionMeta{
			ID:               meta.ID,
			Version:          meta.Version,
			Tags:             append([]string(nil), meta.Tags...),
			Summary:          meta.Summary,
			Description:      meta.Description,
			OperationID:      meta.OperationID,
			Deprecated:       meta.Deprecated,
			InputSchema:      meta.InputSchema,
			OutputSchema:     meta.OutputSchema,
			Category:         meta.Category,
			Risk:             meta.Risk,
			Entity:           meta.Entity,
			Operation:        meta.Operation,
			OpenAPIOperation: meta.OpenAPIOperation,
		}
		out[fid] = cp
	}
	return out
}

// SetTaskResult stores or updates a task result.
func (s *LocalStore) SetTaskResult(taskID, state string, payload []byte, errorMsg string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	if result, exists := s.taskResults[taskID]; exists {
		result.State = state
		result.Payload = payload
		result.Error = errorMsg
		result.UpdatedAt = now
	} else {
		result := &TaskResult{
			TaskID:    taskID,
			State:     state,
			Payload:   payload,
			Error:     errorMsg,
			CreatedAt: now,
			UpdatedAt: now,
		}
		s.taskResults[taskID] = result
	}
}

// GetTaskResult retrieves a task result.
func (s *LocalStore) GetTaskResult(taskID string) (*TaskResult, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result, exists := s.taskResults[taskID]
	return result, exists
}

// RemoveTaskResult removes a task result.
func (s *LocalStore) RemoveTaskResult(taskID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.taskResults, taskID)
}

// CleanupOldTaskResults removes task results older than maxAge.
func (s *LocalStore) CleanupOldTaskResults(maxAge time.Duration) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	removed := 0

	for taskID, result := range s.taskResults {
		if now.Sub(result.UpdatedAt) > maxAge {
			delete(s.taskResults, taskID)
			removed++
		}
	}

	return removed
}
