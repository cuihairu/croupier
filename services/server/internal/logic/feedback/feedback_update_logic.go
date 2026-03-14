// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package feedback

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
)

type FeedbackUpdateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新反馈
func NewFeedbackUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FeedbackUpdateLogic {
	return &FeedbackUpdateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *FeedbackUpdateLogic) FeedbackUpdate(req *types.FeedbackUpdateRequest) (resp *types.FeedbackDetailResponse, err error) {
	if l.svcCtx.FeedbackModel == nil {
		return nil, errors.New("反馈模型未初始化")
	}
	if req == nil {
		return nil, errors.New("请求体不能为空")
	}

	id, err := strconv.ParseUint(req.ID, 10, 64)
	if err != nil {
		return nil, errors.New("反馈ID格式不正确")
	}

	updates := map[string]interface{}{}
	if status := strings.TrimSpace(req.Status); status != "" {
		updates["status"] = status
	}
	if priority := strings.TrimSpace(req.Priority); priority != "" {
		updates["priority"] = priority
	}
	if reply := strings.TrimSpace(req.Reply); reply != "" {
		updates["reply"] = reply
	} else if req.Reply != "" {
		// 客户端显式传入空字符串时清除回复
		updates["reply"] = ""
	}

	if len(updates) == 0 {
		return nil, errors.New("没有需要更新的字段")
	}

	if err := l.svcCtx.FeedbackModel.Update(l.ctx, uint(id), updates); err != nil {
		return nil, err
	}

	updated, err := l.svcCtx.FeedbackModel.FindByID(l.ctx, uint(id))
	if err != nil {
		return nil, err
	}

	return &types.FeedbackDetailResponse{
		Feedback: utils.BuildFeedback(updated),
	}, nil
}
