// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package analytics_overview

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type OverviewLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取分析概览
func NewOverviewLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OverviewLogic {
	return &OverviewLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OverviewLogic) Overview(req *types.OverviewRequest) (resp *types.OverviewResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
