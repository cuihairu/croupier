package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	consoleapi "github.com/cuihairu/croupier/internal/api/console"
	openapiapi "github.com/cuihairu/croupier/internal/api/openapi"
	"github.com/cuihairu/croupier/internal/cache"
	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/dispatch"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	dashboardservice "github.com/cuihairu/croupier/internal/service"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/cuihairu/croupier/internal/transport"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	gsqlite "gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestWireDashboardRegistrationPipelineGeneratesProposal(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))
	require.NoError(t, reg.MigrateAgentSessions(db))

	store := reg.NewStoreWithDB(db)
	svcCtx := &svc.ServiceContext{
		Config:        config.Config{},
		DB:            db,
		RegistryStore: store,
	}
	wireDashboardRegistrationPipeline(svcCtx)

	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "agent-1",
		GameID:   "demo-game",
		Env:      "development",
		ExpireAt: time.Now().Add(time.Minute),
		LastSeen: time.Now(),
		Functions: map[string]reg.FunctionMeta{
			"player.ban": {
				Enabled:      true,
				Version:      "1.0.0",
				Summary:      "Ban player",
				Operation:    "ban",
				Capability:   "action",
				Execution:    "sync",
				InputSchema:  `{"type":"object","properties":{"player_id":{"type":"string"}}}`,
				OutputSchema: `{"type":"object","properties":{"success":{"type":"boolean"}}}`,
			},
		},
	})

	ctx := context.Background()
	contract, err := model.NewFunctionContractModel(db).FindByScopeAndFunctionID(ctx, "demo-game", "development", "player.ban")
	require.NoError(t, err)
	require.Equal(t, true, contract.Approval["required"])
	require.Equal(t, "two_person", contract.Approval["policyKey"])
	proposal, err := model.NewPageProposalModel(db).FindByScopeAndKey(ctx, "demo-game", "development", "operation:player.ban")
	require.NoError(t, err)
	require.Equal(t, "operation--player.ban", proposal.PageKey)
}

func TestDashboardRegistrationProposalPublishesToConsoleAndExecutes(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))
	require.NoError(t, reg.MigrateAgentSessions(db))

	svcCtx, store, caller := newDashboardRegistrationServiceContext(t, db, []byte(`{"success":true}`))
	wireDashboardRegistrationPipeline(svcCtx)

	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "agent-1",
		GameID:   "demo-game",
		Env:      "development",
		ExpireAt: time.Now().Add(time.Minute),
		LastSeen: time.Now(),
		Functions: map[string]reg.FunctionMeta{
			"player.ban": {
				Enabled:      true,
				Version:      "1.0.0",
				Summary:      "Ban player",
				Operation:    "ban",
				Capability:   "action",
				Execution:    "sync",
				InputSchema:  `{"type":"object","properties":{"player_id":{"type":"string"}}}`,
				OutputSchema: `{"type":"object","properties":{"success":{"type":"boolean"}}}`,
			},
		},
	})

	ctx := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "demo-game", Env: "development"})
	ctx = context.WithValue(ctx, "username", "dashboard_tester")
	result, err := dashboardservice.NewProposalService(db).AcceptAndPublishProposal(ctx, "demo-game", "development", "operation:player.ban")
	require.NoError(t, err)
	require.Equal(t, "operation--player.ban", result.PageKey)

	menu, err := consoleapi.NewService(svcCtx).Menu(ctx, &consoleapi.ConsoleMenuRequest{Language: "zh-CN"})
	require.NoError(t, err)
	require.Len(t, menu.Items, 1)
	require.Equal(t, "player", menu.Items[0].Key)
	require.Len(t, menu.Items[0].Children, 1)
	require.Equal(t, "operation--player.ban", menu.Items[0].Children[0].Key)

	consoleSvc := consoleapi.NewService(svcCtx)
	page, err := consoleSvc.Page(ctx, &consoleapi.ConsolePageRequest{PageKey: "operation--player.ban"})
	require.NoError(t, err)
	require.Equal(t, "operation--player.ban", page.Page.PageKey)
	require.Empty(t, page.Page.BindingFreshness)
	require.Len(t, page.Page.Bindings, 1)

	execResp, err := consoleSvc.ExecuteBinding(ctx, &consoleapi.ConsoleExecuteBindingRequest{
		PageKey:   "operation--player.ban",
		BindingID: page.Page.Bindings[0].ID,
		Context: consoleapi.ConsoleBindingExecutionContext{
			Form: json.RawMessage(`{"player_id":"p-001"}`),
		},
	})
	require.NoError(t, err)
	require.Equal(t, spec.PageExecutionKindSync, execResp.Result.Kind)
	require.JSONEq(t, `{"success":true}`, string(execResp.Result.Data))
	require.NotNil(t, caller.lastRequest)
	require.Equal(t, "player.ban", caller.lastRequest.FunctionId)
	require.JSONEq(t, `{"player_id":"p-001"}`, string(caller.lastRequest.Payload))
}

