// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package faq

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/model"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type FAQCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 创建FAQ
func NewFAQCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FAQCreateLogic {
	return &FAQCreateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *FAQCreateLogic) FAQCreate(req *types.FAQCreateRequest) (resp *types.FAQDetailResponse, err error) {
	question, answer, category, err := sanitizeFAQInput(req.Question, req.Answer, req.Category)
	if err != nil {
		return nil, err
	}

	faq := &model.FAQ{
		Question: question,
		Answer:   answer,
		Category: category,
		Tags:     encodeTags(req.Tags),
		Visible:  req.Visible,
		Sort:     req.Sort,
	}

	if err := l.svcCtx.FAQModel.Create(l.ctx, faq); err != nil {
		return nil, err
	}

	return &types.FAQDetailResponse{FAQ: buildFAQResponse(faq)}, nil
}
