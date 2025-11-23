// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package analytics

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AnalyticsBehaviorLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取行为分析
func NewAnalyticsBehaviorLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AnalyticsBehaviorLogic {
	return &AnalyticsBehaviorLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AnalyticsBehaviorLogic) AnalyticsBehavior(req *types.AnalyticsBehaviorRequest) (resp *types.AnalyticsBehaviorResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
