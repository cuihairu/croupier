// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package analytics_retention

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type RetentionLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取留存分析
func NewRetentionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RetentionLogic {
	return &RetentionLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RetentionLogic) Retention(req *types.RetentionRequest) (resp *types.RetentionResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
