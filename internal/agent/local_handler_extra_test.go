package agent

// local_handler supplementary tests: every arm of the four ops entry points
// (unmarshal failure / ops not wired / success / ops failure pass-through —
// the UpdateProcess and SetSchedulePause failure arms are injected via
// override stubs embedded in mockOpsServer), handleRequest's dispatch arms for
// the four ops messages, callProvider's no-session fallback dial arm, the
// skip arms for nil Conn and function mismatch, the explicit failure arm for
// task continuation without a session, callLocalProvider's outboundTLS config
// injection arm, handleProviderDrain's RetryAfterMs grace arm and cancelled
// error wrapping arm, DrainProviderSession's grace fallback arm, the
// listener-side handleDrain handshake-session fallback arm and shared handler
// error pass-through arm, and the provider_keepalive constructor.
//
// Honest skips: none — every arm is deterministically reachable with the
// existing fixtures (net.Pipe sessions / mockOpsServer / unreachable-address
// dials).

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

// reqBytes serializes a request body (failure fails the test).
func reqBytes(t *testing.T, msg proto.Message) []byte {
	t.Helper()
	data, err := proto.Marshal(msg)
	require.NoError(t, err)
	return data
}

// updateProcessErrOps / schedulePauseErrOps: mockOpsServer is fixed to
// succeed; override the corresponding method to inject errors and drive the
// handler's ops failure pass-through arm.
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

	// unmarshal failure
	_, err := h.handleUpdateProcess(context.Background(), []byte{0xff, 0xff, 0xff})
	require.ErrorContains(t, err, "unmarshal UpdateProcessRequest")

	// ops not wired
	_, err = h.handleUpdateProcess(context.Background(), nil)
	require.ErrorContains(t, err, "ops server not configured")

	// success
	h.SetOpsServer(&mockOpsServer{})
	resp, err := h.handleUpdateProcess(context.Background(), reqBytes(t,
		&opsv1.UpdateProcessRequest{ProcessName: "game", StagedPath: "/tmp/new", ExpectedSha256: "abc"}))
	require.NoError(t, err)
	require.NotNil(t, resp)

	// ops failure pass-through
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

	// success (PushFileResponse.Success=true)
	h.SetOpsServer(&mockOpsServer{})
	resp, err := h.handlePushFile(context.Background(), reqBytes(t, &opsv1.PushFileRequest{}))
	require.NoError(t, err)
	out := &opsv1.PushFileResponse{}
	require.NoError(t, proto.Unmarshal(resp, out))
	require.True(t, out.Success)

	// ops failure pass-through
	h.SetOpsServer(&mockOpsServer{pushFileErr: errors.New("disk full")})
	_, err = h.handlePushFile(context.Background(), reqBytes(t, &opsv1.PushFileRequest{}))
	require.ErrorContains(t, err, "push file")
	require.ErrorContains(t, err, "disk full")
}

func TestOpsGetSupervisorLogArms(t *testing.T) {
	h := NewLocalHandler(agentlocal.NewLocalStore(), t.TempDir(), "agent-ops", nil)

	// empty body skips unmarshal and hits ops-not-wired directly
	_, err := h.handleGetSupervisorLog(context.Background(), nil)
	require.ErrorContains(t, err, "ops server not configured")

	// unmarshal failure arm with a non-empty bad body
	_, err = h.handleGetSupervisorLog(context.Background(), []byte{0xff})
	require.ErrorContains(t, err, "unmarshal GetSupervisorLogRequest")

	// success
	h.SetOpsServer(&mockOpsServer{supLogResp: &opsv1.GetSupervisorLogResponse{Content: []byte("log line")}})
	resp, err := h.handleGetSupervisorLog(context.Background(),
		reqBytes(t, &opsv1.GetSupervisorLogRequest{MaxBytes: 128}))
	require.NoError(t, err)
	out := &opsv1.GetSupervisorLogResponse{}
	require.NoError(t, proto.Unmarshal(resp, out))
	require.Equal(t, "log line", string(out.Content))

	// ops failure pass-through
	h.SetOpsServer(&mockOpsServer{supLogErr: errors.New("log rotated")})
	_, err = h.handleGetSupervisorLog(context.Background(),
		reqBytes(t, &opsv1.GetSupervisorLogRequest{MaxBytes: 128}))
	require.ErrorContains(t, err, "get supervisor log")
}

// handleRequest's dispatch arms for the four ops messages.
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

// With no session store, callProvider falls back to the legacy dial; an empty
// address fails the dial immediately.
func TestCallProviderFallsBackToLocalDial(t *testing.T) {
	h := NewLocalHandler(agentlocal.NewLocalStore(), t.TempDir(), "agent-fallback", nil)
	_, err := h.callProvider(context.Background(), "fn.x", nil, "",
		protocol.MsgInvokeRequest, []byte("ping"))
	require.Error(t, err)
}

// callProvider skips sessions with a nil Conn and sessions whose function does
// not match, then falls back to the legacy address dial (127.0.0.1:1
// connection refused).
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

// Task continuation with no session hit and an empty fallbackAddr → explicit
// failure; it must not dial an empty address.
func TestCallProviderTaskContinuationWithoutSession(t *testing.T) {
	handler, _ := newDrainTestHandler(t) // session store exists but is empty
	_, err := handler.callProvider(context.Background(), "fn.y",
		map[string]string{"taskId": "t1"}, "", protocol.MsgInvokeRequest, []byte("x"))
	require.ErrorContains(t, err, "no provider session for task continuation")
}

// handleInvoke's task continuation path (addr left empty, session routing
// delegated to callProvider): with no session hit the error is wrapped and
// passed through as "invoke local provider: ".
func TestInvokeTaskContinuationWrapsSessionMiss(t *testing.T) {
	handler, _ := newDrainTestHandler(t) // session store exists but is empty
	_, err := handler.Handle(context.Background(), protocol.MsgInvokeRequest, 1,
		reqBytes(t, &sdkv1.InvokeRequest{
			FunctionId: "fn.z",
			Metadata:   map[string]string{"taskId": "t9"},
		}))
	require.ErrorContains(t, err, "invoke local provider")
	require.ErrorContains(t, err, "no provider session for task continuation")
}

// The outboundTLS config injected by SetTLSConfig must reach the dial config:
// a missing certificate file fails during certificate loading in NewClient,
// proving the tlsCfg branch is executed.
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
	// CA read fails before the certificate pair loads — the error passes
	// through from create tls config, proving the tlsCfg branch ran and the
	// certificate paths were injected.
	require.ErrorContains(t, err, "create tls config")
}

// RetryAfterMs>0 overrides the default grace; with no target session, ACK
// immediately.
func TestHandleProviderDrainRetryAfterGrace(t *testing.T) {
	handler, _ := newDrainTestHandler(t)
	resp, err := handler.handleProviderDrain(context.Background(),
		reqBytes(t, &sdkv1.ProviderDrainRequest{SessionId: "gone", RetryAfterMs: 250}))
	require.NoError(t, err)
	require.NotNil(t, resp)
}

// In-flight not yet zero and ctx already cancelled: DrainProviderSession
// returns ctx.Err(), and handleProviderDrain wraps and passes it through.
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

// Grace <= 0 → fall back to the default grace; an empty store has no target,
// so return directly.
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

// Listener-side handleDrain: when the request omits session_id, fall back to
// the session id bound at handshake; with no shared LocalHandler wired, just
// reply.
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

// Listener-side drain reaching the shared LocalHandler: with in-flight not yet
// zero and ctx already cancelled, the error is passed through to the SDK
// connection.
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
