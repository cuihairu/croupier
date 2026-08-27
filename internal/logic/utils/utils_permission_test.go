package utils

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupUtilsTestContext(t *testing.T) (*svc.ServiceContext, context.Context) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = model.AutoMigrate(db)
	require.NoError(t, err)

	svcCtx := &svc.ServiceContext{
		DB:              db,
		AdminModel:      model.NewAdminModel(db),
		RoleModel:       model.NewRoleModel(db),
		PermissionModel: model.NewPermissionModel(db),
	}

	// Create test admin
	admin := &model.Admin{Username: "testadmin", Status: 1}
	err = svcCtx.AdminModel.Create(context.Background(), admin, "password")
	require.NoError(t, err)

	role := &model.Role{Name: "admin", Description: "Admin"}
	err = svcCtx.RoleModel.Create(context.Background(), role)
	require.NoError(t, err)

	err = svcCtx.AdminModel.AssignRole(context.Background(), admin.ID, role.ID)
	require.NoError(t, err)

	ctx := context.WithValue(context.Background(), "username", "testadmin")
	return svcCtx, ctx
}

func TestCurrentUsernameSimple(t *testing.T) {
	tests := []struct {
		name     string
		ctx      context.Context
		expected string
		wantErr  bool
	}{
		{
			name:     "username in context",
			ctx:      context.WithValue(context.Background(), "username", "testuser"),
			expected: "testuser",
			wantErr:  false,
		},
		{
			name:     "no username in context",
			ctx:      context.Background(),
			expected: "",
			wantErr:  true,
		},
		{
			name:     "empty username",
			ctx:      context.WithValue(context.Background(), "username", ""),
			expected: "",
			wantErr:  true,
		},
		{
			name:     "whitespace username - trimmed to empty",
			ctx:      context.WithValue(context.Background(), "username", "  "),
			expected: "",
			wantErr:  true, // TrimSpace makes it empty, which returns error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := CurrentUsername(tt.ctx)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestLoadCurrentAdminSimple(t *testing.T) {
	svcCtx, validCtx := setupUtilsTestContext(t)

	t.Run("valid admin", func(t *testing.T) {
		admin, roles, err := LoadCurrentAdmin(validCtx, svcCtx)
		assert.NoError(t, err)
		assert.NotNil(t, admin)
		assert.Equal(t, "testadmin", admin.Username)
		assert.NotEmpty(t, roles)
	})

	t.Run("no username in context", func(t *testing.T) {
		_, _, err := LoadCurrentAdmin(context.Background(), svcCtx)
		assert.Error(t, err)
	})

	t.Run("empty username in context", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), "username", "")
		_, _, err := LoadCurrentAdmin(ctx, svcCtx)
		assert.Error(t, err)
	})

	t.Run("nonexistent user", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), "username", "nonexistent")
		_, _, err := LoadCurrentAdmin(ctx, svcCtx)
		assert.Error(t, err)
	})
}

func TestPermissionIDsFromRolesSimple(t *testing.T) {
	svcCtx, ctx := setupUtilsTestContext(t)

	// Add permissions to role
	admin, roles, err := LoadCurrentAdmin(ctx, svcCtx)
	require.NoError(t, err)
	require.NotNil(t, admin)

	role := roles[0]
	err = svcCtx.RoleModel.ReplacePermissions(context.Background(), role.ID, []string{"admin:all"})
	require.NoError(t, err)

	t.Run("load permissions", func(t *testing.T) {
		permIDs, err := PermissionIDsFromRoles(ctx, svcCtx, roles)
		assert.NoError(t, err)
		assert.NotEmpty(t, permIDs)
	})

	t.Run("empty roles", func(t *testing.T) {
		permIDs, err := PermissionIDsFromRoles(ctx, svcCtx, []model.Role{})
		assert.NoError(t, err)
		assert.Empty(t, permIDs)
	})
}

