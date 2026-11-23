package handler

// AUD-042 回归：管理域→权限矩阵（least-privilege）统一挂载。
//
// 覆盖三层：
//  1. 目录对齐：矩阵引用的权限码必须全部存在于 configs/permissions.json
//     （即 RBAC 页可勾选授予），防「守卫引用了目录里不存在的码」——那是
//     授予面与守卫面脱节，角色永远拿不到该权限。
//  2. 匹配逻辑：最长前缀、前缀边界（/providers 不吞 /providersX）、未命中
//     放行、读/写动词分侧。
//  3. 行为面：零角色账号 403 permission_denied（此前「仅登录即可操作」的
//     洞已关），超管与持域权限操作员放行，未命中路径直通。

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// ---------------------------------------------------------------------------
// 1) 目录对齐
// ---------------------------------------------------------------------------

func TestManagementPermissionMatrix_CatalogAligned(t *testing.T) {
	raw, err := os.ReadFile("../../configs/permissions.json")
	require.NoError(t, err, "读不到 RBAC 目录 configs/permissions.json")

	var catalog []struct {
		Code string `json:"code"`
	}
	require.NoError(t, json.Unmarshal(raw, &catalog), "permissions.json 必须是合法 JSON 数组")

	codes := make(map[string]bool, len(catalog))
	for _, p := range catalog {
		require.NotEmpty(t, p.Code, "目录出现空 code")
		codes[p.Code] = true
	}

	for _, rule := range managementPermissionMatrix {
		require.NotEmpty(t, rule.prefix, "矩阵出现空 prefix")
		for _, code := range append(append([]string{}, rule.read...), rule.write...) {
			require.True(t, codes[code],
				"矩阵前缀 %s 引用的权限码 %q 不在 configs/permissions.json（RBAC 页无法授予，守卫永远拒）",
				rule.prefix, code)
		}
	}
}

// ---------------------------------------------------------------------------
// 2) 匹配逻辑
// ---------------------------------------------------------------------------

func TestMatchPermissionMatrixRule_LongestPrefix(t *testing.T) {
	rule := matchPermissionMatrixRule("/api/v1/admin/announcements")
	require.NotNil(t, rule)
	assert.Equal(t, "/api/v1/admin/announcements", rule.prefix)

	// /api/v1/messages/broadcast 必须命中更具体的 broadcast 规则而不是
	// 不存在 /api/v1/messages 规则（自服务消息面未入表）。
	rule = matchPermissionMatrixRule("/api/v1/messages/broadcast")
	require.NotNil(t, rule)
	assert.Equal(t, "/api/v1/messages/broadcast", rule.prefix)
}

func TestMatchPermissionMatrixRule_Boundary(t *testing.T) {
	// 前缀边界：/api/v1/providers 不得吞掉 /api/v1/providersX。
	assert.Nil(t, matchPermissionMatrixRule("/api/v1/providersX"), "前缀边界必须拒绝相邻路径")
	// 子路径命中（含 :id 段），参数化路由用 FullPath（/api/v1/providers/:id）。
	require.NotNil(t, matchPermissionMatrixRule("/api/v1/providers/:id/reload"))
	// 未命中（公开/自服务/既已服务层门禁）放行。
	assert.Nil(t, matchPermissionMatrixRule("/api/v1/messages/stream"))
	assert.Nil(t, matchPermissionMatrixRule("/api/v1/backups"))
	assert.Nil(t, matchPermissionMatrixRule("/api/v1/admin/users"))
}

func TestIsReadVerb(t *testing.T) {
	assert.True(t, isReadVerb(http.MethodGet))
	assert.True(t, isReadVerb(http.MethodHead))
	assert.True(t, isReadVerb(http.MethodOptions))
	assert.False(t, isReadVerb(http.MethodPost))
	assert.False(t, isReadVerb(http.MethodPut))
	assert.False(t, isReadVerb(http.MethodDelete))
	assert.False(t, isReadVerb(http.MethodPatch))
}

