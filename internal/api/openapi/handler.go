package openapi

import (
	"github.com/cuihairu/croupier/internal/common/response"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// GetSpec handles the request to get function OpenAPI spec
func (h *Handler) GetSpec(c *gin.Context) {
	var req GetSpecRequest
	if err := c.ShouldBindUri(&req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.GetSpec(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// Import handles the request to import OpenAPI spec
func (h *Handler) Import(c *gin.Context) {
	var req ImportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.Import(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// EntityFunctions handles the request to get entity functions
func (h *Handler) EntityFunctions(c *gin.Context) {
	var req EntityFunctionsRequest
	if err := c.ShouldBindUri(&req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.EntityFunctions(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// EntityIndex handles the request to list all entities derived from function registrations
func (h *Handler) EntityIndex(c *gin.Context) {
	var req EntityIndexRequest
	_ = c.ShouldBindQuery(&req)

	resp, err := h.service.EntityIndex(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// EntityFunctionsByName handles the request to get functions for an entity by name
func (h *Handler) EntityFunctionsByName(c *gin.Context) {
	name := c.Param("name")
	resp, err := h.service.EntityFunctionsByName(c.Request.Context(), name)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// GetDocument handles the request to get aggregated OpenAPI document
func (h *Handler) GetDocument(c *gin.Context) {
	var req GetDocumentRequest
	resp, err := h.service.GetDocument(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) BatchGetSpec(c *gin.Context) {
	var req BatchGetSpecRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.BatchGetSpec(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}
