package registry

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/db/dbctx"
	"github.com/cuihairu/croupier/internal/function/registrationguard"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// FunctionMeta describes a function capability on an agent.
type FunctionMeta struct {
	Enabled bool
	Version string

	Tags        []string
	Summary     string
	Description string
	OperationID string
	Deprecated  bool

	InputSchema  string
	OutputSchema string

	Resource          string
	Operation         string
	Capability        string
	Execution         string
	ApprovalRequired  bool
	ApprovalPolicyKey string
	Risk              string
	Permission        string
}

// ProviderSession represents a single provider registered to an agent (via SDK->Agent local registry).
type ProviderSession struct {
	ProviderID   string
	GameID       string
	Env          string
	Addr         string
	Version      string
	SDKLanguage  string // SDK 语言（go, java, python, cpp, csharp, custom）
	SDKVersion   string // SDK 版本
	SDKName      string // SDK 显示名（如 croupier-js-sdk），用户可自定义
	LastSeenUnix int64
	FunctionIDs  []string
	OpenAPIDoc   json.RawMessage
}

// ProviderSessionSnapshot 是在线 Provider 的只读快照（含所属 agent/scope），
// 供 SDK 版本分布等观测 API 消费（F：sdk-stats）。
type ProviderSessionSnapshot struct {
	ProviderID   string   `json:"providerId"`
	AgentID      string   `json:"agentId"`
	GameID       string   `json:"gameId"`
	Env          string   `json:"env"`
	Addr         string   `json:"addr"`
	Version      string   `json:"version"`
	SDKLanguage  string   `json:"sdkLanguage"`
	SDKVersion   string   `json:"sdkVersion"`
	SDKName      string   `json:"sdkName"`
	LastSeenUnix int64    `json:"lastSeenUnix"`
	FunctionIDs  []string `json:"functionIds"`
}

// ProviderSessionSnapshots 返回全部在线 Provider 会话的快照（深拷贝）。
func (s *Store) ProviderSessionSnapshots() []ProviderSessionSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]ProviderSessionSnapshot, 0)
	for _, sess := range s.agents {
		if sess == nil {
			continue
		}
		for _, p := range sess.Providers {
			snapshot := ProviderSessionSnapshot{
				ProviderID:   p.ProviderID,
				AgentID:      sess.AgentID,
				GameID:       sess.GameID,
				Env:          sess.Env,
				Addr:         p.Addr,
				Version:      p.Version,
				SDKLanguage:  p.SDKLanguage,
				SDKVersion:   p.SDKVersion,
				SDKName:      p.SDKName,
				LastSeenUnix: p.LastSeenUnix,
			}
			if len(p.FunctionIDs) > 0 {
				snapshot.FunctionIDs = append([]string(nil), p.FunctionIDs...)
			}
			out = append(out, snapshot)
		}
	}
	return out
}

// AgentSession represents a registered agent instance in the registry.
// RPCAddr is retained only as a compatibility mirror while the runtime moves
// to session-first routing.
type AgentSession struct {
	AgentID   string
	GameID    string
	Env       string
	Addr      string
	Version   string
	Region    string
	Zone      string
	Labels    map[string]string
	Functions map[string]FunctionMeta
	Providers []ProviderSession
	ExpireAt  time.Time
	LastSeen  time.Time // 最后活跃时间
}

// Store keeps lightweight agent registry state in-memory.
type Store struct {
	mu sync.RWMutex
	// registrationMu serializes local registration projection changes. It keeps
	// snapshot classification and cross-database compensation based on one
	// coherent in-memory agent view.
	registrationMu sync.Mutex
	agents         map[string]*AgentSession // agent_id -> session
	// OpenAPI operations storage (replaces legacy manifest)
	openapiOperations    map[string]*openapi3.Operation  // function_id -> OpenAPI operation
	openapiProviders     map[string]*OpenAPIProviderCaps // provider_id -> OpenAPI caps
	registrationWarnings map[string]*FunctionRegistrationWarning
	// sdkHwmMu guards sdkHwm (in-memory SDK version high watermarks for
	// DB-less registries; DB-backed stores read/write the table directly).
	sdkHwmMu sync.Mutex
	sdkHwm   map[string]string
	// fnFloorMu guards fnFloor (in-memory 函数级最低函数版本门槛，DB-less
	// registry 的退化存储；DB-backed 直接读写 function_version_floors 表)。
	fnFloorMu sync.Mutex
	fnFloor   map[string]string
	// Optional database for dual-write persistence
	db *gorm.DB
	// scopeContext resolves the DB/scope context used by game-scoped
	// contract rebuilds triggered outside an HTTP request.
	scopeContext func(gameID, env string) context.Context
	// Contract service for FunctionContract persistence (optional)
	contractService contractMaterializer
}

// contractMaterializer is the registration-side projection boundary. It keeps
// registry snapshots independent from persistence and page-generation details.
type contractMaterializer interface {
	RebuildContractFromFunctionMeta(ctx context.Context, gameID, env, source string, meta spec.FunctionContractInput) error
	RemoveFunctionContract(ctx context.Context, gameID, env, functionID string) (resourceKey string, err error)
	RebuildResourceCapability(ctx context.Context, gameID, env, resourceKey string) error
	RebuildProposalsForResource(ctx context.Context, gameID, env, resourceKey string) error
	RebuildProposalForFunction(ctx context.Context, gameID, env, functionID string) error
	// RegenerateContractTemplates 在注册写入提交后重建 scope 的内置组件
	// 模板（T2 收口）。模板表经全局连接写，注册事务进行中调用会在文件型
	// sqlite 下与事务写锁互等待自死锁——只允许在事务/写入成功之后调用。
	RegenerateContractTemplates(ctx context.Context, gameID, env string) error
}

type FunctionRegistrationWarning struct {
	Key        string
	GameID     string
	Env        string
	AgentID    string
	FunctionID string
	Version    string
	Code       string
	Message    string
	Count      int
	FirstSeen  time.Time
	LastSeen   time.Time
	Read       bool // F：已读状态（删除/已读 UI）
}

