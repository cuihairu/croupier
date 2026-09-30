package sitesettings

// 残余覆盖缺口收口（覆盖率巡检）：GetSecurity / GetOutbound 两个读快照端点
// 整段 0%（路由已挂载但无用例）、sec.allowIPs 与域名后缀清单里「空分段」
// 的 continue 分支、SendTestEmail 的绑定失败与**真实发送路径**（此前全部
// 走 sendTestEmailFn 注入缝隙，default 实现从未执行）。

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

// TestGetSecurityAndOutboundSnapshots 两个读端点的默认值形态与 L3 覆盖后
// 形态：security.* 五键默认全关，sec.* 清单为空串（不限）、ssrfProtection
// false（不拦截）、net.* 缺省 0。
func TestGetSecurityAndOutboundSnapshots(t *testing.T) {
	r, _ := setupRouterWithDB(t)

	rec := doSiteReq(t, r, http.MethodGet, "/api/v1/site/security", "")
	require.Equal(t, http.StatusOK, rec.Code)
	var sec map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &sec))
	assert.Equal(t, false, sec["mfaRequired"], "默认全关")
	assert.Equal(t, false, sec["passwordRequireUppercase"])
	assert.Equal(t, float64(0), sec["passwordMinLength"])

	rec = doSiteReq(t, r, http.MethodGet, "/api/v1/site/outbound", "")
	require.Equal(t, http.StatusOK, rec.Code)
	var out map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	assert.Equal(t, "", out["allowPorts"], "空清单 = 不限")
	assert.Equal(t, "", out["allowIPs"])
	assert.Equal(t, "", out["domainFilter"])
	assert.Equal(t, false, out["ssrfProtection"], "默认不拦截")
	assert.Equal(t, float64(0), out["requestTimeoutMs"], "0 = 沿用调用方缺省")
	assert.Equal(t, float64(0), out["maxRetries"])
	assert.Equal(t, float64(0), out["retryBackoffMs"])
	assert.Equal(t, "default", out["sources"].(map[string]any)["sec.allowPorts"])

	// L3 写入后快照应回显真值且来源标记为 database
	rec = doSiteReq(t, r, http.MethodPut, "/api/v1/site/sec.ssrfProtection", `{"value":true}`)
	require.Equal(t, http.StatusOK, rec.Code)
	rec = doSiteReq(t, r, http.MethodGet, "/api/v1/site/outbound", "")
	require.Equal(t, http.StatusOK, rec.Code)
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	assert.Equal(t, true, out["ssrfProtection"])
	assert.Equal(t, "database", out["sources"].(map[string]any)["sec.ssrfProtection"])

	rec = doSiteReq(t, r, http.MethodPut, "/api/v1/site/security.mfaRequired", `{"value":true}`)
	require.Equal(t, http.StatusOK, rec.Code)
	rec = doSiteReq(t, r, http.MethodGet, "/api/v1/site/security", "")
	require.Equal(t, http.StatusOK, rec.Code)
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &sec))
	assert.Equal(t, true, sec["mfaRequired"])
}

// TestValidateValueEmptySegments 清单类键里「空分段」的 continue 分支：
// 连续逗号 / 首尾逗号 / 全空白项都应被跳过而非判非法。
func TestValidateValueEmptySegments(t *testing.T) {
	r, _ := setupRouterWithDB(t)

	// sec.allowIPs：空分段跳过（连续逗号、首逗号、尾逗号、纯空白项）
	for _, ok := range []string{
		"10.0.0.1,,10.0.0.2",
		",10.0.0.1",
		"10.0.0.1,",
		"10.0.0.1, ,10.0.0.2",
		",,",
	} {
		rec := doSiteReq(t, r, http.MethodPut, "/api/v1/site/sec.allowIPs",
			`{"value":"`+ok+`"}`)
		assert.Equal(t, http.StatusOK, rec.Code, "空分段应跳过: %q", ok)
	}

	// 空分段跳过不掩盖同段内的真错误
	rec := doSiteReq(t, r, http.MethodPut, "/api/v1/site/sec.allowIPs", `{"value":"10.0.0.1,,bogus"}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	rec = doSiteReq(t, r, http.MethodPut, "/api/v1/site/sec.allowIPs", `{"value":"10.0.0.1,,10.0.0.0/999"}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code, "非法 CIDR 仍应被拒")

	// 域名后缀清单：空分段同理（sec.domainFilter 与 auth.email.domainWhitelist
	// 共用 validateDomainSuffixList，两键各验一遍）
	for _, key := range []string{"sec.domainFilter", "auth.email.domainWhitelist"} {
		for _, ok := range []string{"a.com,,b.com", ",a.com", "a.com,", "a.com, ,b.com", ","} {
			rec := doSiteReq(t, r, http.MethodPut, "/api/v1/site/"+key,
				`{"value":"`+ok+`"}`)
			assert.Equal(t, http.StatusOK, rec.Code, "%s 空分段应跳过: %q", key, ok)
		}
		rec = doSiteReq(t, r, http.MethodPut, "/api/v1/site/"+key, `{"value":"a.com,,http://x.io"}`)
		assert.Equal(t, http.StatusBadRequest, rec.Code, "%s 空分段不得掩盖真错误", key)
	}
}

// TestSendTestEmailBindError 请求体不是合法 JSON → 绑定失败 400（此前只测了
// 「字段缺失」与「注入缝隙」两路）。
func TestSendTestEmailBindError(t *testing.T) {
	r, _ := setupRouterWithDB(t)
	rec := doSiteReq(t, r, http.MethodPost, "/api/v1/site/notification/test-email", `{"to":`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// TestSendTestEmailRealSendPath 走**未注入**的 sendTestEmailFn：SMTP 指向
// 本地拒连端口，真实发信必然失败 → 端点把后端错误透出为 500。覆盖 default
// 实现体（构造 EmailSender + WithTransport + Send）此前 0% 的部分。
func TestSendTestEmailRealSendPath(t *testing.T) {
	r, db := setupRouterWithDB(t)
	// 包级 default 实现就位即用（tracked 注入用例均经 t.Cleanup 复原，
	// 且本文件按字母序先于 testemail_test.go 执行，起点必为 default；
	// 原稿误将缝隙变量置 nil——default 只存在于声明初始化器，置 nil 即
	// 调用 nil 函数 panic）。
	require.NotNil(t, sendTestEmailFn, "起点须为 default 实现")

	store := model.NewPlatformSettingModel(db)
	for k, v := range map[string]string{
		"notification.smtpHost": "127.0.0.1",
		"notification.smtpPort": "1", // 保留端口：dial 立即 ECONNREFUSED
		"notification.smtpFrom": "noreply@example.com",
	} {
		raw, err := json.Marshal(v)
		require.NoError(t, err)
		require.NoError(t, store.Set(context.Background(), k, json.RawMessage(raw), "tester"))
	}
	settings.Current().Reload(context.Background(), store)

	rec := doSiteReq(t, r, http.MethodPost, "/api/v1/site/notification/test-email",
		`{"to":"ops@example.com"}`)
	assert.Equal(t, http.StatusInternalServerError, rec.Code, "真实发信失败应透出 500")
	assert.NotContains(t, rec.Body.String(), `"sent":true`)

	// 非法收件地址在拨号前即被拒（validateEmailAddress），不产生外部连接
	rec = doSiteReq(t, r, http.MethodPost, "/api/v1/site/notification/test-email",
		`{"to":"not-an-email"}`)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}
