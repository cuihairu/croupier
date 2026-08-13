package openapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/cache"
	"github.com/cuihairu/croupier/internal/common/errorx"
	dashspec "github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/gin-gonic/gin"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupOpenAPITestService(t *testing.T) *Service {
	t.Helper()
	service, _ := setupOpenAPITestServiceWithPermissions(t, "openapi_sources:read", "openapi_sources:write")
	return service
}

func setupOpenAPITestServiceWithPermissions(t *testing.T, permissions ...string) (*Service, context.Context) {
	service, ctx, _ := setupOpenAPITestServiceWithAudit(t, permissions...)
	return service, ctx
}

func setupOpenAPITestServiceWithAudit(t *testing.T, permissions ...string) (*Service, context.Context, *audit.InMemoryAuditStore) {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := model.AutoMigrate(db); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	store := registry.NewStore()
	store.UpsertAgent(&registry.AgentSession{
		AgentID: "agent-1",
		GameID:  "demo-game",
		Env:     "development",
		Functions: map[string]registry.FunctionMeta{
			"player.list": {Enabled: true, Version: "1.0.0"},
			"player.get":  {Enabled: true, Version: "1.0.0"},
		},
		LastSeen: time.Now(),
	})

	admin := model.Admin{Username: "openapi_tester", Status: 1, PasswordHash: "test"}
	require.NoError(t, db.Create(&admin).Error)
	role := model.Role{Name: "openapi_tester_role", Description: "openapi tester"}
	require.NoError(t, db.Create(&role).Error)
	require.NoError(t, db.Create(&model.AdminRole{AdminID: admin.ID, RoleID: role.ID}).Error)
	for _, permissionID := range permissions {
		grantOpenAPITestPermission(t, db, role.ID, permissionID)
	}

	nullCache := cache.NewNullCache()
	auditStore := audit.NewInMemoryAuditStore()
	svcCtx := &svc.ServiceContext{
		DB:                        db,
		AdminModel:                model.NewAdminModel(db),
		RoleModel:                 model.NewRoleModel(db),
		PermissionModel:           model.NewPermissionModel(db),
		FunctionModel:             model.NewFunctionModel(db),
		PageSpecModel:             model.NewPageSpecModel(db),
		PublishedPageSpecModel:    model.NewPublishedPageSpecModel(db),
		PageVersionModel:          model.NewPageVersionModel(db),
		RegistryStore:             store,
		OpenAPISourceModel:        model.NewOpenAPISourceModel(db),
		OpenAPISourceBindingModel: model.NewOpenAPISourceBindingModel(db),
		AuditService:              audit.NewAuditService(auditStore, nil),
		Cache:                     nullCache,
		CacheHelper:               cache.NewCacheHelper(nullCache),
	}
	ctx := svc.WithGameScope(context.Background(), svc.GameScope{
		GameID: "demo-game",
		Env:    "development",
	})
	ctx = context.WithValue(ctx, "username", admin.Username)
	return NewService(svcCtx), ctx, auditStore
}

func openAPITestContext() context.Context {
	ctx := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "demo-game", Env: "development"})
	return context.WithValue(ctx, "username", "openapi_tester")
}

func grantOpenAPITestPermission(t *testing.T, db *gorm.DB, roleID uint, permissionID string) {
	t.Helper()
	permissionID = strings.TrimSpace(permissionID)
	if permissionID == "" {
		return
	}
	parts := strings.SplitN(permissionID, ":", 2)
	action := "*"
	if len(parts) == 2 {
		action = parts[1]
	}
	permission := model.Permission{
		ID:       permissionID,
		Name:     permissionID,
		Resource: parts[0],
		Action:   action,
		Category: "dashboard",
	}
	require.NoError(t, db.Where("id = ?", permission.ID).FirstOrCreate(&permission).Error)
	require.NoError(t, db.Create(&model.RolePermission{RoleID: roleID, PermissionID: permissionID}).Error)
}

func rawSpec(t *testing.T, spec map[string]interface{}) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(spec)
	require.NoError(t, err)
	return json.RawMessage(data)
}

func assertOpenAPISourceDiagnostic(t *testing.T, err error, code string, severity dashspec.DiagnosticSeverity, fieldSuffix string) {
	t.Helper()
	var codeErr *errorx.CodeError
	require.True(t, errors.As(err, &codeErr), "expected errorx.CodeError, got %T", err)
	rawDiagnostics, ok := codeErr.Details["diagnostics"]
	require.True(t, ok, "expected structured diagnostics in error details: %#v", codeErr.Details)

	diagnostics, ok := rawDiagnostics.([]dashspec.Diagnostic)
	require.True(t, ok, "expected []spec.Diagnostic, got %T", rawDiagnostics)
	for _, diagnostic := range diagnostics {
		if diagnostic.Code != code || diagnostic.Severity != severity {
			continue
		}
		if fieldSuffix == "" || strings.HasSuffix(diagnostic.Field, fieldSuffix) {
			return
		}
	}
	t.Fatalf("diagnostic code=%s severity=%s fieldSuffix=%s not found in %#v", code, severity, fieldSuffix, diagnostics)
}

func assertSourceDiagnostic(t *testing.T, diagnostics []dashspec.Diagnostic, code string, severity dashspec.DiagnosticSeverity) {
	t.Helper()
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == code && diagnostic.Severity == severity {
			return
		}
	}
	t.Fatalf("diagnostic code=%s severity=%s not found in %#v", code, severity, diagnostics)
}

