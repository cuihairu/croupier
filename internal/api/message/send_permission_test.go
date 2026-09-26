package message

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// BUG-026 回归：站内信「发送」（点对点单发）此前没有任何权限校验，任何登录
// 用户都能给任意账号投递消息。修复后与群发（Broadcast）同门槛——仅 admin
// 角色；收件侧（List/Read/UnreadCount）对所有登录用户保持开放。
func TestHandler_Send_RequiresAdminRole(t *testing.T) {
	h, _ := newBroadcastAuthFixture(t)
	r := gin.New()
	r.POST("/messages", h.Send)
	r.GET("/messages", h.List)

	body := `{"to":"alice","type":"system","title":"点对点","content":"仅管理员可发"}`

	// 未认证 → 403
	w := broadcastReq(r, http.MethodPost, "/messages", body, "")
	assert.Equal(t, http.StatusForbidden, w.Code, w.Body.String())

	// 非 admin 角色（ops）→ 403
	w = broadcastReq(r, http.MethodPost, "/messages", body, "alice")
	assert.Equal(t, http.StatusForbidden, w.Code, w.Body.String())

	// admin → 200 且消息真实落库（接收人可见）
	w = broadcastReq(r, http.MethodPost, "/messages", body, "boss")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"to":"alice"`)

	// 收件侧不受影响：普通用户仍能读自己的收件箱
	w = broadcastReq(r, http.MethodGet, "/messages", "", "alice")
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"to":"alice"`)
}
