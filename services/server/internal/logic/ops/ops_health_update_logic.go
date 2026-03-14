// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package ops

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/common/errorx"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
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

func (l *OpsHealthUpdateLogic) OpsHealthUpdate(req *types.OpsHealthUpdateRequest) (resp *types.OpsHealthUpdateResponse, err error) {
	return nil, errorx.NewNotImplemented("OpsHealthUpdate not implemented")
}
