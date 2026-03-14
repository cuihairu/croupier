// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package schema

import (
	"context"
	"time"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

)

type SchemaUpdateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新模式
func NewSchemaUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SchemaUpdateLogic {
	return &SchemaUpdateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SchemaUpdateLogic) SchemaUpdate(req *types.SchemaUpdateRequest) (*types.SchemaUpdateResponse, error) {
	if err := validateSchemaDefinition(req.Schema); err != nil {
		return nil, err
	}

	doc, err := loadSchema(l.svcCtx.Config, req.ID)
	if err != nil {
		return nil, err
	}

	doc.Schema = req.Schema
	doc.UpdatedAt = time.Now()

	if err := saveSchema(l.svcCtx.Config, doc); err != nil {
		return nil, err
	}

	return &types.SchemaUpdateResponse{
		Code:    0,
		Message: "OK",
		Data:    doc.toMap(),
	}, nil
}
