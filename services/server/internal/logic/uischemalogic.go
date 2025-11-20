package logic

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type UISchemaLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUISchemaLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UISchemaLogic {
	return &UISchemaLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UISchemaLogic) UISchema(req *types.UISchemaRequest) (*types.UISchemaResponse, error) {
	if req == nil || strings.TrimSpace(req.Id) == "" {
		return nil, errors.New("id required")
	}
	dir, err := requireSchemaDir(l.svcCtx.SchemaDir())
	if err != nil {
		return nil, err
	}
	id := sanitizeSchemaID(req.Id)
	if id == "" {
		return nil, errors.New("invalid schema id")
	}
	var schema interface{}
	if schemaBytes, err := os.ReadFile(schemaFilePath(dir, id)); err == nil {
		_ = json.Unmarshal(schemaBytes, &schema)
	}
	var ui interface{}
	if uiBytes, err := os.ReadFile(uiSchemaFilePath(dir, id)); err == nil {
		_ = json.Unmarshal(uiBytes, &ui)
	}
	return &types.UISchemaResponse{
		Schema:   schema,
		UISchema: ui,
	}, nil
}
