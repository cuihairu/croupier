package schema

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---- Service Tests ----

func TestServiceV2_Get_EmptyID(t *testing.T) {
	svcCtx := setupSvcCtx(t)
	s := NewService(svcCtx)
	_, err := s.Get(context.Background(), &GetRequest{ID: ""})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "schema ID 不能为空")
}

func TestServiceV2_Delete_EmptyID(t *testing.T) {
	svcCtx := setupSvcCtx(t)
	s := NewService(svcCtx)
	err := s.Delete(context.Background(), &DeleteRequest{ID: ""})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "schema ID 不能为空")
}

func TestServiceV2_Validate_EmptyID(t *testing.T) {
	svcCtx := setupSvcCtx(t)
	s := NewService(svcCtx)
	_, err := s.Validate(context.Background(), &ValidateRequest{ID: "", Data: map[string]interface{}{}})
	assert.Error(t, err)
}

func TestServiceV2_Validate_InvalidPayload(t *testing.T) {
	svcCtx := setupSvcCtx(t)
	s := NewService(svcCtx)
	createResp, err := s.Create(context.Background(), &CreateRequest{
		Name:   "Validate Schema",
		Schema: mustParseSchema(t),
	})
	require.NoError(t, err)

	resp, err := s.Validate(context.Background(), &ValidateRequest{
		ID:   createResp.ID,
		Data: map[string]interface{}{"age": 30}, // missing required "name"
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.False(t, resp.Valid)
	assert.NotEmpty(t, resp.Errors)
}

func TestServiceV2_RawValidate_InvalidSchema(t *testing.T) {
	svcCtx := setupSvcCtx(t)
	s := NewService(svcCtx)
	_, err := s.RawValidate(context.Background(), &RawValidateRequest{
		Schema: map[string]interface{}{"required": "name"}, // invalid schema definition
		Data:   map[string]interface{}{"name": "bob"},
	})
	assert.Error(t, err)
}

func TestServiceV2_RawValidate_InvalidPayload(t *testing.T) {
	svcCtx := setupSvcCtx(t)
	s := NewService(svcCtx)
	resp, err := s.RawValidate(context.Background(), &RawValidateRequest{
		Schema: mustParseSchema(t),
		Data:   map[string]interface{}{"age": 30}, // missing required "name"
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.False(t, resp.Valid)
	assert.NotEmpty(t, resp.Errors)
}

func TestServiceV2_GetUIConfig(t *testing.T) {
	svcCtx := setupSvcCtx(t)
	s := NewService(svcCtx)
	createResp, err := s.Create(context.Background(), &CreateRequest{
		Name:   "UI Schema",
		Schema: mustParseSchema(t),
	})
	require.NoError(t, err)

	resp, err := s.GetUIConfig(context.Background(), &GetUIConfigRequest{ID: createResp.ID})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, createResp.ID, resp.ID)
}

func TestServiceV2_GetUIConfig_NotFound(t *testing.T) {
	svcCtx := setupSvcCtx(t)
	s := NewService(svcCtx)
	_, err := s.GetUIConfig(context.Background(), &GetUIConfigRequest{ID: "nonexistent"})
	assert.Error(t, err)
}

func TestServiceV2_UpdateUIConfig(t *testing.T) {
	svcCtx := setupSvcCtx(t)
	s := NewService(svcCtx)
	createResp, err := s.Create(context.Background(), &CreateRequest{
		Name:   "UI Update Schema",
		Schema: mustParseSchema(t),
	})
	require.NoError(t, err)

	uiConfig := map[string]interface{}{
		"fields": []interface{}{
			map[string]interface{}{"key": "name", "type": "text", "label": "Name"},
		},
	}
	resp, err := s.UpdateUIConfig(context.Background(), &UpdateUIConfigRequest{
		ID:     createResp.ID,
		Config: uiConfig,
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, createResp.ID, resp.ID)
	assert.NotNil(t, resp.UIConfig)
}

func TestServiceV2_UpdateUIConfig_NotFound(t *testing.T) {
	svcCtx := setupSvcCtx(t)
	s := NewService(svcCtx)
	_, err := s.UpdateUIConfig(context.Background(), &UpdateUIConfigRequest{
		ID:     "nonexistent",
		Config: map[string]interface{}{},
	})
	assert.Error(t, err)
}

func TestServiceV2_Create_MultipleSchemas(t *testing.T) {
	svcCtx := setupSvcCtx(t)
	s := NewService(svcCtx)
	for i := 0; i < 5; i++ {
		_, err := s.Create(context.Background(), &CreateRequest{
			Name:   fmt.Sprintf("Schema %d", i),
			Schema: mustParseSchema(t),
		})
		require.NoError(t, err)
	}

	resp, err := s.List(context.Background(), &ListRequest{})
	require.NoError(t, err)
	assert.Equal(t, 5, resp.Total)
	assert.Len(t, resp.Items, 5)
}

func TestServiceV2_Update_InvalidSchema(t *testing.T) {
	svcCtx := setupSvcCtx(t)
	s := NewService(svcCtx)
	createResp, err := s.Create(context.Background(), &CreateRequest{
		Name:   "Update Invalid",
		Schema: mustParseSchema(t),
	})
	require.NoError(t, err)

	_, err = s.Update(context.Background(), &UpdateRequest{
		ID:     createResp.ID,
		Schema: map[string]interface{}{"required": "name"}, // invalid
	})
	assert.Error(t, err)
}

func TestServiceV2_Update_NotFound(t *testing.T) {
	svcCtx := setupSvcCtx(t)
	s := NewService(svcCtx)
	_, err := s.Update(context.Background(), &UpdateRequest{
		ID:     "nonexistent",
		Schema: mustParseSchema(t),
	})
	assert.Error(t, err)
}

// ---- Handler Tests ----

func newRouterV2(h *Handler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/schemas", h.List)
	r.POST("/schemas", h.Create)
	r.GET("/schemas/:id", h.Get)
	r.PUT("/schemas/:id", h.Update)
	r.DELETE("/schemas/:id", h.Delete)
	r.POST("/schemas/:id/validate", h.Validate)
	r.POST("/schemas/raw-validate", h.RawValidate)
	r.GET("/schemas/:id/ui-config", h.GetUIConfig)
	r.PUT("/schemas/:id/ui-config", h.UpdateUIConfig)
	return r
}

func doReqV2(t *testing.T, router *gin.Engine, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestHandlerV2_List_Empty(t *testing.T) {
	handler := NewHandler(NewService(setupSvcCtx(t)))
	router := newRouterV2(handler)
	rec := doReqV2(t, router, http.MethodGet, "/schemas", "")
	assertStatus(t, rec, http.StatusOK)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	items := resp["items"].([]interface{})
	assert.Empty(t, items)
}

func TestHandlerV2_Create_InvalidJSON(t *testing.T) {
	handler := NewHandler(NewService(setupSvcCtx(t)))
	router := newRouterV2(handler)
	rec := doReqV2(t, router, http.MethodPost, "/schemas", `{broken`)
	assertStatus(t, rec, http.StatusBadRequest)
	assertErrorShape(t, rec)
}

func TestHandlerV2_Create_EmptyName(t *testing.T) {
	handler := NewHandler(NewService(setupSvcCtx(t)))
	router := newRouterV2(handler)
	rec := doReqV2(t, router, http.MethodPost, "/schemas",
		`{"name":"  ","schema":`+validSchemaJSON+`}`)
	assert.NotEqual(t, http.StatusOK, rec.Code)
	assertErrorShape(t, rec)
}

func TestHandlerV2_Get_NotFound(t *testing.T) {
	handler := NewHandler(NewService(setupSvcCtx(t)))
	router := newRouterV2(handler)
	rec := doReqV2(t, router, http.MethodGet, "/schemas/missing", "")
	assertStatus(t, rec, http.StatusNotFound)
	assertErrorShape(t, rec)
}

func TestHandlerV2_Update_Success(t *testing.T) {
	handler := NewHandler(NewService(setupSvcCtx(t)))
	router := newRouterV2(handler)

	// Create
	rec := doReqV2(t, router, http.MethodPost, "/schemas",
		`{"name":"ToUpdate","schema":`+validSchemaJSON+`}`)
	assertStatus(t, rec, http.StatusOK)
	var created map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))
	id := created["id"].(string)

	// Update
	newSchema := `{"type":"object","properties":{"age":{"type":"integer"}}}`
	rec2 := doReqV2(t, router, http.MethodPut, "/schemas/"+id,
		`{"schema":`+newSchema+`}`)
	assertStatus(t, rec2, http.StatusOK)
	var updated map[string]interface{}
	require.NoError(t, json.Unmarshal(rec2.Body.Bytes(), &updated))
	assert.Equal(t, id, updated["id"])
}

func TestHandlerV2_Delete_Success(t *testing.T) {
	handler := NewHandler(NewService(setupSvcCtx(t)))
	router := newRouterV2(handler)

	// Create
	rec := doReqV2(t, router, http.MethodPost, "/schemas",
		`{"name":"ToDelete","schema":`+validSchemaJSON+`}`)
	assertStatus(t, rec, http.StatusOK)
	var created map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))
	id := created["id"].(string)

	// Delete
	rec2 := doReqV2(t, router, http.MethodDelete, "/schemas/"+id, "")
	assertStatus(t, rec2, http.StatusOK)

	// Verify deleted
	rec3 := doReqV2(t, router, http.MethodGet, "/schemas/"+id, "")
	assertStatus(t, rec3, http.StatusNotFound)
}

func TestHandlerV2_RawValidate_Success(t *testing.T) {
	handler := NewHandler(NewService(setupSvcCtx(t)))
	router := newRouterV2(handler)
	rec := doReqV2(t, router, http.MethodPost, "/schemas/raw-validate",
		`{"schema":`+validSchemaJSON+`,"data":{"name":"alice"}}`)
	assertStatus(t, rec, http.StatusOK)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.True(t, resp["valid"].(bool))
}

func TestHandlerV2_RawValidate_InvalidJSON(t *testing.T) {
	handler := NewHandler(NewService(setupSvcCtx(t)))
	router := newRouterV2(handler)
	rec := doReqV2(t, router, http.MethodPost, "/schemas/raw-validate", `{bad`)
	assertStatus(t, rec, http.StatusBadRequest)
	assertErrorShape(t, rec)
}

func TestHandlerV2_Validate_InvalidPayload(t *testing.T) {
	handler := NewHandler(NewService(setupSvcCtx(t)))
	router := newRouterV2(handler)

	// Create schema
	rec := doReqV2(t, router, http.MethodPost, "/schemas",
		`{"name":"ValSchema","schema":`+validSchemaJSON+`}`)
	assertStatus(t, rec, http.StatusOK)
	var created map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))
	id := created["id"].(string)

	// Validate with missing required field
	rec2 := doReqV2(t, router, http.MethodPost, "/schemas/"+id+"/validate",
		`{"data":{"age":30}}`)
	assertStatus(t, rec2, http.StatusOK)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(rec2.Body.Bytes(), &resp))
	assert.False(t, resp["valid"].(bool))
}

