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

type SchemaUIConfigLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSchemaUIConfigLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SchemaUIConfigLogic {
	return &SchemaUIConfigLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SchemaUIConfigLogic) SchemaUIConfig(req *types.SchemaUIConfigRequest) (*types.SchemaUIConfigResponse, error) {
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
	path := uiSchemaFilePath(dir, id)
	var ui interface{}
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &ui)
	} else if schemaBytes, err := os.ReadFile(schemaFilePath(dir, id)); err == nil {
		var schema map[string]interface{}
		if json.Unmarshal(schemaBytes, &schema) == nil {
			ui = defaultUISchema(schema)
		}
	}
	return &types.SchemaUIConfigResponse{UIConfig: ui}, nil
}
