package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/cuihairu/croupier/sdks/go/pkg/croupier"
	"github.com/cuihairu/croupier/sdks/go/pkg/croupier/protocol"
	sdkv1 "github.com/cuihairu/croupier/sdks/go/pkg/pb/croupier/sdk/v1"
	"google.golang.org/protobuf/proto"
)

// ---------------------------------------------------------------------------
// Fake agent（本地回环上的最小 Provider 协议对端）
// ---------------------------------------------------------------------------

type compFakeAgent struct {
	ln         net.Listener
	registered chan struct{}
	once       sync.Once
}

func startCompFakeAgent(t *testing.T) *compFakeAgent {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("fake agent listen: %v", err)
	}
	a := &compFakeAgent{ln: ln, registered: make(chan struct{})}
	go a.acceptLoop()
	t.Cleanup(func() { _ = ln.Close() })
	return a
}

func (a *compFakeAgent) addr() string { return a.ln.Addr().String() }

func (a *compFakeAgent) acceptLoop() {
	for {
		conn, err := a.ln.Accept()
		if err != nil {
			return
		}
		go a.serveConn(conn)
	}
}

func (a *compFakeAgent) serveConn(conn net.Conn) {
	defer func() { _ = conn.Close() }()
	for {
		msgID, reqID, _, err := compReadFrame(conn)
		if err != nil {
			return
		}
		var respMsgID uint32
		var respBody []byte
		switch msgID {
		case protocol.MsgProviderConnectRequest:
			respMsgID = protocol.MsgProviderConnectResponse
			respBody, _ = proto.Marshal(&sdkv1.ProviderConnectResponse{SessionId: "sess-comp-main"})
			a.once.Do(func() { close(a.registered) })
		case protocol.MsgProviderHeartbeatRequest:
			respMsgID = protocol.MsgProviderHeartbeatResponse
			respBody, _ = proto.Marshal(&sdkv1.ProviderHeartbeatResponse{})
		default:
			respMsgID = protocol.MsgInvokeResponse
			respBody, _ = proto.Marshal(&sdkv1.InvokeResponse{Payload: []byte(`{}`)})
		}
		if err := compWriteFrame(conn, respMsgID, reqID, respBody); err != nil {
			return
		}
	}
}

