package workspace

// WorkspaceConfig represents a workspace configuration
type WorkspaceConfig struct {
	ObjectKey   string              `json:"objectKey"`
	Title       string              `json:"title"`
	Description string              `json:"description,omitempty"`
	Layout      interface{}         `json:"layout"`
	Published   bool                `json:"published"`
	PublishedAt string              `json:"publishedAt,omitempty"`
	PublishedBy string              `json:"publishedBy,omitempty"`
	MenuOrder   int                 `json:"menuOrder"`
	Status      string              `json:"status"`
	Version     int                 `json:"version,omitempty"`
	CreatedAt   string              `json:"createdAt,omitempty"`
	UpdatedAt   string              `json:"updatedAt,omitempty"`
	Meta        WorkspaceConfigMeta `json:"meta,omitempty"`
}

// ListConfigsRequest is the request to list all workspace configs
type ListConfigsRequest struct{}

// ListConfigsResponse is the response with workspace configs list
type ListConfigsResponse struct {
	Items []WorkspaceConfig `json:"items"`
}

// ListPublishedRequest is the request to list published workspaces
type ListPublishedRequest struct{}

// ListPublishedResponse is the response with published workspaces
type ListPublishedResponse struct {
	Items []WorkspaceConfig `json:"items"`
}

// GetConfigRequest is the request to get a workspace config
type GetConfigRequest struct {
	ObjectKey string `uri:"objectKey" binding:"required"`
}

// GetConfigResponse is the response with workspace config
type GetConfigResponse struct {
	WorkspaceConfig WorkspaceConfig `json:"workspaceConfig"`
}

// SaveConfigRequest is the request to save a workspace config
type SaveConfigRequest struct {
	ObjectKey   string      `uri:"objectKey" binding:"required"`
	Title       string      `json:"title"`
	Description string      `json:"description"`
	Layout      interface{} `json:"layout"`
	MenuOrder   int         `json:"menuOrder"`
	Status      string      `json:"status"`
}

// SaveConfigResponse is the response from saving workspace config
type SaveConfigResponse struct {
	WorkspaceConfig WorkspaceConfig `json:"workspaceConfig"`
}

// DeleteConfigRequest is the request to delete a workspace config
type DeleteConfigRequest struct {
	ObjectKey string `uri:"objectKey" binding:"required"`
}

// DeleteConfigResponse is the response from deleting workspace config
type DeleteConfigResponse struct {
	Message string `json:"message"`
}

// PublishRequest is the request to publish a workspace
type PublishRequest struct {
	ObjectKey   string `json:"objectKey" binding:"required"`
	PublishedBy string `json:"publishedBy"`
}

// PublishResponse is the response from publishing workspace
type PublishResponse struct {
	Published bool   `json:"published"`
	ObjectKey string `json:"objectKey"`
}

// UnpublishRequest is the request to unpublish a workspace
type UnpublishRequest struct {
	ObjectKey string `json:"objectKey" binding:"required"`
}

// UnpublishResponse is the response from unpublishing workspace
type UnpublishResponse struct {
	Published bool   `json:"published"`
	ObjectKey string `json:"objectKey"`
}

// VersionsRequest is the request to list workspace versions
type VersionsRequest struct {
	ObjectKey string `form:"objectKey" binding:"required"`
	From      string `form:"from"`
	To        string `form:"to"`
}

// WorkspaceVersionRecord represents a workspace version
type WorkspaceVersionRecord struct {
	ID                 string      `json:"id"`
	ObjectKey          string      `json:"objectKey"`
	Version            int         `json:"version"`
	Config             interface{} `json:"config"`
	IsCurrentDraft     bool        `json:"isCurrentDraft"`
	IsCurrentPublished bool        `json:"isCurrentPublished"`
	CreatedAt          string      `json:"createdAt"`
	CreatedBy          string      `json:"createdBy"`
	Comment            string      `json:"comment"`
}

// VersionsResponse is the response with workspace versions
type VersionsResponse struct {
	Items []WorkspaceVersionRecord `json:"items"`
}

