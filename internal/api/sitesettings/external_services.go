// External services inventory (read-only): L2 file-configured platform
// interface external services — herald (alert outlet) / alertInbound
// (alert inbound webhook) / serverStatus (maintenance status source).
// server-status-provider design batch S3「设置页已启用外部服务」.
//
// 脱敏口径与 notification 快照一致：只报 set/not-set 与缺项名，永不回显
// 任何值（凭据一律环境变量引用，值不落配置也不进响应）。页面侧配置编辑
// 属「配置整理」单（docs/design/config-organization.md，待用户过目），
// 本端点纯只读、不写 L3 分层设置。
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

// ExternalService 单条外部服务清单。Configured=配置文件有该段；
// Active=启用态且必填项全齐（Missing 为空）；缺项后果与各服务 svc 装配
// 判定一致：herald 缺 token 回落 noop 出口、alertInbound 缺密钥端点 403、
// serverStatus 缺 baseUrl 恒 fail-open 标注（source_unknown 照报）。
// Missing=缺项名（可操作提示，契约键或环境变量名，不回显任何值）。
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

// ExternalServicesResponse 外部服务清单响应。
type ExternalServicesResponse struct {
	Services []ExternalService `json:"services"`
	Source   string            `json:"source"` // 固定 "file"：L2 配置文件真值，L3 覆盖不适用
}

// ExternalServicesHandler 文件侧外部服务清单只读呈现。
type ExternalServicesHandler struct {
	cfg config.Config
}

// NewExternalServicesHandler 构造（serverCtx.Config 快照；config 运行期
// 不变更，装配结果即启动真值）。
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

// herald 告警出口：active = enabled && baseUrl 非空 && token 环境变量有值
// （与 registerHeraldOutlet 装配判定同口径：任一缺=出口未注册，告警页仍真值）。
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

// alertInbound 告警入站 webhook（来源闭集 alertmanager/generic）：active =
// 来源已配置且 secretEnv 环境变量有值（与 inbound handler 门槛同口径）。
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

// serverStatus 维护状态源：provider=noop 无需 baseUrl；真实源（atlas 等）
// active = enabled && provider 已注册 && baseUrl 非空（与
// installServerStatusGate 装配判定同口径）。
func (h *ExternalServicesHandler) serverStatus() ExternalService {
	cfg := h.cfg.ServerStatus
	providerKey := strings.TrimSpace(cfg.Provider)
	if providerKey == "" {
		providerKey = "noop"
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
		if source, ok := cfg.Providers[providerKey]; ok && providerKey != "noop" && source.BaseURL == "" {
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

// appendMissing 缺项追加（空缺才记录，保持 Missing 只增不减的可操作口径）。
func appendMissing(dst []string, cond bool, name string) []string {
	if cond {
		return append(dst, name)
	}
	return dst
}

// matchByOr 匹配键缺省口径（gate 装配同款：非 agentId 启动回落并告警）。
func matchByOr(v string) string {
	if strings.TrimSpace(v) == "" {
		return "agentId"
	}
	return v
}
