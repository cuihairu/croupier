package tool

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// All HTTP endpoint branches: bind failure / validation failure / update of
// a missing target / delete with an invalid ID.
func TestToolHTTPHandlerBranches(t *testing.T) {
	db := newToolTestDB(t)
	h := newToolHandler(db)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		if c.Request.URL.Path == "/tools" && c.Request.Method == http.MethodPost {
			c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), "username", "ops-admin"))
		}
		c.Next()
	})
	r.GET("/tools", h.List)
	r.POST("/tools", h.Create)
	r.PUT("/tools/:id", h.Update)
	r.DELETE("/tools/:id", h.Delete)

	do := func(method, target, body string) int {
		var req *http.Request
		if body == "" {
			req = httptest.NewRequest(method, target, nil)
		} else {
			req = httptest.NewRequest(method, target, strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w.Code
	}

	// List: success (all request fields are optional strings, so query binding cannot fail)
	assert.Equal(t, http.StatusOK, do(http.MethodGet, "/tools?gameId=g1&env=prod", ""))

	// Create: invalid JSON → bind failure
	assert.Equal(t, http.StatusBadRequest, do(http.MethodPost, "/tools", "{broken"))
	// Create: missing name → validation failure 400
	assert.Equal(t, http.StatusBadRequest, do(http.MethodPost, "/tools",
		`{"url":"https://example.com"}`))
	// Create: success (username context present → CreatedBy=ops-admin)
	assert.Equal(t, http.StatusOK, do(http.MethodPost, "/tools",
		`{"name":"Grafana","url":"https://grafana.example.com"}`))

	// Update: no updatable fields → 400
	assert.Equal(t, http.StatusBadRequest, do(http.MethodPut, "/tools/1", `{}`))
	// Update: target does not exist (missing re-read list) → 400
	assert.Equal(t, http.StatusBadRequest, do(http.MethodPut, "/tools/9999",
		`{"name":"Ghost"}`))

	// Delete: invalid ID → 400
	assert.Equal(t, http.StatusBadRequest, do(http.MethodDelete, "/tools/abc", ""))
	// Delete: success
	assert.Equal(t, http.StatusOK, do(http.MethodDelete, "/tools/1", ""))
}

// Underlying table missing: Create/Update model errors are propagated.
func TestToolHTTPModelFailure(t *testing.T) {
	db := newToolTestDB(t)
	require.NoError(t, db.Migrator().DropTable("tool_links"))
	h := newToolHandler(db)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/tools", h.Create)
	r.PUT("/tools/:id", h.Update)

	do := func(method, target, body string) int {
		req := httptest.NewRequest(method, target, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w.Code
	}
	assert.NotEqual(t, http.StatusOK, do(http.MethodPost, "/tools",
		`{"name":"X","url":"https://x.example.com"}`))
	assert.NotEqual(t, http.StatusOK, do(http.MethodPut, "/tools/1", `{"name":"Y"}`))
}
