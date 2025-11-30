// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package analytics_retention

import (
	"context"
	"errors"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type RetentionLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取留存分析
func NewRetentionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RetentionLogic {
	return &RetentionLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RetentionLogic) Retention(req *types.RetentionRequest) (*types.RetentionResponse, error) {
	if l.svcCtx.RetentionModel == nil {
		return nil, errors.New("retention model unavailable")
	}
	if req == nil {
		return nil, errors.New("请求参数不能为空")
	}

	start, end, err := resolveRetentionRange(req.StartDate, req.EndDate)
	if err != nil {
		return nil, err
	}

	gameID := strings.TrimSpace(req.GameId)
	env := strings.TrimSpace(req.Env)
	cohortName := strings.TrimSpace(req.Cohort)

	cohorts, err := l.svcCtx.RetentionModel.ListCohorts(l.ctx, gameID, env, cohortName)
	if err != nil {
		return nil, err
	}

	items := make([]types.RetentionCohort, 0, len(cohorts))
	for _, cohort := range cohorts {
		if !start.IsZero() && cohort.WindowStart.Before(start) {
			continue
		}
		if !end.IsZero() && cohort.WindowStart.After(end) {
			continue
		}
		items = append(items, types.RetentionCohort{
			Cohort:    cohort.Cohort,
			Users:     cohort.Users,
			Retention: parseRetentionValues(cohort.Retention),
		})
	}

	return &types.RetentionResponse{
		Cohorts: items,
	}, nil
}
