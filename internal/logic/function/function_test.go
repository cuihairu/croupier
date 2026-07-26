package function

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/model"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/getkin/kin-openapi/openapi3"
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

func testFunctionFormilySchema(fieldName, component string) map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			fieldName: map[string]interface{}{
				"type":        "string",
				"title":       fieldName,
				"x-component": component,
				"x-decorator": "FormItem",
			},
		},
	}
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

// Tests for FunctionUILogicV2
func TestFunctionUILogicV2_Constructor(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	cfg := config.Config{}
	svcCtx := &svc.ServiceContext{
		DB:            db,
		FunctionModel: model.NewFunctionModel(db),
		Config:        cfg,
	}
	ctx := context.Background()

	logic := NewFunctionUILogicV2(ctx, svcCtx)

	assert.NotNil(t, logic)
	assert.Equal(t, ctx, logic.ctx)
	assert.Equal(t, svcCtx, logic.svcCtx)
}

func TestFunctionUILogicV2_FunctionUI_EmptyID(t *testing.T) {
	db, _ := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})

	cfg := config.Config{}
	svcCtx := &svc.ServiceContext{
		DB:            db,
		FunctionModel: model.NewFunctionModel(db),
		Config:        cfg,
	}

	logic := NewFunctionUILogicV2(context.Background(), svcCtx)
	resp, err := logic.FunctionUI(&FunctionUIRequest{ID: ""})

	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestFunctionUILogicV2_FunctionUI_CreatesFunction(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	err = model.AutoMigrate(db)
	require.NoError(t, err)

	cfg := config.Config{}
	svcCtx := &svc.ServiceContext{
		DB:            db,
		FunctionModel: model.NewFunctionModel(db),
		Config:        cfg,
	}

	logic := NewFunctionUILogicV2(context.Background(), svcCtx)
	resp, err := logic.FunctionUI(&FunctionUIRequest{ID: "new.function"})

	assert.NoError(t, err)
	assert.NotNil(t, resp)

	// Verify function was created
	fn, err := svcCtx.FunctionModel.FindByFunctionID(context.Background(), "new.function")
	assert.NoError(t, err)
	assert.NotNil(t, fn)
	assert.Equal(t, "new.function", fn.FunctionID)
}

// Tests for resolveFunctionUI
func TestResolveFunctionUI(t *testing.T) {
	cfg := config.Config{}

	t.Run("function with custom UI", func(t *testing.T) {
		customUI := testFunctionFormilySchema("name", "Input")
		fn := &model.Function{
			FunctionID: "test",
			Metadata: map[string]interface{}{
				"ui": customUI,
			},
		}
		result := resolveFunctionUI(cfg, fn)
		assert.Equal(t, "custom_metadata", result.UISource)
		assert.Equal(t, customUI, result.Schema)
		assert.True(t, result.Custom)
	})

	t.Run("ignores openapi x-ui", func(t *testing.T) {
		xui := testFunctionFormilySchema("name", "Input")
		fn := &model.Function{
			FunctionID:  "test",
			Metadata:    nil,
			OpenAPISpec: map[string]interface{}{"x-ui": xui},
		}
		result := resolveFunctionUI(cfg, fn)
		assert.Equal(t, "generated_default", result.UISource)
		assert.NotEqual(t, xui, result.Schema)
		assert.False(t, result.Custom)
		assert.True(t, result.HasDefault)
	})
}

// Tests for mergeAny
func TestMergeAny(t *testing.T) {
	t.Run("merge two maps", func(t *testing.T) {
		base := map[string]interface{}{
			"a": 1,
			"b": 2,
		}
		override := map[string]interface{}{
			"b": 20,
			"c": 30,
		}
		result := mergeAny(base, override)
		resultMap, ok := result.(map[string]interface{})
		assert.True(t, ok)
		assert.Equal(t, 1, resultMap["a"])
		assert.Equal(t, 20, resultMap["b"])
		assert.Equal(t, 30, resultMap["c"])
	})

	t.Run("base is not a map", func(t *testing.T) {
		base := "not a map"
		override := map[string]interface{}{"a": 1}
		result := mergeAny(base, override)
		assert.Equal(t, override, result)
	})

	t.Run("both are not maps", func(t *testing.T) {
		base := "base"
		override := "override"
		result := mergeAny(base, override)
		assert.Equal(t, "override", result)
	})
}

