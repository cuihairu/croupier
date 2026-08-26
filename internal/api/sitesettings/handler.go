// Site settings API: public read snapshot + admin L3 write/clear
// (docs/architecture/config-layering.md §5, site-settings-design.md).
package sitesettings

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

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
	// 注意：/site/features 读端点与 :key 路由同组，gin 会把 "features"
	// 当作 :key 命中 PutKey/ClearKey 的参数校验（不在白名单 → 400），
	// 因此读端点必须在参数路由之前注册。
	g.GET("/site/features", h.GetFeatures)
	g.GET("/site/observability", h.GetObservability)
	g.GET("/site/notification", h.GetNotification)
}

// GetNotification serves GET /api/v1/site/notification: channel config with
// secrets masked (only "set" state + last-4 echo).
func (h *Handler) GetNotification(c *gin.Context) {
	response.Success(c, h.layered.NotificationSnapshot())
}

// GetFeatures serves GET /api/v1/site/features: per-domain composed state
// (enabled), L2 trim info, and L3 override presence.
func (h *Handler) GetFeatures(c *gin.Context) {
	response.Success(c, h.layered.FeatureSnapshot())
}

// GetObservability serves GET /api/v1/site/observability: obs.* URLs with
// per-key provenance.
func (h *Handler) GetObservability(c *gin.Context) {
	response.Success(c, h.layered.ObsSnapshot())
}

// GetPublic serves GET /api/v1/public/site.
func (h *Handler) GetPublic(c *gin.Context) {
	response.Success(c, h.layered.SiteSnapshot())
}

type putRequest struct {
	Value json.RawMessage `json:"value"`
}

// PutKey serves PUT /api/v1/site/:key (admin). Value must be a JSON string
// for string keys, a JSON boolean for feature keys, or a JSON array for
// footer.links.
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
	if err := validateValue(key, req.Value); err != nil {
		response.Error(c, errorx.NewBadRequest(err.Error()))
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

// validateValue 校验 L3 写入值的类型与格式。
func validateValue(key string, raw json.RawMessage) error {
	if settings.IsBoolKey(key) {
		var v bool
		if err := json.Unmarshal(raw, &v); err != nil {
			return fmt.Errorf("%s 需要 JSON 布尔值", key)
		}
		return nil
	}
	if settings.IsIntKey(key) {
		var v int
		if err := json.Unmarshal(raw, &v); err != nil {
			return fmt.Errorf("%s 需要 JSON 整数", key)
		}
		if v < 0 || v > 65535 {
			return fmt.Errorf("%s 超出端口范围", key)
		}
		return nil
	}
	var v string
	if err := json.Unmarshal(raw, &v); err != nil {
		// footer.links 例外：JSON 数组。
		if key == settings.KeyFooterLinks {
			var links []settings.FooterLink
			if err2 := json.Unmarshal(raw, &links); err2 == nil {
				return nil
			}
		}
		return fmt.Errorf("%s 需要 JSON 字符串", key)
	}
	if strings.HasPrefix(key, "obs.") {
		if v != "" && !isHTTPLikeURL(v) {
			return fmt.Errorf("%s 需要是 http(s) URL", key)
		}
	}
	if key == settings.KeyNotifyDingtalkURL || key == settings.KeyNotifyWebhookURL {
		if v != "" && !isHTTPLikeURL(v) {
			return fmt.Errorf("%s 需要是 http(s) URL", key)
		}
	}
	return nil
}

func isHTTPLikeURL(s string) bool {
	u, err := url.Parse(s)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
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
