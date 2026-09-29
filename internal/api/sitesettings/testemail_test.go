package sitesettings

// 发送测试邮件端点测试（OPEN-ISSUES #55 补欠 / #51c）：to 必填、SMTP 未
// 配置 400、注入缝隙记录收件人（不真发）、域白名单键写入校验。

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/settings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSendTestEmail(t *testing.T) {
	r, db := setupRouterWithDB(t)
	settings.ResetForTest()
	t.Cleanup(settings.ResetForTest)

	// SMTP 未配置 → 400（避免 EmailSender 空 host 静默假成功）
	rec := doSiteReq(t, r, http.MethodPost, "/api/v1/site/notification/test-email", `{"to":"a@example.com"}`)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "未配置")

	// to 必填
	rec = doSiteReq(t, r, http.MethodPost, "/api/v1/site/notification/test-email", `{}`)
	require.Equal(t, http.StatusBadRequest, rec.Code)

	// 注入缝隙：记录收件人，验证真实调用与响应形态
	var sent []string
	orig := sendTestEmailFn
	sendTestEmailFn = func(_ context.Context, to string) error {
		sent = append(sent, to)
		return nil
	}
	t.Cleanup(func() { sendTestEmailFn = orig })

	// 缝隙注入后 host 仍需非空——经 L3 播种 SMTP host（store 句柄不可达，
	// 直接换用带 settings 初始化的 Router？此文件 harness 无 settings 库，
	// 改用 AuthSnapshot 无关路径：NotifySMTP 读 settings.Current()，未初始
	// 化时 Current() 返回 nil → NotifySMTP 空配置 → 400。注入缝隙下要过
	// host 检查须初始化 settings 并播种 host。
	settings.ResetForTest()
	store := model.NewPlatformSettingModel(db)
	settings.InitLayered(context.Background(), &settings.ConfigInput{}, store)
	require.NoError(t, store.Set(context.Background(), "notification.smtpHost", json.RawMessage(`"smtp.example.com"`), "tester"))
	settings.Current().Reload(context.Background(), store)

	rec = doSiteReq(t, r, http.MethodPost, "/api/v1/site/notification/test-email", `{"to":"ops@example.com"}`)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"sent":true`)
	require.Len(t, sent, 1)
	assert.Equal(t, "ops@example.com", sent[0])

	// 发送失败透出后端错误
	sendTestEmailFn = func(_ context.Context, _ string) error { return assert.AnError }
	rec = doSiteReq(t, r, http.MethodPost, "/api/v1/site/notification/test-email", `{"to":"ops@example.com"}`)
	require.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestPutKey_EmailDomainWhitelist(t *testing.T) {
	r, _ := setupRouterWithDB(t)

	// 协议/路径拒绝（同 sec.domainFilter 规则）
	rec := doSiteReq(t, r, http.MethodPut, "/api/v1/site/auth.email.domainWhitelist", `{"value":"http://example.com"}`)
	require.Equal(t, http.StatusBadRequest, rec.Code)

	// 合法清单与空串
	for _, ok := range []string{"example.com", "example.com, foo.io", ""} {
		rec = doSiteReq(t, r, http.MethodPut, "/api/v1/site/auth.email.domainWhitelist", `{"value":"`+ok+`"}`)
		assert.Equal(t, http.StatusOK, rec.Code, ok)
	}

	// aliasRestriction：bool 键
	rec = doSiteReq(t, r, http.MethodPut, "/api/v1/site/auth.email.aliasRestriction", `{"value":"yes"}`)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	rec = doSiteReq(t, r, http.MethodPut, "/api/v1/site/auth.email.aliasRestriction", `{"value":true}`)
	assert.Equal(t, http.StatusOK, rec.Code)
}
