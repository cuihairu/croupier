package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"net"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/cuihairu/croupier/sdks/go/pkg/croupier/protocol"
	sdkv1 "github.com/cuihairu/croupier/sdks/go/pkg/pb/croupier/sdk/v1"
	"google.golang.org/protobuf/proto"
)

// ---------------------------------------------------------------------------
// Fake agent（本地回环上的最小 Provider 协议对端）
// ---------------------------------------------------------------------------

type demoFakeAgent struct {
	ln         net.Listener
	registered chan struct{}
	once       sync.Once
}

func startDemoFakeAgent(t *testing.T) *demoFakeAgent {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("fake agent listen: %v", err)
	}
	a := &demoFakeAgent{ln: ln, registered: make(chan struct{})}
	go a.acceptLoop()
	t.Cleanup(func() { _ = ln.Close() })
	return a
}

func (a *demoFakeAgent) addr() string { return a.ln.Addr().String() }

func (a *demoFakeAgent) acceptLoop() {
	for {
		conn, err := a.ln.Accept()
		if err != nil {
			return
		}
		go a.serveConn(conn)
	}
}

func (a *demoFakeAgent) serveConn(conn net.Conn) {
	defer func() { _ = conn.Close() }()
	for {
		msgID, reqID, _, err := demoReadFrame(conn)
		if err != nil {
			return
		}
		var respMsgID uint32
		var respBody []byte
		switch msgID {
		case protocol.MsgProviderConnectRequest:
			respMsgID = protocol.MsgProviderConnectResponse
			respBody, _ = proto.Marshal(&sdkv1.ProviderConnectResponse{SessionId: "sess-demo-main"})
			a.once.Do(func() { close(a.registered) })
		case protocol.MsgProviderHeartbeatRequest:
			respMsgID = protocol.MsgProviderHeartbeatResponse
			respBody, _ = proto.Marshal(&sdkv1.ProviderHeartbeatResponse{})
		default:
			respMsgID = protocol.MsgInvokeResponse
			respBody, _ = proto.Marshal(&sdkv1.InvokeResponse{Payload: []byte(`{}`)})
		}
		if err := demoWriteFrame(conn, respMsgID, reqID, respBody); err != nil {
			return
		}
	}
}

