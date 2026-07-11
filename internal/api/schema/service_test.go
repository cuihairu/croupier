package schema

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const validSchemaJSON = `{"type":"object","properties":{"name":{"type":"string"}},"required":["name"]}`

// newSvcCtx builds a ServiceContext whose schema directory points at a fresh
// temp directory (auto-cleaned by the testing runtime).
func setupSvcCtx(t *testing.T) *svc.ServiceContext {
	t.Helper()
	return &svc.ServiceContext{
		Config: config.Config{
			Schemas: config.SchemasConfig{Dir: t.TempDir()},
		},
	}
}

func newTestContext(method, target, body string) (*gin.Context, *httptest.ResponseRecorder) {
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(method, target, strings.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	return ctx, rec
}

func assertStatus(t *testing.T, rec *httptest.ResponseRecorder, want int) {
	t.Helper()
	if rec.Code != want {
		t.Fatalf("expected status %d, got %d body=%s", want, rec.Code, rec.Body.String())
	}
}

func assertErrorShape(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	body := map[string]interface{}{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body), "body not JSON object: %s", rec.Body.String())
	errCode, _ := body["error"].(string)
	require.NotEmpty(t, errCode, "missing 'error' field, body=%s", rec.Body.String())
	msg, _ := body["message"].(string)
	require.NotEmpty(t, msg, "missing 'message' field, body=%s", rec.Body.String())
}

func mustParseSchema(t *testing.T) interface{} {
	t.Helper()
	var s interface{}
	require.NoError(t, json.Unmarshal([]byte(validSchemaJSON), &s))
	return s
}

func TestService_List_Empty(t *testing.T) {
	svcCtx := setupSvcCtx(t)
	svc := NewService(svcCtx)

	resp, err := svc.List(context.Background(), &ListRequest{})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Empty(t, resp.Items)
	assert.Equal(t, 0, resp.Total)
}

func TestService_CreateGet_RoundTrip(t *testing.T) {
	svcCtx := setupSvcCtx(t)
	svc := NewService(svcCtx)

	createResp, err := svc.Create(context.Background(), &CreateRequest{
		Name:   "Player Schema",
		Schema: mustParseSchema(t),
	})
	require.NoError(t, err)
	require.NotNil(t, createResp)
	assert.Equal(t, "Player Schema", createResp.Name)
	assert.NotEmpty(t, createResp.ID)

	getResp, err := svc.Get(context.Background(), &GetRequest{ID: createResp.ID})
	require.NoError(t, err)
	require.NotNil(t, getResp)
	assert.Equal(t, createResp.ID, getResp.ID)
	assert.Equal(t, "Player Schema", getResp.Name)
}

func TestService_Create_EmptyName(t *testing.T) {
	svcCtx := setupSvcCtx(t)
	svc := NewService(svcCtx)

	resp, err := svc.Create(context.Background(), &CreateRequest{
		Name:   "  ",
		Schema: mustParseSchema(t),
	})
	require.Error(t, err)
	assert.Nil(t, resp)
	var codeErr *errorx.CodeError
	require.ErrorAs(t, err, &codeErr)
	assert.Equal(t, http.StatusBadRequest, codeErr.Code)
}

func TestService_Create_InvalidSchema(t *testing.T) {
	svcCtx := setupSvcCtx(t)
	svc := NewService(svcCtx)

	// "required" must be an array; a string value is rejected by gojsonschema.
	resp, err := svc.Create(context.Background(), &CreateRequest{
		Name:   "Bad",
		Schema: map[string]interface{}{"required": "name"},
	})
	require.Error(t, err)
	assert.Nil(t, resp)
}

func TestService_Get_NotFound(t *testing.T) {
	svcCtx := setupSvcCtx(t)
	svc := NewService(svcCtx)

	resp, err := svc.Get(context.Background(), &GetRequest{ID: "missing"})
	require.Error(t, err)
	assert.Nil(t, resp)
}

func TestService_Update_RoundTrip(t *testing.T) {
	svcCtx := setupSvcCtx(t)
	svc := NewService(svcCtx)

	createResp, err := svc.Create(context.Background(), &CreateRequest{
		Name: "Up", Schema: mustParseSchema(t),
	})
	require.NoError(t, err)

	updateResp, err := svc.Update(context.Background(), &UpdateRequest{
		ID: createResp.ID,
		Schema: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{"age": map[string]interface{}{"type": "integer"}},
		},
	})
	require.NoError(t, err)
	assert.NotNil(t, updateResp)
	assert.Equal(t, createResp.ID, updateResp.ID)
}

func TestService_Delete_NotFound(t *testing.T) {
	svcCtx := setupSvcCtx(t)
	svc := NewService(svcCtx)

	err := svc.Delete(context.Background(), &DeleteRequest{ID: "ghost"})
	require.Error(t, err)
}

func TestService_Delete_AfterCreate(t *testing.T) {
	svcCtx := setupSvcCtx(t)
	svc := NewService(svcCtx)

	createResp, err := svc.Create(context.Background(), &CreateRequest{
		Name: "Del", Schema: mustParseSchema(t),
	})
	require.NoError(t, err)

	require.NoError(t, svc.Delete(context.Background(), &DeleteRequest{ID: createResp.ID}))

	_, err = svc.Get(context.Background(), &GetRequest{ID: createResp.ID})
	require.Error(t, err)
}

func TestService_Validate_AgainstSchema(t *testing.T) {
	svcCtx := setupSvcCtx(t)
	svc := NewService(svcCtx)

	createResp, err := svc.Create(context.Background(), &CreateRequest{
		Name: "Val", Schema: mustParseSchema(t),
	})
	require.NoError(t, err)

	// Valid payload (has required "name").
	resp, err := svc.Validate(context.Background(), &ValidateRequest{
		ID:   createResp.ID,
		Data: map[string]interface{}{"name": "alice"},
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.True(t, resp.Valid)

	// Invalid payload (missing required "name").
	badResp, err := svc.Validate(context.Background(), &ValidateRequest{
		ID:   createResp.ID,
		Data: map[string]interface{}{"age": 30},
	})
	require.NoError(t, err)
	require.NotNil(t, badResp)
	assert.False(t, badResp.Valid)
	assert.NotEmpty(t, badResp.Errors)
}

func TestService_RawValidate(t *testing.T) {
	svcCtx := setupSvcCtx(t)
	svc := NewService(svcCtx)

	resp, err := svc.RawValidate(context.Background(), &RawValidateRequest{
		Schema: mustParseSchema(t),
		Data:   map[string]interface{}{"name": "bob"},
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.True(t, resp.Valid)
}
