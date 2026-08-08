package ops

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
	"gorm.io/gorm"
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
		Addr:      "localhost:1001",
		GameID:    "game1",
		Env:       "prod",
		Version:   "1.0.0",
		Labels:    map[string]string{"hostname": "host1"},
		Functions: map[string]registry.FunctionMeta{"func1": {Enabled: true}},
		LastSeen:  time.Now(),
	})

	resp, err := s.OpsAgentsList(ctx, &OpsAgentsListRequest{})
	require.NoError(t, err)
	assert.Len(t, resp.Agents, 1)
	assert.Equal(t, "agent-1", resp.Agents[0].AgentID)
}

func TestServiceOpsAgentsListEmptyRegistry(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svcCtx := &svc.ServiceContext{RegistryStore: nil}
	s := NewService(svcCtx)

	resp, err := s.OpsAgentsList(ctx, &OpsAgentsListRequest{})
	require.NoError(t, err)
	assert.Empty(t, resp.Agents)
}

func TestServiceOpsAgentMeta(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := registry.NewStore()
	svcCtx := &svc.ServiceContext{RegistryStore: store}
	s := NewService(svcCtx)

	store.UpsertAgent(&registry.AgentSession{
		AgentID:   "agent-1",
		Addr:      "localhost:1001",
		GameID:    "game1",
		Env:       "prod",
		Labels:    map[string]string{"os": "linux", "arch": "amd64"},
		Functions: map[string]registry.FunctionMeta{},
		LastSeen:  time.Now(),
	})

	resp, err := s.OpsAgentMeta(ctx, &OpsAgentMetaRequest{AgentId: "agent-1"})
	require.NoError(t, err)
	data, ok := resp.Meta.(OpsAgentSystemInfo)
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
		Addr:      "localhost:1001",
		GameID:    "game1",
		Env:       "prod",
		Labels:    map[string]string{"os": "linux"},
		Functions: map[string]registry.FunctionMeta{},
		LastSeen:  time.Now(),
	})

	resp, err := s.OpsAgentSystemInfo(ctx, &OpsAgentSystemInfoRequest{AgentID: "agent-1"})
	require.NoError(t, err)
	assert.Equal(t, "linux", resp.SystemInfo.OS)
}

func TestServiceOpsAgentMetrics(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svcCtx := &svc.ServiceContext{}
	s := NewService(svcCtx)

	resp, err := s.OpsAgentMetrics(ctx, &OpsAgentMetricsRequest{})
	require.NoError(t, err)
	assert.Empty(t, resp.Metrics)
}

func TestServiceOpsAgentProcesses(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svcCtx := &svc.ServiceContext{}
	s := NewService(svcCtx)

	resp, err := s.OpsAgentProcesses(ctx, &OpsAgentProcessesRequest{})
	require.NoError(t, err)
	assert.Empty(t, resp.Processes)
}

func TestServiceOpsAgentProcessStart(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svcCtx := &svc.ServiceContext{}
	s := NewService(svcCtx)

	_, err := s.OpsAgentProcessStart(ctx, &OpsProcessStartRequest{})
	require.Error(t, err)
}

func TestServiceOpsAgentProcessStop(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svcCtx := &svc.ServiceContext{}
	s := NewService(svcCtx)

	_, err := s.OpsAgentProcessStop(ctx, &OpsProcessActionRequest{})
	require.Error(t, err)
}

func TestServiceOpsAgentProcessRestart(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svcCtx := &svc.ServiceContext{}
	s := NewService(svcCtx)

	_, err := s.OpsAgentProcessRestart(ctx, &OpsProcessActionRequest{})
	require.Error(t, err)
}

// Node operations tests

