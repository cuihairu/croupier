// ARCH-016 provider drain real traffic removal: DrainProviderSession's
// marking / in-flight waiting / grace expiry / undrain / drain-all;
// callProvider skipping draining sessions, explicit rejection when all
// candidates are draining, task-continuation pass-through, in-flight counting
// around calls; handleProviderDrain's reason-sentinel undrain and empty
// session_id node-level drain-all.
package agent

import (
	"context"
	"net"
	"testing"
	"time"

	transportcore "github.com/cuihairu/croupier/core/transport"
	tcptr "github.com/cuihairu/croupier/core/transport/tcp"
	"github.com/cuihairu/croupier/internal/platform/agentlocal"
	"github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
	"github.com/cuihairu/croupier/pkg/protocol"

	"github.com/cuihairu/croupier/agents/sidecar"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

// newDrainTestHandler builds a handler with an agentlocal store plus a
// provider session store.
func newDrainTestHandler(t *testing.T) (*LocalHandler, *ProviderSessionStore) {
	t.Helper()
	store := agentlocal.NewLocalStore()
	handler := NewLocalHandler(store, t.TempDir(), "agent-drain", nil)
	sessStore := NewProviderSessionStore()
	handler.SetProviderSessionStore(sessStore)
	return handler, sessStore
}

// newEchoSession establishes a usable provider session: the client side acts
// as the session Conn (running a read loop to receive responses), the server
// side echoes. When gate is non-nil, the server closes started before
// processing the request and then waits on gate (used to observe the in-flight
// count); when gate is nil it does not block.
func newEchoSession(t *testing.T, sessionID, serviceID, functionID string, started chan<- struct{}, gate <-chan struct{}) *ProviderSession {
	t.Helper()
	serverConn, clientConn := net.Pipe()
	t.Cleanup(func() { _ = serverConn.Close(); _ = clientConn.Close() })

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	server := tcptr.NewMuxConn(serverConn, &tcptr.Config{RecvTimeout: 5 * time.Second, SendTimeout: 5 * time.Second},
		transportcore.HandlerFunc(func(_ context.Context, msgID uint32, _ uint32, body []byte) ([]byte, error) {
			if started != nil {
				close(started)
			}
			if gate != nil {
				<-gate
			}
			return append([]byte("echo:"), body...), nil
		}))
	serverDone := make(chan struct{})
	go func() { defer close(serverDone); _ = server.Run(ctx) }()
	t.Cleanup(func() { _ = server.Close(); <-serverDone })

	client := tcptr.NewMuxConn(clientConn, &tcptr.Config{RecvTimeout: 5 * time.Second, SendTimeout: 5 * time.Second}, nil)
	clientDone := make(chan struct{})
	go func() { defer close(clientDone); _ = client.Run(ctx) }()
	t.Cleanup(func() { _ = client.Close(); <-clientDone })

	sess := sidecar.NewProviderSession(client)
	sess.SessionID = sessionID
	sess.ServiceID = serviceID
	if functionID != "" {
		sess.Functions = []*sdkv1.ProviderFunctionDescriptor{{Id: functionID}}
	}
	return sess
}

// registerDrainSession places the session in both the session store and the
// agentlocal instance table.
func registerDrainSession(t *testing.T, handler *LocalHandler, store *ProviderSessionStore, sess *ProviderSession, functionID string) {
	t.Helper()
	require.NoError(t, store.Add(sess))
	if handler.store != nil && functionID != "" {
		handler.store.Register(sess.SessionID, sess.ServiceID, "127.0.0.1:1", "1.0.0",
			[]*sdkv1.ProviderFunctionDescriptor{{Id: functionID}}, nil)
	}
}

func agentLocalHasProvider(store *agentlocal.LocalStore, functionID, serviceID string) bool {
	if store == nil {
		return false
	}
	svc := store.ListByService()[functionID]
	if svc == nil {
		return false
	}
	return len(svc[serviceID]) > 0
}

// --- DrainProviderSession semantics ---

func TestDrainProviderSession_MarksDrainingAndRemovesFromAgentLocal(t *testing.T) {
	handler, store := newDrainTestHandler(t)
	sess := newEchoSession(t, "drain-1", "svc-drain", "fn.drain", nil, nil)
	registerDrainSession(t, handler, store, sess, "fn.drain")
	require.True(t, agentLocalHasProvider(handler.store, "fn.drain", "svc-drain"))

	n, err := handler.DrainProviderSession(context.Background(), "drain-1", false, 500*time.Millisecond)
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	assert.True(t, sess.Draining())
	// agentlocal instance table entry removed (legacy address dial no longer
	// routes)
	assert.False(t, agentLocalHasProvider(handler.store, "fn.drain", "svc-drain"))
}

func TestDrainProviderSession_WaitsForInflightZero(t *testing.T) {
	handler, store := newDrainTestHandler(t)
	sess := newEchoSession(t, "drain-2", "svc-2", "fn.2", nil, nil)
	registerDrainSession(t, handler, store, sess, "fn.2")

	sess.AddInflight(1)
	go func() {
		time.Sleep(50 * time.Millisecond)
		sess.AddInflight(-1)
	}()

	start := time.Now()
	n, err := handler.DrainProviderSession(context.Background(), "drain-2", false, 3*time.Second)
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	// in-flight reaches zero after 50ms, so drain should return well before the
	// grace period
	assert.Less(t, time.Since(start), 2*time.Second)
	assert.Equal(t, int64(0), sess.Inflight())
}

func TestDrainProviderSession_TimesOutOnGrace(t *testing.T) {
	handler, store := newDrainTestHandler(t)
	sess := newEchoSession(t, "drain-3", "svc-3", "fn.3", nil, nil)
	registerDrainSession(t, handler, store, sess, "fn.3")

	// in-flight that never reaches zero: drain still returns once the grace
	// period elapses (no blocking, no cancelling of execution)
	sess.AddInflight(1)
	start := time.Now()
	n, err := handler.DrainProviderSession(context.Background(), "drain-3", false, 100*time.Millisecond)
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	assert.GreaterOrEqual(t, time.Since(start), 90*time.Millisecond)
	assert.True(t, sess.Draining())
	assert.Equal(t, int64(1), sess.Inflight()) // execution not cancelled
}

func TestDrainProviderSession_UndrainClearsDraining(t *testing.T) {
	handler, store := newDrainTestHandler(t)
	sess := newEchoSession(t, "drain-4", "svc-4", "fn.4", nil, nil)
	registerDrainSession(t, handler, store, sess, "fn.4")

	_, err := handler.DrainProviderSession(context.Background(), "drain-4", false, 500*time.Millisecond)
	require.NoError(t, err)
	require.True(t, sess.Draining())

	start := time.Now()
	n, err := handler.DrainProviderSession(context.Background(), "drain-4", true, 500*time.Millisecond)
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	assert.False(t, sess.Draining())
	assert.Less(t, time.Since(start), 200*time.Millisecond) // undrain returns immediately
}

func TestDrainProviderSession_DrainAll(t *testing.T) {
	handler, store := newDrainTestHandler(t)
	s1 := newEchoSession(t, "all-1", "svc-a", "fn.a", nil, nil)
	s2 := newEchoSession(t, "all-2", "svc-b", "fn.b", nil, nil)
	registerDrainSession(t, handler, store, s1, "fn.a")
	registerDrainSession(t, handler, store, s2, "fn.b")

	n, err := handler.DrainProviderSession(context.Background(), "", false, 500*time.Millisecond)
	require.NoError(t, err)
	assert.Equal(t, 2, n)
	assert.True(t, s1.Draining())
	assert.True(t, s2.Draining())
}

// --- handleProviderDrain: reason sentinel undrain / drain-all ---

func TestHandleProviderDrain_UndrainSentinel(t *testing.T) {
	handler, store := newDrainTestHandler(t)
	sess := newEchoSession(t, "u-1", "svc-u", "fn.u", nil, nil)
	registerDrainSession(t, handler, store, sess, "fn.u")

	// drain sets the flag first
	drainBody, _ := protoMarshalDrain(t, "u-1", "deploy", 0)
	_, err := handler.handleProviderDrain(context.Background(), drainBody)
	require.NoError(t, err)
	require.True(t, sess.Draining())

	// reason=__undrain__ clears it
	undrainBody, _ := protoMarshalDrain(t, "u-1", protocol.ProviderDrainReasonUndrain, 0)
	resp, err := handler.handleProviderDrain(context.Background(), undrainBody)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.False(t, sess.Draining())
}

func TestHandleProviderDrain_DrainAll(t *testing.T) {
	handler, store := newDrainTestHandler(t)
	s1 := newEchoSession(t, "ha-1", "svc-ha1", "fn.ha", nil, nil)
	s2 := newEchoSession(t, "ha-2", "svc-ha2", "fn.ha", nil, nil)
	registerDrainSession(t, handler, store, s1, "fn.ha")
	registerDrainSession(t, handler, store, s2, "fn.ha")

	body, _ := protoMarshalDrain(t, "", "deploy", 0)
	resp, err := handler.handleProviderDrain(context.Background(), body)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.True(t, s1.Draining())
	assert.True(t, s2.Draining())
}

func protoMarshalDrain(t *testing.T, sessionID, reason string, retryAfterMs uint32) ([]byte, error) {
	t.Helper()
	req := &sdkv1.ProviderDrainRequest{SessionId: sessionID, Reason: reason, RetryAfterMs: retryAfterMs}
	data, err := proto.Marshal(req)
	return data, err
}

// --- callProvider: routing and rejection for draining sessions ---

func TestCallProvider_SkipsDrainingSession(t *testing.T) {
	handler, store := newDrainTestHandler(t)
	// two sessions for the same function: it makes no difference whether the
	// draining one comes first or last — the usable non-draining session is
	// selected.
	draining := newEchoSession(t, "cp-1", "svc-cp1", "fn.cp", nil, nil)
	good := newEchoSession(t, "cp-2", "svc-cp2", "fn.cp", nil, nil)
	registerDrainSession(t, handler, store, draining, "fn.cp")
	registerDrainSession(t, handler, store, good, "fn.cp")
	draining.SetDraining(true)

	resp, err := handler.callProvider(context.Background(), "fn.cp", map[string]string{}, "127.0.0.1:1", protocol.MsgInvokeRequest, []byte("hello"))
	require.NoError(t, err)
	assert.Equal(t, "echo:hello", string(resp))
	assert.False(t, good.Draining())
}

func TestCallProvider_RejectsWhenAllDraining(t *testing.T) {
	handler, store := newDrainTestHandler(t)
	sess := newEchoSession(t, "cp-3", "svc-cp3", "fn.cp3", nil, nil)
	registerDrainSession(t, handler, store, sess, "fn.cp3")
	sess.SetDraining(true)

	// function-match path: the only candidate is draining → explicit rejection
	// (no fall-through to the legacy address dial)
	_, err := handler.callProvider(context.Background(), "fn.cp3", map[string]string{}, "127.0.0.1:1", protocol.MsgInvokeRequest, []byte("{}"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "is draining")

	// service_id targeted path: the target session is draining → same
	// rejection
	_, err = handler.callProvider(context.Background(), "fn.cp3", map[string]string{"serviceId": "svc-cp3"}, "127.0.0.1:1", protocol.MsgInvokeRequest, []byte("{}"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "is draining")
	assert.Contains(t, err.Error(), "svc-cp3")
}

func TestCallProvider_TaskContinuationAllowedOnDraining(t *testing.T) {
	handler, store := newDrainTestHandler(t)
	sess := newEchoSession(t, "cp-4", "svc-cp4", "fn.cp4", nil, nil)
	registerDrainSession(t, handler, store, sess, "fn.cp4")
	sess.SetDraining(true)

	// task continuation (accepted before drain): the draining session is still
	// routed to and the call completes normally.
	resp, err := handler.callProvider(context.Background(), "fn.cp4",
		map[string]string{"taskId": "task-1"}, "127.0.0.1:1", protocol.MsgInvokeRequest, []byte("payload"))
	require.NoError(t, err)
	assert.Equal(t, "echo:payload", string(resp))
}

func TestCallProvider_InflightCountedAroundCall(t *testing.T) {
	handler, store := newDrainTestHandler(t)
	started := make(chan struct{})
	gate := make(chan struct{})
	sess := newEchoSession(t, "cp-5", "svc-cp5", "fn.cp5", started, gate)
	registerDrainSession(t, handler, store, sess, "fn.cp5")

	callDone := make(chan struct{})
	go func() {
		defer close(callDone)
		_, _ = handler.callProvider(context.Background(), "fn.cp5", map[string]string{}, "127.0.0.1:1", protocol.MsgInvokeRequest, []byte("x"))
	}()

	<-started // the server received the request ⇒ callSession has added in-flight
	assert.Equal(t, int64(1), sess.Inflight())

	close(gate)
	<-callDone
	assert.Equal(t, int64(0), sess.Inflight())
}

// --- R30-09: tasks accepted but not yet inside the Provider RPC must not be
// truncated by drain ---

// TestCallProvider_TaskContinuationAfterDrainNoAgentLocal verifies that a
// task continuation (taskId present) can still route to the provider session
// even after DrainProviderSession has removed the agentlocal registration:
// handleInvoke skips pickInstance for continuations and callProvider matches
// the session directly (draining allowed for continuations).
func TestCallProvider_TaskContinuationAfterDrainNoAgentLocal(t *testing.T) {
	handler, store := newDrainTestHandler(t)
	sess := newEchoSession(t, "r309-1", "svc-r309", "fn.r309", nil, nil)
	registerDrainSession(t, handler, store, sess, "fn.r309")

	n, err := handler.DrainProviderSession(context.Background(), "r309-1", false, 500*time.Millisecond)
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	require.False(t, agentLocalHasProvider(handler.store, "fn.r309", "svc-r309"), "agentlocal entry must be removed by drain")

	// Simulates executeTask→handleInvoke: task continuation does not read
	// agentlocal (it would certainly report not-registered), and instead hits
	// the session directly via callProvider (draining allowed).
	req := &sdkv1.InvokeRequest{
		FunctionId: "fn.r309",
		Metadata:   map[string]string{"taskId": "task-r309-1"},
	}
	reqBytes, err := proto.Marshal(req)
	require.NoError(t, err)
	resp, err := handler.handleInvoke(context.Background(), reqBytes)
	require.NoError(t, err)
	invokeResp := &sdkv1.InvokeResponse{}
	require.NoError(t, proto.Unmarshal(resp, invokeResp))
	assert.NotEmpty(t, invokeResp.GetPayload())
}

// TestHandleInvoke_TaskContinuationSkipsPickInstanceAfterDrain verifies that
// handleInvoke does NOT fail with "function not registered" for a task
// continuation when the agentlocal store has been drained but the provider
// session is still present (R30-09 reproduction of the reported failure).
func TestHandleInvoke_TaskContinuationSkipsPickInstanceAfterDrain(t *testing.T) {
	handler, store := newDrainTestHandler(t)
	sess := newEchoSession(t, "r309-2", "svc-r309b", "fn.r309b", nil, nil)
	registerDrainSession(t, handler, store, sess, "fn.r309b")
	_, err := handler.DrainProviderSession(context.Background(), "r309-2", false, 500*time.Millisecond)
	require.NoError(t, err)

	req := &sdkv1.InvokeRequest{
		FunctionId: "fn.r309b",
		Metadata:   map[string]string{"taskId": "task-r309b"},
	}
	data, err := proto.Marshal(req)
	require.NoError(t, err)
	resp, err := handler.handleInvoke(context.Background(), data)
	require.NoError(t, err, "task continuation must not be truncated by drain-removed agentlocal entry")
	require.NotNil(t, resp)
}

// TestDrainProviderSession_WaitsForAcceptedTasks verifies that drain waits
// (until grace expires) when a task has been accepted via StartTask ACK but
// the executor has not yet entered the provider call (Inflight==0, Count>0).
// The accepted task then completes on the draining session rather than being
// truncated by the agentlocal removal.
func TestDrainProviderSession_WaitsForAcceptedTasks(t *testing.T) {
	handler, store := newDrainTestHandler(t)
	sess := newEchoSession(t, "r309-3", "svc-r309c", "fn.r309c", nil, nil)
	registerDrainSession(t, handler, store, sess, "fn.r309c")

	// accept the task but do not execute it right away: the executor blocks to
	// simulate "ACKed, not yet at the provider".
	executorGate := make(chan struct{})
	executorReleased := make(chan struct{})
	handler.tasks = sidecar.NewTaskRunner(func(ctx context.Context, req *sdkv1.InvokeRequest) ([]byte, error) {
		close(executorGate)
		<-executorReleased
		// go through handleInvoke to reach the provider (session still usable),
		// verifying the continuation is not truncated.
		data, _ := proto.Marshal(req)
		resp, err := handler.handleInvoke(ctx, data)
		if err != nil {
			return nil, err
		}
		out := &sdkv1.InvokeResponse{}
		if err := proto.Unmarshal(resp, out); err != nil {
			return nil, err
		}
		return out.GetPayload(), nil
	}, nil, nil)

	startedReq := &sdkv1.InvokeRequest{
		FunctionId: "fn.r309c",
		Metadata:   map[string]string{"taskId": "task-r309c-1"},
	}
	taskID := handler.tasks.Start(startedReq)
	assert.NotEmpty(t, taskID)
	<-executorGate // executor entered but has not reached handleInvoke yet
	// Inflight is still 0 here; drain must not return immediately.

	go func() {
		time.Sleep(80 * time.Millisecond)
		close(executorReleased)
	}()

	start := time.Now()
	n, err := handler.DrainProviderSession(context.Background(), "r309-3", false, 3*time.Second)
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	assert.Greater(t, int64(time.Since(start)), int64(40*time.Millisecond), "drain must wait for the accepted task to finish")
	assert.Equal(t, 0, handler.tasks.Count(), "accepted task must complete before drain returns")
}
