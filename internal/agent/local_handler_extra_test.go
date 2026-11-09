package agent

// local_handler 补测：四个 ops 入口的全部臂（unmarshal 失败 / ops 未装配 /
// 成功 / ops 失败透传——UpdateProcess 与 SetSchedulePause 的失败臂经内嵌
// mockOpsServer 的覆写桩注入）、handleRequest 对四个 ops 消息的派发臂、
// callProvider 的无会话回落拨号臂、Conn 为空与函数不匹配的跳过臂、任务延续
// 无会话的显式失败臂、callLocalProvider 的 outboundTLS 配置注入臂、
// handleProviderDrain 的 RetryAfterMs 宽限臂与取消错误包装臂、
// DrainProviderSession 的宽限兜底臂、listener 侧 handleDrain 的握手会话
// 回退臂与共享 handler 错误透传臂、provider_keepalive 构造。
//
// 诚实跳过：无——全部臂在现有桩件（net.Pipe 会话 / mockOpsServer /
// 不可达地址拨号）下确定性可达。

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/cuihairu/croupier/agents/sidecar"
	"github.com/cuihairu/croupier/internal/platform/agentlocal"
	"github.com/cuihairu/croupier/internal/platform/tlsutil"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
)

// reqBytes 序列化请求体（失败即测试失败）。
func reqBytes(t *testing.T, msg proto.Message) []byte {
	t.Helper()
	data, err := proto.Marshal(msg)
	require.NoError(t, err)
	return data
}

// updateProcessErrOps / schedulePauseErrOps：mockOpsServer 固定成功，覆写
// 对应方法注入错误，驱动 handler 的 ops 失败透传臂。
type updateProcessErrOps struct{ mockOpsServer }

func (m *updateProcessErrOps) UpdateProcess(context.Context, *opsv1.UpdateProcessRequest) (*opsv1.UpdateProcessResponse, error) {
	return nil, errors.New("update refused")
}

type schedulePauseErrOps struct{ mockOpsServer }

func (m *schedulePauseErrOps) SetSchedulePause(context.Context, *opsv1.SchedulePauseRequest) (*opsv1.SchedulePauseResponse, error) {
	return nil, errors.New("pause refused")
}

func TestOpsUpdateProcessArms(t *testing.T) {
	h := NewLocalHandler(agentlocal.NewLocalStore(), t.TempDir(), "agent-ops", nil)

	// unmarshal 失败
	_, err := h.handleUpdateProcess(context.Background(), []byte{0xff, 0xff, 0xff})
	require.ErrorContains(t, err, "unmarshal UpdateProcessRequest")

	// ops 未装配
	_, err = h.handleUpdateProcess(context.Background(), nil)
	require.ErrorContains(t, err, "ops server not configured")

	// 成功
	h.SetOpsServer(&mockOpsServer{})
	resp, err := h.handleUpdateProcess(context.Background(), reqBytes(t,
		&opsv1.UpdateProcessRequest{ProcessName: "game", StagedPath: "/tmp/new", ExpectedSha256: "abc"}))
	require.NoError(t, err)
	require.NotNil(t, resp)

	// ops 失败透传
	h.SetOpsServer(&updateProcessErrOps{})
	_, err = h.handleUpdateProcess(context.Background(),
		reqBytes(t, &opsv1.UpdateProcessRequest{ProcessName: "game"}))
	require.ErrorContains(t, err, "update process")
	require.ErrorContains(t, err, "update refused")
}

