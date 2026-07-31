// Package spec defines the canonical strong types for the Dashboard
// Resource/Page model. All API DTOs, normalizer outputs, and storage
// models must converge to these types.
//
// This package must NOT depend on any business logic, database, or
// framework package. It is a pure type-definition layer.
package spec

import "encoding/json"

// ---------------------------------------------------------------------------
// Primitive type aliases
// ---------------------------------------------------------------------------

// LocalizedText maps locale codes (e.g. "zh-CN", "en-US") to display text.
// At minimum the system default locale must be present for a spec to be
// publishable.
type LocalizedText map[string]string

// JSONValue represents arbitrary JSON payload at parsing boundaries.
// Go runtime code stores it as raw JSON instead of an untyped map.
type JSONValue = json.RawMessage

// JSONSchema is a raw JSON Schema object. The canonical form follows
// draft-07 / 2020-12 but the type itself does not enforce validation.
type JSONSchema json.RawMessage

// FormilySchema is a Formily-compatible JSON Schema. It extends JSON Schema
// with x-component, x-decorator, x-reactions, and other Formily extensions.
type FormilySchema json.RawMessage

// ---------------------------------------------------------------------------
// Enums
// ---------------------------------------------------------------------------

// RiskLevel indicates the risk tier of a function or operation.
//
//	safe     – read-only, no side effects
//	warning  – may have moderate side effects
//	high     – significant side effects, requires confirmation
//	danger   – destructive or irreversible, requires approval
type RiskLevel string

const (
	RiskSafe    RiskLevel = "safe"
	RiskWarning RiskLevel = "warning"
	RiskHigh    RiskLevel = "high"
	RiskDanger  RiskLevel = "danger"
)

// PageType classifies the overall shape of a page.
//
//	entity    – object lifecycle management (list / detail / actions)
//	operation – standalone synchronous action
//	task      – async / batch task with progress tracking
//	report    – analytics query with charts / tables
type PageType string

const (
	PageTypeEntity    PageType = "entity"
	PageTypeOperation PageType = "operation"
	PageTypeTask      PageType = "task"
	PageTypeReport    PageType = "report"
)

// PageBindingUsage is the runtime meaning of a page binding. It belongs to
// PageSpec and is not inferred from function registration.
type PageBindingUsage string

const (
	BindingUsageQuery  PageBindingUsage = "query"
	BindingUsageDetail PageBindingUsage = "detail"
	BindingUsageAction PageBindingUsage = "action"
	BindingUsageTask   PageBindingUsage = "task"
	BindingUsageReport PageBindingUsage = "report"
)

// PageExecutionMode describes how a binding is executed.
type PageExecutionMode string

const (
	PageExecutionModeSync PageExecutionMode = "sync"
	PageExecutionModeTask PageExecutionMode = "task"
)

// PageExecutionKind is the normalized result kind returned to the renderer.
type PageExecutionKind string

const (
	PageExecutionKindSync     PageExecutionKind = "sync"
	PageExecutionKindTask     PageExecutionKind = "task"
	PageExecutionKindApproval PageExecutionKind = "approval"
)

// Diagnostic severity levels.
type DiagnosticSeverity string

const (
	SeverityError   DiagnosticSeverity = "error"
	SeverityWarning DiagnosticSeverity = "warning"
	SeverityInfo    DiagnosticSeverity = "info"
)

// CapabilityKind describes what a function does in a resource lifecycle.
// It is capability semantics, not page placement or UI configuration.
type CapabilityKind string

const (
	CapabilityCollectionQuery CapabilityKind = "collection_query"
	CapabilityItemQuery       CapabilityKind = "item_query"
	CapabilityCreate          CapabilityKind = "create"
	CapabilityUpdate          CapabilityKind = "update"
	CapabilityDelete          CapabilityKind = "delete"
	CapabilityAction          CapabilityKind = "action"
	CapabilityTask            CapabilityKind = "task"
	CapabilityReport          CapabilityKind = "report"
)

// IsValidCapabilityKind reports whether capability is one of the controlled
// FunctionContract capability values.
func IsValidCapabilityKind(capability CapabilityKind) bool {
	switch capability {
	case CapabilityCollectionQuery,
		CapabilityItemQuery,
		CapabilityCreate,
		CapabilityUpdate,
		CapabilityDelete,
		CapabilityAction,
		CapabilityTask,
		CapabilityReport:
		return true
	default:
		return false
	}
}

// FunctionExecution describes how a function is executed by the platform.
type FunctionExecution string

const (
	FunctionExecutionSync     FunctionExecution = "sync"
	FunctionExecutionTask     FunctionExecution = "task"
	FunctionExecutionApproval FunctionExecution = "approval"
)

