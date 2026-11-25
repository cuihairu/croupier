// External services inventory (read-only): L2 file-configured platform
// interface external services — herald (alert outlet) / alertInbound
// (alert inbound webhook) / serverStatus (maintenance status source).
// server-status-provider design batch S3 "external services enabled on the
// settings page".
//
// The redaction policy matches the notification snapshot: report only
// set/not-set plus missing-item names, never echo any value (credentials are
// always environment-variable references; the values neither land in config nor
// appear in responses). Page-side config editing belongs to the "config
// organization" item (docs/design/config-organization.md, pending user review);
// this endpoint is purely read-only and does not write the L3 layered settings.
package sitesettings

import (
	"os"
	"strconv"
	"strings"

	"github.com/cuihairu/croupier/internal/common/response"
	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/platform/outlet"
	"github.com/cuihairu/croupier/internal/platform/serverstatus"
	"github.com/gin-gonic/gin"
)

// ExternalService is one entry of the external services inventory.
// Configured = the config file has the section; Active = enabled and all required
// items are present (Missing empty); the consequences of missing items match each
// service's svc assembly decision: herald without a token falls back to a noop
// outlet, alertInbound without a secret answers 403, and serverStatus without a
// baseUrl stays fail-open and annotated (source_unknown is still reported).
// Missing = names of missing items (actionable hints -- contract keys or
// environment variable names -- echoing no values).
type ExternalService struct {
	Name       string   `json:"name"`
	Kind       string   `json:"kind"` // outbound | inbound
	Configured bool     `json:"configured"`
	Active     bool     `json:"active"`
	Missing    []string `json:"missing,omitempty"`
	SecretEnv  string   `json:"secretEnv,omitempty"`
	SecretSet  bool     `json:"secretSet,omitempty"`
	Detail     string   `json:"detail,omitempty"`
}

// ExternalServicesResponse is the external services inventory response.
type ExternalServicesResponse struct {
	Services []ExternalService `json:"services"`
	Source   string            `json:"source"` // always "file": the L2 config-file truth; L3 overrides do not apply
}

// ExternalServicesHandler presents the file-side external services inventory, read-only.
type ExternalServicesHandler struct {
	cfg config.Config
}

// NewExternalServicesHandler builds the handler (a snapshot of serverCtx.Config;
// config does not change at runtime, so the assembly result is the startup truth).
func NewExternalServicesHandler(cfg config.Config) *ExternalServicesHandler {
	return &ExternalServicesHandler{cfg: cfg}
}

// Register mounts GET /site/external-services on the admin group.
func (h *ExternalServicesHandler) Register(g *gin.RouterGroup) {
	g.GET("/site/external-services", h.Get)
}

// Get serves GET /api/v1/site/external-services.
func (h *ExternalServicesHandler) Get(c *gin.Context) {
	response.Success(c, ExternalServicesResponse{
		Services: []ExternalService{
			h.herald(),
			h.alertInbound("alertmanager"),
			h.alertInbound("generic"),
			h.serverStatus(),
		},
		Source: "file",
	})
}

// herald alert outlet: active = enabled && baseUrl non-empty && the token
// environment variable has a value (same criteria as the registerHeraldOutlet
// assembly decision: any missing item = the outlet is not registered; the alert
// page still shows the truth).
func (h *ExternalServicesHandler) herald() ExternalService {
	cfg := h.cfg.Herald
	envName := cfg.TokenEnv
	if envName == "" {
		envName = outlet.HeraldDefaultTokenEnv
	}
	e := ExternalService{
		Name:       "herald",
		Kind:       "outbound",
		Configured: cfg.Enabled || strings.TrimSpace(cfg.BaseURL) != "",
		SecretEnv:  envName,
		SecretSet:  os.Getenv(envName) != "",
		Detail:     cfg.BaseURL,
	}
	e.Missing = appendMissing(e.Missing, !cfg.Enabled, "herald.enabled")
	e.Missing = appendMissing(e.Missing, strings.TrimSpace(cfg.BaseURL) == "", "herald.baseUrl")
	e.Missing = appendMissing(e.Missing, !e.SecretSet, envName)
	e.Active = len(e.Missing) == 0
	return e
}

// alertInbound alert inbound webhook (closed source set: alertmanager/generic):
// active = the source is configured and the secretEnv environment variable has a
// value (same threshold as the inbound handler).
func (h *ExternalServicesHandler) alertInbound(source string) ExternalService {
	src, ok := h.cfg.AlertInbound.Sources[source]
	e := ExternalService{
		Name: "alertInbound." + source,
		Kind: "inbound",
	}
	if !ok || src.SecretEnv == "" {
		e.Missing = appendMissing(nil, true, "alertInbound.sources."+source+".secretEnv")
		return e
	}
	e.Configured = true
	e.SecretEnv = src.SecretEnv
	e.SecretSet = os.Getenv(src.SecretEnv) != ""
	e.Missing = appendMissing(e.Missing, !e.SecretSet, src.SecretEnv)
	if len(src.LabelMapping) > 0 {
		e.Detail = "labelMapping=" + strconv.Itoa(len(src.LabelMapping))
	}
	e.Active = len(e.Missing) == 0
	return e
}

// serverStatus maintenance status source: an empty provider means the default
// atlas (batch S4, same criteria as the installServerStatusGate assembly
// decision); an explicit noop needs no baseUrl. For real sources,
// active = enabled && the provider is registered && baseUrl is non-empty.
func (h *ExternalServicesHandler) serverStatus() ExternalService {
	cfg := h.cfg.ServerStatus
	providerKey := strings.TrimSpace(cfg.Provider)
	if providerKey == "" {
		providerKey = serverstatus.ProviderAtlas
	}
	e := ExternalService{
		Name:       "serverStatus." + providerKey,
		Kind:       "outbound",
		Configured: cfg.Enabled || len(cfg.Providers) > 0,
		Detail:     "provider=" + providerKey + " matchBy=" + matchByOr(cfg.MatchBy),
	}
	e.Missing = appendMissing(e.Missing, !cfg.Enabled, "serverStatus.enabled")
	if cfg.Enabled {
		if serverstatus.Lookup(providerKey) == nil {
			e.Missing = appendMissing(e.Missing, true, "serverStatus.provider:"+providerKey)
		}
		// Flag a missing baseUrl (both a nonexistent providers section and one that
		// exists but is empty count as missing -- the assembly layer likewise bypasses
		// mounting the gate when a non-noop source lacks a baseUrl).
		if providerKey != serverstatus.ProviderNoop && strings.TrimSpace(cfg.Providers[providerKey].BaseURL) == "" {
			e.Missing = appendMissing(e.Missing, true, "serverStatus.providers."+providerKey+".baseUrl")
		}
		if source, ok := cfg.Providers[providerKey]; ok && source.TokenEnv != "" {
			e.SecretEnv = source.TokenEnv
			e.SecretSet = os.Getenv(source.TokenEnv) != ""
			e.Missing = appendMissing(e.Missing, !e.SecretSet, source.TokenEnv)
		}
	}
	e.Active = len(e.Missing) == 0
	return e
}

// appendMissing appends a missing item (recorded only when actually missing,
// keeping Missing append-only for actionable reading).
func appendMissing(dst []string, cond bool, name string) []string {
	if cond {
		return append(dst, name)
	}
	return dst
}

// matchByOr applies the default match key (same as the gate assembly: a start
// without agentId falls back and warns).
func matchByOr(v string) string {
	if strings.TrimSpace(v) == "" {
		return "agentId"
	}
	return v
}
