// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package ops

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/common/errorx"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
)

type OpsHealthRunLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 运行健康检查
func NewOpsHealthRunLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsHealthRunLogic {
	return &OpsHealthRunLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsHealthRunLogic) OpsHealthRun(req *types.OpsHealthRunRequest) (resp *types.OpsHealthRunResponse, err error) {
	return nil, errorx.NewNotImplemented("OpsHealthRun not implemented")
}
