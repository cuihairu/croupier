package extension

import (
	"context"
	"strings"
	"testing"

	jwtutil "github.com/cuihairu/croupier/internal/security/jwtutil"
)

// TestService_AgentSyncPayload_MachineScope verifies ARCH-021: restricted
// machine identities are authorized by scope -- the request passes only when
// the agentID bound to the machine token matches the requested agentID, and
// cross-scope requests are always rejected; a request with no identity at all
// (neither machine nor admin) hits the RBAC branch and is rejected; the admin
// path keeps its behavior.
func TestService_AgentSyncPayload_MachineScope(t *testing.T) {
	db := setupIntegrationTestDB(t)
	svcCtx := setupExtensionTestContext(t, db)
	createTestCatalogData(t, svcCtx)
	s := NewService(svcCtx)

	// Machine identity: bound to test-agent-001.
	claims := &jwtutil.MachineClaims{AgentID: "test-agent-001", GameID: "demo", Env: "prod"}
	machineCtx := jwtutil.WithMachineIdentity(context.Background(), claims)

	// Same scope: allowed and the payload is built normally (the machine
	// identity path does not depend on admin RBAC).
	resp, err := s.AgentSyncPayload(machineCtx, "test-agent-001")
	if err != nil {
		t.Fatalf("matching scope should pass, got error: %v", err)
	}
	if resp == nil || resp.Payload == nil {
		t.Fatal("expected payload to be non-nil")
	}

	// Same scope with surrounding whitespace: still matches after trimming.
	if _, err := s.AgentSyncPayload(machineCtx, "  test-agent-001  "); err != nil {
		t.Fatalf("trimmed matching scope should pass, got error: %v", err)
	}

	// Cross scope: requesting another agent is rejected.
	if _, err := s.AgentSyncPayload(machineCtx, "other-agent"); err == nil {
		t.Fatal("cross-scope request should be rejected")
	} else if !strings.Contains(err.Error(), "scope") {
		t.Fatalf("expected scope mismatch error, got: %v", err)
	}

	// Empty agentID + machine identity: the machine branch grants
	// authorization, but parameter validation still rejects.
	if _, err := s.AgentSyncPayload(machineCtx, ""); err == nil {
		t.Fatal("empty agent id should be rejected")
	}

	// No identity at all: goes through the RBAC branch -> permission error
	// (rejection without credentials).
	if _, err := s.AgentSyncPayload(context.Background(), "test-agent-001"); err == nil {
		t.Fatal("unauthenticated request should be rejected")
	}

	// Admin identity: behavior unchanged (the payload is built normally once
	// RBAC read permission passes).
	adminCtx := setupAdminContext(t, svcCtx)
	resp, err = s.AgentSyncPayload(adminCtx, "test-agent-001")
	if err != nil {
		t.Fatalf("admin path should keep working, got error: %v", err)
	}
	if resp == nil || resp.Payload == nil {
		t.Fatal("expected payload to be non-nil for admin path")
	}
}
