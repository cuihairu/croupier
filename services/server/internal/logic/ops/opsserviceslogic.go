// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package ops

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type OpsServicesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取服务列表
func NewOpsServicesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsServicesLogic {
	return &OpsServicesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsServicesLogic) OpsServices(req *types.OpsServicesRequest) (resp *types.OpsServicesResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
