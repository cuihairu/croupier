// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package support

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type SupportFAQCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 创建FAQ
func NewSupportFAQCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SupportFAQCreateLogic {
	return &SupportFAQCreateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SupportFAQCreateLogic) SupportFAQCreate(req *types.SupportFAQCreateRequest) (resp *types.SupportFAQCreateResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
