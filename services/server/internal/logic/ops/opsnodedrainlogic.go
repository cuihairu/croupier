// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package ops

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type OpsNodeDrainLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 排空节点
func NewOpsNodeDrainLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsNodeDrainLogic {
	return &OpsNodeDrainLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsNodeDrainLogic) OpsNodeDrain(req *types.OpsNodeActionRequest) (resp *types.OpsNodeDrainResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
