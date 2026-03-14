// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package ops

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/common/errorx"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
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

func (l *OpsHealthGetLogic) OpsHealthGet(req *types.OpsHealthGetRequest) (resp *types.OpsHealthGetResponse, err error) {
	return nil, errorx.NewNotImplemented("OpsHealthGet not implemented")
}
