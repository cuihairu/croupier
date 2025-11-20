package logic

import (
	"context"
	"strings"

	"github.com/cuihairu/croupier/services/api/internal/svc"
	"github.com/cuihairu/croupier/services/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type OpsNodeCommandsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewOpsNodeCommandsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsNodeCommandsLogic {
	return &OpsNodeCommandsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsNodeCommandsLogic) OpsNodeCommands(req *types.OpsNodeCommandsQuery) (*types.OpsNodeCommandsResponse, error) {
	if req == nil || strings.TrimSpace(req.NodeId) == "" {
		return nil, ErrInvalidRequest
	}
	cmds := l.svcCtx.PopNodeCommands(req.NodeId)
	return &types.OpsNodeCommandsResponse{Commands: cmds}, nil
}
