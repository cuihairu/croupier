// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package support

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type SupportFAQUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新FAQ
func NewSupportFAQUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SupportFAQUpdateLogic {
	return &SupportFAQUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SupportFAQUpdateLogic) SupportFAQUpdate(req *types.SupportFAQUpdateRequest) (resp *types.SupportFAQUpdateResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
