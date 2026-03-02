// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package faq

import (
	"context"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/common/errorx"
	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type FAQUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新FAQ
func NewFAQUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FAQUpdateLogic {
	return &FAQUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *FAQUpdateLogic) FAQUpdate(req *types.FAQUpdateRequest) (resp *types.FAQDetailResponse, err error) {
	id, err := utils.ParseUintID(req.ID, "FAQ ID")
	if err != nil {
		return nil, err
	}

	updates := make(map[string]interface{})
	if v := strings.TrimSpace(req.Question); v != "" {
		updates["question"] = v
	}
	if v := strings.TrimSpace(req.Answer); v != "" {
		updates["answer"] = v
	}
	if v := strings.TrimSpace(req.Category); v != "" {
		updates["category"] = v
	}
	if req.Tags != nil {
		updates["tags"] = encodeTags(req.Tags)
	}
	if req.Visible != nil {
		updates["visible"] = *req.Visible
	}
	if req.Sort != nil {
		updates["sort"] = *req.Sort
	}

	if len(updates) == 0 {
		return nil, errorx.NewBadRequest("请提供需要更新的字段")
	}

	if err := l.svcCtx.FAQModel.Update(l.ctx, id, updates); err != nil {
		return nil, err
	}

	faq, err := l.svcCtx.FAQModel.FindOne(l.ctx, id)
	if err != nil {
		return nil, err
	}

	return &types.FAQDetailResponse{FAQ: buildFAQResponse(faq)}, nil
}
