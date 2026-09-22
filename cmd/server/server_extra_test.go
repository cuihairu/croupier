package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/cluster"
	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/db/dbctx"
	"github.com/cuihairu/croupier/internal/db/router"
	"github.com/cuihairu/croupier/internal/model"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/server"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gsqlite "gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// ---- taskRunAgentLookup ----

func newTaskLookupDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open(filepath.Join(t.TempDir(), "task.db")), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))
	return db
}

func TestTaskRunAgentLookup(t *testing.T) {
	ctx := context.Background()
	db := newTaskLookupDB(t)
	m := model.NewTaskRunModel(db)

	// nil 接收者与 nil model → 防御错误
	var nilLookup *taskRunAgentLookup
	_, err := nilLookup.AgentForTask(ctx, "t1")
	assert.EqualError(t, err, "task run model unavailable")
	_, err = (&taskRunAgentLookup{}).AgentForTask(ctx, "t1")
	assert.EqualError(t, err, "task run model unavailable")

	lookup := &taskRunAgentLookup{m: m}

	// 无行 → FindByTaskID 的 record not found 透传
	_, err = lookup.AgentForTask(ctx, "missing")
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)

	// 行存在但 AgentID 空（dispatch 未写 agent）→ has no agent
	require.NoError(t, m.Create(ctx, &model.TaskRun{TaskID: "t1", AgentID: ""}))
	_, err = lookup.AgentForTask(ctx, "t1")
	assert.ErrorContains(t, err, "has no agent")

	// 正常：dispatch 发起实例写入的 AgentID 可被对端解析
	require.NoError(t, m.Create(ctx, &model.TaskRun{TaskID: "t2", AgentID: "agent-7"}))
	agentID, err := lookup.AgentForTask(ctx, "t2")
	require.NoError(t, err)
	assert.Equal(t, "agent-7", agentID)
}

// ---- activeAgentIDDirectory + ownerAgentSource 守卫 ----

func TestActiveAgentIDDirectory(t *testing.T) {
	ctx := context.Background()

	// nil resolver → 空集
	ids, err := activeAgentIDDirectory{}.ActiveAgentIDs(ctx)
	require.NoError(t, err)
	assert.Nil(t, ids)

	// 同 agent 多实例持有（跨实例重连竞态）→ 去重
	d := activeAgentIDDirectory{resolver: &refreshOwnerFake{recs: []cluster.AgentOwnerRecord{
		{AgentID: "a1", InstanceID: "s1"},
		{AgentID: "a1", InstanceID: "s2"},
		{AgentID: "a2", InstanceID: "s2"},
	}}}
	ids, err = d.ActiveAgentIDs(ctx)
	require.NoError(t, err)
	assert.Equal(t, []string{"a1", "a2"}, ids)
}

func TestOwnerAgentSource_Guards(t *testing.T) {
	ctx := context.Background()
	src, db := newRemoteSourceFixture(t)

	// owners/db 守卫 → (nil, nil)
	sessions, err := src.RemoteAgentSessions(ctx, "demo", "prod", false)
	require.NoError(t, err)
	assert.Nil(t, sessions)
	src.owners = ownersFunc()
	sessions, err = src.RemoteAgentSessions(ctx, "demo", "prod", false)
	require.NoError(t, err)
	assert.Nil(t, sessions)
	src.db = db
	sessions, err = src.RemoteAgentSessions(ctx, "demo", "prod", false)
	require.NoError(t, err)
	assert.Nil(t, sessions)

	// 归属表故障透传
	src.owners = func(context.Context) ([]cluster.AgentOwnerRecord, error) {
		return nil, context.DeadlineExceeded
	}
	_, err = src.RemoteAgentSessions(ctx, "demo", "prod", false)
	assert.ErrorIs(t, err, context.DeadlineExceeded)

	// var _ 断言的接口实现面
	var _ interface {
		RemoteAgentSessions(context.Context, string, string, bool) ([]*reg.AgentSession, error)
	} = src
}

