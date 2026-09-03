// coverage_v9_test.go 补充 internal/api/ops 未覆盖分支：
// 错误路径（closed DB / 保存失败 / 扩展事件失败）、集群视图、
// 备份/告警/指标/维护/通知等 handler 的成功与服务错误分支。
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
	"github.com/cuihairu/croupier/internal/transport"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	"github.com/gin-gonic/gin"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"gorm.io/gorm"
)

// ---- helpers（V9 后缀，避免与现有 helper 冲突） ----

func closeV9DB(t *testing.T, db *gorm.DB) {
	t.Helper()
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())
}

// newV9BackupSvcCtx 返回带 BackupModel 的 ctx 与底层 db（用于关闭连接制造错误）。
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

// ---- Agent 进程命令 helper：成功 + nil svcCtx + wrapper 错误 ----

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

	svcCtx := &svc.ServiceContext{}
	ctx := context.Background()

	// ExecCommand 成功 → 响应转换
	execResp, err := opsAgentExecCommand(ctx, svcCtx, &OpsExecCommandRequest{AgentID: "agent-1", Command: "ls", Timeout: 3})
	require.NoError(t, err)
	assert.EqualValues(t, 0, execResp.Result.ExitCode)
	assert.Equal(t, "v9-out", execResp.Result.Stdout)

	// Start/Stop/Restart 成功
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

	// ExecuteCommand / StartProcess wrapper 调用失败
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

// ---- 备份/告警：closed DB 错误路径 ----

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

	// 备份不存在 → FindByBackupID 错误
	okBackupCtx, _ := newV9BackupSvcCtx(t)
	_, err = opsBackupDelete(ctx, okBackupCtx, &OpsBackupDeleteRequest{ID: "missing"})
	require.Error(t, err)
	_, err = opsBackupDownload(ctx, okBackupCtx, &OpsBackupDownloadRequest{ID: "missing"})
	require.Error(t, err)

	alertCtx, alertDB := newV9AlertSvcCtx(t)
	closeV9DB(t, alertDB)
	_, err = opsAlerts(ctx, alertCtx, &OpsAlertsRequest{})
	require.Error(t, err)
	// 非法 alertID → Sscanf 错误
	_, err = opsAlertSilence(ctx, alertCtx, &OpsAlertSilenceRequest{AlertID: "not-a-number"})
	require.Error(t, err)
	// 合法 alertID 但 CreateSilence 失败
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

// ---- GetClusterInfo：单实例带 registry 计数 + handler 错误 ----

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

	// handler：nil svcCtx → 服务错误分支
	gin.SetMode(gin.TestMode)
	h := NewHandler(NewService(nil))
	ctx, rec := newOpsTestContext(http.MethodGet, "/api/v1/ops/cluster", "")
	h.ClusterInfo(ctx)
	assert.GreaterOrEqual(t, rec.Code, 400)
}

// ---- GetClusterInfo：mesh 离线对端 + LBStats + LBStatsQuery handler 成功 ----

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
		// 在线成员：inst-1（self）+ inst-2；inst-3 仅存在于 mesh last-known → 离线
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
	// inst-2 在线；inst-3 仅 mesh last-known → 展示为离线
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

	// Service.LBStatsQuery 成功
	result, err := s.LBStatsQuery(context.Background(), "haproxy_up")
	require.NoError(t, err)
	assert.NotNil(t, result)

	// handler 成功
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

// ---- recordOpsAudit：AuditService 非空时记录审计 ----

