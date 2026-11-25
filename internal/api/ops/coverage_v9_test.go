// coverage_v9_test.go adds coverage for branches not covered in internal/api/ops:
// error paths (closed DB / save failure / extension event failure), cluster views,
// and success and service-error branches of the backup/alert/metrics/maintenance/notification
// handlers.
package ops

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cuihairu/croupier/core/transport"
	"github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/cluster"
	extensioninstallation "github.com/cuihairu/croupier/internal/core/extension/installation"
	agentops "github.com/cuihairu/croupier/internal/logic/ops"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/lbstats"
	"github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/platform/settings"
	extensiongorm "github.com/cuihairu/croupier/internal/repo/gorm/extension"
	"github.com/cuihairu/croupier/internal/svc"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	"github.com/gin-gonic/gin"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"gorm.io/gorm"
)

// ---- helpers (V9 suffix avoids conflicts with existing helpers) ----

func closeV9DB(t *testing.T, db *gorm.DB) {
	t.Helper()
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())
}

// newV9BackupSvcCtx returns a ctx carrying BackupModel plus the underlying db (to close
// the connection and force errors).
func newV9BackupSvcCtx(t *testing.T) (*svc.ServiceContext, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Backup{}))
	return &svc.ServiceContext{BackupModel: model.NewBackupModel(db)}, db
}

func newV9AlertSvcCtx(t *testing.T) (*svc.ServiceContext, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Alert{}, &model.AlertSilence{}))
	return &svc.ServiceContext{AlertModel: model.NewAlertModel(db)}, db
}

func newV9InstallationSvcCtx(t *testing.T) (*svc.ServiceContext, *extensioninstallation.Service, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.ExtensionInstallation{}, &model.ExtensionEvent{}, &model.ExtensionRuntimeBinding{}))
	repos := extensiongorm.NewBundle(db)
	installationSvc := extensioninstallation.NewService(repos.Installation, repos.Event, repos.Binding)
	return &svc.ServiceContext{Extensions: &svc.ExtensionServices{Installation: installationSvc}}, installationSvc, db
}

func installV9Notification(t *testing.T, installationSvc *extensioninstallation.Service, config map[string]any) *model.ExtensionInstallation {
	t.Helper()
	installed, err := installationSvc.Install(context.Background(), extensioninstallation.InstallRequest{
		ExtensionID:    officialNotificationID,
		ReleaseVersion: "1.0.0",
		ScopeType:      "system",
		ScopeID:        "global",
		TargetType:     "agent_group",
		TargetID:       "default",
		Config:         config,
		Operator:       "tester",
	})
	require.NoError(t, err)
	require.NotNil(t, installed)
	return installed
}

// ---- Agent process command helpers: success + nil svcCtx + wrapper errors ----

func TestOpsAgentProcessHelpersV9(t *testing.T) {
	caller := &fakeCaller{respFor: map[uint32]proto.Message{
		protocol.MsgExecuteCommandRequest: &opsv1.ExecuteCommandResponse{ExitCode: 0, StdOut: "v9-out", StdErr: ""},
		protocol.MsgStartProcessRequest:   &opsv1.StartProcessResponse{Pid: 777},
		protocol.MsgStopProcessRequest:    &opsv1.StopProcessResponse{},
		protocol.MsgRestartProcessRequest: &opsv1.RestartProcessResponse{},
	}}
	client := agentops.GetAgentOpsClient()
	client.SetSessionResolver(&fakeResolver{conns: map[string]transport.SessionCaller{"agent-1": caller}})
	t.Cleanup(func() { client.SetSessionResolver(nil) })

	store := registry.NewStore()
	require.NoError(t, store.UpsertAgent(&registry.AgentSession{AgentID: "agent-1", GameID: "demo", Env: "prod"}))
	svcCtx := &svc.ServiceContext{RegistryStore: store}
	ctx := context.Background()

	// ExecCommand success → response conversion
	execResp, err := opsAgentExecCommand(ctx, svcCtx, &OpsExecCommandRequest{AgentID: "agent-1", Command: "ls", Timeout: 3})
	require.NoError(t, err)
	assert.EqualValues(t, 0, execResp.Result.ExitCode)
	assert.Equal(t, "v9-out", execResp.Result.Stdout)

	// Start/Stop/Restart success
	startResp, err := opsAgentProcessStart(ctx, svcCtx, &OpsProcessStartRequest{AgentID: "agent-1", Name: "proc-1"})
	require.NoError(t, err)
	assert.EqualValues(t, 777, startResp.Pid)
	_, err = opsAgentProcessStop(ctx, svcCtx, &OpsProcessActionRequest{AgentID: "agent-1", Name: "proc-1"})
	require.NoError(t, err)
	_, err = opsAgentProcessRestart(ctx, svcCtx, &OpsProcessActionRequest{AgentID: "agent-1", Name: "proc-1"})
	require.NoError(t, err)

	// nil svcCtx → bad request
	_, err = opsAgentProcessStart(ctx, nil, &OpsProcessStartRequest{})
	require.Error(t, err)
	_, err = opsAgentProcessStop(ctx, nil, &OpsProcessActionRequest{})
	require.Error(t, err)
	_, err = opsAgentProcessRestart(ctx, nil, &OpsProcessActionRequest{})
	require.Error(t, err)

	// ExecuteCommand / StartProcess wrapper call failure
	errCaller := &fakeCaller{errOn: map[uint32]error{
		protocol.MsgExecuteCommandRequest: assert.AnError,
		protocol.MsgStartProcessRequest:   assert.AnError,
	}}
	client.SetSessionResolver(&fakeResolver{conns: map[string]transport.SessionCaller{"agent-err": errCaller}})
	_, err = opsAgentExecCommand(ctx, svcCtx, &OpsExecCommandRequest{AgentID: "agent-err", Command: "ls"})
	require.Error(t, err)
	_, err = opsAgentProcessStart(ctx, svcCtx, &OpsProcessStartRequest{AgentID: "agent-err", Name: "x"})
	require.Error(t, err)

	s := NewAgentService(svcCtx)
	_, err = s.ExecCommand(ctx, "agent-err", "ls", nil, 1)
	require.Error(t, err)
	_, err = s.StartProcess(ctx, "agent-err", "x", nil, nil, "")
	require.Error(t, err)
}

