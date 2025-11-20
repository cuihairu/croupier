package logic

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

func NewOpsHealthUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsHealthUpdateLogic {
	return &OpsHealthUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsHealthUpdateLogic) OpsHealthUpdate(req *types.OpsHealthUpdateRequest) (*types.GenericOkResponse, error) {
	if req == nil {
		return &types.GenericOkResponse{Ok: true}, nil
	}
	list := make([]svc.HealthCheck, 0, len(req.Checks))
	for _, hc := range req.Checks {
		list = append(list, svc.HealthCheck{
			ID:          hc.Id,
			Kind:        hc.Kind,
			Target:      hc.Target,
			Expect:      hc.Expect,
			IntervalSec: hc.IntervalSec,
			TimeoutMs:   hc.TimeoutMs,
			Region:      hc.Region,
		})
	}
	if err := l.svcCtx.UpdateHealthChecks(list); err != nil {
		return nil, err
	}
	return &types.GenericOkResponse{Ok: true}, nil
}
