package resource

import "github.com/cuihairu/croupier/internal/dashboard/spec"

// ResourceListRequest is the request to list resources.
type ResourceListRequest struct {
	Category string `form:"category"`
	Query    string `form:"q"`
}

// ResourceListResponse is the response with resource list.
type ResourceListResponse struct {
	Items []spec.ResourceSpec `json:"items"`
}

// ResourceDetailRequest is the request to get a resource.
type ResourceDetailRequest struct {
	ResourceKey string `uri:"resourceKey" binding:"required"`
}

// ResourceDetailResponse is the response with resource detail.
type ResourceDetailResponse struct {
	Resource spec.ResourceSpec `json:"resource"`
}

// ResourceOperationsRequest is the request to list resource operations.
type ResourceOperationsRequest struct {
	ResourceKey string `uri:"resourceKey" binding:"required"`
}

// ResourceOperationsResponse is the response with resource operations.
type ResourceOperationsResponse struct {
	Items []spec.OperationSpec `json:"items"`
}

// ResourceGeneratedPagesRequest is the request to get generated pages.
type ResourceGeneratedPagesRequest struct {
	ResourceKey string `uri:"resourceKey" binding:"required"`
}

// ResourceGeneratedPagesResponse is the response with generated pages.
type ResourceGeneratedPagesResponse struct {
	Items       []spec.GeneratedPageSpec `json:"items"`
	Diagnostics []spec.Diagnostic        `json:"diagnostics,omitempty"`
}
