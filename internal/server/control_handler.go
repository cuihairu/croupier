// Package server provides the Server-side control plane for Croupier.
package server

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	versionutil "github.com/cuihairu/croupier/internal/common/version"
	"github.com/cuihairu/croupier/internal/function/converter"
	"github.com/cuihairu/croupier/internal/function/registrationguard"
	"github.com/cuihairu/croupier/internal/function/schemadiff"
	"github.com/cuihairu/croupier/internal/model"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/platform/registry/sdkversion"
	"github.com/cuihairu/croupier/internal/tasks"
	transportcore "github.com/cuihairu/croupier/internal/transport"
	agentv1 "github.com/cuihairu/croupier/pkg/pb/croupier/agent/v1"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	"github.com/getkin/kin-openapi/openapi3"
	"google.golang.org/protobuf/proto"
	"gorm.io/gorm"
)

var (
	functionIDPattern = regexp.MustCompile(`^[a-z0-9]+(?:[._-][a-z0-9]+)*(?:\.[a-z0-9]+(?:[._-][a-z0-9]+)*)+$`)
)

type registerWarning struct {
	Code       string
	FunctionID string
	Version    string
	Message    string
}

// ListenAddr represents a single listen address with transport type.
type ListenAddr struct {
	Addr      string // Raw address (e.g., ":19090", "ipc://croupier-server")
	Transport string // Transport type: "tcp", "ipc", etc.
	URL       string // Full URL (e.g., "tcp://:19090", "ipc://croupier-server")
}

// ParseListenAddr parses a string address into a ListenAddr.
func ParseListenAddr(addr string) ListenAddr {
	if strings.Contains(addr, "://") {
		parts := strings.SplitN(addr, "://", 2)
		return ListenAddr{
			Addr:      parts[1],
			Transport: parts[0],
			URL:       addr,
		}
	}
	return ListenAddr{
		Addr:      addr,
		Transport: "tcp",
		URL:       "tcp://" + addr,
	}
}

// IsLocalTCP checks if an address is a local TCP address.
func IsLocalTCP(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	host = strings.Trim(host, "[]")
	switch strings.ToLower(host) {
	case "", "localhost", "127.0.0.1", "::1":
		return true
	}
	return false
}

// AgentSessionLoader defines the interface for loading and managing agent sessions from a database.
type AgentSessionLoader interface {
	LoadActiveSessions(ctx context.Context) ([]*reg.AgentSession, error)
	Upsert(ctx context.Context, sess *reg.AgentSession) error
	DeleteExpired(ctx context.Context) (int64, error)
}

// TaskStore defines the interface for task run persistence used by handleTaskEvent.
type TaskStore interface {
	UpdateRunIfStatusNotIn(ctx context.Context, taskID string, blockedStatuses []string, updates map[string]interface{}) (bool, error)
	AppendEvent(ctx context.Context, taskID string, eventType tasks.EventType, progress int32, message string, payload []byte) error
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
	systemInfoCache   *reg.SystemInfoCache
	taskStore         TaskStore

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
		OnAgentRegistered(ctx context.Context, agentID, gameID, env string)
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
		systemInfoCache:    reg.NewSystemInfoCache(),
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

// Store returns the registry store.
func (s *ControlService) Store() *reg.Store { return s.registry }

// MetricsStore returns the metrics store.
func (s *ControlService) MetricsStore() *reg.MetricsStore { return s.metricsStore }

// SetMetricsDB sets the database for metrics persistence.
func (s *ControlService) SetMetricsDB(db *gorm.DB) {
	if s.metricsStore != nil {
		s.metricsStore.SetDB(db)
		s.metricsStore.StartCleanupRoutine(s.ctx, 1*time.Hour)
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
		return s.handleRequest(ctx, msgID, body)
	})
}

// StartBackgroundTasks starts background maintenance loops (DB loading, metrics pruning, etc.).
// SetClusterHooks 注入集群归属钩子（幂等，可在装配期任意时刻调用）。
func (s *ControlService) SetClusterHooks(h interface {
	OnAgentRegistered(ctx context.Context, agentID, gameID, env string)
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
	default:
		return nil, fmt.Errorf("unknown message type: 0x%06X", msgID)
	}
}

func (s *ControlService) handleRegister(ctx context.Context, data []byte) ([]byte, error) {
	req := &agentv1.RegisterRequest{}
	if err := proto.Unmarshal(data, req); err != nil {
		return nil, fmt.Errorf("unmarshal RegisterRequest: %w", err)
	}
	resp, err := s.handleRegisterRequest(ctx, req, "")
	if err != nil {
		return nil, err
	}
	return proto.Marshal(resp)
}

func (s *ControlService) handleHeartbeat(ctx context.Context, data []byte) ([]byte, error) {
	req := &agentv1.HeartbeatRequest{}
	if err := proto.Unmarshal(data, req); err != nil {
		return nil, fmt.Errorf("unmarshal HeartbeatRequest: %w", err)
	}
	resp, err := s.handleHeartbeatRequest(ctx, req)
	if err != nil {
		return nil, err
	}
	return proto.Marshal(resp)
}

func (s *ControlService) handleRegisterCapabilities(ctx context.Context, data []byte) ([]byte, error) {
	req := &agentv1.RegisterCapabilitiesRequest{}
	if err := proto.Unmarshal(data, req); err != nil {
		return nil, fmt.Errorf("unmarshal RegisterCapabilitiesRequest: %w", err)
	}
	resp, err := s.handleRegisterCapabilitiesRequest(ctx, req)
	if err != nil {
		return nil, err
	}
	return proto.Marshal(resp)
}

