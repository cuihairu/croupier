package logic

import (
	"context"

	"github.com/cuihairu/croupier/services/api/internal/svc"
	"github.com/cuihairu/croupier/services/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type OpsHealthRunLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewOpsHealthRunLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsHealthRunLogic {
	return &OpsHealthRunLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsHealthRunLogic) OpsHealthRun(req *types.OpsHealthRunRequest) (*types.GenericOkResponse, error) {
	id := ""
	if req != nil {
		id = req.Id
	}
	go l.svcCtx.RunHealthChecks(id)
	return &types.GenericOkResponse{Ok: true}, nil
}
