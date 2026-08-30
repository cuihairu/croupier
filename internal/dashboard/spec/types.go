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

// MarshalJSON implements json.Marshaler to serialize JSONSchema as inline JSON
// rather than base64-encoded bytes. Without this method, Go's default encoder
// treats the underlying []byte as binary data and base64-encodes it.
func (s JSONSchema) MarshalJSON() ([]byte, error) {
	if len(s) == 0 {
		return []byte("null"), nil
	}
	return []byte(s), nil
}

// UnmarshalJSON implements json.Unmarshaler to deserialize JSONSchema from
// inline JSON.
func (s *JSONSchema) UnmarshalJSON(data []byte) error {
	*s = data
	return nil
}

// FunctionContractInput is the complete non-presentation registration input
// accepted before normalization. SDK and OpenAPI adapters must construct this
// type explicitly; page and menu presentation are intentionally absent.
type FunctionContractInput struct {
	ID                string
	Version           string
	Enabled           bool
	Deprecated        bool
	Summary           string
	Description       string
	InputSchema       string
	OutputSchema      string
	Resource          string
	Operation         string
	Capability        string
	Execution         string
	ApprovalRequired  bool
	ApprovalPolicyKey string
	Risk              string
	Permission        string
	Tags              []string
}

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
type PageType string

const (
	PageTypeResource  PageType = "resource"
	PageTypeOperation PageType = "operation"
	PageTypeTask      PageType = "task"
	PageTypeReport    PageType = "report"
	// PageTypeComposite 组合页：一页聚合多个资源的视图（每资源一个
	// view 块，Console 按 tab 渲染）。bindings 数组承载全部资源函数，
	// bindingId 带 "<resourceKey>." 前缀防跨资源冲突。
	PageTypeComposite PageType = "composite"
)

// PageBindingUsage is the runtime meaning of a page binding. It belongs to
// PageSpec and is not inferred from function registration.
type PageBindingUsage string

const (
	BindingUsageQuery      PageBindingUsage = "query"
	BindingUsageDetail     PageBindingUsage = "detail"
	BindingUsageAction     PageBindingUsage = "action"
	BindingUsageTask       PageBindingUsage = "task"
	BindingUsageTaskStatus PageBindingUsage = "task_status"
	BindingUsageTaskEvents PageBindingUsage = "task_events"
	BindingUsageTaskResult PageBindingUsage = "task_result"
	BindingUsageTaskCancel PageBindingUsage = "task_cancel"
	BindingUsageTaskRetry  PageBindingUsage = "task_retry"
	BindingUsageReport     PageBindingUsage = "report"
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
	FunctionExecutionSync FunctionExecution = "sync"
	FunctionExecutionTask FunctionExecution = "task"
)

// IsValidFunctionExecution reports whether execution is one of the controlled
// FunctionContract execution values.
func IsValidFunctionExecution(execution FunctionExecution) bool {
	switch execution {
	case FunctionExecutionSync, FunctionExecutionTask:
		return true
	default:
		return false
	}
}

// JsonScalarType is the allowed scalar value type for capability semantics.
type JsonScalarType string

const (
	JsonScalarString  JsonScalarType = "string"
	JsonScalarNumber  JsonScalarType = "number"
	JsonScalarInteger JsonScalarType = "integer"
	JsonScalarBoolean JsonScalarType = "boolean"
)

// IsValidJsonScalarType reports whether valueType is a controlled scalar type.
func IsValidJsonScalarType(valueType JsonScalarType) bool {
	switch valueType {
	case JsonScalarString, JsonScalarNumber, JsonScalarInteger, JsonScalarBoolean:
		return true
	default:
		return false
	}
}

// FunctionRef points at a concrete FunctionContract snapshot.
type FunctionRef struct {
	FunctionID         string `json:"functionId"`
	ContractVersion    string `json:"contractVersion,omitempty"`
	InputSchemaDigest  string `json:"inputSchemaDigest,omitempty"`
	OutputSchemaDigest string `json:"outputSchemaDigest,omitempty"`
}