func (s *ControlService) handleTaskEvent(ctx context.Context, data []byte) ([]byte, error) {
	req := &sdkv1.TaskEvent{}
	if err := proto.Unmarshal(data, req); err != nil {
		return nil, fmt.Errorf("unmarshal TaskEvent: %w", err)
	}

	s.mu.RLock()
	taskStore := s.taskStore
	s.mu.RUnlock()
	if taskStore == nil {
		return nil, fmt.Errorf("task store not configured")
	}

	taskID := strings.TrimSpace(req.GetTaskId())
	if taskID == "" {
		return nil, fmt.Errorf("task_id is required")
	}

	now := time.Now()
	blockedStatuses := tasks.TerminalStatuses()
	updates := map[string]interface{}{
		"progress": req.GetProgress(),
		"message":  req.GetMessage(),
	}

	switch strings.ToLower(strings.TrimSpace(req.GetType())) {
	case string(tasks.EventStarted):
		blockedStatuses = append(blockedStatuses, tasks.StatusCancelRequested)
		updates["status"] = tasks.StatusRunning
		updates["started_at"] = &now
	case string(tasks.EventProgress), string(tasks.EventLog):
		blockedStatuses = append(blockedStatuses, tasks.StatusCancelRequested)
		updates["status"] = tasks.StatusRunning
	case string(tasks.EventCompleted):
		updates["status"] = tasks.StatusSucceeded
		updates["progress"] = int32(100)
		updates["finished_at"] = &now
		updates["result_payload"] = model.EncodeTaskPayload(req.GetPayload())
	case string(tasks.EventFailed):
		updates["status"] = tasks.StatusFailed
		updates["finished_at"] = &now
		updates["error_message"] = req.GetMessage()
	case string(tasks.EventCancelRequested):
		blockedStatuses = append(blockedStatuses, tasks.StatusCancelRequested)
		updates["status"] = tasks.StatusCancelRequested
		updates["cancel_requested_at"] = &now
	case string(tasks.EventCancelled):
		updates["status"] = tasks.StatusCancelled
		updates["finished_at"] = &now
	default:
		blockedStatuses = append(blockedStatuses, tasks.StatusCancelRequested)
		updates["status"] = tasks.StatusRunning
	}

	updated, err := taskStore.UpdateRunIfStatusNotIn(ctx, taskID, blockedStatuses, updates)
	if err != nil {
		return nil, fmt.Errorf("update task run: %w", err)
	}
	if !updated {
		// A terminal task must remain terminal. Do not append the late event
		// either: task streams stop at a terminal event and must not expose a
		// misleading event after that boundary.
		return nil, nil
	}
	if err := taskStore.AppendEvent(ctx, taskID, tasks.EventType(req.GetType()), req.GetProgress(), req.GetMessage(), req.GetPayload()); err != nil {
		return nil, fmt.Errorf("append task event: %w", err)
	}
	return nil, nil
}

// handleMetricEvent accepts a pushed MetricsReport snapshot from an agent.
// The report is stored in the MetricsStore for later retrieval by the ops API.
func (s *ControlService) handleMetricEvent(ctx context.Context, data []byte) ([]byte, error) {
	req := &opsv1.MetricsReport{}
	if err := proto.Unmarshal(data, req); err != nil {
		return nil, fmt.Errorf("unmarshal MetricsReport: %w", err)
	}

	agentID := strings.TrimSpace(req.GetAgentId())
	if agentID == "" {
		return nil, fmt.Errorf("agent_id is required")
	}

	// Store metrics for later retrieval
	if s.metricsStore != nil {
		s.metricsStore.Add(agentID, req)
	}

	if s.logger != nil {
		s.logger.Debug("received metrics report",
			"agent_id", agentID,
			"cpu_percent", req.GetCpu().GetUsagePercent(),
			"mem_used_bytes", req.GetMemory().GetUsedBytes(),
			"disk_count", len(req.GetDisks()),
			"network_count", len(req.GetNetworks()),
		)
	}
	return nil, nil
}

