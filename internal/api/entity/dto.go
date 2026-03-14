// Package entity provides DTOs for entity operations.
package entity

// EntitiesListRequest represents a request to list entities
type EntitiesListRequest struct {
	Page     int    `form:"page"`
	PageSize int    `form:"pageSize"`
	Type     string `form:"type"`
}

// EntitiesListResponse represents the response for listing entities
type EntitiesListResponse struct {
	Items []EntityItem `json:"items"`
	Total int64        `json:"total"`
	Page  int          `json:"page"`
	Size  int          `json:"size"`
}

// EntityItem represents a single entity in the list
type EntityItem struct {
	ID        string      `json:"id"`
	Type      string      `json:"type"`
	Data      interface{} `json:"data"`
	ProviderID string     `json:"providerId,omitempty"`
	Status    int         `json:"status"`
	CreatedAt string      `json:"createdAt"`
	UpdatedAt string      `json:"updatedAt"`
}

// EntityCreateRequest represents a request to create an entity
type EntityCreateRequest struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

// EntityCreateResponse represents the response for creating an entity
type EntityCreateResponse struct {
	ID        string      `json:"id"`
	Type      string      `json:"type"`
	Data      interface{} `json:"data"`
	ProviderID string     `json:"providerId,omitempty"`
	Status    int         `json:"status"`
	CreatedAt string      `json:"createdAt"`
	UpdatedAt string      `json:"updatedAt"`
}

// EntityDeleteRequest represents a request to delete an entity
type EntityDeleteRequest struct {
	ID string `uri:"id"`
}

// EntityDeleteResponse represents the response for deleting an entity
type EntityDeleteResponse struct {
	// No direct fields - deletion success indicated by 200 OK
}

// EntityDetailRequest represents a request to get entity details
type EntityDetailRequest struct {
	ID string `uri:"id"`
}

// EntityDetailResponse represents the response for getting entity details
type EntityDetailResponse struct {
	ID        string      `json:"id"`
	Type      string      `json:"type"`
	Data      interface{} `json:"data"`
	ProviderID string     `json:"providerId,omitempty"`
	Status    int         `json:"status"`
	CreatedAt string      `json:"createdAt"`
	UpdatedAt string      `json:"updatedAt"`
}

// EntityFunction represents a function that can be performed on an entity
type EntityFunction struct {
	Id        string `json:"id"`
	Operation string `json:"operation"` // create/read/update/delete/custom
	Name      string `json:"name"`
}

// EntityFunctionsRequest represents a request to get entity functions
type EntityFunctionsRequest struct {
	ID string `uri:"id"`
}

// EntityFunctionsResponse represents the response for getting entity functions
type EntityFunctionsResponse struct {
	Items []EntityFunction `json:"items"`
}

// EntityPreviewRequest represents a request to preview an entity
type EntityPreviewRequest struct {
	ID string `uri:"id"`
}

// EntityPreviewResponse represents the response for previewing an entity
type EntityPreviewResponse struct {
	Data interface{} `json:"data"`
}

// EntityUpdateRequest represents a request to update an entity
type EntityUpdateRequest struct {
	ID   string      `uri:"id"`
	Data interface{} `json:"data"`
}

// EntityUpdateResponse represents the response for updating an entity
type EntityUpdateResponse struct {
	ID        string      `json:"id"`
	Type      string      `json:"type"`
	Data      interface{} `json:"data"`
	ProviderID string     `json:"providerId,omitempty"`
	Status    int         `json:"status"`
	CreatedAt string      `json:"createdAt"`
	UpdatedAt string      `json:"updatedAt"`
}

// EntityValidateRequest represents a request to validate entity data
type EntityValidateRequest struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

// EntityValidateResponse represents the response for validating entity data
type EntityValidateResponse struct {
	Valid bool `json:"valid"`
}

// ============================================================================
// Types extracted from internal/types/types.go
// These are the original types with Code/Message/Data wrapper structure.
// Type aliases are provided below for backward compatibility.
// ============================================================================

// OriginalEntitiesListRequest is the original request type from types.go
type OriginalEntitiesListRequest struct {
	Page     int    `form:"page"`
	PageSize int    `form:"pageSize"`
	Type     string `form:"type"`
}

// OriginalEntitiesListResponse is the original response type from types.go
type OriginalEntitiesListResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// OriginalEntityCreateRequest is the original request type from types.go
type OriginalEntityCreateRequest struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

// OriginalEntityCreateResponse is the original response type from types.go
type OriginalEntityCreateResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// OriginalEntityDeleteRequest is the original request type from types.go
type OriginalEntityDeleteRequest struct {
	ID string `uri:"id"`
}

// OriginalEntityDeleteResponse is the original response type from types.go
type OriginalEntityDeleteResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// OriginalEntityDetailRequest is the original request type from types.go
type OriginalEntityDetailRequest struct {
	ID string `uri:"id"`
}

// OriginalEntityDetailResponse is the original response type from types.go
type OriginalEntityDetailResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// OriginalEntityFunction is the original function type from types.go
type OriginalEntityFunction struct {
	Id        string `json:"id"`
	Operation string `json:"operation"` // create/read/update/delete/custom
	Name      string `json:"name"`
}

// OriginalEntityFunctionsRequest is the original request type from types.go
type OriginalEntityFunctionsRequest struct {
	ID string `uri:"id"`
}

// OriginalEntityFunctionsResponse is the original response type from types.go
type OriginalEntityFunctionsResponse struct {
	Items []OriginalEntityFunction `json:"items"`
}

// OriginalEntityPreviewRequest is the original request type from types.go
type OriginalEntityPreviewRequest struct {
	ID string `uri:"id"`
}

// OriginalEntityPreviewResponse is the original response type from types.go
type OriginalEntityPreviewResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// OriginalEntityUpdateRequest is the original request type from types.go
type OriginalEntityUpdateRequest struct {
	ID   string      `uri:"id"`
	Data interface{} `json:"data"`
}

// OriginalEntityUpdateResponse is the original response type from types.go
type OriginalEntityUpdateResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// OriginalEntityValidateRequest is the original request type from types.go
type OriginalEntityValidateRequest struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

// OriginalEntityValidateResponse is the original response type from types.go
type OriginalEntityValidateResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}
