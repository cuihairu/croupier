package dispatch

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/platform/tlsutil"
	functionv1 "github.com/cuihairu/croupier/pkg/pb/croupier/function/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Dispatcher routes function invocations to live agents discovered via registry store.
type Dispatcher struct {
	store         *reg.Store
	mu            sync.RWMutex
	jobRouting    map[string]string // jobID -> agent rpc addr (in-memory cache)
	jobStore      JobRoutingStore   // persistent storage for job routing
	dialTimeout   time.Duration
	invokeTimeout time.Duration
	// TLS configuration
	tlsEnabled bool
	certFile   string
	keyFile    string
	caFile     string
	serverName string
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
		dialTimeout:   5 * time.Second,
		invokeTimeout: 15 * time.Second,
	}

	// Read TLS configuration from environment
	d.tlsEnabled = os.Getenv("CROUPIER_AGENT_TLS_ENABLED") == "true"
	d.certFile = os.Getenv("CROUPIER_CLIENT_CERT_FILE")
	d.keyFile = os.Getenv("CROUPIER_CLIENT_KEY_FILE")
	d.caFile = os.Getenv("CROUPIER_CA_FILE")
	d.serverName = os.Getenv("CROUPIER_SERVER_NAME")

	// Load existing job routing from persistent store
	d.loadJobRouting()

	return d
}

func (d *Dispatcher) Store() *reg.Store {
	return d.store
}

func (d *Dispatcher) Invoke(ctx context.Context, functionID string, payload []byte) ([]byte, error) {
	resp, err := d.InvokeRequest(ctx, &functionv1.InvokeRequest{
		FunctionId: functionID,
		Payload:    payload,
	})
	if err != nil {
		return nil, err
	}
	return resp.GetPayload(), nil
}

// InvokeRequest forwards a fully populated InvokeRequest to a live agent.
func (d *Dispatcher) InvokeRequest(ctx context.Context, req *functionv1.InvokeRequest) (*functionv1.InvokeResponse, error) {
	if req == nil || req.GetFunctionId() == "" {
		return nil, fmt.Errorf("function id is required")
	}
	agent, err := d.pickAgent(req.GetFunctionId())
	if err != nil {
		return nil, err
	}
	conn, client, err := d.dial(agent.RPCAddr)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	callCtx, cancel := context.WithTimeout(ctx, d.invokeTimeout)
	defer cancel()

	if req.Metadata == nil {
		req.Metadata = map[string]string{}
	}
	req.Metadata["agent_id"] = agent.AgentID

	return client.Invoke(callCtx, req)
}

func (d *Dispatcher) StartJob(ctx context.Context, functionID string, payload []byte) (string, error) {
	resp, err := d.StartJobRequest(ctx, &functionv1.InvokeRequest{
		FunctionId: functionID,
		Payload:    payload,
	})
	if err != nil {
		return "", err
	}
	return resp.GetJobId(), nil
}

// StartJobRequest forwards a structured InvokeRequest to the agent StartJob RPC.
func (d *Dispatcher) StartJobRequest(ctx context.Context, req *functionv1.InvokeRequest) (*functionv1.StartJobResponse, error) {
	if req == nil || req.GetFunctionId() == "" {
		return nil, fmt.Errorf("function id is required")
	}
	agent, err := d.pickAgent(req.GetFunctionId())
	if err != nil {
		return nil, err
	}
	conn, client, err := d.dial(agent.RPCAddr)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	callCtx, cancel := context.WithTimeout(ctx, d.invokeTimeout)
	defer cancel()

	if req.Metadata == nil {
		req.Metadata = map[string]string{}
	}
	req.Metadata["agent_id"] = agent.AgentID

	resp, err := client.StartJob(callCtx, req)
	if err != nil {
		return nil, err
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
	conn, client, err := d.dial(addr)
	if err != nil {
		return err
	}
	defer conn.Close()

	callCtx, cancel := context.WithTimeout(ctx, d.invokeTimeout)
	defer cancel()
	_, err = client.CancelJob(callCtx, &functionv1.CancelJobRequest{JobId: jobID})
	if err == nil {
		d.unregisterJob(jobID)
	}
	return err
}

func (d *Dispatcher) StreamJob(ctx context.Context, jobID string) ([]*functionv1.JobEvent, bool, error) {
	var events []*functionv1.JobEvent
	done, err := d.StreamJobRealtime(ctx, jobID, func(evt *functionv1.JobEvent) bool {
		events = append(events, evt)
		return true
	})
	return events, done, err
}

// StreamJobRealtime forwards job events to the provided callback.
func (d *Dispatcher) StreamJobRealtime(ctx context.Context, jobID string, fn func(*functionv1.JobEvent) bool) (bool, error) {
	if jobID == "" {
		return false, fmt.Errorf("job id is required")
	}
	addr, err := d.jobAddr(jobID)
	if err != nil {
		return false, err
	}

	conn, client, err := d.dial(addr)
	if err != nil {
		return false, err
	}
	defer conn.Close()

	stream, err := client.StreamJob(ctx, &functionv1.JobStreamRequest{JobId: jobID})
	if err != nil {
		return false, err
	}

	done := false
	for {
		ev, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return done, err
		}
		if fn != nil {
			if cont := fn(ev); !cont {
				return done, nil
			}
		}
		if isTerminalEvent(ev) {
			done = true
			d.unregisterJob(jobID)
			break
		}
	}
	return done, nil
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

func (d *Dispatcher) dial(addr string) (*grpc.ClientConn, functionv1.FunctionServiceClient, error) {
	if addr == "" {
		return nil, nil, fmt.Errorf("agent rpc address missing")
	}

	ctx, cancel := context.WithTimeout(context.Background(), d.dialTimeout)
	defer cancel()

	var dialOpts []grpc.DialOption
	if d.tlsEnabled {
		// Use TLS
		creds, err := tlsutil.ClientTLS(d.certFile, d.keyFile, d.caFile, d.serverName)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create TLS credentials: %w", err)
		}
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(creds))
	} else {
		// Use insecure connection
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	conn, err := grpc.DialContext(ctx, addr, dialOpts...)
	if err != nil {
		return nil, nil, err
	}
	return conn, functionv1.NewFunctionServiceClient(conn), nil
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
		return "", fmt.Errorf("job %s not tracked", jobID)
	}
	return addr, nil
}

func isTerminalEvent(evt *functionv1.JobEvent) bool {
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

	// Build in-memory cache
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
	return d.jobStore.Close()
}
