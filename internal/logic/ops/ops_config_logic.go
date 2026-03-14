
package ops

import (
	"context"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/svc"
	
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

func (l *OpsConfigLogic) OpsConfig(req *OpsConfigRequest) (resp *OpsConfigResponse, err error) {
	return nil, errorx.NewNotImplemented("OpsConfig not implemented")
}
