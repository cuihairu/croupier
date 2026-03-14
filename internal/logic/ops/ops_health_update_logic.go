
package ops

import (
	"context"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/svc"
	
)

type OpsHealthUpdateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新健康检查配置
func NewOpsHealthUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsHealthUpdateLogic {
	return &OpsHealthUpdateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsHealthUpdateLogic) OpsHealthUpdate(req *OpsHealthUpdateRequest) (resp *OpsHealthUpdateResponse, err error) {
	return nil, errorx.NewNotImplemented("OpsHealthUpdate not implemented")
}
