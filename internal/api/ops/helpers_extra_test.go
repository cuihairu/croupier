package ops

// helpers.go 补测：scope 过滤回退路径（本地视图 game/env 不匹配 continue）、
// metrics/processes 的 store 缺失与快照缺项守卫、systemInfo/meta 的集群
// 归属放行后内存未命中 404、exec/update 审计留痕的 actor 取名分支、
// alert 流转的存在性/越域/落库错误分支、drain/restart/undrain 无连接 404、
// enforceRequestScope 越域 403、recordOpsAudit 落库失败 write-gap。
//
// 诚实跳过（不可达，头部记录）：helpers.go:174（opsAgentMeta 的
// RegistryStore nil 分支）与 :335（opsAgentSystemInfo 的 store nil 透传）——
// 两者前置的 requireAgentScope 在 RegistryStore 为 nil 时已先报
// "registry store unavailable"，分支体不可达。:825/:973 的 proto.Marshal
// 错误分支对空请求体不可触发。

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/cuihairu/croupier/core/transport"
	"github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/cluster"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/executionlog"
	"github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// failAuditStore 落库恒失败的审计 store（仅覆盖 Log 触达的方法）。
type failAuditStore struct {
	audit.AuditStore
}

func (failAuditStore) Create(*audit.AuditRecord) error { return errors.New("audit db down") }
func (failAuditStore) GetLatestRecord() (*audit.AuditRecord, error) {
	return nil, gorm.ErrRecordNotFound
}

// ownerResolverStub 集群共享归属解析替身：FindOwner 按 map 回答，其余空实现。
type ownerResolverStub struct {
	owners map[string]cluster.AgentOwnerRecord
}

func (s *ownerResolverStub) ClaimOwner(context.Context, string, string, string, string, uint64, map[string]string) error {
	return nil
}
func (s *ownerResolverStub) Touch(context.Context, string) error   { return nil }
func (s *ownerResolverStub) Release(context.Context, string) error { return nil }
func (s *ownerResolverStub) ResolveOwner(context.Context, string) (*cluster.PeerInfo, error) {
	return nil, nil
}
func (s *ownerResolverStub) FindOwner(_ context.Context, agentID string) (*cluster.AgentOwnerRecord, error) {
	if rec, ok := s.owners[agentID]; ok {
		return &rec, nil
	}
	return nil, gorm.ErrRecordNotFound
}
func (s *ownerResolverStub) ListAliveOwners(context.Context) ([]cluster.AgentOwnerRecord, error) {
	return nil, nil
}
func (s *ownerResolverStub) CountAgentsByOwner(context.Context) (map[string]int64, error) {
	return nil, nil
}
func (s *ownerResolverStub) SelfOwnerScope(context.Context, string) (string, string, bool) {
	return "", "", false
}
func (s *ownerResolverStub) SetMesh(*cluster.MeshInterconnect) {}

// noConnResolver 连接解析恒未命中的会话解析替身（drain/restart/undrain 404）。
type noConnResolver struct{}

func (noConnResolver) ResolveAgentConn(string) (transport.SessionCaller, bool) { return nil, false }

func TestRecordOpsAuditWriteGap(t *testing.T) {
	// AuditService.Log 落库失败 → ReportWriteGap 兜底（不得 panic）
	svcCtx := &svc.ServiceContext{AuditService: audit.NewAuditService(failAuditStore{}, nil)}
	recordOpsAudit(context.Background(), svcCtx, audit.EventNodeDrain, "n1", "success", nil)

	// AuditService 未装配 → 静默跳过
	recordOpsAudit(context.Background(), &svc.ServiceContext{}, audit.EventNodeDrain, "n1", "success", nil)
	recordOpsAudit(context.Background(), nil, audit.EventNodeDrain, "n1", "success", nil)
}

func TestOpsAgentsListScopeFilterFallback(t *testing.T) {
	store := registry.NewStore()
	require.NoError(t, store.UpsertAgent(&registry.AgentSession{AgentID: "a1", GameID: "demo", Env: "prod"}))
	require.NoError(t, store.UpsertAgent(&registry.AgentSession{AgentID: "a2", GameID: "other", Env: "dev"}))
	svcCtx := &svc.ServiceContext{RegistryStore: store}

	ctx := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "demo", Env: "prod"})
	resp, err := opsAgentsList(ctx, svcCtx, &OpsAgentsListRequest{})
	require.NoError(t, err)
	require.Len(t, resp.Agents, 1, "scope filter must drop out-of-scope agents in the local fallback path")
	require.Equal(t, "a1", resp.Agents[0].AgentID)

	// nil svcCtx → 空列表
	empty, err := opsAgentsList(context.Background(), nil, &OpsAgentsListRequest{})
	require.NoError(t, err)
	require.Empty(t, empty.Agents)
}

