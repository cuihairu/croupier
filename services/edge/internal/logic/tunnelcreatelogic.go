package logic

import (
	"context"
	"fmt"
	"time"

	"github.com/cuihairu/croupier/services/edge/internal/svc"
	"github.com/cuihairu/croupier/services/edge/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type TunnelCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewTunnelCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TunnelCreateLogic {
	return &TunnelCreateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *TunnelCreateLogic) TunnelCreate(req *types.TunnelCreateRequest) (resp *types.TunnelCreateResponse, err error) {
	logx.Infof("Tunnel creation request: agentId=%s, serverId=%s, protocol=%s",
		req.AgentId, req.ServerId, req.Protocol)

	// Validate required fields
	if req.AgentId == "" || req.ServerId == "" || req.Protocol == "" {
		return &types.TunnelCreateResponse{
			Success: false,
			Message: "missing required fields: agent_id, server_id, protocol",
		}, nil
	}

	// Generate unique tunnel ID
	tunnelId := fmt.Sprintf("%s-%s-%d", req.AgentId, req.ServerId, time.Now().Unix())

	// Create tunnel object
	tunnel := &svc.Tunnel{
		ID:         tunnelId,
		AgentID:    req.AgentId,
		ServerID:   req.ServerId,
		Protocol:   req.Protocol,
		RemoteAddr: req.RemoteAddr,
		LocalAddr:  req.LocalAddr,
		Options:    req.Options,
		PublicURL:  l.generatePublicURL(tunnelId, req.Protocol),
	}

	// Try to create tunnel
	if !l.svcCtx.TunnelMgr.CreateTunnel(tunnel) {
		return &types.TunnelCreateResponse{
			Success: false,
			Message: "failed to create tunnel: capacity reached or duplicate ID",
		}, nil
	}

	// Add to load balancer
	l.svcCtx.LoadBalancer.AddTunnel(tunnel)

	// Start tunnel listener
	go l.startTunnelListener(tunnel)

	logx.Infof("Tunnel created successfully: %s", tunnelId)

	return &types.TunnelCreateResponse{
		Success:   true,
		TunnelId:  tunnelId,
		Message:   "Tunnel created successfully",
		PublicUrl: tunnel.PublicURL,
	}, nil
}

func (l *TunnelCreateLogic) generatePublicURL(tunnelID, protocol string) string {
	switch protocol {
	case "http", "https":
		return fmt.Sprintf("https://%s/tunnel/%s", l.svcCtx.Config.Server.PublicAddr, tunnelID)
	case "grpc":
		return fmt.Sprintf("grpc://%s/tunnel/%s", l.svcCtx.Config.Server.PublicAddr, tunnelID)
	case "ws":
		return fmt.Sprintf("wss://%s/tunnel/%s", l.svcCtx.Config.Server.PublicAddr, tunnelID)
	default:
		return fmt.Sprintf("%s/tunnel/%s", l.svcCtx.Config.Server.PublicAddr, tunnelID)
	}
}

func (l *TunnelCreateLogic) startTunnelListener(tunnel *svc.Tunnel) {
	logx.Infof("Starting tunnel listener for: %s", tunnel.ID)

	// Update tunnel status
	l.svcCtx.TunnelMgr.UpdateTunnel(tunnel.ID, func(t *svc.Tunnel) {
		t.Status = "listening"
	})

	// Simulate tunnel establishment
	// In a real implementation, this would:
	// 1. Create a network listener
	// 2. Handle incoming connections
	// 3. Forward data to the agent
	// 4. Manage connection lifecycle

	// Simulate tunnel being ready after 1 second
	time.Sleep(1 * time.Second)

	l.svcCtx.TunnelMgr.UpdateTunnel(tunnel.ID, func(t *svc.Tunnel) {
		t.Status = "active"
	})

	logx.Infof("Tunnel listener ready: %s", tunnel.ID)
}