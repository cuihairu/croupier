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
	require.NoError(t, db.AutoMigrate(&model.Announcement{}, &model.AnnouncementRead{}, &model.Admin{}, &model.Role{}, &model.AdminRole{}))
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

	// List：空列表
	w := doJSON2(r, http.MethodGet, "/admin/announcements", "", "")
	assert.Equal(t, http.StatusOK, w.Code)

	// Create：成功 201 + 非法 body 400
	w = doJSON2(r, http.MethodPost, "/admin/announcements", `{"title":"维护","contentMd":"c","audience":"all","popup":true}`, "")
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	w = doJSON2(r, http.MethodPost, "/admin/announcements", `{bad`, "")
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// Update：成功 / 非法 ID / 非法 body
	w = doJSON2(r, http.MethodPut, "/admin/announcements/1", `{"title":"改"}`, "")
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	w = doJSON2(r, http.MethodPut, "/admin/announcements/abc", `{}`, "")
	assert.Equal(t, http.StatusBadRequest, w.Code)
	w = doJSON2(r, http.MethodPut, "/admin/announcements/1", `{bad`, "")
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// Delete：成功 204 / 非法 ID 400
	w = doJSON2(r, http.MethodDelete, "/admin/announcements/1", "", "")
	assert.Equal(t, http.StatusNoContent, w.Code)
	w = doJSON2(r, http.MethodDelete, "/admin/announcements/xyz", "", "")
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandler_UserSide_ActiveAndDismiss(t *testing.T) {
	_, r := newHandlerFixture(t)

	// 未认证：Active 报错（无 username 上下文）
	w := doJSON2(r, http.MethodGet, "/announcements/active", "", "")
	assert.NotEqual(t, http.StatusOK, w.Code)

	// 未认证：Dismiss 先校验 ID，非法 ID 优先 400
	w = doJSON2(r, http.MethodPost, "/announcements/abc/dismiss", "", "")
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// 创建一条全员公告
	w = doJSON2(r, http.MethodPost, "/admin/announcements", `{"title":"通知","contentMd":"c","audience":"all","popup":true}`, "")
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	// 认证后：Active 可见且 shouldPopup=true；Dismiss 确认后不再弹
	w = doJSON2(r, http.MethodGet, "/announcements/active", "", "alice")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"shouldPopup":true`)

	w = doJSON2(r, http.MethodPost, "/announcements/1/dismiss", "", "alice")
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())

	w = doJSON2(r, http.MethodGet, "/announcements/active", "", "alice")
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"shouldPopup":false`)
}

// service 错误经 response.Error 映射：404（不存在）走 HTTP 层验证。
func TestHandler_AdminCRUD_NotFound(t *testing.T) {
	_, r := newHandlerFixture(t)

	w := doJSON2(r, http.MethodPut, "/admin/announcements/99", `{"title":"x"}`, "")
	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "not_found")

	w = doJSON2(r, http.MethodDelete, "/admin/announcements/99", "", "")
	assert.Equal(t, http.StatusNotFound, w.Code)

	// 认证用户 dismiss 不存在的公告 → 404
	w = doJSON2(r, http.MethodPost, "/announcements/99/dismiss", "", "alice")
	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
}

// List 的 bind 无 body、Active 的 service 错误分支经 HTTP 验证。
func TestHandler_List_CreateDBError(t *testing.T) {
	_, _ = newHandlerFixture(t)

	// 直接构造坏 DB 的 handler：List/Create 走 500 错误分支
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	bad := NewHandler(NewService(&svc.ServiceContext{DB: db}))
	badRouter := gin.New()
	badRouter.GET("/bad/announcements", bad.List)
	badRouter.POST("/bad/announcements", bad.Create)

	w := doJSON2(badRouter, http.MethodGet, "/bad/announcements", "", "")
	assert.Equal(t, http.StatusInternalServerError, w.Code, "List 错误走 internal_error 兜底")

	// POST：错误分支只需保证非 200 且带 error 字段（错误码由驱动错误链决定）
	w = doJSON2(badRouter, http.MethodPost, "/bad/announcements", `{"title":"t","audience":"all"}`, "")
	assert.NotEqual(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "error")
}