// IsValidFunctionExecution reports whether execution is one of the controlled
// FunctionContract execution values.
func IsValidFunctionExecution(execution FunctionExecution) bool {
	switch execution {
	case FunctionExecutionSync, FunctionExecutionTask, FunctionExecutionApproval:
		return true
	default:
		return false
	}
}

// ---------------------------------------------------------------------------
// Diagnostic
// ---------------------------------------------------------------------------

// Diagnostic represents a structured validation or readiness message
// attached to a FunctionSpec, ResourceSpec, OperationSpec, or PageSpec.
type Diagnostic struct {
	Code       string             `json:"code"`
	Severity   DiagnosticSeverity `json:"severity"`
	Message    string             `json:"message"`
	FunctionID string             `json:"functionId,omitempty"`
	Field      string             `json:"field,omitempty"`
}

// BindingFreshnessStatus is the current compatibility state of a published
// binding against the latest FunctionSpec. Published pages freeze contracts;
// runtime must never silently switch to a changed function contract.
type BindingFreshnessStatus string

const (
	BindingFreshnessFresh                BindingFreshnessStatus = "fresh"
	BindingFreshnessContractMissing      BindingFreshnessStatus = "contract_missing"
	BindingFreshnessFunctionMissing      BindingFreshnessStatus = "function_missing"
	BindingFreshnessFunctionVersionStale BindingFreshnessStatus = "function_version_stale"
	BindingFreshnessInputSchemaStale     BindingFreshnessStatus = "input_schema_stale"
	BindingFreshnessOutputSchemaStale    BindingFreshnessStatus = "output_schema_stale"
	BindingFreshnessGovernanceStale      BindingFreshnessStatus = "governance_stale"
	BindingFreshnessExecutionModeStale   BindingFreshnessStatus = "execution_mode_stale"
)

// BindingFreshnessDiagnostic explains why a published binding is no longer
// compatible with the latest FunctionSpec. These diagnostics are read-only
// hints for Page Studio and Console; synchronization still requires an
// explicit draft update and re-publish.
type BindingFreshnessDiagnostic struct {
	BindingID  string                 `json:"bindingId"`
	FunctionID string                 `json:"functionId,omitempty"`
	Status     BindingFreshnessStatus `json:"status"`
	Diagnostic Diagnostic             `json:"diagnostic"`
}

// ---------------------------------------------------------------------------
// FunctionSpec
// ---------------------------------------------------------------------------

// FunctionSpec is the normalized representation of a single registered
// function's executable capability. It is produced by the Descriptor
// Normalizer from SDK / OpenAPI / DB template raw descriptors.
type FunctionSpec struct {
	ID                 string        `json:"id"`
	Version            string        `json:"version"`
	Enabled            bool          `json:"enabled"`
	Deprecated         bool          `json:"deprecated,omitempty"`
	InputSchema        JSONSchema    `json:"inputSchema,omitempty"`
	InputFormilySchema FormilySchema `json:"inputFormilySchema,omitempty"`
	OutputSchema       JSONSchema    `json:"outputSchema,omitempty"`

	// Catalog/search text. These fields are not runtime menu labels.
	Summary     LocalizedText `json:"summary,omitempty"`
	Description LocalizedText `json:"description,omitempty"`

	Resource   string            `json:"resource,omitempty"`
	Operation  string            `json:"operation,omitempty"`
	Capability CapabilityKind    `json:"capability,omitempty"`
	Execution  FunctionExecution `json:"execution,omitempty"`

	// Governance
	Risk       RiskLevel `json:"risk,omitempty"`
	Permission string    `json:"permission,omitempty"`
	Tags       []string  `json:"tags,omitempty"`

	// Diagnostics generated during normalization
	Diagnostics []Diagnostic `json:"diagnostics,omitempty"`
}

// ---------------------------------------------------------------------------
// ResourceSpec
// ---------------------------------------------------------------------------

// ResourceSpec represents a stable business resource or capability domain
// that Dashboard pages are organized around. It is NOT a database table
// and NOT a generic CRUD entity.
type ResourceSpec struct {
	Key         string               `json:"key"`
	Labels      LocalizedText        `json:"labels"`
	Description LocalizedText        `json:"description,omitempty"`
	Category    ResourceCategorySpec `json:"category"`
	Order       int                  `json:"order,omitempty"`
	Tags        []string             `json:"tags,omitempty"`

	Operations  []OperationSpec `json:"operations,omitempty"`
	Diagnostics []Diagnostic    `json:"diagnostics,omitempty"`
}

