// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package schema

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type SchemaRawValidateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 原始模式验证
func NewSchemaRawValidateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SchemaRawValidateLogic {
	return &SchemaRawValidateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SchemaRawValidateLogic) SchemaRawValidate(req *types.SchemaRawValidateRequest) (resp *types.SchemaRawValidateResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
