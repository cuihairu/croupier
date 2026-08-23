package croupier

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/cuihairu/croupier/sdks/go/pkg/croupier/protocol"
	sdkv1 "github.com/cuihairu/croupier/sdks/go/pkg/pb/croupier/sdk/v1"
	"google.golang.org/protobuf/proto"
)

// ---------------------------------------------------------------------------
// Fake Agent (speaks the Croupier provider wire protocol)
// ---------------------------------------------------------------------------

const (
	fakeFrameHeader = 4
)

type fakeAgentServer struct {
	t       *testing.T
	ln      net.Listener
	handler func(msgID uint32, reqID uint32, body []byte) (respMsgID uint32, respBody []byte, ok bool)

	mu         sync.Mutex
	conns      map[net.Conn]struct{}
	wg         sync.WaitGroup
	acceptDone chan struct{}
}

// startFakeAgent binds addr ("127.0.0.1:0" picks a free port) and serves the
// provider protocol. handler returns ok=false to drop the request silently.
func startFakeAgent(t *testing.T, addr string, handler func(msgID, reqID uint32, body []byte) (uint32, []byte, bool)) *fakeAgentServer {
	t.Helper()
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatalf("fake agent listen %s: %v", addr, err)
	}
	s := &fakeAgentServer{
		t:          t,
		ln:         ln,
		handler:    handler,
		conns:      map[net.Conn]struct{}{},
		acceptDone: make(chan struct{}),
	}
	s.wg.Add(1)
	go s.acceptLoop()
	t.Cleanup(s.stop)
	return s
}

func (s *fakeAgentServer) addr() string { return s.ln.Addr().String() }

func (s *fakeAgentServer) acceptLoop() {
	defer s.wg.Done()
	defer close(s.acceptDone)
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			return
		}
		s.mu.Lock()
		s.conns[conn] = struct{}{}
		s.mu.Unlock()
		s.wg.Add(1)
		go s.serveConn(conn)
	}
}

func (s *fakeAgentServer) serveConn(conn net.Conn) {
	defer s.wg.Done()
	defer func() {
		s.mu.Lock()
		delete(s.conns, conn)
		s.mu.Unlock()
		_ = conn.Close()
	}()
	for {
		msgID, reqID, body, err := readAgentFrame(conn)
		if err != nil {
			return
		}
		respMsgID, respBody, ok := s.handler(msgID, reqID, body)
		if !ok {
			return // hang up
		}
		if err := writeAgentFrame(conn, respMsgID, reqID, respBody); err != nil {
			return
		}
	}
}

// stop closes the listener and all live connections. Safe to call twice.
//
// The accept loop is fully drained before connections are closed: callers may
// still be dialing (e.g. an invoker reconnect loop), and a connection accepted
// just before the listener closes would otherwise never be closed, deadlocking
// the final WaitGroup wait.
func (s *fakeAgentServer) stop() {
	if s.ln != nil {
		_ = s.ln.Close()
	}
	<-s.acceptDone
	for {
		s.mu.Lock()
		pending := make([]net.Conn, 0, len(s.conns))
		for conn := range s.conns {
			pending = append(pending, conn)
		}
		s.mu.Unlock()
		if len(pending) == 0 {
			break
		}
		for _, conn := range pending {
			_ = conn.Close()
		}
		time.Sleep(2 * time.Millisecond)
	}
	s.wg.Wait()
}

func readAgentFrame(conn net.Conn) (msgID, reqID uint32, body []byte, err error) {
	header := make([]byte, fakeFrameHeader)
	if _, err = io.ReadFull(conn, header); err != nil {
		return 0, 0, nil, err
	}
	size := binary.BigEndian.Uint32(header)
	if size == 0 {
		return 0, 0, nil, io.ErrUnexpectedEOF
	}
	payload := make([]byte, size)
	if _, err = io.ReadFull(conn, payload); err != nil {
		return 0, 0, nil, err
	}
	if len(payload) < protocol.HeaderSize {
		return 0, 0, nil, io.ErrUnexpectedEOF
	}
	msgID = protocol.GetMsgID(payload[1:4])
	reqID = binary.BigEndian.Uint32(payload[4:8])
	body = payload[protocol.HeaderSize:]
	return msgID, reqID, body, nil
}

func writeAgentFrame(conn net.Conn, msgID, reqID uint32, body []byte) error {
	frameBody := protocol.NewMessageBody(msgID, reqID, body)
	frame := make([]byte, fakeFrameHeader+len(frameBody))
	binary.BigEndian.PutUint32(frame[:fakeFrameHeader], uint32(len(frameBody)))
	copy(frame[fakeFrameHeader:], frameBody)
	_, err := conn.Write(frame)
	return err
}

// agentSendRequest sends an inbound request into the provider connection and
// reads the response frame, returning the response body.
func agentSendRequest(conn net.Conn, msgID uint32, body []byte) ([]byte, error) {
	if err := writeAgentFrame(conn, msgID, 1, body); err != nil {
		return nil, err
	}
	respMsgID, _, respBody, err := readAgentFrame(conn)
	if err != nil {
		return nil, err
	}
	if !protocol.IsResponse(respMsgID) {
		return nil, fmt.Errorf("expected response msgID, got %#x", respMsgID)
	}
	return respBody, nil
}

