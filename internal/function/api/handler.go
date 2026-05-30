// Package api provides REST API handlers for function metadata management.
package api

import (
	"fmt"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/common/response"
	functionv1 "github.com/cuihairu/croupier/pkg/pb/croupier/function/v1"
	"github.com/gin-gonic/gin"
)

// Handler handles function registry HTTP requests.
type Handler struct {
	service *Service
}

// NewHandler creates a new function registry handler.
func NewHandler(service *Service) *Handler {
	return &Handler{
		service: service,
	}
}

// ListFunctions handles GET /api/metadata/functions - List all functions with optional filters.
func (h *Handler) ListFunctions(c *gin.Context) {
	var req ListFunctionsRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Error(c, err)
		return
	}

	// Build options from request
	opts := &ListOptions{}
	if req.Category != "" {
		opts.Category = req.Category
	}
	if req.Tag != "" {
		opts.Tag = req.Tag
	}
	if req.Risk != "" {
		opts.RiskLevel = req.Risk
	}
	if req.Mode != "" {
		opts.Mode = req.Mode
	}

	result, err := h.service.List(c.Request.Context(), opts)
	if err != nil {
		response.Error(c, err)
		return
	}

	// Convert to DTO
	functions := make([]*FunctionMetadata, 0, len(result.Functions))
	for _, metadata := range result.Functions {
		functions = append(functions, ProtoToMetadata(metadata))
	}

	response.Success(c, ListFunctionsResponse{
		Functions: functions,
		Total:     int64(result.Total),
	})
}

// GetFunction handles GET /api/metadata/functions/:id - Get a single function by ID.
func (h *Handler) GetFunction(c *gin.Context) {
	var req GetFunctionRequest
	if err := c.ShouldBindUri(&req); err != nil {
		response.Error(c, err)
		return
	}

	metadata, err := h.service.Get(c.Request.Context(), req.ID)
	if err != nil {
		response.Error(c, errorx.NewNotFound("function not found"))
		return
	}

	response.Success(c, GetFunctionResponse{
		Function: ProtoToMetadata(metadata),
	})
}

// RegisterFunction handles POST /api/metadata/functions - Register a new function.
func (h *Handler) RegisterFunction(c *gin.Context) {
	var req RegisterFunctionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, err)
		return
	}

	// Convert to proto
	metadata := MetadataToProto(&FunctionMetadata{
		ID:           req.ID,
		Version:      req.Version,
		Category:     req.Category,
		Tags:         req.Tags,
		Name:         req.Name,
		Description:  req.Description,
		InputSchema:  req.InputSchema,
		OutputSchema: req.OutputSchema,
		Behavior:     req.Behavior,
		Security:     req.Security,
		Extensions:   req.Extensions,
	})

	// Validate and register
	if err := h.service.Register(c.Request.Context(), metadata); err != nil {
		response.Error(c, errorx.NewConflict(err.Error()))
		return
	}

	response.Created(c, RegisterFunctionResponse{
		Function: ProtoToMetadata(metadata),
	})
}