func TestAppendPermissionIDSSimple(t *testing.T) {
	tests := []struct {
		name          string
		permissionIDs []string
		values        []string
		expectedLen   int
		contains      []string
	}{
		{
			name:          "append to existing",
			permissionIDs: []string{"a", "b"},
			values:        []string{"c", "d"},
			expectedLen:   4,
			contains:      []string{"a", "b", "c", "d"},
		},
		{
			name:          "nil base",
			permissionIDs: nil,
			values:        []string{"a", "b"},
			expectedLen:   2,
			contains:      []string{"a", "b"},
		},
		{
			name:          "duplicates removed",
			permissionIDs: []string{"a", "b"},
			values:        []string{"b", "c"},
			expectedLen:   3,
			contains:      []string{"a", "b", "c"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := appendPermissionIDs(tt.permissionIDs, tt.values...)
			assert.Len(t, result, tt.expectedLen)
			for _, c := range tt.contains {
				assert.Contains(t, result, c)
			}
		})
	}
}

func TestFunctionActionAllowedSimple(t *testing.T) {
	tests := []struct {
		name      string
		roleNames []string
		perms     []model.FunctionPermission
		action    string
		expected  struct {
			allowed bool
			hasRule bool
		}
	}{
		{
			name:      "admin role always allowed",
			roleNames: []string{"admin"},
			perms:     []model.FunctionPermission{},
			action:    "invoke",
			expected: struct {
				allowed bool
				hasRule bool
			}{allowed: true, hasRule: true},
		},
		{
			name:      "no matching rule",
			roleNames: []string{"user"},
			perms:     []model.FunctionPermission{},
			action:    "invoke",
			expected: struct {
				allowed bool
				hasRule bool
			}{allowed: false, hasRule: false},
		},
		{
			name:      "matching role wildcard",
			roleNames: []string{"user"},
			perms: []model.FunctionPermission{
				{Actions: mustEncodeJSON(`["invoke"]`), Roles: mustEncodeJSON(`["*"]`)},
			},
			action: "invoke",
			expected: struct {
				allowed bool
				hasRule bool
			}{allowed: true, hasRule: true},
		},
		{
			name:      "matching specific role",
			roleNames: []string{"operator"},
			perms: []model.FunctionPermission{
				{Actions: mustEncodeJSON(`["invoke"]`), Roles: mustEncodeJSON(`["operator"]`)},
			},
			action: "invoke",
			expected: struct {
				allowed bool
				hasRule bool
			}{allowed: true, hasRule: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowed, hasRule := FunctionActionAllowed(tt.roleNames, tt.perms, tt.action, "", "")
			assert.Equal(t, tt.expected.allowed, allowed, "allowed mismatch")
			assert.Equal(t, tt.expected.hasRule, hasRule, "hasRule mismatch")
		})
	}
}

func TestHasRoleVariants(t *testing.T) {
	tests := []struct {
		name      string
		roleNames []string
		role      string
		want      bool
	}{
		{
			name:      "role exists",
			roleNames: []string{"admin", "user"},
			role:      "admin",
			want:      true,
		},
		{
			name:      "role not exists",
			roleNames: []string{"admin", "user"},
			role:      "guest",
			want:      false,
		},
		{
			name:      "case insensitive",
			roleNames: []string{"ADMIN", "USER"},
			role:      "admin",
			want:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, HasRole(tt.roleNames, tt.role))
		})
	}
}

func TestAppendPermissionIDsEdgeCases(t *testing.T) {
	t.Run("nil base with nil values returns nil", func(t *testing.T) {
		result := appendPermissionIDs(nil, nil...)
		// When no values provided, function returns original (nil)
		assert.Nil(t, result)
	})

	t.Run("empty strings are filtered", func(t *testing.T) {
		result := appendPermissionIDs([]string{"a"}, "", "  ", "b")
		assert.Len(t, result, 2)
		assert.Contains(t, result, "a")
		assert.Contains(t, result, "b")
	})

	t.Run("all empty strings", func(t *testing.T) {
		result := appendPermissionIDs([]string{}, "", "  ", "")
		assert.Empty(t, result)
	})
}

func mustEncodeJSON(s string) model.JSON {
	return model.JSON([]byte(s))
}
