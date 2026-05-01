// Package function tests the builder pattern for function metadata.
package function

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestMetadataBuilder_Basic tests basic builder functionality.
func TestMetadataBuilder_Basic(t *testing.T) {
	metadata, err := NewMetadataBuilder().
		SetID("player.ban").
		SetVersion("1.0.0").
		SetCategory("player").
		SetTags("moderation", "player").
		SetName("Ban Player").
		SetDescription("Ban a player from the game").
		Build()

	assert.NoError(t, err)
	assert.NotNil(t, metadata)
	assert.Equal(t, "player.ban", metadata.ID)
	assert.Equal(t, "1.0.0", metadata.Version)
	assert.Equal(t, "player", metadata.Category)
	assert.Equal(t, "Ban Player", metadata.Name)
	assert.Equal(t, "Ban a player from the game", metadata.Description)
	assert.ElementsMatch(t, []string{"moderation", "player"}, metadata.Tags)
}

// TestMetadataBuilder_WithDefaults tests builder with default values.
// Note: Builder uses Go zero values, not semantic defaults.
// Use RiskBuilder and BehaviorBuilder for semantic defaults.
func TestMetadataBuilder_WithDefaults(t *testing.T) {
	metadata, err := NewMetadataBuilder().
		SetID("test.function").
		SetName("Test Function").
		Build()

	assert.NoError(t, err)
	assert.NotNil(t, metadata)
	assert.NotNil(t, metadata.Risk)
	assert.NotNil(t, metadata.Behavior)
	// Default values from NewMetadataBuilder
	assert.Equal(t, RiskMedium, metadata.Risk.Level)
	assert.Equal(t, ModeUnknown, metadata.Behavior.Mode)
}

// TestMetadataBuilder_WithBehavior tests setting behavior.
func TestMetadataBuilder_WithBehavior(t *testing.T) {
	behavior := NewBehaviorBuilder().
		SetCommand().
		SetIdempotent(false).
		SetTimeoutMs(60000).
		SetRouteStrategy(RouteHash).
		SetCacheable(300).
		Build()

	metadata, err := NewMetadataBuilder().
		SetID("cache.query").
		SetName("Cached Query").
		SetBehavior(behavior).
		Build()

	assert.NoError(t, err)
	assert.Equal(t, ModeCommand, metadata.Behavior.Mode)
	assert.False(t, metadata.Behavior.Idempotent)
	assert.Equal(t, int32(60000), metadata.Behavior.TimeoutMs)
	assert.Equal(t, RouteHash, metadata.Behavior.RouteStrategy)
	assert.True(t, metadata.Behavior.Cacheable)
	assert.Equal(t, int32(300), metadata.Behavior.CacheTtlSeconds)
}

// TestMetadataBuilder_WithRisk tests setting risk.
func TestMetadataBuilder_WithRisk(t *testing.T) {
	risk := NewRiskBuilder().
		SetDanger().
		Build()

	metadata, err := NewMetadataBuilder().
		SetID("admin.delete").
		SetName("Admin Delete").
		SetRisk(risk).
		Build()

	assert.NoError(t, err)
	assert.Equal(t, RiskDanger, metadata.Risk.Level)
}

// TestMetadataBuilder_WithSchemas tests setting JSON schemas.
func TestMetadataBuilder_WithSchemas(t *testing.T) {
	inputSchema := `{"type":"object","properties":{"playerId":{"type":"string"}}}`
	outputSchema := `{"type":"object","properties":{"success":{"type":"boolean"}}}`

	metadata, err := NewMetadataBuilder().
		SetID("player.get").
		SetName("Get Player").
		SetInputSchema(inputSchema).
		SetOutputSchema(outputSchema).
		Build()

	assert.NoError(t, err)
	assert.Equal(t, inputSchema, metadata.InputSchema)
	assert.Equal(t, outputSchema, metadata.OutputSchema)
}

// TestMetadataBuilder_WithExtensions tests setting extension fields.
func TestMetadataBuilder_WithExtensions(t *testing.T) {
	metadata, err := NewMetadataBuilder().
		SetID("custom.function").
		SetName("Custom Function").
		SetExtension("x-entity", "Player").
		SetExtension("x-operation", "delete").
		SetExtension("x-deprecated", "true").
		Build()

	assert.NoError(t, err)
	assert.Equal(t, "Player", metadata.Extensions["x-entity"])
	assert.Equal(t, "delete", metadata.Extensions["x-operation"])
	assert.Equal(t, "true", metadata.Extensions["x-deprecated"])
}

