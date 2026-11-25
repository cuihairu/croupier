// Coverage target: PutKey's binding failure / empty value / secret mask while not
// set / store write error, ClearKey's store delete error, and testOIDCDiscovery's
// certificate validation failure branch.
package sitesettings

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/settings"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// setupRouterWithDB is like setupRouter but also exposes db for callback injection.
func setupRouterWithDB(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	settings.ResetForTest()
	db := newTestDB(t)
	require.NoError(t, model.AutoMigrate(db))
	store := model.NewPlatformSettingModel(db)
	layered := settings.InitLayered(t.Context(), &settings.ConfigInput{}, store)
	h := NewHandler(layered, store)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	api := r.Group("/api/v1")
	RegisterPublic(api.Group("/public"), h)
	h.RegisterAdmin(api.Group("/"))
	return r, db
}

func doSiteReq(t *testing.T, r *gin.Engine, method, path, raw string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, body(raw))
	if raw != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func TestPutKey_MalformedJSON(t *testing.T) {
	r, _ := setupRouterWithDB(t)
	rec := doSiteReq(t, r, http.MethodPut, "/api/v1/site/site.name", `{bad-json`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestPutKey_EmptyValue(t *testing.T) {
	r, _ := setupRouterWithDB(t)
	rec := doSiteReq(t, r, http.MethodPut, "/api/v1/site/site.name", `{}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "value 不能为空")
}

func TestPutKey_SecretMaskButNotSet(t *testing.T) {
	r, _ := setupRouterWithDB(t)
	// The database layer never wrote this secret: saving the mask back must be
	// rejected rather than landing the mask in the database.
	rec := doSiteReq(t, r, http.MethodPut, "/api/v1/site/auth.oidc.clientSecret", `{"value":"****t123"}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "secret 未设置")
}

func injectSettingsCallback(t *testing.T, db *gorm.DB, op string) {
	t.Helper()
	err := errors.New("forced " + op + " failure")
	fn := func(tx *gorm.DB) { _ = tx.AddError(err) }
	switch op {
	case "create":
		require.NoError(t, db.Callback().Create().Before("gorm:create").Register("test:fail_create", fn))
	case "delete":
		require.NoError(t, db.Callback().Delete().Before("gorm:delete").Register("test:fail_delete", fn))
	default:
		t.Fatalf("unsupported op %q", op)
	}
	t.Cleanup(func() {
		_ = db.Callback().Create().Remove("test:fail_create")
		_ = db.Callback().Delete().Remove("test:fail_delete")
	})
}

func TestPutKey_StoreSetError(t *testing.T) {
	r, db := setupRouterWithDB(t)
	injectSettingsCallback(t, db, "create")

	rec := doSiteReq(t, r, http.MethodPut, "/api/v1/site/site.name", `{"value":"My Site"}`)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestClearKey_StoreClearError(t *testing.T) {
	r, db := setupRouterWithDB(t)
	injectSettingsCallback(t, db, "delete")

	rec := doSiteReq(t, r, http.MethodDelete, "/api/v1/site/site.name", "")
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestTestOIDCDiscovery_CertificateError(t *testing.T) {
	// The httptest TLS certificate is self-signed and not trusted by the system -> the
	// x509 validation failure branch.
	srv := httptest.NewTLSServer(nil)
	defer srv.Close()

	err := testOIDCDiscovery(config.OIDCProviderConfig{Issuer: srv.URL})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "证书校验失败")
}
