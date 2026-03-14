// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package node

import (
	"context"
	"fmt"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

)

type NodeDrainLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 排空节点
func NewNodeDrainLogic(ctx context.Context, svcCtx *svc.ServiceContext) *NodeDrainLogic {
	return &NodeDrainLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *NodeDrainLogic) NodeDrain(req *types.NodeDrainRequest) error {
	nodeID, err := utils.ValidateNodeID(req.ID)
	if err != nil {
		return err
	}

	if _, err := l.svcCtx.NodeModel.FindByNodeID(l.ctx, nodeID); err != nil {
		return err
	}

	status := "draining"
	if req.Timeout > 0 {
		status = fmt.Sprintf("draining:%d", req.Timeout)
	}

	return l.svcCtx.NodeModel.UpdateStatus(l.ctx, nodeID, status)
}
