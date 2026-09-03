package utils

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/service/permission"
	"github.com/cuihairu/croupier/internal/svc"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// ---------------------------------------------------------------------------
// json_boundary.go
// ---------------------------------------------------------------------------

func TestRawJSONFromValueV9(t *testing.T) {
	t.Run("nil value", func(t *testing.T) {
		assert.Nil(t, rawJSONFromValue(nil))
	})

	t.Run("json.RawMessage valid", func(t *testing.T) {
		out := rawJSONFromValue(json.RawMessage(`{"a":1}`))
		assert.Equal(t, `{"a":1}`, string(out))
	})

	t.Run("json.RawMessage invalid becomes string", func(t *testing.T) {
		out := rawJSONFromValue(json.RawMessage(`not-json`))
		assert.Equal(t, `"not-json"`, string(out))
	})

	t.Run("bytes valid", func(t *testing.T) {
		out := rawJSONFromValue([]byte(`[1,2]`))
		assert.Equal(t, `[1,2]`, string(out))
	})

	t.Run("string valid", func(t *testing.T) {
		out := rawJSONFromValue(`"hello"`)
		assert.Equal(t, `"hello"`, string(out))
	})

	t.Run("string invalid becomes encoded string", func(t *testing.T) {
		out := rawJSONFromValue(`oops`)
		assert.Equal(t, `"oops"`, string(out))
	})

	t.Run("map marshalled", func(t *testing.T) {
		out := rawJSONFromValue(map[string]interface{}{"k": "v"})
		assert.JSONEq(t, `{"k":"v"}`, string(out))
	})

	t.Run("marshal error returns nil", func(t *testing.T) {
		assert.Nil(t, rawJSONFromValue(struct{ C chan int }{C: make(chan int)}))
	})
}

func TestRawJSONFromBytesV9(t *testing.T) {
	t.Run("empty returns nil", func(t *testing.T) {
		assert.Nil(t, rawJSONFromBytes(nil))
		assert.Nil(t, rawJSONFromBytes([]byte{}))
	})

	t.Run("valid json passthrough", func(t *testing.T) {
		out := rawJSONFromBytes([]byte(`{"x":true}`))
		assert.Equal(t, `{"x":true}`, string(out))
	})

	t.Run("invalid json encoded as string", func(t *testing.T) {
		out := rawJSONFromBytes([]byte(`raw text`))
		assert.Equal(t, `"raw text"`, string(out))
	})
}

// ---------------------------------------------------------------------------
// date_helpers.go: NormalizeDateRange error paths
// ---------------------------------------------------------------------------

func TestNormalizeDateRangeInvalidStartV9(t *testing.T) {
	_, _, err := NormalizeDateRange("not-a-date", "2024-01-02")
	assert.Error(t, err)
}

func TestNormalizeDateRangeInvalidEndV9(t *testing.T) {
	_, _, err := NormalizeDateRange("2024-01-01", "bogus")
	assert.Error(t, err)
}

func TestNormalizeDateRangeSwapV9(t *testing.T) {
	start, end, err := NormalizeDateRange("2024-01-05", "2024-01-01")
	require.NoError(t, err)
	assert.True(t, start.Before(end))
}

func TestNormalizeDateRangeRFC3339NoExpansionV9(t *testing.T) {
	startRaw := "2024-01-01T10:00:00Z"
	endRaw := "2024-01-01T12:00:00Z"
	start, end, err := NormalizeDateRange(startRaw, endRaw)
	require.NoError(t, err)
	assert.Equal(t, 12, end.UTC().Hour())
	assert.True(t, start.Before(end))
}

// ---------------------------------------------------------------------------
// analytics_filters.go: MkdirAll failure
// ---------------------------------------------------------------------------

