package console

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/cache"
	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/dispatch"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/cuihairu/croupier/internal/transport"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"gorm.io/gorm"
)

func TestServiceMenuRequiresConsoleReadPermission(t *testing.T) {
	service, ctx := newConsoleTestService(t)

	_, err := service.Menu(ctx, &ConsoleMenuRequest{})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "无权查看运行控制台")
}

func TestServiceMenuAllowsPagesReadPermission(t *testing.T) {
	service, ctx := newConsoleTestService(t, "pages:read")
	require.NoError(t, seedConsolePublishedPage(service.svcCtx, ctx))

	resp, err := service.Menu(ctx, &ConsoleMenuRequest{Language: "zh-CN"})

	require.NoError(t, err)
	require.Len(t, resp.Items, 1)
	assert.Equal(t, "player", resp.Items[0].Key)
	require.Len(t, resp.Items[0].Children, 1)
	assert.Equal(t, "player.manage", resp.Items[0].Children[0].Key)
}

func TestServiceExecuteBindingRequiresFunctionInvokePermission(t *testing.T) {
	service, ctx := newConsoleTestService(t, "console:read", "pages:read")
	require.NoError(t, seedConsolePublishedPage(service.svcCtx, ctx))

	_, err := service.ExecuteBinding(ctx, &ConsoleExecuteBindingRequest{
		PageKey:   "player.manage",
		BindingID: "player.query",
		Payload:   json.RawMessage(`{}`),
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "无权执行运行控制台操作")
}

func TestServiceExecuteBindingWritesAuditWithPageContext(t *testing.T) {
	service, ctx, auditStore := newConsoleTestServiceWithAudit(t, "function:invoke")
	require.NoError(t, seedConsolePublishedPageWithCurrentContracts(service.svcCtx, ctx))
	caller := &fakeConsoleSessionCaller{
		payload: []byte(`{"ok":true}`),
	}
	service.svcCtx.Dispatcher.SetSessionResolver(fakeConsoleSessionResolver{
		caller: caller,
	})

	resp, err := service.ExecuteBinding(ctx, &ConsoleExecuteBindingRequest{
		PageKey:   "player.manage",
		BindingID: "player.query",
		Payload:   json.RawMessage(`{"keyword":"alice"}`),
	})

	require.NoError(t, err)
	require.NotEmpty(t, resp.Result.RequestID)
	assert.Equal(t, spec.PageExecutionKindSync, resp.Result.Kind)
	assert.JSONEq(t, `{"ok":true}`, string(resp.Result.Data))

	records, total, err := auditStore.List(audit.AuditFilter{
		EventType: []audit.AuditEventType{audit.EventPageExecute},
	}, audit.AuditPage{PageSize: 10})
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Len(t, records, 1)
	record := records[0]
	assert.Equal(t, "console_tester", record.Actor.ID)
	assert.Equal(t, "success", record.Outcome)
	assert.Equal(t, "page", record.Resource.Type)
	assert.Equal(t, "player.manage", record.Resource.ID)
	assert.Equal(t, "demo-game", record.Resource.GameID)
	assert.Equal(t, "development", record.Resource.Environment)
	assert.Equal(t, resp.Result.RequestID, record.Context.RequestID)
	assert.Equal(t, resp.Result.RequestID, record.Details["request_id"])
	assert.Equal(t, "player.manage", record.Details["page_key"])
	assert.Equal(t, "player.query", record.Details["binding_id"])
	assert.Equal(t, "player.query", record.Details["function_id"])
	assert.Equal(t, 1, record.Details["publish_version"])
	require.NotNil(t, caller.lastRequest)
	assert.Equal(t, "player.manage", caller.lastRequest.Metadata["page_key"])
	assert.Equal(t, "player.query", caller.lastRequest.Metadata["binding_id"])
	assert.Equal(t, resp.Result.RequestID, caller.lastRequest.Metadata["page_request_id"])
	assert.Equal(t, "console.binding.execute", caller.lastRequest.Metadata["page_runtime_api"])
}

func newConsoleTestService(t *testing.T, permissions ...string) (*Service, context.Context) {
	service, ctx, _ := newConsoleTestServiceWithAudit(t, permissions...)
	return service, ctx
}

func newConsoleTestServiceWithAudit(t *testing.T, permissions ...string) (*Service, context.Context, *audit.InMemoryAuditStore) {
	t.Helper()

	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)

	store := reg.NewStore()
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "agent-1",
		GameID:   "demo-game",
		Env:      "development",
		ExpireAt: time.Now().Add(time.Minute),
		LastSeen: time.Now(),
		Functions: map[string]reg.FunctionMeta{
			"player.query": {
				Enabled:      true,
				Version:      "1.0.0",
				Resource:     "player",
				Operation:    "query",
				InputSchema:  `{"type":"object","properties":{"keyword":{"type":"string"}}}`,
				OutputSchema: `{"type":"object","properties":{"items":{"type":"array"},"total":{"type":"number"}}}`,
			},
		},
	})
	dispatcher := dispatch.NewDispatcher(store)

	admin := model.Admin{Username: "console_tester", Status: 1, PasswordHash: "test"}
	require.NoError(t, db.Create(&admin).Error)

	role := model.Role{Name: "console_tester_role", Description: "console tester"}
	require.NoError(t, db.Create(&role).Error)
	require.NoError(t, db.Create(&model.AdminRole{AdminID: admin.ID, RoleID: role.ID}).Error)
	for _, permissionID := range permissions {
		grantConsolePermission(t, db, role.ID, permissionID)
	}

	auditStore := audit.NewInMemoryAuditStore()
	nullCache := cache.NewNullCache()
	svcCtx := &svc.ServiceContext{
		DB:                     db,
		AdminModel:             model.NewAdminModel(db),
		RoleModel:              model.NewRoleModel(db),
		PermissionModel:        model.NewPermissionModel(db),
		PageSpecModel:          model.NewPageSpecModel(db),
		PublishedPageSpecModel: model.NewPublishedPageSpecModel(db),
		PageVersionModel:       model.NewPageVersionModel(db),
		RegistryStore:          store,
		Dispatcher:             dispatcher,
		AuditService:           audit.NewAuditService(auditStore, nil),
		Cache:                  nullCache,
		CacheHelper:            cache.NewCacheHelper(nullCache),
	}
	ctx := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "demo-game", Env: "development"})
	ctx = context.WithValue(ctx, "username", admin.Username)
	return NewService(svcCtx), ctx, auditStore
}