// 注册提交后衍生数据重建失败的告警 Code（与函数注册告警同一通道/同一
// 生命周期——内存存储，重启即失；重建成功不自动清除，人工删除兜底）。
const (
	// WarningCodeProposalRebuildFailed 注册写入提交后页面提案重建失败。
	// 提案为衍生数据可由已提交契约全量重算，失败不回滚注册
	// （POST /pages/proposals/rebuild 是手动兜底）。
	WarningCodeProposalRebuildFailed = "proposal_rebuild_failed"
	// WarningCodeTemplateRegenFailed 注册写入提交后内置组件模板重建失败。
	// 手动 POST /component-templates/regenerate 是兜底。
	WarningCodeTemplateRegenFailed = "template_regen_failed"
	// WarningCodeSDKVersionBehind provider 自报 SDK 版本低于 (game_id, env,
	// sdk_language) 高水位但在容忍窗内（同 major 且 minor 差 <2）：函数注册
	// 放行但提示升级。追赶后不自动清除（多语言 provider 共用 agent+code
	// 维度，无法按语言精确清理），人工删除兜底。
	WarningCodeSDKVersionBehind = "sdk_version_behind"
	// WarningCodeSDKVersionFloorRejected provider 自报 SDK 版本落后高水位
	// 超出容忍窗（低 2 个及以上 minor，或 1 个及以上 major）：该 provider
	// 的函数注册被拒（函数不进会话/不物化契约），连接保持。
	WarningCodeSDKVersionFloorRejected = "sdk_version_floor_rejected"
	// WarningCodeSDKVersionBelowMinimum provider 自报 SDK 版本低于配置的
	// 最低版本（registry.sdkVersionMinimums）：该 provider 独占声明的
	// 函数不注册（只产生告警），连接保持。配置门槛是绝对下限，优先于
	// 滑动高水位判定。
	WarningCodeSDKVersionBelowMinimum = "sdk_version_below_minimum"
	// WarningCodeFunctionVersionBelowMinimum 函数描述符自报版本低于函数级
	// 配置的最低版本（function_version_floors 表，UI 按函数设置）：旧版
	// 函数声明不随本次注册物化（只产生告警）——防契约回退。
	WarningCodeFunctionVersionBelowMinimum = "function_version_below_minimum"
)

type RegistrationWarningFilter struct {
	GameID     string
	Env        string
	AgentID    string
	FunctionID string
	Code       string
	Status     string
	Limit      int
}

// functionSnapshotDiff is the deterministic difference between two complete
// registration snapshots. It is intentionally internal to the registry
// projection path; callers consume the resulting materialized state.
type functionSnapshotDiff struct {
	Added     []string
	Changed   []string
	Removed   []string
	Resources []string
}

// OpenAPIProviderCaps represents provider capabilities in OpenAPI format.
type OpenAPIProviderCaps struct {
	ID        string
	Version   string
	Lang      string
	SDK       string
	UpdatedAt time.Time
	// OpenAPI 3.0.3 document as JSON
	OpenAPIDoc []byte
}

func NewStore() *Store {
	return &Store{
		agents:               map[string]*AgentSession{},
		openapiOperations:    make(map[string]*openapi3.Operation),
		openapiProviders:     make(map[string]*OpenAPIProviderCaps),
		registrationWarnings: make(map[string]*FunctionRegistrationWarning),
		sdkHwm:               map[string]string{},
		fnFloor:              map[string]string{},
		db:                   nil,
		scopeContext:         defaultScopeContext,
	}
}

// NewStoreWithDB creates a new Store with database dual-write enabled.
func NewStoreWithDB(db *gorm.DB) *Store {
	return &Store{
		agents:               map[string]*AgentSession{},
		openapiOperations:    make(map[string]*openapi3.Operation),
		openapiProviders:     make(map[string]*OpenAPIProviderCaps),
		registrationWarnings: make(map[string]*FunctionRegistrationWarning),
		sdkHwm:               map[string]string{},
		fnFloor:              map[string]string{},
		db:                   db,
		scopeContext:         defaultScopeContext,
	}
}

// SetContractService sets the contract service for FunctionContract persistence.
func (s *Store) SetContractService(svc contractMaterializer) {
	s.contractService = svc
}

// SessionPersistenceEnabled reports whether the registry itself persists
// AgentSession records. ControlService uses this to avoid writing the same
// session twice when it also receives an AgentSessionLoader.
func (s *Store) SessionPersistenceEnabled() bool {
	return s != nil && s.db != nil
}

// SetScopeContextResolver configures how background registration rebuilds
// acquire the correct game-scoped database context.
func (s *Store) SetScopeContextResolver(resolve func(gameID, env string) context.Context) {
	if resolve == nil {
		s.scopeContext = defaultScopeContext
		return
	}
	s.scopeContext = resolve
}

// Mu exposes the lock for read/update operations when callers need batch views.
func (s *Store) Mu() *sync.RWMutex { return &s.mu }

// AgentsUnsafe returns the internal agents map without copying. Callers MUST hold Mu().RLock/Lock.
func (s *Store) AgentsUnsafe() map[string]*AgentSession { return s.agents }

