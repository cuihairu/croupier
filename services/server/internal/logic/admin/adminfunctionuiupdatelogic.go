// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package admin

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AdminFunctionUiUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新函数UI配置
func NewAdminFunctionUiUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminFunctionUiUpdateLogic {
	return &AdminFunctionUiUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminFunctionUiUpdateLogic) AdminFunctionUiUpdate(req *types.AdminFunctionUIUpdateRequest) (resp *types.AdminFunctionUIUpdateResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
