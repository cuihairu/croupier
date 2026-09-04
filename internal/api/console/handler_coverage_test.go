package console

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/approvals"
	contractsvc "github.com/cuihairu/croupier/internal/service"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newGinRequest(method, target, body string) *http.Request {
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, target, reader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	ctx := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "demo-game", Env: "development"})
	ctx = context.WithValue(ctx, "username", "console_tester")
	return req.WithContext(ctx)
}

func newGinContext(t *testing.T, req *http.Request) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = req
	return ctx, rec
}

func executeBindingURIParams(pageKey, bindingID string) gin.Params {
	return gin.Params{
		{Key: "pageKey", Value: pageKey},
		{Key: "bindingId", Value: bindingID},
	}
}

func TestNewHandlerAndMenuHandler(t *testing.T) {
	service, ctx := newConsoleTestService(t, "console:read")
	require.NoError(t, seedConsolePublishedPage(service.svcCtx, ctx))

	handler := NewHandler(service)

	menuCtx, rec := newGinContext(t, newGinRequest(http.MethodGet, "/api/v1/console/menu?lang=en-US", ""))
	handler.Menu(menuCtx)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "player")

	// Unauthorized request surfaces the permission error.
	unauthorized, _ := newConsoleTestService(t)
	deniedCtx, deniedRec := newGinContext(t, newGinRequest(http.MethodGet, "/api/v1/console/menu", ""))
	NewHandler(unauthorized).Menu(deniedCtx)
	assert.Equal(t, http.StatusForbidden, deniedRec.Code)
}

func TestPagesHandlerReturnsItems(t *testing.T) {
	service, ctx := newConsoleTestService(t, "console:read")
	require.NoError(t, seedConsolePublishedPage(service.svcCtx, ctx))
	handler := NewHandler(service)

	pagesCtx, rec := newGinContext(t, newGinRequest(http.MethodGet, "/api/v1/console/pages", ""))
	handler.Pages(pagesCtx)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "player.manage")

	filteredCtx, filteredRec := newGinContext(t, newGinRequest(http.MethodGet, "/api/v1/console/pages?category=mail", ""))
	handler.Pages(filteredCtx)
	require.Equal(t, http.StatusOK, filteredRec.Code)
	assert.NotContains(t, filteredRec.Body.String(), "player.manage")

	deniedSvc, _ := newConsoleTestService(t)
	deniedCtx, deniedRec := newGinContext(t, newGinRequest(http.MethodGet, "/api/v1/console/pages", ""))
	NewHandler(deniedSvc).Pages(deniedCtx)
	assert.Equal(t, http.StatusForbidden, deniedRec.Code)
}

func TestPageHandlerSuccessAndNotFound(t *testing.T) {
	service, ctx := newConsoleTestService(t, "console:read")
	require.NoError(t, seedConsolePublishedPage(service.svcCtx, ctx))
	handler := NewHandler(service)

	pageCtx, rec := newGinContext(t, newGinRequest(http.MethodGet, "/api/v1/console/pages/player.manage", ""))
	pageCtx.Params = gin.Params{{Key: "pageKey", Value: "player.manage"}}
	handler.Page(pageCtx)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "player.manage")

	missingCtx, missingRec := newGinContext(t, newGinRequest(http.MethodGet, "/api/v1/console/pages/nope", ""))
	missingCtx.Params = gin.Params{{Key: "pageKey", Value: "nope"}}
	handler.Page(missingCtx)
	assert.Equal(t, http.StatusNotFound, missingRec.Code)
}

