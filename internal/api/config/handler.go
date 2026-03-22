package config

import (
	"strconv"

	"github.com/cuihairu/croupier/internal/common/requestbind"
	"github.com/cuihairu/croupier/internal/common/response"
	"github.com/gin-gonic/gin"
)

func bindConfigRequest(c *gin.Context, req interface{}) error {
	if c.Request.Method == "GET" {
		return requestbind.BindQueryCompat(c, req)
	}
	return c.ShouldBindJSON(req)
}

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func configIDFromPath(c *gin.Context) string {
	return c.Param("id")
}

// List handles GET /api/v1/configs.
func (h *Handler) List(c *gin.Context) {
	var req ListConfigsRequest
	if err := bindConfigRequest(c, &req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	resp, err := h.service.ListConfigs(c.Request.Context(), &req)
	if err != nil {
		response.InternalServerError(c, err.Error())
		return
	}
	response.Success(c, resp)
}

// Get handles GET /api/v1/configs/:id.
func (h *Handler) Get(c *gin.Context) {
	resp, err := h.service.GetConfig(c.Request.Context(), &GetConfigRequest{ID: configIDFromPath(c)})
	if err != nil {
		response.InternalServerError(c, err.Error())
		return
	}
	response.Success(c, resp)
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

// Save handles PUT /api/v1/configs/:id.
func (h *Handler) Save(c *gin.Context) {
	var req SaveConfigRequest
	if err := bindConfigRequest(c, &req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	resp, err := h.service.SaveConfig(c.Request.Context(), configIDFromPath(c), &req)
	if err != nil {
		response.InternalServerError(c, err.Error())
		return
	}
	response.Success(c, resp)
}

// Validate handles POST /api/v1/configs/:id/validate.
func (h *Handler) Validate(c *gin.Context) {
	var req ValidateConfigRequest
	if err := bindConfigRequest(c, &req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	resp, err := h.service.ValidateConfig(c.Request.Context(), configIDFromPath(c), &req)
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

// ListVersionsByID handles GET /api/v1/configs/:id/versions.
func (h *Handler) ListVersionsByID(c *gin.Context) {
	resp, err := h.service.ListVersions(c.Request.Context(), &ListVersionsRequest{Key: configIDFromPath(c)})
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

// GetVersionByID handles GET /api/v1/configs/:id/versions/:version.
func (h *Handler) GetVersionByID(c *gin.Context) {
	version, err := strconv.Atoi(c.Param("version"))
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	resp, err := h.service.GetVersion(c.Request.Context(), &GetVersionRequest{
		Key:     configIDFromPath(c),
		Version: version,
	})
	if err != nil {
		response.InternalServerError(c, err.Error())
		return
	}
	response.Success(c, resp)
}
