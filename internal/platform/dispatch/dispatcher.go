package dispatch

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/cuihairu/croupier/core/transport"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/platform/tlsutil"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
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
	// MarkRunFailed 把 failover 失败尝试的行标记为 failed：该轮投递未到
	// agent，不会有事件回流推进状态，不标记会永远停在 dispatching。
	MarkRunFailed(ctx context.Context, taskID, errMsg string) error
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

	// remoteForwarder 把本地无连接的 agent 调用转发到持有其 session 的
	// 实例（HA 多实例 owner 转发）；单实例为 nil，本地 miss 即报错。
	remoteForwarder RemoteForwarder

	// remoteAgentSource 提供归属表视角的远端候选 agent（对端实例持有）。
	// 候选集扩展用：本地候选为空时同步查询，选中后经 remoteForwarder
	// 转发执行。单实例为 nil，路由行为与本地 registry 完全一致。
	remoteAgentSource RemoteAgentSource

	// taskAgentLookup 从共享 task_runs 兜底解析 task → agent。task
	// routing 是 per-instance 内存 map，取消请求落到非发起实例时内存
	// miss，由此查共享库（task_runs 行由发起实例 dispatch 时写入）。
	taskAgentLookup TaskAgentLookup

	// HA features
	healthTracker *HealthTracker
	loadBalancer  *LoadBalancer
	haEnabled     bool
}

// RemoteCall 描述一次经转发执行的 agent 调用。MsgID 是 agent 侧消息类型
// （invoke/start_task/cancel_task），kind 映射由 mesh 层据此翻译——dispatch
// 层不依赖 cluster 包。返回字节是 agent 响应原样（proto 编码）。
type RemoteCall struct {
	MsgID          uint32
	AgentID        string
	FunctionID     string
	Payload        []byte
	Metadata       map[string]string
	IdempotencyKey string
	// TaskID 仅 cancel_task 使用（目标任务的 ID）。
	TaskID string
}

// RemoteForwarder forwards an invoke for an agent whose session lives on
// another server instance. The cluster mesh wiring implements this; the
// dispatcher treats a non-nil implementation as "forward on local miss".
type RemoteForwarder interface {
	Forward(ctx context.Context, call *RemoteCall) ([]byte, error)
}

// RemoteAgentSource 供应归属表视角的远端候选 agent：归属表 TTL 内活跃、
// 由对端实例持有（不含本实例自有 agent）的会话快照，scope 匹配已滤。
// 契约边界：函数注册/enabled 的判断由 dispatcher 统一做（与本地候选
// 同一 agentCanInvoke），source 只负责「哪些远端 agent 活着」。
// 实现方（cmd/server ownerAgentSource）读共享归属表 + agent_sessions
// 快照表；出错时 dispatcher 降级回 noLiveAgent 语义，不放大故障。
type RemoteAgentSource interface {
	RemoteAgentSessions(ctx context.Context, gameID, env string, scoped bool) ([]*reg.AgentSession, error)
}

// TaskAgentLookup 从共享 task_runs 行解析任务所在 agent（跨实例取消兜底）。
// task routing（内存 map + 本地 taskStore）都是 per-instance 的；取消请求
// 落到非发起实例时两者都 miss，唯一可靠来源是共享库的 task_runs 行
// （发起实例 dispatch 时写入 AgentID）。返回错误或不存在的任务由调用方
// 报 "task not tracked"。
type TaskAgentLookup interface {
	AgentForTask(ctx context.Context, taskID string) (string, error)
}

// remoteSourceBudget 限制候选集远端查询的同步开销：归属表 + 快照表两次
// 共享库查询，超预算即放弃（回落 no live agent），避免热路径被 DB 拖垮。
const remoteSourceBudget = time.Second

// errAgentUnreachable 标记「该 agent 当前不可达」（本地无 session 且
// 转发失败/无路由）——可换下一候选重试的错误类。
var errAgentUnreachable = errors.New("agent unreachable")

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

// SessionResolver returns the configured TCP session resolver, or nil.
func (d *Dispatcher) SessionResolver() AgentSessionResolver {
	return d.sessionResolver
}

// SetRemoteForwarder wires the HA owner-forward path: local session miss
// falls back to forwarding the invoke to the instance owning the agent.
func (d *Dispatcher) SetRemoteForwarder(f RemoteForwarder) {
	d.remoteForwarder = f
}

// SetRemoteAgentSource wires the remote candidate source (cluster mode only).
func (d *Dispatcher) SetRemoteAgentSource(s RemoteAgentSource) {
	d.remoteAgentSource = s
}

// SetTaskAgentLookup wires the shared task_runs lookup used to resolve a
// task's agent when local routing misses (cross-instance cancel).
func (d *Dispatcher) SetTaskAgentLookup(l TaskAgentLookup) {
	d.taskAgentLookup = l
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
