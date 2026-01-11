// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package ops

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type OpsHealthGetLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取健康状态
func NewOpsHealthGetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsHealthGetLogic {
	return &OpsHealthGetLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsHealthGetLogic) OpsHealthGet(req *types.OpsHealthGetRequest) (resp *types.OpsHealthGetResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
