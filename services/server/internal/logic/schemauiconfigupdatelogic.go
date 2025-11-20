package logic

import (
	"context"
	"encoding/json"
	"errors"
	"os"

	"github.com/cuihairu/croupier/services/api/internal/svc"
	"github.com/cuihairu/croupier/services/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type SchemaUIConfigUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSchemaUIConfigUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SchemaUIConfigUpdateLogic {
	return &SchemaUIConfigUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SchemaUIConfigUpdateLogic) SchemaUIConfigUpdate(req *types.SchemaUIConfigUpdateRequest) (*types.SchemaUIConfigUpdateResponse, error) {
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
	data, err := json.MarshalIndent(req.UIConfig, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(uiSchemaFilePath(dir, id), data, 0o644); err != nil {
		return nil, err
	}
	return &types.SchemaUIConfigUpdateResponse{Ok: true}, nil
}
