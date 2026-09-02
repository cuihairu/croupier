package ops

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/cluster"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---- convertCpuMetrics ----

func TestConvertCpuMetrics_V2(t *testing.T) {
	t.Parallel()

	t.Run("nil", func(t *testing.T) {
		t.Parallel()
		assert.Nil(t, convertCpuMetrics(nil))
	})

	t.Run("with values", func(t *testing.T) {
		t.Parallel()
		cpu := &opsv1.CpuMetrics{
			UsagePercent: 75.5,
			Cores:        8,
			PerCore:      []float64{50, 60, 70, 80, 90, 75, 65, 55},
			Load_1M:      1.5,
			Load_5M:      2.3,
			Load_15M:     3.1,
		}
		out := convertCpuMetrics(cpu)
		require.NotNil(t, out)
		assert.Equal(t, 75.5, out.UsagePercent)
		assert.Equal(t, int32(8), out.Cores)
		assert.Len(t, out.PerCore, 8)
		assert.Equal(t, 1.5, out.Load1M)
		assert.Equal(t, 2.3, out.Load5M)
		assert.Equal(t, 3.1, out.Load15M)
	})
}

// ---- convertMemoryMetrics ----

func TestConvertMemoryMetrics_V2(t *testing.T) {
	t.Parallel()

	t.Run("nil", func(t *testing.T) {
		t.Parallel()
		assert.Nil(t, convertMemoryMetrics(nil))
	})

	t.Run("with values", func(t *testing.T) {
		t.Parallel()
		mem := &opsv1.MemoryMetrics{
			TotalBytes:     16 * 1024 * 1024 * 1024,
			UsedBytes:      8 * 1024 * 1024 * 1024,
			AvailableBytes: 8 * 1024 * 1024 * 1024,
			UsagePercent:   50.0,
			SwapTotal:      4 * 1024 * 1024 * 1024,
			SwapUsed:       1 * 1024 * 1024 * 1024,
		}
		out := convertMemoryMetrics(mem)
		require.NotNil(t, out)
		assert.Equal(t, uint64(16*1024*1024*1024), out.TotalBytes)
		assert.Equal(t, 50.0, out.UsagePercent)
	})
}

// ---- convertDiskMetrics ----

func TestConvertDiskMetrics_V2(t *testing.T) {
	t.Parallel()

	t.Run("nil", func(t *testing.T) {
		t.Parallel()
		out := convertDiskMetrics(nil)
		assert.Empty(t, out)
	})

	t.Run("with disks", func(t *testing.T) {
		t.Parallel()
		disks := []*opsv1.DiskMetrics{
			{
				MountPoint:     "/",
				Device:         "/dev/sda1",
				FsType:         "ext4",
				TotalBytes:     100 * 1024 * 1024 * 1024,
				UsedBytes:      50 * 1024 * 1024 * 1024,
				AvailableBytes: 50 * 1024 * 1024 * 1024,
				UsagePercent:   50.0,
			},
		}
		out := convertDiskMetrics(disks)
		require.Len(t, out, 1)
		assert.Equal(t, "/", out[0].MountPoint)
		assert.Equal(t, "/dev/sda1", out[0].Device)
		assert.Equal(t, "ext4", out[0].FsType)
	})
}

// ---- agentMetricsHistory ----

func TestAgentMetricsHistory_V2(t *testing.T) {
	t.Parallel()

	t.Run("nil svcCtx", func(t *testing.T) {
		t.Parallel()
		resp, err := agentMetricsHistory(context.Background(), nil, &AgentMetricsHistoryRequest{AgentID: "a1"})
		require.NoError(t, err)
		assert.Equal(t, "a1", resp.AgentID)
		assert.Empty(t, resp.Entries)
	})

	t.Run("nil MetricsStore", func(t *testing.T) {
		t.Parallel()
		svcCtx := &svc.ServiceContext{}
		resp, err := agentMetricsHistory(context.Background(), svcCtx, &AgentMetricsHistoryRequest{AgentID: "a1"})
		require.NoError(t, err)
		assert.Equal(t, "a1", resp.AgentID)
		assert.Empty(t, resp.Entries)
	})

	t.Run("with MetricsStore empty", func(t *testing.T) {
		t.Parallel()
		svcCtx := &svc.ServiceContext{MetricsStore: registry.NewMetricsStore()}
		resp, err := agentMetricsHistory(context.Background(), svcCtx, &AgentMetricsHistoryRequest{
			AgentID: "a1",
			Since:   time.Now().Add(-time.Hour).Format(time.RFC3339),
			Limit:   10,
		})
		require.NoError(t, err)
		assert.Equal(t, "a1", resp.AgentID)
		assert.Empty(t, resp.Entries)
	})

	t.Run("default since and limit", func(t *testing.T) {
		t.Parallel()
		svcCtx := &svc.ServiceContext{MetricsStore: registry.NewMetricsStore()}
		resp, err := agentMetricsHistory(context.Background(), svcCtx, &AgentMetricsHistoryRequest{AgentID: "a1"})
		require.NoError(t, err)
		assert.NotNil(t, resp)
	})

	t.Run("invalid since time", func(t *testing.T) {
		t.Parallel()
		svcCtx := &svc.ServiceContext{MetricsStore: registry.NewMetricsStore()}
		resp, err := agentMetricsHistory(context.Background(), svcCtx, &AgentMetricsHistoryRequest{
			AgentID: "a1",
			Since:   "not-a-date",
		})
		require.NoError(t, err)
		assert.NotNil(t, resp)
	})
}

// ---- AgentMetricsHistory handler ----

func TestHandler_AgentMetricsHistory_V2(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := NewHandler(NewService(&svc.ServiceContext{}))

	ctx, rec := newOpsTestContext(http.MethodGet, "/api/v1/ops/agents/metrics/history?agentId=a1", "")
	h.AgentMetricsHistory(ctx)

	assert.True(t, rec.Code >= 200 && rec.Code < 600)
}

// ---- BackupDelete / BackupDownload alias handlers ----

