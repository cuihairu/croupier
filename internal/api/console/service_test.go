package console

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/cache"
	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/dispatch"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	contractsvc "github.com/cuihairu/croupier/internal/service"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/cuihairu/croupier/internal/telemetry"
	"github.com/cuihairu/croupier/internal/transport"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
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

func TestServiceMenuUsesPublishedPageScopeAndLabels(t *testing.T) {
	service, ctx := newConsoleTestService(t, "console:read")
	require.NoError(t, seedConsolePublishedPage(service.svcCtx, ctx))
	otherScope := svc.WithGameScope(ctx, svc.GameScope{GameID: "demo-game", Env: "production"})
	require.NoError(t, seedConsolePublishedPageForScope(service.svcCtx, otherScope, "mail.send", "mail", "邮件", 5))

	resp, err := service.Menu(ctx, &ConsoleMenuRequest{Language: "zh-CN"})

	require.NoError(t, err)
	require.Len(t, resp.Items, 1)
	assert.Equal(t, "player", resp.Items[0].Key)
	assert.Equal(t, "玩家", resp.Items[0].Title["zh-CN"])
	assert.Equal(t, "/console/player", resp.Items[0].Path)
	require.Len(t, resp.Items[0].Children, 1)
	assert.Equal(t, "/console/player/player.manage", resp.Items[0].Children[0].Path)
}

func TestGenerateMenuFromPagesUsesLowestPublishedPageOrderForCategory(t *testing.T) {
	menu := generateMenuFromPages([]spec.PublishedPageSpec{
		{
			PageSpec: spec.PageSpec{
				PageKey: "late.page",
				Title:   spec.LocalizedText{"zh-CN": "后"},
				Order:   100,
				Category: spec.PageCategorySpec{
					Key:    "late",
					Labels: spec.LocalizedText{"zh-CN": "后分类"},
					Order:  1,
				},
			},
		},
		{
			PageSpec: spec.PageSpec{
				PageKey: "early.page",
				Title:   spec.LocalizedText{"zh-CN": "前"},
				Order:   10,
				Category: spec.PageCategorySpec{
					Key:    "early",
					Labels: spec.LocalizedText{"zh-CN": "前分类"},
					Order:  999,
				},
			},
		},
	}, "zh-CN")

	require.Len(t, menu.Items, 2)
	assert.Equal(t, "early", menu.Items[0].Key)
	assert.Equal(t, 10, menu.Items[0].Order)
	assert.Equal(t, "/console/early", menu.Items[0].Path)
	assert.Equal(t, "/console/player%20ops/player.ban", consolePagePath("player ops", "player.ban"))
}

