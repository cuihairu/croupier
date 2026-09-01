package ops

import (
	"context"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/cluster"
	agentops "github.com/cuihairu/croupier/internal/logic/ops"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/cuihairu/croupier/internal/transport"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"gorm.io/gorm"
)

// ---- fakes ----

// fakeCaller 模拟 agent TCP 会话：按消息类型返回预置 protobuf 响应。
type fakeCaller struct {
	errOn   map[uint32]error         // 指定 msgID 返回错误
	calls   []uint32                 // 记录调用过的 msgID
	respFor map[uint32]proto.Message // msgID → 响应消息
}

func (f *fakeCaller) Call(ctx context.Context, msgID uint32, reqBody []byte) (uint32, []byte, error) {
	f.calls = append(f.calls, msgID)
	if err, ok := f.errOn[msgID]; ok {
		return 0, nil, err
	}
	if resp, ok := f.respFor[msgID]; ok {
		body, err := proto.Marshal(resp)
		if err != nil {
			return 0, nil, err
		}
		return protocol.GetResponseMsgID(msgID), body, nil
	}
	return protocol.GetResponseMsgID(msgID), []byte{}, nil
}

type fakeResolver struct {
	conns map[string]transport.SessionCaller
}

func (r *fakeResolver) ResolveAgentConn(agentID string) (transport.SessionCaller, bool) {
	c, ok := r.conns[agentID]
	return c, ok
}

func newOpsFakeClient(t *testing.T, caller *fakeCaller, agentID string) {
	t.Helper()
	client := agentops.GetAgentOpsClient()
	client.SetSessionResolver(&fakeResolver{conns: map[string]transport.SessionCaller{agentID: caller}})
}

// ---- opsNodeDrain / opsNodeRestart / opsNodeUndrain 成功与错误路径 ----

func TestOpsNodeDrainRestartUndrainPaths(t *testing.T) {
	ctx := context.Background()
	store := svc.NewOpsStateStore(t.TempDir())
	svcCtx := &svc.ServiceContext{OpsStateStore: store}
	caller := &fakeCaller{}
	resolver := &fakeResolver{conns: map[string]transport.SessionCaller{"node-1": caller}}
	svcCtx.AgentSessionResolver = resolver

	// 空 nodeId → 400
	_, err := opsNodeDrain(ctx, svcCtx, &OpsNodeCommandsRequest{})
	require.Error(t, err)
	_, err = opsNodeRestart(ctx, svcCtx, &OpsNodeCommandsRequest{})
	require.Error(t, err)
	_, err = opsNodeUndrain(ctx, svcCtx, &OpsNodeCommandsRequest{})
	require.Error(t, err)

	// 未知节点 → 404
	_, err = opsNodeDrain(ctx, svcCtx, &OpsNodeCommandsRequest{NodeId: "ghost"})
	require.Error(t, err)
	_, err = opsNodeRestart(ctx, svcCtx, &OpsNodeCommandsRequest{NodeId: "ghost"})
	require.Error(t, err)
	_, err = opsNodeUndrain(ctx, svcCtx, &OpsNodeCommandsRequest{NodeId: "ghost"})
	require.Error(t, err)

	// drain 成功（Call 成功）
	resp, err := opsNodeDrain(ctx, svcCtx, &OpsNodeCommandsRequest{NodeId: "node-1"})
	require.NoError(t, err)
	assert.Equal(t, "draining", resp.Status)

	// drain 状态已记录
	snap := store.Snapshot()
	_, drained := snap.Nodes.Drained["node-1"]
	assert.True(t, drained)

	// Call 失败也继续（记录状态不中断）
	errCaller := &fakeCaller{errOn: map[uint32]error{protocol.MsgProviderDrainRequest: assert.AnError}}
	svcCtx.AgentSessionResolver = &fakeResolver{conns: map[string]transport.SessionCaller{"node-2": errCaller}}
	resp, err = opsNodeDrain(ctx, svcCtx, &OpsNodeCommandsRequest{NodeId: "node-2"})
	require.NoError(t, err)
	assert.Equal(t, "node-2", resp.NodeId)

	// restart 成功
	svcCtx.AgentSessionResolver = resolver
	resp2, err := opsNodeRestart(ctx, svcCtx, &OpsNodeCommandsRequest{NodeId: "node-1"})
	require.NoError(t, err)
	assert.Equal(t, "restarting", resp2.Status)

	// undrain 清除 drain 状态
	resp3, err := opsNodeUndrain(ctx, svcCtx, &OpsNodeCommandsRequest{NodeId: "node-1"})
	require.NoError(t, err)
	assert.Equal(t, "active", resp3.Status)
	snap = store.Snapshot()
	_, drained = snap.Nodes.Drained["node-1"]
	assert.False(t, drained)
}

