// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package node

import (
	"context"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/model"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

)

type NodesListLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取节点列表
func NewNodesListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *NodesListLogic {
	return &NodesListLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *NodesListLogic) NodesList(req *types.NodesListRequest) (*types.NodesListResponse, error) {
	opts := model.ListNodesOptions{
		Type:   strings.TrimSpace(req.Type),
		Status: strings.TrimSpace(req.Status),
	}

	nodes, err := l.svcCtx.NodeModel.List(l.ctx, opts)
	if err != nil {
		return nil, err
	}

	items := make([]types.Node, 0, len(nodes))
	for i := range nodes {
		items = append(items, utils.BuildNode(&nodes[i]))
	}

	return &types.NodesListResponse{
		Items: items,
	}, nil
}