// UpsertAgent inserts or updates an agent session by AgentID.
// Contract and capability materialization is part of registration and must
// succeed before the session becomes visible in the runtime registry. Page
// proposals and component templates are derived data rebuilt after the
// registration write commits; their failure is downgraded to a registration
// warning (proposal_rebuild_failed / template_regen_failed) and never rolls
// back the registration itself.
func (s *Store) UpsertAgent(a *AgentSession) error {
	if a == nil || a.AgentID == "" {
		return nil
	}
	if err := validateAgentFunctionContracts(a.Functions); err != nil {
		return err
	}
	s.registrationMu.Lock()
	defer s.registrationMu.Unlock()

	previousSession := s.previousAgentSession(a.AgentID)
	previous := functionSnapshot(previousSession)
	diff := classifyFunctionSnapshot(previous, a.Functions)
	scopeCtx := s.rebuildContext(a.GameID, a.Env)

	materialize := s.materializeAgent

	// A single-database deployment can use one database transaction. A
	// database-per-game deployment uses a durable operation record plus
	// compensation: the game projection commits first, then the meta session
	// and operation status commit together. Any meta failure restores the
	// previous game projection immediately or leaves an explicit recovery row.
	if s.db != nil && dbctx.Get(scopeCtx) == nil {
		var plan *registrationRebuildPlan
		if err := s.db.Transaction(func(tx *gorm.DB) error {
			txCtx := dbctx.WithDB(scopeCtx, tx)
			p, err := materialize(txCtx, a, diff)
			if err != nil {
				return err
			}
			plan = p
			if err := s.writeToDB(txCtx, a); err != nil {
				return fmt.Errorf("write agent session to database: %w", err)
			}
			return nil
		}); err != nil {
			return err
		}
		s.rebuildProposalsAfterRegistration(scopeCtx, a, plan)
		s.regenTemplatesAfterRegistration(scopeCtx, a, diff)
	} else if s.db != nil && dbctx.Get(scopeCtx) != nil && s.contractService != nil && a.Functions != nil {
		operation, err := s.prepareRegistrationOperation(a, previousSession)
		if err != nil {
			return err
		}
		plan, err := s.materializeScopedTransaction(scopeCtx, a, materialize, diff)
		if err != nil {
			s.markRegistrationOperation(operation.OperationID, "aborted", err)
			return err
		}
		metaErr := s.db.Transaction(func(tx *gorm.DB) error {
			txCtx := dbctx.WithDB(context.Background(), tx)
			if err := s.writeToDB(txCtx, a); err != nil {
				return fmt.Errorf("write agent session to database: %w", err)
			}
			return s.markRegistrationOperationWithDB(tx, operation.OperationID, "committed", "")
		})
		if metaErr != nil {
			previousForRestore := previousSession
			if previousForRestore == nil {
				previousForRestore = &AgentSession{
					AgentID:   a.AgentID,
					GameID:    a.GameID,
					Env:       a.Env,
					Functions: map[string]FunctionMeta{},
				}
			}
			reverseDiff := classifyFunctionSnapshot(a.Functions, previousForRestore.Functions)
			// 补偿只需恢复权威状态（契约+能力）：提案不在注册事务内，
			// meta 提交失败时提交后重建尚未发生，无需补偿。
			if _, compensationErr := s.materializeScopedTransaction(scopeCtx, previousForRestore, materialize, reverseDiff); compensationErr != nil {
				s.markRegistrationOperation(operation.OperationID, "compensation_required", compensationErr)
				return fmt.Errorf("write agent session to database: %w; registration compensation failed: %w", metaErr, compensationErr)
			}
			s.markRegistrationOperation(operation.OperationID, "compensated", metaErr)
			return metaErr
		}
		s.rebuildProposalsAfterRegistration(scopeCtx, a, plan)
		s.regenTemplatesAfterRegistration(scopeCtx, a, diff)
	} else {
		plan, err := materialize(scopeCtx, a, diff)
		if err != nil {
			return err
		}
		if s.db != nil {
			if err := s.writeToDB(context.Background(), a); err != nil {
				return fmt.Errorf("write agent session to database: %w", err)
			}
		}
		s.rebuildProposalsAfterRegistration(scopeCtx, a, plan)
		s.regenTemplatesAfterRegistration(scopeCtx, a, diff)
	}

	// Always write to memory (primary store)
	s.mu.Lock()
	defer s.mu.Unlock()
	cur := s.agents[a.AgentID]
	if cur == nil {
		s.agents[a.AgentID] = a
		return nil
	}
	// Merge minimal fields. RPCAddr remains a compatibility mirror and should
	// not be treated as the primary runtime route.
	cur.GameID, cur.Env, cur.Addr, cur.Version = a.GameID, a.Env, a.Addr, a.Version
	cur.Region, cur.Zone = a.Region, a.Zone
	// merge labels: new labels replace old ones
	if a.Labels != nil {
		if cur.Labels == nil {
			cur.Labels = make(map[string]string)
		}
		for k, v := range a.Labels {
			cur.Labels[k] = v
		}
	}
	if a.Functions != nil {
		cur.Functions = a.Functions
	}
	if a.Providers != nil {
		cur.Providers = a.Providers
	}
	cur.ExpireAt = a.ExpireAt
	cur.LastSeen = a.LastSeen
	return nil
}

// regenTemplatesAfterRegistration 在注册写入（事务或直写）成功后单次收口
// 组件模板重建（T2）。快照无 Added/Changed/Removed 时不触发——心跳/重连
// 风暴下函数集未变即不空转（对齐原 digest 门控语义）。失败仅告警（写
// registrationWarnings，Code=template_regen_failed，UI 可见），不影响
// 已提交的注册结果（手动 regenerate 仍是兜底）。
func (s *Store) regenTemplatesAfterRegistration(ctx context.Context, session *AgentSession, diff functionSnapshotDiff) {
	if s.contractService == nil || session == nil {
		return
	}
	if len(diff.Added) == 0 && len(diff.Changed) == 0 && len(diff.Removed) == 0 {
		return
	}
	if err := s.contractService.RegenerateContractTemplates(ctx, session.GameID, session.Env); err != nil {
		slog.Default().Warn("auto regenerate component templates after agent registration failed (manual regenerate remains as fallback)",
			"agent_id", session.AgentID,
			"game_id", session.GameID,
			"env", session.Env,
			"error", err)
		s.UpsertRegistrationWarning(ctx, FunctionRegistrationWarning{
			GameID:  session.GameID,
			Env:     session.Env,
			AgentID: session.AgentID,
			Code:    WarningCodeTemplateRegenFailed,
			Message: err.Error(),
		})
	}
}

// registrationRebuildPlan 注册事务内收集的衍生重建计划（提交后执行）。
// 快照无变化（心跳/重连）时集合为空，提交后重建自然跳过。
type registrationRebuildPlan struct {
	Resources           []string
	StandaloneFunctions []string
}

