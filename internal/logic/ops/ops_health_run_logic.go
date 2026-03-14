package ops

import (
	"context"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/svc"
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

func (l *OpsHealthRunLogic) OpsHealthRun(req *OpsHealthRunRequest) (resp *OpsHealthRunResponse, err error) {
	return nil, errorx.NewNotImplemented("OpsHealthRun not implemented")
}
