package main

import (
	"context"
	"encoding/json"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/cluster"
	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/dispatch"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/server"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/cuihairu/croupier/internal/transport/tcp"
	"github.com/cuihairu/croupier/pkg/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gsqlite "gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// newClusterSvcCtx 建一个可承载协调面表的 sqlite 服务上下文。
func newClusterSvcCtx(t *testing.T) *svc.ServiceContext {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open(filepath.Join(t.TempDir(), "cluster.db")), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))
	require.NoError(t, reg.MigrateAgentSessions(db))
	return &svc.ServiceContext{DB: db, RegistryStore: reg.NewStoreWithDB(db)}
}

func stopCluster(t *testing.T, lc *cluster.Lifecycle, srv *tcp.Server) {
	t.Helper()
	if lc != nil {
		stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		lc.Stop(stopCtx)
	}
	if srv != nil {
		_ = srv.Close()
	}
}

func TestStartCluster_DisabledAndGuards(t *testing.T) {
	ctx := context.Background()
	c := &config.Config{}

	// 未启用 → (nil, nil) 单实例
	lc, srv := startCluster(ctx, c, nil)
	assert.Nil(t, lc)
	assert.Nil(t, srv)

	// 启用但 svcCtx 缺失 → standalone
	c.Cluster.Enabled = true
	lc, srv = startCluster(ctx, c, nil)
	assert.Nil(t, lc)
	assert.Nil(t, srv)

	// db 存储但 DB 缺失 → standalone
	c.Cluster.Store = "db"
	lc, srv = startCluster(ctx, c, &svc.ServiceContext{})
	assert.Nil(t, lc)
	assert.Nil(t, srv)

	// redis 存储但无可用地址 → standalone
	c.Cluster.Store = "redis"
	lc, srv = startCluster(ctx, c, &svc.ServiceContext{})
	assert.Nil(t, lc)
	assert.Nil(t, srv)
}

// redis 存储全流程：miniredis 作为协调面，互联监听落在临时端口。
func TestStartCluster_RedisStore(t *testing.T) {
	mr := miniredis.RunT(t)
	svcCtx := newClusterSvcCtx(t)
	c := &config.Config{}
	c.Cluster = config.ClusterConfig{
		Enabled:       true,
		InstanceID:    "server-test",
		Store:         "redis",
		RedisAddr:     mr.Addr(),
		AdvertiseAddr: "127.0.0.1:0",
		OwnerTTL:      "1m",
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	lc, srv := startCluster(ctx, c, svcCtx)
	require.NotNil(t, lc)
	require.NotNil(t, srv)
	require.NotNil(t, svcCtx.Cluster)
	assert.Equal(t, "server-test", svcCtx.Cluster.InstanceID)
	assert.NotNil(t, svcCtx.Cluster.Mesh)
	assert.NotNil(t, svcCtx.Cluster.Resolver)

	// 装配的闭包可调用（空表 → 空/零值不报错）
	counts := svcCtx.Cluster.OwnerStats(context.Background())
	assert.Empty(t, counts)
	owners, err := svcCtx.Cluster.ListAgentOwners(context.Background())
	require.NoError(t, err)
	assert.Empty(t, owners)

	stopCluster(t, lc, srv)
}

// db 存储全流程 + 互联监听端口被占 → 只返回 lifecycle 不炸。
func TestStartCluster_DBStore_InterconnectListenFailure(t *testing.T) {
	svcCtx := newClusterSvcCtx(t)

	// 预占端口制造互联 listen 失败
	busy, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = busy.Close() })

	c := &config.Config{}
	c.Cluster = config.ClusterConfig{
		Enabled:       true,
		InstanceID:    "server-db",
		Store:         "db",
		AdvertiseAddr: busy.Addr().String(),
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	lc, srv := startCluster(ctx, c, svcCtx)
	require.NotNil(t, lc, "互联监听失败只降级，生命周期仍在")
	assert.Nil(t, srv)
	require.NotNil(t, svcCtx.Cluster)

	stopCluster(t, lc, nil)
}