func TestServiceExecuteBindingRequiresFunctionInvokePermission(t *testing.T) {
	service, ctx := newConsoleTestService(t, "console:read", "pages:read")
	require.NoError(t, seedConsolePublishedPage(service.svcCtx, ctx))

	_, err := service.ExecuteBinding(ctx, &ConsoleExecuteBindingRequest{
		PageKey:   "player.manage",
		BindingID: "player.query",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "无权执行运行控制台操作")
}

func TestServiceExecuteBindingRequiresPublishedSnapshotPermission(t *testing.T) {
	service, ctx := newConsoleTestService(t, "function:invoke")
	require.NoError(t, seedConsolePublishedPageWithCurrentContracts(service.svcCtx, ctx))

	_, err := service.ExecuteBinding(ctx, &ConsoleExecuteBindingRequest{
		PageKey:   "player.manage",
		BindingID: "player.query",
		Context: ConsoleBindingExecutionContext{
			Form: json.RawMessage(`{"keyword":"alice"}`),
		},
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "无权执行该页面操作")
}

func TestServiceExecuteBindingWritesAuditWithPageContext(t *testing.T) {
	service, ctx, auditStore := newConsoleTestServiceWithAudit(t, "function:invoke", "player:query")
	spanRecorder := attachConsoleTestTelemetry(t, service)
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
		Context: ConsoleBindingExecutionContext{
			Form: json.RawMessage(`{"keyword":"alice"}`),
		},
	})

	require.NoError(t, err)
	require.NotEmpty(t, resp.Result.RequestID)
	require.NotEmpty(t, resp.Result.TraceID)
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
	assert.Equal(t, "agent-1", record.Details["target"])
	assert.Equal(t, 1, record.Details["publish_version"])
	assert.Equal(t, "resource:player", record.Details["base_proposal_key"])
	assert.Equal(t, 7, record.Details["base_proposal_version"])
	assert.Equal(t, "function-digest-1", record.Details["function_digest"])
	assert.Equal(t, "semantics-digest-1", record.Details["semantics_digest"])
	assert.Equal(t, "page-generator:test", record.Details["generator_version"])
	assert.Equal(t, "agent-1", record.Resource.Metadata["target"])
	require.NotNil(t, caller.lastRequest)
	assert.Equal(t, "player.manage", caller.lastRequest.Metadata["page_key"])
	assert.Equal(t, "player.query", caller.lastRequest.Metadata["binding_id"])
	assert.Equal(t, "resource:player", caller.lastRequest.Metadata["base_proposal_key"])
	assert.Equal(t, "7", caller.lastRequest.Metadata["base_proposal_version"])
	assert.Equal(t, "function-digest-1", caller.lastRequest.Metadata["function_digest"])
	assert.Equal(t, "semantics-digest-1", caller.lastRequest.Metadata["semantics_digest"])
	assert.Equal(t, "page-generator:test", caller.lastRequest.Metadata["generator_version"])
	assert.Equal(t, "agent-1", caller.lastRequest.Metadata["agent_id"])
	assert.Equal(t, resp.Result.RequestID, caller.lastRequest.Metadata["page_request_id"])
	assert.Equal(t, resp.Result.TraceID, caller.lastRequest.Metadata["trace_id"])
	assert.Equal(t, "console.binding.execute", caller.lastRequest.Metadata["page_runtime_api"])

	pageSpan := findEndedSpan(t, spanRecorder, "page.binding.execute")
	assertSpanStringAttr(t, pageSpan, "request.id", resp.Result.RequestID)
	assertSpanStringAttr(t, pageSpan, "trace_id", resp.Result.TraceID)
	assertSpanStringAttr(t, pageSpan, "game.id", "demo-game")
	assertSpanStringAttr(t, pageSpan, "game.env", "development")
	assertSpanStringAttr(t, pageSpan, "actor", "console_tester")
	assertSpanStringAttr(t, pageSpan, "page.key", "player.manage")
	assertSpanIntAttr(t, pageSpan, "page.publish_version", 1)
	assertSpanStringAttr(t, pageSpan, "page.base_proposal_key", "resource:player")
	assertSpanIntAttr(t, pageSpan, "page.base_proposal_version", 7)
	assertSpanStringAttr(t, pageSpan, "page.function_digest", "function-digest-1")
	assertSpanStringAttr(t, pageSpan, "page.semantics_digest", "semantics-digest-1")
	assertSpanStringAttr(t, pageSpan, "page.generator_version", "page-generator:test")
	assertSpanStringAttr(t, pageSpan, "page.binding_id", "player.query")
	assertSpanStringAttr(t, pageSpan, "page.binding_usage", string(spec.BindingUsageQuery))
	assertSpanStringAttr(t, pageSpan, "page.execution_mode", string(spec.PageExecutionModeSync))
	assertSpanStringAttr(t, pageSpan, "page.result_kind", string(spec.PageExecutionKindSync))
	assertSpanStringAttr(t, pageSpan, "function.id", "player.query")
	assertSpanStringAttr(t, pageSpan, "target", "agent-1")
}