func TestOpsAgentMetricsProcessesSnapshotGuards(t *testing.T) {
	ctx := context.Background()
	store := registry.NewStore()
	require.NoError(t, store.UpsertAgent(&registry.AgentSession{AgentID: "a1", GameID: "demo", Env: "prod"}))

	// MetricsStore 未装配 → 空指标
	svcCtx := &svc.ServiceContext{RegistryStore: store}
	resp, err := opsAgentMetrics(ctx, svcCtx, &OpsAgentMetricsRequest{AgentID: "a1"})
	require.NoError(t, err)
	require.Empty(t, resp.Metrics)

	// 快照未命中 → 空指标
	svcCtx.MetricsStore = registry.NewMetricsStore()
	resp, err = opsAgentMetrics(ctx, svcCtx, &OpsAgentMetricsRequest{AgentID: "a1"})
	require.NoError(t, err)
	require.Empty(t, resp.Metrics)

	// 快照缺项：networks 含 nil 元素（跳过）、supervised 进程含 nil（跳过）
	metrics := registry.NewMetricsStore()
	metrics.Add("a1", &opsv1.MetricsReport{
		Networks: []*opsv1.NetworkMetrics{nil, {Interface: "eth0", BytesSent: 10, BytesRecv: 20}},
		SupervisedProcesses: []*opsv1.SupervisedProcessSnapshot{
			nil,
			{Name: "game", Pid: 7, State: opsv1.ProcessState_PROCESS_STATE_RUNNING, RestartCount: 2},
		},
	})
	svcCtx.MetricsStore = metrics
	resp, err = opsAgentMetrics(ctx, svcCtx, &OpsAgentMetricsRequest{AgentID: "a1"})
	require.NoError(t, err)
	require.Len(t, resp.Metrics, 1)
	require.Len(t, resp.Metrics[0].Networks, 1, "nil network entries must be skipped")
	require.Equal(t, "eth0", resp.Metrics[0].Networks[0].Interface)

	procResp, err := opsAgentProcesses(ctx, svcCtx, &OpsAgentProcessesRequest{AgentID: "a1"})
	require.NoError(t, err)
	require.Len(t, procResp.Processes, 1, "nil snapshots must be skipped")
	require.Equal(t, "game", procResp.Processes[0].Name)
	require.Equal(t, int32(7), procResp.Processes[0].Pid)
	require.Equal(t, "running", procResp.Processes[0].State)

	// processes：MetricsStore 未装配 → 空
	procResp, err = opsAgentProcesses(ctx, &svc.ServiceContext{RegistryStore: store}, &OpsAgentProcessesRequest{AgentID: "a1"})
	require.NoError(t, err)
	require.Empty(t, procResp.Processes)
}

func TestOpsAgentSystemInfoClusterScopeGhost(t *testing.T) {
	ctx := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "demo", Env: "prod"})
	store := registry.NewStore()
	require.NoError(t, store.UpsertAgent(&registry.AgentSession{AgentID: "a1", GameID: "demo", Env: "prod"}))

	// 集群归属放行 ghost（本实例内存/DB 均无）→ requireAgentScope 通过但
	// registry 遍历未命中 → "agent not found"（meta 与 systemInfo 同路径）
	owners := &ownerResolverStub{owners: map[string]cluster.AgentOwnerRecord{
		"ghost": {AgentID: "ghost", GameID: "demo", Env: "prod", LastSeenAt: time.Now()},
	}}
	svcCtx := &svc.ServiceContext{RegistryStore: store, Cluster: &svc.ClusterRuntime{Resolver: owners}}

	_, err := opsAgentMeta(ctx, svcCtx, &OpsAgentMetaRequest{AgentId: "ghost"})
	require.EqualError(t, err, "agent not found")

	_, err = opsAgentSystemInfo(ctx, svcCtx, &OpsAgentSystemInfoRequest{AgentID: "ghost"})
	require.EqualError(t, err, "agent not found")

	// svcCtx 未装配 → 400
	_, err = opsAgentSystemInfo(context.Background(), nil, &OpsAgentSystemInfoRequest{AgentID: "a1"})
	require.ErrorContains(t, err, "service context is required")
}