// TestMetadataBuilder_Chaining tests method chaining.
func TestMetadataBuilder_Chaining(t *testing.T) {
	metadata, err := NewMetadataBuilder().
		SetID("test.chain").
		SetName("Chain Test").
		SetCategory("test").
		SetTags("tag1", "tag2").
		SetDescription("Testing method chaining").
		Build()

	assert.NoError(t, err)
	assert.Equal(t, "test.chain", metadata.ID)
	assert.Equal(t, "Chain Test", metadata.Name)
	assert.Equal(t, "test", metadata.Category)
	assert.Len(t, metadata.Tags, 2)
}

// TestMetadataBuilder_AddTag tests adding individual tags.
func TestMetadataBuilder_AddTag(t *testing.T) {
	metadata, err := NewMetadataBuilder().
		SetID("test.tags").
		SetName("Tags Test").
		AddTag("tag1").
		AddTag("tag2").
		AddTag("tag3").
		Build()

	assert.NoError(t, err)
	assert.ElementsMatch(t, []string{"tag1", "tag2", "tag3"}, metadata.Tags)
}

// TestMetadataBuilder_Validation tests builder validation.
func TestMetadataBuilder_Validation(t *testing.T) {
	tests := []struct {
		name      string
		builder   *MetadataBuilder
		expectErr bool
	}{
		{
			name:      "valid minimal",
			builder:   NewMetadataBuilder().SetID("test.id").SetName("Test"),
			expectErr: false,
		},
		{
			name:      "missing ID",
			builder:   NewMetadataBuilder().SetName("Test"),
			expectErr: true,
		},
		{
			name:      "empty ID",
			builder:   NewMetadataBuilder().SetID("").SetName("Test"),
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metadata, err := tt.builder.Build()
			if tt.expectErr {
				assert.Error(t, err)
				assert.Nil(t, metadata)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, metadata)
			}
		})
	}
}

// TestMetadataBuilder_MustBuild tests MustBuild panic behavior.
func TestMetadataBuilder_MustBuild(t *testing.T) {
	t.Run("valid build", func(t *testing.T) {
		metadata := NewMetadataBuilder().
			SetID("test.id").
			SetName("Test").
			MustBuild()

		assert.NotNil(t, metadata)
		assert.Equal(t, "test.id", metadata.ID)
	})

	t.Run("invalid build panics", func(t *testing.T) {
		assert.Panics(t, func() {
			NewMetadataBuilder().SetName("Test").MustBuild()
		})
	})
}

// TestBehaviorBuilder_Defaults tests default behavior values.
func TestBehaviorBuilder_Defaults(t *testing.T) {
	behavior := NewBehaviorBuilder().Build()

	assert.Equal(t, ModeQuery, behavior.Mode)
	assert.False(t, behavior.Idempotent)
	assert.Equal(t, int32(30000), behavior.TimeoutMs)
	assert.Equal(t, RouteLB, behavior.RouteStrategy)
	assert.False(t, behavior.Cacheable)
}

// TestBehaviorBuilder_SetMode tests setting execution modes.
func TestBehaviorBuilder_SetMode(t *testing.T) {
	tests := []struct {
		name     string
		mode     Mode
		expected Mode
	}{
		{"query", ModeQuery, ModeQuery},
		{"command", ModeCommand, ModeCommand},
		{"unknown", ModeUnknown, ModeUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			behavior := NewBehaviorBuilder().
				SetMode(tt.mode).
				Build()

			assert.Equal(t, tt.expected, behavior.Mode)
		})
	}
}

// TestBehaviorBuilder_SetQuery tests SetQuery shortcut.
func TestBehaviorBuilder_SetQuery(t *testing.T) {
	behavior := NewBehaviorBuilder().
		SetQuery().
		Build()

	assert.Equal(t, ModeQuery, behavior.Mode)
}

// TestBehaviorBuilder_SetCommand tests SetCommand shortcut.
func TestBehaviorBuilder_SetCommand(t *testing.T) {
	behavior := NewBehaviorBuilder().
		SetCommand().
		Build()

	assert.Equal(t, ModeCommand, behavior.Mode)
}

// TestBehaviorBuilder_SetRouteStrategy tests setting route strategies.
func TestBehaviorBuilder_SetRouteStrategy(t *testing.T) {
	tests := []struct {
		name     string
		strategy RouteStrategy
	}{
		{"lb", RouteLB},
		{"broadcast", RouteBroadcast},
		{"targeted", RouteTargeted},
		{"hash", RouteHash},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			behavior := NewBehaviorBuilder().
				SetRouteStrategy(tt.strategy).
				Build()

			assert.Equal(t, tt.strategy, behavior.RouteStrategy)
		})
	}
}

