// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package analytics

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AnalyticsRealtimeSeriesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取实时序列数据
func NewAnalyticsRealtimeSeriesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AnalyticsRealtimeSeriesLogic {
	return &AnalyticsRealtimeSeriesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AnalyticsRealtimeSeriesLogic) AnalyticsRealtimeSeries(req *types.AnalyticsRealtimeSeriesRequest) (resp *types.AnalyticsRealtimeSeriesResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
