// 补齐 announcement 包剩余可覆盖分支（group C）：
// handler.Create 的 service 校验失败分支（audience=role 未指定 role）。
package announcement

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

// 请求体通过 JSON 绑定后，service 的 validateAudience 失败 → response.Error
// 映射 400，覆盖 handler.Create 的 service 错误分支。
func TestAnnouncementHandler_Create_ServiceValidationFailure(t *testing.T) {
	_, r := newHandlerFixture(t)

	w := doJSON2(r, http.MethodPost, "/admin/announcements",
		`{"title":"维护","contentMd":"c","audience":"role"}`, "")
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "audience=role")
}