// Tests for loadUIConfigFromFiles
func TestLoadUIConfigFromFiles_EmptyFunctionID(t *testing.T) {
	cfg := config.Config{}
	result := loadUIConfigFromFiles(cfg, "")
	assert.Nil(t, result)
}

// Tests for parseConfigContent
func TestParseConfigContent(t *testing.T) {
	t.Run("valid JSON", func(t *testing.T) {
		data := []byte(`{"type": "object"}`)
		result, err := parseConfigContent("json", data)
		assert.NoError(t, err)
		assert.NotNil(t, result)
	})

	t.Run("valid YAML", func(t *testing.T) {
		data := []byte("type: object")
		result, err := parseConfigContent("yaml", data)
		assert.NoError(t, err)
		assert.NotNil(t, result)
	})

	t.Run("invalid JSON", func(t *testing.T) {
		data := []byte(`{invalid}`)
		_, err := parseConfigContent("json", data)
		assert.Error(t, err)
	})
}

// Tests for pickFunctionUIConfig
func TestPickFunctionUIConfig(t *testing.T) {
	t.Run("raw is not a map", func(t *testing.T) {
		result := pickFunctionUIConfig("not a map", "test.function")
		assert.Nil(t, result)
	})

	t.Run("matches function ID", func(t *testing.T) {
		raw := map[string]interface{}{
			"test.function": map[string]interface{}{
				"type": "object",
			},
		}
		result := pickFunctionUIConfig(raw, "test.function")
		assert.NotNil(t, result)
	})

	t.Run("no match returns root", func(t *testing.T) {
		raw := map[string]interface{}{
			"type": "root",
		}
		result := pickFunctionUIConfig(raw, "test.function")
		assert.NotNil(t, result)
	})
}

// Tests for unwrapUIConfig
func TestUnwrapUIConfig(t *testing.T) {
	t.Run("keeps x-ui wrapper as invalid legacy payload", func(t *testing.T) {
		v := map[string]interface{}{
			"x-ui": testFunctionFormilySchema("name", "Input"),
		}
		result := unwrapUIConfig(v)
		assert.NotNil(t, result)
		resultMap, ok := result.(map[string]interface{})
		assert.True(t, ok)
		assert.Contains(t, resultMap, "x-ui")
	})

	t.Run("no x-ui key", func(t *testing.T) {
		v := map[string]interface{}{
			"type": "object",
		}
		result := unwrapUIConfig(v)
		assert.Equal(t, v, result)
	})

	t.Run("v is not a map", func(t *testing.T) {
		v := "not a map"
		result := unwrapUIConfig(v)
		assert.Equal(t, "not a map", result)
	})
}

// Tests for uiConfigBaseDirs
func TestUIConfigBaseDirs(t *testing.T) {
	t.Run("empty config", func(t *testing.T) {
		cfg := config.Config{}
		dirs := uiConfigBaseDirs(cfg)
		assert.NotEmpty(t, dirs)
	})

	t.Run("all paths are absolute", func(t *testing.T) {
		cfg := config.Config{}
		dirs := uiConfigBaseDirs(cfg)
		for _, dir := range dirs {
			assert.True(t, filepath.IsAbs(dir), "path %s should be absolute", dir)
		}
	})
}

// Tests for readUIConfigFile
func TestReadUIConfigFile_NonExistentDirectory(t *testing.T) {
	result := readUIConfigFile("/nonexistent/path", "test.function")
	assert.Nil(t, result)
}

