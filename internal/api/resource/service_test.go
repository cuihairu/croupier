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
				Capability:   "collection_query",
				Execution:    "sync",
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
				Capability:   "action",
				Execution:    "sync",
				Permission:   "player:ban",
			},
		},
	})

	svcCtx, ctx := newResourceTestServiceContext(t, store, "resources:read")
	resp, err := NewService(svcCtx).List(ctx, &ResourceListRequest{})
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

func TestServiceGeneratedPagesDoNotUseRegistrationPageExtensions(t *testing.T) {
	store := reg.NewStore()
	require.NoError(t, store.UpsertOpenAPI("player.list", openAPIOperationWithCapability("player", "list", "collection_query", "sync")))
	require.NoError(t, store.UpsertOpenAPI("player.ban", openAPIOperationWithCapability("player", "ban", "action", "sync")))
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
				InputSchema:  `{"type":"object","properties":{"keyword":{"type":"string"}}}`,
				OutputSchema: `{"type":"object","properties":{"items":{"type":"array"},"total":{"type":"number"}}}`,
			},
			"player.ban": {
				Enabled:      true,
				Version:      "1.0.0",
				InputSchema:  `{"type":"object","properties":{"playerId":{"type":"string"},"reason":{"type":"string"}}}`,
				OutputSchema: `{"type":"object","properties":{"success":{"type":"boolean"}}}`,
				Risk:         "danger",
			},
		},
	})

	svcCtx, ctx := newResourceTestServiceContext(t, store, "pages:edit")
	resp, err := NewService(svcCtx).GeneratedPages(ctx, &ResourceGeneratedPagesRequest{ResourceKey: "player"})
	require.NoError(t, err)
	require.Len(t, resp.Items, 2)

	for _, page := range resp.Items {
		assert.Equal(t, spec.PageTypeOperation, page.Type)
		assert.Equal(t, spec.GeneratedPageQualityBasic, page.Quality)
		assert.Equal(t, "player", page.Category.Key)
		require.NotNil(t, page.Operation)
		require.NotNil(t, page.Operation.Form)
		assert.Nil(t, page.Resource)
	}
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
				Capability:  "collection_query",
			},
		},
	})

	svcCtx, ctx := newResourceTestServiceContext(t, store, "resources:diagnose")
	resp, err := NewService(svcCtx).GeneratedPages(ctx, &ResourceGeneratedPagesRequest{ResourceKey: "player"})
	require.NoError(t, err)
	require.Len(t, resp.Items, 1)

	page := resp.Items[0]
	assert.Equal(t, spec.PageTypeOperation, page.Type)
	assert.Equal(t, spec.GeneratedPageQualityBasic, page.Quality)
	require.Len(t, page.Bindings, 1)
	require.NotNil(t, page.Operation)
	require.NotNil(t, page.Operation.Form)
	assert.Empty(t, page.Bindings[0].Selectors.Input.Assignments)
	assert.Empty(t, page.Diagnostics)
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
				Capability:   "action",
			},
		},
	})

	svcCtx, ctx := newResourceTestServiceContext(t, store, "resources:diagnose")
	resp, err := NewService(svcCtx).GeneratedPages(ctx, &ResourceGeneratedPagesRequest{ResourceKey: "mail"})
	require.NoError(t, err)
	require.Len(t, resp.Items, 1)

	page := resp.Items[0]
	assert.Equal(t, spec.PageTypeOperation, page.Type)
	assert.Equal(t, "mail.send", page.PageKey)
	assert.Equal(t, spec.GeneratedPageQualityBasic, page.Quality)
	require.Len(t, page.Bindings, 1)
	require.NotNil(t, page.Bindings[0].Selectors)
	assertSelectorAssignment(t, page.Bindings[0].Selectors.Input, "content", spec.SourceForm, "content")
	assertSelectorAssignment(t, page.Bindings[0].Selectors.Input, "target", spec.SourceForm, "target")
	require.NotNil(t, page.Operation)
	require.NotNil(t, page.Operation.ResultView)
}

func TestServiceListUsesBoundOpenAPISourceFunctionContract(t *testing.T) {
	store := reg.NewStore()
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "agent-1",
		GameID:   "demo-game",
		Env:      "development",
		ExpireAt: time.Now().Add(time.Minute),
		LastSeen: time.Now(),
		Functions: map[string]reg.FunctionMeta{
			"analytics.retention": {Enabled: true, Version: "1.0.0"},
		},
	})

	svcCtx, ctx := newResourceTestServiceContext(t, store, "resources:read")
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
	require.Len(t, resp.Items, 1)
	require.Len(t, resp.Items[0].Operations, 1)

	op := resp.Items[0].Operations[0]
	assert.Equal(t, "analytics.retention", op.FunctionID)
	assert.Equal(t, "analytics", op.ResourceKey)
	assert.Equal(t, "retention", op.Operation)
	assert.Equal(t, spec.CapabilityReport, op.Capability)
	assert.Equal(t, spec.FunctionExecutionSync, op.Execution)
}

