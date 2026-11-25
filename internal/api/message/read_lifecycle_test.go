package message

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/dbenum"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// End-to-end regression: create unread → mark read → the list status flips
// correctly. Previously BuildMessageDTO put the raw dbenum integer enum into
// the map, so the stringValue assertion against a string failed and the list
// status was always an empty string — read messages stayed unread in the UI.
func TestReadLifecycleE2E(t *testing.T) {
	db := newMessageTestDB(t)
	svcCtx := &svc.ServiceContext{MessageModel: model.NewMessageModel(db)}
	require.NoError(t, svcCtx.MessageModel.Create(context.Background(), &model.Message{
		To: "alice", Type: "system", Title: "t", Content: "c", Status: dbenum.MessageStatusUnread,
	}))
	handler := NewHandler(NewService(svcCtx), config.SSEConfig{})

	gin.SetMode(gin.TestMode)
	r := gin.New()
	authed := func(h gin.HandlerFunc) gin.HandlerFunc {
		return func(c *gin.Context) {
			c.Set("username", "alice")
			h(c)
		}
	}
	r.GET("/messages", authed(handler.List))
	r.POST("/messages/:id/read", authed(handler.Read))

	do := func(method, path string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(""))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}

	w1 := do(http.MethodGet, "/messages?status=unread")
	assert.Contains(t, w1.Body.String(), `"status":"unread"`, w1.Body.String())

	w2 := do(http.MethodPost, "/messages/1/read")
	require.Equal(t, http.StatusOK, w2.Code, w2.Body.String())

	w3 := do(http.MethodGet, "/messages?status=unread")
	assert.NotContains(t, w3.Body.String(), `"status":"unread"`, "已读后不应再出现在未读列表: "+w3.Body.String())

	w4 := do(http.MethodGet, "/messages?status=all")
	assert.Contains(t, w4.Body.String(), `"status":"read"`, w4.Body.String())
}