func TestExecuteBindingHandler(t *testing.T) {
	service, ctx, _ := newConsoleTestServiceWithAudit(t, "function:invoke", "player:query")
	require.NoError(t, seedConsolePublishedPageWithCurrentContracts(service.svcCtx, ctx))
	service.svcCtx.Dispatcher.SetSessionResolver(fakeConsoleSessionResolver{
		caller: &fakeConsoleSessionCaller{payload: []byte(`{"ok":true}`)},
	})
	handler := NewHandler(service)

	okCtx, rec := newGinContext(t, newGinRequest(http.MethodPost,
		"/api/v1/console/pages/player.manage/bindings/player.query/execute",
		`{"context":{"form":{"keyword":"alice"}}}`))
	okCtx.Params = executeBindingURIParams("player.manage", "player.query")
	handler.ExecuteBinding(okCtx)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"requestId"`)

	// Missing URI params fail binding validation.
	badUriCtx, badUriRec := newGinContext(t, newGinRequest(http.MethodPost,
		"/api/v1/console/pages//bindings//execute", `{"context":{}}`))
	handler.ExecuteBinding(badUriCtx)
	assert.Equal(t, http.StatusBadRequest, badUriRec.Code)

	// Malformed JSON body fails JSON binding.
	badJsonCtx, badJsonRec := newGinContext(t, newGinRequest(http.MethodPost,
		"/api/v1/console/pages/player.manage/bindings/player.query/execute", `{invalid`))
	badJsonCtx.Params = executeBindingURIParams("player.manage", "player.query")
	handler.ExecuteBinding(badJsonCtx)
	assert.Equal(t, http.StatusBadRequest, badJsonRec.Code)

	// Unknown page maps to 404 through the PageNotFoundError branch.
	unknownCtx, unknownRec := newGinContext(t, newGinRequest(http.MethodPost,
		"/api/v1/console/pages/nope/bindings/x/execute", `{"context":{}}`))
	unknownCtx.Params = executeBindingURIParams("nope", "x")
	handler.ExecuteBinding(unknownCtx)
	assert.Equal(t, http.StatusNotFound, unknownRec.Code)

	// Generic service error (stale snapshot) maps to the plain error branch.
	noContractSvc, noContractCtx := newConsoleTestService(t, "function:invoke", "player:query")
	require.NoError(t, seedConsolePublishedPage(noContractSvc.svcCtx, noContractCtx))
	errCtx, errRec := newGinContext(t, newGinRequest(http.MethodPost,
		"/api/v1/console/pages/player.manage/bindings/player.query/execute", `{"context":{}}`))
	errCtx.Params = executeBindingURIParams("player.manage", "player.query")
	NewHandler(noContractSvc).ExecuteBinding(errCtx)
	assert.Equal(t, http.StatusConflict, errRec.Code)
}

func TestCreatePageApprovalStoresPendingApproval(t *testing.T) {
	service, ctx := newConsoleTestService(t, "function:invoke")
	store := approvals.NewMemStore()
	service.svcCtx.ApprovalsStore = store

	binding := spec.PageFunctionBinding{ID: "b1", FunctionID: "player.query"}
	contract := spec.BindingContractSnapshot{BindingID: "b1", FunctionID: "player.query"}
	approvalID, err := service.createPageApproval(ctx, "demo-game", "development",
		binding, contract, json.RawMessage(`{"keyword":"a"}`), "", map[string]string{"traceId": "t-1"})
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(approvalID, "page_player_query_"))

	stored, getErr := store.Get(approvalID)
	require.NoError(t, getErr)
	assert.Equal(t, "pending", stored.State)
	assert.Equal(t, "demo-game", stored.GameID)
	assert.Equal(t, "development", stored.Env)
	assert.JSONEq(t, `{"keyword":"a"}`, string(stored.Payload))
	assert.Equal(t, "t-1", stored.Metadata["traceId"])
}

func TestCreatePageApprovalSanitizesFunctionID(t *testing.T) {
	service, ctx := newConsoleTestService(t, "function:invoke")
	service.svcCtx.ApprovalsStore = approvals.NewMemStore()

	approvalID, err := service.createPageApproval(ctx, " g ", " e ",
		spec.PageFunctionBinding{ID: "b1", FunctionID: ""},
		spec.BindingContractSnapshot{BindingID: "b1", FunctionID: "  Player.Query#01  "},
		json.RawMessage(`null`), " async ", nil)
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(approvalID, "page_player_query01_"))
}

func TestCreatePageApprovalRejectsMissingStoreOrFunction(t *testing.T) {
	service, ctx := newConsoleTestService(t, "function:invoke")

	_, err := service.createPageApproval(ctx, "g", "e",
		spec.PageFunctionBinding{ID: "b1", FunctionID: "f"},
		spec.BindingContractSnapshot{}, json.RawMessage(`{}`), "", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "approval store unavailable")

	service.svcCtx.ApprovalsStore = approvals.NewMemStore()
	_, err = service.createPageApproval(ctx, "g", "e",
		spec.PageFunctionBinding{ID: "b1", FunctionID: "  "},
		spec.BindingContractSnapshot{}, json.RawMessage(`{}`), "", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "function is required")
}

// seedConsoleApprovedPublishing publishes a page whose binding contract matches
// the persisted function contract and requires approval before execution.
func seedConsoleApprovedPublishing(t *testing.T, svcCtx *svc.ServiceContext, ctx context.Context) error {
	t.Helper()
	scope := svc.GameScopeFromContext(ctx)
	inputSchema := `{"type":"object","properties":{"keyword":{"type":"string"}}}`
	outputSchema := `{"type":"object","properties":{"ok":{"type":"boolean"}}}`
	inputDigest := testDigestRaw([]byte(inputSchema))
	outputDigest := testDigestRaw([]byte(outputSchema))
	dbSvcCtx := &svc.ServiceContext{DB: svcCtx.DB}
	err := contractsvc.NewContractService(dbSvcCtx.DB).RebuildContractFromFunctionMeta(ctx,
		scope.GameID, scope.Env, "sdk", contractsvc.FunctionMetaInput{
			ID:                "player.query",
			Version:           "1.0.0",
			Enabled:           true,
			Resource:          "player",
			Operation:         "query",
			Capability:        string(spec.CapabilityAction),
			Execution:         string(spec.FunctionExecutionSync),
			Risk:              "safe",
			Permission:        "",
			InputSchema:       inputSchema,
			OutputSchema:      outputSchema,
			ApprovalRequired:  true,
			ApprovalPolicyKey: "two_person",
		})
	if err != nil {
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
			ExecutionMode:         spec.PageExecutionModeSync,
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

func TestExecuteBindingCreatesApprovalWhenRequired(t *testing.T) {
	service, ctx, _ := newConsoleTestServiceWithAudit(t, "function:invoke")
	store := approvals.NewMemStore()
	service.svcCtx.ApprovalsStore = store
	scope := svc.GameScopeFromContext(ctx)

	require.NoError(t, seedConsoleApprovedPublishing(t, service.svcCtx, ctx))

	caller := &fakeConsoleSessionCaller{payload: []byte(`{"ok":true}`)}
	service.svcCtx.Dispatcher.SetSessionResolver(fakeConsoleSessionResolver{caller: caller})

	resp, err := service.ExecuteBinding(ctx, &ConsoleExecuteBindingRequest{
		PageKey:   "player.manage",
		BindingID: "player.query",
		Context: ConsoleBindingExecutionContext{
			Form: json.RawMessage(`{"keyword":"alice"}`),
		},
	})
	require.NoError(t, err)
	assert.Equal(t, spec.PageExecutionKindApproval, resp.Result.Kind)
	require.NotEmpty(t, resp.Result.ApprovalID)

	stored, getErr := store.Get(resp.Result.ApprovalID)
	require.NoError(t, getErr)
	assert.Equal(t, "pending", stored.State)
	assert.Equal(t, "player.query", stored.FunctionID)
	assert.Equal(t, scope.GameID, stored.GameID)
	assert.Empty(t, caller.lastRequest)
}

func TestAttachBindingFreshnessErrorBranches(t *testing.T) {
	svcCtx := &svc.ServiceContext{}
	ctx := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "demo-game", Env: "development"})
	service := NewService(svcCtx)

	pages := []spec.PublishedPageSpec{{PageSpec: spec.PageSpec{PageKey: "p"}}}
	_, err := service.attachBindingFreshness(ctx, pages)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")

	err = service.attachBindingFreshnessToPage(ctx, &spec.PublishedPageSpec{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")

	// Empty page list short-circuits without touching the database.
	out, err := service.attachBindingFreshness(ctx, nil)
	require.NoError(t, err)
	assert.Empty(t, out)

	// Nil page is a no-op.
	require.NoError(t, service.attachBindingFreshnessToPage(ctx, nil))

	// A healthy context evaluates freshness against persisted contracts.
	base, _ := newConsoleTestService(t)
	dbSvcCtx := &svc.ServiceContext{DB: base.svcCtx.DB}
	require.NoError(t, upsertConsoleFunctionContract(dbSvcCtx, ctx,
		`{"type":"object","properties":{"keyword":{"type":"string"}}}`,
		`{"type":"object","properties":{"ok":{"type":"boolean"}}}`, "safe", ""))
	healthy := NewService(dbSvcCtx)
	fresh, err := healthy.attachBindingFreshness(ctx, []spec.PublishedPageSpec{
		{PageSpec: spec.PageSpec{PageKey: "p"}},
	})
	require.NoError(t, err)
	assert.Empty(t, fresh[0].BindingFreshness)
}

func TestAuditPageExecuteWithoutAuditService(t *testing.T) {
	service := NewService(&svc.ServiceContext{})
	require.NotPanics(t, func() {
		service.auditPageExecute(context.Background(), "g", "e",
			spec.PublishedPageSpec{}, spec.PageFunctionBinding{}, spec.BindingContractSnapshot{},
			"req", "", spec.PageExecutionResult{}, nil, 0)
	})
}

func TestPickSelectionValuesRowPointerMissing(t *testing.T) {
	val, found, err := pickSelectionValues(json.RawMessage(`[{"id":"a"},{"other":1}]`), "/id")
	require.NoError(t, err)
	assert.False(t, found)
	assert.Nil(t, val)
}

func TestRequireScopeMissingEnv(t *testing.T) {
	_, _, err := requireScope(svc.WithGameScope(context.Background(), svc.GameScope{GameID: "demo-game"}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "X-Env is required")
}

func TestSanitizeApprovalIDKeepsDigitsV2(t *testing.T) {
	assert.Equal(t, "fn_01", sanitizeApprovalID("FN-01"))
}

func TestGetJSONPointerValueScalarRejectsPointer(t *testing.T) {
	val, ok := getJSONPointerValue(json.RawMessage(`"scalar"`), "/a")
	assert.False(t, ok)
	assert.Nil(t, val)
}

func TestValueFromRawContextMissingPath(t *testing.T) {
	val, found, err := valueFromRawContext(json.RawMessage(`{"a":1}`), "/missing", "test")
	require.NoError(t, err)
	assert.False(t, found)
	assert.Nil(t, val)
}

type failingApprovalStore struct{}

func (failingApprovalStore) List(approvals.Filter, approvals.Page) ([]*approvals.Approval, int, error) {
	return nil, 0, nil
}
func (failingApprovalStore) Get(string) (*approvals.Approval, error) { return nil, nil }
func (failingApprovalStore) Approve(string, string) (*approvals.Approval, error) {
	return nil, nil
}
func (failingApprovalStore) Reject(string, string, string) (*approvals.Approval, error) {
	return nil, nil
}
func (failingApprovalStore) Create(*approvals.Approval) (*approvals.Approval, error) {
	return nil, context.DeadlineExceeded
}
func (failingApprovalStore) Update(*approvals.Approval) (*approvals.Approval, error) {
	return nil, nil
}

func TestCreatePageApprovalSurfacesStoreFailure(t *testing.T) {
	service, ctx := newConsoleTestService(t, "function:invoke")
	service.svcCtx.ApprovalsStore = failingApprovalStore{}

	_, err := service.createPageApproval(ctx, "g", "e",
		spec.PageFunctionBinding{ID: "b1", FunctionID: "player.query"},
		spec.BindingContractSnapshot{BindingID: "b1", FunctionID: "player.query"},
		json.RawMessage(`{}`), "", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create page approval")
}
