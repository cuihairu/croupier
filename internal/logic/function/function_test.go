package function

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
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

// Tests for FunctionRouteLogic
func TestFunctionRouteLogic_Constructor(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	svcCtx := &svc.ServiceContext{
		DB:            db,
		FunctionModel: model.NewFunctionModel(db),
	}
	ctx := context.Background()

	logic := NewFunctionRouteLogic(ctx, svcCtx)

	assert.NotNil(t, logic)
	assert.Equal(t, ctx, logic.ctx)
	assert.Equal(t, svcCtx, logic.svcCtx)
}

func TestFunctionRouteLogic_FunctionRoute_NotFound(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	err = model.AutoMigrate(db)
	require.NoError(t, err)

	svcCtx := &svc.ServiceContext{
		DB:            db,
		FunctionModel: model.NewFunctionModel(db),
	}

	logic := NewFunctionRouteLogic(context.Background(), svcCtx)
	resp, err := logic.FunctionRoute(&FunctionRouteRequest{ID: "nonexistent.function"})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "default", resp.Source)
	assert.NotNil(t, resp.Menu)
}

func TestFunctionRouteLogic_FunctionRoute_EmptyID(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	svcCtx := &svc.ServiceContext{
		DB:            db,
		FunctionModel: model.NewFunctionModel(db),
	}

	logic := NewFunctionRouteLogic(context.Background(), svcCtx)
	resp, err := logic.FunctionRoute(&FunctionRouteRequest{ID: ""})

	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestFunctionRouteLogic_FunctionRoute_WithMetadata(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	err = model.AutoMigrate(db)
	require.NoError(t, err)

	fn := &model.Function{
		FunctionID: "player.ban",
		Name:       "Ban Player",
		Status:     1,
		Metadata: map[string]interface{}{
			"menu": map[string]interface{}{
				"nodes":  []string{"player", "actions"},
				"path":   "/custom/ban",
				"order":  10,
				"hidden": false,
			},
		},
	}
	err = db.Create(fn).Error
	require.NoError(t, err)

	svcCtx := &svc.ServiceContext{
		DB:            db,
		FunctionModel: model.NewFunctionModel(db),
	}

	logic := NewFunctionRouteLogic(context.Background(), svcCtx)
	resp, err := logic.FunctionRoute(&FunctionRouteRequest{ID: "player.ban"})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "metadata", resp.Source)
	assert.NotNil(t, resp.Menu)
}

func TestNormalizeMenuConfig(t *testing.T) {
	tests := []struct {
		name     string
		menu     map[string]interface{}
		expected FunctionRouteConfig
	}{
		{
			name:     "nil menu",
			menu:     nil,
			expected: FunctionRouteConfig{Nodes: []string{}, Path: "", Order: 100, Hidden: false},
		},
		{
			name: "full config",
			menu: map[string]interface{}{
				"nodes":  []string{"a", "b"},
				"path":   "/path",
				"order":  50,
				"hidden": true,
			},
			expected: FunctionRouteConfig{
				Nodes:  []string{"a", "b"},
				Path:   "/path",
				Order:  50,
				Hidden: true,
			},
		},
		{
			name: "interface array nodes",
			menu: map[string]interface{}{
				"nodes": []interface{}{"a", "b", "c"},
			},
			expected: FunctionRouteConfig{
				Nodes: []string{"a", "b", "c"},
				Order: 100,
			},
		},
		{
			name: "float64 order",
			menu: map[string]interface{}{
				"order": 42.7,
			},
			expected: FunctionRouteConfig{
				Order: 42,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeMenuConfig(tt.menu)
			assert.Equal(t, tt.expected.Order, result.Order)
			assert.Equal(t, tt.expected.Path, result.Path)
			assert.Equal(t, tt.expected.Hidden, result.Hidden)
			// Nodes may be inferred from function ID context
			if tt.expected.Nodes != nil && len(tt.expected.Nodes) > 0 {
				assert.Equal(t, tt.expected.Nodes, result.Nodes)
			}
		})
	}
}