func TestOpsSchedulePauseArms(t *testing.T) {
	h := NewLocalHandler(agentlocal.NewLocalStore(), t.TempDir(), "agent-ops", nil)

	_, err := h.handleSchedulePause(context.Background(), []byte{0xff, 0xff, 0xff})
	require.ErrorContains(t, err, "unmarshal SchedulePauseRequest")

	_, err = h.handleSchedulePause(context.Background(), nil)
	require.ErrorContains(t, err, "ops server not configured")

	h.SetOpsServer(&mockOpsServer{})
	resp, err := h.handleSchedulePause(context.Background(),
		reqBytes(t, &opsv1.SchedulePauseRequest{ProcessName: "game"}))
	require.NoError(t, err)
	require.NotNil(t, resp)

	h.SetOpsServer(&schedulePauseErrOps{})
	_, err = h.handleSchedulePause(context.Background(),
		reqBytes(t, &opsv1.SchedulePauseRequest{ProcessName: "game"}))
	require.ErrorContains(t, err, "schedule pause")
	require.ErrorContains(t, err, "pause refused")
}

func TestOpsPushFileArms(t *testing.T) {
	h := NewLocalHandler(agentlocal.NewLocalStore(), t.TempDir(), "agent-ops", nil)

	_, err := h.handlePushFile(context.Background(), []byte{0xff, 0xff, 0xff})
	require.ErrorContains(t, err, "unmarshal PushFileRequest")

	// 成功（PushFileResponse.Success=true）
	h.SetOpsServer(&mockOpsServer{})
	resp, err := h.handlePushFile(context.Background(), reqBytes(t, &opsv1.PushFileRequest{}))
	require.NoError(t, err)
	out := &opsv1.PushFileResponse{}
	require.NoError(t, proto.Unmarshal(resp, out))
	require.True(t, out.Success)

	// ops 失败透传
	h.SetOpsServer(&mockOpsServer{pushFileErr: errors.New("disk full")})
	_, err = h.handlePushFile(context.Background(), reqBytes(t, &opsv1.PushFileRequest{}))
	require.ErrorContains(t, err, "push file")
	require.ErrorContains(t, err, "disk full")
}

func TestOpsGetSupervisorLogArms(t *testing.T) {
	h := NewLocalHandler(agentlocal.NewLocalStore(), t.TempDir(), "agent-ops", nil)

	// 空 body 跳过 unmarshal，直接命中 ops 未装配
	_, err := h.handleGetSupervisorLog(context.Background(), nil)
	require.ErrorContains(t, err, "ops server not configured")

	// 非空坏 body 的 unmarshal 失败臂
	_, err = h.handleGetSupervisorLog(context.Background(), []byte{0xff})
	require.ErrorContains(t, err, "unmarshal GetSupervisorLogRequest")

	// 成功
	h.SetOpsServer(&mockOpsServer{supLogResp: &opsv1.GetSupervisorLogResponse{Content: []byte("log line")}})
	resp, err := h.handleGetSupervisorLog(context.Background(),
		reqBytes(t, &opsv1.GetSupervisorLogRequest{MaxBytes: 128}))
	require.NoError(t, err)
	out := &opsv1.GetSupervisorLogResponse{}
	require.NoError(t, proto.Unmarshal(resp, out))
	require.Equal(t, "log line", string(out.Content))

	// ops 失败透传
	h.SetOpsServer(&mockOpsServer{supLogErr: errors.New("log rotated")})
	_, err = h.handleGetSupervisorLog(context.Background(),
		reqBytes(t, &opsv1.GetSupervisorLogRequest{MaxBytes: 128}))
	require.ErrorContains(t, err, "get supervisor log")
}

// handleRequest 对四个 ops 消息的派发臂。
func TestDispatchOpsMessageCases(t *testing.T) {
	h := NewLocalHandler(agentlocal.NewLocalStore(), t.TempDir(), "agent-dispatch", nil)
	h.SetOpsServer(&mockOpsServer{supLogResp: &opsv1.GetSupervisorLogResponse{}})

	cases := []struct {
		name  string
		msgID uint32
		data  []byte
	}{
		{"update", protocol.MsgUpdateProcessRequest,
			reqBytes(t, &opsv1.UpdateProcessRequest{ProcessName: "game"})},
		{"pause", protocol.MsgSchedulePauseRequest,
			reqBytes(t, &opsv1.SchedulePauseRequest{ProcessName: "game"})},
		{"push", protocol.MsgPushFileRequest,
			reqBytes(t, &opsv1.PushFileRequest{})},
		{"suplog", protocol.MsgGetSupervisorLogRequest,
			reqBytes(t, &opsv1.GetSupervisorLogRequest{MaxBytes: 64})},
	}
	for _, tc := range cases {
		resp, err := h.Handle(context.Background(), tc.msgID, 7, tc.data)
		require.NoError(t, err, tc.name)
		require.NotNil(t, resp, tc.name)
	}
}