// VersionDetailRequest is the request to get workspace version detail
type VersionDetailRequest struct {
	ObjectKey string `uri:"objectKey" binding:"required"`
	VersionID string `uri:"versionId" binding:"required"`
}

// VersionDetailResponse is the response with workspace version detail
type VersionDetailResponse struct {
	WorkspaceVersionRecord WorkspaceVersionRecord `json:"workspaceVersionRecord"`
}

// RollbackRequest is the request to rollback a workspace
type RollbackRequest struct {
	ObjectKey string `uri:"objectKey" binding:"required"`
	VersionID string `uri:"versionId" binding:"required"`
}

// RollbackResponse is the response from rolling back workspace
type RollbackResponse struct {
	ObjectKey string `json:"objectKey"`
	Version   int    `json:"version"`
}

// ============================================================
// Types from internal/types/types.go for consistency
// These are the canonical types used across the system
// ============================================================

// WorkspaceConfigMeta represents metadata for workspace configuration
type WorkspaceConfigMeta struct {
	CreatedAt string `json:"createdAt,omitempty"`
	UpdatedAt string `json:"updatedAt,omitempty"`
}

// WorkspaceConfig represents a workspace configuration (canonical type from types.go)
type WorkspaceConfigCanonical struct {
	ObjectKey   string              `json:"objectKey"`
	Title       string              `json:"title,omitempty"`
	Description string              `json:"description,omitempty"`
	Layout      interface{}         `json:"layout,omitempty"`
	Published   bool                `json:"published"`
	Status      string              `json:"status,omitempty"`
	PublishedAt string              `json:"publishedAt,omitempty"`
	PublishedBy string              `json:"publishedBy,omitempty"`
	MenuOrder   int                 `json:"menuOrder"`
	Version     int                 `json:"version,omitempty"`
	Meta        WorkspaceConfigMeta `json:"meta,omitempty"`
}

// WorkspaceConfigDeleteRequest is the request to delete a workspace config
type WorkspaceConfigDeleteRequest struct {
	ObjectKey string `uri:"objectKey"`
}

// WorkspaceConfigDeleteResponse is the response from deleting workspace config
type WorkspaceConfigDeleteResponse struct {
	Message string `json:"message"`
}

// WorkspaceConfigGetRequest is the request to get a workspace config
type WorkspaceConfigGetRequest struct {
	ObjectKey string `uri:"objectKey"`
}

// WorkspaceConfigGetResponse is the response with workspace config
type WorkspaceConfigGetResponse struct {
	WorkspaceConfigCanonical
}

// WorkspaceConfigSaveRequest is the request to save a workspace config
type WorkspaceConfigSaveRequest struct {
	ObjectKey   string      `uri:"objectKey"`
	Title       string      `json:"title"`
	Description string      `json:"description"`
	Layout      interface{} `json:"layout,omitempty"`
	Status      string      `json:"status"`
	MenuOrder   int         `json:"menuOrder"`
}

// WorkspaceConfigSaveResponse is the response from saving workspace config
type WorkspaceConfigSaveResponse struct {
	WorkspaceConfigCanonical
}

// WorkspaceConfigsListRequest is the request to list all workspace configs
type WorkspaceConfigsListRequest struct {
}

// WorkspaceConfigsListResponse is the response with workspace configs list
type WorkspaceConfigsListResponse struct {
	Items []WorkspaceConfigCanonical `json:"items"`
}

// WorkspacePublishRequest is the request to publish a workspace
type WorkspacePublishRequest struct {
	ObjectKey   string `uri:"objectKey"`
	PublishedBy string `json:"publishedBy"`
}

// WorkspacePublishResponse is the response from publishing workspace
type WorkspacePublishResponse struct {
	Published bool   `json:"published"`
	ObjectKey string `json:"objectKey"`
}

// WorkspacePublishedListRequest is the request to list published workspaces
type WorkspacePublishedListRequest struct {
}

// WorkspacePublishedListResponse is the response with published workspaces
type WorkspacePublishedListResponse struct {
	Items []WorkspaceConfigCanonical `json:"items"`
}

// WorkspaceRollbackRequest is the request to rollback a workspace
type WorkspaceRollbackRequest struct {
	ObjectKey string `uri:"objectKey"`
	VersionId string `json:"versionId"`
}

