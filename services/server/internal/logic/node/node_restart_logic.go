// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package node

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type NodeRestartLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 重启节点
func NewNodeRestartLogic(ctx context.Context, svcCtx *svc.ServiceContext) *NodeRestartLogic {
	return &NodeRestartLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *NodeRestartLogic) NodeRestart(req *types.NodeActionRequest) error {
	// todo: add your logic here and delete this line

	return nil
}
