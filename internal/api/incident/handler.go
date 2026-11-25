package incident

import (
	"math"
	"strconv"
	"time"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/common/response"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/gin-gonic/gin"
)

// Handler is the HTTP entry point of incident-reports batch 1
// (category management + incident registration).
type Handler struct {
	service *Service
}

// NewHandler creates an incident handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// ListCategories handles GET /incident-categories?enabledOnly=true.
func (h *Handler) ListCategories(c *gin.Context) {
	resp, err := h.service.ListCategories(c.Request.Context(), c.Query("enabledOnly") == "true")
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// CreateCategory handles POST /incident-categories.
func (h *Handler) CreateCategory(c *gin.Context) {
	var req CategoryUpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, err)
		return
	}
	resp, err := h.service.CreateCategory(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// UpdateCategory handles PUT /incident-categories/:id.
func (h *Handler) UpdateCategory(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		response.Error(c, err)
		return
	}
	var req CategoryUpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, err)
		return
	}
	resp, err := h.service.UpdateCategory(c.Request.Context(), id, &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// CategoryUsage handles GET /incident-categories/:id/usage (delete cascade hint).
func (h *Handler) CategoryUsage(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		response.Error(c, err)
		return
	}
	resp, err := h.service.CategoryUsage(c.Request.Context(), id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// DeleteCategory handles DELETE /incident-categories/:id (cascade merges into uncategorized).
func (h *Handler) DeleteCategory(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		response.Error(c, err)
		return
	}
	if err := h.service.DeleteCategory(c.Request.Context(), id); err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, nil)
}

// ListIncidents handles GET /incidents。
func (h *Handler) ListIncidents(c *gin.Context) {
	opts := model.IncidentQueryOptions{}
	opts.Page, _ = strconv.Atoi(c.Query("page"))
	opts.PageSize, _ = strconv.Atoi(c.Query("pageSize"))
	opts.CategoryID = parseUintQuery(c, "categoryId")
	opts.Subcategory = c.Query("subcategory")
	opts.Status = c.Query("status")
	opts.Severity = c.Query("severity")
	opts.Source = c.Query("source")
	opts.ResponsibleType = c.Query("responsibleType")
	opts.ResponsibleID = c.Query("responsibleId")
	opts.GameID = c.Query("gameId")
	opts.Env = c.Query("env")
	if v := c.Query("from"); v != "" {
		if t, err := parseTime(v); err == nil {
			opts.From = &t
		}
	}
	if v := c.Query("to"); v != "" {
		if t, err := parseTime(v); err == nil {
			opts.To = &t
		}
	}
	resp, err := h.service.ListIncidents(c.Request.Context(), opts)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// CreateIncident handles POST /incidents (panel registration;
// CreatedBy=the JWT operator).
func (h *Handler) CreateIncident(c *gin.Context) {
	var req IncidentCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, err)
		return
	}
	resp, created, err := h.service.CreateIncident(c.Request.Context(), &req, c.GetString("username"))
	if err != nil {
		response.Error(c, err)
		return
	}
	if created {
		response.Created(c, resp)
		return
	}
	// Idempotent replay: return the already-existing row
	response.Success(c, resp)
}

// GetIncident handles GET /incidents/:id。
func (h *Handler) GetIncident(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		response.Error(c, err)
		return
	}
	resp, err := h.service.GetIncident(c.Request.Context(), id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// UpdateIncident handles PUT /incidents/:id。
func (h *Handler) UpdateIncident(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		response.Error(c, err)
		return
	}
	var req IncidentUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, err)
		return
	}
	resp, err := h.service.UpdateIncident(c.Request.Context(), id, &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// TransitionIncident handles POST /incidents/:id/status。
func (h *Handler) TransitionIncident(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		response.Error(c, err)
		return
	}
	var req IncidentStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, err)
		return
	}
	resp, err := h.service.TransitionIncident(c.Request.Context(), id, &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// ---- helpers ----

func parseUintParam(c *gin.Context, name string) (uint, error) {
	// bitSize=strconv.IntSize: parse directly at the platform uint width (code
	// scanning #226/#227); on 32-bit platforms a 64-bit input errors out once it
	// overflows the bound, so uint(v) is never truncated.
	v, err := strconv.ParseUint(c.Param(name), 10, strconv.IntSize)
	if err != nil || v == 0 {
		return 0, errorx.NewBadRequest("无效的 " + name + " 参数")
	}
	return uint(v), nil
}

func parseUintQuery(c *gin.Context, name string) uint {
	v, err := strconv.ParseUint(c.Query(name), 10, strconv.IntSize)
	if err != nil {
		return 0
	}
	return uint(v)
}

// parseIntQuery parses pagination query params (0=fall to the Normalize
// defaults): a direct uint->int conversion wraps negative on 64-bit platforms
// for inputs >MaxInt64 (code scanning #228/#229). After parsing it is clamped to
// MaxInt32 - page values never approach that bound; anything above it falls back
// to the default path.
func parseIntQuery(c *gin.Context, name string) int {
	v, err := strconv.ParseUint(c.Query(name), 10, strconv.IntSize)
	if err != nil || v > math.MaxInt32 {
		return 0
	}
	return int(v)
}

// parseTime supports both RFC3339 and unix seconds (RangePicker toISOString uses the former).
func parseTime(v string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, v); err == nil {
		return t, nil
	}
	if sec, err := strconv.ParseInt(v, 10, 64); err == nil {
		return time.Unix(sec, 0), nil
	}
	return time.Time{}, strconv.ErrSyntax
}
