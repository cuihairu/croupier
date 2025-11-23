// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package analytics

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AnalyticsBehaviorEventsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取行为事件
func NewAnalyticsBehaviorEventsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AnalyticsBehaviorEventsLogic {
	return &AnalyticsBehaviorEventsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AnalyticsBehaviorEventsLogic) AnalyticsBehaviorEvents(req *types.AnalyticsBehaviorEventsRequest) (resp *types.AnalyticsBehaviorEventsResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
