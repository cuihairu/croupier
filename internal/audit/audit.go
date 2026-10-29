package audit

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cuihairu/croupier/internal/ipgeo"
	"github.com/cuihairu/croupier/internal/middleware/reqinfo"
)

// Audit related errors
var (
	ErrAuditNotFound    = errors.New("audit record not found")
	ErrInvalidChain     = errors.New("audit chain validation failed")
	ErrChainBroken      = errors.New("audit chain is broken")
	ErrExportFailed     = errors.New("audit export failed")
	ErrInvalidTimeRange = errors.New("invalid time range")
	// ErrAuditWriteFailed 是 ARCH-007 D6 的失败关闭哨兵：审计存储写入失败
	// 且事件属于失败关闭白名单时，调用方必须据此中止操作（记录未建 →
	// 操作不执行）。白名单之外的事件走失败开放（见 LogOrDegrade）。
	ErrAuditWriteFailed = errors.New("audit write failed; operation must not proceed")
)

// failClosedEvents 是 D6 的失败关闭白名单：不可逆安全变更，缺留痕比审计
// 故障更危险。对这些事件，审计写入失败必须中止操作（返回
// ErrAuditWriteFailed）。身份事件（auth.login* / authz.access_*）刻意
// 不在列——无法失败关闭（审计库不可用时没有身份可鉴）。
var failClosedEvents = map[AuditEventType]bool{
	EventUserDelete:     true,
	EventUserLock:       true,
	EventUserUnlock:     true,
	EventPasswordChange: true,
	EventMFAEnabled:     true,
	EventMFADisabled:    true,
	EventBackupRestore:  true,
}

// AuditEventType defines the type of audit event
type AuditEventType string

const (
	// Authentication events
	EventLogin          AuditEventType = "auth.login"
	EventLogout         AuditEventType = "auth.logout"
	EventLoginFailed    AuditEventType = "auth.login_failed"
	EventPasswordChange AuditEventType = "auth.password_change"
	EventMFAEnabled     AuditEventType = "auth.mfa_enabled"
	EventMFADisabled    AuditEventType = "auth.mfa_disabled"

	// Authorization events
	EventAccessGranted   AuditEventType = "authz.access_granted"
	EventAccessDenied    AuditEventType = "authz.access_denied"
	EventRoleAssigned    AuditEventType = "authz.role_assigned"
	EventRoleRevoked     AuditEventType = "authz.role_revoked"
	EventPermissionGrant AuditEventType = "authz.permission_grant"

	// Approval events
	EventApprovalCreated   AuditEventType = "approval.created"
	EventApprovalApproved  AuditEventType = "approval.approved"
	EventApprovalRejected  AuditEventType = "approval.rejected"
	EventApprovalCancelled AuditEventType = "approval.cancelled"
	EventApprovalExpired   AuditEventType = "approval.expired"
	// EventApprovalStepUpFailed 记高危审批 step-up TOTP 校验未通过/未提供/未绑定；
	// 只记结果档位与账号，绝不记 otp 值本身（对齐字段脱敏要求）。
	EventApprovalStepUpFailed AuditEventType = "approval.stepup_failed"

	// Workflow events
	EventWorkflowStarted   AuditEventType = "workflow.started"
	EventWorkflowCompleted AuditEventType = "workflow.completed"
	EventWorkflowCancelled AuditEventType = "workflow.cancelled"
	EventStepApproved      AuditEventType = "workflow.step_approved"
	EventStepRejected      AuditEventType = "workflow.step_rejected"
	EventDelegationCreated AuditEventType = "workflow.delegation_created"
	EventDelegationRevoked AuditEventType = "workflow.delegation_revoked"

	// Function events
	EventFunctionInvoke     AuditEventType = "function.invoke"
	EventFunctionRegister   AuditEventType = "function.register"
	EventFunctionUnregister AuditEventType = "function.unregister"
	EventFunctionUpdate     AuditEventType = "function.update"
	// EventFunctionContractUpdated 契约更新（含 schema 兼容性 diff 摘要，F13）
	EventFunctionContractUpdated AuditEventType = "function.contract_updated"

	// Dashboard page events
	EventPageDraftSave AuditEventType = "page.draft_save"
	EventPagePublish   AuditEventType = "page.publish"
	EventPageUnpublish AuditEventType = "page.unpublish"
	EventPageRollback  AuditEventType = "page.rollback"
	EventPageExecute   AuditEventType = "page.execute"

	// Semantic events
	EventSemanticUpdate           AuditEventType = "semantic.update"
	EventSemanticConflict         AuditEventType = "semantic.conflict"
	EventSemanticConflictResolve  AuditEventType = "semantic.conflict_resolve"
	EventSemanticProvenanceUpdate AuditEventType = "semantic.provenance_update"

	// OpenAPI Source events
	EventOpenAPISourceCreate        AuditEventType = "openapi_source.create"
	EventOpenAPISourceUpdate        AuditEventType = "openapi_source.update"
	EventOpenAPISourceBindingCreate AuditEventType = "openapi_source.binding_create"
	EventOpenAPISourceBindingDelete AuditEventType = "openapi_source.binding_delete"
	// EventOpenAPISourceBindingAuto 运行时注册命中 unbound 契约自动置 bound（D3/T6）
	EventOpenAPISourceBindingAuto AuditEventType = "openapi_source.binding_auto"

	// Configuration events
	EventConfigCreate        AuditEventType = "config.create"
	EventConfigUpdate        AuditEventType = "config.update"
	EventConfigDelete        AuditEventType = "config.delete"
	EventConfigEmergencyEdit AuditEventType = "config.emergency_edit"
	EventConfigSourceChange  AuditEventType = "config.source_change"

	// Data events
	EventDataAccess AuditEventType = "data.access"
	EventDataExport AuditEventType = "data.export"
	EventDataImport AuditEventType = "data.import"
	EventDataDelete AuditEventType = "data.delete"

	// System events
	EventSystemStart   AuditEventType = "system.start"
	EventSystemStop    AuditEventType = "system.stop"
	EventSystemConfig  AuditEventType = "system.config_change"
	EventBackupCreate  AuditEventType = "system.backup_create"
	EventBackupRestore AuditEventType = "system.backup_restore"

	// Ops node & job events (previously memory-only in OpsStateStore;
	// promoted to persistent audit records)
	EventNodeDrain   AuditEventType = "node.drain"
	EventNodeUndrain AuditEventType = "node.undrain"
	EventNodeRestart AuditEventType = "node.restart"
	EventJobStart    AuditEventType = "job.start"
	EventJobCancel   AuditEventType = "job.cancel"

	// Admin events
	EventUserCreate AuditEventType = "admin.user_create"
	EventUserUpdate AuditEventType = "admin.user_update"
	EventUserDelete AuditEventType = "admin.user_delete"
	EventUserLock   AuditEventType = "admin.user_lock"
	EventUserUnlock AuditEventType = "admin.user_unlock"
)

