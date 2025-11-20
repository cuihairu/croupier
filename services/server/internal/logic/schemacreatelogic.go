package logic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type SchemaCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSchemaCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SchemaCreateLogic {
	return &SchemaCreateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SchemaCreateLogic) SchemaCreate(req *types.SchemaCreateRequest) (*types.SchemaCreateResponse, error) {
	if req == nil {
		return nil, errors.New("request required")
	}
	if req.Schema == nil {
		return nil, errors.New("schema payload required")
	}
	if err := validateSchemaPayload(req.Schema); err != nil {
		return nil, err
	}
	dir, err := requireSchemaDir(l.svcCtx.SchemaDir())
	if err != nil {
		return nil, err
	}
	id, err := coerceSchemaID(req.Id, req.Schema)
	if err != nil {
		return nil, err
	}
	if err := ensureSchemaDirectory(dir); err != nil {
		return nil, err
	}
	path := schemaFilePath(dir, id)
	if _, err := os.Stat(path); err == nil {
		return nil, fmt.Errorf("schema already exists")
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	data, err := json.MarshalIndent(req.Schema, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return nil, err
	}
	if req.UIConfig != nil {
		uiPath := uiSchemaFilePath(dir, id)
		uiData, err := json.MarshalIndent(req.UIConfig, "", "  ")
		if err != nil {
			return nil, err
		}
		if err := os.WriteFile(uiPath, uiData, 0o644); err != nil {
			return nil, err
		}
	}
	return &types.SchemaCreateResponse{Ok: true, Id: id}, nil
}
