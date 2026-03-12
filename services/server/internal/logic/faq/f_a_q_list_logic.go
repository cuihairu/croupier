// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package faq

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type FAQListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取FAQ列表
func NewFAQListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FAQListLogic {
	return &FAQListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *FAQListLogic) FAQList(req *types.FAQListRequest) (resp *types.FAQListResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