// AuditSeverity defines the severity level of an audit event
type AuditSeverity string

const (
	SeverityInfo     AuditSeverity = "info"
	SeverityWarning  AuditSeverity = "warning"
	SeverityError    AuditSeverity = "error"
	SeverityCritical AuditSeverity = "critical"
)

// AuditCategory defines the category of an audit event
type AuditCategory string

const (
	CategorySecurity    AuditCategory = "security"
	CategoryCompliance  AuditCategory = "compliance"
	CategoryOperational AuditCategory = "operational"
	CategoryAdmin       AuditCategory = "admin"
	CategoryData        AuditCategory = "data"
)

// AuditRecord represents a complete audit record
type AuditRecord struct {
	ID           string                 `json:"id"`
	Timestamp    time.Time              `json:"timestamp"`
	EventType    AuditEventType         `json:"eventType"`
	Category     AuditCategory          `json:"category"`
	Severity     AuditSeverity          `json:"severity"`
	Actor        ActorInfo              `json:"actor"`
	Action       string                 `json:"action"`
	Resource     ResourceInfo           `json:"resource"`
	Details      map[string]interface{} `json:"details,omitempty"`
	Changes      *ChangeInfo            `json:"changes,omitempty"`
	Context      AuditContext           `json:"context"`
	Outcome      string                 `json:"outcome"` // success, failure, pending
	ErrorMessage string                 `json:"errorMessage,omitempty"`
	ChainInfo    ChainInfo              `json:"chainInfo"`
}