func TestOpenAPIBindingProposalPublishesToConsoleAndExecutes(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))
	require.NoError(t, reg.MigrateAgentSessions(db))

	svcCtx, store, caller := newDashboardRegistrationServiceContext(t, db, []byte(`{"items":[{"id":"p-001","name":"Ada"}],"total":1}`))

	now := time.Now()
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "agent-1",
		GameID:   "demo-game",
		Env:      "development",
		ExpireAt: now.Add(time.Minute),
		LastSeen: now,
		Functions: map[string]reg.FunctionMeta{
			"player.list": {
				Enabled:      true,
				Version:      "1.0.0",
				Summary:      "List players",
				InputSchema:  `{"type":"object","properties":{"page":{"type":"integer"},"page_size":{"type":"integer"}}}`,
				OutputSchema: `{"type":"object","properties":{"items":{"type":"array","items":{"type":"object","properties":{"id":{"type":"string"},"name":{"type":"string"}}},"total":{"type":"integer"}}}`,
			},
		},
	})

	ctx := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "demo-game", Env: "development"})
	ctx = context.WithValue(ctx, "username", "dashboard_tester")
	openAPIService := openapiapi.NewService(svcCtx)
	source, err := openAPIService.CreateSource(ctx, &openapiapi.OpenAPISourceCreateRequest{Spec: dashboardRegistrationOpenAPISpec(t)})
	require.NoError(t, err)
	require.Len(t, source.Source.Operations, 1)

	binding, err := openAPIService.CreateBinding(ctx, &openapiapi.OpenAPISourceBindingCreateRequest{
		SourceID:    source.Source.SourceID,
		OperationID: "player.list",
		Kind:        "provider",
		FunctionID:  "player.list",
	})
	require.NoError(t, err)
	require.NotNil(t, binding.Proposal)
	require.Equal(t, "resource:players", binding.Proposal.ProposalKey)
	require.Equal(t, "resource--players", binding.Proposal.PageKey)

	publishResult, err := dashboardservice.NewProposalService(db).AcceptAndPublishProposal(ctx, "demo-game", "development", binding.Proposal.ProposalKey)
	require.NoError(t, err)
	require.Equal(t, "resource--players", publishResult.PageKey)

	consoleSvc := consoleapi.NewService(svcCtx)
	menu, err := consoleSvc.Menu(ctx, &consoleapi.ConsoleMenuRequest{Language: "zh-CN"})
	require.NoError(t, err)
	require.Len(t, menu.Items, 1)
	require.Equal(t, "players", menu.Items[0].Key)
	require.Len(t, menu.Items[0].Children, 1)
	require.Equal(t, "resource--players", menu.Items[0].Children[0].Key)

	page, err := consoleSvc.Page(ctx, &consoleapi.ConsolePageRequest{PageKey: "resource--players"})
	require.NoError(t, err)
	queryBinding := findDashboardRegistrationBinding(t, page.Page.Bindings, "list")
	execResp, err := consoleSvc.ExecuteBinding(ctx, &consoleapi.ConsoleExecuteBindingRequest{
		PageKey:   page.Page.PageKey,
		BindingID: queryBinding.ID,
		Context: consoleapi.ConsoleBindingExecutionContext{
			Form: json.RawMessage(`{"current":1,"pageSize":20}`),
		},
	})
	require.NoError(t, err)
	require.Equal(t, spec.PageExecutionKindSync, execResp.Result.Kind)
	require.JSONEq(t, `{"items":[{"id":"p-001","name":"Ada"}],"total":1}`, string(execResp.Result.Data))
	require.NotNil(t, caller.lastRequest)
	require.Equal(t, "player.list", caller.lastRequest.FunctionId)
	require.JSONEq(t, `{"page":1,"page_size":20}`, string(caller.lastRequest.Payload))
}

