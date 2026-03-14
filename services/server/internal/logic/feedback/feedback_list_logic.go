// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package feedback

import (
	"context"
	"errors"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/model"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

)

type FeedbackListLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取反馈列表
func NewFeedbackListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FeedbackListLogic {
	return &FeedbackListLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *FeedbackListLogic) FeedbackList(req *types.FeedbackListRequest) (resp *types.FeedbackListResponse, err error) {
	if l.svcCtx.FeedbackModel == nil {
		return nil, errors.New("反馈模型未初始化")
	}
	if req == nil {
		req = &types.FeedbackListRequest{}
	}

	opts := model.ListFeedbackOptions{
		PaginationOptions: model.PaginationOptions{
			Page:     req.Page,
			PageSize: req.PageSize,
		},
		Status:   strings.TrimSpace(req.Status),
		Category: strings.TrimSpace(req.Category),
		GameID:   strings.TrimSpace(req.GameId),
	}

	entries, total, err := l.svcCtx.FeedbackModel.List(l.ctx, opts)
	if err != nil {
		return nil, err
	}

	items := make([]types.Feedback, 0, len(entries))
	for i := range entries {
		items = append(items, utils.BuildFeedback(&entries[i]))
	}

	return &types.FeedbackListResponse{
		Items: items,
		Total: total,
		Page:  opts.Page,
		Size:  opts.PageSize,
	}, nil
}
