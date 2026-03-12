// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package analytics_overview

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type FiltersUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新分析过滤器
func NewFiltersUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FiltersUpdateLogic {
	return &FiltersUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *FiltersUpdateLogic) FiltersUpdate(req *types.FiltersUpdateRequest) (resp *types.FiltersGetResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
