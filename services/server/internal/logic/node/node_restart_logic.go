// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package node

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
)

type NodeRestartLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 重启节点
func NewNodeRestartLogic(ctx context.Context, svcCtx *svc.ServiceContext) *NodeRestartLogic {
	return &NodeRestartLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *NodeRestartLogic) NodeRestart(req *types.NodeActionRequest) error {
	nodeID, err := utils.ValidateNodeID(req.ID)
	if err != nil {
		return err
	}

	if _, err := l.svcCtx.NodeModel.FindByNodeID(l.ctx, nodeID); err != nil {
		return err
	}

	return l.svcCtx.NodeModel.UpdateStatus(l.ctx, nodeID, "restarting")
}
