// Package server provides the Server-side control plane for Croupier.
package server

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"sync"
	"time"

	transportcore "github.com/cuihairu/croupier/core/transport"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/devopssessions"
	"github.com/cuihairu/croupier/internal/platform/outlet"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/tasks"
	agentv1 "github.com/cuihairu/croupier/pkg/pb/croupier/agent/v1"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	"github.com/getkin/kin-openapi/openapi3"
	"google.golang.org/protobuf/proto"
	"gorm.io/gorm"
)

var (
	functionIDPattern = regexp.MustCompile(`^[a-z0-9]+(?:[._-][a-z0-9]+)*(?:\.[a-z0-9]+(?:[._-][a-z0-9]+)*)+$`)
)

// AgentSessionLoader defines the interface for loading and managing agent sessions from a database.
type AgentSessionLoader interface {
	LoadActiveSessions(ctx context.Context) ([]*reg.AgentSession, error)
	Upsert(ctx context.Context, sess *reg.AgentSession) error
	DeleteExpired(ctx context.Context) (int64, error)
}

// TaskStore defines the interface for task run persistence used by handleTaskEvent.
// RecordTaskEvent 在单一事务内完成带 guard 的状态更新与事件追加（R30-03 /
// ARCH-015）：保证 task_runs 与 task_events 持久一致，事件写失败整体回滚，
// 不再产生「状态已终态但事件缺失」的部分写入（旧两段式写入的缺口）。
type TaskStore interface {
	RecordTaskEvent(ctx context.Context, taskID string, blockedStatuses []string, updates map[string]interface{}, eventType tasks.EventType, progress int32, message string, payload []byte) (bool, error)
}

// TaskReaper defines the D3 watchdog reaper entry (*tasks.Store satisfies it
// structurally; nil = not wired, so the watchdog loop is a no-op). It sweeps
// active task_runs rows with no event past a budget to a target status
// (ARCH-018: timed_out).
type TaskReaper interface {
	ReapStale(ctx context.Context, budget time.Duration, status, message string) (int64, error)
}

// Handler handles control service requests.
type Handler interface {
	HandleRegister(ctx context.Context, req *agentv1.RegisterRequest) (*agentv1.RegisterResponse, error)
	HandleHeartbeat(ctx context.Context, req *agentv1.HeartbeatRequest) (*agentv1.HeartbeatResponse, error)
	HandleRegisterCapabilities(ctx context.Context, req *agentv1.RegisterCapabilitiesRequest) (*agentv1.RegisterCapabilitiesResponse, error)
}