// defaultAgentHandler answers the provider protocol happy path.
func defaultAgentHandler(sessionID string) func(uint32, uint32, []byte) (uint32, []byte, bool) {
	return func(msgID uint32, reqID uint32, body []byte) (uint32, []byte, bool) {
		switch msgID {
		case protocol.MsgProviderConnectRequest:
			resp, _ := proto.Marshal(&sdkv1.ProviderConnectResponse{SessionId: sessionID})
			return protocol.MsgProviderConnectResponse, resp, true
		case protocol.MsgProviderHeartbeatRequest:
			resp, _ := proto.Marshal(&sdkv1.ProviderHeartbeatResponse{})
			return protocol.MsgProviderHeartbeatResponse, resp, true
		default:
			resp, _ := proto.Marshal(&sdkv1.InvokeResponse{Payload: []byte(`{}`)})
			return protocol.MsgInvokeResponse, resp, true
		}
	}
}

func testHandlers() map[string]FunctionHandler {
	return map[string]FunctionHandler{
		"demo.echo": func(ctx context.Context, payload []byte) ([]byte, error) {
			return append([]byte("echo:"), payload...), nil
		},
		"demo.fail": func(ctx context.Context, payload []byte) ([]byte, error) {
			return nil, fmt.Errorf("handler failure")
		},
		"demo.block": func(ctx context.Context, payload []byte) ([]byte, error) {
			<-ctx.Done()
			return nil, ctx.Err()
		},
	}
}

// ---------------------------------------------------------------------------
// TCPManager: connect + register + heartbeat happy path
// ---------------------------------------------------------------------------

func TestTCPManager_ConnectRegisterHeartbeat(t *testing.T) {
	agent := startFakeAgent(t, "127.0.0.1:0", defaultAgentHandler("sess-managed-1"))

	mgr, err := NewTCPManager(ClientConfig{
		AgentAddr:         agent.addr(),
		Insecure:          true,
		HeartbeatInterval: 0, // falls back to default in heartbeatLoop
	}, testHandlers())
	if err != nil {
		t.Fatalf("NewTCPManager: %v", err)
	}
	tcp := mgr.(*TCPManager)

	if err := tcp.Connect(context.Background()); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer tcp.Disconnect()

	sessionID, err := tcp.RegisterWithAgent(context.Background(), "svc", "1.0.0", []ProviderFunctionDescriptor{
		{ID: "demo.echo", Version: "1.0.0"},
	})
	if err != nil {
		t.Fatalf("RegisterWithAgent: %v", err)
	}
	if sessionID != "sess-managed-1" {
		t.Fatalf("sessionID = %q", sessionID)
	}
	if !tcp.IsConnected() {
		t.Fatal("should be connected after register")
	}

	// Heartbeat succeeds against the fake agent.
	if err := tcp.sendHeartbeat(context.Background()); err != nil {
		t.Fatalf("sendHeartbeat: %v", err)
	}
}

func TestTCPManager_RegisterWithAgent_CallFailure(t *testing.T) {
	agent := startFakeAgent(t, "127.0.0.1:0", defaultAgentHandler("sess"))

	mgr, _ := NewTCPManager(ClientConfig{AgentAddr: agent.addr(), Insecure: true}, nil)
	tcp := mgr.(*TCPManager)
	if err := tcp.Connect(context.Background()); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	// Kill the agent so the registration call fails. The context is bounded
	// because a dropped connection leaves pending Call requests waiting until
	// their deadline (see bug note in the final report).
	agent.stop()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := tcp.RegisterWithAgent(ctx, "svc", "1.0.0", nil)
	if err == nil {
		t.Fatal("expected registration call failure")
	}
}