// ---------------------------------------------------------------------------
// 3) 行为面（gin + 真实 svcCtx + 种子账号）
// ---------------------------------------------------------------------------

const (
	matrixSuperAdmin = "matrix_superadmin"
	matrixOperator   = "matrix_provider_operator" // 仅持 providers:write
	matrixViewer     = "matrix_viewer"            // 零角色
)

var (
	matrixDBOnce sync.Once
	matrixDBPtr  *gorm.DB
)

func matrixDB(t *testing.T) *gorm.DB {
	t.Helper()
	matrixDBOnce.Do(func() {
		db, err := gorm.Open(gsqlite.Open("file:aud042_matrix?mode=memory&cache=shared"), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Silent),
		})
		require.NoError(t, err)
		require.NoError(t, model.AutoMigrate(db))
		seedMatrixSuperAdmin(t, db)
		seedMatrixOperator(t, db)
		seedMatrixViewer(t, db)
		matrixDBPtr = db
	})
	return matrixDBPtr
}

func seedMatrixSuperAdmin(t *testing.T, db *gorm.DB) {
	t.Helper()
	ctx := context.Background()
	adminModel := model.NewAdminModel(db)
	roleModel := model.NewRoleModel(db)

	admin, err := adminModel.FindByUsername(ctx, matrixSuperAdmin)
	if err != nil {
		admin = &model.Admin{Username: matrixSuperAdmin, Status: 1}
		require.NoError(t, adminModel.Create(ctx, admin, "password123"))
	}
	has := false
	roles, err := adminModel.GetAdminRoles(ctx, admin.ID)
	require.NoError(t, err)
	for _, r := range roles {
		if r.Name == "super_admin" {
			has = true
		}
	}
	if !has {
		role := &model.Role{Name: "super_admin"}
		require.NoError(t, roleModel.Create(ctx, role))
		require.NoError(t, adminModel.AssignRole(ctx, admin.ID, role.ID))
	}
}

func seedMatrixOperator(t *testing.T, db *gorm.DB) {
	t.Helper()
	ctx := context.Background()
	adminModel := model.NewAdminModel(db)
	roleModel := model.NewRoleModel(db)

	admin, err := adminModel.FindByUsername(ctx, matrixOperator)
	if err != nil {
		admin = &model.Admin{Username: matrixOperator, Status: 1}
		require.NoError(t, adminModel.Create(ctx, admin, "password123"))
	}
	has := false
	roles, err := adminModel.GetAdminRoles(ctx, admin.ID)
	require.NoError(t, err)
	for _, r := range roles {
		if r.Name == "matrix_provider_operator" {
			has = true
		}
	}
	if !has {
		role := &model.Role{Name: "matrix_provider_operator"}
		require.NoError(t, roleModel.Create(ctx, role))
		require.NoError(t, adminModel.AssignRole(ctx, admin.ID, role.ID))
	}
	roles, err = adminModel.GetAdminRoles(ctx, admin.ID)
	require.NoError(t, err)
	for _, r := range roles {
		if r.Name == "matrix_provider_operator" {
			require.NoError(t, roleModel.ReplacePermissions(ctx, r.ID, []string{"providers:write"}))
			break
		}
	}
}

func seedMatrixViewer(t *testing.T, db *gorm.DB) {
	t.Helper()
	ctx := context.Background()
	adminModel := model.NewAdminModel(db)
	admin, err := adminModel.FindByUsername(ctx, matrixViewer)
	if err != nil {
		admin = &model.Admin{Username: matrixViewer, Status: 1}
		require.NoError(t, adminModel.Create(ctx, admin, "password123"))
	}
}

// matrixSvcCtx 返回带种子身份面（Admin/Role/DB）的 svcCtx。
func matrixSvcCtx(t *testing.T) *svc.ServiceContext {
	t.Helper()
	db := matrixDB(t)
	return &svc.ServiceContext{
		DB:         db,
		AdminModel: model.NewAdminModel(db),
		RoleModel:  model.NewRoleModel(db),
	}
}

