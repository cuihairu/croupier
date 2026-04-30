package api

import (
	functionv1 "github.com/cuihairu/croupier/pkg/pb/croupier/function/v1"
)

// ListFunctionsRequest represents a request to list functions with optional filters.
type ListFunctionsRequest struct {
	Category string `form:"category" json:"category"`
	Tag      string `form:"tag" json:"tag"`
	Risk     string `form:"risk" json:"risk"`
	Mode     string `form:"mode" json:"mode"`
	Page     int    `form:"page" json:"page" binding:"min=1"`
	PageSize int    `form:"pageSize" json:"pageSize" binding:"min=1,max=100"`
}

// ListFunctionsResponse represents the response for listing functions.
type ListFunctionsResponse struct {
	Functions []*FunctionMetadata `json:"functions"`
	Total     int64               `json:"total"`
	Page      int                 `json:"page"`
	PageSize  int                 `json:"pageSize"`
}

// GetFunctionRequest represents a request to get a single function by ID.
type GetFunctionRequest struct {
	ID string `uri:"id" binding:"required"`
}

// GetFunctionResponse represents the response for getting a function.
type GetFunctionResponse struct {
	Function *FunctionMetadata `json:"function"`
}

// RegisterFunctionRequest represents a request to register a function.
type RegisterFunctionRequest struct {
	ID           string            `json:"id" binding:"required"`
	Version      string            `json:"version"`
	Category     string            `json:"category"`
	Tags         []string          `json:"tags"`
	Name         string            `json:"name" binding:"required"`
	Description  string            `json:"description"`
	InputSchema  string            `json:"input_schema"`
	OutputSchema string            `json:"output_schema"`
	Behavior     *FunctionBehavior `json:"behavior"`
	Security     *FunctionSecurity `json:"security"`
	Extensions   map[string]string `json:"extensions"`
}

// RegisterFunctionResponse represents the response for registering a function.
type RegisterFunctionResponse struct {
	Function *FunctionMetadata `json:"function"`
}

// UpdateFunctionRequest represents a request to update a function.
type UpdateFunctionRequest struct {
	ID           string            `uri:"id" binding:"required"`
	Name         *string           `json:"name"`
	Description  *string           `json:"description"`
	InputSchema  *string           `json:"input_schema"`
	OutputSchema *string           `json:"output_schema"`
	Behavior     *FunctionBehavior `json:"behavior"`
	Security     *FunctionSecurity `json:"security"`
	Extensions   map[string]string `json:"extensions"`
}

// UpdateFunctionResponse represents the response for updating a function.
type UpdateFunctionResponse struct {
	Function *FunctionMetadata `json:"function"`
}

// DeleteFunctionRequest represents a request to delete a function.
type DeleteFunctionRequest struct {
	ID string `uri:"id" binding:"required"`
}

// FunctionMetadata represents a function metadata for API responses.
type FunctionMetadata struct {
	ID           string            `json:"id"`
	Version      string            `json:"version"`
	Category     string            `json:"category"`
	Tags         []string          `json:"tags"`
	Name         string            `json:"name"`
	Description  string            `json:"description"`
	InputSchema  string            `json:"input_schema"`
	OutputSchema string            `json:"output_schema"`
	Behavior     *FunctionBehavior `json:"behavior"`
	Security     *FunctionSecurity `json:"security"`
	Extensions   map[string]string `json:"extensions"`
	CreatedAt    string            `json:"created_at,omitempty"`
	UpdatedAt    string            `json:"updated_at,omitempty"`
}

// FunctionBehavior represents function behavior in API responses.
type FunctionBehavior struct {
	Mode            string `json:"mode"`
	Idempotent      bool   `json:"idempotent"`
	TimeoutMs       int32  `json:"timeout_ms"`
	RouteStrategy   string `json:"route_strategy"`
	Cacheable       bool   `json:"cacheable"`
	CacheTtlSeconds int32  `json:"cache_ttl_seconds,omitempty"`
}