func compReadFrame(conn net.Conn) (msgID, reqID uint32, body []byte, err error) {
	header := make([]byte, 4)
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

func compWriteFrame(conn net.Conn, msgID, reqID uint32, body []byte) error {
	frameBody := protocol.NewMessageBody(msgID, reqID, body)
	frame := make([]byte, 4+len(frameBody))
	binary.BigEndian.PutUint32(frame[:4], uint32(len(frameBody)))
	copy(frame[4:], frameBody)
	_, err := conn.Write(frame)
	return err
}

// ---------------------------------------------------------------------------
// 可配置的 stub 客户端（生命周期/注册错误分支）
// ---------------------------------------------------------------------------

type lifecycleStubClient struct {
	registered map[string]croupier.FunctionDescriptor
	failIDs    map[string]bool
	connectErr error
	serveErr   error
	stopErr    error
}

func newLifecycleStubClient() *lifecycleStubClient {
	return &lifecycleStubClient{
		registered: make(map[string]croupier.FunctionDescriptor),
		failIDs:    make(map[string]bool),
	}
}

func (s *lifecycleStubClient) RegisterFunction(desc croupier.FunctionDescriptor, _ croupier.FunctionHandler) error {
	if s.failIDs[desc.ID] {
		return fmt.Errorf("rejected: %s", desc.ID)
	}
	s.registered[desc.ID] = desc
	return nil
}
func (s *lifecycleStubClient) Connect(ctx context.Context) error { return s.connectErr }
func (s *lifecycleStubClient) Serve(ctx context.Context) error   { return s.serveErr }
func (s *lifecycleStubClient) Stop() error                       { return s.stopErr }
func (s *lifecycleStubClient) Close() error                      { return nil }

// ---------------------------------------------------------------------------
// demonstrateClientRegistration：五个注册失败分支
// ---------------------------------------------------------------------------

func TestComprehensiveRegistrationErrorBranches(t *testing.T) {
	ids := []string{"player.ban", "item.create", "player.data", "guild.manage", "util.process"}
	for _, id := range ids {
		t.Run(id, func(t *testing.T) {
			client := newLifecycleStubClient()
			client.failIDs[id] = true
			err := demonstrateClientRegistration(client)
			if err == nil {
				t.Fatalf("expected registration failure for %s", id)
			}
			if _, ok := client.registered[id]; ok {
				t.Fatalf("%s should not be registered", id)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// demonstrateClientLifecycle
// ---------------------------------------------------------------------------

func TestComprehensiveLifecycleConnectFailure(t *testing.T) {
	client := newLifecycleStubClient()
	client.connectErr = fmt.Errorf("dial refused")
	if err := demonstrateClientLifecycle(client); err == nil || !bytes.Contains([]byte(err.Error()), []byte("Agent")) {
		t.Fatalf("expected connect failure, got %v", err)
	}
}

func TestComprehensiveLifecycleServeErrorAndStopError(t *testing.T) {
	serveClient := newLifecycleStubClient()
	serveClient.serveErr = fmt.Errorf("serve boom")
	if err := demonstrateClientLifecycle(serveClient); err != nil {
		t.Fatalf("Serve goroutine errors must only be logged, got %v", err)
	}

	stopClient := newLifecycleStubClient()
	stopClient.stopErr = fmt.Errorf("stop boom")
	err := demonstrateClientLifecycle(stopClient)
	if err == nil || !bytes.Contains([]byte(err.Error()), []byte("stop")) {
		t.Fatalf("expected stop failure, got %v", err)
	}
}

func TestComprehensiveLifecycleHappyPath(t *testing.T) {
	agent := startCompFakeAgent(t)
	config := &croupier.ClientConfig{
		AgentAddr:         agent.addr(),
		GameID:            "comprehensive-example",
		Env:               "development",
		ServiceID:         "demo-service-go",
		ServiceVersion:    "1.0.0",
		TimeoutSeconds:    5,
		HeartbeatInterval: 1,
		Insecure:          true,
		DisableLogging:    true,
	}
	client := croupier.NewClient(config)
	if err := demonstrateClientRegistration(client); err != nil {
		t.Fatalf("registration: %v", err)
	}
	if err := demonstrateClientLifecycle(client); err != nil {
		t.Fatalf("demonstrateClientLifecycle: %v", err)
	}
}

// ---------------------------------------------------------------------------
// demonstrateErrorHandling：连接超时打印分支
// ---------------------------------------------------------------------------

func TestComprehensiveErrorHandlingConnectTimeoutPrints(t *testing.T) {
	config := &croupier.ClientConfig{
		AgentAddr:      "127.0.0.1:1",
		GameID:         "comprehensive-example",
		Env:            "development",
		ServiceID:      "demo-service-go",
		ServiceVersion: "1.0.0",
		TimeoutSeconds: 5,
		Insecure:       true,
		DisableLogging: true,
	}
	client := croupier.NewClient(config)
	demonstrateErrorHandling(client)
}

// ---------------------------------------------------------------------------
// demonstrateInvokerInterface
// ---------------------------------------------------------------------------

func compJSON(t *testing.T, v interface{}) []byte {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestComprehensiveInvokerInterfaceCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := demonstrateInvokerInterface(ctx)
	if err == nil || !bytes.Contains([]byte(err.Error()), []byte("context canceled")) {
		t.Fatalf("expected connect failure, got %v", err)
	}
}

func TestComprehensiveInvokerInterfaceHappyFlow(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && len(r.URL.Path) > 0 && r.URL.Path[len(r.URL.Path)-len("/invoke"):] == "/invoke":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"result": map[string]interface{}{"banned": true}})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tasks":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"taskId": "task-happy"})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tasks/task-happy":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"id": "task-happy", "status": "RUNNING", "progress": 42})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tasks/task-happy/events":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"items": []map[string]interface{}{
					{"seq": 1, "type": "progress", "message": "10%"},
					{"seq": 2, "type": "completed", "message": "done"},
				},
				"done": true,
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tasks/task-happy/cancel":
			_, _ = w.Write([]byte(`{}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	t.Setenv("CROUPIER_SERVER_URL", server.URL+"/api/v1")

	if err := demonstrateInvokerInterface(context.Background()); err != nil {
		t.Fatalf("demonstrateInvokerInterface: %v", err)
	}
}

func TestComprehensiveInvokerInterfaceErrorBranches(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && len(r.URL.Path) > 0 && r.URL.Path[len(r.URL.Path)-len("/invoke"):] == "/invoke":
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"invoke_failed"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tasks":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"taskId": "task-mixed"})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tasks/task-mixed":
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte(`{"error":"status_failed"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tasks/task-mixed/events":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"items": []map[string]interface{}{
					{"seq": 1, "type": "error", "message": "agent dispatch failed"},
				},
				"done": true,
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	t.Setenv("CROUPIER_SERVER_URL", server.URL+"/api/v1")

	if err := demonstrateInvokerInterface(context.Background()); err != nil {
		t.Fatalf("error branches must only log, got %v", err)
	}
}

func TestComprehensiveInvokerInterfaceStartTaskFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tasks":
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"task_failed"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	t.Setenv("CROUPIER_SERVER_URL", server.URL+"/api/v1")

	if err := demonstrateInvokerInterface(context.Background()); err != nil {
		t.Fatalf("StartTask failure must only be logged, got %v", err)
	}
}

func TestComprehensiveInvokerInterfaceCancelFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tasks":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"taskId": "task-cancelfail"})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tasks/task-cancelfail":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"id": "task-cancelfail", "status": "RUNNING", "progress": 3})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tasks/task-cancelfail/events":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"items": []map[string]interface{}{
					{"seq": 1, "type": "progress", "message": "1%"},
					{"seq": 2, "type": "completed", "message": "done"},
				},
				"done": true,
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tasks/task-cancelfail/cancel":
			w.WriteHeader(http.StatusConflict)
			_, _ = w.Write([]byte(`{"error":"already_finished"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	t.Setenv("CROUPIER_SERVER_URL", server.URL+"/api/v1")

	if err := demonstrateInvokerInterface(context.Background()); err != nil {
		t.Fatalf("CancelTask failure must only be logged, got %v", err)
	}
}

func TestComprehensiveInvokerInterfaceEventCap(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && len(r.URL.Path) > 0 && r.URL.Path[len(r.URL.Path)-len("/invoke"):] == "/invoke":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"result": map[string]interface{}{"banned": true}})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tasks":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"taskId": "task-cap"})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tasks/task-cap":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"id": "task-cap", "status": "RUNNING", "progress": 1})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tasks/task-cap/events":
			if r.URL.Query().Get("after_seq") == "" || r.URL.Query().Get("after_seq") == "0" {
				items := make([]map[string]interface{}, 0, 5)
				for i := 1; i <= 5; i++ {
					items = append(items, map[string]interface{}{"seq": i, "type": "progress", "message": fmt.Sprintf("%d%%", i*10)})
				}
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"items": items, "done": false})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"items": []map[string]interface{}{}, "done": true})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tasks/task-cap/cancel":
			_, _ = w.Write([]byte(`{}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	t.Setenv("CROUPIER_SERVER_URL", server.URL+"/api/v1")

	done := make(chan error, 1)
	go func() { done <- demonstrateInvokerInterface(context.Background()) }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("demonstrateInvokerInterface: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("demonstrateInvokerInterface did not finish")
	}
	// 轮询协程在第二轮 done=true 后退出，等待 httptest 关闭不阻塞。
	time.Sleep(200 * time.Millisecond)
}

// ---------------------------------------------------------------------------
// main()
// ---------------------------------------------------------------------------

func TestComprehensiveMainRunsToCompletion(t *testing.T) {
	agent := startCompFakeAgent(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && len(r.URL.Path) > 0 && r.URL.Path[len(r.URL.Path)-len("/invoke"):] == "/invoke":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"result": map[string]interface{}{"banned": true}})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tasks":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"taskId": "task-main"})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tasks/task-main":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"id": "task-main", "status": "RUNNING", "progress": 7})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tasks/task-main/events":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"items": []map[string]interface{}{
					{"seq": 1, "type": "progress", "message": "5%"},
					{"seq": 2, "type": "completed", "message": "done"},
				},
				"done": true,
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tasks/task-main/cancel":
			_, _ = w.Write([]byte(`{}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	t.Setenv("CROUPIER_AGENT_ADDR", agent.addr())
	t.Setenv("CROUPIER_SERVER_URL", server.URL+"/api/v1")

	done := make(chan struct{})
	go func() {
		main()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(20 * time.Second):
		t.Fatal("main did not complete")
	}
}

// 生命周期演示的 3 秒等待期内发送 SIGINT：main 的 ctx 被取消，
// demonstrateInvokerInterface 的 Connect 失败并走 log.Printf 分支。
func TestComprehensiveMainEarlySignalCancelsInvokerDemo(t *testing.T) {
	agent := startCompFakeAgent(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"result": map[string]interface{}{"banned": true}})
	}))
	defer server.Close()

	t.Setenv("CROUPIER_AGENT_ADDR", agent.addr())
	t.Setenv("CROUPIER_SERVER_URL", server.URL+"/api/v1")

	done := make(chan struct{})
	go func() {
		main()
		close(done)
	}()

	select {
	case <-agent.registered:
	case <-time.After(5 * time.Second):
		t.Fatal("fake agent did not observe registration")
	}
	if err := syscall.Kill(os.Getpid(), syscall.SIGINT); err != nil {
		t.Fatalf("send SIGINT: %v", err)
	}
	select {
	case <-done:
	case <-time.After(20 * time.Second):
		t.Fatal("main did not complete after early signal")
	}
}

func TestComprehensiveMainLifecycleFailureExits(t *testing.T) {
	if os.Getenv("EXAMPLE_COMP_CHILD") == "1" {
		main()
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=TestComprehensiveMainLifecycleFailureExits")
	cmd.Env = append(os.Environ(),
		"EXAMPLE_COMP_CHILD=1",
		"CROUPIER_AGENT_ADDR=127.0.0.1:1",
	)
	out, err := cmd.CombinedOutput()
	if ee, ok := err.(*exec.ExitError); !ok || ee.ExitCode() != 1 {
		t.Fatalf("expected child exit status 1, got %v\n%s", err, out)
	}
	if !bytes.Contains(out, []byte("生命周期演示失败")) {
		t.Fatalf("unexpected child output:\n%s", out)
	}
}
