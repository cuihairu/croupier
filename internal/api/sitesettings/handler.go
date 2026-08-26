// Site settings API: public read snapshot + admin L3 write/clear
// (docs/architecture/config-layering.md §5, site-settings-design.md).
package sitesettings

import (
	"encoding/json"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/common/response"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/settings"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	layered *settings.Layered
	store   *model.PlatformSettingModel
}

// NewHandler creates a settings handler.
func NewHandler(layered *settings.Layered, store *model.PlatformSettingModel) *Handler {
	return &Handler{layered: layered, store: store}
}

// RegisterPublic mounts GET /site on the public group.
func RegisterPublic(g *gin.RouterGroup, h *Handler) {
	g.GET("/site", h.GetPublic)
}

// RegisterAdmin mounts the admin write endpoints.
func (h *Handler) RegisterAdmin(g *gin.RouterGroup) {
	g.PUT("/site/:key", h.PutKey)
	g.DELETE("/site/:key", h.ClearKey)
}

// GetPublic serves GET /api/v1/public/site.
func (h *Handler) GetPublic(c *gin.Context) {
	response.Success(c, h.layered.SiteSnapshot())
}

type putRequest struct {
	Value json.RawMessage `json:"value"`
}

// PutKey serves PUT /api/v1/site/:key (admin). Value must be a JSON string
// for string keys or a JSON array for footer.links.
func (h *Handler) PutKey(c *gin.Context) {
	key := c.Param("key")
	if !settings.IsValidKey(key) {
		response.Error(c, errorx.NewBadRequest("无效的配置键: "+key))
		return
	}
	var req putRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, err)
		return
	}
	if len(req.Value) == 0 {
		response.Error(c, errorx.NewBadRequest("value 不能为空"))
		return
	}
	updatedBy := currentUsername(c)
	if err := h.store.Set(c.Request.Context(), key, req.Value, updatedBy); err != nil {
		response.Error(c, err)
		return
	}
	h.layered.Reload(c.Request.Context(), h.store)
	response.Success(c, gin.H{"key": key, "source": "database"})
}

// ClearKey serves DELETE /api/v1/site/:key = follow config file again.
func (h *Handler) ClearKey(c *gin.Context) {
	key := c.Param("key")
	if !settings.IsValidKey(key) {
		response.Error(c, errorx.NewBadRequest("无效的配置键: "+key))
		return
	}
	if err := h.store.Clear(c.Request.Context(), key); err != nil {
		response.Error(c, err)
		return
	}
	h.layered.Reload(c.Request.Context(), h.store)
	response.Success(c, gin.H{"key": key, "source": "config"})
}

func currentUsername(c *gin.Context) string {
	if v, ok := c.Get("username"); ok {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	return "system"
}