// ---- registrationContractPipeline scopedDB / 守卫 / 委托 ----

func newSingleDBPipeline(t *testing.T) (*registrationContractPipeline, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open(filepath.Join(t.TempDir(), "pipe.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))
	require.NoError(t, reg.MigrateAgentSessions(db))
	svcCtx := &svc.ServiceContext{DB: db, RegistryStore: reg.NewStoreWithDB(db)}
	wireDashboardRegistrationPipeline(svcCtx)
	return &registrationContractPipeline{svcCtx: svcCtx}, db
}

func TestRegistrationPipeline_ScopedDBBranches(t *testing.T) {
	ctx := context.Background()
	p, db := newSingleDBPipeline(t)

	// 未初始化守卫
	var nilP *registrationContractPipeline
	_, err := nilP.scopedDB(ctx, "g", "e")
	assert.EqualError(t, err, "dashboard registration pipeline is not initialized")
	_, err = (&registrationContractPipeline{}).scopedDB(ctx, "g", "e")
	assert.EqualError(t, err, "dashboard registration pipeline is not initialized")
	_, err = (&registrationContractPipeline{svcCtx: &svc.ServiceContext{}}).scopedDB(ctx, "g", "e")
	assert.EqualError(t, err, "dashboard registration pipeline is not initialized")

	// ctx 内已有 scoped 事务 → 原样返回
	scoped := svc.WithGameScope(ctx, svc.GameScope{GameID: "g", Env: "e"})
	got, err := p.scopedDB(dbctx.WithDB(scoped, db), "g", "e")
	require.NoError(t, err)
	assert.Same(t, db, got)

	// Router nil（单库）→ meta DB 直达
	got, err = p.scopedDB(ctx, "g", "e")
	require.NoError(t, err)
	assert.Same(t, db, got)
}

// 分库模式下 scopedDB 的三个错误分支：空 scope / 绑定缺失 / 逐委托透传。
func TestRegistrationPipeline_ScopedDBGameRouterErrors(t *testing.T) {
	dir := t.TempDir()
	metaDB, err := gorm.Open(gsqlite.Open(filepath.Join(dir, "meta.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, metaDB.AutoMigrate(&model.Game{}, &model.GameEnvBinding{}))

	rt := router.New(router.Config{
		Driver:         "sqlite",
		MetaDSN:        filepath.Join(dir, "meta.db"),
		NameForGame:    func(gameID, env string) string { return "game_" + gameID + "_" + env },
		DSNForDatabase: func(_, dbName string) string { return filepath.Join(dir, dbName+".db") },
		EnsureDatabase: func(_, _ string, dbName string) (string, error) { return filepath.Join(dir, dbName+".db"), nil },
		Open: func(driver, dsn string) (*gorm.DB, error) {
			return gorm.Open(gsqlite.Open(dsn), &gorm.Config{})
		},
	}, metaDB)

	svcCtx := &svc.ServiceContext{
		DB:        metaDB,
		Router:    rt,
		GameModel: model.NewGameModel(metaDB),
	}
	p := &registrationContractPipeline{svcCtx: svcCtx}
	ctx := context.Background()

	// 分库但 scope 为空 → 明确报错
	_, err = p.scopedDB(ctx, "  ", "e")
	assert.ErrorContains(t, err, "game_id and env are required")
	_, err = p.scopedDB(ctx, "g", " ")
	assert.ErrorContains(t, err, "game_id and env are required")

	// 绑定缺失 → game scope not found
	_, err = p.scopedDB(ctx, "ghost", "prod")
	assert.ErrorContains(t, err, "game scope not found")

	// 绑定存在 → 解析到对应 game 库
	require.NoError(t, metaDB.Create(&model.GameEnvBinding{
		GameID: "demo", Env: "prod", DatabaseName: "game_demo_prod",
	}).Error)
	got, err := p.scopedDB(ctx, "demo", "prod")
	require.NoError(t, err)
	require.NotNil(t, got)

	// 六个委托方法在 scopedDB 失败时统一透传错误（空 scope 触发）
	empty := " "
	_, err = p.RemoveFunctionContract(ctx, empty, "e", "fn")
	assert.ErrorContains(t, err, "game_id and env are required")
	err = p.RebuildContractFromFunctionMeta(ctx, empty, "e", "test", spec.FunctionContractInput{})
	assert.ErrorContains(t, err, "game_id and env are required")
	err = p.MarkContractRemovalPending(ctx, empty, "e", "fn")
	assert.ErrorContains(t, err, "game_id and env are required")
	err = p.RebuildResourceCapability(ctx, empty, "e", "players")
	assert.ErrorContains(t, err, "game_id and env are required")
	err = p.RebuildProposalsForResource(ctx, empty, "e", "players")
	assert.ErrorContains(t, err, "game_id and env are required")
	err = p.RebuildProposalForFunction(ctx, empty, "e", "fn")
	assert.ErrorContains(t, err, "game_id and env are required")

	// 未初始化管道走委托同样透传
	emptyP := &registrationContractPipeline{}
	_, err = emptyP.RemoveFunctionContract(ctx, "g", "e", "fn")
	assert.Error(t, err)
}

// FinalizeExpiredContractRemovals 的防御分支矩阵。
func TestRegistrationPipeline_FinalizeGuards(t *testing.T) {
	ctx := context.Background()

	// nil 接收者 / nil svcCtx / nil DB → (0, nil) 静默跳过
	var nilP *registrationContractPipeline
	n, err := nilP.FinalizeExpiredContractRemovals(ctx, time.Minute)
	require.NoError(t, err)
	assert.Zero(t, n)
	n, err = (&registrationContractPipeline{}).FinalizeExpiredContractRemovals(ctx, time.Minute)
	require.NoError(t, err)
	assert.Zero(t, n)
	n, err = (&registrationContractPipeline{svcCtx: &svc.ServiceContext{}}).FinalizeExpiredContractRemovals(ctx, time.Minute)
	require.NoError(t, err)
	assert.Zero(t, n)

	// 分库但 GameModel 未装配 → 明确报错
	p, _ := newSingleDBPipeline(t)
	p.svcCtx.Router = &router.Router{}
	_, err = p.FinalizeExpiredContractRemovals(ctx, time.Minute)
	assert.ErrorContains(t, err, "game model is not initialized")

	// 绑定表缺失 → ListAllEnvBindings 报错透传
	dir := t.TempDir()
	metaDB, err := gorm.Open(gsqlite.Open(filepath.Join(dir, "meta.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, metaDB.AutoMigrate(&model.Game{})) // 不迁移 GameEnvBinding
	rt := router.New(router.Config{
		Driver:         "sqlite",
		MetaDSN:        filepath.Join(dir, "meta.db"),
		NameForGame:    func(gameID, env string) string { return "game_x" },
		DSNForDatabase: func(_, dbName string) string { return filepath.Join(dir, dbName+".db") },
		EnsureDatabase: func(_, _ string, dbName string) (string, error) { return filepath.Join(dir, dbName+".db"), nil },
		Open: func(driver, dsn string) (*gorm.DB, error) {
			return gorm.Open(gsqlite.Open(dsn), &gorm.Config{})
		},
	}, metaDB)
	p2 := &registrationContractPipeline{svcCtx: &svc.ServiceContext{
		DB: metaDB, Router: rt, GameModel: model.NewGameModel(metaDB),
	}}
	_, err = p2.FinalizeExpiredContractRemovals(ctx, time.Minute)
	assert.ErrorContains(t, err, "list game env bindings")

	// 绑定行指向打不开的库 → 逐 scope warn 收集 firstErr，但继续其余 scope
	require.NoError(t, metaDB.AutoMigrate(&model.GameEnvBinding{}))
	require.NoError(t, metaDB.Create(&model.GameEnvBinding{
		GameID: "bad", Env: "prod", DatabaseName: "barrier",
	}).Error)
	barrier := filepath.Join(dir, "barrier")
	require.NoError(t, os.WriteFile(barrier, []byte("x"), 0o644)) // 库名被文件占位 → open 失败
	n, err = p2.FinalizeExpiredContractRemovals(ctx, time.Minute)
	require.Error(t, err)
	assert.Zero(t, n)
}

// RegenerateContractTemplates 未注入闭包时 no-op 返回 nil（注册事务提交后的
// 模板收口由进程级注入；测试进程无注入 → 安全直调）。
func TestRegistrationPipeline_RegenerateTemplatesNoop(t *testing.T) {
	p, _ := newSingleDBPipeline(t)
	assert.NoError(t, p.RegenerateContractTemplates(context.Background(), "g", "e"))
}

// wireDashboardRegistrationPipeline 的装配守卫：nil 上下文 / 缺 RegistryStore
// / 缺 DB 时静默跳过，齐全时注入 pipeline。
func TestWireDashboardRegistrationPipeline(t *testing.T) {
	wireDashboardRegistrationPipeline(nil)
	wireDashboardRegistrationPipeline(&svc.ServiceContext{})

	db, err := gorm.Open(gsqlite.Open(filepath.Join(t.TempDir(), "wire.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))
	partial := &svc.ServiceContext{DB: db}
	wireDashboardRegistrationPipeline(partial)

	store := reg.NewStoreWithDB(db)
	full := &svc.ServiceContext{DB: db, RegistryStore: store}
	wireDashboardRegistrationPipeline(full) // 注入不 panic 即装配路径生效
}

// 单库模式委托成功路径：ghost 函数删除静默返回、不存在资源的 rebuild 走
// 清扫 no-op。
func TestRegistrationPipeline_DelegateSuccessPaths(t *testing.T) {
	ctx := context.Background()
	p, _ := newSingleDBPipeline(t)

	resourceKey, err := p.RemoveFunctionContract(ctx, "g", "e", "ghost-fn")
	require.NoError(t, err)
	assert.Empty(t, resourceKey)

	assert.NoError(t, p.RebuildResourceCapability(ctx, "g", "e", "ghost-resource"))
	assert.NoError(t, p.RebuildProposalsForResource(ctx, "g", "e", "ghost-resource"))
}

// scopedDB 分库错误面：绑定表查询失败 / 绑定存在但 game 库解析失败。
func TestRegistrationPipeline_ScopedDBResolveErrors(t *testing.T) {
	dir := t.TempDir()

	// 1. LookupDatabaseName 查询失败：Game 表迁了但 game_envs 没迁
	metaDB, err := gorm.Open(gsqlite.Open(filepath.Join(dir, "m1.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, metaDB.AutoMigrate(&model.Game{}))
	rt := newRouterFixture(t, dir, "m1.db", func(_, _ string, _ string) (string, error) { return "", nil })
	p := &registrationContractPipeline{svcCtx: &svc.ServiceContext{
		DB: metaDB, Router: rt, GameModel: model.NewGameModel(metaDB),
	}}
	_, err = p.scopedDB(context.Background(), "demo", "prod")
	assert.ErrorContains(t, err, "lookup game database binding")

	// 2. 绑定存在但 EnsureDatabase 失败 → Router.GameDB 错误透传
	metaDB2, err := gorm.Open(gsqlite.Open(filepath.Join(dir, "m2.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, metaDB2.AutoMigrate(&model.Game{}, &model.GameEnvBinding{}))
	require.NoError(t, metaDB2.Create(&model.GameEnvBinding{
		GameID: "demo", Env: "prod", DatabaseName: "game_demo_prod",
	}).Error)
	rt2 := newRouterFixture(t, dir, "m2.db", func(_, _ string, _ string) (string, error) {
		return "", context.DeadlineExceeded
	})
	p2 := &registrationContractPipeline{svcCtx: &svc.ServiceContext{
		DB: metaDB2, Router: rt2, GameModel: model.NewGameModel(metaDB2),
	}}
	_, err = p2.scopedDB(context.Background(), "demo", "prod")
	assert.ErrorContains(t, err, "resolve game database")
}

// FinalizeExpiredContractRemovals 的逐 scope 清扫：坏 scope 解析失败收集
// firstErr 但不拖累好 scope（continue 语义）。
func TestRegistrationPipeline_FinalizeMixedScopes(t *testing.T) {
	dir := t.TempDir()
	metaDB, err := gorm.Open(gsqlite.Open(filepath.Join(dir, "meta.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, metaDB.AutoMigrate(&model.Game{}, &model.GameEnvBinding{}))
	for _, b := range []model.GameEnvBinding{
		{GameID: "bad", Env: "prod", DatabaseName: "game_bad_prod"},
		{GameID: "good", Env: "prod", DatabaseName: "game_good_prod"},
	} {
		require.NoError(t, metaDB.Create(&b).Error)
	}
	ensureCalls := 0
	rt := newRouterFixture(t, dir, "meta.db", func(_, _ string, dbName string) (string, error) {
		ensureCalls++
		if dbName == "game_bad_prod" {
			return "", context.DeadlineExceeded
		}
		return filepath.Join(dir, dbName+".db"), nil
	})
	p := &registrationContractPipeline{svcCtx: &svc.ServiceContext{
		DB: metaDB, Router: rt, GameModel: model.NewGameModel(metaDB),
	}}

	n, err := p.FinalizeExpiredContractRemovals(context.Background(), time.Minute)
	require.ErrorIs(t, err, context.DeadlineExceeded, "坏 scope 的错误被记为 firstErr")
	assert.Equal(t, 2, ensureCalls, "坏 scope 不中断好 scope 清扫")
	assert.Zero(t, n)
}

// newRouterFixture 造一个 sqlite 分库路由（open 回调直连文件；ensure 可注入错误）。
func newRouterFixture(t *testing.T, dir, metaName string, ensure func(_, _ string, dbName string) (string, error)) *router.Router {
	t.Helper()
	metaDB, err := gorm.Open(gsqlite.Open(filepath.Join(dir, metaName)), &gorm.Config{})
	require.NoError(t, err)
	return router.New(router.Config{
		Driver:         "sqlite",
		MetaDSN:        filepath.Join(dir, metaName),
		NameForGame:    func(gameID, env string) string { return "game_" + gameID + "_" + env },
		DSNForDatabase: func(_, dbName string) string { return filepath.Join(dir, dbName+".db") },
		EnsureDatabase: ensure,
		Open: func(driver, dsn string) (*gorm.DB, error) {
			return gorm.Open(gsqlite.Open(dsn), &gorm.Config{})
		},
	}, metaDB)
}

// ---- startPipelineMonitor ----

func TestStartPipelineMonitor(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 立即取消：monitor.Run 在首个 select 即返回

	// nil svcCtx / nil AlertModel → no-op
	startPipelineMonitor(ctx, nil)
	startPipelineMonitor(ctx, &svc.ServiceContext{})

	// 有效 AlertModel：坏 REDIS_URL 只降级告警，监控循环照常启动并随 ctx 退出
	db, err := gorm.Open(gsqlite.Open(filepath.Join(t.TempDir(), "alert.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Alert{}))
	svcCtx := &svc.ServiceContext{AlertModel: model.NewAlertModel(db)}
	t.Setenv("REDIS_URL", "http://[::bad-addr")
	startPipelineMonitor(ctx, svcCtx)

	// 合法 REDIS_URL → deadCounter 客户端装配（不主动连接）
	t.Setenv("REDIS_URL", "redis://127.0.0.1:1/0")
	startPipelineMonitor(ctx, svcCtx)
	cancel()
}

// ---- startRegistryCleanup / wrapHTTPHandler ----

func TestStartRegistryCleanup(t *testing.T) {
	// nil store → 打印跳过提示
	startRegistryCleanup(context.Background(), &svc.ServiceContext{})

	// 真实 store：cleanup 例程响应 ctx 取消
	store := reg.NewStore()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		startRegistryCleanup(ctx, &svc.ServiceContext{RegistryStore: store})
		close(done)
	}()
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("cleanup routine did not exit on ctx cancel")
	}
}

// ---- startControlServer ----

func newControlServerSvcCtx(t *testing.T) (*svc.ServiceContext, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open(filepath.Join(t.TempDir(), "ctl.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))
	require.NoError(t, reg.MigrateAgentSessions(db))
	return &svc.ServiceContext{
		Config:            config.Config{},
		DB:                db,
		RegistryStore:     reg.NewStoreWithDB(db),
		AgentSessionModel: reg.NewAgentSessionModel(db),
	}, db
}

// startControlServer 全流程：insecure 监听就绪、MetricsStore 同步、
// ctx 取消后监听退出；资源句柄可 Close。
func TestStartControlServer_InsecureListen(t *testing.T) {
	svcCtx, _ := newControlServerSvcCtx(t)
	c := &config.Config{}
	c.Control.Addr = "127.0.0.1:0"

	ctx, cancel := context.WithCancel(context.Background())
	rt := startControlServer(ctx, c, svcCtx, server.NewAgentSessionStore())
	require.NotNil(t, rt)
	require.NotNil(t, rt.tcpListener)
	require.NotNil(t, rt.controlService)
	require.NotNil(t, svcCtx.MetricsStore)

	cancel()
	require.NoError(t, rt.tcpListener.Close())
	rt.controlService.Stop()
}

// 证书文件缺失 → TCP listener 创建失败，仍返回仅含 controlService 的运行时。
func TestStartControlServer_CertFailure(t *testing.T) {
	svcCtx, _ := newControlServerSvcCtx(t)
	c := &config.Config{}
	c.Control.Addr = "127.0.0.1:0"
	c.Control.Cert = filepath.Join(t.TempDir(), "missing.crt")
	c.Control.Key = filepath.Join(t.TempDir(), "missing.key")

	rt := startControlServer(context.Background(), c, svcCtx, server.NewAgentSessionStore())
	require.NotNil(t, rt)
	assert.Nil(t, rt.tcpListener)
	require.NotNil(t, rt.controlService)
	rt.controlService.Stop()
}

// wireClusterHooks 的守卫矩阵：nil 资源 / nil Cluster / 有钩子时注入真实
// ControlService（tcpListener 为 nil 的分支）。
func TestWireClusterHooks_Guards(t *testing.T) {
	// nil svcCtx / nil Cluster / nil OwnerHooks
	wireClusterHooks(nil, nil)
	wireClusterHooks(&svc.ServiceContext{}, &controlRuntime{})
	db := newTaskLookupDB(t)
	svcCtx := &svc.ServiceContext{DB: db, RegistryStore: reg.NewStoreWithDB(db)}
	wireClusterHooks(svcCtx, &controlRuntime{})

	// 带 OwnerHooks：resources 为 nil 不炸；带 controlService 时注入成功
	svcCtx.Cluster = &svc.ClusterRuntime{OwnerHooks: cluster.NewOwnerHooks(&refreshOwnerFake{}, "self", 1)}
	wireClusterHooks(svcCtx, nil)
	control := server.NewControlService(svcCtx.RegistryStore, reg.NewAgentSessionModel(db))
	wireClusterHooks(svcCtx, &controlRuntime{controlService: control})
}
