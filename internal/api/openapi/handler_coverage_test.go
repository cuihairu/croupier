package openapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/audit"
	dashspec "github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/model"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/cuihairu/croupier/internal/telemetry"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// ---------------------------------------------------------------------------
// Handler-level tests through a full router
// ---------------------------------------------------------------------------

func newCoverageOpenAPIRouter(t *testing.T) (*gin.Engine, *Service) {
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
	router.PUT("/sources/:sourceId", handler.UpdateSource)
	router.GET("/sources/:sourceId", handler.GetSource)
	router.POST("/sources/:sourceId/bindings", handler.CreateBinding)
	router.DELETE("/sources/:sourceId/bindings/:bindingId", handler.DeleteBinding)
	return router, service
}

func doCoverageRequest(t *testing.T, router *gin.Engine, method, path, body, contentType string) *httptest.ResponseRecorder {
	t.Helper()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func coverageSourceSpec(t *testing.T) string {
	t.Helper()
	spec := map[string]interface{}{
		"openapi": "3.0.3",
		"info": map[string]interface{}{
			"title":   "Coverage API",
			"version": "1.0.0",
		},
		"paths": map[string]interface{}{
			"/users/{id}": map[string]interface{}{
				"parameters": []map[string]interface{}{
					{"name": "id", "in": "path", "required": true, "schema": map[string]interface{}{"type": "string"}},
				},
				"get": map[string]interface{}{
					"operationId": "getUser",
					"summary":     "Get one user",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "OK"},
					},
				},
			},
		},
	}
	raw, err := json.Marshal(spec)
	require.NoError(t, err)
	return string(raw)
}