func TestRecordOpsAuditViaNodeDrainV9(t *testing.T) {
	store := svc.NewOpsStateStore(t.TempDir())
	svcCtx := &svc.ServiceContext{
		OpsStateStore: store,
		AuditService:  audit.NewAuditService(audit.NewInMemoryAuditStore(), nil),
	}
	caller := &fakeCaller{}
	svcCtx.AgentSessionResolver = &fakeResolver{conns: map[string]transport.SessionCaller{"node-1": caller}}

	// 无 username → actor=system
	resp, err := opsNodeDrain(context.Background(), svcCtx, &OpsNodeCommandsRequest{NodeId: "node-1"})
	require.NoError(t, err)
	assert.Equal(t, "draining", resp.Status)

	// 有 username → actor=username
	ctx := context.WithValue(context.Background(), "username", "v9-admin")
	resp2, err := opsNodeDrain(ctx, svcCtx, &OpsNodeCommandsRequest{NodeId: "node-1"})
	require.NoError(t, err)
	assert.Equal(t, "node-1", resp2.NodeId)

	// restart / undrain / maintenance 也走 recordOpsAudit
	_, err = opsNodeRestart(ctx, svcCtx, &OpsNodeCommandsRequest{NodeId: "node-1"})
	require.NoError(t, err)
	_, err = opsNodeUndrain(ctx, svcCtx, &OpsNodeCommandsRequest{NodeId: "node-1"})
	require.NoError(t, err)
	_, err = opsMaintenanceUpdate(ctx, svcCtx, &OpsMaintenanceUpdateRequest{
		Windows: []OpsMaintenanceWindow{{GameID: "g", Env: "prod"}},
	})
	require.NoError(t, err)

	// Call 失败仍继续：restart / undrain 仅记录告警不中断
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

// ---- opsHealthGet / opsHealthRun：填充状态与 http 分支 ----

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

	// 两个检查各跑一次，再重复跑 http → 验证旧状态被替换而非追加；custom 走 default 分支
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

// ---- OpsStateStore 持久化失败 → health/maintenance Update 错误 ----

func TestOpsStateStoreSaveFailureV9(t *testing.T) {
	ctx := context.Background()
	base := t.TempDir()
	// 把状态文件路径占位成目录 → WriteFile 失败
	require.NoError(t, os.Mkdir(filepath.Join(base, "ops_state.json"), 0o755))
	store := svc.NewOpsStateStore(base)
	svcCtx := &svc.ServiceContext{OpsStateStore: store}

	// Update 失败（fn 已应用到内存）
	_, err := opsHealthUpdate(ctx, svcCtx, &OpsHealthUpdateRequest{Checks: []OpsHealthCheck{
		{ID: "hc-1", Kind: "tcp", Target: "127.0.0.1:1"},
	}})
	require.Error(t, err)

	// 内存中 check 仍存在 → opsHealthRun 找到目标后 Update 失败
	_, err = opsHealthRun(ctx, svcCtx, &OpsHealthRunRequest{ID: "hc-1"})
	require.Error(t, err)

	_, err = opsMaintenanceUpdate(ctx, svcCtx, &OpsMaintenanceUpdateRequest{})
	require.Error(t, err)
}

// ---- opsMaintenanceGet：非空 windows 渲染 ----

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

// ---- opsMetrics：gameId 过滤不匹配 ----

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
		LastSeen: now, ExpireAt: now.Add(-time.Hour), // 过期 → expired
	}))
	require.NoError(t, store.UpsertAgent(&registry.AgentSession{
		AgentID: "bare-1", GameID: "g", Env: "prod", Addr: "h:2",
		Functions: map[string]registry.FunctionMeta{},
		// Labels nil、LastSeen 零值
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

// ---- opsConfig：settings 初始化后的主分支 + state/env 回落 ----

func TestOpsConfigSettingsLayerV9(t *testing.T) {
	ctx := context.Background()

	// 初始化全局分层设置（L2 为空）；不并行，避免全局单例竞争
	l := settings.InitLayered(ctx, nil, nil)
	require.NotNil(t, l)
	require.NotNil(t, settings.Current())

	// snap 为空 → state store 回落
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

	// snap + state 均空 → env 回落
	t.Setenv("CROUPIER_ALERTMANAGER_URL", "http://am.env9")
	t.Setenv("CROUPIER_GRAFANA_EXPLORE_URL", "http://grafana.env9")
	t.Setenv("CROUPIER_JAEGER_URL", "http://jaeger.env9")
	resp, err = opsConfig(ctx, &svc.ServiceContext{}, &OpsConfigRequest{})
	require.NoError(t, err)
	assert.Equal(t, "http://am.env9", resp.AlertmanagerURL)
	assert.Equal(t, "http://grafana.env9", resp.GrafanaExploreURL)
	assert.Equal(t, "http://jaeger.env9", resp.JaegerURL)
}

// ---- 通知扩展：配置边界与错误路径 ----

func TestOpsNotificationsEdgeCasesV9(t *testing.T) {
	ctx := context.Background()

	// 空配置 → 未提取 → 返回空默认（ok=true 分支）
	svcCtx1, installSvc1, _ := newV9InstallationSvcCtx(t)
	installV9Notification(t, installSvc1, map[string]any{})
	resp, err := opsNotificationsGet(ctx, svcCtx1, &OpsNotificationsGetRequest{})
	require.NoError(t, err)
	assert.False(t, resp.Enabled)
	assert.Empty(t, resp.Channels)
	assert.Empty(t, resp.Rules)

	// 非法 ConfigJSON → unmarshal 错误
	svcCtx2, installSvc2, db2 := newV9InstallationSvcCtx(t)
	inst2 := installV9Notification(t, installSvc2, map[string]any{})
	require.NoError(t, db2.Model(&model.ExtensionInstallation{}).Where("id = ?", inst2.ID).Update("config_json", "not-json").Error)
	_, err = opsNotificationsGet(ctx, svcCtx2, &OpsNotificationsGetRequest{})
	require.Error(t, err)

	// channels 类型错误 → extract 错误
	svcCtx3, installSvc3, db3 := newV9InstallationSvcCtx(t)
	inst3 := installV9Notification(t, installSvc3, map[string]any{})
	require.NoError(t, db3.Model(&model.ExtensionInstallation{}).Where("id = ?", inst3.ID).Update("config_json", `{"channels":"oops"}`).Error)
	_, err = opsNotificationsGet(ctx, svcCtx3, &OpsNotificationsGetRequest{})
	require.Error(t, err)

	// uninstalled 跳过 → 无激活安装
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

	// List 错误 → Get/Update 均报错
	svcCtx5, installSvc5, db5 := newV9InstallationSvcCtx(t)
	installV9Notification(t, installSvc5, map[string]any{})
	closeV9DB(t, db5)
	_, err = opsNotificationsGet(ctx, svcCtx5, &OpsNotificationsGetRequest{})
	require.Error(t, err)
	_, err = opsNotificationsUpdate(ctx, svcCtx5, &OpsNotificationsUpdateRequest{})
	require.Error(t, err)

	// nil req + 无扩展 → 成功（find 返回 false）
	_, err = opsNotificationsUpdate(ctx, &svc.ServiceContext{}, nil)
	require.NoError(t, err)
}

// ---- 通知更新：保存成功但审计事件失败 ----

func TestOpsNotificationsRecordEventErrorV9(t *testing.T) {
	ctx := context.Background()
	svcCtx, installSvc, db := newV9InstallationSvcCtx(t)
	installV9Notification(t, installSvc, map[string]any{})
	// 仅阻断 notifications_update 事件：update_config 正常落库，审计事件失败
	require.NoError(t, db.Exec(`CREATE TRIGGER v9_block_notif BEFORE INSERT ON extension_events
WHEN NEW.event_type = 'notifications_update'
BEGIN
  SELECT RAISE(ABORT, 'v9 blocked');
END;`).Error)

	_, err := opsNotificationsUpdate(ctx, svcCtx, &OpsNotificationsUpdateRequest{Enabled: true})
	require.Error(t, err)

	// 配置已保存成功
	item, ok, err := findActiveExtensionInstallationByID(ctx, svcCtx, officialNotificationID)
	require.NoError(t, err)
	require.True(t, ok)
	assert.Contains(t, string(item.ConfigJSON), `"enabled"`)
}

// ---- extractNotificationConfig：marshal/unmarshal 错误 ----

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

// ---- handler 成功路径：exec/start/stop/restart/drain/restart/undrain/health/maintenance ----

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

	nodeCtx := &svc.ServiceContext{
		OpsStateStore:        svc.NewOpsStateStore(t.TempDir()),
		AgentSessionResolver: &fakeResolver{conns: map[string]transport.SessionCaller{"node-1": caller}},
	}

	runHandlerV9 := func(h func(*gin.Context), method, body string) int {
		ctx, rec := newOpsTestContext(method, "/api/v1/ops", body)
		h(ctx)
		return rec.Code
	}

	// agent 进程命令成功
	agentH := NewHandler(NewService(&svc.ServiceContext{}))
	assert.Equal(t, http.StatusOK, runHandlerV9(agentH.OpsAgentExecCommand, http.MethodPost, `{"AgentID":"agent-1","command":"ls","args":[],"timeout":1}`))
	assert.Equal(t, http.StatusOK, runHandlerV9(agentH.OpsAgentProcessStart, http.MethodPost, `{"AgentID":"agent-1","Name":"p1"}`))
	assert.Equal(t, http.StatusOK, runHandlerV9(agentH.OpsAgentProcessStop, http.MethodPost, `{"AgentID":"agent-1","Name":"p1"}`))
	assert.Equal(t, http.StatusOK, runHandlerV9(agentH.OpsAgentProcessRestart, http.MethodPost, `{"AgentID":"agent-1","Name":"p1"}`))

	// node 命令成功
	nodeH := NewHandler(NewService(nodeCtx))
	assert.Equal(t, http.StatusOK, runHandlerV9(nodeH.OpsNodeDrain, http.MethodPost, `{"nodeId":"node-1"}`))
	assert.Equal(t, http.StatusOK, runHandlerV9(nodeH.OpsNodeRestart, http.MethodPost, `{"nodeId":"node-1"}`))
	assert.Equal(t, http.StatusOK, runHandlerV9(nodeH.OpsNodeUndrain, http.MethodPost, `{"nodeId":"node-1"}`))

	// health / maintenance 成功
	stateH := NewHandler(NewService(&svc.ServiceContext{OpsStateStore: svc.NewOpsStateStore(t.TempDir())}))
	assert.Equal(t, http.StatusOK, runHandlerV9(stateH.OpsHealthUpdate, http.MethodPost, `{"checks":[{"id":"hc-1","kind":"tcp","target":"127.0.0.1:80"}]}`))
	assert.Equal(t, http.StatusOK, runHandlerV9(stateH.OpsHealthRun, http.MethodPost, `{"id":"hc-1"}`))
	assert.Equal(t, http.StatusOK, runHandlerV9(stateH.OpsMaintenanceUpdate, http.MethodPost, `{"windows":[{"gameId":"g","env":"prod"}]}`))
}

// ---- handler 服务错误路径 ----

func TestHandlerServiceErrorPathsV9(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 备份：closed DB / 不存在
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

	// 告警：closed DB / 非法 ID
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

	// 通知：扩展 List 错误
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

// ---- listNodes：集群归属表 + 过滤 + 排序 ----

func TestListNodesClusterAndDBBranchesV9(t *testing.T) {
	now := time.Now()
	store := registry.NewStore()
	// 远端持有：本地冻结快照（在线判定走归属表）；非空 labels 覆盖拷贝循环
	require.NoError(t, store.UpsertAgent(&registry.AgentSession{
		AgentID: "agent-remote", GameID: "g1", Env: "prod", Addr: "h:2",
		Labels: map[string]string{"zone": "z1"}, Functions: map[string]registry.FunctionMeta{},
		LastSeen: now.Add(-time.Hour), ExpireAt: now.Add(-time.Minute),
	}))
	// 远端持有 + 已 drained
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
					{AgentID: "agent-other-game", InstanceID: "peer", GameID: "gX", Env: "prod", LastSeenAt: now},
					{AgentID: "agent-other-env", InstanceID: "peer", GameID: "g1", Env: "dev", LastSeenAt: now},
				}, nil
			},
		},
	}

	// nil ctx + game/env 过滤
	nodes := listNodes(nil, svcCtx, "g1", "prod", "")
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
	// 仅归属表（不在本地 registry）→ online + owner 标注
	onlyOwner, ok := byID["agent-only-owner"]
	require.True(t, ok)
	assert.Equal(t, "online", onlyOwner.Status)
	assert.Equal(t, "peer", onlyOwner.Labels["ownerInstance"])
	// 过滤掉其他 game/env
	assert.NotContains(t, byID, "agent-other-game")
	assert.NotContains(t, byID, "agent-other-env")

	// 状态过滤不匹配 → 非 active 节点被跳过
	nodes = listNodes(context.Background(), svcCtx, "", "", "active")
	for _, n := range nodes {
		assert.Equal(t, "active", n.Status)
	}

	// 排序：同 rank 不同 lastSeen → 最近优先（agent-only-owner 的归属表
	// lastSeen=now-1min，比 agent-remote 的冻结快照 lastSeen=now-1h 更新）
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