// db 存储正常路径：成员/归属表自建、互联监听成功、Handle 消息路由。
func TestStartCluster_DBStore_FullWiring(t *testing.T) {
	svcCtx := newClusterSvcCtx(t)
	c := &config.Config{}
	c.Cluster = config.ClusterConfig{
		Enabled:           true,
		InstanceID:        "server-full",
		Store:             "db",
		AdvertiseAddr:     "127.0.0.1:0",
		HeartbeatInterval: "200ms",
		PeerPollInterval:  "200ms",
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	svcCtx.Dispatcher = dispatch.NewDispatcher(svcCtx.RegistryStore) // 触发 localInvoker 装配分支
	lc, srv := startCluster(ctx, c, svcCtx)
	require.NotNil(t, lc)
	require.NotNil(t, srv)

	// 互联消息路由：hello / forward（含 fencing）/ 未知消息
	fake := &fakeLocalDispatcher{}
	handler := newInterconnectHandler(lc, localInvoker{dispatcher: fake})
	hctx := context.Background()

	// hello：回 JSON（SelfInfo + epoch）
	helloBody, err := handler.Handle(hctx, protocol.MsgServerHelloRequest, 1, []byte(`{}`))
	require.NoError(t, err)
	var helloInfo map[string]any
	require.NoError(t, json.Unmarshal(helloBody, &helloInfo))
	assert.Contains(t, helloInfo, "instanceId")

	// forward：epoch 匹配 → 本地执行 → OK
	forwardReq := func(epoch uint64) []byte {
		raw, err := json.Marshal(map[string]any{
			"forwarded": true, "callerEpoch": epoch,
			"agentId": "agent-1", "functionId": "demo.fn",
		})
		require.NoError(t, err)
		return raw
	}
	fwdBody, err := handler.Handle(hctx, protocol.MsgForwardInvokeReq, 2, forwardReq(lc.Epoch()))
	require.NoError(t, err)
	var fwd cluster.ForwardedResult
	require.NoError(t, json.Unmarshal(fwdBody, &fwd))
	assert.True(t, fwd.OK)
	assert.Equal(t, "invoke-resp", string(fwd.Payload))
	require.Len(t, fake.invokeAgentIDs, 1)
	assert.Equal(t, "agent-1", fake.invokeAgentIDs[0])

	// forward：epoch 陈旧 → NotOwner（fencing 铁律二）
	fwdBody, err = handler.Handle(hctx, protocol.MsgForwardInvokeReq, 3, forwardReq(lc.Epoch()+999))
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(fwdBody, &fwd))
	assert.True(t, fwd.NotOwner)

	// forward：缺 forwarded 标记 → 拒绝
	raw, err := json.Marshal(map[string]any{"callerEpoch": lc.Epoch()})
	require.NoError(t, err)
	fwdBody, err = handler.Handle(hctx, protocol.MsgForwardInvokeReq, 4, raw)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(fwdBody, &fwd))
	assert.False(t, fwd.OK)

	// 非法 JSON → bad request
	fwdBody, err = handler.Handle(hctx, protocol.MsgForwardInvokeReq, 5, []byte(`{bad`))
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(fwdBody, &fwd))
	assert.False(t, fwd.OK)

	// 未知消息 → 错误
	_, err = handler.Handle(hctx, 0x3fffff, 6, nil)
	assert.ErrorContains(t, err, "unexpected msg")

	// invoker 缺失 → 本地执行报错经 forward 通道回 NotOK
	nilInvoker := newInterconnectHandler(lc, nil)
	fwdBody, err = nilInvoker.Handle(hctx, protocol.MsgForwardInvokeReq, 7, forwardReq(lc.Epoch()))
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(fwdBody, &fwd))
	assert.False(t, fwd.OK)

	stopCluster(t, lc, srv)
}

func TestOpenClusterRedis(t *testing.T) {
	// 显式 redisAddr 优先
	client, err := openClusterRedis(config.ClusterConfig{RedisAddr: "r-addr:6379", RedisPassword: "pw", RedisDB: 3}, nil)
	require.NoError(t, err)
	assert.Equal(t, "r-addr:6379", client.Options().Addr)
	assert.Equal(t, "pw", client.Options().Password)
	assert.Equal(t, 3, client.Options().DB)
	_ = client.Close()

	// 空地址回退 cache 段（同实例复用）
	client, err = openClusterRedis(config.ClusterConfig{}, &config.Config{Cache: config.CacheConfig{
		Type: "redis", Addr: "cache:6379", Password: "cpw", DB: 5,
	}})
	require.NoError(t, err)
	assert.Equal(t, "cache:6379", client.Options().Addr)
	assert.Equal(t, "cpw", client.Options().Password)
	assert.Equal(t, 5, client.Options().DB)
	_ = client.Close()

	// cache 显式字段不覆盖 cluster 段已给出的值
	client, err = openClusterRedis(config.ClusterConfig{RedisDB: 2}, &config.Config{Cache: config.CacheConfig{
		Type: "redis", Addr: "cache:6379", Password: "cpw", DB: 5,
	}})
	require.NoError(t, err)
	assert.Equal(t, 2, client.Options().DB)
	_ = client.Close()

	// 双空 / cache 非 redis → 报错
	_, err = openClusterRedis(config.ClusterConfig{}, nil)
	assert.Error(t, err)
	_, err = openClusterRedis(config.ClusterConfig{}, &config.Config{Cache: config.CacheConfig{Type: "local"}})
	assert.Error(t, err)
}

