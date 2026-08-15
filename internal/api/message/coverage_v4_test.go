package message

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---- Service with nil MessageModel ----

func TestService_List_NilModel_V4(t *testing.T) {
	t.Parallel()
	svcCtx := &svc.ServiceContext{}
	svc := NewService(svcCtx)
	resp, err := svc.List(context.Background(), "user", &MessagesListRequest{Page: 1, PageSize: 10})
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Empty(t, resp.Items)
	assert.Equal(t, int64(0), resp.Total)
}

func TestService_Send_NilModel_V4(t *testing.T) {
	t.Parallel()
	svcCtx := &svc.ServiceContext{}
	svc := NewService(svcCtx)
	_, err := svc.Send(context.Background(), &MessageSendRequest{To: "u", Type: "notice", Content: "c"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "消息服务未初始化")
}

func TestService_Detail_NilModel_V4(t *testing.T) {
	t.Parallel()
	svcCtx := &svc.ServiceContext{}
	svc := NewService(svcCtx)
	_, err := svc.Detail(context.Background(), "user", &MessageDetailRequest{ID: "1"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "消息服务未初始化")
}

func TestService_Read_NilModel_V4(t *testing.T) {
	t.Parallel()
	svcCtx := &svc.ServiceContext{}
	svc := NewService(svcCtx)
	_, err := svc.Read(context.Background(), "user", &MessageReadRequest{ID: "1"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "消息服务未初始化")
}

func TestService_UnreadCount_NilModel_V4(t *testing.T) {
	t.Parallel()
	svcCtx := &svc.ServiceContext{}
	svc := NewService(svcCtx)
	resp, err := svc.UnreadCount(context.Background(), "user", &MessagesUnreadCountRequest{})
	require.NoError(t, err)
	assert.Equal(t, int64(0), resp.Count)
}

func TestService_Stream_NilModel_V4(t *testing.T) {
	t.Parallel()
	svcCtx := &svc.ServiceContext{}
	svc := NewService(svcCtx)
	resp, err := svc.Stream(context.Background(), "user", &StreamMessagesRequest{})
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Empty(t, resp.Items)
}

// ---- Handler with nil model (no DB) ----

func TestHandler_List_NilModel_V4(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)
	handler := NewHandler(NewService(&svc.ServiceContext{}), config.SSEConfig{})

	ctx, rec := newMessageRequest(http.MethodGet, "/api/v1/messages?page=1&pageSize=10", "")
	handler.List(ctx)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var resp MessagesListResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Empty(t, resp.Items)
}

func TestHandler_UnreadCount_NilModel_V4(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)
	handler := NewHandler(NewService(&svc.ServiceContext{}), config.SSEConfig{})

	ctx, rec := newMessageRequest(http.MethodGet, "/api/v1/messages/unread-count", "")
	handler.UnreadCount(ctx)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var resp MessagesUnreadCountResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, int64(0), resp.Count)
}

func TestHandler_Stream_Unauthorized_V4(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)
	handler := NewHandler(NewService(&svc.ServiceContext{}), config.SSEConfig{})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/messages/stream", nil)
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = req
	// No username set — should return unauthorized
	handler.Stream(ctx)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

// ---- Read ownership check ----

func TestService_Read_WrongUsername_V4(t *testing.T) {
	t.Parallel()
	db := newMessageTestDB(t)
	handler := newMessageHandler(db)

	// Send a message to user "owner"
	sendCtx, sendRec := newMessageRequest(http.MethodPost, "/api/v1/messages",
		`{"to":"owner","type":"notice","content":"private"}`)
	handler.Send(sendCtx)
	require.Equal(t, http.StatusOK, sendRec.Code, sendRec.Body.String())

	var sent MessageItem
	require.NoError(t, json.Unmarshal(sendRec.Body.Bytes(), &sent))

	// Try to read as different user
	readCtx, readRec := newMessageRequest(http.MethodGet, "/api/v1/messages/"+jsonStr(sent.ID), "")
	readCtx.Params = gin.Params{{Key: "id", Value: jsonStr(sent.ID)}}
	readCtx.Set("username", "wrong-user")
	handler.Read(readCtx)
	assert.NotEqual(t, http.StatusOK, readRec.Code, readRec.Body.String())
}

