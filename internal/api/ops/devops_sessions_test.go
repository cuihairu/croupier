package ops

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cuihairu/croupier/core/transport"
	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/platform/devopssessions"
	"github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

// devopsBusyCaller input 恒回 busy 的假 agent（复用包内 fakeResolver 的
// conns 形状装配）。
type devopsBusyCaller struct {
	calls []uint32
}

func (f *devopsBusyCaller) Call(_ context.Context, msgID uint32, reqBody []byte) (uint32, []byte, error) {
	f.calls = append(f.calls, msgID)
	switch msgID {
	case protocol.MsgDevopsSessionOpenRequest:
		req := &opsv1.DevopsSessionOpenRequest{}
		_ = proto.Unmarshal(reqBody, req)
		body, _ := proto.Marshal(&opsv1.DevopsSessionOpenResponse{Ok: true, SessionId: req.GetSessionId(), Channel: "ssh"})
		return protocol.GetResponseMsgID(msgID), body, nil
	case protocol.MsgDevopsSessionInputRequest:
		body, _ := proto.Marshal(&opsv1.DevopsSessionInputResponse{Ok: false, Error: "busy"})
		return protocol.GetResponseMsgID(msgID), body, nil
	case protocol.MsgDevopsSessionCloseRequest:
		body, _ := proto.Marshal(&opsv1.DevopsSessionCloseResponse{Ok: true})
		return protocol.GetResponseMsgID(msgID), body, nil
	}
	return protocol.GetResponseMsgID(msgID), []byte{}, nil
}

// newDevopsScopeStore scope 化 registry store（agent-a ∈ demo/prod）。
func newDevopsScopeStore(t *testing.T) *registry.Store {
	t.Helper()
	store := registry.NewStore()
	require.NoError(t, store.UpsertAgent(&registry.AgentSession{AgentID: "agent-a", GameID: "demo", Env: "prod"}))
	return store
}

// newDevopsRESTEnv 面板面测试装配：hub + 假 resolver + scope store，返回
// service 与已开会话 id。
func newDevopsRESTEnv(t *testing.T, caller transport.SessionCaller) (*Service, *devopssessions.Hub, string) {
	t.Helper()
	hub := devopssessions.NewHub(nil)
	svcCtx := &svc.ServiceContext{
		RegistryStore:        newDevopsScopeStore(t),
		DevopsSessions:       hub,
		AgentSessionResolver: &fakeResolver{conns: map[string]transport.SessionCaller{"agent-a": caller}},
	}
	s := NewService(svcCtx)
	ctx := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "demo", Env: "prod"})
	sess, err := s.DevopsSessionOpen(ctx, &DevopsSessionOpenRequest{
		AgentID: "agent-a", Protocol: "ssh", Host: "h1", Port: 22, Cols: 80, Rows: 24,
	})
	require.NoError(t, err)
	return s, hub, sess.SessionID
}

func TestDevopsSessionOpenValidations(t *testing.T) {
	s := NewService(&svc.ServiceContext{RegistryStore: newDevopsScopeStore(t)})
	ctx := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "demo", Env: "prod"})

	_, err := s.DevopsSessionOpen(ctx, &DevopsSessionOpenRequest{Host: "h", Protocol: "ssh", Port: 22})
	require.ErrorContains(t, err, "agentId is required")
	_, err = s.DevopsSessionOpen(ctx, &DevopsSessionOpenRequest{AgentID: "agent-a", Protocol: "ssh", Port: 22})
	require.ErrorContains(t, err, "host is required")
	_, err = s.DevopsSessionOpen(ctx, &DevopsSessionOpenRequest{AgentID: "agent-a", Host: "h", Port: 22})
	require.ErrorContains(t, err, "protocol is required")
	_, err = s.DevopsSessionOpen(ctx, &DevopsSessionOpenRequest{AgentID: "agent-a", Host: "h", Protocol: "ssh", Port: 0})
	require.ErrorContains(t, err, "port must be positive")

	// 跨 scope 打开（AGENT-001 fail-closed 同口径）。
	otherCtx := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "other", Env: "prod"})
	_, err = s.DevopsSessionOpen(otherCtx, &DevopsSessionOpenRequest{AgentID: "agent-a", Protocol: "ssh", Host: "h", Port: 22})
	require.ErrorContains(t, err, "does not belong")

	// relay 未装配 → 503 devops_relay_not_configured。
	s2 := NewService(&svc.ServiceContext{RegistryStore: newDevopsScopeStore(t)})
	_, err = s2.DevopsSessionOpen(ctx, &DevopsSessionOpenRequest{AgentID: "agent-a", Protocol: "ssh", Host: "h", Port: 22})
	codeErr, ok := err.(*errorx.CodeError)
	require.True(t, ok, "expected *errorx.CodeError, got %T", err)
	require.Equal(t, http.StatusServiceUnavailable, codeErr.Code)
	require.Equal(t, "devops_relay_not_configured", codeErr.StableCode)
}

