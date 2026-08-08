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
	Metadata   map[string]string // 元数据（sdk_language, sdk_version 等，参考 Nacos）
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

	Resource   string // x-resource extension
	Operation  string // x-operation extension (business action key)
	Capability string // x-capability extension
	Execution  string // x-execution extension
	Risk       string // x-risk extension
	Permission string // x-permission extension

	// Full OpenAPI operation as JSON (optional, for advanced use cases)
	OpenAPIOperation string // Complete OpenAPI 3.0.3 Operation object as JSON string
}

type LocalStore struct {
	mu sync.RWMutex
	// function_id -> service_id -> instances (双层索引，参考 Nacos)
	data map[string]map[string][]Instance
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
		data:         map[string]map[string][]Instance{},
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
// Uses double-index: function_id -> service_id -> instances (Nacos style)
func (s *LocalStore) Register(providerID, serviceID, addr, version string, funcs []*sdkv1.ProviderFunctionDescriptor, metadata map[string]string) {
	// An empty function list is a no-op, NOT a clear. Registering nothing
	// must never silently wipe a provider's existing functions — that was the
	// demo-site "nothing works" root cause (heartbeat called Register(nil) to
	// "update LastSeen" and cleared everything). Use RemoveProvider to clear.
	if len(funcs) == 0 {
		return
	}
	if serviceID == "" {
		serviceID = "__default__" // 默认服务ID，类似 Nacos 的 DEFAULT_GROUP
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	slog.Debug("[agentlocal] Register called", "provider_id", providerID, "service_id", serviceID, "function_count", len(funcs))
	now := time.Now()
	s.removeProviderLocked(providerID)
	// 复制 metadata 避免外部修改影响内部状态
	metaCopy := make(map[string]string, len(metadata))
	for k, v := range metadata {
		metaCopy[k] = v
	}
	inst := Instance{ProviderID: providerID, Addr: addr, Version: version, Metadata: metaCopy, LastSeen: now}
	for _, fn := range funcs {
		if fn == nil || fn.GetId() == "" {
			continue
		}
		fid := fn.GetId()
		// 双层索引：function_id -> service_id -> instances
		if s.data[fid] == nil {
			s.data[fid] = map[string][]Instance{}
		}
		s.data[fid][serviceID] = append(s.data[fid][serviceID], inst)
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
			Resource:     fn.GetResource(),
			Operation:    fn.GetOperation(),
			Capability:   fn.GetCapability(),
			Execution:    fn.GetExecution(),
			Risk:         fn.GetRisk(),
			Permission:   fn.GetPermission(),
		}
		if op, err := converter.ToOpenAPIOperation(converter.ProviderFunctionDescriptorDesc{
			ID:           meta.ID,
			Version:      meta.Version,
			Tags:         meta.Tags,
			Summary:      meta.Summary,
			Description:  meta.Description,
			OperationID:  meta.OperationID,
			Deprecated:   meta.Deprecated,
			InputSchema:  meta.InputSchema,
			OutputSchema: meta.OutputSchema,
			Resource:     meta.Resource,
			Operation:    meta.Operation,
			Capability:   meta.Capability,
			Execution:    meta.Execution,
			Risk:         meta.Risk,
			Permission:   meta.Permission,
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

// removeProviderLocked deletes all function instances and version entries
// for providerID. Caller must hold s.mu.
func (s *LocalStore) removeProviderLocked(providerID string) {
	for fid, serviceMap := range s.data {
		for sid, arr := range serviceMap {
			next := arr[:0]
			for _, it := range arr {
				if it.ProviderID != providerID {
					next = append(next, it)
				}
			}
			if len(next) == 0 {
				delete(serviceMap, sid)
			} else {
				serviceMap[sid] = next
			}
		}
		if len(serviceMap) == 0 {
			delete(s.data, fid)
		}
	}
	for fid, svc := range s.funcVersions {
		delete(svc, providerID)
		if len(svc) == 0 {
			delete(s.funcVersions, fid)
		}
	}
}

// RemoveProvider explicitly removes all functions registered by providerID.
// This is the only way to clear a provider's functions — Register treats an
// empty function list as a no-op (see Register doc comment).
func (s *LocalStore) RemoveProvider(providerID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.removeProviderLocked(providerID)
	slog.Info("[agentlocal] Removed provider", "provider_id", providerID, "store_size", len(s.data))
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
	for _, serviceMap := range s.data {
		for _, arr := range serviceMap {
			for i := range arr {
				if arr[i].ProviderID == providerID {
					arr[i].LastSeen = now
				}
			}
		}
	}
}

// List snapshot of functions and instances (flattened for compatibility).
func (s *LocalStore) List() map[string][]Instance {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string][]Instance, len(s.data))
	for fid, serviceMap := range s.data {
		var all []Instance
		for _, arr := range serviceMap {
			cp := make([]Instance, len(arr))
			copy(cp, arr)
			all = append(all, cp...)
		}
		if len(all) > 0 {
			out[fid] = all
		}
	}
	return out
}

// ListByService returns double-index snapshot for routing (Nacos style).
func (s *LocalStore) ListByService() map[string]map[string][]Instance {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]map[string][]Instance, len(s.data))
	for fid, serviceMap := range s.data {
		sm := make(map[string][]Instance, len(serviceMap))
		for sid, arr := range serviceMap {
			cp := make([]Instance, len(arr))
			copy(cp, arr)
			sm[sid] = cp
		}
		out[fid] = sm
	}
	return out
}

// Prune removes instances older than maxAge; returns removed count.
func (s *LocalStore) Prune(maxAge time.Duration) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	removed := 0
	for fid, serviceMap := range s.data {
		for sid, arr := range serviceMap {
			next := arr[:0]
			for _, it := range arr {
				if now.Sub(it.LastSeen) <= maxAge {
					next = append(next, it)
				} else {
					removed++
				}
			}
			if len(next) == 0 {
				delete(serviceMap, sid)
			} else {
				serviceMap[sid] = next
			}
		}
		if len(serviceMap) == 0 {
			delete(s.data, fid)
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
			Resource:         meta.Resource,
			Operation:        meta.Operation,
			Capability:       meta.Capability,
			Execution:        meta.Execution,
			Risk:             meta.Risk,
			Permission:       meta.Permission,
			OpenAPIOperation: meta.OpenAPIOperation,
		}
		out[fid] = cp
	}
	return out
}

func cloneStringMap(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]string, len(input))
	for key, value := range input {
		out[key] = value
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
