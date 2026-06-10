package registry

import (
	"context"
	"testing"

	functionv1 "github.com/cuihairu/croupier/pkg/pb/croupier/function/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStore_Get_EmptyID(t *testing.T) {
	ctx := context.Background()
	store := NewStore()

	_, err := store.Get(ctx, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "function ID is required")
}

func TestStore_Get_NotFound(t *testing.T) {
	ctx := context.Background()
	store := NewStore()

	_, err := store.Get(ctx, "nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "function not found")
}

func TestStore_List_EmptyStore(t *testing.T) {
	ctx := context.Background()
	store := NewStore()

	result, err := store.List(ctx)
	assert.NoError(t, err)
	assert.Empty(t, result)
}

func TestStore_ListByCategory_EmptyCategory(t *testing.T) {
	ctx := context.Background()
	store := NewStore()

	// Register some functions
	store.Register(ctx, &functionv1.FunctionMetadata{
		Id:       "func1",
		Category: "test",
		Behavior: &functionv1.FunctionBehavior{Mode: functionv1.FunctionBehavior_MODE_QUERY},
		Security: &functionv1.FunctionSecurity{RiskLevel: functionv1.FunctionSecurity_RISK_LEVEL_LOW},
	})

	// Empty category should return all functions
	result, err := store.ListByCategory(ctx, "")
	assert.NoError(t, err)
	assert.Len(t, result, 1)
}

func TestStore_ListByCategory_NotFound(t *testing.T) {
	ctx := context.Background()
	store := NewStore()

	result, err := store.ListByCategory(ctx, "nonexistent")
	assert.NoError(t, err)
	assert.Empty(t, result)
}

func TestStore_ListByTag_EmptyTag(t *testing.T) {
	ctx := context.Background()
	store := NewStore()

	_, err := store.ListByTag(ctx, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "tag is required")
}

func TestStore_ListByTag_NotFound(t *testing.T) {
	ctx := context.Background()
	store := NewStore()

	result, err := store.ListByTag(ctx, "nonexistent")
	assert.NoError(t, err)
	assert.Empty(t, result)
}

func TestStore_ListByRiskLevel_EmptyRiskLevel(t *testing.T) {
	ctx := context.Background()
	store := NewStore()

	_, err := store.ListByRiskLevel(ctx, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "risk level is required")
}

func TestStore_ListByRiskLevel_NotFound(t *testing.T) {
	ctx := context.Background()
	store := NewStore()

	result, err := store.ListByRiskLevel(ctx, "nonexistent")
	assert.NoError(t, err)
	assert.Empty(t, result)
}

func TestStore_ListByMode_EmptyStore(t *testing.T) {
	ctx := context.Background()
	store := NewStore()

	result, err := store.ListByMode(ctx, functionv1.FunctionBehavior_MODE_QUERY)
	assert.NoError(t, err)
	assert.Empty(t, result)
}

func TestStore_Unregister_EmptyID(t *testing.T) {
	ctx := context.Background()
	store := NewStore()

	err := store.Unregister(ctx, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "function ID is required")
}

func TestStore_Unregister_NotFound(t *testing.T) {
	ctx := context.Background()
	store := NewStore()

	err := store.Unregister(ctx, "nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "function not found")
}

func TestStore_Exists_EmptyID(t *testing.T) {
	ctx := context.Background()
	store := NewStore()

	result := store.Exists(ctx, "")
	assert.False(t, result)
}

func TestStore_Exists_NotFound(t *testing.T) {
	ctx := context.Background()
	store := NewStore()

	result := store.Exists(ctx, "nonexistent")
	assert.False(t, result)
}

func TestStore_Exists_Found(t *testing.T) {
	ctx := context.Background()
	store := NewStore()

	store.Register(ctx, &functionv1.FunctionMetadata{
		Id:       "test.func",
		Category: "test",
		Behavior: &functionv1.FunctionBehavior{Mode: functionv1.FunctionBehavior_MODE_QUERY},
		Security: &functionv1.FunctionSecurity{RiskLevel: functionv1.FunctionSecurity_RISK_LEVEL_LOW},
	})

	result := store.Exists(ctx, "test.func")
	assert.True(t, result)
}

func TestStore_Count_EmptyStore(t *testing.T) {
	ctx := context.Background()
	store := NewStore()

	count := store.Count(ctx)
	assert.Equal(t, 0, count)
}

func TestStore_GetCategories_EmptyStore(t *testing.T) {
	ctx := context.Background()
	store := NewStore()

	categories := store.GetCategories(ctx)
	assert.Empty(t, categories)
}

func TestStore_GetTags_EmptyStore(t *testing.T) {
	ctx := context.Background()
	store := NewStore()

	tags := store.GetTags(ctx)
	assert.Empty(t, tags)
}

func TestStore_GetCreatedAt_EmptyID(t *testing.T) {
	ctx := context.Background()
	store := NewStore()

	_, err := store.GetCreatedAt(ctx, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "function ID is required")
}

func TestStore_GetCreatedAt_NotFound(t *testing.T) {
	ctx := context.Background()
	store := NewStore()

	_, err := store.GetCreatedAt(ctx, "nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "function not found")
}

func TestStore_GetCreatedAt_Found(t *testing.T) {
	ctx := context.Background()
	store := NewStore()

	store.Register(ctx, &functionv1.FunctionMetadata{
		Id:       "test.func",
		Category: "test",
		Behavior: &functionv1.FunctionBehavior{Mode: functionv1.FunctionBehavior_MODE_QUERY},
		Security: &functionv1.FunctionSecurity{RiskLevel: functionv1.FunctionSecurity_RISK_LEVEL_LOW},
	})

	createdAt, err := store.GetCreatedAt(ctx, "test.func")
	assert.NoError(t, err)
	assert.False(t, createdAt.IsZero())
}

func TestStore_GetUpdatedAt_EmptyID(t *testing.T) {
	ctx := context.Background()
	store := NewStore()

	_, err := store.GetUpdatedAt(ctx, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "function ID is required")
}

func TestStore_GetUpdatedAt_NotFound(t *testing.T) {
	ctx := context.Background()
	store := NewStore()

	_, err := store.GetUpdatedAt(ctx, "nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "function not found")
}

func TestStore_GetUpdatedAt_Found(t *testing.T) {
	ctx := context.Background()
	store := NewStore()

	store.Register(ctx, &functionv1.FunctionMetadata{
		Id:       "test.func",
		Category: "test",
		Behavior: &functionv1.FunctionBehavior{Mode: functionv1.FunctionBehavior_MODE_QUERY},
		Security: &functionv1.FunctionSecurity{RiskLevel: functionv1.FunctionSecurity_RISK_LEVEL_LOW},
	})

	updatedAt, err := store.GetUpdatedAt(ctx, "test.func")
	assert.NoError(t, err)
	assert.False(t, updatedAt.IsZero())
}

func TestStore_Filter_ByRiskLevel(t *testing.T) {
	ctx := context.Background()
	store := NewStore()

	store.RegisterBatch(ctx, []*functionv1.FunctionMetadata{
		{
			Id:       "func.low",
			Category: "test",
			Behavior: &functionv1.FunctionBehavior{Mode: functionv1.FunctionBehavior_MODE_QUERY},
			Security: &functionv1.FunctionSecurity{RiskLevel: functionv1.FunctionSecurity_RISK_LEVEL_LOW},
		},
		{
			Id:       "func.high",
			Category: "test",
			Behavior: &functionv1.FunctionBehavior{Mode: functionv1.FunctionBehavior_MODE_QUERY},
			Security: &functionv1.FunctionSecurity{RiskLevel: functionv1.FunctionSecurity_RISK_LEVEL_HIGH},
		},
	})

	// Use the normalized enum name format
	filter := &functionv1.FunctionFilter{
		RiskLevel: "low",
	}
	result, err := store.Filter(ctx, filter)
	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "func.low", result[0].Id)
}

func TestStore_Filter_ByTag(t *testing.T) {
	ctx := context.Background()
	store := NewStore()

	store.RegisterBatch(ctx, []*functionv1.FunctionMetadata{
		{
			Id:       "func1",
			Category: "test",
			Tags:     []string{"tag1"},
			Behavior: &functionv1.FunctionBehavior{Mode: functionv1.FunctionBehavior_MODE_QUERY},
			Security: &functionv1.FunctionSecurity{RiskLevel: functionv1.FunctionSecurity_RISK_LEVEL_LOW},
		},
		{
			Id:       "func2",
			Category: "test",
			Tags:     []string{"tag2"},
			Behavior: &functionv1.FunctionBehavior{Mode: functionv1.FunctionBehavior_MODE_QUERY},
			Security: &functionv1.FunctionSecurity{RiskLevel: functionv1.FunctionSecurity_RISK_LEVEL_LOW},
		},
	})

	filter := &functionv1.FunctionFilter{
		Tags: []string{"tag1"},
	}
	result, err := store.Filter(ctx, filter)
	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "func1", result[0].Id)
}

func TestStore_Filter_WithPageSize(t *testing.T) {
	ctx := context.Background()
	store := NewStore()

	store.RegisterBatch(ctx, []*functionv1.FunctionMetadata{
		{
			Id:       "func1",
			Category: "test",
			Behavior: &functionv1.FunctionBehavior{Mode: functionv1.FunctionBehavior_MODE_QUERY},
			Security: &functionv1.FunctionSecurity{RiskLevel: functionv1.FunctionSecurity_RISK_LEVEL_LOW},
		},
		{
			Id:       "func2",
			Category: "test",
			Behavior: &functionv1.FunctionBehavior{Mode: functionv1.FunctionBehavior_MODE_QUERY},
			Security: &functionv1.FunctionSecurity{RiskLevel: functionv1.FunctionSecurity_RISK_LEVEL_LOW},
		},
		{
			Id:       "func3",
			Category: "test",
			Behavior: &functionv1.FunctionBehavior{Mode: functionv1.FunctionBehavior_MODE_QUERY},
			Security: &functionv1.FunctionSecurity{RiskLevel: functionv1.FunctionSecurity_RISK_LEVEL_LOW},
		},
	})

	filter := &functionv1.FunctionFilter{
		PageSize: 2,
	}
	result, err := store.Filter(ctx, filter)
	require.NoError(t, err)
	assert.Len(t, result, 2)
}

func TestStore_Filter_EmptyFilter(t *testing.T) {
	ctx := context.Background()
	store := NewStore()

	store.RegisterBatch(ctx, []*functionv1.FunctionMetadata{
		{
			Id:       "func1",
			Category: "test",
			Behavior: &functionv1.FunctionBehavior{Mode: functionv1.FunctionBehavior_MODE_QUERY},
			Security: &functionv1.FunctionSecurity{RiskLevel: functionv1.FunctionSecurity_RISK_LEVEL_LOW},
		},
	})

	filter := &functionv1.FunctionFilter{}
	result, err := store.Filter(ctx, filter)
	require.NoError(t, err)
	assert.Len(t, result, 1)
}

func TestCloneMetadata_Nil(t *testing.T) {
	result, err := cloneMetadata(nil)
	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestStore_RegisterBatch_InvalidMetadata(t *testing.T) {
	ctx := context.Background()
	store := NewStore()

	// Try to register batch with metadata missing required fields
	err := store.RegisterBatch(ctx, []*functionv1.FunctionMetadata{
		{
			Id:       "func1",
			Category: "test",
			Behavior: &functionv1.FunctionBehavior{Mode: functionv1.FunctionBehavior_MODE_QUERY},
			Security: &functionv1.FunctionSecurity{RiskLevel: functionv1.FunctionSecurity_RISK_LEVEL_LOW},
		},
		{
			Id: "func2", // Missing required fields (no Category, Behavior, Security)
		},
	})

	// This should succeed as the validation is done by the registry, not the store
	assert.NoError(t, err)
}

func TestNormalizeEnumName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"RISK_LEVEL_LOW", "low"},
		{"RISK_LEVEL_HIGH", "high"},
		{"MODE_QUERY", "query"},
		{"MODE_COMMAND", "command"},
		{"", ""},
		{"LOW", "low"},
		{"HIGH", "high"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := normalizeEnumName(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
