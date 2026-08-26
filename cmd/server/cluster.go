package main

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/cuihairu/croupier/internal/cluster"
	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/cuihairu/croupier/internal/transport/tcp"
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
	if svcCtx == nil || svcCtx.DB == nil {
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

	member := cluster.NewDBMembership(svcCtx.DB, lcCfg.LeaseTTL)
	if err := member.EnsureTable(ctx); err != nil {
		slog.Error("cluster: ensure membership table failed, running standalone", "error", err)
		return nil, nil
	}

	ownerTTL := time.Duration(0)
	if cfg.OwnerTTL != "" {
		if d, perr := time.ParseDuration(cfg.OwnerTTL); perr == nil {
			ownerTTL = d
		}
	}
	resolver := cluster.NewDBOwnerResolver(svcCtx.DB, ownerTTL)
	if err := resolver.EnsureTable(ctx); err != nil {
		slog.Error("cluster: ensure owner table failed, running standalone", "error", err)
		return nil, nil
	}

	lifecycle := cluster.Start(ctx, lcCfg, member, resolver, nil)
	if lifecycle == nil {
		return nil, nil
	}
	svcCtx.Cluster = &svc.ClusterRuntime{
		InstanceID: lcCfg.InstanceID,
		Epoch:      lifecycle.Epoch(),
		Mesh:       lifecycle.Mesh(),
	}

	// 互联端口（默认与 advertiseAddr 同地址；独立监听，Agent-facing 端口外）。
	icAddr := cfg.InterconnectAddr
	if icAddr == "" {
		icAddr = cfg.AdvertiseAddr
	}
	handler := newInterconnectHandler(lifecycle)
	icCfg := &tcp.Config{
		Address:     icAddr,
		RecvTimeout: 90 * time.Second,
		SendTimeout: 30 * time.Second,
	}
	if cfg.InsecureSkipTLS {
		icCfg.InsecureSkipVerify = true
	}
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

func newInterconnectHandler(lc *cluster.Lifecycle) *interconnectHandler {
	h := &interconnectHandler{lifecycle: lc}
	// owner 侧本地执行接线：当前阶段返回明确错误（执行接线未启用），
	// 防止静默假成功。随 RegistryStore 共享化完成后接入 dispatch。
	h.forward = cluster.ServeForwardHandler(lc.Epoch(), func(ctx context.Context, req *cluster.ForwardedInvoke) (*cluster.ForwardedResult, error) {
		return nil, fmt.Errorf("local invoke wiring pending (see HA doc §7)")
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
