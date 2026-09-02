package policy

// 覆盖目标：handler.go 中 GetPolicy/SetPolicy/DeletePolicy 的 manager 错误分支、
// ReloadConfig 的配置文件解析错误分支（通过 DropTable / 写入非法 YAML 触发）。

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"os"
	"sync/atomic"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/policy"
	"github.com/gin-gonic/gin"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

var policyCovSeq uint64

// newPolicyCovDB 每个用例独立的内存库，避免 DropTable 影响其他用例。
func newPolicyCovDB(t *testing.T) *gorm.DB {
	t.Helper()
	name := fmt.Sprintf("policy_cov_%d", atomic.AddUint64(&policyCovSeq, 1))
	db, err := gorm.Open(gsqlite.Open("file:"+name+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.FunctionPolicy{}))
	return db
}

func writePolicyYAML(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := dir + "/default-policies.yaml"
	content := []byte(`low:
  require_approval: false
  require_audit: false
  allowed_roles:
    - user
high:
  require_approval: true
  approval_workflow: single_admin
  require_audit: true
  allowed_roles:
    - admin
`)
	require.NoError(t, os.WriteFile(path, content, 0644))
	return path
}

func newPolicyCovHandler(t *testing.T) (*Handler, *gorm.DB, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db := newPolicyCovDB(t)
	path := writePolicyYAML(t)
	manager, err := policy.NewManager(db, path)
	require.NoError(t, err)
	return NewHandler(manager), db, path
}

func TestHandlerV2_GetPolicy_ManagerError(t *testing.T) {
	handler, db, _ := newPolicyCovHandler(t)
	require.NoError(t, db.Migrator().DropTable(&model.FunctionPolicy{}))

	router := gin.New()
	router.GET("/functions/:function_id/policy", handler.GetPolicy)

	req, _ := http.NewRequest("GET", "/functions/fn.err/policy?risk_level=high", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "error")
}

func TestHandlerV2_SetPolicy_ManagerError(t *testing.T) {
	handler, db, _ := newPolicyCovHandler(t)
	require.NoError(t, db.Migrator().DropTable(&model.FunctionPolicy{}))

	router := gin.New()
	router.PUT("/functions/:function_id/policy", handler.SetPolicy)

	body := `{"require_approval":true,"require_audit":true,"allowed_roles":["admin"]}`
	req, _ := http.NewRequest("PUT", "/functions/fn.err/policy", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandlerV2_DeletePolicy_ManagerError(t *testing.T) {
	handler, db, _ := newPolicyCovHandler(t)
	require.NoError(t, db.Migrator().DropTable(&model.FunctionPolicy{}))

	router := gin.New()
	router.DELETE("/functions/:function_id/policy", handler.DeletePolicy)

	req, _ := http.NewRequest("DELETE", "/functions/fn.err/policy", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandlerV2_ReloadConfig_InvalidYAML(t *testing.T) {
	handler, _, path := newPolicyCovHandler(t)

	// 覆盖原配置为非法 YAML，触发 loadConfig 的 Unmarshal 错误。
	invalid := []byte("low: [unclosed\n  broken: {\n")
	require.NoError(t, os.WriteFile(path, invalid, 0644))

	router := gin.New()
	router.POST("/policies/reload", handler.ReloadConfig)

	req, _ := http.NewRequest("POST", "/policies/reload", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code, "body=%s", w.Body.String())

	// 恢复合法配置后 reload 应再次成功。
	require.NoError(t, os.WriteFile(path, []byte("low:\n  require_approval: false\n"), 0644))
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req)
	assert.Equal(t, http.StatusOK, w2.Code)
	_ = httputil.NewSingleHostReverseProxy // 保持 net/http/httputil 引用（可移除）
}
