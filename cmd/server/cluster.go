package main

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/cluster"
	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/platform/lbstats"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/cuihairu/croupier/internal/transport/tcp"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
)

// startCluster 启动多实例 HA 组件：成员注册、互联端口、发现/心跳循环。
// 返回 (nil, nil) 表示未启用（单实例默认）。
//
// 互联消息处理复用 ControlService 的 TCP 基座；owner 侧本地执行
// （ForwardedInvoke → Agent session）与 caller 侧转发均已接线
// （cmd/server/root.go 的 SetRemoteForwarder + 本文件 localInvoker）。
func startCluster(ctx context.Context, c *config.Config, svcCtx *svc.ServiceContext) (*cluster.Lifecycle, *tcp.Server) {
	cfg := c.Cluster
	if !cfg.Enabled {
		return nil, nil
	}
	if svcCtx == nil {
		slog.Warn("cluster: enabled but service context unavailable, running standalone")
		return nil, nil
	}
	// 协调面存储选择：db（默认，共享关系库）| redis。redis 模式不依赖
	// 共享 DB（成员表/归属表全走 Redis 租约键），DB 仅剩业务库职责。
	useRedis := strings.EqualFold(strings.TrimSpace(cfg.Store), "redis")
	if !useRedis && svcCtx.DB == nil {
		slog.Warn("cluster: enabled but DB unavailable, running standalone")
		return nil, nil
	}

	lcCfg, err := cluster.NormalizeConfig(struct {
		Enabled           bool
		InstanceID        string
		AdvertiseAddr     string
		HeartbeatInterval string
		LeaseTTL          string
		PeerPollInterval  string
	}{
		Enabled:           cfg.Enabled,
		InstanceID:        cfg.InstanceID,
		AdvertiseAddr:     cfg.AdvertiseAddr,
		HeartbeatInterval: cfg.HeartbeatInterval,
		LeaseTTL:          cfg.LeaseTTL,
		PeerPollInterval:  cfg.PeerPollInterval,
	})
	if err != nil {
		slog.Error("cluster: config invalid, running standalone", "error", err)
		return nil, nil
	}

	ownerTTL := time.Duration(0)
	if cfg.OwnerTTL != "" {
		if d, perr := time.ParseDuration(cfg.OwnerTTL); perr == nil {
			ownerTTL = d
		}
	}

	var member cluster.Membership
	var resolver cluster.OwnerStore
	if useRedis {
		rdb, err := openClusterRedis(cfg, c)
		if err != nil {
			slog.Error("cluster: redis store unavailable, running standalone", "error", err)
			return nil, nil
		}
		defer func() { _ = rdb.Close() }()
		pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		pingErr := rdb.Ping(pingCtx).Err()
		cancel()
		err = pingErr
		if err != nil {
			slog.Error("cluster: redis ping failed, running standalone", "error", err)
			return nil, nil
		}
		member = cluster.NewRedisMembership(rdb, lcCfg.LeaseTTL)
		resolver = cluster.NewRedisOwnerResolver(rdb, ownerTTL)
	} else {
		dbm := cluster.NewDBMembership(svcCtx.DB, lcCfg.LeaseTTL)
		if err := dbm.EnsureTable(ctx); err != nil {
			slog.Error("cluster: ensure membership table failed, running standalone", "error", err)
			return nil, nil
		}
		member = dbm
		dbr := cluster.NewDBOwnerResolver(svcCtx.DB, ownerTTL)
		if err := dbr.EnsureTable(ctx); err != nil {
			slog.Error("cluster: ensure owner table failed, running standalone", "error", err)
			return nil, nil
		}
		resolver = dbr
	}

	// 互联当前语义是内网明文：ClusterConfig 没有证书配置面，Insecure
	// 必须置 true——否则 listen() 落入 TLS 分支，空 tls.Config 直接报
	// "neither Certificates ... set"（线上双实例互联监听全挂根因）。
	// dialer 与 server 必须同语义，共用同一 Config。
	icAddr := cfg.InterconnectAddr
	if icAddr == "" {
		icAddr = cfg.AdvertiseAddr
	}
	icCfg := &tcp.Config{
		Address:     icAddr,
		RecvTimeout: 90 * time.Second,
		SendTimeout: 30 * time.Second,
		Insecure:    true,
	}

	lifecycle := cluster.Start(ctx, lcCfg, member, resolver, cluster.NewTCPDialer(icCfg))
	if lifecycle == nil {
		return nil, nil
	}

	// 归属 reconcile 兜底：本实例持有的 agent，只要本地 registry 会话
	// 仍存活（LastSeen 新鲜），周期性续期归属行——防止心跳处理路径的
	// 偶发漏 Touch 把活跃 agent 冻成过期（/ops/nodes 聚合按 TTL 判活）。
	// 本地会话过期（僵尸）则不再续期，行按 TTL 自然衰减。
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if svcCtx.RegistryStore == nil {
					continue
				}
				now := time.Now()
				ids := make([]string, 0, 4)
				svcCtx.RegistryStore.Mu().RLock()
				for id, sess := range svcCtx.RegistryStore.AgentsUnsafe() {
					if sess != nil && now.Sub(sess.LastSeen) < time.Minute {
						ids = append(ids, id)
					}
				}
				svcCtx.RegistryStore.Mu().RUnlock()
				for _, id := range ids {
					if err := resolver.Touch(ctx, id); err != nil {
						slog.Warn("cluster: ownership touch failed", "agent_id", id, "error", err)
					}
				}

				// 远端 agent 快照刷新：owner 表活跃但连接在对端的 agent，
				// 本地 registry 快照的 ExpireAt/函数表冻结（心跳只更新持有
				// 实例的内存），过期后会被内存清理器删除——跨实例函数调用
				// 的候选集随本地实例视角丢失（"no live agent"）。从共享 DB
				// 回读刷新（持有实例的节流落盘保证 DB 行新鲜）。
				refreshRemoteSnapshots(ctx, svcCtx, resolver, lcCfg.InstanceID)
			}
		}
	}()

	svcCtx.Cluster = &svc.ClusterRuntime{
		InstanceID: lcCfg.InstanceID,
		Epoch:      lifecycle.Epoch(),
		Mesh:       lifecycle.Mesh(),
		Resolver:   resolver,
		// LB 监控代理（未配置 Prometheus 时 nil，/ops/lb 隐藏）
		LBStats: lbstats.NewLBStatsService(cfg.LbPrometheusUrl),
	}

	// 归属钩子：Agent 注册/心跳/断连 → 共享归属表
	//（需要在 control/tcp listener 装配后注入，见 wiringClusterHooks）。
	ownerHooks := cluster.NewOwnerHooks(resolver, lcCfg.InstanceID, lifecycle.Epoch())
	svcCtx.Cluster.OwnerHooks = ownerHooks
	svcCtx.Cluster.Membership = member
	svcCtx.Cluster.OwnerStats = func(ctx context.Context) map[string]int64 {
		counts, err := resolver.CountAgentsByOwner(ctx)
		if err != nil {
			return map[string]int64{}
		}
		return counts
	}
	svcCtx.Cluster.ListAgentOwners = func(ctx context.Context) ([]cluster.AgentOwnerRecord, error) {
		return resolver.ListAliveOwners(ctx)
	}

	var invoker cluster.LocalInvoker
	if svcCtx.Dispatcher != nil {
		invoker = localInvoker{dispatcher: svcCtx.Dispatcher, audit: svcCtx.AuditService}
	}
	handler := newInterconnectHandler(lifecycle, invoker)
	srv, err := tcp.NewServer(icCfg, handler)
	if err != nil {
		slog.Error("cluster: interconnect listen failed (mesh dial-out still active)", "addr", icAddr, "error", err)
		return lifecycle, nil
	}
	go func() {
		if err := srv.Serve(ctx); err != nil {
			slog.Warn("cluster: interconnect server stopped", "error", err)
		}
	}()
	slog.Info("cluster: interconnect listening", "addr", icAddr)
	return lifecycle, srv
}

