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

func TestInferCategory(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"dot separator", "player.ban", "player"},
		{"multiple parts", "player.account.get", "player"},
		{"single part", "player", ""},
		{"no separator", "getall", ""},
		{"leading dot", ".player", ""},
		{"trailing dot", "player.", "player"},
		{"empty string", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := inferCategory(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDefaultParamsSchema(t *testing.T) {
	result := defaultParamsSchema()
	assert.NotNil(t, result)
	assert.Equal(t, "object", result["type"])
	assert.NotNil(t, result["properties"])
}

func TestDefaultMenu(t *testing.T) {
	result := defaultMenu()
	assert.NotNil(t, result)
	assert.Equal(t, []string{}, result["nodes"])
	assert.Equal(t, "", result["path"])
	assert.Equal(t, 100, result["order"])
	assert.Equal(t, false, result["hidden"])
}

func TestMergeShallow(t *testing.T) {
	dst := map[string]interface{}{
		"a": 1,
		"b": 2,
	}
	src := map[string]interface{}{
		"b": 20,
		"c": 30,
	}

	mergeShallow(dst, src)

	assert.Equal(t, 1, dst["a"])
	assert.Equal(t, 20, dst["b"])
	assert.Equal(t, 30, dst["c"])
}

func TestMergeShallowNil(t *testing.T) {
	dst := map[string]interface{}{"a": 1}
	src := map[string]interface{}{"b": 2}

	// Nil src should not panic
	mergeShallow(dst, nil)
	assert.Equal(t, 1, dst["a"])

	// Nil dst should not panic
	mergeShallow(nil, src)
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

func TestInferMenuNodes(t *testing.T) {
	tests := []struct {
		name     string
		category string
		entity   string
		fid      string
		wantLen  int
		contains []string
	}{
		{
			name:     "category and entity",
			category: "player",
			entity:   "account",
			fid:      "player.ban",
			wantLen:  2,
			contains: []string{"player", "account"},
		},
		{
			name:     "only category",
			category: "player",
			entity:   "",
			fid:      "",
			wantLen:  1,
			contains: []string{"player"},
		},
		{
			name:     "infer from fid",
			category: "",
			entity:   "",
			fid:      "player.ban",
			wantLen:  2,
			contains: []string{"player", "player"}, // Both category and entity inferred as "player"
		},
		{
			name:     "fallback to fid",
			category: "",
			entity:   "",
			fid:      "unknown",
			wantLen:  1,
			contains: []string{"unknown"},
		},
		{
			name:     "empty all",
			category: "",
			entity:   "",
			fid:      "",
			wantLen:  1,
			contains: []string{"general"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := inferMenuNodes(tt.category, tt.entity, tt.fid)
			assert.Len(t, result, tt.wantLen)
			for _, c := range tt.contains {
				assert.Contains(t, result, c)
			}
		})
	}
}

func TestDefaultFunctionPath(t *testing.T) {
	tests := []struct {
		name     string
		entity   string
		fid      string
		expected string
	}{
		{
			name:     "with entity",
			entity:   "player",
			fid:      "",
			expected: "/game/entities/player",
		},
		{
			name:     "infer entity from fid",
			entity:   "",
			fid:      "player.ban",
			expected: "/game/entities/player",
		},
		{
			name:     "no entity - inferEntityOperationFromID finds entity 'unknown'",
			entity:   "",
			fid:      "unknown.function",
			expected: "/game/entities/unknown",
		},
		{
			name:     "no entity - single word fid uses fallback",
			entity:   "",
			fid:      "justfunction",
			expected: "/game/entities/justfunction",
		},
		{
			name:     "no entity - fid with only stopwords uses invoke",
			entity:   "",
			fid:      "functions.get",
			expected: "/game/functions/invoke?fid=functions.get",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := defaultFunctionPath(tt.entity, tt.fid)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestApplyEntityMenuDefaults(t *testing.T) {
	tests := []struct {
		name          string
		initialMenu   map[string]interface{}
		category      string
		entity        string
		fid           string
		expectNodes   bool
		expectPath    bool
		expectedNodes []string
	}{
		{
			name:          "empty menu gets defaults",
			initialMenu:   map[string]interface{}{},
			category:      "player",
			entity:        "account",
			fid:           "player.ban",
			expectNodes:   true,
			expectPath:    true,
			expectedNodes: []string{"player", "account"},
		},
		{
			name: "existing nodes preserved",
			initialMenu: map[string]interface{}{
				"nodes": []string{"custom", "menu"},
				"path":  "/custom/path",
			},
			category:      "player",
			entity:        "account",
			fid:           "player.ban",
			expectNodes:   true,
			expectPath:    false, // Path already exists
			expectedNodes: []string{"custom", "menu"},
		},
		{
			name:        "nil menu does not panic and returns unchanged",
			initialMenu: nil,
			category:    "player",
			entity:      "",
			fid:         "player.ban",
			expectNodes: false, // Nil menu returns without modification
			expectPath:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			menu := tt.initialMenu
			applyEntityMenuDefaults(menu, tt.category, tt.entity, tt.fid)

			if tt.initialMenu == nil {
				// Nil menu should remain nil (function returns without modification)
				assert.Nil(t, menu, "nil menu should remain nil")
				return
			}

			if menu == nil {
				t.Fatal("menu should not be nil for non-nil initialMenu")
			}

			nodes, ok := menu["nodes"].([]string)
			if tt.expectNodes {
				assert.True(t, ok, "nodes should be []string")
				if tt.expectedNodes != nil {
					assert.Equal(t, tt.expectedNodes, nodes)
				}
			}

			path, ok := menu["path"].(string)
			if tt.expectPath {
				assert.True(t, ok)
				assert.NotEmpty(t, path)
			} else if tt.initialMenu["path"] != nil {
				assert.Equal(t, tt.initialMenu["path"], path)
			}
		})
	}
}

func TestNormalizeTerm(t *testing.T) {
	aliasMap := map[string]map[string]string{
		"entity": {
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
			name:     "normalize entity alias",
			domain:   "entity",
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
			domain:   "entity",
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
			domain:   "entity",
			raw:      "",
			expected: "",
		},
		{
			name:     "case insensitive",
			domain:   "entity",
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

func TestInferEntityOperationFromID(t *testing.T) {
	tests := []struct {
		name           string
		fid            string
		expectedEntity string
		expectedOp     string
	}{
		{
			name:           "player.ban",
			fid:            "player.ban",
			expectedEntity: "player",
			expectedOp:     "ban",
		},
		{
			name:           "player.account.get",
			fid:            "player.account.get",
			expectedEntity: "account",
			expectedOp:     "get",
		},
		{
			name:           "create_player",
			fid:            "create_player",
			expectedEntity: "player",
			expectedOp:     "create",
		},
		{
			name:           "prom-query",
			fid:            "prom-query",
			expectedEntity: "prom",
			expectedOp:     "query",
		},
		{
			name:           "just operation",
			fid:            "get",
			expectedEntity: "",
			expectedOp:     "get",
		},
		{
			name:           "just entity",
			fid:            "player",
			expectedEntity: "player",
			expectedOp:     "",
		},
		{
			name:           "unknown",
			fid:            "unknown",
			expectedEntity: "unknown",
			expectedOp:     "",
		},
		{
			name:           "empty",
			fid:            "",
			expectedEntity: "",
			expectedOp:     "",
		},
		{
			name:           "with stopwords",
			fid:            "functions.player.ban",
			expectedEntity: "player",
			expectedOp:     "ban",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entity, op := inferEntityOperationFromID(tt.fid)
			assert.Equal(t, tt.expectedEntity, entity)
			assert.Equal(t, tt.expectedOp, op)
		})
	}
}

func TestTermDisplay(t *testing.T) {
	displayMap := map[string]map[string]map[string]string{
		"entity": {
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
			name:     "entity display",
			domain:   "entity",
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
			domain:    "entity",
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
			domain:    "entity",
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
		Category:    "test",
		GameId:      "game1",
		Status:      1,
		Version:     "1.0",
		Instances:   3,
	}

	result := convertFromUtilsFunction(u)

	assert.Equal(t, "test.function", result.ID)
	assert.Equal(t, "Test Function", result.Name)
	assert.Equal(t, "Test", result.Description)
	assert.Equal(t, "test", result.Category)
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

func TestParseHistoryTime(t *testing.T) {
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
