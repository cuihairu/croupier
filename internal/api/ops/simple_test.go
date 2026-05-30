package ops

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// This file contains focused tests for the ops module that avoid panics
// from nil models by properly setting up the service context.

func TestNewService(t *testing.T) {
	t.Parallel()

	svcCtx := &svc.ServiceContext{}
	s := NewService(svcCtx)

	assert.NotNil(t, s)
	assert.Same(t, svcCtx, s.svcCtx)
}

// Agent operations tests with proper registry setup

func TestServiceOpsAgentsList(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := registry.NewStore()
	svcCtx := &svc.ServiceContext{RegistryStore: store}
	s := NewService(svcCtx)

	store.UpsertAgent(&registry.AgentSession{
		AgentID:   "agent-1",
		RPCAddr:   "localhost:1001",
		GameID:    "game1",
		Env:       "prod",
		Version:   "1.0.0",
		Labels:    map[string]string{"hostname": "host1"},
		Functions: map[string]registry.FunctionMeta{"func1": {Enabled: true}},
		LastSeen:  time.Now(),
	})

	resp, err := s.OpsAgentsList(ctx, &OpsAgentsListRequest{})
	require.NoError(t, err)
	assert.Equal(t, 0, resp.Code)
	assert.Equal(t, "Success", resp.Message)
	assert.Len(t, resp.Data, 1)
	assert.Equal(t, "agent-1", resp.Data[0].AgentID)
}

func TestServiceOpsAgentsListEmptyRegistry(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svcCtx := &svc.ServiceContext{RegistryStore: nil}
	s := NewService(svcCtx)

	resp, err := s.OpsAgentsList(ctx, &OpsAgentsListRequest{})
	require.NoError(t, err)
	assert.Equal(t, 0, resp.Code)
	assert.Empty(t, resp.Data)
}

func TestServiceOpsAgentMeta(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := registry.NewStore()
	svcCtx := &svc.ServiceContext{RegistryStore: store}
	s := NewService(svcCtx)

	store.UpsertAgent(&registry.AgentSession{
		AgentID:   "agent-1",
		RPCAddr:   "localhost:1001",
		GameID:    "game1",
		Env:       "prod",
		Labels:    map[string]string{"os": "linux", "arch": "amd64"},
		Functions: map[string]registry.FunctionMeta{},
		LastSeen:  time.Now(),
	})

	resp, err := s.OpsAgentMeta(ctx, &OpsAgentMetaRequest{AgentId: "agent-1"})
	require.NoError(t, err)
	assert.Equal(t, 0, resp.Code)
	data, ok := resp.Data.(OpsAgentSystemInfo)
	require.True(t, ok)
	assert.Equal(t, "linux", data.OS)
}

func TestServiceOpsAgentMetaNotFound(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svcCtx := &svc.ServiceContext{RegistryStore: registry.NewStore()}
	s := NewService(svcCtx)

	_, err := s.OpsAgentMeta(ctx, &OpsAgentMetaRequest{AgentId: "nonexistent"})
	assert.Error(t, err)
}

func TestServiceOpsAgentSystemInfo(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := registry.NewStore()
	svcCtx := &svc.ServiceContext{RegistryStore: store}
	s := NewService(svcCtx)

	store.UpsertAgent(&registry.AgentSession{
		AgentID:   "agent-1",
		RPCAddr:   "localhost:1001",
		GameID:    "game1",
		Env:       "prod",
		Labels:    map[string]string{"os": "linux"},
		Functions: map[string]registry.FunctionMeta{},
		LastSeen:  time.Now(),
	})

	resp, err := s.OpsAgentSystemInfo(ctx, &OpsAgentSystemInfoRequest{AgentID: "agent-1"})
	require.NoError(t, err)
	assert.Equal(t, 0, resp.Code)
	assert.Equal(t, "linux", resp.Data.OS)
}

func TestServiceOpsAgentMetrics(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svcCtx := &svc.ServiceContext{}
	s := NewService(svcCtx)

	resp, err := s.OpsAgentMetrics(ctx, &OpsAgentMetricsRequest{})
	require.NoError(t, err)
	assert.Equal(t, 0, resp.Code)
	assert.Empty(t, resp.Data)
}

