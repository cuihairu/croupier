// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package faq

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
)

type FAQDeleteLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 删除FAQ
func NewFAQDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FAQDeleteLogic {
	return &FAQDeleteLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *FAQDeleteLogic) FAQDelete(req *types.FAQDeleteRequest) error {
	id, err := utils.ParseUintID(req.ID, "FAQ ID")
	if err != nil {
		return err
	}
	return l.svcCtx.FAQModel.Delete(l.ctx, id)
}