func TestHandlerV2_GetUIConfig_NotFound(t *testing.T) {
	handler := NewHandler(NewService(setupSvcCtx(t)))
	router := newRouterV2(handler)
	rec := doReqV2(t, router, http.MethodGet, "/schemas/missing/ui-config", "")
	assertStatus(t, rec, http.StatusNotFound)
	assertErrorShape(t, rec)
}

func TestHandlerV2_UpdateUIConfig_Success(t *testing.T) {
	handler := NewHandler(NewService(setupSvcCtx(t)))
	router := newRouterV2(handler)

	// Create
	rec := doReqV2(t, router, http.MethodPost, "/schemas",
		`{"name":"UICfg","schema":`+validSchemaJSON+`}`)
	assertStatus(t, rec, http.StatusOK)
	var created map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))
	id := created["id"].(string)

	// Update UI config
	rec2 := doReqV2(t, router, http.MethodPut, "/schemas/"+id+"/ui-config",
		`{"config":{"theme":"dark"}}`)
	assertStatus(t, rec2, http.StatusOK)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(rec2.Body.Bytes(), &resp))
	assert.Equal(t, id, resp["id"])
}

func TestHandlerV2_UpdateUIConfig_NotFound(t *testing.T) {
	handler := NewHandler(NewService(setupSvcCtx(t)))
	router := newRouterV2(handler)
	rec := doReqV2(t, router, http.MethodPut, "/schemas/missing/ui-config",
		`{"config":{}}`)
	assertStatus(t, rec, http.StatusNotFound)
	assertErrorShape(t, rec)
}

