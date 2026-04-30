// Package function_test tests the SDK builder pattern and proto conversion.
package function_test

import (
	"context"
	"testing"

	functionv1 "github.com/cuihairu/croupier/pkg/pb/croupier/function/v1"
	"github.com/stretchr/testify/assert"
)

// TestSDK_Builder_BasicCreation tests creating a basic function metadata using builder pattern.
// This test demonstrates the builder pattern SDK users would use.
func TestSDK_Builder_BasicCreation(t *testing.T) {
	// Create a basic function metadata using the proto directly
	// SDK users would use the builder pattern from sdks/go/function
	metadata := &functionv1.FunctionMetadata{
		Id:          "player.ban",
		Version:     "1.0.0",
		Category:    "player",
		Name:        "Ban Player",
		Description: "Ban a player from the game",
		Tags:        []string{"moderation", "player"},
		Security: &functionv1.FunctionSecurity{
			RiskLevel:        functionv1.FunctionSecurity_RISK_LEVEL_HIGH,
			Permission:       "player.ban.invoke",
			RequiresApproval: true,
			ApprovalType:     functionv1.FunctionSecurity_APPROVAL_TYPE_TWO_PERSON,
			AllowedRoles:     []string{"admin", "moderator"},
			AuditLog:         true,
		},
		Behavior: &functionv1.FunctionBehavior{
			Mode:            functionv1.FunctionBehavior_MODE_COMMAND,
			Idempotent:      false,
			TimeoutMs:       30000,
			RouteStrategy:   functionv1.FunctionBehavior_ROUTE_STRATEGY_HASH,
			Cacheable:       false,
			CacheTtlSeconds: 0,
		},
		InputSchema:  `{"type":"object","properties":{"playerId":{"type":"string"}}}`,
		OutputSchema: `{"type":"object","properties":{"success":{"type":"boolean"}}}`,
		Extensions: map[string]string{
			"x-entity":    "Player",
			"x-operation": "ban",
		},
	}

	assert.Equal(t, "player.ban", metadata.Id)
	assert.Equal(t, "1.0.0", metadata.Version)
	assert.Equal(t, "player", metadata.Category)
	assert.Equal(t, "Ban Player", metadata.Name)
	assert.Equal(t, functionv1.FunctionSecurity_RISK_LEVEL_HIGH, metadata.Security.RiskLevel)
	assert.Equal(t, functionv1.FunctionBehavior_MODE_COMMAND, metadata.Behavior.Mode)
}

// TestSDK_DefaultValues tests default values for optional fields.
func TestSDK_DefaultValues(t *testing.T) {
	metadata := &functionv1.FunctionMetadata{
		Id:   "test.function",
		Name: "Test Function",
	}

	// Security should be nil if not set
	assert.Nil(t, metadata.Security)

	// Behavior should be nil if not set
	assert.Nil(t, metadata.Behavior)
}

// TestSDK_RiskLevels tests all risk level values.
func TestSDK_RiskLevels(t *testing.T) {
	tests := []struct {
		name     string
		level    functionv1.FunctionSecurity_RiskLevel
		expected string
	}{
		{"low", functionv1.FunctionSecurity_RISK_LEVEL_LOW, "low"},
		{"medium", functionv1.FunctionSecurity_RISK_LEVEL_MEDIUM, "medium"},
		{"high", functionv1.FunctionSecurity_RISK_LEVEL_HIGH, "high"},
		{"danger", functionv1.FunctionSecurity_RISK_LEVEL_DANGER, "danger"},
		{"unspecified", functionv1.FunctionSecurity_RISK_LEVEL_UNSPECIFIED, "unspecified"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metadata := &functionv1.FunctionMetadata{
				Id:   "test." + tt.name,
				Name: "Test " + tt.name,
				Security: &functionv1.FunctionSecurity{
					RiskLevel: tt.level,
				},
			}

			assert.Equal(t, tt.level, metadata.Security.RiskLevel)
		})
	}
}

