// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package node

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
)

type NodeMetaLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取节点元数据
func NewNodeMetaLogic(ctx context.Context, svcCtx *svc.ServiceContext) *NodeMetaLogic {
	return &NodeMetaLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *NodeMetaLogic) NodeMeta(req *types.NodeMetaRequest) (*types.NodeMetaResponse, error) {
	nodeID, err := utils.ValidateNodeID(req.ID)
	if err != nil {
		return nil, err
	}

	node, err := l.svcCtx.NodeModel.FindByNodeID(l.ctx, nodeID)
	if err != nil {
		return nil, err
	}

	return &types.NodeMetaResponse{
		Meta: node.Meta,
	}, nil
}
