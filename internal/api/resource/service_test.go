package resource

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	consoleapi "github.com/cuihairu/croupier/internal/api/console"
	pageapi "github.com/cuihairu/croupier/internal/api/page"
	"github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/cache"
	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/model"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/getkin/kin-openapi/openapi3"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestServiceListCollectsRegistryDescriptorV2Metadata(t *testing.T) {
	store := reg.NewStore()
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "agent-1",
		GameID:   "demo-game",
		Env:      "development",
		ExpireAt: time.Now().Add(time.Minute),
		LastSeen: time.Now(),
		Functions: map[string]reg.FunctionMeta{
			"player.list": {
				Enabled:      true,
				Version:      "1.0.0",
				Tags:         []string{"player"},
				Summary:      "List players",
				Description:  "List player accounts",
				InputSchema:  `{"type":"object","properties":{"keyword":{"type":"string"}}}`,
				OutputSchema: `{"type":"object","properties":{"items":{"type":"array"}}}`,
				Resource:     "player",
				Risk:         "safe",
				Operation:    "list",
				Permission:   "player:list",
			},
			"player.ban": {
				Enabled:      true,
				Version:      "1.0.0",
				Tags:         []string{"player", "moderation"},
				Summary:      "Ban player",
				Description:  "Ban a player account",
				InputSchema:  `{"type":"object","properties":{"player_id":{"type":"string"}}}`,
				OutputSchema: `{"type":"object","properties":{"success":{"type":"boolean"}}}`,
				Resource:     "player",
				Risk:         "danger",
				Operation:    "ban",
				Permission:   "player:ban",
			},
		},
	})

	svcCtx, ctx := newResourceTestServiceContext(t, store, "resources:read")
	service := NewService(svcCtx)
	resp, err := service.List(ctx, &ResourceListRequest{})
	require.NoError(t, err)
	require.Len(t, resp.Items, 1)

	player := resp.Items[0]
	assert.Equal(t, "player", player.Key)
	assert.Equal(t, "player", player.Labels["zh-CN"])
	assert.Equal(t, "player", player.Category.Key)
	assert.Equal(t, "player", player.Category.Labels["zh-CN"])
	require.Len(t, player.Operations, 2)

	ops := map[string]spec.OperationSpec{}
	for _, op := range player.Operations {
		ops[op.FunctionID] = op
	}

	listOp := ops["player.list"]
	assert.Equal(t, "list", listOp.Operation)
	assert.Equal(t, "player:list", listOp.Permission)
	assert.Empty(t, listOp.Diagnostics)

	banOp := ops["player.ban"]
	assert.Equal(t, "ban", banOp.Operation)
	assert.Equal(t, spec.RiskDanger, banOp.Risk)
	assert.Equal(t, "player:ban", banOp.Permission)
	assert.Empty(t, banOp.Diagnostics)
}

func TestServiceGeneratedPagesCreatesEntityCandidateFromExplicitPageContract(t *testing.T) {
	store := reg.NewStore()
	require.NoError(t, store.UpsertOpenAPI("player.list", openAPIOperationWithPageContract("player", "list", map[string]interface{}{
		"version":       "page-contract:1",
		"inputMapping":  map[string]interface{}{"page": "values.page", "pageSize": "values.pageSize"},
		"outputMapping": map[string]interface{}{"stateKey": "players"},
		"pagination": map[string]interface{}{
			"pageField":     "page",
			"pageSizeField": "pageSize",
			"itemsPath":     "items",
			"totalPath":     "total",
		},
		"table": map[string]interface{}{
			"columns": []interface{}{
				map[string]interface{}{"key": "id", "title": map[string]interface{}{"zh-CN": "玩家ID"}, "valuePath": "id"},
			},
		},
	})))
	require.NoError(t, store.UpsertOpenAPI("player.ban", openAPIOperationWithPageContract("player", "ban", map[string]interface{}{
		"version":       "page-contract:1",
		"inputMapping":  map[string]interface{}{"playerId": "row.id", "reason": "values.reason"},
		"outputMapping": map[string]interface{}{"stateKey": "banResult"},
	})))
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "agent-1",
		ExpireAt: time.Now().Add(time.Minute),
		LastSeen: time.Now(),
		Functions: map[string]reg.FunctionMeta{
			"player.list": {
				Enabled:      true,
				Version:      "1.0.0",
				InputSchema:  `{"type":"object","properties":{"keyword":{"type":"string"}}}`,
				OutputSchema: `{"type":"object","properties":{"items":{"type":"array"},"total":{"type":"number"}}}`,
				Resource:     "player",
				Operation:    "list",
			},
			"player.ban": {
				Enabled:      true,
				Version:      "1.0.0",
				InputSchema:  `{"type":"object","properties":{"playerId":{"type":"string"},"reason":{"type":"string"}}}`,
				OutputSchema: `{"type":"object","properties":{"success":{"type":"boolean"}}}`,
				Resource:     "player",
				Operation:    "ban",
				Risk:         "danger",
			},
		},
	})

	svcCtx, ctx := newResourceTestServiceContext(t, store, "pages:edit")
	service := NewService(svcCtx)
	resp, err := service.GeneratedPages(ctx, &ResourceGeneratedPagesRequest{ResourceKey: "player"})
	require.NoError(t, err)
	require.NotEmpty(t, resp.Items)

	page := resp.Items[0]
	assert.Equal(t, spec.PageTypeEntity, page.Type)
	assert.Equal(t, "player.manage", page.PageKey)
	assert.Equal(t, "player", page.Category.Key)
	assert.Contains(t, string(page.Schema), `"x-component":"DataTable"`)
	assert.Contains(t, string(page.Schema), `"bindingId":"player.query"`)
	assert.Contains(t, string(page.Schema), `"bindingId":"player.ban"`)
	assert.Equal(t, spec.GeneratedPageQualityReady, page.Quality)
	assert.NotContains(t, string(page.Schema), `"functionId"`)
	assert.NotContains(t, string(page.Schema), `"operation":"update"`)
}