// ---- backup/alert: closed DB error paths ----

func TestBackupAlertModelErrorPathsV9(t *testing.T) {
	ctx := context.Background()

	backupCtx, backupDB := newV9BackupSvcCtx(t)
	closeV9DB(t, backupDB)
	_, err := opsBackupsList(ctx, backupCtx, &OpsBackupsListRequest{})
	require.Error(t, err)
	_, err = opsBackupCreate(ctx, backupCtx, &OpsBackupCreateRequest{Name: "b1"})
	require.Error(t, err)
	bs := NewBackupService(backupCtx)
	_, err = bs.List(ctx, "", "")
	require.Error(t, err)
	_, err = bs.Create(ctx, "g", "e", "manual", 30)
	require.Error(t, err)

	// backup does not exist → FindByBackupID error
	okBackupCtx, _ := newV9BackupSvcCtx(t)
	_, err = opsBackupDelete(ctx, okBackupCtx, &OpsBackupDeleteRequest{ID: "missing"})
	require.Error(t, err)
	_, err = opsBackupDownload(ctx, okBackupCtx, &OpsBackupDownloadRequest{ID: "missing"})
	require.Error(t, err)

	alertCtx, alertDB := newV9AlertSvcCtx(t)
	closeV9DB(t, alertDB)
	_, err = opsAlerts(ctx, alertCtx, &OpsAlertsRequest{})
	require.Error(t, err)
	// invalid alertID → Sscanf error
	_, err = opsAlertSilence(ctx, alertCtx, &OpsAlertSilenceRequest{AlertID: "not-a-number"})
	require.Error(t, err)
	// valid alertID but CreateSilence fails
	_, err = opsAlertSilence(ctx, alertCtx, &OpsAlertSilenceRequest{AlertID: "9", Duration: 5})
	require.Error(t, err)
	_, err = opsSilenceDelete(ctx, alertCtx, &OpsAlertSilenceRequest{AlertID: "9"})
	require.Error(t, err)
	_, err = opsSilences(ctx, alertCtx, &OpsSilencesRequest{})
	require.Error(t, err)

	as := NewAlertService(alertCtx)
	_, err = as.List(ctx, "", "", "")
	require.Error(t, err)
	_, err = as.Silence(ctx, "9", 5, "c")
	require.Error(t, err)
	_, err = as.ListSilences(ctx, "")
	require.Error(t, err)
}

// ---- GetClusterInfo: single instance with registry counts + handler error ----

func TestOpsClusterInfoStandaloneAndHandlerErrorV9(t *testing.T) {
	store := registry.NewStore()
	require.NoError(t, store.UpsertAgent(&registry.AgentSession{
		AgentID: "a1", GameID: "g", Env: "prod", Addr: "h:1",
		Labels: map[string]string{}, Functions: map[string]registry.FunctionMeta{},
	}))

	svcCtx := &svc.ServiceContext{RegistryStore: store}
	resp, err := NewService(svcCtx).GetClusterInfo(context.Background())
	require.NoError(t, err)
	assert.False(t, resp.Enabled)
	require.Len(t, resp.Items, 1)
	assert.Equal(t, "standalone", resp.Items[0].InstanceID)
	assert.Equal(t, int64(1), resp.Items[0].AgentCount)
	assert.Equal(t, 1, resp.AliveCount)

	// handler: nil svcCtx → service error branch
	gin.SetMode(gin.TestMode)
	h := NewHandler(NewService(nil))
	ctx, rec := newOpsTestContext(http.MethodGet, "/api/v1/ops/cluster", "")
	h.ClusterInfo(ctx)
	assert.GreaterOrEqual(t, rec.Code, 400)
}

// ---- GetClusterInfo: mesh offline peer + LBStats + LBStatsQuery handler success ----