// ---- AgentService 进程命令（AgentOpsClient 真实编解码 + fake 会话） ----

func TestAgentServiceProcessCommandsViaOpsClient(t *testing.T) {
	// 构造 ops client 全局 resolver：agent-1 有会话，其他无
	caller := &fakeCaller{respFor: map[uint32]proto.Message{
		protocol.MsgExecuteCommandRequest: &opsv1.ExecuteCommandResponse{ExitCode: 0, StdOut: "ok-out", StdErr: "err-out"},
		protocol.MsgStartProcessRequest:   &opsv1.StartProcessResponse{Pid: 4242},
		protocol.MsgStopProcessRequest:    &opsv1.StopProcessResponse{},
		protocol.MsgRestartProcessRequest: &opsv1.RestartProcessResponse{},
	}}
	client := agentops.GetAgentOpsClient()
	client.SetSessionResolver(&fakeResolver{conns: map[string]transport.SessionCaller{"agent-1": caller}})
	t.Cleanup(func() { client.SetSessionResolver(nil) }) // 全局单例：用后复位，不污染其他用例
	s := NewAgentService(&svc.ServiceContext{})
	ctx := context.Background()

	res, err := s.ExecCommand(ctx, "agent-1", "ls", []string{"-la"}, 5)
	require.NoError(t, err)
	assert.EqualValues(t, 0, res.ExitCode)
	assert.Equal(t, "ok-out", res.Stdout)
	assert.Equal(t, "err-out", res.Stderr)

	pid, err := s.StartProcess(ctx, "agent-1", "game-server", nil, nil, "")
	require.NoError(t, err)
	assert.Equal(t, 4242, pid)

	require.NoError(t, s.StopProcess(ctx, "agent-1", "game-server"))
	require.NoError(t, s.RestartProcess(ctx, "agent-1", "game-server"))

	// 无会话的 agent → "ops client unavailable"
	_, err = s.ExecCommand(ctx, "ghost", "ls", nil, 5)
	require.ErrorContains(t, err, "ops client unavailable")
	_, err = s.StartProcess(ctx, "ghost", "x", nil, nil, "")
	require.ErrorContains(t, err, "ops client unavailable")
	err = s.StopProcess(ctx, "ghost", "x")
	require.ErrorContains(t, err, "ops client unavailable")
	err = s.RestartProcess(ctx, "ghost", "x")
	require.ErrorContains(t, err, "ops client unavailable")
}

// ---- AgentService.List：集群归属表视图 + 本地回退 + 过滤 ----

