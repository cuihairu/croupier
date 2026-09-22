// 覆盖目标（B 批）：version_floor_handler.go 各 handler 的错误分支
// （checkVersionFloorManage 权限面、store 未初始化、参数/scope 校验失败、
// 底层存储错误）与 contract_versions_handler.go ContractVersionDiff 的
// scope 失败 / Diff 查询失败分支。复用既有测试基建（newFunctionTestContext、
// setupTestServiceContext、setupFloorDBForHandler、seedTwoContractVersions）。
package function

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// ---- checkVersionFloorManage 权限面基建 ----

// newFloorAdminSvcCtx 在既有测试上下文上补 AdminModel/RoleModel
// （setupTestServiceContext 默认不带，checkVersionFloorManage 需要）。
func newFloorAdminSvcCtx(t *testing.T) *svc.ServiceContext {
	t.Helper()
	svcCtx := setupTestServiceContext(t)
	svcCtx.AdminModel = model.NewAdminModel(svcCtx.DB)
	svcCtx.RoleModel = model.NewRoleModel(svcCtx.DB)
	return svcCtx
}

// seedFloorAdmin 创建管理员并按需绑角色与权限（权限以字符串直写
// role_permissions，HasPermissionID 只比对字符串无需 permissions 表行）。
func seedFloorAdmin(t *testing.T, svcCtx *svc.ServiceContext, username, roleName string, perms []string) {
	t.Helper()
	ctx := context.Background()
	admin := &model.Admin{Username: username, Nickname: username, Status: 1}
	require.NoError(t, svcCtx.AdminModel.Create(ctx, admin, "pw"))
	if roleName != "" {
		role := &model.Role{Name: roleName, Category: "cov"}
		require.NoError(t, svcCtx.RoleModel.Create(ctx, role))
		require.NoError(t, svcCtx.AdminModel.AssignRole(ctx, admin.ID, role.ID))
		if len(perms) > 0 {
			require.NoError(t, svcCtx.RoleModel.ReplacePermissions(ctx, role.ID, perms))
		}
	}
}

// withFloorUser 往请求上下文注入登录名（与生产认证中间件同 key），
// 触发 checkVersionFloorManage 的已认证分支。
func withFloorUser(ctx *gin.Context, username string) {
	ctx.Request = ctx.Request.WithContext(
		context.WithValue(ctx.Request.Context(), "username", username))
}

// ---- versionFloorScope：:id param 缺失 ----

// GET 请求未带 :id param → id is required（versionFloorScope 的空 id 分支）。
func TestVersionFloorCovB_GetMissingID(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := NewHandler(NewService(setupTestServiceContext(t)))
	ctx, rec := newFunctionTestContext(http.MethodGet, "/api/v1/functions/version-floor", "")
	withFloorScope(ctx)

	h.VersionFloorGet(ctx)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "id is required") {
		t.Fatalf("expected id-required error, got %s", rec.Body.String())
	}
}

// ---- RegistryStore 未初始化（各 handler 的降级 500）----

