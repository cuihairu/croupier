// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package analytics_overview

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type RealtimeSeriesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取实时序列数据
func NewRealtimeSeriesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RealtimeSeriesLogic {
	return &RealtimeSeriesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RealtimeSeriesLogic) RealtimeSeries(req *types.RealtimeSeriesRequest) (resp *types.RealtimeSeriesResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
