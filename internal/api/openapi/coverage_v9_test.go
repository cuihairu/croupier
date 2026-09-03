package openapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	dashspec "github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/model"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	dashboardservice "github.com/cuihairu/croupier/internal/service"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type failingReaderV9 struct{ err error }

func (r failingReaderV9) Read([]byte) (int, error) { return 0, r.err }

type failingMultipartFileV9 struct{}

func (failingMultipartFileV9) Read([]byte) (int, error) { return 0, errors.New("read failed") }
func (failingMultipartFileV9) ReadAt([]byte, int64) (int, error) {
	return 0, errors.New("readAt failed")
}
func (failingMultipartFileV9) Seek(int64, int) (int64, error) { return 0, errors.New("seek failed") }
func (failingMultipartFileV9) Close() error                   { return errors.New("close failed") }

func newBareGinContextV9(t *testing.T, req *http.Request) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = req
	return ctx, rec
}

func newBareRequestV9(method, target string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, target, body)
	return req.WithContext(openAPITestContext())
}

func TestOpenAPIHandlerURIBindErrorsV9(t *testing.T) {
	handler := NewHandler(setupOpenAPITestService(t))

	uriCases := []struct {
		name   string
		invoke func(*gin.Context)
	}{
		{"GetSpec", handler.GetSpec},
		{"UpdateSource", handler.UpdateSource},
		{"GetSource", handler.GetSource},
		{"SourceDiagnostics", handler.SourceDiagnostics},
		{"DeleteBinding", handler.DeleteBinding},
	}
	for _, tc := range uriCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, rec := newBareGinContextV9(t, newBareRequestV9(http.MethodGet, "/x", nil))
			tc.invoke(ctx)
			assert.Equal(t, http.StatusBadRequest, rec.Code)
		})
	}
}