func TestWriteAnalyticsFiltersFileMkdirErrorV9(t *testing.T) {
	tmpDir := t.TempDir()
	blocker := filepath.Join(tmpDir, "blocker")
	require.NoError(t, os.WriteFile(blocker, []byte("x"), 0o644))

	err := WriteAnalyticsFiltersFile(filepath.Join(blocker, "sub", "filters.json"), []byte(`{}`))
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// function_acl.go: FunctionActionAllowed branch matrix
// ---------------------------------------------------------------------------

func TestFunctionActionAllowedBranchesV9(t *testing.T) {
	perm := func(actions, roles string) model.FunctionPermission {
		return model.FunctionPermission{Actions: mustEncodeJSON(actions), Roles: mustEncodeJSON(roles)}
	}

	t.Run("empty action no rule", func(t *testing.T) {
		allowed, hasRule := FunctionActionAllowed([]string{"user"}, []model.FunctionPermission{perm(`["invoke"]`, `["user"]`)}, "   ", "", "")
		assert.False(t, allowed)
		assert.False(t, hasRule)
	})

	t.Run("invoke synonyms", func(t *testing.T) {
		for _, syn := range []string{"execute", "call", "run", "*"} {
			allowed, hasRule := FunctionActionAllowed([]string{"op"}, []model.FunctionPermission{perm(`["`+syn+`"]`, `["op"]`)}, "invoke", "", "")
			assert.True(t, allowed, "synonym %s should match invoke", syn)
			assert.True(t, hasRule)
		}
	})

	t.Run("read synonyms", func(t *testing.T) {
		for _, syn := range []string{"list", "view"} {
			allowed, hasRule := FunctionActionAllowed([]string{"op"}, []model.FunctionPermission{perm(`["`+syn+`"]`, `["op"]`)}, "read", "", "")
			assert.True(t, allowed, "synonym %s should match read", syn)
			assert.True(t, hasRule)
		}
	})

	t.Run("blank action entries ignored", func(t *testing.T) {
		allowed, hasRule := FunctionActionAllowed([]string{"op"}, []model.FunctionPermission{perm(`["", "  ", "invoke"]`, `["op"]`)}, "invoke", "", "")
		assert.True(t, allowed)
		assert.True(t, hasRule)
	})

	t.Run("rule exists but role mismatch", func(t *testing.T) {
		allowed, hasRule := FunctionActionAllowed([]string{"op"}, []model.FunctionPermission{perm(`["invoke"]`, `["other"]`)}, "invoke", "", "")
		assert.False(t, allowed)
		assert.True(t, hasRule)
	})

	t.Run("roles admin or super_admin always match", func(t *testing.T) {
		for _, r := range []string{"admin", "super_admin"} {
			allowed, _ := FunctionActionAllowed([]string{"op"}, []model.FunctionPermission{perm(`["invoke"]`, `["`+r+`"]`)}, "invoke", "", "")
			assert.True(t, allowed, "role %s should match", r)
		}
	})

	t.Run("blank role entries ignored", func(t *testing.T) {
		allowed, hasRule := FunctionActionAllowed([]string{"op"}, []model.FunctionPermission{perm(`["invoke"]`, `["", "  ", "op"]`)}, "invoke", "", "")
		assert.True(t, allowed)
		assert.True(t, hasRule)
	})

	t.Run("blank role names ignored", func(t *testing.T) {
		allowed, hasRule := FunctionActionAllowed([]string{"", "  ", "op"}, []model.FunctionPermission{perm(`["invoke"]`, `["op"]`)}, "invoke", "", "")
		assert.True(t, allowed)
		assert.True(t, hasRule)
	})

	t.Run("game scoped rule mismatch skipped", func(t *testing.T) {
		allowed, hasRule := FunctionActionAllowed([]string{"op"}, []model.FunctionPermission{
			{GameID: "other", Actions: mustEncodeJSON(`["invoke"]`), Roles: mustEncodeJSON(`["op"]`)},
		}, "invoke", "demo", "")
		assert.False(t, allowed)
		assert.False(t, hasRule)
	})

	t.Run("game scoped rule with empty caller game skipped", func(t *testing.T) {
		allowed, hasRule := FunctionActionAllowed([]string{"op"}, []model.FunctionPermission{
			{GameID: "demo", Actions: mustEncodeJSON(`["invoke"]`), Roles: mustEncodeJSON(`["op"]`)},
		}, "invoke", "", "")
		assert.False(t, allowed)
		assert.False(t, hasRule)
	})

	t.Run("game scoped rule equal-fold match", func(t *testing.T) {
		allowed, hasRule := FunctionActionAllowed([]string{"op"}, []model.FunctionPermission{
			{GameID: "DEMO", Actions: mustEncodeJSON(`["invoke"]`), Roles: mustEncodeJSON(`["op"]`)},
		}, "invoke", "demo", "")
		assert.True(t, allowed)
		assert.True(t, hasRule)
	})

	t.Run("env scoped rule mismatch skipped", func(t *testing.T) {
		allowed, hasRule := FunctionActionAllowed([]string{"op"}, []model.FunctionPermission{
			{Env: "stage", Actions: mustEncodeJSON(`["invoke"]`), Roles: mustEncodeJSON(`["op"]`)},
		}, "invoke", "", "prod")
		assert.False(t, allowed)
		assert.False(t, hasRule)
	})

	t.Run("env scoped rule with empty caller env skipped", func(t *testing.T) {
		allowed, hasRule := FunctionActionAllowed([]string{"op"}, []model.FunctionPermission{
			{Env: "prod", Actions: mustEncodeJSON(`["invoke"]`), Roles: mustEncodeJSON(`["op"]`)},
		}, "invoke", "", "")
		assert.False(t, allowed)
		assert.False(t, hasRule)
	})

	t.Run("env scoped rule equal-fold match", func(t *testing.T) {
		allowed, hasRule := FunctionActionAllowed([]string{"op"}, []model.FunctionPermission{
			{Env: "PROD", Actions: mustEncodeJSON(`["invoke"]`), Roles: mustEncodeJSON(`["op"]`)},
		}, "invoke", "", "prod")
		assert.True(t, allowed)
		assert.True(t, hasRule)
	})

	t.Run("action mismatch skips rule", func(t *testing.T) {
		allowed, hasRule := FunctionActionAllowed([]string{"op"}, []model.FunctionPermission{perm(`["read"]`, `["op"]`)}, "invoke", "", "")
		assert.False(t, allowed)
		assert.False(t, hasRule)
	})
}

// ---------------------------------------------------------------------------
// id_helpers.go / role_helpers.go overflow
// ---------------------------------------------------------------------------

func TestParseUintIDOverflowV9(t *testing.T) {
	_, err := ParseUintID("18446744073709551616", "ID")
	assert.Error(t, err)
}

func TestParseRoleIDOverflowV9(t *testing.T) {
	_, err := ParseRoleID("18446744073709551616")
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// permission_guard.go: appendPermissionIDs case-insensitive dedup
// ---------------------------------------------------------------------------

func TestAppendPermissionIDsCaseInsensitiveV9(t *testing.T) {
	out := appendPermissionIDs([]string{"Functions.Read", "  functions.read  "}, "FUNCTIONS.READ", "new:perm")
	assert.Equal(t, []string{"Functions.Read", "new:perm"}, out)
}

func TestAppendPermissionIDsBlankBaseEntriesV9(t *testing.T) {
	out := appendPermissionIDs([]string{"", "   ", "real:perm"}, "added:perm")
	assert.Equal(t, []string{"real:perm", "added:perm"}, out)
}

func TestLoadCurrentAdminRolesErrorV9(t *testing.T) {
	svcCtx, ctx := setupUtilsTestContext(t)
	require.NoError(t, svcCtx.DB.Migrator().DropTable("admin_roles"))
	_, _, err := LoadCurrentAdmin(ctx, svcCtx)
	assert.Error(t, err)
}

func TestPermissionIDsFromRolesBlankEntriesV9(t *testing.T) {
	svcCtx, ctx := setupUtilsTestContext(t)
	_, roles, err := LoadCurrentAdmin(ctx, svcCtx)
	require.NoError(t, err)
	require.NotEmpty(t, roles)

	require.NoError(t, svcCtx.DB.Create(&model.RolePermission{RoleID: roles[0].ID, PermissionID: "  "}).Error)
	permIDs, err := PermissionIDsFromRoles(ctx, svcCtx, roles)
	require.NoError(t, err)
	assert.NotContains(t, permIDs, "  ")
}

// ---------------------------------------------------------------------------
// profile_helpers.go / permission_ids.go guards
// ---------------------------------------------------------------------------

func TestLoadCurrentAdminNilGuardsV9(t *testing.T) {
	_, _, err := LoadCurrentAdmin(context.Background(), nil)
	assert.Error(t, err)

	_, _, err = LoadCurrentAdmin(context.Background(), &svc.ServiceContext{})
	assert.Error(t, err)
}

func TestPermissionIDsFromRolesGuardsV9(t *testing.T) {
	ctx := context.Background()

	permIDs, err := PermissionIDsFromRoles(ctx, nil, []model.Role{{Name: "admin"}})
	require.NoError(t, err)
	assert.Empty(t, permIDs)

	permIDs, err = PermissionIDsFromRoles(ctx, &svc.ServiceContext{}, []model.Role{{Name: "admin"}})
	require.NoError(t, err)
	assert.Empty(t, permIDs)
}

func TestPermissionIDsFromRolesZeroIDSkippedV9(t *testing.T) {
	svcCtx, ctx := setupUtilsTestContext(t)
	permIDs, err := PermissionIDsFromRoles(ctx, svcCtx, []model.Role{{Name: "admin"}})
	require.NoError(t, err)
	assert.Empty(t, permIDs)
}

func TestPermissionIDsFromRolesQueryErrorV9(t *testing.T) {
	svcCtx, ctx := setupUtilsTestContext(t)
	_, roles, err := LoadCurrentAdmin(ctx, svcCtx)
	require.NoError(t, err)
	require.NotEmpty(t, roles)

	require.NoError(t, svcCtx.DB.Migrator().DropTable("role_permissions"))
	_, err = PermissionIDsFromRoles(ctx, svcCtx, roles)
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// RequireAnyPermission
// ---------------------------------------------------------------------------

func setupNonAdminContextV9(t *testing.T, perms []string) (*svc.ServiceContext, context.Context) {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))

	adminModel := model.NewAdminModel(db)
	roleModel := model.NewRoleModel(db)

	admin := &model.Admin{Username: "plainuser", Status: 1}
	require.NoError(t, adminModel.Create(context.Background(), admin, "password123"))

	role := &model.Role{Name: "operator", Description: "operator"}
	require.NoError(t, roleModel.Create(context.Background(), role))
	require.NoError(t, adminModel.AssignRole(context.Background(), admin.ID, role.ID))
	if len(perms) > 0 {
		require.NoError(t, roleModel.ReplacePermissions(context.Background(), role.ID, perms))
	}

	svcCtx := &svc.ServiceContext{
		DB:              db,
		AdminModel:      adminModel,
		RoleModel:       roleModel,
		PermissionModel: model.NewPermissionModel(db),
	}
	ctx := context.WithValue(context.Background(), "username", "plainuser")
	return svcCtx, ctx
}

func TestRequireAnyPermissionDeniedV9(t *testing.T) {
	svcCtx, ctx := setupNonAdminContextV9(t, []string{"player:read"})

	roles, permIDs, err := RequireAnyPermission(ctx, svcCtx, "", "functions:manage")
	assert.Error(t, err)
	assert.NotEmpty(t, roles)
	assert.NotEmpty(t, permIDs)
	codeErr, ok := err.(*errorx.CodeError)
	require.True(t, ok)
	assert.Equal(t, 403, codeErr.Code)
}

func TestRequireAnyPermissionDeniedDefaultMessageV9(t *testing.T) {
	svcCtx, ctx := setupNonAdminContextV9(t, nil)
	_, _, err := RequireAnyPermission(ctx, svcCtx, "   ")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "无权执行该操作")
}

