// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package analytics

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AnalyticsLevelsEpisodesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取关卡分析
func NewAnalyticsLevelsEpisodesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AnalyticsLevelsEpisodesLogic {
	return &AnalyticsLevelsEpisodesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AnalyticsLevelsEpisodesLogic) AnalyticsLevelsEpisodes(req *types.AnalyticsLevelsEpisodesRequest) (resp *types.AnalyticsLevelsEpisodesResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
