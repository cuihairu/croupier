// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package analytics_behavior

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type BehaviorAdoptionLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取功能采用率
func NewBehaviorAdoptionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BehaviorAdoptionLogic {
	return &BehaviorAdoptionLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *BehaviorAdoptionLogic) BehaviorAdoption(req *types.BehaviorAdoptionRequest) (resp *types.BehaviorAdoptionResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