func TestDevopsSessionOpenHappyPathAndList(t *testing.T) {
	s, hub, sessionID := newDevopsRESTEnv(t, &devopsBusyCaller{})
	require.NotEmpty(t, sessionID)

	// 列表含该会话（scope 内）；跨 scope 上下文看不到。
	ctx := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "demo", Env: "prod"})
	list, err := s.DevopsSessionsList(ctx)
	require.NoError(t, err)
	require.Len(t, list.Sessions, 1)
	require.Equal(t, sessionID, list.Sessions[0].SessionID)
	require.Equal(t, "agent-a", list.Sessions[0].AgentID)
	require.Equal(t, "ssh", list.Sessions[0].Channel)

	otherCtx := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "other", Env: "prod"})
	list, err = s.DevopsSessionsList(otherCtx)
	require.NoError(t, err)
	require.Empty(t, list.Sessions)

	_, ok := hub.Get(sessionID)
	require.True(t, ok)
}

func TestDevopsSessionInputMapping(t *testing.T) {
	s, _, sessionID := newDevopsRESTEnv(t, &devopsBusyCaller{})
	ctx := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "demo", Env: "prod"})

	// 背压 → 429 session_busy（稳定码）。
	err := s.DevopsSessionInput(ctx, sessionID, "ls\n")
	codeErr, ok := err.(*errorx.CodeError)
	require.True(t, ok, "expected *errorx.CodeError, got %T", err)
	require.Equal(t, http.StatusTooManyRequests, codeErr.Code)
	require.Equal(t, "session_busy", codeErr.StableCode)

	// 未知会话 → 404。
	err = s.DevopsSessionInput(ctx, "nope", "x")
	codeErr, ok = err.(*errorx.CodeError)
	require.True(t, ok)
	require.Equal(t, http.StatusNotFound, codeErr.Code)

	// 跨 scope（会话属主 agent-a ∈ demo/prod）→ 拒绝。
	otherCtx := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "other", Env: "prod"})
	err = s.DevopsSessionInput(otherCtx, sessionID, "x")
	require.ErrorContains(t, err, "does not belong")

	// resize 与 close 正常路径（wire 调用 + close 收束）。
	require.NoError(t, s.DevopsSessionResize(ctx, sessionID, 120, 40))
	require.NoError(t, s.DevopsSessionClose(ctx, sessionID, "panel close"))
	// close 幂等语义：会话已收束，再关 → 404。
	err = s.DevopsSessionClose(ctx, sessionID, "again")
	require.ErrorContains(t, err, "not found")
}

func TestMapDevopsRelayError(t *testing.T) {
	require.Nil(t, mapDevopsRelayError(nil))
	err := mapDevopsRelayError(devopssessions.ErrSessionBusy)
	codeErr, ok := err.(*errorx.CodeError)
	require.True(t, ok)
	require.Equal(t, "session_busy", codeErr.StableCode)
	err = mapDevopsRelayError(devopssessions.ErrSessionNotFound)
	require.Contains(t, err.Error(), "session not found")
}

func TestDevopsSessionStreamSSE(t *testing.T) {
	s, hub, sessionID := newDevopsRESTEnv(t, &devopsBusyCaller{})
	handler := NewHandler(s)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/stream/:sessionId", handler.DevopsSessionStream)

	// 请求前路由的输出事件落入回放环（AttachConsumer 快照先行），SSE 首屏
	// 必含——回放语义的直接验证。
	hub.RouteEvent("agent-a", &opsv1.DevopsSessionEvent{SessionId: sessionID, Kind: "output", Data: []byte("hi")})

	rec := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		defer close(done)
		router.ServeHTTP(rec, httptest.NewRequest("GET", "/stream/"+sessionID, nil))
	}()

	// Attach 是 handler 第一步；短暂等待后以 end 帧收束流。
	time.Sleep(100 * time.Millisecond)
	hub.RouteEvent("agent-a", &opsv1.DevopsSessionEvent{SessionId: sessionID, Kind: "end", Outcome: "closed"})
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("SSE handler never returned")
	}

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Header().Get("Content-Type"), "text/event-stream")
	body := rec.Body.String()
	require.Contains(t, body, "event: output")
	// 输出 base64（"hi" → aGk=）。
	require.Contains(t, body, "aGk=")
	require.Contains(t, body, "event: end")
	require.Contains(t, body, `"outcome":"closed"`)
}

func TestDevopsSessionStreamUnknownSession(t *testing.T) {
	s, _, _ := newDevopsRESTEnv(t, &devopsBusyCaller{})
	handler := NewHandler(s)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/stream/:sessionId", handler.DevopsSessionStream)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest("GET", "/stream/nope", nil))
	require.Equal(t, http.StatusNotFound, rec.Code)
}
