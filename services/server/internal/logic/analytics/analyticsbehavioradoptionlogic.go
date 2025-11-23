// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package analytics

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AnalyticsBehaviorAdoptionLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取功能采用率
func NewAnalyticsBehaviorAdoptionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AnalyticsBehaviorAdoptionLogic {
	return &AnalyticsBehaviorAdoptionLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AnalyticsBehaviorAdoptionLogic) AnalyticsBehaviorAdoption(req *types.AnalyticsBehaviorAdoptionRequest) (resp *types.AnalyticsBehaviorAdoptionResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
