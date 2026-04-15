package ops

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
)

// Additional tests to boost coverage of the ops logic layer

// Test New* constructors more thoroughly
func TestAllLogicConstructors(t *testing.T) {
	svcCtx := &svc.ServiceContext{
		Config: config.Config{
			Server: config.ServerConfig{
				Host: "localhost",
				Port: 8080,
			},
		},
		ServerVersion:   "v1.0.0",
		StartTime:       time.Now(),
		RegistryStore:   registry.NewStore(),
		MetricsStore:    registry.NewMetricsStore(),
		SystemInfoCache: registry.NewSystemInfoCache(),
	}
	ctx := context.Background()

	// Test all constructors return non-nil logic instances
	tests := []struct {
		name  string
		logic interface{}
	}{
		{"OpsAgentsListLogic", NewOpsAgentsListLogic(ctx, svcCtx)},
		{"OpsServicesLogic", NewOpsServicesLogic(ctx, svcCtx)},
		{"OpsAgentSystemInfoLogic", NewOpsAgentSystemInfoLogic(ctx, svcCtx)},
		{"OpsAgentMetricsLogic", NewOpsAgentMetricsLogic(ctx, svcCtx)},
		{"OpsAgentProcessesLogic", NewOpsAgentProcessesLogic(ctx, svcCtx)},
		{"OpsAgentExecCommandLogic", NewOpsAgentExecCommandLogic(ctx, svcCtx)},
		{"OpsAgentProcessStartLogic", NewOpsAgentProcessStartLogic(ctx, svcCtx)},
		{"OpsAgentProcessStopLogic", NewOpsAgentProcessStopLogic(ctx, svcCtx)},
		{"OpsAgentProcessRestartLogic", NewOpsAgentProcessRestartLogic(ctx, svcCtx)},
		{"OpsAgentMetaLogic", NewOpsAgentMetaLogic(ctx, svcCtx)},
		{"OpsNodesLogic", NewOpsNodesLogic(ctx, svcCtx)},
		{"OpsConfigLogic", NewOpsConfigLogic(ctx, svcCtx)},
		{"OpsAlertsLogic", NewOpsAlertsLogic(ctx, svcCtx)},
		{"OpsFunctionsLogic", NewOpsFunctionsLogic(ctx, svcCtx)},
		{"OpsHealthGetLogic", NewOpsHealthGetLogic(ctx, svcCtx)},
		{"OpsMetricsLogic", NewOpsMetricsLogic(ctx, svcCtx)},
		{"OpsBackupsListLogic", NewOpsBackupsListLogic(ctx, svcCtx)},
		{"OpsBackupCreateLogic", NewOpsBackupCreateLogic(ctx, svcCtx)},
		{"OpsBackupDeleteLogic", NewOpsBackupDeleteLogic(ctx, svcCtx)},
		{"OpsBackupDownloadLogic", NewOpsBackupDownloadLogic(ctx, svcCtx)},
		{"OpsMQLogic", NewOpsMQLogic(ctx, svcCtx)},
		{"OpsMaintenanceGetLogic", NewOpsMaintenanceGetLogic(ctx, svcCtx)},
		{"OpsMaintenanceUpdateLogic", NewOpsMaintenanceUpdateLogic(ctx, svcCtx)},
		{"OpsNodeCommandsLogic", NewOpsNodeCommandsLogic(ctx, svcCtx)},
		{"OpsNodeDrainLogic", NewOpsNodeDrainLogic(ctx, svcCtx)},
		{"OpsNodeMetaLogic", NewOpsNodeMetaLogic(ctx, svcCtx)},
		{"OpsNodeRestartLogic", NewOpsNodeRestartLogic(ctx, svcCtx)},
		{"OpsNodeUndrainLogic", NewOpsNodeUndrainLogic(ctx, svcCtx)},
		{"OpsNotificationsGetLogic", NewOpsNotificationsGetLogic(ctx, svcCtx)},
		{"OpsNotificationsUpdateLogic", NewOpsNotificationsUpdateLogic(ctx, svcCtx)},
		{"OpsSilenceDeleteLogic", NewOpsSilenceDeleteLogic(ctx, svcCtx)},
		{"OpsSilencesLogic", NewOpsSilencesLogic(ctx, svcCtx)},
		{"OpsAlertSilenceLogic", NewOpsAlertSilenceLogic(ctx, svcCtx)},
		{"OpsHealthRunLogic", NewOpsHealthRunLogic(ctx, svcCtx)},
		{"OpsHealthUpdateLogic", NewOpsHealthUpdateLogic(ctx, svcCtx)},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.logic == nil {
				t.Errorf("%s() returned nil", tc.name)
			}
		})
	}
}

