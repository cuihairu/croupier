// Coverage targets: admin handler storage-error branches (dropped tables) and query bind failures.
package admin

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func doAdminList(t *testing.T, dropTable string, query string) int {
	t.Helper()
	handler, db := setupAdminHandlerTest(t)
	if dropTable != "" {
		require.NoError(t, db.Migrator().DropTable(dropTable))
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(addAdminAuthMiddleware(db))
	router.GET("/api/v1/admin", handler.List)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/admin"+query, nil))
	return rec.Code
}

func TestAdminHandler_List_InvalidPageQuery(t *testing.T) {
	// page is an int form field: an invalid value triggers a bind failure
	code := doAdminList(t, "", "?page=abc")
	assert.NotEqual(t, http.StatusOK, code)
}
