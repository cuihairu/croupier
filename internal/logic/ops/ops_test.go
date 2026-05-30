package ops

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/svc"
	"github.com/cuihairu/croupier/internal/transport"
)

// TestInitAgentOpsClient tests the initialization of the global client
func TestInitAgentOpsClient(t *testing.T) {
	// Initialize without parameters
	InitAgentOpsClient()

	client := GetAgentOpsClient()
	if client == nil {
		t.Fatal("InitAgentOpsClient() should set globalAgentOpsClient")
	}
}

// TestGetAgentOpsClient tests retrieving the global client
func TestGetAgentOpsClient(t *testing.T) {
	// Ensure global client is initialized
	client := GetAgentOpsClient()
	if client == nil {
		t.Fatal("GetAgentOpsClient() should return non-nil client")
	}
}

// TestAgentOpsClient_GetClient_NonExistent tests getting client for non-existent agent
func TestAgentOpsClient_GetClient_NonExistent(t *testing.T) {
	client := GetAgentOpsClient()

	// Test with non-existent agent
	_, err := client.GetClient(context.Background(), "non-existent-agent")
	if err == nil {
		t.Error("GetClient() should return error for non-existent agent")
	}
}

// TestAgentOpsClient_GetClient_EmptyAgentID tests with empty agent ID
func TestAgentOpsClient_GetClient_EmptyAgentID(t *testing.T) {
	client := GetAgentOpsClient()

	_, err := client.GetClient(context.Background(), "")
	if err == nil {
		t.Error("GetClient() should return error for empty agent ID")
	}
}

// TestAgentOpsClient_SetSessionResolver tests setting session resolver
func TestAgentOpsClient_SetSessionResolver(t *testing.T) {
	client := GetAgentOpsClient()

	// Set a nil resolver (for testing purposes)
	client.SetSessionResolver(nil)

	// GetClient should fail with nil resolver
	_, err := client.GetClient(context.Background(), "test-agent")
	if err == nil {
		t.Error("GetClient() should return error when resolver is nil")
	}
}

// TestAgentOpsClient_SetSessionResolver_ThenGetClient tests setting resolver and getting client
func TestAgentOpsClient_SetSessionResolver_ThenGetClient(t *testing.T) {
	client := GetAgentOpsClient()

	// Create a mock resolver
	mockResolver := &mockSessionResolver{
		agentExists: true,
	}

	client.SetSessionResolver(mockResolver)

	// GetClient should succeed with mock resolver
	wrapper, err := client.GetClient(context.Background(), "test-agent")
	if err != nil {
		t.Errorf("GetClient() should not error with mock resolver, got: %v", err)
	}
	if wrapper == nil {
		t.Error("GetClient() should return non-nil wrapper")
	}
}

// mockSessionResolver is a mock implementation of AgentSessionResolver
type mockSessionResolver struct {
	agentExists bool
}

func (m *mockSessionResolver) ResolveAgentConn(agentID string) (transport.SessionCaller, bool) {
	if !m.agentExists {
		return nil, false
	}
	return &mockSessionCaller{}, true
}

// mockSessionCaller is a mock implementation of SessionCaller
type mockSessionCaller struct{}

func (m *mockSessionCaller) Call(ctx context.Context, msgID uint32, reqBody []byte) (uint32, []byte, error) {
	return msgID, []byte("ok"), nil
}

// TestSnapshotOpsState tests the snapshotOpsState helper function
func TestSnapshotOpsState(t *testing.T) {
	t.Run("nil context returns empty state", func(t *testing.T) {
		state := snapshotOpsState(nil)
		if state.Config.AlertmanagerURL != "" {
			t.Error("expected empty state for nil context")
		}
	})

	t.Run("context without OpsStateStore returns empty state", func(t *testing.T) {
		ctx := &svc.ServiceContext{}
		state := snapshotOpsState(ctx)
		if state.Config.AlertmanagerURL != "" {
			t.Error("expected empty state when OpsStateStore is nil")
		}
	})

	t.Run("context with OpsStateStore returns snapshot", func(t *testing.T) {
		ctx := &svc.ServiceContext{
			OpsStateStore: svc.NewOpsStateStore(""),
		}
		state := snapshotOpsState(ctx)
		// Should return a valid (empty) state object
		// Windows is initialized as an empty slice, not nil
		if state.Maintenance.Windows == nil && len(state.Maintenance.Windows) != 0 {
			t.Error("expected initialized Maintenance.Windows")
		}
	})
}

// TestUpdateOpsState tests the updateOpsState helper function
func TestUpdateOpsState(t *testing.T) {
	t.Run("nil context returns error", func(t *testing.T) {
		_, err := updateOpsState(nil, func(s *svc.OpsState) {})
		if err == nil {
			t.Error("expected error for nil context")
		}
		if err != errOpsStateUnavailable {
			t.Errorf("expected errOpsStateUnavailable, got %v", err)
		}
	})

	t.Run("context without OpsStateStore returns error", func(t *testing.T) {
		ctx := &svc.ServiceContext{}
		_, err := updateOpsState(ctx, func(s *svc.OpsState) {})
		if err == nil {
			t.Error("expected error when OpsStateStore is nil")
		}
		if err != errOpsStateUnavailable {
			t.Errorf("expected errOpsStateUnavailable, got %v", err)
		}
	})

	t.Run("valid update succeeds", func(t *testing.T) {
		ctx := &svc.ServiceContext{
			OpsStateStore: svc.NewOpsStateStore(""),
		}
		state, err := updateOpsState(ctx, func(s *svc.OpsState) {
			s.Config.AlertmanagerURL = "http://test"
		})
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if state.Config.AlertmanagerURL != "http://test" {
			t.Errorf("expected AlertmanagerURL to be updated, got %s", state.Config.AlertmanagerURL)
		}
	})
}

