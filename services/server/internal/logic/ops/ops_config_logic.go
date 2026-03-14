// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package ops

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/common/errorx"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
)

type OpsConfigLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取运维配置
func NewOpsConfigLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsConfigLogic {
	return &OpsConfigLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsConfigLogic) OpsConfig(req *types.OpsConfigRequest) (resp *types.OpsConfigResponse, err error) {
	return nil, errorx.NewNotImplemented("OpsConfig not implemented")
}
