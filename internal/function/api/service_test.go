// Package api tests the service layer.
package api

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/function/registry"
	functionv1 "github.com/cuihairu/croupier/pkg/pb/croupier/function/v1"
	"github.com/stretchr/testify/assert"
)

func TestService_RegisterBatch(t *testing.T) {
	store := registry.NewStore()
	service := NewService(store)

	metadatas := []*functionv1.FunctionMetadata{
		{
			Id:       "player.get",
			Name:     "Get Player",
			Security: &functionv1.FunctionSecurity{},
			Behavior: &functionv1.FunctionBehavior{},
		},
		{
			Id:       "player.update",
			Name:     "Update Player",
			Security: &functionv1.FunctionSecurity{},
			Behavior: &functionv1.FunctionBehavior{},
		},
	}

	err := service.RegisterBatch(context.Background(), metadatas)
	assert.Nil(t, err)
	assert.Equal(t, 2, store.Count(context.Background()))
}

func TestService_Update(t *testing.T) {
	store := registry.NewStore()
	service := NewService(store)

	// Register initial function
	original := &functionv1.FunctionMetadata{
		Id:          "player.get",
		Name:        "Get Player",
		Description: "Original description",
		Security:    &functionv1.FunctionSecurity{},
		Behavior:    &functionv1.FunctionBehavior{},
	}
	service.Register(context.Background(), original)

	// Update with new values
	updated := &functionv1.FunctionMetadata{
		Id:          "player.get",
		Name:        "Get Player Updated",
		Description: "New description",
		Version:     "2.0.0",
		Security:    &functionv1.FunctionSecurity{},
		Behavior:    &functionv1.FunctionBehavior{},
	}

	err := service.Update(context.Background(), "player.get", updated)
	assert.Nil(t, err)

	// Verify update
	retrieved, _ := service.Get(context.Background(), "player.get")
	assert.Equal(t, "Get Player Updated", retrieved.Name)
	assert.Equal(t, "New description", retrieved.Description)
	assert.Equal(t, "2.0.0", retrieved.Version)
}

func TestService_Update_IDMismatch(t *testing.T) {
	store := registry.NewStore()
	service := NewService(store)

	service.Register(context.Background(), &functionv1.FunctionMetadata{
		Id:       "player.get",
		Name:     "Get Player",
		Security: &functionv1.FunctionSecurity{},
		Behavior: &functionv1.FunctionBehavior{},
	})

	// Try to update with different ID
	updated := &functionv1.FunctionMetadata{
		Id:       "player.update", // Different ID
		Name:     "Updated",
		Security: &functionv1.FunctionSecurity{},
		Behavior: &functionv1.FunctionBehavior{},
	}

	err := service.Update(context.Background(), "player.get", updated)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "ID mismatch")
}

func TestService_Update_NotFound(t *testing.T) {
	store := registry.NewStore()
	service := NewService(store)

	updated := &functionv1.FunctionMetadata{
		Id:       "not.found",
		Name:     "Updated",
		Security: &functionv1.FunctionSecurity{},
		Behavior: &functionv1.FunctionBehavior{},
	}

	err := service.Update(context.Background(), "not.found", updated)
	assert.Equal(t, ErrNotFound, err)
}

func TestService_List_WithFilters(t *testing.T) {
	store := registry.NewStore()
	service := NewService(store)

	// Register test functions
	service.Register(context.Background(), &functionv1.FunctionMetadata{
		Id:       "player.get",
		Resource: "player",
		Tags:     []string{"read"},
		Security: &functionv1.FunctionSecurity{RiskLevel: functionv1.FunctionSecurity_RISK_LEVEL_LOW},
		Behavior: &functionv1.FunctionBehavior{Mode: functionv1.FunctionBehavior_MODE_QUERY},
	})
	service.Register(context.Background(), &functionv1.FunctionMetadata{
		Id:       "game.create",
		Resource: "game",
		Tags:     []string{"write"},
		Security: &functionv1.FunctionSecurity{RiskLevel: functionv1.FunctionSecurity_RISK_LEVEL_HIGH},
		Behavior: &functionv1.FunctionBehavior{Mode: functionv1.FunctionBehavior_MODE_COMMAND},
	})

	t.Run("filter by resource", func(t *testing.T) {
		result, err := service.List(context.Background(), &ListOptions{Resource: "player"})
		assert.Nil(t, err)
		assert.Equal(t, 1, result.Total)
	})

	t.Run("filter by tag", func(t *testing.T) {
		result, err := service.List(context.Background(), &ListOptions{Tag: "read"})
		assert.Nil(t, err)
		assert.Equal(t, 1, result.Total)
	})

	t.Run("filter by risk level", func(t *testing.T) {
		result, err := service.List(context.Background(), &ListOptions{RiskLevel: "low"})
		assert.Nil(t, err)
		assert.Equal(t, 1, result.Total)
	})

	t.Run("filter by mode", func(t *testing.T) {
		result, err := service.List(context.Background(), &ListOptions{Mode: "command"})
		assert.Nil(t, err)
		assert.Equal(t, 1, result.Total)
	})
}

func TestNormalizeRiskLevelForStore(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"low", "low"},
		{"LOW", "low"},
		{"RISK_LOW", "low"},
		{"RISK_LEVEL_LOW", "low"},
		{"medium", "medium"},
		{"high", "high"},
		{"danger", "danger"},
		{"unknown", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := normalizeRiskLevelForStore(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNormalizeModeForStore(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"query", "query"},
		{"QUERY", "query"},
		{"MODE_QUERY", "query"},
		{"command", "command"},
		{"COMMAND", "command"},
		{"MODE_COMMAND", "command"},
		{"unknown", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := normalizeModeForStore(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
