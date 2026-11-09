package resourcecatalog

import (
	"github.com/cuihairu/croupier/internal/dashboard/spec"
)

// ResourceCatalogItem represents a resource in the catalog.
type ResourceCatalogItem struct {
	ResourceKey   string             `json:"resourceKey"`
	Labels        map[string]string  `json:"labels"`
	Description   map[string]string  `json:"description,omitempty"`
	CategoryKey   string             `json:"categoryKey,omitempty"`
	Status        string             `json:"status"` // identified|pending|conflict|not_executable
	Functions     []FunctionInfo     `json:"functions"`
	Semantics     *SemanticsInfo     `json:"semantics,omitempty"`
	Diagnostics   []DiagnosticInfo   `json:"diagnostics,omitempty"`
	AffectedPages []AffectedPageInfo `json:"affectedPages,omitempty"`
}

// FunctionInfo represents a function in the catalog.
type FunctionInfo struct {
	ID         uint   `json:"id"`
	FunctionID string `json:"functionId"`
	Version    string `json:"version"`
	Capability string `json:"capability"`
	Execution  string `json:"execution"`
	Risk       string `json:"risk"`
	Enabled    bool   `json:"enabled"`
	Source     string `json:"source"`
}

// SemanticsInfo represents capability semantics summary.
type SemanticsInfo struct {
	Version             int                   `json:"version"`
	HasIdentity         bool                  `json:"hasIdentity"`
	HasCollection       bool                  `json:"hasCollection"`
	HasCreate           bool                  `json:"hasCreate"`
	HasUpdate           bool                  `json:"hasUpdate"`
	HasDelete           bool                  `json:"hasDelete"`
	HasActions          bool                  `json:"hasActions"`
	HasTasks            bool                  `json:"hasTasks"`
	HasReports          bool                  `json:"hasReports"`
	Source              string                `json:"source"`
	SourceDigest        string                `json:"sourceDigest,omitempty"`
	IdentityField       string                `json:"identityField,omitempty"`
	IdentityFieldType   string                `json:"identityFieldType,omitempty"`
	IdentityPath        string                `json:"identityPath,omitempty"`
	CollectionQueryID   uint                  `json:"collectionQueryId,omitempty"`
	CollectionPath      string                `json:"collectionPath,omitempty"`
	PageFieldName       string                `json:"pageFieldName,omitempty"`
	PageSizeFieldName   string                `json:"pageSizeFieldName,omitempty"`
	ItemsFieldName      string                `json:"itemsFieldName,omitempty"`
	TotalFieldName      string                `json:"totalFieldName,omitempty"`
	ItemQueryID         uint                  `json:"itemQueryId,omitempty"`
	ItemPath            string                `json:"itemPath,omitempty"`
	CreateID            uint                  `json:"createId,omitempty"`
	UpdateID            uint                  `json:"updateId,omitempty"`
	DeleteID            uint                  `json:"deleteId,omitempty"`
	Actions             []ActionSemanticInfo  `json:"actions,omitempty"`
	Tasks               []spec.TaskSemantic   `json:"tasks,omitempty"`
	Reports             []spec.ReportSemantic `json:"reports,omitempty"`
	UnresolvedConflicts int                   `json:"unresolvedConflicts"`
}

// ActionSemanticInfo describes resource action capability semantics.
type ActionSemanticInfo struct {
	FunctionID    string `json:"functionId"`
	Subject       string `json:"subject"` // resource_item|resource_selection|none
	IdentityInput string `json:"identityInput,omitempty"`
}

// DiagnosticInfo represents a diagnostic message.
type DiagnosticInfo struct {
	Code       string `json:"code"`
	Severity   string `json:"severity"`
	Message    string `json:"message"`
	FunctionID string `json:"functionId,omitempty"`
	Field      string `json:"field,omitempty"`
}

// AffectedPageInfo summarizes UI artifacts impacted by this resource.
type AffectedPageInfo struct {
	PageKey          string                            `json:"pageKey"`
	PageType         string                            `json:"pageType,omitempty"`
	Title            map[string]string                 `json:"title,omitempty"`
	Kind             string                            `json:"kind"` // draft|published|proposal
	Status           string                            `json:"status,omitempty"`
	DraftRevision    int                               `json:"draftRevision,omitempty"`
	PublishedVersion int                               `json:"publishedVersion,omitempty"`
	ProposalKey      string                            `json:"proposalKey,omitempty"`
	ProposalQuality  string                            `json:"proposalQuality,omitempty"`
	ProposalStatus   string                            `json:"proposalStatus,omitempty"`
	Stale            bool                              `json:"stale,omitempty"`
	BindingFreshness []spec.BindingFreshnessDiagnostic `json:"bindingFreshness,omitempty"`
	UpdatedAt        string                            `json:"updatedAt,omitempty"`
}

