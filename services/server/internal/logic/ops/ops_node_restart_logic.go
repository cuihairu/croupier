// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package ops

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type OpsNodeRestartLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 重启节点
func NewOpsNodeRestartLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsNodeRestartLogic {
	return &OpsNodeRestartLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsNodeRestartLogic) OpsNodeRestart(req *types.OpsNodeActionRequest) (resp *types.OpsNodeRestartResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