// Tests for getOrCreateFunctionRecord
func TestGetOrCreateFunctionRecord(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	err = model.AutoMigrate(db)
	require.NoError(t, err)

	svcCtx := &svc.ServiceContext{
		DB:            db,
		FunctionModel: model.NewFunctionModel(db),
	}

	// First call should create
	fn1, err := getOrCreateFunctionRecord(context.Background(), svcCtx, "test.new")
	assert.NoError(t, err)
	assert.NotNil(t, fn1)
	assert.Equal(t, "test.new", fn1.FunctionID)

	// Second call should find existing
	fn2, err := getOrCreateFunctionRecord(context.Background(), svcCtx, "test.new")
	assert.NoError(t, err)
	assert.NotNil(t, fn2)
	assert.Equal(t, fn1.ID, fn2.ID)
}

// Tests for firstNonEmpty edge cases
func TestFirstNonEmpty_EdgeCases(t *testing.T) {
	t.Run("all whitespace", func(t *testing.T) {
		result := firstNonEmpty("  ", "\t", "\n")
		assert.Equal(t, "", result)
	})

	t.Run("nil slice", func(t *testing.T) {
		result := firstNonEmpty()
		assert.Equal(t, "", result)
	})
}

// Tests for sanitizeNodeKey edge cases
func TestSanitizeNodeKey_EdgeCases(t *testing.T) {
	t.Run("multiple consecutive separators", func(t *testing.T) {
		result := sanitizeNodeKey("a___b---c")
		assert.Equal(t, "a_b_c", result)
	})

	t.Run("trailing and leading separators", func(t *testing.T) {
		result := sanitizeNodeKey("-_-test-_-")
		assert.Equal(t, "test", result)
	})

	t.Run("special chars removed", func(t *testing.T) {
		result := sanitizeNodeKey("a!@#$b%^&c")
		assert.Equal(t, "abc", result)
	})
}

// Tests for normalizeTerm edge cases
func TestNormalizeTerm_EdgeCases(t *testing.T) {
	aliasMap := map[string]map[string]string{
		"resource": {
			"user": "player",
		},
	}

	t.Run("whitespace value", func(t *testing.T) {
		result := normalizeTerm(aliasMap, "resource", "  ")
		assert.Equal(t, "", result)
	})

	t.Run("whitespace domain", func(t *testing.T) {
		result := normalizeTerm(aliasMap, "  ", "user")
		assert.Equal(t, "user", result)
	})
}

// Tests for termDisplay edge cases
func TestTermDisplay_EdgeCases(t *testing.T) {
	displayMap := map[string]map[string]map[string]string{
		"resource": {
			"player": {
				"zh": "玩家",
			},
		},
	}

	t.Run("missing language", func(t *testing.T) {
		result := termDisplay(displayMap, "resource", "player")
		assert.NotNil(t, result)
		assert.Equal(t, "玩家", result["zh"])
		_, hasEn := result["en"]
		assert.False(t, hasEn)
	})

	t.Run("empty display map", func(t *testing.T) {
		result := termDisplay(nil, "resource", "player")
		assert.Nil(t, result)
	})
}

// Tests for mergeAny edge cases
func TestMergeAny_EdgeCases(t *testing.T) {
	t.Run("deep merge", func(t *testing.T) {
		base := map[string]interface{}{
			"nested": map[string]interface{}{
				"a": 1,
				"b": 2,
			},
		}
		override := map[string]interface{}{
			"nested": map[string]interface{}{
				"b": 20,
				"c": 30,
			},
		}
		result := mergeAny(base, override)
		resultMap, ok := result.(map[string]interface{})
		assert.True(t, ok)
		nested, ok := resultMap["nested"].(map[string]interface{})
		assert.True(t, ok)
		assert.Equal(t, 1, nested["a"])
		assert.Equal(t, 20, nested["b"])
		assert.Equal(t, 30, nested["c"])
	})

	t.Run("arrays are replaced not merged", func(t *testing.T) {
		base := map[string]interface{}{
			"items": []interface{}{1, 2, 3},
		}
		override := map[string]interface{}{
			"items": []interface{}{4, 5},
		}
		result := mergeAny(base, override)
		resultMap, ok := result.(map[string]interface{})
		assert.True(t, ok)
		items, ok := resultMap["items"].([]interface{})
		assert.True(t, ok)
		assert.Equal(t, 2, len(items))
	})

	t.Run("nil values", func(t *testing.T) {
		base := map[string]interface{}{"a": 1}
		result := mergeAny(base, nil)
		// When override is nil, mergeAny returns the override (nil)
		assert.Nil(t, result)
	})
}