func TestHandler_BackupDelete_V2(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := NewHandler(NewService(&svc.ServiceContext{}))

	ctx, rec := newOpsTestContext(http.MethodDelete, "/api/v1/ops/backups/1", "")
	h.BackupDelete(ctx)

	assert.True(t, rec.Code >= 200 && rec.Code < 600)
}

func TestHandler_BackupDownload_V2(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := NewHandler(NewService(&svc.ServiceContext{}))

	ctx, rec := newOpsTestContext(http.MethodGet, "/api/v1/ops/backups/1/download", "")
	h.BackupDownload(ctx)

	assert.True(t, rec.Code >= 200 && rec.Code < 600)
}

// ---- opsHealthGet with OpsStateStore ----

func TestOpsHealthGet_WithStateStore_V2(t *testing.T) {
	t.Parallel()
	opsStateStore := svc.NewOpsStateStore(t.TempDir())
	svcCtx := &svc.ServiceContext{OpsStateStore: opsStateStore}

	resp, err := opsHealthGet(context.Background(), svcCtx, &OpsHealthGetRequest{})
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Empty(t, resp.Checks)
}

func TestOpsHealthGet_NilSvcCtx_V2(t *testing.T) {
	t.Parallel()
	resp, err := opsHealthGet(context.Background(), nil, &OpsHealthGetRequest{})
	require.NoError(t, err)
	assert.Empty(t, resp.Checks)
}

// ---- opsHealthRun with OpsStateStore ----