func TestOpenAPIHandlerRoutesEndToEnd(t *testing.T) {
	router, _ := newCoverageOpenAPIRouter(t)

	// CreateSource (raw spec body without envelope).
	rec := doCoverageRequest(t, router, http.MethodPost, "/sources", coverageSourceSpec(t), "application/json")
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var created struct {
		Source struct {
			SourceID string `json:"sourceId"`
		} `json:"source"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))
	require.NotEmpty(t, created.Source.SourceID)

	// CreateSource with a JSON envelope {"name":..., "spec":...}.
	envelope := `{"name":"envelope-source","spec":` + coverageSourceSpec(t) + `}`
	rec = doCoverageRequest(t, router, http.MethodPost, "/sources", envelope, "application/json")
	assert.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	// ListSources returns both sources.
	rec = doCoverageRequest(t, router, http.MethodGet, "/sources", "", "")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "items")

	// GetSource + diagnostics.
	rec = doCoverageRequest(t, router, http.MethodGet, "/sources/"+created.Source.SourceID, "", "")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "getUser")

	// UpdateSource with envelope body refreshes the source.
	updateBody := `{"name":"renamed","spec":` + coverageSourceSpec(t) + `}`
	rec = doCoverageRequest(t, router, http.MethodPut, "/sources/"+created.Source.SourceID, updateBody, "application/json")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	// GetDocument aggregates the registry document.
	rec = doCoverageRequest(t, router, http.MethodGet, "/document", "", "")
	assert.Equal(t, http.StatusOK, rec.Code)

	// BatchGetSpec with an unknown id falls back to a stub operation.
	rec = doCoverageRequest(t, router, http.MethodPost, "/batch/spec", `{"functionIds":["player.list",""]}`, "application/json")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "player.list")

	// CreateBinding then DeleteBinding round-trip.
	bindBody := `{"bindingId":"b-coverage","operationId":"getUser","kind":"provider","functionId":"player.get"}`
	rec = doCoverageRequest(t, router, http.MethodPost,
		"/sources/"+created.Source.SourceID+"/bindings", bindBody, "application/json")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	rec = doCoverageRequest(t, router, http.MethodDelete,
		"/sources/"+created.Source.SourceID+"/bindings/b-coverage", "", "")
	assert.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}

func TestOpenAPIHandlerErrorBranches(t *testing.T) {
	router, _ := newCoverageOpenAPIRouter(t)

	// Unknown function spec → error response.
	rec := doCoverageRequest(t, router, http.MethodGet, "/spec/nope.missing", "", "")
	assert.NotEqual(t, http.StatusOK, rec.Code)

	// Malformed batch payload.
	rec = doCoverageRequest(t, router, http.MethodPost, "/batch/spec", `{`, "application/json")
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	// Oversized create body exceeds the 2 MiB limit.
	big := `{"spec":{"pad":"` + strings.Repeat("a", (2<<20)+16) + `"}}`
	rec = doCoverageRequest(t, router, http.MethodPost, "/sources", big, "application/json")
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "2 MiB")

	// Oversized update body as well.
	rec = doCoverageRequest(t, router, http.MethodPut, "/sources/some-id", big, "application/json")
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	// Invalid JSON on update.
	rec = doCoverageRequest(t, router, http.MethodPut, "/sources/some-id", "{nope", "application/json")
	assert.NotEqual(t, http.StatusOK, rec.Code)

	// Missing binding fields fail validation.
	rec = doCoverageRequest(t, router, http.MethodPost, "/sources/src/bindings", `{"kind":""}`, "application/json")
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	// Deleting an unknown binding surfaces the service error.
	rec = doCoverageRequest(t, router, http.MethodDelete, "/sources/ghost/bindings/none", "", "")
	assert.NotEqual(t, http.StatusOK, rec.Code)
}

func TestOpenAPIHandlerCreateSourceMultipart(t *testing.T) {
	router, service := newCoverageOpenAPIRouter(t)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "spec.yaml")
	require.NoError(t, err)
	yamlSpec := "openapi: 3.0.3\ninfo:\n  title: YAML API\n  version: 1.0.0\npaths:\n  /ping:\n    get:\n      operationId: ping\n      responses:\n        '200':\n          description: ok\n"
	_, err = io.Copy(part, strings.NewReader(yamlSpec))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	req := httptest.NewRequest(http.MethodPost, "/sources", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	sources, listErr := service.ListSources(openAPITestContext(), &OpenAPISourceListRequest{})
	require.NoError(t, listErr)
	found := false
	for _, item := range sources.Items {
		if item.Name == "spec.yaml" {
			found = true
		}
	}
	assert.True(t, found, "expected multipart upload named after the file")

	// Multipart request without a file part fails cleanly.
	emptyBody := &bytes.Buffer{}
	emptyWriter := multipart.NewWriter(emptyBody)
	require.NoError(t, emptyWriter.Close())
	req = httptest.NewRequest(http.MethodPost, "/sources", emptyBody)
	req.Header.Set("Content-Type", emptyWriter.FormDataContentType())
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	assert.NotEqual(t, http.StatusCreated, rec.Code)
}

// ---------------------------------------------------------------------------
// Pure helper branches
// ---------------------------------------------------------------------------

func TestMergePathLevelParametersAppendsSharedParams(t *testing.T) {
	shared := &openapi3.ParameterRef{Value: openapi3.NewPathParameter("id")}
	duplicate := &openapi3.ParameterRef{Value: openapi3.NewQueryParameter("id")}

	op := &openapi3.Operation{
		Parameters: []*openapi3.ParameterRef{duplicate},
	}
	pathItem := &openapi3.PathItem{
		Parameters: []*openapi3.ParameterRef{shared, nil},
	}
	mergePathLevelParameters(pathItem, op)
	// The path parameter is appended once; the query duplicate is skipped.
	require.Len(t, op.Parameters, 2)
	assert.Equal(t, "path", op.Parameters[1].Value.In)

	// Nil guards.
	mergePathLevelParameters(nil, op)
	mergePathLevelParameters(pathItem, nil)
	assert.Len(t, op.Parameters, 2)

	// A path-item parameter whose Value is nil must not panic.
	op2 := &openapi3.Operation{}
	item2 := &openapi3.PathItem{Parameters: []*openapi3.ParameterRef{nil}}
	mergePathLevelParameters(item2, op2)
	assert.Empty(t, op2.Parameters)
}

func TestOpenAPIRequestSchemaMergesParamsAndBody(t *testing.T) {
	stringSchema := &openapi3.SchemaRef{Value: openapi3.NewStringSchema()}

	op := &openapi3.Operation{
		Parameters: []*openapi3.ParameterRef{
			{Value: func() *openapi3.Parameter {
				p := openapi3.NewPathParameter("id")
				p.Required = true
				p.Schema = stringSchema
				return p
			}()},
			{Value: func() *openapi3.Parameter {
				p := openapi3.NewQueryParameter("verbose")
				p.Schema = nil
				return p
			}()},
			nil,
			{Value: func() *openapi3.Parameter {
				p := openapi3.NewQueryParameter("")
				p.Schema = stringSchema
				return p
			}()},
		},
	}
	raw := openAPIRequestSchema(op)
	var parsed map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(raw), &parsed))
	assert.Equal(t, "object", parsed["type"])
	props, ok := parsed["properties"].(map[string]interface{})
	require.True(t, ok)
	assert.Contains(t, props, "id")
	assert.Contains(t, props, "verbose")

	// With a JSON request body the body properties merge into the schema.
	boolSchema := &openapi3.SchemaRef{Value: openapi3.NewBoolSchema()}
	op.RequestBody = &openapi3.RequestBodyRef{Value: &openapi3.RequestBody{
		Content: openapi3.Content{
			"application/json": {Schema: &openapi3.SchemaRef{Value: &openapi3.Schema{
				Type:       &openapi3.Types{"object"},
				Properties: openapi3.Schemas{"force": boolSchema},
			}}},
		},
	}}
	merged := openAPIRequestSchema(op)
	var mergedParsed map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(merged), &mergedParsed))
	mergedProps, ok := mergedParsed["properties"].(map[string]interface{})
	require.True(t, ok)
	assert.Contains(t, mergedProps, "force")

	// Parameters only, no body: still object schema (already covered above).
	// Body only: returns the raw body schema.
	bodyOnly := &openapi3.Operation{RequestBody: op.RequestBody}
	assert.Contains(t, openAPIRequestSchema(bodyOnly), `"type"`)

	// Schema-less media types fall back to other content entries.
	contentOnlyText := openapi3.Content{
		"text/plain": {Schema: stringSchema},
	}
	assert.NotEmpty(t, openAPIMediaSchemaJSON(contentOnlyText))

	// JSON entry without a schema defers to the fallback loop.
	jsonNoSchema := openapi3.Content{
		"application/json": {},
		"text/html":        {Schema: stringSchema},
	}
	assert.NotEmpty(t, openAPIMediaSchemaJSON(jsonNoSchema))
}

func TestMustMarshalRawAndFormatTime(t *testing.T) {
	assert.Nil(t, mustMarshalRaw(nil))
	assert.JSONEq(t, `{"a":1}`, string(mustMarshalRaw(map[string]int{"a": 1})))
	assert.Equal(t, "", formatTime(time.Time{}))
	now := time.Now()
	assert.Equal(t, now.Format(time.RFC3339), formatTime(now))
}

func TestResourceKeyFromContractPrefersContractValue(t *testing.T) {
	assert.Equal(t, "fallback", resourceKeyFromContract(nil, "fallback"))
	assert.Equal(t, "", resourceKeyFromContract(nil, "  "))
	contract := &model.FunctionContract{ResourceKey: " player "}
	assert.Equal(t, "player", resourceKeyFromContract(contract, "fallback"))
	assert.Equal(t, "fallback", resourceKeyFromContract(&model.FunctionContract{}, "fallback"))
}

func TestOperationPresentationFieldDetectsForbiddenKeys(t *testing.T) {
	key, found := operationPresentationField(nil)
	assert.False(t, found)
	assert.Empty(t, key)

	clean := &openapi3.Operation{Extensions: map[string]interface{}{"x-custom": true}}
	_, found = operationPresentationField(clean)
	assert.False(t, found)

	forbidden := &openapi3.Operation{Extensions: map[string]interface{}{"formily": map[string]interface{}{}}}
	got, found := operationPresentationField(forbidden)
	assert.True(t, found)
	assert.NotEmpty(t, got)
}

func TestRequireScopeMissingEnvOpenAPI(t *testing.T) {
	gameID, env, err := requireScope(svc.WithGameScope(context.Background(), svc.GameScope{GameID: "g"}))
	assert.Empty(t, gameID)
	assert.Empty(t, env)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "X-Env is required")
}

// ---------------------------------------------------------------------------
// Contract rebuild helpers
// ---------------------------------------------------------------------------

func TestRebuildContractsForSourceBindingsSkipsNonProviderAndUnknownOps(t *testing.T) {
	service := setupOpenAPITestService(t)
	ctx := openAPITestContext()

	err := service.rebuildContractsForSourceBindings(ctx, "demo-game", "development", nil,
		[]OpenAPISourceOperation{}, []model.OpenAPISourceBinding{
			{Kind: "consumer", FunctionID: "player.get", OperationID: "getUser"},
			{Kind: "provider", FunctionID: "player.get", OperationID: "missing-op"},
		})
	require.NoError(t, err)

	// Provider binding for an unregistered runtime function errors out.
	sourceModel := service.svcCtx.OpenAPISourceModel
	source := &model.OpenAPISource{SourceID: "src-x"}
	err = service.rebuildContractsForSourceBindings(ctx, "demo-game", "development", source,
		[]OpenAPISourceOperation{{OperationID: "getUser"}},
		[]model.OpenAPISourceBinding{{Kind: "provider", FunctionID: "not.registered", OperationID: "getUser"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not registered in current game/env runtime")

	_ = sourceModel
}

func TestRebuildContractForSourceBindingGuards(t *testing.T) {
	service := setupOpenAPITestService(t)
	ctx := openAPITestContext()

	// Nil DB guard.
	nilSvc := NewService(&svc.ServiceContext{})
	err := nilSvc.rebuildContractForSourceBinding(ctx, "g", "e", nil,
		OpenAPISourceOperation{OperationID: "op"}, &model.OpenAPISourceBinding{FunctionID: "f"}, runtimeMetaStub())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")

	// Empty function id.
	err = service.rebuildContractForSourceBinding(ctx, "g", "e", nil,
		OpenAPISourceOperation{}, &model.OpenAPISourceBinding{FunctionID: "  "}, runtimeMetaStub())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "functionId is required")

	// Happy path with empty resource key falls back to standalone proposal.
	dbSvcCtx := service.svcCtx
	require.NoError(t, dbSvcCtx.DB.Where("1 = 1").Delete(&model.FunctionContract{}).Error)
	meta := runtimeMetaStub()
	meta.Resource = ""
	err = service.rebuildContractForSourceBinding(ctx, "demo-game", "development",
		nil, OpenAPISourceOperation{OperationID: "op"}, &model.OpenAPISourceBinding{FunctionID: "player.get"}, meta)
	require.NoError(t, err)
}

func runtimeMetaStub() reg.FunctionMeta {
	return reg.FunctionMeta{
		Version:     "1.0.0",
		Enabled:     true,
		Resource:    "",
		Capability:  "item_query",
		Execution:   "sync",
		InputSchema: `{"type":"object"}`,
	}
}

// ---------------------------------------------------------------------------
// Audit + telemetry helpers
// ---------------------------------------------------------------------------

func TestAuditSourceEventDefaults(t *testing.T) {
	service, _, auditStore := setupOpenAPITestServiceWithAudit(t, "openapi_sources:read")

	// Anonymous actor and nil details fall back to safe defaults.
	service.auditSourceEvent(context.Background(), audit.EventOpenAPISourceCreate,
		"g", "e", "src-1", "Src", nil)

	records, total, err := auditStore.List(audit.AuditFilter{
		EventType: []audit.AuditEventType{audit.EventOpenAPISourceCreate},
	}, audit.AuditPage{PageSize: 10})
	require.NoError(t, err)
	assert.Positive(t, total)
	require.NotEmpty(t, records)
	assert.Equal(t, "unknown", records[len(records)-1].Actor.ID)
	assert.Equal(t, "src-1", records[len(records)-1].Resource.ID)
}

func TestStartSourceSpanWithoutTelemetryIsNoop(t *testing.T) {
	service := NewService(&svc.ServiceContext{})
	nextCtx, finish := service.startSourceSpan(context.Background(), "test.span", "g", "e")
	finish(nil)
	require.NotNil(t, nextCtx)
}

// ---------------------------------------------------------------------------
// Misc helpers used by service flows
// ---------------------------------------------------------------------------

func TestFirstNonEmptyTrimsValues(t *testing.T) {
	assert.Equal(t, "a", firstNonEmpty("", "  ", "a"))
	assert.Equal(t, "", firstNonEmpty(" ", ""))
}

// ---------------------------------------------------------------------------
// Contract reconciliation branches
// ---------------------------------------------------------------------------

func TestReconcileContractAfterBindingDeleteBranches(t *testing.T) {
	service := setupOpenAPITestService(t)
	ctx := openAPITestContext()

	// Nil removed binding and empty function id are no-ops.
	require.NoError(t, service.reconcileContractAfterBindingDelete(ctx, "g", "e", nil))
	require.NoError(t, service.reconcileContractAfterBindingDelete(ctx, "g", "e",
		&model.OpenAPISourceBinding{FunctionID: "  "}))

	// Remaining binding pointing to a missing source surfaces the lookup error.
	require.NoError(t, service.svcCtx.OpenAPISourceBindingModel.Upsert(ctx, &model.OpenAPISourceBinding{
		GameID:      "demo-game",
		Env:         "development",
		SourceID:    "ghost-source",
		BindingID:   "b-ghost",
		OperationID: "getUser",
		FunctionID:  "player.get",
		Kind:        "provider",
	}))
	err := service.reconcileContractAfterBindingDelete(ctx, "demo-game", "development",
		&model.OpenAPISourceBinding{FunctionID: "player.get", SourceID: "gone", OperationID: "getUser"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "find remaining binding source")

	// No remaining bindings + registered runtime meta → SDK fallback restore.
	err = service.reconcileContractAfterBindingDelete(ctx, "demo-game", "development",
		&model.OpenAPISourceBinding{FunctionID: "player.list"})
	require.NoError(t, err)

	// No remaining bindings + unregistered function removes the contract.
	err = service.reconcileContractAfterBindingDelete(ctx, "demo-game", "development",
		&model.OpenAPISourceBinding{FunctionID: "ghost.function"})
	require.NoError(t, err)
}

func TestRegisteredFunctionMetaInScopeFallbacks(t *testing.T) {
	service, _ := setupOpenAPITestServiceWithPermissions(t, "openapi_sources:read")
	svcCtx := service.svcCtx

	// Registry hit.
	meta, ok := registeredFunctionMetaInScope(svcCtx, "demo-game", "development", "player.get")
	require.True(t, ok)
	assert.Equal(t, "1.0.0", meta.Version)

	// Wrong env scope misses.
	_, ok = registeredFunctionMetaInScope(svcCtx, "demo-game", "production", "player.get")
	assert.False(t, ok)

	// Empty inputs miss.
	_, ok = registeredFunctionMetaInScope(svcCtx, "", "", "")
	assert.False(t, ok)

	// FunctionModel fallback for functions known only through the database.
	fn := &model.Function{
		FunctionID: "db.only.fn",
		GameID:     "demo-game",
		Name:       "DB Function",
		Version:    "2.0.0",
		Status:     1,
		Resource:   "player",
	}
	require.NoError(t, svcCtx.DB.Create(fn).Error)
	meta, ok = registeredFunctionMetaInScope(svcCtx, "demo-game", "development", "db.only.fn")
	require.True(t, ok)
	assert.Equal(t, "2.0.0", meta.Version)
	assert.Equal(t, "player", meta.Resource)

	// Game mismatch in DB row rejects the lookup.
	_, ok = registeredFunctionMetaInScope(svcCtx, "other-game", "development", "db.only.fn")
	assert.False(t, ok)
}

// ---------------------------------------------------------------------------
// Raw source normalization and diagnostics
// ---------------------------------------------------------------------------

func TestNormalizeRawSourceVariants(t *testing.T) {
	// Empty spec.
	_, _, err := normalizeRawSource(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "spec is required")

	// YAML input converts to JSON.
	data, format, err := normalizeRawSource(json.RawMessage("openapi: \"3.0.3\"\ninfo:\n  title: t\n"))
	require.NoError(t, err)
	assert.Equal(t, "yaml", format)
	assert.True(t, json.Valid(data))

	// Garbage that is neither JSON nor YAML fails.
	_, _, err = normalizeRawSource(json.RawMessage("\t\x00invalid"))
	require.Error(t, err)

	// Nested map[interface{}]interface{} values are normalized recursively.
	normalized := normalizeYAMLValue(map[interface{}]interface{}{
		"a": map[interface{}]interface{}{"b": []interface{}{1, "x"}},
	})
	asMap, ok := normalized.(map[string]interface{})
	require.True(t, ok)
	inner, ok := asMap["a"].(map[string]interface{})
	require.True(t, ok)
	list, ok := inner["b"].([]interface{})
	require.True(t, ok)
	assert.Len(t, list, 2)
}

func newDocWithOps(ops ...*openapi3.Operation) *openapi3.T {
	doc := &openapi3.T{OpenAPI: "3.0.3"}
	item := &openapi3.PathItem{}
	for i, op := range ops {
		switch i % 6 {
		case 0:
			item.Get = op
		case 1:
			item.Post = op
		case 2:
			item.Put = op
		case 3:
			item.Delete = op
		default:
			item.Patch = op
		}
	}
	doc.Paths = &openapi3.Paths{}
	doc.Paths.Set("/a", item)
	return doc
}

func diagCodes(diags []dashspec.Diagnostic) map[string]bool {
	codes := map[string]bool{}
	for _, d := range diags {
		codes[d.Code] = true
	}
	return codes
}

func TestExtractSourceOperationsDiagnostics(t *testing.T) {
	responses := openapi3.NewResponses()

	// Duplicate operation ids are reported.
	dup := &openapi3.Operation{OperationID: "dup", Responses: responses}
	items, diags := extractSourceOperations(newDocWithOps(dup,
		&openapi3.Operation{OperationID: "dup", Responses: responses}), nil)
	// First occurrence is kept; the duplicate is reported and skipped.
	assert.Len(t, items, 1)
	assert.True(t, diagCodes(diags)["openapi_operation_id_duplicate"])

	// Missing operation id is reported.
	items, diags = extractSourceOperations(newDocWithOps(
		&openapi3.Operation{Responses: responses}), nil)
	assert.Empty(t, items)
	assert.True(t, diagCodes(diags)["openapi_operation_id_missing"])

	// Forbidden presentation fields abort the operation.
	forbidden := &openapi3.Operation{
		OperationID: "styled",
		Responses:   responses,
		Extensions:  map[string]interface{}{"formily": map[string]interface{}{}},
	}
	items, diags = extractSourceOperations(newDocWithOps(forbidden), nil)
	assert.Empty(t, items)
	assert.True(t, diagCodes(diags)["openapi_presentation_field_forbidden"])

	// Nil op entries are skipped without diagnostics.
	empty := &openapi3.Operation{OperationID: "", Responses: responses}
	_ = empty
	doc := &openapi3.T{OpenAPI: "3.0.3", Info: &openapi3.Info{Title: "t", Version: "v"}}
	item := &openapi3.PathItem{Get: nil, Post: &openapi3.Operation{OperationID: "real", Responses: responses}}
	doc.Paths = &openapi3.Paths{}
	doc.Paths.Set("/ok", item)
	items, diags = extractSourceOperations(doc, nil)
	require.Len(t, items, 1)
	assert.Equal(t, "real", items[0].OperationID)

	// Missing paths block yields a dedicated diagnostic.
	_, diags = extractSourceOperations(&openapi3.T{}, nil)
	assert.True(t, diagCodes(diags)["openapi_paths_missing"])
}

func TestParseOpenAPISourceDiagnostics(t *testing.T) {
	noIDSpec := `{"openapi":"3.0.3","info":{"title":"d","version":"1"},"paths":{"/a":{"get":{"responses":{"200":{"description":"ok"}}}}}}`
	parsed, err := parseOpenAPISource([]byte(noIDSpec))
	require.NoError(t, err)
	assert.Empty(t, parsed.Operations)
	hasMissing := false
	for _, d := range parsed.Diagnostics {
		if d.Code == "openapi_operation_id_missing" {
			hasMissing = true
		}
	}
	assert.True(t, hasMissing)

	presentationSpec := `{"openapi":"3.0.3","info":{"title":"d","version":"1"},"paths":{"/a":{"get":{"operationId":"ok","formily":{},"responses":{"200":{"description":"ok"}}}}}}`
	parsed, err = parseOpenAPISource([]byte(presentationSpec))
	require.NoError(t, err)
	hasForbidden := false
	for _, d := range parsed.Diagnostics {
		if d.Code == "openapi_presentation_field_forbidden" {
			hasForbidden = true
		}
	}
	assert.True(t, hasForbidden)

	// Documents failing kin-openapi validation surface a parse/validation diagnostic.
	badSpec := `{"openapi":"3.0.3","info":{"title":"d","version":"1"}}`
	parsed, err = parseOpenAPISource([]byte(badSpec))
	require.NoError(t, err)
	hasValidationFailure := false
	for _, d := range parsed.Diagnostics {
		if d.Code == "openapi_validation_failed" {
			hasValidationFailure = true
		}
	}
	assert.True(t, hasValidationFailure)
}

// ---------------------------------------------------------------------------
// Telemetry-backed span helper
// ---------------------------------------------------------------------------

func TestStartSourceSpanEmitsSpanWithTelemetry(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service, ctx, _ := setupOpenAPITestServiceWithAudit(t, "openapi_sources:read")

	spanRecorder := tracetest.NewSpanRecorder()
	previousProvider := otel.GetTracerProvider()
	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithSpanProcessor(spanRecorder),
	)
	otel.SetTracerProvider(tracerProvider)
	t.Cleanup(func() {
		_ = tracerProvider.Shutdown(context.Background())
		otel.SetTracerProvider(previousProvider)
	})

	telemetryService, err := telemetry.NewGameTelemetryService(telemetry.TelemetryConfig{
		ServiceName:    "openapi-test",
		ServiceVersion: "test",
		Environment:    "test",
		GameID:         "demo-game",
		EnableTracing:  false,
		EnableMetrics:  false,
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	require.NoError(t, err)
	service.svcCtx.Telemetry = telemetryService
	t.Cleanup(func() { _ = telemetryService.Shutdown(context.Background()) })

	nextCtx, finish := service.startSourceSpan(ctx, "openapi.coverage.span", "demo-game", "development",
		attribute.String("source.id", "src-1"))
	require.NotNil(t, nextCtx)
	finish(nil, attribute.String("result", "ok"))

	require.Eventually(t, func() bool {
		return len(spanRecorder.Ended()) > 0
	}, 2*time.Second, 10*time.Millisecond)
	var found bool
	for _, span := range spanRecorder.Ended() {
		if span.Name() == "openapi.coverage.span" {
			found = true
		}
	}
	assert.True(t, found)
}

// ---------------------------------------------------------------------------
// Persistence-failure branches via dropped tables
// ---------------------------------------------------------------------------

func TestOpenAPISourceModelFailureBranches(t *testing.T) {
	service, ctx := setupOpenAPITestServiceWithPermissions(t, "openapi_sources:read", "openapi_sources:write")
	require.NoError(t, service.svcCtx.DB.Migrator().DropTable("openapi_sources"))

	// CreateSource reaches the model layer and surfaces the SQL error.
	_, err := service.CreateSource(ctx, &OpenAPISourceCreateRequest{
		Name: "x",
		Spec: json.RawMessage(coverageSourceSpec(t)),
	})
	require.Error(t, err)

	_, err = service.ListSources(ctx, &OpenAPISourceListRequest{})
	require.Error(t, err)

	_, err = service.GetSource(ctx, &OpenAPISourceGetRequest{SourceID: "any"})
	require.Error(t, err)

	_, err = service.SourceDiagnostics(ctx, &OpenAPISourceGetRequest{SourceID: "any"})
	require.Error(t, err)

	_, err = service.UpdateSource(ctx, &OpenAPISourceUpdateRequest{
		SourceID: "any",
		Spec:     json.RawMessage(coverageSourceSpec(t)),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestOpenAPIBindingModelFailureBranches(t *testing.T) {
	service, ctx := setupOpenAPITestServiceWithPermissions(t, "openapi_sources:read", "openapi_sources:write")
	sourceResp, err := service.CreateSource(ctx, &OpenAPISourceCreateRequest{
		Name: "with-bindings",
		Spec: json.RawMessage(coverageSourceSpec(t)),
	})
	require.NoError(t, err)
	sourceID := sourceResp.Source.SourceID

	require.NoError(t, service.svcCtx.DB.Migrator().DropTable("openapi_source_bindings"))

	// UpdateSource fails when listing existing bindings breaks.
	_, err = service.UpdateSource(ctx, &OpenAPISourceUpdateRequest{
		SourceID: sourceID,
		Spec:     json.RawMessage(coverageSourceSpec(t)),
	})
	require.Error(t, err)

	// CreateBinding fails while persisting the binding row.
	_, err = service.CreateBinding(ctx, &OpenAPISourceBindingCreateRequest{
		SourceID:    sourceID,
		BindingID:   "b1",
		OperationID: "getUser",
		Kind:        "provider",
		FunctionID:  "player.get",
	})
	require.Error(t, err)

	// DeleteBinding fails while listing bindings for reconciliation.
	_, err = service.DeleteBinding(ctx, &OpenAPISourceBindingDeleteRequest{
		SourceID:  sourceID,
		BindingID: "b-missing",
	})
	require.Error(t, err)
}

func TestScopedTransactionRequiresDatabase(t *testing.T) {
	service := NewService(&svc.ServiceContext{})
	err := service.scopedTransaction(context.Background(), func(context.Context) error { return nil })
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestHasRegisteredFunctionFallbacks(t *testing.T) {
	assert.False(t, hasRegisteredFunction(nil, "x"))
	service, _ := setupOpenAPITestServiceWithPermissions(t, "openapi_sources:read")
	svcCtx := service.svcCtx
	assert.False(t, hasRegisteredFunction(svcCtx, ""))
	assert.True(t, hasRegisteredFunction(svcCtx, "player.list"))
	assert.False(t, hasRegisteredFunction(svcCtx, "missing.fn"))

	fn := &model.Function{FunctionID: "db.fn", GameID: "demo-game", Status: 1}
	require.NoError(t, svcCtx.DB.Create(fn).Error)
	assert.True(t, hasRegisteredFunction(svcCtx, "db.fn"))
}

func TestNormalizeOpenAPIDocPatchesResponses(t *testing.T) {
	normalizeOpenAPIDoc(nil)

	responses := openapi3.NewResponses()
	responses.Set("200", &openapi3.ResponseRef{})
	responses.Set("404", &openapi3.ResponseRef{Value: &openapi3.Response{}})
	doc := &openapi3.T{}
	item := &openapi3.PathItem{
		Get: &openapi3.Operation{
			OperationID: "patched",
			Responses:   responses,
		},
	}
	doc.Paths = &openapi3.Paths{}
	doc.Paths.Set("/p", item)
	normalizeOpenAPIDoc(doc)

	ref := doc.Paths.Map()["/p"].Get.Responses.Value("200")
	require.NotNil(t, ref.Value)
	require.NotNil(t, ref.Value.Description)
	assert.Contains(t, *ref.Value.Description, "200")

	blank := "   "
	blankResponses := openapi3.NewResponses()
	blankResponses.Set("500", &openapi3.ResponseRef{Value: &openapi3.Response{Description: &blank}})
	doc2 := &openapi3.T{}
	doc2.Paths = &openapi3.Paths{}
	doc2.Paths.Set("/q", &openapi3.PathItem{
		Get: &openapi3.Operation{
			OperationID: "blank-desc",
			Responses:   blankResponses,
		},
	})
	normalizeOpenAPIDoc(doc2)
	// A whitespace-only description counts as missing and gets patched.
	ref2 := doc2.Paths.Map()["/q"].Get.Responses.Value("500")
	assert.Contains(t, *ref2.Value.Description, "500")
}

func TestNormalizeRawSourceOversizeLimit(t *testing.T) {
	huge := json.RawMessage(`{"pad":"` + strings.Repeat("a", (2<<20)+16) + `"}`)
	_, _, err := normalizeRawSource(huge)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "2 MiB")
}

// ---------------------------------------------------------------------------
// Handler service-error branches through a broken backend
// ---------------------------------------------------------------------------

func TestOpenAPIHandlersSurfaceServiceErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := setupOpenAPITestService(t)
	require.NoError(t, service.svcCtx.DB.Migrator().DropTable("openapi_sources", "openapi_source_bindings"))

	handler := NewHandler(service)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Request = c.Request.WithContext(openAPITestContext())
		c.Next()
	})
	router.GET("/sources", handler.ListSources)
	router.POST("/sources", handler.CreateSource)
	router.PUT("/sources/:sourceId", handler.UpdateSource)
	router.GET("/sources/:sourceId", handler.GetSource)
	router.GET("/spec/:id", handler.GetSpec)
	router.POST("/sources/:sourceId/bindings", handler.CreateBinding)
	router.DELETE("/sources/:sourceId/bindings/:bindingId", handler.DeleteBinding)

	rec := doCoverageRequest(t, router, http.MethodGet, "/sources", "", "")
	assert.Equal(t, http.StatusInternalServerError, rec.Code)

	rec = doCoverageRequest(t, router, http.MethodPost, "/sources", coverageSourceSpec(t), "application/json")
	assert.Equal(t, http.StatusInternalServerError, rec.Code)

	rec = doCoverageRequest(t, router, http.MethodPut, "/sources/src-1",
		coverageSourceSpec(t), "application/json")
	assert.NotEqual(t, http.StatusOK, rec.Code)

	rec = doCoverageRequest(t, router, http.MethodGet, "/sources/src-1", "", "")
	assert.Equal(t, http.StatusNotFound, rec.Code)

	// CreateBinding fails before persistence because the source is gone.
	bindBody := `{"operationId":"getUser","kind":"provider","functionId":"player.get"}`
	rec = doCoverageRequest(t, router, http.MethodPost, "/sources/src-1/bindings", bindBody, "application/json")
	assert.Equal(t, http.StatusNotFound, rec.Code)

	rec = doCoverageRequest(t, router, http.MethodDelete, "/sources/src-1/bindings/b1", "", "")
	assert.NotEqual(t, http.StatusOK, rec.Code)
}

func TestRebuildProposalsAfterContractReconcileBranches(t *testing.T) {
	service := setupOpenAPITestService(t)
	ctx := openAPITestContext()

	// Distinct previous/current resources rebuild both proposals.
	err := service.rebuildProposalsAfterContractReconcile(ctx, "demo-game", "development",
		"player", "player.get", "guild")
	require.NoError(t, err)

	// Empty current resource falls back to the standalone function proposal.
	err = service.rebuildProposalsAfterContractReconcile(ctx, "demo-game", "development",
		"", "player.get", "")
	require.NoError(t, err)

	// Blank resources with blank function id are a no-op loop.
	err = service.rebuildProposalsAfterContractReconcile(ctx, "demo-game", "development", "", "", "")
	require.NoError(t, err)
}

func TestReconcileContractAfterDeleteWithRealSource(t *testing.T) {
	service, ctx := setupOpenAPITestServiceWithPermissions(t, "openapi_sources:read", "openapi_sources:write")

	sourceResp, err := service.CreateSource(ctx, &OpenAPISourceCreateRequest{
		Name: "reconcile-source",
		Spec: json.RawMessage(coverageSourceSpec(t)),
	})
	require.NoError(t, err)

	_, err = service.CreateBinding(ctx, &OpenAPISourceBindingCreateRequest{
		SourceID:    sourceResp.Source.SourceID,
		BindingID:   "b-reconcile",
		OperationID: "getUser",
		Kind:        "provider",
		FunctionID:  "player.get",
	})
	require.NoError(t, err)

	// Deleting the binding reconciles against the surviving source snapshot.
	resp, err := service.DeleteBinding(ctx, &OpenAPISourceBindingDeleteRequest{
		SourceID:  sourceResp.Source.SourceID,
		BindingID: "b-reconcile",
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
}

// ---------------------------------------------------------------------------
// Permission guards and remaining helper branches
// ---------------------------------------------------------------------------

func TestOpenAPIServicePermissionGuards(t *testing.T) {
	service, ctx := setupOpenAPITestServiceWithPermissions(t)

	_, err := service.ListSources(ctx, &OpenAPISourceListRequest{})
	require.Error(t, err)
	_, err = service.GetSource(ctx, &OpenAPISourceGetRequest{SourceID: "s"})
	require.Error(t, err)
	_, err = service.SourceDiagnostics(ctx, &OpenAPISourceGetRequest{SourceID: "s"})
	require.Error(t, err)
	_, err = service.CreateSource(ctx, &OpenAPISourceCreateRequest{Spec: json.RawMessage(`{}`)})
	require.Error(t, err)
	_, err = service.UpdateSource(ctx, &OpenAPISourceUpdateRequest{SourceID: "s", Spec: json.RawMessage(`{}`)})
	require.Error(t, err)
	_, err = service.CreateBinding(ctx, &OpenAPISourceBindingCreateRequest{
		SourceID: "s", OperationID: "op", Kind: "provider",
	})
	require.Error(t, err)
	_, err = service.DeleteBinding(ctx, &OpenAPISourceBindingDeleteRequest{SourceID: "s", BindingID: "b"})
	require.Error(t, err)
}

func TestRequireScopeMissingGameIDOpenAPI(t *testing.T) {
	_, _, err := requireScope(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "X-Game-ID is required")
}

func TestAuditSourceEventNilServiceGuard(t *testing.T) {
	NewService(&svc.ServiceContext{}).auditSourceEvent(
		context.Background(), audit.EventOpenAPISourceCreate, "g", "e", "src", "name", nil)
	var nilService *Service
	nilService.auditSourceEvent(context.Background(), audit.EventOpenAPISourceCreate,
		"g", "e", "src", "name", nil)
}

func TestGeneratedProposalForBindingGuards(t *testing.T) {
	assert.Nil(t, NewService(&svc.ServiceContext{}).generatedProposalForBinding(
		openAPITestContext(), "g", "e", "fn"))

	service := setupOpenAPITestService(t)
	// Unknown function has no contract → no proposal.
	assert.Nil(t, service.generatedProposalForBinding(openAPITestContext(),
		"demo-game", "development", "ghost.fn"))
}

func TestOperationKeyForCapabilityFallbacks(t *testing.T) {
	assert.Equal(t, "", operationKeyForCapability(dashspec.CapabilityKind("bogus"), "/players/{id}"))
	assert.Equal(t, "action", operationKeyForCapability(dashspec.CapabilityAction, "/"))
}

func TestOpenAPISchemaRefJSONRefOnly(t *testing.T) {
	out := openAPISchemaRefJSON(&openapi3.SchemaRef{Ref: "#/components/schemas/User"})
	assert.Contains(t, out, "$ref")

	empty := openAPISchemaRefJSON(&openapi3.SchemaRef{})
	assert.Equal(t, "", empty)
}

func TestMergePathLevelParametersSkipsDuplicatesByLocationKey(t *testing.T) {
	opParam := &openapi3.ParameterRef{Value: openapi3.NewQueryParameter("id")}
	pathItem := &openapi3.PathItem{
		Parameters: []*openapi3.ParameterRef{
			opParam, // same in+name key → skipped
		},
	}
	op := &openapi3.Operation{Parameters: []*openapi3.ParameterRef{opParam}}
	mergePathLevelParameters(pathItem, op)
	assert.Len(t, op.Parameters, 1)
}

func TestOpenAPIOperationsByIDToleratesNullPathItems(t *testing.T) {
	raw := json.RawMessage(`{
		"openapi": "3.0.3",
		"info": {"title": "t", "version": "1"},
		"paths": {"/broken": null}
	}`)
	ops := openAPIOperationsByID(raw)
	assert.Empty(t, ops)
}

func TestOpenAPIRequestSchemaBodyRequiredDedupAndSkips(t *testing.T) {
	stringSchema := &openapi3.SchemaRef{Value: openapi3.NewStringSchema()}

	bodyRequired := &openapi3.SchemaRef{Value: &openapi3.Schema{
		Type: &openapi3.Types{"object"},
		Properties: openapi3.Schemas{
			"force": &openapi3.SchemaRef{Value: openapi3.NewBoolSchema()},
			"id":    stringSchema,
		},
		Required: []string{"force", "id"},
	}}
	op := &openapi3.Operation{
		Parameters: []*openapi3.ParameterRef{
			{Value: func() *openapi3.Parameter {
				p := openapi3.NewPathParameter("id")
				p.Required = true
				p.Schema = stringSchema
				return p
			}()},
			{Value: func() *openapi3.Parameter {
				p := openapi3.NewCookieParameter("session")
				p.Schema = stringSchema
				return p
			}()},
			{Value: func() *openapi3.Parameter {
				p := openapi3.NewQueryParameter("filter")
				p.Schema = &openapi3.SchemaRef{}
				return p
			}()},
		},
		RequestBody: &openapi3.RequestBodyRef{Value: &openapi3.RequestBody{
			Content: openapi3.Content{"application/json": {Schema: bodyRequired}},
		}},
	}
	raw := openAPIRequestSchema(op)
	var parsed struct {
		Properties map[string]json.RawMessage `json:"properties"`
		Required   []string                   `json:"required"`
	}
	require.NoError(t, json.Unmarshal([]byte(raw), &parsed))
	assert.Contains(t, parsed.Properties, "id")
	assert.Contains(t, parsed.Properties, "force")

	foundForce := false
	for name := range parsed.Properties {
		if name == "force" {
			foundForce = true
		}
	}
	assert.True(t, foundForce)

	countID := 0
	for _, name := range parsed.Required {
		if name == "id" {
			countID++
		}
	}
	assert.Equal(t, 1, countID, "required entries must be deduplicated")
}

// ---------------------------------------------------------------------------
// Final branch sweep: scope errors, persistence failures, schema edge cases
// ---------------------------------------------------------------------------

func TestOpenAPIServiceScopeErrorsWithWritePermission(t *testing.T) {
	service, _, _ := setupOpenAPITestServiceWithAudit(t, "openapi_sources:write")
	ctx := context.WithValue(context.Background(), "username", "openapi_tester")

	_, err := service.CreateSource(ctx, &OpenAPISourceCreateRequest{Spec: json.RawMessage(`{}`)})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "X-Game-ID")

	_, err = service.UpdateSource(ctx, &OpenAPISourceUpdateRequest{SourceID: "s", Spec: json.RawMessage(`{}`)})
	require.Error(t, err)
	_, err = service.CreateBinding(ctx, &OpenAPISourceBindingCreateRequest{
		SourceID: "s", OperationID: "op", Kind: "provider",
	})
	require.Error(t, err)
	_, err = service.DeleteBinding(ctx, &OpenAPISourceBindingDeleteRequest{SourceID: "s", BindingID: "b"})
	require.Error(t, err)
}

func TestRebuildContractForSourceBindingPersistenceFailures(t *testing.T) {
	buildService := func(t *testing.T) (*Service, context.Context) {
		service, _ := setupOpenAPITestServiceWithPermissions(t, "openapi_sources:read", "openapi_sources:write")
		return service, openAPITestContext()
	}
	binding := &model.OpenAPISourceBinding{FunctionID: "player.get", OperationID: "getUser"}
	operation := OpenAPISourceOperation{OperationID: "getUser"}

	// Contract table gone → rebuild fails.
	service, ctx := buildService(t)
	require.NoError(t, service.svcCtx.DB.Migrator().DropTable("function_contracts"))
	err := service.rebuildContractForSourceBinding(ctx, "demo-game", "development",
		nil, operation, binding, runtimeMetaStub())
	require.Error(t, err)

	// Resource capability table gone → resource rebuild fails.
	service, ctx = buildService(t)
	require.NoError(t, service.svcCtx.DB.Migrator().DropTable("resource_capabilities"))
	meta := runtimeMetaStub()
	meta.Resource = "player"
	err = service.rebuildContractForSourceBinding(ctx, "demo-game", "development",
		nil, operation, binding, meta)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rebuild OpenAPI resource capability")

	// Proposal table gone → proposal rebuild fails.
	service, ctx = buildService(t)
	require.NoError(t, service.svcCtx.DB.Migrator().DropTable("page_proposals"))
	meta = runtimeMetaStub()
	meta.Resource = "player"
	err = service.rebuildContractForSourceBinding(ctx, "demo-game", "development",
		nil, operation, binding, meta)
	require.Error(t, err)

	// Standalone proposal failure for functions without a resource key.
	service, ctx = buildService(t)
	require.NoError(t, service.svcCtx.DB.Migrator().DropTable("page_proposals"))
	err = service.rebuildContractForSourceBinding(ctx, "demo-game", "development",
		nil, operation, binding, runtimeMetaStub())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "standalone page proposal")
}

func TestUpdateSourceSurfacesRebuildFailure(t *testing.T) {
	service, ctx := setupOpenAPITestServiceWithPermissions(t, "openapi_sources:read", "openapi_sources:write")

	sourceResp, err := service.CreateSource(ctx, &OpenAPISourceCreateRequest{
		Name: "update-rebuild",
		Spec: json.RawMessage(coverageSourceSpec(t)),
	})
	require.NoError(t, err)

	_, err = service.CreateBinding(ctx, &OpenAPISourceBindingCreateRequest{
		SourceID:    sourceResp.Source.SourceID,
		BindingID:   "b-update",
		OperationID: "getUser",
		Kind:        "provider",
		FunctionID:  "player.get",
	})
	require.NoError(t, err)

	require.NoError(t, service.svcCtx.DB.Migrator().DropTable("function_contracts"))

	_, err = service.UpdateSource(ctx, &OpenAPISourceUpdateRequest{
		SourceID: sourceResp.Source.SourceID,
		Spec:     json.RawMessage(coverageSourceSpec(t)),
	})
	require.Error(t, err)
}

func TestReconcileSurfacesLookupAndRestoreFailures(t *testing.T) {
	// Remaining-binding listing fails when the binding table is gone.
	service, ctx := setupOpenAPITestServiceWithPermissions(t, "openapi_sources:read", "openapi_sources:write")
	require.NoError(t, service.svcCtx.DB.Migrator().DropTable("openapi_source_bindings"))
	err := service.reconcileContractAfterBindingDelete(ctx, "demo-game", "development",
		&model.OpenAPISourceBinding{FunctionID: "player.get"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "list remaining OpenAPI bindings")

	// SDK restore fails when contracts cannot be loaded or rebuilt.
	service, ctx = setupOpenAPITestServiceWithPermissions(t, "openapi_sources:read", "openapi_sources:write")
	require.NoError(t, service.svcCtx.DB.Migrator().DropTable("function_contracts"))
	err = service.reconcileContractAfterBindingDelete(ctx, "demo-game", "development",
		&model.OpenAPISourceBinding{FunctionID: "player.get"})
	require.Error(t, err)
}

func TestLoadSourceWithBindingsModelFailures(t *testing.T) {
	service, ctx := setupOpenAPITestServiceWithPermissions(t, "openapi_sources:read", "openapi_sources:write")
	require.NoError(t, service.svcCtx.DB.Migrator().DropTable("openapi_sources"))

	_, _, err := service.loadSourceWithBindings(ctx, "any")
	require.Error(t, err)

	service, ctx = setupOpenAPITestServiceWithPermissions(t, "openapi_sources:read", "openapi_sources:write")
	require.NoError(t, service.svcCtx.DB.Migrator().DropTable("openapi_source_bindings"))
	_, _, err = service.loadSourceWithBindings(ctx, "any")
	require.Error(t, err)
}

func TestCorruptedOperationsJSONSurfacesThroughFlows(t *testing.T) {
	service, ctx := setupOpenAPITestServiceWithPermissions(t, "openapi_sources:read", "openapi_sources:write")

	sourceResp, err := service.CreateSource(ctx, &OpenAPISourceCreateRequest{
		Name: "corrupt-me",
		Spec: json.RawMessage(coverageSourceSpec(t)),
	})
	require.NoError(t, err)
	sourceID := sourceResp.Source.SourceID

	// Corrupt the stored operations payload directly.
	require.NoError(t, service.svcCtx.DB.Model(&model.OpenAPISource{}).
		Where("source_id = ?", sourceID).
		Update("operations_json", "{not-json").Error)

	_, err = service.CreateBinding(ctx, &OpenAPISourceBindingCreateRequest{
		SourceID:    sourceID,
		BindingID:   "b-corrupt",
		OperationID: "getUser",
		Kind:        "provider",
		FunctionID:  "player.get",
	})
	require.Error(t, err)
}

func TestOpenAPIResponseSchemaFallbackCodes(t *testing.T) {
	stringSchema := &openapi3.SchemaRef{Value: openapi3.NewStringSchema()}
	responses := openapi3.NewResponses()
	responses.Set("400", &openapi3.ResponseRef{Value: &openapi3.Response{
		Description: &[]string{"bad"}[0],
		Content:     openapi3.Content{"application/json": {Schema: stringSchema}},
	}})
	op := &openapi3.Operation{Responses: responses}
	assert.Contains(t, openAPIResponseSchema(op), "string")

	noSchemaContent := openapi3.Content{"text/plain": {}}
	assert.Equal(t, "", openAPIMediaSchemaJSON(noSchemaContent))
}
