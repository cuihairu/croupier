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

// JSONSchema is a raw JSON Schema object. The canonical form follows
// draft-07 / 2020-12 but the type itself does not enforce validation.
type JSONSchema json.RawMessage

// FormilySchema is a Formily-compatible JSON Schema. It extends JSON Schema
// with x-component, x-decorator, x-reactions, and other Formily extensions.
type FormilySchema json.RawMessage

// ---------------------------------------------------------------------------
// Enums
// ---------------------------------------------------------------------------

// OperationKind expresses the page-generation semantic of a function.
//
//	list       – list query
//	get        – single object read
//	create     – new object
//	update     – existing object
//	delete     – remove object
//	action     – synchronous command
//	task       – asynchronous / batch task
//	report     – analytics / report query
type OperationKind string

const (
	OperationKindList   OperationKind = "list"
	OperationKindGet    OperationKind = "get"
	OperationKindCreate OperationKind = "create"
	OperationKindUpdate OperationKind = "update"
	OperationKindDelete OperationKind = "delete"
	OperationKindAction OperationKind = "action"
	OperationKindTask   OperationKind = "task"
	OperationKindReport OperationKind = "report"
)

// OperationPlacement describes where an operation is rendered in a page.
//
//	query         – query/filter form area
//	tableData     – table data source
//	detailData    – detail panel data source
//	rowAction     – per-row action in a table
//	detailAction  – action in a detail panel
//	toolbarAction – page-level toolbar button
//	batchAction   – bulk action on selected rows
//	standalone    – independent single-purpose page
type OperationPlacement string

const (
	PlacementQuery         OperationPlacement = "query"
	PlacementTableData     OperationPlacement = "tableData"
	PlacementDetailData    OperationPlacement = "detailData"
	PlacementRowAction     OperationPlacement = "rowAction"
	PlacementDetailAction  OperationPlacement = "detailAction"
	PlacementToolbarAction OperationPlacement = "toolbarAction"
	PlacementBatchAction   OperationPlacement = "batchAction"
	PlacementStandalone    OperationPlacement = "standalone"
)

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

	// Display & search
	DisplayName LocalizedText `json:"displayName,omitempty"`
	Summary     LocalizedText `json:"summary,omitempty"`
	Description LocalizedText `json:"description,omitempty"`

	// Resource / page semantic
	Category         string             `json:"category,omitempty"`
	CategoryDisplay  LocalizedText      `json:"categoryDisplay,omitempty"`
	Entity           string             `json:"entity,omitempty"`
	EntityDisplay    LocalizedText      `json:"entityDisplay,omitempty"`
	Operation        string             `json:"operation,omitempty"`
	OperationDisplay LocalizedText      `json:"operationDisplay,omitempty"`
	OperationKind    OperationKind      `json:"operationKind,omitempty"`
	Placement        OperationPlacement `json:"placement,omitempty"`
	PageHint         string             `json:"pageHint,omitempty"`

	// Governance
	Risk RiskLevel `json:"risk,omitempty"`
	Tags []string  `json:"tags,omitempty"`

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

// OperationSpec describes how a function participates in a Resource or Page:
// its business action key, page-generation semantic, and recommended
// placement in the UI.
type OperationSpec struct {
	FunctionID  string             `json:"functionId"`
	ResourceKey string             `json:"resourceKey,omitempty"`
	Operation   string             `json:"operation"`
	Kind        OperationKind      `json:"kind"`
	Placement   OperationPlacement `json:"placement"`
	Labels      LocalizedText      `json:"labels"`
	Risk        RiskLevel          `json:"risk,omitempty"`
	Enabled     bool               `json:"enabled"`

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

	// Bindings lists the functions this page uses and their roles.
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

// PageFunctionBinding ties a function to a specific role in a page.
type PageFunctionBinding struct {
	FunctionID string             `json:"functionId"`
	Role       OperationPlacement `json:"role"`
}

// ---------------------------------------------------------------------------
// PublishedPageSpec
// ---------------------------------------------------------------------------

// PublishedPageSpec is an immutable snapshot of a PageSpec that has passed
// validation and is consumable by the runtime console. Once published the
// snapshot must not be mutated; a new version is created on each publish.
type PublishedPageSpec struct {
	PageSpec

	// Version is the monotonic publish version number.
	Version int `json:"version"`

	// PublishedAt is the RFC 3339 timestamp of publication.
	PublishedAt string `json:"publishedAt"`

	// PublishedBy identifies the user or system that published.
	PublishedBy string `json:"publishedBy,omitempty"`
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
	PageKey          string           `json:"pageKey"`
	Type             PageType         `json:"type"`
	ResourceKey      string           `json:"resourceKey,omitempty"`
	Title            LocalizedText    `json:"title"`
	Category         PageCategorySpec `json:"category"`
	Status           PageDraftStatus  `json:"status"`
	DraftVersion     int              `json:"draftVersion"`
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