func TestRequireAnyPermissionAllowedV9(t *testing.T) {
	svcCtx, ctx := setupNonAdminContextV9(t, []string{"functions:manage"})
	roles, permIDs, err := RequireAnyPermission(ctx, svcCtx, "denied", "functions:manage", "other:read")
	require.NoError(t, err)
	assert.NotEmpty(t, roles)
	assert.Contains(t, permIDs, "functions:manage")
}

func TestRequireAnyPermissionAdminLoadFailsV9(t *testing.T) {
	svcCtx, _ := setupNonAdminContextV9(t, nil)
	_, _, err := RequireAnyPermission(context.Background(), svcCtx, "denied", "functions:manage")
	assert.Error(t, err)
}

func TestRequireAnyPermissionPermIDQueryFailsV9(t *testing.T) {
	svcCtx, ctx := setupNonAdminContextV9(t, nil)
	require.NoError(t, svcCtx.DB.Migrator().DropTable("role_permissions"))
	_, _, err := RequireAnyPermission(ctx, svcCtx, "denied", "functions:manage")
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// CheckInvokePermission
// ---------------------------------------------------------------------------

func setupInvokePermContextV9(t *testing.T) *svc.ServiceContext {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))
	return &svc.ServiceContext{
		DB:            db,
		FunctionModel: model.NewFunctionModel(db),
	}
}

