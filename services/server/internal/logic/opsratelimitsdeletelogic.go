package logic

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type OpsRateLimitsDeleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewOpsRateLimitsDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsRateLimitsDeleteLogic {
	return &OpsRateLimitsDeleteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsRateLimitsDeleteLogic) OpsRateLimitsDelete(req *types.RateLimitDeleteRequest) (*types.GenericOkResponse, error) {
	if err := l.svcCtx.DeleteRateLimitRule(req.Scope, req.Key); err != nil {
		return nil, err
	}
	return &types.GenericOkResponse{Ok: true}, nil
}
