package dispatch

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	apperrors "github.com/cuihairu/croupier/internal/errors"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/platform/tlsutil"
	"github.com/cuihairu/croupier/internal/telemetry"
	"github.com/cuihairu/croupier/internal/transport"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/protobuf/proto"
)

var ErrTaskRunNotFound = errors.New("task run not found")

func dispatchTracer() trace.Tracer {
	return otel.Tracer("croupier.dispatch")
}

// generateTaskID creates a server-side unique task identifier.
func generateTaskID() string {
	return "task-" + uuid.NewString()
}

// AgentSessionResolver finds active TCP sessions for connected Agents.
// The server package's AgentSessionStore implements this interface.
type AgentSessionResolver interface {
	ResolveAgentConn(agentID string) (transport.SessionCaller, bool)
}

// TaskEventRecord keeps the persistent event sequence alongside the wire event.
// The sequence is not part of the SDK TaskEvent protobuf, but the dispatcher
// needs it to advance polling cursors without replaying old rows.
type TaskEventRecord struct {
	Seq   int64
	Event *sdkv1.TaskEvent
}

type TaskRunState struct {
	TaskID        string
	Status        string
	Progress      int32
	Message       string
	ResultPayload []byte
	ErrorMessage  string
}

// TaskEventQuery queries task events from persistent storage.
type TaskEventQuery interface {
	ListEvents(ctx context.Context, taskID string, afterSeq int64) ([]TaskEventRecord, error)
	GetRun(ctx context.Context, taskID string) (*TaskRunState, error)
}

// TaskRunWriter persists a task run record when a task is dispatched. It
// closes the feedback loop: the server creates a row before dispatch, so
// events coming back from the agent (keyed by the same task ID) can update
// the correct row.
type TaskRunWriter interface {
	CreateRun(ctx context.Context, taskID, functionID, agentID, gameID, env, status string, inputPayload []byte) error
	CreateRunWithMeta(ctx context.Context, taskID, functionID, agentID, gameID, env, status, actor, addr, traceID string, inputPayload []byte) error
}

// TaskRoutingInfo is a minimal DTO for task routing records.

// Dispatcher routes function invocations to live agents discovered via registry store.
// Uses TCP session routing for all agent communication.
// Supports HA features: health tracking, circuit breaker, load balancing.
type Dispatcher struct {
	store          *reg.Store
	mu             sync.RWMutex
	taskRouting    map[string]string // taskID -> agentID (in-memory cache)
	taskStore      TaskRoutingStore  // persistent storage for task routing
	taskEventQuery TaskEventQuery    // task event query from persistent storage
	taskRunWriter  TaskRunWriter     // creates task_runs rows on dispatch
	dialTimeout    time.Duration
	invokeTimeout  time.Duration
	tlsCfg         *tlsutil.ClientTLSConfig

	// TCP session routing
	sessionResolver AgentSessionResolver

	// HA features
	healthTracker *HealthTracker
	loadBalancer  *LoadBalancer
	haEnabled     bool
}

func NewDispatcher(store *reg.Store) *Dispatcher {
	return NewDispatcherWithTaskStore(store, nil, nil)
}

// NewDispatcherWithTaskStore creates a new Dispatcher with optional task routing store and task event query
func NewDispatcherWithTaskStore(store *reg.Store, taskStore TaskRoutingStore, taskEventQuery TaskEventQuery) *Dispatcher {
	return NewDispatcherWithHA(store, taskStore, taskEventQuery, false, StrategyMinID, nil)
}