func TestAgentServiceListClusterAndLocalViews(t *testing.T) {
	store := registry.NewStore()
	require.NoError(t, store.UpsertAgent(&registry.AgentSession{
		AgentID: "agent-local", GameID: "demo", Env: "prod", Addr: "1.2.3.4:19090",
		Version: "1.2.3", Labels: map[string]string{"os": "linux"},
		Functions: map[string]registry.FunctionMeta{"f1": {}, "f2": {}},
	}))

	// 无 store → 空列表
	empty := NewAgentService(&svc.ServiceContext{})
	list, err := empty.List(context.Background(), "", "", "")
	require.NoError(t, err)
	assert.Empty(t, list)

	// 集群模式：归属表含本地 agent + 远端 agent
	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		Cluster: &svc.ClusterRuntime{
			ListAgentOwners: func(ctx context.Context) ([]cluster.AgentOwnerRecord, error) {
				return []cluster.AgentOwnerRecord{
					{AgentID: "agent-local", InstanceID: "inst-self", GameID: "demo", Env: "prod"},
					{AgentID: "agent-remote", InstanceID: "inst-peer", GameID: "other", Env: "dev"},
				}, nil
			},
		},
	}
	s := NewAgentService(svcCtx)
	list, err = s.List(context.Background(), "", "", "")
	require.NoError(t, err)
	require.Len(t, list, 2)
	// 本地条目：补全详情 + Connected
	var localItem, remoteItem *OpsAgentInfo
	for i := range list {
		if list[i].AgentID == "agent-local" {
			localItem = &list[i]
		} else {
			remoteItem = &list[i]
		}
	}
	require.NotNil(t, localItem)
	assert.True(t, localItem.Connected)
	assert.Equal(t, "1.2.3.4:19090", localItem.Addr)
	assert.Len(t, localItem.Functions, 2)
	require.NotNil(t, remoteItem)
	assert.False(t, remoteItem.Connected)
	assert.Equal(t, "inst-peer", remoteItem.OwnerInstance)

	// game/env 过滤
	list, err = s.List(context.Background(), "demo", "prod", "")
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "agent-local", list[0].AgentID)

	// 归属表读取失败 → 回退本地视图
	svcCtx.Cluster.ListAgentOwners = func(ctx context.Context) ([]cluster.AgentOwnerRecord, error) {
		return nil, assert.AnError
	}
	list, err = s.List(context.Background(), "", "", "")
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "agent-local", list[0].AgentID)
	assert.True(t, list[0].Connected)

	// 非集群：本地视图 + 过滤（env 不匹配）
	plain := NewAgentService(&svc.ServiceContext{RegistryStore: store})
	list, err = plain.List(context.Background(), "", "dev", "")
	require.NoError(t, err)
	assert.Empty(t, list)
	list, err = plain.List(context.Background(), "demo", "", "")
	require.NoError(t, err)
	require.Len(t, list, 1)
}

// ---- runtimeNodeListItem：providers/metrics/过期时间分支 ----

func TestRuntimeNodeListItemBranches(t *testing.T) {
	sess := &registry.AgentSession{
		AgentID: "n1", Addr: "a:1", GameID: "g", Env: "prod",
		Labels:    map[string]string{"hostname": "host-1"},
		Functions: map[string]registry.FunctionMeta{"f": {}},
		Providers: []registry.ProviderSession{{SDKLanguage: "go", SDKVersion: "1.1", SDKName: "croupier-go-sdk"}},
		ExpireAt:  time.Now().Add(-time.Hour), // 已过期 → 0
		LastSeen:  time.Now(),
	}

	item := runtimeNodeListItem(sess, "online", nil)
	node := item.node
	assert.Equal(t, "n1", node.Id)
	assert.Equal(t, "go", node.SDKLanguage)
	assert.Equal(t, int64(0), node.ExpiresInSec)
	assert.Nil(t, node.CPU) // metricsStore nil

	// 有 metrics：CPU/Memory 填充
	ms := registry.NewMetricsStore()
	report := &opsv1.MetricsReport{
		Cpu:    &opsv1.CpuMetrics{UsagePercent: 55.5, Cores: 8, PerCore: []float64{1, 2}, Load_1M: 0.5, Load_5M: 0.4, Load_15M: 0.3},
		Memory: &opsv1.MemoryMetrics{TotalBytes: 100, UsedBytes: 50},
	}
	ms.Add("n1", report)
	item = runtimeNodeListItem(sess, "online", ms)
	node = item.node
	require.NotNil(t, node.CPU)
	assert.Equal(t, float64(55.5), node.CPU.UsagePercent)
	require.NotNil(t, node.Memory)
	assert.EqualValues(t, 100, node.Memory.TotalBytes)
}