// handleRegisterRequest implements the actual Register logic.
func (s *ControlService) handleRegisterRequest(ctx context.Context, req *agentv1.RegisterRequest, remoteAddr string) (*agentv1.RegisterResponse, error) {
	if s.upstream != nil {
		return s.upstream.HandleRegister(ctx, req)
	}

	ttl := 24 * time.Hour
	if req.TtlSeconds > 0 {
		ttl = time.Duration(req.TtlSeconds) * time.Second
	}

	functions, warnings := validateAndNormalizeFunctions(req.GetFunctions())
	warningTexts := make([]string, 0, len(warnings))
	for _, warnMsg := range warnings {
		warningTexts = append(warningTexts, warnMsg.Message)
		s.logger.Warn("register validation warning", "agent_id", req.AgentId, "warning", warnMsg.Message, "code", warnMsg.Code, "function_id", warnMsg.FunctionID, "version", warnMsg.Version)
		s.registry.UpsertRegistrationWarning(ctx, reg.FunctionRegistrationWarning{
			GameID:     req.GameId,
			Env:        req.Env,
			AgentID:    req.AgentId,
			FunctionID: warnMsg.FunctionID,
			Version:    warnMsg.Version,
			Code:       warnMsg.Code,
			Message:    warnMsg.Message,
		})
	}

	// SDK 滑动版本门槛（针对函数注册）：按 (game_id, env, sdk_language) 的
	// 历史最高 SDK 版本评估各 provider 进程；落后超容忍窗（低 2 个及以上
	// minor 或 1 个及以上 major）的进程其函数不进本次注册。判定在
	// sess.Functions 构造之前完成；抬升高水位在 UpsertAgent 成功之后。
	sdkFloor := s.evaluateSDKVersionFloor(ctx, req, &warningTexts)

	sess := &reg.AgentSession{
		AgentID: req.AgentId,
		GameID:  req.GameId,
		Env:     req.Env,
		Addr:    remoteAddr,
		Version: req.Version,
		Region:  "",
		Zone:    "",
		// 保留 agent 自报系统标签（os/arch/hostname/ip 等）：IP 列显示的是
		// server 看到的 TCP 对端（经 LB 时全为 LB 地址），自报 ip/hostname
		// 是区分 agent 的关键信息。心跳路径的 reportedOwner 在后续心跳写入。
		Labels:    sanitizeAgentLabels(req.GetLabels()),
		ExpireAt:  time.Now().Add(ttl),
		LastSeen:  time.Now(),
		Functions: map[string]reg.FunctionMeta{},
	}

	// validateAndNormalizeFunctions 的后置条件：输出切片中每个 f 非 nil 且
	// Id 为 trim+lower 后非空、通过 functionIDPattern 校验的值（nil 与
	// 空 Id 在该函数内已全部 continue 剔除并转为 warning），故此处原
	// `if f == nil || f.Id == "" { continue }` 防御分支为死代码，已删。
	for _, f := range functions {
		if sdkFloor.rejected[f.Id] {
			continue
		}
		sess.Functions[f.Id] = reg.FunctionMeta{
			Enabled:           f.Enabled,
			Version:           f.Version,
			Tags:              append([]string(nil), f.Tags...),
			Summary:           f.GetSummary(),
			Description:       f.GetDescription(),
			OperationID:       f.Id,
			Deprecated:        f.GetDeprecated(),
			InputSchema:       f.GetInputSchema(),
			OutputSchema:      f.GetOutputSchema(),
			Resource:          f.GetResource(),
			Operation:         f.GetOperation(),
			Capability:        f.GetCapability(),
			Execution:         f.GetExecution(),
			ApprovalRequired:  f.GetApprovalRequired(),
			ApprovalPolicyKey: f.GetApprovalPolicyKey(),
			Risk:              f.GetRisk(),
			Permission:        f.GetPermission(),
		}
		if op, err := converter.ToOpenAPIOperation(converter.ProviderFunctionDescriptorDesc{
			ID:           f.Id,
			Version:      f.Version,
			Tags:         f.Tags,
			Summary:      f.GetSummary(),
			Description:  f.GetDescription(),
			OperationID:  f.Id,
			Deprecated:   f.GetDeprecated(),
			InputSchema:  f.GetInputSchema(),
			OutputSchema: f.GetOutputSchema(),
			Resource:     f.GetResource(),
			Operation:    f.GetOperation(),
			Capability:   f.GetCapability(),
			Execution:    f.GetExecution(),
			Risk:         f.GetRisk(),
			Permission:   f.GetPermission(),
		}); err == nil {
			// UpsertOpenAPI 的错误在合法输入域不可达（C 类）：functionID 非空
			// 由 validateAndNormalizeFunctions 后置条件保证（空 Id 被 skip）；op 非
			// nil 由 converter 成功路径恒返回字面量构造保证；cloneOpenAPIOperation
			// 的 MarshalJSON 对「Unmarshal 成功产物」恒成功——与 versioning 曾误删
			// 的场景（未校验的 JSONSchema 原始文本直接透传进 Marshal，可被
			// "{invalid" 击穿）本质不同：非法 InputSchema 在 converter 的第一层
			// json.Unmarshal 即被拒绝（走下方 else Warn 分支），进入 clone 的
			// Schema 是 kin-openapi 类型化字段 + Extensions(仅 string 字面量
			// x-resource/x-risk 等)，json.Unmarshal 产出的值类型集合对 Marshal
			// 无条件可序列化。已用 28 组恶意 InputSchema（2000 层深嵌套、lone
			// surrogate、任意 x- 扩展 map、大整数、混合 enum、$ref 等）实证
			// 无法构造 UpsertOpenAPI 失败。该 Warn 为防御性错误处理保留，
			// 经 upsertOpenAPI 注入点测试驱动。
			upsert := s.upsertOpenAPI
			if upsert == nil {
				upsert = s.registry.UpsertOpenAPI
			}
			if err := upsert(f.Id, op); err != nil {
				s.logger.Warn("failed to upsert openapi operation from register request", "function_id", f.Id, "error", err)
			}
		} else {
			s.logger.Warn("failed to convert function descriptor to openapi operation", "function_id", f.Id, "error", err)
		}
	}

	// F12：注册时 schema 兼容性检查——与会话中上一次注册的 schema 对比，
	// 破坏性变更写入注册警告并随 RegisterResponse.warnings 返回 agent。
	// 只告警不阻断；必须在 UpsertAgent 覆盖会话之前执行。
	if s.schemaDiffWarn {
		// 同上：functions 经 validateAndNormalizeFunctions 后置条件保证非 nil
		// 且 Id 非空，原防御分支为死代码，已删。被门槛拒绝的函数不进本次
		// 注册，其 schema 对比是无意义的噪音，跳过。
		for _, f := range functions {
			if sdkFloor.rejected[f.Id] {
				continue
			}
			oldInput, oldOutput, ok := s.registry.PreviousFunctionSchema(req.AgentId, req.GameId, req.Env, f.Id)
			if !ok {
				continue
			}
			findings := schemadiff.DiffSchemas("inputSchema", json.RawMessage(oldInput), json.RawMessage(f.GetInputSchema()))
			findings = append(findings,
				schemadiff.DiffSchemas("outputSchema", json.RawMessage(oldOutput), json.RawMessage(f.GetOutputSchema()))...)
			for _, finding := range findings {
				if finding.Severity != schemadiff.SeverityBreaking {
					continue
				}
				warning := fmt.Sprintf("function %s %s%s: %s", f.Id, finding.Source, finding.Path, finding.Reason)
				warningTexts = append(warningTexts, warning)
				s.logger.Warn("register schema breaking change", "agent_id", req.AgentId, "function_id", f.Id, "source", finding.Source, "path", finding.Path, "reason", finding.Reason)
				s.registry.UpsertRegistrationWarning(ctx, reg.FunctionRegistrationWarning{
					GameID:     req.GameId,
					Env:        req.Env,
					AgentID:    req.AgentId,
					FunctionID: f.Id,
					Version:    f.Version,
					Code:       "schema_breaking_change",
					Message:    warning,
				})
			}
		}
	}

	if len(req.Processes) > 0 {
		providers := make([]reg.ProviderSession, 0, len(req.Processes))
		for _, p := range req.Processes {
			if p == nil || p.ServiceId == "" {
				continue
			}
			// Provider（SDK/自定义游戏服）scope 校验（作用域规范 §14.5）：
			// agent 会话绑定单一 (game_id, env)，provider 上报的 scope 必须
			// 一致——空值同样视为 mismatch，无兼容分支。不一致说明 SDK 侧
			// 配置错误：产生业务层注册警告（路由仍按 agent scope 保持稳定）。
			s.validateProviderScope(ctx, req, p, &warningTexts)
			// provider 会话版本与函数槽同门槛：非 semver（含空/"unknown"/误传
			// providerID）不进 version 槽——函数实例页该列即「契约版本」，
			// 脏值直通会让下游 sdkversion gate/排障全部失真。
			providerVersion := strings.TrimSpace(p.Version)
			if !versionutil.IsValid(providerVersion) {
				normalized := versionutil.ValidOrDefault(providerVersion)
				if providerVersion != "" {
					warningTexts = append(warningTexts, fmt.Sprintf("provider_version_invalid: service=%s version=%q is not valid semver; normalized to %s", p.ServiceId, p.Version, normalized))
				}
				providerVersion = normalized
			}
			providers = append(providers, reg.ProviderSession{
				ProviderID:   p.ServiceId,
				GameID:       req.GameId,
				Env:          req.Env,
				Addr:         p.Addr,
				Version:      providerVersion,
				SDKLanguage:  p.SdkLanguage,
				SDKVersion:   p.SdkVersion,
				SDKName:      p.SdkName,
				LastSeenUnix: p.LastSeenUnix,
				FunctionIDs:  p.FunctionIds,
			})
		}
		sess.Providers = providers
	}

	if err := s.registry.UpsertAgent(sess); err != nil {
		return nil, fmt.Errorf("register agent dashboard contract rebuild failed: %w", err)
	}

	// 门槛放行的 provider 注册成功后抬升 (game_id, env, sdk_language) 高水位，
	// 作为后续注册的滑动下限。放行 ≠ 抬升：低版本放行（容忍窗内）不抬；
	// 不可解析版本（"unknown"）不抬。
	for _, raise := range sdkFloor.raises {
		s.registry.ObserveSDKVersion(req.GameId, req.Env, raise.language, raise.version)
	}

	// 集群归属：本实例持有该 Agent 连接（多实例 HA 转发路由依据）。
	s.mu.RLock()
	hooks := s.clusterHooks
	s.mu.RUnlock()
	if hooks != nil {
		hooks.OnAgentRegistered(ctx, req.AgentId, req.GameId, req.Env)
	}

	// NewStoreWithDB persists the session only after contract/proposal
	// materialization succeeds. Do not duplicate that write through the loader;
	// the loader remains the persistence path for memory-only registries and is
	// still used by heartbeat/expiry maintenance.
	if s.agentSessionLoader != nil && !s.registry.SessionPersistenceEnabled() {
		if err := s.agentSessionLoader.Upsert(ctx, sess); err != nil {
			s.logger.Error("failed to write agent session to database", "agent_id", req.AgentId, "error", err)
		}
	}

	s.logger.Info("Agent registered", "agent_id", req.AgentId, "game_id", req.GameId, "functions", len(functions), "warnings", len(warnings))

	s.mu.RLock()
	selfID := s.clusterInstanceID
	s.mu.RUnlock()
	return &agentv1.RegisterResponse{
		Warnings:   warningTexts,
		InstanceId: selfID,
	}, nil
}