func TestServiceGeneratedPagesDoesNotGuessTableContract(t *testing.T) {
	store := reg.NewStore()
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "agent-1",
		GameID:   "demo-game",
		Env:      "development",
		ExpireAt: time.Now().Add(time.Minute),
		LastSeen: time.Now(),
		Functions: map[string]reg.FunctionMeta{
			"player.list": {
				Enabled:     true,
				Version:     "1.0.0",
				InputSchema: `{"type":"object"}`,
				Resource:    "player",
				Operation:   "list",
			},
		},
	})

	svcCtx, ctx := newResourceTestServiceContext(t, store, "resources:diagnose")
	service := NewService(svcCtx)
	resp, err := service.GeneratedPages(ctx, &ResourceGeneratedPagesRequest{ResourceKey: "player"})
	require.NoError(t, err)
	require.NotEmpty(t, resp.Items)

	page := resp.Items[0]
	assert.Equal(t, spec.PageTypeOperation, page.Type)
	assert.Equal(t, spec.GeneratedPageQualityBasic, page.Quality)
	require.Len(t, page.Bindings, 1)
	assert.JSONEq(t, `{}`, string(page.Bindings[0].OutputMapping))
	assert.NotContains(t, string(page.Schema), `"x-component":"DataTable"`)
	assert.Contains(t, string(page.Schema), `"x-component":"QueryForm"`)
	require.NotEmpty(t, page.Diagnostics)
	assert.Equal(t, "page_contract_missing", page.Diagnostics[0].Code)
}

func TestServiceGeneratedPagesKeepsMailSendAsOperationPage(t *testing.T) {
	store := reg.NewStore()
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "agent-1",
		GameID:   "demo-game",
		Env:      "development",
		ExpireAt: time.Now().Add(time.Minute),
		LastSeen: time.Now(),
		Functions: map[string]reg.FunctionMeta{
			"mail.send": {
				Enabled:      true,
				Version:      "1.0.0",
				InputSchema:  `{"type":"object","properties":{"target":{"type":"string"},"content":{"type":"string"}}}`,
				OutputSchema: `{"type":"object","properties":{"messageId":{"type":"string"}}}`,
				Resource:     "mail",
				Operation:    "send",
			},
		},
	})

	svcCtx, ctx := newResourceTestServiceContext(t, store, "resources:diagnose")
	service := NewService(svcCtx)
	resp, err := service.GeneratedPages(ctx, &ResourceGeneratedPagesRequest{ResourceKey: "mail"})
	require.NoError(t, err)
	require.Len(t, resp.Items, 1)

	page := resp.Items[0]
	assert.Equal(t, spec.PageTypeOperation, page.Type)
	assert.Equal(t, "mail.send", page.PageKey)
	assert.Equal(t, spec.GeneratedPageQualityBasic, page.Quality)
	require.Len(t, page.Bindings, 1)
	assert.JSONEq(t, `{"content":"values.content","target":"values.target"}`, string(page.Bindings[0].InputMapping))
	assert.JSONEq(t, `{}`, string(page.Bindings[0].OutputMapping))
	assert.Contains(t, string(page.Schema), `"x-component":"QueryForm"`)
	assert.Contains(t, string(page.Schema), `"x-component":"ResultPanel"`)
	assert.NotContains(t, string(page.Schema), `"x-component":"DataTable"`)
}

