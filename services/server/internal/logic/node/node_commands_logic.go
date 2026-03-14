// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package node

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
)

type NodeCommandsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取节点命令
func NewNodeCommandsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *NodeCommandsLogic {
	return &NodeCommandsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *NodeCommandsLogic) NodeCommands(req *types.NodeCommandsRequest) (*types.NodeCommandsResponse, error) {
	commands, err := l.svcCtx.NodeModel.ListCommands(l.ctx)
	if err != nil {
		return nil, err
	}

	items := make([]types.NodeCommand, 0, len(commands))
	for _, cmd := range commands {
		items = append(items, types.NodeCommand{
			Name:        cmd.Name,
			Description: cmd.Description,
		})
	}

	return &types.NodeCommandsResponse{
		Items: items,
	}, nil
}
