package sitesettings

// SMTP 传输细节键（OPEN-ISSUES #55）：encryption/authType 枚举校验，
// 空串 = 恢复自动/默认；insecureSkipVerify 走 bool 校验（validateValue 通用路径）。

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPutKey_SmtpTransportKeys(t *testing.T) {
	r, _ := setupRouterWithDB(t)

	// encryption：非法值拒绝
	rec := doSiteReq(t, r, http.MethodPut, "/api/v1/site/notification.smtpEncryption", `{"value":"tls-wrong"}`)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "none / ssl / starttls")

	// encryption：合法值与空串
	for _, v := range []string{"none", "ssl", "starttls", ""} {
		rec = doSiteReq(t, r, http.MethodPut, "/api/v1/site/notification.smtpEncryption", `{"value":"`+v+`"}`)
		assert.Equal(t, http.StatusOK, rec.Code, v)
	}

	// authType：非法值拒绝 + 合法值
	rec = doSiteReq(t, r, http.MethodPut, "/api/v1/site/notification.smtpAuthType", `{"value":"ntlm"}`)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "plain / login")

	for _, v := range []string{"plain", "login", ""} {
		rec = doSiteReq(t, r, http.MethodPut, "/api/v1/site/notification.smtpAuthType", `{"value":"`+v+`"}`)
		assert.Equal(t, http.StatusOK, rec.Code, v)
	}

	// insecureSkipVerify：bool 键（非法类型拒绝，合法布尔通过）
	rec = doSiteReq(t, r, http.MethodPut, "/api/v1/site/notification.smtpInsecureSkipVerify", `{"value":"yes"}`)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	rec = doSiteReq(t, r, http.MethodPut, "/api/v1/site/notification.smtpInsecureSkipVerify", `{"value":true}`)
	assert.Equal(t, http.StatusOK, rec.Code)
}