func TestServiceOpsAgentProcesses(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svcCtx := &svc.ServiceContext{}
	s := NewService(svcCtx)

	resp, err := s.OpsAgentProcesses(ctx, &OpsAgentProcessesRequest{})
	require.NoError(t, err)
	assert.Equal(t, 0, resp.Code)
	assert.Empty(t, resp.Data)
}

func TestServiceOpsAgentProcessStart(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svcCtx := &svc.ServiceContext{}
	s := NewService(svcCtx)

	resp, err := s.OpsAgentProcessStart(ctx, &OpsProcessStartRequest{})
	require.NoError(t, err)
	assert.Equal(t, 0, resp.Code)
	assert.Contains(t, resp.Message, "not implemented")
}

func TestServiceOpsAgentProcessStop(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svcCtx := &svc.ServiceContext{}
	s := NewService(svcCtx)

	resp, err := s.OpsAgentProcessStop(ctx, &OpsProcessActionRequest{})
	require.NoError(t, err)
	assert.Equal(t, 0, resp.Code)
	assert.Contains(t, resp.Message, "not implemented")
}

func TestServiceOpsAgentProcessRestart(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svcCtx := &svc.ServiceContext{}
	s := NewService(svcCtx)

	resp, err := s.OpsAgentProcessRestart(ctx, &OpsProcessActionRequest{})
	require.NoError(t, err)
	assert.Equal(t, 0, resp.Code)
	assert.Contains(t, resp.Message, "not implemented")
}

// Node operations tests

func TestServiceOpsNodes(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := registry.NewStore()
	svcCtx := &svc.ServiceContext{RegistryStore: store}
	s := NewService(svcCtx)

	store.UpsertAgent(&registry.AgentSession{
		AgentID:   "node-1",
		RPCAddr:   "localhost:2001",
		GameID:    "game1",
		Env:       "prod",
		Labels:    map[string]string{"hostname": "node1"},
		Functions: map[string]registry.FunctionMeta{},
		LastSeen:  time.Now(),
	})

	resp, err := s.OpsNodes(ctx, &OpsNodesRequest{})
	require.NoError(t, err)
	assert.Len(t, resp.Nodes, 1)
}

func TestServiceOpsNodesEmptyRegistry(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svcCtx := &svc.ServiceContext{RegistryStore: nil}
	s := NewService(svcCtx)

	resp, err := s.OpsNodes(ctx, &OpsNodesRequest{})
	require.NoError(t, err)
	assert.Empty(t, resp.Nodes)
}

func TestServiceOpsNodeCommands(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svcCtx := &svc.ServiceContext{}
	s := NewService(svcCtx)

	resp, err := s.OpsNodeCommands(ctx, &OpsNodeCommandsRequest{})
	require.NoError(t, err)
	assert.Equal(t, 0, resp.Code)
	data, ok := resp.Data.([]NodeCommand)
	require.True(t, ok)
	assert.Len(t, data, 3) // drain, undrain, restart
}

func TestServiceOpsNodeDrain(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svcCtx := &svc.ServiceContext{}
	s := NewService(svcCtx)

	resp, err := s.OpsNodeDrain(ctx, &OpsNodeCommandsRequest{})
	require.NoError(t, err)
	assert.Equal(t, 0, resp.Code)
	assert.Contains(t, resp.Message, "drained")
}

func TestServiceOpsNodeMeta(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := registry.NewStore()
	svcCtx := &svc.ServiceContext{RegistryStore: store}
	s := NewService(svcCtx)

	store.UpsertAgent(&registry.AgentSession{
		AgentID:   "node-1",
		RPCAddr:   "localhost:2001",
		GameID:    "game1",
		Env:       "prod",
		Labels:    map[string]string{"zone": "us-east-1"},
		Functions: map[string]registry.FunctionMeta{},
		LastSeen:  time.Now(),
	})

	resp, err := s.OpsNodeMeta(ctx, &OpsNodeMetaRequest{NodeID: "node-1"})
	require.NoError(t, err)
	assert.Equal(t, 0, resp.Code)
	data, ok := resp.Data.(map[string]string)
	require.True(t, ok)
	assert.Equal(t, "us-east-1", data["zone"])
}