func TestCheckInvokePermissionAdminRoleV9(t *testing.T) {
	svcCtx := setupInvokePermContextV9(t)
	require.NoError(t, svcCtx.FunctionModel.ReplacePermissions(context.Background(), "fn.1", []model.FunctionPermission{
		{FunctionID: "fn.1", Actions: mustEncodeJSON(`["invoke"]`), Roles: mustEncodeJSON(`["nobody"]`)},
	}))
	require.NoError(t, CheckInvokePermission(context.Background(), nil, []string{"admin"}, nil, "fn.1", "", ""))
}

func TestCheckInvokePermissionNilSvcCtxV9(t *testing.T) {
	err := CheckInvokePermission(context.Background(), nil, []string{"user"}, nil, "fn.1", "", "")
	require.Error(t, err)
	codeErr, ok := err.(*errorx.CodeError)
	require.True(t, ok)
	assert.Equal(t, 403, codeErr.Code)
}

func TestCheckInvokePermissionNilFunctionModelV9(t *testing.T) {
	err := CheckInvokePermission(context.Background(), &svc.ServiceContext{}, []string{"user"}, nil, "fn.1", "", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "函数权限模型未初始化")
}

func TestCheckInvokePermissionRuleAllowsV9(t *testing.T) {
	svcCtx := setupInvokePermContextV9(t)
	require.NoError(t, svcCtx.FunctionModel.ReplacePermissions(context.Background(), "fn.1", []model.FunctionPermission{
		{FunctionID: "fn.1", Actions: mustEncodeJSON(`["invoke"]`), Roles: mustEncodeJSON(`["operator"]`)},
	}))
	require.NoError(t, CheckInvokePermission(context.Background(), svcCtx, []string{"operator"}, nil, "fn.1", "", ""))
}

