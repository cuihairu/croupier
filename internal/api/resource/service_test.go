package resource

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/cache"
	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/model"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	dashboardservice "github.com/cuihairu/croupier/internal/service"
	"github.com/cuihairu/croupier/internal/svc"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestServiceListUsesPersistentFunctionContracts(t *testing.T) {
	svcCtx, ctx := newResourceTestServiceContext(t, reg.NewStore(), "resources:read")
	contractService := dashboardservice.NewContractService(svcCtx.DB)
	for _, meta := range []dashboardservice.FunctionMetaInput{
		{
			ID:           "player.list",
			Version:      "1.0.0",
			Enabled:      true,
			Summary:      "List players",
			Description:  "List player accounts",
			InputSchema:  `{"type":"object","properties":{"keyword":{"type":"string"}}}`,
			OutputSchema: `{"type":"object","properties":{"items":{"type":"array","items":{"type":"object","properties":{"id":{"type":"string"},"name":{"type":"string"}}}},"total":{"type":"integer"}}}`,
			Resource:     "player",
			Risk:         "safe",
			Operation:    "list",
			Capability:   "collection_query",
			Execution:    "sync",
			Permission:   "player:list",
			Tags:         []string{"player"},
		},
		{
			ID:           "player.ban",
			Version:      "1.0.0",
			Enabled:      true,
			Summary:      "Ban player",
			Description:  "Ban a player account",
			InputSchema:  `{"type":"object","properties":{"player_id":{"type":"string"}}}`,
			OutputSchema: `{"type":"object","properties":{"success":{"type":"boolean"}}}`,
			Resource:     "player",
			Risk:         "danger",
			Operation:    "ban",
			Capability:   "action",
			Execution:    "sync",
			Permission:   "player:ban",
			Tags:         []string{"player", "moderation"},
		},
	} {
		require.NoError(t, contractService.RebuildContractFromFunctionMeta(ctx, "demo-game", "development", "sdk", meta))
	}
	require.NoError(t, contractService.RebuildResourceCapability(ctx, "demo-game", "development", "player"))

	resp, err := NewService(svcCtx).List(ctx, &ResourceListRequest{})
	require.NoError(t, err)
	require.Len(t, resp.Items, 1)

	player := resp.Items[0]
	assert.Equal(t, "player", player.Key)
	assert.Equal(t, "Player", player.Labels["zh-CN"])
	assert.Equal(t, "player", player.Category.Key)
	assert.Equal(t, "Player", player.Category.Labels["zh-CN"])
	require.Len(t, player.Operations, 2)

	ops := map[string]spec.OperationSpec{}
	for _, op := range player.Operations {
		ops[op.FunctionID] = op
	}

	listOp := ops["player.list"]
	assert.Equal(t, "list", listOp.Operation)
	assert.Equal(t, spec.CapabilityCollectionQuery, listOp.Capability)
	assert.Equal(t, spec.FunctionExecutionSync, listOp.Execution)
	assert.Equal(t, "player:list", listOp.Permission)
	assert.Empty(t, listOp.Diagnostics)

	banOp := ops["player.ban"]
	assert.Equal(t, "ban", banOp.Operation)
	assert.Equal(t, spec.CapabilityAction, banOp.Capability)
	assert.Equal(t, spec.RiskDanger, banOp.Risk)
	assert.Equal(t, "player:ban", banOp.Permission)
	assert.Empty(t, banOp.Diagnostics)
}

func TestServiceListDoesNotInterpretOpenAPISourceBindingsAtReadTime(t *testing.T) {
	svcCtx, ctx := newResourceTestServiceContext(t, reg.NewStore(), "resources:read")
	source := openAPISourceModel(
		t,
		"analytics-source",
		"analytics.retention",
		"analytics",
		"retention",
		"report",
		"sync",
		openAPISourceDocument(t, "analytics.retention", "analytics", "retention", "report", "sync"),
	)
	require.NoError(t, svcCtx.OpenAPISourceModel.Create(ctx, source))
	require.NoError(t, svcCtx.OpenAPISourceBindingModel.Upsert(ctx, &model.OpenAPISourceBinding{
		GameID:      "demo-game",
		Env:         "development",
		SourceID:    source.SourceID,
		BindingID:   "analytics.retention",
		OperationID: "analytics.retention",
		Kind:        "provider",
		FunctionID:  "analytics.retention",
	}))

	resp, err := NewService(svcCtx).List(ctx, &ResourceListRequest{})
	require.NoError(t, err)
	assert.Empty(t, resp.Items)
}

