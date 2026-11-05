package server

import (
	"context"
	"strings"
	"testing"
	"time"

	cluster2 "github.com/cuihairu/croupier/internal/cluster"
	"github.com/cuihairu/croupier/internal/platform/registry"
	agentv1 "github.com/cuihairu/croupier/pkg/pb/croupier/agent/v1"
	gosqlite "github.com/glebarez/sqlite"
	"gorm.io/gorm"
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

// 心跳自愈：本地会话丢失但 TCP 仍活 → 从归属表回读本实例 scope 重建
// 会话并重新 Claim（僵尸连接不再静默成功）。
func TestHandleHeartbeatRequest_SelfHealsMissingSession(t *testing.T) {
	gdb, err := gorm.Open(gosqlite.Open(t.TempDir()+"/owner.db"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := gdb.AutoMigrate(&cluster2.AgentOwnerRecord{}); err != nil {
		t.Fatal(err)
	}
	resolver := cluster2.NewDBOwnerResolverForSelf(gdb, time.Minute, "croupier-server")
	if err := resolver.EnsureTable(context.Background()); err != nil {
		t.Fatal(err)
	}

	svc := NewControlService(registry.NewStore(), nil)
	// 模拟本实例为 croupier-server 的持有者：
	if err := resolver.ClaimOwner(context.Background(), "agent-1", "demo_game", "development", "croupier-server", 10, nil); err != nil {
		t.Fatal(err)
	}
	svc.SetHeartbeatOwnerLookup(resolver)

	if _, err := svc.handleHeartbeatRequest(context.Background(), &agentv1.HeartbeatRequest{AgentId: "agent-1"}); err != nil {
		t.Fatal(err)
	}

	// 会话被重建且 scope 从归属表回读正确。
	sess := svc.registry.AgentsUnsafe()["agent-1"]
	if sess == nil {
		t.Fatal("session not re-seeded")
	}
	if sess.GameID != "demo_game" || sess.Env != "development" {
		t.Fatalf("reseeded session = %+v", sess)
	}

	// 第二次心跳：会话已存在，走正常续期路径（不再自愈分支）。
	hbBefore := sess.LastSeen
	time.Sleep(5 * time.Millisecond)
	if _, err := svc.handleHeartbeatRequest(context.Background(), &agentv1.HeartbeatRequest{AgentId: "agent-1"}); err != nil {
		t.Fatal(err)
	}
	if !sess.LastSeen.After(hbBefore) {
		t.Fatal("normal heartbeat path should refresh LastSeen")
	}
}

// 非本实例持有的 agent 心跳：不自愈（避免跨实例复活僵尸）。
func TestHandleHeartbeatRequest_ForeignClaimNoSelfHeal(t *testing.T) {
	gdb, err := gorm.Open(gosqlite.Open(t.TempDir()+"/owner.db"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := gdb.AutoMigrate(&cluster2.AgentOwnerRecord{}); err != nil {
		t.Fatal(err)
	}
	resolver := cluster2.NewDBOwnerResolver(gdb, time.Minute)
	if err := resolver.EnsureTable(context.Background()); err != nil {
		t.Fatal(err)
	}

	svc := NewControlService(registry.NewStore(), nil)
	svc.SetHeartbeatOwnerLookup(resolver)

	if _, err := svc.handleHeartbeatRequest(context.Background(), &agentv1.HeartbeatRequest{AgentId: "ghost"}); err != nil {
		t.Fatal(err)
	}
	if sess := svc.registry.AgentsUnsafe()["ghost"]; sess != nil {
		t.Fatalf("foreign claim must not self-heal, got %+v", sess)
	}
}

// 三方对账（agent 视角）：注册响应回传 instanceId；agent 自报 owner
// 存入 session labels 供 nodes 视图对账。
func TestRegisterResponseCarriesInstanceID(t *testing.T) {
	svc := newScopeTestService(t)
	svc.SetClusterInstanceID("croupier-server")

	resp, err := svc.handleRegisterRequest(context.Background(), &agentv1.RegisterRequest{
		AgentId: "agent-1", GameId: "g", Env: "e",
	}, "10.0.0.1:12345")
	if err != nil {
		t.Fatal(err)
	}
	if resp.GetInstanceId() != "croupier-server" {
		t.Fatalf("instanceId = %q", resp.GetInstanceId())
	}
}

func TestHeartbeatStoresReportedOwner(t *testing.T) {
	svc := newScopeTestService(t)
	// 先注册（建立会话），再心跳携带自报 owner。
	if _, err := svc.handleRegisterRequest(context.Background(), &agentv1.RegisterRequest{
		AgentId: "agent-1", GameId: "g", Env: "e",
	}, "10.0.0.1:12345"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.handleHeartbeatRequest(context.Background(), &agentv1.HeartbeatRequest{
		AgentId: "agent-1", OwnerInstanceId: "croupier-server2",
	}); err != nil {
		t.Fatal(err)
	}
	sess := svc.registry.AgentsUnsafe()["agent-1"]
	if sess == nil || sess.Labels["reportedOwner"] != "croupier-server2" {
		t.Fatalf("reportedOwner not stored: %+v", sess)
	}
}

// AGENT-001 scope 隔离矩阵：同 AgentID 已绑定到某 (game_id, env) 后，
// 再次以不同 scope 注册必须拒绝——防止同一 agent 身份跨 scope 指派导致
// 路由/契约物化串扰（同 ID 冲突）。同 scope 重注册（agent 重启/重连）
// 应正常通过。
func TestHandleRegisterRequest_SameAgentIDCrossScopeRejected(t *testing.T) {
	svc := newScopeTestService(t)

	if _, err := svc.handleRegisterRequest(context.Background(), &agentv1.RegisterRequest{
		AgentId: "agent-1", GameId: "demo-game", Env: "development",
	}, "10.0.0.1:12345"); err != nil {
		t.Fatalf("first register in scope A should succeed: %v", err)
	}

	// 同 ID 换 env → 拒绝。
	if _, err := svc.handleRegisterRequest(context.Background(), &agentv1.RegisterRequest{
		AgentId: "agent-1", GameId: "demo-game", Env: "production",
	}, "10.0.0.1:12346"); err == nil {
		t.Fatal("expected cross-env re-register to be rejected")
	} else if !strings.Contains(err.Error(), "already bound to game_id") {
		t.Fatalf("unexpected error = %v", err)
	}

	// 同 ID 换 game → 拒绝。
	if _, err := svc.handleRegisterRequest(context.Background(), &agentv1.RegisterRequest{
		AgentId: "agent-1", GameId: "other-game", Env: "development",
	}, "10.0.0.1:12347"); err == nil {
		t.Fatal("expected cross-game re-register to be rejected")
	} else if !strings.Contains(err.Error(), "already bound to game_id") {
		t.Fatalf("unexpected error = %v", err)
	}

	// 同 ID 同 scope（重连/重启）→ 通过。
	if _, err := svc.handleRegisterRequest(context.Background(), &agentv1.RegisterRequest{
		AgentId: "agent-1", GameId: "demo-game", Env: "development",
	}, "10.0.0.1:12348"); err != nil {
		t.Fatalf("same-scope re-register should succeed: %v", err)
	}

	// 原 scope 会话保留，未被跨 scope 尝试污染。
	sess := svc.registry.AgentsUnsafe()["agent-1"]
	if sess == nil {
		t.Fatal("session missing")
	}
	if sess.GameID != "demo-game" || sess.Env != "development" {
		t.Fatalf("session scope polluted = %+v", sess)
	}
}

// 0063 审核发现：注册入口未限制 agent_id 长度，而 agent_id 列均按
// size:64 建表，PostgreSQL 等严格库会在写入/迁移时拒绝超长 ID。注册
// 必须在入口拒绝超 64 字节的 agent_id，确保任何 store 与 0063 迁移
// 不会遇到不可收敛的超长行。
func TestHandleRegisterRequest_OverlongAgentIDRejected(t *testing.T) {
	svc := newScopeTestService(t)

	long := strings.Repeat("a", maxAgentIDLen+1)
	if _, err := svc.handleRegisterRequest(context.Background(), &agentv1.RegisterRequest{
		AgentId: long, GameId: "g", Env: "e",
	}, "10.0.0.1:12345"); err == nil {
		t.Fatal("expected over-long agent_id registration to be rejected")
	} else if !strings.Contains(err.Error(), "agent_id too long") {
		t.Fatalf("unexpected error = %v", err)
	}

	// 恰好 64 字节边界允许。
	boundary := strings.Repeat("a", maxAgentIDLen)
	if _, err := svc.handleRegisterRequest(context.Background(), &agentv1.RegisterRequest{
		AgentId: boundary, GameId: "g", Env: "e",
	}, "10.0.0.1:12346"); err != nil {
		t.Fatalf("boundary-length agent_id should register: %v", err)
	}
}