func (s *ControlService) handleHeartbeatRequest(ctx context.Context, req *agentv1.HeartbeatRequest) (*agentv1.HeartbeatResponse, error) {
	if s.upstream != nil {
		return s.upstream.HandleHeartbeat(ctx, req)
	}

	if req.AgentId == "" {
		return &agentv1.HeartbeatResponse{}, nil
	}

	s.registry.Mu().Lock()
	agent := s.registry.AgentsUnsafe()[req.AgentId]
	// agent 自报 owner（三方对账，agent 视角）：存入 session labels 供
	// nodes/cluster 视图与归属表 claim、LB 会话对照（半开/漂移探测）。
	if agent != nil && req.OwnerInstanceId != "" {
		if agent.Labels == nil {
			agent.Labels = map[string]string{}
		}
		agent.Labels["reportedOwner"] = req.OwnerInstanceId
	}
	if agent == nil {
		s.registry.Mu().Unlock()
		// 僵尸防线：会话在本地 registry 意外丢失（过期清理/替换竞态）但
		// TCP 连接仍活——心跳不能再静默成功，否则归属行冻结、agent 永不
		// 自愈。从共享归属表回读真实 scope 重建最小会话并重新 Claim。
		var gameID, env string
		var owns bool
		s.mu.RLock()
		lookup := s.heartbeatOwnerLookup
		s.mu.RUnlock()
		if lookup != nil {
			gameID, env, owns = lookup.SelfOwnerScope(ctx, req.AgentId)
		}
		if !owns {
			s.logger.Error("heartbeat for unknown agent session; cannot self-heal (not this instance's claim or no owner record)", "agent_id", req.AgentId)
			return &agentv1.HeartbeatResponse{}, nil
		}
		ttl := s.defaultSessionTTL
		if ttl <= 0 {
			ttl = 24 * time.Hour
		}
		reseed := &reg.AgentSession{
			AgentID:   req.AgentId,
			GameID:    gameID,
			Env:       env,
			ExpireAt:  time.Now().Add(ttl),
			LastSeen:  time.Now(),
			Labels:    map[string]string{"reseeded": "true"},
			Functions: map[string]reg.FunctionMeta{},
			Providers: []reg.ProviderSession{},
		}
		if err := s.registry.UpsertAgent(reseed); err != nil {
			s.logger.Error("heartbeat self-heal reseed failed", "agent_id", req.AgentId, "error", err)
			return &agentv1.HeartbeatResponse{}, nil
		}
		s.mu.RLock()
		hooks := s.clusterHooks
		s.mu.RUnlock()
		if hooks != nil {
			hooks.OnAgentRegistered(ctx, req.AgentId, gameID, env)
		}
		s.logger.Warn("heartbeat self-healed: session was missing, re-seeded from ownership table",
			"agent_id", req.AgentId, "game_id", gameID, "env", env)
		return &agentv1.HeartbeatResponse{}, nil
	}
	if agent != nil {
		agent.ExpireAt = time.Now().Add(s.defaultSessionTTL)
		agent.LastSeen = time.Now()
		s.registry.Mu().Unlock()
		s.mu.RLock()
		hooks := s.clusterHooks
		s.mu.RUnlock()
		if hooks != nil {
			hooks.OnAgentHeartbeat(ctx, req.AgentId)
		}
		s.registry.Mu().Lock()
		if s.agentSessionLoader != nil && s.shouldPersistSession(req.AgentId) {
			// 快照后再异步落盘：agent 指针仍挂在 registry 上，后续心跳会在
			// registry 锁内继续写 ExpireAt/LastSeen 与 Labels["reportedOwner"]，
			// 异步 Upsert 直接读活动指针构成 data race。
			agentToUpdate := snapshotAgentSession(agent)
			go func() {
				if err := s.agentSessionLoader.Upsert(context.Background(), agentToUpdate); err != nil {
					s.logger.Error("failed to update agent session in database", "agent_id", req.AgentId, "error", err)
				}
			}()
		}
	}
	s.registry.Mu().Unlock()

	return &agentv1.HeartbeatResponse{}, nil
}

