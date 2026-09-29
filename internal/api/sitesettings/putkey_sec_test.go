package sitesettings

// 出站安全与限制键（OPEN-ISSUES #56）：清单格式校验（端口 1-65535、
// IP/CIDR、域名后缀禁协议/路径），空串 = 不限；ssrfProtection 走 bool
// 校验（validateValue 通用路径）。

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPutKey_SecOutboundKeys(t *testing.T) {
	r, _ := setupRouterWithDB(t)

	// allowPorts：非法片段拒绝（越界/非数字/空片段）
	for _, bad := range []string{"70000", "443,bad", "0", "443,,8080"} {
		rec := doSiteReq(t, r, http.MethodPut, "/api/v1/site/sec.allowPorts", `{"value":"`+bad+`"}`)
		require.Equal(t, http.StatusBadRequest, rec.Code, bad)
		assert.Contains(t, rec.Body.String(), "1-65535")
	}

	// allowPorts：合法清单与空串（清空 = 不限）
	for _, ok := range []string{"443", "443, 8080", ""} {
		rec := doSiteReq(t, r, http.MethodPut, "/api/v1/site/sec.allowPorts", `{"value":"`+ok+`"}`)
		assert.Equal(t, http.StatusOK, rec.Code, ok)
	}

	// allowIPs：非法 IP / 非法 CIDR 拒绝
	rec := doSiteReq(t, r, http.MethodPut, "/api/v1/site/sec.allowIPs", `{"value":"10.0.0.0/8"}`)
	assert.Equal(t, http.StatusOK, rec.Code)
	rec = doSiteReq(t, r, http.MethodPut, "/api/v1/site/sec.allowIPs", `{"value":"notanip"}`)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "非法")
	rec = doSiteReq(t, r, http.MethodPut, "/api/v1/site/sec.allowIPs", `{"value":"10.0.0.0/99"}`)
	require.Equal(t, http.StatusBadRequest, rec.Code)

	// domainFilter：协议/路径拒绝；域名后缀与空串通过
	rec = doSiteReq(t, r, http.MethodPut, "/api/v1/site/sec.domainFilter", `{"value":"http://example.com"}`)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "不带协议")
	rec = doSiteReq(t, r, http.MethodPut, "/api/v1/site/sec.domainFilter", `{"value":"example.com/x"}`)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	for _, ok := range []string{"example.com", "example.com, foo.io", ""} {
		rec = doSiteReq(t, r, http.MethodPut, "/api/v1/site/sec.domainFilter", `{"value":"`+ok+`"}`)
		assert.Equal(t, http.StatusOK, rec.Code, ok)
	}

	// ssrfProtection：bool 键（非法类型拒绝，合法布尔通过）
	rec = doSiteReq(t, r, http.MethodPut, "/api/v1/site/sec.ssrfProtection", `{"value":"on"}`)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	rec = doSiteReq(t, r, http.MethodPut, "/api/v1/site/sec.ssrfProtection", `{"value":true}`)
	assert.Equal(t, http.StatusOK, rec.Code)

	// 未声明的 sec.* 键仍拒绝
	rec = doSiteReq(t, r, http.MethodPut, "/api/v1/site/sec.unknown", `{"value":"x"}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	// net.* 出站调用策略（OPEN-ISSUES #57）：int 通用校验（负数/越界拒，
	// 0 = 沿用调用方缺省）
	for key := range map[string]struct{}{
		"net.requestTimeoutMs": {},
		"net.maxRetries":       {},
		"net.retryBackoffMs":   {},
	} {
		rec = doSiteReq(t, r, http.MethodPut, "/api/v1/site/"+key, `{"value":-1}`)
		require.Equal(t, http.StatusBadRequest, rec.Code, key)
		rec = doSiteReq(t, r, http.MethodPut, "/api/v1/site/"+key, `{"value":70000}`)
		require.Equal(t, http.StatusBadRequest, rec.Code, key)
		rec = doSiteReq(t, r, http.MethodPut, "/api/v1/site/"+key, `{"value":3}`)
		assert.Equal(t, http.StatusOK, rec.Code, key)
	}
}
