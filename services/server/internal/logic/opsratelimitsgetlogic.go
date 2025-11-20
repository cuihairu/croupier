package logic

import (
	"context"

	"github.com/cuihairu/croupier/services/api/internal/svc"
	"github.com/cuihairu/croupier/services/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type OpsRateLimitsGetLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewOpsRateLimitsGetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsRateLimitsGetLogic {
	return &OpsRateLimitsGetLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsRateLimitsGetLogic) OpsRateLimitsGet() (*types.RateLimitRulesResponse, error) {
	rules := l.svcCtx.RateLimitRules()
	out := make([]types.RateLimitRule, 0, len(rules))
	for _, r := range rules {
		out = append(out, types.RateLimitRule{
			Scope:    r.Scope,
			Key:      r.Key,
			LimitQPS: r.LimitQPS,
			Match:    r.Match,
			Percent:  r.Percent,
		})
	}
	return &types.RateLimitRulesResponse{Rules: out}, nil
}