// sanitizeAgentLabels 复制 agent 注册时上报的系统标签。限制数量与键值
// 长度，防止注册路径被塞入任意大的标签集合；空键/空值丢弃，超长值截断。
func sanitizeAgentLabels(labels map[string]string) map[string]string {
	const (
		maxLabels = 32
		maxKeyLen = 64
		maxValLen = 256
	)
	out := make(map[string]string, min(len(labels), maxLabels))
	for k, v := range labels {
		if len(out) >= maxLabels {
			break
		}
		k = strings.TrimSpace(k)
		if k == "" || len(k) > maxKeyLen {
			continue
		}
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if len(v) > maxValLen {
			v = v[:maxValLen]
		}
		out[k] = v
	}
	return out
}

// snapshotAgentSession 在 registry 锁内拷贝会话快照（异步落盘用）。
// 值拷贝覆盖标量与 map 引用：Functions/Providers 是替换语义（重新注册
// 换新 map，旧 map 不再被写），引用拷贝即安全；Labels 是原地 merge
// 语义（心跳写 reportedOwner、注册 merge 标签），必须深拷贝。
func snapshotAgentSession(a *reg.AgentSession) *reg.AgentSession {
	if a == nil {
		return nil
	}
	snap := *a
	if a.Labels != nil {
		snap.Labels = make(map[string]string, len(a.Labels))
		for k, v := range a.Labels {
			snap.Labels[k] = v
		}
	}
	return &snap
}

// shouldPersistSession 心跳落盘节流：间隔内只落一次（注册路径不受限）。
func (s *ControlService) shouldPersistSession(agentID string) bool {
	s.sessionPersistMu.Lock()
	defer s.sessionPersistMu.Unlock()
	now := time.Now()
	if s.sessionPersistInterval <= 0 {
		s.sessionPersistInterval = time.Minute
	}
	if s.sessionPersistAt == nil {
		s.sessionPersistAt = map[string]time.Time{}
	}
	if last, ok := s.sessionPersistAt[agentID]; ok && now.Sub(last) < s.sessionPersistInterval {
		return false
	}
	s.sessionPersistAt[agentID] = now
	return true
}

// validateProviderScope 校验 provider 上报的 game/env 与 agent 会话 scope
// 是否一致。警告只进业务层注册警告（/system/functions/warnings，研发
// 视角）——分层原则：业务层（接入契约）与支持层（运维告警，资源/
// 基础设施）分开，运维不关注业务接入问题，研发不关注支持层信息。
// 修复闭环同样在业务层：一致注册时清除该 provider 的历史 mismatch
// 警告（生命周期跟随注册行为）。硬切无兼容：空 scope 即 mismatch。
func (s *ControlService) validateProviderScope(ctx context.Context, req *agentv1.RegisterRequest, p *agentv1.AgentProcess, warningTexts *[]string) {
	// 硬切无兼容（作用域规范 §14）：provider 必须上报 scope 且与 agent
	// 会话一致——空值同样视为 mismatch（SDK 必须显式携带 scope）。
	mismatches := make([]string, 0, 2)
	if p.GameId != req.GameId {
		mismatches = append(mismatches, fmt.Sprintf("game_id mismatch: provider=%q agent=%q", p.GameId, req.GameId))
	}
	if p.Env != req.Env {
		mismatches = append(mismatches, fmt.Sprintf("env mismatch: provider=%q agent=%q", p.Env, req.Env))
	}

	if len(mismatches) == 0 {
		// 一致注册：清除该 agent 的历史 scope mismatch 警告（修复闭环）。
		s.registry.RemoveRegistrationWarnings(reg.RegistrationWarningFilter{
			GameID:  req.GameId,
			Env:     req.Env,
			AgentID: req.AgentId,
			Code:    "provider_scope_mismatch",
		})
		return
	}

	msg := fmt.Sprintf("service_id=%s: %s", p.ServiceId, strings.Join(mismatches, "; "))
	*warningTexts = append(*warningTexts, msg)
	s.logger.Warn("provider scope mismatch", "agent_id", req.AgentId, "service_id", p.ServiceId, "detail", msg)
	s.registry.UpsertRegistrationWarning(ctx, reg.FunctionRegistrationWarning{
		GameID:  req.GameId,
		Env:     req.Env,
		AgentID: req.AgentId,
		Code:    "provider_scope_mismatch",
		Message: msg,
	})
}

// sdkFloorRaise 是一次通过门槛且需要抬升高水位的版本观测。
type sdkFloorRaise struct {
	language string
	version  string
}

// sdkFloorOutcome 是一次注册请求的 SDK 滑动版本门槛评估结果：
// rejected 为被拒函数集合（不进 sess.Functions、不物化契约），raises 为
// 注册成功后待抬升的 (language, version) 观测。
type sdkFloorOutcome struct {
	rejected map[string]bool
	raises   []sdkFloorRaise
}