func (s *Store) materializeScopedTransaction(
	scopeCtx context.Context,
	session *AgentSession,
	materialize func(context.Context, *AgentSession, functionSnapshotDiff) (*registrationRebuildPlan, error),
	diff functionSnapshotDiff,
) (*registrationRebuildPlan, error) {
	scopeDB := dbctx.Resolve(scopeCtx, s.db)
	if scopeDB == nil {
		return nil, fmt.Errorf("registration projection database is not initialized")
	}
	var plan *registrationRebuildPlan
	if err := scopeDB.Transaction(func(tx *gorm.DB) error {
		p, err := materialize(dbctx.WithDB(scopeCtx, tx), session, diff)
		if err != nil {
			return err
		}
		plan = p
		return nil
	}); err != nil {
		return nil, err
	}
	return plan, nil
}

// materializeAgent 物化注册的权威状态（契约 + 资源能力聚合），并收集
// 页面提案的衍生重建计划。提案重建不在本事务内执行——见
// rebuildProposalsAfterRegistration。
func (s *Store) materializeAgent(ctx context.Context, session *AgentSession, sessionDiff functionSnapshotDiff) (*registrationRebuildPlan, error) {
	if s.contractService == nil || session == nil || session.Functions == nil {
		return nil, nil
	}
	var rebuildErrors []error
	resources := stringSet(sessionDiff.Resources)
	standaloneFunctions := make(map[string]bool)
	for _, functionID := range sortedFunctionIDs(session.Functions) {
		meta := session.Functions[functionID]
		if err := s.rebuildFunctionContract(ctx, session.GameID, session.Env, functionID, meta, resources, standaloneFunctions); err != nil {
			rebuildErrors = append(rebuildErrors, err)
		}
	}
	for _, functionID := range sessionDiff.Removed {
		if meta, ok := s.survivingFunctionMeta(session.AgentID, session.GameID, session.Env, functionID); ok {
			if err := s.rebuildFunctionContract(ctx, session.GameID, session.Env, functionID, meta, resources, standaloneFunctions); err != nil {
				rebuildErrors = append(rebuildErrors, err)
			}
			continue
		}
		resourceKey, err := s.contractService.RemoveFunctionContract(ctx, session.GameID, session.Env, functionID)
		if err != nil {
			rebuildErrors = append(rebuildErrors, fmt.Errorf("remove function contract %s: %w", functionID, err))
			continue
		}
		if resourceKey != "" {
			resources[resourceKey] = true
		}
	}
	for _, resource := range sortedStringSet(resources) {
		if err := s.contractService.RebuildResourceCapability(ctx, session.GameID, session.Env, resource); err != nil {
			rebuildErrors = append(rebuildErrors, fmt.Errorf("rebuild resource capability %s: %w", resource, err))
			continue
		}
	}
	if len(rebuildErrors) > 0 {
		return nil, fmt.Errorf("agent registration contract rebuild failed: %w", errors.Join(rebuildErrors...))
	}
	return &registrationRebuildPlan{
		Resources:           sortedStringSet(resources),
		StandaloneFunctions: sortedStringSet(standaloneFunctions),
	}, nil
}

// rebuildProposalsAfterRegistration 在注册写入（事务或直写）成功后重建
// 页面提案（衍生数据，可由已提交契约全量重算）。失败仅告警
// （Code=proposal_rebuild_failed，UI 可见），不影响已提交的注册结果——
// 手动 POST /pages/proposals/rebuild 仍是兜底。
func (s *Store) rebuildProposalsAfterRegistration(ctx context.Context, session *AgentSession, plan *registrationRebuildPlan) {
	if s.contractService == nil || session == nil || plan == nil {
		return
	}
	for _, resource := range plan.Resources {
		if err := s.contractService.RebuildProposalsForResource(ctx, session.GameID, session.Env, resource); err != nil {
			slog.Default().Warn("rebuild page proposals after agent registration failed (manual proposals rebuild remains as fallback)",
				"agent_id", session.AgentID,
				"game_id", session.GameID,
				"env", session.Env,
				"resource", resource,
				"error", err)
			s.UpsertRegistrationWarning(ctx, FunctionRegistrationWarning{
				GameID:  session.GameID,
				Env:     session.Env,
				AgentID: session.AgentID,
				Code:    WarningCodeProposalRebuildFailed,
				Message: fmt.Sprintf("resource %s: %s", resource, err.Error()),
			})
		}
	}
	for _, functionID := range plan.StandaloneFunctions {
		if err := s.contractService.RebuildProposalForFunction(ctx, session.GameID, session.Env, functionID); err != nil {
			slog.Default().Warn("rebuild standalone page proposal after agent registration failed (manual proposals rebuild remains as fallback)",
				"agent_id", session.AgentID,
				"game_id", session.GameID,
				"env", session.Env,
				"function_id", functionID,
				"error", err)
			s.UpsertRegistrationWarning(ctx, FunctionRegistrationWarning{
				GameID:  session.GameID,
				Env:     session.Env,
				AgentID: session.AgentID,
				Code:    WarningCodeProposalRebuildFailed,
				Message: fmt.Sprintf("function %s: %s", functionID, err.Error()),
			})
		}
	}
}

func (s *Store) previousAgentSession(agentID string) *AgentSession {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return cloneAgentSession(s.agents[agentID])
}

func functionSnapshot(session *AgentSession) map[string]FunctionMeta {
	if session == nil || session.Functions == nil {
		return nil
	}
	functions := make(map[string]FunctionMeta, len(session.Functions))
	for functionID, meta := range session.Functions {
		functions[functionID] = meta
	}
	return functions
}

func cloneAgentSession(session *AgentSession) *AgentSession {
	if session == nil {
		return nil
	}
	encoded, err := json.Marshal(session)
	if err != nil {
		return nil
	}
	var clone AgentSession
	// encoded 是 json.Marshal 的产物，对同一结构再 Unmarshal 恒成功，
	// err 分支为死代码已删（Marshal 的 err 路径因 ProviderSession.OpenAPIDoc
	// 为 json.RawMessage 可失败，可达需保留）。
	_ = json.Unmarshal(encoded, &clone)
	return &clone
}