func TestServiceOpsNodeMetaNotFound(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svcCtx := &svc.ServiceContext{RegistryStore: registry.NewStore()}
	s := NewService(svcCtx)

	_, err := s.OpsNodeMeta(ctx, &OpsNodeMetaRequest{NodeID: "nonexistent"})
	assert.Error(t, err)
}

func TestServiceOpsNodeRestart(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svcCtx := &svc.ServiceContext{}
	s := NewService(svcCtx)

	resp, err := s.OpsNodeRestart(ctx, &OpsNodeCommandsRequest{})
	require.NoError(t, err)
	assert.Equal(t, 0, resp.Code)
	assert.Contains(t, resp.Message, "restart")
}

func TestServiceOpsNodeUndrain(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svcCtx := &svc.ServiceContext{}
	s := NewService(svcCtx)

	resp, err := s.OpsNodeUndrain(ctx, &OpsNodeCommandsRequest{})
	require.NoError(t, err)
	assert.Equal(t, 0, resp.Code)
	assert.Contains(t, resp.Message, "undrained")
}

// Health operations tests

func TestServiceOpsHealthGet(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svcCtx := &svc.ServiceContext{}
	s := NewService(svcCtx)

	resp, err := s.OpsHealthGet(ctx, &OpsHealthGetRequest{})
	require.NoError(t, err)
	assert.Equal(t, 0, resp.Code)
	assert.Empty(t, resp.Data)
}

func TestServiceOpsHealthRun(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svcCtx := &svc.ServiceContext{}
	s := NewService(svcCtx)

	resp, err := s.OpsHealthRun(ctx, &OpsHealthRunRequest{})
	require.NoError(t, err)
	assert.Equal(t, 0, resp.Code)
	assert.Empty(t, resp.Data)
}

func TestServiceOpsHealthUpdate(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svcCtx := &svc.ServiceContext{}
	s := NewService(svcCtx)

	resp, err := s.OpsHealthUpdate(ctx, &OpsHealthUpdateRequest{})
	require.NoError(t, err)
	assert.Equal(t, 0, resp.Code)
	assert.Contains(t, resp.Message, "updated")
}

// Maintenance operations tests

func TestServiceOpsMaintenanceGet(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svcCtx := &svc.ServiceContext{}
	s := NewService(svcCtx)

	resp, err := s.OpsMaintenanceGet(ctx, &OpsMaintenanceGetRequest{})
	require.NoError(t, err)
	assert.Equal(t, 0, resp.Code)
	assert.Empty(t, resp.Data)
}

func TestServiceOpsMaintenanceUpdate(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svcCtx := &svc.ServiceContext{}
	s := NewService(svcCtx)

	resp, err := s.OpsMaintenanceUpdate(ctx, &OpsMaintenanceUpdateRequest{})
	require.NoError(t, err)
	assert.Equal(t, 0, resp.Code)
	assert.Contains(t, resp.Message, "updated")
}

// Metrics tests

func TestServiceOpsMetrics(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svcCtx := &svc.ServiceContext{}
	s := NewService(svcCtx)

	resp, err := s.OpsMetrics(ctx, &OpsMetricsRequest{})
	require.NoError(t, err)
	assert.Equal(t, 0, resp.Code)
	assert.Empty(t, resp.Data)
}

// Config tests

func TestServiceOpsConfig(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svcCtx := &svc.ServiceContext{}
	s := NewService(svcCtx)

	resp, err := s.OpsConfig(ctx, &OpsConfigRequest{})
	require.NoError(t, err)
	assert.Empty(t, resp.AlertmanagerURL)
	assert.Empty(t, resp.GrafanaExploreURL)
	assert.Empty(t, resp.JaegerURL)
}

// Services tests

