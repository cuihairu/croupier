// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package faq

import (
	"context"

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
	// todo: add your logic here and delete this line

	return
}