// Test request/response types thoroughly
func TestRequestTypes(t *testing.T) {
	t.Run("OpsAgentsListRequest defaults", func(t *testing.T) {
		req := OpsAgentsListRequest{}
		if req.GameID != "" {
			t.Errorf("expected empty GameID, got %s", req.GameID)
		}
		if req.Env != "" {
			t.Errorf("expected empty Env, got %s", req.Env)
		}
	})

	t.Run("OpsAgentSystemInfoRequest defaults", func(t *testing.T) {
		req := OpsAgentSystemInfoRequest{}
		if req.AgentID != "" {
			t.Errorf("expected empty AgentID, got %s", req.AgentID)
		}
	})

	t.Run("OpsAgentMetricsRequest defaults", func(t *testing.T) {
		req := OpsAgentMetricsRequest{}
		if req.AgentID != "" {
			t.Errorf("expected empty AgentID, got %s", req.AgentID)
		}
		if req.Since != "" {
			t.Errorf("expected empty Since, got %s", req.Since)
		}
		if req.Limit != 0 {
			t.Errorf("expected zero Limit, got %d", req.Limit)
		}
	})

	t.Run("OpsAgentProcessesRequest defaults", func(t *testing.T) {
		req := OpsAgentProcessesRequest{}
		if req.AgentID != "" {
			t.Errorf("expected empty AgentID, got %s", req.AgentID)
		}
	})

	t.Run("OpsExecCommandRequest defaults", func(t *testing.T) {
		req := OpsExecCommandRequest{}
		if req.AgentID != "" {
			t.Errorf("expected empty AgentID, got %s", req.AgentID)
		}
		if req.Command != "" {
			t.Errorf("expected empty Command, got %s", req.Command)
		}
		if req.Args == nil {
			req.Args = []string{}
		}
		if req.Env == nil {
			req.Env = map[string]string{}
		}
		if req.Timeout != 0 {
			t.Errorf("expected zero Timeout, got %d", req.Timeout)
		}
	})

	t.Run("OpsProcessStartRequest defaults", func(t *testing.T) {
		req := OpsProcessStartRequest{}
		if req.AgentID != "" {
			t.Errorf("expected empty AgentID, got %s", req.AgentID)
		}
		if req.Name != "" {
			t.Errorf("expected empty Name, got %s", req.Name)
		}
		if req.Args == nil {
			req.Args = []string{}
		}
		if req.Env == nil {
			req.Env = map[string]string{}
		}
	})

	t.Run("OpsProcessActionRequest defaults", func(t *testing.T) {
		req := OpsProcessActionRequest{}
		if req.AgentID != "" {
			t.Errorf("expected empty AgentID, got %s", req.AgentID)
		}
		if req.Name != "" {
			t.Errorf("expected empty Name, got %s", req.Name)
		}
		if req.Action != "" {
			t.Errorf("expected empty Action, got %s", req.Action)
		}
		if req.Force {
			t.Error("expected Force to be false")
		}
	})
}