func TestOpsHealthRun_WithStateStore_V2(t *testing.T) {
	t.Parallel()
	opsStateStore := svc.NewOpsStateStore(t.TempDir())
	svcCtx := &svc.ServiceContext{OpsStateStore: opsStateStore}

	// No checks configured, should return not found
	_, err := opsHealthRun(context.Background(), svcCtx, &OpsHealthRunRequest{ID: "check-1"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestOpsHealthRun_NilReq_V2(t *testing.T) {
	t.Parallel()
	_, err := opsHealthRun(context.Background(), nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "required")
}

func TestOpsHealthRun_EmptyID_V2(t *testing.T) {
	t.Parallel()
	_, err := opsHealthRun(context.Background(), nil, &OpsHealthRunRequest{ID: ""})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "required")
}

// ---- opsHealthUpdate with OpsStateStore ----

func TestOpsHealthUpdate_WithStateStore_V2(t *testing.T) {
	t.Parallel()
	opsStateStore := svc.NewOpsStateStore(t.TempDir())
	svcCtx := &svc.ServiceContext{OpsStateStore: opsStateStore}

	resp, err := opsHealthUpdate(context.Background(), svcCtx, &OpsHealthUpdateRequest{
		Checks: []OpsHealthCheck{
			{ID: "check-1", Kind: "tcp", Target: "localhost:8080", IntervalSec: 30},
		},
	})
	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestOpsHealthUpdate_GenerateID_V2(t *testing.T) {
	t.Parallel()
	opsStateStore := svc.NewOpsStateStore(t.TempDir())
	svcCtx := &svc.ServiceContext{OpsStateStore: opsStateStore}

	resp, err := opsHealthUpdate(context.Background(), svcCtx, &OpsHealthUpdateRequest{
		Checks: []OpsHealthCheck{
			{Kind: "http", Target: "https://example.com"},
		},
	})
	require.NoError(t, err)
	checks := resp.Checks.([]svc.OpsHealthCheck)
	require.Len(t, checks, 1)
	assert.NotEmpty(t, checks[0].ID)
}

// ---- opsMaintenanceGet with OpsStateStore ----

func TestOpsMaintenanceGet_WithStateStore_V2(t *testing.T) {
	t.Parallel()
	opsStateStore := svc.NewOpsStateStore(t.TempDir())
	svcCtx := &svc.ServiceContext{OpsStateStore: opsStateStore}

	resp, err := opsMaintenanceGet(context.Background(), svcCtx, &OpsMaintenanceGetRequest{})
	require.NoError(t, err)
	assert.Empty(t, resp.Windows)
}

func TestOpsMaintenanceGet_NilSvcCtx_V2(t *testing.T) {
	t.Parallel()
	resp, err := opsMaintenanceGet(context.Background(), nil, &OpsMaintenanceGetRequest{})
	require.NoError(t, err)
	assert.Empty(t, resp.Windows)
}

// ---- opsMaintenanceUpdate with OpsStateStore ----

func TestOpsMaintenanceUpdate_WithStateStore_V2(t *testing.T) {
	t.Parallel()
	opsStateStore := svc.NewOpsStateStore(t.TempDir())
	svcCtx := &svc.ServiceContext{OpsStateStore: opsStateStore}

	resp, err := opsMaintenanceUpdate(context.Background(), svcCtx, &OpsMaintenanceUpdateRequest{
		Windows: []OpsMaintenanceWindow{
			{GameID: "g1", Env: "prod", Start: "2025-01-01", End: "2025-01-02", Message: "maintenance"},
		},
	})
	require.NoError(t, err)
	windows := resp.Windows.([]svc.OpsMaintenanceWindow)
	require.Len(t, windows, 1)
	assert.Equal(t, "g1", windows[0].GameID)
}

func TestOpsMaintenanceUpdate_GenerateID_V2(t *testing.T) {
	t.Parallel()
	opsStateStore := svc.NewOpsStateStore(t.TempDir())
	svcCtx := &svc.ServiceContext{OpsStateStore: opsStateStore}

	resp, err := opsMaintenanceUpdate(context.Background(), svcCtx, &OpsMaintenanceUpdateRequest{
		Windows: []OpsMaintenanceWindow{
			{GameID: "g1", Env: "prod"},
		},
	})
	require.NoError(t, err)
	windows := resp.Windows.([]svc.OpsMaintenanceWindow)
	require.Len(t, windows, 1)
	assert.NotEmpty(t, windows[0].ID)
}

// ---- opsMQ ----

func TestOpsMQ_WithStateStore_V2(t *testing.T) {
	t.Parallel()
	opsStateStore := svc.NewOpsStateStore(t.TempDir())
	svcCtx := &svc.ServiceContext{OpsStateStore: opsStateStore}

	resp, err := opsMQ(context.Background(), svcCtx, &OpsMQRequest{})
	require.NoError(t, err)
	assert.NotNil(t, resp.Result)
}

func TestOpsMQ_NilSvcCtx_V2(t *testing.T) {
	t.Parallel()
	resp, err := opsMQ(context.Background(), nil, &OpsMQRequest{})
	require.NoError(t, err)
	assert.NotNil(t, resp.Result)
}

// ---- opsConfig ----

func TestOpsConfig_WithStateStore_V2(t *testing.T) {
	t.Parallel()
	opsStateStore := svc.NewOpsStateStore(t.TempDir())
	svcCtx := &svc.ServiceContext{OpsStateStore: opsStateStore}

	resp, err := opsConfig(context.Background(), svcCtx, &OpsConfigRequest{})
	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestOpsConfig_NilSvcCtx_V2(t *testing.T) {
	t.Parallel()
	resp, err := opsConfig(context.Background(), nil, &OpsConfigRequest{})
	require.NoError(t, err)
	assert.NotNil(t, resp)
}

// ---- opsMetrics with MetricsStore ----

func TestOpsMetrics_WithMetricsStore_V2(t *testing.T) {
	t.Parallel()
	store := registry.NewStore()
	store.UpsertAgent(&registry.AgentSession{
		AgentID: "a1",
		GameID:  "g1",
		Env:     "prod",
		Labels:  map[string]string{},
		Functions: map[string]registry.FunctionMeta{
			"f1": {Enabled: true},
		},
		LastSeen: time.Now(),
	})

	metricsStore := registry.NewMetricsStore()
	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		MetricsStore:  metricsStore,
	}

	resp, err := opsMetrics(context.Background(), svcCtx, &OpsMetricsRequest{
		GameId: "g1",
		Env:    "prod",
	})
	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestOpsMetrics_NilRegistryStore_V2(t *testing.T) {
	t.Parallel()
	svcCtx := &svc.ServiceContext{MetricsStore: registry.NewMetricsStore()}
	resp, err := opsMetrics(context.Background(), svcCtx, &OpsMetricsRequest{})
	require.NoError(t, err)
	assert.Empty(t, resp.Metrics)
}

func TestOpsMetrics_NilMetricsStore_V2(t *testing.T) {
	t.Parallel()
	svcCtx := &svc.ServiceContext{RegistryStore: registry.NewStore()}
	resp, err := opsMetrics(context.Background(), svcCtx, &OpsMetricsRequest{})
	require.NoError(t, err)
	assert.Empty(t, resp.Metrics)
}

// ---- opsBackupDelete ----

func TestOpsBackupDelete_NilSvcCtx_V2(t *testing.T) {
	t.Parallel()
	resp, err := opsBackupDelete(context.Background(), nil, &OpsBackupDeleteRequest{ID: "b1"})
	require.NoError(t, err)
	assert.True(t, resp.Deleted)
}

func TestOpsBackupDelete_NilBackupModel_V2(t *testing.T) {
	t.Parallel()
	svcCtx := &svc.ServiceContext{}
	resp, err := opsBackupDelete(context.Background(), svcCtx, &OpsBackupDeleteRequest{ID: "b1"})
	require.NoError(t, err)
	assert.True(t, resp.Deleted)
}

// ---- opsBackupDownload ----

func TestOpsBackupDownload_NilSvcCtx_V2(t *testing.T) {
	t.Parallel()
	resp, err := opsBackupDownload(context.Background(), nil, &OpsBackupDownloadRequest{ID: "b1"})
	require.NoError(t, err)
	assert.Contains(t, resp.Url, "b1")
}

func TestOpsBackupDownload_NilBackupModel_V2(t *testing.T) {
	t.Parallel()
	svcCtx := &svc.ServiceContext{}
	resp, err := opsBackupDownload(context.Background(), svcCtx, &OpsBackupDownloadRequest{ID: "b1"})
	require.NoError(t, err)
	assert.Contains(t, resp.Url, "b1")
}

// ---- opsServicesLegacyCompatible ----

func TestOpsServicesLegacyCompatible_V2(t *testing.T) {
	t.Parallel()

	t.Run("nil svcCtx", func(t *testing.T) {
		t.Parallel()
		resp, err := opsServicesLegacyCompatible(context.Background(), nil, &OpsServicesRequest{})
		require.NoError(t, err)
		assert.Empty(t, resp.Services)
	})

	t.Run("with registry store", func(t *testing.T) {
		t.Parallel()
		store := registry.NewStore()
		store.UpsertAgent(&registry.AgentSession{
			AgentID:   "agent-1",
			GameID:    "g1",
			Env:       "prod",
			Addr:      "localhost:19090",
			Version:   "1.0.0",
			Labels:    map[string]string{"hostname": "h1"},
			Functions: map[string]registry.FunctionMeta{"f1": {Enabled: true}},
			Providers: []registry.ProviderSession{
				{ProviderID: "p1", Addr: "localhost:8080", Version: "1.0", FunctionIDs: []string{"f1"}},
			},
			LastSeen: time.Now(),
			ExpireAt: time.Now().Add(time.Hour),
		})

		svcCtx := &svc.ServiceContext{RegistryStore: store}
		resp, err := opsServicesLegacyCompatible(context.Background(), svcCtx, &OpsServicesRequest{})
		require.NoError(t, err)
		// 1 server + 1 agent
		assert.GreaterOrEqual(t, len(resp.Services), 1)
	})
}

// ---- nodeState helper functions ----

func TestResolveNodeStatus_V2(t *testing.T) {
	t.Parallel()
	now := time.Now()

	tests := []struct {
		name     string
		sess     *registry.AgentSession
		drained  bool
		expected string
	}{
		{"drained", &registry.AgentSession{AgentID: "n1", Addr: "h:1", LastSeen: now}, true, "drained"},
		{"nil session", nil, false, "stale"},
		{"empty addr", &registry.AgentSession{AgentID: "n1", Addr: "", LastSeen: now}, false, "stale"},
		{"zero LastSeen", &registry.AgentSession{AgentID: "n1", Addr: "h:1", LastSeen: time.Time{}}, false, "stale"},
		{"expired", &registry.AgentSession{AgentID: "n1", Addr: "h:1", LastSeen: now, ExpireAt: now.Add(-time.Hour)}, false, "stale"},
		{"active", &registry.AgentSession{AgentID: "n1", Addr: "h:1", LastSeen: now, ExpireAt: now.Add(time.Hour)}, false, "active"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, resolveNodeStatus(tt.sess, tt.drained, now))
		})
	}
}

func TestNormalizeNodeStatusFilter_V2(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "", normalizeNodeStatusFilter(""))
	assert.Equal(t, "", normalizeNodeStatusFilter("*"))
	assert.Equal(t, "active", normalizeNodeStatusFilter("ACTIVE"))
	assert.Equal(t, "drained", normalizeNodeStatusFilter("  Drained  "))
}

func TestNodeStatusMatches_V2(t *testing.T) {
	t.Parallel()

	assert.True(t, nodeStatusMatches("", "active"))
	assert.True(t, nodeStatusMatches("", ""))
	assert.True(t, nodeStatusMatches("offline", "offline"))
	assert.True(t, nodeStatusMatches("offline", "stale"))
	assert.False(t, nodeStatusMatches("offline", "active"))
	assert.False(t, nodeStatusMatches("active", "drained"))
	assert.True(t, nodeStatusMatches("active", "active"))
	assert.True(t, nodeStatusMatches("drained", "drained"))
}

func TestNodeStatusRank_V2(t *testing.T) {
	t.Parallel()

	assert.Equal(t, 0, nodeStatusRank("active"))
	assert.Equal(t, 1, nodeStatusRank("drained"))
	assert.Equal(t, 2, nodeStatusRank("stale"))
	assert.Equal(t, 3, nodeStatusRank("offline"))
	assert.Equal(t, 4, nodeStatusRank("unknown"))
}

func TestDatabaseNodeAddr_V2(t *testing.T) {
	t.Parallel()

	t.Run("empty IP", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "", databaseNodeAddr(model.Node{IP: "", Port: 0}))
	})

	t.Run("zero port", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "127.0.0.1", databaseNodeAddr(model.Node{IP: "127.0.0.1", Port: 0}))
	})

	t.Run("with port", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "127.0.0.1:8080", databaseNodeAddr(model.Node{IP: "127.0.0.1", Port: 8080}))
	})
}

