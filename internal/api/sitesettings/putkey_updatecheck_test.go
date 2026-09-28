package sitesettings

// 系统维护（OPEN-ISSUES #52）新增 L3 键 system.updateCheckUrl 的写入面：
// 键在白名单、值为 http(s) URL 或空串清空；非 URL 被拒绝。

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPutKey_SystemUpdateCheckURL(t *testing.T) {
	r, _ := setupRouterWithDB(t)

	// 非 URL 被拒
	rec := doSiteReq(t, r, http.MethodPut, "/api/v1/site/system.updateCheckUrl", `{"value":"not-a-url"}`)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "http(s) URL")

	// 合法 URL 落库
	rec = doSiteReq(t, r, http.MethodPut, "/api/v1/site/system.updateCheckUrl", `{"value":"https://releases.example.com/latest.json"}`)
	require.Equal(t, http.StatusOK, rec.Code)

	// 空串 = 清除更新源（允许，回未配置语义）
	rec = doSiteReq(t, r, http.MethodPut, "/api/v1/site/system.updateCheckUrl", `{"value":""}`)
	assert.Equal(t, http.StatusOK, rec.Code)
}
