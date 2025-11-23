// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package support

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type SupportFAQDeleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 删除FAQ
func NewSupportFAQDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SupportFAQDeleteLogic {
	return &SupportFAQDeleteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SupportFAQDeleteLogic) SupportFAQDelete(req *types.SupportFAQDeleteRequest) (resp *types.SupportFAQDeleteResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
