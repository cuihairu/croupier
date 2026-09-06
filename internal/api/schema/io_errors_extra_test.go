// 覆盖目标：handler 绑定失败分支、saveSchema 序列化/写盘失败、loadSchema
// 非常规读错误、listSchemas 断链条目、validateSchemaPath 的 MkdirAll 与
// Abs 失败、service 的 Update/UpdateUIConfig/Validate 错误路径。
package schema

import (
	"bytes"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandler_List_BindQueryError(t *testing.T) {
	svcCtx := &svc.ServiceContext{Config: config.Config{Schemas: config.SchemasConfig{Dir: t.TempDir()}}}
	router := newRouter(NewHandler(NewService(svcCtx)))

	rec := doReq(t, router, http.MethodGet, "/schemas?page=abc", "")
	assertStatus(t, rec, http.StatusBadRequest)
}

func TestHandler_Update_InvalidSchemaRejected(t *testing.T) {
	svcCtx := &svc.ServiceContext{Config: config.Config{Schemas: config.SchemasConfig{Dir: t.TempDir()}}}
	router := newRouter(NewHandler(NewService(svcCtx)))

	// schema 定义非法 → service.Update 校验失败 → 400
	rec := doReq(t, router, http.MethodPut, "/schemas/whatever", `{"schema":{"type":"banana"}}`)
	assertStatus(t, rec, http.StatusBadRequest)
}

func TestHandler_RawValidate_MalformedJSON(t *testing.T) {
	svcCtx := &svc.ServiceContext{Config: config.Config{Schemas: config.SchemasConfig{Dir: t.TempDir()}}}
	router := newRouter(NewHandler(NewService(svcCtx)))

	rec := doReq(t, router, http.MethodPost, "/schemas/raw-validate", `{bad-json`)
	assertStatus(t, rec, http.StatusBadRequest)
}

func TestService_Create_NaNSchemaMarshalFails(t *testing.T) {
	s, _ := newExtraService(t)
	// NaN 能通过 jsonschema Compile，但 json.MarshalIndent 必定失败。
	_, err := s.Create(t.Context(), &CreateRequest{
		Name:   "NaN Schema",
		Schema: map[string]interface{}{"type": "object", "nan": math.NaN()},
	})
	require.Error(t, err)
}

func TestLoadSchema_IDResolvesToDirectory(t *testing.T) {
	_, cfg := newExtraService(t)
	dir, err := ensureSchemasDir(cfg)
	require.NoError(t, err)
	// 目录名恰好以 .json 结尾：ReadFile 返回 EISDIR（非 NotExist）错误。
	require.NoError(t, os.Mkdir(filepath.Join(dir, "direntry.json"), 0o755))

	_, err = loadSchema(cfg, "direntry")
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "不存在", "EISDIR 不应被翻译为 NotFound")
}

func TestListSchemas_SkipsBrokenSymlink(t *testing.T) {
	_, cfg := newExtraService(t)
	dir, err := ensureSchemasDir(cfg)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "good.json"), []byte(`{"id":"good","name":"G"}`), 0o644))
	// 断链符号链接：ReadFile 返回 ENOENT → entry 被跳过。
	require.NoError(t, os.Symlink(filepath.Join(dir, "missing-target.json"), filepath.Join(dir, "broken.json")))

	items, err := listSchemas(cfg)
	require.NoError(t, err)
	for _, it := range items {
		assert.NotEqual(t, "broken", it.ID)
	}
}

func TestService_Validate_CorruptSchemaDefinition(t *testing.T) {
	s, cfg := newExtraService(t)
	dir, err := ensureSchemasDir(cfg)
	require.NoError(t, err)
	// 手写文件绕过 Create 校验：loadSchema 成功但 schema 定义非法，
	// validatePayloadAgainst 的 Compile 失败。
	require.NoError(t, os.WriteFile(filepath.Join(dir, "broken.json"),
		[]byte(`{"id":"broken","name":"B","schema":{"type":"banana"}}`), 0o644))

	_, err = s.Validate(t.Context(), &ValidateRequest{ID: "broken", Data: map[string]interface{}{}})
	require.Error(t, err)
}

