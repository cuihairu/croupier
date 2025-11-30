// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package faq

import (
	"context"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/model"
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
	opts := model.ListFAQOptions{
		PaginationOptions: model.PaginationOptions{
			Page:     req.Page,
			PageSize: req.PageSize,
		},
		Category: strings.TrimSpace(req.Category),
		Keyword:  strings.TrimSpace(req.Keyword),
		Visible:  req.Visible,
	}

	items, total, err := l.svcCtx.FAQModel.List(l.ctx, opts)
	if err != nil {
		return nil, err
	}

	resp = &types.FAQListResponse{
		Items: make([]types.FAQ, 0, len(items)),
		Total: total,
		Page:  opts.Page,
		Size:  opts.PageSize,
	}
	for i := range items {
		resp.Items = append(resp.Items, buildFAQResponse(&items[i]))
	}

	return resp, nil
}