// interconnectHandler 把互联消息路由到 cluster 处理器。
type interconnectHandler struct {
	lifecycle *cluster.Lifecycle
	forward   func(ctx context.Context, body []byte) []byte
	hello     func(ctx context.Context, body []byte) []byte
}

func newInterconnectHandler(lc *cluster.Lifecycle, invoker cluster.LocalInvoker) *interconnectHandler {
	h := &interconnectHandler{lifecycle: lc}
	h.forward = cluster.ServeForwardHandler(lc.Epoch(), func(ctx context.Context, req *cluster.ForwardedInvoke) (*cluster.ForwardedResult, error) {
		if invoker == nil {
			return nil, fmt.Errorf("local invoker unavailable")
		}
		return invoker.InvokeLocal(ctx, req)
	})
	h.hello = cluster.ServeHelloHandler(lc.Mesh().SelfInfo(), lc.Epoch())
	return h
}

// Handle implements transportcore.Handler.
func (h *interconnectHandler) Handle(ctx context.Context, msgID uint32, reqID uint32, body []byte) ([]byte, error) {
	switch msgID {
	case protocol.MsgServerHelloRequest:
		return h.hello(ctx, body), nil
	case protocol.MsgForwardInvokeReq:
		return h.forward(ctx, body), nil
	default:
		return nil, fmt.Errorf("cluster: unexpected msg 0x%06x", msgID)
	}
}

