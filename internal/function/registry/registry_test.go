// Package registry tests the function registry.
package registry

import (
	"context"
	"testing"

	functionv1 "github.com/cuihairu/croupier/pkg/pb/croupier/function/v1"
	"github.com/stretchr/testify/assert"
)

func TestRegistry_New(t *testing.T) {
	reg := New()

	assert.NotNil(t, reg.store)
	assert.NotNil(t, reg)
	assert.NotNil(t, reg.GetStore())
}

func TestRegistry_NewWithStore(t *testing.T) {
	store := NewStore()
	reg := NewWithStore(store)

	assert.NotNil(t, reg)
	assert.Equal(t, store, reg.store)
}

func TestRegistry_Register(t *testing.T) {
	reg := New()

	metadata := &functionv1.FunctionMetadata{
		Id:       "player.get",
		Name:     "Get Player",
		Category: "player",
		Security: &functionv1.FunctionSecurity{
			RiskLevel: functionv1.FunctionSecurity_RISK_LEVEL_LOW,
		},
		Behavior: &functionv1.FunctionBehavior{
			Mode: functionv1.FunctionBehavior_MODE_QUERY,
		},
	}

	err := reg.Register(context.Background(), metadata)
	assert.Nil(t, err)

	// Verify it was registered
	retrieved, err := reg.Get(context.Background(), "player.get")
	assert.Nil(t, err)
	assert.Equal(t, "player.get", retrieved.Id)
}

func TestRegistry_Register_ValidationErrors(t *testing.T) {
	reg := New()

	tests := []struct {
		name      string
		metadata  *functionv1.FunctionMetadata
		expectErr bool
	}{
		{
			name:      "nil metadata",
			metadata:  nil,
			expectErr: true,
		},
		{
			name: "missing ID",
			metadata: &functionv1.FunctionMetadata{
				Name:     "Test",
				Security: &functionv1.FunctionSecurity{},
				Behavior: &functionv1.FunctionBehavior{},
			},
			expectErr: true,
		},
		{
			name: "empty ID",
			metadata: &functionv1.FunctionMetadata{
				Id:       "",
				Name:     "Test",
				Security: &functionv1.FunctionSecurity{},
				Behavior: &functionv1.FunctionBehavior{},
			},
			expectErr: true,
		},
		{
			name: "invalid ID format - single part",
			metadata: &functionv1.FunctionMetadata{
				Id:       "player",
				Name:     "Test",
				Security: &functionv1.FunctionSecurity{},
				Behavior: &functionv1.FunctionBehavior{},
			},
			expectErr: true,
		},
		{
			name: "missing security",
			metadata: &functionv1.FunctionMetadata{
				Id:       "player.get",
				Name:     "Test",
				Behavior: &functionv1.FunctionBehavior{},
			},
			expectErr: true,
		},
		{
			name: "missing behavior",
			metadata: &functionv1.FunctionMetadata{
				Id:       "player.get",
				Name:     "Test",
				Security: &functionv1.FunctionSecurity{},
			},
			expectErr: true,
		},
		{
			name: "valid - two part ID",
			metadata: &functionv1.FunctionMetadata{
				Id:       "player.get",
				Name:     "Get Player",
				Security: &functionv1.FunctionSecurity{},
				Behavior: &functionv1.FunctionBehavior{},
			},
			expectErr: false,
		},
		{
			name: "valid - three part ID",
			metadata: &functionv1.FunctionMetadata{
				Id:       "game.player.ban",
				Name:     "Ban Player",
				Security: &functionv1.FunctionSecurity{},
				Behavior: &functionv1.FunctionBehavior{},
			},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := reg.Register(context.Background(), tt.metadata)
			if tt.expectErr {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
			}
		})
	}
}

func TestRegistry_RegisterBatch(t *testing.T) {
	reg := New()

	// Test nil list
	err := reg.RegisterBatch(context.Background(), nil)
	assert.NotNil(t, err)

	// Test empty list
	err = reg.RegisterBatch(context.Background(), &functionv1.FunctionMetadataList{})
	assert.Nil(t, err)

	// Test valid batch
	list := &functionv1.FunctionMetadataList{
		Functions: []*functionv1.FunctionMetadata{
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
		},
	}

	err = reg.RegisterBatch(context.Background(), list)
	assert.Nil(t, err)

	// Verify both were registered
	assert.Equal(t, 2, reg.Count(context.Background()))
}

func TestRegistry_RegisterBatch_ValidationError(t *testing.T) {
	reg := New()

	list := &functionv1.FunctionMetadataList{
		Functions: []*functionv1.FunctionMetadata{
			{
				Id:       "player.get",
				Name:     "Get Player",
				Security: &functionv1.FunctionSecurity{},
				Behavior: &functionv1.FunctionBehavior{},
			},
			{
				// Invalid: missing security
				Id:   "player.update",
				Name: "Update Player",
			},
		},
	}

	err := reg.RegisterBatch(context.Background(), list)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "player.update")
}