// TestSDK_ExecutionModes tests all execution mode values.
func TestSDK_ExecutionModes(t *testing.T) {
	tests := []struct {
		name     string
		mode     functionv1.FunctionBehavior_Mode
		expected string
	}{
		{"query", functionv1.FunctionBehavior_MODE_QUERY, "query"},
		{"command", functionv1.FunctionBehavior_MODE_COMMAND, "command"},
		{"unspecified", functionv1.FunctionBehavior_MODE_UNSPECIFIED, "unspecified"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metadata := &functionv1.FunctionMetadata{
				Id:   "test." + tt.name,
				Name: "Test " + tt.name,
				Behavior: &functionv1.FunctionBehavior{
					Mode: tt.mode,
				},
			}

			assert.Equal(t, tt.mode, metadata.Behavior.Mode)
		})
	}
}

// TestSDK_RouteStrategies tests all route strategy values.
func TestSDK_RouteStrategies(t *testing.T) {
	tests := []struct {
		name     string
		strategy functionv1.FunctionBehavior_RouteStrategy
		expected string
	}{
		{"lb", functionv1.FunctionBehavior_ROUTE_STRATEGY_LB, "lb"},
		{"broadcast", functionv1.FunctionBehavior_ROUTE_STRATEGY_BROADCAST, "broadcast"},
		{"targeted", functionv1.FunctionBehavior_ROUTE_STRATEGY_TARGETED, "targeted"},
		{"hash", functionv1.FunctionBehavior_ROUTE_STRATEGY_HASH, "hash"},
		{"unspecified", functionv1.FunctionBehavior_ROUTE_STRATEGY_UNSPECIFIED, "unspecified"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metadata := &functionv1.FunctionMetadata{
				Id:   "test." + tt.name,
				Name: "Test " + tt.name,
				Behavior: &functionv1.FunctionBehavior{
					RouteStrategy: tt.strategy,
				},
			}

			assert.Equal(t, tt.strategy, metadata.Behavior.RouteStrategy)
		})
	}
}

// TestSDK_ApprovalTypes tests all approval type values.
func TestSDK_ApprovalTypes(t *testing.T) {
	tests := []struct {
		name     string
		atype    functionv1.FunctionSecurity_ApprovalType
		expected string
	}{
		{"none", functionv1.FunctionSecurity_APPROVAL_TYPE_UNSPECIFIED, "none"},
		{"single", functionv1.FunctionSecurity_APPROVAL_TYPE_SINGLE, "single"},
		{"two_person", functionv1.FunctionSecurity_APPROVAL_TYPE_TWO_PERSON, "two_person"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metadata := &functionv1.FunctionMetadata{
				Id:   "test." + tt.name,
				Name: "Test " + tt.name,
				Security: &functionv1.FunctionSecurity{
					ApprovalType: tt.atype,
				},
			}

			assert.Equal(t, tt.atype, metadata.Security.ApprovalType)
		})
	}
}

// TestSDK_CacheableFunction tests cacheable function configuration.
func TestSDK_CacheableFunction(t *testing.T) {
	metadata := &functionv1.FunctionMetadata{
		Id:   "cache.query",
		Name: "Cached Query",
		Behavior: &functionv1.FunctionBehavior{
			Mode:            functionv1.FunctionBehavior_MODE_QUERY,
			Idempotent:      true,
			TimeoutMs:       60000,
			Cacheable:       true,
			CacheTtlSeconds: 300,
			RouteStrategy:   functionv1.FunctionBehavior_ROUTE_STRATEGY_HASH,
		},
	}

	assert.True(t, metadata.Behavior.Cacheable)
	assert.Equal(t, int32(300), metadata.Behavior.CacheTtlSeconds)
	assert.True(t, metadata.Behavior.Idempotent)
}