// wireClusterHooks 把归属钩子注入 control service 与 TCP listener。
// controlResources 在 startControlServer 之后调用（cluster.Start 已先行）。
func wireClusterHooks(svcCtx *svc.ServiceContext, resources *controlRuntime) {
	if svcCtx == nil || svcCtx.Cluster == nil || svcCtx.Cluster.OwnerHooks == nil {
		return
	}
	if resources != nil && resources.controlService != nil {
		resources.controlService.SetClusterHooks(svcCtx.Cluster.OwnerHooks)
	}
	if resources != nil && resources.tcpListener != nil {
		resources.tcpListener.SetClusterHooks(svcCtx.Cluster.OwnerHooks)
	}
}

// localInvoker 适配 dispatcher 为集群本地执行器：转发请求 → 本地 Agent 连接。
// 按 ForwardedInvoke.Kind 分发三类调用（invoke/start_task/cancel_task）。
type localInvoker struct {
	dispatcher interface {
		InvokeRequestOnAgent(ctx context.Context, agentID string, req *sdkv1.InvokeRequest) ([]byte, error)
		StartTaskOnAgent(ctx context.Context, agentID string, req *sdkv1.InvokeRequest) ([]byte, error)
		CancelTaskOnAgent(ctx context.Context, agentID, taskID string) ([]byte, error)
	}
	// audit 为 nil（AuditService 未初始化，如 DB 不可用）时跳过 owner 侧
	// 审计——转发执行不受影响。
	audit *audit.AuditService
}

func (li localInvoker) InvokeLocal(ctx context.Context, req *cluster.ForwardedInvoke) (*cluster.ForwardedResult, error) {
	if li.dispatcher == nil {
		return nil, fmt.Errorf("dispatcher unavailable")
	}
	res, err := li.dispatch(ctx, req)
	li.auditForward(ctx, req, res, err)
	return res, err
}

