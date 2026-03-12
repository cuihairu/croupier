// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package analytics_behavior

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type BehaviorPathsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取行为路径
func NewBehaviorPathsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BehaviorPathsLogic {
	return &BehaviorPathsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *BehaviorPathsLogic) BehaviorPaths(req *types.BehaviorPathsRequest) (resp *types.BehaviorPathsResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