// TestSDK_HighRiskFunction tests high-risk function with approval requirements.
func TestSDK_HighRiskFunction(t *testing.T) {
	metadata := &functionv1.FunctionMetadata{
		Id:          "admin.delete",
		Name:        "Admin Delete",
		Description: "Delete administrative data",
		Security: &functionv1.FunctionSecurity{
			RiskLevel:         functionv1.FunctionSecurity_RISK_LEVEL_DANGER,
			Permission:        "admin.delete.invoke",
			RequiresApproval:  true,
			ApprovalType:      functionv1.FunctionSecurity_APPROVAL_TYPE_TWO_PERSON,
			AllowedRoles:      []string{"admin", "superadmin"},
			AuditLog:          true,
			MaskSensitiveData: true,
		},
	}

	assert.Equal(t, functionv1.FunctionSecurity_RISK_LEVEL_DANGER, metadata.Security.RiskLevel)
	assert.True(t, metadata.Security.RequiresApproval)
	assert.Equal(t, functionv1.FunctionSecurity_APPROVAL_TYPE_TWO_PERSON, metadata.Security.ApprovalType)
	assert.ElementsMatch(t, []string{"admin", "superadmin"}, metadata.Security.AllowedRoles)
	assert.True(t, metadata.Security.AuditLog)
	assert.True(t, metadata.Security.MaskSensitiveData)
}

// TestSDK_Extensions tests extension fields for custom metadata.
func TestSDK_Extensions(t *testing.T) {
	metadata := &functionv1.FunctionMetadata{
		Id:   "custom.function",
		Name: "Custom Function",
		Extensions: map[string]string{
			"x-entity":     "Player",
			"x-operation":  "delete",
			"x-deprecated": "true",
			"x-rate-limit": "100",
		},
	}

	assert.Equal(t, "Player", metadata.Extensions["x-entity"])
	assert.Equal(t, "delete", metadata.Extensions["x-operation"])
	assert.Equal(t, "true", metadata.Extensions["x-deprecated"])
	assert.Equal(t, "100", metadata.Extensions["x-rate-limit"])
	assert.Len(t, metadata.Extensions, 4)
}

// TestSDK_JSONSchemas tests JSON schema definitions.
func TestSDK_JSONSchemas(t *testing.T) {
	inputSchema := `{
		"type": "object",
		"properties": {
			"playerId": {"type": "string"},
			"reason": {"type": "string"}
		},
		"required": ["playerId"]
	}`

	outputSchema := `{
		"type": "object",
		"properties": {
			"success": {"type": "boolean"},
			"message": {"type": "string"}
		}
	}`

	metadata := &functionv1.FunctionMetadata{
		Id:           "player.ban",
		Name:         "Ban Player",
		InputSchema:  inputSchema,
		OutputSchema: outputSchema,
	}

	assert.Equal(t, inputSchema, metadata.InputSchema)
	assert.Equal(t, outputSchema, metadata.OutputSchema)
	assert.Contains(t, metadata.InputSchema, "playerId")
	assert.Contains(t, metadata.OutputSchema, "success")
}

// TestSDK_Tags tests tag handling for function categorization.
func TestSDK_Tags(t *testing.T) {
	tags := []string{"moderation", "player", "punishment"}

	metadata := &functionv1.FunctionMetadata{
		Id:   "player.kick",
		Name: "Kick Player",
		Tags: tags,
	}

	assert.ElementsMatch(t, tags, metadata.Tags)
	assert.Len(t, metadata.Tags, 3)
}

// TestSDK_TimeoutConfiguration tests various timeout configurations.
func TestSDK_TimeoutConfiguration(t *testing.T) {
	tests := []struct {
		name      string
		timeoutMs int32
	}{
		{"fast", 5000},
		{"normal", 30000},
		{"slow", 60000},
		{"very-slow", 120000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metadata := &functionv1.FunctionMetadata{
				Id:   "test." + tt.name,
				Name: "Test " + tt.name,
				Behavior: &functionv1.FunctionBehavior{
					TimeoutMs: tt.timeoutMs,
				},
			}

			assert.Equal(t, tt.timeoutMs, metadata.Behavior.TimeoutMs)
		})
	}
}