// evaluateSDKVersionFloor 按 (game_id, env, sdk_language) 的历史最高 SDK
// 版本（高水位）评估注册请求中的 provider 进程（sdkversion 三档判定）：
//
//   - 配置了 registry.sdkVersionMinimums 的语言，自报版本低于配置值
//     的进程直接拒注册（绝对下限，优先于高水位判定，无高水位也生效）；
//   - 无 sdk_language/sdk_version 自报的进程（自定义游戏服直连）不参与
//     门槛，函数照常放行且不抬升高水位；
//   - 版本解析失败的进程一律放行（门槛只在两端都可解析时生效）；
//   - 落后超容忍窗（低 2 个及以上 minor 或 1 个及以上 major）的进程，
//     其独占声明的函数从本次注册中剔除——多个进程交叉提供同一函数时，
//     只要任一放行进程也声明了该函数就不剔（避免误伤），即使其声明者
//     部分被拒。
//
// 剔除只作用于函数面（sess.Functions 与物化路径）；provider 进程记录与
// 其自报 function_ids 保留在会话中（进程存在是事实，调度候选第一道门
// 是 agent.Functions，不会误路由）。连接保持，拒绝以注册警告 +
// RegisterResponse.warnings 传达 agent 日志。
func (s *ControlService) evaluateSDKVersionFloor(ctx context.Context, req *agentv1.RegisterRequest, warningTexts *[]string) sdkFloorOutcome {
	outcome := sdkFloorOutcome{rejected: map[string]bool{}}
	if len(req.Processes) == 0 {
		return outcome
	}
	rejectedByProc := map[string][]string{} // service_id -> 被拒声明的函数
	allowedDeclared := map[string]bool{}    // 任一放行进程声明的函数
	// 函数级最低版本门槛（function_version_floors 表，UI 按函数配置）：
	// 批量取一次，逐函数判定——低于该函数配置值的声明单独被拒（不影响
	// 同进程其他达标函数），优先级高于语言级 yaml 与高水位（后两者是
	// 进程级判定，函数级在放行路径上再叠一层）。未自报版本的进程
	// （SdkVersion 为空）Below 恒 false，不受函数级门槛限制（保守一致）。
	fnFloors := s.registry.GetFunctionVersionFloors(req.GameId, req.Env)
	fnRejected := map[string][]string{} // service_id -> 函数级被拒的 "fid(<min)"
	allowWithFloor := func(p *agentv1.AgentProcess) {
		for _, fid := range p.FunctionIds {
			if min, ok := fnFloors[fid]; ok && sdkversion.Below(p.SdkVersion, min) {
				fnRejected[p.ServiceId] = append(fnRejected[p.ServiceId], fid+"(<"+min+")")
				continue
			}
			allowedDeclared[fid] = true
		}
	}
	for _, p := range req.Processes {
		if p == nil || p.ServiceId == "" {
			continue
		}
		language := strings.TrimSpace(p.SdkLanguage)
		if language == "" || strings.TrimSpace(p.SdkVersion) == "" {
			for _, fid := range p.FunctionIds {
				allowedDeclared[fid] = true
			}
			continue
		}
		// 配置最低版本门槛（registry.sdkVersionMinimums）：绝对下限，
		// 优先于滑动高水位——即使尚无高水位观测，低于配置值也拒注册。
		// 版本解析失败不触发（Below 恒 false，与高水位门槛同样保守）。
		if minimum, ok := s.sdkVersionMinimums[strings.ToLower(language)]; ok && sdkversion.Below(p.SdkVersion, minimum) {
			rejectedByProc[p.ServiceId] = p.FunctionIds
			msg := fmt.Sprintf("service_id=%s sdk=%s/%s below configured minimum %s: function registration rejected (%d functions)", p.ServiceId, language, p.SdkVersion, minimum, len(p.FunctionIds))
			*warningTexts = append(*warningTexts, msg)
			s.logger.Warn("sdk version below configured minimum", "agent_id", req.AgentId, "service_id", p.ServiceId, "sdk_language", language, "sdk_version", p.SdkVersion, "configured_minimum", minimum, "functions", len(p.FunctionIds))
			s.registry.UpsertRegistrationWarning(ctx, reg.FunctionRegistrationWarning{
				GameID:  req.GameId,
				Env:     req.Env,
				AgentID: req.AgentId,
				Code:    reg.WarningCodeSDKVersionBelowMinimum,
				Message: msg,
			})
			continue
		}
		floor := s.registry.GetSDKVersionFloor(req.GameId, req.Env, language)
		switch sdkversion.Compare(p.SdkVersion, floor) {
		case sdkversion.VerdictAllow:
			allowWithFloor(p)
			// 不可解析版本 Compare 恒 Allow，但高水位只记可解析版本。
			if sdkversion.Parseable(p.SdkVersion) {
				outcome.raises = append(outcome.raises, sdkFloorRaise{language: language, version: p.SdkVersion})
			}
		case sdkversion.VerdictAllowWithWarning:
			allowWithFloor(p)
			msg := fmt.Sprintf("service_id=%s sdk=%s/%s behind high watermark %s: registration accepted, upgrade the SDK", p.ServiceId, language, p.SdkVersion, floor)
			*warningTexts = append(*warningTexts, msg)
			s.logger.Warn("sdk version behind high watermark", "agent_id", req.AgentId, "service_id", p.ServiceId, "sdk_language", language, "sdk_version", p.SdkVersion, "high_watermark", floor)
			s.registry.UpsertRegistrationWarning(ctx, reg.FunctionRegistrationWarning{
				GameID:  req.GameId,
				Env:     req.Env,
				AgentID: req.AgentId,
				Code:    reg.WarningCodeSDKVersionBehind,
				Message: msg,
			})
		case sdkversion.VerdictReject:
			rejectedByProc[p.ServiceId] = p.FunctionIds
			msg := fmt.Sprintf("service_id=%s sdk=%s/%s behind high watermark %s by 2+ minors or 1+ major: function registration rejected (%d functions)", p.ServiceId, language, p.SdkVersion, floor, len(p.FunctionIds))
			*warningTexts = append(*warningTexts, msg)
			s.logger.Warn("sdk version floor rejected registration", "agent_id", req.AgentId, "service_id", p.ServiceId, "sdk_language", language, "sdk_version", p.SdkVersion, "high_watermark", floor, "functions", len(p.FunctionIds))
			s.registry.UpsertRegistrationWarning(ctx, reg.FunctionRegistrationWarning{
				GameID:  req.GameId,
				Env:     req.Env,
				AgentID: req.AgentId,
				Code:    reg.WarningCodeSDKVersionFloorRejected,
				Message: msg,
			})
		}
	}
	// 函数级门槛拒绝：按进程聚合一条告警（fid(<min) 列出各函数的门槛
	// 值），拒绝集与进程级判定共用同一交叉提供保护。
	for serviceID, fids := range fnRejected {
		rejectedByProc[serviceID] = append(rejectedByProc[serviceID], bareFunctionIDs(fids)...)
		msg := fmt.Sprintf("service_id=%s sdk below function configured minimum: function registration rejected (%d functions: %s)", serviceID, len(fids), strings.Join(fids, ", "))
		*warningTexts = append(*warningTexts, msg)
		s.logger.Warn("function version floor rejected registration", "agent_id", req.AgentId, "service_id", serviceID, "functions", fids)
		s.registry.UpsertRegistrationWarning(ctx, reg.FunctionRegistrationWarning{
			GameID:  req.GameId,
			Env:     req.Env,
			AgentID: req.AgentId,
			Code:    reg.WarningCodeFunctionVersionBelowMinimum,
			Message: msg,
		})
	}
	for _, fids := range rejectedByProc {
		for _, fid := range fids {
			// 交叉提供保护：任一放行进程（或无 SDK 自报进程）也声明该函数
			// 时不剔——函数仍可由放行方提供，拒绝不应造成可用性缺口。
			if !allowedDeclared[fid] {
				outcome.rejected[fid] = true
			}
		}
	}
	return outcome
}