// ---- opsBackupDelete / opsBackupDownload：真实模型 ----

func newBackupTestCtx(t *testing.T) *svc.ServiceContext {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Backup{}))
	return &svc.ServiceContext{BackupModel: model.NewBackupModel(db)}
}

func TestOpsBackupDeleteAndDownloadWithModel(t *testing.T) {
	ctx := context.Background()
	svcCtx := newBackupTestCtx(t)

	created, err := opsBackupCreate(ctx, svcCtx, &OpsBackupCreateRequest{Name: "b1"})
	require.NoError(t, err)

	// 删除存在 → true
	resp, err := opsBackupDelete(ctx, svcCtx, &OpsBackupDeleteRequest{ID: created.BackupID})
	require.NoError(t, err)
	assert.True(t, resp.Deleted)

	// 再删 → 错误（找不到）
	_, err = opsBackupDelete(ctx, svcCtx, &OpsBackupDeleteRequest{ID: created.BackupID})
	require.Error(t, err)

	// 下载：Location 为空 → 回退占位路由
	created2, err := opsBackupCreate(ctx, svcCtx, &OpsBackupCreateRequest{Name: "b2"})
	require.NoError(t, err)
	dl, err := opsBackupDownload(ctx, svcCtx, &OpsBackupDownloadRequest{ID: created2.BackupID})
	require.NoError(t, err)
	assert.Equal(t, "/backups/"+created2.BackupID+"/download", dl.Url)

	// 下载不存在 → 错误
	_, err = opsBackupDownload(ctx, svcCtx, &OpsBackupDownloadRequest{ID: "nope"})
	require.Error(t, err)

	// 无模型 → 静默成功/占位 URL
	empty := &svc.ServiceContext{}
	resp, err = opsBackupDelete(ctx, empty, &OpsBackupDeleteRequest{ID: "x"})
	require.NoError(t, err)
	assert.True(t, resp.Deleted)
	dl, err = opsBackupDownload(ctx, empty, &OpsBackupDownloadRequest{ID: "x"})
	require.NoError(t, err)
	assert.Equal(t, "/backups/x/download", dl.Url)
}

// ---- opsHealthRun：全链路 ----

func TestOpsHealthRunFullPaths(t *testing.T) {
	ctx := context.Background()
	store := svc.NewOpsStateStore(t.TempDir())
	svcCtx := &svc.ServiceContext{OpsStateStore: store}

	// 先注册检查项
	_, err := opsHealthUpdate(ctx, svcCtx, &OpsHealthUpdateRequest{Checks: []OpsHealthCheck{{
		ID: "hc-tcp", Kind: "tcp", Target: "127.0.0.1:80", IntervalSec: 30, TimeoutMs: 1000,
	}}})
	require.NoError(t, err)

	resp, err := opsHealthRun(ctx, svcCtx, &OpsHealthRunRequest{ID: "hc-tcp"})
	require.NoError(t, err)
	assert.Equal(t, "hc-tcp", resp.Id)
	assert.True(t, resp.Ok)

	// 重复执行：旧状态被替换（不是追加）
	_, err = opsHealthRun(ctx, svcCtx, &OpsHealthRunRequest{ID: "hc-tcp"})
	require.NoError(t, err)
	snap := store.Snapshot()
	assert.Len(t, snap.Health.Status, 1)

	// 未知检查项 → 404
	_, err = opsHealthRun(ctx, svcCtx, &OpsHealthRunRequest{ID: "ghost"})
	require.Error(t, err)

	// 无 state store → 错误
	_, err = opsHealthRun(ctx, &svc.ServiceContext{}, &OpsHealthRunRequest{ID: "x"})
	require.Error(t, err)
	_, err = opsHealthRun(ctx, nil, &OpsHealthRunRequest{ID: "x"})
	require.Error(t, err)
}