func newDashboardRegistrationServiceContext(t *testing.T, db *gorm.DB, responsePayload []byte) (*svc.ServiceContext, *reg.Store, *dashboardRegistrationSessionCaller) {
	t.Helper()
	store := reg.NewStoreWithDB(db)
	seedDashboardRegistrationUser(t, db, "dashboard_tester", "admin:all", "console:read", "pages:read", "pages:publish", "openapi_sources:read", "openapi_sources:write", "function:invoke")
	nullCache := cache.NewNullCache()
	dispatcher := dispatch.NewDispatcher(store)
	caller := &dashboardRegistrationSessionCaller{payload: responsePayload}
	dispatcher.SetSessionResolver(dashboardRegistrationSessionResolver{caller: caller})
	return &svc.ServiceContext{
		Config:                    config.Config{},
		DB:                        db,
		AdminModel:                model.NewAdminModel(db),
		RoleModel:                 model.NewRoleModel(db),
		PermissionModel:           model.NewPermissionModel(db),
		FunctionModel:             model.NewFunctionModel(db),
		PageSpecModel:             model.NewPageSpecModel(db),
		PageVersionModel:          model.NewPageVersionModel(db),
		PublishedPageSpecModel:    model.NewPublishedPageSpecModel(db),
		RegistryStore:             store,
		OpenAPISourceModel:        model.NewOpenAPISourceModel(db),
		OpenAPISourceBindingModel: model.NewOpenAPISourceBindingModel(db),
		Dispatcher:                dispatcher,
		Cache:                     nullCache,
		CacheHelper:               cache.NewCacheHelper(nullCache),
	}, store, caller
}

func dashboardRegistrationOpenAPISpec(t *testing.T) json.RawMessage {
	t.Helper()
	doc := map[string]interface{}{
		"openapi": "3.0.3",
		"info": map[string]interface{}{
			"title":   "Player API",
			"version": "1.0.0",
		},
		"paths": map[string]interface{}{
			"/players": map[string]interface{}{
				"get": map[string]interface{}{
					"operationId": "player.list",
					"summary":     "List players",
					"parameters": []map[string]interface{}{
						{
							"name":   "page",
							"in":     "query",
							"schema": map[string]interface{}{"type": "integer"},
						},
						{
							"name":   "page_size",
							"in":     "query",
							"schema": map[string]interface{}{"type": "integer"},
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
	raw, err := json.Marshal(doc)
	require.NoError(t, err)
	return raw
}

func findDashboardRegistrationBinding(t *testing.T, bindings []spec.PageFunctionBinding, bindingID string) spec.PageFunctionBinding {
	t.Helper()
	for _, binding := range bindings {
		if binding.ID == bindingID {
			return binding
		}
	}
	t.Fatalf("binding %s not found in %#v", bindingID, bindings)
	return spec.PageFunctionBinding{}
}

func seedDashboardRegistrationUser(t *testing.T, db *gorm.DB, username string, permissions ...string) {
	t.Helper()
	admin := model.Admin{Username: username, Status: 1, PasswordHash: "test"}
	require.NoError(t, db.Create(&admin).Error)
	roleName := username + "_role"
	for _, permissionID := range permissions {
		if strings.TrimSpace(permissionID) == "admin:all" {
			roleName = "admin"
			break
		}
	}
	role := model.Role{Name: roleName, Description: "dashboard registration tester"}
	require.NoError(t, db.Create(&role).Error)
	require.NoError(t, db.Create(&model.AdminRole{AdminID: admin.ID, RoleID: role.ID}).Error)
	for _, permissionID := range permissions {
		permissionID = strings.TrimSpace(permissionID)
		if permissionID == "" {
			continue
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
		require.NoError(t, db.Create(&model.RolePermission{RoleID: role.ID, PermissionID: permissionID}).Error)
	}
}

type dashboardRegistrationSessionResolver struct {
	caller transport.SessionCaller
}

func (r dashboardRegistrationSessionResolver) ResolveAgentConn(string) (transport.SessionCaller, bool) {
	return r.caller, r.caller != nil
}

type dashboardRegistrationSessionCaller struct {
	payload     []byte
	lastRequest *sdkv1.InvokeRequest
}

func (c *dashboardRegistrationSessionCaller) Call(ctx context.Context, msgID uint32, reqBody []byte) (uint32, []byte, error) {
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
