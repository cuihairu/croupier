package message

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// BUG-026 regression: in-app message "sending" (point-to-point single send)
// previously had no permission check at all, so any logged-in user could
// deliver a message to an arbitrary account. After the fix it shares the same
// gate as broadcast (Broadcast) — the admin role only; the receiving side
// (List/Read/UnreadCount) stays open to every logged-in user.
func TestHandler_Send_RequiresAdminRole(t *testing.T) {
	h, _ := newBroadcastAuthFixture(t)
	r := gin.New()
	r.POST("/messages", h.Send)
	r.GET("/messages", h.List)

	body := `{"to":"alice","type":"system","title":"点对点","content":"仅管理员可发"}`

	// unauthenticated → 403
	w := broadcastReq(r, http.MethodPost, "/messages", body, "")
	assert.Equal(t, http.StatusForbidden, w.Code, w.Body.String())

	// non-admin role (ops) → 403
	w = broadcastReq(r, http.MethodPost, "/messages", body, "alice")
	assert.Equal(t, http.StatusForbidden, w.Code, w.Body.String())

	// admin → 200 and the message really lands in the database (visible to the recipient)
	w = broadcastReq(r, http.MethodPost, "/messages", body, "boss")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"to":"alice"`)

	// Receiving side unaffected: ordinary users can still read their own
	// inbox.
	w = broadcastReq(r, http.MethodGet, "/messages", "", "alice")
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"to":"alice"`)
}
