// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package schema

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type SchemaValidateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 验证模式数据
func NewSchemaValidateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SchemaValidateLogic {
	return &SchemaValidateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SchemaValidateLogic) SchemaValidate(req *types.SchemaValidateRequest) (resp *types.SchemaValidateResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