func TestGetClusterInfoMeshOfflineLBStatsV9(t *testing.T) {
	prom := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[]}}`))
	}))
	t.Cleanup(prom.Close)

	mesh := cluster.NewMeshInterconnect(
		cluster.PeerInfo{InstanceID: "inst-1"},
		nil,
		&fakeMembership{peers: []cluster.PeerInfo{
			{InstanceID: "inst-1", AdvertiseAddr: "10.0.0.1:1", Epoch: 1, StartedAt: time.Now()},
			{InstanceID: "inst-2", AdvertiseAddr: "10.0.0.2:1", Epoch: 2, StartedAt: time.Now()},
			{InstanceID: "inst-3", AdvertiseAddr: "10.0.0.3:1", Epoch: 3, StartedAt: time.Now()},
		}},
	)
	mesh.RefreshPeers(context.Background())

	svcCtx := &svc.ServiceContext{Cluster: &svc.ClusterRuntime{
		InstanceID: "inst-1",
		// alive members: inst-1 (self) + inst-2; inst-3 exists only in mesh last-known → offline
		Membership: &fakeMembership{peers: []cluster.PeerInfo{
			{InstanceID: "inst-1", AdvertiseAddr: "10.0.0.1:1", Epoch: 1, StartedAt: time.Now()},
			{InstanceID: "inst-2", AdvertiseAddr: "10.0.0.2:1", Epoch: 2, StartedAt: time.Now()},
		}},
		Mesh:    mesh,
		LBStats: lbstats.NewLBStatsService(prom.URL),
	}}
	s := NewService(svcCtx)

	resp, err := s.GetClusterInfo(context.Background())
	require.NoError(t, err)
	assert.True(t, resp.Enabled)
	assert.Equal(t, "inst-1", resp.Self)
	require.NotNil(t, resp.LbStats)
	assert.True(t, resp.LbStats.Enabled)
	// inst-2 is alive; inst-3 exists only in mesh last-known → shown as offline
	require.Len(t, resp.Items, 3)
	byInst := map[string]ClusterInstance{}
	for _, it := range resp.Items {
		byInst[it.InstanceID] = it
	}
	assert.True(t, byInst["inst-1"].Self)
	assert.True(t, byInst["inst-2"].Alive)
	assert.False(t, byInst["inst-3"].Alive)
	assert.Equal(t, 3, resp.Total)
	assert.Equal(t, 2, resp.AliveCount)

	// Service.LBStatsQuery success
	result, err := s.LBStatsQuery(context.Background(), "haproxy_up")
	require.NoError(t, err)
	assert.NotNil(t, result)

	// handler success
	gin.SetMode(gin.TestMode)
	h := NewHandler(s)
	r := gin.New()
	r.POST("/lb-stats", h.LBStatsQuery)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/lb-stats", strings.NewReader(`{"query":"haproxy_up"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

// ---- recordOpsAudit: audit recorded when AuditService is non-nil ----

func TestRecordOpsAuditViaNodeDrainV9(t *testing.T) {
	store := svc.NewOpsStateStore(t.TempDir())
	regStore := registry.NewStore()
	for _, id := range []string{"node-1", "node-err-restart", "node-err-undrain"} {
		_ = regStore.UpsertAgent(&registry.AgentSession{AgentID: id, GameID: "demo", Env: "prod"})
	}
	svcCtx := &svc.ServiceContext{
		OpsStateStore: store,
		RegistryStore: regStore,
		AuditService:  audit.NewAuditService(audit.NewInMemoryAuditStore(), nil),
	}
	caller := &fakeCaller{}
	svcCtx.AgentSessionResolver = &fakeResolver{conns: map[string]transport.SessionCaller{"node-1": caller}}

	// no username → actor=system
	resp, err := opsNodeDrain(context.Background(), svcCtx, &OpsNodeCommandsRequest{NodeId: "node-1"})
	require.NoError(t, err)
	assert.Equal(t, "draining", resp.Status)

	// with username → actor=username
	ctx := context.WithValue(context.Background(), "username", "v9-admin")
	resp2, err := opsNodeDrain(ctx, svcCtx, &OpsNodeCommandsRequest{NodeId: "node-1"})
	require.NoError(t, err)
	assert.Equal(t, "node-1", resp2.NodeId)

	// restart / undrain / maintenance also go through recordOpsAudit
	_, err = opsNodeRestart(ctx, svcCtx, &OpsNodeCommandsRequest{NodeId: "node-1"})
	require.NoError(t, err)
	_, err = opsNodeUndrain(ctx, svcCtx, &OpsNodeCommandsRequest{NodeId: "node-1"})
	require.NoError(t, err)
	_, err = opsMaintenanceUpdate(ctx, svcCtx, &OpsMaintenanceUpdateRequest{
		Windows: []OpsMaintenanceWindow{{GameID: "g", Env: "prod"}},
	})
	require.NoError(t, err)

	// Call failure still proceeds: restart / undrain only log a warning, no interruption
	svcCtx.AgentSessionResolver = &fakeResolver{conns: map[string]transport.SessionCaller{
		"node-err-restart": &fakeCaller{errOn: map[uint32]error{protocol.MsgRestartProcessRequest: assert.AnError}},
		"node-err-undrain": &fakeCaller{errOn: map[uint32]error{protocol.MsgProviderDrainRequest: assert.AnError}},
	}}
	restartResp, err := opsNodeRestart(ctx, svcCtx, &OpsNodeCommandsRequest{NodeId: "node-err-restart"})
	require.NoError(t, err)
	assert.Equal(t, "restarting", restartResp.Status)
	undrainResp, err := opsNodeUndrain(ctx, svcCtx, &OpsNodeCommandsRequest{NodeId: "node-err-undrain"})
	require.NoError(t, err)
	assert.Equal(t, "active", undrainResp.Status)
}

// ---- opsHealthGet / opsHealthRun: populated state and http branches ----

func TestOpsHealthGetPopulatedStateV9(t *testing.T) {
	ctx := context.Background()
	store := svc.NewOpsStateStore(t.TempDir())
	svcCtx := &svc.ServiceContext{OpsStateStore: store}

	_, err := opsHealthUpdate(ctx, svcCtx, &OpsHealthUpdateRequest{Checks: []OpsHealthCheck{
		{ID: "hc-tcp", Kind: "tcp", Target: "127.0.0.1:80"},
		{ID: "hc-http", Kind: "http", Target: "https://example.com"},
		{ID: "hc-custom", Kind: "icmp", Target: "10.0.0.1"},
	}})
	require.NoError(t, err)

	// run each of the two checks once, then run http again → verify old state is replaced
	// rather than appended; custom takes the default branch
	_, err = opsHealthRun(ctx, svcCtx, &OpsHealthRunRequest{ID: "hc-tcp"})
	require.NoError(t, err)
	_, err = opsHealthRun(ctx, svcCtx, &OpsHealthRunRequest{ID: "hc-http"})
	require.NoError(t, err)
	_, err = opsHealthRun(ctx, svcCtx, &OpsHealthRunRequest{ID: "hc-custom"})
	require.NoError(t, err)
	_, err = opsHealthRun(ctx, svcCtx, &OpsHealthRunRequest{ID: "hc-http"})
	require.NoError(t, err)

	getResp, err := opsHealthGet(ctx, svcCtx, &OpsHealthGetRequest{})
	require.NoError(t, err)
	assert.Len(t, getResp.Checks, 3)
	assert.Len(t, getResp.Status, 3)
	assert.NotEmpty(t, getResp.UpdatedAt)

	snap := store.Snapshot()
	assert.Len(t, snap.Health.Status, 3)
}

// ---- OpsStateStore persistence failure → health/maintenance Update error ----

func TestOpsStateStoreSaveFailureV9(t *testing.T) {
	ctx := context.Background()
	base := t.TempDir()
	// occupy the state-file path with a directory → WriteFile fails
	require.NoError(t, os.Mkdir(filepath.Join(base, "ops_state.json"), 0o755))
	store := svc.NewOpsStateStore(base)
	svcCtx := &svc.ServiceContext{OpsStateStore: store}

	// Update fails (fn has already been applied in memory)
	_, err := opsHealthUpdate(ctx, svcCtx, &OpsHealthUpdateRequest{Checks: []OpsHealthCheck{
		{ID: "hc-1", Kind: "tcp", Target: "127.0.0.1:1"},
	}})
	require.Error(t, err)

	// the check is still in memory → opsHealthRun finds the target and then Update fails
	_, err = opsHealthRun(ctx, svcCtx, &OpsHealthRunRequest{ID: "hc-1"})
	require.Error(t, err)

	_, err = opsMaintenanceUpdate(ctx, svcCtx, &OpsMaintenanceUpdateRequest{})
	require.Error(t, err)
}

// ---- opsMaintenanceGet: non-empty windows rendering ----

func TestOpsMaintenanceGetPopulatedV9(t *testing.T) {
	ctx := context.Background()
	store := svc.NewOpsStateStore(t.TempDir())
	_, err := store.Update(func(state *svc.OpsState) {
		state.Maintenance.Windows = []svc.OpsMaintenanceWindow{
			{ID: "mw-1", GameID: "g", Env: "prod", Start: "2025-01-01", End: "2025-01-02", Message: "m", BlockWrites: true},
		}
		state.Maintenance.UpdatedAt = time.Now()
	})
	require.NoError(t, err)

	resp, err := opsMaintenanceGet(ctx, &svc.ServiceContext{OpsStateStore: store}, &OpsMaintenanceGetRequest{})
	require.NoError(t, err)
	require.Len(t, resp.Windows, 1)
	assert.True(t, resp.Windows[0].BlockWrites)
	assert.NotEmpty(t, resp.UpdatedAt)
}

// ---- opsMetrics: gameId filter mismatch ----

func TestOpsMetricsGameFilterV9(t *testing.T) {
	now := time.Now()
	store := registry.NewStore()
	require.NoError(t, store.UpsertAgent(&registry.AgentSession{
		AgentID: "m1", GameID: "g1", Env: "prod", Addr: "h:1",
		Labels: map[string]string{}, Functions: map[string]registry.FunctionMeta{},
		LastSeen: now, ExpireAt: now.Add(time.Minute),
	}))

	svcCtx := &svc.ServiceContext{RegistryStore: store, MetricsStore: registry.NewMetricsStore()}
	resp, err := opsMetrics(context.Background(), svcCtx, &OpsMetricsRequest{GameId: "other-game"})
	require.NoError(t, err)
	assert.Empty(t, resp.Metrics)
}

// ---- opsServices：StartTime / expired / nil labels / zero LastSeen ----

func TestOpsServicesBranchesV9(t *testing.T) {
	now := time.Now()
	store := registry.NewStore()
	require.NoError(t, store.UpsertAgent(&registry.AgentSession{
		AgentID: "exp-1", GameID: "g", Env: "prod", Addr: "h:1",
		Labels: map[string]string{}, Functions: map[string]registry.FunctionMeta{},
		LastSeen: now, ExpireAt: now.Add(-time.Hour), // expired → expired status
	}))
	require.NoError(t, store.UpsertAgent(&registry.AgentSession{
		AgentID: "bare-1", GameID: "g", Env: "prod", Addr: "h:2",
		Functions: map[string]registry.FunctionMeta{},
		// Labels nil, LastSeen zero value
	}))

	svcCtx := &svc.ServiceContext{RegistryStore: store, StartTime: now}
	resp, err := opsServices(context.Background(), svcCtx, &OpsServicesRequest{})
	require.NoError(t, err)
	// 1 server + 2 agents
	require.Len(t, resp.Services, 3)
	assert.NotEmpty(t, resp.Services[0].LastSeen) // StartTime → lastSeen

	byID := map[string]OpsServiceItem{}
	for _, it := range resp.Services {
		byID[it.ID] = it
	}
	assert.Equal(t, "expired", byID["exp-1"].Status)
	bare := byID["bare-1"]
	assert.Equal(t, "healthy", bare.Status)
	assert.NotNil(t, bare.Labels)
	assert.Empty(t, bare.Labels)
	assert.Empty(t, bare.LastSeen)
}

// ---- opsConfig: main branch after settings init + state/env fallback ----

func TestOpsConfigSettingsLayerV9(t *testing.T) {
	ctx := context.Background()

	// initialize global layered settings (L2 empty); not parallel, to avoid global singleton races
	l := settings.InitLayered(ctx, nil, nil)
	require.NotNil(t, l)
	require.NotNil(t, settings.Current())

	// snap empty → fall back to state store
	store := svc.NewOpsStateStore(t.TempDir())
	_, err := store.Update(func(state *svc.OpsState) {
		state.Config.AlertmanagerURL = "http://am.v9"
		state.Config.GrafanaExploreURL = "http://grafana.v9"
		state.Config.JaegerURL = "http://jaeger.v9"
	})
	require.NoError(t, err)
	resp, err := opsConfig(ctx, &svc.ServiceContext{OpsStateStore: store}, &OpsConfigRequest{})
	require.NoError(t, err)
	assert.Equal(t, "http://am.v9", resp.AlertmanagerURL)
	assert.Equal(t, "http://grafana.v9", resp.GrafanaExploreURL)
	assert.Equal(t, "http://jaeger.v9", resp.JaegerURL)

	// both snap and state empty → fall back to env
	t.Setenv("CROUPIER_ALERTMANAGER_URL", "http://am.env9")
	t.Setenv("CROUPIER_GRAFANA_EXPLORE_URL", "http://grafana.env9")
	t.Setenv("CROUPIER_JAEGER_URL", "http://jaeger.env9")
	resp, err = opsConfig(ctx, &svc.ServiceContext{}, &OpsConfigRequest{})
	require.NoError(t, err)
	assert.Equal(t, "http://am.env9", resp.AlertmanagerURL)
	assert.Equal(t, "http://grafana.env9", resp.GrafanaExploreURL)
	assert.Equal(t, "http://jaeger.env9", resp.JaegerURL)
}

// ---- notification extension: config boundaries and error paths ----

func TestOpsNotificationsEdgeCasesV9(t *testing.T) {
	ctx := context.Background()

	// empty config → nothing extracted → empty defaults returned (ok=true branch)
	svcCtx1, installSvc1, _ := newV9InstallationSvcCtx(t)
	installV9Notification(t, installSvc1, map[string]any{})
	resp, err := opsNotificationsGet(ctx, svcCtx1, &OpsNotificationsGetRequest{})
	require.NoError(t, err)
	assert.False(t, resp.Enabled)
	assert.Empty(t, resp.Channels)
	assert.Empty(t, resp.Rules)

	// invalid ConfigJSON → unmarshal error
	svcCtx2, installSvc2, db2 := newV9InstallationSvcCtx(t)
	inst2 := installV9Notification(t, installSvc2, map[string]any{})
	require.NoError(t, db2.Model(&model.ExtensionInstallation{}).Where("id = ?", inst2.ID).Update("config_json", "not-json").Error)
	_, err = opsNotificationsGet(ctx, svcCtx2, &OpsNotificationsGetRequest{})
	require.Error(t, err)

	// channels wrong type → extract error
	svcCtx3, installSvc3, db3 := newV9InstallationSvcCtx(t)
	inst3 := installV9Notification(t, installSvc3, map[string]any{})
	require.NoError(t, db3.Model(&model.ExtensionInstallation{}).Where("id = ?", inst3.ID).Update("config_json", `{"channels":"oops"}`).Error)
	_, err = opsNotificationsGet(ctx, svcCtx3, &OpsNotificationsGetRequest{})
	require.Error(t, err)

	// uninstalled skipped → no active installation
	svcCtx4, installSvc4, _ := newV9InstallationSvcCtx(t)
	inst4 := installV9Notification(t, installSvc4, map[string]any{})
	require.NoError(t, installSvc4.Uninstall(ctx, inst4.ID, "tester"))
	item, ok, err := findActiveExtensionInstallationByID(ctx, svcCtx4, officialNotificationID)
	require.NoError(t, err)
	assert.False(t, ok)
	assert.Nil(t, item)
	resp4, err := opsNotificationsGet(ctx, svcCtx4, &OpsNotificationsGetRequest{})
	require.NoError(t, err)
	assert.False(t, resp4.Enabled)

	// List error → Get/Update both fail
	svcCtx5, installSvc5, db5 := newV9InstallationSvcCtx(t)
	installV9Notification(t, installSvc5, map[string]any{})
	closeV9DB(t, db5)
	_, err = opsNotificationsGet(ctx, svcCtx5, &OpsNotificationsGetRequest{})
	require.Error(t, err)
	_, err = opsNotificationsUpdate(ctx, svcCtx5, &OpsNotificationsUpdateRequest{})
	require.Error(t, err)

	// nil req + no extension → success (find returns false)
	_, err = opsNotificationsUpdate(ctx, &svc.ServiceContext{}, nil)
	require.NoError(t, err)
}

// ---- notification update: save succeeds but audit event fails ----

func TestOpsNotificationsRecordEventErrorV9(t *testing.T) {
	ctx := context.Background()
	svcCtx, installSvc, db := newV9InstallationSvcCtx(t)
	installV9Notification(t, installSvc, map[string]any{})
	// block only the notifications_update event: update_config lands normally, the audit event fails
	require.NoError(t, db.Exec(`CREATE TRIGGER v9_block_notif BEFORE INSERT ON extension_events
WHEN NEW.event_type = 'notifications_update'
BEGIN
  SELECT RAISE(ABORT, 'v9 blocked');
END;`).Error)

	_, err := opsNotificationsUpdate(ctx, svcCtx, &OpsNotificationsUpdateRequest{Enabled: true})
	require.Error(t, err)

	// config was saved successfully
	item, ok, err := findActiveExtensionInstallationByID(ctx, svcCtx, officialNotificationID)
	require.NoError(t, err)
	require.True(t, ok)
	assert.Contains(t, string(item.ConfigJSON), `"enabled"`)
}

// ---- extractNotificationConfig: marshal/unmarshal errors ----

func TestExtractNotificationConfigErrorsV9(t *testing.T) {
	t.Run("enabled marshal error", func(t *testing.T) {
		_, _, _, _, err := extractNotificationConfig(map[string]any{"enabled": make(chan int)})
		require.Error(t, err)
	})
	t.Run("channels marshal error", func(t *testing.T) {
		_, _, _, _, err := extractNotificationConfig(map[string]any{"channels": make(chan int)})
		require.Error(t, err)
	})
	t.Run("channels unmarshal error", func(t *testing.T) {
		_, _, _, _, err := extractNotificationConfig(map[string]any{"channels": "not-an-array"})
		require.Error(t, err)
	})
	t.Run("rules marshal error", func(t *testing.T) {
		_, _, _, _, err := extractNotificationConfig(map[string]any{"rules": make(chan int)})
		require.Error(t, err)
	})
	t.Run("rules unmarshal error", func(t *testing.T) {
		_, _, _, _, err := extractNotificationConfig(map[string]any{"rules": 42})
		require.Error(t, err)
	})
}

// ---- handler success paths: exec/start/stop/restart/drain/restart/undrain/health/maintenance ----

func TestHandlerSuccessPathsV9(t *testing.T) {
	gin.SetMode(gin.TestMode)

	caller := &fakeCaller{respFor: map[uint32]proto.Message{
		protocol.MsgExecuteCommandRequest: &opsv1.ExecuteCommandResponse{ExitCode: 0, StdOut: "ok"},
		protocol.MsgStartProcessRequest:   &opsv1.StartProcessResponse{Pid: 99},
		protocol.MsgStopProcessRequest:    &opsv1.StopProcessResponse{},
		protocol.MsgRestartProcessRequest: &opsv1.RestartProcessResponse{},
	}}
	client := agentops.GetAgentOpsClient()
	client.SetSessionResolver(&fakeResolver{conns: map[string]transport.SessionCaller{"agent-1": caller}})
	t.Cleanup(func() { client.SetSessionResolver(nil) })

	nodeRegStore := registry.NewStore()
	require.NoError(t, nodeRegStore.UpsertAgent(&registry.AgentSession{AgentID: "node-1", GameID: "demo", Env: "prod"}))
	nodeCtx := &svc.ServiceContext{
		OpsStateStore:        svc.NewOpsStateStore(t.TempDir()),
		RegistryStore:        nodeRegStore,
		AgentSessionResolver: &fakeResolver{conns: map[string]transport.SessionCaller{"node-1": caller}},
	}

	runHandlerV9 := func(h func(*gin.Context), method, body string) int {
		ctx, rec := newOpsTestContext(method, "/api/v1/ops", body)
		h(ctx)
		return rec.Code
	}

	// agent process command success
	agentStore := registry.NewStore()
	require.NoError(t, agentStore.UpsertAgent(&registry.AgentSession{AgentID: "agent-1"}))
	agentH := NewHandler(NewService(&svc.ServiceContext{RegistryStore: agentStore}))
	assert.Equal(t, http.StatusOK, runHandlerV9(agentH.OpsAgentExecCommand, http.MethodPost, `{"AgentID":"agent-1","command":"ls","args":[],"timeout":1}`))

	// node command success
	nodeH := NewHandler(NewService(nodeCtx))
	assert.Equal(t, http.StatusOK, runHandlerV9(nodeH.OpsNodeDrain, http.MethodPost, `{"nodeId":"node-1"}`))
	assert.Equal(t, http.StatusOK, runHandlerV9(nodeH.OpsNodeRestart, http.MethodPost, `{"nodeId":"node-1"}`))
	assert.Equal(t, http.StatusOK, runHandlerV9(nodeH.OpsNodeUndrain, http.MethodPost, `{"nodeId":"node-1"}`))

	// health / maintenance success
	stateH := NewHandler(NewService(&svc.ServiceContext{OpsStateStore: svc.NewOpsStateStore(t.TempDir())}))
	assert.Equal(t, http.StatusOK, runHandlerV9(stateH.OpsHealthUpdate, http.MethodPost, `{"checks":[{"id":"hc-1","kind":"tcp","target":"127.0.0.1:80"}]}`))
	assert.Equal(t, http.StatusOK, runHandlerV9(stateH.OpsHealthRun, http.MethodPost, `{"id":"hc-1"}`))
	assert.Equal(t, http.StatusOK, runHandlerV9(stateH.OpsMaintenanceUpdate, http.MethodPost, `{"windows":[{"gameId":"g","env":"prod"}]}`))
}

// TestHandlerProcessActionSuccessPathV9 drives the process action surface via a real router
// (path parameters): the panel sends a bodyless POST, so agentId/name come only from path
// parameters (the same binding gap as #59: ShouldBindJSON reports EOF on an empty body, and
// uri tags are not read by JSON binding). This proves the inputs flow from the path through
// to the agent ops tunnel call.
func TestHandlerProcessActionSuccessPathV9(t *testing.T) {
	gin.SetMode(gin.TestMode)

	caller := &fakeCaller{respFor: map[uint32]proto.Message{
		protocol.MsgStartProcessRequest:   &opsv1.StartProcessResponse{Pid: 99},
		protocol.MsgStopProcessRequest:    &opsv1.StopProcessResponse{},
		protocol.MsgRestartProcessRequest: &opsv1.RestartProcessResponse{},
		protocol.MsgSchedulePauseRequest:  &opsv1.SchedulePauseResponse{Success: true, Suspended: true},
	}}
	client := agentops.GetAgentOpsClient()
	client.SetSessionResolver(&fakeResolver{conns: map[string]transport.SessionCaller{"agent-1": caller}})
	t.Cleanup(func() { client.SetSessionResolver(nil) })

	processStore := registry.NewStore()
	require.NoError(t, processStore.UpsertAgent(&registry.AgentSession{AgentID: "agent-1"}))
	h := NewHandler(NewService(&svc.ServiceContext{RegistryStore: processStore}))
	router := gin.New()
	g := router.Group("/api/v1/ops/agents/:agentId/processes/:name")
	g.POST("/start", h.OpsAgentProcessStart)
	g.POST("/stop", h.OpsAgentProcessStop)
	g.POST("/restart", h.OpsAgentProcessRestart)
	g.POST("/schedule/pause", h.OpsAgentProcessSchedulePause)
	g.POST("/schedule/resume", h.AgentProcessScheduleResume)

	for _, sub := range []string{"/start", "/stop", "/restart", "/schedule/pause", "/schedule/resume"} {
		target := "/api/v1/ops/agents/agent-1/processes/p1" + sub
		req := httptest.NewRequest(http.MethodPost, target, strings.NewReader(""))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code, "%s: %s", sub, rec.Body.String())
	}
}