func TestServiceGeneratedPagesUsesBoundOpenAPISourceCapability(t *testing.T) {
	store := reg.NewStore()
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "agent-1",
		GameID:   "demo-game",
		Env:      "development",
		ExpireAt: time.Now().Add(time.Minute),
		LastSeen: time.Now(),
		Functions: map[string]reg.FunctionMeta{
			"reward.batchGrant": {Enabled: true, Version: "1.0.0"},
		},
	})

	svcCtx, ctx := newResourceTestServiceContext(t, store, "resources:diagnose")
	source := openAPISourceModel(
		t,
		"reward-source",
		"reward.batchGrant",
		"reward",
		"batchGrant",
		"task",
		"task",
		openAPISourceDocument(t, "reward.batchGrant", "reward", "batchGrant", "task", "task"),
	)
	require.NoError(t, svcCtx.OpenAPISourceModel.Create(ctx, source))
	require.NoError(t, svcCtx.OpenAPISourceBindingModel.Upsert(ctx, &model.OpenAPISourceBinding{
		GameID:      "demo-game",
		Env:         "development",
		SourceID:    source.SourceID,
		BindingID:   "reward.batchGrant",
		OperationID: "reward.batchGrant",
		Kind:        "provider",
		FunctionID:  "reward.batchGrant",
	}))

	resp, err := NewService(svcCtx).GeneratedPages(ctx, &ResourceGeneratedPagesRequest{ResourceKey: "reward"})
	require.NoError(t, err)
	require.Len(t, resp.Items, 1)

	page := resp.Items[0]
	assert.Equal(t, spec.PageTypeTask, page.Type)
	assert.Equal(t, spec.GeneratedPageQualityNeedsReview, page.Quality)
	assert.Equal(t, spec.BindingUsageTask, page.Bindings[0].Usage)
	assert.Equal(t, spec.PageExecutionModeTask, page.Bindings[0].Execution.Mode)
	require.NotNil(t, page.Task)
	require.NotNil(t, page.Task.TaskView)
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
	source := openAPISourceModel(
		t,
		"player-source",
		"player.list",
		"player",
		"list",
		"collection_query",
		"sync",
		openAPISourceDocument(t, "player.list", "player", "list", "collection_query", "sync"),
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
	require.Len(t, generated.Items, 1)
	candidate := generated.Items[0]
	require.Equal(t, spec.GeneratedPageQualityBasic, candidate.Quality)
	require.Equal(t, "player.list", candidate.PageKey)
	require.Len(t, candidate.Bindings, 1)

	pageService := pageapi.NewService(svcCtx)
	revision := 0
	saved, err := pageService.SaveDraft(ctx, &pageapi.PageSaveRequest{
		PageKey:       candidate.PageKey,
		DraftRevision: &revision,
		Type:          candidate.Type,
		ResourceKey:   candidate.ResourceKey,
		Title:         map[string]string(candidate.Title),
		Category:      candidate.Category,
		Operation:     candidate.Operation,
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
	assert.Equal(t, "player.list", menu.Items[0].Children[0].Key)
	assert.Equal(t, "/console/player/player.list", menu.Items[0].Children[0].Path)
	assert.False(t, menu.Items[0].Children[0].Locale)

	page, err := consoleService.Page(ctx, &consoleapi.ConsolePageRequest{PageKey: candidate.PageKey})
	require.NoError(t, err)
	assert.Equal(t, "demo-game", page.Page.GameID)
	assert.Equal(t, "development", page.Page.Env)
	assert.Equal(t, candidate.PageKey, page.Page.PageKey)
	assert.Equal(t, saved.DraftRevision, page.Page.Version)
	require.Len(t, page.Page.BindingContracts, 1)
	assert.Equal(t, "player.main", page.Page.BindingContracts[0].BindingID)
	assert.Equal(t, "player.list", page.Page.BindingContracts[0].FunctionID)
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

func openAPIOperationWithCapability(resource string, operation string, capability string, execution string) *openapi3.Operation {
	return &openapi3.Operation{
		Extensions: map[string]interface{}{
			"x-resource":   resource,
			"x-operation":  operation,
			"x-capability": capability,
			"x-execution":  execution,
		},
	}
}

func assertSelectorAssignment(t *testing.T, selector spec.SelectorAST, target string, sourceType spec.SelectorSourceType, sourcePath string) {
	t.Helper()
	for _, assignment := range selector.Assignments {
		if assignment.Target == target && assignment.Source.Type == sourceType && assignment.Source.Path == sourcePath {
			return
		}
	}
	t.Fatalf("expected selector assignment %s <- %s:%s, got %#v", target, sourceType, sourcePath, selector.Assignments)
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
