// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package analytics

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AnalyticsBehaviorPathsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取行为路径
func NewAnalyticsBehaviorPathsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AnalyticsBehaviorPathsLogic {
	return &AnalyticsBehaviorPathsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AnalyticsBehaviorPathsLogic) AnalyticsBehaviorPaths(req *types.AnalyticsBehaviorPathsRequest) (resp *types.AnalyticsBehaviorPathsResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
