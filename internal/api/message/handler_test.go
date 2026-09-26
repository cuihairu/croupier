package message

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/dbenum"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

var messageDBSeq uint64

func newMessageTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	name := fmt.Sprintf("message_%d", atomic.AddUint64(&messageDBSeq, 1))
	db, err := gorm.Open(gsqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", name)), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))
	return db
}

func newMessageHandler(db *gorm.DB) *Handler {
	svcCtx := &svc.ServiceContext{MessageModel: model.NewMessageModel(db)}
	return NewHandler(NewService(svcCtx), config.SSEConfig{})
}

// newMessageHandlerWithAdmin 在 newMessageHandler 基础上接入 AdminModel
// 并落一个 admin 角色账号 "boss"——BUG-026 后发送侧（Send）仅 admin 可用，
// 发送主流程用例用它构造有权限的登录态。
func newMessageHandlerWithAdmin(t *testing.T) *Handler {
	t.Helper()
	db := newMessageTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Admin{}, &model.Role{}, &model.AdminRole{}))
	adminModel := model.NewAdminModel(db)
	require.NoError(t, adminModel.Create(context.Background(), &model.Admin{Username: "boss", Nickname: "Boss"}, "x"))
	boss, err := adminModel.FindByUsername(context.Background(), "boss")
	require.NoError(t, err)
	role := &model.Role{Name: "admin"}
	require.NoError(t, db.Create(role).Error)
	require.NoError(t, adminModel.AssignRole(context.Background(), boss.ID, role.ID))
	svcCtx := &svc.ServiceContext{MessageModel: model.NewMessageModel(db), AdminModel: adminModel}
	return NewHandler(NewService(svcCtx), config.SSEConfig{})
}

// withReqUsername 把登录名写进 request context（isBroadcaster →
// LoadCurrentAdmin 从 request context 读 username，与 gin context 的
// c.Set("username", ...) 是两条通道）。
func withReqUsername(ctx *gin.Context, username string) {
	ctx.Request = ctx.Request.WithContext(
		context.WithValue(ctx.Request.Context(), "username", username))
}

// seedMessage 经模型层直接落一条未读消息：收件侧用例不应依赖发送 API
// （BUG-026 后 Send 仅 admin 可用，无登录态的 handler.Send 做种子会 403）。
func seedMessage(t *testing.T, db *gorm.DB, to, msgType, content string) *model.Message {
	t.Helper()
	msg := &model.Message{
		To:      to,
		Type:    msgType,
		Content: content,
		Status:  dbenum.MessageStatusUnread,
	}
	require.NoError(t, model.NewMessageModel(db).Create(context.Background(), msg))
	return msg
}