func setupOpenAPITestHandler(t *testing.T) (*Handler, *gin.Engine) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	service := setupOpenAPITestService(t)
	handler := NewHandler(service)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Request = c.Request.WithContext(openAPITestContext())
		c.Next()
	})
	router.GET("/spec/:id", handler.GetSpec)
	router.GET("/document", handler.GetDocument)
	router.POST("/batch/spec", handler.BatchGetSpec)
	router.GET("/sources", handler.ListSources)
	router.POST("/sources", handler.CreateSource)
	router.GET("/sources/:sourceId", handler.GetSource)
	router.PUT("/sources/:sourceId", handler.UpdateSource)
	router.GET("/sources/:sourceId/diagnostics", handler.SourceDiagnostics)
	router.POST("/sources/:sourceId/bindings", handler.CreateBinding)

	return handler, router
}

// Service Tests

func TestService_GetSpec_RegisteredFunction(t *testing.T) {
	t.Parallel()

	service := setupOpenAPITestService(t)
	resp, err := service.GetSpec(context.Background(), &GetSpecRequest{ID: "player.list"})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.NotNil(t, resp.Spec)

	var op openapi3.Operation
	require.NoError(t, json.Unmarshal(resp.Spec, &op))
	assert.Equal(t, "player.list", op.OperationID)
}

func TestService_GetSpec_UnregisteredFunction(t *testing.T) {
	t.Parallel()

	service := setupOpenAPITestService(t)
	_, err := service.GetSpec(context.Background(), &GetSpecRequest{ID: "nonexistent"})
	assert.Error(t, err)
}

func TestService_GetSpec_EmptyID(t *testing.T) {
	t.Parallel()

	service := setupOpenAPITestService(t)
	_, err := service.GetSpec(context.Background(), &GetSpecRequest{ID: ""})
	assert.Error(t, err)
}

func TestService_CreateSource_ValidSpec(t *testing.T) {
	t.Parallel()

	service := setupOpenAPITestService(t)

	spec := map[string]interface{}{
		"openapi": "3.0.3",
		"info": map[string]interface{}{
			"title":   "Test API",
			"version": "1.0.0",
		},
		"paths": map[string]interface{}{
			"/users": map[string]interface{}{
				"get": map[string]interface{}{
					"operationId": "getUsers",
					"summary":     "Get all users",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Success",
						},
					},
				},
			},
		},
	}

	resp, err := service.CreateSource(openAPITestContext(), &OpenAPISourceCreateRequest{
		Name: "Test API",
		Spec: rawSpec(t, spec),
	})
	require.NoError(t, err)
	assert.NotEmpty(t, resp.Source.SourceID)
	assert.Equal(t, "Test API", resp.Source.Name)
	assert.Len(t, resp.Source.Operations, 1)
	assert.Equal(t, "getUsers", resp.Source.Operations[0].OperationID)
	assertSourceDiagnostic(t, resp.Source.Diagnostics, "rest_capability_inferred", dashspec.SeverityInfo)

	_, err = service.svcCtx.RegistryStore.GetOpenAPI("getUsers")
	assert.Error(t, err, "source upload must not register executable functions")
}

func TestService_OpenAPISourceRequiresReadPermission(t *testing.T) {
	t.Parallel()

	service, ctx := setupOpenAPITestServiceWithPermissions(t)
	_, err := service.ListSources(ctx, &OpenAPISourceListRequest{})
	assert.Error(t, err)
}

func TestService_OpenAPISourceWriteRequiresPermission(t *testing.T) {
	t.Parallel()

	service, ctx := setupOpenAPITestServiceWithPermissions(t, "openapi_sources:read")
	spec := map[string]interface{}{
		"openapi": "3.0.3",
		"info": map[string]interface{}{
			"title":   "Read Only API",
			"version": "1.0.0",
		},
		"paths": map[string]interface{}{},
	}
	_, err := service.CreateSource(ctx, &OpenAPISourceCreateRequest{Spec: rawSpec(t, spec)})
	assert.Error(t, err)
}

