// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package ops

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/common/errorx"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
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

func (l *OpsNodeRestartLogic) OpsNodeRestart(req *types.OpsNodeActionRequest) (resp *types.OpsNodeRestartResponse, err error) {
	return nil, errorx.NewNotImplemented("OpsNodeRestart not implemented")
}
