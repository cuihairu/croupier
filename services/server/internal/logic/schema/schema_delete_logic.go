// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package schema

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type SchemaDeleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 删除模式
func NewSchemaDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SchemaDeleteLogic {
	return &SchemaDeleteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SchemaDeleteLogic) SchemaDelete(req *types.SchemaDeleteRequest) (resp *types.SchemaDeleteResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
