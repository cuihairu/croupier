package session

import (
	"context"
	"io"
	"log/slog"
	"net"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/cuihairu/croupier/agents/devops/config"
	"github.com/cuihairu/croupier/core/execlog"
	"github.com/cuihairu/croupier/core/transport/tcp"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	"google.golang.org/protobuf/proto"
)

// relayPeerServer 会话中继 e2e 的 server 侧（真 MuxConn）：向 agent 下行
// 会话请求（Call），接收上行会话事件（handler 捕获）。
type relayPeerServer struct {
	t      *testing.T
	ln     net.Listener
	events chan *opsv1.DevopsSessionEvent

	mu   sync.Mutex
	_mux *tcp.MuxConn // mu 保护；accept goroutine 写、测试读
}

func (p *relayPeerServer) setMux(m *tcp.MuxConn) {
	p.mu.Lock()
	p._mux = m
	p.mu.Unlock()
}

func (p *relayPeerServer) ready() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p._mux != nil
}

func (p *relayPeerServer) mux() *tcp.MuxConn {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p._mux
}

func newRelayPeerServer(t *testing.T) (*relayPeerServer, net.Listener) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	p := &relayPeerServer{t: t, ln: ln, events: make(chan *opsv1.DevopsSessionEvent, 64)}
	handler := eventCaptureHandler(p.events)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		m := tcp.NewMuxConn(conn, &tcp.Config{RecvTimeout: 2 * time.Second, SendTimeout: 2 * time.Second}, handler)
		p.setMux(m)
		_ = m.Run(context.Background())
	}()
	return p, ln
}

func eventCaptureHandler(ch chan<- *opsv1.DevopsSessionEvent) *relayEventHandler {
	return &relayEventHandler{events: ch}
}

type relayEventHandler struct {
	events chan<- *opsv1.DevopsSessionEvent
}

func (h *relayEventHandler) Handle(_ context.Context, msgID uint32, _ uint32, body []byte) ([]byte, error) {
	if msgID == protocol.MsgDevopsSessionEvent {
		ev := &opsv1.DevopsSessionEvent{}
		if err := proto.Unmarshal(body, ev); err == nil {
			h.events <- ev
		}
	}
	return nil, nil
}

// call marshals req, sends one request frame down the wire, decodes the
// response into resp (mirrors server-side ResolveAgentConn callers).
func (p *relayPeerServer) call(t *testing.T, msgID uint32, req, resp proto.Message) {
	t.Helper()
	data, err := proto.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	_, respData, err := p.mux().Call(context.Background(), msgID, data)
	if err != nil {
		t.Fatalf("peer call %s: %v", protocol.MsgIDString(msgID), err)
	}
	if err := proto.Unmarshal(respData, resp); err != nil {
		t.Fatalf("unmarshal %s response: %v", protocol.MsgIDString(msgID), err)
	}
}

// waitEvent 取一条上行事件（超时即失败）。
func (p *relayPeerServer) waitEvent(t *testing.T) *opsv1.DevopsSessionEvent {
	t.Helper()
	select {
	case ev := <-p.events:
		return ev
	case <-time.After(10 * time.Second):
		t.Fatal("session event never arrived")
		return nil
	}
}

// drainOutputEvents 收集 output 事件直到拼出 want 或超时（echo 分片不保证
// 单帧到达）。
func (p *relayPeerServer) drainOutputEvents(t *testing.T, want string) string {
	t.Helper()
	var got []byte
	deadline := time.After(10 * time.Second)
	for {
		select {
		case ev := <-p.events:
			if ev.GetKind() == "output" {
				got = append(got, ev.GetData()...)
				if string(got) == want {
					return string(got)
				}
			}
			if ev.GetKind() == "end" {
				t.Fatalf("session ended before echo: outcome=%q got=%q", ev.GetOutcome(), got)
			}
		case <-deadline:
			t.Fatalf("echo never arrived: got %q want %q", got, want)
		}
	}
}

// newRelayEnv 真实链路装配：echo SSH 服务端 + 真 sshTarget + Broker/Hub +
// Relay（audit 走 fake 断言建立链）。
func newRelayEnv(t *testing.T) (*Relay, *Hub, *fakeAudit, *echoSSHServer, string, int) {
	t.Helper()
	srv := newEchoSSHServer(t, "ops", "secret")
	host, port := srv.HostPort()
	tgt := newTestTarget(t, "ops", "secret")
	dir := t.TempDir()
	rec, err := OpenRecorder(filepath.Join(dir, "sessions"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = rec.Close() })
	audit := &fakeAudit{}
	allow := NewAllowlist([]config.AllowHost{
		{Host: host, Ports: []int{port}, Protocols: []string{"ssh"}},
	})
	broker := NewBroker(allow, audit, tgt, rec, "devops", "d1", "demo", "prod")
	hub := NewHub(broker, rec, filepath.Join(dir, "casts"), HubConfig{},
		slog.New(slog.NewTextHandler(io.Discard, nil)))
	t.Cleanup(func() { _ = hub.Close() })
	relay := NewRelay(hub, slog.New(slog.NewTextHandler(io.Discard, nil)))
	return relay, hub, audit, srv, host, port
}