// ---- Detail ownership check ----

func TestService_Detail_WrongUsername_V4(t *testing.T) {
	t.Parallel()
	db := newMessageTestDB(t)
	handler := newMessageHandler(db)

	// Send message to "owner"
	sendCtx, sendRec := newMessageRequest(http.MethodPost, "/api/v1/messages",
		`{"to":"owner","type":"notice","content":"secret"}`)
	handler.Send(sendCtx)
	require.Equal(t, http.StatusOK, sendRec.Code, sendRec.Body.String())

	var sent MessageItem
	require.NoError(t, json.Unmarshal(sendRec.Body.Bytes(), &sent))

	// Try to access as different user
	detailCtx, detailRec := newMessageRequest(http.MethodGet, "/api/v1/messages/"+jsonStr(sent.ID), "")
	detailCtx.Params = gin.Params{{Key: "id", Value: jsonStr(sent.ID)}}
	detailCtx.Set("username", "wrong-user")
	handler.Detail(detailCtx)
	assert.NotEqual(t, http.StatusOK, detailRec.Code, detailRec.Body.String())
}

// ---- Read with invalid ID ----

func TestService_Read_InvalidID_V4(t *testing.T) {
	t.Parallel()
	db := newMessageTestDB(t)
	handler := newMessageHandler(db)

	ctx, rec := newMessageRequest(http.MethodGet, "/api/v1/messages/abc", "")
	ctx.Params = gin.Params{{Key: "id", Value: "abc"}}
	handler.Read(ctx)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// ---- Service.Send validation ----

func TestService_Send_EmptyContent_V4(t *testing.T) {
	t.Parallel()
	db := newMessageTestDB(t)
	handler := newMessageHandler(db)

	ctx, rec := newMessageRequest(http.MethodPost, "/api/v1/messages",
		`{"to":"u","type":"notice","content":"   "}`)
	handler.Send(ctx)
	assert.NotEqual(t, http.StatusOK, rec.Code, "expected error for whitespace-only content")
}

func TestService_Send_EmptyTo_V4(t *testing.T) {
	t.Parallel()
	db := newMessageTestDB(t)
	handler := newMessageHandler(db)

	ctx, rec := newMessageRequest(http.MethodPost, "/api/v1/messages",
		`{"to":"","type":"notice","content":"hello"}`)
	handler.Send(ctx)
	assert.NotEqual(t, http.StatusOK, rec.Code, "expected error for empty to")
}

// ---- sendMessagesEvent error path ----

func TestHandler_SendMessagesEvent_Error_V4(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)
	// Service with nil model will cause Stream to return empty items (not error)
	// so we test the success path of sendMessagesEvent
	handler := NewHandler(NewService(&svc.ServiceContext{}), config.SSEConfig{})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/messages/stream", nil)
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = req

	// sendMessagesEvent with nil model — should not panic, should produce SSE output
	handler.sendMessagesEvent(ctx, "testuser")

	// Should have written SSE event
	body := rec.Body.String()
	assert.Contains(t, body, "event: messages")
	assert.Contains(t, body, "data:")
}

// ---- Stream handler with message ----

func TestHandler_Stream_WithMessages_V4(t *testing.T) {
	t.Parallel()
	db := newMessageTestDB(t)
	handler := newMessageHandler(db)

	// Seed a message
	sendCtx, sendRec := newMessageRequest(http.MethodPost, "/api/v1/messages",
		`{"to":"stream-user","type":"notice","content":"stream-content"}`)
	handler.Send(sendCtx)
	require.Equal(t, http.StatusOK, sendRec.Code)

	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	cancelCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/messages/stream", nil).WithContext(cancelCtx)
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = req
	ctx.Set("username", "stream-user")

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

	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "event: messages")
}
