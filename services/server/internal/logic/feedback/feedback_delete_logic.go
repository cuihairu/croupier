// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package feedback

import (
	"context"
	"errors"
	"strconv"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
)

type FeedbackDeleteLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 删除反馈
func NewFeedbackDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FeedbackDeleteLogic {
	return &FeedbackDeleteLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *FeedbackDeleteLogic) FeedbackDelete(req *types.FeedbackDeleteRequest) error {
	if l.svcCtx.FeedbackModel == nil {
		return errors.New("反馈模型未初始化")
	}
	if req == nil {
		return errors.New("请求体不能为空")
	}

	id, err := strconv.ParseUint(req.ID, 10, 64)
	if err != nil {
		return errors.New("反馈ID格式不正确")
	}

	if _, err := l.svcCtx.FeedbackModel.FindByID(l.ctx, uint(id)); err != nil {
		return err
	}

	return l.svcCtx.FeedbackModel.Delete(l.ctx, uint(id))
}