// ---- handler service error paths ----

func TestHandlerServiceErrorPathsV9(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// backup: closed DB / nonexistent
	backupCtx, backupDB := newV9BackupSvcCtx(t)
	h := NewHandler(NewService(backupCtx))
	closeV9DB(t, backupDB)
	gctx, rec := newOpsTestContext(http.MethodGet, "/api/v1/ops/backups", "")
	h.OpsBackupsList(gctx)
	assert.GreaterOrEqual(t, rec.Code, 400)
	gctx, rec = newOpsTestContext(http.MethodPost, "/api/v1/ops/backups", `{"name":"b1"}`)
	h.OpsBackupCreate(gctx)
	assert.GreaterOrEqual(t, rec.Code, 400)
	gctx, rec = newOpsTestContext(http.MethodDelete, "/api/v1/ops/backups", `{"id":"ghost"}`)
	h.OpsBackupDelete(gctx)
	assert.GreaterOrEqual(t, rec.Code, 400)
	gctx, rec = newOpsTestContext(http.MethodGet, "/api/v1/ops/backups", "")
	h.OpsBackupDownload(gctx)
	assert.GreaterOrEqual(t, rec.Code, 400)

	// alert: closed DB / invalid ID
	alertCtx, alertDB := newV9AlertSvcCtx(t)
	closeV9DB(t, alertDB)
	ha := NewHandler(NewService(alertCtx))
	gctx, rec = newOpsTestContext(http.MethodGet, "/api/v1/ops/alerts", "")
	ha.OpsAlerts(gctx)
	assert.GreaterOrEqual(t, rec.Code, 400)
	gctx, rec = newOpsTestContext(http.MethodPost, "/api/v1/ops/alerts/silence", `{"alertId":"abc"}`)
	ha.OpsAlertSilence(gctx)
	assert.GreaterOrEqual(t, rec.Code, 400)
	gctx, rec = newOpsTestContext(http.MethodGet, "/api/v1/ops/silences", "")
	ha.OpsSilences(gctx)
	assert.GreaterOrEqual(t, rec.Code, 400)

	// notification: extension List error
	extCtx, _, db := newV9InstallationSvcCtx(t)
	hn := NewHandler(NewService(extCtx))
	closeV9DB(t, db)
	gctx, rec = newOpsTestContext(http.MethodGet, "/api/v1/ops/notifications", "")
	hn.OpsNotificationsGet(gctx)
	assert.GreaterOrEqual(t, rec.Code, 400)
	gctx, rec = newOpsTestContext(http.MethodPost, "/api/v1/ops/notifications", `{}`)
	hn.OpsNotificationsUpdate(gctx)
	assert.GreaterOrEqual(t, rec.Code, 400)
}

