package transport

import (
	"context"
	"net"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cuihairu/croupier/sdks/go/pkg/croupier/protocol"
)

// 双车道测试：业务队列打满（fail-fast busy）时，心跳（控制车道）仍被
// 处理并应答——会话存活不受业务过载影响。

// blockingBizOnlyInbound：业务请求阻塞，控制请求即时应答。
func blockingBizOnlyInbound(ctx context.Context, msgID uint32, reqID uint32, body []byte) ([]byte, error) {
	if protocol.IsControlRequest(msgID) {
		return []byte(`{"ok":true}`), nil
	}
	time.Sleep(2 * time.Second)
	return []byte(`{}`), nil
}

func TestTCPClient_DualLane_HeartbeatSurvivesBusinessSaturation(t *testing.T) {
	t.Parallel()

	agentListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer agentListener.Close()

	var hbHandled atomic.Int64
	heartbeatAnswered := make(chan struct{}, 1)

	go func() {
		conn, err := agentListener.Accept()
		if err != nil {
			return
		}
		a := &agentConn{conn: conn}
		defer conn.Close()

		// 打满业务车道：1 worker（阻塞 2s）+ 队列 1 = 容量 2。
		a.send(protocol.MsgInvokeRequest, 3001, []byte(`{}`))
		a.send(protocol.MsgInvokeRequest, 3002, []byte(`{}`))
		// 第 3 个业务请求：必收到 fail-fast busy。
		a.send(protocol.MsgInvokeRequest, 3003, []byte(`{}`))

		// 业务饱和后立刻发心跳：控制车道必须照常处理并应答。
		a.send(protocol.MsgProviderHeartbeatRequest, 3004, []byte(`{}`))

		deadline := time.Now().Add(3 * time.Second)
		sawBusy, sawHeartbeat := false, false
		for !(sawBusy && sawHeartbeat) {
			if time.Now().After(deadline) {
				heartbeatAnswered <- struct{}{}
				return
			}
			text, err := a.readUntil("}", 500*time.Millisecond)
			if err != nil {
				continue
			}
			if strings.Contains(text, "inbound queue full") {
				sawBusy = true
			}
			if strings.Contains(text, `"ok":true`) {
				sawHeartbeat = true
			}
		}
		heartbeatAnswered <- struct{}{}
	}()

	client, err := NewTCPClient(&Config{
		Address:     agentListener.Addr().String(),
		Insecure:    true,
		DialTimeout: 5 * time.Second,
		InboundHandler: func(ctx context.Context, msgID uint32, reqID uint32, body []byte) ([]byte, error) {
			if protocol.IsControlRequest(msgID) {
				hbHandled.Add(1)
				return []byte(`{"ok":true}`), nil
			}
			time.Sleep(2 * time.Second)
			return []byte(`{}`), nil
		},
		InboundWorkers: 1,
		InboundQLen:    1,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	select {
	case <-heartbeatAnswered:
	case <-time.After(5 * time.Second):
		t.Fatal("agent peer did not finish")
	}

	if hbHandled.Load() == 0 {
		t.Fatal("heartbeat was not handled while business lane saturated — control lane must stay reachable")
	}
}