// NewDispatcherWithHA creates a new Dispatcher with HA features enabled
func NewDispatcherWithHA(store *reg.Store, taskStore TaskRoutingStore, taskEventQuery TaskEventQuery, haEnabled bool, strategy LoadBalanceStrategy, healthConfig *HealthCheckConfig) *Dispatcher {
	if store == nil {
		store = reg.NewStore()
	}

	// Default to memory store if none provided.
	if taskStore == nil {
		taskStore = NewMemoryTaskRoutingStore()
	}

	d := &Dispatcher{
		store:          store,
		taskRouting:    map[string]string{},
		taskStore:      taskStore,
		taskEventQuery: taskEventQuery,
		dialTimeout:    5 * time.Second,
		invokeTimeout:  15 * time.Second,
		haEnabled:      haEnabled,
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

// SetTaskRunWriter injects a TaskRunWriter so the Dispatcher can persist
// task_runs rows when dispatching tasks. When set, StartTaskRequest generates
// a server-side task ID, creates a row, and passes the ID to the agent.
func (d *Dispatcher) SetTaskRunWriter(w TaskRunWriter) {
	d.mu.Lock()
	d.taskRunWriter = w
	d.mu.Unlock()
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

// SetTaskEventQuery sets the task event query for persistent storage access.
func (d *Dispatcher) SetTaskEventQuery(query TaskEventQuery) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.taskEventQuery = query
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
	ctx = telemetry.ExtractContext(ctx, req.Metadata)
	ctx, span := dispatchTracer().Start(ctx, "function.dispatch.invoke",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(attribute.String("function.id", req.GetFunctionId())),
	)
	defer span.End()

	agent, err := d.pickAgentWithRouting(req.GetFunctionId(), req.Metadata)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}
	span.SetAttributes(attribute.String("agent.id", agent.AgentID))

	// Track connection start
	if d.healthTracker != nil {
		d.healthTracker.IncrementConnections(agent.AgentID)
		defer d.healthTracker.DecrementConnections(agent.AgentID)
	}

	if req.Metadata == nil {
		req.Metadata = map[string]string{}
	}
	req.Metadata["agent_id"] = agent.AgentID
	req.Metadata = telemetry.InjectContext(ctx, req.Metadata)

	reqBytes, err := proto.Marshal(req)
	if err != nil {
		if d.healthTracker != nil {
			d.healthTracker.RecordFailure(agent.AgentID)
		}
		err = fmt.Errorf("marshal request: %w", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	respBytes, err := d.callAgent(ctx, agent.AgentID, protocol.MsgInvokeRequest, reqBytes)
	if err != nil {
		if d.healthTracker != nil {
			d.healthTracker.RecordFailure(agent.AgentID)
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	resp := &sdkv1.InvokeResponse{}
	if err := proto.Unmarshal(respBytes, resp); err != nil {
		if d.healthTracker != nil {
			d.healthTracker.RecordFailure(agent.AgentID)
		}
		err = fmt.Errorf("unmarshal response: %w", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	if d.healthTracker != nil {
		d.healthTracker.RecordSuccess(agent.AgentID)
	}

	span.SetStatus(codes.Ok, "")
	return resp, nil
}

// BroadcastAgentResult captures the outcome of a single agent invocation
// within a broadcast. Either Response or Err is set.
type BroadcastAgentResult struct {
	AgentID  string
	Response *sdkv1.InvokeResponse
	Err      error
}

// BroadcastInvocation aggregates per-agent outcomes from InvokeBroadcast.
// Successes and Failures together always sum to Total.
type BroadcastInvocation struct {
	Total     int
	Successes []*BroadcastAgentResult
	Failures  []*BroadcastAgentResult
}

// InvokeBroadcast delivers the same request to every live agent that owns the
// function and returns the per-agent outcomes. Individual agent failures are
// captured in the result instead of aborting the whole broadcast, so callers
// can act on partial results.
func (d *Dispatcher) InvokeBroadcast(ctx context.Context, req *sdkv1.InvokeRequest) (*BroadcastInvocation, error) {
	if req == nil || req.GetFunctionId() == "" {
		return nil, fmt.Errorf("function id is required")
	}
	ctx = telemetry.ExtractContext(ctx, req.Metadata)
	ctx, span := dispatchTracer().Start(ctx, "function.dispatch.broadcast",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(attribute.String("function.id", req.GetFunctionId())),
	)
	defer span.End()

	gameID, env, scoped, err := routingScopeFromMetadata(req.Metadata)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}
	agents := d.listAgentsForFunctionInScope(req.GetFunctionId(), gameID, env, scoped)
	if len(agents) == 0 {
		err := noLiveAgentError(req.GetFunctionId(), gameID, env, scoped)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}
	span.SetAttributes(attribute.Int("agent.count", len(agents)))

	result := &BroadcastInvocation{Total: len(agents)}

	var (
		wg sync.WaitGroup
		mu sync.Mutex
	)

	for _, agent := range agents {
		wg.Add(1)
		go func(agent *reg.AgentSession) {
			defer wg.Done()

			// Per-agent clone so concurrent metadata writes don't race.
			localReq := proto.Clone(req).(*sdkv1.InvokeRequest)
			if localReq.Metadata == nil {
				localReq.Metadata = map[string]string{}
			}
			localReq.Metadata["agent_id"] = agent.AgentID
			localReq.Metadata = telemetry.InjectContext(ctx, localReq.Metadata)

			if d.healthTracker != nil {
				d.healthTracker.IncrementConnections(agent.AgentID)
				defer d.healthTracker.DecrementConnections(agent.AgentID)
			}

			out := &BroadcastAgentResult{AgentID: agent.AgentID}

			reqBytes, err := proto.Marshal(localReq)
			if err != nil {
				out.Err = fmt.Errorf("marshal request: %w", err)
			} else {
				respBytes, callErr := d.callAgent(ctx, agent.AgentID, protocol.MsgInvokeRequest, reqBytes)
				if callErr != nil {
					out.Err = callErr
				} else {
					resp := &sdkv1.InvokeResponse{}
					if err := proto.Unmarshal(respBytes, resp); err != nil {
						out.Err = fmt.Errorf("unmarshal response: %w", err)
					} else {
						out.Response = resp
					}
				}
			}

			mu.Lock()
			if out.Err != nil {
				result.Failures = append(result.Failures, out)
			} else {
				result.Successes = append(result.Successes, out)
			}
			mu.Unlock()

			if d.healthTracker != nil {
				if out.Err != nil {
					d.healthTracker.RecordFailure(agent.AgentID)
				} else {
					d.healthTracker.RecordSuccess(agent.AgentID)
				}
			}
		}(agent)
	}

	wg.Wait()
	span.SetAttributes(attribute.Int("function.broadcast.success", len(result.Successes)), attribute.Int("function.broadcast.failure", len(result.Failures)))
	if len(result.Failures) > 0 {
		span.SetStatus(codes.Error, "broadcast had failed agent calls")
	} else {
		span.SetStatus(codes.Ok, "")
	}
	return result, nil
}

// listAgentsForFunction returns every live agent that currently exposes the
// given function. It is kept for scope-neutral diagnostics and legacy callers.
func (d *Dispatcher) listAgentsForFunction(functionID string) []*reg.AgentSession {
	return d.listAgentsForFunctionInScope(functionID, "", "", false)
}

// listAgentsForFunctionInScope returns live agents that expose functionID in
// the requested game/environment. Scoped invocations must never be delivered
// to an Agent registered by another game or environment.
func (d *Dispatcher) listAgentsForFunctionInScope(functionID, gameID, env string, scoped bool) []*reg.AgentSession {
	now := time.Now()

	d.store.Mu().RLock()
	defer d.store.Mu().RUnlock()

	var out []*reg.AgentSession
	for _, agent := range d.store.AgentsUnsafe() {
		if !agentCanInvoke(agent, functionID, now, gameID, env, scoped) {
			continue
		}
		out = append(out, agent)
	}
	return out
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
	ctx = telemetry.ExtractContext(ctx, req.Metadata)
	ctx, span := dispatchTracer().Start(ctx, "function.dispatch.task",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(attribute.String("function.id", req.GetFunctionId())),
	)
	defer span.End()

	agent, err := d.pickAgentWithRouting(req.GetFunctionId(), req.Metadata)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}
	span.SetAttributes(attribute.String("agent.id", agent.AgentID))

	if d.healthTracker != nil {
		d.healthTracker.IncrementConnections(agent.AgentID)
		defer d.healthTracker.DecrementConnections(agent.AgentID)
	}

	if req.Metadata == nil {
		req.Metadata = map[string]string{}
	}
	req.Metadata["agent_id"] = agent.AgentID
	req.Metadata = telemetry.InjectContext(ctx, req.Metadata)

	// Generate a server-side task ID so events flowing back from the agent
	// can be matched to a task_runs row. This closes the feedback loop.
	d.mu.RLock()
	writer := d.taskRunWriter
	d.mu.RUnlock()

	if writer != nil {
		taskID := generateTaskID()
		req.Metadata["task_id"] = taskID
		span.SetAttributes(attribute.String("task.id", taskID))
		gameID := req.Metadata["game_id"]
		env := req.Metadata["env"]
		actor := req.Metadata["actor"]
		addr := agent.Addr
		traceID := telemetry.TraceIDFromMetadata(req.Metadata)
		// Best-effort: create the run row. If this fails the task still
		// dispatches — the agent will use the provided task_id and events
		// will be orphaned but not lost (they land in task_events).
		_ = writer.CreateRunWithMeta(ctx, taskID, req.GetFunctionId(), agent.AgentID, gameID, env, "dispatching", actor, addr, traceID, req.GetPayload())
	}

	reqBytes, err := proto.Marshal(req)
	if err != nil {
		if d.healthTracker != nil {
			d.healthTracker.RecordFailure(agent.AgentID)
		}
		err = fmt.Errorf("marshal request: %w", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	respBytes, err := d.callAgent(ctx, agent.AgentID, protocol.MsgStartTaskRequest, reqBytes)
	if err != nil {
		if d.healthTracker != nil {
			d.healthTracker.RecordFailure(agent.AgentID)
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	resp := &sdkv1.StartTaskResponse{}
	if err := proto.Unmarshal(respBytes, resp); err != nil {
		if d.healthTracker != nil {
			d.healthTracker.RecordFailure(agent.AgentID)
		}
		err = fmt.Errorf("unmarshal response: %w", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	if d.healthTracker != nil {
		d.healthTracker.RecordSuccess(agent.AgentID)
	}

	if taskID := resp.GetTaskId(); taskID != "" {
		span.SetAttributes(attribute.String("task.id", taskID))
		d.registerTask(taskID, agent.AgentID)
	}

	span.SetStatus(codes.Ok, "")
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
	return d.StreamTaskAfterSeq(ctx, taskID, 0)
}

// StreamTaskAfterSeq streams task events after a given sequence number.
// Returns events, whether the task is done, and any error.
func (d *Dispatcher) StreamTaskAfterSeq(ctx context.Context, taskID string, afterSeq int64) ([]*sdkv1.TaskEvent, bool, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil, false, fmt.Errorf("task id is required")
	}

	d.mu.RLock()
	query := d.taskEventQuery
	d.mu.RUnlock()

	if query == nil {
		return nil, false, fmt.Errorf("task event query not configured")
	}

	records, err := query.ListEvents(ctx, taskID, afterSeq)
	if err != nil {
		return nil, false, fmt.Errorf("query events: %w", err)
	}
	events := taskEventsFromRecords(records)

	run, err := query.GetRun(ctx, taskID)
	if err != nil {
		if !errors.Is(err, ErrTaskRunNotFound) {
			return nil, false, fmt.Errorf("query task run: %w", err)
		}
		return events, false, nil
	}

	done := isTaskRunDone(run.Status)
	return events, done, nil
}

// StreamTaskRealtime forwards task events to the provided callback.
func (d *Dispatcher) StreamTaskRealtime(ctx context.Context, taskID string, fn func(*sdkv1.TaskEvent) bool) (bool, error) {
	return d.StreamTaskRealtimeAfterSeq(ctx, taskID, 0, fn)
}

// StreamTaskRealtimeAfterSeq streams task events to the callback after a given sequence number.
// Returns whether the task is done and any error.
func (d *Dispatcher) StreamTaskRealtimeAfterSeq(ctx context.Context, taskID string, afterSeq int64, fn func(*sdkv1.TaskEvent) bool) (bool, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return false, fmt.Errorf("task id is required")
	}

	d.mu.RLock()
	query := d.taskEventQuery
	d.mu.RUnlock()

	if query == nil {
		return false, fmt.Errorf("task event query not configured")
	}

	for {
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		default:
		}

		records, err := query.ListEvents(ctx, taskID, afterSeq)
		if err != nil {
			return false, fmt.Errorf("query events: %w", err)
		}

		done := false
		run, err := query.GetRun(ctx, taskID)
		if err != nil {
			if !errors.Is(err, ErrTaskRunNotFound) {
				return false, fmt.Errorf("query task run: %w", err)
			}
		} else {
			done = isTaskRunDone(run.Status)
		}

		for _, record := range records {
			if record.Seq > afterSeq {
				afterSeq = record.Seq
			}
			evt := record.Event
			if evt == nil {
				continue
			}
			if !fn(evt) {
				return done, nil // Callback stopped streaming
			}
		}

		if done {
			return done, nil
		}

		// Wait before next poll
		time.Sleep(taskStreamPollInterval)
	}
}

var taskStreamPollInterval = 500 * time.Millisecond

func taskEventsFromRecords(records []TaskEventRecord) []*sdkv1.TaskEvent {
	events := make([]*sdkv1.TaskEvent, 0, len(records))
	for _, record := range records {
		if record.Event != nil {
			events = append(events, record.Event)
		}
	}
	return events
}

// isTaskRunDone checks if a task run status indicates completion.
func isTaskRunDone(status string) bool {
	switch strings.ToLower(status) {
	case "succeeded", "success", "done", "completed":
		return true
	case "failed", "error":
		return true
	case "cancelled", "canceled":
		return true
	case "timed_out", "timeout":
		return true
	default:
		return false
	}
}

// isTaskEventTypeDone checks if a task event type indicates completion.
func isTaskEventTypeDone(eventType string) bool {
	switch strings.ToLower(eventType) {
	case "completed", "success", "succeeded":
		return true
	case "failed", "error":
		return true
	case "cancelled", "canceled":
		return true
	default:
		return false
	}
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

// pickAgent returns a live agent that owns the function without a scope
// constraint. Scoped HTTP execution always uses pickAgentWithRouting.
func (d *Dispatcher) pickAgent(functionID string) (*reg.AgentSession, error) {
	return d.pickAgentInScope(functionID, "", "", false)
}

func (d *Dispatcher) pickAgentInScope(functionID, gameID, env string, scoped bool) (*reg.AgentSession, error) {
	candidates := d.listAgentsForFunctionInScope(functionID, gameID, env, scoped)
	return d.selectAgent(functionID, candidates, gameID, env, scoped)
}

func (d *Dispatcher) selectAgent(functionID string, candidates []*reg.AgentSession, gameID, env string, scoped bool) (*reg.AgentSession, error) {
	if len(candidates) == 0 {
		return nil, noLiveAgentError(functionID, gameID, env, scoped)
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
			return nil, noHealthyAgentError(functionID, gameID, env, scoped)
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
	gameID, env, scoped, err := routingScopeFromMetadata(metadata)
	if err != nil {
		return nil, err
	}

	serviceID := strings.TrimSpace(metadata["target_service_id"])
	hashKey := strings.TrimSpace(metadata["hash_key"])
	candidates := d.listAgentsForFunctionInScope(functionID, gameID, env, scoped)
	if len(candidates) == 0 {
		return nil, noLiveAgentError(functionID, gameID, env, scoped)
	}

	// Targeted: choose the agent that owns the service_id.
	if serviceID != "" {
		var chosen *reg.AgentSession
		for _, agent := range candidates {
			if agentHasService(agent, serviceID, functionID) {
				if chosen == nil || agent.AgentID < chosen.AgentID {
					chosen = agent
				}
			}
		}
		if chosen != nil {
			return chosen, nil
		}
		return nil, apperrors.Newf(apperrors.ErrCodeServiceUnavailable, "select_agent", nil,
			"no live agent for function %s%s with service_id %s", functionID, formatRoutingScope(gameID, env, scoped), serviceID)
	}

	// Hash: choose a stable agent among the already scope-filtered candidates.
	if hashKey != "" {
		sort.Slice(candidates, func(i, j int) bool {
			return candidates[i].AgentID < candidates[j].AgentID
		})
		chosen := pickAgentByHash(candidates, hashKey)
		if chosen == nil {
			return nil, noLiveAgentError(functionID, gameID, env, scoped)
		}
		return chosen, nil
	}

	return d.selectAgent(functionID, candidates, gameID, env, scoped)
}

func routingScopeFromMetadata(metadata map[string]string) (gameID, env string, scoped bool, err error) {
	if len(metadata) == 0 {
		return "", "", false, nil
	}
	gameID = strings.TrimSpace(metadata["game_id"])
	env = strings.TrimSpace(metadata["env"])
	if gameID == "" && env == "" {
		return "", "", false, nil
	}
	if gameID == "" || env == "" {
		return "", "", false, fmt.Errorf("game_id and env must be supplied together for agent routing")
	}
	return gameID, env, true, nil
}

func agentCanInvoke(agent *reg.AgentSession, functionID string, now time.Time, gameID, env string, scoped bool) bool {
	if agent == nil || !agent.ExpireAt.After(now) {
		return false
	}
	if scoped && (strings.TrimSpace(agent.GameID) != gameID || strings.TrimSpace(agent.Env) != env) {
		return false
	}
	meta, ok := agent.Functions[functionID]
	return ok && meta.Enabled
}

func formatRoutingScope(gameID, env string, scoped bool) string {
	if !scoped {
		return ""
	}
	return fmt.Sprintf(" in game_id %s env %s", gameID, env)
}

func noLiveAgentError(functionID, gameID, env string, scoped bool) error {
	return apperrors.Newf(apperrors.ErrCodeServiceUnavailable, "invoke", nil,
		"no live agent for function %s%s", functionID, formatRoutingScope(gameID, env, scoped))
}

func noHealthyAgentError(functionID, gameID, env string, scoped bool) error {
	return apperrors.Newf(apperrors.ErrCodeServiceUnavailable, "invoke", nil,
		"no healthy agents available for function %s%s", functionID, formatRoutingScope(gameID, env, scoped))
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
