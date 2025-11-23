// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package analytics

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AnalyticsLevelsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取等级分析
func NewAnalyticsLevelsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AnalyticsLevelsLogic {
	return &AnalyticsLevelsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AnalyticsLevelsLogic) AnalyticsLevels(req *types.AnalyticsLevelsRequest) (resp *types.AnalyticsLevelsResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