// ActorInfo contains information about the actor performing the action
type ActorInfo struct {
	ID          string            `json:"id"`
	Type        string            `json:"type"` // user, service, system
	Name        string            `json:"name,omitempty"`
	Email       string            `json:"email,omitempty"`
	Roles       []string          `json:"roles,omitempty"`
	IPAddress   string            `json:"ipAddress,omitempty"`
	UserAgent   string            `json:"userAgent,omitempty"`
	SessionID   string            `json:"sessionId,omitempty"`
	MFAUsed     bool              `json:"mfaUsed,omitempty"`
	DelegatedBy string            `json:"delegatedBy,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// ResourceInfo contains information about the affected resource
type ResourceInfo struct {
	Type        string            `json:"type"`
	ID          string            `json:"id"`
	Name        string            `json:"name,omitempty"`
	GameID      string            `json:"gameId,omitempty"`
	Environment string            `json:"environment,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// ChangeInfo contains before/after values for changes
type ChangeInfo struct {
	Before     map[string]interface{} `json:"before,omitempty"`
	After      map[string]interface{} `json:"after,omitempty"`
	DiffFields []string               `json:"diffFields,omitempty"`
}

// AuditContext contains contextual information about the audit
type AuditContext struct {
	RequestID     string            `json:"requestId,omitempty"`
	TraceID       string            `json:"traceId,omitempty"`
	CorrelationID string            `json:"correlationId,omitempty"`
	Service       string            `json:"service,omitempty"`
	Version       string            `json:"version,omitempty"`
	Environment   string            `json:"environment,omitempty"`
	Tags          map[string]string `json:"tags,omitempty"`
}

// ChainInfo contains blockchain-like chain information for integrity
type ChainInfo struct {
	Hash      string `json:"hash"`
	PrevHash  string `json:"prevHash"`
	Sequence  int64  `json:"sequence"`
	SignerID  string `json:"signerId,omitempty"`
	Signature string `json:"signature,omitempty"`
	// HashVersion selects the digest algorithm/field set for this record
	// (ARCH-007 D3). 1 = legacy narrow-field djb2 32-bit; 2 = SHA-256 over
	// the canonical full payload. ValidateChain dispatches per record.
	HashVersion int `json:"hashVersion"`
}

// Hash algorithm versions (ARCH-007 D2/D3).
const (
	// HashVersionLegacy is the pre-ARCH-008 digest: djb2 32-bit over a
	// narrow field list (timestamp/event_type/actor_id/resource.ID/outcome/
	// prev_hash/sequence/details). Kept only to verify records written before
	// the upgrade; never used for new records.
	HashVersionLegacy = 1
	// HashVersionCurrent is the ARCH-008 digest: SHA-256 over the canonical
	// full payload (D1). New records always use this.
	HashVersionCurrent = 2
)

// AuditFilter for filtering audit records
type AuditFilter struct {
	EventType    []AuditEventType
	Category     []AuditCategory
	Severity     []AuditSeverity
	ActorID      string
	ResourceID   string
	ResourceType string
	GameID       string
	Environment  string
	Outcome      string
	StartTime    *time.Time
	EndTime      *time.Time
	SearchText   string
	Tags         map[string]string
}

// AuditPage for pagination
type AuditPage struct {
	Page     int
	PageSize int
	SortBy   string // timestamp, severity, actor
	SortDesc bool
}

// AuditStats contains statistics about audit records
type AuditStats struct {
	TotalRecords    int64                  `json:"totalRecords"`
	RecordsToday    int64                  `json:"recordsToday"`
	RecordsThisWeek int64                  `json:"recordsThisWeek"`
	ByEventType     map[AuditEventType]int `json:"byEventType"`
	ByCategory      map[AuditCategory]int  `json:"byCategory"`
	BySeverity      map[AuditSeverity]int  `json:"bySeverity"`
	ByActor         map[string]int         `json:"byActor"`
	TopActors       []ActorStat            `json:"topActors"`
	FailureRate     float64                `json:"failureRate"`
}

// ActorStat contains statistics for an actor
type ActorStat struct {
	ActorID string `json:"actorId"`
	Count   int    `json:"count"`
}

// AuditStore interface for audit persistence
type AuditStore interface {
	Create(record *AuditRecord) error
	Get(id string) (*AuditRecord, error)
	List(filter AuditFilter, page AuditPage) ([]*AuditRecord, int, error)
	Delete(id string) error
	DeleteBefore(timestamp time.Time) (int64, error)

	// Chain operations
	GetLatestRecord() (*AuditRecord, error)
	GetBySequence(seq int64) (*AuditRecord, error)
	GetChainRange(startSeq, endSeq int64) ([]*AuditRecord, error)

	// Statistics
	GetStats(startTime, endTime time.Time) (*AuditStats, error)
	CountByFilter(filter AuditFilter) (int64, error)

	// Export
	Export(filter AuditFilter, format string) ([]byte, error)
}

// AuditService provides audit logging functionality
type AuditService struct {
	store           AuditStore
	signer          AuditSigner
	sensitiveFields []string
	mu              sync.RWMutex
	notifier        AuditNotifier
	// writeFailures 统计审计存储写入失败次数（ARCH-007 D6 呈现信号；
	// 白名单与失败开放路径共用）。导出经 WriteFailures。
	writeFailures atomic.Int64
}

// WriteFailures returns the cumulative audit-store write failure count. A
// monotonic observability signal: a nonzero/rising count means audit rows are
// being lost and should alert (D6). Reset is not exposed; restart clears it.
func (s *AuditService) WriteFailures() int64 {
	if s == nil {
		return 0
	}
	return s.writeFailures.Load()
}

// noteWriteFailure 是 D6 的统一呈现点：每次审计持久化失败计数 +1 并落
// error 日志（含事件类型与原因），任何调用路径（Log 直调或 LogOrDegrade
// 策略层）都不再静默丢行。
func (s *AuditService) noteWriteFailure(record *AuditRecord, cause error) {
	s.writeFailures.Add(1)
	slog.Error("audit write failed",
		"eventType", record.EventType,
		"sequence", record.ChainInfo.Sequence,
		"err", cause,
	)
}

// degradedKey / degradedFlag 构成每请求一次的降级标记：HTTP 中间件把
// *degradedFlag 植入请求 ctx，LogOrDegrade 在失败开放路径 set，响应阶段
// 中间件读到后写 X-Audit-Degraded 头。非 HTTP 调用路径没有植入时
// markDegraded 为安全 no-op（计数与日志照常生效）。
type degradedKey struct{}

type degradedFlag struct{ set atomic.Bool }

func markDegraded(ctx context.Context) {
	if ctx == nil {
		return
	}
	if flag, ok := ctx.Value(degradedKey{}).(*degradedFlag); ok && flag != nil {
		flag.set.Store(true)
	}
}

// WasDegraded reports whether an audit write was degraded on this context.
// Planted by WithDegradedFlag (see DegradedMiddleware).
func WasDegraded(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	if flag, ok := ctx.Value(degradedKey{}).(*degradedFlag); ok && flag != nil {
		return flag.set.Load()
	}
	return false
}

// WithDegradedFlag plants a per-request degraded flag into ctx so
// markDegraded/WasDegraded can communicate across the (immutable) context
// boundary.
func WithDegradedFlag(ctx context.Context) context.Context {
	return context.WithValue(ctx, degradedKey{}, &degradedFlag{})
}

// LogOrDegrade 是 ARCH-007 D6 的策略咽喉点：
//   - 成功：返回 (record, nil)。
//   - 失败且事件在失败关闭白名单：返回 (nil, ErrAuditWriteFailed)，调用方
//     必须中止操作（记录未建 → 操作不执行）。
//   - 失败且非白名单：失败开放——返回 (nil, nil)，操作照常进行；失败已
//     经由计数器与 error 日志呈现（noteWriteFailure），并尽力标记请求
//     降级（WasDegraded → X-Audit-Degraded 头）。
//
// 调用方约定：非白名单事件不再据返回值阻断业务（旧代码里分裂的
// `_, err` 上抛 / `_, _` 静默两种形态统一收敛到这里）。
func (s *AuditService) LogOrDegrade(ctx context.Context, eventType AuditEventType, opts ...AuditOption) (*AuditRecord, error) {
	record, err := s.Log(ctx, eventType, opts...)
	if err == nil {
		return record, nil
	}
	if failClosedEvents[eventType] {
		return nil, ErrAuditWriteFailed
	}
	markDegraded(ctx)
	return nil, nil
}

// NewAuditService creates a new audit service
func NewAuditService(store AuditStore, signer AuditSigner) *AuditService {
	return &AuditService{
		store:           store,
		signer:          signer,
		sensitiveFields: defaultSensitiveFields,
	}
}

// SetNotifier sets the audit notifier
// Store 暴露底层存储（链校验等管理场景使用）。
func (s *AuditService) Store() AuditStore {
	if s == nil {
		return nil
	}
	return s.store
}

func (s *AuditService) SetNotifier(notifier AuditNotifier) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.notifier = notifier
}

