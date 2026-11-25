// Site settings API: public read snapshot + admin L3 write/clear
// (docs/architecture/config-layering.md §5, site-settings-design.md).
package sitesettings

import (
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/common/response"
	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/settings"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	layered *settings.Layered
	store   *model.PlatformSettingModel
	// onAuthChange is invoked after an auth.* key is saved (hot refresh of login
	// providers -- Harbor style, save takes effect immediately; nil = not wired up,
	// config only takes effect on the next restart).
	onAuthChange func(cfg config.AuthProvidersConfig) error
}

// NewHandler creates a settings handler.
func NewHandler(layered *settings.Layered, store *model.PlatformSettingModel) *Handler {
	return &Handler{layered: layered, store: store}
}

// SetAuthChangeCallback injects the login-provider hot refresh callback (assembly time).
func (h *Handler) SetAuthChangeCallback(fn func(cfg config.AuthProvidersConfig) error) {
	h.onAuthChange = fn
}

// RegisterPublic mounts GET /site on the public group.
func RegisterPublic(g *gin.RouterGroup, h *Handler) {
	g.GET("/site", h.GetPublic)
}

// RegisterAdmin mounts the admin write endpoints.
func (h *Handler) RegisterAdmin(g *gin.RouterGroup) {
	g.PUT("/site/:key", h.PutKey)
	g.DELETE("/site/:key", h.ClearKey)
	// Note: the /site/features read endpoints share the group with the :key routes,
	// and gin would otherwise match "features" as :key, running PutKey/ClearKey's
	// parameter validation (not in the whitelist -> 400), so the read endpoints
	// must be registered before the parameter routes.
	g.GET("/site/features", h.GetFeatures)
	g.GET("/site/observability", h.GetObservability)
	g.GET("/site/notification", h.GetNotification)
	g.GET("/site/security", h.GetSecurity)
	g.GET("/site/outbound", h.GetOutbound)
	g.POST("/site/notification/test-email", h.SendTestEmail)
}

// GetNotification serves GET /api/v1/site/notification: channel config with
// secrets masked (only "set" state + last-4 echo).
func (h *Handler) GetNotification(c *gin.Context) {
	response.Success(c, h.layered.NotificationSnapshot())
}

// GetSecurity serves GET /api/v1/site/security: the effective account security
// policy values (the six security.* keys, all off by default; the only exception
// is approvalStepUpOtp, on by default, #61).
func (h *Handler) GetSecurity(c *gin.Context) {
	response.Success(c, h.layered.SecurityPolicy())
}

// GetOutbound serves GET /api/v1/site/outbound: the effective outbound security
// and restriction values (the four sec.* keys, all off by default = no limits,
// no blocking).
func (h *Handler) GetOutbound(c *gin.Context) {
	response.Success(c, h.layered.OutboundSnapshot())
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
	// Secret placeholder preservation: the read endpoint echoes a mask (**** + last
	// 4). If the frontend saves that echo back verbatim, it must not overwrite the
	// real value with the mask -- when the mask shape is detected, reuse the stored
	// value instead.
	if settings.IsSecretKey(key) {
		var v string
		if err := json.Unmarshal(req.Value, &v); err == nil && strings.HasPrefix(v, "****") {
			if raw, src, ok := h.layered.GetString(c.Request.Context(), key); ok && src == "database" {
				req.Value, _ = json.Marshal(raw)
			} else {
				response.Error(c, errorx.NewBadRequest("secret 未设置，请输入真实值而非掩码"))
				return
			}
		}
	}
	updatedBy := currentUsername(c)
	if err := h.store.Set(c.Request.Context(), key, req.Value, updatedBy); err != nil {
		response.Error(c, err)
		return
	}
	h.layered.Reload(c.Request.Context(), h.store)
	// Login provider key: hot refresh the identity providers (on failure, roll back
	// the key so the current state stays usable)
	if strings.HasPrefix(key, "auth.") && h.onAuthChange != nil {
		if err := h.onAuthChange(h.layered.AuthProviderConfig()); err != nil {
			_ = h.store.Clear(c.Request.Context(), key)
			h.layered.Reload(c.Request.Context(), h.store)
			response.Error(c, errorx.NewBadRequest("登录方式配置无效，已回滚: "+err.Error()))
			return
		}
	}
	response.Success(c, gin.H{"key": key, "source": "database"})
}

// validateValue validates the type and format of an L3 write value.
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
		// footer.links exception: a JSON array.
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
	if key == settings.KeySystemUpdateCheckURL {
		if v != "" && !isHTTPLikeURL(v) {
			return fmt.Errorf("%s 需要是 http(s) URL", key)
		}
	}
	if key == settings.KeyNotifyDingtalkURL || key == settings.KeyNotifyWebhookURL ||
		key == settings.KeyNotifyWecomURL || key == settings.KeyNotifyFeishuURL {
		if v != "" && !isHTTPLikeURL(v) {
			return fmt.Errorf("%s 需要是 http(s) URL", key)
		}
	}
	// #55 SMTP transport details: enum value validation (empty string = restore
	// auto/default)
	if key == settings.KeyNotifySMTPEncryption {
		switch v {
		case "", "none", "ssl", "starttls":
		default:
			return fmt.Errorf("%s 仅支持 none / ssl / starttls（留空 = 自动）", key)
		}
	}
	if key == settings.KeyNotifySMTPAuthType {
		switch v {
		case "", "plain", "login":
		default:
			return fmt.Errorf("%s 仅支持 plain / login（留空 = plain）", key)
		}
	}
	// #56 outbound security: list format validation (empty string = no limits)
	if key == settings.KeySecAllowPorts && v != "" {
		for _, part := range strings.Split(strings.ReplaceAll(v, " ", ""), ",") {
			n, err := strconv.Atoi(part)
			if err != nil || n < 1 || n > 65535 {
				return fmt.Errorf("%s 需为 1-65535 端口的逗号分隔清单", key)
			}
		}
	}
	if key == settings.KeySecAllowIPs && v != "" {
		for _, part := range strings.Split(v, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			if strings.Contains(part, "/") {
				if _, _, err := net.ParseCIDR(part); err != nil {
					return fmt.Errorf("%s 含非法 CIDR：%s", key, part)
				}
				continue
			}
			if net.ParseIP(part) == nil {
				return fmt.Errorf("%s 含非法 IP：%s", key, part)
			}
		}
	}
	if key == settings.KeySecDomainFilter && v != "" {
		if err := validateDomainSuffixList(key, v); err != nil {
			return err
		}
	}
	// #51c registration email domain whitelist: the same domain suffix list rules
	if key == settings.KeyAuthEmailDomainWhitelist && v != "" {
		if err := validateDomainSuffixList(key, v); err != nil {
			return err
		}
	}
	return nil
}

// validateDomainSuffixList validates a comma-separated list of domain suffixes
// (protocol, path, and spaces are forbidden).
func validateDomainSuffixList(key, v string) error {
	for _, part := range strings.Split(v, ",") {
		part = strings.ToLower(strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(part), ".")))
		if part == "" {
			continue
		}
		if strings.Contains(part, "://") || strings.ContainsAny(part, "/ ") {
			return fmt.Errorf("%s 需为域名后缀清单（如 example.com，不带协议/路径）", key)
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
