package audit

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
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
)

// Chain versions for the audit integrity digest (ARCH-007/ARCH-008).
// Version 1 is the legacy DJB2 digest over the original 7-field subset,
// used by records written before the version field existed. Version 2 is
// the canonical SHA-256 digest over the full semantic record. A version of
// 0 (unset, records built before the version field existed or by code that
// did not set it) is treated as version 1 for backward compatibility.
const (
	ChainVersionLegacy    = 1
	ChainVersionCanonical = 2
)

// WriteFailurePolicy decides what Log does when the audit store rejects a
// write after the chain-conflict retries are exhausted (ARCH-007 D4).
type WriteFailurePolicy string

const (
	// WritePolicyStrict fails the Log call (fail-closed). This is the
	// default: an audit trail must not silently proceed past a gap, so the
	// operator learns the write failed.
	WritePolicyStrict WriteFailurePolicy = "strict"
	// WritePolicyBestEffort logs the gap at WARN, fires a rate-limited
	// notifier alert and returns the record with ChainInfo.WriteFailed set
	// instead of erroring. Opt-in; it trades trail integrity for control
	// plane availability during an audit-store outage, and the gap is
	// surfaced (alert) rather than silent.
	WritePolicyBestEffort WriteFailurePolicy = "best_effort"
)

// defaultGapAlertInterval caps the notifier alert rate for best-effort
// write failures so an extended outage does not spam the notify channel.
const defaultGapAlertInterval = 5 * time.Minute

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

	// Provision events (OPS-002 缩容与排水：销毁/护栏动作留痕)
	EventProvisionDestroy        AuditEventType = "provision.destroy"
	EventProvisionDestroyRequest AuditEventType = "provision.destroy_request"
	EventProvisionProtect        AuditEventType = "provision.protect"
	EventProvisionUnprotect      AuditEventType = "provision.unprotect"

	// OPS-006 batch operation events (config push / restricted command)
	EventBatchOpCreate  AuditEventType = "batch_op.create"
	EventBatchOpDryRun  AuditEventType = "batch_op.dry_run"
	EventBatchOpExecute AuditEventType = "batch_op.execute"

	// OPS-009 runbook automation events (parameterized step sequences
	// referencing OPS-004 deploy / OPS-006 batch capabilities)
	EventRunbookCreate AuditEventType = "runbook.create"
	EventRunbookDelete AuditEventType = "runbook.delete"
	EventRunbookRun    AuditEventType = "runbook.run"

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
	// ChainVersion pins the hash algorithm and field set used for this
	// record (1 = legacy DJB2, 2 = canonical SHA-256). 0 is read as 1.
	ChainVersion int `json:"chainVersion,omitempty"`
	// WriteFailed is set on the record returned by Log in best-effort mode
	// when the store rejected the write; the row was not persisted.
	WriteFailed bool `json:"writeFailed,omitempty"`
}

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

	// writeFailurePolicy decides the audit-store write-failure behavior
	// (ARCH-007 D4). Guarded by mu for the setter, read under RLock.
	writeFailurePolicy WriteFailurePolicy
	// gapAlert state rate-limits the best-effort write-gap notifier alert.
	gapAlertMu       sync.Mutex
	lastGapAlert     time.Time
	gapAlertInterval time.Duration
}

// NewAuditService creates a new audit service
func NewAuditService(store AuditStore, signer AuditSigner) *AuditService {
	return &AuditService{
		store:              store,
		signer:             signer,
		sensitiveFields:    defaultSensitiveFields,
		writeFailurePolicy: WritePolicyStrict,
		gapAlertInterval:   defaultGapAlertInterval,
	}
}

// SetWriteFailurePolicy configures the audit-store write-failure policy
// (ARCH-007 D4). An empty or unknown value falls back to strict.
func (s *AuditService) SetWriteFailurePolicy(policy string) {
	p := WriteFailurePolicy(strings.TrimSpace(strings.ToLower(policy)))
	if p != WritePolicyStrict && p != WritePolicyBestEffort {
		p = WritePolicyStrict
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.writeFailurePolicy = p
}

// WriteFailurePolicy returns the current write-failure policy.
func (s *AuditService) WriteFailurePolicy() WriteFailurePolicy {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.writeFailurePolicy
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

	// ARCH-008 canonical digest: new records use chain version 2 (SHA-256
	// over the full semantic record). The timestamp is truncated to
	// millisecond so the hash input survives the database round-trip
	// (ValidateChain recomputes on the read-back form; postgres timestamptz
	// and MySQL datetime(3) do not preserve nanoseconds). Free-form map
	// normalization happens inside canonicalHashInput, on copies, so the
	// caller-facing record keeps its original Go value shapes.
	record.Timestamp = record.Timestamp.UTC().Truncate(time.Millisecond)
	record.ChainInfo.ChainVersion = ChainVersionCanonical

	// Build chain info
	if err := s.buildChainInfo(record); err != nil {
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
			record.ChainInfo.WriteFailed = false
			break
		}
		if !errors.Is(err, ErrChainSequenceConflict) || attempt >= maxChainRetries-1 {
			// Apply the configured write-failure policy (ARCH-007 D4):
			// strict (default) fails, best-effort surfaces the gap and
			// continues.
			return s.handleWriteFailure(record, err)
		}
		if err := s.buildChainInfo(record); err != nil {
			return nil, fmt.Errorf("failed to rebuild chain info: %w", err)
		}
	}

	// Notify for critical events
	if record.Severity == SeverityCritical && s.notifier != nil {
		_ = s.notifier.NotifyAudit(ctx, record)
	}

	return record, nil
}
