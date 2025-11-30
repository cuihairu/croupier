// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package node

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type NodeUndrainLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 取消排空节点
func NewNodeUndrainLogic(ctx context.Context, svcCtx *svc.ServiceContext) *NodeUndrainLogic {
	return &NodeUndrainLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *NodeUndrainLogic) NodeUndrain(req *types.NodeActionRequest) error {
	nodeID, err := utils.ValidateNodeID(req.ID)
	if err != nil {
		return err
	}

	if _, err := l.svcCtx.NodeModel.FindByNodeID(l.ctx, nodeID); err != nil {
		return err
	}

	return l.svcCtx.NodeModel.UpdateStatus(l.ctx, nodeID, "active")
}
