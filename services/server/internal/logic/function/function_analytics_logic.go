// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package function

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type FunctionAnalyticsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取函数分析数据
func NewFunctionAnalyticsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FunctionAnalyticsLogic {
	return &FunctionAnalyticsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *FunctionAnalyticsLogic) FunctionAnalytics(req *types.FunctionAnalyticsRequest) (resp *types.FunctionAnalyticsResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
