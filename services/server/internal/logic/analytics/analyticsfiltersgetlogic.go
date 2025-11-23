// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package analytics

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AnalyticsFiltersGetLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取分析过滤器
func NewAnalyticsFiltersGetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AnalyticsFiltersGetLogic {
	return &AnalyticsFiltersGetLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AnalyticsFiltersGetLogic) AnalyticsFiltersGet(req *types.AnalyticsFiltersGetRequest) (resp *types.AnalyticsFiltersGetResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
