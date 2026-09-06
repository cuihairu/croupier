// 补齐 openapi 包剩余可覆盖分支：UpdateSource 模型保存失败、ListSources
// 缺少 scope、DeleteBinding 删除失败、auditSourceEvent 写审计失败。
package openapi

import (
	"context"
	"errors"
	"testing"

	"github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func coverageFinalSpec(t *testing.T) map[string]interface{} {
	t.Helper()
	return map[string]interface{}{
		"openapi": "3.0.3",
		"info": map[string]interface{}{
			"title":   "Coverage API",
			"version": "1.0.0",
		},
		"paths": map[string]interface{}{
			"/players": map[string]interface{}{
				"get": map[string]interface{}{
					"operationId": "player.list",
					"x-resource":  "player",
					"x-operation": "list",
					"responses":   map[string]interface{}{"200": map[string]interface{}{"description": "OK"}},
				},
			},
		},
	}
}

// UpdateSource：模型 Save 失败（gorm update 回调注入错误）→ 透传错误。
func TestService_UpdateSource_ModelUpdateFails(t *testing.T) {
	t.Parallel()

	service, ctx := setupOpenAPITestServiceWithPermissions(t, "openapi_sources:read", "openapi_sources:write")
	created, err := service.CreateSource(ctx, &OpenAPISourceCreateRequest{Spec: rawSpec(t, coverageFinalSpec(t))})
	require.NoError(t, err)

	require.NoError(t, service.svcCtx.DB.Callback().Update().Before("gorm:update").Register("test/fail_openapi_source_update", func(tx *gorm.DB) {
		if _, ok := tx.Statement.Dest.(*model.OpenAPISource); ok {
			_ = tx.AddError(errors.New("forced update failure"))
		}
	}))
	t.Cleanup(func() { service.svcCtx.DB.Callback().Update().Remove("test/fail_openapi_source_update") })

	_, err = service.UpdateSource(ctx, &OpenAPISourceUpdateRequest{
		SourceID: created.Source.SourceID,
		Spec:     rawSpec(t, coverageFinalSpec(t)),
	})
	require.Error(t, err)
}

// ListSources：上下文缺少 game scope → requireScope 报错。
func TestService_ListSources_RequiresScope(t *testing.T) {
	t.Parallel()

	service, _ := setupOpenAPITestServiceWithPermissions(t, "openapi_sources:read")
	ctx := context.WithValue(context.Background(), "username", "openapi_tester")

	_, err := service.ListSources(ctx, &OpenAPISourceListRequest{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "X-Game-ID")
}

// DeleteBinding：绑定删除失败（gorm delete 回调注入错误）→ 透传错误。
func TestService_DeleteBinding_ModelDeleteFails(t *testing.T) {
	t.Parallel()

	service, ctx := setupOpenAPITestServiceWithPermissions(t, "openapi_sources:read", "openapi_sources:write")
	created, err := service.CreateSource(ctx, &OpenAPISourceCreateRequest{Spec: rawSpec(t, coverageFinalSpec(t))})
	require.NoError(t, err)
	_, err = service.CreateBinding(ctx, &OpenAPISourceBindingCreateRequest{
		SourceID:    created.Source.SourceID,
		OperationID: "player.list",
		Kind:        "provider",
		FunctionID:  "player.list",
	})
	require.NoError(t, err)

	require.NoError(t, service.svcCtx.DB.Callback().Delete().Before("gorm:delete").Register("test/fail_openapi_binding_delete", func(tx *gorm.DB) {
		if _, ok := tx.Statement.Dest.(*model.OpenAPISourceBinding); ok {
			_ = tx.AddError(errors.New("forced delete failure"))
		}
	}))
	t.Cleanup(func() { service.svcCtx.DB.Callback().Delete().Remove("test/fail_openapi_binding_delete") })

	_, err = service.DeleteBinding(ctx, &OpenAPISourceBindingDeleteRequest{
		SourceID:  created.Source.SourceID,
		BindingID: "player.list",
	})
	require.Error(t, err)
}

// failingCreateAuditStore 覆盖 Create 使审计写入失败，其余行为继承内存实现。
type failingCreateAuditStore struct {
	*audit.InMemoryAuditStore
}

func (f *failingCreateAuditStore) Create(record *audit.AuditRecord) error {
	return errors.New("audit store unavailable")
}

// auditSourceEvent：审计写入失败仅记录日志，不影响业务返回。
func TestService_AuditSourceEvent_LogFails(t *testing.T) {
	t.Parallel()

	service, ctx := setupOpenAPITestServiceWithPermissions(t, "openapi_sources:read", "openapi_sources:write")
	service.svcCtx.AuditService = audit.NewAuditService(&failingCreateAuditStore{audit.NewInMemoryAuditStore()}, nil)

	resp, err := service.CreateSource(ctx, &OpenAPISourceCreateRequest{Spec: rawSpec(t, coverageFinalSpec(t))})
	require.NoError(t, err)
	assert.NotEmpty(t, resp.Source.SourceID)
	assert.Equal(t, 1, resp.Source.Revision)
}
