// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package analytics

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AnalyticsFiltersUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新分析过滤器
func NewAnalyticsFiltersUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AnalyticsFiltersUpdateLogic {
	return &AnalyticsFiltersUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AnalyticsFiltersUpdateLogic) AnalyticsFiltersUpdate(req *types.AnalyticsFiltersUpdateRequest) (resp *types.AnalyticsFiltersUpdateResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
