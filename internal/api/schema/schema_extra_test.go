// 覆盖目标：schema helpers 的错误分支（空 ID、坏 JSON、路径遍历、
// schema 定义/负载校验失败、唯一 ID 冲突）与 handler 的 service 错误路径。
package schema

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newExtraService(t *testing.T) (*Service, config.Config) {
	t.Helper()
	cfg := config.Config{Schemas: config.SchemasConfig{Dir: t.TempDir()}}
	return NewService(&svc.ServiceContext{Config: cfg}), cfg
}

func TestService_UIConfig_RoundTrip(t *testing.T) {
	svc, _ := newExtraService(t)
	ctx := context.Background()

	created, err := svc.Create(ctx, &CreateRequest{Name: "UI Schema", Schema: mustParseSchema(t)})
	require.NoError(t, err)

	get, err := svc.GetUIConfig(ctx, &GetUIConfigRequest{ID: created.ID})
	require.NoError(t, err)
	assert.Equal(t, created.ID, get.ID)

	upd, err := svc.UpdateUIConfig(ctx, &UpdateUIConfigRequest{ID: created.ID, Config: map[string]interface{}{"widget": "textarea"}})
	require.NoError(t, err)
	require.NotNil(t, upd)

	get2, err := svc.GetUIConfig(ctx, &GetUIConfigRequest{ID: created.ID})
	require.NoError(t, err)
	require.NotNil(t, get2.UIConfig)
}

func TestService_RawValidate_ValidAndInvalid(t *testing.T) {
	svc, _ := newExtraService(t)
	ctx := context.Background()
	schema := map[string]interface{}{
		"type":       "object",
		"required":   []interface{}{"a"},
		"properties": map[string]interface{}{"a": map[string]interface{}{"type": "integer"}},
	}

	resp, err := svc.RawValidate(ctx, &RawValidateRequest{Schema: schema, Data: map[string]interface{}{"a": "bad"}})
	require.NoError(t, err)
	assert.False(t, resp.Valid)
	assert.NotEmpty(t, resp.Errors)

	resp2, err := svc.RawValidate(ctx, &RawValidateRequest{Schema: schema, Data: map[string]interface{}{"a": 1}})
	require.NoError(t, err)
	assert.True(t, resp2.Valid)
}

func TestService_Validate_ExistingSchema(t *testing.T) {
	svc, _ := newExtraService(t)
	ctx := context.Background()
	created, err := svc.Create(ctx, &CreateRequest{Name: "V Schema", Schema: mustParseSchema(t)})
	require.NoError(t, err)

	// validSchemaJSON 要求 name 字段：合法/非法 payload 两个方向都覆盖
	resp, err := svc.Validate(ctx, &ValidateRequest{ID: created.ID, Data: map[string]interface{}{"name": "ok"}})
	require.NoError(t, err)
	assert.True(t, resp.Valid)

	respBad, err := svc.Validate(ctx, &ValidateRequest{ID: created.ID, Data: map[string]interface{}{}})
	require.NoError(t, err)
	assert.False(t, respBad.Valid)
}

func TestLoadSchema_EmptyID(t *testing.T) {
	_, cfg := newExtraService(t)
	_, err := loadSchema(cfg, "  ")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "不能为空")
}

