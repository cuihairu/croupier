// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package analytics_behavior

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type BehaviorAdoptionBreakdownLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取采用率明细
func NewBehaviorAdoptionBreakdownLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BehaviorAdoptionBreakdownLogic {
	return &BehaviorAdoptionBreakdownLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *BehaviorAdoptionBreakdownLogic) BehaviorAdoptionBreakdown(req *types.BehaviorAdoptionBreakdownRequest) (resp *types.BehaviorAdoptionBreakdownResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
