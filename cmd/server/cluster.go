package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/cuihairu/croupier/internal/cluster"
	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/cuihairu/croupier/internal/transport/tcp"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
)

// startCluster 启动多实例 HA 组件：成员注册、互联端口、发现/心跳循环。
// 返回 (nil, nil) 表示未启用（单实例默认）。
//
// 互联消息处理复用 ControlService 的 TCP 基座；owner 侧本地执行
// （ForwardedInvoke → Agent session）由 dispatch 接线提供，当前阶段
// 落地为成员表 + 互联通道 + fencing 校验（转发执行接线随 RegistryStore
// 共享化启用，见 HA 文档 §7 拆解）。
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
			}
		}
	}()

	svcCtx.Cluster = &svc.ClusterRuntime{
		InstanceID: lcCfg.InstanceID,
		Epoch:      lifecycle.Epoch(),
		Mesh:       lifecycle.Mesh(),
		Resolver:   resolver,
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
		invoker = localInvoker{dispatcher: svcCtx.Dispatcher}
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
type localInvoker struct {
	dispatcher interface {
		InvokeRequestOnAgent(ctx context.Context, agentID string, req *sdkv1.InvokeRequest) ([]byte, error)
	}
}

func (li localInvoker) InvokeLocal(ctx context.Context, req *cluster.ForwardedInvoke) (*cluster.ForwardedResult, error) {
	if li.dispatcher == nil {
		return nil, fmt.Errorf("dispatcher unavailable")
	}
	invokeReq := &sdkv1.InvokeRequest{
		FunctionId:     req.FunctionID,
		Payload:        req.Payload,
		Metadata:       req.Metadata,
		IdempotencyKey: req.IdempotencyKey,
	}
	if invokeReq.Metadata == nil {
		invokeReq.Metadata = map[string]string{}
	}
	// 转发标记不透传 Agent；调用者信息进 metadata 供审计。
	invokeReq.Metadata["forwarded_by"] = req.Caller.Username
	if req.Caller.GameID != "" {
		invokeReq.Metadata["game_id"] = req.Caller.GameID
	}
	if req.Caller.Env != "" {
		invokeReq.Metadata["env"] = req.Caller.Env
	}
	respBytes, err := li.dispatcher.InvokeRequestOnAgent(ctx, req.AgentID, invokeReq)
	if err != nil {
		return &cluster.ForwardedResult{OK: false, Error: err.Error()}, nil
	}
	return &cluster.ForwardedResult{OK: true, Payload: json.RawMessage(respBytes)}, nil
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
