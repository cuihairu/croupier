package function

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupFunctionTestContextSimple(t *testing.T) (*svc.ServiceContext, context.Context) {
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
		RegistryStore:   nil,
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

func TestNewFunctionInvokeLogic(t *testing.T) {
	svcCtx, ctx := setupFunctionTestContextSimple(t)

	logic := NewFunctionInvokeLogic(ctx, svcCtx)

	assert.NotNil(t, logic)
	assert.Equal(t, ctx, logic.ctx)
	assert.Equal(t, svcCtx, logic.svcCtx)
}

func TestNewFunctionDetailLogic(t *testing.T) {
	svcCtx, ctx := setupFunctionTestContextSimple(t)

	logic := NewFunctionDetailLogic(ctx, svcCtx)

	assert.NotNil(t, logic)
	assert.Equal(t, ctx, logic.ctx)
	assert.Equal(t, svcCtx, logic.svcCtx)
}

func TestNewFunctionsListLogic(t *testing.T) {
	svcCtx, ctx := setupFunctionTestContextSimple(t)

	logic := NewFunctionsListLogic(ctx, svcCtx)

	assert.NotNil(t, logic)
	assert.Equal(t, ctx, logic.ctx)
	assert.Equal(t, svcCtx, logic.svcCtx)
}

func TestExtractRoleNamesFromModels(t *testing.T) {
	roles := []model.Role{
		{Name: "admin"},
		{Name: "user"},
		{Name: "guest"},
	}

	result := ExtractRoleNames(roles)

	assert.Equal(t, []string{"admin", "user", "guest"}, result)
}

func TestExtractRoleNamesFromModelsEmpty(t *testing.T) {
	result := ExtractRoleNames([]model.Role{})
	assert.NotNil(t, result)
	assert.Empty(t, result)
}

func TestFunctionHistoryRequestValidation(t *testing.T) {
	t.Run("valid request", func(t *testing.T) {
		req := &FunctionHistoryRequest{
			ID: "test.function",
		}

		_, err := utils.ValidateFunctionID(req.ID)
		assert.NoError(t, err)
	})

	t.Run("empty function ID", func(t *testing.T) {
		req := &FunctionHistoryRequest{
			ID: "",
		}

		_, err := utils.ValidateFunctionID(req.ID)
		assert.Error(t, err)
	})
}

func TestFunctionInvokeRequestValidation(t *testing.T) {
	t.Run("empty function ID", func(t *testing.T) {
		req := &FunctionInvokeRequest{
			ID: "",
		}

		_, err := utils.ValidateFunctionID(req.ID)
		assert.Error(t, err)
	})

	t.Run("valid function ID with whitespace", func(t *testing.T) {
		req := &FunctionInvokeRequest{
			ID: "  test.function  ",
		}

		got, err := utils.ValidateFunctionID(req.ID)
		assert.NoError(t, err)
		assert.Equal(t, "test.function", got)
	})
}

func TestNewFunctionHistoryLogic(t *testing.T) {
	svcCtx, ctx := setupFunctionTestContextSimple(t)

	logic := NewFunctionHistoryLogic(ctx, svcCtx)

	assert.NotNil(t, logic)
	assert.Equal(t, ctx, logic.ctx)
	assert.Equal(t, svcCtx, logic.svcCtx)
}

func TestConvertHelpers(t *testing.T) {
	t.Run("convertFromUtilsFunction", func(t *testing.T) {
		u := utils.Function{
			Id:          "test.fn",
			Name:        "Test Function",
			Description: "Test",
			GameId:      "game1",
			Status:      1,
			Version:     "1.0",
			Instances:   3,
		}

		result := convertFromUtilsFunction(u)

		assert.Equal(t, "test.fn", result.ID)
		assert.Equal(t, "Test Function", result.Name)
		assert.Equal(t, "game1", result.GameId)
		assert.Equal(t, 1, result.Status)
	})

	t.Run("convertFromUtilsFunctionSlice", func(t *testing.T) {
		utilsFuncs := []utils.Function{
			{Id: "fn1"},
			{Id: "fn2"},
		}

		result := convertFromUtilsFunctionSlice(utilsFuncs)

		assert.Len(t, result, 2)
		assert.Equal(t, "fn1", result[0].ID)
		assert.Equal(t, "fn2", result[1].ID)
	})

	t.Run("convertToUtilsPermission", func(t *testing.T) {
		f := FunctionPermission{
			Resource: "test.resource",
			Actions:  []string{"read"},
			Roles:    []string{"admin"},
		}

		result := convertToUtilsPermission(f)

		assert.Equal(t, "test.resource", result.Resource)
		assert.Equal(t, []string{"read"}, result.Actions)
	})

	t.Run("convertFromUtilsPermissions", func(t *testing.T) {
		utilsPerms := []utils.FunctionPermission{
			{Resource: "r1"},
			{Resource: "r2"},
		}

		result := convertFromUtilsPermissions(utilsPerms)

		assert.Len(t, result, 2)
		assert.Equal(t, "r1", result[0].Resource)
		assert.Equal(t, "r2", result[1].Resource)
	})

	t.Run("convertToUtilsPermissions", func(t *testing.T) {
		funcPerms := []FunctionPermission{
			{Resource: "r1"},
			{Resource: "r2"},
		}

		result := convertToUtilsPermissions(funcPerms)

		assert.Len(t, result, 2)
		assert.Equal(t, "r1", result[0].Resource)
		assert.Equal(t, "r2", result[1].Resource)
	})
}