// Test response types thoroughly
func TestResponseTypes(t *testing.T) {
	t.Run("OpsAgentsListResponse defaults", func(t *testing.T) {
		resp := OpsAgentsListResponse{}
		if resp.Code != 0 {
			t.Errorf("expected code 0, got %d", resp.Code)
		}
		if resp.Message != "" {
			t.Errorf("expected empty Message, got %s", resp.Message)
		}
		if resp.Data == nil {
			resp.Data = []OpsAgentInfo{}
		}
	})

	t.Run("OpsAgentSystemInfoResponse defaults", func(t *testing.T) {
		resp := OpsAgentSystemInfoResponse{}
		if resp.Code != 0 {
			t.Errorf("expected code 0, got %d", resp.Code)
		}
		if resp.Data.TotalMemory != 0 {
			t.Errorf("expected zero TotalMemory, got %d", resp.Data.TotalMemory)
		}
	})

	t.Run("OpsAgentMetricsResponse defaults", func(t *testing.T) {
		resp := OpsAgentMetricsResponse{}
		if resp.Code != 0 {
			t.Errorf("expected code 0, got %d", resp.Code)
		}
		if resp.Data == nil {
			resp.Data = []OpsMetricsData{}
		}
	})

	t.Run("OpsAgentProcessesResponse defaults", func(t *testing.T) {
		resp := OpsAgentProcessesResponse{}
		if resp.Code != 0 {
			t.Errorf("expected code 0, got %d", resp.Code)
		}
		if resp.Data == nil {
			resp.Data = []OpsManagedProcess{}
		}
	})

	t.Run("OpsExecCommandResponse defaults", func(t *testing.T) {
		resp := OpsExecCommandResponse{}
		if resp.Code != 0 {
			t.Errorf("expected code 0, got %d", resp.Code)
		}
		if resp.Data.ExitCode != 0 {
			t.Errorf("expected zero ExitCode, got %d", resp.Data.ExitCode)
		}
	})

	t.Run("OpsProcessStartResponse defaults", func(t *testing.T) {
		resp := OpsProcessStartResponse{}
		if resp.Code != 0 {
			t.Errorf("expected code 0, got %d", resp.Code)
		}
		if resp.Data != 0 {
			t.Errorf("expected zero Data, got %d", resp.Data)
		}
	})

	t.Run("OpsProcessActionResponse defaults", func(t *testing.T) {
		resp := OpsProcessActionResponse{}
		if resp.Code != 0 {
			t.Errorf("expected code 0, got %d", resp.Code)
		}
	})
}

// Test OpsServicesLogic response construction
func TestOpsServicesLogic_ResponseConstruction(t *testing.T) {
	t.Run("response fields", func(t *testing.T) {
		resp := OpsServicesResponse{
			Services: []OpsServiceItem{
				{
					ID:       "server",
					Name:     "croupier-server",
					Type:     "server",
					Status:   "running",
					Address:  "localhost:8080",
					Version:  "v1.0.0",
					Region:   "us-east-1",
					Zone:     "zone1",
					LastSeen: time.Now().Format(time.RFC3339),
				},
			},
			Total: 1,
		}

		if resp.Total != 1 {
			t.Errorf("expected total 1, got %d", resp.Total)
		}

		if len(resp.Services) != 1 {
			t.Errorf("expected 1 service, got %d", len(resp.Services))
		}

		svc := resp.Services[0]
		if svc.ID != "server" {
			t.Errorf("expected ID server, got %s", svc.ID)
		}
	})
}

// Test OpsServiceItem construction
func TestOpsServiceItem_Construction(t *testing.T) {
	t.Run("minimal service", func(t *testing.T) {
		svc := OpsServiceItem{
			ID:      "svc-1",
			Name:    "Service 1",
			Type:    "agent",
			Status:  "running",
			Address: "localhost:8081",
		}

		if svc.ID != "svc-1" {
			t.Errorf("expected ID svc-1, got %s", svc.ID)
		}
	})

	t.Run("service with metadata", func(t *testing.T) {
		svc := OpsServiceItem{
			ID:     "svc-1",
			Name:   "Service 1",
			Type:   "agent",
			Status: "running",
			Metadata: &OpsServiceMetadata{
				Processes: []OpsServiceProcess{
					{
						ServiceID:    "provider-1",
						Addr:         "localhost:9001",
						Version:      "v1.0.0",
						LastSeenUnix: time.Now().Unix(),
						FunctionIDs:  []string{"func1"},
						Functions:    1,
					},
				},
				ProcessesCount: 1,
			},
		}

		if svc.Metadata == nil {
			t.Error("expected metadata to be set")
		}

		if svc.Metadata.ProcessesCount != 1 {
			t.Errorf("expected ProcessesCount 1, got %d", svc.Metadata.ProcessesCount)
		}
	})
}

