package logic

import (
	"context"
	"errors"
	"os"

	"github.com/cuihairu/croupier/services/api/internal/svc"
	"github.com/cuihairu/croupier/services/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type SchemaDeleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSchemaDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SchemaDeleteLogic {
	return &SchemaDeleteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SchemaDeleteLogic) SchemaDelete(req *types.SchemaDeleteRequest) (*types.SchemaDeleteResponse, error) {
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
	path := schemaFilePath(dir, id)
	if err := os.Remove(path); err != nil {
		return nil, err
	}
	_ = os.Remove(uiSchemaFilePath(dir, id))
	return &types.SchemaDeleteResponse{Ok: true}, nil
}
