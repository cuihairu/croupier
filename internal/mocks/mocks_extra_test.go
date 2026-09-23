// 覆盖目标：MockServiceContext 的 SetConfig。
// （原 MockGRPCClient 用例随 gRPC 时代遗留 mock 一并清除，
// 见 docs/grpc-investigation.md。）
package mocks

import (
	"testing"
)

func TestMockServiceContext_SetConfig(t *testing.T) {
	ctx := NewMockServiceContext()

	if ctx.Config.AgentID != "test-agent-001" {
		t.Errorf("default Config.AgentID = %q, want test-agent-001", ctx.Config.AgentID)
	}

	newConfig := &MockConfig{
		AgentID: "agent-x",
		GameID:  "game-y",
		Env:     "prod",
	}
	ctx.SetConfig(newConfig)

	if ctx.Config != newConfig {
		t.Errorf("SetConfig() did not replace Config: %+v", ctx.Config)
	}
	if ctx.Config.AgentID != "agent-x" || ctx.Config.Env != "prod" {
		t.Errorf("Config = %+v, want AgentID=agent-x Env=prod", ctx.Config)
	}
}