func TestServiceOpsNodes(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := registry.NewStore()
	opsStateStore := svc.NewOpsStateStore(t.TempDir())
	svcCtx := &svc.ServiceContext{RegistryStore: store, OpsStateStore: opsStateStore}
	s := NewService(svcCtx)

	now := time.Now()
	store.UpsertAgent(&registry.AgentSession{
		AgentID:   "node-active",
		Addr:      "localhost:2001",
		GameID:    "game1",
		Env:       "prod",
		Labels:    map[string]string{"hostname": "node-active"},
		Functions: map[string]registry.FunctionMeta{},
		LastSeen:  now,
		ExpireAt:  now.Add(time.Hour),
	})
	store.UpsertAgent(&registry.AgentSession{
		AgentID:   "node-drained",
		Addr:      "localhost:2002",
		GameID:    "game1",
		Env:       "prod",
		Labels:    map[string]string{"hostname": "node-drained"},
		Functions: map[string]registry.FunctionMeta{},
		LastSeen:  now.Add(-time.Minute),
		ExpireAt:  now.Add(time.Hour),
	})
	store.UpsertAgent(&registry.AgentSession{
		AgentID:   "node-stale",
		GameID:    "game1",
		Env:       "prod",
		Labels:    map[string]string{"hostname": "node-stale"},
		Functions: map[string]registry.FunctionMeta{},
		LastSeen:  now.Add(-2 * time.Hour),
		ExpireAt:  now.Add(time.Hour),
	})
	_, err := opsStateStore.Update(func(state *svc.OpsState) {
		if state.Nodes.Drained == nil {
			state.Nodes.Drained = make(map[string]time.Time)
		}
		state.Nodes.Drained["node-drained"] = now
	})
	require.NoError(t, err)

	resp, err := s.OpsNodes(ctx, &OpsNodesRequest{})
	require.NoError(t, err)
	require.Len(t, resp.Nodes, 3)
	assert.Equal(t, "node-active", resp.Nodes[0].Id)
	assert.Equal(t, "active", resp.Nodes[0].Status)
	assert.Equal(t, "node-drained", resp.Nodes[1].Id)
	assert.Equal(t, "drained", resp.Nodes[1].Status)
	assert.Equal(t, "node-stale", resp.Nodes[2].Id)
	assert.Equal(t, "stale", resp.Nodes[2].Status)
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

func TestServiceOpsNodesDatabaseOnlyNodeIsOffline(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Node{}))

	nodeModel := model.NewNodeModel(db)
	require.NoError(t, nodeModel.Upsert(ctx, &model.Node{
		NodeID: "node-db-only",
		Name:   "DB Only",
		Type:   "agent",
		Status: "active",
		IP:     "127.0.0.1",
		Port:   2001,
		Meta: datatypes.JSONMap{
			"gameId":   "game1",
			"env":      "prod",
			"hostname": "db-only-host",
		},
	}))

	svcCtx := &svc.ServiceContext{
		RegistryStore: registry.NewStore(),
		NodeModel:     nodeModel,
	}
	s := NewService(svcCtx)

	resp, err := s.OpsNodes(ctx, &OpsNodesRequest{})
	require.NoError(t, err)
	require.Len(t, resp.Nodes, 1)
	assert.Equal(t, "node-db-only", resp.Nodes[0].Id)
	assert.Equal(t, "offline", resp.Nodes[0].Status)
	assert.Equal(t, "127.0.0.1:2001", resp.Nodes[0].Addr)
	assert.Equal(t, "game1", resp.Nodes[0].GameId)
	assert.Equal(t, "prod", resp.Nodes[0].Env)
}

func TestServiceOpsNodesRegistryOverridesDatabaseStatus(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Node{}))

	nodeModel := model.NewNodeModel(db)
	require.NoError(t, nodeModel.Upsert(ctx, &model.Node{
		NodeID: "node-registered",
		Name:   "Registered",
		Type:   "agent",
		Status: "offline",
		IP:     "127.0.0.1",
		Port:   2001,
	}))

	store := registry.NewStore()
	now := time.Now()
	store.UpsertAgent(&registry.AgentSession{
		AgentID:   "node-registered",
		Addr:      "localhost:2001",
		GameID:    "game1",
		Env:       "prod",
		Labels:    map[string]string{"hostname": "runtime-host"},
		Functions: map[string]registry.FunctionMeta{},
		LastSeen:  now,
		ExpireAt:  now.Add(time.Hour),
	})

	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		NodeModel:     nodeModel,
	}
	s := NewService(svcCtx)

	resp, err := s.OpsNodes(ctx, &OpsNodesRequest{})
	require.NoError(t, err)
	require.Len(t, resp.Nodes, 1)
	assert.Equal(t, "node-registered", resp.Nodes[0].Id)
	assert.Equal(t, "active", resp.Nodes[0].Status)
	assert.Equal(t, "runtime-host", resp.Nodes[0].Hostname)
}

func TestServiceOpsNodeCommands(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svcCtx := &svc.ServiceContext{}
	s := NewService(svcCtx)

	resp, err := s.OpsNodeCommands(ctx, &OpsNodeCommandsRequest{})
	require.NoError(t, err)
	assert.Len(t, resp.Commands, 3) // drain, undrain, restart
}