// Log creates and stores an audit record
func (s *AuditService) Log(ctx context.Context, eventType AuditEventType, opts ...AuditOption) (*AuditRecord, error) {
	record := &AuditRecord{
		ID:        generateAuditID(),
		Timestamp: time.Now().UTC(),
		EventType: eventType,
		Outcome:   "success",
		Details:   make(map[string]interface{}),
	}

	// Apply options
	for _, opt := range opts {
		opt(record)
	}

	// Fill client identity from the request context when the caller did not
	// pass it explicitly (WithIPAddress), so every HTTP-originated event
	// carries ip/userAgent without each call site repeating the plumbing.
	if record.Actor.IPAddress == "" && ctx != nil {
		if info, ok := reqinfo.FromContext(ctx); ok && info.IP != "" {
			record.Actor.IPAddress = strings.TrimSpace(info.IP)
			if record.Actor.UserAgent == "" {
				record.Actor.UserAgent = strings.TrimSpace(info.UserAgent)
			}
		}
	}
	// Resolve the IP region once at write time (needs the IP2Location LITE
	// BIN database; empty when absent). Stored in Details so queries and the
	// UI show 属地 without re-resolving on every read.
	if ip := strings.TrimSpace(record.Actor.IPAddress); ip != "" && record.Details != nil {
		if _, exists := record.Details["ipRegion"]; !exists {
			if region := ipgeo.Region(ip); region != "" {
				record.Details["ipRegion"] = region
			}
		}
	}

	// Set defaults based on event type
	s.setDefaults(record)

	// Mask sensitive data
	s.maskSensitiveData(record)

	// Build chain info
	if err := s.buildChainInfo(record); err != nil {
		s.noteWriteFailure(record, err)
		return nil, fmt.Errorf("failed to build chain info: %w", err)
	}

	// Store the record。多实例部署下链 sequence 由各实例独立分配，并发
	// 窗口内会撞唯一约束（ErrChainSequenceConflict）——重查 DB 真实链尾
	// 重算 sequence/prev/hash 后重试。唯一约束兜底 + 重试保证只有与链尾
	// 正确衔接的行能落库，链完整性不因多实例并发分叉。
	const maxChainRetries = 3
	for attempt := 0; ; attempt++ {
		err := s.store.Create(record)
		if err == nil {
			break
		}
		if !errors.Is(err, ErrChainSequenceConflict) || attempt >= maxChainRetries-1 {
			s.noteWriteFailure(record, err)
			return nil, fmt.Errorf("failed to store audit record: %w", err)
		}
		if err := s.buildChainInfo(record); err != nil {
			s.noteWriteFailure(record, err)
			return nil, fmt.Errorf("failed to rebuild chain info: %w", err)
		}
	}

	// Notify for critical events
	if record.Severity == SeverityCritical && s.notifier != nil {
		_ = s.notifier.NotifyAudit(ctx, record)
	}

	return record, nil
}

