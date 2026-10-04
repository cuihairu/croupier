// 第三方服务健康探针（OPEN-ISSUES #57）：对已配置的第三方外呼目标做一次
// 手动探活——通知 webhook 四渠道（HTTP GET 只读探测）、检查更新源（GET）、
// SMTP（TCP+EHLO，不发信）。探测经出站守卫（sec.*）与调用策略（net.*
// 超时），结果含延迟/状态/错误。
package ops

import (
	"github.com/cuihairu/croupier/internal/common/response"
	"github.com/cuihairu/croupier/internal/platform/settings"
	"github.com/cuihairu/croupier/internal/security/secguard"
	"github.com/gin-gonic/gin"
)

// probeChannels 探针支持的渠道（键 = URL 探测目标；smtp 走 ProbeSMTP）。
var probeChannels = map[string]bool{
	"dingtalk": true,
	"wecom":    true,
	"feishu":   true,
	"webhook":  true,
	"update":   true,
	"smtp":     true,
}

// ThirdPartyProbe serves POST /api/v1/ops/probes/:channel。
// 未配置目标返回 configured=false（200，便于前端逐渠道渲染），非法渠道 404。
func (h *Handler) ThirdPartyProbe(c *gin.Context) {
	channel := c.Param("channel")
	if !probeChannels[channel] {
		response.NotFound(c, "未知探针渠道: "+channel)
		return
	}
	l := settings.Current()
	guard := secguard.Resolve(l)
	snap := l.NotificationSnapshot()
	res := probeView{Channel: channel}
	url := ""
	switch channel {
	case "dingtalk":
		url = snap.DingtalkURL
	case "wecom":
		url = snap.WecomURL
	case "feishu":
		url = snap.FeishuURL
	case "webhook":
		url = snap.WebhookURL
	case "update":
		url = systemUpdateCheckURLFn()
	case "smtp":
		smtpCfg := l.NotifySMTP()
		if smtpCfg.Host == "" {
			res.Configured = false
			response.Success(c, res)
			return
		}
		res.Configured = true
		pr := secguard.ProbeSMTP(c.Request.Context(), guard, smtpCfg.Host, smtpCfg.Port)
		fillProbeView(&res, pr)
		response.Success(c, res)
		return
	}
	res.Configured = url != ""
	if !res.Configured {
		response.Success(c, res)
		return
	}
	fillProbeView(&res, secguard.Probe(c.Request.Context(), guard, url))
	response.Success(c, res)
}

// probeView 探针响应体。
type probeView struct {
	Channel    string `json:"channel"`
	Configured bool   `json:"configured"`
	OK         bool   `json:"ok"`
	Status     int    `json:"status"`
	LatencyMs  int64  `json:"latencyMs"`
	Error      string `json:"error,omitempty"`
}

func fillProbeView(v *probeView, pr secguard.ProbeResult) {
	v.OK = pr.OK
	v.Status = pr.Status
	v.LatencyMs = pr.LatencyMs
	v.Error = pr.Error
}
