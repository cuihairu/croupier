// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package analytics_overview

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type FiltersGetLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取分析过滤器
func NewFiltersGetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FiltersGetLogic {
	return &FiltersGetLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *FiltersGetLogic) FiltersGet(req *types.FiltersGetRequest) (resp *types.FiltersGetResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
