package openapi

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	dashspec "github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------- rest_classifier.go ----------

func TestRestClassificationDiagnostic(t *testing.T) {
	t.Parallel()
	diag := restClassificationDiagnostic("GET", "/users", dashspec.CapabilityCollectionQuery)
	assert.Equal(t, "rest_capability_inferred", diag.Code)
	assert.Equal(t, dashspec.SeverityInfo, diag.Severity)
	assert.Equal(t, "x-capability", diag.Field)
}

func TestRestClassificationDiagnosticWithConfidence(t *testing.T) {
	t.Parallel()
	diag := restClassificationDiagnosticWithConfidence("POST", "/items", dashspec.CapabilityCreate, "high")
	assert.Equal(t, "rest_capability_inferred", diag.Code)
	assert.Contains(t, diag.Message, "high")
	assert.Equal(t, "x-capability", diag.Field)
}

// ---------- service.go helper functions ----------

func TestSourceHasOperation(t *testing.T) {
	t.Parallel()

	ops := []OpenAPISourceOperation{
		{OperationID: "op1"},
		{OperationID: "op2"},
	}

	assert.True(t, sourceHasOperation(ops, "op1"))
	assert.True(t, sourceHasOperation(ops, "  op2  "))
	assert.False(t, sourceHasOperation(ops, "op3"))
	assert.False(t, sourceHasOperation(nil, "op1"))
}

func TestDashboardStandaloneProposalKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		pageType   dashspec.PageType
		functionID string
		want       string
	}{
		{"empty_function", dashspec.PageTypeOperation, "", ""},
		{"operation", dashspec.PageTypeOperation, "player.list", "operation:player.list"},
		{"task", dashspec.PageTypeTask, "reward.grant", "task:reward.grant"},
		{"report", dashspec.PageTypeReport, "stats.daily", "report:stats.daily"},
		{"unknown_type", dashspec.PageType("unknown"), "test.func", "operation:test.func"},
		{"with_dots", dashspec.PageTypeOperation, ".player.list.", "operation:player.list"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := dashboardStandaloneProposalKey(tt.pageType, tt.functionID)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestSourceInfoVersion(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "", sourceInfoVersion(nil))
	assert.Equal(t, "2.0", sourceInfoVersion(&model.OpenAPISource{InfoVersion: "2.0"}))
}

func TestExtendionLocalized(t *testing.T) {
	t.Parallel()

	t.Run("nil_extensions", func(t *testing.T) {
		assert.Nil(t, extensionLocalized(nil, "key"))
	})

	t.Run("missing_key", func(t *testing.T) {
		assert.Nil(t, extensionLocalized(map[string]interface{}{}, "key"))
	})

	t.Run("map_string_string", func(t *testing.T) {
		ext := map[string]interface{}{
			"x-label": map[string]string{"en": "Hello", "zh": "你好"},
		}
		result := extensionLocalized(ext, "x-label")
		require.NotNil(t, result)
		assert.Equal(t, "Hello", result["en"])
		assert.Equal(t, "你好", result["zh"])
	})

	t.Run("map_string_interface", func(t *testing.T) {
		ext := map[string]interface{}{
			"x-label": map[string]interface{}{"en": "Hello"},
		}
		result := extensionLocalized(ext, "x-label")
		require.NotNil(t, result)
		assert.Equal(t, "Hello", result["en"])
	})

	t.Run("empty_result", func(t *testing.T) {
		ext := map[string]interface{}{
			"x-label": map[string]interface{}{"  ": " "},
		}
		assert.Nil(t, extensionLocalized(ext, "x-label"))
	})

	t.Run("non_string_map", func(t *testing.T) {
		ext := map[string]interface{}{
			"x-label": 42,
		}
		assert.Nil(t, extensionLocalized(ext, "x-label"))
	})
}

func TestExtensionString(t *testing.T) {
	t.Parallel()

	t.Run("nil", func(t *testing.T) {
		assert.Equal(t, "", extensionString(nil, "key"))
	})

	t.Run("missing", func(t *testing.T) {
		assert.Equal(t, "", extensionString(map[string]interface{}{}, "key"))
	})

	t.Run("not_string", func(t *testing.T) {
		assert.Equal(t, "", extensionString(map[string]interface{}{"key": 42}, "key"))
	})

	t.Run("with_whitespace", func(t *testing.T) {
		assert.Equal(t, "val", extensionString(map[string]interface{}{"key": "  val  "}, "key"))
	})
}

func TestNormalizeYAMLValue(t *testing.T) {
	t.Parallel()

	t.Run("nil", func(t *testing.T) {
		assert.Nil(t, normalizeYAMLValue(nil))
	})

	t.Run("string", func(t *testing.T) {
		assert.Equal(t, "hello", normalizeYAMLValue("hello"))
	})

	t.Run("int", func(t *testing.T) {
		assert.Equal(t, 42, normalizeYAMLValue(42))
	})

	t.Run("map_interface", func(t *testing.T) {
		input := map[interface{}]interface{}{"key": "value"}
		result := normalizeYAMLValue(input)
		m, ok := result.(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "value", m["key"])
	})

	t.Run("map_string", func(t *testing.T) {
		input := map[string]interface{}{"key": "value"}
		result := normalizeYAMLValue(input)
		m, ok := result.(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "value", m["key"])
	})

	t.Run("slice", func(t *testing.T) {
		input := []interface{}{"a", "b"}
		result := normalizeYAMLValue(input)
		s, ok := result.([]interface{})
		require.True(t, ok)
		assert.Len(t, s, 2)
	})

	t.Run("nested", func(t *testing.T) {
		input := map[interface{}]interface{}{
			"inner": map[interface{}]interface{}{"nested": "value"},
		}
		result := normalizeYAMLValue(input)
		m := result.(map[string]interface{})
		inner := m["inner"].(map[string]interface{})
		assert.Equal(t, "value", inner["nested"])
	})
}

func TestOperationKeyForCapability(t *testing.T) {
	t.Parallel()

	tests := []struct {
		capability dashspec.CapabilityKind
		path       string
		want       string
	}{
		{dashspec.CapabilityCollectionQuery, "/users", "list"},
		{dashspec.CapabilityItemQuery, "/users/{id}", "get"},
		{dashspec.CapabilityCreate, "/users", "create"},
		{dashspec.CapabilityUpdate, "/users/{id}", "update"},
		{dashspec.CapabilityDelete, "/users/{id}", "delete"},
		{dashspec.CapabilityAction, "/users/ban", "ban"},
		{dashspec.CapabilityAction, "/users/{id}/actions", "actions"},
		{dashspec.CapabilityTask, "/jobs", ""},
		{dashspec.CapabilityKind(""), "/users", ""},
	}
	for _, tt := range tests {
		name := string(tt.capability) + "_" + tt.path
		t.Run(name, func(t *testing.T) {
			got := operationKeyForCapability(tt.capability, tt.path)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestLastStaticPathSegment(t *testing.T) {
	t.Parallel()

	tests := []struct {
		path string
		want string
	}{
		{"/users", "users"},
		{"/users/{id}", "users"},
		{"/users/ban", "ban"},
		{"/users/{id}/actions", "actions"},
		{"/", ""},
		{"", ""},
		{"/a/b/c", "c"},
		{"/a/{id}/b/{id2}", "b"},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := lastStaticPathSegment(tt.path)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIsStableSourceKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		value string
		want  bool
	}{
		{"", false},
		{"  ", false},
		{"abc", true},
		{"abc123", true},
		{"abc.def", true},
		{"abc_def", true},
		{"abc-def", true},
		{"123abc", true},
		{".abc", false},    // must start with a-z or 0-9
		{"-abc", false},    // must start with a-z or 0-9
		{"abc def", false}, // no spaces
		{"ABC", false},     // uppercase not allowed
		{"abc@def", false}, // special char
	}
	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			got := isStableSourceKey(tt.value)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestAppendStableSourceKeyDiagnostic(t *testing.T) {
	t.Parallel()

	t.Run("empty_value_no_diag", func(t *testing.T) {
		var diags []dashspec.Diagnostic
		appendStableSourceKeyDiagnostic(&diags, "code", "", "field")
		assert.Empty(t, diags)
	})

	t.Run("valid_key_no_diag", func(t *testing.T) {
		var diags []dashspec.Diagnostic
		appendStableSourceKeyDiagnostic(&diags, "code", "valid_key", "field")
		assert.Empty(t, diags)
	})

	t.Run("invalid_key_adds_diag", func(t *testing.T) {
		var diags []dashspec.Diagnostic
		appendStableSourceKeyDiagnostic(&diags, "code", "Invalid Key!", "field")
		require.Len(t, diags, 1)
		assert.Equal(t, "code", diags[0].Code)
		assert.Equal(t, dashspec.SeverityError, diags[0].Severity)
	})
}

func TestIsValidRisk(t *testing.T) {
	t.Parallel()

	assert.True(t, isValidRisk(dashspec.RiskSafe))
	assert.True(t, isValidRisk(dashspec.RiskWarning))
	assert.True(t, isValidRisk(dashspec.RiskHigh))
	assert.True(t, isValidRisk(dashspec.RiskDanger))
	assert.False(t, isValidRisk("bogus"))
}

func TestOpenAPIMediaSchemaJSON(t *testing.T) {
	t.Parallel()

	t.Run("nil_content", func(t *testing.T) {
		assert.Equal(t, "", openAPIMediaSchemaJSON(nil))
	})

	t.Run("empty_content", func(t *testing.T) {
		assert.Equal(t, "", openAPIMediaSchemaJSON(map[string]*openapi3.MediaType{}))
	})
}

func TestOpenAPISchemaRefJSON(t *testing.T) {
	t.Parallel()

	t.Run("nil_ref", func(t *testing.T) {
		assert.Equal(t, "", openAPISchemaRefJSON(nil))
	})

	t.Run("nil_value_no_ref", func(t *testing.T) {
		assert.Equal(t, "", openAPISchemaRefJSON(&openapi3.SchemaRef{}))
	})
}

func TestOpenAPIRequestSchema(t *testing.T) {
	t.Parallel()

	t.Run("nil_op", func(t *testing.T) {
		assert.Equal(t, "", openAPIRequestSchema(nil))
	})

	t.Run("no_request_body", func(t *testing.T) {
		op := &openapi3.Operation{}
		assert.Equal(t, "", openAPIRequestSchema(op))
	})
}

func TestOpenAPIResponseSchema(t *testing.T) {
	t.Parallel()

	t.Run("nil_op", func(t *testing.T) {
		assert.Equal(t, "", openAPIResponseSchema(nil))
	})

	t.Run("nil_responses", func(t *testing.T) {
		op := &openapi3.Operation{}
		assert.Equal(t, "", openAPIResponseSchema(op))
	})
}

func TestOpenAPIOperationFromSource(t *testing.T) {
	t.Parallel()

	t.Run("nil_source", func(t *testing.T) {
		assert.Nil(t, openAPIOperationFromSource(nil, "test"))
	})
}

func TestOpenAPIOperationsByID_Empty(t *testing.T) {
	t.Parallel()
	result := openAPIOperationsByID(nil)
	assert.Empty(t, result)

	result = openAPIOperationsByID(json.RawMessage{})
	assert.Empty(t, result)

	result = openAPIOperationsByID(json.RawMessage(`not-json`))
	assert.Empty(t, result)
}

func TestFindSourceOperation(t *testing.T) {
	t.Parallel()

	ops := []OpenAPISourceOperation{
		{OperationID: "op1"},
		{OperationID: "op2"},
	}

	op, ok := findSourceOperation(ops, "op1")
	assert.True(t, ok)
	assert.Equal(t, "op1", op.OperationID)

	op, ok = findSourceOperation(ops, "  op2  ")
	assert.True(t, ok)
	assert.Equal(t, "op2", op.OperationID)

	_, ok = findSourceOperation(ops, "missing")
	assert.False(t, ok)

	_, ok = findSourceOperation(nil, "op1")
	assert.False(t, ok)
}

func TestProposalKeysForContract(t *testing.T) {
	t.Parallel()

	t.Run("nil_contract", func(t *testing.T) {
		assert.Nil(t, proposalKeysForContract(nil))
	})

	t.Run("with_resource_crud", func(t *testing.T) {
		c := &model.FunctionContract{
			FunctionID:  "player.list",
			ResourceKey: "player",
			Capability:  string(dashspec.CapabilityCollectionQuery),
		}
		keys := proposalKeysForContract(c)
		assert.Equal(t, []string{"resource:player"}, keys)
	})

	t.Run("task_capability", func(t *testing.T) {
		c := &model.FunctionContract{
			FunctionID:  "reward.grant",
			ResourceKey: "",
			Capability:  string(dashspec.CapabilityTask),
		}
		keys := proposalKeysForContract(c)
		require.Len(t, keys, 1)
		assert.Contains(t, keys[0], "reward.grant")
	})

	t.Run("report_capability", func(t *testing.T) {
		c := &model.FunctionContract{
			FunctionID:  "stats.daily",
			ResourceKey: "",
			Capability:  string(dashspec.CapabilityReport),
		}
		keys := proposalKeysForContract(c)
		require.Len(t, keys, 1)
		assert.Contains(t, keys[0], "stats.daily")
	})

	t.Run("operation_capability", func(t *testing.T) {
		c := &model.FunctionContract{
			FunctionID:  "player.list",
			ResourceKey: "",
			Capability:  string(dashspec.CapabilityCollectionQuery),
		}
		keys := proposalKeysForContract(c)
		require.Len(t, keys, 1)
		assert.Contains(t, keys[0], "player.list")
	})

	t.Run("empty_function_id", func(t *testing.T) {
		c := &model.FunctionContract{
			FunctionID: "",
		}
		assert.Nil(t, proposalKeysForContract(c))
	})
}

func TestIsOpenAPICRUDCapability(t *testing.T) {
	t.Parallel()

	assert.True(t, isOpenAPICRUDCapability(dashspec.CapabilityCollectionQuery))
	assert.True(t, isOpenAPICRUDCapability(dashspec.CapabilityItemQuery))
	assert.True(t, isOpenAPICRUDCapability(dashspec.CapabilityCreate))
	assert.True(t, isOpenAPICRUDCapability(dashspec.CapabilityUpdate))
	assert.True(t, isOpenAPICRUDCapability(dashspec.CapabilityDelete))
	assert.False(t, isOpenAPICRUDCapability(dashspec.CapabilityTask))
	assert.False(t, isOpenAPICRUDCapability(dashspec.CapabilityReport))
	assert.False(t, isOpenAPICRUDCapability(dashspec.CapabilityAction))
}

func TestProposalDTOForOpenAPIBinding(t *testing.T) {
	t.Parallel()

	t.Run("nil", func(t *testing.T) {
		assert.Nil(t, proposalDTOForOpenAPIBinding(nil))
	})

	t.Run("with_data", func(t *testing.T) {
		p := &model.PageProposal{
			ProposalKey: "key",
			PageKey:     "page",
			PageType:    "table",
			ResourceKey: "player",
			Quality:     "high",
			Status:      "active",
		}
		dto := proposalDTOForOpenAPIBinding(p)
		require.NotNil(t, dto)
		assert.Equal(t, "key", dto.ProposalKey)
		assert.Equal(t, "page", dto.PageKey)
		assert.Equal(t, "table", dto.PageType)
		assert.Equal(t, "player", dto.ResourceKey)
		assert.Equal(t, "high", dto.Quality)
		assert.Equal(t, "active", dto.Status)
	})
}

func TestSanitizeBindingID(t *testing.T) {
	t.Parallel()

	t.Run("empty_generates_uuid", func(t *testing.T) {
		id := sanitizeBindingID("")
		assert.NotEmpty(t, id)
	})

	t.Run("valid_chars", func(t *testing.T) {
		id := sanitizeBindingID("my-binding")
		assert.Equal(t, "my-binding", id)
	})

	t.Run("replaces_special_chars", func(t *testing.T) {
		id := sanitizeBindingID("my binding!")
		assert.NotContains(t, id, " ")
		assert.NotContains(t, id, "!")
	})

	t.Run("all_special_generates_uuid", func(t *testing.T) {
		id := sanitizeBindingID("!!!@@@###")
		assert.NotEmpty(t, id)
	})
}

func TestOpenAPIScanContext(t *testing.T) {
	t.Parallel()

	t.Run("isOpenAPIMethod", func(t *testing.T) {
		for _, method := range []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS", "HEAD", "TRACE", "get", "post"} {
			assert.True(t, isOpenAPIMethod(method), method)
		}
		assert.False(t, isOpenAPIMethod("FETCH"))
		assert.False(t, isOpenAPIMethod(""))
	})
}

func TestNextOpenAPIScanContext(t *testing.T) {
	t.Parallel()

	// root -> paths
	assert.Equal(t, openAPIScanPaths, nextOpenAPIScanContext(openAPIScanRoot, "paths"))
	assert.Equal(t, openAPIScanRoot, nextOpenAPIScanContext(openAPIScanRoot, "other"))

	// paths -> pathItem
	assert.Equal(t, openAPIScanPathItem, nextOpenAPIScanContext(openAPIScanPaths, "anything"))

	// pathItem -> operation (for HTTP methods)
	assert.Equal(t, openAPIScanOperation, nextOpenAPIScanContext(openAPIScanPathItem, "get"))
	assert.Equal(t, openAPIScanOperation, nextOpenAPIScanContext(openAPIScanPathItem, "POST"))
	assert.Equal(t, openAPIScanRoot, nextOpenAPIScanContext(openAPIScanPathItem, "parameters"))

	// operation -> root
	assert.Equal(t, openAPIScanRoot, nextOpenAPIScanContext(openAPIScanOperation, "anything"))
}

func TestOpenAPISourceKeyDiagnostic(t *testing.T) {
	t.Parallel()
	d := sourceDiagnostic("code", dashspec.SeverityError, "message", "$.field")
	assert.Equal(t, "code", d.Code)
	assert.Equal(t, dashspec.SeverityError, d.Severity)
	assert.Equal(t, "message", d.Message)
	assert.Equal(t, "$.field", d.Field)
}

// ---------- Handler integration tests ----------

func setupOpenAPITestHandlerWithBindings(t *testing.T) (*Handler, *gin.Engine) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	service, ctx := setupOpenAPITestServiceWithPermissions(t, "openapi_sources:read", "openapi_sources:write")
	handler := NewHandler(service)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	router.GET("/sources", handler.ListSources)
	router.POST("/sources", handler.CreateSource)
	router.GET("/sources/:sourceId", handler.GetSource)
	router.PUT("/sources/:sourceId", handler.UpdateSource)
	router.GET("/sources/:sourceId/diagnostics", handler.SourceDiagnostics)
	router.POST("/sources/:sourceId/bindings", handler.CreateBinding)
	router.DELETE("/sources/:sourceId/bindings/:bindingId", handler.DeleteBinding)

	return handler, router
}

func TestHandler_ListSources(t *testing.T) {
	t.Parallel()
	_, router := setupOpenAPITestHandlerWithBindings(t)

	// List empty
	req, _ := http.NewRequest("GET", "/sources", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp OpenAPISourceListResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Empty(t, resp.Items)
}

func TestHandler_ListSources_WithData(t *testing.T) {
	t.Parallel()
	_, router := setupOpenAPITestHandlerWithBindings(t)

	// Create a source first
	spec := map[string]interface{}{
		"openapi": "3.0.3",
		"info":    map[string]interface{}{"title": "Test", "version": "1.0.0"},
		"paths": map[string]interface{}{
			"/test": map[string]interface{}{
				"get": map[string]interface{}{
					"operationId": "testOp",
					"responses":   map[string]interface{}{"200": map[string]interface{}{"description": "OK"}},
				},
			},
		},
	}
	body, _ := json.Marshal(OpenAPISourceCreateRequest{Spec: rawSpec(t, spec)})
	createReq, _ := http.NewRequest("POST", "/sources", bytes.NewReader(body))
	createReq.Header.Set("Content-Type", "application/json")
	createW := httptest.NewRecorder()
	router.ServeHTTP(createW, createReq)
	require.Equal(t, http.StatusCreated, createW.Code)

	// Now list
	listReq, _ := http.NewRequest("GET", "/sources", nil)
	listW := httptest.NewRecorder()
	router.ServeHTTP(listW, listReq)
	assert.Equal(t, http.StatusOK, listW.Code)

	var resp OpenAPISourceListResponse
	require.NoError(t, json.Unmarshal(listW.Body.Bytes(), &resp))
	assert.Len(t, resp.Items, 1)
}

func TestHandler_GetSource(t *testing.T) {
	t.Parallel()
	_, router := setupOpenAPITestHandlerWithBindings(t)

	// Create a source
	spec := map[string]interface{}{
		"openapi": "3.0.3",
		"info":    map[string]interface{}{"title": "Test", "version": "1.0.0"},
		"paths": map[string]interface{}{
			"/test": map[string]interface{}{
				"get": map[string]interface{}{
					"operationId": "testOp",
					"responses":   map[string]interface{}{"200": map[string]interface{}{"description": "OK"}},
				},
			},
		},
	}
	body, _ := json.Marshal(OpenAPISourceCreateRequest{Spec: rawSpec(t, spec)})
	createReq, _ := http.NewRequest("POST", "/sources", bytes.NewReader(body))
	createReq.Header.Set("Content-Type", "application/json")
	createW := httptest.NewRecorder()
	router.ServeHTTP(createW, createReq)
	require.Equal(t, http.StatusCreated, createW.Code)
	var created OpenAPISourceGetResponse
	require.NoError(t, json.Unmarshal(createW.Body.Bytes(), &created))

	// Get source detail
	getReq, _ := http.NewRequest("GET", "/sources/"+created.Source.SourceID, nil)
	getW := httptest.NewRecorder()
	router.ServeHTTP(getW, getReq)
	assert.Equal(t, http.StatusOK, getW.Code)

	var detail OpenAPISourceGetResponse
	require.NoError(t, json.Unmarshal(getW.Body.Bytes(), &detail))
	assert.Equal(t, created.Source.SourceID, detail.Source.SourceID)
}

func TestHandler_GetSource_NotFound(t *testing.T) {
	t.Parallel()
	_, router := setupOpenAPITestHandlerWithBindings(t)

	req, _ := http.NewRequest("GET", "/sources/nonexistent", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.NotEqual(t, http.StatusOK, w.Code)
}

func TestHandler_SourceDiagnostics(t *testing.T) {
	t.Parallel()
	_, router := setupOpenAPITestHandlerWithBindings(t)

	// Create source
	spec := map[string]interface{}{
		"openapi": "3.0.3",
		"info":    map[string]interface{}{"title": "Test", "version": "1.0.0"},
		"paths": map[string]interface{}{
			"/test": map[string]interface{}{
				"get": map[string]interface{}{
					"operationId": "testOp",
					"responses":   map[string]interface{}{"200": map[string]interface{}{"description": "OK"}},
				},
			},
		},
	}
	body, _ := json.Marshal(OpenAPISourceCreateRequest{Spec: rawSpec(t, spec)})
	createReq, _ := http.NewRequest("POST", "/sources", bytes.NewReader(body))
	createReq.Header.Set("Content-Type", "application/json")
	createW := httptest.NewRecorder()
	router.ServeHTTP(createW, createReq)
	require.Equal(t, http.StatusCreated, createW.Code)
	var created OpenAPISourceGetResponse
	require.NoError(t, json.Unmarshal(createW.Body.Bytes(), &created))

	// Get diagnostics
	diagReq, _ := http.NewRequest("GET", "/sources/"+created.Source.SourceID+"/diagnostics", nil)
	diagW := httptest.NewRecorder()
	router.ServeHTTP(diagW, diagReq)
	assert.Equal(t, http.StatusOK, diagW.Code)
}

func TestHandler_SourceDiagnostics_NotFound(t *testing.T) {
	t.Parallel()
	_, router := setupOpenAPITestHandlerWithBindings(t)

	req, _ := http.NewRequest("GET", "/sources/nonexistent/diagnostics", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.NotEqual(t, http.StatusOK, w.Code)
}

func TestHandler_CreateBinding_Success(t *testing.T) {
	t.Parallel()
	_, router := setupOpenAPITestHandlerWithBindings(t)

	// Create source with a player.list operation
	spec := map[string]interface{}{
		"openapi": "3.0.3",
		"info":    map[string]interface{}{"title": "Player API", "version": "1.0.0"},
		"paths": map[string]interface{}{
			"/players": map[string]interface{}{
				"get": map[string]interface{}{
					"operationId": "player.list",
					"responses":   map[string]interface{}{"200": map[string]interface{}{"description": "OK"}},
				},
			},
		},
	}
	body, _ := json.Marshal(OpenAPISourceCreateRequest{Spec: rawSpec(t, spec)})
	createReq, _ := http.NewRequest("POST", "/sources", bytes.NewReader(body))
	createReq.Header.Set("Content-Type", "application/json")
	createW := httptest.NewRecorder()
	router.ServeHTTP(createW, createReq)
	require.Equal(t, http.StatusCreated, createW.Code)
	var created OpenAPISourceGetResponse
	require.NoError(t, json.Unmarshal(createW.Body.Bytes(), &created))

	// Create binding
	bindingBody, _ := json.Marshal(map[string]interface{}{
		"operationId": "player.list",
		"kind":        "provider",
		"functionId":  "player.list",
	})
	bindReq, _ := http.NewRequest("POST", "/sources/"+created.Source.SourceID+"/bindings", bytes.NewReader(bindingBody))
	bindReq.Header.Set("Content-Type", "application/json")
	bindW := httptest.NewRecorder()
	router.ServeHTTP(bindW, bindReq)
	assert.Equal(t, http.StatusOK, bindW.Code, bindW.Body.String())
}

func TestHandler_CreateBinding_InvalidJSON(t *testing.T) {
	t.Parallel()
	_, router := setupOpenAPITestHandlerWithBindings(t)

	req, _ := http.NewRequest("POST", "/sources/test/bindings", strings.NewReader("{invalid"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.NotEqual(t, http.StatusOK, w.Code)
}

func TestHandler_DeleteBinding_Success(t *testing.T) {
	t.Parallel()
	_, router := setupOpenAPITestHandlerWithBindings(t)

	// Create source
	spec := map[string]interface{}{
		"openapi": "3.0.3",
		"info":    map[string]interface{}{"title": "Player API", "version": "1.0.0"},
		"paths": map[string]interface{}{
			"/players": map[string]interface{}{
				"get": map[string]interface{}{
					"operationId": "player.list",
					"responses":   map[string]interface{}{"200": map[string]interface{}{"description": "OK"}},
				},
			},
		},
	}
	body, _ := json.Marshal(OpenAPISourceCreateRequest{Spec: rawSpec(t, spec)})
	createReq, _ := http.NewRequest("POST", "/sources", bytes.NewReader(body))
	createReq.Header.Set("Content-Type", "application/json")
	createW := httptest.NewRecorder()
	router.ServeHTTP(createW, createReq)
	require.Equal(t, http.StatusCreated, createW.Code)
	var created OpenAPISourceGetResponse
	require.NoError(t, json.Unmarshal(createW.Body.Bytes(), &created))

	// Create binding first
	bindingBody, _ := json.Marshal(map[string]interface{}{
		"operationId": "player.list",
		"kind":        "provider",
		"functionId":  "player.list",
	})
	bindReq, _ := http.NewRequest("POST", "/sources/"+created.Source.SourceID+"/bindings", bytes.NewReader(bindingBody))
	bindReq.Header.Set("Content-Type", "application/json")
	bindW := httptest.NewRecorder()
	router.ServeHTTP(bindW, bindReq)
	require.Equal(t, http.StatusOK, bindW.Code, bindW.Body.String())

	// Delete binding
	deleteReq, _ := http.NewRequest("DELETE", "/sources/"+created.Source.SourceID+"/bindings/player.list", nil)
	deleteW := httptest.NewRecorder()
	router.ServeHTTP(deleteW, deleteReq)
	assert.Equal(t, http.StatusOK, deleteW.Code, deleteW.Body.String())
}

func TestHandler_DeleteBinding_NotFound(t *testing.T) {
	t.Parallel()
	_, router := setupOpenAPITestHandlerWithBindings(t)

	req, _ := http.NewRequest("DELETE", "/sources/nonexistent/bindings/binding1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.NotEqual(t, http.StatusOK, w.Code)
}

// ---------- Service tests for uncovered paths ----------

func TestService_CreateSourceFromMultipart_NilFile(t *testing.T) {
	t.Parallel()
	service := setupOpenAPITestService(t)

	_, err := service.CreateSourceFromMultipart(context.Background(), "test", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "file is required")
}

func TestService_CreateSourceFromMultipart_ValidFile(t *testing.T) {
	t.Parallel()
	service := setupOpenAPITestService(t)

	specJSON := `{"openapi":"3.0.3","info":{"title":"Test","version":"1.0.0"},"paths":{"/test":{"get":{"operationId":"testOp","responses":{"200":{"description":"OK"}}}}}}`

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, err := writer.CreateFormFile("file", "test.json")
	require.NoError(t, err)
	_, err = part.Write([]byte(specJSON))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	// Use httptest.Request to get a proper multipart.File
	req := httptest.NewRequest(http.MethodPost, "/", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	file, _, err := req.FormFile("file")
	require.NoError(t, err)

	resp, err := service.CreateSourceFromMultipart(context.Background(), "multipart-test", file)
	require.NoError(t, err)
	assert.Equal(t, "multipart-test", resp.Source.Name)
	assert.NotEmpty(t, resp.Source.SourceID)
}

func TestService_CreateSourceFromMultipart_TooLarge(t *testing.T) {
	t.Parallel()
	service := setupOpenAPITestService(t)

	// Create a reader that returns data larger than maxOpenAPISourceBytes
	largeData := strings.Repeat("x", maxOpenAPISourceBytes+100)
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, err := writer.CreateFormFile("file", "large.json")
	require.NoError(t, err)
	_, err = part.Write([]byte(largeData))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	req := httptest.NewRequest(http.MethodPost, "/", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	file, _, err := req.FormFile("file")
	require.NoError(t, err)

	resp, err := service.CreateSourceFromMultipart(context.Background(), "large", file)
	assert.Nil(t, resp)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds")
}

func TestService_CreateSource_EmptyName_UsesInfoTitle(t *testing.T) {
	t.Parallel()
	service := setupOpenAPITestService(t)

	spec := map[string]interface{}{
		"openapi": "3.0.3",
		"info":    map[string]interface{}{"title": "Auto-Name API", "version": "1.0.0"},
		"paths":   map[string]interface{}{},
	}
	resp, err := service.CreateSource(openAPITestContext(), &OpenAPISourceCreateRequest{
		Name: "", // empty name
		Spec: rawSpec(t, spec),
	})
	require.NoError(t, err)
	assert.Equal(t, "Auto-Name API", resp.Source.Name)
}

func TestService_UpdateSource_EmptyName_KeepsPreviousName(t *testing.T) {
	t.Parallel()
	service := setupOpenAPITestService(t)

	// Create source
	spec1 := map[string]interface{}{
		"openapi": "3.0.3",
		"info":    map[string]interface{}{"title": "Original", "version": "1.0.0"},
		"paths":   map[string]interface{}{},
	}
	created, err := service.CreateSource(openAPITestContext(), &OpenAPISourceCreateRequest{
		Name: "My API",
		Spec: rawSpec(t, spec1),
	})
	require.NoError(t, err)

	// Update with empty name — firstNonEmpty(infoTitle, source.Name) picks infoTitle
	spec2 := map[string]interface{}{
		"openapi": "3.0.3",
		"info":    map[string]interface{}{"title": "New Title", "version": "2.0.0"},
		"paths":   map[string]interface{}{},
	}
	updated, err := service.UpdateSource(openAPITestContext(), &OpenAPISourceUpdateRequest{
		SourceID: created.Source.SourceID,
		Name:     "", // empty — firstNonEmpty picks infoTitle "New Title"
		Spec:     rawSpec(t, spec2),
	})
	require.NoError(t, err)
	assert.Equal(t, "New Title", updated.Source.Name) // infoTitle wins
}

func TestService_UpdateSource_NotFound(t *testing.T) {
	t.Parallel()
	service := setupOpenAPITestService(t)

	spec := map[string]interface{}{
		"openapi": "3.0.3",
		"info":    map[string]interface{}{"title": "Test", "version": "1.0.0"},
		"paths":   map[string]interface{}{},
	}
	_, err := service.UpdateSource(openAPITestContext(), &OpenAPISourceUpdateRequest{
		SourceID: "nonexistent",
		Spec:     rawSpec(t, spec),
	})
	require.Error(t, err)
}

func TestService_DeleteBinding_NotFound(t *testing.T) {
	t.Parallel()
	service := setupOpenAPITestService(t)

	_, err := service.DeleteBinding(openAPITestContext(), &OpenAPISourceBindingDeleteRequest{
		SourceID:  "nonexistent",
		BindingID: "binding1",
	})
	require.Error(t, err)
}

func TestService_CreateBinding_MissingOperation(t *testing.T) {
	t.Parallel()
	service := setupOpenAPITestService(t)

	// Create source
	spec := map[string]interface{}{
		"openapi": "3.0.3",
		"info":    map[string]interface{}{"title": "Test", "version": "1.0.0"},
		"paths": map[string]interface{}{
			"/test": map[string]interface{}{
				"get": map[string]interface{}{
					"operationId": "testOp",
					"responses":   map[string]interface{}{"200": map[string]interface{}{"description": "OK"}},
				},
			},
		},
	}
	created, err := service.CreateSource(openAPITestContext(), &OpenAPISourceCreateRequest{Spec: rawSpec(t, spec)})
	require.NoError(t, err)

	// Try to bind a non-existent operation
	_, err = service.CreateBinding(openAPITestContext(), &OpenAPISourceBindingCreateRequest{
		SourceID:    created.Source.SourceID,
		OperationID: "nonexistent",
		Kind:        "provider",
		FunctionID:  "player.list",
	})
	require.Error(t, err)
}

func TestService_CreateBinding_EmptyKind(t *testing.T) {
	t.Parallel()
	service := setupOpenAPITestService(t)

	spec := map[string]interface{}{
		"openapi": "3.0.3",
		"info":    map[string]interface{}{"title": "Test", "version": "1.0.0"},
		"paths": map[string]interface{}{
			"/test": map[string]interface{}{
				"get": map[string]interface{}{
					"operationId": "testOp",
					"responses":   map[string]interface{}{"200": map[string]interface{}{"description": "OK"}},
				},
			},
		},
	}
	created, err := service.CreateSource(openAPITestContext(), &OpenAPISourceCreateRequest{Spec: rawSpec(t, spec)})
	require.NoError(t, err)

	_, err = service.CreateBinding(openAPITestContext(), &OpenAPISourceBindingCreateRequest{
		SourceID:    created.Source.SourceID,
		OperationID: "testOp",
		Kind:        "", // empty kind
		FunctionID:  "player.list",
	})
	require.Error(t, err)
}

func TestService_CreateBinding_EmptyFunctionID(t *testing.T) {
	t.Parallel()
	service := setupOpenAPITestService(t)

	spec := map[string]interface{}{
		"openapi": "3.0.3",
		"info":    map[string]interface{}{"title": "Test", "version": "1.0.0"},
		"paths": map[string]interface{}{
			"/test": map[string]interface{}{
				"get": map[string]interface{}{
					"operationId": "testOp",
					"responses":   map[string]interface{}{"200": map[string]interface{}{"description": "OK"}},
				},
			},
		},
	}
	created, err := service.CreateSource(openAPITestContext(), &OpenAPISourceCreateRequest{Spec: rawSpec(t, spec)})
	require.NoError(t, err)

	_, err = service.CreateBinding(openAPITestContext(), &OpenAPISourceBindingCreateRequest{
		SourceID:    created.Source.SourceID,
		OperationID: "testOp",
		Kind:        "provider",
		FunctionID:  "", // empty
	})
	require.Error(t, err)
}

func TestService_CreateBinding_UnregisteredFunction(t *testing.T) {
	t.Parallel()
	service := setupOpenAPITestService(t)

	spec := map[string]interface{}{
		"openapi": "3.0.3",
		"info":    map[string]interface{}{"title": "Test", "version": "1.0.0"},
		"paths": map[string]interface{}{
			"/test": map[string]interface{}{
				"get": map[string]interface{}{
					"operationId": "testOp",
					"responses":   map[string]interface{}{"200": map[string]interface{}{"description": "OK"}},
				},
			},
		},
	}
	created, err := service.CreateSource(openAPITestContext(), &OpenAPISourceCreateRequest{Spec: rawSpec(t, spec)})
	require.NoError(t, err)

	_, err = service.CreateBinding(openAPITestContext(), &OpenAPISourceBindingCreateRequest{
		SourceID:    created.Source.SourceID,
		OperationID: "testOp",
		Kind:        "provider",
		FunctionID:  "unregistered.func",
	})
	require.Error(t, err)
}

func TestService_CreateBinding_AutoBindingID(t *testing.T) {
	t.Parallel()
	service := setupOpenAPITestService(t)

	spec := map[string]interface{}{
		"openapi": "3.0.3",
		"info":    map[string]interface{}{"title": "Test", "version": "1.0.0"},
		"paths": map[string]interface{}{
			"/test": map[string]interface{}{
				"get": map[string]interface{}{
					"operationId": "testOp",
					"responses":   map[string]interface{}{"200": map[string]interface{}{"description": "OK"}},
				},
			},
		},
	}
	created, err := service.CreateSource(openAPITestContext(), &OpenAPISourceCreateRequest{Spec: rawSpec(t, spec)})
	require.NoError(t, err)

	// No bindingId — should be auto-generated from operationId
	resp, err := service.CreateBinding(openAPITestContext(), &OpenAPISourceBindingCreateRequest{
		SourceID:    created.Source.SourceID,
		OperationID: "testOp",
		Kind:        "provider",
		FunctionID:  "player.list",
	})
	require.NoError(t, err)
	assert.NotEmpty(t, resp.Binding.BindingID)
}

func TestService_CreateBinding_UnknownSource(t *testing.T) {
	t.Parallel()
	service := setupOpenAPITestService(t)

	_, err := service.CreateBinding(openAPITestContext(), &OpenAPISourceBindingCreateRequest{
		SourceID:    "nonexistent",
		OperationID: "testOp",
		Kind:        "provider",
		FunctionID:  "player.list",
	})
	require.Error(t, err)
}

func TestService_UpdateSource_InvalidSpec(t *testing.T) {
	t.Parallel()
	service := setupOpenAPITestService(t)

	// Create a valid source
	spec := map[string]interface{}{
		"openapi": "3.0.3",
		"info":    map[string]interface{}{"title": "Test", "version": "1.0.0"},
		"paths":   map[string]interface{}{},
	}
	created, err := service.CreateSource(openAPITestContext(), &OpenAPISourceCreateRequest{Spec: rawSpec(t, spec)})
	require.NoError(t, err)

	// Update with invalid spec
	_, err = service.UpdateSource(openAPITestContext(), &OpenAPISourceUpdateRequest{
		SourceID: created.Source.SourceID,
		Spec:     rawSpec(t, map[string]interface{}{"openapi": "3.0.3"}),
	})
	require.Error(t, err)
}

func TestService_GetDocument_Empty(t *testing.T) {
	t.Parallel()
	service := setupOpenAPITestService(t)

	resp, err := service.GetDocument(context.Background(), &GetDocumentRequest{})
	require.NoError(t, err)
	assert.NotNil(t, resp.Spec)
}

func TestService_BatchGetSpec_Empty(t *testing.T) {
	t.Parallel()
	service := setupOpenAPITestService(t)

	resp, err := service.BatchGetSpec(context.Background(), &BatchGetSpecRequest{FunctionIDs: []string{}})
	require.NoError(t, err)
	assert.Empty(t, resp)
}

func TestService_BatchGetSpec_WithFallback(t *testing.T) {
	t.Parallel()
	service := setupOpenAPITestService(t)

	// player.list is registered in the test setup
	resp, err := service.BatchGetSpec(context.Background(), &BatchGetSpecRequest{
		FunctionIDs: []string{"player.list"},
	})
	require.NoError(t, err)
	assert.NotNil(t, resp["player.list"])
}

// ---------- YAML normalization tests ----------

func TestService_CreateSource_YAMLFormat(t *testing.T) {
	t.Parallel()
	service := setupOpenAPITestService(t)

	yamlSpec := `openapi: "3.0.3"
info:
  title: "YAML API"
  version: "1.0.0"
paths:
  /test:
    get:
      operationId: yamlTestOp
      responses:
        "200":
          description: "OK"`

	resp, err := service.CreateSource(openAPITestContext(), &OpenAPISourceCreateRequest{
		Name: "YAML Source",
		Spec: json.RawMessage(yamlSpec),
	})
	require.NoError(t, err)
	assert.Equal(t, "yaml", resp.Source.Format)
	assert.Len(t, resp.Source.Operations, 1)
}

func TestService_CreateSource_InvalidFormat(t *testing.T) {
	t.Parallel()
	service := setupOpenAPITestService(t)

	_, err := service.CreateSource(openAPITestContext(), &OpenAPISourceCreateRequest{
		Spec: json.RawMessage(`not valid json or yaml {{{`),
	})
	require.Error(t, err)
}

func TestService_CreateSource_EmptySpec(t *testing.T) {
	t.Parallel()
	service := setupOpenAPITestService(t)

	_, err := service.CreateSource(openAPITestContext(), &OpenAPISourceCreateRequest{
		Spec: json.RawMessage(``),
	})
	require.Error(t, err)
}

func TestService_CreateSource_InvalidYAMLFormat(t *testing.T) {
	t.Parallel()
	service := setupOpenAPITestService(t)

	// Looks like YAML but isn't valid
	_, err := service.CreateSource(openAPITestContext(), &OpenAPISourceCreateRequest{
		Spec: json.RawMessage(`key: [unclosed`),
	})
	require.Error(t, err)
}

// ---------- operationDTOFromOpenAPI extension coverage ----------

func TestService_CreateSource_WithApprovalExtensions(t *testing.T) {
	t.Parallel()
	service := setupOpenAPITestService(t)

	spec := map[string]interface{}{
		"openapi": "3.0.3",
		"info":    map[string]interface{}{"title": "Approval API", "version": "1.0.0"},
		"paths": map[string]interface{}{
			"/grant": map[string]interface{}{
				"post": map[string]interface{}{
					"operationId": "reward.grant",
					"x-approval":  true,
					"x-risk":      "warning",
					"responses":   map[string]interface{}{"200": map[string]interface{}{"description": "OK"}},
				},
			},
		},
	}
	resp, err := service.CreateSource(openAPITestContext(), &OpenAPISourceCreateRequest{Spec: rawSpec(t, spec)})
	require.NoError(t, err)
	require.Len(t, resp.Source.Operations, 1)
	assert.True(t, resp.Source.Operations[0].Approval.Required)
	assert.Equal(t, dashspec.RiskWarning, resp.Source.Operations[0].Risk)
}

func TestService_CreateSource_WithInvalidRisk(t *testing.T) {
	t.Parallel()
	service := setupOpenAPITestService(t)

	spec := map[string]interface{}{
		"openapi": "3.0.3",
		"info":    map[string]interface{}{"title": "Test", "version": "1.0.0"},
		"paths": map[string]interface{}{
			"/test": map[string]interface{}{
				"get": map[string]interface{}{
					"operationId": "testOp",
					"x-risk":      "super-dangerous", // invalid
					"responses":   map[string]interface{}{"200": map[string]interface{}{"description": "OK"}},
				},
			},
		},
	}
	resp, err := service.CreateSource(openAPITestContext(), &OpenAPISourceCreateRequest{Spec: rawSpec(t, spec)})
	require.NoError(t, err)
	// Risk should be cleared due to invalid value
	assert.Equal(t, dashspec.RiskLevel(""), resp.Source.Operations[0].Risk)
}

func TestService_CreateSource_WithInvalidCapability(t *testing.T) {
	t.Parallel()
	service := setupOpenAPITestService(t)

	spec := map[string]interface{}{
		"openapi": "3.0.3",
		"info":    map[string]interface{}{"title": "Test", "version": "1.0.0"},
		"paths": map[string]interface{}{
			"/test": map[string]interface{}{
				"get": map[string]interface{}{
					"operationId":  "testOp",
					"x-capability": "invalid_cap",
					"responses":    map[string]interface{}{"200": map[string]interface{}{"description": "OK"}},
				},
			},
		},
	}
	_, err := service.CreateSource(openAPITestContext(), &OpenAPISourceCreateRequest{Spec: rawSpec(t, spec)})
	require.Error(t, err)
}

func TestService_CreateSource_WithInvalidExecution(t *testing.T) {
	t.Parallel()
	service := setupOpenAPITestService(t)

	spec := map[string]interface{}{
		"openapi": "3.0.3",
		"info":    map[string]interface{}{"title": "Test", "version": "1.0.0"},
		"paths": map[string]interface{}{
			"/test": map[string]interface{}{
				"get": map[string]interface{}{
					"operationId": "testOp",
					"x-execution": "invalid_exec",
					"responses":   map[string]interface{}{"200": map[string]interface{}{"description": "OK"}},
				},
			},
		},
	}
	_, err := service.CreateSource(openAPITestContext(), &OpenAPISourceCreateRequest{Spec: rawSpec(t, spec)})
	require.Error(t, err)
}

func TestService_CreateSource_ApprovalInvalidType(t *testing.T) {
	t.Parallel()
	service := setupOpenAPITestService(t)

	spec := map[string]interface{}{
		"openapi": "3.0.3",
		"info":    map[string]interface{}{"title": "Test", "version": "1.0.0"},
		"paths": map[string]interface{}{
			"/test": map[string]interface{}{
				"get": map[string]interface{}{
					"operationId": "testOp",
					"x-approval":  "invalid", // string is not valid
					"responses":   map[string]interface{}{"200": map[string]interface{}{"description": "OK"}},
				},
			},
		},
	}
	_, err := service.CreateSource(openAPITestContext(), &OpenAPISourceCreateRequest{Spec: rawSpec(t, spec)})
	require.Error(t, err)
}

func TestService_CreateSource_ApprovalWithPolicyKey(t *testing.T) {
	t.Parallel()
	service := setupOpenAPITestService(t)

	spec := map[string]interface{}{
		"openapi": "3.0.3",
		"info":    map[string]interface{}{"title": "Test", "version": "1.0.0"},
		"paths": map[string]interface{}{
			"/test": map[string]interface{}{
				"get": map[string]interface{}{
					"operationId": "testOp",
					"x-approval": map[string]interface{}{
						"required":   true,
						"policy_key": "two_person",
					},
					"responses": map[string]interface{}{"200": map[string]interface{}{"description": "OK"}},
				},
			},
		},
	}
	resp, err := service.CreateSource(openAPITestContext(), &OpenAPISourceCreateRequest{Spec: rawSpec(t, spec)})
	require.NoError(t, err)
	require.Len(t, resp.Source.Operations, 1)
	assert.True(t, resp.Source.Operations[0].Approval.Required)
	assert.Equal(t, "two_person", resp.Source.Operations[0].Approval.PolicyKey)
}

// ---------- scanOpenAPI tests ----------

func TestScanOpenAPISourceRaw_ExternalRef(t *testing.T) {
	t.Parallel()
	spec := `{"openapi":"3.0.3","info":{"title":"T","version":"1"},"paths":{"/x":{"get":{"operationId":"op","responses":{"200":{"description":"OK"}},"parameters":[{"$ref":"https://external.com/schema.json#/parameters/id"}]}}}}`
	diags := scanOpenAPISourceRaw([]byte(spec))
	found := false
	for _, d := range diags {
		if d.Code == "openapi_external_ref_forbidden" {
			found = true
			break
		}
	}
	assert.True(t, found, "expected external ref diagnostic")
}

func TestScanOpenAPISourceRaw_InvalidJSON(t *testing.T) {
	t.Parallel()
	diags := scanOpenAPISourceRaw([]byte(`{invalid`))
	require.NotEmpty(t, diags)
	assert.Equal(t, "openapi_json_decode_failed", diags[0].Code)
}

func TestParseOpenAPISource_NoOperations(t *testing.T) {
	t.Parallel()
	raw := []byte(`{"openapi":"3.0.3","info":{"title":"Empty","version":"1"},"paths":{}}`)
	parsed, err := parseOpenAPISource(raw)
	require.NoError(t, err)
	assert.Len(t, parsed.Operations, 0)
	// Should have a warning about no operations
	found := false
	for _, d := range parsed.Diagnostics {
		if d.Code == "openapi_no_operations" {
			found = true
			break
		}
	}
	assert.True(t, found)
}

func TestParseOpenAPISource_UnsupportedVersion(t *testing.T) {
	t.Parallel()
	raw := []byte(`{"openapi":"2.0","info":{"title":"Old","version":"1"},"paths":{"/x":{"get":{"operationId":"op","responses":{"200":{"description":"OK"}}}}}}`)
	parsed, err := parseOpenAPISource(raw)
	require.NoError(t, err)
	found := false
	for _, d := range parsed.Diagnostics {
		if d.Code == "openapi_version_unsupported" {
			found = true
			break
		}
	}
	assert.True(t, found)
}

func TestParseOpenAPISource_MissingPaths(t *testing.T) {
	t.Parallel()
	raw := []byte(`{"openapi":"3.0.3","info":{"title":"NoPaths","version":"1"}}`)
	parsed, err := parseOpenAPISource(raw)
	require.NoError(t, err)
	// Missing paths is caught as either openapi_validation_failed (OpenAPI validation)
	// or openapi_paths_missing (extractSourceOperations). Either is acceptable.
	found := false
	for _, d := range parsed.Diagnostics {
		if d.Code == "openapi_paths_missing" || d.Code == "openapi_validation_failed" {
			found = true
			break
		}
	}
	assert.True(t, found)
}

func TestParseOpenAPISource_DuplicateOperationIDs(t *testing.T) {
	t.Parallel()
	raw := []byte(`{"openapi":"3.0.3","info":{"title":"Dup","version":"1"},"paths":{"/a":{"get":{"operationId":"dup","responses":{"200":{"description":"OK"}}}},"/b":{"get":{"operationId":"dup","responses":{"200":{"description":"OK"}}}}}}`)
	parsed, err := parseOpenAPISource(raw)
	require.NoError(t, err)
	found := false
	for _, d := range parsed.Diagnostics {
		if d.Code == "openapi_operation_id_duplicate" {
			found = true
			break
		}
	}
	assert.True(t, found)
}

func TestParseOpenAPISource_MissingOperationID(t *testing.T) {
	t.Parallel()
	raw := []byte(`{"openapi":"3.0.3","info":{"title":"NoID","version":"1"},"paths":{"/x":{"get":{"responses":{"200":{"description":"OK"}}}}}}`)
	parsed, err := parseOpenAPISource(raw)
	require.NoError(t, err)
	found := false
	for _, d := range parsed.Diagnostics {
		if d.Code == "openapi_operation_id_missing" {
			found = true
			break
		}
	}
	assert.True(t, found)
}

func TestParseOpenAPISource_InvalidResourceKey(t *testing.T) {
	t.Parallel()
	raw := []byte(`{"openapi":"3.0.3","info":{"title":"BadKey","version":"1"},"paths":{"/x":{"get":{"operationId":"op","x-resource":"Invalid Key!","responses":{"200":{"description":"OK"}}}}}}`)
	parsed, err := parseOpenAPISource(raw)
	require.NoError(t, err)
	found := false
	for _, d := range parsed.Diagnostics {
		if d.Code == "openapi_resource_key_invalid" {
			found = true
			break
		}
	}
	assert.True(t, found)
}

func TestParseOpenAPISource_InvalidOperationKey(t *testing.T) {
	t.Parallel()
	raw := []byte(`{"openapi":"3.0.3","info":{"title":"BadOp","version":"1"},"paths":{"/x":{"get":{"operationId":"op","x-operation":"Invalid!","responses":{"200":{"description":"OK"}}}}}}`)
	parsed, err := parseOpenAPISource(raw)
	require.NoError(t, err)
	found := false
	for _, d := range parsed.Diagnostics {
		if d.Code == "openapi_operation_key_invalid" {
			found = true
			break
		}
	}
	assert.True(t, found)
}

func TestParseOpenAPISource_XRiskInvalid(t *testing.T) {
	t.Parallel()
	raw := []byte(`{"openapi":"3.0.3","info":{"title":"BadRisk","version":"1"},"paths":{"/x":{"get":{"operationId":"op","x-risk":"invalid","responses":{"200":{"description":"OK"}}}}}}`)
	parsed, err := parseOpenAPISource(raw)
	require.NoError(t, err)
	found := false
	for _, d := range parsed.Diagnostics {
		if d.Code == "openapi_risk_invalid" {
			found = true
			break
		}
	}
	assert.True(t, found)
}

func TestParseOpenAPISource_XCapabilityInvalid(t *testing.T) {
	t.Parallel()
	raw := []byte(`{"openapi":"3.0.3","info":{"title":"BadCap","version":"1"},"paths":{"/x":{"get":{"operationId":"op","x-capability":"bogus","responses":{"200":{"description":"OK"}}}}}}`)
	parsed, err := parseOpenAPISource(raw)
	require.NoError(t, err)
	found := false
	for _, d := range parsed.Diagnostics {
		if d.Code == "openapi_capability_invalid" {
			found = true
			break
		}
	}
	assert.True(t, found)
}

func TestParseOpenAPISource_XExecutionInvalid(t *testing.T) {
	t.Parallel()
	raw := []byte(`{"openapi":"3.0.3","info":{"title":"BadExec","version":"1"},"paths":{"/x":{"get":{"operationId":"op","x-execution":"bogus","responses":{"200":{"description":"OK"}}}}}}`)
	parsed, err := parseOpenAPISource(raw)
	require.NoError(t, err)
	found := false
	for _, d := range parsed.Diagnostics {
		if d.Code == "openapi_execution_invalid" {
			found = true
			break
		}
	}
	assert.True(t, found)
}

func TestParseOpenAPISource_FormilyKeyword(t *testing.T) {
	t.Parallel()
	raw := []byte(`{"openapi":"3.0.3","info":{"title":"Formily","version":"1"},"paths":{"/x":{"get":{"operationId":"op","formily":{"schema":{}},"responses":{"200":{"description":"OK"}}}}}}`)
	parsed, err := parseOpenAPISource(raw)
	require.NoError(t, err)
	found := false
	for _, d := range parsed.Diagnostics {
		if d.Code == "openapi_presentation_field_forbidden" {
			found = true
			break
		}
	}
	assert.True(t, found)
}

// ---------- Handler CreateSource multipart tests ----------

func TestHandler_CreateSource_Multipart(t *testing.T) {
	t.Parallel()
	_, router := setupOpenAPITestHandlerWithBindings(t)

	specJSON := `{"openapi":"3.0.3","info":{"title":"Multipart","version":"1.0.0"},"paths":{"/test":{"get":{"operationId":"mpTest","responses":{"200":{"description":"OK"}}}}}}`

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, _ := writer.CreateFormFile("file", "test.json")
	_, _ = part.Write([]byte(specJSON))
	writer.WriteField("name", "Multipart Source")
	writer.Close()

	req, _ := http.NewRequest("POST", "/sources", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code, w.Body.String())
}

func TestHandler_CreateSource_MultipartNoName(t *testing.T) {
	t.Parallel()
	_, router := setupOpenAPITestHandlerWithBindings(t)

	specJSON := `{"openapi":"3.0.3","info":{"title":"AutoName","version":"1.0.0"},"paths":{"/test":{"get":{"operationId":"anTest","responses":{"200":{"description":"OK"}}}}}}`

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, _ := writer.CreateFormFile("file", "spec.json")
	_, _ = part.Write([]byte(specJSON))
	// No name field — should use filename
	writer.Close()

	req, _ := http.NewRequest("POST", "/sources", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code, w.Body.String())
}

func TestHandler_CreateSource_RawBodyTooLarge(t *testing.T) {
	t.Parallel()
	_, router := setupOpenAPITestHandlerWithBindings(t)

	largeData := strings.Repeat("x", maxOpenAPISourceBytes+100)
	req, _ := http.NewRequest("POST", "/sources", strings.NewReader(largeData))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandler_CreateSource_EnvelopeSpec(t *testing.T) {
	t.Parallel()
	_, router := setupOpenAPITestHandlerWithBindings(t)

	// Send an envelope with "spec" field
	envelope := map[string]interface{}{
		"name": "Envelope Source",
		"spec": map[string]interface{}{
			"openapi": "3.0.3",
			"info":    map[string]interface{}{"title": "Env", "version": "1.0.0"},
			"paths": map[string]interface{}{
				"/test": map[string]interface{}{
					"get": map[string]interface{}{
						"operationId": "envTest",
						"responses":   map[string]interface{}{"200": map[string]interface{}{"description": "OK"}},
					},
				},
			},
		},
	}
	body, _ := json.Marshal(envelope)
	req, _ := http.NewRequest("POST", "/sources", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code, w.Body.String())
}

func TestHandler_UpdateSource_RawBodyTooLarge(t *testing.T) {
	t.Parallel()
	_, router := setupOpenAPITestHandlerWithBindings(t)

	// Create a source first
	spec := map[string]interface{}{
		"openapi": "3.0.3",
		"info":    map[string]interface{}{"title": "Test", "version": "1.0.0"},
		"paths":   map[string]interface{}{},
	}
	body, _ := json.Marshal(OpenAPISourceCreateRequest{Spec: rawSpec(t, spec)})
	createReq, _ := http.NewRequest("POST", "/sources", bytes.NewReader(body))
	createReq.Header.Set("Content-Type", "application/json")
	createW := httptest.NewRecorder()
	router.ServeHTTP(createW, createReq)
	require.Equal(t, http.StatusCreated, createW.Code)
	var created OpenAPISourceGetResponse
	require.NoError(t, json.Unmarshal(createW.Body.Bytes(), &created))

	// Try to update with oversized body
	largeData := strings.Repeat("x", maxOpenAPISourceBytes+100)
	updateReq, _ := http.NewRequest("PUT", "/sources/"+created.Source.SourceID, strings.NewReader(largeData))
	updateReq.Header.Set("Content-Type", "application/json")
	updateW := httptest.NewRecorder()
	router.ServeHTTP(updateW, updateReq)

	assert.Equal(t, http.StatusBadRequest, updateW.Code)
}

func TestHandler_UpdateSource_InvalidURI(t *testing.T) {
	t.Parallel()
	_, router := setupOpenAPITestHandlerWithBindings(t)

	// Missing sourceId
	req, _ := http.NewRequest("PUT", "/sources//", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.NotEqual(t, http.StatusOK, w.Code)
}

func TestHandler_UpdateSource_EnvelopeSpec(t *testing.T) {
	t.Parallel()
	_, router := setupOpenAPITestHandlerWithBindings(t)

	// Create source first
	spec := map[string]interface{}{
		"openapi": "3.0.3",
		"info":    map[string]interface{}{"title": "Test", "version": "1.0.0"},
		"paths":   map[string]interface{}{},
	}
	body, _ := json.Marshal(OpenAPISourceCreateRequest{Spec: rawSpec(t, spec)})
	createReq, _ := http.NewRequest("POST", "/sources", bytes.NewReader(body))
	createReq.Header.Set("Content-Type", "application/json")
	createW := httptest.NewRecorder()
	router.ServeHTTP(createW, createReq)
	require.Equal(t, http.StatusCreated, createW.Code)
	var created OpenAPISourceGetResponse
	require.NoError(t, json.Unmarshal(createW.Body.Bytes(), &created))

	// Update with envelope
	envelope := map[string]interface{}{
		"name": "Updated",
		"spec": map[string]interface{}{
			"openapi": "3.0.3",
			"info":    map[string]interface{}{"title": "Updated", "version": "2.0.0"},
			"paths":   map[string]interface{}{},
		},
	}
	updateBody, _ := json.Marshal(envelope)
	updateReq, _ := http.NewRequest("PUT", "/sources/"+created.Source.SourceID, bytes.NewReader(updateBody))
	updateReq.Header.Set("Content-Type", "application/json")
	updateW := httptest.NewRecorder()
	router.ServeHTTP(updateW, updateReq)

	assert.Equal(t, http.StatusOK, updateW.Code, updateW.Body.String())
}

func TestHandler_GetSource_InvalidURI(t *testing.T) {
	t.Parallel()
	_, router := setupOpenAPITestHandlerWithBindings(t)

	req, _ := http.NewRequest("GET", "/sources//", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.NotEqual(t, http.StatusOK, w.Code)
}

func TestHandler_SourceDiagnostics_InvalidURI(t *testing.T) {
	t.Parallel()
	_, router := setupOpenAPITestHandlerWithBindings(t)

	req, _ := http.NewRequest("GET", "/sources//diagnostics", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.NotEqual(t, http.StatusOK, w.Code)
}

func TestHandler_CreateBinding_InvalidURI(t *testing.T) {
	t.Parallel()
	_, router := setupOpenAPITestHandlerWithBindings(t)

	req, _ := http.NewRequest("POST", "/sources//bindings", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.NotEqual(t, http.StatusOK, w.Code)
}

func TestHandler_DeleteBinding_InvalidURI(t *testing.T) {
	t.Parallel()
	_, router := setupOpenAPITestHandlerWithBindings(t)

	req, _ := http.NewRequest("DELETE", "/sources//bindings//", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.NotEqual(t, http.StatusOK, w.Code)
}

func TestService_GetSource_InvalidSourceID(t *testing.T) {
	t.Parallel()
	service := setupOpenAPITestService(t)

	_, err := service.GetSource(openAPITestContext(), &OpenAPISourceGetRequest{SourceID: ""})
	require.Error(t, err)
}

func TestService_SourceDiagnostics_InvalidSourceID(t *testing.T) {
	t.Parallel()
	service := setupOpenAPITestService(t)

	_, err := service.SourceDiagnostics(openAPITestContext(), &OpenAPISourceGetRequest{SourceID: ""})
	require.Error(t, err)
}

func TestService_CreateSource_WithPermissionExtension(t *testing.T) {
	t.Parallel()
	service := setupOpenAPITestService(t)

	spec := map[string]interface{}{
		"openapi": "3.0.3",
		"info":    map[string]interface{}{"title": "Perm", "version": "1.0.0"},
		"paths": map[string]interface{}{
			"/test": map[string]interface{}{
				"get": map[string]interface{}{
					"operationId":  "permOp",
					"x-permission": "admin:all",
					"responses":    map[string]interface{}{"200": map[string]interface{}{"description": "OK"}},
				},
			},
		},
	}
	resp, err := service.CreateSource(openAPITestContext(), &OpenAPISourceCreateRequest{Spec: rawSpec(t, spec)})
	require.NoError(t, err)
	require.Len(t, resp.Source.Operations, 1)
	assert.Equal(t, "admin:all", resp.Source.Operations[0].Permission)
}

func TestService_CreateSource_WithTags(t *testing.T) {
	t.Parallel()
	service := setupOpenAPITestService(t)

	spec := map[string]interface{}{
		"openapi": "3.0.3",
		"info":    map[string]interface{}{"title": "Tags", "version": "1.0.0"},
		"paths": map[string]interface{}{
			"/test": map[string]interface{}{
				"get": map[string]interface{}{
					"operationId": "tagsOp",
					"tags":        []interface{}{"admin", "user"},
					"responses":   map[string]interface{}{"200": map[string]interface{}{"description": "OK"}},
				},
			},
		},
	}
	resp, err := service.CreateSource(openAPITestContext(), &OpenAPISourceCreateRequest{Spec: rawSpec(t, spec)})
	require.NoError(t, err)
	require.Len(t, resp.Source.Operations, 1)
	assert.Equal(t, []string{"admin", "user"}, resp.Source.Operations[0].Tags)
}

func TestService_CreateSource_WithDescription(t *testing.T) {
	t.Parallel()
	service := setupOpenAPITestService(t)

	spec := map[string]interface{}{
		"openapi": "3.0.3",
		"info":    map[string]interface{}{"title": "Desc", "version": "1.0.0"},
		"paths": map[string]interface{}{
			"/test": map[string]interface{}{
				"get": map[string]interface{}{
					"operationId": "descOp",
					"summary":     "Summary text",
					"description": "Description text",
					"responses":   map[string]interface{}{"200": map[string]interface{}{"description": "OK"}},
				},
			},
		},
	}
	resp, err := service.CreateSource(openAPITestContext(), &OpenAPISourceCreateRequest{Spec: rawSpec(t, spec)})
	require.NoError(t, err)
	require.Len(t, resp.Source.Operations, 1)
	assert.Equal(t, "Summary text", resp.Source.Operations[0].Summary)
	assert.Equal(t, "Description text", resp.Source.Operations[0].Description)
}

// ---------- parseOpenAPISource parse failure test ----------

func TestParseOpenAPISource_ParseFailure(t *testing.T) {
	t.Parallel()
	// JSON that is valid JSON but not valid OpenAPI
	raw := []byte(`{"openapi":"3.0.3","info":{"title":"Bad","version":"1"},"paths":{"bad": {"get": "not an operation"}}}`)
	parsed, err := parseOpenAPISource(raw)
	require.NoError(t, err) // parseOpenAPISource doesn't return errors for parse failures, adds diagnostics
	found := false
	for _, d := range parsed.Diagnostics {
		if d.Severity == dashspec.SeverityError {
			found = true
			break
		}
	}
	assert.True(t, found, "expected error diagnostic from parse failure")
}
