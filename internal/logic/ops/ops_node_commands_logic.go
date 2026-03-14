
package ops

import (
	"context"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/svc"
	
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

func (l *OpsNodeCommandsLogic) OpsNodeCommands(req *OpsNodeCommandsQuery) (resp *OpsNodeCommandsResponse, err error) {
	return nil, errorx.NewNotImplemented("OpsNodeCommands not implemented")
}
