// Coverage targets (batch B): the error branches of the handlers in
// version_floor_handler.go (the checkVersionFloorManage permission surface,
// store not initialized, parameter/scope validation failures, underlying store
// errors) plus the scope-failure / Diff query-failure branches of
// ContractVersionDiff in contract_versions_handler.go. Reuses the existing
// test infrastructure (newFunctionTestContext, setupTestServiceContext,
// setupFloorDBForHandler, seedTwoContractVersions).
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

// ---- checkVersionFloorManage permission-surface infrastructure ----

// newFloorAdminSvcCtx adds AdminModel/RoleModel on top of the existing test
// context (setupTestServiceContext omits them by default;
// checkVersionFloorManage needs them).
func newFloorAdminSvcCtx(t *testing.T) *svc.ServiceContext {
	t.Helper()
	svcCtx := setupTestServiceContext(t)
	svcCtx.AdminModel = model.NewAdminModel(svcCtx.DB)
	svcCtx.RoleModel = model.NewRoleModel(svcCtx.DB)
	return svcCtx
}

// seedFloorAdmin creates an admin and binds a role and permissions as needed
// (permissions are written as strings straight into role_permissions, and
// HasPermissionID compares strings only, so no permissions table rows are
// required).
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

// withFloorUser injects the login name into the request context (same key as
// the production auth middleware), triggering the authenticated branch of
// checkVersionFloorManage.
func withFloorUser(ctx *gin.Context, username string) {
	ctx.Request = ctx.Request.WithContext(
		context.WithValue(ctx.Request.Context(), "username", username))
}

// ---- versionFloorScope: missing :id param ----

// A GET request without a :id param -> id is required (the empty-id branch of
// versionFloorScope).
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

// ---- RegistryStore not initialized (each handler's degraded 500) ----

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

// ---- checkVersionFloorManage: authenticated branches ----

// Admin does not exist -> LoadCurrentAdmin errors -> write path 500 (not
// 403). Also covers the checkVersionFloorManage err-forwarding branch at the
// head of PUT.
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

// The role-permission expansion query fails (the role_permissions table is
// dropped to inject a non-NotFound error) -> PermissionIDsFromRoles err ->
// write path 500.
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

// Admin-level role (HasAdminRole) -> allowed -> PUT succeeds (covers the
// allow return nil branch).
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
	// updatedBy echoes the current login name.
	if resp.UpdatedBy != "adminop" || resp.MinVersion != "0.3.0" {
		t.Fatalf("expected adminop/0.3.0, got %+v", resp)
	}
}

// functions:manage permission (HasPermissionID) -> allowed (a second allow
// variant, the same if block).
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

// Authenticated but neither an admin role nor functions:manage -> 403
// (Forbidden branch).
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

// DELETE: insufficient permission 403 (checkVersionFloorManage err-forwarding
// branch).
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

// Batch: insufficient permission 403 (checkVersionFloorManage err-forwarding
// branch).
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

// ---- Remaining parameter/scope validation and store error branches ----

// PUT missing scope -> 400 (versionFloorScope err-forwarding branch).
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

// PUT with malformed JSON body -> 400 (ShouldBindJSON err branch).
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

// DELETE missing scope -> 400 (versionFloorScope err-forwarding branch).
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

// DELETE: after the underlying connection is closed,
// DeleteFunctionVersionFloor errors -> 500 (reusing the established
// floor-specific DB and closed-connection technique).
func TestVersionFloorCovB_DeleteStoreError(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	floorDB := setupFloorDBForHandler(t)
	sqlDB, err := floorDB.DB()
	require.NoError(t, err)

	h := NewHandler(NewService(&svc.ServiceContext{RegistryStore: registry.NewStoreWithDB(floorDB)}))

	// Seed the floor while the connection is alive; the delete path errors
	// after it is closed.
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

// ---- ContractVersionDiff gap branches ----

// Diff request missing scope -> 400 (contractVersionsScope err-forwarding
// branch).
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

// Diff's from version does not exist -> logic.Diff reports NotFound -> 404
// (Diff err-forwarding branch).
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
