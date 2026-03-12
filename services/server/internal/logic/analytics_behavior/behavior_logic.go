// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package analytics_behavior

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type BehaviorLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取行为分析
func NewBehaviorLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BehaviorLogic {
	return &BehaviorLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *BehaviorLogic) Behavior(req *types.BehaviorRequest) (resp *types.BehaviorResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