// bareFunctionIDs 把 "fid(<min)" 展示串还原为裸函数 id（拒绝集与
// allowedDeclared 的 key 都是裸 id）。
func bareFunctionIDs(annotated []string) []string {
	out := make([]string, 0, len(annotated))
	for _, item := range annotated {
		if i := strings.Index(item, "(<"); i > 0 {
			out = append(out, item[:i])
			continue
		}
		out = append(out, item)
	}
	return out
}

func (s *ControlService) handleRegisterCapabilitiesRequest(ctx context.Context, req *agentv1.RegisterCapabilitiesRequest) (*agentv1.RegisterCapabilitiesResponse, error) {
	if s.upstream != nil {
		return s.upstream.HandleRegisterCapabilities(ctx, req)
	}

	if req.Provider == nil || req.Provider.Id == "" {
		return nil, fmt.Errorf("provider metadata is required")
	}

	manifestData, err := decompressManifest(req.ManifestJsonGz)
	if err != nil {
		return nil, fmt.Errorf("invalid manifest (gzip): %w", err)
	}

	providerCaps := reg.OpenAPIProviderCaps{
		ID:         req.Provider.Id,
		Version:    req.Provider.Version,
		Lang:       req.Provider.Lang,
		SDK:        req.Provider.Sdk,
		OpenAPIDoc: manifestData,
		UpdatedAt:  time.Now(),
	}
	// Upsert 仅在 ID 为空时报错；ID 已由上游 provider 注册校验保证非空。
	_ = s.registry.UpsertOpenAPIProvider(providerCaps)

	s.logger.Info("Provider capabilities registered", "provider_id", req.Provider.Id)

	return &agentv1.RegisterCapabilitiesResponse{}, nil
}

// pruneMetricsOnce 执行一轮指标/缓存修剪（供循环调用与直接测试）。
func (s *ControlService) pruneMetricsOnce() {
	s.metricsStore.Prune(time.Hour)
	s.systemInfoCache.Prune(time.Hour)
}

// backgroundLoopInterval 是指标修剪/会话清理两个后台循环的定时间隔；
// 抽为包级变量仅为测试注入短间隔覆盖 ticker 分支，默认 5 分钟与历史行为一致。
var backgroundLoopInterval = 5 * time.Minute

func (s *ControlService) pruneOldMetrics() {
	ticker := time.NewTicker(backgroundLoopInterval)
	defer ticker.Stop()
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.pruneMetricsOnce()
		}
	}
}

// runSessionCleanup 执行一轮过期会话清理（供循环调用与直接测试）。
func (s *ControlService) runSessionCleanup() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	deleted, err := s.agentSessionLoader.DeleteExpired(ctx)
	if err != nil {
		s.logger.Error("failed to delete expired sessions", "error", err)
	} else if deleted > 0 {
		s.logger.Info("deleted expired sessions from database", "count", deleted)
	}
}

func (s *ControlService) cleanupLoop() {
	ticker := time.NewTicker(backgroundLoopInterval)
	defer ticker.Stop()
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.runSessionCleanup()
		}
	}
}

// LoadAgentSessions loads active agent sessions from the database into memory.
// 多实例时按共享归属表活跃全集过滤——断连 agent 的残留快照行不再作为
// 僵尸复活（归属表无行 = 无任何实例持有连接）。
func (s *ControlService) LoadAgentSessions() error {
	if s.agentSessionLoader == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var keep func(*reg.AgentSession) bool
	s.mu.RLock()
	directory := s.activeAgentDirectory
	s.mu.RUnlock()
	if directory != nil {
		ids, err := directory.ActiveAgentIDs(ctx)
		if err != nil {
			// 查询失败降级全量恢复：启动路径不放大归属表故障。
			s.logger.Warn("active agent directory unavailable, restoring all snapshots", "error", err)
		} else {
			alive := make(map[string]bool, len(ids))
			for _, id := range ids {
				alive[id] = true
			}
			keep = func(sess *reg.AgentSession) bool { return alive[sess.AgentID] }
		}
	}
	if err := s.registry.LoadFromDBFiltered(ctx, s.agentSessionLoader, keep); err != nil {
		return fmt.Errorf("failed to load agent sessions: %w", err)
	}
	return nil
}

