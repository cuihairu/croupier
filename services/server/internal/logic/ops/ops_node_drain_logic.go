// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package ops

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/common/errorx"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
)

type OpsNodeDrainLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 排空节点
func NewOpsNodeDrainLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsNodeDrainLogic {
	return &OpsNodeDrainLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsNodeDrainLogic) OpsNodeDrain(req *types.OpsNodeActionRequest) (resp *types.OpsNodeDrainResponse, err error) {
	return nil, errorx.NewNotImplemented("OpsNodeDrain not implemented")
}
