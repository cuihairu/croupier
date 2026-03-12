// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package schema

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type SchemaUiConfigLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取模式UI配置
func NewSchemaUiConfigLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SchemaUiConfigLogic {
	return &SchemaUiConfigLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SchemaUiConfigLogic) SchemaUiConfig(req *types.SchemaUIConfigRequest) (resp *types.SchemaUIConfigResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