// FunctionSecurity represents function security in API responses.
type FunctionSecurity struct {
	RiskLevel         string   `json:"risk_level"`
	Permission        string   `json:"permission,omitempty"`
	RequiresApproval  bool     `json:"requires_approval"`
	ApprovalType      string   `json:"approval_type,omitempty"`
	AllowedRoles      []string `json:"allowed_roles,omitempty"`
	AuditLog          bool     `json:"audit_log"`
	MaskSensitiveData bool     `json:"mask_sensitive_data"`
}

// ImportFromOpenAPIRequest represents a request to import functions from OpenAPI.
type ImportFromOpenAPIRequest struct {
	Spec    []byte         `json:"spec" binding:"required"`
	Options *ImportOptions `json:"options"`
}

// ImportFromOpenAPIResponse represents the response for importing functions.
type ImportFromOpenAPIResponse struct {
	Functions     []*FunctionMetadata `json:"functions"`
	ImportedCount int                 `json:"imported_count"`
	FailedCount   int                 `json:"failed_count"`
	Errors        []string            `json:"errors,omitempty"`
}

// ImportOptions controls the import behavior.
type ImportOptions struct {
	CategoryPrefix   string `json:"category_prefix,omitempty"`
	TagPrefix        string `json:"tag_prefix,omitempty"`
	DefaultTimeoutMs int32  `json:"default_timeout_ms,omitempty"`
	ContinueOnError  bool   `json:"continue_on_error,omitempty"`
}

// ProtoToMetadata converts a proto FunctionMetadata to API DTO.
func ProtoToMetadata(pb *functionv1.FunctionMetadata) *FunctionMetadata {
	if pb == nil {
		return nil
	}

	metadata := &FunctionMetadata{
		ID:           pb.Id,
		Version:      pb.Version,
		Category:     pb.Category,
		Tags:         pb.Tags,
		Name:         pb.Name,
		Description:  pb.Description,
		InputSchema:  pb.InputSchema,
		OutputSchema: pb.OutputSchema,
		Extensions:   pb.Extensions,
	}

	if pb.Behavior != nil {
		metadata.Behavior = &FunctionBehavior{
			Mode:            normalizeMode(pb.Behavior.Mode),
			Idempotent:      pb.Behavior.Idempotent,
			TimeoutMs:       pb.Behavior.TimeoutMs,
			RouteStrategy:   normalizeRouteStrategy(pb.Behavior.RouteStrategy),
			Cacheable:       pb.Behavior.Cacheable,
			CacheTtlSeconds: pb.Behavior.CacheTtlSeconds,
		}
	}

	if pb.Security != nil {
		metadata.Security = &FunctionSecurity{
			RiskLevel:         normalizeRiskLevel(pb.Security.RiskLevel),
			Permission:        pb.Security.Permission,
			RequiresApproval:  pb.Security.RequiresApproval,
			ApprovalType:      normalizeApprovalType(pb.Security.ApprovalType),
			AllowedRoles:      pb.Security.AllowedRoles,
			AuditLog:          pb.Security.AuditLog,
			MaskSensitiveData: pb.Security.MaskSensitiveData,
		}
	}

	return metadata
}

// MetadataToProto converts API DTO to proto FunctionMetadata.
func MetadataToProto(dto *FunctionMetadata) *functionv1.FunctionMetadata {
	if dto == nil {
		return nil
	}

	pb := &functionv1.FunctionMetadata{
		Id:           dto.ID,
		Version:      dto.Version,
		Category:     dto.Category,
		Tags:         dto.Tags,
		Name:         dto.Name,
		Description:  dto.Description,
		InputSchema:  dto.InputSchema,
		OutputSchema: dto.OutputSchema,
		Extensions:   dto.Extensions,
	}

	if dto.Behavior != nil {
		pb.Behavior = &functionv1.FunctionBehavior{
			Mode:            parseMode(dto.Behavior.Mode),
			Idempotent:      dto.Behavior.Idempotent,
			TimeoutMs:       dto.Behavior.TimeoutMs,
			RouteStrategy:   parseRouteStrategy(dto.Behavior.RouteStrategy),
			Cacheable:       dto.Behavior.Cacheable,
			CacheTtlSeconds: dto.Behavior.CacheTtlSeconds,
		}
	}

	if dto.Security != nil {
		pb.Security = &functionv1.FunctionSecurity{
			RiskLevel:         parseRiskLevel(dto.Security.RiskLevel),
			Permission:        dto.Security.Permission,
			RequiresApproval:  dto.Security.RequiresApproval,
			ApprovalType:      parseApprovalType(dto.Security.ApprovalType),
			AllowedRoles:      dto.Security.AllowedRoles,
			AuditLog:          dto.Security.AuditLog,
			MaskSensitiveData: dto.Security.MaskSensitiveData,
		}
	}

	return pb
}