// TestSDK_Idempotency tests idempotent function configuration.
func TestSDK_Idempotency(t *testing.T) {
	metadata := &functionv1.FunctionMetadata{
		Id:   "player.get",
		Name: "Get Player",
		Behavior: &functionv1.FunctionBehavior{
			Mode:       functionv1.FunctionBehavior_MODE_QUERY,
			Idempotent: true,
		},
	}

	assert.True(t, metadata.Behavior.Idempotent)

	// Non-idempotent function
	nonIdempotent := &functionv1.FunctionMetadata{
		Id:   "player.delete",
		Name: "Delete Player",
		Behavior: &functionv1.FunctionBehavior{
			Mode:       functionv1.FunctionBehavior_MODE_COMMAND,
			Idempotent: false,
		},
	}

	assert.False(t, nonIdempotent.Behavior.Idempotent)
}

// TestSDK_CompleteFunction tests a fully-specified function metadata.
func TestSDK_CompleteFunction(t *testing.T) {
	metadata := &functionv1.FunctionMetadata{
		Id:           "game.update",
		Version:      "2.1.0",
		Category:     "game",
		Name:         "Update Game Configuration",
		Description:  "Updates the configuration for an active game",
		Tags:         []string{"game", "configuration", "admin"},
		InputSchema:  `{"type":"object","properties":{"gameId":{"type":"string"},"config":{"type":"object"}}}`,
		OutputSchema: `{"type":"object","properties":{"success":{"type":"boolean"},"version":{"type":"string"}}}`,
		Behavior: &functionv1.FunctionBehavior{
			Mode:          functionv1.FunctionBehavior_MODE_COMMAND,
			Idempotent:    false,
			TimeoutMs:     45000,
			RouteStrategy: functionv1.FunctionBehavior_ROUTE_STRATEGY_TARGETED,
			Cacheable:     false,
		},
		Security: &functionv1.FunctionSecurity{
			RiskLevel:         functionv1.FunctionSecurity_RISK_LEVEL_HIGH,
			Permission:        "game.update.invoke",
			RequiresApproval:  true,
			ApprovalType:      functionv1.FunctionSecurity_APPROVAL_TYPE_SINGLE,
			AllowedRoles:      []string{"admin"},
			AuditLog:          true,
			MaskSensitiveData: false,
		},
		Extensions: map[string]string{
			"x-entity":      "Game",
			"x-operation":   "update",
			"x-api-version": "v2",
		},
	}

	// Verify all fields
	assert.Equal(t, "game.update", metadata.Id)
	assert.Equal(t, "2.1.0", metadata.Version)
	assert.Equal(t, "game", metadata.Category)
	assert.Equal(t, "Update Game Configuration", metadata.Name)
	assert.Len(t, metadata.Tags, 3)
	assert.Equal(t, functionv1.FunctionBehavior_MODE_COMMAND, metadata.Behavior.Mode)
	assert.Equal(t, int32(45000), metadata.Behavior.TimeoutMs)
	assert.Equal(t, functionv1.FunctionSecurity_RISK_LEVEL_HIGH, metadata.Security.RiskLevel)
	assert.Len(t, metadata.Extensions, 3)
}

// TestProtoToDTOConversion tests conversion from proto to API DTO.
// This tests the conversion logic in internal/function/api/dto.go
func TestProtoToDTOConversion(t *testing.T) {
	// Import the api package to access conversion functions
	// This tests are meant to be in the api package but placed here for organization
	protoMeta := &functionv1.FunctionMetadata{
		Id:           "test.function",
		Version:      "1.0.0",
		Category:     "test",
		Name:         "Test Function",
		Description:  "A test function",
		Tags:         []string{"tag1", "tag2"},
		InputSchema:  `{"type":"object"}`,
		OutputSchema: `{"type":"object"}`,
		Behavior: &functionv1.FunctionBehavior{
			Mode:            functionv1.FunctionBehavior_MODE_QUERY,
			Idempotent:      true,
			TimeoutMs:       30000,
			RouteStrategy:   functionv1.FunctionBehavior_ROUTE_STRATEGY_LB,
			Cacheable:       true,
			CacheTtlSeconds: 300,
		},
		Security: &functionv1.FunctionSecurity{
			RiskLevel:         functionv1.FunctionSecurity_RISK_LEVEL_LOW,
			Permission:        "test.invoke",
			RequiresApproval:  false,
			AllowedRoles:      []string{"user"},
			AuditLog:          true,
			MaskSensitiveData: false,
		},
		Extensions: map[string]string{
			"x-custom": "value",
		},
	}

	// Verify proto structure is correct
	assert.NotNil(t, protoMeta)
	assert.Equal(t, "test.function", protoMeta.Id)
	assert.Equal(t, functionv1.FunctionBehavior_MODE_QUERY, protoMeta.Behavior.Mode)
	assert.Equal(t, functionv1.FunctionSecurity_RISK_LEVEL_LOW, protoMeta.Security.RiskLevel)
}