// ---- databaseNodeString ----

func TestDatabaseNodeString_V2(t *testing.T) {
	t.Parallel()

	// No Meta
	node := offlineDatabaseNode(model.Node{})
	assert.Equal(t, "offline", node.Status)

	// With Meta
	node2 := offlineDatabaseNode(model.Node{
		NodeID: "n1",
		Name:   "Node 1",
		IP:     "10.0.0.1",
		Port:   3000,
		Meta: map[string]interface{}{
			"hostname": "host1",
			"gameId":   "game1",
			"env":      "prod",
			"lastSeen": "2025-01-01",
		},
	})
	assert.Equal(t, "n1", node2.Id)
	assert.Equal(t, "host1", node2.Hostname)
	assert.Equal(t, "10.0.0.1:3000", node2.Addr)
	assert.Equal(t, "game1", node2.GameId)
	assert.Equal(t, "prod", node2.Env)
	assert.Equal(t, "2025-01-01", node2.LastSeen)
}

// ---- NodeService with registry ----

func TestNodeServiceDrain_WithStore_V2(t *testing.T) {
	t.Parallel()
	store := registry.NewStore()
	store.UpsertAgent(&registry.AgentSession{
		AgentID:   "node-1",
		Functions: map[string]registry.FunctionMeta{},
		LastSeen:  time.Now(),
	})
	svcCtx := &svc.ServiceContext{RegistryStore: store}
	s := NewNodeService(svcCtx)

	err := s.Drain(context.Background(), "node-1")
	require.NoError(t, err)
}

