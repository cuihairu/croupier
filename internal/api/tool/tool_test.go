package tool

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

var toolDBSeq uint64

func newToolTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	name := fmt.Sprintf("tool_%d", atomic.AddUint64(&toolDBSeq, 1))
	db, err := gorm.Open(gsqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", name)), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))
	return db
}

func newToolHandler(db *gorm.DB) *Handler {
	svcCtx := &svc.ServiceContext{ToolModel: model.NewToolLinkModel(db)}
	return NewHandler(NewService(svcCtx))
}

func toolRequest(method, target, body string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	reader := strings.NewReader(body)
	if body == "" {
		reader = strings.NewReader("")
	}
	c.Request = httptest.NewRequest(method, target, reader)
	c.Request.Header.Set("Content-Type", "application/json")
	return c, w
}

func TestToolCRUD(t *testing.T) {
	db := newToolTestDB(t)
	h := newToolHandler(db)

	// Create two tools: one global, one scoped.
	c, w := toolRequest(http.MethodPost, "/tools",
		`{"name":"Jenkins","url":"https://ci.acme.io","category":"ci","sort":10}`)
	h.Create(c)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	c, w = toolRequest(http.MethodPost, "/tools",
		`{"name":"Grafana-demo","url":"https://grafana.acme.io","category":"monitor","gameId":"demo","env":"prod"}`)
	h.Create(c)
	require.Equal(t, http.StatusOK, w.Code)

	// Global scope sees only the global tool.
	c, w = toolRequest(http.MethodGet, "/tools", "")
	h.List(c)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Jenkins")
	assert.NotContains(t, w.Body.String(), "Grafana-demo")

	// Scoped view sees both.
	c, w = toolRequest(http.MethodGet, "/tools?gameId=demo&env=prod", "")
	h.List(c)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Jenkins")
	assert.Contains(t, w.Body.String(), "Grafana-demo")

	// Disable the global tool → hidden from both views.
	c, w = toolRequest(http.MethodPut, "/tools/1", `{"enabled":false}`)
	c.Params = gin.Params{{Key: "id", Value: "1"}}
	h.Update(c)
	require.Equal(t, http.StatusOK, w.Code)
	c, w = toolRequest(http.MethodGet, "/tools?gameId=demo&env=prod", "")
	h.List(c)
	require.Equal(t, http.StatusOK, w.Code)
	assert.NotContains(t, w.Body.String(), "Jenkins")
}

func TestToolCreate_Validation(t *testing.T) {
	db := newToolTestDB(t)
	h := newToolHandler(db)

	cases := []struct {
		name string
		body string
	}{
		{"empty name", `{"name":"","url":"https://a.b"}`},
		{"javascript url", `{"name":"x","url":"javascript:alert(1)"}`},
		{"relative url", `{"name":"x","url":"/internal"}`},
		{"bad category", `{"name":"x","url":"https://a.b","category":"chat"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, w := toolRequest(http.MethodPost, "/tools", tc.body)
			h.Create(c)
			assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
		})
	}
}
