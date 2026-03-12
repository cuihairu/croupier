// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package analytics_retention

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type LevelsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取关卡分析
func NewLevelsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LevelsLogic {
	return &LevelsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *LevelsLogic) Levels(req *types.LevelsRequest) (resp *types.LevelsResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