func TestCheckInvokePermissionRuleDeniesV9(t *testing.T) {
	svcCtx := setupInvokePermContextV9(t)
	require.NoError(t, svcCtx.FunctionModel.ReplacePermissions(context.Background(), "fn.1", []model.FunctionPermission{
		{FunctionID: "fn.1", Actions: mustEncodeJSON(`["invoke"]`), Roles: mustEncodeJSON(`["other"]`)},
	}))
	err := CheckInvokePermission(context.Background(), svcCtx, []string{"operator"}, nil, "fn.1", "", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "无权调用该函数")
}

func TestCheckInvokePermissionListErrorV9(t *testing.T) {
	svcCtx := setupInvokePermContextV9(t)
	require.NoError(t, svcCtx.DB.Migrator().DropTable("function_permissions"))
	err := CheckInvokePermission(context.Background(), svcCtx, []string{"operator"}, nil, "fn.1", "", "")
	assert.Error(t, err)
}

func TestCheckInvokePermissionDefaultPolicyV9(t *testing.T) {
	svcCtx := setupInvokePermContextV9(t)

	require.NoError(t, CheckInvokePermission(context.Background(), svcCtx, []string{"operator"}, []string{"*"}, "fn.1", "", ""))
	require.NoError(t, CheckInvokePermission(context.Background(), svcCtx, []string{"operator"}, []string{"function:invoke"}, "fn.1", "", ""))

	err := CheckInvokePermission(context.Background(), svcCtx, []string{"operator"}, []string{"player:read"}, "fn.1", "", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "function:invoke")
}

