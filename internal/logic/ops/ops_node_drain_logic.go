package ops

import (
	"context"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/svc"
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

func (l *OpsNodeDrainLogic) OpsNodeDrain(req *OpsNodeActionRequest) (resp *OpsNodeDrainResponse, err error) {
	return nil, errorx.NewNotImplemented("OpsNodeDrain not implemented")
}
