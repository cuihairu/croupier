package console

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/approvals"
	contractsvc "github.com/cuihairu/croupier/internal/service"
	notify "github.com/cuihairu/croupier/internal/service/notify"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestConsoleHandlerPageBindAndGenericErrorV9(t *testing.T) {
	service, _ := newConsoleTestService(t, "console:read")
	handler := NewHandler(service)

	// Missing pageKey URI param fails binding validation.
	bindCtx, bindRec := newGinContext(t, newGinRequest(http.MethodGet, "/api/v1/console/pages/player.manage", ""))
	bindCtx.Params = gin.Params{}
	handler.Page(bindCtx)
	assert.Equal(t, http.StatusBadRequest, bindRec.Code)

	// Non PageNotFoundError (permission denied) goes through the generic error branch.
	denied, _ := newConsoleTestService(t)
	pageCtx, pageRec := newGinContext(t, newGinRequest(http.MethodGet, "/api/v1/console/pages/player.manage", ""))
	pageCtx.Params = gin.Params{{Key: "pageKey", Value: "player.manage"}}
	NewHandler(denied).Page(pageCtx)
	assert.Equal(t, http.StatusForbidden, pageRec.Code)
}

func TestConsoleServiceScopeAndListFailuresV9(t *testing.T) {
	service, scopedCtx := newConsoleTestService(t, "console:read")
	scopeless := context.WithValue(context.Background(), "username", "console_tester")

	_, err := service.Menu(scopeless, &ConsoleMenuRequest{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "X-Game-ID is required")

	_, err = service.Pages(scopeless, &ConsolePagesRequest{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "X-Game-ID is required")

	brokenDB, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	brokenSQL, err := brokenDB.DB()
	require.NoError(t, err)
	require.NoError(t, brokenSQL.Close())
	service.svcCtx.PublishedPageSpecModel = model.NewPublishedPageSpecModel(brokenDB)

	_, err = service.Menu(scopedCtx, &ConsoleMenuRequest{})
	require.Error(t, err)

	_, err = service.Pages(scopedCtx, &ConsolePagesRequest{})
	require.Error(t, err)
}

func TestConsoleServiceFreshnessLoadFailureV9(t *testing.T) {
	service, ctx := newConsoleTestService(t, "console:read", "function:invoke")
	require.NoError(t, seedConsolePublishedPageWithCurrentContracts(service.svcCtx, ctx))
	service.svcCtx.DB = nil

	_, err := service.Pages(ctx, &ConsolePagesRequest{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")

	_, err = service.Page(ctx, &ConsolePageRequest{PageKey: "player.manage"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")

	_, err = service.ExecuteBinding(ctx, &ConsoleExecuteBindingRequest{
		PageKey:   "player.manage",
		BindingID: "player.query",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestConsoleServicePageInvalidSpecV9(t *testing.T) {
	service, ctx := newConsoleTestService(t, "console:read", "function:invoke")
	require.NoError(t, service.svcCtx.PublishedPageSpecModel.Create(ctx, &model.PublishedPageSpec{
		GameID:                "demo-game",
		Env:                   "development",
		PageKey:               "player.manage",
		Version:               1,
		SpecJSON:              "not-json",
		RendererSchemaVersion: "page-spec:1",
		Active:                true,
		PublishedAt:           time.Now(),
		PublishedBy:           "console_tester",
	}))

	_, err := service.Page(ctx, &ConsolePageRequest{PageKey: "player.manage"})
	require.Error(t, err)
	var notFound *PageNotFoundError
	require.ErrorAs(t, err, &notFound)

	_, err = service.ExecuteBinding(ctx, &ConsoleExecuteBindingRequest{
		PageKey:   "player.manage",
		BindingID: "player.query",
	})
	require.ErrorAs(t, err, &notFound)
}

func TestConsoleServiceExecuteBindingMissingContractV9(t *testing.T) {
	service, ctx := newConsoleTestService(t, "function:invoke")
	contractsJSON, err := json.Marshal([]spec.BindingContractSnapshot{{BindingID: "other.binding"}})
	require.NoError(t, err)
	require.NoError(t, seedConsolePublishedPageCustomV9(service.svcCtx, ctx, string(contractsJSON),
		spec.PageFunctionBinding{
			ID:         "player.query",
			FunctionID: "player.query",
			Usage:      spec.BindingUsageQuery,
			Execution:  spec.PageBindingExecution{Mode: spec.PageExecutionModeSync},
		}))

	_, err = service.ExecuteBinding(ctx, &ConsoleExecuteBindingRequest{
		PageKey:   "player.manage",
		BindingID: "player.query",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "binding contract snapshot missing")
}

func TestConsoleServiceExecuteBindingSelectorErrorV9(t *testing.T) {
	service, ctx := newConsoleTestService(t, "function:invoke", "player:query")
	selector := spec.SelectorAST{Assignments: []spec.InputAssignment{
		{
			Target: "/keyword",
			Source: spec.ValueSource{
				Kind:      spec.SourceLiteral,
				Transform: &spec.TransformSpec{Type: spec.TransformType("explode")},
			},
		},
	}}
	require.NoError(t, seedConsolePublishedPageWithSchemaAndSelector(service.svcCtx, ctx,
		`{"type":"object","properties":{"keyword":{"type":"string"}}}`,
		`{"type":"object","properties":{"ok":{"type":"boolean"}}}`,
		selector))

	_, err := service.ExecuteBinding(ctx, &ConsoleExecuteBindingRequest{
		PageKey:   "player.manage",
		BindingID: "player.query",
		Context:   ConsoleBindingExecutionContext{Form: json.RawMessage(`{}`)},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "binding selector transform is not supported")
}

func TestConsoleServiceExecuteBindingTaskApprovalNoStoreV9(t *testing.T) {
	service, ctx := newConsoleTestService(t, "function:invoke")
	require.NoError(t, seedConsoleTaskApprovalPublishingV9(t, service.svcCtx, ctx))

	_, err := service.ExecuteBinding(ctx, &ConsoleExecuteBindingRequest{
		PageKey:   "player.manage",
		BindingID: "player.query",
		Context:   ConsoleBindingExecutionContext{Form: json.RawMessage(`{"keyword":"alice"}`)},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "approval store unavailable")
}

func TestConsoleServiceExecuteBindingInvokeFailureV9(t *testing.T) {
	service, ctx := newConsoleTestService(t, "function:invoke", "player:query")
	require.NoError(t, seedConsolePublishedPageWithCurrentContracts(service.svcCtx, ctx))
	service.svcCtx.Dispatcher.SetSessionResolver(fakeConsoleSessionResolver{
		caller: &fakeConsoleSessionCaller{err: errors.New("agent unreachable")},
	})

	_, err := service.ExecuteBinding(ctx, &ConsoleExecuteBindingRequest{
		PageKey:   "player.manage",
		BindingID: "player.query",
		Context:   ConsoleBindingExecutionContext{Form: json.RawMessage(`{"keyword":"alice"}`)},
	})
	require.Error(t, err)
}

func TestCreatePageApprovalDispatchesNotifyV9(t *testing.T) {
	svcCtx := &svc.ServiceContext{
		ApprovalsStore: approvals.NewMemStore(),
		NotifyService:  notify.New(nil, nil),
	}
	service := NewService(svcCtx)
	ctx := context.WithValue(context.Background(), "username", "notify_tester")

	approvalID, err := service.createPageApproval(ctx, "demo-game", "development",
		spec.PageFunctionBinding{ID: "b1", FunctionID: "player.query"},
		spec.BindingContractSnapshot{BindingID: "b1", FunctionID: "player.query"},
		json.RawMessage(`{"keyword":"a"}`), "", map[string]string{"traceId": "t-9"})
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(approvalID, "page_player_query_"))
}

func TestApprovalRecipientsBranchesV9(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)

	role := model.Role{Name: "admin", Description: "v9 recipients"}
	require.NoError(t, db.Create(&role).Error)
	alice := model.Admin{Username: "alice", Status: 1, PasswordHash: "x"}
	require.NoError(t, db.Create(&alice).Error)
	blank := model.Admin{Username: "", Status: 1, PasswordHash: "x"}
	require.NoError(t, db.Create(&blank).Error)
	require.NoError(t, db.Create(&model.AdminRole{AdminID: alice.ID, RoleID: role.ID}).Error)
	require.NoError(t, db.Create(&model.AdminRole{AdminID: blank.ID, RoleID: role.ID}).Error)

	svcCtx := &svc.ServiceContext{AdminModel: model.NewAdminModel(db)}
	recipients := approvalRecipients(context.Background(), svcCtx)
	assert.Equal(t, []string{"alice"}, recipients)

	require.NoError(t, sqlDB.Close())
	recipients = approvalRecipients(context.Background(), svcCtx)
	assert.Nil(t, recipients)
}

func TestJSONPointerInvalidContainersV9(t *testing.T) {
	_, ok := getJSONPointerValue(json.RawMessage(`{"a":`), "/a")
	assert.False(t, ok)

	_, ok = getJSONPointerValue(json.RawMessage(`[1,`), "/0")
	assert.False(t, ok)

	err := setJSONPointerValue(map[string]json.RawMessage{}, "/a//b", json.RawMessage(`1`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty object key")
}

func TestStartPageExecuteSpanFinisherAttributesV9(t *testing.T) {
	service, _ := newConsoleTestService(t, "function:invoke")
	attachConsoleTestTelemetry(t, service)

	page := spec.PublishedPageSpec{PageSpec: spec.PageSpec{PageKey: "player.manage"}}
	binding := spec.PageFunctionBinding{ID: "player.query", FunctionID: "player.query"}
	contract := spec.BindingContractSnapshot{BindingID: "player.query"}

	_, finishTask := service.startPageExecuteSpan(context.Background(), "demo-game", "development",
		page, binding, contract, "req-task", "tester")
	finishTask(nil, spec.PageExecutionResult{Kind: spec.PageExecutionKindTask, TaskID: "task-1"}, "agent-1")

	_, finishApproval := service.startPageExecuteSpan(context.Background(), "demo-game", "development",
		page, binding, contract, "req-approval", "tester")
	finishApproval(nil, spec.PageExecutionResult{Kind: spec.PageExecutionKindApproval, ApprovalID: "appr-1"}, "")
}

func TestAuditPageExecuteStoreFailureV9(t *testing.T) {
	service, ctx := newConsoleTestService(t, "function:invoke")
	service.svcCtx.AuditService = audit.NewAuditService(failingConsoleAuditStoreV9{}, nil)

	require.NotPanics(t, func() {
		service.auditPageExecute(ctx, "demo-game", "development",
			spec.PublishedPageSpec{PageSpec: spec.PageSpec{PageKey: "player.manage"}},
			spec.PageFunctionBinding{ID: "player.query", FunctionID: "player.query"},
			spec.BindingContractSnapshot{BindingID: "player.query"},
			"req-1", "", spec.PageExecutionResult{}, errors.New("boom"), 5)
	})
}

func seedConsolePublishedPageCustomV9(svcCtx *svc.ServiceContext, ctx context.Context, contractsJSON string, bindings ...spec.PageFunctionBinding) error {
	scope := svc.GameScopeFromContext(ctx)
	page := spec.PageSpec{
		PageKey:     "player.manage",
		Type:        spec.PageTypeOperation,
		ResourceKey: "player",
		Title:       spec.LocalizedText{"zh-CN": "玩家管理"},
		Category: spec.PageCategorySpec{
			Key:    "player",
			Labels: spec.LocalizedText{"zh-CN": "玩家"},
		},
		Bindings: bindings,
	}
	specJSON, err := json.Marshal(page)
	if err != nil {
		return err
	}
	return svcCtx.PublishedPageSpecModel.Create(ctx, &model.PublishedPageSpec{
		GameID:                scope.GameID,
		Env:                   scope.Env,
		PageKey:               "player.manage",
		Version:               1,
		SpecJSON:              string(specJSON),
		BindingContractsJSON:  contractsJSON,
		RendererSchemaVersion: "page-spec:1",
		Active:                true,
		PublishedAt:           time.Now(),
		PublishedBy:           "console_tester",
	})
}

// seedConsoleTaskApprovalPublishingV9 publishes a task-mode binding whose
// contract requires approval, with the current function contract persisted.
func seedConsoleTaskApprovalPublishingV9(t *testing.T, svcCtx *svc.ServiceContext, ctx context.Context) error {
	t.Helper()
	scope := svc.GameScopeFromContext(ctx)
	inputSchema := `{"type":"object","properties":{"keyword":{"type":"string"}}}`
	outputSchema := `{"type":"object","properties":{"ok":{"type":"boolean"}}}`
	inputDigest := testDigestRaw([]byte(inputSchema))
	outputDigest := testDigestRaw([]byte(outputSchema))
	if err := contractsvc.NewContractService(svcCtx.DB).RebuildContractFromFunctionMeta(ctx,
		scope.GameID, scope.Env, "sdk", contractsvc.FunctionMetaInput{
			ID:                "player.query",
			Version:           "1.0.0",
			Enabled:           true,
			Resource:          "player",
			Operation:         "query",
			Capability:        string(spec.CapabilityAction),
			Execution:         string(spec.FunctionExecutionTask),
			Risk:              "safe",
			Permission:        "",
			InputSchema:       inputSchema,
			OutputSchema:      outputSchema,
			ApprovalRequired:  true,
			ApprovalPolicyKey: "two_person",
		}); err != nil {
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
		Bindings: []spec.PageFunctionBinding{
			{
				ID:         "player.query",
				FunctionID: "player.query",
				Usage:      spec.BindingUsageQuery,
				Selectors: &spec.BindingSelectors{
					Input: spec.DefaultSelector(spec.JSONSchema(inputSchema)),
				},
				Execution: spec.PageBindingExecution{Mode: spec.PageExecutionModeTask},
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
			ExecutionMode:         spec.PageExecutionModeTask,
			RendererSchemaVersion: "page-spec:1",
			Approval:              spec.ApprovalPolicy{Required: true, PolicyKey: "two_person"},
		},
	})
	if err != nil {
		return err
	}
	return svcCtx.PublishedPageSpecModel.Create(ctx, &model.PublishedPageSpec{
		GameID:                scope.GameID,
		Env:                   scope.Env,
		PageKey:               "player.manage",
		Version:               1,
		SpecJSON:              string(specJSON),
		BindingContractsJSON:  string(contractsJSON),
		RendererSchemaVersion: "page-spec:1",
		Active:                true,
		PublishedAt:           time.Now(),
		PublishedBy:           "console_tester",
	})
}

type failingConsoleAuditStoreV9 struct{}

func (failingConsoleAuditStoreV9) Create(*audit.AuditRecord) error {
	return errors.New("audit store down")
}
func (failingConsoleAuditStoreV9) Get(string) (*audit.AuditRecord, error) { return nil, nil }
func (failingConsoleAuditStoreV9) List(audit.AuditFilter, audit.AuditPage) ([]*audit.AuditRecord, int, error) {
	return nil, 0, nil
}
func (failingConsoleAuditStoreV9) Delete(string) error { return nil }
func (failingConsoleAuditStoreV9) DeleteBefore(time.Time) (int64, error) {
	return 0, nil
}
func (failingConsoleAuditStoreV9) GetLatestRecord() (*audit.AuditRecord, error) {
	return nil, errors.New("audit chain unavailable")
}
func (failingConsoleAuditStoreV9) GetBySequence(int64) (*audit.AuditRecord, error) {
	return nil, nil
}
func (failingConsoleAuditStoreV9) GetChainRange(int64, int64) ([]*audit.AuditRecord, error) {
	return nil, nil
}
func (failingConsoleAuditStoreV9) GetStats(time.Time, time.Time) (*audit.AuditStats, error) {
	return nil, nil
}
func (failingConsoleAuditStoreV9) CountByFilter(audit.AuditFilter) (int64, error) {
	return 0, nil
}
func (failingConsoleAuditStoreV9) Export(audit.AuditFilter, string) ([]byte, error) {
	return nil, nil
}