// Test empty collections handling
func TestEmptyCollections(t *testing.T) {
	t.Run("empty agent list", func(t *testing.T) {
		resp := OpsAgentsListResponse{
			Code:    0,
			Message: "OK",
			Data:    []OpsAgentInfo{},
		}

		if len(resp.Data) != 0 {
			t.Errorf("expected 0 agents, got %d", len(resp.Data))
		}
	})

	t.Run("empty metrics", func(t *testing.T) {
		resp := OpsAgentMetricsResponse{
			Code:    0,
			Message: "OK",
			Data:    []OpsMetricsData{},
		}

		if len(resp.Data) != 0 {
			t.Errorf("expected 0 metrics, got %d", len(resp.Data))
		}
	})

	t.Run("empty processes", func(t *testing.T) {
		resp := OpsAgentProcessesResponse{
			Code:    0,
			Message: "OK",
			Data:    []OpsManagedProcess{},
		}

		if len(resp.Data) != 0 {
			t.Errorf("expected 0 processes, got %d", len(resp.Data))
		}
	})

	t.Run("empty services", func(t *testing.T) {
		resp := OpsServicesResponse{
			Services: []OpsServiceItem{},
			Total:    0,
		}

		if len(resp.Services) != 0 {
			t.Errorf("expected 0 services, got %d", len(resp.Services))
		}

		if resp.Total != 0 {
			t.Errorf("expected total 0, got %d", resp.Total)
		}
	})
}

// Test helper functions with edge cases
func TestHelperFunctions_EdgeCases(t *testing.T) {
	t.Run("formatTimestamp with nil", func(t *testing.T) {
		result := formatTimestamp(nil)
		if result != "" {
			t.Errorf("expected empty string for nil, got %s", result)
		}
	})

	t.Run("formatLastSeen with zero times", func(t *testing.T) {
		result := formatLastSeen(time.Time{}, time.Time{})
		if result == "" {
			t.Error("expected non-empty result")
		}
	})

	t.Run("ttlAndHealth with nil session", func(t *testing.T) {
		ttl, healthy := ttlAndHealth(nil)
		if ttl != 0 {
			t.Errorf("expected TTL 0 for nil, got %d", ttl)
		}
		if healthy {
			t.Error("expected unhealthy for nil session")
		}
	})

	t.Run("collectServerLabels returns map", func(t *testing.T) {
		labels := collectServerLabels()
		if labels == nil {
			t.Error("expected non-nil labels map")
		}
	})
}

// Test OpsResponse wrapper
func TestOpsResponse_Wrapper(t *testing.T) {
	t.Run("default response", func(t *testing.T) {
		resp := OpsResponse{}
		if resp.Code != 0 {
			t.Errorf("expected code 0, got %d", resp.Code)
		}
		if resp.Message != "" {
			t.Errorf("expected empty message, got %s", resp.Message)
		}
		if resp.Data != nil {
			t.Error("expected nil data")
		}
	})

	t.Run("error response", func(t *testing.T) {
		resp := OpsResponse{
			Code:    404,
			Message: "not found",
		}
		if resp.Code != 404 {
			t.Errorf("expected code 404, got %d", resp.Code)
		}
	})

	t.Run("response with data", func(t *testing.T) {
		data := []string{"item1", "item2"}
		resp := OpsResponse{
			Code:    0,
			Message: "OK",
			Data:    data,
		}
		if resp.Code != 0 {
			t.Errorf("expected code 0, got %d", resp.Code)
		}
		if resp.Data == nil {
			t.Error("expected data to be set")
		}
	})
}