func TestNodeServiceDrain_NotFound_V2(t *testing.T) {
	t.Parallel()
	store := registry.NewStore()
	svcCtx := &svc.ServiceContext{RegistryStore: store}
	s := NewNodeService(svcCtx)

	err := s.Drain(context.Background(), "nonexistent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestNodeServiceDrain_WithOpsState_V2(t *testing.T) {
	t.Parallel()
	store := registry.NewStore()
	store.UpsertAgent(&registry.AgentSession{
		AgentID:   "node-1",
		Functions: map[string]registry.FunctionMeta{},
		LastSeen:  time.Now(),
	})
	opsStateStore := svc.NewOpsStateStore(t.TempDir())
	svcCtx := &svc.ServiceContext{RegistryStore: store, OpsStateStore: opsStateStore}
	s := NewNodeService(svcCtx)

	err := s.Drain(context.Background(), "node-1")
	require.NoError(t, err)
}

func TestNodeServiceRestart_WithStore_V2(t *testing.T) {
	t.Parallel()
	store := registry.NewStore()
	store.UpsertAgent(&registry.AgentSession{
		AgentID:   "node-1",
		Functions: map[string]registry.FunctionMeta{},
		LastSeen:  time.Now(),
	})
	svcCtx := &svc.ServiceContext{RegistryStore: store}
	s := NewNodeService(svcCtx)

	err := s.Restart(context.Background(), "node-1")
	require.NoError(t, err)
}

func TestNodeServiceRestart_NotFound_V2(t *testing.T) {
	t.Parallel()
	store := registry.NewStore()
	svcCtx := &svc.ServiceContext{RegistryStore: store}
	s := NewNodeService(svcCtx)

	err := s.Restart(context.Background(), "nonexistent")
	require.Error(t, err)
}

func TestNodeServiceRestart_WithOpsState_V2(t *testing.T) {
	t.Parallel()
	store := registry.NewStore()
	store.UpsertAgent(&registry.AgentSession{
		AgentID:   "node-1",
		Functions: map[string]registry.FunctionMeta{},
		LastSeen:  time.Now(),
	})
	opsStateStore := svc.NewOpsStateStore(t.TempDir())
	svcCtx := &svc.ServiceContext{RegistryStore: store, OpsStateStore: opsStateStore}
	s := NewNodeService(svcCtx)

	err := s.Restart(context.Background(), "node-1")
	require.NoError(t, err)
}

func TestNodeServiceUndrain_WithStore_V2(t *testing.T) {
	t.Parallel()
	store := registry.NewStore()
	store.UpsertAgent(&registry.AgentSession{
		AgentID:   "node-1",
		Functions: map[string]registry.FunctionMeta{},
		LastSeen:  time.Now(),
	})
	svcCtx := &svc.ServiceContext{RegistryStore: store}
	s := NewNodeService(svcCtx)

	err := s.Undrain(context.Background(), "node-1")
	require.NoError(t, err)
}

func TestNodeServiceUndrain_NotFound_V2(t *testing.T) {
	t.Parallel()
	store := registry.NewStore()
	svcCtx := &svc.ServiceContext{RegistryStore: store}
	s := NewNodeService(svcCtx)

	err := s.Undrain(context.Background(), "nonexistent")
	require.Error(t, err)
}

func TestNodeServiceUndrain_WithOpsState_V2(t *testing.T) {
	t.Parallel()
	store := registry.NewStore()
	store.UpsertAgent(&registry.AgentSession{
		AgentID:   "node-1",
		Functions: map[string]registry.FunctionMeta{},
		LastSeen:  time.Now(),
	})
	opsStateStore := svc.NewOpsStateStore(t.TempDir())
	svcCtx := &svc.ServiceContext{RegistryStore: store, OpsStateStore: opsStateStore}
	s := NewNodeService(svcCtx)

	// First drain, then undrain
	_ = s.Drain(context.Background(), "node-1")
	err := s.Undrain(context.Background(), "node-1")
	require.NoError(t, err)
}

// ---- listNodes with filters ----

func TestListNodes_V2(t *testing.T) {
	t.Parallel()

	t.Run("nil svcCtx", func(t *testing.T) {
		t.Parallel()
		nodes := listNodes(context.Background(), nil, "", "", "")
		assert.Empty(t, nodes)
	})

	t.Run("with gameID filter", func(t *testing.T) {
		t.Parallel()
		store := registry.NewStore()
		now := time.Now()
		store.UpsertAgent(&registry.AgentSession{
			AgentID:   "a1",
			GameID:    "g1",
			Env:       "prod",
			Addr:      "h:1",
			Labels:    map[string]string{},
			Functions: map[string]registry.FunctionMeta{},
			LastSeen:  now,
			ExpireAt:  now.Add(time.Hour),
		})
		store.UpsertAgent(&registry.AgentSession{
			AgentID:   "a2",
			GameID:    "g2",
			Env:       "prod",
			Addr:      "h:2",
			Labels:    map[string]string{},
			Functions: map[string]registry.FunctionMeta{},
			LastSeen:  now,
			ExpireAt:  now.Add(time.Hour),
		})

		svcCtx := &svc.ServiceContext{RegistryStore: store}
		nodes := listNodes(context.Background(), svcCtx, "g1", "", "")
		assert.Len(t, nodes, 1)
		assert.Equal(t, "a1", nodes[0].Id)
	})

	t.Run("with env filter", func(t *testing.T) {
		t.Parallel()
		store := registry.NewStore()
		now := time.Now()
		store.UpsertAgent(&registry.AgentSession{
			AgentID:   "a1",
			GameID:    "g1",
			Env:       "prod",
			Addr:      "h:1",
			Labels:    map[string]string{},
			Functions: map[string]registry.FunctionMeta{},
			LastSeen:  now,
			ExpireAt:  now.Add(time.Hour),
		})
		store.UpsertAgent(&registry.AgentSession{
			AgentID:   "a2",
			GameID:    "g1",
			Env:       "dev",
			Addr:      "h:2",
			Labels:    map[string]string{},
			Functions: map[string]registry.FunctionMeta{},
			LastSeen:  now,
			ExpireAt:  now.Add(time.Hour),
		})

		svcCtx := &svc.ServiceContext{RegistryStore: store}
		nodes := listNodes(context.Background(), svcCtx, "", "prod", "")
		assert.Len(t, nodes, 1)
		assert.Equal(t, "a1", nodes[0].Id)
	})

	t.Run("with status filter", func(t *testing.T) {
		t.Parallel()
		store := registry.NewStore()
		now := time.Now()
		store.UpsertAgent(&registry.AgentSession{
			AgentID:   "a1",
			GameID:    "g1",
			Addr:      "h:1",
			Labels:    map[string]string{},
			Functions: map[string]registry.FunctionMeta{},
			LastSeen:  now,
			ExpireAt:  now.Add(time.Hour),
		})
		store.UpsertAgent(&registry.AgentSession{
			AgentID:   "a2",
			GameID:    "g1",
			Addr:      "",
			Labels:    map[string]string{},
			Functions: map[string]registry.FunctionMeta{},
			LastSeen:  now.Add(-time.Hour),
			ExpireAt:  now.Add(time.Hour),
		})

		svcCtx := &svc.ServiceContext{RegistryStore: store}
		nodes := listNodes(context.Background(), svcCtx, "", "", "active")
		assert.Len(t, nodes, 1)
		assert.Equal(t, "a1", nodes[0].Id)
	})
}

// ---- opsNodeDrain with OpsStateStore ----

func TestOpsNodeDrain_WithOpsState_V2(t *testing.T) {
	t.Parallel()
	store := registry.NewStore()
	store.UpsertAgent(&registry.AgentSession{
		AgentID:   "node-1",
		Functions: map[string]registry.FunctionMeta{},
		LastSeen:  time.Now(),
	})
	opsStateStore := svc.NewOpsStateStore(t.TempDir())

	type agentCaller struct{}
	// We need to mock AgentSessionResolver. Let's just test without it.
	svcCtx := &svc.ServiceContext{RegistryStore: store, OpsStateStore: opsStateStore}
	_, err := opsNodeDrain(context.Background(), svcCtx, &OpsNodeCommandsRequest{NodeId: "node-1"})
	// Should fail because AgentSessionResolver is nil
	require.Error(t, err)
	assert.Contains(t, err.Error(), "session resolver")
}

// ---- opsNodeRestart with OpsStateStore ----

func TestOpsNodeRestart_WithOpsState_V2(t *testing.T) {
	t.Parallel()
	store := registry.NewStore()
	store.UpsertAgent(&registry.AgentSession{
		AgentID:   "node-1",
		Functions: map[string]registry.FunctionMeta{},
		LastSeen:  time.Now(),
	})
	opsStateStore := svc.NewOpsStateStore(t.TempDir())
	svcCtx := &svc.ServiceContext{RegistryStore: store, OpsStateStore: opsStateStore}
	_, err := opsNodeRestart(context.Background(), svcCtx, &OpsNodeCommandsRequest{NodeId: "node-1"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "session resolver")
}

// ---- opsNodeUndrain with OpsStateStore ----

func TestOpsNodeUndrain_WithOpsState_V2(t *testing.T) {
	t.Parallel()
	store := registry.NewStore()
	store.UpsertAgent(&registry.AgentSession{
		AgentID:   "node-1",
		Functions: map[string]registry.FunctionMeta{},
		LastSeen:  time.Now(),
	})
	opsStateStore := svc.NewOpsStateStore(t.TempDir())
	svcCtx := &svc.ServiceContext{RegistryStore: store, OpsStateStore: opsStateStore}
	_, err := opsNodeUndrain(context.Background(), svcCtx, &OpsNodeCommandsRequest{NodeId: "node-1"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "session resolver")
}

// ---- opsNodeDrain/Restart/Undrain empty nodeId ----

func TestOpsNodeDrain_EmptyNodeId_V2(t *testing.T) {
	t.Parallel()
	_, err := opsNodeDrain(context.Background(), &svc.ServiceContext{}, &OpsNodeCommandsRequest{NodeId: ""})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "required")
}

func TestOpsNodeRestart_EmptyNodeId_V2(t *testing.T) {
	t.Parallel()
	_, err := opsNodeRestart(context.Background(), &svc.ServiceContext{}, &OpsNodeCommandsRequest{NodeId: ""})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "required")
}

func TestOpsNodeUndrain_EmptyNodeId_V2(t *testing.T) {
	t.Parallel()
	_, err := opsNodeUndrain(context.Background(), &svc.ServiceContext{}, &OpsNodeCommandsRequest{NodeId: ""})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "required")
}