func TestOwnerInstance(t *testing.T) {
	owners := []cluster.AgentOwnerRecord{
		{AgentID: "a1", InstanceID: "s1"},
		{AgentID: "a2", InstanceID: "s2"},
	}
	assert.Equal(t, "s2", ownerInstance(owners, "a2"))
	assert.Equal(t, "", ownerInstance(owners, "ghost"))
	assert.Equal(t, "", ownerInstance(nil, "a1"))
}

// ---- meshForwarder ----

// meshForwarder 错误路径：mesh 无 owner 路由 → 错误统一 wrap 为可换候选重试。
func TestMeshForwarder_NoRoute(t *testing.T) {
	mesh := cluster.NewMeshInterconnect(cluster.PeerInfo{InstanceID: "self"}, &refreshOwnerFake{}, nil)
	f := newMeshForwarder(mesh)

	// 三种 MsgID（invoke/start/cancel）都走同一错误通道（Kind 映射先于转发）
	for _, msgID := range []uint32{0, protocol.MsgStartTaskRequest, protocol.MsgCancelTaskRequest} {
		_, err := f.Forward(context.Background(), &dispatch.RemoteCall{
			AgentID:    "ghost-agent",
			FunctionID: "demo.fn",
			MsgID:      msgID,
		})
		require.ErrorContains(t, err, "mesh forward:", "msgID=%d", msgID)
	}
}

// forwardInvokeRequest 的 metadata 注入（nil metadata 兜底）。
func TestForwardInvokeRequest_MetadataInjection(t *testing.T) {
	req := forwardInvokeRequest(&cluster.ForwardedInvoke{
		FunctionID: "demo.fn",
		Caller:     cluster.CallerContext{Username: "bob", GameID: "demo", Env: "prod"},
	})
	assert.Equal(t, "demo.fn", req.FunctionId)
	assert.Equal(t, "bob", req.Metadata["forwarded_by"])
	assert.Equal(t, "demo", req.Metadata["gameId"])
	assert.Equal(t, "prod", req.Metadata["env"])

	// nil metadata → 自动建 map 再注入
	req = forwardInvokeRequest(&cluster.ForwardedInvoke{Caller: cluster.CallerContext{Username: "x"}})
	assert.Equal(t, "x", req.Metadata["forwarded_by"])
	assert.NotContains(t, req.Metadata, "gameId")
}

// auditForward 的 outcome 分支补漏：res.OK=false（非 error 通道）落 failure，
// AdminID/TaskID 进 details。
func TestAuditForward_NotOKOutcome(t *testing.T) {
	store := audit.NewInMemoryAuditStore()
	li := localInvoker{dispatcher: &fakeLocalDispatcher{}, audit: audit.NewAuditService(store, nil)}

	li.auditForward(context.Background(),
		&cluster.ForwardedInvoke{
			AgentID: "a1", FunctionID: "demo.fn", TaskID: "task-1",
			Caller: cluster.CallerContext{AdminID: 5, GameID: "g", Env: "e", TraceID: "tr"},
		},
		&cluster.ForwardedResult{OK: false, Error: "agent offline"}, nil)

	recs, _, err := store.List(audit.AuditFilter{EventType: []audit.AuditEventType{audit.EventFunctionInvoke}}, audit.AuditPage{})
	require.NoError(t, err)
	require.Len(t, recs, 1)
	rec := recs[0]
	assert.Equal(t, "failure", rec.Outcome)
	assert.Equal(t, "agent offline", rec.ErrorMessage)
	assert.Equal(t, "task-1", rec.Details["task_id"])
	assert.Equal(t, uint(5), rec.Details["admin_id"])
	assert.Equal(t, "tr", rec.Details["traceId"])
}

// ---- startCluster 降级分支矩阵 ----