// ---------------------------------------------------------------------------
// RequireGameEnvScope
// ---------------------------------------------------------------------------

func setupGameScopeContextV9(t *testing.T) (*svc.ServiceContext, *model.Game) {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))

	gameModel := model.NewGameModel(db)
	adminModel := model.NewAdminModel(db)
	game := &model.Game{GameID: "demo", Name: "Demo", Enabled: true}
	require.NoError(t, gameModel.Create(context.Background(), game))

	admin := &model.Admin{Username: "scopeduser", Status: 1}
	require.NoError(t, adminModel.Create(context.Background(), admin, "password123"))

	svcCtx := &svc.ServiceContext{
		DB:                db,
		GameModel:         gameModel,
		AdminModel:        adminModel,
		RoleModel:         model.NewRoleModel(db),
		PermissionModel:   model.NewPermissionModel(db),
		PermissionService: permission.NewPermissionService(db),
	}
	return svcCtx, game
}

func TestRequireGameEnvScopeAdminRoleV9(t *testing.T) {
	_, game := setupGameScopeContextV9(t)
	require.NoError(t, RequireGameEnvScope(context.Background(), nil, 1, []string{"admin"}, "", ""))
	_ = game
}

func TestRequireGameEnvScopeMissingGameIDV9(t *testing.T) {
	svcCtx, _ := setupGameScopeContextV9(t)
	err := RequireGameEnvScope(context.Background(), svcCtx, 1, []string{"user"}, "   ", "prod")
	require.Error(t, err)
	codeErr, ok := err.(*errorx.CodeError)
	require.True(t, ok)
	assert.Equal(t, 400, codeErr.Code)
}

func TestRequireGameEnvScopeMissingEnvV9(t *testing.T) {
	svcCtx, _ := setupGameScopeContextV9(t)
	err := RequireGameEnvScope(context.Background(), svcCtx, 1, []string{"user"}, "demo", "  ")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "env is required")
}

func TestRequireGameEnvScopeNilSvcCtxV9(t *testing.T) {
	err := RequireGameEnvScope(context.Background(), &svc.ServiceContext{}, 1, []string{"user"}, "demo", "prod")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "scope checker not initialized")
}

func TestRequireGameEnvScopeGameNotFoundV9(t *testing.T) {
	svcCtx, _ := setupGameScopeContextV9(t)
	err := RequireGameEnvScope(context.Background(), svcCtx, 1, []string{"user"}, "missing-game", "prod")
	require.Error(t, err)
	codeErr, ok := err.(*errorx.CodeError)
	require.True(t, ok)
	assert.Equal(t, 404, codeErr.Code)
}

func TestRequireGameEnvScopeEnvScopeAllowedV9(t *testing.T) {
	svcCtx, game := setupGameScopeContextV9(t)
	admin, err := svcCtx.AdminModel.FindByUsername(context.Background(), "scopeduser")
	require.NoError(t, err)
	require.NoError(t, svcCtx.AdminModel.SetGameEnvScope(context.Background(), admin.ID, game.ID, "prod"))

	require.NoError(t, RequireGameEnvScope(context.Background(), svcCtx, admin.ID, []string{"user"}, "demo", "prod"))
}

func TestRequireGameEnvScopeEnvScopeDeniedV9(t *testing.T) {
	svcCtx, game := setupGameScopeContextV9(t)
	admin, err := svcCtx.AdminModel.FindByUsername(context.Background(), "scopeduser")
	require.NoError(t, err)
	require.NoError(t, svcCtx.AdminModel.SetGameEnvScope(context.Background(), admin.ID, game.ID, "stage"))

	err = RequireGameEnvScope(context.Background(), svcCtx, admin.ID, []string{"user"}, "demo", "prod")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "无权访问该环境")
}