func newMessageRequest(method, target, body string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(method, target, strings.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	return ctx, rec
}

func assertMessageErrorShape(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Contains(t, body, "error")
	assert.Contains(t, body, "message")
}

func TestHandler_List_Empty_Success(t *testing.T) {
	handler := newMessageHandler(newMessageTestDB(t))

	ctx, rec := newMessageRequest(http.MethodGet, "/api/v1/messages?page=1&pageSize=10", "")
	handler.List(ctx)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var resp MessagesListResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Empty(t, resp.Items)
	assert.Equal(t, int64(0), resp.Total)
}

func TestHandler_SendAndDetail_RoundTrip(t *testing.T) {
	handler := newMessageHandlerWithAdmin(t)

	sendCtx, sendRec := newMessageRequest(http.MethodPost, "/api/v1/messages",
		`{"to":"user-1","type":"notice","title":"Hello","content":"Welcome aboard","data":{"x":1}}`)
	// 发送侧需 admin 登录态（BUG-026）：isBroadcaster 读 request context
	withReqUsername(sendCtx, "boss")
	handler.Send(sendCtx)
	require.Equal(t, http.StatusOK, sendRec.Code, sendRec.Body.String())

	var sent MessageItem
	require.NoError(t, json.Unmarshal(sendRec.Body.Bytes(), &sent))
	assert.Equal(t, "user-1", sent.To)
	assert.Equal(t, "notice", sent.Type)
	assert.Equal(t, "Welcome aboard", sent.Content)

	// List reflects the new message.
	listCtx, listRec := newMessageRequest(http.MethodGet, "/api/v1/messages?page=1&pageSize=10", "")
	listCtx.Set("username", "user-1")
	handler.List(listCtx)
	require.Equal(t, http.StatusOK, listRec.Code)
	var listResp MessagesListResponse
	require.NoError(t, json.Unmarshal(listRec.Body.Bytes(), &listResp))
	require.Len(t, listResp.Items, 1)
	assert.Equal(t, sent.To, listResp.Items[0].To)

	// Fetch detail by id.
	detailCtx, detailRec := newMessageRequest(http.MethodGet, fmt.Sprintf("/api/v1/messages/%v", sent.ID), "")
	detailCtx.Params = gin.Params{{Key: "id", Value: fmt.Sprint(sent.ID)}}
	detailCtx.Set("username", "user-1")
	handler.Detail(detailCtx)
	require.Equal(t, http.StatusOK, detailRec.Code, detailRec.Body.String())
}

func TestHandler_Send_MissingFields_BadRequest(t *testing.T) {
	// 用 admin 登录态隔离出「参数校验」这一层（无登录态会先被
	// BUG-026 权限门禁 403，测不到校验逻辑）
	handler := newMessageHandlerWithAdmin(t)

	tests := []struct {
		name string
		body string
	}{
		{"missing to", `{"type":"notice","content":"c"}`},
		{"missing type", `{"to":"u","content":"c"}`},
		{"missing content", `{"to":"u","type":"notice"}`},
		{"empty object", `{}`},
		{"invalid json", `not-json`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, rec := newMessageRequest(http.MethodPost, "/api/v1/messages", tt.body)
			withReqUsername(ctx, "boss")
			handler.Send(ctx)
			assert.NotEqual(t, http.StatusOK, rec.Code, "expected rejection, got 200 body=%s", rec.Body.String())
			assertMessageErrorShape(t, rec)
		})
	}
}

func TestHandler_Detail_InvalidID_BadRequest(t *testing.T) {
	handler := newMessageHandler(newMessageTestDB(t))

	tests := []struct {
		name  string
		idVal string
	}{
		{"empty", ""},
		{"non-numeric", "abc"},
		{"zero", "0"},
		{"negative", "-1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, rec := newMessageRequest(http.MethodGet, "/api/v1/messages/"+tt.idVal, "")
			ctx.Params = gin.Params{{Key: "id", Value: tt.idVal}}
			handler.Detail(ctx)
			assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
			assertMessageErrorShape(t, rec)
		})
	}
}

func TestHandler_Detail_NotFound(t *testing.T) {
	handler := newMessageHandler(newMessageTestDB(t))

	ctx, rec := newMessageRequest(http.MethodGet, "/api/v1/messages/99999", "")
	ctx.Params = gin.Params{{Key: "id", Value: "99999"}}
	handler.Detail(ctx)

	assert.NotEqual(t, http.StatusOK, rec.Code, rec.Body.String())
	assertMessageErrorShape(t, rec)
}

func TestHandler_UnreadCount_Success(t *testing.T) {
	db := newMessageTestDB(t)
	handler := newMessageHandler(db)

	// Seed an unread message（模型层直种，发送权限见 BUG-026）。
	seedMessage(t, db, "u", "notice", "c")

	ctx, rec := newMessageRequest(http.MethodGet, "/api/v1/messages/unread-count", "")
	handler.UnreadCount(ctx)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var resp MessagesUnreadCountResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.GreaterOrEqual(t, resp.Count, int64(1))
}

func TestHandler_Stream_Success(t *testing.T) {
	db := newMessageTestDB(t)
	handler := newMessageHandler(db)

	// Seed a message so the stream has content（模型层直种）。
	seedMessage(t, db, "u", "notice", "streamed")

	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	cancelCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/messages/stream", nil).WithContext(cancelCtx)
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = req
	ctx.Set("username", "u")

	done := make(chan struct{})
	go func() {
		handler.Stream(ctx)
		close(done)
	}()

	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Stream handler did not exit")
	}

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}
