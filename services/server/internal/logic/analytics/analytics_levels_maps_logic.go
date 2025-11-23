// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package analytics

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AnalyticsLevelsMapsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取地图分析
func NewAnalyticsLevelsMapsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AnalyticsLevelsMapsLogic {
	return &AnalyticsLevelsMapsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AnalyticsLevelsMapsLogic) AnalyticsLevelsMaps(req *types.AnalyticsLevelsMapsRequest) (resp *types.AnalyticsLevelsMapsResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
