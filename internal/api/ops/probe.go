// Third-party service health probe (OPEN-ISSUES #57): one manual liveness probe
// against configured third-party outbound targets — the four notification
// webhook channels (HTTP GET read-only probe), the update source (GET), and
// SMTP (TCP+EHLO, no mail sent). Probing passes through the outbound guard
// (sec.*) and the call policy (net.* timeouts); the result carries
// latency/status/error.
package ops

import (
	"github.com/cuihairu/croupier/internal/common/response"
	"github.com/cuihairu/croupier/internal/platform/settings"
	"github.com/cuihairu/croupier/internal/security/secguard"
	"github.com/gin-gonic/gin"
)

// probeChannels lists the channels the probe supports (key = the URL probe
// target; smtp goes through ProbeSMTP).
var probeChannels = map[string]bool{
	"dingtalk": true,
	"wecom":    true,
	"feishu":   true,
	"webhook":  true,
	"update":   true,
	"smtp":     true,
}

// ThirdPartyProbe serves POST /api/v1/ops/probes/:channel.
// An unconfigured target returns configured=false (200, so the frontend can
// render channel by channel); an invalid channel returns 404.
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

// probeView is the probe response body.
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