// redis open 成功但 ping 失败（协调面实际不可用）→ standalone。
func TestStartCluster_RedisPingFailure(t *testing.T) {
	mr := miniredis.RunT(t)
	addr := mr.Addr()
	mr.Close() // 关闭后 open 惰性成功、ping 必失败

	svcCtx := newClusterSvcCtx(t)
	c := &config.Config{}
	c.Cluster = config.ClusterConfig{
		Enabled: true, InstanceID: "server-ping", Store: "redis",
		RedisAddr: addr, AdvertiseAddr: "127.0.0.1:0",
	}
	lc, srv := startCluster(context.Background(), c, svcCtx)
	assert.Nil(t, lc)
	assert.Nil(t, srv)
}

// db 存储但成员表 DDL 失败（file: URI 只读打开已存在的库）→ standalone。
func TestStartCluster_DBEnsureTableFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ro.db")
	seed, err := gorm.Open(gsqlite.Open(path), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, seed.Exec("SELECT 1").Error)

	db, err := gorm.Open(gsqlite.Open("file:"+path+"?mode=ro"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	c := &config.Config{}
	c.Cluster = config.ClusterConfig{
		Enabled: true, InstanceID: "server-ro", Store: "db",
		AdvertiseAddr: "127.0.0.1:0",
	}
	lc, srv := startCluster(context.Background(), c, &svc.ServiceContext{DB: db})
	assert.Nil(t, lc)
	assert.Nil(t, srv)
}

// 归属表 DDL 失败（成员表已建成但 owner 表建不上）需要两个独立 DDL 命运
// 分叉，单连接上无法干净构造；该分支归入 DDL 基础设施失败防御豁免。

// AdvertiseAddr 为空 → cluster.Start 拒绝 → standalone。
func TestStartCluster_EmptyAdvertiseAddr(t *testing.T) {
	svcCtx := newClusterSvcCtx(t)
	c := &config.Config{}
	c.Cluster = config.ClusterConfig{Enabled: true, InstanceID: "server-noaddr", Store: "db"}
	lc, srv := startCluster(context.Background(), c, svcCtx)
	assert.Nil(t, lc)
	assert.Nil(t, srv)
}

// 归属 reconcile 兜底循环体：已声明的归属行被周期续期（Touch 只续期不
// 创建，创建靠 ClaimOwner）、RegistryStore 缺失时跳过、循环随 ctx 退出；
// ticker 间隔走包级注入点。
func TestStartCluster_ReconcileLoop(t *testing.T) {
	old := reconcileTickerInterval
	reconcileTickerInterval = 20 * time.Millisecond
	t.Cleanup(func() { reconcileTickerInterval = old })

	svcCtx := newClusterSvcCtx(t)
	now := time.Now()
	require.NoError(t, svcCtx.RegistryStore.UpsertAgent(&reg.AgentSession{
		AgentID: "agent-recon", GameID: "demo", Env: "prod",
		ExpireAt: now.Add(time.Hour), LastSeen: now,
	}))

	c := &config.Config{}
	c.Cluster = config.ClusterConfig{
		Enabled: true, InstanceID: "server-recon", Store: "db",
		AdvertiseAddr: "127.0.0.1:0", HeartbeatInterval: "200ms",
	}
	ctx, cancel := context.WithCancel(context.Background())
	lc, srv := startCluster(ctx, c, svcCtx)
	require.NotNil(t, lc)
	require.NotNil(t, srv)

	// 声明归属行后把 last_seen_at 拨旧，等 reconcile tick 续期回新
	require.NoError(t, svcCtx.Cluster.Resolver.ClaimOwner(
		context.Background(), "agent-recon", "demo", "prod", "server-recon", lc.Epoch()))
	stale := time.Now().Add(-2 * time.Minute)
	require.NoError(t, svcCtx.DB.Model(&cluster.AgentOwnerRecord{}).
		Where("agent_id = ?", "agent-recon").
		Update("last_seen_at", stale.UTC()).Error)

	deadline := time.Now().Add(5 * time.Second)
	for {
		var rec cluster.AgentOwnerRecord
		err := svcCtx.DB.Where("agent_id = ?", "agent-recon").First(&rec).Error
		require.NoError(t, err)
		if rec.LastSeenAt.After(stale.Add(time.Minute)) {
			break // 已被 reconcile 续期
		}
		if time.Now().After(deadline) {
			t.Fatal("reconcile loop did not touch owner row")
		}
		time.Sleep(20 * time.Millisecond)
	}

	// tick 顺带驱动 refreshRemoteSnapshots（owners 全是本实例 → 无远端回灌）
	svcCtx.Cluster.OwnerStats(context.Background())

	cancel()
	stopCluster(t, lc, srv)

	// RegistryStore 缺失 → 循环内 continue 分支（不续期、不炸）
	svcCtx2 := newClusterSvcCtx(t)
	svcCtx2.RegistryStore = nil
	c2 := &config.Config{}
	c2.Cluster = config.ClusterConfig{
		Enabled: true, InstanceID: "server-recon2", Store: "db",
		AdvertiseAddr: "127.0.0.1:0",
	}
	ctx2, cancel2 := context.WithCancel(context.Background())
	lc2, srv2 := startCluster(ctx2, c2, svcCtx2)
	require.NotNil(t, lc2)
	time.Sleep(60 * time.Millisecond) // 至少一个 tick 走 RegistryStore nil 分支
	cancel2()
	stopCluster(t, lc2, srv2)
}

// OwnerStats 闭包在 resolver 故障时回空 map（LB 监控不因协调面抖动报 500）。
func TestStartCluster_OwnerStatsError(t *testing.T) {
	mr := miniredis.RunT(t)
	svcCtx := newClusterSvcCtx(t)
	c := &config.Config{}
	c.Cluster = config.ClusterConfig{
		Enabled: true, InstanceID: "server-stats", Store: "redis",
		RedisAddr: mr.Addr(), AdvertiseAddr: "127.0.0.1:0",
	}
	ctx, cancel := context.WithCancel(context.Background())
	lc, srv := startCluster(ctx, c, svcCtx)
	require.NotNil(t, lc)

	cancel() // ctx 取消 → rdb 关闭 → CountAgentsByOwner 报错 → 空 map
	stopCluster(t, lc, srv)
	assert.Empty(t, svcCtx.Cluster.OwnerStats(context.Background()))
}

// ---- localInvoker 分发错误与审计留痕 ----

// 三种 Kind 的 dispatcher 错误统一收敛为 OK=false（错误文本进 res.Error，
// 外层 err 通道仅在 dispatcher 缺失时非 nil）。
func TestLocalInvoker_DispatchErrors(t *testing.T) {
	store := audit.NewInMemoryAuditStore()
	fake := &fakeLocalDispatcher{invokeErr: context.DeadlineExceeded}
	li := localInvoker{dispatcher: fake, audit: audit.NewAuditService(store, nil)}
	ctx := context.Background()

	res, err := li.InvokeLocal(ctx, &cluster.ForwardedInvoke{AgentID: "a1", FunctionID: "f"})
	require.NoError(t, err)
	assert.False(t, res.OK)
	assert.Contains(t, res.Error, "context deadline exceeded")

	fake.startErr = context.DeadlineExceeded
	res, err = li.InvokeLocal(ctx, &cluster.ForwardedInvoke{AgentID: "a1", Kind: cluster.ForwardKindStartTask})
	require.NoError(t, err)
	assert.False(t, res.OK)
	assert.Contains(t, res.Error, "context deadline exceeded")

	fake.cancelErr = context.DeadlineExceeded
	res, err = li.InvokeLocal(ctx, &cluster.ForwardedInvoke{AgentID: "a1", Kind: cluster.ForwardKindCancel, TaskID: "t1"})
	require.NoError(t, err)
	assert.False(t, res.OK)
	assert.Contains(t, res.Error, "context deadline exceeded")

	// dispatcher 为 nil → 明确报错（唯一的 err 通道出口）
	nilLi := localInvoker{}
	_, err = nilLi.InvokeLocal(ctx, &cluster.ForwardedInvoke{})
	assert.EqualError(t, err, "dispatcher unavailable")
}

// auditForward 的 err 通道分支：dispatch 返回错误（非 res.OK=false）时
// outcome=failure 且 ErrorMessage 取 err 文本。
func TestAuditForward_DispatchErrorOutcome(t *testing.T) {
	store := audit.NewInMemoryAuditStore()
	li := localInvoker{dispatcher: &fakeLocalDispatcher{}, audit: audit.NewAuditService(store, nil)}

	li.auditForward(context.Background(),
		&cluster.ForwardedInvoke{AgentID: "a1", FunctionID: "demo.fn"},
		nil, context.DeadlineExceeded)

	recs, _, err := store.List(audit.AuditFilter{EventType: []audit.AuditEventType{audit.EventFunctionInvoke}}, audit.AuditPage{})
	require.NoError(t, err)
	require.Len(t, recs, 1)
	assert.Equal(t, "failure", recs[0].Outcome)
	assert.Equal(t, "context deadline exceeded", recs[0].ErrorMessage)
}

// failingAuditStore 除 Create 外全部委托内存实现，Create 恒报错（owner 侧
// 审计写失败只 warn 不影响转发主路径）。
type failingAuditStore struct {
	audit.AuditStore
}

func (f *failingAuditStore) Create(*audit.AuditRecord) error { return context.DeadlineExceeded }

func TestAuditForward_AuditLogError(t *testing.T) {
	li := localInvoker{
		dispatcher: &fakeLocalDispatcher{},
		audit:      audit.NewAuditService(&failingAuditStore{AuditStore: audit.NewInMemoryAuditStore()}, nil),
	}
	// 不 panic、不改变主路径返回值即视为留痕分支生效
	res, err := li.InvokeLocal(context.Background(), &cluster.ForwardedInvoke{AgentID: "a1", FunctionID: "f"})
	require.NoError(t, err)
	assert.True(t, res.OK)
}

// ---- refreshRemoteSnapshots 错误面 ----

// 归属表不可达 / DB 会话加载失败 / 非远端成员跳过 / store 缺失。
func TestRefreshRemoteSnapshots_Guards(t *testing.T) {
	ctx := context.Background()

	// ListAliveOwners 失败 → 既不回灌也不对账
	broken := &refreshOwnerFake{listErr: context.DeadlineExceeded}
	svcCtx := &svc.ServiceContext{RegistryStore: reg.NewStore()}
	refreshRemoteSnapshots(ctx, svcCtx, broken, "self")

	// pruneOrphanSnapshots：store 缺失 → 早退
	pruneOrphanSnapshots(&svc.ServiceContext{}, nil)

	// LoadActiveSessions 失败（DB 无 agent_sessions 表）→ 静默返回
	dir := t.TempDir()
	metaDB, err := gorm.Open(gsqlite.Open(filepath.Join(dir, "m.db")), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	svcCtx = &svc.ServiceContext{DB: metaDB, RegistryStore: reg.NewStore()}
	refreshRemoteSnapshots(ctx, svcCtx, &refreshOwnerFake{recs: []cluster.AgentOwnerRecord{
		{AgentID: "a1", InstanceID: "peer"},
	}}, "self")

	// 会话集合含非远端成员（!need → continue）与远端命中（无本地快照 → 回灌）
	fullDB, err := gorm.Open(gsqlite.Open(filepath.Join(dir, "full.db")), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, reg.MigrateAgentSessions(fullDB))
	now := time.Now()
	m := reg.NewAgentSessionModel(fullDB)
	for _, id := range []string{"remote-1", "local-1"} {
		require.NoError(t, m.Upsert(ctx, &reg.AgentSession{
			AgentID: id, GameID: "demo", Env: "prod",
			ExpireAt: now.Add(time.Hour), LastSeen: now,
		}))
	}
	svcCtx.DB = fullDB
	refreshRemoteSnapshots(ctx, svcCtx, &refreshOwnerFake{recs: []cluster.AgentOwnerRecord{
		{AgentID: "remote-1", InstanceID: "peer"},
	}}, "self")

	svcCtx.RegistryStore.Mu().RLock()
	_, hasRemote := svcCtx.RegistryStore.AgentsUnsafe()["remote-1"]
	_, hasLocal := svcCtx.RegistryStore.AgentsUnsafe()["local-1"]
	svcCtx.RegistryStore.Mu().RUnlock()
	assert.True(t, hasRemote, "远端持有且 DB 有会话 → 回灌本地快照")
	assert.False(t, hasLocal, "非远端成员不回灌")
}

// wireClusterHooks 的 TCP listener 注入分支（带真实监听的 controlRuntime）。
func TestWireClusterHooks_WithTCPListener(t *testing.T) {
	svcCtx, _ := newControlServerSvcCtx(t)
	svcCtx.Cluster = &svc.ClusterRuntime{OwnerHooks: cluster.NewOwnerHooks(&refreshOwnerFake{}, "self", 1)}

	c := &config.Config{}
	c.Control.Addr = "127.0.0.1:0"
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	rt := startControlServer(ctx, c, svcCtx, server.NewAgentSessionStore())
	require.NotNil(t, rt.tcpListener)
	wireClusterHooks(svcCtx, rt)
	require.NoError(t, rt.tcpListener.Close())
	rt.controlService.Stop()
}