// ---- listNodes: cluster ownership table + filtering + sorting ----

func TestListNodesClusterAndDBBranchesV9(t *testing.T) {
	now := time.Now()
	store := registry.NewStore()
	// held remotely: local frozen snapshot (liveness decided via the ownership table);
	// non-empty labels exercise the copy-over loop
	require.NoError(t, store.UpsertAgent(&registry.AgentSession{
		AgentID: "agent-remote", GameID: "g1", Env: "prod", Addr: "h:2",
		Labels: map[string]string{"zone": "z1"}, Functions: map[string]registry.FunctionMeta{},
		LastSeen: now.Add(-time.Hour), ExpireAt: now.Add(-time.Minute),
	}))
	// held remotely + already drained
	require.NoError(t, store.UpsertAgent(&registry.AgentSession{
		AgentID: "agent-drained", GameID: "g1", Env: "prod", Addr: "h:3",
		Labels: map[string]string{}, Functions: map[string]registry.FunctionMeta{},
		LastSeen: now.Add(-time.Hour), ExpireAt: now.Add(-time.Minute),
	}))

	opsStore := svc.NewOpsStateStore(t.TempDir())
	_, err := opsStore.Update(func(state *svc.OpsState) {
		state.Nodes.Drained = map[string]time.Time{"agent-drained": now}
	})
	require.NoError(t, err)

	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		OpsStateStore: opsStore,
		Cluster: &svc.ClusterRuntime{
			InstanceID: "self-inst",
			ListAgentOwners: func(context.Context) ([]cluster.AgentOwnerRecord, error) {
				return []cluster.AgentOwnerRecord{
					{AgentID: "agent-remote", InstanceID: "peer", GameID: "g1", Env: "prod", LastSeenAt: now},
					{AgentID: "agent-drained", InstanceID: "peer", GameID: "g1", Env: "prod", LastSeenAt: now},
					{AgentID: "agent-only-owner", InstanceID: "peer", GameID: "g1", Env: "prod", LastSeenAt: now.Add(-time.Minute)},
					{AgentID: "agent-owner-capture", InstanceID: "peer", GameID: "g1", Env: "prod", LabelsMap: map[string]string{"role": "capture"}, LastSeenAt: now},
					{AgentID: "agent-other-game", InstanceID: "peer", GameID: "gX", Env: "prod", LastSeenAt: now},
					{AgentID: "agent-other-env", InstanceID: "peer", GameID: "g1", Env: "dev", LastSeenAt: now},
				}, nil
			},
		},
	}

	// nil ctx + game/env filtering
	nodes := listNodes(context.TODO(), svcCtx, "g1", "prod", "")
	byID := map[string]Node{}
	for _, n := range nodes {
		byID[n.Id] = n
	}
	remote, ok := byID["agent-remote"]
	require.True(t, ok)
	assert.Equal(t, "online", remote.Status)
	assert.Equal(t, "peer", remote.Labels["ownerInstance"])
	drained, ok := byID["agent-drained"]
	require.True(t, ok)
	assert.Equal(t, "drained", drained.Status)
	// ownership table only (not in local registry) → online + owner annotation
	onlyOwner, ok := byID["agent-only-owner"]
	require.True(t, ok)
	assert.Equal(t, "online", onlyOwner.Status)
	assert.Equal(t, "peer", onlyOwner.Labels["ownerInstance"])

	// the four page types filter by role: remote agents visible only in the ownership
	// table still keep their type.
	captureNodes := listNodes(context.Background(), svcCtx, "g1", "prod", "capture")
	captureByID := map[string]Node{}
	for _, n := range captureNodes {
		captureByID[n.Id] = n
	}
	require.Contains(t, captureByID, "agent-owner-capture")
	assert.Equal(t, "capture", captureByID["agent-owner-capture"].Labels["role"])
	assert.NotContains(t, captureByID, "agent-other-game")
	assert.NotContains(t, captureByID, "agent-other-env")
	// other game/env filtered out
	assert.NotContains(t, byID, "agent-other-game")
	assert.NotContains(t, byID, "agent-other-env")

	// status filter mismatch → non-active nodes skipped
	nodes = listNodes(context.Background(), svcCtx, "", "", "active")
	for _, n := range nodes {
		assert.Equal(t, "active", n.Status)
	}

	// sorting: same rank, different lastSeen → most recent first (agent-only-owner's
	// ownership-table lastSeen=now-1min is newer than agent-remote's frozen snapshot
	// lastSeen=now-1h)
	all := listNodes(context.Background(), svcCtx, "", "", "")
	var order []string
	for _, n := range all {
		if n.Labels["ownerInstance"] == "peer" && (n.Id == "agent-remote" || n.Id == "agent-only-owner") {
			order = append(order, n.Id)
		}
	}
	require.Len(t, order, 2)
	assert.Equal(t, "agent-only-owner", order[0])
}