// matrixIdentityFromHeader 是注入 username 的测试中间件（与生产 Authority
// 的「username」约定键一致）；用户名经 X-Test-Username 头逐请求注入。
func matrixIdentityFromHeader() gin.HandlerFunc {
	return func(c *gin.Context) {
		username := c.GetHeader("X-Test-Username")
		if username != "" {
			ctx := context.WithValue(c.Request.Context(), "username", username)
			c.Request = c.Request.WithContext(ctx)
		}
		c.Next()
	}
}

// newMatrixEngine 挂矩阵守卫 + 一组与真实前缀同形的测试路由。身份经
// X-Test-Username 头注入（模拟 Authority 已认证）。
func newMatrixEngine(t *testing.T, svc *svc.ServiceContext) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(matrixIdentityFromHeader())
	r.Use(permissionMatrixGuard(svc))
	r.GET("/api/v1/providers", func(c *gin.Context) { c.Status(http.StatusOK) })
	r.POST("/api/v1/providers/:id/reload", func(c *gin.Context) { c.Status(http.StatusOK) })
	r.GET("/api/v1/messages/stream", func(c *gin.Context) { c.Status(http.StatusOK) })  // 自服务面
	r.GET("/api/v1/backups", func(c *gin.Context) { c.Status(http.StatusOK) })          // 已服务层门禁
	r.GET("/api/v1/monitoring/nodes", func(c *gin.Context) { c.Status(http.StatusOK) }) // 未命中
	return r
}

func doRequest(t *testing.T, r *gin.Engine, method, path, username string) int {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	req.Header.Set("X-Test-Username", username)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec.Code
}

func TestPermissionMatrixGuard_ZeroRoleDenied(t *testing.T) {
	r := newMatrixEngine(t, matrixSvcCtx(t))
	// 写面：零角色此前「仅登录即可重载 provider」，现必须 403。
	assert.Equal(t, http.StatusForbidden, doRequest(t, r, http.MethodPost, "/api/v1/providers/7/reload", matrixViewer))
	// 读面同样 403（least-privilege 读也门禁）。
	assert.Equal(t, http.StatusForbidden, doRequest(t, r, http.MethodGet, "/api/v1/providers", matrixViewer))
}

func TestPermissionMatrixGuard_SuperAdminAllowed(t *testing.T) {
	r := newMatrixEngine(t, matrixSvcCtx(t))
	assert.Equal(t, http.StatusOK, doRequest(t, r, http.MethodPost, "/api/v1/providers/7/reload", matrixSuperAdmin))
	assert.Equal(t, http.StatusOK, doRequest(t, r, http.MethodGet, "/api/v1/providers", matrixSuperAdmin))
}

func TestPermissionMatrixGuard_DomainPermissionGrantsWriteNotRead(t *testing.T) {
	r := newMatrixEngine(t, matrixSvcCtx(t))
	// 仅持 providers:write：写放行。
	assert.Equal(t, http.StatusOK, doRequest(t, r, http.MethodPost, "/api/v1/providers/7/reload", matrixOperator))
	// 读面需要 providers:read，操作员未持 → 403。
	assert.Equal(t, http.StatusForbidden, doRequest(t, r, http.MethodGet, "/api/v1/providers", matrixOperator))
}

func TestPermissionMatrixGuard_UnmatchedRoutesPassThrough(t *testing.T) {
	r := newMatrixEngine(t, matrixSvcCtx(t))
	// 未命中矩阵的路径（自服务消息、已服务层门禁的 backup、monitoring）
	// 不受矩阵二次门禁，零角色也直通（其安全由既有认证/服务层守卫负责）。
	assert.Equal(t, http.StatusOK, doRequest(t, r, http.MethodGet, "/api/v1/messages/stream", matrixViewer))
	assert.Equal(t, http.StatusOK, doRequest(t, r, http.MethodGet, "/api/v1/backups", matrixViewer))
	assert.Equal(t, http.StatusOK, doRequest(t, r, http.MethodGet, "/api/v1/monitoring/nodes", matrixViewer))
}