func TestServiceGeneratedPagesUsesBoundOpenAPISourcePageContract(t *testing.T) {
	store := reg.NewStore()
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "agent-1",
		GameID:   "demo-game",
		Env:      "development",
		ExpireAt: time.Now().Add(time.Minute),
		LastSeen: time.Now(),
		Functions: map[string]reg.FunctionMeta{
			"player.list": {Enabled: true, Version: "1.0.0"},
		},
	})

	svcCtx, ctx := newResourceTestServiceContext(t, store, "resources:diagnose")
	playerListContract := map[string]interface{}{
		"version":       "page-contract:1",
		"inputMapping":  map[string]interface{}{"page": "values.page", "pageSize": "values.pageSize"},
		"outputMapping": map[string]interface{}{"stateKey": "players"},
		"pagination": map[string]interface{}{
			"pageField":     "page",
			"pageSizeField": "pageSize",
			"itemsPath":     "items",
			"totalPath":     "total",
		},
		"table": map[string]interface{}{
			"columns": []interface{}{
				map[string]interface{}{"key": "id", "title": map[string]interface{}{"zh-CN": "玩家ID"}, "valuePath": "id"},
			},
		},
	}
	source := openAPISourceModel(
		t,
		"player-source",
		"player.list",
		"player",
		"list",
		playerListContract,
		openAPISourceDocument(t, "player.list", "player", "list", playerListContract),
	)
	require.NoError(t, svcCtx.OpenAPISourceModel.Create(ctx, source))
	require.NoError(t, svcCtx.OpenAPISourceBindingModel.Upsert(ctx, &model.OpenAPISourceBinding{
		GameID:      "demo-game",
		Env:         "development",
		SourceID:    source.SourceID,
		BindingID:   "player.list",
		OperationID: "player.list",
		Kind:        "provider",
		FunctionID:  "player.list",
	}))

	resp, err := NewService(svcCtx).GeneratedPages(ctx, &ResourceGeneratedPagesRequest{ResourceKey: "player"})
	require.NoError(t, err)
	require.NotEmpty(t, resp.Items)

	page := resp.Items[0]
	assert.Equal(t, spec.PageTypeEntity, page.Type)
	assert.Equal(t, spec.GeneratedPageQualityReady, page.Quality)
	assert.Contains(t, string(page.Schema), `"x-component":"DataTable"`)
	assert.Contains(t, string(page.Schema), `"bindingId":"player.query"`)
}

