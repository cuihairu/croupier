
package ops

import (
	"context"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/svc"
	
)

type OpsNodeRestartLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 重启节点
func NewOpsNodeRestartLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsNodeRestartLogic {
	return &OpsNodeRestartLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsNodeRestartLogic) OpsNodeRestart(req *OpsNodeActionRequest) (resp *OpsNodeRestartResponse, err error) {
	return nil, errorx.NewNotImplemented("OpsNodeRestart not implemented")
}
