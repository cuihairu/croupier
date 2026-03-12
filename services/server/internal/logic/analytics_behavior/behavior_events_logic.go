// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package analytics_behavior

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type BehaviorEventsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取行为事件
func NewBehaviorEventsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BehaviorEventsLogic {
	return &BehaviorEventsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *BehaviorEventsLogic) BehaviorEvents(req *types.BehaviorEventsRequest) (resp *types.BehaviorEventsResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