// ResourceCategorySpec groups resources into navigation categories.
type ResourceCategorySpec struct {
	Key    string        `json:"key"`
	Labels LocalizedText `json:"labels"`
	Order  int           `json:"order,omitempty"`
}

// ---------------------------------------------------------------------------
// OperationSpec
// ---------------------------------------------------------------------------

// OperationSpec describes a function capability grouped under a resource.
// It is not a page placement model; PageSpec decides page usage and layout.
type OperationSpec struct {
	FunctionID  string            `json:"functionId"`
	ResourceKey string            `json:"resourceKey,omitempty"`
	Operation   string            `json:"operation"`
	Capability  CapabilityKind    `json:"capability,omitempty"`
	Execution   FunctionExecution `json:"execution,omitempty"`
	Risk        RiskLevel         `json:"risk,omitempty"`
	Permission  string            `json:"permission,omitempty"`
	Enabled     bool              `json:"enabled"`

	Diagnostics []Diagnostic `json:"diagnostics,omitempty"`
}

// ---------------------------------------------------------------------------
// PageSpec
// ---------------------------------------------------------------------------

// PageSpec is the complete page orchestration artifact. Its Schema field
// must be a Formily JSON Schema. PageSpec is the single source of truth
// for page structure; no second "layout" runtime protocol exists.
type PageSpec struct {
	PageKey     string           `json:"pageKey"`
	Type        PageType         `json:"type"`
	ResourceKey string           `json:"resourceKey,omitempty"`
	Title       LocalizedText    `json:"title"`
	Description LocalizedText    `json:"description,omitempty"`
	Category    PageCategorySpec `json:"category"`
	Order       int              `json:"order,omitempty"`
	Icon        string           `json:"icon,omitempty"`

	// Schema is the page-level Formily component tree.
	Schema FormilySchema `json:"schema"`

	// Bindings lists the functions this page uses. Schema components must
	// reference these bindings by bindingId; direct functionId references are
	// invalid in published PageSpec.
	Bindings []PageFunctionBinding `json:"bindings"`

	// Metadata holds arbitrary extension data for the page.
	Metadata map[string]json.RawMessage `json:"metadata,omitempty"`
}

// PageCategorySpec groups pages into navigation categories.
type PageCategorySpec struct {
	Key    string        `json:"key"`
	Labels LocalizedText `json:"labels"`
	Order  int           `json:"order,omitempty"`
}

// PageFunctionBinding ties a function to a stable runtime binding in a page.
type PageFunctionBinding struct {
	ID            string               `json:"id"`
	FunctionID    string               `json:"functionId"`
	Usage         PageBindingUsage     `json:"usage"`
	InputMapping  json.RawMessage      `json:"inputMapping,omitempty"`
	OutputMapping json.RawMessage      `json:"outputMapping,omitempty"`
	Execution     PageBindingExecution `json:"execution"`
}

// PageBindingExecution is the execution policy selected by Page Studio.
type PageBindingExecution struct {
	Mode           PageExecutionMode `json:"mode"`
	RequireConfirm bool              `json:"requireConfirm,omitempty"`
}

// BindingContractSnapshot freezes the executable contract used by a published
// binding. Runtime execution compares the latest FunctionSpec against this
// snapshot and refuses stale bindings by default.
type BindingContractSnapshot struct {
	BindingID             string            `json:"bindingId"`
	FunctionID            string            `json:"functionId"`
	FunctionVersion       string            `json:"functionVersion,omitempty"`
	InputSchemaDigest     string            `json:"inputSchemaDigest,omitempty"`
	OutputSchemaDigest    string            `json:"outputSchemaDigest,omitempty"`
	Risk                  RiskLevel         `json:"risk,omitempty"`
	Permission            string            `json:"permission,omitempty"`
	ExecutionMode         PageExecutionMode `json:"executionMode"`
	RendererSchemaVersion string            `json:"rendererSchemaVersion"`
}

// ---------------------------------------------------------------------------
// PublishedPageSpec
// ---------------------------------------------------------------------------

// PublishedPageSpec is an immutable snapshot of a PageSpec that has passed
// validation and is consumable by the runtime console. Once published the
// snapshot must not be mutated; a new version is created on each publish.
type PublishedPageSpec struct {
	PageSpec

	// GameID and Env are the scope part of PageIdentity.
	GameID string `json:"gameId,omitempty"`
	Env    string `json:"env,omitempty"`

	// Version is the monotonic publish version number.
	Version int `json:"version"`

	// PublishedAt is the RFC 3339 timestamp of publication.
	PublishedAt string `json:"publishedAt"`

	// PublishedBy identifies the user or system that published.
	PublishedBy string `json:"publishedBy,omitempty"`

	// RendererSchemaVersion identifies the server-validated page renderer ABI.
	RendererSchemaVersion string `json:"rendererSchemaVersion"`

	// BindingContracts freezes the function contract for every binding.
	BindingContracts []BindingContractSnapshot `json:"bindingContracts"`

	// BindingFreshness reports contract drift against the latest FunctionSpec.
	// Empty means all bindings are fresh.
	BindingFreshness []BindingFreshnessDiagnostic `json:"bindingFreshness,omitempty"`
}