func makeReadOnlyFile(t *testing.T, path string) {
	t.Helper()
	require.NoError(t, os.Chmod(path, 0o400))
	t.Cleanup(func() { _ = os.Chmod(path, 0o644) })
}

func TestService_Update_SaveFailsOnReadOnlyFile(t *testing.T) {
	s, cfg := newExtraService(t)
	ctx := t.Context()
	created, err := s.Create(ctx, &CreateRequest{Name: "RO Schema", Schema: mustParseSchema(t)})
	require.NoError(t, err)

	dir, err := ensureSchemasDir(cfg)
	require.NoError(t, err)
	// 文件只读：O_TRUNC 打开失败（目录仍可读，loadSchema 成功）。
	makeReadOnlyFile(t, filepath.Join(dir, created.ID+".json"))

	_, err = s.Update(ctx, &UpdateRequest{ID: created.ID, Schema: mustParseSchema(t)})
	require.Error(t, err)
}

func TestService_UpdateUIConfig_SaveFailsOnReadOnlyFile(t *testing.T) {
	s, cfg := newExtraService(t)
	ctx := t.Context()
	created, err := s.Create(ctx, &CreateRequest{Name: "RO UI Schema", Schema: mustParseSchema(t)})
	require.NoError(t, err)

	dir, err := ensureSchemasDir(cfg)
	require.NoError(t, err)
	makeReadOnlyFile(t, filepath.Join(dir, created.ID+".json"))

	_, err = s.UpdateUIConfig(ctx, &UpdateUIConfigRequest{ID: created.ID, Config: map[string]interface{}{"widget": "input"}})
	require.Error(t, err)
}

func makeReadOnly(t *testing.T, dir string) {
	t.Helper()
	require.NoError(t, os.Chmod(dir, 0o500))
	t.Cleanup(func() { _ = os.Chmod(dir, 0o750) })
}

func TestValidateSchemaPath_MkdirAllFails(t *testing.T) {
	parent := t.TempDir()
	makeReadOnly(t, parent)
	cfg := config.Config{Schemas: config.SchemasConfig{Dir: filepath.Join(parent, "schemas")}}

	_, err := validateSchemaPath(cfg, filepath.Join(parent, "schemas", "custom", "x.json"))
	require.Error(t, err)
}

func TestValidateSchemaPath_RelativePathAbsFails(t *testing.T) {
	// cwd 被删除后 filepath.Abs(相对路径) 失败（Getwd ENOENT）。
	wd := t.TempDir()
	base := t.TempDir()
	t.Chdir(wd)
	require.NoError(t, os.RemoveAll(wd))

	cfg := config.Config{Schemas: config.SchemasConfig{Dir: base}}
	_, err := validateSchemaPath(cfg, "relative/x.json")
	require.Error(t, err)
}

func TestRawValidateHandlerBranches(t *testing.T) {
	handler := setupHandler(t)
	h, r := handler, newRouter(handler)
	_ = h

	// bind 失败：非法 JSON
	req := httptest.NewRequest(http.MethodPost, "/schemas/raw-validate", bytes.NewReader([]byte("{bad")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusBadRequest, w.Code)

	// service 错误：schema 带不可解析 $ref → 编译失败 → BadRequest
	body := `{"schema":{"$ref":"#/$defs/missing"},"data":{}}`
	req = httptest.NewRequest(http.MethodPost, "/schemas/raw-validate", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestDeleteSchemaByID_RemoveFailsOnReadOnlyDir(t *testing.T) {
	s, cfg := newExtraService(t)
	ctx := t.Context()
	created, err := s.Create(ctx, &CreateRequest{Name: "RO Del", Schema: mustParseSchema(t)})
	require.NoError(t, err)

	dir, err := ensureSchemasDir(cfg)
	require.NoError(t, err)
	makeReadOnly(t, dir)

	require.Error(t, deleteSchemaByID(cfg, created.ID))
}

func TestValidatePayloadAgainstCompileFailure(t *testing.T) {
	_, _, err := validatePayloadAgainst(map[string]interface{}{"$ref": "#/$defs/missing"}, nil)
	require.Error(t, err)
}