func TestRegistry_List(t *testing.T) {
	reg := New()

	// Register some functions
	reg.Register(context.Background(), &functionv1.FunctionMetadata{
		Id:       "player.get",
		Category: "player",
		Security: &functionv1.FunctionSecurity{},
		Behavior: &functionv1.FunctionBehavior{},
	})
	reg.Register(context.Background(), &functionv1.FunctionMetadata{
		Id:       "game.create",
		Category: "game",
		Security: &functionv1.FunctionSecurity{},
		Behavior: &functionv1.FunctionBehavior{},
	})

	functions, err := reg.List(context.Background())
	assert.Nil(t, err)
	assert.Equal(t, 2, len(functions))
}

func TestRegistry_ListByCategory(t *testing.T) {
	reg := New()

	reg.Register(context.Background(), &functionv1.FunctionMetadata{
		Id:       "player.get",
		Category: "player",
		Security: &functionv1.FunctionSecurity{},
		Behavior: &functionv1.FunctionBehavior{},
	})
	reg.Register(context.Background(), &functionv1.FunctionMetadata{
		Id:       "game.create",
		Category: "game",
		Security: &functionv1.FunctionSecurity{},
		Behavior: &functionv1.FunctionBehavior{},
	})
	reg.Register(context.Background(), &functionv1.FunctionMetadata{
		Id:       "player.update",
		Category: "player",
		Security: &functionv1.FunctionSecurity{},
		Behavior: &functionv1.FunctionBehavior{},
	})

	playerFuncs, err := reg.ListByCategory(context.Background(), "player")
	assert.Nil(t, err)
	assert.Equal(t, 2, len(playerFuncs))

	gameFuncs, err := reg.ListByCategory(context.Background(), "game")
	assert.Nil(t, err)
	assert.Equal(t, 1, len(gameFuncs))
}

func TestRegistry_ListByTag(t *testing.T) {
	reg := New()

	reg.Register(context.Background(), &functionv1.FunctionMetadata{
		Id:       "player.get",
		Tags:     []string{"read", "player"},
		Security: &functionv1.FunctionSecurity{},
		Behavior: &functionv1.FunctionBehavior{},
	})
	reg.Register(context.Background(), &functionv1.FunctionMetadata{
		Id:       "player.update",
		Tags:     []string{"write", "player"},
		Security: &functionv1.FunctionSecurity{},
		Behavior: &functionv1.FunctionBehavior{},
	})
	reg.Register(context.Background(), &functionv1.FunctionMetadata{
		Id:       "game.create",
		Tags:     []string{"write", "game"},
		Security: &functionv1.FunctionSecurity{},
		Behavior: &functionv1.FunctionBehavior{},
	})

	readFuncs, err := reg.ListByTag(context.Background(), "read")
	assert.Nil(t, err)
	assert.Equal(t, 1, len(readFuncs))
	assert.Equal(t, "player.get", readFuncs[0].Id)

	writeFuncs, err := reg.ListByTag(context.Background(), "write")
	assert.Nil(t, err)
	assert.Equal(t, 2, len(writeFuncs))
}

func TestRegistry_ListByRiskLevel(t *testing.T) {
	reg := New()

	reg.Register(context.Background(), &functionv1.FunctionMetadata{
		Id:   "player.get",
		Name: "Get Player",
		Security: &functionv1.FunctionSecurity{
			RiskLevel: functionv1.FunctionSecurity_RISK_LEVEL_LOW,
		},
		Behavior: &functionv1.FunctionBehavior{},
	})
	reg.Register(context.Background(), &functionv1.FunctionMetadata{
		Id:   "player.delete",
		Name: "Delete Player",
		Security: &functionv1.FunctionSecurity{
			RiskLevel: functionv1.FunctionSecurity_RISK_LEVEL_HIGH,
		},
		Behavior: &functionv1.FunctionBehavior{},
	})

	lowRiskFuncs, err := reg.ListByRiskLevel(context.Background(), "RISK_LEVEL_LOW")
	assert.Nil(t, err)
	assert.Equal(t, 1, len(lowRiskFuncs))
	assert.Equal(t, "player.get", lowRiskFuncs[0].Id)

	highRiskFuncs, err := reg.ListByRiskLevel(context.Background(), "RISK_LEVEL_HIGH")
	assert.Nil(t, err)
	assert.Equal(t, 1, len(highRiskFuncs))
	assert.Equal(t, "player.delete", highRiskFuncs[0].Id)
}

