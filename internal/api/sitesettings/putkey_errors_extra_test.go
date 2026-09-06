// 覆盖目标：PutKey 的绑定失败/空 value/secret 掩码未设置/store 写入错误、
// ClearKey 的 store 删除错误、testOIDCDiscovery 的证书校验失败分支。
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

// setupRouterWithDB 同 setupRouter 但暴露 db 供回调注入。
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
	// 数据库层从未写入该 secret：掩码回存必须被拒绝而非落库掩码。
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
	// httptest TLS 证书自签且不被系统信任 → x509 校验失败分支。
	srv := httptest.NewTLSServer(nil)
	defer srv.Close()

	err := testOIDCDiscovery(config.OIDCProviderConfig{Issuer: srv.URL})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "证书校验失败")
}