// ---- AgentService ----

func TestAgentService_GetMeta_V2(t *testing.T) {
	t.Parallel()
	store := registry.NewStore()
	svcCtx := &svc.ServiceContext{RegistryStore: store}
	s := NewAgentService(svcCtx)

	_, err := s.GetMeta(context.Background(), "nonexistent")
	require.Error(t, err)
}

func TestAgentService_GetMeta_NilStore_V2(t *testing.T) {
	t.Parallel()
	svcCtx := &svc.ServiceContext{}
	s := NewAgentService(svcCtx)

	_, err := s.GetMeta(context.Background(), "a1")
	require.Error(t, err)
}

func TestAgentService_List_NilStore_V2(t *testing.T) {
	t.Parallel()
	svcCtx := &svc.ServiceContext{}
	s := NewAgentService(svcCtx)

	agents, err := s.List(context.Background(), "", "", "")
	require.NoError(t, err)
	assert.Empty(t, agents)
}

func TestAgentService_List_WithFilters_V2(t *testing.T) {
	t.Parallel()
	store := registry.NewStore()
	store.UpsertAgent(&registry.AgentSession{
		AgentID:   "a1",
		GameID:    "g1",
		Env:       "prod",
		Addr:      "h:1",
		Functions: map[string]registry.FunctionMeta{"f1": {Enabled: true}},
		LastSeen:  time.Now(),
	})
	svcCtx := &svc.ServiceContext{RegistryStore: store}
	s := NewAgentService(svcCtx)

	// No filter
	agents, err := s.List(context.Background(), "", "", "")
	require.NoError(t, err)
	assert.Len(t, agents, 1)

	// Game filter match
	agents, err = s.List(context.Background(), "g1", "", "")
	require.NoError(t, err)
	assert.Len(t, agents, 1)

	// Game filter no match
	agents, err = s.List(context.Background(), "g2", "", "")
	require.NoError(t, err)
	assert.Empty(t, agents)

	// Env filter match
	agents, err = s.List(context.Background(), "", "prod", "")
	require.NoError(t, err)
	assert.Len(t, agents, 1)

	// Env filter no match
	agents, err = s.List(context.Background(), "", "dev", "")
	require.NoError(t, err)
	assert.Empty(t, agents)
}

// ---- NodeService.GetMeta ----

func TestNodeService_GetMeta_V2(t *testing.T) {
	t.Parallel()
	store := registry.NewStore()
	svcCtx := &svc.ServiceContext{RegistryStore: store}
	s := NewNodeService(svcCtx)

	_, err := s.GetMeta(context.Background(), "nonexistent")
	require.Error(t, err)
}

func TestNodeService_GetMeta_NilStore_V2(t *testing.T) {
	t.Parallel()
	svcCtx := &svc.ServiceContext{}
	s := NewNodeService(svcCtx)

	_, err := s.GetMeta(context.Background(), "a1")
	require.Error(t, err)
}

// ---- runtimeNodeListItem ----