func (s *Store) prepareRegistrationOperation(target, previous *AgentSession) (*AgentRegistrationOperationDB, error) {
	targetJSON, err := json.Marshal(target)
	if err != nil {
		return nil, fmt.Errorf("marshal target registration session: %w", err)
	}
	previousJSON, err := json.Marshal(previous)
	if err != nil {
		return nil, fmt.Errorf("marshal previous registration session: %w", err)
	}
	operation := &AgentRegistrationOperationDB{
		OperationID:     uuid.NewString(),
		AgentID:         target.AgentID,
		GameID:          target.GameID,
		Env:             target.Env,
		PreviousSession: string(previousJSON),
		TargetSession:   string(targetJSON),
		Status:          "pending",
	}
	if err := s.db.Create(operation).Error; err != nil {
		return nil, fmt.Errorf("create registration recovery operation: %w", err)
	}
	return operation, nil
}

func (s *Store) markRegistrationOperation(operationID, status string, cause error) {
	if s == nil || s.db == nil || strings.TrimSpace(operationID) == "" {
		return
	}
	lastError := ""
	if cause != nil {
		lastError = cause.Error()
	}
	if err := s.markRegistrationOperationWithDB(s.db, operationID, status, lastError); err != nil {
		slog.Default().Error("failed to update registration recovery operation", "operation_id", operationID, "status", status, "error", err)
	}
}

func (s *Store) markRegistrationOperationWithDB(db *gorm.DB, operationID, status, lastError string) error {
	if db == nil {
		return fmt.Errorf("registration recovery database is not initialized")
	}
	return db.Model(&AgentRegistrationOperationDB{}).
		Where("operation_id = ?", operationID).
		Updates(map[string]interface{}{"status": status, "last_error": lastError}).Error
}

func (s *Store) rebuildFunctionContract(
	ctx context.Context,
	gameID string,
	env string,
	functionID string,
	meta FunctionMeta,
	resources map[string]bool,
	standaloneFunctions map[string]bool,
) error {
	input := spec.FunctionContractInput{
		ID:                functionID,
		Version:           meta.Version,
		Enabled:           meta.Enabled,
		Deprecated:        meta.Deprecated,
		Summary:           meta.Summary,
		Description:       meta.Description,
		InputSchema:       meta.InputSchema,
		OutputSchema:      meta.OutputSchema,
		Resource:          meta.Resource,
		Operation:         meta.Operation,
		Capability:        meta.Capability,
		Execution:         meta.Execution,
		ApprovalRequired:  meta.ApprovalRequired,
		ApprovalPolicyKey: meta.ApprovalPolicyKey,
		Risk:              meta.Risk,
		Permission:        meta.Permission,
		Tags:              meta.Tags,
	}
	if err := s.contractService.RebuildContractFromFunctionMeta(ctx, gameID, env, "sdk", input); err != nil {
		return fmt.Errorf("rebuild function contract %s: %w", functionID, err)
	}
	if meta.Resource != "" {
		resources[meta.Resource] = true
	} else {
		standaloneFunctions[functionID] = true
	}
	return nil
}

func (s *Store) previousFunctions(agentID string) map[string]FunctionMeta {
	s.mu.RLock()
	defer s.mu.RUnlock()
	current := s.agents[agentID]
	if current == nil || current.Functions == nil {
		return nil
	}
	functions := make(map[string]FunctionMeta, len(current.Functions))
	for functionID, meta := range current.Functions {
		functions[functionID] = meta
	}
	return functions
}

// DeleteRegistrationWarning 删除单条注册警告（key 为去重主键）。
func (s *Store) DeleteRegistrationWarning(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.registrationWarnings[key]; !ok {
		return false
	}
	delete(s.registrationWarnings, key)
	return true
}

// DeleteAllRegistrationWarnings 清空注册警告（「全部清除」UI）。
// 返回清除条数。
func (s *Store) DeleteAllRegistrationWarnings() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := len(s.registrationWarnings)
	s.registrationWarnings = map[string]*FunctionRegistrationWarning{}
	return n
}

// MarkRegistrationWarningRead 标记单条警告已读。
func (s *Store) MarkRegistrationWarningRead(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.registrationWarnings[key]
	if !ok || item == nil {
		return false
	}
	item.Read = true
	return true
}

// MarkAllRegistrationWarningsRead 全部标记已读，返回条数。
func (s *Store) MarkAllRegistrationWarningsRead() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, item := range s.registrationWarnings {
		if item != nil && !item.Read {
			item.Read = true
			n++
		}
	}
	return n
}

// PreviousFunctionSchema 返回该 agent 当前会话中函数的上一次注册
// input/output schema（同 scope），用于注册时 schema 兼容性 diff。
// 会话不存在 / scope 不一致 / 函数不在会话中均返回 ok=false（视为首次注册）。
func (s *Store) PreviousFunctionSchema(agentID, gameID, env, functionID string) (inputSchema, outputSchema string, ok bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	current := s.agents[agentID]
	if current == nil || current.GameID != gameID || current.Env != env {
		return "", "", false
	}
	meta, exists := current.Functions[functionID]
	if !exists {
		return "", "", false
	}
	return meta.InputSchema, meta.OutputSchema, true
}

func (s *Store) survivingFunctionMeta(agentID, gameID, env, functionID string) (FunctionMeta, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	agentIDs := make([]string, 0, len(s.agents))
	for id := range s.agents {
		agentIDs = append(agentIDs, id)
	}
	sort.Strings(agentIDs)
	for _, id := range agentIDs {
		if id == agentID {
			continue
		}
		session := s.agents[id]
		if session == nil || session.GameID != gameID || session.Env != env {
			continue
		}
		meta, ok := session.Functions[functionID]
		if ok {
			return meta, true
		}
	}
	return FunctionMeta{}, false
}

func sortedFunctionIDs(functions map[string]FunctionMeta) []string {
	ids := make([]string, 0, len(functions))
	for functionID := range functions {
		ids = append(ids, functionID)
	}
	sort.Strings(ids)
	return ids
}