func TestAuditAgentExecAndUpdateActorName(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.ExecutionLog{}))
	writer := executionlog.NewWriter(db, executionlog.Config{Enabled: true, FlushInterval: 50 * time.Millisecond})
	writer.Run(context.Background())
	t.Cleanup(writer.Stop)

	svcCtx := &svc.ServiceContext{ExecutionLogWriter: writer}
	// ctx 带 username → actor 取实名（而非 "unknown"）
	ctx := context.WithValue(context.Background(), "username", "alice") //nolint:staticcheck // 与 utils.CurrentUsername 的键契约一致

	auditAgentExec(ctx, svcCtx, &OpsExecCommandRequest{AgentID: "a1", Command: "ls", Args: []string{"-l"}, Timeout: 5},
		time.Now(), &OpsExecCommandResult{ExitCode: 0, Stdout: "ok"}, nil)

	auditAgentUpdate(ctx, svcCtx, &OpsProcessUpdateRequest{AgentID: "a1", Name: "game", StagedPath: "/tmp/x", ExpectedSha256: "sha"},
		time.Now(), &opsv1.UpdateProcessResponse{Success: true, NewFingerprint: "fp"}, nil)
}

func TestOpsAlertHandleBranches(t *testing.T) {
	ctx := context.Background()

	// req 缺失 → 422
	_, err := opsAlertHandle(ctx, &svc.ServiceContext{}, "1", nil)
	require.Error(t, err)

	// alerts 表缺失 → FindByIDs 错误透传
	db, err := gorm.Open(gsqlite.Open(t.TempDir()+"/al.db"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Alert{}))
	require.NoError(t, db.Migrator().DropTable("alerts"))
	svcCtx := &svc.ServiceContext{AlertModel: model.NewAlertModel(db)}
	_, err = opsAlertHandle(ctx, svcCtx, "1", &OpsAlertHandleRequest{Action: "acknowledge"})
	require.Error(t, err)

	// 重建表：越域（env 不匹配）与落库失败分支
	db2, err := gorm.Open(gsqlite.Open(t.TempDir()+"/al2.db"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db2.AutoMigrate(&model.Alert{}))
	require.NoError(t, model.NewAlertModel(db2).Create(ctx, &model.Alert{
		Level: "warn", Message: "cpu high", GameID: "demo", Env: "prod",
	}))
	svcCtx2 := &svc.ServiceContext{AlertModel: model.NewAlertModel(db2)}

	// env 越域 → 拒绝
	scopedCtx := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "demo", Env: "staging"})
	_, err = opsAlertHandle(scopedCtx, svcCtx2, "1", &OpsAlertHandleRequest{Action: "acknowledge"})
	require.ErrorContains(t, err, "does not belong to env")

	// handled_at 列缺失 → FindByIDs 通过但 UpdateHandling 落库失败
	require.NoError(t, db2.Migrator().DropColumn(&model.Alert{}, "handled_at"))
	_, err = opsAlertHandle(ctx, svcCtx2, "1", &OpsAlertHandleRequest{Action: "escalate", Remark: " paging"})
	require.Error(t, err, "missing handled_at column must fail the persistence")
}

func TestOpsNodeActionsNoConnection(t *testing.T) {
	ctx := context.Background()
	store := registry.NewStore()
	require.NoError(t, store.UpsertAgent(&registry.AgentSession{AgentID: "a1", GameID: "demo", Env: "prod"}))
	svcCtx := &svc.ServiceContext{RegistryStore: store, AgentSessionResolver: noConnResolver{}}

	_, err := opsNodeDrain(ctx, svcCtx, &OpsNodeCommandsRequest{NodeId: "a1"})
	require.ErrorContains(t, err, "no active connection")

	_, err = opsNodeRestart(ctx, svcCtx, &OpsNodeCommandsRequest{NodeId: "a1"})
	require.ErrorContains(t, err, "no active connection")

	_, err = opsNodeUndrain(ctx, svcCtx, &OpsNodeCommandsRequest{NodeId: "a1"})
	require.ErrorContains(t, err, "no active connection")
}

func TestEnforceRequestScopeBranches(t *testing.T) {
	ctx := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "demo", Env: "prod"})

	// code 走 errorx 结构（渲染层输出 scope_mismatch，见 provision_test.go:238），
	// 直调断言走可读消息
	require.ErrorContains(t, enforceRequestScope(ctx, "demo", "staging"), "not in authenticated scope")
	require.ErrorContains(t, enforceRequestScope(ctx, "other", "prod"), "not in authenticated scope")
	require.NoError(t, enforceRequestScope(ctx, "demo", "prod"))
	// 平台级 scope 不限制
	require.NoError(t, enforceRequestScope(context.Background(), "any", "any"))
}