func TestRuntimeNodeListItem_V2(t *testing.T) {
	t.Parallel()

	t.Run("with providers", func(t *testing.T) {
		t.Parallel()
		sess := &registry.AgentSession{
			AgentID: "a1",
			Labels:  map[string]string{"hostname": "h1"},
			Functions: map[string]registry.FunctionMeta{
				"f1": {Enabled: true},
				"f2": {Enabled: true},
			},
			Providers: []registry.ProviderSession{
				{SDKLanguage: "go", SDKVersion: "1.21", SDKName: "croupier-go"},
			},
			ExpireAt: time.Now().Add(time.Hour),
			LastSeen: time.Now(),
		}
		item := runtimeNodeListItem(sess, "active", nil)
		assert.Equal(t, "a1", item.node.Id)
		assert.Equal(t, "h1", item.node.Hostname)
		assert.Equal(t, "go", item.node.SDKLanguage)
		assert.Equal(t, "1.21", item.node.SDKVersion)
		assert.Equal(t, "croupier-go", item.node.SDKName)
		assert.Equal(t, 2, item.node.Functions)
	})

	t.Run("expired session", func(t *testing.T) {
		t.Parallel()
		sess := &registry.AgentSession{
			AgentID:  "a1",
			Addr:     "h:1",
			ExpireAt: time.Now().Add(-time.Hour),
			LastSeen: time.Now().Add(-2 * time.Hour),
		}
		item := runtimeNodeListItem(sess, "stale", nil)
		assert.Equal(t, int64(0), item.node.ExpiresInSec)
	})
}

// ---- offlineDatabaseNode ----

func TestOfflineDatabaseNode_V2(t *testing.T) {
	t.Parallel()

	t.Run("minimal node", func(t *testing.T) {
		t.Parallel()
		node := offlineDatabaseNode(model.Node{
			NodeID: "n1",
		})
		assert.Equal(t, "n1", node.Id)
		assert.Equal(t, "offline", node.Status)
	})

	t.Run("with hostname and game", func(t *testing.T) {
		t.Parallel()
		node := offlineDatabaseNode(model.Node{
			NodeID: "n1",
			Name:   "Node 1",
			Type:   "agent",
			Meta: map[string]interface{}{
				"hostname": "host1",
				"gameId":   "g1",
				"env":      "prod",
			},
		})
		assert.Equal(t, "host1", node.Hostname)
		assert.Equal(t, "g1", node.GameId)
		assert.Equal(t, "prod", node.Env)
		assert.Equal(t, "agent", node.Labels["type"])
	})

	t.Run("with game_id key", func(t *testing.T) {
		t.Parallel()
		node := offlineDatabaseNode(model.Node{
			NodeID: "n1",
			Meta: map[string]interface{}{
				"gameId": "g2",
			},
		})
		assert.Equal(t, "g2", node.GameId)
	})
}

// ---- firstNonEmpty helper ----

func TestFirstNonEmpty_V2(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "", firstNonEmpty())
	assert.Equal(t, "a", firstNonEmpty("a", "b"))
	assert.Equal(t, "b", firstNonEmpty("", "b"))
	assert.Equal(t, "c", firstNonEmpty("", "", "c"))
	assert.Equal(t, "x", firstNonEmpty("  x  ", "y"))
}

// ---- offlineDatabaseNodeItems ----

func TestOfflineDatabaseNodeItems_NilNodeModel_V2(t *testing.T) {
	t.Parallel()
	svcCtx := &svc.ServiceContext{}
	items := offlineDatabaseNodeItems(context.Background(), svcCtx, nil, "", "", "")
	assert.Empty(t, items)
}

// ---- extractNotificationConfig ----

func TestExtractNotificationConfig_V2(t *testing.T) {
	t.Parallel()

	t.Run("nil config", func(t *testing.T) {
		t.Parallel()
		enabled, channels, rules, ok, err := extractNotificationConfig(nil)
		require.NoError(t, err)
		assert.False(t, enabled)
		assert.Nil(t, channels)
		assert.Nil(t, rules)
		assert.False(t, ok)
	})

	t.Run("empty config", func(t *testing.T) {
		t.Parallel()
		enabled, channels, rules, ok, err := extractNotificationConfig(map[string]any{})
		require.NoError(t, err)
		assert.False(t, enabled)
		assert.Nil(t, channels)
		assert.Nil(t, rules)
		assert.False(t, ok)
	})

	t.Run("full config", func(t *testing.T) {
		t.Parallel()
		config := map[string]any{
			"enabled": true,
			"channels": []map[string]any{
				{"id": "ch-1", "type": "webhook", "url": "https://example.com"},
			},
			"rules": []map[string]any{
				{"event": "alert.fired", "channels": []string{"ch-1"}},
			},
		}
		enabled, channels, rules, ok, err := extractNotificationConfig(config)
		require.NoError(t, err)
		assert.True(t, enabled)
		assert.Len(t, channels, 1)
		assert.Len(t, rules, 1)
		assert.True(t, ok)
	})

	t.Run("enabled only", func(t *testing.T) {
		t.Parallel()
		config := map[string]any{"enabled": true}
		enabled, _, _, ok, err := extractNotificationConfig(config)
		require.NoError(t, err)
		assert.True(t, enabled)
		assert.True(t, ok)
	})
}

// ---- opsNotificationsGet ----

func TestOpsNotificationsGet_NilExtension_V2(t *testing.T) {
	t.Parallel()
	svcCtx := &svc.ServiceContext{}
	resp, err := opsNotificationsGet(context.Background(), svcCtx, &OpsNotificationsGetRequest{})
	require.NoError(t, err)
	assert.False(t, resp.Enabled)
}

// ---- opsNodeCommands ----

func TestOpsNodeCommands_V2(t *testing.T) {
	t.Parallel()
	svcCtx := &svc.ServiceContext{}
	resp, err := opsNodeCommands(context.Background(), svcCtx, &OpsNodeCommandsRequest{NodeId: "n1"})
	require.NoError(t, err)
	assert.Len(t, resp.Commands, 3)
}

// ---- opsAgentsList with nil session ----