// WorkspaceRollbackResponse is the response from rolling back workspace
type WorkspaceRollbackResponse struct {
	ObjectKey string `json:"objectKey"`
	Version   int    `json:"version"`
}

// WorkspaceUnpublishRequest is the request to unpublish a workspace
type WorkspaceUnpublishRequest struct {
	ObjectKey string `uri:"objectKey"`
}

// WorkspaceUnpublishResponse is the response from unpublishing workspace
type WorkspaceUnpublishResponse struct {
	Published bool   `json:"published"`
	ObjectKey string `json:"objectKey"`
}

// WorkspaceVersionDetailRequest is the request to get workspace version detail
type WorkspaceVersionDetailRequest struct {
	ObjectKey string `uri:"objectKey"`
	VersionId string `uri:"versionId"`
}

// WorkspaceVersionDetailResponse is the response with workspace version detail
type WorkspaceVersionDetailResponse struct {
	WorkspaceVersionRecordCanonical
}

// WorkspaceVersionRecord represents a workspace version
type WorkspaceVersionRecordCanonical struct {
	Id                 string      `json:"id"`
	ObjectKey          string      `json:"objectKey"`
	Version            int         `json:"version"`
	Config             interface{} `json:"config"`
	IsCurrentDraft     bool        `json:"isCurrentDraft"`
	IsCurrentPublished bool        `json:"isCurrentPublished"`
	CreatedAt          string      `json:"createdAt,omitempty"`
	CreatedBy          string      `json:"createdBy,omitempty"`
	Comment            string      `json:"comment,omitempty"`
}

// WorkspaceVersionsRequest is the request to list workspace versions
type WorkspaceVersionsRequest struct {
	ObjectKey string `uri:"objectKey"`
	From      string `form:"from"`
	To        string `form:"to"`
}

// WorkspaceVersionsResponse is the response with workspace versions
type WorkspaceVersionsResponse struct {
	Items                   []WorkspaceVersionRecordCanonical `json:"items"`
	CurrentDraftVersion     int                               `json:"currentDraftVersion"`
	CurrentPublishedVersion int                               `json:"currentPublishedVersion"`
}

// ============================================================
// Type aliases for backward compatibility
// These map the canonical types to existing service code
// ============================================================

// Aliases for WorkspaceConfig variants
const (
// Using WorkspaceConfig as-is since it's already defined
)

// Aliases for requests
type (
	// Config operations
	SaveConfigRequestAlias    = WorkspaceConfigSaveRequest
	GetConfigRequestAlias     = WorkspaceConfigGetRequest
	DeleteConfigRequestAlias  = WorkspaceConfigDeleteRequest
	ListConfigsRequestAlias   = WorkspaceConfigsListRequest
	SaveConfigResponseAlias   = WorkspaceConfigSaveResponse
	GetConfigResponseAlias    = WorkspaceConfigGetResponse
	DeleteConfigResponseAlias = WorkspaceConfigDeleteResponse
	ListConfigsResponseAlias  = WorkspaceConfigsListResponse

	// Publish operations
	PublishRequestAlias        = WorkspacePublishRequest
	PublishResponseAlias       = WorkspacePublishResponse
	UnpublishRequestAlias      = WorkspaceUnpublishRequest
	UnpublishResponseAlias     = WorkspaceUnpublishResponse
	ListPublishedRequestAlias  = WorkspacePublishedListRequest
	ListPublishedResponseAlias = WorkspacePublishedListResponse

	// Version operations
	VersionsRequestAlias       = WorkspaceVersionsRequest
	VersionsResponseAlias      = WorkspaceVersionsResponse
	VersionDetailRequestAlias  = WorkspaceVersionDetailRequest
	VersionDetailResponseAlias = WorkspaceVersionDetailResponse
	RollbackRequestAlias       = WorkspaceRollbackRequest
	RollbackResponseAlias      = WorkspaceRollbackResponse
)

// Aliases for WorkspaceVersionRecord
type WorkspaceVersionRecordAlias = WorkspaceVersionRecordCanonical
