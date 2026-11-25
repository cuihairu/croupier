// Send test email (OPEN-ISSUES #55 boundary debt / #51c): the "send test email"
// entry point on the ops SMTP card. Builds an EmailSender from the current L3 SMTP
// config on the fly and really sends (it does not check emailEnabled -- a test
// email is itself the way to validate the config); the sendMail function seam is
// available for test injection.
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

// sendTestEmailFn is the injectable seam (same style as ops/systemUpdateFetchFn).
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

// SendTestEmail serves POST /api/v1/site/notification/test-email (admin).
// Returns 400 when the SMTP host is not configured (EmailSender.Send silently
// no-ops on an empty host, so this up-front interception avoids a false success).
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
