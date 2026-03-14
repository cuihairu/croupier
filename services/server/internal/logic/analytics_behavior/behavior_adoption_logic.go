// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package analytics_behavior

import (
	"context"
	"errors"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
)

type BehaviorAdoptionLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取功能采用率
func NewBehaviorAdoptionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BehaviorAdoptionLogic {
	return &BehaviorAdoptionLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *BehaviorAdoptionLogic) BehaviorAdoption(req *types.BehaviorAdoptionRequest) (resp *types.BehaviorAdoptionResponse, err error) {
	if l.svcCtx.BehaviorModel == nil {
		return nil, errors.New("behavior analytics unavailable")
	}
	if req == nil {
		return nil, errors.New("缺少请求参数")
	}

	gameID := strings.TrimSpace(req.GameId)
	env := strings.TrimSpace(req.Env)
	items, err := l.svcCtx.BehaviorModel.ListFeatureAdoptions(l.ctx, gameID, env)
	if err != nil {
		return nil, err
	}

	features := make([]types.FeatureAdoption, 0, len(items))
	for _, item := range items {
		if req.Feature != "" && !strings.EqualFold(item.Feature, req.Feature) {
			continue
		}
		features = append(features, types.FeatureAdoption{
			Feature:      item.Feature,
			Users:        item.Users,
			AdoptionRate: item.AdoptionRate,
			Frequency:    item.Frequency,
		})
	}

	return &types.BehaviorAdoptionResponse{
		Features: features,
	}, nil
}
