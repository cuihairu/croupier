package logic

import (
	"context"
	"errors"
	"strings"

	validation "github.com/cuihairu/croupier/internal/validation"
	"github.com/cuihairu/croupier/services/api/internal/svc"
	"github.com/cuihairu/croupier/services/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type SchemaRawValidateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSchemaRawValidateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SchemaRawValidateLogic {
	return &SchemaRawValidateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SchemaRawValidateLogic) SchemaRawValidate(req *types.SchemaRawValidateRequest) (*types.SchemaRawValidateResponse, error) {
	if req == nil || req.Schema == nil {
		return nil, errors.New("schema payload required")
	}
	errorsList := validation.ValidateEntityDefinition(map[string]any{
		"id":     "temp",
		"type":   "entity",
		"schema": req.Schema,
	})
	var schemaErrors []string
	for _, item := range errorsList {
		if strings.Contains(item, "schema") {
			schemaErrors = append(schemaErrors, item)
		}
	}
	return &types.SchemaRawValidateResponse{
		Valid:  len(schemaErrors) == 0,
		Errors: schemaErrors,
	}, nil
}