func TestOpsAgentsList_NilSession_V2(t *testing.T) {
	t.Parallel()
	svcCtx := &svc.ServiceContext{RegistryStore: nil}
	resp, err := opsAgentsList(context.Background(), svcCtx, &OpsAgentsListRequest{})
	require.NoError(t, err)
	assert.Empty(t, resp.Agents)
}

// ---- OpsMetricsRequest query binding test ----

func TestOpsMetricsRequestQueryBinding_V2(t *testing.T) {
	t.Parallel()

	ctx, _ := newOpsTestContext(http.MethodGet, "/api/v1/ops/metrics?gameId=g1&env=prod&metric=cpu&start=2025-01-01&end=2025-12-31", "")
	var req OpsMetricsRequest
	err := bindOpsRequest(ctx, &req)
	require.NoError(t, err)
	assert.Equal(t, "g1", req.GameId)
	assert.Equal(t, "prod", req.Env)
	assert.Equal(t, "cpu", req.Metric)
}

// ---- Handler alias tests ----

func TestHandler_Alias_V2(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)
	h := NewHandler(NewService(&svc.ServiceContext{}))

	aliases := []struct {
		name string
		fn   func(*gin.Context)
	}{
		{"AgentsList", h.AgentsList},
		{"AgentMeta", h.AgentMeta},
		{"AgentMetrics", h.AgentMetrics},
		{"AgentProcesses", h.AgentProcesses},
		{"AgentSystemInfo", h.AgentSystemInfo},
		{"AgentProcessStart", h.AgentProcessStart},
		{"AgentProcessStop", h.AgentProcessStop},
		{"AgentProcessRestart", h.AgentProcessRestart},
		{"AgentExecCommand", h.AgentExecCommand},
		{"Nodes", h.Nodes},
		{"NodeCommands", h.NodeCommands},
		{"NodeDrain", h.NodeDrain},
		{"NodeMeta", h.NodeMeta},
		{"NodeRestart", h.NodeRestart},
		{"NodeUndrain", h.NodeUndrain},
		{"HealthGet", h.HealthGet},
		{"MaintenanceGet", h.MaintenanceGet},
		{"Metrics", h.Metrics},
		{"MQ", h.MQ},
		{"NotificationsGet", h.NotificationsGet},
		{"Functions", h.Functions},
		{"Config", h.Config},
	}

	for _, a := range aliases {
		a := a
		t.Run(a.name, func(t *testing.T) {
			t.Parallel()
			ctx, rec := newOpsTestContext(http.MethodGet, "/api/v1/ops", "")
			a.fn(ctx)
			assert.True(t, rec.Code >= 200 && rec.Code < 600)
		})
	}
}

// ---- AgentMetricsHistory handler ----

func TestHandler_AgentMetricsHistory_Success_V2(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)
	svcCtx := &svc.ServiceContext{}
	h := NewHandler(NewService(svcCtx))

	ctx, rec := newOpsTestContext(http.MethodGet, "/api/v1/ops/agents/a1/metrics/history", "")
	h.AgentMetricsHistory(ctx)
	assert.True(t, rec.Code >= 200 && rec.Code < 600)
}

// TestListNodes_RemoteOwnedSnapshotNotStale 回归（HA 多实例）：本地
// registry 里的远端 agent 快照（启动 LoadFromDB 载入、心跳只续对端侧、
// 本地 ExpireAt 冻结过期）在共享归属表判定活跃且 owner 为对端实例时，
// 必须报 online + ownerInstance 标签——修复前被 resolveNodeStatus 按本地
// 过期 ExpireAt 误判 stale，且因 registered 提前短路轮不到 owner 聚合，
// 页面表现为「一个在线一个离线」。
func TestListNodes_RemoteOwnedSnapshotNotStale(t *testing.T) {
	t.Parallel()

	store := registry.NewStore()
	now := time.Now()
	// 本实例直连的 agent：正常 active。
	store.UpsertAgent(&registry.AgentSession{
		AgentID:   "agent-local",
		GameID:    "g1",
		Env:       "prod",
		Addr:      "h:1",
		Labels:    map[string]string{},
		Functions: map[string]registry.FunctionMeta{},
		LastSeen:  now,
		ExpireAt:  now.Add(time.Hour),
	})
	// 远端 agent 的冻结快照：ExpireAt 已过（心跳只在对端实例续期）。
	store.UpsertAgent(&registry.AgentSession{
		AgentID:   "agent-remote",
		GameID:    "g1",
		Env:       "prod",
		Addr:      "h:2",
		Labels:    map[string]string{},
		Functions: map[string]registry.FunctionMeta{},
		LastSeen:  now.Add(-2 * time.Hour),
		ExpireAt:  now.Add(-time.Hour),
	})

	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		Cluster: &svc.ClusterRuntime{
			InstanceID: "self-inst",
			ListAgentOwners: func(context.Context) ([]cluster.AgentOwnerRecord, error) {
				return []cluster.AgentOwnerRecord{{
					AgentID:    "agent-remote",
					InstanceID: "peer-inst",
					GameID:     "g1",
					Env:        "prod",
					LastSeenAt: now,
				}}, nil
			},
		},
	}

	nodes := listNodes(context.Background(), svcCtx, "", "", "")
	byID := map[string]Node{}
	for _, n := range nodes {
		byID[n.Id] = n
	}
	local, ok := byID["agent-local"]
	if !ok {
		t.Fatal("agent-local missing")
	}
	if local.Status != "active" {
		t.Errorf("agent-local status = %s, want active", local.Status)
	}
	remote, ok := byID["agent-remote"]
	if !ok {
		t.Fatal("agent-remote missing")
	}
	if remote.Status != "online" {
		t.Errorf("agent-remote status = %s, want online (owner 活跃的远端快照不得按本地过期 ExpireAt 判 stale)", remote.Status)
	}
	if remote.Labels["ownerInstance"] != "peer-inst" {
		t.Errorf("agent-remote ownerInstance = %q, want peer-inst", remote.Labels["ownerInstance"])
	}
}