func TestService_UpdateSource_RefreshesRevisionOperationsAndAudit(t *testing.T) {
	t.Parallel()

	service, ctx, auditStore := setupOpenAPITestServiceWithAudit(t, "openapi_sources:read", "openapi_sources:write")
	initialSpec := map[string]interface{}{
		"openapi": "3.0.3",
		"info": map[string]interface{}{
			"title":   "Player API",
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
	created, err := service.CreateSource(ctx, &OpenAPISourceCreateRequest{Spec: rawSpec(t, initialSpec)})
	require.NoError(t, err)
	_, err = service.CreateBinding(ctx, &OpenAPISourceBindingCreateRequest{
		SourceID:    created.Source.SourceID,
		OperationID: "player.list",
		Kind:        "provider",
		FunctionID:  "player.list",
	})
	require.NoError(t, err)

	nextSpec := map[string]interface{}{
		"openapi": "3.0.3",
		"info": map[string]interface{}{
			"title":   "Player API v2",
			"version": "2.0.0",
		},
		"paths": map[string]interface{}{
			"/players/detail": map[string]interface{}{
				"get": map[string]interface{}{
					"operationId": "player.get",
					"x-resource":  "player",
					"x-operation": "get",
					"responses":   map[string]interface{}{"200": map[string]interface{}{"description": "OK"}},
				},
			},
		},
	}
	updated, err := service.UpdateSource(ctx, &OpenAPISourceUpdateRequest{
		SourceID: created.Source.SourceID,
		Spec:     rawSpec(t, nextSpec),
	})
	require.NoError(t, err)
	assert.Equal(t, 2, updated.Source.Revision)
	assert.Equal(t, "Player API v2", updated.Source.Name)
	require.Len(t, updated.Source.Operations, 1)
	assert.Equal(t, "player.get", updated.Source.Operations[0].OperationID)
	assert.False(t, updated.Source.Operations[0].Bound)
	require.Len(t, updated.Source.Bindings, 1, "Source revision update keeps explicit bindings for operator review")
	assert.Equal(t, "player.list", updated.Source.Bindings[0].OperationID)

	records, total, err := auditStore.List(audit.AuditFilter{EventType: []audit.AuditEventType{audit.EventOpenAPISourceUpdate}}, audit.AuditPage{PageSize: 10})
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Len(t, records, 1)
	assert.Equal(t, 1, records[0].Details["previous_revision"])
	assert.Equal(t, 2, records[0].Details["revision"])
	assert.Equal(t, "3.0.3", records[0].Details["openapi_version"])
}

func TestService_UpdateSourceRequiresWritePermission(t *testing.T) {
	t.Parallel()

	writer, writerCtx := setupOpenAPITestServiceWithPermissions(t, "openapi_sources:read", "openapi_sources:write")
	spec := map[string]interface{}{
		"openapi": "3.0.3",
		"info": map[string]interface{}{
			"title":   "Permission API",
			"version": "1.0.0",
		},
		"paths": map[string]interface{}{},
	}
	created, err := writer.CreateSource(writerCtx, &OpenAPISourceCreateRequest{Spec: rawSpec(t, spec)})
	require.NoError(t, err)

	reader := NewService(writer.svcCtx)
	readerCtx := context.WithValue(
		svc.WithGameScope(context.Background(), svc.GameScope{GameID: "demo-game", Env: "development"}),
		"username",
		"readonly_openapi_tester",
	)
	admin := model.Admin{Username: "readonly_openapi_tester", Status: 1, PasswordHash: "test"}
	require.NoError(t, writer.svcCtx.DB.Create(&admin).Error)
	role := model.Role{Name: "readonly_openapi_tester_role", Description: "readonly openapi tester"}
	require.NoError(t, writer.svcCtx.DB.Create(&role).Error)
	require.NoError(t, writer.svcCtx.DB.Create(&model.AdminRole{AdminID: admin.ID, RoleID: role.ID}).Error)
	grantOpenAPITestPermission(t, writer.svcCtx.DB, role.ID, "openapi_sources:read")

	_, err = reader.UpdateSource(readerCtx, &OpenAPISourceUpdateRequest{
		SourceID: created.Source.SourceID,
		Spec:     rawSpec(t, spec),
	})
	assert.Error(t, err)
}

func TestService_OpenAPISourceWritesAuditEvents(t *testing.T) {
	t.Parallel()

	service, ctx, auditStore := setupOpenAPITestServiceWithAudit(t, "openapi_sources:read", "openapi_sources:write")
	spec := map[string]interface{}{
		"openapi": "3.0.3",
		"info": map[string]interface{}{
			"title":   "Audit API",
			"version": "1.0.0",
		},
		"paths": map[string]interface{}{
			"/players": map[string]interface{}{
				"get": map[string]interface{}{
					"operationId": "playerList",
					"x-resource":  "player",
					"x-operation": "list",
					"responses":   map[string]interface{}{"200": map[string]interface{}{"description": "OK"}},
				},
			},
		},
	}
	created, err := service.CreateSource(ctx, &OpenAPISourceCreateRequest{Spec: rawSpec(t, spec)})
	require.NoError(t, err)
	_, err = service.CreateBinding(ctx, &OpenAPISourceBindingCreateRequest{
		SourceID:    created.Source.SourceID,
		OperationID: "playerList",
		Kind:        "provider",
		FunctionID:  "player.list",
	})
	require.NoError(t, err)
	_, err = service.DeleteBinding(ctx, &OpenAPISourceBindingDeleteRequest{
		SourceID:  created.Source.SourceID,
		BindingID: "playerList",
	})
	require.NoError(t, err)

	records, total, err := auditStore.List(audit.AuditFilter{}, audit.AuditPage{PageSize: 10})
	require.NoError(t, err)
	require.Equal(t, 3, total)
	require.Len(t, records, 3)

	byEvent := map[audit.AuditEventType]*audit.AuditRecord{}
	for _, record := range records {
		byEvent[record.EventType] = record
		assert.Equal(t, "openapi_tester", record.Actor.ID)
		assert.Equal(t, "openapi_source", record.Resource.Type)
		assert.Equal(t, created.Source.SourceID, record.Resource.ID)
		assert.Equal(t, "demo-game", record.Resource.GameID)
		assert.Equal(t, "development", record.Resource.Environment)
		assert.Equal(t, "success", record.Outcome)
	}
	assert.Equal(t, 1, byEvent[audit.EventOpenAPISourceCreate].Details["revision"])
	assert.Equal(t, "playerList", byEvent[audit.EventOpenAPISourceBindingCreate].Details["operation_id"])
	assert.Equal(t, "player.list", byEvent[audit.EventOpenAPISourceBindingCreate].Details["function_id"])
	assert.Equal(t, "playerList", byEvent[audit.EventOpenAPISourceBindingDelete].Details["binding_id"])
}

func TestDeleteBindingRestoresSDKContractAndProposal(t *testing.T) {
	service, ctx := setupOpenAPITestServiceWithPermissions(t, "openapi_sources:read", "openapi_sources:write")
	service.svcCtx.RegistryStore.Mu().Lock()
	service.svcCtx.RegistryStore.AgentsUnsafe()["agent-1"].Functions["player.list"] = registry.FunctionMeta{
		Enabled:      true,
		Version:      "1.0.0",
		InputSchema:  `{"type":"object"}`,
		OutputSchema: `{"type":"object","properties":{"success":{"type":"boolean"}}}`,
	}
	service.svcCtx.RegistryStore.Mu().Unlock()

	created, err := service.CreateSource(ctx, &OpenAPISourceCreateRequest{Spec: rawSpec(t, map[string]interface{}{
		"openapi": "3.0.3",
		"info":    map[string]interface{}{"title": "Player API", "version": "1.0.0"},
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
	})})
	require.NoError(t, err)
	_, err = service.CreateBinding(ctx, &OpenAPISourceBindingCreateRequest{
		SourceID: created.Source.SourceID, OperationID: "player.list", Kind: "provider", FunctionID: "player.list",
	})
	require.NoError(t, err)

	contractModel := model.NewFunctionContractModel(service.svcCtx.DB)
	contract, err := contractModel.FindByScopeAndFunctionID(ctx, "demo-game", "development", "player.list")
	require.NoError(t, err)
	assert.Equal(t, "openapi", contract.Source)
	assert.Equal(t, "player", contract.ResourceKey)

	_, err = service.DeleteBinding(ctx, &OpenAPISourceBindingDeleteRequest{SourceID: created.Source.SourceID, BindingID: "player.list"})
	require.NoError(t, err)
	contract, err = contractModel.FindByScopeAndFunctionID(ctx, "demo-game", "development", "player.list")
	require.NoError(t, err)
	assert.Equal(t, "sdk", contract.Source)
	assert.Empty(t, contract.ResourceKey)
	_, err = model.NewPageProposalModel(service.svcCtx.DB).FindByScopeAndKey(ctx, "demo-game", "development", "operation:player.list")
	require.NoError(t, err)
}

func TestDeleteBindingRemovesOnlyOpenAPIContractAndResourceProposal(t *testing.T) {
	service, ctx := setupOpenAPITestServiceWithPermissions(t, "openapi_sources:read", "openapi_sources:write")
	created, err := service.CreateSource(ctx, &OpenAPISourceCreateRequest{Spec: rawSpec(t, map[string]interface{}{
		"openapi": "3.0.3",
		"info":    map[string]interface{}{"title": "Player API", "version": "1.0.0"},
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
	})})
	require.NoError(t, err)
	_, err = service.CreateBinding(ctx, &OpenAPISourceBindingCreateRequest{
		SourceID: created.Source.SourceID, OperationID: "player.list", Kind: "provider", FunctionID: "player.list",
	})
	require.NoError(t, err)
	// Exercise the no-runtime fallback branch only after the binding has been
	// created; CreateBinding correctly requires the function to be registered.
	service.svcCtx.RegistryStore.Mu().Lock()
	delete(service.svcCtx.RegistryStore.AgentsUnsafe()["agent-1"].Functions, "player.list")
	service.svcCtx.RegistryStore.Mu().Unlock()

	_, err = service.DeleteBinding(ctx, &OpenAPISourceBindingDeleteRequest{SourceID: created.Source.SourceID, BindingID: "player.list"})
	require.NoError(t, err)
	_, err = model.NewFunctionContractModel(service.svcCtx.DB).FindByScopeAndFunctionID(ctx, "demo-game", "development", "player.list")
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
	_, err = model.NewPageProposalModel(service.svcCtx.DB).FindByScopeAndKey(ctx, "demo-game", "development", "resource:player")
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

func TestDeleteBindingRebuildsContractFromRemainingOpenAPIBinding(t *testing.T) {
	service, ctx := setupOpenAPITestServiceWithPermissions(t, "openapi_sources:read", "openapi_sources:write")
	created, err := service.CreateSource(ctx, &OpenAPISourceCreateRequest{Spec: rawSpec(t, map[string]interface{}{
		"openapi": "3.0.3",
		"info":    map[string]interface{}{"title": "Player API", "version": "1.0.0"},
		"paths": map[string]interface{}{
			"/players": map[string]interface{}{
				"get": map[string]interface{}{
					"operationId": "player.list",
					"x-resource":  "player",
					"x-operation": "list",
					"responses":   map[string]interface{}{"200": map[string]interface{}{"description": "OK"}},
				},
				"post": map[string]interface{}{
					"operationId": "player.create",
					"x-resource":  "player",
					"x-operation": "create",
					"responses":   map[string]interface{}{"200": map[string]interface{}{"description": "OK"}},
				},
			},
		},
	})})
	require.NoError(t, err)
	for _, operationID := range []string{"player.list", "player.create"} {
		_, err = service.CreateBinding(ctx, &OpenAPISourceBindingCreateRequest{
			SourceID: created.Source.SourceID, OperationID: operationID, Kind: "provider", FunctionID: "player.list",
		})
		require.NoError(t, err)
	}

	_, err = service.DeleteBinding(ctx, &OpenAPISourceBindingDeleteRequest{SourceID: created.Source.SourceID, BindingID: "player.list"})
	require.NoError(t, err)
	contract, err := model.NewFunctionContractModel(service.svcCtx.DB).FindByScopeAndFunctionID(ctx, "demo-game", "development", "player.list")
	require.NoError(t, err)
	assert.Equal(t, "openapi", contract.Source)
	assert.Equal(t, "player", contract.ResourceKey)
	assert.Equal(t, "create", contract.OperationKey)
}

func TestService_CreateSource_InvalidSpec(t *testing.T) {
	t.Parallel()

	service := setupOpenAPITestService(t)

	spec := map[string]interface{}{
		"openapi": "3.0.3",
		// Missing required info field
	}

	_, err := service.CreateSource(openAPITestContext(), &OpenAPISourceCreateRequest{Spec: rawSpec(t, spec)})
	require.Error(t, err)
	var codeErr *errorx.CodeError
	require.True(t, errors.As(err, &codeErr))
	assert.Equal(t, http.StatusBadRequest, codeErr.Code)
}

func TestService_CreateSource_RejectsUIExtensions(t *testing.T) {
	t.Parallel()

	service := setupOpenAPITestService(t)

	spec := map[string]interface{}{
		"openapi": "3.0.3",
		"info": map[string]interface{}{
			"title":   "Test API",
			"version": "1.0.0",
		},
		"paths": map[string]interface{}{
			"/users": map[string]interface{}{
				"post": map[string]interface{}{
					"operationId": "createUser",
					"x-ui": map[string]interface{}{
						"type": "object",
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Success",
						},
					},
				},
			},
		},
	}

	_, err := service.CreateSource(openAPITestContext(), &OpenAPISourceCreateRequest{Spec: rawSpec(t, spec)})
	require.Error(t, err)
	assertOpenAPISourceDiagnostic(t, err, "openapi_presentation_field_forbidden", dashspec.SeverityError, ".x-ui")
}

func TestService_CreateSource_RejectsPresentationContractFields(t *testing.T) {
	t.Parallel()

	for _, field := range []string{"x-page-contract", "inputMapping", "outputMapping", "pagination", "table", "title", "labels", "columns"} {
		field := field
		t.Run(field, func(t *testing.T) {
			t.Parallel()

			service := setupOpenAPITestService(t)
			specDoc := map[string]interface{}{
				"openapi": "3.0.3",
				"info": map[string]interface{}{
					"title":   "Presentation Contract API",
					"version": "1.0.0",
				},
				"paths": map[string]interface{}{
					"/users": map[string]interface{}{
						"post": map[string]interface{}{
							"operationId": field + "Operation",
							field: map[string]interface{}{
								"enabled": true,
							},
							"responses": map[string]interface{}{
								"200": map[string]interface{}{"description": "Success"},
							},
						},
					},
				},
			}

			_, err := service.CreateSource(openAPITestContext(), &OpenAPISourceCreateRequest{Spec: rawSpec(t, specDoc)})
			require.Error(t, err)
			assertOpenAPISourceDiagnostic(t, err, "openapi_presentation_field_forbidden", dashspec.SeverityError, "."+field)
		})
	}
}

func TestService_CreateSource_AllowsSchemaPropertiesNamedAfterPresentationTerms(t *testing.T) {
	t.Parallel()

	service := setupOpenAPITestService(t)
	specDoc := map[string]interface{}{
		"openapi": "3.0.3",
		"info": map[string]interface{}{
			"title":   "Business Payload API",
			"version": "1.0.0",
		},
		"paths": map[string]interface{}{
			"/users": map[string]interface{}{
				"post": map[string]interface{}{
					"operationId": "createUser",
					"requestBody": map[string]interface{}{
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"type": "object",
									"properties": map[string]interface{}{
										"menu":  map[string]interface{}{"type": "string"},
										"table": map[string]interface{}{"type": "string"},
									},
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "Success"},
					},
				},
			},
		},
	}

	created, err := service.CreateSource(openAPITestContext(), &OpenAPISourceCreateRequest{Spec: rawSpec(t, specDoc)})
	require.NoError(t, err)
	assert.Equal(t, 1, created.Source.OperationCount)
}

func TestService_CreateSource_RejectsFormilySchemaKeyword(t *testing.T) {
	t.Parallel()

	service := setupOpenAPITestService(t)
	specDoc := map[string]interface{}{
		"openapi": "3.0.3",
		"info": map[string]interface{}{
			"title":   "Invalid Schema API",
			"version": "1.0.0",
		},
		"paths": map[string]interface{}{
			"/users": map[string]interface{}{
				"post": map[string]interface{}{
					"operationId": "createUser",
					"requestBody": map[string]interface{}{
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"type":    "object",
									"formily": map[string]interface{}{"schema": map[string]interface{}{}},
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "Success"},
					},
				},
			},
		},
	}

	_, err := service.CreateSource(openAPITestContext(), &OpenAPISourceCreateRequest{Spec: rawSpec(t, specDoc)})
	require.Error(t, err)
	assertOpenAPISourceDiagnostic(t, err, "openapi_presentation_field_forbidden", dashspec.SeverityError, ".formily")
}

