// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package ops

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/common/errorx"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
)

type OpsNodeCommandsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取节点命令
func NewOpsNodeCommandsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsNodeCommandsLogic {
	return &OpsNodeCommandsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsNodeCommandsLogic) OpsNodeCommands(req *types.OpsNodeCommandsQuery) (resp *types.OpsNodeCommandsResponse, err error) {
	return nil, errorx.NewNotImplemented("OpsNodeCommands not implemented")
}