func TestServiceOpsServices(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svcCtx := &svc.ServiceContext{}
	s := NewService(svcCtx)

	resp, err := s.OpsServices(ctx, &OpsServicesRequest{})
	require.NoError(t, err)
	require.Len(t, resp.Services, 1)
	assert.Equal(t, "server", resp.Services[0].ID)
	assert.Equal(t, 1, resp.Total)
}

// Functions tests

func TestServiceOpsFunctions(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := registry.NewStore()
	svcCtx := &svc.ServiceContext{RegistryStore: store}
	s := NewService(svcCtx)

	store.UpsertAgent(&registry.AgentSession{
		AgentID:   "agent-1",
		RPCAddr:   "localhost:1001",
		GameID:    "game1",
		Env:       "prod",
		Functions: map[string]registry.FunctionMeta{"func1": {Enabled: true}},
		LastSeen:  time.Now(),
	})

	resp, err := s.OpsFunctions(ctx, &OpsFunctionsRequest{})
	require.NoError(t, err)
	assert.Equal(t, 0, resp.Code)
	data, ok := resp.Data.(map[string][]string)
	require.True(t, ok)
	assert.Len(t, data["func1"], 1)
}

func TestServiceOpsFunctionsEmptyRegistry(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svcCtx := &svc.ServiceContext{RegistryStore: nil}
	s := NewService(svcCtx)

	resp, err := s.OpsFunctions(ctx, &OpsFunctionsRequest{})
	require.NoError(t, err)
	assert.Equal(t, 0, resp.Code)
	data, ok := resp.Data.(map[string][]string)
	require.True(t, ok)
	assert.Empty(t, data)
}

// MQ tests

func TestServiceOpsMQ(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svcCtx := &svc.ServiceContext{}
	s := NewService(svcCtx)

	resp, err := s.OpsMQ(ctx, &OpsMQRequest{})
	require.NoError(t, err)
	assert.Equal(t, 0, resp.Code)
	assert.Empty(t, resp.Data)
}

// Notifications tests

func TestServiceOpsNotificationsGet(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svcCtx := &svc.ServiceContext{}
	s := NewService(svcCtx)

	resp, err := s.OpsNotificationsGet(ctx, &OpsNotificationsGetRequest{})
	require.NoError(t, err)
	assert.False(t, resp.Enabled)
}

func TestServiceOpsNotificationsUpdate(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svcCtx := &svc.ServiceContext{}
	s := NewService(svcCtx)

	resp, err := s.OpsNotificationsUpdate(ctx, &OpsNotificationsUpdateRequest{
		Enabled:  true,
		Channels: []OpsNotificationChannel{},
		Rules:    []OpsNotificationRule{},
	})
	require.NoError(t, err)
	assert.Equal(t, 0, resp.Code)
}

func TestServiceOpsNotificationsUpdateNilRequest(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svcCtx := &svc.ServiceContext{}
	s := NewService(svcCtx)

	resp, err := s.OpsNotificationsUpdate(ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, 0, resp.Code)
}

// Backup operations - safe tests only

func TestServiceOpsBackupDelete(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svcCtx := &svc.ServiceContext{}
	s := NewService(svcCtx)

	resp, err := s.OpsBackupDelete(ctx, &OpsBackupDeleteRequest{})
	require.NoError(t, err)
	data, ok := resp.Data.(bool)
	require.True(t, ok)
	assert.True(t, data)
}

func TestServiceOpsBackupDownload(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svcCtx := &svc.ServiceContext{}
	s := NewService(svcCtx)

	resp, err := s.OpsBackupDownload(ctx, &OpsBackupDownloadRequest{})
	require.NoError(t, err)
	data, ok := resp.Data.(string)
	require.True(t, ok)
	assert.Contains(t, data, "/backups/")
}

// Alert silence delete test - safe path

func TestServiceOpsSilenceDelete(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svcCtx := &svc.ServiceContext{}
	s := NewService(svcCtx)

	resp, err := s.OpsSilenceDelete(ctx, &OpsAlertSilenceRequest{AlertID: "invalid"})
	require.NoError(t, err)
	assert.Equal(t, 1, resp.Code)
}