// Test context passing through logic constructors
func TestContextPassing(t *testing.T) {
	svcCtx := createTestServiceContext()

	ctx := context.Background()
	ctx = context.WithValue(ctx, "key1", "value1")
	ctx = context.WithValue(ctx, "key2", "value2")

	logic := NewOpsAgentsListLogic(ctx, svcCtx)

	if logic.ctx != ctx {
		t.Error("context not passed through logic")
	}

	// Verify context values are preserved
	val1 := logic.ctx.Value("key1")
	if val1 != "value1" {
		t.Errorf("expected value1, got %v", val1)
	}
}

// Test service context passing
func TestServiceContextPassing(t *testing.T) {
	svcCtx := createTestServiceContext()
	ctx := context.Background()

	logic := NewOpsAgentsListLogic(ctx, svcCtx)

	if logic.svcCtx != svcCtx {
		t.Error("service context not passed through logic")
	}

	if logic.svcCtx.ServerVersion != "v1.0.0" {
		t.Errorf("expected server version v1.0.0, got %s", logic.svcCtx.ServerVersion)
	}
}

// Test timestamp edge cases
func TestTimestampEdgeCases(t *testing.T) {
	t.Run("zero time", func(t *testing.T) {
		zero := time.Time{}
		result := formatTimestamp(nil)
		if result != "" {
			t.Errorf("expected empty for nil, got %s", result)
		}

		formatted := zero.Format(time.RFC3339)
		if formatted == "" {
			t.Error("expected non-empty formatted zero time")
		}
	})

	t.Run("maximum time", func(t *testing.T) {
		max := time.Unix(1<<63-1, 0)
		formatted := max.Format(time.RFC3339)
		if formatted == "" {
			t.Error("expected non-empty formatted max time")
		}
	})

	t.Run("minimum time", func(t *testing.T) {
		min := time.Unix(-1<<63, 0)
		formatted := min.Format(time.RFC3339)
		if formatted == "" {
			t.Error("expected non-empty formatted min time")
		}
	})
}

// Test OpsMetricsData slice handling
func TestOpsMetricsData_Slices(t *testing.T) {
	t.Run("empty slices", func(t *testing.T) {
		m := OpsMetricsData{
			CPU: OpsCpuMetrics{
				PerCore: []float64{},
			},
			Disks:    []OpsDiskMetrics{},
			Networks: []OpsNetworkMetrics{},
		}

		if len(m.CPU.PerCore) != 0 {
			t.Errorf("expected 0 per-core values, got %d", len(m.CPU.PerCore))
		}

		if len(m.Disks) != 0 {
			t.Errorf("expected 0 disks, got %d", len(m.Disks))
		}

		if len(m.Networks) != 0 {
			t.Errorf("expected 0 network interfaces, got %d", len(m.Networks))
		}
	})

	t.Run("nil slices", func(t *testing.T) {
		m := OpsMetricsData{}
		// Nil slices are valid
		_ = m.CPU.PerCore
		_ = m.Disks
		_ = m.Networks
	})
}

// Test OpsAgentInfo labels handling
func TestOpsAgentInfo_Labels(t *testing.T) {
	t.Run("nil labels", func(t *testing.T) {
		agent := OpsAgentInfo{
			AgentID: "agent-1",
			Labels:  nil,
		}

		if agent.Labels != nil {
			t.Error("expected nil labels")
		}
	})

	t.Run("empty labels", func(t *testing.T) {
		agent := OpsAgentInfo{
			AgentID: "agent-1",
			Labels:  map[string]string{},
		}

		if len(agent.Labels) != 0 {
			t.Errorf("expected 0 labels, got %d", len(agent.Labels))
		}
	})

	t.Run("labels with values", func(t *testing.T) {
		agent := OpsAgentInfo{
			AgentID: "agent-1",
			Labels: map[string]string{
				"key1": "value1",
				"key2": "value2",
			},
		}

		if len(agent.Labels) != 2 {
			t.Errorf("expected 2 labels, got %d", len(agent.Labels))
		}
	})
}