// TestBehaviorBuilder_SetCacheable tests cacheable configuration.
func TestBehaviorBuilder_SetCacheable(t *testing.T) {
	tests := []struct {
		name            string
		cacheTtlSeconds int32
	}{
		{"short", 60},
		{"medium", 300},
		{"long", 3600},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			behavior := NewBehaviorBuilder().
				SetCacheable(tt.cacheTtlSeconds).
				Build()

			assert.True(t, behavior.Cacheable)
			assert.Equal(t, tt.cacheTtlSeconds, behavior.CacheTtlSeconds)
		})
	}
}

// TestRiskBuilder_Defaults tests default risk values.
func TestRiskBuilder_Defaults(t *testing.T) {
	risk := NewRiskBuilder().Build()

	assert.Equal(t, RiskMedium, risk.Level)
}

// TestRiskBuilder_SetLevel tests setting risk levels.
func TestRiskBuilder_SetLevel(t *testing.T) {
	tests := []struct {
		name     string
		level    RiskLevel
		expected RiskLevel
	}{
		{"low", RiskLow, RiskLow},
		{"medium", RiskMedium, RiskMedium},
		{"high", RiskHigh, RiskHigh},
		{"danger", RiskDanger, RiskDanger},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			risk := NewRiskBuilder().
				SetLevel(tt.level).
				Build()

			assert.Equal(t, tt.expected, risk.Level)
		})
	}
}

// TestRiskBuilder_LevelShortcuts tests risk level shortcut methods.
func TestRiskBuilder_LevelShortcuts(t *testing.T) {
	tests := []struct {
		name     string
		builder  *RiskBuilder
		expected RiskLevel
	}{
		{"low", NewRiskBuilder().SetLow(), RiskLow},
		{"medium", NewRiskBuilder().SetMedium(), RiskMedium},
		{"high", NewRiskBuilder().SetHigh(), RiskHigh},
		{"danger", NewRiskBuilder().SetDanger(), RiskDanger},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			risk := tt.builder.Build()
			assert.Equal(t, tt.expected, risk.Level)
		})
	}
}

// TestMode_String tests Mode string representation.
func TestMode_String(t *testing.T) {
	tests := []struct {
		mode     Mode
		expected string
	}{
		{ModeUnknown, "unknown"},
		{ModeQuery, "query"},
		{ModeCommand, "command"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.mode.String())
		})
	}
}

// TestRouteStrategy_String tests RouteStrategy string representation.
func TestRouteStrategy_String(t *testing.T) {
	tests := []struct {
		strategy RouteStrategy
		expected string
	}{
		{RouteUnknown, "unknown"},
		{RouteLB, "lb"},
		{RouteBroadcast, "broadcast"},
		{RouteTargeted, "targeted"},
		{RouteHash, "hash"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.strategy.String())
		})
	}
}

// TestRiskLevel_String tests RiskLevel string representation.
func TestRiskLevel_String(t *testing.T) {
	tests := []struct {
		level    RiskLevel
		expected string
	}{
		{RiskUnknown, "unknown"},
		{RiskLow, "low"},
		{RiskMedium, "medium"},
		{RiskHigh, "high"},
		{RiskDanger, "danger"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.level.String())
		})
	}
}

// TestMetadataBuilder_CompleteExample tests a complete realistic example.
func TestMetadataBuilder_CompleteExample(t *testing.T) {
	behavior := NewBehaviorBuilder().
		SetCommand().
		SetIdempotent(false).
		SetTimeoutMs(45000).
		SetRouteStrategy(RouteTargeted).
		Build()

	risk := NewRiskBuilder().
		SetHigh().
		Build()

	metadata, err := NewMetadataBuilder().
		SetID("game.update").
		SetVersion("2.0.0").
		SetCategory("game").
		SetTags("game", "configuration", "admin").
		SetName("Update Game Configuration").
		SetDescription("Updates the configuration for an active game").
		SetInputSchema(`{"type":"object","properties":{"gameId":{"type":"string"},"config":{"type":"object"}}}`).
		SetOutputSchema(`{"type":"object","properties":{"success":{"type":"boolean"}}}`).
		SetBehavior(behavior).
		SetRisk(risk).
		SetExtension("x-entity", "Game").
		SetExtension("x-operation", "update").
		Build()

	assert.NoError(t, err)
	assert.NotNil(t, metadata)
	assert.Equal(t, "game.update", metadata.ID)
	assert.Equal(t, "2.0.0", metadata.Version)
	assert.Equal(t, ModeCommand, metadata.Behavior.Mode)
	assert.Equal(t, RiskHigh, metadata.Risk.Level)
	assert.Equal(t, "Game", metadata.Extensions["x-entity"])
}