// Tests for extractOperationRequestSchema
func TestExtractOperationRequestSchema(t *testing.T) {
	t.Run("nil operation", func(t *testing.T) {
		result := extractOperationRequestSchema(nil)
		assert.Nil(t, result)
	})

	t.Run("operation without request body", func(t *testing.T) {
		op := &openapi3.Operation{}
		result := extractOperationRequestSchema(op)
		assert.Nil(t, result)
	})

	t.Run("operation with empty content", func(t *testing.T) {
		op := &openapi3.Operation{
			RequestBody: &openapi3.RequestBodyRef{
				Value: &openapi3.RequestBody{
					Content: map[string]*openapi3.MediaType{},
				},
			},
		}
		result := extractOperationRequestSchema(op)
		assert.Nil(t, result)
	})

	t.Run("operation with JSON schema", func(t *testing.T) {
		objectType := openapi3.Types{"object"}
		stringType := openapi3.Types{"string"}
		op := &openapi3.Operation{
			RequestBody: &openapi3.RequestBodyRef{
				Value: &openapi3.RequestBody{
					Content: map[string]*openapi3.MediaType{
						"application/json": {
							Schema: &openapi3.SchemaRef{
								Value: &openapi3.Schema{
									Type: &objectType,
									Properties: map[string]*openapi3.SchemaRef{
										"name": {Value: &openapi3.Schema{Type: &stringType}},
									},
								},
							},
						},
					},
				},
			},
		}
		result := extractOperationRequestSchema(op)
		assert.NotNil(t, result)
		assert.Equal(t, "object", result["type"])
	})

	t.Run("operation with $ref", func(t *testing.T) {
		op := &openapi3.Operation{
			RequestBody: &openapi3.RequestBodyRef{
				Value: &openapi3.RequestBody{
					Content: map[string]*openapi3.MediaType{
						"application/json": {
							Schema: &openapi3.SchemaRef{
								Ref: "#/components/schemas/Player",
							},
						},
					},
				},
			},
		}
		result := extractOperationRequestSchema(op)
		assert.NotNil(t, result)
		assert.Equal(t, "#/components/schemas/Player", result["$ref"])
	})

	t.Run("operation with non-JSON media type", func(t *testing.T) {
		op := &openapi3.Operation{
			RequestBody: &openapi3.RequestBodyRef{
				Value: &openapi3.RequestBody{
					Content: map[string]*openapi3.MediaType{
						"text/plain": {
							Schema: &openapi3.SchemaRef{
								Value: &openapi3.Schema{Type: &openapi3.Types{"string"}},
							},
						},
					},
				},
			},
		}
		result := extractOperationRequestSchema(op)
		assert.NotNil(t, result)
	})
}

