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
