package logic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/cuihairu/croupier/services/api/internal/svc"
	"github.com/cuihairu/croupier/services/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type XRenderGenerateSchemaLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewXRenderGenerateSchemaLogic(ctx context.Context, svcCtx *svc.ServiceContext) *XRenderGenerateSchemaLogic {
	return &XRenderGenerateSchemaLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *XRenderGenerateSchemaLogic) XRenderGenerateSchema(req *types.XRenderGenerateRequest) (*types.XRenderGenerateResponse, error) {
	if req == nil {
		return nil, errors.New("request required")
	}
	if strings.TrimSpace(req.SchemaId) == "" {
		return nil, errors.New("schema_id required")
	}
	if len(req.Components) == 0 {
		return nil, errors.New("components required")
	}
	dir, err := requireSchemaDir(l.svcCtx.SchemaDir())
	if err != nil {
		return nil, err
	}
	id := sanitizeSchemaID(req.SchemaId)
	if id == "" || id != req.SchemaId {
		return nil, errors.New("invalid schema id")
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
	schema := map[string]interface{}{
		"$schema":     "http://json-schema.org/draft-07/schema#",
		"type":        "object",
		"title":       id,
		"description": "Generated from x-render components",
		"properties":  map[string]interface{}{},
		"required":    []string{},
	}
	uiSchema := map[string]interface{}{
		"type":        "object",
		"displayType": "column",
		"properties":  map[string]interface{}{},
	}
	generateSchemaFromComponents(schema, uiSchema, req.Components)
	schemaBytes, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, schemaBytes, 0o644); err != nil {
		return nil, err
	}
	uiBytes, err := json.MarshalIndent(uiSchema, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(uiSchemaFilePath(dir, id), uiBytes, 0o644); err != nil {
		return nil, err
	}
	return &types.XRenderGenerateResponse{
		Id:       id,
		Schema:   schema,
		UISchema: uiSchema,
	}, nil
}
