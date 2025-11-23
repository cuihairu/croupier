// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package support

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type SupportFAQListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取FAQ列表
func NewSupportFAQListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SupportFAQListLogic {
	return &SupportFAQListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SupportFAQListLogic) SupportFAQList(req *types.SupportFAQListRequest) (resp *types.SupportFAQListResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
