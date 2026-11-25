// Coverage target: the handler binding failure branch, saveSchema
// serialization/write-to-disk failures, non-standard read errors in loadSchema,
// broken entries in listSchemas, the MkdirAll and Abs failures of
// validateSchemaPath, and the Update/UpdateUIConfig/Validate error paths of the
// service.
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

	// An invalid schema definition -> service.Update validation fails -> 400
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
	// A NaN value passes jsonschema Compile, but json.MarshalIndent is guaranteed to fail.
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
	// A directory whose name happens to end in .json: ReadFile returns an EISDIR error
	// (not NotExist).
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
	// A dangling symlink: ReadFile returns ENOENT -> the entry is skipped.
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
	// Hand-write the file to bypass Create's validation: loadSchema succeeds but the
	// schema definition is invalid, so validatePayloadAgainst's Compile fails.
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
	// A read-only file: opening with O_TRUNC fails (the directory is still readable,
	// so loadSchema succeeds).
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
	// After the cwd is removed, filepath.Abs on a relative path fails (Getwd ENOENT).
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

	// Binding fails: malformed JSON
	req := httptest.NewRequest(http.MethodPost, "/schemas/raw-validate", bytes.NewReader([]byte("{bad")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusBadRequest, w.Code)

	// Service error: a schema with an unresolvable $ref -> compilation fails -> BadRequest
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

func TestValidateSchemaAddResourceFailsWhenCwdRemoved(t *testing.T) {
	// With the cwd removed, jsonschema v6's Compiler.AddResource calls absolute() on the
	// relative URL "schema.json", which depends on Getwd (ENOENT); the error
	// propagates as an AddResource failure, covering the AddResource err branches
	// of validateSchemaDefinition / validatePayloadAgainst.
	wd := t.TempDir()
	t.Chdir(wd)
	require.NoError(t, os.RemoveAll(wd))

	err := validateSchemaDefinition(map[string]interface{}{"type": "object"})
	require.Error(t, err)

	_, _, err = validatePayloadAgainst(map[string]interface{}{"type": "object"}, map[string]interface{}{})
	require.Error(t, err)
}
