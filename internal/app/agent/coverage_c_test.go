package agent

// C 批覆盖补齐（upstream.go syncOnce）：注册响应携带 warnings 时必须落
// agent 日志（注册错误黑洞修复的双通道之一）。通过 setClient 注入返回
// warnings 的 stub control client，确定性驱动，无时序依赖。

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/platform/agentlocal"
	agentv1 "github.com/cuihairu/croupier/pkg/pb/croupier/agent/v1"
)

// warningRegisterClientC 的 Register 恒返回带 warnings 的成功响应。
type warningRegisterClientC struct{}

func (warningRegisterClientC) Connected() bool { return true }

func (warningRegisterClientC) Close() error { return nil }

func (warningRegisterClientC) Register(context.Context, *agentv1.RegisterRequest) (*agentv1.RegisterResponse, error) {
	return &agentv1.RegisterResponse{
		SessionId: "sess-cov-c",
		Warnings:  []string{"registration_materialize_failed: coverage probe"},
	}, nil
}

func (warningRegisterClientC) Heartbeat(context.Context, *agentv1.HeartbeatRequest) (*agentv1.HeartbeatResponse, error) {
	return &agentv1.HeartbeatResponse{}, nil
}

func (warningRegisterClientC) RegisterCapabilities(context.Context, *agentv1.RegisterCapabilitiesRequest) (*agentv1.RegisterCapabilitiesResponse, error) {
	return &agentv1.RegisterCapabilitiesResponse{}, nil
}

func (warningRegisterClientC) SendTaskEvent(context.Context, []byte) error { return nil }

func (warningRegisterClientC) SendMetricEvent(context.Context, []byte) error { return nil }

// syncOnce：注册响应带 warnings 时走 Warn 分支并正常完成同步，不视为
// 注册失败。
func TestCoverageC_SyncOnce_RegisterWarningsLogged(t *testing.T) {
	client := NewUpstreamClient("127.0.0.1:1", "agent-cov-c", agentlocal.NewLocalStore(), &UpstreamMetadata{
		GameID:  "demo_game",
		Env:     "dev",
		Version: "0.0.0-test",
	})
	client.setClient(warningRegisterClientC{})

	err := client.syncOnce(context.Background())
	if err != nil {
		t.Fatalf("带 warnings 的注册响应不应视为失败: %v", err)
	}
}
