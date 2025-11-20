package logic

import (
	"context"
	"errors"
	"strings"

	"github.com/cuihairu/croupier/services/api/internal/svc"
	"github.com/cuihairu/croupier/services/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AnalyticsFiltersUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAnalyticsFiltersUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AnalyticsFiltersUpdateLogic {
	return &AnalyticsFiltersUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AnalyticsFiltersUpdateLogic) AnalyticsFiltersUpdate(req *types.AnalyticsFiltersUpdateRequest) (*types.AnalyticsFiltersUpdateResponse, error) {
	if req == nil || strings.TrimSpace(req.GameId) == "" {
		return nil, errors.New("game_id required")
	}
	events := uniqueStrings(req.Events)
	payments := true
	if req.PaymentsEnabled != nil {
		payments = *req.PaymentsEnabled
	}
	sample := svc.DefaultSampleGlobal
	if req.SampleGlobal != nil {
		sample = clampSample(*req.SampleGlobal)
	}
	if err := l.svcCtx.UpdateAnalyticsFilter(req.GameId, req.Env, events, payments, sample); err != nil {
		return nil, err
	}
	return &types.AnalyticsFiltersUpdateResponse{Ok: true}, nil
}

func uniqueStrings(items []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(items))
	for _, v := range items {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

func clampSample(v int) int {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}