// AuditOption is a function that modifies an audit record
type AuditOption func(*AuditRecord)

// WithActor sets the actor information
func WithActor(actor ActorInfo) AuditOption {
	return func(r *AuditRecord) {
		r.Actor = actor
	}
}

// WithActorID sets the actor ID
func WithActorID(id, actorType, name string) AuditOption {
	return func(r *AuditRecord) {
		r.Actor = ActorInfo{
			ID:   id,
			Type: actorType,
			Name: name,
		}
	}
}

// WithResource sets the resource information
func WithResource(resource ResourceInfo) AuditOption {
	return func(r *AuditRecord) {
		r.Resource = resource
	}
}

// WithResourceID sets the resource ID
func WithResourceID(resourceType, id string) AuditOption {
	return func(r *AuditRecord) {
		r.Resource = ResourceInfo{
			Type: resourceType,
			ID:   id,
		}
	}
}

// WithDetails sets the details
func WithDetails(details map[string]interface{}) AuditOption {
	return func(r *AuditRecord) {
		r.Details = details
	}
}

// WithChanges sets the change information
func WithChanges(before, after map[string]interface{}) AuditOption {
	return func(r *AuditRecord) {
		r.Changes = &ChangeInfo{
			Before: before,
			After:  after,
		}
		// Calculate diff fields
		r.Changes.DiffFields = calculateDiffFields(before, after)
	}
}

// WithContext sets the audit context
func WithContext(ctx AuditContext) AuditOption {
	return func(r *AuditRecord) {
		r.Context = ctx
	}
}

// WithOutcome sets the outcome
func WithOutcome(outcome, errMsg string) AuditOption {
	return func(r *AuditRecord) {
		r.Outcome = outcome
		r.ErrorMessage = errMsg
	}
}

// WithSeverity sets the severity
func WithSeverity(severity AuditSeverity) AuditOption {
	return func(r *AuditRecord) {
		r.Severity = severity
	}
}

// WithCategory sets the category
func WithCategory(category AuditCategory) AuditOption {
	return func(r *AuditRecord) {
		r.Category = category
	}
}

// WithGameID sets the game ID
func WithGameID(gameID, env string) AuditOption {
	return func(r *AuditRecord) {
		r.Resource.GameID = gameID
		r.Resource.Environment = env
	}
}

// WithIPAddress sets the IP address
func WithIPAddress(ip, userAgent string) AuditOption {
	return func(r *AuditRecord) {
		r.Actor.IPAddress = ip
		r.Actor.UserAgent = userAgent
	}
}