func TestRequireGameEnvScopeGameScopeAllowedV9(t *testing.T) {
	svcCtx, game := setupGameScopeContextV9(t)
	admin, err := svcCtx.AdminModel.FindByUsername(context.Background(), "scopeduser")
	require.NoError(t, err)
	require.NoError(t, svcCtx.AdminModel.SetGameScope(context.Background(), admin.ID, game.ID))

	require.NoError(t, RequireGameEnvScope(context.Background(), svcCtx, admin.ID, []string{"user"}, "demo", "prod"))
}

func TestRequireGameEnvScopeGameScopeDeniedV9(t *testing.T) {
	svcCtx, game := setupGameScopeContextV9(t)
	admin, err := svcCtx.AdminModel.FindByUsername(context.Background(), "scopeduser")
	require.NoError(t, err)
	require.NoError(t, svcCtx.AdminModel.SetGameScope(context.Background(), admin.ID, game.ID+1000))

	err = RequireGameEnvScope(context.Background(), svcCtx, admin.ID, []string{"user"}, "demo", "prod")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "无权访问该游戏")
}

func TestRequireGameEnvScopeEnvCountQueryFailsV9(t *testing.T) {
	svcCtx, _ := setupGameScopeContextV9(t)
	require.NoError(t, svcCtx.DB.Migrator().DropTable("admin_game_env_scopes"))
	err := RequireGameEnvScope(context.Background(), svcCtx, 1, []string{"user"}, "demo", "prod")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "check env scope failed")
}

func TestRequireGameEnvScopeCheckErrorV9(t *testing.T) {
	svcCtx, game := setupGameScopeContextV9(t)
	// adminID = 0 makes CheckGameEnvScope / CheckGameScope return ErrAdminNotFound.
	require.NoError(t, svcCtx.AdminModel.SetGameEnvScope(context.Background(), 0, game.ID, "prod"))
	err := RequireGameEnvScope(context.Background(), svcCtx, 0, []string{"user"}, "demo", "prod")
	require.Error(t, err)

	// Remove env scopes so the checker falls back to game scope, where
	// adminID = 0 likewise makes CheckGameScope return ErrAdminNotFound.
	require.NoError(t, svcCtx.DB.Migrator().DropTable("admin_game_env_scopes"))
	err = RequireGameEnvScope(context.Background(), svcCtx, 0, []string{"user"}, "demo", "prod")
	require.Error(t, err)

	// Recreate the table without rows: env scope count is 0 and the game
	// scope check runs (and fails) for adminID = 0.
	require.NoError(t, svcCtx.DB.Migrator().CreateTable(&model.AdminGameEnvScope{}))
	err = RequireGameEnvScope(context.Background(), svcCtx, 0, []string{"user"}, "demo", "prod")
	require.Error(t, err)
}

// ---------------------------------------------------------------------------
// CurrentUsername nil ctx
// ---------------------------------------------------------------------------

func TestCurrentUsernameNilCtxV9(t *testing.T) {
	name, err := CurrentUsername(nil)
	assert.Error(t, err)
	assert.Empty(t, name)
}

// ---------------------------------------------------------------------------
// BuildFunctionDTO with populated OpenAPISpec (rawJSONFromValue default branch)
// ---------------------------------------------------------------------------

func TestBuildFunctionDTOOpenAPISpecV9(t *testing.T) {
	now := time.Now()
	fn := &model.Function{
		Model:       gorm.Model{CreatedAt: now, UpdatedAt: now},
		FunctionID:  "fn.spec",
		Name:        "Spec",
		GameID:      "demo",
		Status:      1,
		OpenAPISpec: map[string]interface{}{"summary": "s"},
	}
	out := BuildFunctionDTO(fn)
	assert.JSONEq(t, `{"summary":"s"}`, string(out.OpenAPISpec))
	assert.NotEmpty(t, out.CreatedAt)
	assert.NotEmpty(t, out.UpdatedAt)
}
