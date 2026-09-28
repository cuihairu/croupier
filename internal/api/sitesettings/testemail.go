// 发送测试邮件（OPEN-ISSUES #55 边界补欠 / #51c）：运维 SMTP 卡的
// 「发送测试邮件」入口。按 L3 当前 SMTP 配置即时构造 EmailSender 真实
// 发信（不查 emailEnabled——测试邮件本身就是验证配置的手段），sendMail
// 函数缝隙供测试注入。
package sitesettings

import (
	"context"
	"strings"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/common/response"
	"github.com/cuihairu/croupier/internal/platform/approvals"
	"github.com/cuihairu/croupier/internal/platform/settings"
	"github.com/gin-gonic/gin"
)

// sendTestEmailFn 可注入缝隙（同 ops/systemUpdateFetchFn 口径）。
var sendTestEmailFn = func(ctx context.Context, to string) error {
	smtpCfg := settings.Current().NotifySMTP()
	sender := approvals.NewEmailSender(smtpCfg.Host, smtpCfg.Port, smtpCfg.User, smtpCfg.Password, smtpCfg.From).
		WithTransport(smtpCfg.Encryption, smtpCfg.AuthType, smtpCfg.InsecureSkipVerify)
	return sender.Send(ctx, to, approvals.NotificationEvent{
		Type:     "test_email",
		Title:    "Croupier 测试邮件",
		Message:  "这是一封来自 Croupier 站点设置的测试邮件，收到即代表 SMTP 配置可用。",
		Priority: "normal",
	})
}

// SendTestEmail serves POST /api/v1/site/notification/test-email (admin)。
// SMTP host 未配置时 400（EmailSender.Send 对空 host 静默 no-op，前置拦截
// 避免假成功）。
func (h *Handler) SendTestEmail(c *gin.Context) {
	var req struct {
		To string `json:"to"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, err)
		return
	}
	to := strings.TrimSpace(req.To)
	if to == "" {
		response.Error(c, errorx.NewBadRequest("to 不能为空"))
		return
	}
	if smtpCfg := settings.Current().NotifySMTP(); smtpCfg.Host == "" {
		response.Error(c, errorx.NewBadRequest("SMTP 服务器未配置，请先保存 SMTP 主机"))
		return
	}
	if err := sendTestEmailFn(c.Request.Context(), to); err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, gin.H{"sent": true, "to": to})
}