// setDefaults sets default values based on event type
func (s *AuditService) setDefaults(record *AuditRecord) {
	// Set category based on event type
	if record.Category == "" {
		record.Category = s.inferCategory(record.EventType)
	}

	// Set severity based on event type
	if record.Severity == "" {
		record.Severity = s.inferSeverity(record.EventType)
	}

	// Set action if not set
	if record.Action == "" {
		record.Action = string(record.EventType)
	}
}

// inferCategory infers category from event type
func (s *AuditService) inferCategory(eventType AuditEventType) AuditCategory {
	switch eventType {
	case EventLogin, EventLogout, EventLoginFailed, EventAccessGranted, EventAccessDenied:
		return CategorySecurity
	case EventUserCreate, EventUserUpdate, EventUserDelete:
		return CategoryAdmin
	case EventFunctionInvoke, EventFunctionRegister, EventFunctionUnregister,
		EventFunctionUpdate, EventFunctionContractUpdated,
		EventPageDraftSave, EventPagePublish,
		EventPageUnpublish, EventPageRollback, EventPageExecute,
		EventOpenAPISourceCreate, EventOpenAPISourceBindingCreate,
		EventOpenAPISourceUpdate, EventOpenAPISourceBindingDelete,
		EventOpenAPISourceBindingAuto,
		EventConfigUpdate, EventConfigEmergencyEdit,
		EventConfigSourceChange:
		return CategoryOperational
	case EventDataAccess, EventDataExport, EventDataDelete:
		return CategoryData
	default:
		return CategoryCompliance
	}
}

// inferSeverity infers severity from event type
func (s *AuditService) inferSeverity(eventType AuditEventType) AuditSeverity {
	switch eventType {
	case EventLoginFailed, EventAccessDenied:
		return SeverityWarning
	case EventUserDelete, EventBackupRestore:
		return SeverityCritical
	default:
		return SeverityInfo
	}
}

// maskSensitiveData masks sensitive fields in the record
func (s *AuditService) maskSensitiveData(record *AuditRecord) {
	if record.Details != nil {
		record.Details = maskMap(record.Details, s.sensitiveFields)
	}
	if record.Changes != nil {
		if record.Changes.Before != nil {
			record.Changes.Before = maskMap(record.Changes.Before, s.sensitiveFields)
		}
		if record.Changes.After != nil {
			record.Changes.After = maskMap(record.Changes.After, s.sensitiveFields)
		}
	}
}

// defaultSensitiveFields 是平台统一的敏感字段清单（精确匹配）。
var defaultSensitiveFields = []string{
	"password", "secret", "token", "key", "credential",
	"api_key", "private_key", "access_token",
}

// MaskSensitiveValue 递归掩码任意 JSON 值中的敏感字段（与审计记录同一
// 敏感字段清单）。供执行留痕等需要同口径脱敏的场景复用。
func MaskSensitiveValue(value interface{}) interface{} {
	switch v := value.(type) {
	case map[string]interface{}:
		return maskMap(v, defaultSensitiveFields)
	case []interface{}:
		out := make([]interface{}, len(v))
		for i, item := range v {
			out[i] = MaskSensitiveValue(item)
		}
		return out
	default:
		return value
	}
}

// buildChainInfo builds the chain information for the record
func (s *AuditService) buildChainInfo(record *AuditRecord) error {
	// Get the latest record for prev hash
	latest, err := s.store.GetLatestRecord()
	if err != nil && err != ErrAuditNotFound {
		return err
	}

	var prevHash string
	var sequence int64 = 1

	if latest != nil {
		prevHash = latest.ChainInfo.Hash
		sequence = latest.ChainInfo.Sequence + 1
	}

	record.ChainInfo = ChainInfo{
		PrevHash:    prevHash,
		Sequence:    sequence,
		HashVersion: HashVersionCurrent,
	}

	// Calculate hash
	hash, err := s.calculateHash(record, HashVersionCurrent)
	if err != nil {
		return err
	}
	record.ChainInfo.Hash = hash

	// Sign if signer is available
	if s.signer != nil {
		signature, signerID, err := s.signer.Sign(record)
		if err != nil {
			return err
		}
		record.ChainInfo.Signature = signature
		record.ChainInfo.SignerID = signerID
	}

	return nil
}

// calculateHash dispatches by digest version (ARCH-007 D3). v1 recomputes the
// legacy digest so pre-upgrade records stay verifiable; v2 (and any unknown
// version, treated as current) computes the canonical full-payload SHA-256.
func (s *AuditService) calculateHash(record *AuditRecord, version int) (string, error) {
	switch version {
	case HashVersionLegacy:
		return legacyHash(record)
	default:
		return sha256Canonical(record)
	}
}