// 无会话存储时 callProvider 回落 legacy 拨号；空地址立即拨号失败。
func TestCallProviderFallsBackToLocalDial(t *testing.T) {
	h := NewLocalHandler(agentlocal.NewLocalStore(), t.TempDir(), "agent-fallback", nil)
	_, err := h.callProvider(context.Background(), "fn.x", nil, "",
		protocol.MsgInvokeRequest, []byte("ping"))
	require.Error(t, err)
}

// callProvider 跳过 Conn 为空的会话与函数不匹配的会话，随后回落 legacy
// 地址拨号（127.0.0.1:1 连接拒绝）。
func TestCallProviderSkipsUnusableSessions(t *testing.T) {
	handler, store := newDrainTestHandler(t)

	nilConn := sidecar.NewProviderSession(nil)
	nilConn.SessionID = "s-nil"
	nilConn.ServiceID = "svc-nil"
	nilConn.Functions = []*sdkv1.ProviderFunctionDescriptor{{Id: "fn.x"}}
	require.NoError(t, store.Add(nilConn))

	mismatch := newEchoSession(t, "s-wrong", "svc-wrong", "fn.other", nil, nil)
	require.NoError(t, store.Add(mismatch))

	_, err := handler.callProvider(context.Background(), "fn.x", nil, "127.0.0.1:1",
		protocol.MsgInvokeRequest, []byte("ping"))
	require.Error(t, err)
}

// 任务延续且无会话命中、fallbackAddr 为空 → 显式失败，不得拿空地址拨号。
func TestCallProviderTaskContinuationWithoutSession(t *testing.T) {
	handler, _ := newDrainTestHandler(t) // 会话存储存在但为空
	_, err := handler.callProvider(context.Background(), "fn.y",
		map[string]string{"taskId": "t1"}, "", protocol.MsgInvokeRequest, []byte("x"))
	require.ErrorContains(t, err, "no provider session for task continuation")
}

// handleInvoke 的任务延续路径（addr 留空交给 callProvider 会话路由）：无
// 会话命中时错误经「invoke local provider: 」包装透传。
func TestInvokeTaskContinuationWrapsSessionMiss(t *testing.T) {
	handler, _ := newDrainTestHandler(t) // 会话存储存在但为空
	_, err := handler.Handle(context.Background(), protocol.MsgInvokeRequest, 1,
		reqBytes(t, &sdkv1.InvokeRequest{
			FunctionId: "fn.z",
			Metadata:   map[string]string{"taskId": "t9"},
		}))
	require.ErrorContains(t, err, "invoke local provider")
	require.ErrorContains(t, err, "no provider session for task continuation")
}

// SetTLSConfig 注入的 outboundTLS 配置须传入拨号配置：缺失证书文件在
// NewClient 的证书加载处失败，证明 tlsCfg 分支被执行。
func TestCallLocalProviderAppliesTLSConfig(t *testing.T) {
	h := NewLocalHandler(agentlocal.NewLocalStore(), t.TempDir(), "agent-tls", nil)
	dir := t.TempDir()
	h.SetTLSConfig(&tlsutil.ClientTLSConfig{
		CertFile:           filepath.Join(dir, "missing.crt"),
		KeyFile:            filepath.Join(dir, "missing.key"),
		CAFile:             filepath.Join(dir, "missing.ca"),
		ServerName:         "provider.local",
		InsecureSkipVerify: true,
	})
	_, err := h.callLocalProvider(context.Background(), "127.0.0.1:1",
		protocol.MsgInvokeRequest, []byte("x"))
	// CA 读取先于证书对加载失败——错误自 create tls config 透传，即证明
	// tlsCfg 分支已执行并注入了证书路径。
	require.ErrorContains(t, err, "create tls config")
}

