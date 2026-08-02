package function

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/model"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupFunctionTestContext(t *testing.T) (*svc.ServiceContext, context.Context) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = model.AutoMigrate(db)
	require.NoError(t, err)

	svcCtx := &svc.ServiceContext{
		DB:              db,
		FunctionModel:   model.NewFunctionModel(db),
		AdminModel:      model.NewAdminModel(db),
		RoleModel:       model.NewRoleModel(db),
		PermissionModel: model.NewPermissionModel(db),
		RegistryStore:   reg.NewStore(),
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

	err = svcCtx.RoleModel.ReplacePermissions(context.Background(), role.ID, []string{"admin:all"})
	require.NoError(t, err)

	ctx := context.WithValue(context.Background(), "username", "testadmin")
	return svcCtx, ctx
}

func TestFirstNonEmpty(t *testing.T) {
	tests := []struct {
		name     string
		values   []string
		expected string
	}{
		{"first non-empty", []string{"a", "b", "c"}, "a"},
		{"empty first", []string{"", "b", "c"}, "b"},
		{"whitespace first", []string{"  ", "b", "c"}, "b"},
		{"all empty", []string{"", "", ""}, ""},
		{"no values", []string{}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := firstNonEmpty(tt.values...)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSanitizeNodeKey(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"alphanumeric", "abc123", "abc123"},
		{"with spaces", "abc def", "abc_def"},
		{"with special chars", "abc@def#ghi", "abcdefghi"},
		{"mixed separators", "abc-def.ghi_jlk", "abc-def.ghi_jlk"},
		{"leading/trailing separators", "-_-abc-_-", "abc"},
		{"duplicate separators", "abc--def", "abc_def"},
		{"uppercase lowercase", "ABC", "abc"},
		{"empty string", "", ""},
		{"only separators", "-_.-", ""},
		{"with slash", "abc/def", "abc_def"},
		{"with colon", "abc:def", "abc_def"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeNodeKey(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNormalizeTerm(t *testing.T) {
	aliasMap := map[string]map[string]string{
		"resource": {
			"player": "player",
			"user":   "player",
		},
		"operation": {
			"ban":    "block",
			"delete": "remove",
		},
	}

	tests := []struct {
		name     string
		domain   string
		raw      string
		expected string
	}{
		{
			name:     "normalize resource alias",
			domain:   "resource",
			raw:      "user",
			expected: "player",
		},
		{
			name:     "normalize operation alias",
			domain:   "operation",
			raw:      "ban",
			expected: "block",
		},
		{
			name:     "no alias found",
			domain:   "resource",
			raw:      "game",
			expected: "game",
		},
		{
			name:     "empty domain",
			domain:   "",
			raw:      "user",
			expected: "user",
		},
		{
			name:     "empty raw",
			domain:   "resource",
			raw:      "",
			expected: "",
		},
		{
			name:     "case insensitive",
			domain:   "resource",
			raw:      "User",
			expected: "player",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeTerm(aliasMap, tt.domain, tt.raw)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTermDisplay(t *testing.T) {
	displayMap := map[string]map[string]map[string]string{
		"resource": {
			"player": {
				"zh": "玩家",
				"en": "Player",
			},
		},
		"operation": {
			"ban": {
				"zh": "封禁",
				"en": "Ban",
			},
		},
	}

	tests := []struct {
		name      string
		domain    string
		key       string
		expectNil bool
		expectZh  string
		expectEn  string
	}{
		{
			name:     "resource display",
			domain:   "resource",
			key:      "player",
			expectZh: "玩家",
			expectEn: "Player",
		},
		{
			name:     "operation display",
			domain:   "operation",
			key:      "ban",
			expectZh: "封禁",
			expectEn: "Ban",
		},
		{
			name:      "not found",
			domain:    "resource",
			key:       "unknown",
			expectNil: true,
		},
		{
			name:      "empty domain",
			domain:    "",
			key:       "player",
			expectNil: true,
		},
		{
			name:      "empty key",
			domain:    "resource",
			key:       "",
			expectNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := termDisplay(displayMap, tt.domain, tt.key)
			if tt.expectNil {
				assert.Nil(t, result)
			} else {
				assert.NotNil(t, result)
				assert.Equal(t, tt.expectZh, result["zh"])
				assert.Equal(t, tt.expectEn, result["en"])
			}
		})
	}
}

func TestConvertFromUtilsFunction(t *testing.T) {
	u := utils.Function{
		Id:          "test.function",
		Name:        "Test Function",
		Description: "Test",
		Resource:    "test",
		GameId:      "game1",
		Status:      1,
		Version:     "1.0",
		Instances:   3,
	}

	result := convertFromUtilsFunction(u)

	assert.Equal(t, "test.function", result.ID)
	assert.Equal(t, "Test Function", result.Name)
	assert.Equal(t, "Test", result.Description)
	assert.Equal(t, "test", result.Resource)
	assert.Equal(t, "game1", result.GameId)
	assert.Equal(t, 1, result.Status)
	assert.Equal(t, "1.0", result.Version)
	assert.Equal(t, 3, result.Instances)
}

func TestConvertFromUtilsFunctionSlice(t *testing.T) {
	utilsFuncs := []utils.Function{
		{Id: "fn1", Name: "Function 1"},
		{Id: "fn2", Name: "Function 2"},
	}

	result := convertFromUtilsFunctionSlice(utilsFuncs)

	assert.Len(t, result, 2)
	assert.Equal(t, "fn1", result[0].ID)
	assert.Equal(t, "fn2", result[1].ID)
}

func TestConvertFromUtilsPermission(t *testing.T) {
	u := utils.FunctionPermission{
		Resource: "test.resource",
		Actions:  []string{"read", "write"},
		Roles:    []string{"admin", "user"},
	}

	result := convertFromUtilsPermission(u)

	assert.Equal(t, "test.resource", result.Resource)
	assert.Equal(t, []string{"read", "write"}, result.Actions)
	assert.Equal(t, []string{"admin", "user"}, result.Roles)
}

func TestConvertToUtilsPermission(t *testing.T) {
	f := FunctionPermission{
		Resource: "test.resource",
		Actions:  []string{"read", "write"},
		Roles:    []string{"admin", "user"},
	}

	result := convertToUtilsPermission(f)

	assert.Equal(t, "test.resource", result.Resource)
	assert.Equal(t, []string{"read", "write"}, result.Actions)
	assert.Equal(t, []string{"admin", "user"}, result.Roles)
}

func TestConvertFromUtilsPermissions(t *testing.T) {
	utilsPerms := []utils.FunctionPermission{
		{Resource: "r1", Actions: []string{"read"}},
		{Resource: "r2", Actions: []string{"write"}},
	}

	result := convertFromUtilsPermissions(utilsPerms)

	assert.Len(t, result, 2)
	assert.Equal(t, "r1", result[0].Resource)
	assert.Equal(t, "r2", result[1].Resource)
}

func TestConvertToUtilsPermissions(t *testing.T) {
	funcPerms := []FunctionPermission{
		{Resource: "r1", Actions: []string{"read"}},
		{Resource: "r2", Actions: []string{"write"}},
	}

	result := convertToUtilsPermissions(funcPerms)

	assert.Len(t, result, 2)
	assert.Equal(t, "r1", result[0].Resource)
	assert.Equal(t, "r2", result[1].Resource)
}

func TestParseHistoryTime_Additional(t *testing.T) {
	validTime := "2024-03-15T10:00:00Z"
	result := parseHistoryTime(validTime)
	assert.False(t, result.IsZero())

	invalidTime := "not a time"
	result2 := parseHistoryTime(invalidTime)
	assert.False(t, result2.IsZero()) // Should return NowUTC for invalid
}

func TestExtractRoleNames(t *testing.T) {
	roles := []model.Role{
		{Name: "admin"},
		{Name: "user"},
		{Name: "guest"},
	}

	result := ExtractRoleNames(roles)

	assert.Len(t, result, 3)
	assert.Equal(t, "admin", result[0])
	assert.Equal(t, "user", result[1])
	assert.Equal(t, "guest", result[2])
}

func TestExtractRoleNamesEmpty(t *testing.T) {
	result := ExtractRoleNames([]model.Role{})
	assert.NotNil(t, result)
	assert.Empty(t, result)
}

func TestFunctionsList_RuntimeResourceUsesRegisteredMetadataOnly(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))

	store := reg.NewStore()
	store.UpsertAgent(&reg.AgentSession{
		AgentID: "agent-1",
		GameID:  "game-1",
		Functions: map[string]reg.FunctionMeta{
			"player.ban": {
				Enabled: true,
				Version: "1.0.0",
			},
			"mail.send": {
				Enabled:  true,
				Version:  "1.0.0",
				Resource: "ops",
			},
		},
	})

	logic := NewFunctionsListLogic(context.Background(), &svc.ServiceContext{
		DB:            db,
		FunctionModel: model.NewFunctionModel(db),
		RegistryStore: store,
	})

	resp, err := logic.FunctionsList(&FunctionsListRequest{Page: 1, PageSize: 20})
	require.NoError(t, err)
	require.NotNil(t, resp)

	byID := map[string]Function{}
	for _, item := range resp.Items {
		byID[item.ID] = item
	}

	assert.Equal(t, "", byID["player.ban"].Resource)
	assert.Equal(t, "ops", byID["mail.send"].Resource)
}

// Tests for FunctionHistoryLogic
func TestFunctionHistoryLogic_Constructor(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	svcCtx := &svc.ServiceContext{
		DB:                 db,
		FunctionModel:      model.NewFunctionModel(db),
		ConfigVersionModel: nil,
	}
	ctx := context.Background()

	logic := NewFunctionHistoryLogic(ctx, svcCtx)

	assert.NotNil(t, logic)
	assert.Equal(t, ctx, logic.ctx)
	assert.Equal(t, svcCtx, logic.svcCtx)
}

func TestFunctionHistoryLogic_FunctionHistory_CreatesFunction(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	err = model.AutoMigrate(db)
	require.NoError(t, err)

	svcCtx := &svc.ServiceContext{
		DB:                 db,
		FunctionModel:      model.NewFunctionModel(db),
		ConfigVersionModel: nil,
	}

	logic := NewFunctionHistoryLogic(context.Background(), svcCtx)
	items, err := logic.FunctionHistory(&FunctionHistoryRequest{ID: "new.function"})

	assert.NoError(t, err)
	assert.NotNil(t, items)
	assert.Len(t, items, 1)
	assert.Equal(t, "function_created", items[0].Action)

	// Verify function was created
	fn, err := svcCtx.FunctionModel.FindByFunctionID(context.Background(), "new.function")
	assert.NoError(t, err)
	assert.NotNil(t, fn)
	assert.Equal(t, "new.function", fn.FunctionID)
}

func TestFunctionHistoryLogic_FunctionHistory_EmptyID(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	svcCtx := &svc.ServiceContext{
		DB:                 db,
		FunctionModel:      model.NewFunctionModel(db),
		ConfigVersionModel: nil,
	}

	logic := NewFunctionHistoryLogic(context.Background(), svcCtx)
	items, err := logic.FunctionHistory(&FunctionHistoryRequest{ID: ""})

	assert.Error(t, err)
	assert.Nil(t, items)
}
