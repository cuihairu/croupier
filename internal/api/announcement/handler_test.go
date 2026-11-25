package announcement

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func newHandlerFixture(t *testing.T) (*Handler, *gin.Engine) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Announcement{}, &model.AnnouncementRead{}, &model.AnnouncementGame{}, &model.Admin{}, &model.Role{}, &model.AdminRole{}))
	svcCtx := &svc.ServiceContext{DB: db, AdminModel: model.NewAdminModel(db)}
	require.NoError(t, svcCtx.AdminModel.Create(context.Background(), &model.Admin{Username: "alice", Nickname: "Alice"}, "x"))
	h := NewHandler(NewService(svcCtx))
	r := gin.New()
	admin := r.Group("")
	{
		admin.GET("/admin/announcements", h.List)
		admin.POST("/admin/announcements", h.Create)
		admin.PUT("/admin/announcements/:id", h.Update)
		admin.DELETE("/admin/announcements/:id", h.Delete)
	}
	r.GET("/announcements/active", h.Active)
	r.POST("/announcements/:id/dismiss", h.Dismiss)
	return h, r
}

func doJSON2(r *gin.Engine, method, path, body string, username string) *httptest.ResponseRecorder {
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req, _ := http.NewRequest(method, path, reader)
	if username != "" {
		req = req.WithContext(context.WithValue(req.Context(), "username", username))
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestHandler_AdminCRUD(t *testing.T) {
	_, r := newHandlerFixture(t)

	// List: empty list
	w := doJSON2(r, http.MethodGet, "/admin/announcements", "", "")
	assert.Equal(t, http.StatusOK, w.Code)

	// Create: 201 on success + 400 on invalid body
	w = doJSON2(r, http.MethodPost, "/admin/announcements", `{"title":"维护","contentMd":"c","audience":"all","popup":true}`, "")
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	w = doJSON2(r, http.MethodPost, "/admin/announcements", `{bad`, "")
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// Update: success / invalid ID / invalid body
	w = doJSON2(r, http.MethodPut, "/admin/announcements/1", `{"title":"改"}`, "")
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	w = doJSON2(r, http.MethodPut, "/admin/announcements/abc", `{}`, "")
	assert.Equal(t, http.StatusBadRequest, w.Code)
	w = doJSON2(r, http.MethodPut, "/admin/announcements/1", `{bad`, "")
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// Delete: 204 on success / 400 on invalid ID
	w = doJSON2(r, http.MethodDelete, "/admin/announcements/1", "", "")
	assert.Equal(t, http.StatusNoContent, w.Code)
	w = doJSON2(r, http.MethodDelete, "/admin/announcements/xyz", "", "")
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandler_UserSide_ActiveAndDismiss(t *testing.T) {
	_, r := newHandlerFixture(t)

	// Unauthenticated: Active errors out (no username context)
	w := doJSON2(r, http.MethodGet, "/announcements/active", "", "")
	assert.NotEqual(t, http.StatusOK, w.Code)

	// Unauthenticated: Dismiss validates the ID first, so an invalid ID wins
	// with 400
	w = doJSON2(r, http.MethodPost, "/announcements/abc/dismiss", "", "")
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// Create an all-users announcement
	w = doJSON2(r, http.MethodPost, "/admin/announcements", `{"title":"通知","contentMd":"c","audience":"all","popup":true}`, "")
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	// Authenticated: Active is visible with shouldPopup=true; after Dismiss
	// the popup stops
	w = doJSON2(r, http.MethodGet, "/announcements/active", "", "alice")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"shouldPopup":true`)

	w = doJSON2(r, http.MethodPost, "/announcements/1/dismiss", "", "alice")
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())

	w = doJSON2(r, http.MethodGet, "/announcements/active", "", "alice")
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"shouldPopup":false`)
}

// Service errors mapped by response.Error: the 404 (missing) path is
// verified at the HTTP layer.
func TestHandler_AdminCRUD_NotFound(t *testing.T) {
	_, r := newHandlerFixture(t)

	w := doJSON2(r, http.MethodPut, "/admin/announcements/99", `{"title":"x"}`, "")
	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "not_found")

	w = doJSON2(r, http.MethodDelete, "/admin/announcements/99", "", "")
	assert.Equal(t, http.StatusNotFound, w.Code)

	// Authenticated user dismisses a missing announcement -> 404
	w = doJSON2(r, http.MethodPost, "/announcements/99/dismiss", "", "alice")
	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
}

// List's bind without a body and Active's service error branch, verified
// over HTTP.
func TestHandler_List_CreateDBError(t *testing.T) {
	_, _ = newHandlerFixture(t)

	// Build a handler around a broken DB directly: List/Create take the 500
	// error branches
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	bad := NewHandler(NewService(&svc.ServiceContext{DB: db}))
	badRouter := gin.New()
	badRouter.GET("/bad/announcements", bad.List)
	badRouter.POST("/bad/announcements", bad.Create)

	w := doJSON2(badRouter, http.MethodGet, "/bad/announcements", "", "")
	assert.Equal(t, http.StatusInternalServerError, w.Code, "List 错误走 internal_error 兜底")

	// POST: the error branch only needs to be non-200 and carry an error
	// field (the error code depends on the driver error chain)
	w = doJSON2(badRouter, http.MethodPost, "/bad/announcements", `{"title":"t","audience":"all"}`, "")
	assert.NotEqual(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "error")
}
