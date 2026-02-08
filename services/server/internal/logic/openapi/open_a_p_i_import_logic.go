// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package openapi

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/zeromicro/go-zero/core/logx"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
)

type OpenAPIImportLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 导入 OpenAPI spec
func NewOpenAPIImportLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpenAPIImportLogic {
	return &OpenAPIImportLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpenAPIImportLogic) OpenAPIImport(req *types.OpenAPIImportRequest) (resp *types.OpenAPIImportResponse, err error) {
	// 参数验证
	if req.Spec == nil {
		return nil, fmt.Errorf("spec is required")
	}

	// 将 interface{} 转换为 OpenAPI 文档
	specBytes, err := json.Marshal(req.Spec)
	if err != nil {
		l.Errorf("failed to marshal spec: %v", err)
		return nil, err
	}

	// 加载 OpenAPI 文档
	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromData(specBytes)
	if err != nil {
		l.Errorf("failed to load OpenAPI spec: %v", err)
		return &types.OpenAPIImportResponse{
			Imported: 0,
			Failed:   []string{err.Error()},
		}, nil
	}

	// 验证文档
	if err := doc.Validate(loader.Context); err != nil {
		l.Errorf("invalid OpenAPI spec: %v", err)
		return &types.OpenAPIImportResponse{
			Imported: 0,
			Failed:   []string{err.Error()},
		}, nil
	}

	// 提取所有 operations 并导入到 registry
	imported := 0
	failed := []string{}

	for path, pathItem := range doc.Paths.Map() {
		if pathItem == nil {
			continue
		}

		// 处理所有 HTTP 方法的操作
		operations := map[string]*openapi3.Operation{
			"GET":    pathItem.Get,
			"POST":   pathItem.Post,
			"PUT":    pathItem.Put,
			"PATCH":  pathItem.Patch,
			"DELETE": pathItem.Delete,
		}

		for method, op := range operations {
			if op == nil {
				continue
			}

			// 生成 function ID: {method}{path} (如 getUsers, createUser)
			funcID := method + path

			// 存储到 registry
			if err := l.svcCtx.RegistryStore.UpsertOpenAPI(funcID, op); err != nil {
				l.Errorf("failed to upsert operation '%s': %v", funcID, err)
				failed = append(failed, funcID+": "+err.Error())
			} else {
				imported++
			}
		}
	}

	l.Infof("OpenAPI import completed: %d imported, %d failed", imported, len(failed))

	return &types.OpenAPIImportResponse{
		Imported: imported,
		Failed:   failed,
	}, nil
}