// ---- opsMQ：Redis/Kafka/Lengths/Groups 渲染 ----

func TestOpsMQRendersState(t *testing.T) {
	ctx := context.Background()
	store := svc.NewOpsStateStore(t.TempDir())
	_, err := store.Update(func(state *svc.OpsState) {
		state.MQ.Type = "redis"
		state.MQ.Redis = &svc.OpsRedisMQ{URL: "redis://x:6379", Streams: map[string]string{"s1": "group"}}
		state.MQ.Kafka = &svc.OpsKafkaMQ{Brokers: "b1:9092", Topics: map[string]string{"t1": "app"}}
		state.MQ.Lengths = map[string]int{"s1": 10}
		state.MQ.Groups = []svc.OpsMQGroup{{Stream: "s1", Name: "g1", Consumers: 2, Pending: 3, Lag: 4}}
	})
	require.NoError(t, err)

	resp, err := opsMQ(ctx, &svc.ServiceContext{OpsStateStore: store}, &OpsMQRequest{})
	require.NoError(t, err)
	result := resp.Result.(map[string]interface{})
	assert.Equal(t, "redis", result["type"])
	require.Contains(t, result, "redis")
	require.Contains(t, result, "kafka")
	require.Contains(t, result, "lengths")
	require.Contains(t, result, "groups")
	groups, _ := result["groups"].([]map[string]interface{})
	require.Len(t, groups, 1)
	assert.Equal(t, "g1", groups[0]["name"])

	// 空 ctx → 空 result
	empty, err := opsMQ(ctx, &svc.ServiceContext{}, &OpsMQRequest{})
	require.NoError(t, err)
	assert.Empty(t, empty.Result)
}

// ---- opsConfig：state store 回退 + env 回退 ----

func TestOpsConfigFallbacks(t *testing.T) {
	ctx := context.Background()

	// state store 值兜底
	store := svc.NewOpsStateStore(t.TempDir())
	_, err := store.Update(func(state *svc.OpsState) {
		state.Config.AlertmanagerURL = "http://am.internal"
		state.Config.GrafanaExploreURL = "http://grafana.internal"
		state.Config.JaegerURL = "http://jaeger.internal"
	})
	require.NoError(t, err)
	resp, err := opsConfig(ctx, &svc.ServiceContext{OpsStateStore: store}, &OpsConfigRequest{})
	require.NoError(t, err)
	// settings 层可能已初始化并返回非空——只断言最终三个字段非空（来自 state 或更上层）
	if resp.AlertmanagerURL == "" || resp.GrafanaExploreURL == "" || resp.JaegerURL == "" {
		t.Fatalf("config should fall back to state store: %+v", resp)
	}

	// 完全空 → env 变量
	t.Setenv("CROUPIER_ALERTMANAGER_URL", "http://am-env")
	t.Setenv("CROUPIER_GRAFANA_EXPLORE_URL", "http://g-env")
	t.Setenv("CROUPIER_JAEGER_URL", "http://j-env")
	resp, err = opsConfig(ctx, &svc.ServiceContext{}, &OpsConfigRequest{})
	require.NoError(t, err)
	assert.Equal(t, "http://am-env", resp.AlertmanagerURL)
	assert.Equal(t, "http://g-env", resp.GrafanaExploreURL)
	assert.Equal(t, "http://j-env", resp.JaegerURL)
}

// ---- agentMetricsHistory：since 解析/limit 默认/cpu-mem-disk 渲染 ----