// TestRelayEndToEnd 真实 SSH 全链：peer(server 角色) 下行 open/input/resize/
// close → Broker/Hub/sshTarget → echo SSH 服务端 → 上行 output/end 事件，
// 断线清理可验（hub 摘除 + 结束审计 + peer 侧 end 事件）。
func TestRelayEndToEnd(t *testing.T) {
	relay, hub, audit, srv, host, port := newRelayEnv(t)
	peer, ln := newRelayPeerServer(t)

	// agent 侧：真 MuxConn 挂 Relay；上行 sender 走该连接（runner 同款装配）。
	muxCh := make(chan *tcp.MuxConn, 1)
	go func() {
		conn, err := net.Dial("tcp", ln.Addr().String())
		if err != nil {
			return
		}
		m := tcp.NewMuxConn(conn, &tcp.Config{RecvTimeout: 2 * time.Second, SendTimeout: 2 * time.Second}, relay)
		muxCh <- m
		_ = m.Run(context.Background())
	}()
	var agentMux *tcp.MuxConn
	select {
	case agentMux = <-muxCh:
	case <-time.After(5 * time.Second):
		t.Fatal("agent mux never connected")
	}
	t.Cleanup(func() { _ = agentMux.Close() })
	relay.SetSender(func(ctx context.Context, msgID uint32, msg proto.Message) error {
		data, err := proto.Marshal(msg)
		if err != nil {
			return err
		}
		return agentMux.Send(ctx, msgID, data)
	})

	// 等 server 侧 mux 就绪（Accept 完成）。
	waitFor(t, peer.ready)

	// 1) open：server 下行 → 建立链放行 → 会话进入数据面。
	openResp := &opsv1.DevopsSessionOpenResponse{}
	peer.call(t, protocol.MsgDevopsSessionOpenRequest, &opsv1.DevopsSessionOpenRequest{
		SessionId: "sess-relay-1", Operator: "alice", Protocol: "ssh",
		Host: host, Port: int32(port), Cols: 100, Rows: 30,
	}, openResp)
	if !openResp.GetOk() {
		t.Fatalf("open refused: %s", openResp.GetError())
	}
	if openResp.GetSessionId() != "sess-relay-1" || openResp.GetChannel() != sshChannelName {
		t.Fatalf("open response = %+v", openResp)
	}
	if hub.Count() != 1 {
		t.Fatalf("live = %d", hub.Count())
	}

	// 2) input：键盘数据下行 → echo SSH 服务端回显 → 上行 output 事件。
	inputResp := &opsv1.DevopsSessionInputResponse{}
	peer.call(t, protocol.MsgDevopsSessionInputRequest, &opsv1.DevopsSessionInputRequest{
		SessionId: "sess-relay-1", Data: []byte("hello relay\n"),
	}, inputResp)
	if !inputResp.GetOk() {
		t.Fatalf("input refused: %s", inputResp.GetError())
	}
	got := peer.drainOutputEvents(t, "hello relay\n")

	// 3) resize：窗口调整下行 → echo 服务端收到 window-change。
	resizeResp := &opsv1.DevopsSessionResizeResponse{}
	peer.call(t, protocol.MsgDevopsSessionResizeRequest, &opsv1.DevopsSessionResizeRequest{
		SessionId: "sess-relay-1", Cols: 120, Rows: 40,
	}, resizeResp)
	if !resizeResp.GetOk() {
		t.Fatalf("resize refused: %s", resizeResp.GetError())
	}
	waitFor(t, func() bool { return len(srv.Resizes()) == 1 })
	if rz := srv.Resizes(); rz[0] != [2]int{120, 40} {
		t.Fatalf("resizes = %v", rz)
	}

	// 4) close：主动断开 → 完整断线清理链 → end 事件上行。
	closeResp := &opsv1.DevopsSessionCloseResponse{}
	peer.call(t, protocol.MsgDevopsSessionCloseRequest, &opsv1.DevopsSessionCloseRequest{
		SessionId: "sess-relay-1", Reason: "panel close",
	}, closeResp)
	if !closeResp.GetOk() {
		t.Fatalf("close refused: %s", closeResp.GetError())
	}
	waitFor(t, func() bool { return hub.Count() == 0 })
	endSeen := false
	for i := 0; i < 16 && !endSeen; i++ {
		select {
		case ev := <-peer.events:
			if ev.GetKind() == "end" && ev.GetSessionId() == "sess-relay-1" && ev.GetOutcome() == "closed" {
				endSeen = true
			}
		case <-time.After(2 * time.Second):
		}
	}
	if !endSeen {
		t.Fatal("end event (closed) never arrived at server")
	}
	_ = got

	// 断线清理审计链：admitted + success + session.end(closed)。
	records := audit.snapshot()
	if len(records) < 3 {
		t.Fatalf("audit records = %d", len(records))
	}
	if records[0].Status != execlog.StatusAdmitted || records[1].Status != execlog.StatusSuccess {
		t.Fatalf("open chain = %s/%s", records[0].Status, records[1].Status)
	}
	last := records[len(records)-1]
	if last.Action != "session.end" || last.Status != execlog.StatusSuccess || last.Summary != "closed" {
		t.Fatalf("end record = %+v", last)
	}
}