func TestVersionFloorCovB_GetStoreNil(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := NewHandler(NewService(&svc.ServiceContext{}))
	ctx, rec := newFunctionTestContext(http.MethodGet, "/api/v1/functions/player.ban/version-floor", "")
	ctx.Params = gin.Params{{Key: "id", Value: "player.ban"}}
	withFloorScope(ctx)

	h.VersionFloorGet(ctx)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestVersionFloorCovB_PutStoreNil(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := NewHandler(NewService(&svc.ServiceContext{}))
	ctx, rec := newFunctionTestContext(http.MethodPut, "/api/v1/functions/player.ban/version-floor", `{"minVersion":"0.3.0"}`)
	ctx.Params = gin.Params{{Key: "id", Value: "player.ban"}}
	withFloorScope(ctx)

	h.VersionFloorPut(ctx)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestVersionFloorCovB_DeleteStoreNil(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := NewHandler(NewService(&svc.ServiceContext{}))
	ctx, rec := newFunctionTestContext(http.MethodDelete, "/api/v1/functions/player.ban/version-floor", "")
	ctx.Params = gin.Params{{Key: "id", Value: "player.ban"}}
	withFloorScope(ctx)

	h.VersionFloorDelete(ctx)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestVersionFloorCovB_ListStoreNil(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := NewHandler(NewService(&svc.ServiceContext{}))
	ctx, rec := newFunctionTestContext(http.MethodGet, "/api/v1/functions/version-floors", "")
	withFloorScope(ctx)

	h.VersionFloorsList(ctx)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestVersionFloorCovB_BatchStoreNil(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := NewHandler(NewService(&svc.ServiceContext{}))
	ctx, rec := newFunctionTestContext(http.MethodPost, "/api/v1/functions/version-floor/batch",
		`{"functionIds":["player.ban"],"minVersion":"0.3.0"}`)
	withFloorScope(ctx)

	h.VersionFloorBatch(ctx)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d body=%s", rec.Code, rec.Body.String())
	}
}

// ---- checkVersionFloorManage：已认证各分支 ----

// 管理员不存在 → LoadCurrentAdmin 报错 → 写路径 500（而非 403）。
// 同时覆盖 PUT 头部的 checkVersionFloorManage err 转发分支。
func TestVersionFloorCovB_PutAdminLoadError(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := NewHandler(NewService(newFloorAdminSvcCtx(t)))
	ctx, rec := newFunctionTestContext(http.MethodPut, "/api/v1/functions/player.ban/version-floor", `{"minVersion":"0.3.0"}`)
	ctx.Params = gin.Params{{Key: "id", Value: "player.ban"}}
	withFloorScope(ctx)
	withFloorUser(ctx, "ghost-admin")

	h.VersionFloorPut(ctx)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d body=%s", rec.Code, rec.Body.String())
	}
}

// 角色权限展开查询失败（role_permissions 表被移除注入非 NotFound 错误）
// → PermissionIDsFromRoles err → 写路径 500。
func TestVersionFloorCovB_PutPermissionQueryError(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	svcCtx := newFloorAdminSvcCtx(t)
	seedFloorAdmin(t, svcCtx, "permop", "viewer", nil)
	require.NoError(t, svcCtx.DB.Exec("DROP TABLE role_permissions").Error)

	h := NewHandler(NewService(svcCtx))
	ctx, rec := newFunctionTestContext(http.MethodPut, "/api/v1/functions/player.ban/version-floor", `{"minVersion":"0.3.0"}`)
	ctx.Params = gin.Params{{Key: "id", Value: "player.ban"}}
	withFloorScope(ctx)
	withFloorUser(ctx, "permop")

	h.VersionFloorPut(ctx)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d body=%s", rec.Code, rec.Body.String())
	}
}

// admin 级角色（HasAdminRole）→ 放行 → PUT 成功（覆盖放行 return nil 分支）。
func TestVersionFloorCovB_PutAllowedAsAdminRole(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	svcCtx := newFloorAdminSvcCtx(t)
	seedFloorAdmin(t, svcCtx, "adminop", "admin", nil)
	h := NewHandler(NewService(svcCtx))

	ctx, rec := newFunctionTestContext(http.MethodPut, "/api/v1/functions/player.ban/version-floor", `{"minVersion":"0.3.0"}`)
	ctx.Params = gin.Params{{Key: "id", Value: "player.ban"}}
	withFloorScope(ctx)
	withFloorUser(ctx, "adminop")
	h.VersionFloorPut(ctx)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp versionFloorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v body=%s", err, rec.Body.String())
	}
	// updatedBy 回显当前登录名
	if resp.UpdatedBy != "adminop" || resp.MinVersion != "0.3.0" {
		t.Fatalf("expected adminop/0.3.0, got %+v", resp)
	}
}

// functions:manage 权限（HasPermissionID）→ 放行（第二放行变体，同一 if 块）。
func TestVersionFloorCovB_PutAllowedByPermission(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	svcCtx := newFloorAdminSvcCtx(t)
	seedFloorAdmin(t, svcCtx, "fnmanager", "function-manager", []string{"functions:manage"})
	h := NewHandler(NewService(svcCtx))

	ctx, rec := newFunctionTestContext(http.MethodPut, "/api/v1/functions/player.ban/version-floor", `{"minVersion":"1.2.0"}`)
	ctx.Params = gin.Params{{Key: "id", Value: "player.ban"}}
	withFloorScope(ctx)
	withFloorUser(ctx, "fnmanager")
	h.VersionFloorPut(ctx)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
}

// 已认证但既非 admin 角色也无 functions:manage → 403（Forbidden 分支）。
func TestVersionFloorCovB_PutForbidden(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	svcCtx := newFloorAdminSvcCtx(t)
	seedFloorAdmin(t, svcCtx, "viewerop", "viewer", nil)
	h := NewHandler(NewService(svcCtx))

	ctx, rec := newFunctionTestContext(http.MethodPut, "/api/v1/functions/player.ban/version-floor", `{"minVersion":"0.3.0"}`)
	ctx.Params = gin.Params{{Key: "id", Value: "player.ban"}}
	withFloorScope(ctx)
	withFloorUser(ctx, "viewerop")
	h.VersionFloorPut(ctx)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "无权设置函数版本门槛") {
		t.Fatalf("expected forbidden message, got %s", rec.Body.String())
	}
}

// DELETE：权限不足 403（checkVersionFloorManage err 转发分支）。
func TestVersionFloorCovB_DeleteForbidden(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	svcCtx := newFloorAdminSvcCtx(t)
	seedFloorAdmin(t, svcCtx, "viewerdel", "viewer", nil)
	h := NewHandler(NewService(svcCtx))

	ctx, rec := newFunctionTestContext(http.MethodDelete, "/api/v1/functions/player.ban/version-floor", "")
	ctx.Params = gin.Params{{Key: "id", Value: "player.ban"}}
	withFloorScope(ctx)
	withFloorUser(ctx, "viewerdel")
	h.VersionFloorDelete(ctx)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d body=%s", rec.Code, rec.Body.String())
	}
}

