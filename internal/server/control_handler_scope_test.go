package server

import (
	"context"
	"strings"
	"testing"

	"github.com/cuihairu/croupier/internal/platform/registry"
	agentv1 "github.com/cuihairu/croupier/pkg/pb/croupier/agent/v1"
)

func newScopeTestService(t *testing.T) *ControlService {
	t.Helper()
	return NewControlService(registry.NewStore(), nil)
}

// SDK/自定义游戏服注册时 scope 与 agent 不一致 → 业务层注册警告
// （/system/functions/warnings，研发视角）；不进运维告警（分层原则：
// 运维不关注业务接入问题）。
func TestValidateProviderScope_MismatchWritesRegistrationWarning(t *testing.T) {
	svc := newScopeTestService(t)
	req := &agentv1.RegisterRequest{AgentId: "agent-1", GameId: "demo-game", Env: "development"}
	p := &agentv1.AgentProcess{ServiceId: "svc-1", GameId: "other-game", Env: "development"}
	var warnings []string

	svc.validateProviderScope(context.Background(), req, p, &warnings)

	if len(warnings) != 1 || !strings.Contains(warnings[0], "game_id mismatch") {
		t.Fatalf("warnings = %v", warnings)
	}
	got := svc.registry.ListRegistrationWarnings(registry.RegistrationWarningFilter{
		AgentID: "agent-1", Code: "provider_scope_mismatch",
	})
	if len(got) != 1 {
		t.Fatalf("registration warnings = %v", got)
	}
	if got[0].GameID != "demo-game" || !strings.Contains(got[0].Message, "svc-1") {
		t.Fatalf("warning = %+v", got[0])
	}

	// 重复注册同 mismatch：Upsert 计数累加而非新增条目。
	var w2 []string
	svc.validateProviderScope(context.Background(), req, p, &w2)
	got = svc.registry.ListRegistrationWarnings(registry.RegistrationWarningFilter{
		AgentID: "agent-1", Code: "provider_scope_mismatch",
	})
	if len(got) != 1 || got[0].Count < 2 {
		t.Fatalf("expected count>=2, got %+v", got)
	}
}

// provider 修正配置后一致注册 → 业务层闭环：历史警告被清除。
func TestValidateProviderScope_MatchClearsWarning(t *testing.T) {
	svc := newScopeTestService(t)
	req := &agentv1.RegisterRequest{AgentId: "agent-1", GameId: "demo-game", Env: "development"}
	bad := &agentv1.AgentProcess{ServiceId: "svc-1", GameId: "other-game", Env: "development"}
	good := &agentv1.AgentProcess{ServiceId: "svc-1", GameId: "demo-game", Env: "development"}
	var warnings []string

	svc.validateProviderScope(context.Background(), req, bad, &warnings)
	if len(svc.registry.ListRegistrationWarnings(registry.RegistrationWarningFilter{AgentID: "agent-1"})) != 1 {
		t.Fatal("expected warning recorded")
	}

	var w2 []string
	svc.validateProviderScope(context.Background(), req, good, &w2)
	if len(w2) != 0 {
		t.Fatalf("no warnings expected on match, got %v", w2)
	}
	got := svc.registry.ListRegistrationWarnings(registry.RegistrationWarningFilter{AgentID: "agent-1"})
	if len(got) != 0 {
		t.Fatalf("warning should be cleared after fix, got %+v", got)
	}
}

// 硬切无兼容：provider 未上报 scope（空值）同样视为 mismatch——
// SDK 必须显式携带 game_id/env（作用域规范 §14）。
func TestValidateProviderScope_EmptyProviderScopeIsMismatch(t *testing.T) {
	svc := newScopeTestService(t)
	req := &agentv1.RegisterRequest{AgentId: "agent-1", GameId: "demo-game", Env: "development"}
	p := &agentv1.AgentProcess{ServiceId: "svc-1"}
	var warnings []string

	svc.validateProviderScope(context.Background(), req, p, &warnings)
	if len(warnings) != 1 || !strings.Contains(warnings[0], "game_id mismatch") {
		t.Fatalf("warnings = %v", warnings)
	}
	got := svc.registry.ListRegistrationWarnings(registry.RegistrationWarningFilter{
		AgentID: "agent-1", Code: "provider_scope_mismatch",
	})
	if len(got) != 1 || !strings.Contains(got[0].Message, "game_id mismatch") {
		t.Fatalf("registration warnings = %+v", got)
	}
}