// Sub-service tests

func TestNewAgentService(t *testing.T) {
	t.Parallel()

	svcCtx := &svc.ServiceContext{}
	s := NewAgentService(svcCtx)

	assert.NotNil(t, s)
	assert.Same(t, svcCtx, s.svcCtx)
}

func TestAgentServiceList(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := registry.NewStore()
	svcCtx := &svc.ServiceContext{RegistryStore: store}
	s := NewAgentService(svcCtx)

	store.UpsertAgent(&registry.AgentSession{
		AgentID:   "agent-1",
		RPCAddr:   "localhost:1001",
		GameID:    "game1",
		Env:       "prod",
		Functions: map[string]registry.FunctionMeta{},
		LastSeen:  time.Now(),
	})

	agents, err := s.List(ctx, "", "", "")
	require.NoError(t, err)
	assert.Len(t, agents, 1)
}

func TestAgentServiceListFilterByGameID(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := registry.NewStore()
	svcCtx := &svc.ServiceContext{RegistryStore: store}
	s := NewAgentService(svcCtx)

	store.UpsertAgent(&registry.AgentSession{
		AgentID:   "agent-1",
		RPCAddr:   "localhost:1001",
		GameID:    "game1",
		Env:       "prod",
		Functions: map[string]registry.FunctionMeta{},
		LastSeen:  time.Now(),
	})
	store.UpsertAgent(&registry.AgentSession{
		AgentID:   "agent-2",
		RPCAddr:   "localhost:1002",
		GameID:    "game2",
		Env:       "prod",
		Functions: map[string]registry.FunctionMeta{},
		LastSeen:  time.Now(),
	})

	agents, err := s.List(ctx, "game1", "", "")
	require.NoError(t, err)
	assert.Len(t, agents, 1)
}

func TestAgentServiceGetMeta(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := registry.NewStore()
	svcCtx := &svc.ServiceContext{RegistryStore: store}
	s := NewAgentService(svcCtx)

	store.UpsertAgent(&registry.AgentSession{
		AgentID:   "agent-1",
		RPCAddr:   "localhost:1001",
		GameID:    "game1",
		Env:       "prod",
		Labels:    map[string]string{"os": "linux"},
		Functions: map[string]registry.FunctionMeta{},
		LastSeen:  time.Now(),
	})

	meta, err := s.GetMeta(ctx, "agent-1")
	require.NoError(t, err)
	assert.Equal(t, "linux", meta.OS)
}

func TestAgentServiceGetMetaNotFound(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svcCtx := &svc.ServiceContext{RegistryStore: registry.NewStore()}
	s := NewAgentService(svcCtx)

	_, err := s.GetMeta(ctx, "nonexistent")
	assert.Error(t, err)
}

func TestNewNodeService(t *testing.T) {
	t.Parallel()

	svcCtx := &svc.ServiceContext{}
	s := NewNodeService(svcCtx)

	assert.NotNil(t, s)
	assert.Same(t, svcCtx, s.svcCtx)
}

func TestNodeServiceList(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := registry.NewStore()
	svcCtx := &svc.ServiceContext{RegistryStore: store}
	s := NewNodeService(svcCtx)

	store.UpsertAgent(&registry.AgentSession{
		AgentID:   "node-1",
		RPCAddr:   "localhost:2001",
		GameID:    "game1",
		Env:       "prod",
		Functions: map[string]registry.FunctionMeta{},
		LastSeen:  time.Now(),
	})

	nodes, err := s.List(ctx, "", "", "")
	require.NoError(t, err)
	assert.Len(t, nodes, 1)
}

func TestNodeServiceGetCommands(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svcCtx := &svc.ServiceContext{RegistryStore: registry.NewStore()}
	s := NewNodeService(svcCtx)

	commands, err := s.GetCommands(ctx, "node-1")
	require.NoError(t, err)
	assert.Len(t, commands, 3)
}