func (li localInvoker) dispatch(ctx context.Context, req *cluster.ForwardedInvoke) (*cluster.ForwardedResult, error) {
	switch req.Kind {
	case cluster.ForwardKindCancel:
		// 取消：TaskID 随帧；路由清理是 caller 职责（task routing 在
		// caller 实例内存态），owner 只投递给本地 agent 连接。
		if strings.TrimSpace(req.TaskID) == "" {
			return &cluster.ForwardedResult{OK: false, Error: "cancel_task forward missing taskId"}, nil
		}
		if _, err := li.dispatcher.CancelTaskOnAgent(ctx, req.AgentID, req.TaskID); err != nil {
			return &cluster.ForwardedResult{OK: false, Error: err.Error()}, nil
		}
		return &cluster.ForwardedResult{OK: true}, nil
	case cluster.ForwardKindStartTask:
		// 异步任务：task_runs 行与服务端 task ID 均为 caller 职责（ID 在
		// metadata.taskId 里随帧带到），owner 只做本地投递。
		respBytes, err := li.dispatcher.StartTaskOnAgent(ctx, req.AgentID, forwardInvokeRequest(req))
		if err != nil {
			return &cluster.ForwardedResult{OK: false, Error: err.Error()}, nil
		}
		return &cluster.ForwardedResult{OK: true, Payload: respBytes}, nil
	default:
		// Kind 为空 = invoke（转发链路首个使用者，旧 caller 兼容）。
		respBytes, err := li.dispatcher.InvokeRequestOnAgent(ctx, req.AgentID, forwardInvokeRequest(req))
		if err != nil {
			return &cluster.ForwardedResult{OK: false, Error: err.Error()}, nil
		}
		return &cluster.ForwardedResult{OK: true, Payload: respBytes}, nil
	}
}

// auditForward 落 owner 侧转发执行审计（成功与失败各一条语义，本方法
// 每次投递调用一次）。
//
// 信任边界：互联 mesh 是内网明文 TCP、hello 仅校验角色字符串，Caller
// 身份由 caller 实例注入，owner **不重查** policy/approval——caller 已
// 走完完整鉴权链（policy+审批），重查会卡审批续跑。此设计的前提是
// 互联端口仅集群内网可达，接 mTLS 前不得暴露公网
// （docs/architecture/server-ha-multi-instance.md §5.3）。
func (li localInvoker) auditForward(ctx context.Context, req *cluster.ForwardedInvoke, res *cluster.ForwardedResult, err error) {
	if li.audit == nil {
		return
	}
	// ForwardKindInvoke 常量本身是空串（wire 兼容旧 caller），审计
	// details 里落显式字面便于检索。
	kind := req.Kind
	if kind == "" {
		kind = "invoke"
	}
	// actor 与 caller 侧审计（internal/api/function）同键：username 作
	// id，后台派发（scheduler，无身份）回落 system。
	actor := req.Caller.Username
	if actor == "" {
		actor = "system"
	}
	outcome, errMsg := "success", ""
	if err != nil {
		outcome, errMsg = "failure", err.Error()
	} else if res != nil && !res.OK {
		outcome, errMsg = "failure", res.Error
	}
	details := map[string]interface{}{
		"agent_id":  req.AgentID,
		"kind":      kind,
		"forwarded": true,
		"gameId":    req.Caller.GameID,
		"env":       req.Caller.Env,
		"traceId":   req.Caller.TraceID,
	}
	if req.Caller.AdminID != 0 {
		details["admin_id"] = req.Caller.AdminID
	}
	if req.TaskID != "" {
		details["task_id"] = req.TaskID
	}
	// 审计失败不影响转发主路径（与 caller 侧审计同策略），但必须留痕：
	// 双实例链 sequence 撞号重试耗尽等写库失败若静默吞掉，排障时只能靠
	// postgres 日志抓现场（2026-09-11 线上即此路径）。
	if _, err := li.audit.Log(ctx, audit.EventFunctionInvoke,
		audit.WithActorID(actor, "user", actor),
		audit.WithResourceID("function", req.FunctionID),
		audit.WithDetails(details),
		audit.WithOutcome(outcome, errMsg),
	); err != nil {
		slog.Default().Warn("cluster: owner-side forward audit failed",
			"function", req.FunctionID, "agent", req.AgentID, "kind", kind, "error", err)
	}
}