func TestServiceListIgnoresUnboundOpenAPISource(t *testing.T) {
	store := reg.NewStore()
	svcCtx, ctx := newResourceTestServiceContext(t, store, "resources:read")
	source := openAPISourceModel(
		t,
		"mail-source",
		"mail.send",
		"mail",
		"send",
		"action",
		"sync",
		openAPISourceDocument(t, "mail.send", "mail", "send", "action", "sync"),
	)
	require.NoError(t, svcCtx.OpenAPISourceModel.Create(ctx, source))

	resp, err := NewService(svcCtx).List(ctx, &ResourceListRequest{})
	require.NoError(t, err)
	assert.Empty(t, resp.Items)
}

func openAPISourceDocument(t *testing.T, operationID string, resource string, operation string, capability string, execution string) json.RawMessage {
	t.Helper()
	doc := map[string]interface{}{
		"openapi": "3.0.3",
		"info": map[string]interface{}{
			"title":   "Source API",
			"version": "1.0.0",
		},
		"paths": map[string]interface{}{
			"/operation": map[string]interface{}{
				"post": map[string]interface{}{
					"operationId":  operationID,
					"summary":      operationID,
					"x-resource":   resource,
					"x-operation":  operation,
					"x-capability": capability,
					"x-execution":  execution,
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"type": "object",
									"properties": map[string]interface{}{
										"page":     map[string]interface{}{"type": "integer"},
										"pageSize": map[string]interface{}{"type": "integer"},
									},
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "OK",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"items": map[string]interface{}{
												"type":  "array",
												"items": map[string]interface{}{"type": "object"},
											},
											"total": map[string]interface{}{"type": "integer"},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	raw, err := json.Marshal(doc)
	require.NoError(t, err)
	return raw
}

func openAPISourceModel(
	t *testing.T,
	sourceID string,
	operationID string,
	resource string,
	operation string,
	capability string,
	execution string,
	raw json.RawMessage,
) *model.OpenAPISource {
	t.Helper()
	source := &model.OpenAPISource{
		GameID:         "demo-game",
		Env:            "development",
		SourceID:       sourceID,
		Name:           sourceID,
		Revision:       1,
		Format:         "json",
		OpenAPIVersion: "3.0.3",
		InfoTitle:      "Source API",
		InfoVersion:    "1.0.0",
		ContentHash:    "test-hash-" + sourceID,
	}
	source.SetSpec(raw)
	require.NoError(t, source.SetOperations([]map[string]interface{}{
		{
			"operationId": operationID,
			"summary":     operationID,
			"resource":    resource,
			"operation":   operation,
			"capability":  capability,
			"execution":   execution,
		},
	}))
	require.NoError(t, source.SetDiagnostics([]spec.Diagnostic{}))
	return source
}

func TestServiceListRequiresResourcePermission(t *testing.T) {
	store := reg.NewStore()
	svcCtx, ctx := newResourceTestServiceContext(t, store)

	_, err := NewService(svcCtx).List(ctx, &ResourceListRequest{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "无权查看资源")
}

func newResourceTestServiceContext(t *testing.T, store *reg.Store, permissions ...string) (*svc.ServiceContext, context.Context) {
	t.Helper()

	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)

	admin := model.Admin{Username: "resource_tester", Status: 1, PasswordHash: "test"}
	require.NoError(t, db.Create(&admin).Error)

	role := model.Role{Name: "resource_tester_role", Description: "resource tester"}
	require.NoError(t, db.Create(&role).Error)
	require.NoError(t, db.Create(&model.AdminRole{AdminID: admin.ID, RoleID: role.ID}).Error)

	for _, permissionID := range permissions {
		permissionID = strings.TrimSpace(permissionID)
		if permissionID == "" {
			continue
		}
		permission := model.Permission{
			ID:       permissionID,
			Name:     permissionID,
			Resource: strings.SplitN(permissionID, ":", 2)[0],
			Action:   "read",
			Category: "dashboard",
		}
		require.NoError(t, db.Where("id = ?", permission.ID).FirstOrCreate(&permission).Error)
		require.NoError(t, db.Create(&model.RolePermission{RoleID: role.ID, PermissionID: permissionID}).Error)
	}

	nullCache := cache.NewNullCache()
	svcCtx := &svc.ServiceContext{
		DB:                        db,
		AdminModel:                model.NewAdminModel(db),
		RoleModel:                 model.NewRoleModel(db),
		PermissionModel:           model.NewPermissionModel(db),
		RegistryStore:             store,
		FunctionModel:             model.NewFunctionModel(db),
		PageSpecModel:             model.NewPageSpecModel(db),
		PublishedPageSpecModel:    model.NewPublishedPageSpecModel(db),
		PageVersionModel:          model.NewPageVersionModel(db),
		OpenAPISourceModel:        model.NewOpenAPISourceModel(db),
		OpenAPISourceBindingModel: model.NewOpenAPISourceBindingModel(db),
		AuditService:              audit.NewAuditService(audit.NewInMemoryAuditStore(), nil),
		Cache:                     nullCache,
		CacheHelper:               cache.NewCacheHelper(nullCache),
	}
	ctx := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "demo-game", Env: "development"})
	ctx = context.WithValue(ctx, "username", admin.Username)
	return svcCtx, ctx
}

func TestServiceDetail_UsesPersistentFunctionContracts(t *testing.T) {
	svcCtx, ctx := newResourceTestServiceContext(t, reg.NewStore(), "resources:read")
	contractService := dashboardservice.NewContractService(svcCtx.DB)
	require.NoError(t, contractService.RebuildContractFromFunctionMeta(ctx, "demo-game", "development", "sdk", dashboardservice.FunctionMetaInput{
		ID:         "player.list",
		Version:    "1.0.0",
		Enabled:    true,
		Summary:    "List players",
		Resource:   "player",
		Risk:       "safe",
		Operation:  "list",
		Capability: "collection_query",
		Execution:  "sync",
		Permission: "player:list",
	}))
	require.NoError(t, contractService.RebuildResourceCapability(ctx, "demo-game", "development", "player"))

	resp, err := NewService(svcCtx).Detail(ctx, &ResourceDetailRequest{ResourceKey: "player"})
	require.NoError(t, err)
	assert.Equal(t, "player", resp.Resource.Key)
}

func TestServiceDetail_NotFound(t *testing.T) {
	svcCtx, ctx := newResourceTestServiceContext(t, reg.NewStore(), "resources:read")
	_, err := NewService(svcCtx).Detail(ctx, &ResourceDetailRequest{ResourceKey: "nonexistent"})
	assert.Error(t, err)
}

func TestServiceOperations_UsesPersistentFunctionContracts(t *testing.T) {
	svcCtx, ctx := newResourceTestServiceContext(t, reg.NewStore(), "resources:read")
	contractService := dashboardservice.NewContractService(svcCtx.DB)
	require.NoError(t, contractService.RebuildContractFromFunctionMeta(ctx, "demo-game", "development", "sdk", dashboardservice.FunctionMetaInput{
		ID:         "player.list",
		Version:    "1.0.0",
		Enabled:    true,
		Resource:   "player",
		Risk:       "safe",
		Operation:  "list",
		Capability: "collection_query",
		Execution:  "sync",
		Permission: "player:list",
	}))
	require.NoError(t, contractService.RebuildResourceCapability(ctx, "demo-game", "development", "player"))

	resp, err := NewService(svcCtx).Operations(ctx, &ResourceOperationsRequest{ResourceKey: "player"})
	require.NoError(t, err)
	assert.Len(t, resp.Items, 1)
	assert.Equal(t, "player.list", resp.Items[0].FunctionID)
}

func TestServiceOperations_NotFound(t *testing.T) {
	svcCtx, ctx := newResourceTestServiceContext(t, reg.NewStore(), "resources:read")
	_, err := NewService(svcCtx).Operations(ctx, &ResourceOperationsRequest{ResourceKey: "nonexistent"})
	assert.Error(t, err)
}

func TestServiceDetail_Unauthorized(t *testing.T) {
	svcCtx, _ := newResourceTestServiceContext(t, reg.NewStore())
	_, err := NewService(svcCtx).Detail(context.Background(), &ResourceDetailRequest{ResourceKey: "player"})
	assert.Error(t, err)
}

func TestServiceOperations_Unauthorized(t *testing.T) {
	svcCtx, _ := newResourceTestServiceContext(t, reg.NewStore())
	_, err := NewService(svcCtx).Operations(context.Background(), &ResourceOperationsRequest{ResourceKey: "player"})
	assert.Error(t, err)
}
