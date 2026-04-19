package dispatch

import (
	"context"
	"fmt"
	"log"
	"net"
	"sort"
	"strings"
	"sync"
	"time"

	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/platform/tlsutil"
	"github.com/cuihairu/croupier/internal/transport"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	"google.golang.org/protobuf/proto"
)

// AgentSessionResolver finds active TCP sessions for connected Agents.
// The server package's AgentSessionStore implements this interface.
type AgentSessionResolver interface {
	ResolveAgentConn(agentID string) (transport.SessionCaller, bool)
}

// Dispatcher routes function invocations to live agents discovered via registry store.
// Uses TCP session routing for all agent communication.
// Supports HA features: health tracking, circuit breaker, load balancing.
type Dispatcher struct {
	store         *reg.Store
	mu            sync.RWMutex
	taskRouting   map[string]string // taskID -> agentID (in-memory cache)
	taskStore     TaskRoutingStore  // persistent storage for task routing
	dialTimeout   time.Duration
	invokeTimeout time.Duration
	tlsCfg        *tlsutil.ClientTLSConfig

	// TCP session routing
	sessionResolver AgentSessionResolver

	// HA features
	healthTracker *HealthTracker
	loadBalancer  *LoadBalancer
	haEnabled     bool
}

func NewDispatcher(store *reg.Store) *Dispatcher {
	return NewDispatcherWithTaskStore(store, nil)
}

// NewDispatcherWithTaskStore creates a new Dispatcher with optional task routing store
func NewDispatcherWithTaskStore(store *reg.Store, taskStore TaskRoutingStore) *Dispatcher {
	return NewDispatcherWithHA(store, taskStore, false, StrategyMinID, nil)
}

// NewDispatcherWithHA creates a new Dispatcher with HA features enabled
func NewDispatcherWithHA(store *reg.Store, taskStore TaskRoutingStore, haEnabled bool, strategy LoadBalanceStrategy, healthConfig *HealthCheckConfig) *Dispatcher {
	if store == nil {
		store = reg.NewStore()
	}

	// Default to memory store if none provided.
	if taskStore == nil {
		taskStore = NewMemoryTaskRoutingStore()
	}

	d := &Dispatcher{
		store:         store,
		taskRouting:   map[string]string{},
		taskStore:     taskStore,
		dialTimeout:   5 * time.Second,
		invokeTimeout: 15 * time.Second,
		haEnabled:     haEnabled,
	}

	// Initialize HA components if enabled
	if haEnabled {
		if healthConfig == nil {
			healthConfig = DefaultHealthCheckConfig()
		}
		d.healthTracker = NewHealthTracker(healthConfig)
		d.loadBalancer = NewLoadBalancer(strategy, d.healthTracker)
		d.healthTracker.Start()
	}

	// Load existing task routing from persistent store.
	d.loadTaskRouting()

	return d
}

// SetHAEnabled enables or disables HA features at runtime
func (d *Dispatcher) SetHAEnabled(enabled bool) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.haEnabled == enabled {
		return
	}

	if enabled && d.healthTracker == nil {
		healthConfig := DefaultHealthCheckConfig()
		d.healthTracker = NewHealthTracker(healthConfig)
		d.loadBalancer = NewLoadBalancer(StrategyMinID, d.healthTracker)
		d.healthTracker.Start()
	} else if !enabled && d.healthTracker != nil {
		d.healthTracker.Stop()
		d.healthTracker = nil
		d.loadBalancer = nil
	}

	d.haEnabled = enabled
}

// SetLoadBalanceStrategy changes the load balancing strategy
func (d *Dispatcher) SetLoadBalanceStrategy(strategy LoadBalanceStrategy) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.loadBalancer != nil {
		d.loadBalancer.SetStrategy(strategy)
	}
}

// GetLoadBalanceStrategy returns the current load balancing strategy
func (d *Dispatcher) GetLoadBalanceStrategy() LoadBalanceStrategy {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if d.loadBalancer != nil {
		return d.loadBalancer.GetStrategy()
	}
	return StrategyMinID
}

