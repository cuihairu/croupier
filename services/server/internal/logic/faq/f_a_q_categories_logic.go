// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package faq

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type FAQCategoriesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取FAQ分类
func NewFAQCategoriesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FAQCategoriesLogic {
	return &FAQCategoriesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *FAQCategoriesLogic) FAQCategories(req *types.FAQCategoriesRequest) (resp *types.FAQCategoriesResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
