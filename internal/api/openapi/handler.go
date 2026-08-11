package openapi

import (
	"encoding/json"
	"io"
	"strings"

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

func (h *Handler) CreateSource(c *gin.Context) {
	if strings.HasPrefix(strings.ToLower(c.GetHeader("Content-Type")), "multipart/form-data") {
		file, header, err := c.Request.FormFile("file")
		if err != nil {
			response.Error(c, err)
			return
		}
		defer file.Close()
		name := strings.TrimSpace(c.PostForm("name"))
		if name == "" && header != nil {
			name = header.Filename
		}
		resp, err := h.service.CreateSourceFromMultipart(c.Request.Context(), name, file)
		if err != nil {
			response.Error(c, err)
			return
		}
		response.Created(c, resp)
		return
	}

	data, err := io.ReadAll(io.LimitReader(c.Request.Body, maxOpenAPISourceBytes+1))
	if err != nil {
		response.Error(c, err)
		return
	}
	if len(data) > maxOpenAPISourceBytes {
		response.BadRequest(c, "OpenAPI source exceeds 2 MiB limit")
		return
	}
	req := OpenAPISourceCreateRequest{Spec: json.RawMessage(data)}
	if json.Valid(data) {
		var envelope OpenAPISourceCreateRequest
		if err := json.Unmarshal(data, &envelope); err == nil && len(envelope.Spec) > 0 {
			req = envelope
		}
	}
	resp, err := h.service.CreateSource(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, resp)
}

func (h *Handler) UpdateSource(c *gin.Context) {
	var uriReq OpenAPISourceGetRequest
	if err := c.ShouldBindUri(&uriReq); err != nil {
		response.Error(c, err)
		return
	}
	data, err := io.ReadAll(io.LimitReader(c.Request.Body, maxOpenAPISourceBytes+1))
	if err != nil {
		response.Error(c, err)
		return
	}
	if len(data) > maxOpenAPISourceBytes {
		response.BadRequest(c, "OpenAPI source exceeds 2 MiB limit")
		return
	}
	req := OpenAPISourceUpdateRequest{
		SourceID: uriReq.SourceID,
		Spec:     json.RawMessage(data),
	}
	if json.Valid(data) {
		var envelope OpenAPISourceUpdateRequest
		if err := json.Unmarshal(data, &envelope); err == nil && len(envelope.Spec) > 0 {
			req.Name = envelope.Name
			req.Spec = envelope.Spec
		}
	}
	resp, err := h.service.UpdateSource(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) ListSources(c *gin.Context) {
	resp, err := h.service.ListSources(c.Request.Context(), &OpenAPISourceListRequest{})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) GetSource(c *gin.Context) {
	var req OpenAPISourceGetRequest
	if err := c.ShouldBindUri(&req); err != nil {
		response.Error(c, err)
		return
	}
	resp, err := h.service.GetSource(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) SourceDiagnostics(c *gin.Context) {
	var req OpenAPISourceGetRequest
	if err := c.ShouldBindUri(&req); err != nil {
		response.Error(c, err)
		return
	}
	resp, err := h.service.SourceDiagnostics(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) CreateBinding(c *gin.Context) {
	var req OpenAPISourceBindingCreateRequest
	req.SourceID = c.Param("sourceId")
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, err)
		return
	}
	resp, err := h.service.CreateBinding(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) DeleteBinding(c *gin.Context) {
	var req OpenAPISourceBindingDeleteRequest
	if err := c.ShouldBindUri(&req); err != nil {
		response.Error(c, err)
		return
	}
	resp, err := h.service.DeleteBinding(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}