// ---- validation helpers ----

func validateAndNormalizeFunctions(items []*agentv1.FunctionDescriptor) ([]*agentv1.FunctionDescriptor, []registerWarning) {
	if len(items) == 0 {
		return nil, nil
	}
	byID := make(map[string]*agentv1.FunctionDescriptor, len(items))
	warnings := make([]registerWarning, 0)
	for idx, f := range items {
		if f == nil {
			warnings = append(warnings, registerWarning{Code: "nil_function", Message: fmt.Sprintf("functions[%d] is nil and skipped", idx)})
			continue
		}
		fid := strings.TrimSpace(strings.ToLower(f.GetId()))
		if fid == "" {
			warnings = append(warnings, registerWarning{Code: "empty_function_id", Message: fmt.Sprintf("functions[%d] has empty function_id and skipped", idx)})
			continue
		}
		if !functionIDPattern.MatchString(fid) {
			warnings = append(warnings, registerWarning{Code: "invalid_function_id", FunctionID: fid, Version: f.GetVersion(), Message: fmt.Sprintf("function_id=%s invalid format (expected lowercase dotted id) and skipped", fid)})
			continue
		}
		version := strings.TrimSpace(f.GetVersion())
		if !isValidSemver(version) {
			warnings = append(warnings, registerWarning{Code: "invalid_version", FunctionID: fid, Version: version, Message: fmt.Sprintf("function_id=%s version=%s invalid semver and skipped", fid, version)})
			continue
		}
		if forbiddenKey, ok := descriptorPresentationField(f); ok {
			warnings = append(warnings, registerWarning{Code: "function_presentation_field_not_allowed", FunctionID: fid, Version: version, Message: fmt.Sprintf("function_id=%s registers presentation field %q; function registration only accepts executable capability contract and is skipped", fid, forbiddenKey)})
			continue
		}
		f.Id = fid
		if prev, ok := byID[fid]; ok {
			compare := compareSemver(f.GetVersion(), prev.GetVersion())
			if compare > 0 {
				warnings = append(warnings, registerWarning{Code: "duplicate_function_id", FunctionID: fid, Version: f.GetVersion(), Message: fmt.Sprintf("duplicate function_id=%s detected; keep higher version %s over %s", fid, f.GetVersion(), prev.GetVersion())})
				byID[fid] = f
			} else {
				warnings = append(warnings, registerWarning{Code: "duplicate_function_id", FunctionID: fid, Version: prev.GetVersion(), Message: fmt.Sprintf("duplicate function_id=%s detected; keep higher version %s over %s", fid, prev.GetVersion(), f.GetVersion())})
			}
			continue
		}
		byID[fid] = f
	}
	out := make([]*agentv1.FunctionDescriptor, 0, len(byID))
	for _, f := range byID {
		out = append(out, f)
	}
	return out, warnings
}

func descriptorPresentationField(f *agentv1.FunctionDescriptor) (string, bool) {
	if f == nil {
		return "", false
	}
	// FunctionDescriptor has no presentation fields. Reject unknown protobuf
	// fields at the registration boundary instead of silently carrying an old
	// page/UI extension into FunctionContract.
	if unknown := f.ProtoReflect().GetUnknown(); len(unknown) > 0 {
		return "unknown_proto_fields", true
	}
	// Schemas belong to the executable capability contract, but they must not
	// carry dashboard presentation extensions (x-menu, x-table-columns,
	// formily, ...). Reject the whole descriptor instead of silently
	// persisting a smuggled UI hint.
	if field, _, ok := registrationguard.ScanJSON(f.GetInputSchema()); ok {
		return "inputSchema." + field, true
	}
	if field, _, ok := registrationguard.ScanJSON(f.GetOutputSchema()); ok {
		return "outputSchema." + field, true
	}
	return "", false
}

func isValidSemver(v string) bool {
	return versionutil.IsValid(v)
}

func compareSemver(a, b string) int {
	parse := func(raw string) [3]int {
		s := strings.TrimPrefix(strings.TrimSpace(raw), "v")
		parts := strings.SplitN(s, "-", 2)
		num := parts[0]
		p := strings.Split(num, ".")
		out := [3]int{0, 0, 0}
		for i := 0; i < len(p) && i < 3; i++ {
			n, err := strconv.Atoi(p[i])
			if err != nil {
				return out
			}
			out[i] = n
		}
		return out
	}
	va := parse(a)
	vb := parse(b)
	for i := 0; i < 3; i++ {
		if va[i] > vb[i] {
			return 1
		}
		if va[i] < vb[i] {
			return -1
		}
	}
	return 0
}

func decompressManifest(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("manifest data is empty")
	}
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer func() { _ = reader.Close() }()
	return io.ReadAll(reader)
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

// ControlHandler wraps ControlService to implement the Handler interface.
type ControlHandler struct {
	service *ControlService
}

// NewControlHandler creates a new control handler.
func NewControlHandler(service *ControlService) *ControlHandler {
	return &ControlHandler{service: service}
}

func (h *ControlHandler) HandleRegister(ctx context.Context, req *agentv1.RegisterRequest) (*agentv1.RegisterResponse, error) {
	return h.service.handleRegisterRequest(ctx, req, "")
}

func (h *ControlHandler) HandleHeartbeat(ctx context.Context, req *agentv1.HeartbeatRequest) (*agentv1.HeartbeatResponse, error) {
	return h.service.handleHeartbeatRequest(ctx, req)
}

func (h *ControlHandler) HandleRegisterCapabilities(ctx context.Context, req *agentv1.RegisterCapabilitiesRequest) (*agentv1.RegisterCapabilitiesResponse, error) {
	return h.service.handleRegisterCapabilitiesRequest(ctx, req)
}
