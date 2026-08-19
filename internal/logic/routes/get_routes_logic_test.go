package routes

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupRoutesSvc(t *testing.T) (*svc.ServiceContext, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Function{}))
	return &svc.ServiceContext{FunctionModel: model.NewFunctionModel(db)}, db
}

func TestGetRoutes_GroupsByObject(t *testing.T) {
	svcCtx, _ := setupRoutesSvc(t)
	ctx := context.Background()

	fns := []model.Function{
		{FunctionID: "player.getList", Name: "Get Player List", Resource: "player", Status: 1},
		{FunctionID: "player.getById", Resource: "player", Status: 1},
		{FunctionID: "item.create", Name: "Create Item", Status: 1},
		{FunctionID: "disabled.fn", Status: 0},
	}
	for i := range fns {
		require.NoError(t, svcCtx.FunctionModel.Create(ctx, &fns[i]))
	}

	resp, err := NewGetRoutesLogic(ctx, svcCtx).GetRoutes()
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, 0, resp.Code)
	assert.Equal(t, "OK", resp.Message)
	require.Len(t, resp.Data, 2)

	byPath := map[string]RouteItem{}
	for _, r := range resp.Data {
		byPath[r.Path] = r
	}

	player := byPath["/functions/player"]
	require.Len(t, player.Routes, 2)
	assert.Equal(t, "PlayerFunctions", player.Name)
	assert.Equal(t, "user", player.Icon)

	item := byPath["/functions/item"]
	require.Len(t, item.Routes, 1)
	// Empty resource falls back to the object name.
	assert.Equal(t, "item", item.Routes[0].Meta["resource"])
	assert.Equal(t, "Create Item", item.Routes[0].Name)
	assert.Equal(t, "/functions/item/create", item.Routes[0].Path)
	assert.Equal(t, "item.create", item.Routes[0].Meta["functionId"])
}

func TestGetRoutes_NoFunctions(t *testing.T) {
	svcCtx, _ := setupRoutesSvc(t)

	resp, err := NewGetRoutesLogic(context.Background(), svcCtx).GetRoutes()
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Empty(t, resp.Data)
}

func TestGetRoutes_ListError(t *testing.T) {
	svcCtx, db := setupRoutesSvc(t)
	require.NoError(t, db.Migrator().DropTable(&model.Function{}))

	resp, err := NewGetRoutesLogic(context.Background(), svcCtx).GetRoutes()
	require.Error(t, err)
	assert.Nil(t, resp)
}