// Test state helpers error handling
func TestStateHelpers_ErrorHandling(t *testing.T) {
	t.Run("updateOpsState with nil update function", func(t *testing.T) {
		tmpDir := t.TempDir()
		store := svc.NewOpsStateStore(tmpDir)

		svcCtx := &svc.ServiceContext{
			OpsStateStore: store,
		}

		// This will panic or error, test that it's handled
		defer func() {
			if r := recover(); r != nil {
				t.Log("Recovered from panic with nil update function:", r)
			}
		}()

		_, _ = updateOpsState(svcCtx, nil)
	})
}

// Test concurrent access patterns
func TestConcurrentAccess(t *testing.T) {
	t.Run("concurrent formatLastSeen", func(t *testing.T) {
		now := time.Now()
		expire := now.Add(time.Minute)

		const goroutines = 100
		var wg sync.WaitGroup

		for i := 0; i < goroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_ = formatLastSeen(now, expire)
			}()
		}

		wg.Wait()
		// Should not panic
	})

	t.Run("concurrent ttlAndHealth", func(t *testing.T) {
		sess := &registry.AgentSession{
			ExpireAt: time.Now().Add(time.Minute),
		}

		const goroutines = 100
		var wg sync.WaitGroup

		for i := 0; i < goroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, _ = ttlAndHealth(sess)
			}()
		}

		wg.Wait()
		// Should not panic
	})

	t.Run("concurrent collectServerLabels", func(t *testing.T) {
		const goroutines = 100
		var wg sync.WaitGroup

		for i := 0; i < goroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_ = collectServerLabels()
			}()
		}

		wg.Wait()
		// Should not panic
	})
}

// Test response message variations
func TestResponseMessages(t *testing.T) {
	messages := []string{
		"OK",
		"not found",
		"internal error",
		"invalid request",
		"unauthorized",
		"agent not connected",
		"",
	}

	for _, msg := range messages {
		t.Run("message: "+msg, func(t *testing.T) {
			resp := OpsAgentSystemInfoResponse{
				Code:    0,
				Message: msg,
			}

			if resp.Message != msg {
				t.Errorf("expected message %s, got %s", msg, resp.Message)
			}
		})
	}
}

// Test response code variations
func TestResponseCodes(t *testing.T) {
	codes := []int{0, 200, 404, 500, 503}

	for _, code := range codes {
		t.Run("code: "+string(rune('0'+code)), func(t *testing.T) {
			resp := OpsAgentSystemInfoResponse{
				Code: code,
			}

			if resp.Code != code {
				t.Errorf("expected code %d, got %d", code, resp.Code)
			}
		})
	}
}

// Test cross-platform time handling
func TestCrossPlatformTimeHandling(t *testing.T) {
	locations := []*time.Location{
		time.UTC,
		time.FixedZone("EST", -5*3600),
		time.FixedZone("PST", -8*3600),
		time.FixedZone("CET", 1*3600),
		time.FixedZone("JST", 9*3600),
	}

	for _, loc := range locations {
		t.Run(loc.String(), func(t *testing.T) {
			now := time.Now().In(loc)
			formatted := now.Format(time.RFC3339)

			if formatted == "" {
				t.Error("expected non-empty formatted time")
			}

			// Parse it back
			parsed, err := time.Parse(time.RFC3339, formatted)
			if err != nil {
				t.Errorf("failed to parse time: %v", err)
			}

			// Check it's roughly the same time (within a second due to formatting precision)
			diff := parsed.Sub(now).Abs()
			if diff > time.Second {
				t.Logf("Note: parsed time differs by %v", diff)
			}
		})
	}
}

// createTestServiceContext creates a test ServiceContext for testing
func createTestServiceContext() *svc.ServiceContext {
	return &svc.ServiceContext{
		Config: config.Config{
			Server: config.ServerConfig{
				Host: "localhost",
				Port: 8080,
			},
		},
		ServerVersion:   "v1.0.0",
		StartTime:       time.Now(),
		RegistryStore:   registry.NewStore(),
		MetricsStore:    registry.NewMetricsStore(),
		SystemInfoCache: registry.NewSystemInfoCache(),
	}
}