// ControlService implements the control-plane business logic (register, heartbeat, capabilities).
// It is transport-agnostic — TCPListener delegates inbound frames to this service.
type ControlService struct {
	registry           *reg.Store
	agentSessionLoader AgentSessionLoader

	defaultSessionTTL time.Duration
	metricsStore      *reg.MetricsStore
	logStore          *reg.LogStore
	systemInfoCache   *reg.SystemInfoCache
	taskStore         TaskStore
	taskReaper        TaskReaper
	watchdogBudget    time.Duration
	watchdogInterval  time.Duration
	alertModel        *model.AlertModel
	probeEventModel   *model.ProbeEventModel
	// probeEvents 探测事件诊断环（§13.1 R2 接收侧：明细留证 + 窗口记日志）。
	probeEvents *probeEventSink
	// lifecycle 探测窗口→Alert 生命周期消费者（§12 R3）：probe.window 事件
	// 开/合转换 firing/resolved。nil = 未装配（仅诊断环留证）。
	lifecycle     *probeLifecycle
	outletManager *outlet.Manager
	configModel   *model.ConfigVersionModel
	// dbSourceModel DB 源模型（meta 库 db_sources；R4 归属派生 db 探针目标）。
	dbSourceModel *model.DBSourceModel
	gameDBs       GameDBResolver

	upstream Handler

	// schemaDiffWarn 注册时 schema 兼容性检查开关（默认开）：破坏性
	// schema 变更写入注册警告并随 RegisterResponse.warnings 返回 agent，
	// 不阻断注册。
	schemaDiffWarn bool

	// sdkVersionMinimums 按语言配置的最低可注册 SDK 版本（来自
	// registry.sdkVersionMinimums，键已归一小写）：自报版本低于配置值
	// 的 provider 独占声明函数不注册（只产生告警）。绝对下限，优先于
	// 滑动高水位门槛；nil/空表示未配置。
	sdkVersionMinimums map[string]string

	// upsertOpenAPI 是注册链路 UpsertOpenAPI 的注入点：生产为 nil（走
	// registry.UpsertOpenAPI 真实实现），测试注入失败以驱动注册警告分支
	//（registry 为具体类型 *reg.Store、无接口 seam，该 Warn 分支的生产
	// 不可达论证见 handleRegisterRequest 内注释）。
	upsertOpenAPI func(functionID string, op *openapi3.Operation) error

	// clusterHooks 集群归属钩子（多实例 HA；nil = 未启用，no-op）。
	clusterHooks interface {
		OnAgentRegistered(ctx context.Context, agentID, gameID, env string, labels map[string]string)
		OnAgentHeartbeat(ctx context.Context, agentID string)
		OnAgentDisconnected(ctx context.Context, agentID string)
	}

	// sessionPersistMu/sessionPersistAt 心跳 DB 落盘节流：距上次落盘
	// 不足 sessionPersistInterval 的心跳跳过异步 Upsert——全量 JSON
	// 会话每 30s 写一次既浪费也放大 sqlite/小实例的写锁竞争
	//（real-dashboard fixture 曾因与注册事务互锁挂死）。
	sessionPersistMu       sync.Mutex
	sessionPersistAt       map[string]time.Time
	sessionPersistInterval time.Duration

	// heartbeatOwnerLookup 心跳自愈用：本地会话丢失时从共享归属表回读
	// 本实例持有的 agent 真实 scope 重建会话（装配期注入；nil = 无自愈）。
	heartbeatOwnerLookup interface {
		SelfOwnerScope(ctx context.Context, agentID string) (gameID, env string, ok bool)
	}

	// activeAgentDirectory 启动恢复过滤用：共享归属表的活跃 agent 全集
	// （任一实例持有连接即活跃；装配期注入；nil = 单实例，全量恢复不变）。
	activeAgentDirectory interface {
		ActiveAgentIDs(ctx context.Context) ([]string, error)
	}

	// clusterInstanceID 本实例的集群身份（注册响应回传给 agent 做三方
	// 对账；单实例/未启用时为空）。
	clusterInstanceID string

	// devopsSessions devops 会话中继运营面（AGENT-002）：agent 上行的
	// DevopsSessionEvent 经此扇出给面板 SSE 消费者。nil = 未装配，事件
	// 静默丢弃（事件面不是审计真值，丢失不影响 agent 侧存档）。
	devopsSessions *devopssessions.Hub

	mu     sync.RWMutex
	ctx    context.Context
	cancel context.CancelFunc
	logger *slog.Logger

	backgroundOnce sync.Once
}

// NewControlService creates a new ControlService.
func NewControlService(registry *reg.Store, loader AgentSessionLoader) *ControlService {
	if registry == nil {
		registry = reg.NewStore()
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &ControlService{
		registry:           registry,
		agentSessionLoader: loader,
		metricsStore:       reg.NewMetricsStore(),
		logStore:           reg.NewLogStore(),
		systemInfoCache:    reg.NewSystemInfoCache(),
		probeEvents:        newProbeEventSink(),
		defaultSessionTTL:  5 * time.Minute,
		schemaDiffWarn:     true,
		ctx:                ctx,
		cancel:             cancel,
		logger:             slog.Default(),
	}
}

// SetSchemaDiffWarnEnabled 开关注册时 schema 兼容性告警（默认开启）。
func (s *ControlService) SetSchemaDiffWarnEnabled(enabled bool) {
	s.schemaDiffWarn = enabled
}

// SetSDKVersionMinimums 注入按语言配置的最低可注册 SDK 版本
// （registry.sdkVersionMinimums）。语言键归一小写，空版本值的条目
// 丢弃；传 nil/空表即未配置（仅滑动高水位门槛生效）。
func (s *ControlService) SetSDKVersionMinimums(minimums map[string]string) {
	if len(minimums) == 0 {
		return
	}
	normalized := make(map[string]string, len(minimums))
	for language, minimum := range minimums {
		key := strings.ToLower(strings.TrimSpace(language))
		value := strings.TrimSpace(minimum)
		if key == "" || value == "" {
			continue
		}
		normalized[key] = value
	}
	if len(normalized) > 0 {
		s.sdkVersionMinimums = normalized
	}
}

func (s *ControlService) SetTaskStore(store TaskStore) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.taskStore = store
}

// SetTaskReaper 注入 D3 看门狗收割器与无事件预算（budget 决定收割 tick
// 间隔）。必须在 StartBackgroundTasks 之前调用，否则后台循环不启动。
func (s *ControlService) SetTaskReaper(reaper TaskReaper, budget time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.taskReaper = reaper
	s.watchdogBudget = budget
	s.watchdogInterval = watchdogIntervalFor(budget)
}