// ---- runtimeNodeListItem：disks 分支 ----

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

// ---- offlineDatabaseNodeItems：过滤与错误分支 ----

func TestOfflineDatabaseNodeItemsBranchesV9(t *testing.T) {
	ctx := context.Background()
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Node{}))

	now := time.Now()
	seed := []model.Node{
		{NodeID: "db-off", Name: "off1", Type: "agent", IP: "10.0.0.1", Port: 1000,
			Meta: map[string]any{"gameId": "g1", "env": "prod"}},
		{NodeID: "", Name: "empty-id", Type: "agent"},                                            // 空 NodeID 跳过
		{NodeID: "db-game", Name: "game-node", Type: "game"},                                     // 非 agent 类型跳过
		{NodeID: "db-other-game", Type: "agent", Meta: map[string]any{"gameId": "gX"}},           // game 过滤
		{NodeID: "db-other-env", Type: "agent", Meta: map[string]any{"gameId": "g1", "env": 42}}, // env 过滤 + 非字符串 meta
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
	// registry 中已注册的 DB 节点 → 跳过
	require.NoError(t, db.Create(&model.Node{NodeID: "db-reg", Type: "agent",
		Meta: map[string]any{"gameId": "g1", "env": "prod"}}).Error)

	svcCtx := &svc.ServiceContext{RegistryStore: regStore, NodeModel: model.NewNodeModel(db)}
	registered := map[string]bool{"db-reg": true}

	items := offlineDatabaseNodeItems(ctx, svcCtx, registered, "g1", "prod", "")
	require.Len(t, items, 1)
	assert.Equal(t, "db-off", items[0].node.Id)
	assert.Equal(t, "offline", items[0].node.Status)
	assert.Equal(t, "10.0.0.1:1000", items[0].node.Addr)

	// 状态过滤不匹配 → 空
	items = offlineDatabaseNodeItems(ctx, svcCtx, registered, "g1", "prod", "active")
	assert.Empty(t, items)

	// List 错误 → 空
	badCtx := &svc.ServiceContext{NodeModel: model.NewNodeModel(db)}
	closeV9DB(t, db)
	items = offlineDatabaseNodeItems(ctx, badCtx, nil, "", "", "")
	assert.Empty(t, items)
}

// ---- opsAgentsList：集群分支 env scope 过滤 ----

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

// ---- AgentService.List：集群分支 env 过滤 ----

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
