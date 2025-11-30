// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package edge

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/cuihairu/croupier/services/edge/internal/svc"
	"github.com/cuihairu/croupier/services/edge/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type TunnelCloseLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewTunnelCloseLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TunnelCloseLogic {
	return &TunnelCloseLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *TunnelCloseLogic) TunnelClose(req *types.TunnelCloseRequest) (*types.TunnelCloseResponse, error) {
	if req == nil || strings.TrimSpace(req.TunnelId) == "" {
		return nil, errors.New("tunnel_id 不能为空")
	}

	state := l.svcCtx.State
	state.Mu.Lock()
	defer state.Mu.Unlock()

	record := state.Tunnels[req.TunnelId]
	if record == nil {
		return nil, errors.New("tunnel not found")
	}

	record.Status = "closed"
	record.LastActive = time.Now()

	return &types.TunnelCloseResponse{
		Success: true,
		Message: "tunnel closed",
	}, nil
}