// SetAlertModel sets the alert model for capture rule-hit persistence
// (nil = 未配置，告警上行返回错误，同 task store 语义).
func (s *ControlService) SetAlertModel(m *model.AlertModel) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.alertModel = m
}

// SetProbeEventModel configures durable probe uplink storage.
func (s *ControlService) SetProbeEventModel(m *model.ProbeEventModel) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.probeEventModel = m
}

// SetOutletManager sets the alert outlet manager for capture rule-hit
// dispatch (nil = 出口未配置，只落库不分发).
func (s *ControlService) SetOutletManager(m *outlet.Manager) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.outletManager = m
}

// GameDBResolver multiGame 分库模式下按 (gameID, env) 解析库并把覆盖
// 注入 ctx（dbctx）。结构性满足 *router.Router——control 包不引 internal/db。
type GameDBResolver interface {
	Resolve(ctx context.Context, gameID, env string) (context.Context, *gorm.DB, error)
}

// SetConfigVersionModel sets the config-version model for capture gate-rule
// pull (nil = 未配置，规则下行返回错误).
func (s *ControlService) SetConfigVersionModel(m *model.ConfigVersionModel) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.configModel = m
}

// SetDBSourceModel sets the DB source model for probe db-target derivation
// (R4: 周期 db 探测按 owner_agent_id 归属下发给 agent；nil = 未配置，目标
// 下行返回错误，agent 保持现有目标).
func (s *ControlService) SetDBSourceModel(m *model.DBSourceModel) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dbSourceModel = m
}

// SetGameDBResolver 注入 game 分库解析器（multiGame 模式必配；nil = 单库，
// FindLatestByScope 走默认连接）.
func (s *ControlService) SetGameDBResolver(r GameDBResolver) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.gameDBs = r
}

// Store returns the registry store.
func (s *ControlService) Store() *reg.Store { return s.registry }

// MetricsStore returns the metrics store.
func (s *ControlService) MetricsStore() *reg.MetricsStore { return s.metricsStore }

// LogStore returns the OPS-005 application log store.
func (s *ControlService) LogStore() *reg.LogStore { return s.logStore }

// SetMetricsDB sets the database for metrics persistence.
func (s *ControlService) SetMetricsDB(db *gorm.DB) {
	if s.metricsStore != nil {
		s.metricsStore.SetDB(db)
		s.metricsStore.StartCleanupRoutine(s.ctx, 1*time.Hour)
	}
	if s.logStore != nil {
		s.logStore.SetDB(db)
		s.logStore.StartCleanupRoutine(s.ctx, 1*time.Hour)
	}
}

// SystemInfoCache returns the system info cache.
func (s *ControlService) SystemInfoCache() *reg.SystemInfoCache { return s.systemInfoCache }

// SetDefaultSessionTTL sets the default session TTL.
func (s *ControlService) SetDefaultSessionTTL(ttl time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.defaultSessionTTL = ttl
}

// SetUpstreamHandler sets an upstream handler for forwarding requests.
func (s *ControlService) SetUpstreamHandler(h Handler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.upstream = h
}

// SetLogger sets the logger.
func (s *ControlService) SetLogger(logger *slog.Logger) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.logger = logger
}

// TransportHandler exposes the control-plane request handler for TCP transport.
func (s *ControlService) TransportHandler() transportcore.Handler {
	return transportcore.HandlerFunc(func(ctx context.Context, msgID uint32, reqID uint32, body []byte) ([]byte, error) {
		if identity := AgentIdentityFromContext(ctx); identity != nil {
			return s.handleRequestAsAgent(ctx, msgID, body, identity)
		}
		return s.handleRequest(ctx, msgID, body)
	})
}

// StartBackgroundTasks starts background maintenance loops (DB loading, metrics pruning, etc.).
// SetClusterHooks 注入集群归属钩子（幂等，可在装配期任意时刻调用）。
func (s *ControlService) SetClusterHooks(h interface {
	OnAgentRegistered(ctx context.Context, agentID, gameID, env string, labels map[string]string)
	OnAgentHeartbeat(ctx context.Context, agentID string)
	OnAgentDisconnected(ctx context.Context, agentID string)
}) {
	s.mu.Lock()
	s.clusterHooks = h
	s.mu.Unlock()
}

// SetClusterInstanceID 注入本实例集群身份（注册响应回传 agent 三方对账）。
func (s *ControlService) SetClusterInstanceID(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clusterInstanceID = id
}