// ---- runtimeNodeListItem: disks branch ----

func TestRuntimeNodeListItemDiskMetricsV9(t *testing.T) {
	ms := registry.NewMetricsStore()
	ms.Add("n1", &opsv1.MetricsReport{
		Cpu:    &opsv1.CpuMetrics{UsagePercent: 20},
		Memory: &opsv1.MemoryMetrics{TotalBytes: 10},
		Disks: []*opsv1.DiskMetrics{
			{MountPoint: "/data", Device: "/dev/sdb", FsType: "ext4", TotalBytes: 100, UsedBytes: 40, AvailableBytes: 60, UsagePercent: 40, InodeTotal: 9, InodeUsed: 1},
		},
	})
	sess := &registry.AgentSession{
		AgentID: "n1", Addr: "a:1", Labels: map[string]string{},
		Functions: map[string]registry.FunctionMeta{}, LastSeen: time.Now(),
	}
	item := runtimeNodeListItem(sess, "active", ms)
	require.Len(t, item.node.Disks, 1)
	assert.Equal(t, "/data", item.node.Disks[0].MountPoint)
	assert.EqualValues(t, 9, item.node.Disks[0].InodeTotal)
}

// ---- offlineDatabaseNodeItems: filtering and error branches ----

func TestOfflineDatabaseNodeItemsBranchesV9(t *testing.T) {
	ctx := context.Background()
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Node{}))

	now := time.Now()
	seed := []model.Node{
		{NodeID: "db-off", Name: "off1", Type: "agent", IP: "10.0.0.1", Port: 1000,
			Meta: map[string]any{"gameId": "g1", "env": "prod"}},
		{NodeID: "", Name: "empty-id", Type: "agent"},                                            // empty NodeID skipped
		{NodeID: "db-game", Name: "game-node", Type: "game"},                                     // non-agent type skipped
		{NodeID: "db-other-game", Type: "agent", Meta: map[string]any{"gameId": "gX"}},           // game filter
		{NodeID: "db-other-env", Type: "agent", Meta: map[string]any{"gameId": "g1", "env": 42}}, // env filter + non-string meta
	}
	for i := range seed {
		require.NoError(t, db.Create(&seed[i]).Error)
	}

	regStore := registry.NewStore()
	require.NoError(t, regStore.UpsertAgent(&registry.AgentSession{
		AgentID: "db-reg", GameID: "g1", Env: "prod", Addr: "h:1",
		Labels: map[string]string{}, Functions: map[string]registry.FunctionMeta{},
		LastSeen: now, ExpireAt: now.Add(time.Hour),
	}))
	// DB node already registered in registry → skipped
	require.NoError(t, db.Create(&model.Node{NodeID: "db-reg", Type: "agent",
		Meta: map[string]any{"gameId": "g1", "env": "prod"}}).Error)

	svcCtx := &svc.ServiceContext{RegistryStore: regStore, NodeModel: model.NewNodeModel(db)}
	registered := map[string]bool{"db-reg": true}

	items := offlineDatabaseNodeItems(ctx, svcCtx, registered, "g1", "prod", "")
	require.Len(t, items, 1)
	assert.Equal(t, "db-off", items[0].node.Id)
	assert.Equal(t, "offline", items[0].node.Status)
	assert.Equal(t, "10.0.0.1:1000", items[0].node.Addr)

	// status filter mismatch → empty
	items = offlineDatabaseNodeItems(ctx, svcCtx, registered, "g1", "prod", "active")
	assert.Empty(t, items)

	// List error → empty
	badCtx := &svc.ServiceContext{NodeModel: model.NewNodeModel(db)}
	closeV9DB(t, db)
	items = offlineDatabaseNodeItems(ctx, badCtx, nil, "", "", "")
	assert.Empty(t, items)
}