func demoReadFrame(conn net.Conn) (msgID, reqID uint32, body []byte, err error) {
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

func demoWriteFrame(conn net.Conn, msgID, reqID uint32, body []byte) error {
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

func TestDemoMainRunsToCompletion(t *testing.T) {
	agent := startDemoFakeAgent(t)
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

func TestDemoMainServeFailureExits(t *testing.T) {
	if os.Getenv("EXAMPLE_DEMO_CHILD") == "1" {
		main()
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=TestDemoMainServeFailureExits")
	cmd.Env = append(os.Environ(),
		"EXAMPLE_DEMO_CHILD=1",
		"CROUPIER_AGENT_ADDR=127.0.0.1:1",
	)
	out, err := cmd.CombinedOutput()
	if ee, ok := err.(*exec.ExitError); !ok || ee.ExitCode() != 1 {
		t.Fatalf("expected child exit status 1, got %v\n%s", err, out)
	}
	if !bytes.Contains(out, []byte("serve failed")) {
		t.Fatalf("unexpected child output:\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// handler 分支补充
// ---------------------------------------------------------------------------

func TestDemoHandlersRejectInvalidJSON(t *testing.T) {
	store := newDemoStore()
	fns := []struct {
		name string
		fn   func(context.Context, []byte) ([]byte, error)
	}{
		{"playerUpdate", store.playerUpdate},
		{"playerDelete", store.playerDelete},
		{"playerBan", store.playerBan},
		{"playerBatchBan", store.playerBatchBan},
		{"playerRecharge", store.playerRecharge},
		{"playerList", store.playerList},
		{"orderCreate", store.orderCreate},
		{"orderUpdate", store.orderUpdate},
		{"orderDelete", store.orderDelete},
		{"leaderboardList", store.leaderboardList},
		{"leaderboardUpsert", store.leaderboardUpsert},
		{"leaderboardReset", store.leaderboardReset},
		{"inventoryList", store.inventoryList},
		{"inventoryGrant", store.inventoryGrant},
		{"inventoryConsume", store.inventoryConsume},
		{"mailSend", store.mailSend},
		{"mailList", store.mailList},
		{"mailClaim", store.mailClaim},
	}
	for _, fn := range fns {
		t.Run(fn.name, func(t *testing.T) {
			if err := runHandler(t, store, fn.fn, "not-json"); err == nil {
				t.Fatal("expected invalid JSON error")
			}
		})
	}
}

func TestDemoPlayerUpdateAllOptionalFields(t *testing.T) {
	store := newDemoStore()
	out, err := store.playerUpdate(context.Background(), []byte(`{"playerId":"player_1001","name":"NewName","vip":9,"status":"frozen","server":"s9"}`))
	if err != nil {
		t.Fatalf("playerUpdate: %v", err)
	}
	for _, want := range []string{"NewName", "frozen", "s9"} {
		if !contains(string(out), want) {
			t.Fatalf("update result %s missing %q", out, want)
		}
	}
}

func TestDemoPlayerBanBranches(t *testing.T) {
	store := newDemoStore()
	if err := runHandler(t, store, store.playerBan, `{}`); err == nil {
		t.Fatal("expected playerId required error")
	}
	if err := runHandler(t, store, store.playerBan, `{"playerId":"ghost"}`); err == nil {
		t.Fatal("expected player not found error")
	}
	out, err := store.playerBan(context.Background(), []byte(`{"playerId":"player_1001","reason":"cheat","durationHours":72}`))
	if err != nil {
		t.Fatalf("playerBan: %v", err)
	}
	if !contains(string(out), `"durationHours":72`) || !contains(string(out), "cheat") {
		t.Fatalf("unexpected ban result: %s", out)
	}
}

func TestDemoPlayerBatchBanBranches(t *testing.T) {
	store := newDemoStore()
	if err := runHandler(t, store, store.playerBatchBan, `{}`); err == nil {
		t.Fatal("expected ids required error")
	}
	// playerIds 备用键 + 缺失玩家。
	out, err := store.playerBatchBan(context.Background(), []byte(`{"playerIds":["player_1001","ghost"],"reason":"bulk"}`))
	if err != nil {
		t.Fatalf("playerBatchBan: %v", err)
	}
	if !contains(string(out), `"missing":["ghost"]`) || !contains(string(out), "bulk") {
		t.Fatalf("unexpected batch ban result: %s", out)
	}
}

func TestDemoStringSliceValueBranches(t *testing.T) {
	if got := stringSliceValue(map[string]any{}, "ids", "playerIds"); got != nil {
		t.Fatalf("absent keys = %v, want nil", got)
	}
	if got := stringSliceValue(map[string]any{"ids": []any{1, 2}}, "ids"); len(got) != 0 {
		t.Fatalf("non-string items = %v, want empty", got)
	}
}

func TestDemoPlayerRechargeBranches(t *testing.T) {
	store := newDemoStore()
	if err := runHandler(t, store, store.playerRecharge, `{}`); err == nil {
		t.Fatal("expected playerId required error")
	}
	if err := runHandler(t, store, store.playerRecharge, `{"playerId":"player_1001","amount":0}`); err == nil {
		t.Fatal("expected amount must be positive error")
	}
	if err := runHandler(t, store, store.playerRecharge, `{"playerId":"ghost","amount":5}`); err == nil {
		t.Fatal("expected player not found error")
	}
}

func TestDemoOrderBranches(t *testing.T) {
	store := newDemoStore()
	out, err := store.orderCreate(context.Background(), []byte(`{"playerId":"player_1001"}`))
	if err != nil {
		t.Fatalf("orderCreate auto ID: %v", err)
	}
	if !contains(string(out), "order_") {
		t.Fatalf("auto order ID missing: %s", out)
	}

	updated, err := store.orderUpdate(context.Background(), []byte(`{"orderId":"order_3001","channel":"alipay","amount":100}`))
	if err != nil {
		t.Fatalf("orderUpdate: %v", err)
	}
	if !contains(string(updated), "alipay") {
		t.Fatalf("channel not updated: %s", updated)
	}

	// 不带 playerId 过滤时对多条订单排序。
	fresh := newDemoStore()
	listed, err := fresh.orderList(context.Background(), []byte(`{}`))
	if err != nil {
		t.Fatalf("orderList: %v", err)
	}
	if !contains(string(listed), `"total":2`) {
		t.Fatalf("unexpected order list: %s", listed)
	}
}

func TestDemoLeaderboardTieBreaker(t *testing.T) {
	store := newDemoStore()
	if _, err := store.leaderboardUpsert(context.Background(), []byte(`{"playerId":"player_1001","score":500}`)); err != nil {
		t.Fatalf("upsert 1: %v", err)
	}
	if _, err := store.leaderboardUpsert(context.Background(), []byte(`{"playerId":"player_1002","score":500}`)); err != nil {
		t.Fatalf("upsert 2: %v", err)
	}
	out, err := store.leaderboardList(context.Background(), []byte(`{}`))
	if err != nil {
		t.Fatalf("leaderboardList: %v", err)
	}
	if !contains(string(out), `"total":2`) {
		t.Fatalf("unexpected leaderboard list: %s", out)
	}
}

func TestDemoMailListSortsMultipleMails(t *testing.T) {
	store := newDemoStore()
	if _, err := store.mailSend(context.Background(), []byte(`{"playerId":"player_1002","title":"first"}`)); err != nil {
		t.Fatalf("mailSend 1: %v", err)
	}
	if _, err := store.mailSend(context.Background(), []byte(`{"playerId":"player_1002","title":"second"}`)); err != nil {
		t.Fatalf("mailSend 2: %v", err)
	}
	out, err := store.mailList(context.Background(), []byte(`{"playerId":"player_1002"}`))
	if err != nil {
		t.Fatalf("mailList: %v", err)
	}
	if !contains(string(out), `"total":2`) {
		t.Fatalf("unexpected mail list: %s", out)
	}
}