func TestNodeServiceGetMeta(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := registry.NewStore()
	svcCtx := &svc.ServiceContext{RegistryStore: store}
	s := NewNodeService(svcCtx)

	store.UpsertAgent(&registry.AgentSession{
		AgentID:   "node-1",
		RPCAddr:   "localhost:2001",
		GameID:    "game1",
		Env:       "prod",
		Labels:    map[string]string{"zone": "us-east-1"},
		Functions: map[string]registry.FunctionMeta{},
		LastSeen:  time.Now(),
	})

	meta, err := s.GetMeta(ctx, "node-1")
	require.NoError(t, err)
	assert.Equal(t, "us-east-1", meta["zone"])
}

func TestNodeServiceGetMetaNotFound(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svcCtx := &svc.ServiceContext{RegistryStore: registry.NewStore()}
	s := NewNodeService(svcCtx)

	_, err := s.GetMeta(ctx, "nonexistent")
	assert.Error(t, err)
}

func TestNewBackupService(t *testing.T) {
	t.Parallel()

	svcCtx := &svc.ServiceContext{}
	s := NewBackupService(svcCtx)

	assert.NotNil(t, s)
	assert.Same(t, svcCtx, s.svcCtx)
}

func TestNewAlertService(t *testing.T) {
	t.Parallel()

	svcCtx := &svc.ServiceContext{}
	s := NewAlertService(svcCtx)

	assert.NotNil(t, s)
	assert.Same(t, svcCtx, s.svcCtx)
}

// Handler tests with proper setup

func TestNewHandler(t *testing.T) {
	t.Parallel()

	svcCtx := &svc.ServiceContext{}
	s := NewService(svcCtx)
	h := NewHandler(s)

	assert.NotNil(t, h)
	assert.Same(t, s, h.service)
}

// Handler tests that use GET requests (no body, safe with nil models)

func TestOpsAgentsListHandler(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	store := registry.NewStore()
	svcCtx := &svc.ServiceContext{RegistryStore: store}
	h := NewHandler(NewService(svcCtx))
	ctx, rec := newOpsTestContext(http.MethodGet, "/api/v1/ops/agents", "")

	h.OpsAgentsList(ctx)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "Success")
}

