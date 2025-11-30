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

	"github.com/zeromicro/go-zero/core/logx"
)

type FeedbackCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 创建反馈
func NewFeedbackCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FeedbackCreateLogic {
	return &FeedbackCreateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *FeedbackCreateLogic) FeedbackCreate(req *types.FeedbackCreateRequest) (resp *types.FeedbackDetailResponse, err error) {
	if l.svcCtx.FeedbackModel == nil {
		return nil, errors.New("反馈模型未初始化")
	}
	if req == nil {
		return nil, errors.New("请求体不能为空")
	}

	contact := strings.TrimSpace(req.Contact)
	if contact == "" {
		return nil, errors.New("联系方式不能为空")
	}

	content := strings.TrimSpace(req.Content)
	if content == "" {
		return nil, errors.New("反馈内容不能为空")
	}

	category := strings.TrimSpace(req.Category)
	if category == "" {
		return nil, errors.New("反馈分类不能为空")
	}

	feedback := &model.Feedback{
		PlayerID: strings.TrimSpace(req.PlayerId),
		Contact:  contact,
		Content:  content,
		Category: category,
		Priority: "normal",
		Status:   "open",
		Rating:   utils.NormalizeFeedbackRating(req.Rating),
		Attach:   strings.TrimSpace(req.Attach),
		GameID:   strings.TrimSpace(req.GameId),
		Env:      strings.TrimSpace(req.Env),
	}

	if err := l.svcCtx.FeedbackModel.Create(l.ctx, feedback); err != nil {
		return nil, err
	}

	return &types.FeedbackDetailResponse{
		Feedback: utils.BuildFeedback(feedback),
	}, nil
}