// ---- Helpers tests ----

func TestResolveSchemasDir_EmptyConfig(t *testing.T) {
	dir := resolveSchemasDir(config.Config{})
	assert.Contains(t, dir, "schemas")
	assert.Contains(t, dir, "custom")
}

func TestResolveSchemasDir_CustomDir(t *testing.T) {
	dir := resolveSchemasDir(config.Config{
		Schemas: config.SchemasConfig{Dir: "/tmp/myschemas"},
	})
	assert.Contains(t, dir, "myschemas")
	assert.Contains(t, dir, "custom")
}

func TestGenerateSchemaID_Simple(t *testing.T) {
	id := generateSchemaID("Player Schema")
	assert.Equal(t, "player-schema", id)
}

func TestGenerateSchemaID_SpecialChars(t *testing.T) {
	id := generateSchemaID("Hello World! @#$")
	assert.Equal(t, "hello-world", id)
}

func TestGenerateSchemaID_Empty(t *testing.T) {
	id := generateSchemaID("   ")
	// Empty slug falls back to UUID
	assert.NotEmpty(t, id)
	assert.NotContains(t, id, " ")
}

func TestValidateSchemaDefinition_Valid(t *testing.T) {
	err := validateSchemaDefinition(mustParseSchema(t))
	assert.NoError(t, err)
}

func TestValidateSchemaDefinition_Invalid(t *testing.T) {
	err := validateSchemaDefinition(map[string]interface{}{"required": "name"})
	assert.Error(t, err)
}

func TestSchemaDocFromFile_ZeroDates(t *testing.T) {
	file := schemaFileModel{
		ID:        "test",
		Name:      "Test",
		Schema:    map[string]interface{}{},
		CreatedAt: "",
		UpdatedAt: "",
	}
	doc := schemaDocFromFile(file)
	assert.Equal(t, "test", doc.ID)
	assert.False(t, doc.CreatedAt.IsZero())
	// updatedAt defaults to createdAt when empty
	assert.False(t, doc.UpdatedAt.IsZero())
}

func TestSchemaDocFromFile_PartialDates(t *testing.T) {
	file := schemaFileModel{
		ID:        "partial",
		Name:      "Partial",
		Schema:    map[string]interface{}{},
		CreatedAt: "2025-01-01T00:00:00Z",
		UpdatedAt: "",
	}
	doc := schemaDocFromFile(file)
	assert.Equal(t, "partial", doc.ID)
	// updatedAt defaults to createdAt
	assert.Equal(t, doc.CreatedAt, doc.UpdatedAt)
}
