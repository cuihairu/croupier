package extension

import (
	"context"
	"strings"
	"testing"

	jwtutil "github.com/cuihairu/croupier/internal/security/jwtutil"
)

// TestService_AgentSyncPayload_MachineScope 验证 ARCH-021：受限机器身份走
// scope 授权——机器 token 绑定的 agentID 与请求的 agentID 一致才放行，
// 跨 scope 一律拒绝；无任何身份（既非机器也非管理员）走 RBAC 分支被拒；
// 管理员路径行为不变。
func TestService_AgentSyncPayload_MachineScope(t *testing.T) {
	db := setupIntegrationTestDB(t)
	svcCtx := setupExtensionTestContext(t, db)
	createTestCatalogData(t, svcCtx)
	s := NewService(svcCtx)

	// 机器身份：绑定 test-agent-001。
	claims := &jwtutil.MachineClaims{AgentID: "test-agent-001", GameID: "demo", Env: "prod"}
	machineCtx := jwtutil.WithMachineIdentity(context.Background(), claims)

	// 同 scope：放行且 payload 正常构建（机器身份路径不依赖管理员 RBAC）。
	resp, err := s.AgentSyncPayload(machineCtx, "test-agent-001")
	if err != nil {
		t.Fatalf("matching scope should pass, got error: %v", err)
	}
	if resp == nil || resp.Payload == nil {
		t.Fatal("expected payload to be non-nil")
	}

	// 同 scope + 前后空白：归一后仍匹配。
	if _, err := s.AgentSyncPayload(machineCtx, "  test-agent-001  "); err != nil {
		t.Fatalf("trimmed matching scope should pass, got error: %v", err)
	}

	// 跨 scope：请求其它 agent 即拒绝。
	if _, err := s.AgentSyncPayload(machineCtx, "other-agent"); err == nil {
		t.Fatal("cross-scope request should be rejected")
	} else if !strings.Contains(err.Error(), "scope") {
		t.Fatalf("expected scope mismatch error, got: %v", err)
	}

	// 空 agentID + 机器身份：机器分支放行授权，但参数校验仍拒绝。
	if _, err := s.AgentSyncPayload(machineCtx, ""); err == nil {
		t.Fatal("empty agent id should be rejected")
	}

	// 无任何身份：走 RBAC 分支 → 无权限错误（无凭据拒绝）。
	if _, err := s.AgentSyncPayload(context.Background(), "test-agent-001"); err == nil {
		t.Fatal("unauthenticated request should be rejected")
	}

	// 管理员身份：行为不变（RBAC 读权限通过后正常构建 payload）。
	adminCtx := setupAdminContext(t, svcCtx)
	resp, err = s.AgentSyncPayload(adminCtx, "test-agent-001")
	if err != nil {
		t.Fatalf("admin path should keep working, got error: %v", err)
	}
	if resp == nil || resp.Payload == nil {
		t.Fatal("expected payload to be non-nil for admin path")
	}
}