func seedConsolePublishedPage(svcCtx *svc.ServiceContext, ctx context.Context) error {
	page := spec.PageSpec{
		PageKey:     "player.manage",
		Type:        spec.PageTypeOperation,
		ResourceKey: "player",
		Title:       spec.LocalizedText{"zh-CN": "玩家管理"},
		Category: spec.PageCategorySpec{
			Key:    "player",
			Labels: spec.LocalizedText{"zh-CN": "玩家"},
		},
		Schema: spec.FormilySchema(`{
			"type":"object",
			"x-component":"ConsolePage",
			"x-component-props":{"schemaVersion":"formily-page:1"},
			"properties":{"query":{"type":"object","x-component":"QueryForm","x-component-props":{"bindingId":"player.query"}}}
		}`),
		Bindings: []spec.PageFunctionBinding{
			{
				ID:         "player.query",
				FunctionID: "player.query",
				Usage:      spec.BindingUsageQuery,
				Execution:  spec.PageBindingExecution{Mode: spec.PageExecutionModeSync},
			},
		},
	}
	specJSON, err := json.Marshal(page)
	if err != nil {
		return err
	}
	contractsJSON, err := json.Marshal([]spec.BindingContractSnapshot{
		{
			BindingID:             "player.query",
			FunctionID:            "player.query",
			FunctionVersion:       "1.0.0",
			InputSchemaDigest:     "unused-in-permission-test",
			OutputSchemaDigest:    "unused-in-permission-test",
			ExecutionMode:         spec.PageExecutionModeSync,
			RendererSchemaVersion: "formily-page:1",
		},
	})
	if err != nil {
		return err
	}
	return svcCtx.PublishedPageSpecModel.Create(ctx, &model.PublishedPageSpec{
		GameID:                "demo-game",
		Env:                   "development",
		PageKey:               "player.manage",
		Version:               1,
		SpecJSON:              string(specJSON),
		BindingContractsJSON:  string(contractsJSON),
		RendererSchemaVersion: "formily-page:1",
		Active:                true,
		PublishedAt:           time.Now(),
		PublishedBy:           "console_tester",
	})
}

