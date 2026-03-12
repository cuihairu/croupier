// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package node

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type NodeCommandsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取节点命令
func NewNodeCommandsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *NodeCommandsLogic {
	return &NodeCommandsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *NodeCommandsLogic) NodeCommands(req *types.NodeCommandsRequest) (resp *types.NodeCommandsResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