// legacyHash is the pre-ARCH-008 digest, byte-identical to the old
// calculateHash so existing v1 records verify unchanged. Deliberately frozen:
// it must never gain fields, or old chains would stop verifying.
func legacyHash(record *AuditRecord) (string, error) {
	data := map[string]interface{}{
		"timestamp":  record.Timestamp.UnixNano(),
		"event_type": record.EventType,
		"actor_id":   record.Actor.ID,
		"resource":   record.Resource.ID,
		"outcome":    record.Outcome,
		"prev_hash":  record.ChainInfo.PrevHash,
		"sequence":   record.ChainInfo.Sequence,
	}

	if record.Details != nil {
		data["details"] = record.Details
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return "", err
	}

	return hashBytes(jsonData), nil
}

// sha256Canonical computes the ARCH-008 digest (D1/D2): SHA-256 over the
// canonical full-payload JSON, with the JSON sub-objects embedded as raw bytes.
func sha256Canonical(record *AuditRecord) (string, error) {
	payload, err := hashPayload(record)
	if err != nil {
		return "", err
	}
	return sha256Hex(payload), nil
}

// hashPayload builds the canonical payload for the current digest. It covers
// every integrity-relevant field; JSON sub-objects are embedded as their raw
// bytes so the write-time and verify-time digests are identical by
// construction (D1 — eliminates the >2^53 integer round-trip divergence).
// Excluded: chain.hash, chain.signature, chain.signer_id (self-referential /
// keyed-cover) and created_at (store insertion timestamp).
func hashPayload(record *AuditRecord) ([]byte, error) {
	actor, err := json.Marshal(record.Actor)
	if err != nil {
		return nil, fmt.Errorf("hash payload: actor: %w", err)
	}
	resource, err := json.Marshal(record.Resource)
	if err != nil {
		return nil, fmt.Errorf("hash payload: resource: %w", err)
	}
	details, err := json.Marshal(record.Details)
	if err != nil {
		return nil, fmt.Errorf("hash payload: details: %w", err)
	}
	changes, err := json.Marshal(record.Changes)
	if err != nil {
		return nil, fmt.Errorf("hash payload: changes: %w", err)
	}
	contextJSON, err := json.Marshal(record.Context)
	if err != nil {
		return nil, fmt.Errorf("hash payload: context: %w", err)
	}

	data := map[string]interface{}{
		"id":            record.ID,
		"timestamp":     record.Timestamp.UnixNano(),
		"event_type":    record.EventType,
		"category":      record.Category,
		"severity":      record.Severity,
		"action":        record.Action,
		"outcome":       record.Outcome,
		"error_message": record.ErrorMessage,
		"sequence":      record.ChainInfo.Sequence,
		"prev_hash":     record.ChainInfo.PrevHash,
		"actor":         json.RawMessage(actor),
		"resource":      json.RawMessage(resource),
		"details":       json.RawMessage(details),
		"changes":       json.RawMessage(changes),
		"context":       json.RawMessage(contextJSON),
	}
	return json.Marshal(data)
}

// sha256Hex returns the lowercase hex SHA-256 of data.
func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// Get retrieves an audit record by ID
func (s *AuditService) Get(id string) (*AuditRecord, error) {
	return s.store.Get(id)
}

// List lists audit records with filtering and pagination
func (s *AuditService) List(filter AuditFilter, page AuditPage) ([]*AuditRecord, int, error) {
	return s.store.List(filter, page)
}

// GetStats gets audit statistics
func (s *AuditService) GetStats(startTime, endTime time.Time) (*AuditStats, error) {
	return s.store.GetStats(startTime, endTime)
}

