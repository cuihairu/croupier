// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package ops

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type OpsNodeCommandsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取节点命令
func NewOpsNodeCommandsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsNodeCommandsLogic {
	return &OpsNodeCommandsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsNodeCommandsLogic) OpsNodeCommands(req *types.OpsNodeCommandsQuery) (resp *types.OpsNodeCommandsResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
