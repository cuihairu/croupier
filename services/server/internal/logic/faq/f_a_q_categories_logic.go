// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package faq

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

)

type FAQCategoriesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取FAQ分类
func NewFAQCategoriesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FAQCategoriesLogic {
	return &FAQCategoriesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *FAQCategoriesLogic) FAQCategories(req *types.FAQCategoriesRequest) (resp *types.FAQCategoriesResponse, err error) {
	cats, err := l.svcCtx.FAQModel.ListCategories(l.ctx)
	if err != nil {
		return nil, err
	}

	resp = &types.FAQCategoriesResponse{
		Items: make([]types.FAQCategory, 0, len(cats)),
	}
	for i := range cats {
		resp.Items = append(resp.Items, types.FAQCategory{
			Name:  cats[i].Name,
			Count: cats[i].Count,
		})
	}
	return resp, nil
}
