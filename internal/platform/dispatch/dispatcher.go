package dispatch

import (
	"context"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/cuihairu/croupier/internal/nng"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/platform/tlsutil"
	"github.com/cuihairu/croupier/pkg/protocol"
	sdkv1 "github.com/cuihairu/croupier/sdks/go/pkg/pb/croupier/sdk/v1"
	"google.golang.org/protobuf/proto"
)

// Dispatcher routes function invocations to live agents discovered via registry store.
// Now uses NNG instead of gRPC for Agent communication.
type Dispatcher struct {
	store         *reg.Store
	mu            sync.RWMutex
	jobRouting    map[string]string // jobID -> agent rpc addr (in-memory cache)
	jobStore      JobRoutingStore   // persistent storage for job routing
	dialTimeout   time.Duration
	invokeTimeout time.Duration
	tlsCfg        *tlsutil.ClientTLSConfig
	nngClients    map[string]*nng.Client // cached NNG clients
	clientsMu     sync.RWMutex
}

func NewDispatcher(store *reg.Store) *Dispatcher {
	return NewDispatcherWithJobStore(store, nil)
}

// NewDispatcherWithJobStore creates a new Dispatcher with optional job routing store
func NewDispatcherWithJobStore(store *reg.Store, jobStore JobRoutingStore) *Dispatcher {
	if store == nil {
		store = reg.NewStore()
	}

	// Default to memory store if none provided
	if jobStore == nil {
		jobStore = NewMemoryJobRoutingStore()
	}

	d := &Dispatcher{
		store:         store,
		jobRouting:    map[string]string{},
		jobStore:      jobStore,
		nngClients:    make(map[string]*nng.Client),
		dialTimeout:   5 * time.Second,
		invokeTimeout: 15 * time.Second,
	}

	// Load existing job routing from persistent store
	d.loadJobRouting()

	return d
}

