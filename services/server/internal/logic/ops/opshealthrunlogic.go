// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package ops

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type OpsHealthRunLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 运行健康检查
func NewOpsHealthRunLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsHealthRunLogic {
	return &OpsHealthRunLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsHealthRunLogic) OpsHealthRun(req *types.OpsHealthRunRequest) (resp *types.OpsHealthRunResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