// ActionSemantic describes resource action context. It does not define button UI.
type ActionSemantic struct {
	Function      FunctionRef `json:"function"`
	Subject       string      `json:"subject"` // resource_item|resource_selection|none
	IdentityInput string      `json:"identityInput,omitempty"`
}

// TaskSemantic describes a task lifecycle around a start function.
type TaskSemantic struct {
	Start  FunctionRef          `json:"start"`
	TaskID TaskIDSemantic       `json:"taskId"`
	Status TaskStatusSemantic   `json:"status"`
	Events *TaskEventsSemantic  `json:"events,omitempty"`
	Result *TaskResultSemantic  `json:"result,omitempty"`
	Cancel *TaskCommandSemantic `json:"cancel,omitempty"`
	Retry  *TaskCommandSemantic `json:"retry,omitempty"`
}

// TaskIDSemantic extracts the task id from the start function result.
type TaskIDSemantic struct {
	ResultPath string         `json:"resultPath"`
	ValueType  JsonScalarType `json:"valueType"`
}

// TaskStatusSemantic describes how to query task state.
type TaskStatusSemantic struct {
	Function    FunctionRef `json:"function"`
	TaskIDInput string      `json:"taskIdInput"`
	StatePath   string      `json:"statePath"`
}

// TaskEventsSemantic describes how to query task events.
type TaskEventsSemantic struct {
	Function    FunctionRef `json:"function"`
	TaskIDInput string      `json:"taskIdInput"`
	EventsPath  string      `json:"eventsPath"`
}

// TaskResultSemantic describes how to query final task result.
type TaskResultSemantic struct {
	Function    FunctionRef `json:"function"`
	TaskIDInput string      `json:"taskIdInput"`
	ResultPath  string      `json:"resultPath"`
}

// TaskCommandSemantic describes a task command function such as cancel/retry.
type TaskCommandSemantic struct {
	Function    FunctionRef `json:"function"`
	TaskIDInput string      `json:"taskIdInput"`
}

// ReportSemantic describes the dataset exposed by a report function.
type ReportSemantic struct {
	Query       FunctionRef `json:"query"`
	DatasetPath string      `json:"datasetPath"`
	Dimensions  []string    `json:"dimensions"`
	Metrics     []string    `json:"metrics"`
}