// ValidateChain validates the integrity of the audit chain
func (s *AuditService) ValidateChain(startSeq, endSeq int64) (*ChainValidationResult, error) {
	records, err := s.store.GetChainRange(startSeq, endSeq)
	if err != nil {
		return nil, err
	}

	result := &ChainValidationResult{
		Valid:        true,
		TotalRecords: len(records),
		Errors:       []ChainError{},
	}

	for i, record := range records {
		// Verify hash. Dispatch by the record's digest version: v1 rows
		// (written before ARCH-008, hash_version backfilled to 1) verify
		// under the frozen legacy digest; v2 rows under the canonical
		// SHA-256. Unknown/zero version is treated as legacy — the safe
		// reading for rows that predate the column.
		version := record.ChainInfo.HashVersion
		if version == 0 {
			version = HashVersionLegacy
		}
		expectedHash, err := s.calculateHash(record, version)
		if err != nil {
			result.Valid = false
			result.Errors = append(result.Errors, ChainError{
				Sequence: record.ChainInfo.Sequence,
				Type:     "hash_calculation_error",
				Message:  err.Error(),
			})
			continue
		}

		if record.ChainInfo.Hash != expectedHash {
			result.Valid = false
			result.Errors = append(result.Errors, ChainError{
				Sequence: record.ChainInfo.Sequence,
				Type:     "hash_mismatch",
				Message:  "Record hash does not match calculated hash",
			})
		}

		// Verify chain link
		if i > 0 {
			prevRecord := records[i-1]
			if record.ChainInfo.PrevHash != prevRecord.ChainInfo.Hash {
				result.Valid = false
				result.Errors = append(result.Errors, ChainError{
					Sequence: record.ChainInfo.Sequence,
					Type:     "chain_broken",
					Message:  "Previous hash does not match",
				})
			}
		}

		// Verify signature if present
		if s.signer != nil && record.ChainInfo.Signature != "" {
			if err := s.signer.Verify(record); err != nil {
				result.Valid = false
				result.Errors = append(result.Errors, ChainError{
					Sequence: record.ChainInfo.Sequence,
					Type:     "signature_invalid",
					Message:  err.Error(),
				})
			}
		}
	}

	return result, nil
}

// ChainValidationResult contains the result of chain validation
type ChainValidationResult struct {
	Valid        bool         `json:"valid"`
	TotalRecords int          `json:"totalRecords"`
	Errors       []ChainError `json:"errors,omitempty"`
}

// ChainError represents an error found during chain validation
type ChainError struct {
	Sequence int64  `json:"sequence"`
	Type     string `json:"type"`
	Message  string `json:"message"`
}

// Export exports audit records
func (s *AuditService) Export(filter AuditFilter, format string) ([]byte, error) {
	return s.store.Export(filter, format)
}

// Archive archives old audit records
func (s *AuditService) Archive(before time.Time, archivePath string) (int64, error) {
	// Export records to archive
	filter := AuditFilter{
		EndTime: &before,
	}

	data, err := s.store.Export(filter, "json")
	if err != nil {
		return 0, err
	}

	// Here you would typically write to archive storage
	// For now, we'll just delete old records
	// In production, you'd write to S3, tape, etc. first
	// data contains the exported records that would be written to archive
	_ = data // Archive data would be written to archivePath

	// Delete archived records
	count, err := s.store.DeleteBefore(before)
	if err != nil {
		return 0, err
	}

	return count, nil
}

// AuditSigner interface for signing audit records
type AuditSigner interface {
	Sign(record *AuditRecord) (signature, signerID string, err error)
	Verify(record *AuditRecord) error
}

// AuditNotifier interface for audit notifications
type AuditNotifier interface {
	NotifyAudit(ctx context.Context, record *AuditRecord) error
}

// Helper functions

func generateAuditID() string {
	// Use crypto/rand for more unique IDs
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return fmt.Sprintf("audit_%d_%x", time.Now().UnixNano(), b)
}

func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	_, _ = rand.Read(b)
	for i := range b {
		b[i] = letters[int(b[i])%len(letters)]
	}
	return string(b)
}

func hashBytes(data []byte) string {
	// Using SHA256 for hashing
	// In production, you'd use crypto/sha256
	h := uint32(5381)
	for _, b := range data {
		h = ((h << 5) + h) + uint32(b)
	}
	return fmt.Sprintf("%08x", h)
}

func maskMap(m map[string]interface{}, sensitiveFields []string) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range m {
		if isSensitiveField(k, sensitiveFields) {
			result[k] = "***MASKED***"
		} else {
			if nested, ok := v.(map[string]interface{}); ok {
				result[k] = maskMap(nested, sensitiveFields)
			} else {
				result[k] = v
			}
		}
	}
	return result
}

func isSensitiveField(field string, sensitiveFields []string) bool {
	fieldLower := field
	for _, sf := range sensitiveFields {
		if fieldLower == sf {
			return true
		}
	}
	return false
}

func calculateDiffFields(before, after map[string]interface{}) []string {
	var diff []string
	allKeys := make(map[string]bool)

	for k := range before {
		allKeys[k] = true
	}
	for k := range after {
		allKeys[k] = true
	}

	for k := range allKeys {
		bv, bExists := before[k]
		av, aExists := after[k]

		if !bExists || !aExists {
			diff = append(diff, k)
			continue
		}

		// Simple comparison - in production, you'd do deep comparison
		if fmt.Sprintf("%v", bv) != fmt.Sprintf("%v", av) {
			diff = append(diff, k)
		}
	}

	return diff
}
