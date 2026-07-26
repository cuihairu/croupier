package page

import (
	"encoding/json"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
)

type PageDraftListRequest struct {
	ResourceKey string `form:"resourceKey"`
	Status      string `form:"status"`
}

type PageDraftListResponse struct {
	Items []spec.PageSpecDraftSummary `json:"items"`
}

type PageDraftRequest struct {
	PageKey string `uri:"pageKey" binding:"required"`
}

type PageDraftResponse struct {
	spec.PageSpec
	GameID           string            `json:"gameId,omitempty"`
	Env              string            `json:"env,omitempty"`
	Status           string            `json:"status"`
	DraftRevision    int               `json:"draftRevision"`
	PublishedVersion int               `json:"publishedVersion,omitempty"`
	Diagnostics      []spec.Diagnostic `json:"diagnostics,omitempty"`
	UpdatedAt        string            `json:"updatedAt"`
	UpdatedBy        string            `json:"updatedBy,omitempty"`
}

type PageSaveRequest struct {
	PageKey       string                     `uri:"pageKey" binding:"required"`
	DraftRevision *int                       `json:"draftRevision" binding:"required"`
	Type          spec.PageType              `json:"type" binding:"required"`
	ResourceKey   string                     `json:"resourceKey,omitempty"`
	Title         map[string]string          `json:"title" binding:"required"`
	Description   map[string]string          `json:"description,omitempty"`
	Category      spec.PageCategorySpec      `json:"category" binding:"required"`
	Order         int                        `json:"order,omitempty"`
	Icon          string                     `json:"icon,omitempty"`
	Schema        json.RawMessage            `json:"schema" binding:"required"`
	Bindings      []spec.PageFunctionBinding `json:"bindings"`
	Metadata      map[string]json.RawMessage `json:"metadata,omitempty"`
}

type PageSaveResponse struct {
	PageKey       string `json:"pageKey"`
	DraftRevision int    `json:"draftRevision"`
}

type PageValidateRequest struct {
	PageKey string `uri:"pageKey" binding:"required"`
}

type PageValidateResponse struct {
	Valid       bool              `json:"valid"`
	Diagnostics []spec.Diagnostic `json:"diagnostics"`
}

type PagePreviewRequest struct {
	PageKey string `uri:"pageKey" binding:"required"`
}

type PagePreviewResponse struct {
	Page spec.PageSpec `json:"page"`
}

type PagePublishRequest struct {
	PageKey       string `uri:"pageKey" binding:"required"`
	DraftRevision *int   `json:"draftRevision" binding:"required"`
}

type PagePublishResponse struct {
	PageKey          string `json:"pageKey"`
	Published        bool   `json:"published"`
	PublishedVersion int    `json:"publishedVersion"`
}

type PageUnpublishRequest struct {
	PageKey string `uri:"pageKey" binding:"required"`
}

type PageUnpublishResponse struct {
	PageKey   string `json:"pageKey"`
	Published bool   `json:"published"`
}

type PageVersionsRequest struct {
	PageKey string `uri:"pageKey" binding:"required"`
}

type PageVersionsResponse struct {
	CurrentDraftRevision    int                    `json:"currentDraftRevision"`
	CurrentPublishedVersion int                    `json:"currentPublishedVersion,omitempty"`
	Items                   []spec.PageVersionItem `json:"items"`
}

type PageVersionDetailRequest struct {
	PageKey   string `uri:"pageKey" binding:"required"`
	VersionID string `uri:"versionId" binding:"required"`
}

type PageVersionDetailResponse struct {
	Version   int           `json:"version"`
	Status    string        `json:"status"`
	Message   string        `json:"message,omitempty"`
	CreatedAt string        `json:"createdAt"`
	CreatedBy string        `json:"createdBy,omitempty"`
	Page      spec.PageSpec `json:"page"`
}

type PageRollbackRequest struct {
	PageKey   string `uri:"pageKey" binding:"required"`
	VersionID string `json:"versionId" binding:"required"`
}

type PageRollbackResponse struct {
	PageKey       string `json:"pageKey"`
	DraftRevision int    `json:"draftRevision"`
}