// ApprovalPolicy is independent from execution mode. A sync or task function
// may require approval before the actual execution starts.
type ApprovalPolicy struct {
	Required  bool   `json:"required"`
	PolicyKey string `json:"policyKey,omitempty"`
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

// ---------------------------------------------------------------------------
// SemanticProvenance
// ---------------------------------------------------------------------------

// SemanticSource indicates where a semantic value originated.
type SemanticSource string

const (
	SemanticSourceOpenAPIRest    SemanticSource = "openapi_rest"
	SemanticSourceSDKExplicit    SemanticSource = "sdk_explicit"
	SemanticSourcePlatformReview SemanticSource = "platform_review"
)

// SemanticProvenance tracks the origin and confidence of a single semantic field.
// Each field in CapabilitySemantics can have its own provenance record.
type SemanticProvenance struct {
	// Field is the semantic field name (e.g., "identityField", "collectionQueryID")
	Field string `json:"field"`

	// Source indicates where this value came from
	Source SemanticSource `json:"source"`

	// SourceDigest is the SHA-256 of the source descriptor that provided this value
	SourceDigest string `json:"sourceDigest"`

	// Confidence indicates how confident the platform is in this value
	// "high" = explicit SDK or platform_review
	// "low" = inferred from REST patterns
	Confidence string `json:"confidence"` // high|low

	// Status indicates the current state of this provenance
	// "effective" = this value is currently used
	// "overridden" = replaced by higher-priority source
	// "conflict" = multiple sources disagree
	Status string `json:"status"` // effective|overridden|conflict

	// ConflictingSources lists sources that provided different values (when status=conflict)
	ConflictingSources []SemanticSource `json:"conflictingSources,omitempty"`

	// Value is the current effective value (JSON encoded)
	Value json.RawMessage `json:"value,omitempty"`

	// OverriddenValue is the value that was overridden (when status=overridden)
	OverriddenValue json.RawMessage `json:"overriddenValue,omitempty"`

	// UpdatedAt is when this provenance was last updated
	UpdatedAt string `json:"updatedAt"`

	// UpdatedBy identifies who/what updated this provenance
	UpdatedBy string `json:"updatedBy"`
}

// SemanticConflict represents a conflict between multiple sources for the same field.
type SemanticConflict struct {
	// Field is the semantic field name
	Field string `json:"field"`

	// Values maps each conflicting source to its proposed value
	Values map[SemanticSource]json.RawMessage `json:"values"`

	// Resolution indicates how this conflict was resolved (if at all)
	// "" = unresolved
	// "platform_review" = admin chose platform_review value
	// "sdk_explicit" = admin chose sdk_explicit value
	Resolution SemanticSource `json:"resolution,omitempty"`

	// ResolvedAt is when this conflict was resolved
	ResolvedAt string `json:"resolvedAt,omitempty"`

	// ResolvedBy identifies who resolved this conflict
	ResolvedBy string `json:"resolvedBy,omitempty"`
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
	ID           string     `json:"id"`
	Version      string     `json:"version"`
	Enabled      bool       `json:"enabled"`
	Deprecated   bool       `json:"deprecated,omitempty"`
	InputSchema  JSONSchema `json:"inputSchema,omitempty"`
	OutputSchema JSONSchema `json:"outputSchema,omitempty"`

	// Catalog/search text. These fields are not runtime menu labels.
	Summary     LocalizedText `json:"summary,omitempty"`
	Description LocalizedText `json:"description,omitempty"`

	Resource   string            `json:"resource,omitempty"`
	Operation  string            `json:"operation,omitempty"`
	Capability CapabilityKind    `json:"capability,omitempty"`
	Execution  FunctionExecution `json:"execution,omitempty"`
	Approval   ApprovalPolicy    `json:"approval"`

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
	Approval    ApprovalPolicy    `json:"approval"`
	Risk        RiskLevel         `json:"risk,omitempty"`
	Permission  string            `json:"permission,omitempty"`
	Enabled     bool              `json:"enabled"`

	Diagnostics []Diagnostic `json:"diagnostics,omitempty"`
}

// ---------------------------------------------------------------------------
// PageSpec
// ---------------------------------------------------------------------------

// PageSpec is the complete page orchestration artifact.
// It is a strong typed business DSL; renderer adapters may translate it to
// ProComponents, but PageSpec never stores React props or component trees.
type PageSpec struct {
	PageKey     string           `json:"pageKey"`
	Type        PageType         `json:"type"`
	ResourceKey string           `json:"resourceKey,omitempty"`
	Title       LocalizedText    `json:"title"`
	Description LocalizedText    `json:"description,omitempty"`
	Category    PageCategorySpec `json:"category"`
	Order       int              `json:"order,omitempty"`
	Icon        string           `json:"icon,omitempty"`

	Navigation *NavigationSpec   `json:"navigation,omitempty"`
	Resource   *ResourcePageSpec `json:"resource,omitempty"`
	// Composite 仅 PageType=composite 时有效：多资源 view 块。
	Composite *CompositePageSpec `json:"composite,omitempty"`
	Operation *OperationPageSpec `json:"operation,omitempty"`
	Task      *TaskPageSpec      `json:"task,omitempty"`
	Report    *ReportPageSpec    `json:"report,omitempty"`

	// Bindings lists the functions this page uses. Page content references
	// bindings by bindingId; direct functionId references are invalid in
	// published PageSpec.
	Bindings []PageFunctionBinding `json:"bindings"`
}

// CompositePageSpec 是自由组合页：区块（section）布局，每区块绑定
// 一个函数并声明视图形态。区块间通过 page_state 联动——一个区块的
// 输出（OutputAssignment.stateKey）可作为另一区块输入
// （SourcePageState）——实现"选玩家 → 联动刷新订单/背包/操作"。
type CompositePageSpec struct {
	Sections []CompositeSection `json:"sections"`
}

// CompositeSection 是组合页的一个区块。
type CompositeSection struct {
	// Key 区块标识（布局内引用；建议与 bindingId 对应）。
	Key string `json:"key"`
	// BindingID 引用 PageSpec.Bindings 的绑定（任意函数）。
	BindingID string        `json:"bindingId"`
	Title     LocalizedText `json:"title,omitempty"`
	// View 区块形态：table（列表）/ fields（键值）/ form（表单操作）/
	// actions（按钮组）。渲染按形态消费 binding 输出。
	View string `json:"view"`
	// Span 栅格宽度（1-24，antd Col；0=整行）。同排区块span和≤24。
	Span int `json:"span,omitempty"`
	// AutoRun 页面加载即执行（查询类区块）；默认 false。
	AutoRun bool `json:"autoRun,omitempty"`
	// RefreshOn 声明依赖的 stateKey 列表——任一变化自动重跑本区块。
	RefreshOn []string `json:"refreshOn,omitempty"`
	// Table 视图参数（view=table）。
	Table *CompositeTableSpec `json:"table,omitempty"`
	// Fields 视图参数（view=fields）。
	Fields []DetailFieldSpec `json:"fields,omitempty"`
	// Form 视图参数（view=form）。
	Form *FormPresentationSpec `json:"form,omitempty"`
	// Display 展示位置：inline（默认，栅格内）/ dialog（弹窗，不占
	// 栅格，由行操作/工具栏按钮触发——「按钮 → 弹窗执行操作」组合）。
	Display string `json:"display,omitempty"`
	// Group 弹窗分组：display=dialog 且 Group 相同的区块渲染进同一弹窗
	//（弹窗内可含表单+字段卡+文本等多组件）；动作目标指向 Group。
	Group string `json:"group,omitempty"`
	// Toolbar 视图参数（view=toolbar）：页面级按钮组，动作打开弹窗
	// 或直接执行。
	Toolbar *CompositeToolbarSpec `json:"toolbar,omitempty"`
	// OnSuccessRefresh 操作成功后自动重跑的区块 key 列表
	//（如发邮件成功后刷新玩家表格）。
	OnSuccessRefresh []string `json:"onSuccessRefresh,omitempty"`
	// Events 通用事件绑定（编辑器全组件事件的发布触发点）：
	// rowClick/rowSelected（table）、success/error（form）、click（fields/text）。
	Events []CompositeEventBinding `json:"events,omitempty"`
}

// CompositeEventBinding 事件绑定：事件名 → 动作步骤（含参数来源）。
type CompositeEventBinding struct {
	// Event 事件名：rowClick|rowSelected|success|error|click。
	Event string `json:"event"`
	// Action 触发的动作（与动作链单步同构）。
	Action CompositeActionStep `json:"action"`
	// Chain 后续步骤。
	Chain []CompositeActionStep `json:"chain,omitempty"`
}

// CompositeTableSpec 是组合页 table 区块参数。
type CompositeTableSpec struct {
	Columns     []ColumnSpec    `json:"columns"`
	Pagination  *PaginationSpec `json:"pagination,omitempty"`
	RowSchema   JSONSchema      `json:"rowSchema,omitempty"`
	IdentityKey string          `json:"identityKey,omitempty"`
	// RowActions 行尾操作按钮：打开弹窗表单（行字段映射进参数）或
	// 直接执行函数。GM 后台最常见组合（列表行内封禁/发邮件）。
	RowActions []CompositeRowAction `json:"rowActions,omitempty"`
}

// CompositeRowAction 表格行操作。
type CompositeRowAction struct {
	// Label 按钮文案。
	Label LocalizedText `json:"label"`
	// TargetSection 弹窗形态（display=dialog）的区块 key。
	TargetSection string `json:"targetSection,omitempty"`
	// Params 参数映射：目标表单参数名 → 行字段名（如
	// player_id: uid 表示弹窗打开时把本行 uid 填入 player_id）。
	Params map[string]string `json:"params,omitempty"`
	// Danger 危险操作样式 + 二次确认。
	Danger bool `json:"danger,omitempty"`
	// Chain 动作链：点击后按序执行的附加步骤（执行/刷新）。
	Chain []CompositeActionStep `json:"chain,omitempty"`
}

// CompositeActionStep 动作链单步。
type CompositeActionStep struct {
	// Kind runBinding|refreshNode|closeModal|navigate|showMessage|openModal。
	Kind string `json:"kind"`
	// Target 目标区块 key / 弹窗分组名（无目标动作为空）。
	Target string `json:"target"`
	// Params 动作参数：navigate={url}、showMessage={message}、
	// runBinding=参数来源映射（"节点id.字段" / "row.字段" / 字面量）。
	Params map[string]string `json:"params,omitempty"`
}

// CompositeToolbarSpec

// CompositeToolbarSpec 工具栏按钮组（view=toolbar）。
type CompositeToolbarSpec struct {
	Actions []CompositeToolbarAction `json:"actions"`
}

// CompositeToolbarAction 工具栏按钮动作。
type CompositeToolbarAction struct {
	Label LocalizedText `json:"label"`
	// TargetSection 弹窗形态区块 key 或弹窗分组名（打开弹窗）。
	TargetSection string `json:"targetSection,omitempty"`
	// Params 静态参数（弹窗表单初始值）。
	Params map[string]string `json:"params,omitempty"`
	Danger bool              `json:"danger,omitempty"`
	// Chain 动作链（同 RowAction.Chain）。
	Chain []CompositeActionStep `json:"chain,omitempty"`
}

// PageCategorySpec groups pages into navigation categories.
type PageCategorySpec struct {
	Key    string        `json:"key"`
	Labels LocalizedText `json:"labels"`
	Order  int           `json:"order,omitempty"`
}

// PageFunctionBinding ties a function to a stable runtime binding in a page.
type PageFunctionBinding struct {
	ID         string               `json:"id"`
	FunctionID string               `json:"functionId"`
	Usage      PageBindingUsage     `json:"usage"`
	Selectors  *BindingSelectors    `json:"selectors,omitempty"`
	Execution  PageBindingExecution `json:"execution"`
}

// BindingSelectors holds input and output selectors for a binding.
type BindingSelectors struct {
	Input  SelectorAST        `json:"input"`
	Output []OutputAssignment `json:"output,omitempty"`
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
	Approval              ApprovalPolicy    `json:"approval"`
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

	// Proposal/source tracking freezes how this page was produced.
	BaseProposalKey     string `json:"baseProposalKey,omitempty"`
	BaseProposalVersion int    `json:"baseProposalVersion,omitempty"`
	FunctionDigest      string `json:"functionDigest,omitempty"`
	SemanticsDigest     string `json:"semanticsDigest,omitempty"`
	GeneratorVersion    string `json:"generatorVersion,omitempty"`

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
)

// GeneratedPageSpec is a PageSpec suggestion produced by the normalizer /
// generator before the user confirms it. It carries a quality indicator
// and diagnostics explaining why it may not be ready for publication.
type GeneratedPageSpec struct {
	PageSpec

	// Quality indicates readiness: "ready", "basic", or "needs_review".
	Quality     GeneratedPageQuality `json:"quality"`
	Diagnostics []Diagnostic         `json:"diagnostics,omitempty"`
}

// BlockedProposalIssue represents a proposal that cannot be materialized into
// a PageSpec. It only contains diagnostics and repair hints, not a spec.
// "blocked" is NOT a Proposal quality; it's a separate issue type.
type BlockedProposalIssue struct {
	// ID is the unique identifier for this issue.
	ID string `json:"id"`

	// GameID and Env identify the scope.
	GameID string `json:"gameId"`
	Env    string `json:"env"`

	// ResourceKey identifies the resource (if applicable).
	ResourceKey string `json:"resourceKey,omitempty"`

	// FunctionID identifies the function (if applicable).
	FunctionID string `json:"functionId,omitempty"`

	// Diagnostics explains why the proposal cannot be materialized.
	Diagnostics []Diagnostic `json:"diagnostics"`

	// RepairHint provides guidance on how to resolve the issue.
	RepairHint LocalizedText `json:"repairHint"`

	// CreatedAt is when this issue was created.
	CreatedAt string `json:"createdAt"`

	// Status tracks whether this issue has been addressed.
	// "open" | "resolved" | "dismissed"
	Status string `json:"status"`
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