// Tests for FunctionHistoryLogic with more scenarios
func TestFunctionHistoryLogic_FunctionHistory_MoreScenarios(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	err = model.AutoMigrate(db)
	require.NoError(t, err)

	// Create a function
	fn := &model.Function{
		FunctionID: "test.history",
		Name:       "Test History",
		Status:     1,
	}
	err = db.Create(fn).Error
	require.NoError(t, err)

	t.Run("with config versions", func(t *testing.T) {
		configModel := model.NewConfigVersionModel(db)

		// UI config version
		uiConfig := map[string]interface{}{
			"schema": map[string]interface{}{
				"type": "object",
			},
		}
		uiConfigJSON, _ := json.Marshal(uiConfig)
		_, err = configModel.Create(context.Background(), "ui.test.history", string(uiConfigJSON), "testuser")
		require.NoError(t, err)

		svcCtx := &svc.ServiceContext{
			DB:                 db,
			FunctionModel:      model.NewFunctionModel(db),
			ConfigVersionModel: configModel,
		}

		logic := NewFunctionHistoryLogic(context.Background(), svcCtx)
		items, err := logic.FunctionHistory(&FunctionHistoryRequest{ID: "test.history"})

		assert.NoError(t, err)
		assert.GreaterOrEqual(t, len(items), 2)

		// Check we have the right types
		hasCreated := false
		hasUIUpdate := false
		for _, item := range items {
			switch item.Action {
			case "function_created":
				hasCreated = true
			case "ui_config_updated":
				hasUIUpdate = true
			}
		}
		assert.True(t, hasCreated, "should have function_created item")
		assert.True(t, hasUIUpdate, "should have ui_config_updated item")
	})

	t.Run("with nil config version model", func(t *testing.T) {
		svcCtx := &svc.ServiceContext{
			DB:                 db,
			FunctionModel:      model.NewFunctionModel(db),
			ConfigVersionModel: nil,
		}

		logic := NewFunctionHistoryLogic(context.Background(), svcCtx)
		items, err := logic.FunctionHistory(&FunctionHistoryRequest{ID: "test.history"})

		assert.NoError(t, err)
		assert.Len(t, items, 1)
		assert.Equal(t, "function_created", items[0].Action)
	})
}

// Tests for getOrCreateFunctionRecord edge cases
func TestGetOrCreateFunctionRecord_EdgeCases(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	err = model.AutoMigrate(db)
	require.NoError(t, err)

	svcCtx := &svc.ServiceContext{
		DB:            db,
		FunctionModel: model.NewFunctionModel(db),
	}
	ctx := context.Background()

	t.Run("creates new function", func(t *testing.T) {
		fn, err := getOrCreateFunctionRecord(ctx, svcCtx, "new.function")
		assert.NoError(t, err)
		assert.NotNil(t, fn)
		assert.Equal(t, "new.function", fn.FunctionID)
	})

	t.Run("finds existing function", func(t *testing.T) {
		// First call creates
		fn1, err := getOrCreateFunctionRecord(ctx, svcCtx, "existing.function")
		assert.NoError(t, err)
		assert.NotNil(t, fn1)

		// Second call should find existing
		fn2, err := getOrCreateFunctionRecord(ctx, svcCtx, "existing.function")
		assert.NoError(t, err)
		assert.Equal(t, fn1.ID, fn2.ID)
	})

	t.Run("handles duplicate key error", func(t *testing.T) {
		// Create the function manually first
		fn := &model.Function{
			FunctionID: "duplicate.function",
			Name:       "Duplicate",
			Status:     1,
		}
		err := svcCtx.FunctionModel.Create(ctx, fn)
		assert.NoError(t, err)

		// Try to create again - should find the existing one
		fn2, err := getOrCreateFunctionRecord(ctx, svcCtx, "duplicate.function")
		assert.NoError(t, err)
		assert.NotNil(t, fn2)
		assert.Equal(t, fn.ID, fn2.ID)
	})
}