func TestServiceOpsNodeDrain(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svcCtx := &svc.ServiceContext{}
	s := NewService(svcCtx)

	_, err := s.OpsNodeDrain(ctx, &OpsNodeCommandsRequest{NodeId: "node-1"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "session resolver unavailable")
}

func TestServiceOpsNodeMeta(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := registry.NewStore()
	svcCtx := &svc.ServiceContext{RegistryStore: store}
	s := NewService(svcCtx)

	store.UpsertAgent(&registry.AgentSession{
		AgentID:   "node-1",
		Addr:      "localhost:2001",
		GameID:    "game1",
		Env:       "prod",
		Labels:    map[string]string{"zone": "us-east-1"},
		Functions: map[string]registry.FunctionMeta{},
		LastSeen:  time.Now(),
	})

	resp, err := s.OpsNodeMeta(ctx, &OpsNodeMetaRequest{NodeID: "node-1"})
	require.NoError(t, err)
	assert.Equal(t, "us-east-1", resp.Labels["zone"])
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

	_, err := s.OpsNodeRestart(ctx, &OpsNodeCommandsRequest{NodeId: "node-1"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "session resolver unavailable")
}

func TestServiceOpsNodeUndrain(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svcCtx := &svc.ServiceContext{}
	s := NewService(svcCtx)

	_, err := s.OpsNodeUndrain(ctx, &OpsNodeCommandsRequest{NodeId: "node-1"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "session resolver unavailable")
}

// Health operations tests

func TestServiceOpsHealthGet(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svcCtx := &svc.ServiceContext{}
	s := NewService(svcCtx)

	_, err := s.OpsHealthGet(ctx, &OpsHealthGetRequest{})
	require.NoError(t, err)
}

func TestServiceOpsHealthRun(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svcCtx := &svc.ServiceContext{}
	s := NewService(svcCtx)

	_, err := s.OpsHealthRun(ctx, &OpsHealthRunRequest{ID: "check-1"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ops state store unavailable")
}

func TestServiceOpsHealthUpdate(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svcCtx := &svc.ServiceContext{}
	s := NewService(svcCtx)

	_, err := s.OpsHealthUpdate(ctx, &OpsHealthUpdateRequest{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ops state store unavailable")
}

// Maintenance operations tests

func TestServiceOpsMaintenanceGet(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svcCtx := &svc.ServiceContext{}
	s := NewService(svcCtx)

	_, err := s.OpsMaintenanceGet(ctx, &OpsMaintenanceGetRequest{})
	require.NoError(t, err)
}

func TestServiceOpsMaintenanceUpdate(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svcCtx := &svc.ServiceContext{}
	s := NewService(svcCtx)

	_, err := s.OpsMaintenanceUpdate(ctx, &OpsMaintenanceUpdateRequest{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ops state store unavailable")
}

// Metrics tests

func TestServiceOpsMetrics(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svcCtx := &svc.ServiceContext{}
	s := NewService(svcCtx)

	_, err := s.OpsMetrics(ctx, &OpsMetricsRequest{})
	require.NoError(t, err)
}

// Config tests

func TestServiceOpsConfig(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svcCtx := &svc.ServiceContext{}
	s := NewService(svcCtx)

	resp, err := s.OpsConfig(ctx, &OpsConfigRequest{})
	require.NoError(t, err)
	assert.NotNil(t, resp)
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
		Addr:      "localhost:1001",
		GameID:    "game1",
		Env:       "prod",
		Functions: map[string]registry.FunctionMeta{"func1": {Enabled: true}},
		LastSeen:  time.Now(),
	})

	resp, err := s.OpsFunctions(ctx, &OpsFunctionsRequest{})
	require.NoError(t, err)
	assert.Len(t, resp.Functions["func1"], 1)
}

func TestServiceOpsFunctionsEmptyRegistry(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svcCtx := &svc.ServiceContext{RegistryStore: nil}
	s := NewService(svcCtx)

	resp, err := s.OpsFunctions(ctx, &OpsFunctionsRequest{})
	require.NoError(t, err)
	assert.Empty(t, resp.Functions)
}

// MQ tests

func TestServiceOpsMQ(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svcCtx := &svc.ServiceContext{}
	s := NewService(svcCtx)

	_, err := s.OpsMQ(ctx, &OpsMQRequest{})
	require.NoError(t, err)
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

	_, err := s.OpsNotificationsUpdate(ctx, &OpsNotificationsUpdateRequest{
		Enabled:  true,
		Channels: []OpsNotificationChannel{},
		Rules:    []OpsNotificationRule{},
	})
	require.NoError(t, err)
}

func TestServiceOpsNotificationsUpdateNilRequest(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svcCtx := &svc.ServiceContext{}
	s := NewService(svcCtx)

	_, err := s.OpsNotificationsUpdate(ctx, nil)
	require.NoError(t, err)
}

// Backup operations - safe tests only

func TestServiceOpsBackupDelete(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svcCtx := &svc.ServiceContext{}
	s := NewService(svcCtx)

	resp, err := s.OpsBackupDelete(ctx, &OpsBackupDeleteRequest{})
	require.NoError(t, err)
	assert.True(t, resp.Deleted)
}

func TestServiceOpsBackupDownload(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svcCtx := &svc.ServiceContext{}
	s := NewService(svcCtx)

	resp, err := s.OpsBackupDownload(ctx, &OpsBackupDownloadRequest{})
	require.NoError(t, err)
	assert.Contains(t, resp.Url, "/backups/")
}

// Alert silence delete test - safe path

func TestServiceOpsSilenceDelete(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svcCtx := &svc.ServiceContext{}
	s := NewService(svcCtx)

	_, err := s.OpsSilenceDelete(ctx, &OpsAlertSilenceRequest{AlertID: "invalid"})
	require.Error(t, err)
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
		Addr:      "localhost:1001",
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
		Addr:      "localhost:1001",
		GameID:    "game1",
		Env:       "prod",
		Functions: map[string]registry.FunctionMeta{},
		LastSeen:  time.Now(),
	})
	store.UpsertAgent(&registry.AgentSession{
		AgentID:   "agent-2",
		Addr:      "localhost:1002",
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
		Addr:      "localhost:1001",
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
		Addr:      "localhost:2001",
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
		Addr:      "localhost:2001",
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
	assert.Contains(t, rec.Body.String(), "agents")
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

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestOpsNodeRestartHandler(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	svcCtx := &svc.ServiceContext{}
	h := NewHandler(NewService(svcCtx))
	ctx, rec := newOpsTestContext(http.MethodPost, "/api/v1/ops/node/restart", `{"nodeId":"node-1"}`)

	h.OpsNodeRestart(ctx)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestOpsNodeUndrainHandler(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	svcCtx := &svc.ServiceContext{}
	h := NewHandler(NewService(svcCtx))
	ctx, rec := newOpsTestContext(http.MethodPost, "/api/v1/ops/node/undrain", `{"nodeId":"node-1"}`)

	h.OpsNodeUndrain(ctx)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestOpsNodeCommandsHandler(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	svcCtx := &svc.ServiceContext{}
	h := NewHandler(NewService(svcCtx))
	ctx, rec := newOpsTestContext(http.MethodPost, "/api/v1/ops/node/commands", `{"nodeId":"node-1"}`)

	h.OpsNodeCommands(ctx)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "commands")
}

func TestOpsHealthRunHandler(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	svcCtx := &svc.ServiceContext{}
	h := NewHandler(NewService(svcCtx))
	ctx, rec := newOpsTestContext(http.MethodPost, "/api/v1/ops/health/run", `{"id":"check-1"}`)

	h.OpsHealthRun(ctx)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestOpsHealthUpdateHandler(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	svcCtx := &svc.ServiceContext{}
	h := NewHandler(NewService(svcCtx))
	ctx, rec := newOpsTestContext(http.MethodPost, "/api/v1/ops/health/update", `{"enabled":true,"checks":[]}`)

	h.OpsHealthUpdate(ctx)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestOpsMaintenanceUpdateHandler(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	svcCtx := &svc.ServiceContext{}
	h := NewHandler(NewService(svcCtx))
	ctx, rec := newOpsTestContext(http.MethodPost, "/api/v1/ops/maintenance/update", `{"enabled":true,"windows":[]}`)

	h.OpsMaintenanceUpdate(ctx)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
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

	// Without an agent ops client configured, the call fails server-side.
	assert.GreaterOrEqual(t, rec.Code, http.StatusBadRequest)
}

func TestOpsAgentProcessStopHandler(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	svcCtx := &svc.ServiceContext{}
	h := NewHandler(NewService(svcCtx))
	ctx, rec := newOpsTestContext(http.MethodGet, "/api/v1/ops/agent/process/stop", "")

	h.OpsAgentProcessStop(ctx)

	assert.GreaterOrEqual(t, rec.Code, http.StatusBadRequest)
}

func TestOpsAgentProcessRestartHandler(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	svcCtx := &svc.ServiceContext{}
	h := NewHandler(NewService(svcCtx))
	ctx, rec := newOpsTestContext(http.MethodGet, "/api/v1/ops/agent/process/restart", "")

	h.OpsAgentProcessRestart(ctx)

	assert.GreaterOrEqual(t, rec.Code, http.StatusBadRequest)
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
