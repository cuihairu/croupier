// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package analytics

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AnalyticsRetentionLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取留存分析
func NewAnalyticsRetentionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AnalyticsRetentionLogic {
	return &AnalyticsRetentionLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AnalyticsRetentionLogic) AnalyticsRetention(req *types.AnalyticsRetentionRequest) (resp *types.AnalyticsRetentionResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
