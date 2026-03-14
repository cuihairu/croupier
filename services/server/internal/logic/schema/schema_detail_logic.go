// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package schema

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
)

type SchemaDetailLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取模式详情
func NewSchemaDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SchemaDetailLogic {
	return &SchemaDetailLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SchemaDetailLogic) SchemaDetail(req *types.SchemaDetailRequest) (*types.SchemaDetailResponse, error) {
	doc, err := loadSchema(l.svcCtx.Config, req.ID)
	if err != nil {
		return nil, err
	}

	return &types.SchemaDetailResponse{
		Code:    0,
		Message: "OK",
		Data:    doc.toMap(),
	}, nil
}