// TestRelayRefusals 建立链拒绝经中继如实回帧：白名单外主机、未知会话的
// input/close。
func TestRelayRefusals(t *testing.T) {
	relay, _, audit, _, host, port := newRelayEnv(t)

	openResp := &opsv1.DevopsSessionOpenResponse{}
	if err := relayHandle(t, relay, protocol.MsgDevopsSessionOpenRequest, &opsv1.DevopsSessionOpenRequest{
		Operator: "bob", Protocol: "ssh", Host: host, Port: int32(port + 1), // 白名单外端口
	}, openResp); err != nil {
		t.Fatalf("handle open: %v", err)
	}
	if openResp.GetOk() {
		t.Fatal("open outside allowlist must be refused")
	}

	inResp := &opsv1.DevopsSessionInputResponse{}
	if err := relayHandle(t, relay, protocol.MsgDevopsSessionInputRequest, &opsv1.DevopsSessionInputRequest{
		SessionId: "nope", Data: []byte("x"),
	}, inResp); err != nil {
		t.Fatalf("handle input: %v", err)
	}
	if inResp.GetOk() || inResp.GetError() != "session not found" {
		t.Fatalf("input response = %+v", inResp)
	}

	closeResp := &opsv1.DevopsSessionCloseResponse{}
	if err := relayHandle(t, relay, protocol.MsgDevopsSessionCloseRequest, &opsv1.DevopsSessionCloseRequest{
		SessionId: "nope",
	}, closeResp); err != nil {
		t.Fatalf("handle close: %v", err)
	}
	if closeResp.GetOk() || closeResp.GetError() != "session not found" {
		t.Fatalf("close response = %+v", closeResp)
	}

	// 白名单拒绝留痕（链内 result 记录）。
	waitFor(t, func() bool { return len(audit.snapshot()) >= 1 })
}

// TestRelayDisabledPlane hub 未装配（devops.enabled=false 同款）时 open
// 如实回「会话面关闭」，不伪装成功。
func TestRelayDisabledPlane(t *testing.T) {
	relay := NewRelay(nil, nil)
	openResp := &opsv1.DevopsSessionOpenResponse{}
	if err := relayHandle(t, relay, protocol.MsgDevopsSessionOpenRequest, &opsv1.DevopsSessionOpenRequest{
		Operator: "bob", Protocol: "ssh", Host: "h", Port: 22,
	}, openResp); err != nil {
		t.Fatalf("handle open: %v", err)
	}
	if openResp.GetOk() || openResp.GetError() == "" {
		t.Fatalf("disabled plane response = %+v", openResp)
	}
}

// TestRelayControlLossClosesSessions 控制面丢失 = 会话面关闭：fake 终端
// 会话经 OnConnClosed 全量收束（hub 摘除 + session.end 审计）。
func TestRelayControlLossClosesSessions(t *testing.T) {
	term := newFakeTerminal("s-loss")
	env := newHubEnv(t, term, HubConfig{InputWait: time.Second})
	relay := NewRelay(env.hub, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if _, err := env.hub.Open(context.Background(), hubRequest()); err != nil {
		t.Fatalf("open: %v", err)
	}
	if env.hub.Count() != 1 {
		t.Fatalf("live = %d", env.hub.Count())
	}
	relay.OnConnClosed()
	waitFor(t, func() bool { return env.hub.Count() == 0 })
	// 结束审计已落（清理链末步）。
	waitFor(t, func() bool {
		for _, rec := range env.audit.snapshot() {
			if rec.Action == "session.end" {
				return true
			}
		}
		return false
	})
}

// relayHandle 直接驱动 Relay.Handle（绕过 wire，供拒绝分支用例聚焦语义）。
func relayHandle(t *testing.T, relay *Relay, msgID uint32, req, resp proto.Message) error {
	t.Helper()
	data, err := proto.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	respData, err := relay.Handle(context.Background(), msgID, 1, data)
	if err != nil {
		return err
	}
	return proto.Unmarshal(respData, resp)
}

// TestRelayUnknownMessage 未知消息回空帧不回错（读循环不断链）。
func TestRelayUnknownMessage(t *testing.T) {
	relay := NewRelay(nil, nil)
	body, err := relay.Handle(context.Background(), protocol.MsgHeartbeatRequest, 1, []byte("{}"))
	if err != nil {
		t.Fatalf("unknown message must not error: %v", err)
	}
	if body != nil {
		t.Fatalf("unknown message body = %q", body)
	}
}