// Tests for FunctionUILogicV2 with more scenarios
func TestFunctionUILogicV2_MoreScenarios(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	err = model.AutoMigrate(db)
	require.NoError(t, err)

	cfg := config.Config{}

	t.Run("with nil metadata and nil openapi", func(t *testing.T) {
		fn := &model.Function{
			FunctionID:  "test.nilboth",
			Name:        "Test Nil Both",
			Status:      1,
			Metadata:    nil,
			OpenAPISpec: nil,
		}
		err = db.Create(fn).Error
		require.NoError(t, err)

		svcCtx := &svc.ServiceContext{
			DB:            db,
			FunctionModel: model.NewFunctionModel(db),
			Config:        cfg,
		}

		logic := NewFunctionUILogicV2(context.Background(), svcCtx)
		resp, err := logic.FunctionUI(&FunctionUIRequest{ID: "test.nilboth"})

		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.False(t, resp.Custom)
		assert.True(t, resp.HasDefault)
		assert.Equal(t, "generated_default", resp.UISource)
		assert.NotNil(t, resp.Schema)
	})

	t.Run("ignores legacy layout and components in metadata", func(t *testing.T) {
		customLayout := map[string]interface{}{
			"type": "vertical",
			"cols": 1,
		}
		customComponents := map[string]interface{}{
			"field1": map[string]interface{}{
				"component": "Input",
			},
		}

		fn := &model.Function{
			FunctionID: "test.layoutcomp",
			Name:       "Test Layout Components",
			Status:     1,
			Metadata: map[string]interface{}{
				"layout":     customLayout,
				"components": customComponents,
			},
		}
		err = db.Create(fn).Error
		require.NoError(t, err)

		svcCtx := &svc.ServiceContext{
			DB:            db,
			FunctionModel: model.NewFunctionModel(db),
			Config:        cfg,
		}

		logic := NewFunctionUILogicV2(context.Background(), svcCtx)
		resp, err := logic.FunctionUI(&FunctionUIRequest{ID: "test.layoutcomp"})

		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.NotNil(t, resp.Schema)
		assert.False(t, resp.Custom)
		assert.Equal(t, "generated_default", resp.UISource)
	})

	t.Run("with complex nested schema", func(t *testing.T) {
		complexSchema := map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"player": map[string]interface{}{
					"type":        "object",
					"title":       "Player",
					"x-component": "Card",
					"x-decorator": "FormItem",
					"properties": map[string]interface{}{
						"id": map[string]interface{}{
							"type":        "string",
							"title":       "ID",
							"x-component": "Input",
							"x-decorator": "FormItem",
						},
						"name": map[string]interface{}{
							"type":        "string",
							"title":       "Name",
							"x-component": "Input",
							"x-decorator": "FormItem",
						},
					},
				},
				"actions": map[string]interface{}{
					"type":        "array",
					"title":       "Actions",
					"x-component": "ArrayItems",
					"x-decorator": "FormItem",
					"items": map[string]interface{}{
						"type":        "string",
						"title":       "Action",
						"x-component": "Input",
						"x-decorator": "FormItem",
					},
				},
			},
			"required": []interface{}{"player"},
		}

		fn := &model.Function{
			FunctionID: "test.complex",
			Name:       "Test Complex",
			Status:     1,
			Metadata: map[string]interface{}{
				"ui": complexSchema,
			},
		}
		err = db.Create(fn).Error
		require.NoError(t, err)

		svcCtx := &svc.ServiceContext{
			DB:            db,
			FunctionModel: model.NewFunctionModel(db),
			Config:        cfg,
		}

		logic := NewFunctionUILogicV2(context.Background(), svcCtx)
		resp, err := logic.FunctionUI(&FunctionUIRequest{ID: "test.complex"})

		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.True(t, resp.Custom)

		schema, err := jsonObjectFromRaw(resp.Schema)
		assert.NoError(t, err)
		assert.Equal(t, "object", schema["type"])
	})
}

// Tests for readUIConfigFile edge cases
func TestReadUIConfigFile_EdgeCases(t *testing.T) {
	t.Run("non-existent directory", func(t *testing.T) {
		result := readUIConfigFile("/nonexistent/path", "test.function")
		assert.Nil(t, result)
	})

	t.Run("directory exists but no matching files", func(t *testing.T) {
		tmpDir := t.TempDir()
		result := readUIConfigFile(tmpDir, "nonexistent")
		assert.Nil(t, result)
	})
}

