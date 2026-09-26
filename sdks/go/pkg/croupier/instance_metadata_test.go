// 实例元数据回归：ClientConfig.InstanceMetadata 必须穿过 ManagerConfig 交接
// 链到达 TCP 管理器——线上曾因 client.Connect 手工拷贝 ManagerConfig 漏字段，
// 元数据在 SDK 内部被吞（API 侧 instances 一律 meta=None）。
package croupier

import (
	"context"
	"testing"
	"time"

	sdkv1 "github.com/cuihairu/croupier/sdks/go/pkg/pb/croupier/sdk/v1"
	"google.golang.org/protobuf/proto"
)

func TestNewManagerCarriesInstanceMetadata(t *testing.T) {
	want := map[string]string{"serverId": "s1", "pod": "claude-e2e"}
	m, err := NewManager(ManagerConfig{
		AgentAddr:        "127.0.0.1:1",
		InstanceMetadata: want,
	}, nil)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	tm, ok := m.(*TCPManager)
	if !ok {
		t.Fatalf("NewManager() returned %T, want *TCPManager", m)
	}
	if tm.config.InstanceMetadata["serverId"] != "s1" || tm.config.InstanceMetadata["pod"] != "claude-e2e" {
		t.Fatalf("InstanceMetadata lost in NewManager handoff: got %v", tm.config.InstanceMetadata)
	}
}

func TestNewTCPManagerCarriesInstanceMetadata(t *testing.T) {
	want := map[string]string{"serverId": "s1"}
	m, err := NewTCPManager(ClientConfig{
		AgentAddr:        "127.0.0.1:1",
		InstanceMetadata: want,
	}, nil)
	if err != nil {
		t.Fatalf("NewTCPManager() error = %v", err)
	}
	tm := m.(*TCPManager)
	if tm.config.InstanceMetadata["serverId"] != "s1" {
		t.Fatalf("InstanceMetadata lost in NewTCPManager: got %v", tm.config.InstanceMetadata)
	}
}

// 帧级回归：经 NewManager 交接链构造的 Manager，其注册帧必须携带元数据。
func TestRegisterFrameCarriesInstanceMetadata(t *testing.T) {
	var got *sdkv1.ProviderConnectRequest
	handler := func(msgID uint32, reqID uint32, body []byte) (uint32, []byte, bool) {
		if msgID == 0x050101 { // protocol.MsgProviderConnectRequest
			req := &sdkv1.ProviderConnectRequest{}
			if err := proto.Unmarshal(body, req); err != nil {
				t.Fatalf("unmarshal ProviderConnectRequest: %v", err)
			}
			got = req
			resp, _ := proto.Marshal(&sdkv1.ProviderConnectResponse{SessionId: "sess-meta-frame"})
			return 0x050102, resp, true
		}
		return 0, nil, false
	}
	agent := startFakeAgent(t, "127.0.0.1:0", handler)

	mgr, err := NewManager(ManagerConfig{
		AgentAddr:        agent.addr(),
		Insecure:         true,
		InstanceMetadata: map[string]string{"serverId": "s1", "pod": "claude-e2e"},
	}, nil)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	tcp := mgr.(*TCPManager)
	if err := tcp.Connect(context.Background()); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer tcp.Disconnect()

	done := make(chan struct{})
	go func() {
		defer close(done)
		if _, err := tcp.RegisterWithAgent(context.Background(), "svc-meta", "1.0.0", nil); err != nil {
			t.Errorf("RegisterWithAgent: %v", err)
		}
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("RegisterWithAgent timeout")
	}

	if got == nil {
		t.Fatal("agent never received ProviderConnectRequest")
	}
	if got.GetMetadata()["serverId"] != "s1" || got.GetMetadata()["pod"] != "claude-e2e" {
		t.Fatalf("register frame metadata = %v, want serverId/pod", got.GetMetadata())
	}
}

// 重连路径回归：断线重连经 reconnectWithBackoff 重建 Manager 时同样必须携带
// 实例元数据——线上 demo 在 agent 重启后走该路径恢复会话，元数据在此第三次
// 被吞（Connect 首连正常、重连后实例 meta 变空）。
func TestReconnectCarriesInstanceMetadata(t *testing.T) {
	frames := make(chan *sdkv1.ProviderConnectRequest, 4)
	handler := func(msgID uint32, _ uint32, body []byte) (uint32, []byte, bool) {
		if msgID == 0x050101 { // protocol.MsgProviderConnectRequest
			req := &sdkv1.ProviderConnectRequest{}
			if err := proto.Unmarshal(body, req); err != nil {
				t.Errorf("unmarshal ProviderConnectRequest: %v", err)
				return 0, nil, false
			}
			frames <- req
			resp, _ := proto.Marshal(&sdkv1.ProviderConnectResponse{SessionId: "sess-reconnect"})
			return 0x050102, resp, true
		}
		return 0, nil, false
	}
	agent := startFakeAgent(t, "127.0.0.1:0", handler)

	client := NewClient(&ClientConfig{
		AgentAddr:        agent.addr(),
		Insecure:         true,
		InstanceMetadata: map[string]string{"serverId": "s1"},
		Reconnect: &ReconnectConfig{
			Enabled:           true,
			InitialDelayMs:    50,
			MaxDelayMs:        200,
			BackoffMultiplier: 2,
			MaxAttempts:       10,
		},
	})
	defer client.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	// Serve 进入断线监听循环，重连由它驱动。
	go func() { _ = client.Serve(ctx) }()

	select {
	case req := <-frames:
		if req.GetMetadata()["serverId"] != "s1" {
			t.Fatalf("first connect metadata = %v, want serverId=s1", req.GetMetadata())
		}
	case <-time.After(5 * time.Second):
		t.Fatal("first ProviderConnectRequest never arrived")
	}

	// 从 fake agent 侧掐断连接，触发 SDK 断线重连。
	agent.mu.Lock()
	for conn := range agent.conns {
		_ = conn.Close()
	}
	agent.mu.Unlock()

	select {
	case req := <-frames:
		if req.GetMetadata()["serverId"] != "s1" {
			t.Fatalf("reconnect frame metadata lost: %v", req.GetMetadata())
		}
	case <-time.After(10 * time.Second):
		t.Fatal("reconnect ProviderConnectRequest never arrived")
	}
}
