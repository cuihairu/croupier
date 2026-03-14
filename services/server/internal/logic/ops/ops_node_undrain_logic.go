// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package ops

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/common/errorx"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
)

type OpsNodeUndrainLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 取消排空节点
func NewOpsNodeUndrainLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsNodeUndrainLogic {
	return &OpsNodeUndrainLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsNodeUndrainLogic) OpsNodeUndrain(req *types.OpsNodeActionRequest) (resp *types.OpsNodeUndrainResponse, err error) {
	return nil, errorx.NewNotImplemented("OpsNodeUndrain not implemented")
}