// PageExecutionResult is the only response shape exposed to Page renderer
// execution. Raw function responses are wrapped in Data for sync calls.
type PageExecutionResult struct {
	Kind        PageExecutionKind `json:"kind"`
	RequestID   string            `json:"requestId"`
	TraceID     string            `json:"traceId,omitempty"`
	Data        json.RawMessage   `json:"data,omitempty"`
	TaskID      string            `json:"taskId,omitempty"`
	ApprovalID  string            `json:"approvalId,omitempty"`
	Diagnostics []Diagnostic      `json:"diagnostics,omitempty"`
}

// ---------------------------------------------------------------------------
// ConsoleMenuSpec
// ---------------------------------------------------------------------------

// ConsoleMenuSpec is the runtime console's left-side navigation menu.
// It is generated from PublishedPageSpec[] and must not store business
// configuration.
type ConsoleMenuSpec struct {
	Items []ConsoleMenuItem `json:"items"`
}

// ConsoleMenuItem is a single entry in the console menu tree.
type ConsoleMenuItem struct {
	Key      string            `json:"key"`
	Path     string            `json:"path"`
	Title    LocalizedText     `json:"title"`
	Locale   bool              `json:"locale"` // dynamic items are always false
	Icon     string            `json:"icon,omitempty"`
	Order    int               `json:"order,omitempty"`
	Children []ConsoleMenuItem `json:"children,omitempty"`
}

// ---------------------------------------------------------------------------
// Generated page suggestion (pre-publish)
// ---------------------------------------------------------------------------

// GeneratedPageQuality indicates how much review is needed before publication.
type GeneratedPageQuality string

const (
	GeneratedPageQualityReady       GeneratedPageQuality = "ready"
	GeneratedPageQualityBasic       GeneratedPageQuality = "basic"
	GeneratedPageQualityNeedsReview GeneratedPageQuality = "needs_review"
	GeneratedPageQualityBlocked     GeneratedPageQuality = "blocked"
)

// GeneratedPageSpec is a PageSpec suggestion produced by the normalizer /
// generator before the user confirms it. It carries a quality indicator
// and diagnostics explaining why it may not be ready for publication.
type GeneratedPageSpec struct {
	PageSpec

	// Quality indicates readiness: "ready", "basic", "needs_review", or "blocked".
	Quality     GeneratedPageQuality `json:"quality"`
	Diagnostics []Diagnostic         `json:"diagnostics,omitempty"`
}

// ---------------------------------------------------------------------------
// Page draft
// ---------------------------------------------------------------------------

// PageDraftStatus represents the status of a page draft.
type PageDraftStatus string

const (
	PageDraftStatusDraft     PageDraftStatus = "draft"
	PageDraftStatusPublished PageDraftStatus = "published"
	PageDraftStatusArchived  PageDraftStatus = "archived"
)

// PageSpecDraftSummary is a summary of a page draft for list views.
type PageSpecDraftSummary struct {
	GameID           string           `json:"gameId,omitempty"`
	Env              string           `json:"env,omitempty"`
	PageKey          string           `json:"pageKey"`
	Type             PageType         `json:"type"`
	ResourceKey      string           `json:"resourceKey,omitempty"`
	Title            LocalizedText    `json:"title"`
	Category         PageCategorySpec `json:"category"`
	Status           PageDraftStatus  `json:"status"`
	DraftRevision    int              `json:"draftRevision"`
	PublishedVersion int              `json:"publishedVersion,omitempty"`
	UpdatedAt        string           `json:"updatedAt"`
	UpdatedBy        string           `json:"updatedBy,omitempty"`
}

// PageVersionItem represents a single version in the page version history.
type PageVersionItem struct {
	Version            int    `json:"version"`
	Status             string `json:"status"`
	Message            string `json:"message,omitempty"`
	IsCurrentDraft     bool   `json:"isCurrentDraft"`
	IsCurrentPublished bool   `json:"isCurrentPublished"`
	CreatedAt          string `json:"createdAt"`
	CreatedBy          string `json:"createdBy,omitempty"`
}