func seedConsolePublishedPageWithCurrentContracts(svcCtx *svc.ServiceContext, ctx context.Context) error {
	inputDigest := digestRaw([]byte(`{"type":"object","properties":{"keyword":{"type":"string"}}}`))
	outputDigest := digestRaw([]byte(`{"type":"object","properties":{"items":{"type":"array"},"total":{"type":"number"}}}`))
	page := spec.PageSpec{
		PageKey:     "player.manage",
		Type:        spec.PageTypeOperation,
		ResourceKey: "player",
		Title:       spec.LocalizedText{"zh-CN": "玩家管理"},
		Category: spec.PageCategorySpec{
			Key:    "player",
			Labels: spec.LocalizedText{"zh-CN": "玩家"},
		},
		Schema: spec.FormilySchema(`{
			"type":"object",
			"x-component":"ConsolePage",
			"x-component-props":{"schemaVersion":"formily-page:1"},
			"properties":{"query":{"type":"object","x-component":"QueryForm","x-component-props":{"bindingId":"player.query"}}}
		}`),
		Bindings: []spec.PageFunctionBinding{
			{
				ID:         "player.query",
				FunctionID: "player.query",
				Usage:      spec.BindingUsageQuery,
				Execution:  spec.PageBindingExecution{Mode: spec.PageExecutionModeSync},
			},
		},
	}
	specJSON, err := json.Marshal(page)
	if err != nil {
		return err
	}
	contractsJSON, err := json.Marshal([]spec.BindingContractSnapshot{
		{
			BindingID:             "player.query",
			FunctionID:            "player.query",
			FunctionVersion:       "1.0.0",
			InputSchemaDigest:     inputDigest,
			OutputSchemaDigest:    outputDigest,
			ExecutionMode:         spec.PageExecutionModeSync,
			RendererSchemaVersion: "formily-page:1",
		},
	})
	if err != nil {
		return err
	}
	return svcCtx.PublishedPageSpecModel.Create(ctx, &model.PublishedPageSpec{
		GameID:                "demo-game",
		Env:                   "development",
		PageKey:               "player.manage",
		Version:               1,
		SpecJSON:              string(specJSON),
		BindingContractsJSON:  string(contractsJSON),
		RendererSchemaVersion: "formily-page:1",
		Active:                true,
		PublishedAt:           time.Now(),
		PublishedBy:           "console_tester",
	})
}

type fakeConsoleSessionResolver struct {
	caller transport.SessionCaller
}

func (r fakeConsoleSessionResolver) ResolveAgentConn(string) (transport.SessionCaller, bool) {
	return r.caller, r.caller != nil
}

type fakeConsoleSessionCaller struct {
	payload     []byte
	err         error
	lastRequest *sdkv1.InvokeRequest
}

func (c *fakeConsoleSessionCaller) Call(ctx context.Context, msgID uint32, reqBody []byte) (uint32, []byte, error) {
	if c.err != nil {
		return 0, nil, c.err
	}
	req := &sdkv1.InvokeRequest{}
	if err := proto.Unmarshal(reqBody, req); err != nil {
		return 0, nil, err
	}
	c.lastRequest = req
	resp, err := proto.Marshal(&sdkv1.InvokeResponse{Payload: c.payload})
	if err != nil {
		return 0, nil, err
	}
	return protocol.MsgInvokeResponse, resp, nil
}

func grantConsolePermission(t *testing.T, db *gorm.DB, roleID uint, permissionID string) {
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
