package logic

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type SchemasListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSchemasListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SchemasListLogic {
	return &SchemasListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SchemasListLogic) SchemasList(req *types.SchemasQuery) (*types.SchemasListResponse, error) {
	schemaDir, err := requireSchemaDir(l.svcCtx.SchemaDir())
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(schemaDir)
	if err != nil {
		return nil, err
	}
	var schemas []types.SchemaInfo
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".schema.json") {
			continue
		}
		id := strings.TrimSuffix(entry.Name(), ".schema.json")
		if req.Search != "" && !strings.Contains(strings.ToLower(id), strings.ToLower(req.Search)) {
			continue
		}
		path := filepath.Join(schemaDir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var schema map[string]interface{}
		_ = json.Unmarshal(data, &schema)
		category := ""
		if cat, ok := schema["category"].(string); ok {
			category = cat
		}
		if req.Category != "" && category != req.Category {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		schemas = append(schemas, types.SchemaInfo{
			Id:          id,
			Title:       schema["title"],
			Description: schema["description"],
			Category:    category,
			Type:        schema["type"],
			Version:     schema["version"],
			UpdatedAt:   info.ModTime().Format(time.RFC3339),
			Size:        int64(len(data)),
		})
	}
	sort.Slice(schemas, func(i, j int) bool {
		return schemas[i].Id < schemas[j].Id
	})
	return &types.SchemasListResponse{
		Schemas: schemas,
		Total:   int64(len(schemas)),
	}, nil
}
