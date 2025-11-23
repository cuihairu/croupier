// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package xrender

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type UiSchemaLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取UI模式
func NewUiSchemaLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UiSchemaLogic {
	return &UiSchemaLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UiSchemaLogic) UiSchema(req *types.UISchemaRequest) (resp *types.UISchemaResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
