// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package node

import (
	"context"
	"errors"

	"gorm.io/datatypes"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

)

type NodeMetaUpdateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新节点元数据
func NewNodeMetaUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *NodeMetaUpdateLogic {
	return &NodeMetaUpdateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *NodeMetaUpdateLogic) NodeMetaUpdate(req *types.NodeMetaUpdateRequest) (*types.NodeMetaResponse, error) {
	nodeID, err := utils.ValidateNodeID(req.ID)
	if err != nil {
		return nil, err
	}

	metaMap, ok := req.Meta.(map[string]interface{})
	if !ok {
		return nil, errors.New("meta 必须是对象")
	}

	if err := l.svcCtx.NodeModel.UpdateMeta(l.ctx, nodeID, map[string]interface{}{
		"meta": datatypes.JSONMap(metaMap),
	}); err != nil {
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