// Tests for parseConfigContent
func TestParseConfigContent_MoreScenarios(t *testing.T) {
	t.Run("empty data", func(t *testing.T) {
		_, err := parseConfigContent("json", []byte{})
		assert.Error(t, err)
	})

	t.Run("yml with nested structures", func(t *testing.T) {
		data := []byte("nested:\n  key: value\n  key2: 5")
		result, err := parseConfigContent("yml", data)
		assert.NoError(t, err)
		resultMap, ok := result.(map[string]interface{})
		assert.True(t, ok)
		assert.Equal(t, "value", resultMap["nested"].(map[string]interface{})["key"])
	})

	t.Run("invalid JSON", func(t *testing.T) {
		_, err := parseConfigContent("json", []byte("{invalid json}"))
		assert.Error(t, err)
	})

	t.Run("invalid YAML", func(t *testing.T) {
		// Using invalid YAML syntax (unmatched brackets)
		_, err := parseConfigContent("yaml", []byte("{invalid: yaml"))
		assert.Error(t, err)
	})
}

// Tests for pickFunctionUIConfig edge cases
func TestPickFunctionUIConfig_EdgeCases(t *testing.T) {
	t.Run("empty map", func(t *testing.T) {
		result := pickFunctionUIConfig(map[string]interface{}{}, "test.function")
		// unwrapUIConfig returns the map itself when no x-ui key
		resultMap, ok := result.(map[string]interface{})
		assert.True(t, ok)
		assert.Empty(t, resultMap)
	})

	t.Run("root as fallback", func(t *testing.T) {
		raw := map[string]interface{}{
			"type": "root",
		}
		result := pickFunctionUIConfig(raw, "unknown.function")
		assert.NotNil(t, result)
		// unwrapUIConfig returns the whole map when no x-ui key
		resultMap, ok := result.(map[string]interface{})
		assert.True(t, ok)
		assert.Equal(t, "root", resultMap["type"])
	})
}

// Tests for unwrapUIConfig edge cases
func TestUnwrapUIConfig_EdgeCases(t *testing.T) {
	t.Run("x-ui with null stays wrapped", func(t *testing.T) {
		v := map[string]interface{}{
			"x-ui": nil,
		}
		result := unwrapUIConfig(v)
		assert.Equal(t, v, result)
	})

	t.Run("x-ui with string stays wrapped", func(t *testing.T) {
		v := map[string]interface{}{
			"x-ui": "string value",
		}
		result := unwrapUIConfig(v)
		assert.Equal(t, v, result)
	})

	t.Run("x-ui with array stays wrapped", func(t *testing.T) {
		arr := []interface{}{"a", "b"}
		v := map[string]interface{}{
			"x-ui": arr,
		}
		result := unwrapUIConfig(v)
		assert.Equal(t, v, result)
	})
}

// Tests for uiConfigBaseDirs edge cases
func TestUIConfigBaseDirs_EdgeCases(t *testing.T) {
	t.Run("ignores empty paths", func(t *testing.T) {
		cfg := config.Config{
			BootstrapData: config.BootstrapDataConfig{
				BaseDir: "",
			},
		}
		dirs := uiConfigBaseDirs(cfg)
		for _, dir := range dirs {
			assert.NotEmpty(t, dir)
		}
	})

	t.Run("deduplicates paths", func(t *testing.T) {
		tmpDir := t.TempDir()
		cfg := config.Config{
			BootstrapData: config.BootstrapDataConfig{
				BaseDir: tmpDir,
			},
		}
		dirs := uiConfigBaseDirs(cfg)
		// Count how many times the tmpDir/ui path appears
		count := 0
		for _, dir := range dirs {
			if dir == filepath.Join(tmpDir, "ui") {
				count++
			}
		}
		assert.Equal(t, 1, count, "should have exactly one occurrence")
	})
}