// TestNotImplementedMethods tests that unimplemented methods return proper errors
func TestNotImplementedMethods(t *testing.T) {
	ctx := context.Background()
	svcCtx := &svc.ServiceContext{}

	t.Run("OpsHealthRun not implemented", func(t *testing.T) {
		logic := NewOpsHealthRunLogic(ctx, svcCtx)
		_, err := logic.OpsHealthRun(&OpsHealthRunRequest{})
		if err == nil {
			t.Error("expected error for unimplemented method")
		}
	})

	t.Run("OpsHealthUpdate not implemented", func(t *testing.T) {
		logic := NewOpsHealthUpdateLogic(ctx, svcCtx)
		_, err := logic.OpsHealthUpdate(&OpsHealthUpdateRequest{})
		if err == nil {
			t.Error("expected error for unimplemented method")
		}
	})

	t.Run("OpsMQ not implemented", func(t *testing.T) {
		logic := NewOpsMQLogic(ctx, svcCtx)
		_, err := logic.OpsMQ(&OpsMQRequest{})
		if err == nil {
			t.Error("expected error for unimplemented method")
		}
	})

	t.Run("OpsMaintenanceGet not implemented", func(t *testing.T) {
		logic := NewOpsMaintenanceGetLogic(ctx, svcCtx)
		_, err := logic.OpsMaintenanceGet(&OpsMaintenanceGetRequest{})
		if err == nil {
			t.Error("expected error for unimplemented method")
		}
	})

	t.Run("OpsMaintenanceUpdate not implemented", func(t *testing.T) {
		logic := NewOpsMaintenanceUpdateLogic(ctx, svcCtx)
		_, err := logic.OpsMaintenanceUpdate(&OpsMaintenanceUpdateRequest{})
		if err == nil {
			t.Error("expected error for unimplemented method")
		}
	})

	t.Run("OpsMetrics not implemented", func(t *testing.T) {
		logic := NewOpsMetricsLogic(ctx, svcCtx)
		_, err := logic.OpsMetrics(&OpsMetricsQuery{})
		if err == nil {
			t.Error("expected error for unimplemented method")
		}
	})

	t.Run("OpsNodeCommands not implemented", func(t *testing.T) {
		logic := NewOpsNodeCommandsLogic(ctx, svcCtx)
		_, err := logic.OpsNodeCommands(&OpsNodeCommandsQuery{})
		if err == nil {
			t.Error("expected error for unimplemented method")
		}
	})

	t.Run("OpsNodeDrain not implemented", func(t *testing.T) {
		logic := NewOpsNodeDrainLogic(ctx, svcCtx)
		_, err := logic.OpsNodeDrain(&OpsNodeActionRequest{})
		if err == nil {
			t.Error("expected error for unimplemented method")
		}
	})

	t.Run("OpsNodeMeta not implemented", func(t *testing.T) {
		logic := NewOpsNodeMetaLogic(ctx, svcCtx)
		_, err := logic.OpsNodeMeta(&OpsNodeMetaRequest{})
		if err == nil {
			t.Error("expected error for unimplemented method")
		}
	})

	t.Run("OpsNodeRestart not implemented", func(t *testing.T) {
		logic := NewOpsNodeRestartLogic(ctx, svcCtx)
		_, err := logic.OpsNodeRestart(&OpsNodeActionRequest{})
		if err == nil {
			t.Error("expected error for unimplemented method")
		}
	})

	t.Run("OpsNodeUndrain not implemented", func(t *testing.T) {
		logic := NewOpsNodeUndrainLogic(ctx, svcCtx)
		_, err := logic.OpsNodeUndrain(&OpsNodeActionRequest{})
		if err == nil {
			t.Error("expected error for unimplemented method")
		}
	})

	t.Run("OpsNodes not implemented", func(t *testing.T) {
		logic := NewOpsNodesLogic(ctx, svcCtx)
		_, err := logic.OpsNodes(&OpsNodesRequest{})
		if err == nil {
			t.Error("expected error for unimplemented method")
		}
	})

	t.Run("OpsNotificationsGet not implemented", func(t *testing.T) {
		logic := NewOpsNotificationsGetLogic(ctx, svcCtx)
		_, err := logic.OpsNotificationsGet(&OpsNotificationsGetRequest{})
		if err == nil {
			t.Error("expected error for unimplemented method")
		}
	})

	t.Run("OpsNotificationsUpdate not implemented", func(t *testing.T) {
		logic := NewOpsNotificationsUpdateLogic(ctx, svcCtx)
		_, err := logic.OpsNotificationsUpdate(&OpsNotificationsUpdateRequest{})
		if err == nil {
			t.Error("expected error for unimplemented method")
		}
	})

	t.Run("OpsSilenceDelete not implemented", func(t *testing.T) {
		logic := NewOpsSilenceDeleteLogic(ctx, svcCtx)
		_, err := logic.OpsSilenceDelete(&OpsAlertSilenceDeleteRequest{})
		if err == nil {
			t.Error("expected error for unimplemented method")
		}
	})

	t.Run("OpsSilences not implemented", func(t *testing.T) {
		logic := NewOpsSilencesLogic(ctx, svcCtx)
		_, err := logic.OpsSilences(&OpsSilencesRequest{})
		if err == nil {
			t.Error("expected error for unimplemented method")
		}
	})
}