func TestServiceGeneratedOpenAPISourceCandidateCanBePublishedToConsole(t *testing.T) {
	store := reg.NewStore()
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "agent-1",
		GameID:   "demo-game",
		Env:      "development",
		ExpireAt: time.Now().Add(time.Minute),
		LastSeen: time.Now(),
		Functions: map[string]reg.FunctionMeta{
			"player.list": {
				Enabled:      true,
				Version:      "1.0.0",
				InputSchema:  `{"type":"object","properties":{"page":{"type":"integer"},"pageSize":{"type":"integer"}}}`,
				OutputSchema: `{"type":"object","properties":{"items":{"type":"array","items":{"type":"object"}},"total":{"type":"integer"}}}`,
			},
		},
	})

	svcCtx, ctx := newResourceTestServiceContext(t, store, "resources:diagnose", "pages:edit", "pages:publish", "pages:read")
	playerListContract := map[string]interface{}{
		"version":       "page-contract:1",
		"inputMapping":  map[string]interface{}{"page": "values.page", "pageSize": "values.pageSize"},
		"outputMapping": map[string]interface{}{"stateKey": "players"},
		"pagination": map[string]interface{}{
			"pageField":     "page",
			"pageSizeField": "pageSize",
			"itemsPath":     "items",
			"totalPath":     "total",
		},
		"table": map[string]interface{}{
			"columns": []interface{}{
				map[string]interface{}{"key": "id", "title": map[string]interface{}{"zh-CN": "玩家ID"}, "valuePath": "id"},
			},
		},
	}
	source := openAPISourceModel(
		t,
		"player-source",
		"player.list",
		"player",
		"list",
		playerListContract,
		openAPISourceDocument(t, "player.list", "player", "list", playerListContract),
	)
	require.NoError(t, svcCtx.OpenAPISourceModel.Create(ctx, source))
	require.NoError(t, svcCtx.OpenAPISourceBindingModel.Upsert(ctx, &model.OpenAPISourceBinding{
		GameID:      "demo-game",
		Env:         "development",
		SourceID:    source.SourceID,
		BindingID:   "player.list",
		OperationID: "player.list",
		Kind:        "provider",
		FunctionID:  "player.list",
	}))

	generated, err := NewService(svcCtx).GeneratedPages(ctx, &ResourceGeneratedPagesRequest{ResourceKey: "player"})
	require.NoError(t, err)
	require.NotEmpty(t, generated.Items)
	candidate := generated.Items[0]
	require.Equal(t, spec.GeneratedPageQualityReady, candidate.Quality)

	pageService := pageapi.NewService(svcCtx)
	revision := 0
	saved, err := pageService.SaveDraft(ctx, &pageapi.PageSaveRequest{
		PageKey:       candidate.PageKey,
		DraftRevision: &revision,
		Type:          candidate.Type,
		ResourceKey:   candidate.ResourceKey,
		Title:         map[string]string(candidate.Title),
		Category:      candidate.Category,
		Schema:        json.RawMessage(candidate.Schema),
		Bindings:      candidate.Bindings,
	})
	require.NoError(t, err)

	published, err := pageService.Publish(ctx, &pageapi.PagePublishRequest{
		PageKey:       candidate.PageKey,
		DraftRevision: &saved.DraftRevision,
	})
	require.NoError(t, err)
	assert.True(t, published.Published)

	consoleService := consoleapi.NewService(svcCtx)
	menu, err := consoleService.Menu(ctx, &consoleapi.ConsoleMenuRequest{Language: "zh-CN"})
	require.NoError(t, err)
	require.Len(t, menu.Items, 1)
	assert.Equal(t, "player", menu.Items[0].Key)
	require.Len(t, menu.Items[0].Children, 1)
	assert.Equal(t, "player.manage", menu.Items[0].Children[0].Key)
	assert.Equal(t, "/console/player/player.manage", menu.Items[0].Children[0].Path)
	assert.False(t, menu.Items[0].Children[0].Locale)

	page, err := consoleService.Page(ctx, &consoleapi.ConsolePageRequest{PageKey: candidate.PageKey})
	require.NoError(t, err)
	assert.Equal(t, "demo-game", page.Page.GameID)
	assert.Equal(t, "development", page.Page.Env)
	assert.Equal(t, candidate.PageKey, page.Page.PageKey)
	assert.Equal(t, saved.DraftRevision, page.Page.Version)
	require.Len(t, page.Page.BindingContracts, 1)
	assert.Equal(t, "player.query", page.Page.BindingContracts[0].BindingID)
	assert.Equal(t, "player.list", page.Page.BindingContracts[0].FunctionID)
}

func TestServiceListIgnoresUnboundOpenAPISource(t *testing.T) {
	store := reg.NewStore()
	svcCtx, ctx := newResourceTestServiceContext(t, store, "resources:read")
	mailSendContract := map[string]interface{}{"version": "page-contract:1"}
	source := openAPISourceModel(
		t,
		"mail-source",
		"mail.send",
		"mail",
		"send",
		mailSendContract,
		openAPISourceDocument(t, "mail.send", "mail", "send", mailSendContract),
	)
	require.NoError(t, svcCtx.OpenAPISourceModel.Create(ctx, source))

	resp, err := NewService(svcCtx).List(ctx, &ResourceListRequest{})
	require.NoError(t, err)
	assert.Empty(t, resp.Items)
}

func openAPIOperationWithPageContract(resource string, operation string, pageContract map[string]interface{}) *openapi3.Operation {
	return &openapi3.Operation{
		Extensions: map[string]interface{}{
			"x-resource":      resource,
			"x-operation":     operation,
			"x-page-contract": pageContract,
		},
	}
}

func openAPISourceDocument(t *testing.T, operationID string, resource string, operation string, pageContract map[string]interface{}) json.RawMessage {
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
					"operationId":     operationID,
					"summary":         operationID,
					"x-resource":      resource,
					"x-operation":     operation,
					"x-page-contract": pageContract,
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
	pageContract map[string]interface{},
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
			"operationId":  operationID,
			"summary":      operationID,
			"resource":     resource,
			"operation":    operation,
			"pageContract": pageContract,
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