// RetryAfterMs>0 覆盖默认宽限；无目标会话时立即 ACK。
func TestHandleProviderDrainRetryAfterGrace(t *testing.T) {
	handler, _ := newDrainTestHandler(t)
	resp, err := handler.handleProviderDrain(context.Background(),
		reqBytes(t, &sdkv1.ProviderDrainRequest{SessionId: "gone", RetryAfterMs: 250}))
	require.NoError(t, err)
	require.NotNil(t, resp)
}

// 在途未归零且 ctx 已取消：DrainProviderSession 返回 ctx.Err()，
// handleProviderDrain 包装透传。
func TestHandleProviderDrainCancelledWait(t *testing.T) {
	handler, store := newDrainTestHandler(t)
	sess := newEchoSession(t, "drain-busy", "svc-busy", "fn.busy", nil, nil)
	sess.AddInflight(1)
	require.NoError(t, store.Add(sess))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := handler.handleProviderDrain(ctx,
		reqBytes(t, &sdkv1.ProviderDrainRequest{SessionId: "drain-busy"}))
	require.ErrorContains(t, err, "drain provider sessions")
	require.ErrorContains(t, err, "context canceled")
}

// 宽限 <=0 → 回落默认宽限；空存储无目标，直接返回。
func TestDrainProviderSessionDefaultsGrace(t *testing.T) {
	handler, _ := newDrainTestHandler(t)
	n, err := handler.DrainProviderSession(context.Background(), "", false, 0)
	require.NoError(t, err)
	require.Equal(t, 0, n)
}

func TestProviderKeepaliveConstructor(t *testing.T) {
	ka := NewProviderKeepalive(NewProviderSessionStore(), 50*time.Millisecond, nil)
	require.NotNil(t, ka)
}

// listener 侧 handleDrain：请求省略 session_id 时回退到握手绑定的会话 id；
// 未装配共享 LocalHandler 时仅应答。
func TestListenerDrainStateFallback(t *testing.T) {
	listener, err := NewTCPLocalListener(
		&TCPLocalListenerConfig{Address: "127.0.0.1:0"}, NewProviderSessionStore(), nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })

	ph := &providerSessionHandler{listener: listener, registered: true}
	resp, err := ph.Handle(context.Background(), protocol.MsgProviderDrainRequest, 1,
		reqBytes(t, &sdkv1.ProviderDrainRequest{}))
	require.NoError(t, err)
	require.NotNil(t, resp)
}

// listener 侧 drain 命中共享 LocalHandler：在途未归零且 ctx 已取消时错误
// 透传给 SDK 连接。
func TestListenerDrainReturnsSharedHandlerError(t *testing.T) {
	handler, store := newDrainTestHandler(t)
	sess := newEchoSession(t, "drain-l", "svc-l", "fn.l", nil, nil)
	sess.AddInflight(1)
	require.NoError(t, store.Add(sess))

	listener, err := NewTCPLocalListener(
		&TCPLocalListenerConfig{Address: "127.0.0.1:0"}, store, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })
	listener.SetLocalHandler(handler)

	ph := &providerSessionHandler{listener: listener, registered: true, sessionID: "drain-l"}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = ph.Handle(ctx, protocol.MsgProviderDrainRequest, 1,
		reqBytes(t, &sdkv1.ProviderDrainRequest{SessionId: "drain-l"}))
	require.ErrorContains(t, err, "context canceled")
}
