package function

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/getkin/kin-openapi/openapi3"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// ---------------------------------------------------------------------------
// Helper to set up a full test context with DB, models, and optional registry
// ---------------------------------------------------------------------------

func setupFullTestContext(t *testing.T) (*svc.ServiceContext, context.Context) {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))

	svcCtx := &svc.ServiceContext{
		DB:              db,
		FunctionModel:   model.NewFunctionModel(db),
		AdminModel:      model.NewAdminModel(db),
		RoleModel:       model.NewRoleModel(db),
		PermissionModel: model.NewPermissionModel(db),
		RegistryStore:   reg.NewStore(),
	}

	// Create test admin with admin role
	admin := &model.Admin{Username: "testadmin", Status: 1}
	err = svcCtx.AdminModel.Create(context.Background(), admin, "password")
	require.NoError(t, err)

	role := &model.Role{Name: "admin", Description: "Admin"}
	err = svcCtx.RoleModel.Create(context.Background(), role)
	require.NoError(t, err)

	err = svcCtx.AdminModel.AssignRole(context.Background(), admin.ID, role.ID)
	require.NoError(t, err)

	err = svcCtx.RoleModel.ReplacePermissions(context.Background(), role.ID, []string{"admin:all"})
	require.NoError(t, err)

	ctx := context.WithValue(context.Background(), "username", "testadmin")
	return svcCtx, ctx
}

func setupNoAuthTestContext(t *testing.T) (*svc.ServiceContext, context.Context) {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))

	svcCtx := &svc.ServiceContext{
		DB:              db,
		FunctionModel:   model.NewFunctionModel(db),
		AdminModel:      model.NewAdminModel(db),
		RoleModel:       model.NewRoleModel(db),
		PermissionModel: model.NewPermissionModel(db),
		RegistryStore:   reg.NewStore(),
	}
	return svcCtx, context.Background()
}

// ---------------------------------------------------------------------------
// extractOperationRequestSchema & schemaRefToMap
// ---------------------------------------------------------------------------

func TestExtractOperationRequestSchema_Nil(t *testing.T) {
	assert.Nil(t, extractOperationRequestSchema(nil))
}

func TestExtractOperationRequestSchema_NilRequestBody(t *testing.T) {
	op := &openapi3.Operation{}
	assert.Nil(t, extractOperationRequestSchema(op))
}

func TestExtractOperationRequestSchema_EmptyContent(t *testing.T) {
	op := &openapi3.Operation{
		RequestBody: &openapi3.RequestBodyRef{
			Value: &openapi3.RequestBody{
				Content: openapi3.Content{},
			},
		},
	}
	assert.Nil(t, extractOperationRequestSchema(op))
}

func TestExtractOperationRequestSchema_WithJSON(t *testing.T) {
	schemaType := openapi3.Types{"object"}
	op := &openapi3.Operation{
		RequestBody: &openapi3.RequestBodyRef{
			Value: &openapi3.RequestBody{
				Content: openapi3.Content{
					"application/json": &openapi3.MediaType{
						Schema: &openapi3.SchemaRef{
							Value: &openapi3.Schema{
								Type: &schemaType,
							},
						},
					},
				},
			},
		},
	}
	result := extractOperationRequestSchema(op)
	assert.NotNil(t, result)
	assert.Equal(t, "object", result["type"])
}

func TestExtractOperationRequestSchema_WithNonJSONContentType(t *testing.T) {
	schemaType := openapi3.Types{"object"}
	op := &openapi3.Operation{
		RequestBody: &openapi3.RequestBodyRef{
			Value: &openapi3.RequestBody{
				Content: openapi3.Content{
					"text/plain": &openapi3.MediaType{
						Schema: &openapi3.SchemaRef{
							Value: &openapi3.Schema{
								Type: &schemaType,
							},
						},
					},
				},
			},
		},
	}
	result := extractOperationRequestSchema(op)
	assert.NotNil(t, result)
}

func TestExtractOperationRequestSchema_NilMedia(t *testing.T) {
	op := &openapi3.Operation{
		RequestBody: &openapi3.RequestBodyRef{
			Value: &openapi3.RequestBody{
				Content: openapi3.Content{
					"application/json": nil,
				},
			},
		},
	}
	assert.Nil(t, extractOperationRequestSchema(op))
}

func TestSchemaRefToMap_Nil(t *testing.T) {
	assert.Nil(t, schemaRefToMap(nil))
}

func TestSchemaRefToMap_WithValue(t *testing.T) {
	schemaType := openapi3.Types{"string"}
	ref := &openapi3.SchemaRef{
		Value: &openapi3.Schema{
			Type: &schemaType,
		},
	}
	result := schemaRefToMap(ref)
	assert.NotNil(t, result)
	assert.Equal(t, "string", result["type"])
}

func TestSchemaRefToMap_WithRef(t *testing.T) {
	ref := &openapi3.SchemaRef{
		Ref: "#/components/schemas/Player",
	}
	result := schemaRefToMap(ref)
	assert.NotNil(t, result)
	assert.Equal(t, "#/components/schemas/Player", result["$ref"])
}

func TestSchemaRefToMap_NilValueAndRef(t *testing.T) {
	ref := &openapi3.SchemaRef{}
	assert.Nil(t, schemaRefToMap(ref))
}

// ---------------------------------------------------------------------------
// invokePayloadBytes & jsonValid
// ---------------------------------------------------------------------------

func TestInvokePayloadBytes_NilReq(t *testing.T) {
	result, err := invokePayloadBytes(nil)
	require.NoError(t, err)
	assert.Equal(t, []byte("null"), result)
}

func TestInvokePayloadBytes_WithPayload(t *testing.T) {
	req := &FunctionInvokeRequest{
		Payload: json.RawMessage(`{"key":"value"}`),
	}
	result, err := invokePayloadBytes(req)
	require.NoError(t, err)
	assert.JSONEq(t, `{"key":"value"}`, string(result))
}

func TestInvokePayloadBytes_WithParams(t *testing.T) {
	req := &FunctionInvokeRequest{
		Params: json.RawMessage(`{"param1":"val1"}`),
	}
	result, err := invokePayloadBytes(req)
	require.NoError(t, err)
	assert.JSONEq(t, `{"param1":"val1"}`, string(result))
}

func TestInvokePayloadBytes_Empty(t *testing.T) {
	req := &FunctionInvokeRequest{}
	result, err := invokePayloadBytes(req)
	require.NoError(t, err)
	assert.Equal(t, []byte("{}"), result)
}

func TestInvokePayloadBytes_InvalidPayload(t *testing.T) {
	req := &FunctionInvokeRequest{
		Payload: json.RawMessage(`not json`),
	}
	_, err := invokePayloadBytes(req)
	assert.Error(t, err)
}

func TestInvokePayloadBytes_InvalidParams(t *testing.T) {
	req := &FunctionInvokeRequest{
		Params: json.RawMessage(`not json`),
	}
	_, err := invokePayloadBytes(req)
	assert.Error(t, err)
}

func TestJsonValid(t *testing.T) {
	assert.True(t, jsonValid(json.RawMessage(`{"a":1}`)))
	assert.True(t, jsonValid(json.RawMessage(`"hello"`)))
	assert.False(t, jsonValid(json.RawMessage(`not json`)))
	assert.False(t, jsonValid(nil))
	assert.False(t, jsonValid([]byte{}))
}

// ---------------------------------------------------------------------------
// buildBroadcastResponse
// ---------------------------------------------------------------------------