func TestService_CreateSource_RejectsInvalidCapabilityAndExecution(t *testing.T) {
	t.Parallel()

	service := setupOpenAPITestService(t)
	specDoc := map[string]interface{}{
		"openapi": "3.0.3",
		"info": map[string]interface{}{
			"title":   "Invalid Contract API",
			"version": "1.0.0",
		},
		"paths": map[string]interface{}{
			"/users": map[string]interface{}{
				"post": map[string]interface{}{
					"operationId":  "createUser",
					"x-capability": "row_button",
					"x-execution":  "modal",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "Success"},
					},
				},
			},
		},
	}

	_, err := service.CreateSource(openAPITestContext(), &OpenAPISourceCreateRequest{Spec: rawSpec(t, specDoc)})
	require.Error(t, err)
	assertOpenAPISourceDiagnostic(t, err, "openapi_capability_invalid", dashspec.SeverityError, ".x-capability")
	assertOpenAPISourceDiagnostic(t, err, "openapi_execution_invalid", dashspec.SeverityError, ".x-execution")
}

func TestService_CreateSource_CarriesFunctionContractFields(t *testing.T) {
	t.Parallel()

	service := setupOpenAPITestService(t)
	specDoc := map[string]interface{}{
		"openapi": "3.0.3",
		"info": map[string]interface{}{
			"title":   "Function Contract API",
			"version": "1.0.0",
		},
		"paths": map[string]interface{}{
			"/reward/batch-grant": map[string]interface{}{
				"post": map[string]interface{}{
					"operationId":  "reward.batch_grant",
					"x-resource":   "reward",
					"x-operation":  "batch_grant",
					"x-capability": "task",
					"x-execution":  "task",
					"x-approval": map[string]interface{}{
						"required":  true,
						"policyKey": "two_person",
					},
					"x-risk":       "warning",
					"x-permission": "reward:grant",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "Success"},
					},
				},
			},
		},
	}

	resp, err := service.CreateSource(openAPITestContext(), &OpenAPISourceCreateRequest{Spec: rawSpec(t, specDoc)})
	require.NoError(t, err)
	require.Len(t, resp.Source.Operations, 1)

	op := resp.Source.Operations[0]
	assert.Equal(t, "reward.batch_grant", op.OperationID)
	assert.Equal(t, "reward", op.Resource)
	assert.Equal(t, "batch_grant", op.Operation)
	assert.Equal(t, dashspec.CapabilityTask, op.Capability)
	assert.Equal(t, dashspec.FunctionExecutionTask, op.Execution)
	assert.True(t, op.Approval.Required)
	assert.Equal(t, "two_person", op.Approval.PolicyKey)
	assert.Equal(t, dashspec.RiskWarning, op.Risk)
	assert.Equal(t, "reward:grant", op.Permission)
}