func classifyFunctionSnapshot(previous, current map[string]FunctionMeta) functionSnapshotDiff {
	// nil means the registration omitted the snapshot; it must not delete all
	// existing functions during heartbeat-compatible registrations.
	if current == nil {
		return functionSnapshotDiff{}
	}
	diff := functionSnapshotDiff{}
	resources := map[string]struct{}{}
	for functionID, previousMeta := range previous {
		currentMeta, ok := current[functionID]
		if !ok {
			diff.Removed = append(diff.Removed, functionID)
			if resource := strings.TrimSpace(previousMeta.Resource); resource != "" {
				resources[resource] = struct{}{}
			}
			continue
		}
		if !reflect.DeepEqual(previousMeta, currentMeta) {
			diff.Changed = append(diff.Changed, functionID)
		}
		for _, resource := range []string{previousMeta.Resource, currentMeta.Resource} {
			if resource = strings.TrimSpace(resource); resource != "" {
				resources[resource] = struct{}{}
			}
		}
	}
	for functionID, currentMeta := range current {
		if _, ok := previous[functionID]; !ok {
			diff.Added = append(diff.Added, functionID)
		}
		if resource := strings.TrimSpace(currentMeta.Resource); resource != "" {
			resources[resource] = struct{}{}
		}
	}
	sort.Strings(diff.Added)
	sort.Strings(diff.Changed)
	sort.Strings(diff.Removed)
	for resource := range resources {
		diff.Resources = append(diff.Resources, resource)
	}
	sort.Strings(diff.Resources)
	return diff
}

func sortedStringSet(values map[string]bool) []string {
	items := make([]string, 0, len(values))
	for value := range values {
		if value != "" {
			items = append(items, value)
		}
	}
	sort.Strings(items)
	return items
}

func stringSet(values []string) map[string]bool {
	set := make(map[string]bool, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			set[value] = true
		}
	}
	return set
}

// validateAgentFunctionContracts keeps the registry write boundary aligned
// with the SDK and OpenAPI adapters. A caller that bypasses the control
// handler must not persist a dashboard presentation extension in a schema.
func validateAgentFunctionContracts(functions map[string]FunctionMeta) error {
	functionIDs := make([]string, 0, len(functions))
	for functionID := range functions {
		functionIDs = append(functionIDs, functionID)
	}
	sort.Strings(functionIDs)

	for _, functionID := range functionIDs {
		meta := functions[functionID]
		if violation, ok := registrationguard.FindPresentationViolation(nil, meta.InputSchema, meta.OutputSchema); ok {
			return fmt.Errorf("function %q %s contains forbidden presentation field %q", functionID, violation.Location, violation.Field)
		}
	}
	return nil
}

func (s *Store) rebuildContext(gameID, env string) context.Context {
	if s != nil && s.scopeContext != nil {
		if ctx := s.scopeContext(gameID, env); ctx != nil {
			return ctx
		}
	}
	return defaultScopeContext(gameID, env)
}

func defaultScopeContext(string, string) context.Context {
	return context.Background()
}

// writeToDB persists agent session to database for recovery purposes.
// Only called when db is enabled.
func (s *Store) writeToDB(ctx context.Context, a *AgentSession) error {
	if s.db == nil {
		return nil
	}

	// Convert to database model — initialise JSON columns to valid empty
	// JSON so nil maps/slices don't produce empty strings that PostgreSQL's
	// json type rejects.
	dbSess := struct {
		AgentID   string
		GameID    string
		Env       string
		Version   string
		Addr      string
		Region    string
		Zone      string
		Labels    string `gorm:"type:text"`
		Functions string `gorm:"type:text"`
		Providers string `gorm:"type:text"`
		ExpireAt  time.Time
		LastSeen  time.Time
	}{
		AgentID:   a.AgentID,
		GameID:    a.GameID,
		Env:       a.Env,
		Version:   a.Version,
		Addr:      a.Addr,
		Region:    a.Region,
		Zone:      a.Zone,
		ExpireAt:  a.ExpireAt,
		LastSeen:  a.LastSeen,
		Labels:    "{}",
		Functions: "{}",
		Providers: "[]",
	}

	// Marshal Labels to JSON
	if a.Labels != nil {
		if labelsJSON, err := json.Marshal(a.Labels); err == nil {
			dbSess.Labels = string(labelsJSON)
		}
	}

	// Marshal Functions to JSON
	if a.Functions != nil {
		if functionsJSON, err := json.Marshal(a.Functions); err == nil {
			dbSess.Functions = string(functionsJSON)
		}
	}

	// Marshal Providers to JSON
	if a.Providers != nil {
		if providersJSON, err := json.Marshal(a.Providers); err == nil {
			dbSess.Providers = string(providersJSON)
		}
	}

	// Perform upsert using GORM
	return dbctx.Resolve(ctx, s.db).WithContext(ctx).
		Table("agent_sessions").
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "agent_id"}},
			UpdateAll: true,
		}).
		Create(&dbSess).Error
}

// ========== OpenAPI 3.0.3 Methods (replaces legacy manifest) ==========

