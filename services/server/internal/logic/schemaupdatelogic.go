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

type SchemaUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSchemaUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SchemaUpdateLogic {
	return &SchemaUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SchemaUpdateLogic) SchemaUpdate(req *types.SchemaUpdateRequest) (*types.SchemaUpdateResponse, error) {
	if req == nil || req.Id == "" {
		return nil, errors.New("schema id required")
	}
	if req.Schema == nil && req.UIConfig == nil {
		return nil, errors.New("no update payload provided")
	}
	dir, err := requireSchemaDir(l.svcCtx.SchemaDir())
	if err != nil {
		return nil, err
	}
	id := sanitizeSchemaID(req.Id)
	if id == "" {
		return nil, errors.New("invalid schema id")
	}
	schemaPath := schemaFilePath(dir, id)
	if _, err := os.Stat(schemaPath); err != nil {
		return nil, err
	}
	if req.Schema != nil {
		if err := validateSchemaPayload(req.Schema); err != nil {
			return nil, err
		}
		data, err := json.MarshalIndent(req.Schema, "", "  ")
		if err != nil {
			return nil, err
		}
		if err := os.WriteFile(schemaPath, data, 0o644); err != nil {
			return nil, err
		}
	}
	if req.UIConfig != nil {
		uiData, err := json.MarshalIndent(req.UIConfig, "", "  ")
		if err != nil {
			return nil, err
		}
		if err := os.WriteFile(uiSchemaFilePath(dir, id), uiData, 0o644); err != nil {
			return nil, err
		}
	}
	return &types.SchemaUpdateResponse{Ok: true}, nil
}
