// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package ops

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type OpsHealthUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新健康检查配置
func NewOpsHealthUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsHealthUpdateLogic {
	return &OpsHealthUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsHealthUpdateLogic) OpsHealthUpdate(req *types.OpsHealthUpdateRequest) (resp *types.OpsHealthUpdateResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