// UpsertOpenAPI inserts or updates an OpenAPI operation by function ID.
func (s *Store) UpsertOpenAPI(functionID string, operation *openapi3.Operation) error {
	if functionID == "" || operation == nil {
		return fmt.Errorf("function ID and operation are required")
	}
	clone, err := cloneOpenAPIOperation(operation)
	if err != nil {
		return fmt.Errorf("clone operation failed: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.openapiOperations[functionID] = clone
	return nil
}

// GetOpenAPI retrieves an OpenAPI operation by function ID.
func (s *Store) GetOpenAPI(functionID string) (*openapi3.Operation, error) {
	if functionID == "" {
		return nil, fmt.Errorf("function ID is required")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	op, exists := s.openapiOperations[functionID]
	if !exists {
		return nil, fmt.Errorf("operation not found: %s", functionID)
	}

	return cloneOpenAPIOperation(op)
}

// ListOpenAPIOperations returns all OpenAPI operations.
func (s *Store) ListOpenAPIOperations() map[string]*openapi3.Operation {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]*openapi3.Operation, len(s.openapiOperations))
	for id, op := range s.openapiOperations {
		clone, err := cloneOpenAPIOperation(op)
		if err != nil {
			continue
		}
		result[id] = clone
	}

	return result
}

func cloneOpenAPIOperation(op *openapi3.Operation) (*openapi3.Operation, error) {
	if op == nil {
		return nil, fmt.Errorf("operation is nil")
	}
	b, err := op.MarshalJSON()
	if err != nil {
		return nil, err
	}
	var clone openapi3.Operation
	if err := clone.UnmarshalJSON(b); err != nil {
		return nil, err
	}
	return &clone, nil
}

// UpsertOpenAPIProvider inserts or updates provider capabilities in OpenAPI format.
func (s *Store) UpsertOpenAPIProvider(caps OpenAPIProviderCaps) error {
	if caps.ID == "" {
		return fmt.Errorf("provider ID is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	caps.UpdatedAt = time.Now()
	s.openapiProviders[caps.ID] = &caps
	return nil
}

// GetOpenAPIProvider retrieves OpenAPI provider capabilities by provider ID.
func (s *Store) GetOpenAPIProvider(providerID string) (*OpenAPIProviderCaps, error) {
	if providerID == "" {
		return nil, fmt.Errorf("provider ID is required")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	caps, exists := s.openapiProviders[providerID]
	if !exists {
		return nil, fmt.Errorf("provider not found: %s", providerID)
	}

	return caps, nil
}

// ListOpenAPIProviders returns all OpenAPI providers.
func (s *Store) ListOpenAPIProviders() []*OpenAPIProviderCaps {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*OpenAPIProviderCaps, 0, len(s.openapiProviders))
	for _, caps := range s.openapiProviders {
		result = append(result, caps)
	}

	return result
}

// BuildOpenAPISpec constructs a complete OpenAPI 3.0.3 specification from all registered operations.
func (s *Store) BuildOpenAPISpec() (*openapi3.T, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	doc := &openapi3.T{
		OpenAPI: "3.0.3",
		Info: &openapi3.Info{
			Title:       "Croupier Functions",
			Description: "Auto-generated OpenAPI specification from registered functions",
			Version:     "1.0.0",
		},
		Paths: openapi3.NewPaths(),
	}

	// Add paths from operations
	for functionID, op := range s.openapiOperations {
		if op == nil {
			continue
		}

		pathItem := &openapi3.PathItem{}
		pathItem.Post = op

		path := fmt.Sprintf("/functions/%s", functionID)
		doc.Paths.Set(path, pathItem)
	}

	return doc, nil
}

// DeleteOpenAPI removes an OpenAPI operation by function ID.
func (s *Store) DeleteOpenAPI(functionID string) error {
	if functionID == "" {
		return fmt.Errorf("function ID is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.openapiOperations[functionID]; !exists {
		return fmt.Errorf("operation not found: %s", functionID)
	}

	delete(s.openapiOperations, functionID)
	return nil
}

// DeleteOpenAPIProvider removes an OpenAPI provider by provider ID.
func (s *Store) DeleteOpenAPIProvider(providerID string) error {
	if providerID == "" {
		return fmt.Errorf("provider ID is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.openapiProviders[providerID]; !exists {
		return fmt.Errorf("provider not found: %s", providerID)
	}

	delete(s.openapiProviders, providerID)
	return nil
}

// 设计债清理：原签名声明 error 但纯内存实现恒返 nil，已删除 error 返回值。
func (s *Store) UpsertRegistrationWarning(ctx context.Context, item FunctionRegistrationWarning) {
	if item.Message == "" {
		return
	}
	key := item.Key
	if key == "" {
		// Use SHA-256 hash for fixed-length key, include game_id and env for isolation
		h := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%s|%s|%s|%s", item.GameID, item.Env, item.AgentID, item.FunctionID, item.Code, item.Message)))
		key = hex.EncodeToString(h[:16]) // Use first 16 bytes (32 hex chars) for brevity
	}
	now := time.Now()

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.registrationWarnings == nil {
		s.registrationWarnings = make(map[string]*FunctionRegistrationWarning)
	}
	existing, ok := s.registrationWarnings[key]
	if !ok || existing == nil {
		cp := item
		cp.Key = key
		if cp.FirstSeen.IsZero() {
			cp.FirstSeen = now
		}
		if cp.LastSeen.IsZero() {
			cp.LastSeen = cp.FirstSeen
		}
		if cp.Count <= 0 {
			cp.Count = 1
		}
		s.registrationWarnings[key] = &cp
		return
	}
	existing.Count++
	existing.LastSeen = now
	if existing.Version == "" && item.Version != "" {
		existing.Version = item.Version
	}
	if existing.AgentID == "" && item.AgentID != "" {
		existing.AgentID = item.AgentID
	}
	if existing.FunctionID == "" && item.FunctionID != "" {
		existing.FunctionID = item.FunctionID
	}
}

func (s *Store) ListRegistrationWarnings(filter RegistrationWarningFilter) []FunctionRegistrationWarning {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]FunctionRegistrationWarning, 0, len(s.registrationWarnings))
	for _, item := range s.registrationWarnings {
		if item == nil {
			continue
		}
		if filter.GameID != "" && item.GameID != filter.GameID {
			continue
		}
		if filter.Env != "" && item.Env != filter.Env {
			continue
		}
		if filter.AgentID != "" && item.AgentID != filter.AgentID {
			continue
		}
		if filter.FunctionID != "" && item.FunctionID != filter.FunctionID {
			continue
		}
		if filter.Code != "" && item.Code != filter.Code {
			continue
		}
		out = append(out, *item)
	}
	// Sort by most recent warnings first.
	sort.Slice(out, func(i, j int) bool {
		return out[i].LastSeen.After(out[j].LastSeen)
	})
	if filter.Limit > 0 && len(out) > filter.Limit {
		out = out[:filter.Limit]
	}
	return out
}

// AgentSessionLoader defines the interface for loading agent sessions from database.
type AgentSessionLoader interface {
	LoadActiveSessions(ctx context.Context) ([]*AgentSession, error)
}

// RemoveAgentIfStale 删除 agent 的注册会话（断连/对账清理）。notAfter
// 之后的新注册或心跳（两者都刷新 LastSeen）会中止删除——断连清理与
// 瞬间重连注册的竞态里不能删掉新会话。返回是否实际删除。
func (s *Store) RemoveAgentIfStale(agentID string, notAfter time.Time) bool {
	if agentID == "" {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	cur := s.agents[agentID]
	if cur == nil {
		return false
	}
	if cur.LastSeen.After(notAfter) {
		return false
	}
	delete(s.agents, agentID)
	return true
}

// LoadFromDB loads active agent sessions from the database and populates the in-memory store.
func (s *Store) LoadFromDB(ctx context.Context, loader AgentSessionLoader) error {
	return s.LoadFromDBFiltered(ctx, loader, nil)
}

// LoadFromDBFiltered 按 keep 过滤灌回快照行；keep == nil 等价全量恢复
// （单实例/归属目录不可用时的语义）。
func (s *Store) LoadFromDBFiltered(ctx context.Context, loader AgentSessionLoader, keep func(*AgentSession) bool) error {
	if s.db == nil {
		return fmt.Errorf("database not enabled")
	}
	if loader == nil {
		return fmt.Errorf("agent session loader is required")
	}

	sessions, err := loader.LoadActiveSessions(ctx)
	if err != nil {
		return fmt.Errorf("failed to load sessions: %w", err)
	}

	s.mu.Lock()
	for _, sess := range sessions {
		if keep != nil && !keep(sess) {
			continue
		}
		s.agents[sess.AgentID] = sess
	}
	s.mu.Unlock()

	if err := s.recoverPendingRegistrationOperations(ctx); err != nil {
		return err
	}

	slog.Info("loaded agent sessions from database", "count", len(sessions))
	return nil
}

func (s *Store) recoverPendingRegistrationOperations(ctx context.Context) error {
	if s == nil || s.db == nil {
		return nil
	}
	if s.contractService == nil {
		return nil
	}
	var operations []AgentRegistrationOperationDB
	if err := s.db.WithContext(ctx).
		Where("status IN ?", []string{"pending", "compensation_required"}).
		Order("created_at, id").
		Find(&operations).Error; err != nil {
		return fmt.Errorf("list pending registration recovery operations: %w", err)
	}
	for _, operation := range operations {
		previous, err := decodeRegistrationSession(operation.PreviousSession)
		if err != nil {
			s.markRegistrationOperation(operation.OperationID, "compensation_required", err)
			return fmt.Errorf("decode previous registration session %s: %w", operation.OperationID, err)
		}
		target, err := decodeRegistrationSession(operation.TargetSession)
		if err != nil || target == nil {
			if err == nil {
				err = fmt.Errorf("target session is empty")
			}
			s.markRegistrationOperation(operation.OperationID, "compensation_required", err)
			return fmt.Errorf("decode target registration session %s: %w", operation.OperationID, err)
		}
		if previous == nil {
			previous = &AgentSession{
				AgentID:   target.AgentID,
				GameID:    target.GameID,
				Env:       target.Env,
				Functions: map[string]FunctionMeta{},
			}
		}
		if previous.AgentID == "" {
			previous.AgentID = target.AgentID
		}
		if previous.GameID == "" {
			previous.GameID = target.GameID
		}
		if previous.Env == "" {
			previous.Env = target.Env
		}

		scopeCtx := s.rebuildContext(target.GameID, target.Env)
		reverseDiff := classifyFunctionSnapshot(target.Functions, previous.Functions)
		if _, err := s.materializeScopedTransaction(scopeCtx, previous, s.materializeAgent, reverseDiff); err != nil {
			s.markRegistrationOperation(operation.OperationID, "compensation_required", err)
			return fmt.Errorf("recover registration operation %s: %w", operation.OperationID, err)
		}
		s.markRegistrationOperation(operation.OperationID, "compensated", nil)
		s.regenTemplatesAfterRegistration(scopeCtx, previous, reverseDiff)
	}
	return nil
}

func decodeRegistrationSession(raw string) (*AgentSession, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "null" {
		return nil, nil
	}
	var session AgentSession
	if err := json.Unmarshal([]byte(raw), &session); err != nil {
		return nil, err
	}
	return &session, nil
}

// StartCleanupRoutine 启动后台清理过期 Session 的 goroutine
// 定期从内存中删除过期的 AgentSession，保持内存数据有效
func (s *Store) StartCleanupRoutine(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 1 * time.Minute // 默认每分钟清理一次
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				slog.Info("Registry cleanup routine stopped")
				return
			case <-ticker.C:
				s.cleanupExpiredSessions()
			}
		}
	}()

	slog.Info("Started registry cleanup routine", "interval", interval)
}

// cleanupExpiredSessions 清理过期的 AgentSession
func (s *Store) cleanupExpiredSessions() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	expiredCount := 0

	for agentID, sess := range s.agents {
		if sess == nil {
			continue
		}

		// 检查是否过期
		if sess.ExpireAt.Before(now) {
			delete(s.agents, agentID)
			expiredCount++

			slog.Debug("Cleaned up expired agent session", "agent_id", agentID, "expired_at", sess.ExpireAt.Format(time.RFC3339))
		}
	}

	if expiredCount > 0 {
		slog.Info("Cleaned up expired agent sessions", "expired_count", expiredCount, "remaining", len(s.agents))
	}
}

// RemoveRegistrationWarnings 删除匹配的注册警告（修复闭环：如 provider
// scope 修正后再次注册一致时，清除历史 mismatch 警告——业务层警告的
// 生命周期跟随注册行为，不借运维告警状态机）。
func (s *Store) RemoveRegistrationWarnings(filter RegistrationWarningFilter) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	removed := 0
	for key, item := range s.registrationWarnings {
		if item == nil {
			continue
		}
		if filter.GameID != "" && item.GameID != filter.GameID {
			continue
		}
		if filter.Env != "" && item.Env != filter.Env {
			continue
		}
		if filter.AgentID != "" && item.AgentID != filter.AgentID {
			continue
		}
		if filter.FunctionID != "" && item.FunctionID != filter.FunctionID {
			continue
		}
		if filter.Code != "" && item.Code != filter.Code {
			continue
		}
		delete(s.registrationWarnings, key)
		removed++
	}
	return removed
}