func TestTCPManager_RegisterWithAgent_BadResponses(t *testing.T) {
	cases := []struct {
		name    string
		handler func(uint32, uint32, []byte) (uint32, []byte, bool)
		wantErr string
	}{
		{
			name: "wrong message id",
			handler: func(_, _ uint32, _ []byte) (uint32, []byte, bool) {
				return protocol.MsgHeartbeatResponse, []byte{}, true
			},
			wantErr: "unexpected response message",
		},
		{
			name: "invalid proto body",
			handler: func(_ uint32, _ uint32, _ []byte) (uint32, []byte, bool) {
				return protocol.MsgProviderConnectResponse, []byte{0xff, 0xff, 0xff}, true
			},
			wantErr: "unmarshal response",
		},
		{
			name: "empty session id",
			handler: func(_, _ uint32, _ []byte) (uint32, []byte, bool) {
				resp, _ := proto.Marshal(&sdkv1.ProviderConnectResponse{})
				return protocol.MsgProviderConnectResponse, resp, true
			},
			wantErr: "no session ID",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			agent := startFakeAgent(t, "127.0.0.1:0", tc.handler)
			mgr, _ := NewTCPManager(ClientConfig{AgentAddr: agent.addr(), Insecure: true}, nil)
			tcp := mgr.(*TCPManager)
			if err := tcp.Connect(context.Background()); err != nil {
				t.Fatalf("Connect: %v", err)
			}
			_, err := tcp.RegisterWithAgent(context.Background(), "svc", "1.0.0", nil)
			if err == nil || !containsStr(err.Error(), tc.wantErr) {
				t.Fatalf("expected error containing %q, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestTCPManager_SendHeartbeat_NotConnected(t *testing.T) {
	mgr, _ := NewTCPManager(ClientConfig{AgentAddr: "127.0.0.1:1", Insecure: true}, nil)
	tcp := mgr.(*TCPManager)
	if err := tcp.sendHeartbeat(context.Background()); err != nil {
		t.Fatalf("sendHeartbeat while disconnected = %v, want nil", err)
	}
}

// ---------------------------------------------------------------------------
// TCPManager: heartbeat failure triggers disconnect callback
// ---------------------------------------------------------------------------

func TestTCPManager_HeartbeatFailureTriggersDisconnect(t *testing.T) {
	agent := startFakeAgent(t, "127.0.0.1:0", defaultAgentHandler("sess-hb"))

	mgr, _ := NewTCPManager(ClientConfig{
		AgentAddr:         agent.addr(),
		Insecure:          true,
		HeartbeatInterval: 1,
	}, testHandlers())
	tcp := mgr.(*TCPManager)

	if err := tcp.Connect(context.Background()); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	sessionID, err := tcp.RegisterWithAgent(context.Background(), "svc", "1.0.0", nil)
	if err != nil {
		t.Fatalf("RegisterWithAgent: %v", err)
	}
	if sessionID == "" {
		t.Fatal("sessionID should not be empty")
	}

	disconnected := make(chan struct{})
	tcp.SetOnDisconnect(func() { close(disconnected) })

	// Simulate a dead transport: closing the client makes every pending or
	// future Call fail immediately ("client is closing"), so two heartbeat
	// ticks fail fast and handleDisconnect fires.
	if err := tcp.client.Close(); err != nil {
		t.Fatalf("close client: %v", err)
	}

	select {
	case <-disconnected:
	case <-time.After(6 * time.Second):
		t.Fatal("disconnect callback not triggered by heartbeat failures")
	}
	if tcp.IsConnected() {
		t.Fatal("manager should report disconnected")
	}
}

func TestTCPManager_HandleDisconnect_Idempotent(t *testing.T) {
	mgr, _ := NewTCPManager(ClientConfig{AgentAddr: "127.0.0.1:1", Insecure: true}, nil)
	tcp := mgr.(*TCPManager)

	calls := 0
	tcp.SetOnDisconnect(func() { calls++ })

	// handleDisconnect on a disconnected manager is a no-op.
	tcp.handleDisconnect()
	if calls != 0 {
		t.Fatalf("onDisconnect called %d times while not connected", calls)
	}
}

// ---------------------------------------------------------------------------
// TCPManager: Reconnect
// ---------------------------------------------------------------------------

func TestTCPManager_Reconnect_Success(t *testing.T) {
	agent := startFakeAgent(t, "127.0.0.1:0", defaultAgentHandler("sess-reconnect-1"))
	addr := agent.addr()
	_ = addr

	mgr, _ := NewTCPManager(ClientConfig{AgentAddr: addr, Insecure: true}, testHandlers())
	tcp := mgr.(*TCPManager)

	if err := tcp.Connect(context.Background()); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	if _, err := tcp.RegisterWithAgent(context.Background(), "svc", "1.0.0", []ProviderFunctionDescriptor{{ID: "demo.echo"}}); err != nil {
		t.Fatalf("RegisterWithAgent: %v", err)
	}

	// Simulate connection loss, then bring the agent back on the same port.
	agent.stop()
	restarted := startFakeAgent(t, addr, defaultAgentHandler("sess-reconnect-2"))
	_ = restarted

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := tcp.Reconnect(ctx); err != nil {
		t.Fatalf("Reconnect: %v", err)
	}
	defer tcp.Disconnect()
	if !tcp.IsConnected() {
		t.Fatal("should be connected after Reconnect")
	}

	// Second Reconnect while connected returns nil immediately.
	if err := tcp.Reconnect(context.Background()); err != nil {
		t.Fatalf("Reconnect while connected = %v, want nil", err)
	}
}

func TestTCPManager_Reconnect_DialFailure(t *testing.T) {
	mgr, _ := NewTCPManager(ClientConfig{AgentAddr: "127.0.0.1:1", Insecure: true}, nil)
	tcp := mgr.(*TCPManager)
	if err := tcp.Reconnect(context.Background()); err == nil {
		t.Fatal("expected reconnect dial failure")
	}
}

func TestTCPManager_Reconnect_BadResponses(t *testing.T) {
	cases := []struct {
		name    string
		handler func(uint32, uint32, []byte) (uint32, []byte, bool)
		wantErr string
	}{
		{
			name: "call failure",
			handler: func(_, _ uint32, _ []byte) (uint32, []byte, bool) {
				return 0, nil, false // hang up
			},
			wantErr: "reconnect register",
		},
		{
			name: "wrong message id",
			handler: func(_, _ uint32, _ []byte) (uint32, []byte, bool) {
				return protocol.MsgHeartbeatResponse, nil, true
			},
			wantErr: "unexpected response",
		},
		{
			name: "invalid proto",
			handler: func(_, _ uint32, _ []byte) (uint32, []byte, bool) {
				return protocol.MsgProviderConnectResponse, []byte{0xfe, 0xfe}, true
			},
			wantErr: "unmarshal response",
		},
		{
			name: "empty session",
			handler: func(_, _ uint32, _ []byte) (uint32, []byte, bool) {
				resp, _ := proto.Marshal(&sdkv1.ProviderConnectResponse{})
				return protocol.MsgProviderConnectResponse, resp, true
			},
			wantErr: "no session ID",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			agent := startFakeAgent(t, "127.0.0.1:0", tc.handler)
			mgr, _ := NewTCPManager(ClientConfig{AgentAddr: agent.addr(), Insecure: true}, nil)
			tcp := mgr.(*TCPManager)
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			err := tcp.Reconnect(ctx)
			if err == nil || !containsStr(err.Error(), tc.wantErr) {
				t.Fatalf("expected error containing %q, got %v", tc.wantErr, err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// tcpRPCHandler: invoke dispatch via inbound frames
// ---------------------------------------------------------------------------

func TestTCPManager_InboundInvokeDispatch(t *testing.T) {
	// One-shot agent that owns its connection exclusively: it answers the
	// registration, then pushes an invoke request and reads the provider's
	// response without competing readers.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() { _ = ln.Close() }()

	type result struct {
		payload string
		err     error
	}
	done := make(chan result, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			done <- result{err: err}
			return
		}
		defer func() { _ = conn.Close() }()

		// Answer the provider registration.
		msgID, reqID, _, err := readAgentFrame(conn)
		if err != nil {
			done <- result{err: err}
			return
		}
		if msgID != protocol.MsgProviderConnectRequest {
			done <- result{err: fmt.Errorf("expected connect request, got %#x", msgID)}
			return
		}
		resp, _ := proto.Marshal(&sdkv1.ProviderConnectResponse{SessionId: "sess-invoke"})
		if err := writeAgentFrame(conn, protocol.MsgProviderConnectResponse, reqID, resp); err != nil {
			done <- result{err: err}
			return
		}

		// Push a successful invoke request.
		req, _ := proto.Marshal(&sdkv1.InvokeRequest{FunctionId: "demo.echo", Payload: []byte(`"hi"`)})
		respBody, err := agentSendRequest(conn, protocol.MsgInvokeRequest, req)
		if err != nil {
			done <- result{err: err}
			return
		}
		invokeResp := &sdkv1.InvokeResponse{}
		if err := proto.Unmarshal(respBody, invokeResp); err != nil {
			done <- result{err: err}
			return
		}
		first := string(invokeResp.Payload)

		// Push an invoke for an unregistered function; the SDK answers with
		// an error-carrying InvokeResponse.
		missing, _ := proto.Marshal(&sdkv1.InvokeRequest{FunctionId: "nope"})
		respBody, err = agentSendRequest(conn, protocol.MsgInvokeRequest, missing)
		if err != nil {
			done <- result{err: err}
			return
		}
		missingResp := &sdkv1.InvokeResponse{}
		if err := proto.Unmarshal(respBody, missingResp); err != nil {
			done <- result{err: err}
			return
		}
		done <- result{payload: first + "|" + string(missingResp.Payload)}
	}()

	mgr, _ := NewTCPManager(ClientConfig{AgentAddr: ln.Addr().String(), Insecure: true}, testHandlers())
	tcp := mgr.(*TCPManager)
	if err := tcp.Connect(context.Background()); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	if _, err := tcp.RegisterWithAgent(context.Background(), "svc", "1.0.0", nil); err != nil {
		t.Fatalf("RegisterWithAgent: %v", err)
	}
	defer tcp.Disconnect()

	select {
	case res := <-done:
		if res.err != nil {
			t.Fatalf("agent-side flow error: %v", res.err)
		}
		if res.payload != `echo:"hi"|{"error":"function not found: nope"}` {
			t.Fatalf("unexpected payloads: %q", res.payload)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("inbound invoke dispatch timed out")
	}
}

// ---------------------------------------------------------------------------
// tcpRPCHandler direct unit tests
// ---------------------------------------------------------------------------

func TestTCPRPCHandler_InvokeBranches(t *testing.T) {
	mgr, _ := NewTCPManager(ClientConfig{}, testHandlers())
	tcp := mgr.(*TCPManager)
	h := tcp.rpcHandler
	ctx := context.Background()

	if _, err := h.invoke(ctx, protocol.MsgInvokeRequest, 1, []byte{0xff}); err == nil {
		t.Fatal("expected unmarshal error")
	}

	if _, err := h.invoke(ctx, protocol.MsgInvokeRequest, 1, mustMarshalInvoke(t, "missing.fn", nil)); err == nil {
		t.Fatal("expected function-not-found error")
	}

	body, err := h.invoke(ctx, protocol.MsgInvokeRequest, 1, mustMarshalInvoke(t, "demo.fail", nil))
	if err == nil {
		t.Fatal("expected handler error")
	}
	_ = body

	body, err = h.invoke(ctx, protocol.MsgInvokeRequest, 1, mustMarshalInvoke(t, "demo.echo", []byte("x")))
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}
	resp := &sdkv1.InvokeResponse{}
	if err := proto.Unmarshal(body, resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if string(resp.Payload) != "echo:x" {
		t.Fatalf("payload = %q", resp.Payload)
	}
}

func mustMarshalInvoke(t *testing.T, functionID string, payload []byte) []byte {
	t.Helper()
	req := &sdkv1.InvokeRequest{FunctionId: functionID, Payload: payload}
	body, err := proto.Marshal(req)
	if err != nil {
		t.Fatalf("marshal InvokeRequest: %v", err)
	}
	return body
}

func TestTCPRPCHandler_TaskLifecycle(t *testing.T) {
	mgr, _ := NewTCPManager(ClientConfig{}, testHandlers())
	tcp := mgr.(*TCPManager)
	h := tcp.rpcHandler
	ctx := context.Background()

	if _, err := h.startTask(ctx, protocol.MsgStartTaskRequest, 1, []byte{0xff}); err == nil {
		t.Fatal("expected startTask unmarshal error")
	}
	if _, err := h.startTask(ctx, protocol.MsgStartTaskRequest, 1, mustMarshalInvoke(t, "missing.fn", nil)); err == nil {
		t.Fatal("expected startTask function-not-found error")
	}

	body, err := h.startTask(ctx, protocol.MsgStartTaskRequest, 1, mustMarshalInvoke(t, "demo.echo", []byte("job")))
	if err != nil {
		t.Fatalf("startTask: %v", err)
	}
	startResp := &sdkv1.StartTaskResponse{}
	if err := proto.Unmarshal(body, startResp); err != nil {
		t.Fatalf("unmarshal StartTaskResponse: %v", err)
	}
	taskID := startResp.TaskId
	if taskID == "" {
		t.Fatal("task ID should not be empty")
	}

	// Wait for the async handler to finish (fields are read under the task
	// mutex because the worker goroutine writes them).
	deadline := time.Now().Add(2 * time.Second)
	succeeded := false
	for time.Now().Before(deadline) {
		tcp.tasksMutex.RLock()
		task := tcp.tasks[taskID]
		succeeded = task != nil && task.Status.String() == "TASK_STATUS_SUCCEEDED"
		tcp.tasksMutex.RUnlock()
		if succeeded {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !succeeded {
		t.Fatal("task did not succeed in time")
	}

	// streamTask returns a progress event.
	streamReq, _ := proto.Marshal(&sdkv1.TaskStreamRequest{TaskId: taskID})
	body, err = h.streamTask(ctx, protocol.MsgStreamTaskRequest, 1, streamReq)
	if err != nil {
		t.Fatalf("streamTask: %v", err)
	}
	resp := &sdkv1.InvokeResponse{}
	if err := proto.Unmarshal(body, resp); err != nil {
		t.Fatalf("unmarshal stream response: %v", err)
	}
	event := &sdkv1.TaskEvent{}
	if err := proto.Unmarshal(resp.Payload, event); err != nil {
		t.Fatalf("unmarshal TaskEvent: %v", err)
	}
	if event.TaskId != taskID {
		t.Fatalf("event task = %q", event.TaskId)
	}

	if _, err := h.streamTask(ctx, protocol.MsgStreamTaskRequest, 1, []byte{0xff}); err == nil {
		t.Fatal("expected streamTask unmarshal error")
	}
	unknownStream, _ := proto.Marshal(&sdkv1.TaskStreamRequest{TaskId: "ghost"})
	if _, err := h.streamTask(ctx, protocol.MsgStreamTaskRequest, 1, unknownStream); err == nil {
		t.Fatal("expected task-not-found error")
	}

	// cancelTask marks the task cancelled.
	cancelReq, _ := proto.Marshal(&sdkv1.CancelTaskRequest{TaskId: taskID})
	body, err = h.cancelTask(ctx, protocol.MsgCancelTaskRequest, 1, cancelReq)
	if err != nil {
		t.Fatalf("cancelTask: %v", err)
	}
	cancelResp := &sdkv1.InvokeResponse{}
	if err := proto.Unmarshal(body, cancelResp); err != nil {
		t.Fatalf("unmarshal cancel response: %v", err)
	}
	if !containsStr(string(cancelResp.Payload), `"cancelled":true`) {
		t.Fatalf("payload = %q", cancelResp.Payload)
	}

	if _, err := h.cancelTask(ctx, protocol.MsgCancelTaskRequest, 1, []byte{0xff}); err == nil {
		t.Fatal("expected cancelTask unmarshal error")
	}
}

func TestTCPRPCHandler_StartTaskHandlerError(t *testing.T) {
	mgr, _ := NewTCPManager(ClientConfig{}, testHandlers())
	tcp := mgr.(*TCPManager)
	h := tcp.rpcHandler
	ctx := context.Background()

	body, err := h.startTask(ctx, protocol.MsgStartTaskRequest, 1, mustMarshalInvoke(t, "demo.fail", nil))
	if err != nil {
		t.Fatalf("startTask: %v", err)
	}
	startResp := &sdkv1.StartTaskResponse{}
	if err := proto.Unmarshal(body, startResp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		failed := false
		tcp.tasksMutex.RLock()
		task := tcp.tasks[startResp.TaskId]
		failed = task != nil && task.Status.String() == "TASK_STATUS_FAILED" && task.Error != ""
		tcp.tasksMutex.RUnlock()
		if failed {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("task should record the handler failure")
}

// ---------------------------------------------------------------------------
// client: Serve + reconnection flows
// ---------------------------------------------------------------------------

func newTestClientConfig(agentAddr string) *ClientConfig {
	return &ClientConfig{
		AgentAddr:         agentAddr,
		Insecure:          true,
		HeartbeatInterval: 1,
		TimeoutSeconds:    5,
		ServiceID:         "test-service",
		ServiceVersion:    "1.0.0",
		Reconnect: &ReconnectConfig{
			Enabled:           true,
			InitialDelayMs:    50,
			MaxDelayMs:        200,
			BackoffMultiplier: 1.5,
			JitterFactor:      0,
		},
		DisableLogging: true,
	}
}

func TestClient_Serve_StopReturnsNil(t *testing.T) {
	agent := startFakeAgent(t, "127.0.0.1:0", defaultAgentHandler("sess-serve"))
	c := NewClient(newTestClientConfig(agent.addr()))
	if err := c.RegisterFunction(FunctionDescriptor{ID: "demo.echo", Version: "1.0.0"}, testHandlers()["demo.echo"]); err != nil {
		t.Fatalf("RegisterFunction: %v", err)
	}
	// Connect up front so Serve's fast path skips Connect; this keeps Stop()
	// from racing with the manager write in Connect (recorded product bug).
	if err := c.Connect(context.Background()); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	serveErr := make(chan error, 1)
	go func() { serveErr <- c.Serve(context.Background()) }()
	time.Sleep(200 * time.Millisecond)

	if err := c.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	select {
	case err := <-serveErr:
		if err != nil {
			t.Fatalf("Serve after Stop = %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Serve did not return after Stop")
	}
}

func TestClient_Serve_ContextCancelReturnsNil(t *testing.T) {
	agent := startFakeAgent(t, "127.0.0.1:0", defaultAgentHandler("sess-ctx"))
	c := NewClient(newTestClientConfig(agent.addr()))
	_ = c.RegisterFunction(FunctionDescriptor{ID: "demo.echo", Version: "1.0.0"}, testHandlers()["demo.echo"])
	if err := c.Connect(context.Background()); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	serveErr := make(chan error, 1)
	go func() { serveErr <- c.Serve(ctx) }()
	time.Sleep(200 * time.Millisecond)
	cancel()

	select {
	case err := <-serveErr:
		if err != nil {
			t.Fatalf("Serve after cancel = %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Serve did not return after context cancel")
	}
	// Stop only after Serve returned (channel receive gives the
	// happens-before edge over the Serve goroutine's manager writes).
	_ = c.Stop()
}

func TestClient_Serve_ConnectionFailed(t *testing.T) {
	// No agent on this port.
	c := NewClient(newTestClientConfig("127.0.0.1:1"))
	err := c.Serve(context.Background())
	if err == nil {
		t.Fatal("expected Serve to fail without an agent")
	}
}

func TestClient_Serve_ReconnectsAfterAgentRestart(t *testing.T) {
	agent := startFakeAgent(t, "127.0.0.1:0", defaultAgentHandler("sess-restart-1"))
	addr := agent.addr()

	c := NewClient(newTestClientConfig(addr))
	_ = c.RegisterFunction(FunctionDescriptor{ID: "demo.echo", Version: "1.0.0"}, testHandlers()["demo.echo"])

	serveCtx, serveCtxCancel := context.WithCancel(context.Background())
	serveErr := make(chan error, 1)
	go func() { serveErr <- c.Serve(serveCtx) }()
	time.Sleep(300 * time.Millisecond)

	// Kill the agent; the heartbeat loop notices within ~2s.
	agent.stop()
	// Restart the agent on the same port before the client gives up.
	go func() {
		time.Sleep(300 * time.Millisecond)
		_ = agent
		agent2 := startFakeAgent(t, addr, defaultAgentHandler("sess-restart-2"))
		_ = agent2
	}()

	// Give the reconnect a window to complete, then end via context cancel
	// and wait for Serve to return before touching Stop (avoids racing the
	// reconnect path's manager writes).
	time.Sleep(4 * time.Second)
	serveCtxCancel()
	select {
	case err := <-serveErr:
		if err != nil {
			t.Fatalf("Serve returned with error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Serve did not return after context cancel")
	}
	_ = c.Stop()
}

func TestClient_Serve_ReconnectDisabledFailsFast(t *testing.T) {
	agent := startFakeAgent(t, "127.0.0.1:0", defaultAgentHandler("sess-noreconnect"))
	c := NewClient(newTestClientConfig(agent.addr()))
	c.(*client).config.Reconnect.Enabled = false
	_ = c.RegisterFunction(FunctionDescriptor{ID: "demo.echo", Version: "1.0.0"}, testHandlers()["demo.echo"])

	serveErr := make(chan error, 1)
	go func() { serveErr <- c.Serve(context.Background()) }()
	time.Sleep(300 * time.Millisecond)

	agent.stop()
	select {
	case err := <-serveErr:
		if err == nil {
			t.Fatal("expected Serve error when reconnection is disabled")
		}
		if !containsStr(err.Error(), "reconnection is disabled") {
			t.Fatalf("unexpected error: %v", err)
		}
	case <-time.After(8 * time.Second):
		t.Fatal("Serve did not fail after disconnect")
	}
}

func TestClient_Serve_MaxAttemptsExceeded(t *testing.T) {
	agent := startFakeAgent(t, "127.0.0.1:0", defaultAgentHandler("sess-max"))
	cfg := newTestClientConfig(agent.addr())
	cfg.Reconnect.MaxAttempts = 1
	c := NewClient(cfg)
	_ = c.RegisterFunction(FunctionDescriptor{ID: "demo.echo", Version: "1.0.0"}, testHandlers()["demo.echo"])

	serveErr := make(chan error, 1)
	go func() { serveErr <- c.Serve(context.Background()) }()
	time.Sleep(300 * time.Millisecond)

	agent.stop()
	select {
	case err := <-serveErr:
		if err == nil || !containsStr(err.Error(), "max reconnection attempts") {
			t.Fatalf("unexpected error: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Serve did not fail after max attempts")
	}
}

// ---------------------------------------------------------------------------
// client helpers
// ---------------------------------------------------------------------------

func TestNextBackoffDelay(t *testing.T) {
	c := &client{}
	rc := &ReconnectConfig{BackoffMultiplier: 2.0, JitterFactor: 0}

	next := c.nextBackoffDelay(100*time.Millisecond, time.Second, rc)
	if next != 200*time.Millisecond {
		t.Fatalf("next = %v, want 200ms", next)
	}
	// Cap at max.
	next = c.nextBackoffDelay(time.Second, time.Second, rc)
	if next != time.Second {
		t.Fatalf("next = %v, want 1s (cap)", next)
	}
	// Jitter keeps the value in a sane range.
	rc.JitterFactor = 0.5
	next = c.nextBackoffDelay(100*time.Millisecond, time.Second, rc)
	if next < 0 || next > time.Second {
		t.Fatalf("jittered next out of range: %v", next)
	}
	// Minimum delay floor.
	rc2 := &ReconnectConfig{BackoffMultiplier: 0.001, JitterFactor: 0}
	next = c.nextBackoffDelay(time.Millisecond, time.Second, rc2)
	if next < time.Millisecond {
		t.Fatalf("next = %v, want >= 1ms", next)
	}
}

func TestFirstNonEmptySlice(t *testing.T) {
	fallback := []string{"a"}
	if got := firstNonEmptySlice([]string{"x"}, fallback); got[0] != "x" {
		t.Fatalf("got %v", got)
	}
	if got := firstNonEmptySlice(nil, fallback); got[0] != "a" {
		t.Fatalf("got %v", got)
	}
	if got := firstNonEmptySlice([]string{}, fallback); len(got) != 1 {
		t.Fatalf("got %v", got)
	}
}

func TestClient_RegisterFunctionValidation(t *testing.T) {
	c := NewClient(&ClientConfig{DisableLogging: true})
	if err := c.RegisterFunction(FunctionDescriptor{}, nil); err == nil {
		t.Fatal("expected empty-ID error")
	}
	// Version defaults to 1.0.0.
	if err := c.RegisterFunction(FunctionDescriptor{ID: "f"}, testHandlers()["demo.echo"]); err != nil {
		t.Fatalf("RegisterFunction: %v", err)
	}
	// Close and then register fails.
	if err := c.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := c.RegisterFunction(FunctionDescriptor{ID: "f2"}, testHandlers()["demo.echo"]); err == nil {
		t.Fatal("expected closed-client error")
	}
}

func TestClient_ReconnectStoppedMidway(t *testing.T) {
	agent := startFakeAgent(t, "127.0.0.1:0", defaultAgentHandler("sess"))
	c := NewClient(newTestClientConfig(agent.addr()))
	cl := c.(*client)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := cl.reconnectWithBackoff(ctx); err == nil {
		t.Fatal("expected context canceled error")
	}
}

// ---------------------------------------------------------------------------
// loggers
// ---------------------------------------------------------------------------

func TestNoOpLogger_Methods(t *testing.T) {
	l := &NoOpLogger{}
	l.Debugf("x %s", "y")
	l.Infof("x")
	l.Warnf("x")
	l.Errorf("x")
}

func TestDefaultLogger_AllLevels(t *testing.T) {
	l := NewDefaultLogger(true, nil)
	l.Debugf("debug %d", 1)
	l.Infof("info")
	l.Warnf("warn")
	l.Errorf("error")

	l2 := NewDefaultLogger(false, nil)
	l2.Debugf("hidden")
}

// ---------------------------------------------------------------------------
// config validation gaps
// ---------------------------------------------------------------------------

func TestValidateProviderFunctionDescriptor_Branches(t *testing.T) {
	if err := ValidateProviderFunctionDescriptor(&ProviderFunctionDescriptor{}); err == nil {
		t.Fatal("expected ID required error")
	}
	if err := ValidateProviderFunctionDescriptor(&ProviderFunctionDescriptor{ID: "f"}); err == nil {
		t.Fatal("expected version required error")
	}
	bad := &ProviderFunctionDescriptor{ID: "f", Version: "1.0.0", InputSchema: "not json"}
	if err := ValidateProviderFunctionDescriptor(bad); err == nil {
		t.Fatal("expected invalid input schema error")
	}
	badOut := &ProviderFunctionDescriptor{ID: "f", Version: "1.0.0", OutputSchema: "{oops"}
	if err := ValidateProviderFunctionDescriptor(badOut); err == nil {
		t.Fatal("expected invalid output schema error")
	}
	good := &ProviderFunctionDescriptor{ID: "f", Version: "1.0.0", InputSchema: `{"type":"object"}`}
	if err := ValidateProviderFunctionDescriptor(good); err != nil {
		t.Fatalf("valid descriptor rejected: %v", err)
	}
}

func TestIsValidJSONSchema(t *testing.T) {
	if IsValidJSONSchema("nope") {
		t.Fatal("invalid JSON should not be a schema")
	}
	if !IsValidJSONSchema(`{"$schema":"http://json-schema.org/draft-07/schema#"}`) {
		t.Fatal("$schema key should validate")
	}
	if !IsValidJSONSchema(`{"type":"object"}`) {
		t.Fatal("type key should validate")
	}
	if !IsValidJSONSchema(`{"properties":{}}`) {
		t.Fatal("any JSON object should validate")
	}
}

func TestValidateRetryConfig_Branches(t *testing.T) {
	if err := ValidateRetryConfig(nil); err != nil {
		t.Fatalf("nil retry config = %v", err)
	}
	if err := ValidateRetryConfig(&RetryConfig{BackoffMultiplier: 1.0}); err != nil {
		t.Fatalf("minimal config = %v", err)
	}
	cases := []struct {
		cfg  *RetryConfig
		want string
	}{
		{&RetryConfig{InitialDelayMs: -1}, "initial_delay_ms"},
		{&RetryConfig{MaxDelayMs: -1}, "max_delay_ms"},
		{&RetryConfig{BackoffMultiplier: 0.5}, "backoff_multiplier"},
		{&RetryConfig{BackoffMultiplier: 1.0, JitterFactor: -1}, "jitter_factor"},
		{&RetryConfig{BackoffMultiplier: 1.0, JitterFactor: 2}, "jitter_factor"},
		{&RetryConfig{Enabled: true, BackoffMultiplier: 1.0}, "max_attempts"},
		{&RetryConfig{Enabled: true, BackoffMultiplier: 1.0, MaxAttempts: 11}, "max_attempts"},
	}
	for _, tc := range cases {
		if err := ValidateRetryConfig(tc.cfg); err == nil || !containsStr(err.Error(), tc.want) {
			t.Fatalf("expected %q error for %+v, got %v", tc.want, tc.cfg, err)
		}
	}
}

func TestCalculateRetryDelay(t *testing.T) {
	if d := CalculateRetryDelay(nil, 1); d != 0 {
		t.Fatalf("nil config delay = %v", d)
	}
	cfg := &RetryConfig{InitialDelayMs: 100, MaxDelayMs: 1000}
	if d := CalculateRetryDelay(cfg, 0); d != 100*time.Millisecond {
		t.Fatalf("delay = %v", d)
	}
	// Attempt 10 would overflow the max; capped at 1s.
	if d := CalculateRetryDelay(cfg, 20); d > 1000*time.Millisecond {
		t.Fatalf("delay = %v, want cap 1s", d)
	}
	jittered := &RetryConfig{InitialDelayMs: 100, MaxDelayMs: 1000, JitterFactor: 0.5}
	if d := CalculateRetryDelay(jittered, 2); d < 0 || d > 1000*time.Millisecond {
		t.Fatalf("jittered delay out of range: %v", d)
	}
}

func TestMergeWithDefaults(t *testing.T) {
	if got := MergeWithDefaults(nil); got == nil {
		t.Fatal("nil partial must return defaults")
	}

	partial := &ClientConfig{
		ServiceID:      "svc-x",
		AgentAddr:      "127.0.0.1:19999",
		GameID:         "game",
		Env:            "prod",
		TimeoutSeconds: 9,
		Insecure:       true,
		AuthToken:      "tok",
		LogLevel:       "debug",
		DisableLogging: true,
		Headers:        map[string]string{"X-Custom": "1"},
	}
	merged := MergeWithDefaults(partial)
	if merged.ServiceID != "svc-x" || merged.AgentAddr != "127.0.0.1:19999" {
		t.Fatalf("merged = %+v", merged)
	}
	if !merged.Insecure || !merged.DisableLogging {
		t.Fatal("flags not merged")
	}
	if merged.Headers["X-Custom"] != "1" {
		t.Fatalf("headers not merged: %v", merged.Headers)
	}
	if merged.TimeoutSeconds != 9 {
		t.Fatalf("timeout = %d", merged.TimeoutSeconds)
	}
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && (len(sub) == 0 || indexOfStr(s, sub) >= 0)
}

func indexOfStr(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