// SetDevopsSessions 注入 devops 会话中继运营面（AGENT-002；nil=未装配）。
func (s *ControlService) SetDevopsSessions(h *devopssessions.Hub) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.devopsSessions = h
}

// SetHeartbeatOwnerLookup 注入共享归属表回读能力（心跳自愈用）。
func (s *ControlService) SetHeartbeatOwnerLookup(l interface {
	SelfOwnerScope(ctx context.Context, agentID string) (gameID, env string, ok bool)
}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.heartbeatOwnerLookup = l
}

// SetActiveAgentDirectory 注入共享归属表活跃全集查询（启动恢复过滤用）。
func (s *ControlService) SetActiveAgentDirectory(d interface {
	ActiveAgentIDs(ctx context.Context) ([]string, error)
}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.activeAgentDirectory = d
}

func (s *ControlService) StartBackgroundTasks() {
	s.backgroundOnce.Do(func() {
		if err := s.LoadAgentSessions(); err != nil {
			s.logger.Error("failed to load agent sessions from database", "error", err)
		}
		go s.pruneOldMetrics()
		if s.agentSessionLoader != nil {
			go s.cleanupLoop()
		}
		go s.contractRemovalSweepLoop()
		if s.taskReaper != nil {
			go s.taskWatchdogLoop()
		}
	})
}

// Stop cancels background goroutines.
func (s *ControlService) Stop() {
	s.cancel()
}

// GetStats returns server statistics.
func (s *ControlService) GetStats() map[string]interface{} {
	s.registry.Mu().RLock()
	defer s.registry.Mu().RUnlock()
	agents := s.registry.AgentsUnsafe()
	return map[string]interface{}{
		"agent_count": len(agents),
		"session_ttl": s.defaultSessionTTL.String(),
	}
}

// ---- internal ----

func (s *ControlService) handleRequest(ctx context.Context, msgID uint32, data []byte) ([]byte, error) {
	switch msgID {
	case protocol.MsgRegisterRequest:
		return s.handleRegister(ctx, data)
	case protocol.MsgHeartbeatRequest:
		return s.handleHeartbeat(ctx, data)
	case protocol.MsgRegisterCapabilitiesReq:
		return s.handleRegisterCapabilities(ctx, data)
	case protocol.MsgTaskEvent:
		return s.handleTaskEvent(ctx, data)
	case protocol.MsgMetricEvent:
		return s.handleMetricEvent(ctx, data)
	case protocol.MsgAlertEventRequest:
		return s.handleAlertEvent(ctx, data)
	case protocol.MsgAlertEvent:
		return s.handleProbeEvent(ctx, data)
	case protocol.MsgCaptureConfigRequest:
		return s.handleCaptureConfig(ctx, data)
	case protocol.MsgProbePolicyRequest:
		return s.handleProbePolicyConfig(ctx, data)
	case protocol.MsgProbeDBTargetsRequest:
		return s.handleProbeDBTargets(ctx, data)
	case protocol.MsgDevopsAllowlistConfigRequest:
		return s.handleDevopsAllowlistConfig(ctx, data)
	case protocol.MsgSyntheticsTargetsConfigRequest:
		return s.handleSyntheticsTargetsConfig(ctx, data)
	default:
		return nil, fmt.Errorf("unknown message type: 0x%06X", msgID)
	}
}

func (s *ControlService) handleRequestAsAgent(ctx context.Context, msgID uint32, data []byte, identity *AgentIdentity) ([]byte, error) {
	switch msgID {
	case protocol.MsgMetricEvent:
		return s.handleMetricEventAsAgent(ctx, data, identity)
	case protocol.MsgAlertEventRequest:
		return s.handleAlertEventAsAgent(ctx, data, identity)
	case protocol.MsgAlertEvent:
		return s.handleProbeEventAsAgent(ctx, data, identity)
	case protocol.MsgDevopsSessionEvent:
		// 单向事件帧（agent → server，MuxConn 事件路径丢弃返回值）：
		// 路由到中继运营面扇出给面板消费者；未知会话/未装配静默丢弃。
		s.mu.RLock()
		hub := s.devopsSessions
		s.mu.RUnlock()
		if hub != nil {
			ev := &opsv1.DevopsSessionEvent{}
			if err := proto.Unmarshal(data, ev); err == nil {
				hub.RouteEvent(identity.AgentID, ev)
			}
		}
		return nil, nil
	default:
		return s.handleRequest(ctx, msgID, data)
	}
}
