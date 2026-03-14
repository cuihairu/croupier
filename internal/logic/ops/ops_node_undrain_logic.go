
package ops

import (
	"context"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/svc"
	
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

func (l *OpsNodeUndrainLogic) OpsNodeUndrain(req *OpsNodeActionRequest) (resp *OpsNodeUndrainResponse, err error) {
	return nil, errorx.NewNotImplemented("OpsNodeUndrain not implemented")
}