func TestBuildBroadcastResponse_Nil(t *testing.T) {
	result := buildBroadcastResponse(nil)
	assert.NotNil(t, result)
	assert.NotNil(t, result.Broadcast)
	assert.Equal(t, 0, result.Broadcast.Total)
}

// ---------------------------------------------------------------------------
// toString helper
// ---------------------------------------------------------------------------

func TestToString(t *testing.T) {
	assert.Equal(t, "", toString(nil))
	assert.Equal(t, "hello", toString("hello"))
	assert.Equal(t, "", toString(42))
	assert.Equal(t, "", toString(true))
}

// ---------------------------------------------------------------------------
// backfillFromRegistry
// ---------------------------------------------------------------------------

func TestBackfillFromRegistry_NilSvcCtx(t *testing.T) {
	fn := &model.Function{}
	backfillFromRegistry(nil, "test.fn", fn)
}

func TestBackfillFromRegistry_NilRegistryStore(t *testing.T) {
	svcCtx := &svc.ServiceContext{}
	fn := &model.Function{}
	backfillFromRegistry(svcCtx, "test.fn", fn)
}

func TestBackfillFromRegistry_WithMatchingFunction(t *testing.T) {
	store := reg.NewStore()
	store.UpsertAgent(&reg.AgentSession{
		AgentID: "agent-1",
		GameID:  "game-1",
		Functions: map[string]reg.FunctionMeta{
			"player.ban": {
				Enabled:  true,
				Version:  "2.0.0",
				Summary:  "Ban a player",
				Tags:     []string{"admin"},
				Resource: "player",
			},
		},
	})
	svcCtx := &svc.ServiceContext{RegistryStore: store}
	fn := &model.Function{}
	backfillFromRegistry(svcCtx, "player.ban", fn)

	assert.Equal(t, "Ban a player", fn.Description)
	assert.Equal(t, "player", fn.Resource)
	assert.Equal(t, "2.0.0", fn.Version)
}

func TestBackfillFromRegistry_NoMatch(t *testing.T) {
	store := reg.NewStore()
	store.UpsertAgent(&reg.AgentSession{
		AgentID:   "agent-1",
		GameID:    "game-1",
		Functions: map[string]reg.FunctionMeta{"other.fn": {}},
	})
	svcCtx := &svc.ServiceContext{RegistryStore: store}
	fn := &model.Function{}
	backfillFromRegistry(svcCtx, "player.ban", fn)
	assert.Equal(t, "", fn.Description)
}

func TestBackfillFromRegistry_PreservesExistingFields(t *testing.T) {
	store := reg.NewStore()
	store.UpsertAgent(&reg.AgentSession{
		AgentID: "agent-1",
		GameID:  "game-1",
		Functions: map[string]reg.FunctionMeta{
			"player.ban": {
				Enabled:  true,
				Version:  "2.0.0",
				Summary:  "Ban",
				Resource: "player",
			},
		},
	})
	svcCtx := &svc.ServiceContext{RegistryStore: store}
	fn := &model.Function{
		Description: "Existing description",
		Resource:    "existing",
		Version:     "1.0.0",
	}
	backfillFromRegistry(svcCtx, "player.ban", fn)
	assert.Equal(t, "Existing description", fn.Description)
	assert.Equal(t, "existing", fn.Resource)
	assert.Equal(t, "1.0.0", fn.Version)
}

// ---------------------------------------------------------------------------
// FunctionCopy
// ---------------------------------------------------------------------------

func TestFunctionCopy(t *testing.T) {
	svcCtx, ctx := setupFullTestContext(t)
	fn := &model.Function{
		FunctionID: "player.ban",
		Name:       "Ban Player",
		Status:     1,
	}
	require.NoError(t, svcCtx.FunctionModel.Create(ctx, fn))

	logic := NewFunctionCopyLogic(ctx, svcCtx)
	resp, err := logic.FunctionCopy(&FunctionCopyRequest{ID: "player.ban"})
	require.NoError(t, err)
	assert.Equal(t, "player.ban", resp.FunctionId)
	assert.NotEmpty(t, resp.NewId)
}

