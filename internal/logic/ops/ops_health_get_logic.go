
package ops

import (
	"context"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/svc"
	
)

type OpsHealthGetLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取健康状态
func NewOpsHealthGetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsHealthGetLogic {
	return &OpsHealthGetLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsHealthGetLogic) OpsHealthGet(req *OpsHealthGetRequest) (resp *OpsHealthGetResponse, err error) {
	return nil, errorx.NewNotImplemented("OpsHealthGet not implemented")
}
