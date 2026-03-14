package component

import "github.com/cuihairu/croupier/internal/pack"

// Component represents a component in the system
type Component struct {
	ID           string                   `json:"id"`
	Name         string                   `json:"name"`
	Version      string                   `json:"version"`
	Description  string                   `json:"description"`
	Category     string                   `json:"category"`
	Dependencies []string                 `json:"dependencies"`
	Functions    []pack.ComponentFunction `json:"functions"`
	Author       string                   `json:"author"`
	License      string                   `json:"license"`
	Status       string                   `json:"status"`
	Path         string                   `json:"path,omitempty"`
}

// ComponentActionRequest is the request to perform an action on a component
type ComponentActionRequest struct {
	ID string `uri:"id"`
}

// ComponentDetailRequest is the request to get component details
type ComponentDetailRequest struct {
	ID string `uri:"id"`
}

// ComponentPatchRequest is the request to patch a component
type ComponentPatchRequest struct {
	ID    string      `uri:"id"`
	Patch interface{} `json:"patch"`
}

// ComponentsDeleteResponse is the response from deleting components
type ComponentsDeleteResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// ComponentsDetailResponse is the response with component details
type ComponentsDetailResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// ComponentsDisableResponse is the response from disabling components
type ComponentsDisableResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// ComponentsEnableResponse is the response from enabling components
type ComponentsEnableResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// ComponentsInstallRequest is the request to install components
type ComponentsInstallRequest struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// ComponentsInstallResponse is the response from installing components
type ComponentsInstallResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// ComponentsListRequest is the request to list components
type ComponentsListRequest struct {
	Page     int `form:"page"`
	PageSize int `form:"pageSize"`
}

// ComponentsListResponse is the response with component list
type ComponentsListResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// ComponentsPatchResponse is the response from patching components
type ComponentsPatchResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Service types for component operations

// ListRequest is the request to list components
type ListRequest struct {
	Page     int `form:"page"`
	PageSize int `form:"pageSize"`
}

// ListResponse is the response with component list
type ListResponse struct {
	Items  []Component    `json:"items"`
	Total  int            `json:"total"`
	Page   int            `json:"page"`
	Size   int            `json:"size"`
	Counts map[string]int `json:"counts"`
}

// InstallRequest is the request to install a component
type InstallRequest struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// InstallResponse is the response from installing a component
type InstallResponse struct {
	Component Component `json:"component"`
}

// GetRequest is the request to get component details
type GetRequest struct {
	ID string `uri:"id"`
}

// GetResponse is the response with component details
type GetResponse struct {
	Component Component `json:"component"`
}

// EnableRequest is the request to enable a component
type EnableRequest struct {
	ID string `uri:"id"`
}

// EnableResponse is the response from enabling a component
type EnableResponse struct {
	Component Component `json:"component"`
}

// DisableRequest is the request to disable a component
type DisableRequest struct {
	ID string `uri:"id"`
}

// DisableResponse is the response from disabling a component
type DisableResponse struct {
	Component Component `json:"component"`
}

// DeleteRequest is the request to delete a component
type DeleteRequest struct {
	ID string `uri:"id"`
}

// DeleteResponse is the response from deleting a component
type DeleteResponse struct {
	Component Component `json:"component"`
}

// PatchRequest is the request to patch a component
type PatchRequest struct {
	ID    string      `uri:"id"`
	Patch interface{} `json:"patch"`
}

// PatchResponse is the response from patching a component
type PatchResponse struct {
	Component Component `json:"component"`
}
