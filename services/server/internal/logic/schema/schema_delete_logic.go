// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package schema

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
)

type SchemaDeleteLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 删除模式
func NewSchemaDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SchemaDeleteLogic {
	return &SchemaDeleteLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SchemaDeleteLogic) SchemaDelete(req *types.SchemaDeleteRequest) (*types.SchemaDeleteResponse, error) {
	if err := deleteSchema(l.svcCtx.Config, req.ID); err != nil {
		return nil, err
	}

	return &types.SchemaDeleteResponse{
		Code:    0,
		Message: "OK",
		Data: map[string]interface{}{
			"id": req.ID,
		},
	}, nil
}