// ---- opsAgentsList: cluster branch env scope filtering ----

func TestOpsAgentsListEnvScopeFilterV9(t *testing.T) {
	now := time.Now()
	store := registry.NewStore()
	require.NoError(t, store.UpsertAgent(&registry.AgentSession{
		AgentID: "a1", GameID: "g1", Env: "prod", Addr: "h:1",
		Labels: map[string]string{}, Functions: map[string]registry.FunctionMeta{},
		LastSeen: now, ExpireAt: now.Add(time.Minute),
	}))

	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		Cluster: &svc.ClusterRuntime{
			ListAgentOwners: func(context.Context) ([]cluster.AgentOwnerRecord, error) {
				return []cluster.AgentOwnerRecord{
					{AgentID: "a1", InstanceID: "self", GameID: "g1", Env: "prod", LastSeenAt: now},
					{AgentID: "a2", InstanceID: "peer", GameID: "g1", Env: "dev", LastSeenAt: now},
				}, nil
			},
		},
	}

	ctx := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "g1", Env: "prod"})
	resp, err := opsAgentsList(ctx, svcCtx, &OpsAgentsListRequest{})
	require.NoError(t, err)
	require.Len(t, resp.Agents, 1)
	assert.Equal(t, "a1", resp.Agents[0].AgentID)
}

// ---- AgentService.List: cluster branch env filtering ----

func TestAgentServiceListClusterEnvFilterV9(t *testing.T) {
	store := registry.NewStore()
	require.NoError(t, store.UpsertAgent(&registry.AgentSession{
		AgentID: "a1", GameID: "demo", Env: "prod", Addr: "h:1",
		Labels: map[string]string{}, Functions: map[string]registry.FunctionMeta{},
	}))
	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		Cluster: &svc.ClusterRuntime{
			ListAgentOwners: func(context.Context) ([]cluster.AgentOwnerRecord, error) {
				return []cluster.AgentOwnerRecord{
					{AgentID: "a1", InstanceID: "self", GameID: "demo", Env: "prod"},
					{AgentID: "a2", InstanceID: "peer", GameID: "demo", Env: "staging"},
				}, nil
			},
		},
	}
	s := NewAgentService(svcCtx)
	list, err := s.List(context.Background(), "demo", "staging", "")
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "a2", list[0].AgentID)
	assert.Equal(t, "peer", list[0].OwnerInstance)
}