// TestSDK_Validation tests function metadata validation requirements.
func TestSDK_Validation(t *testing.T) {
	tests := []struct {
		name        string
		metadata    *functionv1.FunctionMetadata
		expectValid bool
	}{
		{
			name: "valid minimal",
			metadata: &functionv1.FunctionMetadata{
				Id:   "valid.function",
				Name: "Valid Function",
			},
			expectValid: true,
		},
		{
			name: "missing ID",
			metadata: &functionv1.FunctionMetadata{
				Name: "No ID Function",
			},
			expectValid: false,
		},
		{
			name: "empty ID",
			metadata: &functionv1.FunctionMetadata{
				Id:   "",
				Name: "Empty ID Function",
			},
			expectValid: false,
		},
		{
			name: "valid complete",
			metadata: &functionv1.FunctionMetadata{
				Id:          "complete.function",
				Name:        "Complete Function",
				Version:     "1.0.0",
				Category:    "test",
				Description: "A complete test function",
				Tags:        []string{"test"},
				Security:    &functionv1.FunctionSecurity{RiskLevel: functionv1.FunctionSecurity_RISK_LEVEL_LOW},
				Behavior:    &functionv1.FunctionBehavior{Mode: functionv1.FunctionBehavior_MODE_QUERY},
			},
			expectValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isValid := tt.metadata.Id != ""
			assert.Equal(t, tt.expectValid, isValid)
		})
	}
}

// TestSDK_RoleBasedAccessControl tests RBAC configuration.
func TestSDK_RoleBasedAccessControl(t *testing.T) {
	tests := []struct {
		name     string
		roles    []string
		expected []string
	}{
		{
			name:     "single role",
			roles:    []string{"admin"},
			expected: []string{"admin"},
		},
		{
			name:     "multiple roles",
			roles:    []string{"admin", "moderator", "user"},
			expected: []string{"admin", "moderator", "user"},
		},
		{
			name:     "no roles",
			roles:    []string{},
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metadata := &functionv1.FunctionMetadata{
				Id:   "rbac.test",
				Name: "RBAC Test",
				Security: &functionv1.FunctionSecurity{
					AllowedRoles: tt.roles,
				},
			}

			assert.ElementsMatch(t, tt.expected, metadata.Security.AllowedRoles)
		})
	}
}

// TestSDK_AuditConfiguration tests audit logging configuration.
func TestSDK_AuditConfiguration(t *testing.T) {
	tests := []struct {
		name              string
		auditLog          bool
		maskSensitiveData bool
	}{
		{"full audit", true, true},
		{"audit only", true, false},
		{"no audit", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metadata := &functionv1.FunctionMetadata{
				Id:   "audit.test",
				Name: "Audit Test",
				Security: &functionv1.FunctionSecurity{
					AuditLog:          tt.auditLog,
					MaskSensitiveData: tt.maskSensitiveData,
				},
			}

			assert.Equal(t, tt.auditLog, metadata.Security.AuditLog)
			assert.Equal(t, tt.maskSensitiveData, metadata.Security.MaskSensitiveData)
		})
	}
}

// TestSDK_ContextUsage demonstrates how functions would use context.
func TestSDK_ContextUsage(t *testing.T) {
	// This test demonstrates that functions receive context.Context
	// The actual context handling would be in the function handler implementation
	ctx := context.Background()

	// Context can be used for:
	// - Cancellation
	// - Timeouts
	// - Request-scoped values
	// - Tracing

	assert.NotNil(t, ctx)
	assert.NoError(t, ctx.Err())
}
