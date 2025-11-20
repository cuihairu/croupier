package logic

import (
	"context"

	"github.com/cuihairu/croupier/services/edge/internal/svc"
	"github.com/cuihairu/croupier/services/edge/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type TunnelListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewTunnelListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TunnelListLogic {
	return &TunnelListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *TunnelListLogic) TunnelList(req *types.TunnelListRequest) (resp *types.TunnelListResponse, err error) {
	logx.Infof("Tunnel list request: agentId=%s, status=%s, page=%d, size=%d",
		req.AgentId, req.Status, req.Page, req.Size)

	// Get tunnels from manager
	tunnels, total := l.svcCtx.TunnelMgr.ListTunnels(req.AgentId, req.Status, req.Page, req.Size)

	// Convert to response format
	var tunnelResponses []types.TunnelStatusResponse
	for _, tunnel := range tunnels {
		tunnelResp := types.TunnelStatusResponse{
			TunnelId:    tunnel.ID,
			Status:      tunnel.Status,
			Protocol:    tunnel.Protocol,
			RemoteAddr:  tunnel.RemoteAddr,
			LocalAddr:   tunnel.LocalAddr,
			Connections: tunnel.Connections,
			BytesIn:     tunnel.BytesIn,
			BytesOut:    tunnel.BytesOut,
			CreatedAt:   tunnel.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			LastActive:  tunnel.LastActive.Format("2006-01-02T15:04:05Z07:00"),
		}
		tunnelResponses = append(tunnelResponses, tunnelResp)
	}

	return &types.TunnelListResponse{
		Tunnels: tunnelResponses,
		Total:   total,
		Page:    req.Page,
		Size:    req.Size,
	}, nil
}