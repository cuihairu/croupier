// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package admin

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AdminPasswordResetLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 重置管理员密码
func NewAdminPasswordResetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminPasswordResetLogic {
	return &AdminPasswordResetLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminPasswordResetLogic) AdminPasswordReset(req *types.AdminPasswordResetRequest) error {
	// todo: add your logic here and delete this line

	return nil
}