func TestService_CreateSource_MultipleOperations(t *testing.T) {
	t.Parallel()

	service := setupOpenAPITestService(t)

	spec := map[string]interface{}{
		"openapi": "3.0.3",
		"info": map[string]interface{}{
			"title":   "Test API",
			"version": "1.0.0",
		},
		"paths": map[string]interface{}{
			"/users": map[string]interface{}{
				"get": map[string]interface{}{
					"operationId": "getUsers",
					"summary":     "Get all users",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Success",
						},
					},
				},
				"post": map[string]interface{}{
					"operationId": "createUser",
					"summary":     "Create user",
					"responses": map[string]interface{}{
						"201": map[string]interface{}{
							"description": "Created",
						},
					},
				},
			},
		},
	}

	resp, err := service.CreateSource(openAPITestContext(), &OpenAPISourceCreateRequest{Spec: rawSpec(t, spec)})
	require.NoError(t, err)
	assert.Len(t, resp.Source.Operations, 2)
}

func TestService_OpenAPISourceListDiagnosticsAndBinding(t *testing.T) {
	t.Parallel()

	service := setupOpenAPITestService(t)
	spec := map[string]interface{}{
		"openapi": "3.0.3",
		"info": map[string]interface{}{
			"title":   "Player API",
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
	created, err := service.CreateSource(openAPITestContext(), &OpenAPISourceCreateRequest{Spec: rawSpec(t, spec)})
	require.NoError(t, err)

	list, err := service.ListSources(openAPITestContext(), &OpenAPISourceListRequest{})
	require.NoError(t, err)
	require.Len(t, list.Items, 1)
	assert.Equal(t, created.Source.SourceID, list.Items[0].SourceID)
	assert.Equal(t, 1, list.Items[0].OperationCount)

	diags, err := service.SourceDiagnostics(openAPITestContext(), &OpenAPISourceGetRequest{SourceID: created.Source.SourceID})
	require.NoError(t, err)
	assertSourceDiagnostic(t, diags.Diagnostics, "rest_capability_inferred", dashspec.SeverityInfo)

	binding, err := service.CreateBinding(openAPITestContext(), &OpenAPISourceBindingCreateRequest{
		SourceID:    created.Source.SourceID,
		OperationID: "player.list",
		Kind:        "provider",
		FunctionID:  "player.list",
	})
	require.NoError(t, err)
	assert.Equal(t, "player.list", binding.Binding.OperationID)
	assert.Equal(t, "player.list", binding.Binding.FunctionID)

	detail, err := service.GetSource(openAPITestContext(), &OpenAPISourceGetRequest{SourceID: created.Source.SourceID})
	require.NoError(t, err)
	require.Len(t, detail.Source.Operations, 1)
	assert.True(t, detail.Source.Operations[0].Bound)
	assert.Equal(t, "player.list", detail.Source.Operations[0].FunctionID)
}

func TestService_CreateBindingRebuildsContractAndProposal(t *testing.T) {
	t.Parallel()

	service := setupOpenAPITestService(t)
	specDoc := map[string]interface{}{
		"openapi": "3.0.3",
		"info": map[string]interface{}{
			"title":   "Player API",
			"version": "1.0.0",
		},
		"paths": map[string]interface{}{
			"/player": map[string]interface{}{
				"get": map[string]interface{}{
					"operationId": "player.list",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "OK",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"items": map[string]interface{}{
												"type": "array",
												"items": map[string]interface{}{
													"type": "object",
													"properties": map[string]interface{}{
														"id":   map[string]interface{}{"type": "string"},
														"name": map[string]interface{}{"type": "string"},
													},
												},
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
	created, err := service.CreateSource(openAPITestContext(), &OpenAPISourceCreateRequest{Spec: rawSpec(t, specDoc)})
	require.NoError(t, err)

	_, err = service.CreateBinding(openAPITestContext(), &OpenAPISourceBindingCreateRequest{
		SourceID:    created.Source.SourceID,
		OperationID: "player.list",
		Kind:        "provider",
		FunctionID:  "player.list",
	})
	require.NoError(t, err)

	contract, err := model.NewFunctionContractModel(service.svcCtx.DB).FindByScopeAndFunctionID(openAPITestContext(), "demo-game", "development", "player.list")
	require.NoError(t, err)
	assert.Equal(t, "openapi", contract.Source)
	assert.Equal(t, "player", contract.ResourceKey)
	assert.Equal(t, "list", contract.OperationKey)
	assert.Equal(t, string(dashspec.CapabilityCollectionQuery), contract.Capability)

	semantics, err := model.NewCapabilitySemanticsModel(service.svcCtx.DB).FindByScopeAndResourceKey(openAPITestContext(), "demo-game", "development", "player")
	require.NoError(t, err)
	assert.Equal(t, string(dashspec.SemanticSourceOpenAPIRest), semantics.Source)
	assert.Equal(t, "id", semantics.IdentityField)

	proposal, err := model.NewPageProposalModel(service.svcCtx.DB).FindByScopeAndKey(openAPITestContext(), "demo-game", "development", "resource:player")
	require.NoError(t, err)
	assert.Equal(t, "resource--player", proposal.PageKey)
	assert.NotEmpty(t, proposal.PageSpec)
}

func TestService_CreateBindingRejectsHttpConnectorWithoutPersisting(t *testing.T) {
	t.Parallel()

	service, ctx, auditStore := setupOpenAPITestServiceWithAudit(t, "openapi_sources:read", "openapi_sources:write")
	spec := map[string]interface{}{
		"openapi": "3.0.3",
		"info": map[string]interface{}{
			"title":   "Player HTTP API",
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
	created, err := service.CreateSource(ctx, &OpenAPISourceCreateRequest{Spec: rawSpec(t, spec)})
	require.NoError(t, err)

	_, err = service.CreateBinding(ctx, &OpenAPISourceBindingCreateRequest{
		SourceID:    created.Source.SourceID,
		BindingID:   "player-http",
		OperationID: "player.list",
		Kind:        "httpConnector",
		FunctionID:  "player.list",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "httpConnector requires allowlist")

	bindings, err := service.svcCtx.OpenAPISourceBindingModel.ListBySource(ctx, "demo-game", "development", created.Source.SourceID)
	require.NoError(t, err)
	assert.Empty(t, bindings, "disabled httpConnector must not persist a half-created binding")

	records, total, err := auditStore.List(audit.AuditFilter{}, audit.AuditPage{PageSize: 10})
	require.NoError(t, err)
	require.Equal(t, 1, total, "failed binding creation must not write success audit records")
	require.Len(t, records, 1)
	assert.Equal(t, audit.EventOpenAPISourceCreate, records[0].EventType)
}

func TestService_GetDocument(t *testing.T) {
	t.Parallel()

	service := setupOpenAPITestService(t)

	// Add an operation
	op := &openapi3.Operation{
		OperationID: "testOperation",
		Summary:     "Test operation",
	}
	service.svcCtx.RegistryStore.UpsertOpenAPI("testOperation", op)

	resp, err := service.GetDocument(context.Background(), &GetDocumentRequest{})
	require.NoError(t, err)
	assert.NotNil(t, resp.Spec)
}

func TestService_BatchGetSpec_MultipleIDs(t *testing.T) {
	t.Parallel()

	service := setupOpenAPITestService(t)

	// Add operations
	op1 := &openapi3.Operation{OperationID: "func1", Summary: "Function 1"}
	op2 := &openapi3.Operation{OperationID: "func2", Summary: "Function 2"}
	service.svcCtx.RegistryStore.UpsertOpenAPI("func1", op1)
	service.svcCtx.RegistryStore.UpsertOpenAPI("func2", op2)

	resp, err := service.BatchGetSpec(context.Background(), &BatchGetSpecRequest{
		FunctionIDs: []string{"func1", "func2", "player.list"},
	})
	require.NoError(t, err)
	assert.Len(t, resp, 3)
	assert.NotNil(t, resp["func1"])
	assert.NotNil(t, resp["func2"])
	assert.NotNil(t, resp["player.list"])
}

func TestService_BatchGetSpec_WithNonExistent(t *testing.T) {
	t.Parallel()

	service := setupOpenAPITestService(t)

	resp, err := service.BatchGetSpec(context.Background(), &BatchGetSpecRequest{
		FunctionIDs: []string{"nonexistent"},
	})
	require.NoError(t, err)
	assert.Nil(t, resp["nonexistent"])
}

func TestService_BatchGetSpec_EmptyIDs(t *testing.T) {
	t.Parallel()

	service := setupOpenAPITestService(t)

	resp, err := service.BatchGetSpec(context.Background(), &BatchGetSpecRequest{
		FunctionIDs: []string{},
	})
	require.NoError(t, err)
	assert.Empty(t, resp)
}

func TestService_BatchGetSpec_WithEmptyStringID(t *testing.T) {
	t.Parallel()

	service := setupOpenAPITestService(t)

	resp, err := service.BatchGetSpec(context.Background(), &BatchGetSpecRequest{
		FunctionIDs: []string{"", "player.list"},
	})
	require.NoError(t, err)
	// Empty string should be skipped
	assert.Len(t, resp, 1)
}

// Handler Tests

func TestHandler_GetSpec_Success(t *testing.T) {
	t.Parallel()

	_, router := setupOpenAPITestHandler(t)

	req, _ := http.NewRequest("GET", "/spec/player.list", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandler_GetSpec_NotFound(t *testing.T) {
	t.Parallel()

	_, router := setupOpenAPITestHandler(t)

	req, _ := http.NewRequest("GET", "/spec/nonexistent", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Handler might return 500 for not found
	assert.True(t, w.Code == http.StatusNotFound || w.Code == http.StatusInternalServerError)
}

func TestHandler_CreateSource_Success(t *testing.T) {
	t.Parallel()

	_, router := setupOpenAPITestHandler(t)

	spec := map[string]interface{}{
		"openapi": "3.0.3",
		"info": map[string]interface{}{
			"title":   "Test API",
			"version": "1.0.0",
		},
		"paths": map[string]interface{}{
			"/users": map[string]interface{}{
				"get": map[string]interface{}{
					"operationId": "getUsers",
					"responses":   map[string]interface{}{"200": map[string]interface{}{"description": "OK"}},
				},
			},
		},
	}

	body, _ := json.Marshal(spec)
	req, _ := http.NewRequest("POST", "/sources", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	var resp OpenAPISourceGetResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Len(t, resp.Source.Operations, 1)
}

func TestHandler_UpdateSource_Success(t *testing.T) {
	t.Parallel()

	_, router := setupOpenAPITestHandler(t)
	initialSpec := map[string]interface{}{
		"openapi": "3.0.3",
		"info": map[string]interface{}{
			"title":   "Handler API",
			"version": "1.0.0",
		},
		"paths": map[string]interface{}{
			"/users": map[string]interface{}{
				"get": map[string]interface{}{
					"operationId": "getUsers",
					"responses":   map[string]interface{}{"200": map[string]interface{}{"description": "OK"}},
				},
			},
		},
	}
	body, _ := json.Marshal(OpenAPISourceCreateRequest{Spec: rawSpec(t, initialSpec)})
	req, _ := http.NewRequest("POST", "/sources", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code)
	var created OpenAPISourceGetResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))

	nextSpec := map[string]interface{}{
		"openapi": "3.0.3",
		"info": map[string]interface{}{
			"title":   "Handler API v2",
			"version": "2.0.0",
		},
		"paths": map[string]interface{}{
			"/users/detail": map[string]interface{}{
				"get": map[string]interface{}{
					"operationId": "getUser",
					"responses":   map[string]interface{}{"200": map[string]interface{}{"description": "OK"}},
				},
			},
		},
	}
	updateBody, _ := json.Marshal(OpenAPISourceUpdateRequest{
		Name: "Renamed Handler API",
		Spec: rawSpec(t, nextSpec),
	})
	req, _ = http.NewRequest("PUT", "/sources/"+created.Source.SourceID, strings.NewReader(string(updateBody)))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var updated OpenAPISourceGetResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &updated))
	assert.Equal(t, 2, updated.Source.Revision)
	assert.Equal(t, "Renamed Handler API", updated.Source.Name)
	require.Len(t, updated.Source.Operations, 1)
	assert.Equal(t, "getUser", updated.Source.Operations[0].OperationID)
}

func TestHandler_GetDocument_Success(t *testing.T) {
	t.Parallel()

	_, router := setupOpenAPITestHandler(t)

	req, _ := http.NewRequest("GET", "/document", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandler_BatchGetSpec_Success(t *testing.T) {
	t.Parallel()

	_, router := setupOpenAPITestHandler(t)

	reqBody := BatchGetSpecRequest{
		FunctionIDs: []string{"player.list", "player.get"},
	}
	body, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST", "/batch/spec", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp BatchGetSpecResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Len(t, resp, 2)
}

func TestHandler_BatchGetSpec_MissingBody(t *testing.T) {
	t.Parallel()

	_, router := setupOpenAPITestHandler(t)

	req, _ := http.NewRequest("POST", "/batch/spec", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Handler might return 500 for missing body
	assert.True(t, w.Code == http.StatusBadRequest || w.Code == http.StatusInternalServerError)
}

// Tests for hasRegisteredFunction through public API

func TestHasRegisteredFunction_WithRegistryStore(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	store := registry.NewStore()
	store.UpsertAgent(&registry.AgentSession{
		AgentID: "agent-1",
		Functions: map[string]registry.FunctionMeta{
			"test.func": {Enabled: true},
		},
	})

	svcCtx := &svc.ServiceContext{
		DB:            db,
		RegistryStore: store,
		FunctionModel: model.NewFunctionModel(db),
	}

	// Test through GetSpec which calls hasRegisteredFunction
	resp, err := NewService(svcCtx).GetSpec(context.Background(), &GetSpecRequest{ID: "test.func"})
	require.NoError(t, err)
	assert.NotNil(t, resp.Spec)
}

func TestHasRegisteredFunction_NilSvcCtx(t *testing.T) {
	t.Parallel()

	// Test with nil context
	service := NewService(&svc.ServiceContext{
		RegistryStore: registry.NewStore(),
	})

	_, err := service.GetSpec(context.Background(), &GetSpecRequest{ID: "test"})
	assert.Error(t, err)
}

func TestNormalizeOpenAPIDoc_NilDoc(t *testing.T) {
	t.Parallel()

	normalizeOpenAPIDoc(nil)
	// Should not panic
}

func TestNormalizeOpenAPIDoc_NilPaths(t *testing.T) {
	t.Parallel()

	doc := &openapi3.T{}
	normalizeOpenAPIDoc(doc)
	// Should not panic
}

func TestNormalizeOpenAPIDoc_MissingResponseDescription(t *testing.T) {
	t.Parallel()

	// Test normalization through Source upload which calls normalizeOpenAPIDoc
	service := setupOpenAPITestService(t)

	// Spec with missing response descriptions
	spec := map[string]interface{}{
		"openapi": "3.0.3",
		"info": map[string]interface{}{
			"title":   "Test API",
			"version": "1.0.0",
		},
		"paths": map[string]interface{}{
			"/users": map[string]interface{}{
				"get": map[string]interface{}{
					"operationId": "getUsers",
					"responses": map[string]interface{}{
						// Missing description for 200
						"200": map[string]interface{}{},
					},
				},
			},
		},
	}

	resp, err := service.CreateSource(openAPITestContext(), &OpenAPISourceCreateRequest{Spec: rawSpec(t, spec)})
	require.NoError(t, err)
	assert.Len(t, resp.Source.Operations, 1)
}
