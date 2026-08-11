package routes

import (
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
)

func TestExtractObjectName(t *testing.T) {
	logic := &GetRoutesLogic{}

	tests := []struct {
		name       string
		functionID string
		expected   string
	}{
		{
			name:       "with dot",
			functionID: "player.getList",
			expected:   "player",
		},
		{
			name:       "no dot",
			functionID: "getPlayer",
			expected:   "getPlayer",
		},
		{
			name:       "multiple dots",
			functionID: "game.player.getList",
			expected:   "game",
		},
		{
			name:       "empty string",
			functionID: "",
			expected:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := logic.extractObjectName(tt.functionID)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetActionName(t *testing.T) {
	logic := &GetRoutesLogic{}

	tests := []struct {
		name       string
		functionID string
		expected   string
	}{
		{
			name:       "with dot",
			functionID: "player.getList",
			expected:   "getList",
		},
		{
			name:       "no dot",
			functionID: "getPlayer",
			expected:   "getPlayer",
		},
		{
			name:       "multiple dots",
			functionID: "game.player.getList",
			expected:   "player.getList",
		},
		{
			name:       "empty string",
			functionID: "",
			expected:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := logic.getActionName(tt.functionID)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetDisplayName(t *testing.T) {
	logic := &GetRoutesLogic{}

	tests := []struct {
		name     string
		fn       model.Function
		expected string
	}{
		{
			name:     "with name",
			fn:       model.Function{Name: "Get Player List"},
			expected: "Get Player List",
		},
		{
			name:     "without name",
			fn:       model.Function{FunctionID: "player.getList"},
			expected: "player.getList",
		},
		{
			name:     "empty name",
			fn:       model.Function{Name: "", FunctionID: "player.getList"},
			expected: "player.getList",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := logic.getDisplayName(tt.fn)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetIcon(t *testing.T) {
	logic := &GetRoutesLogic{}

	tests := []struct {
		name       string
		objectName string
		expected   string
	}{
		{
			name:       "player",
			objectName: "player",
			expected:   "user",
		},
		{
			name:       "item",
			objectName: "item",
			expected:   "inbox",
		},
		{
			name:       "quest",
			objectName: "quest",
			expected:   "file-text",
		},
		{
			name:       "guild",
			objectName: "guild",
			expected:   "team",
		},
		{
			name:       "mail",
			objectName: "mail",
			expected:   "mail",
		},
		{
			name:       "shop",
			objectName: "shop",
			expected:   "shopping-cart",
		},
		{
			name:       "battle",
			objectName: "battle",
			expected:   "thunderbolt",
		},
		{
			name:       "chat",
			objectName: "chat",
			expected:   "message",
		},
		{
			name:       "ranking",
			objectName: "ranking",
			expected:   "trophy",
		},
		{
			name:       "activity",
			objectName: "activity",
			expected:   "calendar",
		},
		{
			name:       "system",
			objectName: "system",
			expected:   "setting",
		},
		{
			name:       "unknown",
			objectName: "unknown",
			expected:   "api",
		},
		{
			name:       "empty",
			objectName: "",
			expected:   "api",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := logic.getIcon(tt.objectName)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBuildRoute(t *testing.T) {
	logic := &GetRoutesLogic{}

	functions := []model.Function{
		{
			FunctionID: "player.getList",
			Name:       "Get Player List",
			Resource:   "player",
		},
		{
			FunctionID: "player.getById",
			Name:       "Get Player By ID",
			Resource:   "player",
		},
	}

	route := logic.buildRoute("player", functions)

	assert.Equal(t, "/functions/player", route.Path)
	assert.Equal(t, "PlayerFunctions", route.Name)
	assert.Equal(t, "user", route.Icon) // player -> user icon
	assert.Len(t, route.Routes, 2)

	// Check first sub-route
	assert.Equal(t, "/functions/player/getList", route.Routes[0].Path)
	assert.Equal(t, "Get Player List", route.Routes[0].Name)
	assert.Equal(t, "../pages/Functions/DynamicInvoker", route.Routes[0].Component)
	assert.Equal(t, "player.getList", route.Routes[0].Meta["functionId"])

	// Check second sub-route
	assert.Equal(t, "/functions/player/getById", route.Routes[1].Path)
}

func TestBuildRoute_EmptyResource(t *testing.T) {
	logic := &GetRoutesLogic{}

	functions := []model.Function{
		{
			FunctionID: "item.create",
			Name:       "Create Item",
			Resource:   "", // Empty resource
		},
	}

	route := logic.buildRoute("item", functions)

	assert.Len(t, route.Routes, 1)
	// Should use objectName as resource
	assert.Equal(t, "item", route.Routes[0].Meta["resource"])
}

func TestNewGetRoutesLogic(t *testing.T) {
	logic := NewGetRoutesLogic(nil, nil)
	assert.NotNil(t, logic)
}

func TestGetRoutesLogic_Methods(t *testing.T) {
	logic := &GetRoutesLogic{
		ctx:    nil,
		svcCtx: nil,
	}

	// Test extractObjectName with various inputs
	assert.Equal(t, "player", logic.extractObjectName("player.getList"))
	assert.Equal(t, "item", logic.extractObjectName("item.create"))
	assert.Equal(t, "system", logic.extractObjectName("system.config"))
}

func TestBuildRoute_SingleFunction(t *testing.T) {
	logic := &GetRoutesLogic{}

	functions := []model.Function{
		{
			FunctionID: "guild.info",
			Name:       "Guild Info",
			Resource:   "guild",
		},
	}

	route := logic.buildRoute("guild", functions)

	assert.Equal(t, "/functions/guild", route.Path)
	assert.Equal(t, "GuildFunctions", route.Name)
	assert.Equal(t, "team", route.Icon) // guild -> team icon
	assert.Len(t, route.Routes, 1)
	assert.Equal(t, "/functions/guild/info", route.Routes[0].Path)
}

func TestBuildRoute_EmptyFunctionID(t *testing.T) {
	logic := &GetRoutesLogic{}

	functions := []model.Function{
		{
			FunctionID: "",
			Name:       "Empty Function",
			Resource:   "test",
		},
	}

	route := logic.buildRoute("test", functions)

	assert.Len(t, route.Routes, 1)
	// Empty functionID should still work
	assert.Equal(t, "/functions/test/", route.Routes[0].Path)
}