func TestOpsNodesHandler(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	store := registry.NewStore()
	svcCtx := &svc.ServiceContext{RegistryStore: store}
	h := NewHandler(NewService(svcCtx))
	ctx, rec := newOpsTestContext(http.MethodGet, "/api/v1/ops/nodes", "")

	h.OpsNodes(ctx)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestOpsHealthGetHandler(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	svcCtx := &svc.ServiceContext{}
	h := NewHandler(NewService(svcCtx))
	ctx, rec := newOpsTestContext(http.MethodGet, "/api/v1/ops/health", "")

	h.OpsHealthGet(ctx)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestOpsMaintenanceGetHandler(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	svcCtx := &svc.ServiceContext{}
	h := NewHandler(NewService(svcCtx))
	ctx, rec := newOpsTestContext(http.MethodGet, "/api/v1/ops/maintenance", "")

	h.OpsMaintenanceGet(ctx)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestOpsMetricsHandler(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	svcCtx := &svc.ServiceContext{}
	h := NewHandler(NewService(svcCtx))
	ctx, rec := newOpsTestContext(http.MethodGet, "/api/v1/ops/metrics?gameId=test&env=prod", "")

	h.OpsMetrics(ctx)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestOpsConfigHandler(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	svcCtx := &svc.ServiceContext{}
	h := NewHandler(NewService(svcCtx))
	ctx, rec := newOpsTestContext(http.MethodGet, "/api/v1/ops/config", "")

	h.OpsConfig(ctx)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestOpsNotificationsGetHandler(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	svcCtx := &svc.ServiceContext{}
	h := NewHandler(NewService(svcCtx))
	ctx, rec := newOpsTestContext(http.MethodGet, "/api/v1/ops/notifications", "")

	h.OpsNotificationsGet(ctx)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestOpsServicesHandler(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	svcCtx := &svc.ServiceContext{}
	h := NewHandler(NewService(svcCtx))
	ctx, rec := newOpsTestContext(http.MethodGet, "/api/v1/ops/services", "")

	h.OpsServices(ctx)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestOpsFunctionsHandler(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	store := registry.NewStore()
	svcCtx := &svc.ServiceContext{RegistryStore: store}
	h := NewHandler(NewService(svcCtx))
	ctx, rec := newOpsTestContext(http.MethodGet, "/api/v1/ops/functions", "")

	h.OpsFunctions(ctx)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestOpsMQHandler(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	svcCtx := &svc.ServiceContext{}
	h := NewHandler(NewService(svcCtx))
	ctx, rec := newOpsTestContext(http.MethodGet, "/api/v1/ops/mq", "")

	h.OpsMQ(ctx)

	assert.Equal(t, http.StatusOK, rec.Code)
}

// Handler tests for POST requests with simple implementations

func TestOpsNodeDrainHandler(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	svcCtx := &svc.ServiceContext{}
	h := NewHandler(NewService(svcCtx))
	ctx, rec := newOpsTestContext(http.MethodPost, "/api/v1/ops/node/drain", `{"nodeId":"node-1"}`)

	h.OpsNodeDrain(ctx)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "drained")
}

func TestOpsNodeRestartHandler(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	svcCtx := &svc.ServiceContext{}
	h := NewHandler(NewService(svcCtx))
	ctx, rec := newOpsTestContext(http.MethodPost, "/api/v1/ops/node/restart", `{"nodeId":"node-1"}`)

	h.OpsNodeRestart(ctx)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "restart")
}

func TestOpsNodeUndrainHandler(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	svcCtx := &svc.ServiceContext{}
	h := NewHandler(NewService(svcCtx))
	ctx, rec := newOpsTestContext(http.MethodPost, "/api/v1/ops/node/undrain", `{"nodeId":"node-1"}`)

	h.OpsNodeUndrain(ctx)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "undrained")
}

func TestOpsNodeCommandsHandler(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	svcCtx := &svc.ServiceContext{}
	h := NewHandler(NewService(svcCtx))
	ctx, rec := newOpsTestContext(http.MethodPost, "/api/v1/ops/node/commands", `{"nodeId":"node-1"}`)

	h.OpsNodeCommands(ctx)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "Success")
}

func TestOpsHealthRunHandler(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	svcCtx := &svc.ServiceContext{}
	h := NewHandler(NewService(svcCtx))
	ctx, rec := newOpsTestContext(http.MethodPost, "/api/v1/ops/health/run", `{"id":"check-1"}`)

	h.OpsHealthRun(ctx)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestOpsHealthUpdateHandler(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	svcCtx := &svc.ServiceContext{}
	h := NewHandler(NewService(svcCtx))
	ctx, rec := newOpsTestContext(http.MethodPost, "/api/v1/ops/health/update", `{"enabled":true,"checks":[]}`)

	h.OpsHealthUpdate(ctx)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestOpsMaintenanceUpdateHandler(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	svcCtx := &svc.ServiceContext{}
	h := NewHandler(NewService(svcCtx))
	ctx, rec := newOpsTestContext(http.MethodPost, "/api/v1/ops/maintenance/update", `{"enabled":true,"windows":[]}`)

	h.OpsMaintenanceUpdate(ctx)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestOpsNotificationsUpdateHandler(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	svcCtx := &svc.ServiceContext{}
	h := NewHandler(NewService(svcCtx))
	ctx, rec := newOpsTestContext(http.MethodPost, "/api/v1/ops/notifications/update", `{"enabled":true,"channels":[],"rules":[]}`)

	h.OpsNotificationsUpdate(ctx)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestOpsBackupDeleteHandler(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	svcCtx := &svc.ServiceContext{}
	h := NewHandler(NewService(svcCtx))
	ctx, rec := newOpsTestContext(http.MethodPost, "/api/v1/ops/backup/delete", `{"id":"backup-1"}`)

	h.OpsBackupDelete(ctx)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestOpsBackupDownloadHandler(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	svcCtx := &svc.ServiceContext{}
	h := NewHandler(NewService(svcCtx))
	ctx, rec := newOpsTestContext(http.MethodPost, "/api/v1/ops/backup/download", `{"id":"backup-1"}`)

	h.OpsBackupDownload(ctx)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "/backups/")
}

// Agent handlers with proper registry setup

func TestOpsAgentProcessesHandler(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	svcCtx := &svc.ServiceContext{}
	h := NewHandler(NewService(svcCtx))
	ctx, rec := newOpsTestContext(http.MethodGet, "/api/v1/ops/agent/processes", "")

	h.OpsAgentProcesses(ctx)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestOpsAgentMetricsHandler(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	svcCtx := &svc.ServiceContext{}
	h := NewHandler(NewService(svcCtx))
	ctx, rec := newOpsTestContext(http.MethodGet, "/api/v1/ops/agent/metrics?agentId=agent-1", "")

	h.OpsAgentMetrics(ctx)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestOpsAgentProcessStartHandler(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	svcCtx := &svc.ServiceContext{}
	h := NewHandler(NewService(svcCtx))
	ctx, rec := newOpsTestContext(http.MethodGet, "/api/v1/ops/agent/process/start", "")

	h.OpsAgentProcessStart(ctx)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestOpsAgentProcessStopHandler(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	svcCtx := &svc.ServiceContext{}
	h := NewHandler(NewService(svcCtx))
	ctx, rec := newOpsTestContext(http.MethodGet, "/api/v1/ops/agent/process/stop", "")

	h.OpsAgentProcessStop(ctx)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestOpsAgentProcessRestartHandler(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	svcCtx := &svc.ServiceContext{}
	h := NewHandler(NewService(svcCtx))
	ctx, rec := newOpsTestContext(http.MethodGet, "/api/v1/ops/agent/process/restart", "")

	h.OpsAgentProcessRestart(ctx)

	assert.Equal(t, http.StatusOK, rec.Code)
}

// Alias handler tests

func TestHandlerAliasMethods(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	svcCtx := &svc.ServiceContext{RegistryStore: registry.NewStore()}
	h := NewHandler(NewService(svcCtx))

	cases := []struct {
		name string
		fn   func(*gin.Context)
	}{
		// Original aliases (already tested)
		{name: "AgentsList", fn: h.AgentsList},
		{name: "Metrics", fn: h.Metrics},
		{name: "MQ", fn: h.MQ},
		{name: "Services", fn: h.Services},
		{name: "Functions", fn: h.Functions},
		{name: "Config", fn: h.Config},
		// Additional alias methods that don't require DB models
		{name: "AgentMeta", fn: h.AgentMeta},
		{name: "AgentMetrics", fn: h.AgentMetrics},
		{name: "AgentProcesses", fn: h.AgentProcesses},
		{name: "AgentSystemInfo", fn: h.AgentSystemInfo},
		{name: "AgentProcessStart", fn: h.AgentProcessStart},
		{name: "AgentProcessStop", fn: h.AgentProcessStop},
		{name: "AgentProcessRestart", fn: h.AgentProcessRestart},
		{name: "AgentExecCommand", fn: h.AgentExecCommand},
		{name: "Nodes", fn: h.Nodes},
		{name: "NodeCommands", fn: h.NodeCommands},
		{name: "NodeDrain", fn: h.NodeDrain},
		{name: "NodeMeta", fn: h.NodeMeta},
		{name: "NodeRestart", fn: h.NodeRestart},
		{name: "NodeUndrain", fn: h.NodeUndrain},
		{name: "HealthGet", fn: h.HealthGet},
		{name: "HealthRun", fn: h.HealthRun},
		{name: "HealthUpdate", fn: h.HealthUpdate},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx, rec := newOpsTestContext(http.MethodGet, "/api/v1/ops", "")
			tc.fn(ctx)
			// Should not panic - status codes 200-599 are acceptable for this test
			assert.True(t, rec.Code >= 200 && rec.Code <= 599, "expected status code between 200-599, got %d", rec.Code)
		})
	}
}