// UpdateFunction handles PUT /api/metadata/functions/:id - Update an existing function.
func (h *Handler) UpdateFunction(c *gin.Context) {
	var uriReq UpdateFunctionRequest
	if err := c.ShouldBindUri(&uriReq); err != nil {
		response.Error(c, err)
		return
	}

	// Define a body-only type to avoid ID validation issue
	type updateFunctionBody struct {
		Name         *string           `json:"name"`
		Description  *string           `json:"description"`
		InputSchema  *string           `json:"input_schema"`
		OutputSchema *string           `json:"output_schema"`
		Behavior     *FunctionBehavior `json:"behavior"`
		Security     *FunctionSecurity `json:"security"`
		Extensions   map[string]string `json:"extensions"`
	}

	var body updateFunctionBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Error(c, err)
		return
	}

	// Get existing metadata first
	metadata, err := h.service.Get(c.Request.Context(), uriReq.ID)
	if err != nil {
		response.Error(c, errorx.NewNotFound("function not found"))
		return
	}

	// Apply updates
	if body.Name != nil {
		metadata.Name = *body.Name
	}
	if body.Description != nil {
		metadata.Description = *body.Description
	}
	if body.InputSchema != nil {
		metadata.InputSchema = *body.InputSchema
	}
	if body.OutputSchema != nil {
		metadata.OutputSchema = *body.OutputSchema
	}
	if body.Behavior != nil {
		metadata.Behavior = &functionv1.FunctionBehavior{
			Mode:            parseMode(body.Behavior.Mode),
			Idempotent:      body.Behavior.Idempotent,
			TimeoutMs:       body.Behavior.TimeoutMs,
			RouteStrategy:   parseRouteStrategy(body.Behavior.RouteStrategy),
			Cacheable:       body.Behavior.Cacheable,
			CacheTtlSeconds: body.Behavior.CacheTtlSeconds,
		}
	}
	if body.Security != nil {
		metadata.Security = &functionv1.FunctionSecurity{
			RiskLevel:         parseRiskLevel(body.Security.RiskLevel),
			Permission:        body.Security.Permission,
			RequiresApproval:  body.Security.RequiresApproval,
			ApprovalType:      parseApprovalType(body.Security.ApprovalType),
			AllowedRoles:      body.Security.AllowedRoles,
			AuditLog:          body.Security.AuditLog,
			MaskSensitiveData: body.Security.MaskSensitiveData,
		}
	}
	if body.Extensions != nil {
		metadata.Extensions = body.Extensions
	}

	// Update
	if err := h.service.Update(c.Request.Context(), uriReq.ID, metadata); err != nil {
		response.Error(c, err)
		return
	}

	response.Success(c, UpdateFunctionResponse{
		Function: ProtoToMetadata(metadata),
	})
}

// DeleteFunction handles DELETE /api/metadata/functions/:id - Delete a function.
func (h *Handler) DeleteFunction(c *gin.Context) {
	var req DeleteFunctionRequest
	if err := c.ShouldBindUri(&req); err != nil {
		response.Error(c, err)
		return
	}

	if err := h.service.Delete(c.Request.Context(), req.ID); err != nil {
		response.Error(c, errorx.NewNotFound("function not found"))
		return
	}

	response.NoContent(c)
}

// ImportFromOpenAPI handles POST /api/metadata/functions/import/openapi - Import functions from OpenAPI spec.
func (h *Handler) ImportFromOpenAPI(c *gin.Context) {
	var req ImportFromOpenAPIRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, err)
		return
	}

	var opts *ImportOptions
	if req.Options != nil {
		opts = &ImportOptions{
			CategoryPrefix:   req.Options.CategoryPrefix,
			TagPrefix:        req.Options.TagPrefix,
			DefaultTimeoutMs: req.Options.DefaultTimeoutMs,
			ContinueOnError:  req.Options.ContinueOnError,
		}
	}

	metadatas, err := h.service.ImportFromOpenAPI(c.Request.Context(), req.Spec, opts)
	if err != nil {
		response.Error(c, errorx.NewBadRequest(fmt.Sprintf("failed to import: %v", err)))
		return
	}

	// Convert to DTO
	functions := make([]*FunctionMetadata, 0, len(metadatas))
	for _, metadata := range metadatas {
		functions = append(functions, ProtoToMetadata(metadata))
	}

	response.Success(c, ImportFromOpenAPIResponse{
		Functions:     functions,
		ImportedCount: len(functions),
	})
}

// GetCategories handles GET /api/metadata/functions/categories - List all categories.
func (h *Handler) GetCategories(c *gin.Context) {
	categories := h.service.GetCategories(c.Request.Context())
	response.Success(c, gin.H{
		"categories": categories,
	})
}

// GetTags handles GET /api/metadata/functions/tags - List all tags.
func (h *Handler) GetTags(c *gin.Context) {
	tags := h.service.GetTags(c.Request.Context())
	response.Success(c, gin.H{
		"tags": tags,
	})
}