func TestOpenAPIHandlerCreateSourceBodyReadErrorV9(t *testing.T) {
	handler := NewHandler(setupOpenAPITestService(t))
	req := newBareRequestV9(http.MethodPost, "/sources", failingReaderV9{err: errors.New("boom")})
	req.Header.Set("Content-Type", "application/json")

	ctx, rec := newBareGinContextV9(t, req)
	handler.CreateSource(ctx)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestOpenAPIHandlerUpdateSourceBodyReadErrorV9(t *testing.T) {
	handler := NewHandler(setupOpenAPITestService(t))
	req := newBareRequestV9(http.MethodPut, "/sources/src-1", failingReaderV9{err: errors.New("boom")})
	req.Header.Set("Content-Type", "application/json")

	ctx, rec := newBareGinContextV9(t, req)
	ctx.Params = gin.Params{{Key: "sourceId", Value: "src-1"}}
	handler.UpdateSource(ctx)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestOpenAPIHandlerCreateSourceMultipartInvalidSpecV9(t *testing.T) {
	router, _ := newCoverageOpenAPIRouter(t)

	body := &strings.Builder{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "broken.txt")
	require.NoError(t, err)
	_, err = io.Copy(part, strings.NewReader("definitely not an openapi document"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	req := httptest.NewRequest(http.MethodPost, "/sources", strings.NewReader(body.String()))
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestServiceCreateSourceFromMultipartReadErrorV9(t *testing.T) {
	service, ctx := setupOpenAPITestServiceWithPermissions(t, "openapi_sources:read", "openapi_sources:write")
	_, err := service.CreateSourceFromMultipart(ctx, "broken-upload", failingMultipartFileV9{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read failed")
}

func TestServiceSourceDiagnosticsCorruptDiagnosticsV9(t *testing.T) {
	service, ctx := setupOpenAPITestServiceWithPermissions(t, "openapi_sources:read", "openapi_sources:write")
	created, err := service.CreateSource(ctx, &OpenAPISourceCreateRequest{
		Name: "corrupt-diags",
		Spec: json.RawMessage(coverageSourceSpec(t)),
	})
	require.NoError(t, err)

	require.NoError(t, service.svcCtx.DB.Model(&model.OpenAPISource{}).
		Where("source_id = ?", created.Source.SourceID).
		Update("diagnostics_json", "{not-json").Error)

	_, err = service.SourceDiagnostics(ctx, &OpenAPISourceGetRequest{SourceID: created.Source.SourceID})
	require.Error(t, err)
}

func TestServiceDeleteBindingUnknownBindingOnExistingSourceV9(t *testing.T) {
	service, ctx := setupOpenAPITestServiceWithPermissions(t, "openapi_sources:read", "openapi_sources:write")
	created, err := service.CreateSource(ctx, &OpenAPISourceCreateRequest{
		Name: "delete-ghost",
		Spec: json.RawMessage(coverageSourceSpec(t)),
	})
	require.NoError(t, err)

	_, err = service.DeleteBinding(ctx, &OpenAPISourceBindingDeleteRequest{
		SourceID:  created.Source.SourceID,
		BindingID: "ghost-binding",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "OpenAPI source binding not found")
}

func TestReconcileSDKRestoreFailsOnPresentationSchemaV9(t *testing.T) {
	service, ctx := setupOpenAPITestServiceWithPermissions(t, "openapi_sources:read", "openapi_sources:write")
	store := service.svcCtx.RegistryStore
	store.Mu().Lock()
	store.AgentsUnsafe()["agent-bad-v9"] = &reg.AgentSession{
		AgentID:  "agent-bad-v9",
		GameID:   "demo-game",
		Env:      "development",
		LastSeen: time.Now(),
		Functions: map[string]reg.FunctionMeta{
			"bad.fn": {Enabled: true, Version: "1.0.0", InputSchema: `{"type":"object","x-layout":1}`},
		},
	}
	store.Mu().Unlock()

	err := service.reconcileContractAfterBindingDelete(ctx, "demo-game", "development",
		&model.OpenAPISourceBinding{FunctionID: "bad.fn"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "restore SDK function contract")
}

func TestReconcileRemoveContractFailureV9(t *testing.T) {
	service, ctx := setupOpenAPITestServiceWithPermissions(t, "openapi_sources:read", "openapi_sources:write")
	require.NoError(t, service.svcCtx.DB.Migrator().DropTable("function_contracts"))

	err := service.reconcileContractAfterBindingDelete(ctx, "demo-game", "development",
		&model.OpenAPISourceBinding{FunctionID: "ghost.fn"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "remove OpenAPI function contract")
}

func TestRebuildProposalsAfterReconcileFailuresV9(t *testing.T) {
	t.Run("resource capability failure", func(t *testing.T) {
		service, ctx := setupOpenAPITestServiceWithPermissions(t, "openapi_sources:write")
		require.NoError(t, service.svcCtx.DB.Migrator().DropTable("resource_capabilities"))
		err := service.rebuildProposalsAfterContractReconcile(ctx, "demo-game", "development",
			"player", "fn.v9", "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "rebuild resource capability after binding delete")
	})

	t.Run("resource proposals failure", func(t *testing.T) {
		service, ctx := setupOpenAPITestServiceWithPermissions(t, "openapi_sources:write")
		require.NoError(t, service.svcCtx.DB.Migrator().DropTable("page_proposals"))
		err := service.rebuildProposalsAfterContractReconcile(ctx, "demo-game", "development",
			"player", "fn.v9", "guild")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "rebuild page proposals after binding delete")
	})

	t.Run("standalone proposal failure", func(t *testing.T) {
		service, ctx := setupOpenAPITestServiceWithPermissions(t, "openapi_sources:write")
		require.NoError(t, dashboardservice.NewContractService(service.svcCtx.DB).RebuildContractFromFunctionMeta(
			ctx, "demo-game", "development", "sdk", dashboardservice.FunctionMetaInput{
				ID: "fn.v9", Version: "1.0.0", Enabled: true,
				Capability:   "action",
				Execution:    "sync",
				Risk:         "safe",
				InputSchema:  `{"type":"object"}`,
				OutputSchema: `{"type":"object"}`,
			}))
		require.NoError(t, service.svcCtx.DB.Migrator().DropTable("page_proposals"))
		err := service.rebuildProposalsAfterContractReconcile(ctx, "demo-game", "development",
			"", "fn.v9", "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "rebuild standalone proposal after binding delete")
	})
}

func TestRebuildContractFromRemainingBindingFailuresV9(t *testing.T) {
	t.Run("corrupted operations payload", func(t *testing.T) {
		service, ctx := setupOpenAPITestServiceWithPermissions(t, "openapi_sources:read", "openapi_sources:write")
		created, err := service.CreateSource(ctx, &OpenAPISourceCreateRequest{
			Name: "remaining-corrupt",
			Spec: json.RawMessage(coverageSourceSpec(t)),
		})
		require.NoError(t, err)
		require.NoError(t, service.svcCtx.DB.Model(&model.OpenAPISource{}).
			Where("source_id = ?", created.Source.SourceID).
			Update("operations_json", "{not-json").Error)

		err = service.rebuildContractFromRemainingBinding(ctx, "demo-game", "development",
			model.OpenAPISourceBinding{SourceID: created.Source.SourceID, OperationID: "getUser", FunctionID: "player.get"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "read remaining binding operations")
	})

	t.Run("operation missing from source", func(t *testing.T) {
		service, ctx := setupOpenAPITestServiceWithPermissions(t, "openapi_sources:read", "openapi_sources:write")
		created, err := service.CreateSource(ctx, &OpenAPISourceCreateRequest{
			Name: "remaining-ghost-op",
			Spec: json.RawMessage(coverageSourceSpec(t)),
		})
		require.NoError(t, err)

		err = service.rebuildContractFromRemainingBinding(ctx, "demo-game", "development",
			model.OpenAPISourceBinding{SourceID: created.Source.SourceID, OperationID: "ghostOp", FunctionID: "player.get"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "is missing from source")
	})
}

func TestServiceGetSourceScopeErrorV9(t *testing.T) {
	service, _ := setupOpenAPITestServiceWithPermissions(t, "openapi_sources:read")
	ctx := context.WithValue(context.Background(), "username", "openapi_tester")

	_, err := service.GetSource(ctx, &OpenAPISourceGetRequest{SourceID: "any"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "X-Game-ID is required")
}

func TestLoadSourceWithBindingsListFailureV9(t *testing.T) {
	service, ctx := setupOpenAPITestServiceWithPermissions(t, "openapi_sources:read", "openapi_sources:write")
	created, err := service.CreateSource(ctx, &OpenAPISourceCreateRequest{
		Name: "list-bindings-fail",
		Spec: json.RawMessage(coverageSourceSpec(t)),
	})
	require.NoError(t, err)
	require.NoError(t, service.svcCtx.DB.Migrator().DropTable("openapi_source_bindings"))

	_, _, err = service.loadSourceWithBindings(ctx, created.Source.SourceID)
	require.Error(t, err)
}

func TestNormalizeRawSourceYAMLNaNV9(t *testing.T) {
	_, _, err := normalizeRawSource(json.RawMessage("value: .nan"))
	require.Error(t, err)
}

func TestExtractSourceOperationsSortsAcrossPathsAndMethodsV9(t *testing.T) {
	paths := openapi3.NewPaths()
	mkOp := func(id string) *openapi3.Operation {
		return &openapi3.Operation{
			OperationID: id,
			Responses:   openapi3.NewResponses(),
		}
	}
	paths.Set("/b", &openapi3.PathItem{Get: mkOp("opB")})
	paths.Set("/a", &openapi3.PathItem{Get: mkOp("opA1"), Post: mkOp("opA2")})
	doc := &openapi3.T{Paths: paths}

	ops, _ := extractSourceOperations(doc, nil)
	require.Len(t, ops, 3)
	assert.Equal(t, "/a", ops[0].Path)
	assert.Equal(t, "GET", ops[0].Method)
	assert.Equal(t, "POST", ops[1].Method)
	assert.Equal(t, "/b", ops[2].Path)
}

func TestOpenAPIMethodOperationsSkipsNilPathItemV9(t *testing.T) {
	paths := openapi3.NewPaths()
	paths.Set("/nil", nil)
	paths.Set("/ok", &openapi3.PathItem{
		Get: &openapi3.Operation{OperationID: "ok"},
	})

	out := openAPIMethodOperations(paths)
	for _, candidate := range out {
		assert.NotEqual(t, "/nil", candidate.path, "nil path items must be skipped")
	}
	assert.Len(t, out, 8)
}

func TestRegisteredFunctionMetaSkipsNilSessionV9(t *testing.T) {
	service, _ := setupOpenAPITestServiceWithPermissions(t, "openapi_sources:read")
	store := service.svcCtx.RegistryStore
	store.Mu().Lock()
	store.AgentsUnsafe()["nil-agent-v9"] = nil
	store.Mu().Unlock()

	meta, ok := registeredFunctionMetaInScope(service.svcCtx, "demo-game", "development", "player.list")
	require.True(t, ok)
	assert.True(t, meta.Enabled)

	assert.True(t, hasRegisteredFunction(service.svcCtx, "player.list"))
}

func TestMergePathLevelParametersSkipsNilOpParamV9(t *testing.T) {
	pathParam := &openapi3.ParameterRef{Value: openapi3.NewPathParameter("id")}
	op := &openapi3.Operation{Parameters: []*openapi3.ParameterRef{nil}}
	item := &openapi3.PathItem{Parameters: []*openapi3.ParameterRef{pathParam}}

	mergePathLevelParameters(item, op)
	require.Len(t, op.Parameters, 2)
	assert.Equal(t, "path", op.Parameters[1].Value.In)
}

func TestOpenAPISchemaRefJSONMarshalFailureV9(t *testing.T) {
	ref := &openapi3.SchemaRef{Value: &openapi3.Schema{
		Extensions: map[string]interface{}{"x-boom": make(chan int)},
	}}
	assert.Empty(t, openAPISchemaRefJSON(ref))
}

func TestExtensionIntRejectsNonNumericValueV9(t *testing.T) {
	diags := []dashspec.Diagnostic{}
	got := extensionInt(map[string]interface{}{"x-timeout-ms": true}, "x-timeout-ms", "$.op", &diags)
	assert.Equal(t, 0, got)
	require.Len(t, diags, 1)
	assert.Equal(t, "openapi_timeout_ms_invalid", diags[0].Code)
}

func TestMustMarshalRawFailureV9(t *testing.T) {
	assert.Nil(t, mustMarshalRaw(make(chan int)))
}

func TestNormalizeOpenAPIDocSkipsNilEntriesV9(t *testing.T) {
	responses := openapi3.NewResponses()
	responses.Set("204", nil)
	paths := openapi3.NewPaths()
	paths.Set("/nil", nil)
	paths.Set("/ok", &openapi3.PathItem{
		Get: &openapi3.Operation{OperationID: "ok", Responses: responses},
	})
	doc := &openapi3.T{Paths: paths}

	require.NotPanics(t, func() { normalizeOpenAPIDoc(doc) })
}

func TestOpenAPIScopelessServiceContextGuardsV9(t *testing.T) {
	service := NewService(&svc.ServiceContext{})
	ctx := context.WithValue(context.Background(), "username", "openapi_tester")
	_, _, err := service.loadSourceWithBindings(ctx, "any")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "X-Game-ID is required")
}
