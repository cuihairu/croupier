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

// JSONValue represents arbitrary JSON data at parsing boundaries. Core DTOs
// should prefer typed structs or json.RawMessage and only use JSONValue when
// traversing JSON Schema / extension documents.
type JSONValue = any

// JSONObject is an explicitly named JSON object shape used by schema parsing
// code instead of leaking map[string]interface{} through public contracts.
type JSONObject map[string]JSONValue

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

	Resource  string `json:"resource,omitempty"`
	Operation string `json:"operation,omitempty"`

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
	FunctionID  string    `json:"functionId"`
	ResourceKey string    `json:"resourceKey,omitempty"`
	Operation   string    `json:"operation"`
	Risk        RiskLevel `json:"risk,omitempty"`
	Permission  string    `json:"permission,omitempty"`
	Enabled     bool      `json:"enabled"`

	PageContract *PageContract `json:"pageContract,omitempty"`
	Diagnostics  []Diagnostic  `json:"diagnostics,omitempty"`
}

// PageContract is an optional, machine-readable data contract used only by
// the generator to create higher-quality PageSpec candidates. It is not a UI
// schema and must not contain Formily components, routes, menus, or layouts.
type PageContract struct {
	Version       string                  `json:"version"`
	ExecutionMode PageExecutionMode       `json:"executionMode,omitempty"`
	InputMapping  json.RawMessage         `json:"inputMapping,omitempty"`
	OutputMapping json.RawMessage         `json:"outputMapping,omitempty"`
	Pagination    *PagePaginationContract `json:"pagination,omitempty"`
	Table         *PageTableContract      `json:"table,omitempty"`
	Task          *PageTaskContract       `json:"task,omitempty"`
	Report        *PageReportContract     `json:"report,omitempty"`
}

// PagePaginationContract describes stable request and response paths for a
// paginated data source.
type PagePaginationContract struct {
	PageField     string `json:"pageField"`
	PageSizeField string `json:"pageSizeField"`
	ItemsPath     string `json:"itemsPath"`
	TotalPath     string `json:"totalPath"`
}

// PageTableContract describes stable columns for a DataTable. Either Columns
// or ColumnsPath must be present for a generated table to be publishable.
type PageTableContract struct {
	Columns     []PageTableColumnContract `json:"columns,omitempty"`
	ColumnsPath string                    `json:"columnsPath,omitempty"`
}

type PageTableColumnContract struct {
	Key       string        `json:"key"`
	Title     LocalizedText `json:"title"`
	ValuePath string        `json:"valuePath"`
}

// PageTaskContract describes external task tracking data needed by TaskPage.
type PageTaskContract struct {
	TaskIDPath string `json:"taskIdPath,omitempty"`
	StatusPath string `json:"statusPath,omitempty"`
	EventsPath string `json:"eventsPath,omitempty"`
	ResultPath string `json:"resultPath,omitempty"`
}

// PageReportContract describes report/chart data mappings. Without this
// contract the generator must not produce a ready ChartPanel.
type PageReportContract struct {
	ChartType    string `json:"chartType,omitempty"`
	CategoryPath string `json:"categoryPath,omitempty"`
	SeriesPath   string `json:"seriesPath,omitempty"`
	ValuePath    string `json:"valuePath,omitempty"`
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

// GeneratedPageSpec is a PageSpec suggestion produced by the normalizer /
// generator before the user confirms it. It carries a quality indicator
// and diagnostics explaining why it may not be ready for publication.
type GeneratedPageSpec struct {
	PageSpec

	// Quality indicates readiness: "ready", "needs_review", or "blocked".
	Quality     string       `json:"quality"`
	Diagnostics []Diagnostic `json:"diagnostics,omitempty"`
}

// ---------------------------------------------------------------------------
// Page draft (workspace)
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