func TestAgentMetricsHistoryBranches(t *testing.T) {
	ctx := context.Background()

	// 无 store → 空 entries 带 AgentID
	resp, err := agentMetricsHistory(ctx, &svc.ServiceContext{}, &AgentMetricsHistoryRequest{AgentID: "a1"})
	require.NoError(t, err)
	assert.Equal(t, "a1", resp.AgentID)
	assert.Empty(t, resp.Entries)

	ms := registry.NewMetricsStore()
	ms.Add("a1", &opsv1.MetricsReport{
		Cpu:    &opsv1.CpuMetrics{UsagePercent: 10},
		Memory: &opsv1.MemoryMetrics{TotalBytes: 1},
		Disks:  []*opsv1.DiskMetrics{{MountPoint: "/", UsedBytes: 1, TotalBytes: 2}},
	})
	svcCtx := &svc.ServiceContext{MetricsStore: ms}

	// since/limit 默认
	resp, err = agentMetricsHistory(ctx, svcCtx, &AgentMetricsHistoryRequest{AgentID: "a1"})
	require.NoError(t, err)
	require.Len(t, resp.Entries, 1)
	require.NotNil(t, resp.Entries[0].CPU)
	require.NotNil(t, resp.Entries[0].Memory)
	require.Len(t, resp.Entries[0].Disks, 1)

	// since 在未来 → 无条目
	future := time.Now().Add(time.Hour).Format(time.RFC3339)
	resp, err = agentMetricsHistory(ctx, svcCtx, &AgentMetricsHistoryRequest{AgentID: "a1", Since: future})
	require.NoError(t, err)
	assert.Empty(t, resp.Entries)

	// 非法 since → 忽略回退默认
	resp, err = agentMetricsHistory(ctx, svcCtx, &AgentMetricsHistoryRequest{AgentID: "a1", Since: "not-a-time"})
	require.NoError(t, err)
	assert.Len(t, resp.Entries, 1)
}

// ---- GetClusterInfo：成员表 + mesh 离线 + OwnerStats ----

type fakeMembership struct {
	peers []cluster.PeerInfo
	err   error
}

func (m *fakeMembership) Register(ctx context.Context, info cluster.PeerInfo) (uint64, error) {
	return 1, nil
}
func (m *fakeMembership) Renew(ctx context.Context, instanceID string) error { return nil }
func (m *fakeMembership) ListAlive(ctx context.Context) ([]cluster.PeerInfo, error) {
	return m.peers, m.err
}
func (m *fakeMembership) Resign(ctx context.Context, instanceID string) error { return nil }

func TestGetClusterInfoMembersMeshOwnerStats(t *testing.T) {
	svcCtx := &svc.ServiceContext{
		Cluster: &svc.ClusterRuntime{
			InstanceID: "inst-1",
			Membership: &fakeMembership{peers: []cluster.PeerInfo{
				{InstanceID: "inst-1", AdvertiseAddr: "10.0.0.1:1", Epoch: 1, StartedAt: time.Now()},
				{InstanceID: "inst-2", AdvertiseAddr: "10.0.0.2:1", Epoch: 2, StartedAt: time.Now()},
			}},
			OwnerStats: func(ctx context.Context) map[string]int64 {
				return map[string]int64{"inst-1": 3, "inst-2": 5}
			},
		},
	}
	s := NewService(svcCtx)
	resp, err := s.GetClusterInfo(context.Background())
	require.NoError(t, err)
	require.Len(t, resp.Items, 2)
	assert.Equal(t, int64(3), resp.Items[0].AgentCount)
	assert.Equal(t, int64(5), resp.Items[1].AgentCount)
	assert.True(t, resp.Items[0].Self)
	assert.False(t, resp.Items[1].Self)

	// Membership 报错 → 不中断（空成员表）
	svcCtx.Cluster.Membership = &fakeMembership{err: assert.AnError}
	resp, err = s.GetClusterInfo(context.Background())
	require.NoError(t, err)
	assert.Empty(t, resp.Items)
}
