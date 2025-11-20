package logic

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"time"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type SchemaDetailLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSchemaDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SchemaDetailLogic {
	return &SchemaDetailLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SchemaDetailLogic) SchemaDetail(req *types.SchemaDetailRequest) (*types.SchemaDetailResponse, error) {
	if req == nil || strings.TrimSpace(req.Id) == "" {
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
	schemaPath := schemaFilePath(dir, id)
	data, err := os.ReadFile(schemaPath)
	if err != nil {
		return nil, err
	}
	var schema interface{}
	if err := json.Unmarshal(data, &schema); err != nil {
		return nil, err
	}
	var ui interface{}
	if uiData, err := os.ReadFile(uiSchemaFilePath(dir, id)); err == nil {
		_ = json.Unmarshal(uiData, &ui)
	}
	info, err := os.Stat(schemaPath)
	if err != nil {
		return nil, err
	}
	return &types.SchemaDetailResponse{
		Id:        id,
		Schema:    schema,
		UISchema:  ui,
		UpdatedAt: info.ModTime().Format(time.RFC3339),
		Size:      int64(len(data)),
	}, nil
}