func TestFunctionCopy_EmptyID(t *testing.T) {
	svcCtx, ctx := setupFullTestContext(t)
	logic := NewFunctionCopyLogic(ctx, svcCtx)
	_, err := logic.FunctionCopy(&FunctionCopyRequest{ID: ""})
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// FunctionDelete
// ---------------------------------------------------------------------------

func TestFunctionDelete(t *testing.T) {
	svcCtx, ctx := setupFullTestContext(t)
	fn := &model.Function{
		FunctionID: "player.ban",
		Name:       "Ban Player",
		Status:     1,
	}
	require.NoError(t, svcCtx.FunctionModel.Create(ctx, fn))

	logic := NewFunctionDeleteLogic(ctx, svcCtx)
	err := logic.FunctionDelete(&FunctionActionRequest{ID: "player.ban"})
	assert.NoError(t, err)

	_, err = svcCtx.FunctionModel.FindByFunctionID(ctx, "player.ban")
	assert.Error(t, err)
}

func TestFunctionDelete_EmptyID(t *testing.T) {
	svcCtx, ctx := setupFullTestContext(t)
	logic := NewFunctionDeleteLogic(ctx, svcCtx)
	err := logic.FunctionDelete(&FunctionActionRequest{ID: ""})
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// FunctionEnable
// ---------------------------------------------------------------------------

func TestFunctionEnable(t *testing.T) {
	svcCtx, ctx := setupFullTestContext(t)
	fn := &model.Function{
		FunctionID: "player.ban",
		Name:       "Ban Player",
		Status:     0,
	}
	require.NoError(t, svcCtx.FunctionModel.Create(ctx, fn))

	logic := NewFunctionEnableLogic(ctx, svcCtx)
	err := logic.FunctionEnable(&FunctionActionRequest{ID: "player.ban"})
	assert.NoError(t, err)

	updated, err := svcCtx.FunctionModel.FindByFunctionID(ctx, "player.ban")
	require.NoError(t, err)
	assert.Equal(t, model.StatusEnabled, updated.Status)
}

// ---------------------------------------------------------------------------
// FunctionDisable
// ---------------------------------------------------------------------------

func TestFunctionDisable(t *testing.T) {
	svcCtx, ctx := setupFullTestContext(t)
	fn := &model.Function{
		FunctionID: "player.ban",
		Name:       "Ban Player",
		Status:     1,
	}
	require.NoError(t, svcCtx.FunctionModel.Create(ctx, fn))

	logic := NewFunctionDisableLogic(ctx, svcCtx)
	err := logic.FunctionDisable(&FunctionActionRequest{ID: "player.ban"})
	assert.NoError(t, err)

	updated, err := svcCtx.FunctionModel.FindByFunctionID(ctx, "player.ban")
	require.NoError(t, err)
	assert.Equal(t, model.StatusDisabled, updated.Status)
}

// ---------------------------------------------------------------------------
// FunctionPublish
// ---------------------------------------------------------------------------

func TestFunctionPublish(t *testing.T) {
	svcCtx, ctx := setupFullTestContext(t)
	fn := &model.Function{
		FunctionID: "player.ban",
		Name:       "Ban Player",
		Status:     0,
	}
	require.NoError(t, svcCtx.FunctionModel.Create(ctx, fn))

	logic := NewFunctionPublishLogic(ctx, svcCtx)
	resp, err := logic.FunctionPublish(&FunctionPublishRequest{ID: "player.ban"})
	require.NoError(t, err)
	assert.True(t, resp.Published)

	updated, err := svcCtx.FunctionModel.FindByFunctionID(ctx, "player.ban")
	require.NoError(t, err)
	assert.Equal(t, model.StatusEnabled, updated.Status)
}

func TestFunctionPublish_NotFound(t *testing.T) {
	svcCtx, ctx := setupFullTestContext(t)
	logic := NewFunctionPublishLogic(ctx, svcCtx)
	_, err := logic.FunctionPublish(&FunctionPublishRequest{ID: "nonexistent"})
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// FunctionDetail
// ---------------------------------------------------------------------------

func TestFunctionDetail_FromDB(t *testing.T) {
	svcCtx, ctx := setupFullTestContext(t)
	fn := &model.Function{
		FunctionID: "player.ban",
		Name:       "Ban Player",
		Status:     1,
		Version:    "1.0.0",
	}
	require.NoError(t, svcCtx.FunctionModel.Create(ctx, fn))

	logic := NewFunctionDetailLogic(ctx, svcCtx)
	resp, err := logic.FunctionDetail(&FunctionDetailRequest{ID: "player.ban"})
	require.NoError(t, err)
	assert.Equal(t, "player.ban", resp.Function.ID)
	assert.Equal(t, "Ban Player", resp.Function.Name)
	assert.Equal(t, 1, resp.Function.Status)
}

func TestFunctionDetail_FromRuntime(t *testing.T) {
	svcCtx, ctx := setupFullTestContext(t)
	svcCtx.RegistryStore.UpsertAgent(&reg.AgentSession{
		AgentID: "agent-1",
		GameID:  "game-1",
		Functions: map[string]reg.FunctionMeta{
			"player.ban": {
				Enabled: true,
				Version: "2.0.0",
			},
		},
	})

	logic := NewFunctionDetailLogic(ctx, svcCtx)
	resp, err := logic.FunctionDetail(&FunctionDetailRequest{ID: "player.ban"})
	require.NoError(t, err)
	assert.Equal(t, "player.ban", resp.Function.ID)

	fn, err := svcCtx.FunctionModel.FindByFunctionID(ctx, "player.ban")
	assert.NoError(t, err)
	assert.NotNil(t, fn)
}

func TestFunctionDetail_NotFound(t *testing.T) {
	svcCtx, ctx := setupFullTestContext(t)
	logic := NewFunctionDetailLogic(ctx, svcCtx)
	_, err := logic.FunctionDetail(&FunctionDetailRequest{ID: "nonexistent"})
	assert.Error(t, err)
}

func TestFunctionDetail_WithDescriptors(t *testing.T) {
	svcCtx, ctx := setupFullTestContext(t)
	fn := &model.Function{
		FunctionID: "player.ban",
		Name:       "Ban Player",
		Status:     1,
		Version:    "1.0.0",
	}
	require.NoError(t, svcCtx.FunctionModel.Create(ctx, fn))

	desc := &model.FunctionDescriptor{
		FunctionID: "player.ban",
		Input:      datatypes.JSONMap{"type": "object"},
		Output:     datatypes.JSONMap{"type": "object"},
		Schema:     datatypes.JSONMap{"type": "object"},
	}
	require.NoError(t, svcCtx.FunctionModel.UpsertDescriptor(ctx, desc))

	logic := NewFunctionDetailLogic(ctx, svcCtx)
	resp, err := logic.FunctionDetail(&FunctionDetailRequest{ID: "player.ban"})
	require.NoError(t, err)
	assert.NotNil(t, resp.Descriptor.Input)
	assert.NotNil(t, resp.Descriptor.Output)
	assert.NotNil(t, resp.Descriptor.Schema)
}

func TestFunctionDetail_EmptyNameFallsBackToID(t *testing.T) {
	svcCtx, ctx := setupFullTestContext(t)
	fn := &model.Function{
		FunctionID: "player.ban",
		Name:       "",
		Status:     1,
	}
	require.NoError(t, svcCtx.FunctionModel.Create(ctx, fn))

	logic := NewFunctionDetailLogic(ctx, svcCtx)
	resp, err := logic.FunctionDetail(&FunctionDetailRequest{ID: "player.ban"})
	require.NoError(t, err)
	assert.Equal(t, "player.ban", resp.Function.Name)
}

func TestFunctionDetail_RuntimeFallbackFields(t *testing.T) {
	svcCtx, ctx := setupFullTestContext(t)
	fn := &model.Function{
		FunctionID: "player.ban",
		Name:       "",
		Status:     1,
		GameID:     "",
		Version:    "",
		Instances:  0,
	}
	require.NoError(t, svcCtx.FunctionModel.Create(ctx, fn))

	svcCtx.RegistryStore.UpsertAgent(&reg.AgentSession{
		AgentID:   "agent-1",
		GameID:    "game-1",
		ExpireAt:  time.Now().Add(time.Hour),
		Functions: map[string]reg.FunctionMeta{"player.ban": {Version: "3.0.0"}},
	})

	logic := NewFunctionDetailLogic(ctx, svcCtx)
	resp, err := logic.FunctionDetail(&FunctionDetailRequest{ID: "player.ban"})
	require.NoError(t, err)
	assert.Equal(t, "game-1", resp.Function.GameId)
	assert.Equal(t, "3.0.0", resp.Function.Version)
}

// ---------------------------------------------------------------------------
// loadRuntimeFunctionDetail
// ---------------------------------------------------------------------------

func TestLoadRuntimeFunctionDetail_NilStore(t *testing.T) {
	result := loadRuntimeFunctionDetail(nil, "test.fn")
	assert.Nil(t, result)
}

func TestLoadRuntimeFunctionDetail_EmptyStore(t *testing.T) {
	store := reg.NewStore()
	result := loadRuntimeFunctionDetail(store, "test.fn")
	assert.Nil(t, result)
}

func TestLoadRuntimeFunctionDetail_WithFunction(t *testing.T) {
	store := reg.NewStore()
	store.UpsertAgent(&reg.AgentSession{
		AgentID:   "agent-1",
		GameID:    "game-1",
		ExpireAt:  time.Now().Add(time.Hour),
		Functions: map[string]reg.FunctionMeta{"player.ban": {Version: "1.0.0"}},
	})
	result := loadRuntimeFunctionDetail(store, "player.ban")
	require.NotNil(t, result)
	assert.Equal(t, "1.0.0", result.version)
	assert.Equal(t, "game-1", result.gameID)
	assert.Equal(t, 1, result.instances)
}

func TestLoadRuntimeFunctionDetail_MultipleAgents(t *testing.T) {
	store := reg.NewStore()
	store.UpsertAgent(&reg.AgentSession{
		AgentID:   "agent-1",
		GameID:    "game-1",
		ExpireAt:  time.Now().Add(time.Hour),
		Functions: map[string]reg.FunctionMeta{"player.ban": {Version: "1.0.0"}},
	})
	store.UpsertAgent(&reg.AgentSession{
		AgentID:   "agent-2",
		GameID:    "game-2",
		ExpireAt:  time.Now().Add(time.Hour),
		Functions: map[string]reg.FunctionMeta{"player.ban": {Version: "2.0.0"}},
	})
	result := loadRuntimeFunctionDetail(store, "player.ban")
	require.NotNil(t, result)
	assert.Equal(t, 2, result.instances)
}

func TestLoadRuntimeFunctionDetail_WithOpenAPI(t *testing.T) {
	store := reg.NewStore()
	schemaType := openapi3.Types{"object"}
	store.UpsertAgent(&reg.AgentSession{
		AgentID:   "agent-1",
		GameID:    "game-1",
		ExpireAt:  time.Now().Add(time.Hour),
		Functions: map[string]reg.FunctionMeta{"player.ban": {Version: "1.0.0"}},
	})
	store.UpsertOpenAPI("player.ban", &openapi3.Operation{
		OperationID: "player.ban",
		RequestBody: &openapi3.RequestBodyRef{
			Value: &openapi3.RequestBody{
				Content: openapi3.Content{
					"application/json": &openapi3.MediaType{
						Schema: &openapi3.SchemaRef{
							Value: &openapi3.Schema{Type: &schemaType},
						},
					},
				},
			},
		},
		Responses: openapi3.NewResponses(),
	})
	result := loadRuntimeFunctionDetail(store, "player.ban")
	require.NotNil(t, result)
	assert.NotNil(t, result.descriptor.Schema)
	assert.NotNil(t, result.descriptor.Input)
	assert.NotNil(t, result.descriptor.Output)
}

func TestLoadRuntimeFunctionDetail_FallbackOpenAPI(t *testing.T) {
	store := reg.NewStore()
	store.UpsertAgent(&reg.AgentSession{
		AgentID:   "agent-1",
		GameID:    "game-1",
		ExpireAt:  time.Now().Add(time.Hour),
		Functions: map[string]reg.FunctionMeta{"player.ban": {Version: "1.0.0"}},
	})
	result := loadRuntimeFunctionDetail(store, "player.ban")
	require.NotNil(t, result)
	assert.NotNil(t, result.descriptor.Schema)
}

func TestLoadRuntimeFunctionDetail_FallbackOpenAPI_NilRequestBody(t *testing.T) {
	store := reg.NewStore()
	store.UpsertAgent(&reg.AgentSession{
		AgentID:   "agent-1",
		GameID:    "game-1",
		ExpireAt:  time.Now().Add(time.Hour),
		Functions: map[string]reg.FunctionMeta{"player.ban": {Version: "1.0.0"}},
	})
	store.UpsertOpenAPI("player.ban", &openapi3.Operation{
		OperationID: "player.ban",
	})
	result := loadRuntimeFunctionDetail(store, "player.ban")
	require.NotNil(t, result)
}

// ---------------------------------------------------------------------------
// FunctionInstancesAll
// ---------------------------------------------------------------------------

func TestFunctionInstancesAll_NilStore(t *testing.T) {
	svcCtx, ctx := setupNoAuthTestContext(t)
	svcCtx.RegistryStore = nil
	logic := NewFunctionInstancesAllLogic(ctx, svcCtx)
	resp, err := logic.FunctionInstancesAll()
	require.NoError(t, err)
	assert.Empty(t, resp.Instances)
}

func TestFunctionInstancesAll_EmptyStore(t *testing.T) {
	svcCtx, ctx := setupNoAuthTestContext(t)
	logic := NewFunctionInstancesAllLogic(ctx, svcCtx)
	resp, err := logic.FunctionInstancesAll()
	require.NoError(t, err)
	assert.Empty(t, resp.Instances)
}

func TestFunctionInstancesAll_WithProviders(t *testing.T) {
	svcCtx, ctx := setupNoAuthTestContext(t)
	svcCtx.RegistryStore.UpsertAgent(&reg.AgentSession{
		AgentID:  "agent-1",
		GameID:   "game-1",
		Env:      "prod",
		ExpireAt: time.Now().Add(time.Hour),
		Providers: []reg.ProviderSession{
			{
				ProviderID:  "p1",
				Addr:        "localhost:9090",
				Version:     "1.0",
				FunctionIDs: []string{"fn1", "fn2"},
			},
		},
	})
	logic := NewFunctionInstancesAllLogic(ctx, svcCtx)
	resp, err := logic.FunctionInstancesAll()
	require.NoError(t, err)
	assert.Len(t, resp.Instances, 2)
}

func TestFunctionInstancesAll_EmptyFunctionIDs(t *testing.T) {
	svcCtx, ctx := setupNoAuthTestContext(t)
	svcCtx.RegistryStore.UpsertAgent(&reg.AgentSession{
		AgentID:  "agent-1",
		GameID:   "game-1",
		ExpireAt: time.Now().Add(time.Hour),
		Providers: []reg.ProviderSession{
			{
				ProviderID:  "p1",
				Addr:        "localhost:9090",
				FunctionIDs: []string{},
			},
		},
	})
	logic := NewFunctionInstancesAllLogic(ctx, svcCtx)
	resp, err := logic.FunctionInstancesAll()
	require.NoError(t, err)
	assert.Empty(t, resp.Instances)
}

func TestFunctionInstancesAll_EmptyFunctionIDInList(t *testing.T) {
	svcCtx, ctx := setupNoAuthTestContext(t)
	svcCtx.RegistryStore.UpsertAgent(&reg.AgentSession{
		AgentID:  "agent-1",
		GameID:   "game-1",
		ExpireAt: time.Now().Add(time.Hour),
		Providers: []reg.ProviderSession{
			{
				ProviderID:  "p1",
				Addr:        "localhost:9090",
				FunctionIDs: []string{"", "fn1"},
			},
		},
	})
	logic := NewFunctionInstancesAllLogic(ctx, svcCtx)
	resp, err := logic.FunctionInstancesAll()
	require.NoError(t, err)
	assert.Len(t, resp.Instances, 1)
}

func TestFunctionInstancesAll_WithZeroLastSeen(t *testing.T) {
	svcCtx, ctx := setupNoAuthTestContext(t)
	svcCtx.RegistryStore.UpsertAgent(&reg.AgentSession{
		AgentID:  "agent-1",
		GameID:   "game-1",
		ExpireAt: time.Now().Add(time.Hour),
		// LastSeen is zero
		Providers: []reg.ProviderSession{
			{
				ProviderID:  "p1",
				Addr:        "localhost:9090",
				FunctionIDs: []string{"fn1"},
			},
		},
	})
	logic := NewFunctionInstancesAllLogic(ctx, svcCtx)
	resp, err := logic.FunctionInstancesAll()
	require.NoError(t, err)
	assert.Len(t, resp.Instances, 1)
}

// ---------------------------------------------------------------------------
// FunctionInstances
// ---------------------------------------------------------------------------

func TestFunctionInstances_WithRegistry(t *testing.T) {
	svcCtx, ctx := setupNoAuthTestContext(t)
	svcCtx.RegistryStore.UpsertAgent(&reg.AgentSession{
		AgentID:   "agent-1",
		GameID:    "game-1",
		ExpireAt:  time.Now().Add(time.Hour),
		LastSeen:  time.Now(),
		Functions: map[string]reg.FunctionMeta{"player.ban": {Version: "1.0.0"}},
		Providers: []reg.ProviderSession{
			{
				ProviderID:  "p1",
				Addr:        "localhost:9090",
				Version:     "1.0",
				FunctionIDs: []string{"player.ban"},
			},
		},
	})
	logic := NewFunctionInstancesLogic(ctx, svcCtx)
	resp, err := logic.FunctionInstances(&FunctionInstancesRequest{ID: "player.ban"})
	require.NoError(t, err)
	assert.NotEmpty(t, resp.Instances)
}

func TestFunctionInstances_NilStore(t *testing.T) {
	svcCtx, ctx := setupNoAuthTestContext(t)
	svcCtx.RegistryStore = nil
	logic := NewFunctionInstancesLogic(ctx, svcCtx)
	resp, err := logic.FunctionInstances(&FunctionInstancesRequest{ID: "player.ban"})
	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestFunctionInstances_WithZeroLastSeen(t *testing.T) {
	svcCtx, ctx := setupNoAuthTestContext(t)
	svcCtx.RegistryStore.UpsertAgent(&reg.AgentSession{
		AgentID:  "agent-1",
		GameID:   "game-1",
		ExpireAt: time.Now().Add(time.Hour),
		Providers: []reg.ProviderSession{
			{
				ProviderID:  "p1",
				Addr:        "localhost:9090",
				Version:     "1.0",
				FunctionIDs: []string{"player.ban"},
			},
		},
	})
	logic := NewFunctionInstancesLogic(ctx, svcCtx)
	resp, err := logic.FunctionInstances(&FunctionInstancesRequest{ID: "player.ban"})
	require.NoError(t, err)
	assert.NotEmpty(t, resp.Instances)
}

func TestFunctionInstances_AgentNil(t *testing.T) {
	svcCtx, ctx := setupNoAuthTestContext(t)
	svcCtx.RegistryStore.UpsertAgent(&reg.AgentSession{
		AgentID:  "agent-1",
		GameID:   "game-1",
		ExpireAt: time.Now().Add(time.Hour),
		Providers: []reg.ProviderSession{
			{
				ProviderID:  "p1",
				Addr:        "localhost:9090",
				FunctionIDs: []string{"other.fn"},
			},
		},
	})
	logic := NewFunctionInstancesLogic(ctx, svcCtx)
	resp, err := logic.FunctionInstances(&FunctionInstancesRequest{ID: "player.ban"})
	require.NoError(t, err)
	assert.Empty(t, resp.Instances)
}

func TestFunctionInstances_NilAgentID(t *testing.T) {
	svcCtx, ctx := setupNoAuthTestContext(t)
	svcCtx.RegistryStore.UpsertAgent(&reg.AgentSession{
		AgentID:  "", // empty agent ID
		GameID:   "game-1",
		ExpireAt: time.Now().Add(time.Hour),
		Providers: []reg.ProviderSession{
			{
				ProviderID:  "p1",
				Addr:        "localhost:9090",
				FunctionIDs: []string{"player.ban"},
			},
		},
	})
	logic := NewFunctionInstancesLogic(ctx, svcCtx)
	resp, err := logic.FunctionInstances(&FunctionInstancesRequest{ID: "player.ban"})
	require.NoError(t, err)
	assert.Empty(t, resp.Instances) // skipped because agent ID is empty
}

// ---------------------------------------------------------------------------
// FunctionWarnings
// ---------------------------------------------------------------------------

func TestFunctionWarnings_NilStore(t *testing.T) {
	svcCtx, ctx := setupNoAuthTestContext(t)
	svcCtx.RegistryStore = nil
	logic := NewFunctionWarningsLogic(ctx, svcCtx)
	resp, err := logic.FunctionWarnings(&FunctionWarningsRequest{ID: "test"})
	require.NoError(t, err)
	assert.Empty(t, resp.Items)
}

func TestFunctionWarnings_WithStore(t *testing.T) {
	svcCtx, ctx := setupNoAuthTestContext(t)
	logic := NewFunctionWarningsLogic(ctx, svcCtx)
	resp, err := logic.FunctionWarnings(&FunctionWarningsRequest{
		ID:         "test",
		FunctionID: "player.ban",
	})
	require.NoError(t, err)
	assert.NotNil(t, resp)
}

// ---------------------------------------------------------------------------
// FunctionsPending
// ---------------------------------------------------------------------------

func TestFunctionsPending(t *testing.T) {
	svcCtx, ctx := setupNoAuthTestContext(t)
	logic := NewFunctionsPendingLogic(ctx, svcCtx)
	resp, err := logic.FunctionsPending(&FunctionsPendingRequest{})
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Empty(t, resp.Items)
}

// ---------------------------------------------------------------------------
// FunctionInvoke
// ---------------------------------------------------------------------------

func TestFunctionInvoke_EmptyID(t *testing.T) {
	svcCtx, ctx := setupFullTestContext(t)
	logic := NewFunctionInvokeLogic(ctx, svcCtx)
	_, err := logic.FunctionInvoke(&FunctionInvokeRequest{ID: ""})
	assert.Error(t, err)
}

func TestFunctionInvoke_InvalidRoute(t *testing.T) {
	svcCtx, ctx := setupFullTestContext(t)
	logic := NewFunctionInvokeLogic(ctx, svcCtx)
	_, err := logic.FunctionInvoke(&FunctionInvokeRequest{
		ID:    "player.ban",
		Route: "invalid",
	})
	assert.Error(t, err)
}

func TestFunctionInvoke_TargetedNoServiceID(t *testing.T) {
	svcCtx, ctx := setupFullTestContext(t)
	logic := NewFunctionInvokeLogic(ctx, svcCtx)
	_, err := logic.FunctionInvoke(&FunctionInvokeRequest{
		ID:    "player.ban",
		Route: "targeted",
	})
	assert.Error(t, err)
}

func TestFunctionInvoke_HashNoKey(t *testing.T) {
	svcCtx, ctx := setupFullTestContext(t)
	logic := NewFunctionInvokeLogic(ctx, svcCtx)
	_, err := logic.FunctionInvoke(&FunctionInvokeRequest{
		ID:    "player.ban",
		Route: "hash",
	})
	assert.Error(t, err)
}

func TestFunctionInvoke_BroadcastWithAsync(t *testing.T) {
	svcCtx, ctx := setupFullTestContext(t)
	logic := NewFunctionInvokeLogic(ctx, svcCtx)
	_, err := logic.FunctionInvoke(&FunctionInvokeRequest{
		ID:    "player.ban",
		Route: "broadcast",
		Mode:  "async",
	})
	assert.Error(t, err)
}

func TestFunctionInvoke_InvalidPayload(t *testing.T) {
	svcCtx, ctx := setupFullTestContext(t)
	logic := NewFunctionInvokeLogic(ctx, svcCtx)
	_, err := logic.FunctionInvoke(&FunctionInvokeRequest{
		ID:      "player.ban",
		Payload: json.RawMessage(`not json`),
	})
	assert.Error(t, err)
}

func TestFunctionInvoke_InvalidParams(t *testing.T) {
	svcCtx, ctx := setupFullTestContext(t)
	logic := NewFunctionInvokeLogic(ctx, svcCtx)
	_, err := logic.FunctionInvoke(&FunctionInvokeRequest{
		ID:     "player.ban",
		Params: json.RawMessage(`not json`),
	})
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// FunctionsList
// ---------------------------------------------------------------------------

func TestFunctionsList_RuntimeFunctions_NilStore(t *testing.T) {
	svcCtx, ctx := setupNoAuthTestContext(t)
	svcCtx.RegistryStore = nil
	logic := NewFunctionsListLogic(ctx, svcCtx)
	resp, err := logic.FunctionsList(&FunctionsListRequest{Page: 1, PageSize: 10})
	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestFunctionsList_RuntimeFunctions_WithFilter(t *testing.T) {
	svcCtx, ctx := setupNoAuthTestContext(t)
	svcCtx.RegistryStore.UpsertAgent(&reg.AgentSession{
		AgentID: "agent-1",
		GameID:  "game-1",
		Functions: map[string]reg.FunctionMeta{
			"player.ban": {Enabled: true, Version: "1.0.0", Resource: "player"},
			"mail.send":  {Enabled: true, Version: "2.0.0", Resource: "mail"},
		},
	})

	logic := NewFunctionsListLogic(ctx, svcCtx)
	resp, err := logic.FunctionsList(&FunctionsListRequest{
		Page:     1,
		PageSize: 10,
		Resource: "player",
	})
	require.NoError(t, err)
	for _, item := range resp.Items {
		if item.Resource != "" {
			assert.Equal(t, "player", item.Resource)
		}
	}
}

func TestFunctionsList_RuntimeFunctions_MergesVersionAndInstances(t *testing.T) {
	svcCtx, ctx := setupNoAuthTestContext(t)
	fn := &model.Function{
		FunctionID: "player.ban",
		Name:       "Ban Player",
		Status:     1,
		Version:    "1.0.0",
	}
	require.NoError(t, svcCtx.FunctionModel.Create(ctx, fn))

	svcCtx.RegistryStore.UpsertAgent(&reg.AgentSession{
		AgentID: "agent-1",
		GameID:  "game-1",
		Functions: map[string]reg.FunctionMeta{
			"player.ban": {Enabled: true, Version: "2.0.0"},
		},
	})

	logic := NewFunctionsListLogic(ctx, svcCtx)
	resp, err := logic.FunctionsList(&FunctionsListRequest{Page: 1, PageSize: 10})
	require.NoError(t, err)

	byID := map[string]Function{}
	for _, item := range resp.Items {
		byID[item.ID] = item
	}
	if fn, ok := byID["player.ban"]; ok {
		assert.Equal(t, "2.0.0", fn.Version)
	}
}

func TestFunctionsList_RuntimeFunctions_StatusFilter(t *testing.T) {
	svcCtx, ctx := setupNoAuthTestContext(t)
	svcCtx.RegistryStore.UpsertAgent(&reg.AgentSession{
		AgentID: "agent-1",
		GameID:  "game-1",
		Functions: map[string]reg.FunctionMeta{
			"player.ban": {Enabled: true, Version: "1.0.0"},
			"mail.send":  {Enabled: true, Version: "1.0.0"},
		},
	})

	logic := NewFunctionsListLogic(ctx, svcCtx)
	resp, err := logic.FunctionsList(&FunctionsListRequest{
		Page:     1,
		PageSize: 10,
		Status:   1,
	})
	require.NoError(t, err)
	for _, item := range resp.Items {
		assert.Equal(t, 1, item.Status)
	}
}

func TestFunctionsList_RuntimeFunctions_GameIDFilter(t *testing.T) {
	svcCtx, ctx := setupNoAuthTestContext(t)
	svcCtx.RegistryStore.UpsertAgent(&reg.AgentSession{
		AgentID: "agent-1",
		GameID:  "game-1",
		Functions: map[string]reg.FunctionMeta{
			"player.ban": {Enabled: true, Version: "1.0.0"},
		},
	})
	svcCtx.RegistryStore.UpsertAgent(&reg.AgentSession{
		AgentID: "agent-2",
		GameID:  "game-2",
		Functions: map[string]reg.FunctionMeta{
			"mail.send": {Enabled: true, Version: "1.0.0"},
		},
	})

	logic := NewFunctionsListLogic(ctx, svcCtx)
	resp, err := logic.FunctionsList(&FunctionsListRequest{
		Page:     1,
		PageSize: 10,
		GameId:   "game-1",
	})
	require.NoError(t, err)
	for _, item := range resp.Items {
		if item.GameId != "" {
			assert.Equal(t, "game-1", item.GameId)
		}
	}
}

func TestFunctionsList_RuntimeFunctions_NilAgent(t *testing.T) {
	svcCtx, ctx := setupNoAuthTestContext(t)
	// Insert agent with nil Functions map
	svcCtx.RegistryStore.UpsertAgent(&reg.AgentSession{
		AgentID: "agent-1",
		GameID:  "game-1",
	})
	logic := NewFunctionsListLogic(ctx, svcCtx)
	resp, err := logic.FunctionsList(&FunctionsListRequest{Page: 1, PageSize: 10})
	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestFunctionsList_RuntimeFunctions_EmptyFunctionID(t *testing.T) {
	svcCtx, ctx := setupNoAuthTestContext(t)
	svcCtx.RegistryStore.UpsertAgent(&reg.AgentSession{
		AgentID: "agent-1",
		GameID:  "game-1",
		Functions: map[string]reg.FunctionMeta{
			"": {Enabled: true},
		},
	})
	logic := NewFunctionsListLogic(ctx, svcCtx)
	resp, err := logic.FunctionsList(&FunctionsListRequest{Page: 1, PageSize: 10})
	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestFunctionsList_RuntimeFunctions_VersionUpgrade(t *testing.T) {
	svcCtx, ctx := setupNoAuthTestContext(t)
	svcCtx.RegistryStore.UpsertAgent(&reg.AgentSession{
		AgentID: "agent-1",
		GameID:  "game-1",
		Functions: map[string]reg.FunctionMeta{
			"player.ban": {Enabled: true, Version: "3.0.0"},
		},
	})
	svcCtx.RegistryStore.UpsertAgent(&reg.AgentSession{
		AgentID: "agent-2",
		GameID:  "game-1",
		Functions: map[string]reg.FunctionMeta{
			"player.ban": {Enabled: true, Version: "2.0.0"},
		},
	})

	logic := NewFunctionsListLogic(ctx, svcCtx)
	resp, err := logic.FunctionsList(&FunctionsListRequest{Page: 1, PageSize: 10})
	require.NoError(t, err)

	byID := map[string]Function{}
	for _, item := range resp.Items {
		byID[item.ID] = item
	}
	if fn, ok := byID["player.ban"]; ok {
		assert.Equal(t, "3.0.0", fn.Version)
		assert.Equal(t, 2, fn.Instances)
	}
}

func TestFunctionsList_RuntimeFunctions_GameIDFromSession(t *testing.T) {
	svcCtx, ctx := setupNoAuthTestContext(t)
	svcCtx.RegistryStore.UpsertAgent(&reg.AgentSession{
		AgentID: "agent-1",
		GameID:  "game-1",
		Functions: map[string]reg.FunctionMeta{
			"player.ban": {Enabled: true, Version: "1.0.0"},
		},
	})

	logic := NewFunctionsListLogic(ctx, svcCtx)
	resp, err := logic.FunctionsList(&FunctionsListRequest{Page: 1, PageSize: 10})
	require.NoError(t, err)

	byID := map[string]Function{}
	for _, item := range resp.Items {
		byID[item.ID] = item
	}
	if fn, ok := byID["player.ban"]; ok {
		assert.Equal(t, "game-1", fn.GameId)
	}
}

// ---------------------------------------------------------------------------
// FunctionsList - pagination edge cases
// ---------------------------------------------------------------------------

func TestFunctionsList_PaginationEdgeCases(t *testing.T) {
	svcCtx, ctx := setupNoAuthTestContext(t)
	for i := 0; i < 5; i++ {
		fn := &model.Function{
			FunctionID: "fn" + string(rune('0'+i)),
			Name:       "Function " + string(rune('0'+i)),
			Status:     1,
		}
		require.NoError(t, svcCtx.FunctionModel.Create(ctx, fn))
	}

	logic := NewFunctionsListLogic(ctx, svcCtx)

	resp, err := logic.FunctionsList(&FunctionsListRequest{Page: 100, PageSize: 10})
	require.NoError(t, err)
	assert.Empty(t, resp.Items)

	resp, err = logic.FunctionsList(&FunctionsListRequest{Page: 1, PageSize: 2})
	require.NoError(t, err)
	assert.Len(t, resp.Items, 2)
	assert.Equal(t, int64(5), resp.Total)
}

// ---------------------------------------------------------------------------
// FunctionHistory
// ---------------------------------------------------------------------------

func TestFunctionHistory_WithNilConfigVersionModel(t *testing.T) {
	svcCtx, ctx := setupNoAuthTestContext(t)
	svcCtx.ConfigVersionModel = nil
	logic := NewFunctionHistoryLogic(ctx, svcCtx)
	items, _, err := logic.FunctionHistory(&FunctionHistoryRequest{ID: "test.fn"})
	require.NoError(t, err)
	assert.Len(t, items, 1)
}

// ---------------------------------------------------------------------------
// FunctionAnalytics
// ---------------------------------------------------------------------------

func TestFunctionAnalytics_Basic(t *testing.T) {
	svcCtx, ctx := setupNoAuthTestContext(t)
	svcCtx.ConfigVersionModel = nil
	logic := NewFunctionAnalyticsLogic(ctx, svcCtx)
	resp, err := logic.FunctionAnalytics(&FunctionAnalyticsRequest{ID: "test.fn"})
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, int64(0), resp.TotalCalls)
	assert.Equal(t, float64(100), resp.SuccessRate)
}

// ---------------------------------------------------------------------------
// FunctionPermissions
// ---------------------------------------------------------------------------

func TestFunctionPermissions(t *testing.T) {
	svcCtx, ctx := setupFullTestContext(t)
	logic := NewFunctionPermissionsLogic(ctx, svcCtx)
	resp, err := logic.FunctionPermissions(&FunctionPermissionsRequest{ID: "player.ban"})
	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestFunctionPermissions_EmptyID(t *testing.T) {
	svcCtx, ctx := setupFullTestContext(t)
	logic := NewFunctionPermissionsLogic(ctx, svcCtx)
	_, err := logic.FunctionPermissions(&FunctionPermissionsRequest{ID: ""})
	assert.Error(t, err)
}

func TestFunctionPermissions_NilModel(t *testing.T) {
	svcCtx, ctx := setupFullTestContext(t)
	svcCtx.FunctionModel = nil
	logic := NewFunctionPermissionsLogic(ctx, svcCtx)
	_, err := logic.FunctionPermissions(&FunctionPermissionsRequest{ID: "player.ban"})
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// FunctionPermissionsUpdate
// ---------------------------------------------------------------------------

func TestFunctionPermissionsUpdate(t *testing.T) {
	svcCtx, ctx := setupFullTestContext(t)
	logic := NewFunctionPermissionsUpdateLogic(ctx, svcCtx)
	resp, err := logic.FunctionPermissionsUpdate(&FunctionPermissionsUpdateRequest{
		ID: "player.ban",
		Permissions: []FunctionPermission{
			{Resource: "player.ban", Actions: []string{"read"}, Roles: []string{"admin"}},
		},
	})
	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestFunctionPermissionsUpdate_EmptyID(t *testing.T) {
	svcCtx, ctx := setupFullTestContext(t)
	logic := NewFunctionPermissionsUpdateLogic(ctx, svcCtx)
	_, err := logic.FunctionPermissionsUpdate(&FunctionPermissionsUpdateRequest{
		ID:          "",
		Permissions: []FunctionPermission{},
	})
	assert.Error(t, err)
}

func TestFunctionPermissionsUpdate_NilModel(t *testing.T) {
	svcCtx, ctx := setupFullTestContext(t)
	svcCtx.FunctionModel = nil
	logic := NewFunctionPermissionsUpdateLogic(ctx, svcCtx)
	_, err := logic.FunctionPermissionsUpdate(&FunctionPermissionsUpdateRequest{
		ID:          "player.ban",
		Permissions: []FunctionPermission{},
	})
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// rawJSONFromValue edge cases
// ---------------------------------------------------------------------------

func TestRawJSONFromValue_MarshalError(t *testing.T) {
	ch := make(chan int)
	result := rawJSONFromValue(ch)
	assert.Nil(t, result)
}

func TestRawJSONFromValue_StringWithWhitespace(t *testing.T) {
	result := rawJSONFromValue(`  {"key":"value"}  `)
	assert.JSONEq(t, `{"key":"value"}`, string(result))
}

// ---------------------------------------------------------------------------
// rawJSONFromBytes edge cases
// ---------------------------------------------------------------------------

func TestRawJSONFromBytes_InvalidJSON(t *testing.T) {
	result := rawJSONFromBytes([]byte("not json"))
	assert.NotNil(t, result)
	assert.True(t, json.Valid(result))
	var s string
	err := json.Unmarshal(result, &s)
	assert.NoError(t, err)
	assert.Equal(t, "not json", s)
}

// ---------------------------------------------------------------------------
// jsonValueFromRaw & jsonObjectFromRaw empty
// ---------------------------------------------------------------------------

func TestJsonValueFromRaw_EmptyRawMessage(t *testing.T) {
	v, err := jsonValueFromRaw(json.RawMessage{})
	assert.NoError(t, err)
	assert.Nil(t, v)
}

func TestJsonObjectFromRaw_EmptyRawMessage(t *testing.T) {
	m, err := jsonObjectFromRaw(json.RawMessage{})
	assert.NoError(t, err)
	assert.Nil(t, m)
}

// ---------------------------------------------------------------------------
// getOrCreateFunctionRecord
// ---------------------------------------------------------------------------

func TestGetOrCreateFunctionRecord_DuplicateKey(t *testing.T) {
	svcCtx, ctx := setupNoAuthTestContext(t)
	fn := &model.Function{
		FunctionID: "player.ban",
		Name:       "Ban Player",
		Status:     1,
	}
	require.NoError(t, svcCtx.FunctionModel.Create(ctx, fn))

	result, err := getOrCreateFunctionRecord(ctx, svcCtx, "player.ban")
	require.NoError(t, err)
	assert.Equal(t, "player.ban", result.FunctionID)
}

func TestGetOrCreateFunctionRecord_NewFunction(t *testing.T) {
	svcCtx, ctx := setupNoAuthTestContext(t)
	result, err := getOrCreateFunctionRecord(ctx, svcCtx, "new.fn")
	require.NoError(t, err)
	assert.Equal(t, "new.fn", result.FunctionID)
	assert.Equal(t, 1, result.Status)
}

func TestGetOrCreateFunctionRecord_WithRiskLevel(t *testing.T) {
	svcCtx, ctx := setupNoAuthTestContext(t)
	result, err := getOrCreateFunctionRecordWithRisk(ctx, svcCtx, "risk.fn", "high")
	require.NoError(t, err)
	assert.Equal(t, "risk.fn", result.FunctionID)
}

func TestGetOrCreateFunctionRecord_DuplicateKeyRace(t *testing.T) {
	svcCtx, ctx := setupNoAuthTestContext(t)
	fn := &model.Function{
		FunctionID: "race.fn",
		Name:       "Race Function",
		Status:     1,
	}
	require.NoError(t, svcCtx.FunctionModel.Create(ctx, fn))

	result1, err := getOrCreateFunctionRecord(ctx, svcCtx, "race.fn")
	require.NoError(t, err)
	result2, err := getOrCreateFunctionRecord(ctx, svcCtx, "race.fn")
	require.NoError(t, err)
	assert.Equal(t, result1.FunctionID, result2.FunctionID)
}

// ---------------------------------------------------------------------------
// backfillFromRegistry - with OpenAPI operations
// ---------------------------------------------------------------------------

func TestBackfillFromRegistry_WithOpenAPI(t *testing.T) {
	store := reg.NewStore()
	schemaType := openapi3.Types{"object"}
	store.UpsertAgent(&reg.AgentSession{
		AgentID: "agent-1",
		GameID:  "game-1",
		Functions: map[string]reg.FunctionMeta{
			"player.ban": {
				Enabled:  true,
				Version:  "1.0.0",
				Summary:  "Ban player",
				Resource: "player",
			},
		},
	})
	store.UpsertOpenAPI("player.ban", &openapi3.Operation{
		OperationID: "player.ban",
		Summary:     "Ban a player",
		RequestBody: &openapi3.RequestBodyRef{
			Value: &openapi3.RequestBody{
				Content: openapi3.Content{
					"application/json": &openapi3.MediaType{
						Schema: &openapi3.SchemaRef{
							Value: &openapi3.Schema{Type: &schemaType},
						},
					},
				},
			},
		},
		Extensions: map[string]interface{}{
			"x-resource":  "player",
			"x-version":   "3.0.0",
			"x-operation": "ban",
		},
	})
	svcCtx := &svc.ServiceContext{RegistryStore: store}
	fn := &model.Function{}
	backfillFromRegistry(svcCtx, "player.ban", fn)

	assert.Equal(t, "player", fn.Resource)
	assert.Equal(t, "3.0.0", fn.Version)
	assert.Equal(t, "openapi3.0.3", fn.SpecFormat)
	assert.NotNil(t, fn.OpenAPISpec)
}

func TestBackfillFromRegistry_OpenAPINoExtensions(t *testing.T) {
	store := reg.NewStore()
	schemaType := openapi3.Types{"object"}
	store.UpsertAgent(&reg.AgentSession{
		AgentID: "agent-1",
		GameID:  "game-1",
		Functions: map[string]reg.FunctionMeta{
			"player.ban": {Enabled: true, Version: "1.0.0"},
		},
	})
	store.UpsertOpenAPI("player.ban", &openapi3.Operation{
		OperationID: "player.ban",
		Summary:     "Ban a player",
		RequestBody: &openapi3.RequestBodyRef{
			Value: &openapi3.RequestBody{
				Content: openapi3.Content{
					"application/json": &openapi3.MediaType{
						Schema: &openapi3.SchemaRef{
							Value: &openapi3.Schema{Type: &schemaType},
						},
					},
				},
			},
		},
	})
	svcCtx := &svc.ServiceContext{RegistryStore: store}
	fn := &model.Function{}
	backfillFromRegistry(svcCtx, "player.ban", fn)

	assert.Equal(t, "openapi3.0.3", fn.SpecFormat)
	assert.NotNil(t, fn.OpenAPISpec)
}

func TestBackfillFromRegistry_OpenAPINilExtensions(t *testing.T) {
	store := reg.NewStore()
	store.UpsertAgent(&reg.AgentSession{
		AgentID: "agent-1",
		GameID:  "game-1",
		Functions: map[string]reg.FunctionMeta{
			"player.ban": {Enabled: true, Version: "1.0.0"},
		},
	})
	store.UpsertOpenAPI("player.ban", &openapi3.Operation{
		OperationID: "player.ban",
		Summary:     "Ban a player",
	})
	svcCtx := &svc.ServiceContext{RegistryStore: store}
	fn := &model.Function{}
	backfillFromRegistry(svcCtx, "player.ban", fn)

	assert.Equal(t, "Ban a player", fn.Description)
}

func TestBackfillFromRegistry_OpenAPIOperationNil(t *testing.T) {
	store := reg.NewStore()
	store.UpsertAgent(&reg.AgentSession{
		AgentID: "agent-1",
		GameID:  "game-1",
		Functions: map[string]reg.FunctionMeta{
			"player.ban": {Enabled: true, Version: "1.0.0"},
		},
	})
	// No OpenAPI operation stored
	svcCtx := &svc.ServiceContext{RegistryStore: store}
	fn := &model.Function{}
	backfillFromRegistry(svcCtx, "player.ban", fn)

	assert.Equal(t, "1.0.0", fn.Version)
}

// ---------------------------------------------------------------------------
// FunctionsList - admin not found
// ---------------------------------------------------------------------------

func TestFunctionsList_AdminNotFound(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))

	svcCtx := &svc.ServiceContext{
		DB:            db,
		FunctionModel: model.NewFunctionModel(db),
		RegistryStore: reg.NewStore(),
	}

	ctx := context.WithValue(context.Background(), "username", "nonexistent")
	logic := NewFunctionsListLogic(ctx, svcCtx)
	resp, err := logic.FunctionsList(&FunctionsListRequest{Page: 1, PageSize: 10})
	require.NoError(t, err)
	assert.NotNil(t, resp)
}

// ---------------------------------------------------------------------------
// DescriptorsLogic - scope validation
// ---------------------------------------------------------------------------

func TestDescriptorsLogic_RequiresScope(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.FunctionContract{}))

	svcCtx := &svc.ServiceContext{DB: db}
	logic := NewDescriptorsLogic(context.Background(), svcCtx)
	_, err = logic.DescriptorsV2(&DescriptorsRequest{})
	assert.Error(t, err)
}

func TestDescriptorsLogic_NilSvcCtx(t *testing.T) {
	ctx := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "game1", Env: "dev"})
	logic := NewDescriptorsLogic(ctx, nil)
	_, err := logic.DescriptorsV2(&DescriptorsRequest{})
	assert.Error(t, err)
}

func TestDescriptorsLogic_NilDB(t *testing.T) {
	ctx := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "game1", Env: "dev"})
	svcCtx := &svc.ServiceContext{}
	logic := NewDescriptorsLogic(ctx, svcCtx)
	_, err := logic.DescriptorsV2(&DescriptorsRequest{})
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// FunctionsList - with no roles
// ---------------------------------------------------------------------------

func TestFunctionsList_NoAdminRoles(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))

	svcCtx := &svc.ServiceContext{
		DB:            db,
		FunctionModel: model.NewFunctionModel(db),
		RegistryStore: reg.NewStore(),
	}

	logic := NewFunctionsListLogic(context.Background(), svcCtx)
	resp, err := logic.FunctionsList(&FunctionsListRequest{Page: 1, PageSize: 10})
	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestTermDisplay_Extended(t *testing.T) {
	displayMap := map[string]map[string]map[string]string{
		"resource": {
			"player": {
				"zh": "玩家",
				"en": "Player",
			},
		},
		"action": {
			"ban": {
				"zh": "封禁",
				"en": "Ban",
			},
		},
	}

	tests := []struct {
		name     string
		domain   string
		key      string
		expected map[string]string
	}{
		{
			name:     "valid domain and key",
			domain:   "resource",
			key:      "player",
			expected: map[string]string{"zh": "玩家", "en": "Player"},
		},
		{
			name:     "valid domain, missing key",
			domain:   "resource",
			key:      "unknown",
			expected: nil,
		},
		{
			name:     "missing domain",
			domain:   "unknown",
			key:      "player",
			expected: nil,
		},
		{
			name:     "empty domain",
			domain:   "",
			key:      "player",
			expected: nil,
		},
		{
			name:     "empty key",
			domain:   "resource",
			key:      "",
			expected: nil,
		},
		{
			name:     "case insensitive",
			domain:   "RESOURCE",
			key:      "PLAYER",
			expected: map[string]string{"zh": "玩家", "en": "Player"},
		},
		{
			name:     "partial display",
			domain:   "action",
			key:      "ban",
			expected: map[string]string{"zh": "封禁", "en": "Ban"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := termDisplay(displayMap, tt.domain, tt.key)
			if tt.expected == nil {
				assert.Nil(t, result)
			} else {
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}
