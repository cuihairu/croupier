package menu

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func newMenuTestRouter(t *testing.T, permissions ...string) (*gin.Engine, context.Context) {
	return newMenuTestRouterWithScope(t, true, permissions...)
}

func newMenuTestRouterWithScope(t *testing.T, withScope bool, permissions ...string) (*gin.Engine, context.Context) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)

	admin := model.Admin{Username: "menu_handler_tester", Status: 1, PasswordHash: "test"}
	require.NoError(t, db.Create(&admin).Error)
	role := model.Role{Name: "menu_handler_tester_role", Description: "menu handler tester"}
	require.NoError(t, db.Create(&role).Error)
	require.NoError(t, db.Create(&model.AdminRole{AdminID: admin.ID, RoleID: role.ID}).Error)
	for _, permissionID := range permissions {
		grantMenuPermission(t, db, role.ID, permissionID)
	}

	svcCtx := &svc.ServiceContext{
		DB:              db,
		AdminModel:      model.NewAdminModel(db),
		RoleModel:       model.NewRoleModel(db),
		PermissionModel: model.NewPermissionModel(db),
		MenuModel:       model.NewMenuItemModel(db),
	}
	baseCtx := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "demo-game", Env: "development"})
	baseCtx = context.WithValue(baseCtx, "username", admin.Username)

	r := gin.New()
	g := r.Group("/api/v1/menus", func(c *gin.Context) {
		if withScope {
			c.Request = c.Request.WithContext(baseCtx)
		} else {
			c.Request = c.Request.WithContext(context.WithValue(context.Background(), "username", admin.Username))
		}
		c.Next()
	})
	RegisterMenuRoutes(g, svcCtx)
	return r, baseCtx
}

func doMenuRequest(t *testing.T, r *gin.Engine, method, target, body string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	var reader *bytes.Reader
	if body == "" {
		reader = bytes.NewReader(nil)
	} else {
		reader = bytes.NewReader([]byte(body))
	}
	req := httptest.NewRequest(method, target, reader)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	var payload map[string]any
	if rec.Body.Len() > 0 {
		_ = json.Unmarshal(rec.Body.Bytes(), &payload)
	}
	return rec, payload
}

func TestMenuHandlerCRUDRoutes(t *testing.T) {
	r, _ := newMenuTestRouter(t, "menu:create", "menu:read", "menu:update", "menu:delete", "menu:sort")

	// POST /api/v1/menus → 201
	rec, created := doMenuRequest(t, r, http.MethodPost, "/api/v1/menus",
		`{"menuKey":"resource","labels":{"zh-CN":"资源管理","en-US":"Resource"},"icon":"DatabaseOutlined","sortOrder":1}`)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	assert.Equal(t, "resource", created["menuKey"])
	assert.Equal(t, true, created["isVisible"])
	id, ok := created["id"].(float64)
	require.True(t, ok)
	idStr := strconv.FormatInt(int64(id), 10)

	// 子菜单
	rec, _ = doMenuRequest(t, r, http.MethodPost, "/api/v1/menus",
		`{"menuKey":"player","parentId":`+idStr+`,"labels":{"zh-CN":"玩家管理"}}`)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	// GET /api/v1/menus → 树
	rec, list := doMenuRequest(t, r, http.MethodGet, "/api/v1/menus", "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	items, ok := list["items"].([]any)
	require.True(t, ok)
	require.Len(t, items, 1)
	children, ok := items[0].(map[string]any)["children"].([]any)
	require.True(t, ok)
	require.Len(t, children, 1)
	assert.Equal(t, "player", children[0].(map[string]any)["menuKey"])

	// PUT /api/v1/menus/:id → 更新 icon
	rec, updated := doMenuRequest(t, r, http.MethodPut, "/api/v1/menus/"+idStr, `{"icon":"TeamOutlined"}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "TeamOutlined", updated["icon"])

	// PUT /api/v1/menus/:id/sort → 排序
	rec, sorted := doMenuRequest(t, r, http.MethodPut, "/api/v1/menus/"+idStr+"/sort", `{"sortOrder":7}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.EqualValues(t, 7, sorted["sortOrder"])

	// DELETE /api/v1/menus/:id → 204
	rec, _ = doMenuRequest(t, r, http.MethodDelete, "/api/v1/menus/"+idStr, "")
	require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())

	// 删除后 GET → 空树
	rec, list = doMenuRequest(t, r, http.MethodGet, "/api/v1/menus", "")
	require.Equal(t, http.StatusOK, rec.Code)
	items, _ = list["items"].([]any)
	assert.Empty(t, items)
}

func TestMenuHandlerErrorShapes(t *testing.T) {
	r, _ := newMenuTestRouter(t, "menu:create", "menu:read", "menu:update")

	// 400：非法 key（统一错误对象 error/message）
	rec, payload := doMenuRequest(t, r, http.MethodPost, "/api/v1/menus",
		`{"menuKey":"bad key!","labels":{"zh-CN":"x"}}`)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, "bad_request", payload["error"])
	assert.NotEmpty(t, payload["message"])

	// 409：重复 key（先成功创建一次）
	rec, payload = doMenuRequest(t, r, http.MethodPost, "/api/v1/menus",
		`{"menuKey":"resource","labels":{"zh-CN":"资源管理"}}`)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	rec, payload = doMenuRequest(t, r, http.MethodPost, "/api/v1/menus",
		`{"menuKey":"resource","labels":{"zh-CN":"资源管理"}}`)
	require.Equal(t, http.StatusConflict, rec.Code)
	assert.Equal(t, "conflict", payload["error"])

	// 404：更新不存在的菜单
	rec, payload = doMenuRequest(t, r, http.MethodPut, "/api/v1/menus/424242", `{"icon":"X"}`)
	require.Equal(t, http.StatusNotFound, rec.Code)
	assert.Equal(t, "not_found", payload["error"])

	// 403：无 delete 权限
	rec, payload = doMenuRequest(t, r, http.MethodDelete, "/api/v1/menus/1", "")
	require.Equal(t, http.StatusForbidden, rec.Code)
	assert.Equal(t, "forbidden", payload["error"])
}

func TestMenuHandlerAccessibleRoute(t *testing.T) {
	r, _ := newMenuTestRouter(t, "menu:create", "secret:read")

	// 建一个需要 secret:read 的菜单（当前用户有）+ 一个无权限要求的
	rec, _ := doMenuRequest(t, r, http.MethodPost, "/api/v1/menus",
		`{"menuKey":"gated","labels":{"zh-CN":"受限"},"permission":"secret:read"}`)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	rec, _ = doMenuRequest(t, r, http.MethodPost, "/api/v1/menus",
		`{"menuKey":"open","labels":{"zh-CN":"开放"}}`)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	rec, resp := doMenuRequest(t, r, http.MethodGet, "/api/v1/menus/accessible", "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	items, ok := resp["items"].([]any)
	require.True(t, ok)
	require.Len(t, items, 2)
}

func TestMenuHandlerMissingScope(t *testing.T) {
	r, _ := newMenuTestRouterWithScope(t, false, "admin:all")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/menus", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.True(t, strings.Contains(rec.Body.String(), "X-Game-ID"), rec.Body.String())
}