// GetHealthTracker returns the health tracker (for monitoring/inspection)
func (d *Dispatcher) GetHealthTracker() *HealthTracker {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.healthTracker
}

// GetLoadBalancer returns the load balancer (for monitoring/inspection)
func (d *Dispatcher) GetLoadBalancer() *LoadBalancer {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.loadBalancer
}

func (d *Dispatcher) SetTLSConfig(cfg *tlsutil.ClientTLSConfig) {
	d.tlsCfg = cfg
}

// SetSessionResolver sets the TCP session resolver for routing requests over
// established Agent sessions.
func (d *Dispatcher) SetSessionResolver(resolver AgentSessionResolver) {
	d.sessionResolver = resolver
}

func (d *Dispatcher) Store() *reg.Store {
	return d.store
}

func (d *Dispatcher) Invoke(ctx context.Context, functionID string, payload []byte) ([]byte, error) {
	resp, err := d.InvokeRequest(ctx, &sdkv1.InvokeRequest{
		FunctionId: functionID,
		Payload:    payload,
	})
	if err != nil {
		return nil, err
	}
	return resp.GetPayload(), nil
}

// InvokeRequest forwards a fully populated InvokeRequest to a live agent.
func (d *Dispatcher) InvokeRequest(ctx context.Context, req *sdkv1.InvokeRequest) (*sdkv1.InvokeResponse, error) {
	if req == nil || req.GetFunctionId() == "" {
		return nil, fmt.Errorf("function id is required")
	}
	agent, err := d.pickAgentWithRouting(req.GetFunctionId(), req.Metadata)
	if err != nil {
		return nil, err
	}

	// Track connection start
	if d.healthTracker != nil {
		d.healthTracker.IncrementConnections(agent.AgentID)
		defer d.healthTracker.DecrementConnections(agent.AgentID)
	}

	if req.Metadata == nil {
		req.Metadata = map[string]string{}
	}
	req.Metadata["agent_id"] = agent.AgentID

	reqBytes, err := proto.Marshal(req)
	if err != nil {
		if d.healthTracker != nil {
			d.healthTracker.RecordFailure(agent.AgentID)
		}
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	respBytes, err := d.callAgent(ctx, agent.AgentID, protocol.MsgInvokeRequest, reqBytes)
	if err != nil {
		if d.healthTracker != nil {
			d.healthTracker.RecordFailure(agent.AgentID)
		}
		return nil, err
	}

	resp := &sdkv1.InvokeResponse{}
	if err := proto.Unmarshal(respBytes, resp); err != nil {
		if d.healthTracker != nil {
			d.healthTracker.RecordFailure(agent.AgentID)
		}
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	if d.healthTracker != nil {
		d.healthTracker.RecordSuccess(agent.AgentID)
	}

	return resp, nil
}

func (d *Dispatcher) StartTask(ctx context.Context, functionID string, payload []byte) (string, error) {
	resp, err := d.StartTaskRequest(ctx, &sdkv1.InvokeRequest{
		FunctionId: functionID,
		Payload:    payload,
	})
	if err != nil {
		return "", err
	}
	return resp.GetTaskId(), nil
}

// StartTaskRequest forwards a structured InvokeRequest to the agent StartTask RPC.
func (d *Dispatcher) StartTaskRequest(ctx context.Context, req *sdkv1.InvokeRequest) (*sdkv1.StartTaskResponse, error) {
	if req == nil || req.GetFunctionId() == "" {
		return nil, fmt.Errorf("function id is required")
	}
	agent, err := d.pickAgentWithRouting(req.GetFunctionId(), req.Metadata)
	if err != nil {
		return nil, err
	}

	if d.healthTracker != nil {
		d.healthTracker.IncrementConnections(agent.AgentID)
		defer d.healthTracker.DecrementConnections(agent.AgentID)
	}

	if req.Metadata == nil {
		req.Metadata = map[string]string{}
	}
	req.Metadata["agent_id"] = agent.AgentID

	reqBytes, err := proto.Marshal(req)
	if err != nil {
		if d.healthTracker != nil {
			d.healthTracker.RecordFailure(agent.AgentID)
		}
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	respBytes, err := d.callAgent(ctx, agent.AgentID, protocol.MsgStartTaskRequest, reqBytes)
	if err != nil {
		if d.healthTracker != nil {
			d.healthTracker.RecordFailure(agent.AgentID)
		}
		return nil, err
	}

	resp := &sdkv1.StartTaskResponse{}
	if err := proto.Unmarshal(respBytes, resp); err != nil {
		if d.healthTracker != nil {
			d.healthTracker.RecordFailure(agent.AgentID)
		}
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	if d.healthTracker != nil {
		d.healthTracker.RecordSuccess(agent.AgentID)
	}

	if taskID := resp.GetTaskId(); taskID != "" {
		d.registerTask(taskID, agent.AgentID)
	}

	return resp, nil
}

func (d *Dispatcher) CancelTask(ctx context.Context, taskID string) error {
	agentID, err := d.taskAgentID(taskID)
	if err != nil {
		return err
	}

	req := &sdkv1.CancelTaskRequest{TaskId: taskID}
	reqBytes, err := proto.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	_, err = d.callAgent(ctx, agentID, protocol.MsgCancelTaskRequest, reqBytes)
	if err == nil {
		d.unregisterTask(taskID)
	}

	return err
}

func (d *Dispatcher) StreamTask(ctx context.Context, taskID string) ([]*sdkv1.TaskEvent, bool, error) {
	return nil, false, fmt.Errorf("streaming not yet implemented")
}

// StreamTaskRealtime forwards task events to the provided callback.
func (d *Dispatcher) StreamTaskRealtime(ctx context.Context, taskID string, fn func(*sdkv1.TaskEvent) bool) (bool, error) {
	return false, fmt.Errorf("streaming not yet implemented")
}

// ListFunctionAgents returns agent IDs that currently expose the function.
func (d *Dispatcher) ListFunctionAgents(functionID string) []string {
	now := time.Now()
	d.store.Mu().RLock()
	defer d.store.Mu().RUnlock()
	var ids []string
	for _, agent := range d.store.AgentsUnsafe() {
		if agent == nil || agent.AgentID == "" {
			continue
		}
		if !agent.ExpireAt.After(now) {
			continue
		}
		if meta, ok := agent.Functions[functionID]; ok && meta.Enabled {
			ids = append(ids, agent.AgentID)
		}
	}
	return ids
}

func agentHasService(agent *reg.AgentSession, providerID, functionID string) bool {
	if agent == nil || providerID == "" {
		return false
	}
	for _, p := range agent.Providers {
		if p.ProviderID != providerID {
			continue
		}
		if functionID == "" {
			return true
		}
		for _, fid := range p.FunctionIDs {
			if fid == functionID {
				return true
			}
		}
		return false
	}
	return false
}

func pickAgentByHash(candidates []*reg.AgentSession, key string) *reg.AgentSession {
	if len(candidates) == 0 {
		return nil
	}
	if len(candidates) == 1 {
		return candidates[0]
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return candidates[0]
	}
	// FNV-1a
	var h uint32 = 2166136261
	for i := 0; i < len(key); i++ {
		h ^= uint32(key[i])
		h *= 16777619
	}
	idx := int(h % uint32(len(candidates)))
	return candidates[idx]
}

// TaskAgentID exposes tracked task routing (primarily for diagnostics).
// Returns the agentID for the task.
func (d *Dispatcher) TaskAgentID(taskID string) (string, bool) {
	d.mu.RLock()
	agentID, ok := d.taskRouting[taskID]
	d.mu.RUnlock()
	return agentID, ok
}

// pickAgent returns a live agent that owns the function.
func (d *Dispatcher) pickAgent(functionID string) (*reg.AgentSession, error) {
	now := time.Now()
	d.store.Mu().RLock()
	defer d.store.Mu().RUnlock()

	// Collect all candidates
	var candidates []*reg.AgentSession
	for _, agent := range d.store.AgentsUnsafe() {
		if agent == nil {
			continue
		}
		if !agent.ExpireAt.After(now) {
			continue
		}
		meta, ok := agent.Functions[functionID]
		if !ok || !meta.Enabled {
			continue
		}
		candidates = append(candidates, agent)
	}

	if len(candidates) == 0 {
		return nil, fmt.Errorf("no live agent for function %s", functionID)
	}

	// Use load balancer if HA is enabled
	d.mu.RLock()
	useLoadBalancer := d.loadBalancer != nil
	tracker := d.healthTracker
	d.mu.RUnlock()

	if useLoadBalancer && tracker != nil {
		// Build candidates with health state
		candidateList := d.loadBalancer.BuildCandidates(candidates, functionID)
		if len(candidateList) == 0 {
			return nil, fmt.Errorf("no healthy agents available for function %s", functionID)
		}

		selected, err := d.loadBalancer.Select(functionID, candidateList)
		if err != nil {
			return nil, err
		}
		return selected.Session, nil
	}

	// Fallback to original min_id selection
	var chosen *reg.AgentSession
	for _, agent := range candidates {
		if chosen == nil || agent.AgentID < chosen.AgentID {
			chosen = agent
		}
	}
	return chosen, nil
}

func (d *Dispatcher) pickAgentWithRouting(functionID string, metadata map[string]string) (*reg.AgentSession, error) {
	if metadata == nil {
		return d.pickAgent(functionID)
	}

	serviceID := strings.TrimSpace(metadata["target_service_id"])
	hashKey := strings.TrimSpace(metadata["hash_key"])

	now := time.Now()
	d.store.Mu().RLock()
	defer d.store.Mu().RUnlock()

	// Targeted: choose the agent that owns the service_id.
	if serviceID != "" {
		for _, agent := range d.store.AgentsUnsafe() {
			if agent == nil || !agent.ExpireAt.After(now) {
				continue
			}
			meta, ok := agent.Functions[functionID]
			if !ok || !meta.Enabled {
				continue
			}
			if agentHasService(agent, serviceID, functionID) {
				return agent, nil
			}
		}
		return nil, fmt.Errorf("no live agent for function %s with service_id %s", functionID, serviceID)
	}

	// Hash: choose a stable agent among all candidates.
	if hashKey != "" {
		cands := make([]*reg.AgentSession, 0)
		for _, agent := range d.store.AgentsUnsafe() {
			if agent == nil || !agent.ExpireAt.After(now) {
				continue
			}
			meta, ok := agent.Functions[functionID]
			if !ok || !meta.Enabled {
				continue
			}
			cands = append(cands, agent)
		}
		sort.Slice(cands, func(i, j int) bool {
			return cands[i].AgentID < cands[j].AgentID
		})
		chosen := pickAgentByHash(cands, hashKey)
		if chosen == nil {
			return nil, fmt.Errorf("no live agent for function %s", functionID)
		}
		return chosen, nil
	}

	return d.pickAgent(functionID)
}

// callAgent sends a request to an Agent via its established TCP session.
func (d *Dispatcher) callAgent(ctx context.Context, agentID string, msgID uint32, reqBody []byte) ([]byte, error) {
	if d.sessionResolver == nil {
		return nil, fmt.Errorf("session resolver not configured")
	}

	caller, ok := d.sessionResolver.ResolveAgentConn(agentID)
	if !ok {
		return nil, fmt.Errorf("agent %s session not found", agentID)
	}

	callCtx, cancel := context.WithTimeout(ctx, d.invokeTimeout)
	defer cancel()

	_, respBody, err := caller.Call(callCtx, msgID, reqBody)
	return respBody, err
}

// callAgentByAddr sends a request to an Agent by address.
// NOTE: This method is not functional in TCP-only mode.
// Task routing stores agentID, not RPC addr.
func (d *Dispatcher) callAgentByAddr(ctx context.Context, rpcAddr string, msgID uint32, reqBody []byte) ([]byte, error) {
	return nil, fmt.Errorf("callAgentByAddr not supported in TCP-only mode; task routing stores agentID")
}

func hostFromAddr(addr string) string {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return ""
	}
	host, _, err := net.SplitHostPort(addr)
	if err == nil {
		return strings.Trim(host, "[]")
	}
	return strings.Trim(strings.TrimPrefix(addr, "["), "]")
}

// RegisterTask registers a task routing.
func (d *Dispatcher) RegisterTask(taskID, agentID string) {
	d.registerTask(taskID, agentID)
}

// ListTaskRoutings returns persisted task routing entries.
func (d *Dispatcher) ListTaskRoutings() ([]*TaskRouting, error) {
	if d == nil || d.taskStore == nil {
		return []*TaskRouting{}, nil
	}
	return d.taskStore.List()
}

// UnregisterTask unregisters a task routing.
func (d *Dispatcher) UnregisterTask(taskID string) {
	d.unregisterTask(taskID)
}

func (d *Dispatcher) taskAgentID(taskID string) (string, error) {
	d.mu.RLock()
	agentID, ok := d.taskRouting[taskID]
	d.mu.RUnlock()
	if !ok {
		if d.taskStore != nil {
			routing, err := d.taskStore.Get(taskID)
			if err == nil && routing != nil && routing.AgentID != "" {
				d.mu.Lock()
				d.taskRouting[taskID] = routing.AgentID
				d.mu.Unlock()
				return routing.AgentID, nil
			}
		}
		return "", fmt.Errorf("task %s not tracked", taskID)
	}
	return agentID, nil
}

func isTerminalEvent(evt *sdkv1.TaskEvent) bool {
	switch strings.ToLower(evt.GetType()) {
	case "done", "completed", "error", "failed", "cancelled", "canceled", "succeeded", "success":
		return true
	default:
		return false
	}
}

// loadTaskRouting loads task routing from persistent store.
func (d *Dispatcher) loadTaskRouting() {
	routings, err := d.taskStore.List()
	if err != nil {
		// Log error but continue with empty cache
		log.Printf("[dispatch] Warning: failed to load task routing from store: %v", err)
		return
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	// Rebuild in-memory cache.
	d.taskRouting = make(map[string]string, len(routings))
	for _, routing := range routings {
		d.taskRouting[routing.TaskID] = routing.AgentID
	}
}

// registerTask registers task routing to both memory cache and persistent store.
func (d *Dispatcher) registerTask(taskID, agentID string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Update memory cache
	d.taskRouting[taskID] = agentID

	// Update persistent store
	if err := d.taskStore.Set(taskID, agentID); err != nil {
		// Log error but continue
		log.Printf("[dispatch] Warning: failed to persist task routing: %v", err)
	}
}

// unregisterTask removes task routing from both memory cache and persistent store.
func (d *Dispatcher) unregisterTask(taskID string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Remove from memory cache
	delete(d.taskRouting, taskID)

	// Remove from persistent store
	if err := d.taskStore.Delete(taskID); err != nil {
		// Log error but continue
		log.Printf("[dispatch] Warning: failed to delete task routing: %v", err)
	}
}

// CleanupOldTasks removes old task routing entries.
func (d *Dispatcher) CleanupOldTasks(ttl time.Duration) error {
	// Cleanup persistent store
	if err := d.taskStore.Cleanup(ttl); err != nil {
		return err
	}

	// Reload cache to sync with persistent store
	d.loadTaskRouting()

	return nil
}

// Close closes the dispatcher and its resources
func (d *Dispatcher) Close() error {
	// Stop health tracker if enabled
	d.mu.Lock()
	if d.healthTracker != nil {
		d.healthTracker.Stop()
	}
	d.mu.Unlock()

	return d.taskStore.Close()
}
