package config

import (
	"github.com/cuihairu/croupier/internal/pkg2/response"
	"github.com/gin-gonic/gin"
)

func bindConfigRequest(c *gin.Context, req interface{}) error {
	if c.Request.Method == "GET" {
		return c.ShouldBindQuery(req)
	}
	return c.ShouldBindJSON(req)
}

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// Upsert handles the config create/update request
func (h *Handler) Upsert(c *gin.Context) {
	var req UpsertRequest
	if err := bindConfigRequest(c, &req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	resp, err := h.service.Upsert(c.Request.Context(), &req)
	if err != nil {
		response.InternalServerError(c, err.Error())
		return
	}
	response.Success(c, resp)
}

// ListVersions handles the config versions list request
func (h *Handler) ListVersions(c *gin.Context) {
	var req ListVersionsRequest
	if err := bindConfigRequest(c, &req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	resp, err := h.service.ListVersions(c.Request.Context(), &req)
	if err != nil {
		response.InternalServerError(c, err.Error())
		return
	}
	response.Success(c, resp)
}

// GetVersion handles the config version detail request
func (h *Handler) GetVersion(c *gin.Context) {
	var req GetVersionRequest
	if err := bindConfigRequest(c, &req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	resp, err := h.service.GetVersion(c.Request.Context(), &req)
	if err != nil {
		response.InternalServerError(c, err.Error())
		return
	}
	response.Success(c, resp)
}
