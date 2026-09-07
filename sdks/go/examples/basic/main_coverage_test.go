package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"io"
	"net"
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

type basicFakeAgent struct {
	ln         net.Listener
	registered chan struct{}
	once       sync.Once
}

func startBasicFakeAgent(t *testing.T) *basicFakeAgent {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("fake agent listen: %v", err)
	}
	a := &basicFakeAgent{ln: ln, registered: make(chan struct{})}
	go a.acceptLoop()
	t.Cleanup(func() { _ = ln.Close() })
	return a
}

func (a *basicFakeAgent) addr() string { return a.ln.Addr().String() }

func (a *basicFakeAgent) acceptLoop() {
	for {
		conn, err := a.ln.Accept()
		if err != nil {
			return
		}
		go a.serveConn(conn)
	}
}

func (a *basicFakeAgent) serveConn(conn net.Conn) {
	defer func() { _ = conn.Close() }()
	for {
		msgID, reqID, _, err := basicReadFrame(conn)
		if err != nil {
			return
		}
		var respMsgID uint32
		var respBody []byte
		switch msgID {
		case protocol.MsgProviderConnectRequest:
			respMsgID = protocol.MsgProviderConnectResponse
			respBody, _ = proto.Marshal(&sdkv1.ProviderConnectResponse{SessionId: "sess-basic-main"})
			a.once.Do(func() { close(a.registered) })
		case protocol.MsgProviderHeartbeatRequest:
			respMsgID = protocol.MsgProviderHeartbeatResponse
			respBody, _ = proto.Marshal(&sdkv1.ProviderHeartbeatResponse{})
		default:
			respMsgID = protocol.MsgInvokeResponse
			respBody, _ = proto.Marshal(&sdkv1.InvokeResponse{Payload: []byte(`{}`)})
		}
		if err := basicWriteFrame(conn, respMsgID, reqID, respBody); err != nil {
			return
		}
	}
}

func basicReadFrame(conn net.Conn) (msgID, reqID uint32, body []byte, err error) {
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

func basicWriteFrame(conn net.Conn, msgID, reqID uint32, body []byte) error {
	frameBody := protocol.NewMessageBody(msgID, reqID, body)
	frame := make([]byte, 4+len(frameBody))
	binary.BigEndian.PutUint32(frame[:4], uint32(len(frameBody)))
	copy(frame[4:], frameBody)
	_, err := conn.Write(frame)
	return err
}

// ---------------------------------------------------------------------------
// main()
// ---------------------------------------------------------------------------

func TestBasicMainRunsToCompletion(t *testing.T) {
	agent := startBasicFakeAgent(t)
	t.Setenv("CROUPIER_AGENT_ADDR", agent.addr())

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
	// 等待 Serve 进入事件循环，随后用 SIGINT 触发 main 内部的优雅退出路径。
	time.Sleep(300 * time.Millisecond)
	if err := syscall.Kill(os.Getpid(), syscall.SIGINT); err != nil {
		t.Fatalf("send SIGINT: %v", err)
	}

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("main did not return after SIGINT")
	}
}

// ---------------------------------------------------------------------------
// main() 失败路径（log.Fatalf → os.Exit，只能在子进程中验证）
// ---------------------------------------------------------------------------

func TestBasicMainServeFailureExits(t *testing.T) {
	if os.Getenv("EXAMPLE_BASIC_CHILD") == "1" {
		main()
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=TestBasicMainServeFailureExits")
	cmd.Env = append(os.Environ(),
		"EXAMPLE_BASIC_CHILD=1",
		"CROUPIER_AGENT_ADDR=127.0.0.1:1",
	)
	out, err := cmd.CombinedOutput()
	if ee, ok := err.(*exec.ExitError); !ok || ee.ExitCode() != 1 {
		t.Fatalf("expected child exit status 1, got %v\n%s", err, out)
	}
	if !bytes.Contains(out, []byte("Failed to serve")) {
		t.Fatalf("unexpected child output:\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// registerFunctions 内注册的 handler 闭包
// ---------------------------------------------------------------------------

type handlerCapturingClient struct {
	handlers map[string]croupier.FunctionHandler
}

func newHandlerCapturingClient() *handlerCapturingClient {
	return &handlerCapturingClient{handlers: make(map[string]croupier.FunctionHandler)}
}

func (c *handlerCapturingClient) RegisterFunction(desc croupier.FunctionDescriptor, handler croupier.FunctionHandler) error {
	c.handlers[desc.ID] = handler
	return nil
}
func (c *handlerCapturingClient) Connect(ctx context.Context) error { return nil }
func (c *handlerCapturingClient) Serve(ctx context.Context) error   { return nil }
func (c *handlerCapturingClient) Stop() error                       { return nil }
func (c *handlerCapturingClient) Close() error                      { return nil }

func TestBasicRegisterFunctionHandlers(t *testing.T) {
	client := newHandlerCapturingClient()
	if err := registerFunctions(client); err != nil {
		t.Fatalf("registerFunctions: %v", err)
	}

	banOut, err := client.handlers["player.ban"](context.Background(), []byte(`{"id":"p1"}`))
	if err != nil {
		t.Fatalf("player.ban handler: %v", err)
	}
	var banResult map[string]interface{}
	if err := json.Unmarshal(banOut, &banResult); err != nil {
		t.Fatalf("player.ban output invalid JSON: %v", err)
	}
	if banResult["status"] != "success" || banResult["action"] != "ban" {
		t.Fatalf("unexpected player.ban result: %s", banOut)
	}

	itemOut, err := client.handlers["item.create"](context.Background(), []byte(`{}`))
	if err != nil {
		t.Fatalf("item.create handler: %v", err)
	}
	var itemResult map[string]interface{}
	if err := json.Unmarshal(itemOut, &itemResult); err != nil {
		t.Fatalf("item.create output invalid JSON: %v", err)
	}
	if itemResult["status"] != "success" || itemResult["action"] != "create" {
		t.Fatalf("unexpected item.create result: %s", itemOut)
	}
}