func TestServiceExecuteBindingWritesAuditOnBindingStale(t *testing.T) {
	service, ctx, auditStore := newConsoleTestServiceWithAudit(t, "function:invoke")
	require.NoError(t, seedConsolePublishedPage(service.svcCtx, ctx))

	_, err := service.ExecuteBinding(ctx, &ConsoleExecuteBindingRequest{
		PageKey:   "player.manage",
		BindingID: "player.query",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "binding_stale")

	records, total, listErr := auditStore.List(audit.AuditFilter{
		EventType: []audit.AuditEventType{audit.EventPageExecute},
	}, audit.AuditPage{PageSize: 10})
	require.NoError(t, listErr)
	require.Equal(t, 1, total)
	require.Len(t, records, 1)
	assert.Equal(t, "failure", records[0].Outcome)
	assert.Equal(t, "player.manage", records[0].Details["page_key"])
	assert.Equal(t, "player.query", records[0].Details["binding_id"])
	assert.Equal(t, "player.query", records[0].Details["function_id"])
	assert.NotEmpty(t, records[0].Details["request_id"])
}

func TestServiceExecuteBindingRejectsPayloadTypeMismatch(t *testing.T) {
	service, ctx := newConsoleTestService(t, "function:invoke", "player:query")
	require.NoError(t, seedConsolePublishedPageWithCurrentContracts(service.svcCtx, ctx))
	caller := &fakeConsoleSessionCaller{
		payload: []byte(`{"ok":true}`),
	}
	service.svcCtx.Dispatcher.SetSessionResolver(fakeConsoleSessionResolver{
		caller: caller,
	})

	_, err := service.ExecuteBinding(ctx, &ConsoleExecuteBindingRequest{
		PageKey:   "player.manage",
		BindingID: "player.query",
		Context: ConsoleBindingExecutionContext{
			Form: json.RawMessage(`{"keyword":123}`),
		},
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "binding payload does not match input schema")
	assert.Nil(t, caller.lastRequest)
}

func TestServiceExecuteBindingIgnoresUnselectedContextFields(t *testing.T) {
	service, ctx := newConsoleTestService(t, "function:invoke", "player:query")
	require.NoError(t, seedConsolePublishedPageWithCurrentContracts(service.svcCtx, ctx))
	caller := &fakeConsoleSessionCaller{
		payload: []byte(`{"ok":true}`),
	}
	service.svcCtx.Dispatcher.SetSessionResolver(fakeConsoleSessionResolver{
		caller: caller,
	})

	_, err := service.ExecuteBinding(ctx, &ConsoleExecuteBindingRequest{
		PageKey:   "player.manage",
		BindingID: "player.query",
		Context: ConsoleBindingExecutionContext{
			Form: json.RawMessage(`{"keyword":"alice","role":"admin"}`),
		},
	})

	require.NoError(t, err)
	require.NotNil(t, caller.lastRequest)
	assert.JSONEq(t, `{"keyword":"alice"}`, string(caller.lastRequest.Payload))
}

func TestServiceExecuteBindingUsesRowSelectorWithoutPassingWholeRow(t *testing.T) {
	service, ctx := newConsoleTestService(t, "function:invoke", "player:query")
	inputSchema := `{"type":"object","properties":{"playerId":{"type":"string"}},"required":["playerId"]}`
	outputSchema := `{"type":"object","properties":{"ok":{"type":"boolean"}}}`
	selector := spec.SelectorAST{Assignments: []spec.InputAssignment{
		{
			Target: "/playerId",
			Source: spec.ValueSource{
				Kind: spec.SourceRow,
				Path: "/id",
			},
		},
	}}
	require.NoError(t, seedConsolePublishedPageWithSchemaAndSelector(service.svcCtx, ctx, inputSchema, outputSchema, selector))
	caller := &fakeConsoleSessionCaller{
		payload: []byte(`{"ok":true}`),
	}
	service.svcCtx.Dispatcher.SetSessionResolver(fakeConsoleSessionResolver{
		caller: caller,
	})

	_, err := service.ExecuteBinding(ctx, &ConsoleExecuteBindingRequest{
		PageKey:   "player.manage",
		BindingID: "player.query",
		Context: ConsoleBindingExecutionContext{
			Row: json.RawMessage(`{"id":"p1","role":"admin","balance":999}`),
		},
	})

	require.NoError(t, err)
	require.NotNil(t, caller.lastRequest)
	assert.JSONEq(t, `{"playerId":"p1"}`, string(caller.lastRequest.Payload))
}

func TestServiceExecuteBindingRejectsMissingRequiredPayloadField(t *testing.T) {
	service, ctx := newConsoleTestService(t, "function:invoke", "player:query")
	inputSchema := `{"type":"object","properties":{"playerId":{"type":"string"},"reason":{"type":"string"}},"required":["playerId"]}`
	outputSchema := `{"type":"object","properties":{"ok":{"type":"boolean"}}}`
	require.NoError(t, seedConsolePublishedPageWithSchema(service.svcCtx, ctx, inputSchema, outputSchema))
	caller := &fakeConsoleSessionCaller{
		payload: []byte(`{"ok":true}`),
	}
	service.svcCtx.Dispatcher.SetSessionResolver(fakeConsoleSessionResolver{
		caller: caller,
	})

	_, err := service.ExecuteBinding(ctx, &ConsoleExecuteBindingRequest{
		PageKey:   "player.manage",
		BindingID: "player.query",
		Context: ConsoleBindingExecutionContext{
			Form: json.RawMessage(`{"reason":"abuse"}`),
		},
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "binding payload does not match input schema")
	assert.Nil(t, caller.lastRequest)
}

func TestServiceExecuteBindingRejectsChangedRiskOrPermission(t *testing.T) {
	service, ctx := newConsoleTestService(t, "function:invoke")
	require.NoError(t, seedConsolePublishedPageWithCurrentContracts(service.svcCtx, ctx))
	require.NoError(t, upsertConsoleFunctionContract(service.svcCtx, ctx,
		`{"type":"object","properties":{"keyword":{"type":"string"}}}`,
		`{"type":"object","properties":{"items":{"type":"array"},"total":{"type":"number"}}}`,
		"danger",
		"player:admin",
	))

	_, err := service.ExecuteBinding(ctx, &ConsoleExecuteBindingRequest{
		PageKey:   "player.manage",
		BindingID: "player.query",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "binding_stale")
}

func TestServicePageReturnsBindingFreshnessDiagnostics(t *testing.T) {
	service, ctx := newConsoleTestService(t, "console:read")
	require.NoError(t, seedConsolePublishedPageWithCurrentContracts(service.svcCtx, ctx))
	require.NoError(t, upsertConsoleFunctionContract(service.svcCtx, ctx,
		`{"type":"object","properties":{"keyword":{"type":"string"},"region":{"type":"string"}}}`,
		`{"type":"object","properties":{"items":{"type":"array"},"total":{"type":"number"}}}`,
		"safe",
		"player:query",
	))

	pageResp, err := service.Page(ctx, &ConsolePageRequest{PageKey: "player.manage"})
	require.NoError(t, err)
	require.Len(t, pageResp.Page.BindingFreshness, 1)
	assert.Equal(t, spec.BindingFreshnessInputSchemaStale, pageResp.Page.BindingFreshness[0].Status)
	assert.Equal(t, "binding_input_schema_stale", pageResp.Page.BindingFreshness[0].Diagnostic.Code)

	pagesResp, err := service.Pages(ctx, &ConsolePagesRequest{})
	require.NoError(t, err)
	require.Len(t, pagesResp.Items, 1)
	require.Len(t, pagesResp.Items[0].BindingFreshness, 1)
	assert.Equal(t, spec.BindingFreshnessInputSchemaStale, pagesResp.Items[0].BindingFreshness[0].Status)
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
				Risk:         "safe",
				Permission:   "player:query",
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

func attachConsoleTestTelemetry(t *testing.T, service *Service) *tracetest.SpanRecorder {
	t.Helper()

	spanRecorder := tracetest.NewSpanRecorder()
	previousProvider := otel.GetTracerProvider()
	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithSpanProcessor(spanRecorder),
	)
	otel.SetTracerProvider(tracerProvider)

	telemetryService, err := telemetry.NewGameTelemetryService(telemetry.TelemetryConfig{
		ServiceName:    "console-test",
		ServiceVersion: "test",
		Environment:    "test",
		CollectorURL:   "localhost:4318",
		GameID:         "demo-game",
		EnableTracing:  false,
		EnableMetrics:  false,
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	require.NoError(t, err)
	service.svcCtx.Telemetry = telemetryService

	t.Cleanup(func() {
		require.NoError(t, telemetryService.Shutdown(context.Background()))
		require.NoError(t, tracerProvider.Shutdown(context.Background()))
		otel.SetTracerProvider(previousProvider)
	})
	return spanRecorder
}

func findEndedSpan(t *testing.T, recorder *tracetest.SpanRecorder, name string) sdktrace.ReadOnlySpan {
	t.Helper()
	for _, span := range recorder.Ended() {
		if span.Name() == name {
			return span
		}
	}
	t.Fatalf("span %q not found", name)
	return nil
}

func assertSpanStringAttr(t *testing.T, span sdktrace.ReadOnlySpan, key string, want string) {
	t.Helper()
	value, ok := spanAttribute(span, key)
	require.Truef(t, ok, "span attribute %q missing", key)
	assert.Equal(t, want, value.AsString())
}

func assertSpanIntAttr(t *testing.T, span sdktrace.ReadOnlySpan, key string, want int64) {
	t.Helper()
	value, ok := spanAttribute(span, key)
	require.Truef(t, ok, "span attribute %q missing", key)
	assert.Equal(t, want, value.AsInt64())
}

func spanAttribute(span sdktrace.ReadOnlySpan, key string) (attribute.Value, bool) {
	if span == nil {
		return attribute.Value{}, false
	}
	for _, attr := range span.Attributes() {
		if string(attr.Key) == key {
			return attr.Value, true
		}
	}
	return attribute.Value{}, false
}

func seedConsolePublishedPage(svcCtx *svc.ServiceContext, ctx context.Context) error {
	return seedConsolePublishedPageForScope(svcCtx, ctx, "player.manage", "player", "玩家", 1)
}

func seedConsolePublishedPageForScope(svcCtx *svc.ServiceContext, ctx context.Context, pageKey string, categoryKey string, categoryTitle string, order int) error {
	gameID, env := svc.GameScopeFromContext(ctx)
	page := spec.PageSpec{
		PageKey:     pageKey,
		Type:        spec.PageTypeOperation,
		ResourceKey: categoryKey,
		Title:       spec.LocalizedText{"zh-CN": pageKey},
		Category: spec.PageCategorySpec{
			Key:    categoryKey,
			Labels: spec.LocalizedText{"zh-CN": categoryTitle},
		},
		Order:     order,
		Operation: testConsoleOperationPageSpec(),
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
			RendererSchemaVersion: "page-spec:1",
		},
	})
	if err != nil {
		return err
	}
	return svcCtx.PublishedPageSpecModel.Create(ctx, &model.PublishedPageSpec{
		GameID:                gameID,
		Env:                   env,
		PageKey:               pageKey,
		Version:               1,
		SpecJSON:              string(specJSON),
		BindingContractsJSON:  string(contractsJSON),
		RendererSchemaVersion: "page-spec:1",
		BaseProposalKey:       "resource:player",
		BaseProposalVersion:   7,
		FunctionDigest:        "function-digest-1",
		SemanticsDigest:       "semantics-digest-1",
		GeneratorVersion:      "page-generator:test",
		Active:                true,
		PublishedAt:           time.Now(),
		PublishedBy:           "console_tester",
	})
}

func seedConsolePublishedPageWithCurrentContracts(svcCtx *svc.ServiceContext, ctx context.Context) error {
	return seedConsolePublishedPageWithSchema(
		svcCtx,
		ctx,
		`{"type":"object","properties":{"keyword":{"type":"string"}}}`,
		`{"type":"object","properties":{"items":{"type":"array"},"total":{"type":"number"}}}`,
	)
}

func seedConsolePublishedPageWithSchema(svcCtx *svc.ServiceContext, ctx context.Context, inputSchema string, outputSchema string) error {
	return seedConsolePublishedPageWithSchemaAndSelector(
		svcCtx,
		ctx,
		inputSchema,
		outputSchema,
		spec.DefaultSelector(spec.JSONSchema(inputSchema)),
	)
}

func seedConsolePublishedPageWithSchemaAndSelector(svcCtx *svc.ServiceContext, ctx context.Context, inputSchema string, outputSchema string, selector spec.SelectorAST) error {
	gameID, env := svc.GameScopeFromContext(ctx)
	inputDigest := testDigestRaw([]byte(inputSchema))
	outputDigest := testDigestRaw([]byte(outputSchema))
	if err := upsertConsoleFunctionContract(svcCtx, ctx, inputSchema, outputSchema, "safe", "player:query"); err != nil {
		return err
	}
	page := spec.PageSpec{
		PageKey:     "player.manage",
		Type:        spec.PageTypeOperation,
		ResourceKey: "player",
		Title:       spec.LocalizedText{"zh-CN": "玩家管理"},
		Category: spec.PageCategorySpec{
			Key:    "player",
			Labels: spec.LocalizedText{"zh-CN": "玩家"},
		},
		Operation: testConsoleOperationPageSpec(),
		Bindings: []spec.PageFunctionBinding{
			{
				ID:         "player.query",
				FunctionID: "player.query",
				Usage:      spec.BindingUsageQuery,
				Selectors: &spec.BindingSelectors{
					Input: selector,
				},
				Execution: spec.PageBindingExecution{Mode: spec.PageExecutionModeSync},
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
			Risk:                  spec.RiskSafe,
			Permission:            "player:query",
			ExecutionMode:         spec.PageExecutionModeSync,
			RendererSchemaVersion: "page-spec:1",
		},
	})
	if err != nil {
		return err
	}
	return svcCtx.PublishedPageSpecModel.Create(ctx, &model.PublishedPageSpec{
		GameID:                gameID,
		Env:                   env,
		PageKey:               "player.manage",
		Version:               1,
		SpecJSON:              string(specJSON),
		BindingContractsJSON:  string(contractsJSON),
		RendererSchemaVersion: "page-spec:1",
		BaseProposalKey:       "resource:player",
		BaseProposalVersion:   7,
		FunctionDigest:        "function-digest-1",
		SemanticsDigest:       "semantics-digest-1",
		GeneratorVersion:      "page-generator:test",
		Active:                true,
		PublishedAt:           time.Now(),
		PublishedBy:           "console_tester",
	})
}

func upsertConsoleFunctionContract(svcCtx *svc.ServiceContext, ctx context.Context, inputSchema string, outputSchema string, risk string, permission string) error {
	gameID, env := svc.GameScopeFromContext(ctx)
	return contractsvc.NewContractService(svcCtx.DB).RebuildContractFromFunctionMeta(ctx, gameID, env, "sdk", contractsvc.FunctionMetaInput{
		ID:           "player.query",
		Version:      "1.0.0",
		Enabled:      true,
		Resource:     "player",
		Operation:    "query",
		Capability:   string(spec.CapabilityAction),
		Execution:    string(spec.FunctionExecutionSync),
		Risk:         risk,
		Permission:   permission,
		InputSchema:  inputSchema,
		OutputSchema: outputSchema,
	})
}

func testConsoleOperationPageSpec() *spec.OperationPageSpec {
	return &spec.OperationPageSpec{
		Form: spec.DefaultFormPresentation(spec.JSONSchema(`{
			"type":"object",
			"properties":{"keyword":{"type":"string"}}
		}`)),
		ResultView: &spec.ResultViewSpec{
			SuccessMessage: spec.LocalizedText{"zh-CN": "操作成功"},
			ErrorMessage:   spec.LocalizedText{"zh-CN": "操作失败"},
		},
	}
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

func testDigestRaw(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