// Enum normalization functions

func normalizeMode(mode functionv1.FunctionBehavior_Mode) string {
	switch mode {
	case functionv1.FunctionBehavior_QUERY:
		return "query"
	case functionv1.FunctionBehavior_COMMAND:
		return "command"
	default:
		return "unknown"
	}
}

func normalizeRouteStrategy(strategy functionv1.FunctionBehavior_RouteStrategy) string {
	switch strategy {
	case functionv1.FunctionBehavior_ROUTE_LB:
		return "lb"
	case functionv1.FunctionBehavior_ROUTE_BROADCAST:
		return "broadcast"
	case functionv1.FunctionBehavior_ROUTE_TARGETED:
		return "targeted"
	case functionv1.FunctionBehavior_ROUTE_HASH:
		return "hash"
	default:
		return "unknown"
	}
}

func normalizeRiskLevel(level functionv1.FunctionSecurity_RiskLevel) string {
	switch level {
	case functionv1.FunctionSecurity_RISK_LOW:
		return "low"
	case functionv1.FunctionSecurity_RISK_MEDIUM:
		return "medium"
	case functionv1.FunctionSecurity_RISK_HIGH:
		return "high"
	case functionv1.FunctionSecurity_RISK_DANGER:
		return "danger"
	default:
		return "unknown"
	}
}

func normalizeApprovalType(atype functionv1.FunctionSecurity_ApprovalType) string {
	switch atype {
	case functionv1.FunctionSecurity_APPROVAL_NONE:
		return "none"
	case functionv1.FunctionSecurity_APPROVAL_SINGLE:
		return "single"
	case functionv1.FunctionSecurity_APPROVAL_TWO_PERSON:
		return "two_person"
	default:
		return "unknown"
	}
}

func parseMode(mode string) functionv1.FunctionBehavior_Mode {
	switch mode {
	case "query":
		return functionv1.FunctionBehavior_QUERY
	case "command":
		return functionv1.FunctionBehavior_COMMAND
	default:
		return functionv1.FunctionBehavior_MODE_UNKNOWN
	}
}

func parseRouteStrategy(strategy string) functionv1.FunctionBehavior_RouteStrategy {
	switch strategy {
	case "lb":
		return functionv1.FunctionBehavior_ROUTE_LB
	case "broadcast":
		return functionv1.FunctionBehavior_ROUTE_BROADCAST
	case "targeted":
		return functionv1.FunctionBehavior_ROUTE_TARGETED
	case "hash":
		return functionv1.FunctionBehavior_ROUTE_HASH
	default:
		return functionv1.FunctionBehavior_ROUTE_UNKNOWN
	}
}

func parseRiskLevel(level string) functionv1.FunctionSecurity_RiskLevel {
	switch level {
	case "low":
		return functionv1.FunctionSecurity_RISK_LOW
	case "medium":
		return functionv1.FunctionSecurity_RISK_MEDIUM
	case "high":
		return functionv1.FunctionSecurity_RISK_HIGH
	case "danger":
		return functionv1.FunctionSecurity_RISK_DANGER
	default:
		return functionv1.FunctionSecurity_RISK_UNKNOWN
	}
}

func parseApprovalType(atype string) functionv1.FunctionSecurity_ApprovalType {
	switch atype {
	case "none":
		return functionv1.FunctionSecurity_APPROVAL_NONE
	case "single":
		return functionv1.FunctionSecurity_APPROVAL_SINGLE
	case "two_person":
		return functionv1.FunctionSecurity_APPROVAL_TWO_PERSON
	default:
		return functionv1.FunctionSecurity_APPROVAL_NONE
	}
}
