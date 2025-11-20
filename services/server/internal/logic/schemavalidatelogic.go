package logic

import (
	"context"
	"errors"
	"os"

	"github.com/cuihairu/croupier/services/api/internal/svc"
	"github.com/cuihairu/croupier/services/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type SchemaValidateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSchemaValidateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SchemaValidateLogic {
	return &SchemaValidateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SchemaValidateLogic) SchemaValidate(req *types.SchemaValidateRequest) (*types.SchemaValidateResponse, error) {
	if req == nil || req.Id == "" {
		return nil, errors.New("schema id required")
	}
	dir, err := requireSchemaDir(l.svcCtx.SchemaDir())
	if err != nil {
		return nil, err
	}
	id := sanitizeSchemaID(req.Id)
	if id == "" {
		return nil, errors.New("invalid schema id")
	}
	if _, err := os.Stat(schemaFilePath(dir, id)); err != nil {
		return nil, err
	}
	return &types.SchemaValidateResponse{Valid: true, Errors: []string{}}, nil
}
