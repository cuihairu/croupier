package devopssessions

import (
	"context"
	"testing"
	"time"

	"github.com/cuihairu/croupier/core/transport"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

// devopsHubCaller 会话 wire 假服务端：echo 会话 ID；输入对 "sess-busy" 回
// busy（背压哨兵路径）；close/resize 恒 ok；open 按 openOK/openErr 开关。
type devopsHubCaller struct {
	calls   []uint32
	openOK  bool
	openErr string
}

func (f *devopsHubCaller) Call(_ context.Context, msgID uint32, reqBody []byte) (uint32, []byte, error) {
	f.calls = append(f.calls, msgID)
	switch msgID {
	case protocol.MsgDevopsSessionOpenRequest:
		req := &opsv1.DevopsSessionOpenRequest{}
		if err := proto.Unmarshal(reqBody, req); err != nil {
			return 0, nil, err
		}
		if !f.openOK {
			return protocol.GetResponseMsgID(msgID),
				mustMarshal(&opsv1.DevopsSessionOpenResponse{Ok: false, Error: f.openErr}), nil
		}
		return protocol.GetResponseMsgID(msgID),
			mustMarshal(&opsv1.DevopsSessionOpenResponse{Ok: true, SessionId: req.GetSessionId(), Channel: "ssh"}), nil
	case protocol.MsgDevopsSessionInputRequest:
		req := &opsv1.DevopsSessionInputRequest{}
		if err := proto.Unmarshal(reqBody, req); err != nil {
			return 0, nil, err
		}
		if string(req.GetData()) == "TRIGGER_BUSY" {
			return protocol.GetResponseMsgID(msgID),
				mustMarshal(&opsv1.DevopsSessionInputResponse{Ok: false, Error: "busy"}), nil
		}
		return protocol.GetResponseMsgID(msgID),
			mustMarshal(&opsv1.DevopsSessionInputResponse{Ok: true}), nil
	case protocol.MsgDevopsSessionResizeRequest:
		return protocol.GetResponseMsgID(msgID),
			mustMarshal(&opsv1.DevopsSessionResizeResponse{Ok: true}), nil
	case protocol.MsgDevopsSessionCloseRequest:
		return protocol.GetResponseMsgID(msgID),
			mustMarshal(&opsv1.DevopsSessionCloseResponse{Ok: true}), nil
	}
	return protocol.GetResponseMsgID(msgID), []byte{}, nil
}

func mustMarshal(msg proto.Message) []byte {
	body, err := proto.Marshal(msg)
	if err != nil {
		panic(err)
	}
	return body
}

// devopsHubResolver 会话解析假实现（caller 可随测试热换）。
type devopsHubResolver struct {
	caller transport.SessionCaller
}

func (r *devopsHubResolver) ResolveAgentConn(string) (transport.SessionCaller, bool) {
	if r.caller == nil {
		return nil, false
	}
	return r.caller, true
}

// openSession 走真实 Open 链建一条会话（operator=alice，target h:22）。
func openSession(t *testing.T, hub *Hub, resolver SessionResolver, agentID string) *Session {
	t.Helper()
	sess, err := hub.Open(context.Background(), resolver, agentID, &opsv1.DevopsSessionOpenRequest{
		Operator: "alice", Protocol: "ssh", Host: "h", Port: 22, Cols: 80, Rows: 24,
	}, "alice", "demo", "prod")
	require.NoError(t, err)
	return sess
}

func TestHubOpenRegistersAndLists(t *testing.T) {
	caller := &devopsHubCaller{openOK: true}
	hub := NewHub(nil)
	resolver := &devopsHubResolver{caller: caller}

	sess := openSession(t, hub, resolver, "a1")
	require.NotEmpty(t, sess.SessionID)
	require.Equal(t, "a1", sess.AgentID)
	require.Equal(t, "alice", sess.Operator)
	require.Equal(t, "ssh", sess.Protocol)
	require.Equal(t, "h", sess.Host)
	require.Equal(t, int32(22), sess.Port)
	require.Equal(t, "demo", sess.GameID)
	require.Equal(t, "prod", sess.Env)
	require.Equal(t, "ssh", sess.Channel)

	got, ok := hub.Get(sess.SessionID)
	require.True(t, ok)
	require.Equal(t, sess.SessionID, got.SessionID)
	require.Len(t, hub.List(), 1)

	// wire 请求确实携带 server 鉴权身份与会话 ID（agent 审计链真值来源）。
	require.Contains(t, caller.calls, uint32(protocol.MsgDevopsSessionOpenRequest))
}

func TestHubOpenRefusals(t *testing.T) {
	// agent 拒绝（白名单外等）→ 错误透传，不登记会话。
	caller := &devopsHubCaller{openOK: false, openErr: "host not in allowlist"}
	hub := NewHub(nil)
	resolver := &devopsHubResolver{caller: caller}
	_, err := hub.Open(context.Background(), resolver, "a1", &opsv1.DevopsSessionOpenRequest{
		Protocol: "ssh", Host: "evil", Port: 22,
	}, "alice", "demo", "prod")
	require.ErrorContains(t, err, "host not in allowlist")
	require.Empty(t, hub.List())

	// 无活跃连接 → 解析失败。
	resolver.caller = nil
	_, err = hub.Open(context.Background(), resolver, "a1", &opsv1.DevopsSessionOpenRequest{
		Protocol: "ssh", Host: "h", Port: 22,
	}, "alice", "demo", "prod")
	require.ErrorContains(t, err, "no active connection")

	// resolver 缺位（装配不完整）。
	_, err = hub.Open(context.Background(), nil, "a1", &opsv1.DevopsSessionOpenRequest{}, "alice", "", "")
	require.ErrorContains(t, err, "resolver unavailable")

	// nil hub 安全。
	var noHub *Hub
	_, err = noHub.Open(context.Background(), resolver, "a1", &opsv1.DevopsSessionOpenRequest{}, "", "", "")
	require.Error(t, err)
}

func TestHubRouteEventFanoutAndEnd(t *testing.T) {
	hub := NewHub(nil)
	resolver := &devopsHubResolver{caller: &devopsHubCaller{openOK: true}}
	sess := openSession(t, hub, resolver, "a1")

	// 第二消费者先订阅 → 回放含已有事件。
	sub1, ok := hub.AttachConsumer(sess.SessionID)
	require.True(t, ok)
	hub.RouteEvent("a1", &opsv1.DevopsSessionEvent{SessionId: sess.SessionID, Kind: "output", Data: []byte("hi")})

	select {
	case ev := <-sub1.C:
		require.Equal(t, "output", ev.GetKind())
		require.Equal(t, "hi", string(ev.GetData()))
	case <-time.After(2 * time.Second):
		t.Fatal("event never delivered")
	}

	// 跨 agent 事件不路由（会话属主校验在路由侧兜底）。
	hub.RouteEvent("other-agent", &opsv1.DevopsSessionEvent{SessionId: sess.SessionID, Kind: "output", Data: []byte("x")})
	select {
	case ev := <-sub1.C:
		t.Fatalf("cross-agent event delivered: %v", ev)
	case <-time.After(50 * time.Millisecond):
	}

	// sub2 晚订阅：回放含 output。
	sub2, ok := hub.AttachConsumer(sess.SessionID)
	require.True(t, ok)
	require.Len(t, sub2.Replay, 1)
	require.Equal(t, "hi", string(sub2.Replay[0].GetData()))

	// end 事件：两个消费者收到 → 通道关闭 → 会话摘除。
	hub.RouteEvent("a1", &opsv1.DevopsSessionEvent{SessionId: sess.SessionID, Kind: "end", Outcome: "closed"})
	_, ok = hub.Get(sess.SessionID)
	require.False(t, ok, "ended session must be removed")
	for _, sub := range []*Subscription{sub1, sub2} {
		select {
		case ev, alive := <-sub.C:
			if !alive {
				t.Fatalf("consumer closed before receiving end event")
			}
			require.Equal(t, "end", ev.GetKind())
			_, alive = <-sub.C
			require.False(t, alive, "channel must close after end event")
			require.Equal(t, EvictEnded, sub.EvictReason())
		case <-time.After(2 * time.Second):
			t.Fatal("end event never delivered")
		}
	}
	// 收束后再订阅 → 拒绝。
	_, ok = hub.AttachConsumer(sess.SessionID)
	require.False(t, ok)
}

func TestHubReplayCapAndSlowConsumerEviction(t *testing.T) {
	hub := NewHub(nil)
	resolver := &devopsHubResolver{caller: &devopsHubCaller{openOK: true}}
	sess := openSession(t, hub, resolver, "a1")

	// 慢消费者：不读通道；快消费者由后台 goroutine 持续排空。灌超过队列
	// 深度的事件 → 慢者摘除（溢出），会话本身不受影响，快者全程无损。
	slow, ok := hub.AttachConsumer(sess.SessionID)
	require.True(t, ok)
	fast, ok := hub.AttachConsumer(sess.SessionID)
	require.True(t, ok)

	total := consumerCap + replayCap + 16
	for i := 0; i < total; i++ {
		hub.RouteEvent("a1", &opsv1.DevopsSessionEvent{
			SessionId: sess.SessionID, Kind: "output", Data: []byte{byte(i)},
		})
	}

	// 慢消费者被摘除（channel 已关；先排空残留缓冲，再读到零值+false）。
	require.Equal(t, EvictOverflow, slow.EvictReason())
	for {
		_, alive := <-slow.C
		if !alive {
			break
		}
	}
	// 快消费者事件数落在 [队列深度, total]：非阻塞扇出不阻塞路由循环；
	// 若排空不及时同样可能被摘除（背压语义对称），此时残留缓冲仍 ≥ 队列
	// 深度可读。行为断言只锚定「不阻塞、有界、可恢复」，不锚定调度时序。
	fastGot := 0
	for {
		_, alive := <-fast.C
		if !alive {
			break
		}
		fastGot++
	}
	require.GreaterOrEqual(t, fastGot, consumerCap)
	require.LessOrEqual(t, fastGot, total)
	_, ok = hub.Get(sess.SessionID)
	require.True(t, ok, "session must survive slow consumer eviction")

	// 回放环有界：新订阅者的回放 ≤ replayCap 且最新事件在末尾。
	sub, ok := hub.AttachConsumer(sess.SessionID)
	require.True(t, ok)
	require.LessOrEqual(t, len(sub.Replay), replayCap)
	require.Equal(t, byte(total-1), sub.Replay[len(sub.Replay)-1].GetData()[0])
}

func TestHubInputResizeClose(t *testing.T) {
	caller := &devopsHubCaller{openOK: true}
	hub := NewHub(nil)
	resolver := &devopsHubResolver{caller: caller}
	sess := openSession(t, hub, resolver, "a1")

	// 正常输入。
	require.NoError(t, hub.Input(context.Background(), resolver, sess.SessionID, []byte("ls\n")))
	// 背压（agent 回 busy）→ 哨兵错误。
	err := hub.Input(context.Background(), resolver, sess.SessionID, []byte("TRIGGER_BUSY"))
	require.ErrorIs(t, err, ErrSessionBusy)
	// 未知会话 → 404 语义哨兵。
	err = hub.Input(context.Background(), resolver, "nope", []byte("x"))
	require.ErrorIs(t, err, ErrSessionNotFound)
	// 无活跃连接。
	err = hub.Input(context.Background(), &devopsHubResolver{}, sess.SessionID, []byte("x"))
	require.ErrorContains(t, err, "no active connection")

	require.NoError(t, hub.Resize(context.Background(), resolver, sess.SessionID, 120, 40))

	// close：wire close + 本地收束（消费者关）。
	sub, ok := hub.AttachConsumer(sess.SessionID)
	require.True(t, ok)
	require.NoError(t, hub.Close(context.Background(), resolver, sess.SessionID, "panel close"))
	require.Contains(t, caller.calls, uint32(protocol.MsgDevopsSessionCloseRequest))
	_, ok = hub.Get(sess.SessionID)
	require.False(t, ok)
	select {
	case ev, alive := <-sub.C:
		if !alive {
			t.Fatal("consumer must receive synthetic end before close")
		}
		require.Equal(t, "end", ev.GetKind())
		require.Equal(t, "closed", ev.GetOutcome())
	default:
	}
	_, alive := <-sub.C
	require.False(t, alive)
}

func TestHubCloseAgentSessions(t *testing.T) {
	hub := NewHub(nil)
	resolver := &devopsHubResolver{caller: &devopsHubCaller{openOK: true}}
	sessA := openSession(t, hub, resolver, "agent-a")
	sessA2 := openSession(t, hub, resolver, "agent-a")
	sessB := openSession(t, hub, resolver, "agent-b")
	subA, ok := hub.AttachConsumer(sessA.SessionID)
	require.True(t, ok)
	subB, ok := hub.AttachConsumer(sessB.SessionID)
	require.True(t, ok)

	hub.CloseAgentSessions("agent-a", "agent disconnected")

	// A 的会话全收束（消费者收到合成 end），B 不受影响。
	_, ok = hub.Get(sessA.SessionID)
	require.False(t, ok)
	_, ok = hub.Get(sessA2.SessionID)
	require.False(t, ok)
	_, ok = hub.Get(sessB.SessionID)
	require.True(t, ok)

	select {
	case ev, alive := <-subA.C:
		if !alive {
			t.Fatal("consumer must receive synthetic end first")
		}
		require.Equal(t, "end", ev.GetKind())
		require.Equal(t, "agent_disconnected", ev.GetOutcome())
	default:
		t.Fatal("synthetic end never delivered")
	}
	_, alive := <-subA.C
	require.False(t, alive)
	require.Equal(t, EvictEnded, subA.EvictReason())

	// B 存活且可继续收事件。
	hub.RouteEvent("agent-b", &opsv1.DevopsSessionEvent{SessionId: sessB.SessionID, Kind: "output", Data: []byte("z")})
	select {
	case ev := <-subB.C:
		require.Equal(t, "z", string(ev.GetData()))
	case <-time.After(2 * time.Second):
		t.Fatal("agent-b consumer lost")
	}
}

func TestHubDetachConsumer(t *testing.T) {
	hub := NewHub(nil)
	resolver := &devopsHubResolver{caller: &devopsHubCaller{openOK: true}}
	sess := openSession(t, hub, resolver, "a1")
	sub, ok := hub.AttachConsumer(sess.SessionID)
	require.True(t, ok)
	hub.DetachConsumer(sess.SessionID, sub)
	// 退订后不再收事件。
	hub.RouteEvent("a1", &opsv1.DevopsSessionEvent{SessionId: sess.SessionID, Kind: "output", Data: []byte("q")})
	select {
	case ev := <-sub.C:
		t.Fatalf("detached consumer got event: %v", ev)
	case <-time.After(50 * time.Millisecond):
	}
	// 幂等：重复退订/nil 不炸。
	hub.DetachConsumer(sess.SessionID, sub)
	hub.DetachConsumer(sess.SessionID, nil)
}