// Batch：权限不足 403（checkVersionFloorManage err 转发分支）。
func TestVersionFloorCovB_BatchForbidden(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	svcCtx := newFloorAdminSvcCtx(t)
	seedFloorAdmin(t, svcCtx, "viewerbatch", "viewer", nil)
	h := NewHandler(NewService(svcCtx))

	ctx, rec := newFunctionTestContext(http.MethodPost, "/api/v1/functions/version-floor/batch",
		`{"functionIds":["player.ban"],"minVersion":"0.3.0"}`)
	withFloorScope(ctx)
	withFloorUser(ctx, "viewerbatch")
	h.VersionFloorBatch(ctx)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d body=%s", rec.Code, rec.Body.String())
	}
}

// ---- 其余参数/scope 校验与存储错误分支 ----

// PUT 缺 scope → 400（versionFloorScope err 转发分支）。
func TestVersionFloorCovB_PutMissingScope(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := NewHandler(NewService(setupTestServiceContext(t)))
	ctx, rec := newFunctionTestContext(http.MethodPut, "/api/v1/functions/player.ban/version-floor", `{"minVersion":"0.3.0"}`)
	ctx.Params = gin.Params{{Key: "id", Value: "player.ban"}}

	h.VersionFloorPut(ctx)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "X-Game-ID") {
		t.Fatalf("expected scope error, got %s", rec.Body.String())
	}
}

// PUT 请求体非法 JSON → 400（ShouldBindJSON err 分支）。
func TestVersionFloorCovB_PutMalformedJSON(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := NewHandler(NewService(setupTestServiceContext(t)))
	ctx, rec := newFunctionTestContext(http.MethodPut, "/api/v1/functions/player.ban/version-floor", `{`)
	ctx.Params = gin.Params{{Key: "id", Value: "player.ban"}}
	withFloorScope(ctx)

	h.VersionFloorPut(ctx)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
}

// DELETE 缺 scope → 400（versionFloorScope err 转发分支）。
func TestVersionFloorCovB_DeleteMissingScope(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := NewHandler(NewService(setupTestServiceContext(t)))
	ctx, rec := newFunctionTestContext(http.MethodDelete, "/api/v1/functions/player.ban/version-floor", "")
	ctx.Params = gin.Params{{Key: "id", Value: "player.ban"}}

	h.VersionFloorDelete(ctx)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
}

// DELETE：底层连接关闭后 DeleteFunctionVersionFloor 报错 → 500
// （复用既有 floor 专用库与关连接手法）。
func TestVersionFloorCovB_DeleteStoreError(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	floorDB := setupFloorDBForHandler(t)
	sqlDB, err := floorDB.DB()
	require.NoError(t, err)

	h := NewHandler(NewService(&svc.ServiceContext{RegistryStore: registry.NewStoreWithDB(floorDB)}))

	// 连接存活时播种门槛，关闭后删除路径报错
	require.NoError(t, h.service.SvcCtx().RegistryStore.SetFunctionVersionFloor("g", "e", "player.ban", "0.3.0", "tester"))
	_ = sqlDB.Close()

	ctx, rec := newFunctionTestContext(http.MethodDelete, "/api/v1/functions/player.ban/version-floor", "")
	ctx.Params = gin.Params{{Key: "id", Value: "player.ban"}}
	withFloorScope(ctx)
	h.VersionFloorDelete(ctx)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d body=%s", rec.Code, rec.Body.String())
	}
}

// ---- ContractVersionDiff 缺口分支 ----

// Diff 请求缺 scope → 400（contractVersionsScope err 转发分支）。
func TestContractVersionDiffCovB_MissingScope(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := NewHandler(NewService(setupTestServiceContext(t)))
	ctx, rec := newFunctionTestContext(http.MethodGet, "/api/v1/functions/player.ban/contract-versions/diff?from=1&to=2", "")
	ctx.Params = gin.Params{{Key: "id", Value: "player.ban"}}

	h.ContractVersionDiff(ctx)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "X-Game-ID") {
		t.Fatalf("expected scope error, got %s", rec.Body.String())
	}
}

// Diff 的 from 版本不存在 → logic.Diff 报 NotFound → 404（Diff err 转发分支）。
func TestContractVersionDiffCovB_VersionNotFound(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	svcCtx := setupTestServiceContext(t)
	seedTwoContractVersions(t, svcCtx)
	h := NewHandler(NewService(svcCtx))

	ctx, rec := newFunctionTestContext(http.MethodGet, "/api/v1/functions/player.ban/contract-versions/diff?from=999&to=2", "")
	ctx.Params = gin.Params{{Key: "id", Value: "player.ban"}}
	ctx.Request = ctx.Request.WithContext(
		svc.WithGameScope(ctx.Request.Context(), svc.GameScope{GameID: "g", Env: "e"}))

	h.ContractVersionDiff(ctx)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "版本不存在") {
		t.Fatalf("expected not-found error, got %s", rec.Body.String())
	}
}
