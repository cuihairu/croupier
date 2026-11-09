package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/db/dbctx"
	"github.com/getkin/kin-openapi/openapi3"
	"gorm.io/gorm"
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
	ProviderID  string
	GameID      string
	Env         string
	Addr        string
	Version     string
	SDKLanguage string // SDK 语言（go, java, python, cpp, csharp, custom）
	SDKVersion  string // SDK 版本
	SDKName     string // SDK 显示名（如 croupier-js-sdk），用户可自定义
	// Metadata 是 provider 自报的用户实例元数据（serverId 等多 KV，保留键已
	// 在 agent 侧剥离）。仅观测展示/搜索用，永不参与路由。
	Metadata map[string]string
	// FirstSeenUnix 是该 provider 在本进程内首次被观测到的时间（#27② 导入
	// 时间）。零值时展示层回退 LastSeenUnix。内存态：随会话过期/断连消失，
	// 重启即重置——只保证「本进程窗口内」语义。
	FirstSeenUnix int64
	// VersionHWM 是本进程内观测到的最高已注册版本（#27② 最新版本，走高
	// 不回退）。空值时展示层回退 Version。内存态边界同 FirstSeenUnix。
	VersionHWM   string
	LastSeenUnix int64
	FunctionIDs  []string
	OpenAPIDoc   json.RawMessage
}

// ProviderSessionSnapshot 是在线 Provider 的只读快照（含所属 agent/scope），
// 供 SDK 版本分布等观测 API 消费（F：sdk-stats）。
type ProviderSessionSnapshot struct {
	ProviderID  string            `json:"providerId"`
	AgentID     string            `json:"agentId"`
	GameID      string            `json:"gameId"`
	Env         string            `json:"env"`
	Addr        string            `json:"addr"`
	Version     string            `json:"version"`
	SDKLanguage string            `json:"sdkLanguage"`
	SDKVersion  string            `json:"sdkVersion"`
	SDKName     string            `json:"sdkName"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	// #27②：导入时间（零值展示层回退 LastSeenUnix）与版本高水位（空回退
	// Version）。内存态，边界见 ProviderSession 注释。
	FirstSeenUnix int64    `json:"firstSeenUnix"`
	VersionHWM    string   `json:"versionHwm,omitempty"`
	LastSeenUnix  int64    `json:"lastSeenUnix"`
	FunctionIDs   []string `json:"functionIds"`
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
				ProviderID:    p.ProviderID,
				AgentID:       sess.AgentID,
				GameID:        sess.GameID,
				Env:           sess.Env,
				Addr:          p.Addr,
				Version:       p.Version,
				SDKLanguage:   p.SDKLanguage,
				SDKVersion:    p.SDKVersion,
				SDKName:       p.SDKName,
				FirstSeenUnix: p.FirstSeenUnix,
				VersionHWM:    p.VersionHWM,
				LastSeenUnix:  p.LastSeenUnix,
			}
			if len(p.Metadata) > 0 {
				snapshot.Metadata = make(map[string]string, len(p.Metadata))
				for key, value := range p.Metadata {
					snapshot.Metadata[key] = value
				}
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
	// Draining 为 true 时调度器不再给该 agent 路由新请求（agentCanInvoke
	// 拦截）；由 opsNodeDrain/opsNodeUndrain 经 SetAgentDraining 设置。
	// UpsertAgent 在会话刷新时原样保留（不因注册/心跳清空）。
	Draining bool
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
	// metaOptionsMu guards metaOptions（实例元数据去重键值聚合的进程内缓存，
	// #11/#2：每次注册刷新，短 TTL 兜底；详见 store_metadata.go）。
	metaOptionsMu sync.Mutex
	metaOptions   map[string]cachedMetaOptions
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
	// MarkContractRemovalPending 注册链函数消失走摘除宽限（不立即真删）：
	// 宽限期内重注册自动清除，过期由 FinalizeExpiredContractRemovals 真删。
	MarkContractRemovalPending(ctx context.Context, gameID, env, functionID string) error
	FinalizeExpiredContractRemovals(ctx context.Context, grace time.Duration) (int, error)
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
		metaOptions:          make(map[string]cachedMetaOptions),
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
		metaOptions:          make(map[string]cachedMetaOptions),
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
	// #11：注册成功后把 provider 元数据投影进 DB（best-effort，失败不回滚
	// 注册）。previousSession 是本次注册前的内存会话，用于清掉本次消失的
	// service 行。无 DB 时为空操作（在线会话聚合退化路径仍可用）。
	s.refreshProviderMetadata(previousSession, a)

	s.mu.Lock()
	defer s.mu.Unlock()
	cur := s.agents[a.AgentID]
	if cur == nil {
		carryProviderSessionHistory(nil, a.Providers)
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
		carryProviderSessionHistory(cur.Providers, a.Providers)
		cur.Providers = a.Providers
	}
	cur.ExpireAt = a.ExpireAt
	cur.LastSeen = a.LastSeen
	// 会话刷新（注册/心跳）不得静默清除 drain：draining 是 ops 侧显式状态，
	// 只经 SetAgentDraining 清除。注册方不携带该意图，故取并保留。
	cur.Draining = cur.Draining || a.Draining
	return nil
}