func TestRegistry_ListByMode(t *testing.T) {
	reg := New()

	reg.Register(context.Background(), &functionv1.FunctionMetadata{
		Id:   "player.get",
		Name: "Get Player",
		Behavior: &functionv1.FunctionBehavior{
			Mode: functionv1.FunctionBehavior_MODE_QUERY,
		},
		Security: &functionv1.FunctionSecurity{},
	})
	reg.Register(context.Background(), &functionv1.FunctionMetadata{
		Id:   "player.update",
		Name: "Update Player",
		Behavior: &functionv1.FunctionBehavior{
			Mode: functionv1.FunctionBehavior_MODE_COMMAND,
		},
		Security: &functionv1.FunctionSecurity{},
	})

	queryFuncs, err := reg.ListByMode(context.Background(), functionv1.FunctionBehavior_MODE_QUERY)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(queryFuncs))
	assert.Equal(t, "player.get", queryFuncs[0].Id)

	cmdFuncs, err := reg.ListByMode(context.Background(), functionv1.FunctionBehavior_MODE_COMMAND)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(cmdFuncs))
	assert.Equal(t, "player.update", cmdFuncs[0].Id)
}

func TestRegistry_Filter(t *testing.T) {
	reg := New()

	reg.Register(context.Background(), &functionv1.FunctionMetadata{
		Id:       "player.get",
		Category: "player",
		Tags:     []string{"read"},
		Security: &functionv1.FunctionSecurity{RiskLevel: functionv1.FunctionSecurity_RISK_LEVEL_LOW},
		Behavior: &functionv1.FunctionBehavior{Mode: functionv1.FunctionBehavior_MODE_QUERY},
	})
	reg.Register(context.Background(), &functionv1.FunctionMetadata{
		Id:       "player.update",
		Category: "player",
		Tags:     []string{"write"},
		Security: &functionv1.FunctionSecurity{RiskLevel: functionv1.FunctionSecurity_RISK_LEVEL_HIGH},
		Behavior: &functionv1.FunctionBehavior{Mode: functionv1.FunctionBehavior_MODE_COMMAND},
	})

	t.Run("filter by category", func(t *testing.T) {
		filter := &functionv1.FunctionFilter{
			Category: "player",
		}
		results, _, err := reg.Filter(context.Background(), filter)
		assert.Nil(t, err)
		assert.Equal(t, 2, len(results))
	})

	t.Run("filter by tag", func(t *testing.T) {
		filter := &functionv1.FunctionFilter{
			Tags: []string{"read"},
		}
		results, _, err := reg.Filter(context.Background(), filter)
		assert.Nil(t, err)
		assert.Equal(t, 1, len(results))
		assert.Equal(t, "player.get", results[0].Id)
	})

	t.Run("filter by risk level", func(t *testing.T) {
		filter := &functionv1.FunctionFilter{
			RiskLevel: "low", // Normalized form used in index
		}
		results, _, err := reg.Filter(context.Background(), filter)
		assert.Nil(t, err)
		assert.Equal(t, 1, len(results))
	})

	t.Run("filter by mode", func(t *testing.T) {
		filter := &functionv1.FunctionFilter{
			Mode: "query", // Normalized form used in index
		}
		results, _, err := reg.Filter(context.Background(), filter)
		assert.Nil(t, err)
		assert.Equal(t, 1, len(results))
	})

	t.Run("filter with pagination", func(t *testing.T) {
		filter := &functionv1.FunctionFilter{
			PageSize: 1,
		}
		results, nextPageToken, err := reg.Filter(context.Background(), filter)
		assert.Nil(t, err)
		assert.Equal(t, 1, len(results))
		assert.NotEmpty(t, nextPageToken)
	})
}

func TestRegistry_Unregister(t *testing.T) {
	reg := New()

	reg.Register(context.Background(), &functionv1.FunctionMetadata{
		Id:       "player.get",
		Name:     "Get Player",
		Security: &functionv1.FunctionSecurity{},
		Behavior: &functionv1.FunctionBehavior{},
	})

	// Verify exists
	assert.True(t, reg.Exists(context.Background(), "player.get"))

	// Unregister
	err := reg.Unregister(context.Background(), "player.get")
	assert.Nil(t, err)

	// Verify removed
	assert.False(t, reg.Exists(context.Background(), "player.get"))
}

func TestRegistry_Exists(t *testing.T) {
	reg := New()

	assert.False(t, reg.Exists(context.Background(), "nonexistent"))

	reg.Register(context.Background(), &functionv1.FunctionMetadata{
		Id:       "player.get",
		Name:     "Get Player",
		Security: &functionv1.FunctionSecurity{},
		Behavior: &functionv1.FunctionBehavior{},
	})

	assert.True(t, reg.Exists(context.Background(), "player.get"))
}

func TestRegistry_Count(t *testing.T) {
	reg := New()

	assert.Equal(t, 0, reg.Count(context.Background()))

	reg.Register(context.Background(), &functionv1.FunctionMetadata{
		Id:       "player.get",
		Name:     "Get Player",
		Security: &functionv1.FunctionSecurity{},
		Behavior: &functionv1.FunctionBehavior{},
	})

	assert.Equal(t, 1, reg.Count(context.Background()))
}