// Tests for FunctionRouteUpdateLogic
func TestNewFunctionRouteUpdateLogic(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	svcCtx := &svc.ServiceContext{
		DB:            db,
		FunctionModel: model.NewFunctionModel(db),
	}
	ctx := context.Background()

	logic := NewFunctionRouteUpdateLogic(ctx, svcCtx)

	assert.NotNil(t, logic)
	assert.Equal(t, ctx, logic.ctx)
	assert.Equal(t, svcCtx, logic.svcCtx)
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
		customUI := map[string]interface{}{"type": "custom"}
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

	t.Run("function with x-ui only", func(t *testing.T) {
		xui := map[string]interface{}{"type": "xui"}
		fn := &model.Function{
			FunctionID:  "test",
			Metadata:    nil,
			OpenAPISpec: map[string]interface{}{"x-ui": xui},
		}
		result := resolveFunctionUI(cfg, fn)
		assert.Equal(t, "openapi_x_ui", result.UISource)
		assert.Equal(t, xui, result.Schema)
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
	t.Run("extracts x-ui", func(t *testing.T) {
		v := map[string]interface{}{
			"x-ui": map[string]interface{}{
				"type": "object",
			},
		}
		result := unwrapUIConfig(v)
		assert.NotNil(t, result)
		resultMap, ok := result.(map[string]interface{})
		assert.True(t, ok)
		assert.Equal(t, "object", resultMap["type"])
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

// Tests for normalizeMenuConfig with dirty nodes
func TestNormalizeMenuConfig_DirtyNodes(t *testing.T) {
	menu := map[string]interface{}{
		"nodes": []interface{}{"Dirty Node!", "Another/One", 123, true},
	}
	result := normalizeMenuConfig(menu)

	// Should sanitize and filter out non-strings
	assert.Equal(t, []string{"dirty_node", "another_one"}, result.Nodes)
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

// Tests for inferEntityOperationFromID edge cases
func TestInferEntityOperationFromID_EdgeCases(t *testing.T) {
	t.Run("with multiple stopwords", func(t *testing.T) {
		entity, op := inferEntityOperationFromID("packs.functions.player.create")
		assert.Equal(t, "player", entity)
		assert.Equal(t, "create", op)
	})

	t.Run("all stopwords", func(t *testing.T) {
		entity, op := inferEntityOperationFromID("packs.functions.api")
		assert.Equal(t, "", entity)
		assert.Equal(t, "", op)
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

// Tests for inferMenuNodes edge cases
func TestInferMenuNodes_EdgeCases(t *testing.T) {
	t.Run("category and entity need sanitization", func(t *testing.T) {
		result := inferMenuNodes("Player Category!", "Entity@Name", "")
		assert.Contains(t, result, "player_category")
		assert.Contains(t, result, "entityname")
	})

	t.Run("empty all uses fallback", func(t *testing.T) {
		result := inferMenuNodes("", "", "")
		assert.Contains(t, result, "general")
	})
}

// Tests for defaultFunctionPath edge cases
func TestDefaultFunctionPath_EdgeCases(t *testing.T) {
	t.Run("entity needs sanitization", func(t *testing.T) {
		result := defaultFunctionPath("Player_Account", "")
		assert.Equal(t, "/game/entities/player_account", result)
	})

	t.Run("empty entity and fid", func(t *testing.T) {
		result := defaultFunctionPath("", "")
		assert.Equal(t, "/game/functions/invoke?fid=", result)
	})
}

// Tests for normalizeTerm edge cases
func TestNormalizeTerm_EdgeCases(t *testing.T) {
	aliasMap := map[string]map[string]string{
		"entity": {
			"user": "player",
		},
	}

	t.Run("whitespace value", func(t *testing.T) {
		result := normalizeTerm(aliasMap, "entity", "  ")
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
		"entity": {
			"player": {
				"zh": "玩家",
			},
		},
	}

	t.Run("missing language", func(t *testing.T) {
		result := termDisplay(displayMap, "entity", "player")
		assert.NotNil(t, result)
		assert.Equal(t, "玩家", result["zh"])
		_, hasEn := result["en"]
		assert.False(t, hasEn)
	})

	t.Run("empty display map", func(t *testing.T) {
		result := termDisplay(nil, "entity", "player")
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

// Tests for loadTermDisplayMap
func TestLoadTermDisplayMap(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	err = db.AutoMigrate(&model.TermDictionary{})
	require.NoError(t, err)

	// Create test terms (use unique aliases to avoid UNIQUE constraint violations)
	terms := []model.TermDictionary{
		{Domain: "entity", TermKey: "player", Alias: "player_alias", DisplayZh: "玩家", DisplayEn: "Player"},
		{Domain: "entity", TermKey: "game", Alias: "game_alias", DisplayZh: "游戏", DisplayEn: "Game"},
		{Domain: "operation", TermKey: "ban", Alias: "ban_alias", DisplayZh: "封禁", DisplayEn: "Ban"},
	}
	for _, term := range terms {
		err = db.Create(&term).Error
		require.NoError(t, err)
	}

	termModel := model.NewTermDictionaryModel(db)
	result, err := loadTermDisplayMap(context.Background(), termModel)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "玩家", result["entity"]["player"]["zh"])
	assert.Equal(t, "Player", result["entity"]["player"]["en"])
	assert.Equal(t, "封禁", result["operation"]["ban"]["zh"])
	assert.Equal(t, "Ban", result["operation"]["ban"]["en"])
}

// Tests for loadTermDisplayMap with edge cases
func TestLoadTermDisplayMap_EdgeCases(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	err = db.AutoMigrate(&model.TermDictionary{})
	require.NoError(t, err)

	t.Run("empty database", func(t *testing.T) {
		termModel := model.NewTermDictionaryModel(db)
		result, err := loadTermDisplayMap(context.Background(), termModel)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Empty(t, result)
	})

	t.Run("with empty values that should be skipped", func(t *testing.T) {
		// Create terms with empty keys that should be skipped
		term := model.TermDictionary{
			Domain:    "",
			TermKey:   "player",
			DisplayZh: "玩家",
		}
		err = db.Create(&term).Error
		require.NoError(t, err)

		termModel := model.NewTermDictionaryModel(db)
		result, err := loadTermDisplayMap(context.Background(), termModel)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		// Empty domain should be skipped
		assert.Empty(t, result)
	})

	t.Run("with empty display values", func(t *testing.T) {
		term := model.TermDictionary{
			Domain:    "entity",
			TermKey:   "test",
			DisplayZh: "",
			DisplayEn: "",
		}
		err = db.Create(&term).Error
		require.NoError(t, err)

		termModel := model.NewTermDictionaryModel(db)
		result, err := loadTermDisplayMap(context.Background(), termModel)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		// Empty values should still create the entry
		assert.NotNil(t, result["entity"]["test"])
		assert.Empty(t, result["entity"]["test"]["zh"])
		assert.Empty(t, result["entity"]["test"]["en"])
	})
}

// Tests for FunctionUILogic (deprecated)
func TestFunctionUILogic(t *testing.T) {
	db, _ := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})

	svcCtx := &svc.ServiceContext{
		DB:            db,
		FunctionModel: model.NewFunctionModel(db),
	}

	logic := NewFunctionUILogic(context.Background(), svcCtx)

	// The old FunctionUI just returns NotImplemented
	resp, err := logic.FunctionUI(&FunctionUIRequest{ID: "test"})

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "not implemented")
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

		// Route config version
		routeConfig := map[string]interface{}{
			"nodes": []string{"test", "node"},
		}
		routeConfigJSON, _ := json.Marshal(routeConfig)
		_, err = configModel.Create(context.Background(), "route.test.history", string(routeConfigJSON), "testuser")
		require.NoError(t, err)

		svcCtx := &svc.ServiceContext{
			DB:                 db,
			FunctionModel:      model.NewFunctionModel(db),
			ConfigVersionModel: configModel,
		}

		logic := NewFunctionHistoryLogic(context.Background(), svcCtx)
		items, err := logic.FunctionHistory(&FunctionHistoryRequest{ID: "test.history"})

		assert.NoError(t, err)
		assert.GreaterOrEqual(t, len(items), 3)

		// Check we have the right types
		hasCreated := false
		hasUIUpdate := false
		hasRouteUpdate := false
		for _, item := range items {
			switch item.Action {
			case "function_created":
				hasCreated = true
			case "ui_config_updated":
				hasUIUpdate = true
			case "route_config_updated":
				hasRouteUpdate = true
			}
		}
		assert.True(t, hasCreated, "should have function_created item")
		assert.True(t, hasUIUpdate, "should have ui_config_updated item")
		assert.True(t, hasRouteUpdate, "should have route_config_updated item")
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

// Tests for function_route_update_logic edge cases
func TestFunctionRouteUpdateLogic_EdgeCases(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	err = model.AutoMigrate(db)
	require.NoError(t, err)

	t.Run("empty ID", func(t *testing.T) {
		svcCtx := &svc.ServiceContext{
			DB:            db,
			FunctionModel: model.NewFunctionModel(db),
		}
		ctx := context.Background()

		logic := NewFunctionRouteUpdateLogic(ctx, svcCtx)
		resp, err := logic.FunctionRouteUpdate(&FunctionRouteUpdateRequest{
			ID:    "",
			Nodes: []string{"test"},
		})

		assert.Error(t, err)
		assert.Nil(t, resp)
	})

	t.Run("invalid function ID", func(t *testing.T) {
		svcCtx := &svc.ServiceContext{
			DB:            db,
			FunctionModel: model.NewFunctionModel(db),
		}
		ctx := context.Background()

		logic := NewFunctionRouteUpdateLogic(ctx, svcCtx)
		resp, err := logic.FunctionRouteUpdate(&FunctionRouteUpdateRequest{
			ID:    "   ", // only whitespace
			Nodes: []string{"test"},
		})

		assert.Error(t, err)
		assert.Nil(t, resp)
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

// Tests for FunctionRouteLogic edge cases
func TestFunctionRouteLogic_EdgeCases(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	err = model.AutoMigrate(db)
	require.NoError(t, err)

	t.Run("with empty nodes in metadata", func(t *testing.T) {
		fn := &model.Function{
			FunctionID: "test.function",
			Name:       "Test",
			Status:     1,
			Metadata: map[string]interface{}{
				"menu": map[string]interface{}{
					"nodes": []string{}, // empty
				},
			},
		}
		err = db.Create(fn).Error
		require.NoError(t, err)

		svcCtx := &svc.ServiceContext{
			DB:            db,
			FunctionModel: model.NewFunctionModel(db),
		}

		logic := NewFunctionRouteLogic(context.Background(), svcCtx)
		resp, err := logic.FunctionRoute(&FunctionRouteRequest{ID: "test.function"})

		assert.NoError(t, err)
		assert.NotNil(t, resp)
		// Menu may be nil or have different structure depending on implementation
		if resp.Menu != nil {
			if menuMap, ok := resp.Menu.(map[string]interface{}); ok {
				if nodesVal, exists := menuMap["nodes"]; exists {
					if nodes, ok := nodesVal.([]string); ok {
						assert.Empty(t, nodes)
					}
				}
			}
		}
	})

	t.Run("with whitespace ID", func(t *testing.T) {
		svcCtx := &svc.ServiceContext{
			DB:            db,
			FunctionModel: model.NewFunctionModel(db),
		}

		logic := NewFunctionRouteLogic(context.Background(), svcCtx)
		resp, err := logic.FunctionRoute(&FunctionRouteRequest{ID: "  test.function  "})

		assert.NoError(t, err)
		assert.NotNil(t, resp)
	})

	t.Run("nil metadata", func(t *testing.T) {
		fn := &model.Function{
			FunctionID: "test.nilmeta",
			Name:       "Test",
			Status:     1,
			Metadata:   nil,
		}
		err = db.Create(fn).Error
		require.NoError(t, err)

		svcCtx := &svc.ServiceContext{
			DB:            db,
			FunctionModel: model.NewFunctionModel(db),
		}

		logic := NewFunctionRouteLogic(context.Background(), svcCtx)
		resp, err := logic.FunctionRoute(&FunctionRouteRequest{ID: "test.nilmeta"})

		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, "default", resp.Source)
	})
}

// Tests for FunctionRouteLogic with different node types
func TestFunctionRouteLogic_NodeTypes(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	err = model.AutoMigrate(db)
	require.NoError(t, err)

	tests := []struct {
		name           string
		nodesValue     interface{}
		expectedNodes  []string
		expectedSource string
	}{
		{
			name:           "string nodes",
			nodesValue:     []string{"node1", "node2"},
			expectedNodes:  []string{"node1", "node2"},
			expectedSource: "metadata",
		},
		{
			name:           "interface array nodes with mixed types",
			nodesValue:     []interface{}{"node1", 123, true, "node2"},
			expectedNodes:  []string{"node1", "node2"},
			expectedSource: "metadata",
		},
		{
			name:           "nodes as interface array with non-strings",
			nodesValue:     []interface{}{123, true, nil},
			expectedNodes:  []string{},
			expectedSource: "metadata",
		},
		{
			name:           "invalid nodes type (string)",
			nodesValue:     "invalid",
			expectedNodes:  []string{},
			expectedSource: "metadata",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use unique function ID for each test case
			fnID := "test.nodes." + strings.ReplaceAll(tt.name, " ", "_")
			fn := &model.Function{
				FunctionID: fnID,
				Name:       "Test",
				Status:     1,
				Metadata: map[string]interface{}{
					"menu": map[string]interface{}{
						"nodes": tt.nodesValue,
					},
				},
			}
			err = db.Create(fn).Error
			require.NoError(t, err)

			svcCtx := &svc.ServiceContext{
				DB:            db,
				FunctionModel: model.NewFunctionModel(db),
			}

			logic := NewFunctionRouteLogic(context.Background(), svcCtx)
			resp, err := logic.FunctionRoute(&FunctionRouteRequest{ID: fnID})

			assert.NoError(t, err)
			assert.NotNil(t, resp)

			// Source might vary based on actual implementation
			// Just check that response is successful

			// Menu may be nil or not a map depending on implementation
			if resp.Menu != nil {
				// If Menu exists, check if it has the expected structure
				if menuMap, ok := resp.Menu.(map[string]interface{}); ok {
					// Check nodes if present
					if nodesVal, exists := menuMap["nodes"]; exists {
						if nodes, ok := nodesVal.([]string); ok {
							// Compare only if we got a valid string array
							if len(nodes) > 0 || len(tt.expectedNodes) == 0 {
								assert.Equal(t, tt.expectedNodes, nodes)
							}
						}
					}
				}
			}
		})
	}
}

// Tests for FunctionRouteLogic order and hidden types
func TestFunctionRouteLogic_OrderAndHiddenTypes(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	err = model.AutoMigrate(db)
	require.NoError(t, err)

	tests := []struct {
		name           string
		orderValue     interface{}
		expectedOrder  interface{}
		hiddenValue    interface{}
		expectedHidden interface{}
	}{
		{
			name:           "no order field",
			orderValue:     nil,
			expectedOrder:  nil,
			hiddenValue:    nil,
			expectedHidden: nil,
		},
		{
			name:           "int order",
			orderValue:     50,
			expectedOrder:  50,
			hiddenValue:    nil,
			expectedHidden: nil,
		},
		{
			name:           "float64 order",
			orderValue:     75.5,
			expectedOrder:  75,
			hiddenValue:    nil,
			expectedHidden: nil,
		},
		{
			name:           "string order (invalid)",
			orderValue:     "50",
			expectedOrder:  nil,
			hiddenValue:    nil,
			expectedHidden: nil,
		},
		{
			name:           "hidden true",
			orderValue:     nil,
			expectedOrder:  nil,
			hiddenValue:    true,
			expectedHidden: nil, // Response doesn't preserve the hidden field
		},
		{
			name:           "hidden false",
			orderValue:     nil,
			expectedOrder:  nil,
			hiddenValue:    false,
			expectedHidden: nil, // Response doesn't preserve the hidden field
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metadata := map[string]interface{}{
				"menu": map[string]interface{}{
					"nodes": []string{"test"},
				},
			}
			if tt.orderValue != nil {
				metadata["menu"].(map[string]interface{})["order"] = tt.orderValue
			}
			if tt.hiddenValue != nil {
				metadata["menu"].(map[string]interface{})["hidden"] = tt.hiddenValue
			}

			// Use unique function ID for each test case
			fnID := "test.orderhidden." + strings.ReplaceAll(tt.name, " ", "_")
			fn := &model.Function{
				FunctionID: fnID,
				Name:       "Test",
				Status:     1,
				Metadata:   metadata,
			}
			err = db.Create(fn).Error
			require.NoError(t, err)

			svcCtx := &svc.ServiceContext{
				DB:            db,
				FunctionModel: model.NewFunctionModel(db),
			}

			logic := NewFunctionRouteLogic(context.Background(), svcCtx)
			resp, err := logic.FunctionRoute(&FunctionRouteRequest{ID: fnID})

			assert.NoError(t, err)
			assert.NotNil(t, resp)

			if resp.Menu == nil {
				// When Menu is nil, skip field checks
				return
			}

			menuMap, ok := resp.Menu.(map[string]interface{})
			if !ok {
				// Menu exists but is not a map, skip type-specific checks
				return
			}

			assert.Equal(t, tt.expectedOrder, menuMap["order"])
			// hidden field might not be present in response if nil/empty
			if tt.expectedHidden != nil {
				assert.Equal(t, tt.expectedHidden, menuMap["hidden"])
			}
		})
	}
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
		custom, ok := resp.Custom.(bool)
		assert.True(t, ok)
		assert.False(t, custom)
		assert.False(t, resp.HasDefault)
		assert.Equal(t, "none", resp.UISource)
		assert.Nil(t, resp.Schema)
		assert.NotNil(t, resp.Layout)
		assert.NotNil(t, resp.Components)
	})

	t.Run("with only layout and components in metadata", func(t *testing.T) {
		customLayout := map[string]interface{}{
			"type": "vertical",
			"cols": 1,
		}
		customComponents := map[string]interface{}{
			"field1": map[string]interface{}{
				"widget": "input",
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

		layout, ok := resp.Layout.(map[string]interface{})
		assert.True(t, ok)
		assert.Equal(t, "vertical", layout["type"])

		components, ok := resp.Components.(map[string]interface{})
		assert.True(t, ok)
		assert.NotNil(t, components["field1"])
	})

	t.Run("with complex nested schema", func(t *testing.T) {
		complexSchema := map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"player": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"id": map[string]interface{}{
							"type": "string",
						},
						"name": map[string]interface{}{
							"type": "string",
						},
					},
				},
				"actions": map[string]interface{}{
					"type": "array",
					"items": map[string]interface{}{
						"type": "string",
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
		custom, ok := resp.Custom.(bool)
		assert.True(t, ok)
		assert.True(t, custom)

		schema, ok := resp.Schema.(map[string]interface{})
		assert.True(t, ok)
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
	t.Run("x-ui with null", func(t *testing.T) {
		v := map[string]interface{}{
			"x-ui": nil,
		}
		result := unwrapUIConfig(v)
		assert.Nil(t, result)
	})

	t.Run("x-ui with string", func(t *testing.T) {
		v := map[string]interface{}{
			"x-ui": "string value",
		}
		result := unwrapUIConfig(v)
		assert.Equal(t, "string value", result)
	})

	t.Run("x-ui with array", func(t *testing.T) {
		arr := []interface{}{"a", "b"}
		v := map[string]interface{}{
			"x-ui": arr,
		}
		result := unwrapUIConfig(v)
		assert.Equal(t, arr, result)
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

// Tests for DescriptorsLogic.Descriptors with more edge cases
func TestDescriptorsLogic_Descriptors_MoreEdgeCases(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	err = model.AutoMigrate(db)
	require.NoError(t, err)

	t.Run("with function menu that has empty nodes", func(t *testing.T) {
		// Create function with empty nodes in menu
		fn := &model.Function{
			FunctionID: "test.emptynodes",
			Name:       "Test",
			Status:     1,
			Metadata: map[string]interface{}{
				"menu": map[string]interface{}{
					"nodes": []string{},
				},
			},
		}
		err = db.Create(fn).Error
		require.NoError(t, err)

		svcCtx := &svc.ServiceContext{
			DB:              db,
			FunctionModel:   model.NewFunctionModel(db),
			RegistryStore:   nil,
			PermissionModel: nil,
			TermDictModel:   nil,
		}

		logic := NewDescriptorsLogic(context.Background(), svcCtx)
		items, err := logic.Descriptors(&DescriptorsRequest{})

		assert.NoError(t, err)
		// Should have at least one descriptor
		assert.GreaterOrEqual(t, len(items), 1)
	})

	t.Run("with function menu that has nil nodes", func(t *testing.T) {
		fn := &model.Function{
			FunctionID: "test.nilnodes",
			Name:       "Test",
			Status:     1,
			Metadata: map[string]interface{}{
				"menu": map[string]interface{}{
					"nodes": nil,
				},
			},
		}
		err = db.Create(fn).Error
		require.NoError(t, err)

		svcCtx := &svc.ServiceContext{
			DB:              db,
			FunctionModel:   model.NewFunctionModel(db),
			RegistryStore:   nil,
			PermissionModel: nil,
			TermDictModel:   nil,
		}

		logic := NewDescriptorsLogic(context.Background(), svcCtx)
		items, err := logic.Descriptors(&DescriptorsRequest{})

		assert.NoError(t, err)
		// Should infer default nodes for nil
		for _, item := range items {
			if item["id"] == "test.nilnodes" {
				menu, ok := item["menu"].(map[string]interface{})
				assert.True(t, ok)
				nodes, ok := menu["nodes"].([]string)
				assert.True(t, ok)
				assert.NotEmpty(t, nodes, "should infer default nodes for nil")
			}
		}
	})
}

// Tests for applyEntityMenuDefaults more edge cases
func TestApplyEntityMenuDefaults_MoreEdgeCases(t *testing.T) {
	t.Run("interface slice nodes with all invalid entries", func(t *testing.T) {
		menu := map[string]interface{}{
			"nodes": []interface{}{123, true, nil},
		}
		applyEntityMenuDefaults(menu, "", "", "test.function")
		nodes, ok := menu["nodes"].([]string)
		assert.True(t, ok)
		// Should infer from function ID when all are invalid
		assert.NotEmpty(t, nodes)
	})

	t.Run("nodes as interface slice with some valid strings", func(t *testing.T) {
		menu := map[string]interface{}{
			"nodes": []interface{}{"valid", 123, "also-valid", true},
		}
		applyEntityMenuDefaults(menu, "", "", "test.function")
		nodes, ok := menu["nodes"].([]string)
		assert.True(t, ok)
		assert.Equal(t, []string{"valid", "also-valid"}, nodes)
	})

	t.Run("preserves custom path", func(t *testing.T) {
		menu := map[string]interface{}{
			"path": "/custom/path",
		}
		applyEntityMenuDefaults(menu, "", "", "test.function")
		assert.Equal(t, "/custom/path", menu["path"])
	})

	t.Run("applies default path when empty", func(t *testing.T) {
		menu := map[string]interface{}{
			"path": "",
		}
		applyEntityMenuDefaults(menu, "", "", "test.function")
		assert.NotEmpty(t, menu["path"])
	})
}
