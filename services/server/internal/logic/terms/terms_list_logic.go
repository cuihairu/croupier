// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package terms

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

)

type TermsListLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取术语列表
func NewTermsListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TermsListLogic {
	return &TermsListLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *TermsListLogic) TermsList(req *types.TermsListRequest) (resp *types.TermsListResponse, err error) {
	// 查询术语字典
	terms, err := l.svcCtx.TermDictModel.List(l.ctx, req.Domain)
	if err != nil {
		return nil, err
	}

	// 转换为响应格式
	items := make([]types.TermItem, 0, len(terms))
	for _, term := range terms {
		items = append(items, types.TermItem{
			Id:        int64(term.ID),
			Domain:    term.Domain,
			TermKey:   term.TermKey,
			Alias:     term.Alias,
			DisplayZh: term.DisplayZh,
			DisplayEn: term.DisplayEn,
			Order:     int64(term.SortOrder),
		})
	}

	return &types.TermsListResponse{
		Items: items,
	}, nil
}