// CategoryOption is one distinct category available for filtering.
type CategoryOption struct {
	CategoryKey string `json:"categoryKey"`
	Count       int    `json:"count"`
}

// CategoriesResponse is the response for the category aggregation endpoint.
type CategoriesResponse struct {
	Items []CategoryOption `json:"items"`
}

// form tag 必须显式小写：gin 对无 tag 字段按字段名精确匹配（大小写敏感），
// 前端发的 category/query 从未绑定成功过（见 requestbind.BindQueryCompat
// 与 OPEN-ISSUES #5）。
type ListRequest struct {
	GameID   string `form:"gameId"`
	Env      string `form:"env"`
	Category string `form:"category"`
	Query    string `form:"query"`
}

// ListResponse is the response for listing resources.
type ListResponse struct {
	Items []ResourceCatalogItem `json:"items"`
	Total int                   `json:"total"`
}

// DetailRequest is the request for resource detail.
type DetailRequest struct {
	GameID      string `json:"-"`
	Env         string `json:"-"`
	ResourceKey string `uri:"resourceKey" binding:"required"`
}

// UpdateSemanticsRequest is the request for updating resource semantics.
type UpdateSemanticsRequest struct {
	GameID      string `json:"-"`
	Env         string `json:"-"`
	ResourceKey string `json:"-" uri:"resourceKey" binding:"required"`

	// Identity configuration
	IdentityField     string `json:"identityField,omitempty"`
	IdentityFieldType string `json:"identityFieldType,omitempty"` // string|number|integer
	IdentityPath      string `json:"identityPath,omitempty"`

	// Collection configuration
	CollectionQueryID uint   `json:"collectionQueryId,omitempty"`
	CollectionPath    string `json:"collectionPath,omitempty"`
	PageFieldName     string `json:"pageFieldName,omitempty"`
	PageSizeFieldName string `json:"pageSizeFieldName,omitempty"`
	ItemsFieldName    string `json:"itemsFieldName,omitempty"`
	TotalFieldName    string `json:"totalFieldName,omitempty"`

	// Item query configuration
	ItemQueryID uint   `json:"itemQueryId,omitempty"`
	ItemPath    string `json:"itemPath,omitempty"`

	// Lifecycle configuration
	CreateID uint `json:"createId,omitempty"`
	UpdateID uint `json:"updateId,omitempty"`
	DeleteID uint `json:"deleteId,omitempty"`

	// Resource action semantics. This is capability context, not button UI.
	Actions []ActionSemanticInfo `json:"actions,omitempty"`

	// Task/report semantics. These are capability data semantics, not page UI.
	Tasks   []spec.TaskSemantic   `json:"tasks,omitempty"`
	Reports []spec.ReportSemantic `json:"reports,omitempty"`

	// Change reason for audit
	ChangeReason string `json:"changeReason,omitempty"`
}

// UpdateSemanticsResponse is the response for updating semantics.
type UpdateSemanticsResponse struct {
	Version int    `json:"version"`
	Source  string `json:"source"`
	Message string `json:"message"`
}

// SemanticVersionInfo represents a stored CapabilitySemantics snapshot version.
type SemanticVersionInfo struct {
	Version      int    `json:"version"`
	SourceDigest string `json:"sourceDigest,omitempty"`
	ChangeReason string `json:"changeReason,omitempty"`
	CreatedAt    string `json:"createdAt"`
	CreatedBy    string `json:"createdBy,omitempty"`
}

// ListSemanticVersionsRequest is the request for semantic version history.
type ListSemanticVersionsRequest struct {
	GameID      string `json:"-"`
	Env         string `json:"-"`
	ResourceKey string `json:"-" uri:"resourceKey" binding:"required"`
	// Limit caps the page size; defaults to semanticVersionDefaultLimit when 0.
	Limit int `form:"limit"`
	// Offset skips older versions; newest first ordering is preserved.
	Offset int `form:"offset"`
}

const (
	semanticVersionDefaultLimit = 5
	semanticVersionMaxLimit     = 100
)

// ListSemanticVersionsResponse is the response for semantic version history.
type ListSemanticVersionsResponse struct {
	Items []SemanticVersionInfo `json:"items"`
	Total int64                 `json:"total"`
}
