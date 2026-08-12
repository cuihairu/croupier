package openapi

import (
	"encoding/json"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
)

// Descriptor represents a function descriptor
type Descriptor struct {
	Id          string          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Resource    string          `json:"resource,omitempty"`
	Operation   string          `json:"operation,omitempty"`
	Schema      json.RawMessage `json:"schema"`
}

// DescriptorsRequest is the request to get descriptors
type DescriptorsRequest struct {
	Type   string `form:"type"`
	GameId string `form:"gameId"`
}

// DescriptorsResponse is the response with descriptors
type DescriptorsResponse struct {
	Items []Descriptor `json:"items"`
}

// OpenAPIDocumentRequest is the request to get aggregated OpenAPI document
type OpenAPIDocumentRequest struct{}

// OpenAPIDocumentResponse is the response with aggregated OpenAPI document
type OpenAPIDocumentResponse struct {
	Spec json.RawMessage `json:"spec"` // OpenAPI 3.x Document
}

// OpenAPISpecRequest is the request to get function OpenAPI spec
type OpenAPISpecRequest struct {
	ID string `uri:"id" binding:"required"`
}

// OpenAPISpecResponse is the response with function OpenAPI spec
type OpenAPISpecResponse struct {
	Spec json.RawMessage `json:"spec"` // OpenAPI 3.x Operation Object
}

// GetSpecRequest is the request to get function OpenAPI spec
// Deprecated: Use OpenAPISpecRequest instead
type GetSpecRequest = OpenAPISpecRequest

// GetSpecResponse is the response with function OpenAPI spec
// Deprecated: Use OpenAPISpecResponse instead
type GetSpecResponse = OpenAPISpecResponse

type BatchGetSpecRequest struct {
	FunctionIDs []string `json:"functionIds" binding:"required"`
}

type BatchGetSpecResponse map[string]json.RawMessage

type OpenAPISourceCreateRequest struct {
	Name string          `json:"name,omitempty"`
	Spec json.RawMessage `json:"spec" binding:"required"`
}

type OpenAPISourceUpdateRequest struct {
	SourceID string          `uri:"sourceId" binding:"required"`
	Name     string          `json:"name,omitempty"`
	Spec     json.RawMessage `json:"spec" binding:"required"`
}

type OpenAPISourceSummary struct {
	SourceID        string            `json:"sourceId"`
	GameID          string            `json:"gameId,omitempty"`
	Env             string            `json:"env,omitempty"`
	Name            string            `json:"name"`
	Revision        int               `json:"revision"`
	Format          string            `json:"format"`
	OpenAPIVersion  string            `json:"openapiVersion"`
	InfoTitle       string            `json:"infoTitle,omitempty"`
	InfoVersion     string            `json:"infoVersion,omitempty"`
	ContentHash     string            `json:"contentHash"`
	OperationCount  int               `json:"operationCount"`
	DiagnosticCount int               `json:"diagnosticCount"`
	CreatedAt       string            `json:"createdAt"`
	UpdatedAt       string            `json:"updatedAt"`
	Diagnostics     []spec.Diagnostic `json:"diagnostics,omitempty"`
}

type OpenAPISourceOperation struct {
	OperationID string                 `json:"operationId"`
	Method      string                 `json:"method"`
	Path        string                 `json:"path"`
	Summary     string                 `json:"summary,omitempty"`
	Description string                 `json:"description,omitempty"`
	Tags        []string               `json:"tags,omitempty"`
	Operation   string                 `json:"operation,omitempty"`
	Resource    string                 `json:"resource,omitempty"`
	Capability  spec.CapabilityKind    `json:"capability,omitempty"`
	Execution   spec.FunctionExecution `json:"execution,omitempty"`
	Approval    spec.ApprovalPolicy    `json:"approval"`
	Risk        spec.RiskLevel         `json:"risk,omitempty"`
	Permission  string                 `json:"permission,omitempty"`
	Bound       bool                   `json:"bound"`
	BindingID   string                 `json:"bindingId,omitempty"`
	FunctionID  string                 `json:"functionId,omitempty"`
}

type OpenAPISourceDetail struct {
	OpenAPISourceSummary
	Spec       json.RawMessage           `json:"spec,omitempty"`
	Operations []OpenAPISourceOperation  `json:"operations"`
	Bindings   []OpenAPISourceBindingDTO `json:"bindings,omitempty"`
}

type OpenAPISourceListRequest struct{}

type OpenAPISourceListResponse struct {
	Items []OpenAPISourceSummary `json:"items"`
}

type OpenAPISourceGetRequest struct {
	SourceID string `uri:"sourceId" binding:"required"`
}

type OpenAPISourceGetResponse struct {
	Source OpenAPISourceDetail `json:"source"`
}

type OpenAPISourceDiagnosticsResponse struct {
	SourceID    string            `json:"sourceId"`
	Diagnostics []spec.Diagnostic `json:"diagnostics"`
}

type OpenAPISourceBindingCreateRequest struct {
	SourceID    string `uri:"sourceId" binding:"required"`
	BindingID   string `json:"bindingId,omitempty"`
	OperationID string `json:"operationId" binding:"required"`
	Kind        string `json:"kind" binding:"required"`
	FunctionID  string `json:"functionId,omitempty"`
	ProviderID  string `json:"providerId,omitempty"`
}

type OpenAPISourceBindingDeleteRequest struct {
	SourceID  string `uri:"sourceId" binding:"required"`
	BindingID string `uri:"bindingId" binding:"required"`
}

type OpenAPISourceBindingDTO struct {
	BindingID   string `json:"bindingId"`
	OperationID string `json:"operationId"`
	Kind        string `json:"kind"`
	FunctionID  string `json:"functionId,omitempty"`
	ProviderID  string `json:"providerId,omitempty"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
}

type OpenAPIBindingProposalDTO struct {
	ProposalKey string `json:"proposalKey"`
	PageKey     string `json:"pageKey"`
	PageType    string `json:"pageType"`
	ResourceKey string `json:"resourceKey,omitempty"`
	Quality     string `json:"quality"`
	Status      string `json:"status"`
}

type OpenAPISourceBindingResponse struct {
	Binding  OpenAPISourceBindingDTO    `json:"binding"`
	Proposal *OpenAPIBindingProposalDTO `json:"proposal,omitempty"`
}

// GetDocumentRequest is the request to get aggregated OpenAPI document
// Deprecated: Use OpenAPIDocumentRequest instead
type GetDocumentRequest = OpenAPIDocumentRequest

// GetDocumentResponse is the response with aggregated OpenAPI document
// Deprecated: Use OpenAPIDocumentResponse instead
type GetDocumentResponse = OpenAPIDocumentResponse
