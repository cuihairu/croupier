// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package schema

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
)

type SchemaRawValidateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 原始模式验证
func NewSchemaRawValidateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SchemaRawValidateLogic {
	return &SchemaRawValidateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SchemaRawValidateLogic) SchemaRawValidate(req *types.SchemaRawValidateRequest) (*types.SchemaRawValidateResponse, error) {
	if err := validateSchemaDefinition(req.Schema); err != nil {
		return nil, err
	}

	valid, issues, err := validatePayloadAgainst(req.Schema, req.Data)
	if err != nil {
		return nil, err
	}

	return &types.SchemaRawValidateResponse{
		Code:    0,
		Message: "OK",
		Data: map[string]interface{}{
			"valid":  valid,
			"errors": issues,
		},
	}, nil
}