func TestLoadSchema_CorruptJSON(t *testing.T) {
	_, cfg := newExtraService(t)
	dir, err := ensureSchemasDir(cfg)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bad.json"), []byte("{not-json"), 0o644))

	_, err = loadSchema(cfg, "bad")
	require.Error(t, err)
}

func TestLoadSchema_PathTraversal(t *testing.T) {
	_, cfg := newExtraService(t)
	// id 带 ../ 会被 validateSchemaPath 拒绝
	_, err := loadSchema(cfg, "../../etc/passwd")
	require.Error(t, err)
}

func TestDeleteSchemaByID_EmptyID_NotFound(t *testing.T) {
	_, cfg := newExtraService(t)
	require.Error(t, deleteSchemaByID(cfg, ""))
	require.Error(t, deleteSchemaByID(cfg, "missing-id"))
}

func TestDeleteSchemaByID_PathTraversal(t *testing.T) {
	_, cfg := newExtraService(t)
	require.Error(t, deleteSchemaByID(cfg, "../escape"))
}

func TestSaveSchema_DirUnwritable(t *testing.T) {
	_, cfg := newExtraService(t)
	// 把 schemas/custom 换成一个文件，MkdirAll 失败
	cfg.Schemas.Dir = filepath.Join(t.TempDir(), "not-a-dir")
	require.NoError(t, os.WriteFile(cfg.Schemas.Dir, []byte("x"), 0o644))
	err := saveSchema(cfg, &schemaDocument{ID: "s1", Name: "s1"})
	require.Error(t, err)
}

func TestEnsureUniqueSchemaID_Collision(t *testing.T) {
	_, cfg := newExtraService(t)
	dir, err := ensureSchemasDir(cfg)
	require.NoError(t, err)
	first := ensureUniqueSchemaID(cfg, "Same Name")
	require.NoError(t, os.WriteFile(filepath.Join(dir, first+".json"), []byte("{}"), 0o644))
	second := ensureUniqueSchemaID(cfg, "Same Name")
	assert.NotEqual(t, first, second, "collision should force a distinct id")
}

func TestValidateSchemaDefinition_BadDraftResource(t *testing.T) {
	require.Error(t, validateSchemaDefinition(map[string]interface{}{"type": 123}))
	require.Error(t, validateSchemaDefinition("not-a-map"))
}

func TestValidatePayloadAgainst_Errors(t *testing.T) {
	ok, errs, err := validatePayloadAgainst(
		map[string]interface{}{"type": "object", "required": []interface{}{"a"}, "properties": map[string]interface{}{"a": map[string]interface{}{"type": "integer"}}},
		map[string]interface{}{"a": "not-int"},
	)
	require.NoError(t, err)
	assert.False(t, ok)
	assert.NotEmpty(t, errs)

	_, _, err = validatePayloadAgainst(map[string]interface{}{"type": 123}, map[string]interface{}{})
	require.Error(t, err)
}

func TestListSchemas_MixedEntries(t *testing.T) {
	_, cfg := newExtraService(t)
	dir, err := ensureSchemasDir(cfg)
	require.NoError(t, err)
	// 一个合法 + 一个非 json 后缀 + 一个损坏文件，listSchemas 应跳过坏文件
	require.NoError(t, os.WriteFile(filepath.Join(dir, "good.json"), []byte(`{"id":"good","name":"G"}`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("skip"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "corrupt.json"), []byte("{"), 0o644))

	items, err := listSchemas(cfg)
	require.NoError(t, err)
	found := false
	for _, it := range items {
		if it.ID == "good" {
			found = true
		}
	}
	assert.True(t, found, "good.json should be listed")
}

func TestService_GetUIConfig_NotFound(t *testing.T) {
	svc, _ := newExtraService(t)
	_, err := svc.GetUIConfig(context.Background(), &GetUIConfigRequest{ID: "missing"})
	require.Error(t, err)
}

func TestHandler_Update_MalformedJSON_BadRequest(t *testing.T) {
	svcCtx := &svc.ServiceContext{Config: config.Config{Schemas: config.SchemasConfig{Dir: t.TempDir()}}}
	router := newRouter(NewHandler(NewService(svcCtx)))

	rec := doReq(t, router, http.MethodPut, "/schemas/x", `{bad-json`)
	assertStatus(t, rec, http.StatusBadRequest)
}

func TestHandler_List_ServiceError(t *testing.T) {
	// Dir 指向文件：listSchemas 失败
	dir := filepath.Join(t.TempDir(), "f")
	require.NoError(t, os.WriteFile(dir, []byte("x"), 0o644))
	router := newRouter(NewHandler(NewService(&svc.ServiceContext{Config: config.Config{Schemas: config.SchemasConfig{Dir: dir}}})))

	rec := doReq(t, router, http.MethodGet, "/schemas", "")
	assert.NotEqual(t, http.StatusOK, rec.Code)
}

func TestHandler_Validate_MissingSchema_NotFound(t *testing.T) {
	svc, _ := newExtraService(t)
	router := newRouter(NewHandler(svc))

	rec := doReq(t, router, http.MethodPost, "/schemas/ghost/validate", `{"payload":{}}`)
	assert.NotEqual(t, http.StatusOK, rec.Code)
}

func TestHandler_Delete_NotFound(t *testing.T) {
	svc, _ := newExtraService(t)
	router := newRouter(NewHandler(svc))

	rec := doReq(t, router, http.MethodDelete, "/schemas/ghost", "")
	assertStatus(t, rec, http.StatusNotFound)
}

func TestHandler_UIConfig_RoundTrip(t *testing.T) {
	svc, _ := newExtraService(t)
	created, err := svc.Create(context.Background(), &CreateRequest{Name: "H UI", Schema: mustParseSchema(t)})
	require.NoError(t, err)

	r := gin.New()
	gin.SetMode(gin.TestMode)
	r.GET("/schemas/:id/ui-config", NewHandler(svc).GetUIConfig)
	r.PUT("/schemas/:id/ui-config", NewHandler(svc).UpdateUIConfig)

	rec := doReq(t, r, http.MethodGet, "/schemas/"+created.ID+"/ui-config", "")
	assertStatus(t, rec, http.StatusOK)

	rec2 := doReq(t, r, http.MethodPut, "/schemas/"+created.ID+"/ui-config", `{"config":{"widget":"input"}}`)
	assertStatus(t, rec2, http.StatusOK)

	rec3 := doReq(t, r, http.MethodPut, "/schemas/"+created.ID+"/ui-config", `{bad`)
	assertStatus(t, rec3, http.StatusBadRequest)
}
