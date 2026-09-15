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
	// TimeoutMs 同步调用契约预算（毫秒，x-timeout-ms 扩展）；0 = 未声明。
	TimeoutMs  int    `json:"timeoutMs,omitempty"`
	Bound      bool   `json:"bound"`
	BindingID  string `json:"bindingId,omitempty"`
	FunctionID string `json:"functionId,omitempty"`
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
	// Summary 上传即成页管线摘要（D4 后半、T5）：仅 CreateSource/
	// UpdateSource 响应携带，GetSource 恒为 nil（wire 增量字段）。
	Summary *OpenAPISourcePipelineSummary `json:"summary,omitempty"`
}

// OpenAPISourcePipelineSummary 上传即成页管线摘要：单请求内
// 解析→契约→组件模板→页面提案 全链生成后的观测计数（lowerCamelCase
// 契约命名规范）。计数口径：
//   - Operations：文档解析出的 operation 总数；
//   - ContractsCreated：本次上传新建的 unbound 契约数（重传幂等为 0）；
//   - TemplatesUpdated：内容新建/变化的内置组件模板数（T2 契约联动 +
//     提交后显式重建的合并结果，按 key→digest 快照 diff）；
//   - ProposalsCreated：本次上传新生成的页面提案数（按 scope 提案 key
//     快照 diff，更新不计入）；
//   - Diagnostics：解析诊断透传（info/warning；error 级已在上传入口拒绝）。
type OpenAPISourcePipelineSummary struct {
	Operations       int               `json:"operations"`
	ContractsCreated int               `json:"contractsCreated"`
	TemplatesUpdated int               `json:"templatesUpdated"`
	ProposalsCreated int               `json:"proposalsCreated"`
	Diagnostics      []spec.Diagnostic `json:"diagnostics,omitempty"`
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

// RuntimeProviderItem 是一条运行时导入：当前 scope 下某个 Agent 侧
// openapi provider（providers.yaml 或扩展下发）注册的函数集合。
// 与 OpenAPI Source（控制台上传的契约候选）相对，这里展示的是
// 已经在运行时注册、可直接绑定为 kind=provider 的函数来源。
type RuntimeProviderItem struct {
	ProviderID    string   `json:"providerId"`
	Name          string   `json:"name"`
	AgentID       string   `json:"agentId"`
	GameID        string   `json:"gameId"`
	Env           string   `json:"env"`
	Version       string   `json:"version,omitempty"`
	FunctionCount int      `json:"functionCount"`
	Functions     []string `json:"functions"`
	LastSeenUnix  int64    `json:"lastSeenUnix"`
}

type RuntimeSourcesListRequest struct{}

type RuntimeSourcesListResponse struct {
	Items []RuntimeProviderItem `json:"items"`
	Total int                   `json:"total"`
}