// forwardInvokeRequest 从转发帧重建 InvokeRequest（invoke/start_task 共用）：
// 转发标记不透传 Agent；调用者信息进 metadata 供审计与 scope 路由。
func forwardInvokeRequest(req *cluster.ForwardedInvoke) *sdkv1.InvokeRequest {
	invokeReq := &sdkv1.InvokeRequest{
		FunctionId:     req.FunctionID,
		Payload:        req.Payload,
		Metadata:       req.Metadata,
		IdempotencyKey: req.IdempotencyKey,
	}
	if invokeReq.Metadata == nil {
		invokeReq.Metadata = map[string]string{}
	}
	invokeReq.Metadata["forwarded_by"] = req.Caller.Username
	if req.Caller.GameID != "" {
		invokeReq.Metadata["gameId"] = req.Caller.GameID
	}
	if req.Caller.Env != "" {
		invokeReq.Metadata["env"] = req.Caller.Env
	}
	return invokeReq
}

// refreshRemoteSnapshots 把归属表活跃、连接在对端实例的 agent 的 DB
// 会话刷回本地 registry（函数表 + ExpireAt），维持跨实例调用候选集。
func refreshRemoteSnapshots(ctx context.Context, svcCtx *svc.ServiceContext, resolver cluster.OwnerStore, selfID string) {
	owners, err := resolver.ListAliveOwners(ctx)
	if err != nil || len(owners) == 0 {
		return
	}
	remoteIDs := make([]string, 0, len(owners))
	for _, rec := range owners {
		if rec.InstanceID != selfID {
			remoteIDs = append(remoteIDs, rec.AgentID)
		}
	}
	if len(remoteIDs) == 0 || svcCtx.DB == nil || svcCtx.RegistryStore == nil {
		return
	}
	sessions, err := reg.NewAgentSessionModel(svcCtx.DB).LoadActiveSessions(ctx)
	if err != nil {
		return
	}
	store := svcCtx.RegistryStore
	store.Mu().RLock()
	local := store.AgentsUnsafe()
	store.Mu().RUnlock()
	for i := range sessions {
		sess := sessions[i]
		if sess == nil {
			continue
		}
		need := false
		for _, id := range remoteIDs {
			if id == sess.AgentID {
				need = true
				break
			}
		}
		if !need {
			continue
		}
		cur, ok := local[sess.AgentID]
		// 本地无快照（被清理）或快照临期/函数表/Provider 表落后：刷回。
		// 持有连接的实例永远走本地实时会话，这里的 Upsert 不会覆盖活跃
		// 本地视图（need 集合已排除本实例持有的 agent）。Functions 与
		// Providers 都是「只升不降」：收缩（provider 断开）不在此回灌，
		// 等 ExpireAt 临期条件兜底，避免 DB 写节流窗口内来回抖动。
		if !ok || cur == nil || len(sess.Functions) > len(cur.Functions) || len(sess.Providers) > len(cur.Providers) || time.Until(cur.ExpireAt) < 10*time.Minute {
			if err := store.UpsertAgent(sess); err == nil && (!ok || cur == nil) {
				slog.Info("cluster: refreshed remote agent snapshot", "agent_id", sess.AgentID, "owner", ownerInstance(owners, sess.AgentID))
			}
		}
	}
}

func ownerInstance(owners []cluster.AgentOwnerRecord, agentID string) string {
	for _, rec := range owners {
		if rec.AgentID == agentID {
			return rec.InstanceID
		}
	}
	return ""
}

// openClusterRedis 解析协调面 Redis 连接：cluster.redisAddr 优先，
// 空则回退 cache 段的 redis 配置（复用同一实例，不额外搭存储）。
func openClusterRedis(cfg config.ClusterConfig, c *config.Config) (*redis.Client, error) {
	addr := strings.TrimSpace(cfg.RedisAddr)
	password := cfg.RedisPassword
	db := cfg.RedisDB
	if addr == "" && c != nil && strings.EqualFold(strings.TrimSpace(c.Cache.Type), "redis") {
		addr = strings.TrimSpace(c.Cache.Addr)
		if password == "" {
			password = c.Cache.Password
		}
		if db == 0 {
			db = c.Cache.DB
		}
	}
	if addr == "" {
		return nil, fmt.Errorf("cluster.store=redis 但未配置 redisAddr，且 cache 段非 redis，无可用连接")
	}
	return redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     password,
		DB:           db,
		DialTimeout:  3 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	}), nil
}
