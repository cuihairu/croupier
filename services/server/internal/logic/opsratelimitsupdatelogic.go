package logic

import (
	"context"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type OpsRateLimitsUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewOpsRateLimitsUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsRateLimitsUpdateLogic {
	return &OpsRateLimitsUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsRateLimitsUpdateLogic) OpsRateLimitsUpdate(req *types.RateLimitRulesRequest) (*types.GenericOkResponse, error) {
	if req == nil || len(req.Rules) == 0 {
		return nil, ErrRateRuleInvalid
	}
	var normalized []svc.RateLimitRule
	for _, r := range req.Rules {
		scope := strings.ToLower(strings.TrimSpace(r.Scope))
		key := strings.TrimSpace(r.Key)
		if scope == "" || key == "" || r.LimitQPS <= 0 {
			continue
		}
		percent := r.Percent
		if percent <= 0 {
			percent = 100
		}
		normalized = append(normalized, svc.RateLimitRule{
			Scope:    scope,
			Key:      key,
			LimitQPS: r.LimitQPS,
			Match:    r.Match,
			Percent:  percent,
		})
	}
	if len(normalized) == 0 {
		return nil, ErrRateRuleInvalid
	}
	if err := l.svcCtx.ReplaceRateLimitRules(normalized); err != nil {
		return nil, err
	}
	return &types.GenericOkResponse{Ok: true}, nil
}