func TestRegistry_GetCategories(t *testing.T) {
	reg := New()

	reg.Register(context.Background(), &functionv1.FunctionMetadata{
		Id:       "player.get",
		Category: "player",
		Security: &functionv1.FunctionSecurity{},
		Behavior: &functionv1.FunctionBehavior{},
	})
	reg.Register(context.Background(), &functionv1.FunctionMetadata{
		Id:       "game.create",
		Category: "game",
		Security: &functionv1.FunctionSecurity{},
		Behavior: &functionv1.FunctionBehavior{},
	})
	reg.Register(context.Background(), &functionv1.FunctionMetadata{
		Id:       "player.update",
		Category: "player",
		Security: &functionv1.FunctionSecurity{},
		Behavior: &functionv1.FunctionBehavior{},
	})

	categories := reg.GetCategories(context.Background())
	assert.Equal(t, 2, len(categories))
	assert.Contains(t, categories, "player")
	assert.Contains(t, categories, "game")
}

func TestRegistry_GetTags(t *testing.T) {
	reg := New()

	reg.Register(context.Background(), &functionv1.FunctionMetadata{
		Id:       "player.get",
		Tags:     []string{"read", "player"},
		Security: &functionv1.FunctionSecurity{},
		Behavior: &functionv1.FunctionBehavior{},
	})
	reg.Register(context.Background(), &functionv1.FunctionMetadata{
		Id:       "player.update",
		Tags:     []string{"write", "player"},
		Security: &functionv1.FunctionSecurity{},
		Behavior: &functionv1.FunctionBehavior{},
	})

	tags := reg.GetTags(context.Background())
	assert.Equal(t, 3, len(tags))
	assert.Contains(t, tags, "read")
	assert.Contains(t, tags, "write")
	assert.Contains(t, tags, "player")
}

func TestRegistry_VersionValidation(t *testing.T) {
	reg := New()

	tests := []struct {
		name      string
		version   string
		expectErr bool
	}{
		{"empty version", "", false},
		{"valid semver", "1.0.0", false},
		{"valid semver with prerelease", "1.0.0-beta", false}, // Only checks major.minor are numeric
		{"valid major.minor", "1.0", false},
		{"invalid - no numbers", "a.b.c", false}, // Only warns, doesn't error
		{"invalid - single part", "1", false},    // Only warns, doesn't error
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metadata := &functionv1.FunctionMetadata{
				Id:      "player.get",
				Version: tt.version,
				Name:    "Get Player",
				Security: &functionv1.FunctionSecurity{
					RiskLevel: functionv1.FunctionSecurity_RISK_LEVEL_LOW,
				},
				Behavior: &functionv1.FunctionBehavior{
					Mode: functionv1.FunctionBehavior_MODE_QUERY,
				},
			}

			err := reg.Register(context.Background(), metadata)
			if tt.expectErr {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
			}
		})
	}
}

func TestSplitID(t *testing.T) {
	tests := []struct {
		id    string
		parts int
	}{
		{"player.get", 2},
		{"game.player.ban", 3},
		{"a.b.c.d", 4},
		{"single", 1},
		{"", 0},
		{" spaced . out ", 2},
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			parts := splitID(tt.id)
			assert.Equal(t, tt.parts, len(parts))
		})
	}
}

func TestSplitAndTrim(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		sep      string
		expected []string
	}{
		{"basic", "a,b,c", ",", []string{"a", "b", "c"}},
		{"with spaces", " a , b , c ", ",", []string{"a", "b", "c"}},
		{"empty parts", "a,,c", ",", []string{"a", "c"}},
		{"empty input", "", ",", []string{}},
		{"tabs", "\ta\t,\tb\t", ",", []string{"a", "b"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := splitAndTrim(tt.input, tt.sep)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsValidSemVer(t *testing.T) {
	tests := []struct {
		version string
		valid   bool
	}{
		{"1.0.0", true},
		{"2.1.3", true},
		{"10.20.30", true},
		{"1.0", true},
		{"2.5", true},
		{"1", false},
		{"a.b.c", false},
		{"1.0.0-beta", true}, // Simplified validation: only checks first 2 parts are numeric
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			result := isValidSemVer(tt.version)
			assert.Equal(t, tt.valid, result)
		})
	}
}

func TestIsNumeric(t *testing.T) {
	tests := []struct {
		input string
		valid bool
	}{
		{"123", true},
		{"0", true},
		{"456", true},
		{"", false},
		{"abc", false},
		{"1a2", false},
		{"12.3", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := isNumeric(tt.input)
			assert.Equal(t, tt.valid, result)
		})
	}
}