func (d *Dispatcher) SetTLSConfig(cfg *tlsutil.ClientTLSConfig) {
	d.tlsCfg = cfg
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

	client, err := d.getNNGClient(agent.RPCAddr)
	if err != nil {
		return nil, err
	}

	callCtx, cancel := context.WithTimeout(ctx, d.invokeTimeout)
	defer cancel()

	if req.Metadata == nil {
		req.Metadata = map[string]string{}
	}
	req.Metadata["agent_id"] = agent.AgentID

	// Marshal request
	reqBytes, err := proto.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	// Send via NNG
	respBytes, err := client.Call(callCtx, protocol.MsgInvokeRequest, reqBytes)
	if err != nil {
		return nil, err
	}

	resp := &sdkv1.InvokeResponse{}
	if err := proto.Unmarshal(respBytes, resp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return resp, nil
}

func (d *Dispatcher) StartJob(ctx context.Context, functionID string, payload []byte) (string, error) {
	resp, err := d.StartJobRequest(ctx, &sdkv1.InvokeRequest{
		FunctionId: functionID,
		Payload:    payload,
	})
	if err != nil {
		return "", err
	}
	return resp.GetJobId(), nil
}

// StartJobRequest forwards a structured InvokeRequest to the agent StartJob RPC.
func (d *Dispatcher) StartJobRequest(ctx context.Context, req *sdkv1.InvokeRequest) (*sdkv1.StartJobResponse, error) {
	if req == nil || req.GetFunctionId() == "" {
		return nil, fmt.Errorf("function id is required")
	}
	agent, err := d.pickAgentWithRouting(req.GetFunctionId(), req.Metadata)
	if err != nil {
		return nil, err
	}

	client, err := d.getNNGClient(agent.RPCAddr)
	if err != nil {
		return nil, err
	}

	callCtx, cancel := context.WithTimeout(ctx, d.invokeTimeout)
	defer cancel()

	if req.Metadata == nil {
		req.Metadata = map[string]string{}
	}
	req.Metadata["agent_id"] = agent.AgentID

	// Marshal request
	reqBytes, err := proto.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	// Send via NNG
	respBytes, err := client.Call(callCtx, protocol.MsgStartJobRequest, reqBytes)
	if err != nil {
		return nil, err
	}

	resp := &sdkv1.StartJobResponse{}
	if err := proto.Unmarshal(respBytes, resp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	if jobID := resp.GetJobId(); jobID != "" {
		d.registerJob(jobID, agent.RPCAddr)
	}

	return resp, nil
}

func (d *Dispatcher) CancelJob(ctx context.Context, jobID string) error {
	addr, err := d.jobAddr(jobID)
	if err != nil {
		return err
	}

	client, err := d.getNNGClient(addr)
	if err != nil {
		return err
	}

	callCtx, cancel := context.WithTimeout(ctx, d.invokeTimeout)
	defer cancel()

	// Marshal request
	req := &sdkv1.CancelJobRequest{JobId: jobID}
	reqBytes, err := proto.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	_, err = client.Call(callCtx, protocol.MsgCancelJobRequest, reqBytes)
	if err == nil {
		d.unregisterJob(jobID)
	}

	return err
}

func (d *Dispatcher) StreamJob(ctx context.Context, jobID string) ([]*sdkv1.JobEvent, bool, error) {
	// Note: Streaming with NNG would require Pair protocol
	// For now, return a simplified response
	return nil, false, fmt.Errorf("streaming not yet implemented for NNG")
}

// StreamJobRealtime forwards job events to the provided callback.
func (d *Dispatcher) StreamJobRealtime(ctx context.Context, jobID string, fn func(*sdkv1.JobEvent) bool) (bool, error) {
	// Note: Streaming with NNG would require Pair protocol
	// For now, return a simplified response
	return false, fmt.Errorf("streaming not yet implemented for NNG")
}

// ListFunctionAgents returns agent IDs that currently expose the function.
func (d *Dispatcher) ListFunctionAgents(functionID string) []string {
	now := time.Now()
	d.store.Mu().RLock()
	defer d.store.Mu().RUnlock()
	var ids []string
	for _, agent := range d.store.AgentsUnsafe() {
		if agent == nil || agent.AgentID == "" || agent.RPCAddr == "" {
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

func agentHasService(agent *reg.AgentSession, serviceID, functionID string) bool {
	if agent == nil || serviceID == "" {
		return false
	}
	for _, p := range agent.Processes {
		if p.ServiceID != serviceID {
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

// JobAddr exposes tracked job routing addresses (primarily for diagnostics).
func (d *Dispatcher) JobAddr(jobID string) (string, bool) {
	d.mu.RLock()
	addr, ok := d.jobRouting[jobID]
	d.mu.RUnlock()
	return addr, ok
}

// pickAgent returns a live agent that owns the function.
func (d *Dispatcher) pickAgent(functionID string) (*reg.AgentSession, error) {
	now := time.Now()
	d.store.Mu().RLock()
	defer d.store.Mu().RUnlock()

	var chosen *reg.AgentSession
	for _, agent := range d.store.AgentsUnsafe() {
		if agent == nil || agent.RPCAddr == "" {
			continue
		}
		if !agent.ExpireAt.After(now) {
			continue
		}
		meta, ok := agent.Functions[functionID]
		if !ok || !meta.Enabled {
			continue
		}
		if chosen == nil || agent.AgentID < chosen.AgentID {
			chosen = agent
		}
	}

	if chosen == nil {
		return nil, fmt.Errorf("no live agent for function %s", functionID)
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
			if agent == nil || agent.RPCAddr == "" || !agent.ExpireAt.After(now) {
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
			if agent == nil || agent.RPCAddr == "" || !agent.ExpireAt.After(now) {
				continue
			}
			meta, ok := agent.Functions[functionID]
			if !ok || !meta.Enabled {
				continue
			}
			cands = append(cands, agent)
		}
		chosen := pickAgentByHash(cands, hashKey)
		if chosen == nil {
			return nil, fmt.Errorf("no live agent for function %s", functionID)
		}
		return chosen, nil
	}

	return d.pickAgent(functionID)
}

// getNNGClient gets or creates an NNG client for the given address
func (d *Dispatcher) getNNGClient(addr string) (*nng.Client, error) {
	d.clientsMu.RLock()
	client, ok := d.nngClients[addr]
	d.clientsMu.RUnlock()

	if ok && client.IsRunning() {
		return client, nil
	}

	d.clientsMu.Lock()
	defer d.clientsMu.Unlock()

	// Double-check after acquiring write lock
	if client, ok := d.nngClients[addr]; ok && client.IsRunning() {
		return client, nil
	}

	// Create new client
	if addr == "" {
		return nil, fmt.Errorf("agent address missing")
	}

	// Build NNG address
	nngAddr := addr
	if !strings.Contains(nngAddr, ":") {
		nngAddr = net.JoinHostPort(nngAddr, "19091") // Default Agent NNG port
	}

	client = nng.NewClient(nngAddr)
	if err := client.Dial(); err != nil {
		return nil, fmt.Errorf("failed to dial NNG: %w", err)
	}

	d.nngClients[addr] = client
	return client, nil
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

// RegisterJob registers a job routing (exported method)
func (d *Dispatcher) RegisterJob(jobID, addr string) {
	d.registerJob(jobID, addr)
}

// UnregisterJob unregisters a job routing (exported method)
func (d *Dispatcher) UnregisterJob(jobID string) {
	d.unregisterJob(jobID)
}

func (d *Dispatcher) jobAddr(jobID string) (string, error) {
	d.mu.RLock()
	addr, ok := d.jobRouting[jobID]
	d.mu.RUnlock()
	if !ok {
		if d.jobStore != nil {
			routing, err := d.jobStore.Get(jobID)
			if err == nil && routing != nil && routing.AgentAddr != "" {
				d.mu.Lock()
				d.jobRouting[jobID] = routing.AgentAddr
				d.mu.Unlock()
				return routing.AgentAddr, nil
			}
		}
		return "", fmt.Errorf("job %s not tracked", jobID)
	}
	return addr, nil
}

func isTerminalEvent(evt *sdkv1.JobEvent) bool {
	switch strings.ToLower(evt.GetType()) {
	case "done", "completed", "error", "failed", "cancelled", "canceled", "succeeded", "success":
		return true
	default:
		return false
	}
}

// loadJobRouting loads job routing from persistent store
func (d *Dispatcher) loadJobRouting() {
	routings, err := d.jobStore.List()
	if err != nil {
		// Log error but continue with empty cache
		log.Printf("[dispatch] Warning: failed to load job routing from store: %v", err)
		return
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	// Rebuild in-memory cache (clear removed jobs too).
	d.jobRouting = make(map[string]string, len(routings))
	for _, routing := range routings {
		d.jobRouting[routing.JobID] = routing.AgentAddr
	}
}

// registerJob registers job routing to both memory cache and persistent store
func (d *Dispatcher) registerJob(jobID, addr string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Update memory cache
	d.jobRouting[jobID] = addr

	// Update persistent store
	if err := d.jobStore.Set(jobID, addr); err != nil {
		// Log error but continue
		log.Printf("[dispatch] Warning: failed to persist job routing: %v", err)
	}
}

// unregisterJob removes job routing from both memory cache and persistent store
func (d *Dispatcher) unregisterJob(jobID string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Remove from memory cache
	delete(d.jobRouting, jobID)

	// Remove from persistent store
	if err := d.jobStore.Delete(jobID); err != nil {
		// Log error but continue
		log.Printf("[dispatch] Warning: failed to delete job routing: %v", err)
	}
}

// CleanupOldJobs removes old job routing entries
func (d *Dispatcher) CleanupOldJobs(ttl time.Duration) error {
	// Cleanup persistent store
	if err := d.jobStore.Cleanup(ttl); err != nil {
		return err
	}

	// Reload cache to sync with persistent store
	d.loadJobRouting()

	return nil
}

// Close closes the dispatcher and its resources
func (d *Dispatcher) Close() error {
	// Close all NNG clients
	d.clientsMu.Lock()
	for _, client := range d.nngClients {
		client.Close()
	}
	d.nngClients = make(map[string]*nng.Client)
	d.clientsMu.Unlock()

	return d.jobStore.Close()
}
