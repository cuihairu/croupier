package page

import (
	"encoding/json"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
)

// PageDraftListRequest is the request to list page drafts.
type PageDraftListRequest struct {
	ResourceKey string `form:"resourceKey"`
	Status      string `form:"status"`
}

// PageDraftListResponse is the response with page draft list.
type PageDraftListResponse struct {
	Items []spec.PageSpecDraftSummary `json:"items"`
}

// PageDraftRequest is the request to get a page draft.
type PageDraftRequest struct {
	PageKey string `uri:"pageKey" binding:"required"`
}

// PageDraftResponse is the response with page draft.
type PageDraftResponse struct {
	spec.PageSpec
	Status           string            `json:"status"`
	DraftVersion     int               `json:"draftVersion"`
	PublishedVersion int               `json:"publishedVersion,omitempty"`
	Diagnostics      []spec.Diagnostic `json:"diagnostics,omitempty"`
	UpdatedAt        string            `json:"updatedAt"`
	UpdatedBy        string            `json:"updatedBy,omitempty"`
}

// PageSaveRequest is the request to save a page draft.
type PageSaveRequest struct {
	PageKey     string                     `uri:"pageKey" binding:"required"`
	Type        string                     `json:"type" binding:"required"`
	ResourceKey string                     `json:"resourceKey,omitempty"`
	Title       map[string]string          `json:"title" binding:"required"`
	Description map[string]string          `json:"description,omitempty"`
	Category    spec.PageCategorySpec      `json:"category" binding:"required"`
	Order       int                        `json:"order,omitempty"`
	Icon        string                     `json:"icon,omitempty"`
	Schema      json.RawMessage            `json:"schema" binding:"required"`
	Bindings    []spec.PageFunctionBinding `json:"bindings"`
	Metadata    map[string]json.RawMessage `json:"metadata,omitempty"`
}

// PageSaveResponse is the response from saving a page draft.
type PageSaveResponse struct {
	PageKey      string `json:"pageKey"`
	DraftVersion int    `json:"draftVersion"`
}

// PageValidateRequest is the request to validate a page draft.
type PageValidateRequest struct {
	PageKey string `uri:"pageKey" binding:"required"`
}

// PageValidateResponse is the response with validation result.
type PageValidateResponse struct {
	Valid       bool              `json:"valid"`
	Diagnostics []spec.Diagnostic `json:"diagnostics"`
}

// PagePreviewRequest is the request to preview a page draft.
type PagePreviewRequest struct {
	PageKey string `uri:"pageKey" binding:"required"`
}

// PagePreviewResponse is the response with preview.
type PagePreviewResponse struct {
	Page spec.PageSpec `json:"page"`
}

// PagePublishRequest is the request to publish a page.
type PagePublishRequest struct {
	PageKey     string `uri:"pageKey" binding:"required"`
	PublishedBy string `json:"publishedBy,omitempty"`
}

// PagePublishResponse is the response from publishing.
type PagePublishResponse struct {
	PageKey          string `json:"pageKey"`
	Published        bool   `json:"published"`
	PublishedVersion int    `json:"publishedVersion"`
}

// PageUnpublishRequest is the request to unpublish a page.
type PageUnpublishRequest struct {
	PageKey string `uri:"pageKey" binding:"required"`
}

// PageUnpublishResponse is the response from unpublishing.
type PageUnpublishResponse struct {
	PageKey   string `json:"pageKey"`
	Published bool   `json:"published"`
}

// PageVersionsRequest is the request to list page versions.
type PageVersionsRequest struct {
	PageKey string `uri:"pageKey" binding:"required"`
}

// PageVersionsResponse is the response with page versions.
type PageVersionsResponse struct {
	CurrentDraftVersion     int                    `json:"currentDraftVersion"`
	CurrentPublishedVersion int                    `json:"currentPublishedVersion,omitempty"`
	Items                   []spec.PageVersionItem `json:"items"`
}

// PageVersionDetailRequest is the request to get a version detail.
type PageVersionDetailRequest struct {
	PageKey   string `uri:"pageKey" binding:"required"`
	VersionID string `uri:"versionId" binding:"required"`
}

// PageVersionDetailResponse is the response with a full page version.
type PageVersionDetailResponse struct {
	Version   int           `json:"version"`
	Status    string        `json:"status"`
	Message   string        `json:"message,omitempty"`
	CreatedAt string        `json:"createdAt"`
	CreatedBy string        `json:"createdBy,omitempty"`
	Page      spec.PageSpec `json:"page"`
}

// PageRollbackRequest is the request to rollback a page.
type PageRollbackRequest struct {
	PageKey   string `uri:"pageKey" binding:"required"`
	VersionID string `uri:"versionId" binding:"required"`
}

// PageRollbackResponse is the response from rollback.
type PageRollbackResponse struct {
	PageKey string `json:"pageKey"`
	Version int    `json:"version"`
}
