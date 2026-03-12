// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package openapi

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type FunctionOpenAPISpecLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取函数的 OpenAPI spec
func NewFunctionOpenAPISpecLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FunctionOpenAPISpecLogic {
	return &FunctionOpenAPISpecLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *FunctionOpenAPISpecLogic) FunctionOpenAPISpec(req *types.OpenAPISpecRequest) (resp *types.OpenAPISpecResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
