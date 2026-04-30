package migration

import (
	"testing"

	functionv1 "github.com/cuihairu/croupier/pkg/pb/croupier/function/v1"
)

func TestLocalFunctionDescriptorToMetadata(t *testing.T) {
	tests := []struct {
		name  string
		desc  LocalFunctionDescriptor
		check func(*testing.T, *functionv1.FunctionMetadata)
	}{
		{
			name: "basic descriptor",
			desc: LocalFunctionDescriptor{
				ID:           "player.ban",
				Version:      "1.0.0",
				Category:     "player",
				Tags:         []string{"moderation"},
				Summary:      "Ban Player",
				Description:  "Ban a player from the game",
				InputSchema:  `{"type":"object"}`,
				OutputSchema: `{"type":"object"}`,
				Risk:         "high",
				Entity:       "Player",
				Operation:    "delete",
			},
			check: func(t *testing.T, metadata *functionv1.FunctionMetadata) {
				if metadata.Id != "player.ban" {
					t.Errorf("ID = %v, want player.ban", metadata.Id)
				}
				if metadata.Name != "Ban Player" {
					t.Errorf("Name = %v, want Ban Player", metadata.Name)
				}
				if metadata.Security.RiskLevel != functionv1.FunctionSecurity_RISK_LEVEL_HIGH {
					t.Errorf("RiskLevel = %v, want RISK_HIGH", metadata.Security.RiskLevel)
				}
				if metadata.Extensions["x-entity"] != "Player" {
					t.Errorf("x-entity = %v, want Player", metadata.Extensions["x-entity"])
				}
			},
		},
		{
			name: "query operation",
			desc: LocalFunctionDescriptor{
				ID:       "player.get",
				Category: "player",
			},
			check: func(t *testing.T, metadata *functionv1.FunctionMetadata) {
				if metadata.Behavior.Mode != functionv1.FunctionBehavior_MODE_QUERY {
					t.Errorf("Mode = %v, want QUERY", metadata.Behavior.Mode)
				}
			},
		},
		{
			name: "empty descriptor",
			desc: LocalFunctionDescriptor{},
			check: func(t *testing.T, metadata *functionv1.FunctionMetadata) {
				if metadata != nil {
					t.Error("expected nil metadata for empty ID")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := LocalFunctionDescriptorToMetadata(tt.desc)
			if tt.check != nil {
				tt.check(t, got)
			}
		})
	}
}

func TestFunctionDescriptorToMetadata(t *testing.T) {
	desc := FunctionDescriptor{
		ID:        "player.get",
		Version:   "1.0.0",
		Category:  "player",
		Risk:      "low",
		Entity:    "Player",
		Operation: "read",
		Enabled:   true,
	}

	metadata := FunctionDescriptorToMetadata(desc)
	if metadata == nil {
		t.Fatal("expected non-nil metadata")
	}

	if metadata.Id != desc.ID {
		t.Errorf("ID = %v, want %v", metadata.Id, desc.ID)
	}

	if metadata.Behavior.Mode != functionv1.FunctionBehavior_MODE_QUERY {
		t.Errorf("Mode = %v, want QUERY", metadata.Behavior.Mode)
	}

	if metadata.Security.RiskLevel != functionv1.FunctionSecurity_RISK_LEVEL_LOW {
		t.Errorf("RiskLevel = %v, want RISK_LOW", metadata.Security.RiskLevel)
	}

	// Check that entity is in tags and extensions
	foundEntity := false
	for _, tag := range metadata.Tags {
		if tag == "Player" {
			foundEntity = true
			break
		}
	}
	if !foundEntity {
		t.Error("Entity 'Player' not found in tags")
	}

	if metadata.Extensions["x-entity"] != "Player" {
		t.Errorf("x-entity = %v, want Player", metadata.Extensions["x-entity"])
	}
}

func TestInferModeFromID(t *testing.T) {
	tests := []struct {
		id   string
		want functionv1.FunctionBehavior_Mode
	}{
		{"player.get", functionv1.FunctionBehavior_MODE_QUERY},
		{"player.list", functionv1.FunctionBehavior_MODE_QUERY},
		{"player.find", functionv1.FunctionBehavior_MODE_QUERY},
		{"player.query", functionv1.FunctionBehavior_MODE_QUERY},
		{"player.create", functionv1.FunctionBehavior_MODE_COMMAND},
		{"player.update", functionv1.FunctionBehavior_MODE_COMMAND},
		{"player.delete", functionv1.FunctionBehavior_MODE_COMMAND},
		{"player.send", functionv1.FunctionBehavior_MODE_COMMAND},
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			got := inferModeFromID(tt.id)
			if got != tt.want {
				t.Errorf("inferModeFromID(%q) = %v, want %v", tt.id, got, tt.want)
			}
		})
	}
}

func TestInferIdempotency(t *testing.T) {
	tests := []struct {
		id   string
		want bool
	}{
		{"player.set", true},
		{"player.update", true},
		{"player.delete", true},
		{"player.ban", true},
		{"player.create", false},
		{"player.add", false},
		{"player.send", false},
		{"player.grant", false},
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			got := inferIdempotency(tt.id)
			if got != tt.want {
				t.Errorf("inferIdempotency(%q) = %v, want %v", tt.id, got, tt.want)
			}
		})
	}
}

func TestInferRiskLevel(t *testing.T) {
	tests := []struct {
		risk string
		want functionv1.FunctionSecurity_RiskLevel
	}{
		{"low", functionv1.FunctionSecurity_RISK_LEVEL_LOW},
		{"safe", functionv1.FunctionSecurity_RISK_LEVEL_LOW},
		{"medium", functionv1.FunctionSecurity_RISK_LEVEL_MEDIUM},
		{"warning", functionv1.FunctionSecurity_RISK_LEVEL_MEDIUM},
		{"high", functionv1.FunctionSecurity_RISK_LEVEL_HIGH},
		{"error", functionv1.FunctionSecurity_RISK_LEVEL_HIGH},
		{"critical", functionv1.FunctionSecurity_RISK_LEVEL_DANGER},
		{"fatal", functionv1.FunctionSecurity_RISK_LEVEL_DANGER},
		{"unknown", functionv1.FunctionSecurity_RISK_LEVEL_MEDIUM},
	}

	for _, tt := range tests {
		t.Run(tt.risk, func(t *testing.T) {
			got := inferRiskLevel(tt.risk)
			if got != tt.want {
				t.Errorf("inferRiskLevel(%q) = %v, want %v", tt.risk, got, tt.want)
			}
		})
	}
}

func TestInferRequiresApproval(t *testing.T) {
	tests := []struct {
		risk string
		want bool
	}{
		{"low", false},
		{"medium", false},
		{"high", true},
		{"danger", true},
		{"critical", true},
		{"fatal", true},
		{"unknown", false},
	}

	for _, tt := range tests {
		t.Run(tt.risk, func(t *testing.T) {
			got := inferRequiresApproval(tt.risk)
			if got != tt.want {
				t.Errorf("inferRequiresApproval(%q) = %v, want %v", tt.risk, got, tt.want)
			}
		})
	}
}

func TestRoundTripConversion(t *testing.T) {
	original := LocalFunctionDescriptor{
		ID:           "player.ban",
		Version:      "1.0.0",
		Category:     "player",
		Tags:         []string{"moderation"},
		Summary:      "Ban Player",
		Description:  "Ban a player",
		InputSchema:  `{"type":"object"}`,
		OutputSchema: `{"type":"object"}`,
		Risk:         "high",
		Entity:       "Player",
		Operation:    "delete",
		OperationID:  "playerBan",
		Deprecated:   false,
	}

	// Convert to metadata
	metadata := LocalFunctionDescriptorToMetadata(original)
	if metadata == nil {
		t.Fatal("conversion failed")
	}

	// Convert back to descriptor
	converted := MetadataToLocalFunctionDescriptor(metadata)

	// Check key fields
	if converted.ID != original.ID {
		t.Errorf("ID = %v, want %v", converted.ID, original.ID)
	}
	if converted.Category != original.Category {
		t.Errorf("Category = %v, want %v", converted.Category, original.Category)
	}
	if converted.Risk != original.Risk {
		t.Errorf("Risk = %v, want %v", converted.Risk, original.Risk)
	}
	if converted.OperationID != original.OperationID {
		t.Errorf("OperationID = %v, want %v", converted.OperationID, original.OperationID)
	}
}

func TestNormalizeRiskLevel(t *testing.T) {
	tests := []struct {
		level    functionv1.FunctionSecurity_RiskLevel
		expected string
	}{
		{functionv1.FunctionSecurity_RISK_LEVEL_LOW, "low"},
		{functionv1.FunctionSecurity_RISK_LEVEL_MEDIUM, "medium"},
		{functionv1.FunctionSecurity_RISK_LEVEL_HIGH, "high"},
		{functionv1.FunctionSecurity_RISK_LEVEL_DANGER, "danger"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			got := normalizeRiskLevel(tt.level)
			if got != tt.expected {
				t.Errorf("normalizeRiskLevel(%v) = %v, want %v", tt.level, got, tt.expected)
			}
		})
	}
}

func TestLocalFunctionDescriptorJSONToMetadata(t *testing.T) {
	t.Run("valid JSON descriptor", func(t *testing.T) {
		jsonData := []byte(`{
			"id": "player.ban",
			"version": "1.0.0",
			"category": "player",
			"summary": "Ban Player",
			"description": "Ban a player from the game",
			"risk": "high",
			"entity": "Player",
			"operation": "delete"
		}`)

		metadata, err := LocalFunctionDescriptorJSONToMetadata(jsonData)
		if err != nil {
			t.Fatalf("LocalFunctionDescriptorJSONToMetadata failed: %v", err)
		}

		if metadata.Id != "player.ban" {
			t.Errorf("ID = %v, want player.ban", metadata.Id)
		}
		if metadata.Name != "Ban Player" {
			t.Errorf("Name = %v, want Ban Player", metadata.Name)
		}
		if metadata.Security.RiskLevel != functionv1.FunctionSecurity_RISK_LEVEL_HIGH {
			t.Errorf("RiskLevel = %v, want RISK_HIGH", metadata.Security.RiskLevel)
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		invalidData := []byte(`{invalid json}`)

		_, err := LocalFunctionDescriptorJSONToMetadata(invalidData)
		if err == nil {
			t.Error("Expected error for invalid JSON, got nil")
		}
	})

	t.Run("empty ID returns nil", func(t *testing.T) {
		jsonData := []byte(`{
			"id": "",
			"summary": "Test"
		}`)

		metadata, err := LocalFunctionDescriptorJSONToMetadata(jsonData)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if metadata != nil {
			t.Error("Expected nil metadata for empty ID")
		}
	})
}

func TestFunctionDescriptorJSONToMetadata(t *testing.T) {
	t.Run("valid JSON descriptor", func(t *testing.T) {
		jsonData := []byte(`{
			"id": "player.get",
			"version": "1.0.0",
			"category": "player",
			"risk": "low",
			"entity": "Player",
			"operation": "read",
			"enabled": true
		}`)

		metadata, err := FunctionDescriptorJSONToMetadata(jsonData)
		if err != nil {
			t.Fatalf("FunctionDescriptorJSONToMetadata failed: %v", err)
		}

		if metadata.Id != "player.get" {
			t.Errorf("ID = %v, want player.get", metadata.Id)
		}
		if metadata.Behavior.Mode != functionv1.FunctionBehavior_MODE_QUERY {
			t.Errorf("Mode = %v, want QUERY", metadata.Behavior.Mode)
		}
		if metadata.Security.RiskLevel != functionv1.FunctionSecurity_RISK_LEVEL_LOW {
			t.Errorf("RiskLevel = %v, want RISK_LOW", metadata.Security.RiskLevel)
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		invalidData := []byte(`{invalid}`)

		_, err := FunctionDescriptorJSONToMetadata(invalidData)
		if err == nil {
			t.Error("Expected error for invalid JSON, got nil")
		}
	})
}

func TestMetadataToLocalFunctionDescriptorJSON(t *testing.T) {
	metadata := &functionv1.FunctionMetadata{
		Id:           "player.ban",
		Version:      "1.0.0",
		Category:     "player",
		Name:         "Ban Player",
		Description:  "Ban a player",
		InputSchema:  `{"type":"object"}`,
		OutputSchema: `{"type":"object"}`,
		Behavior:     &functionv1.FunctionBehavior{Mode: functionv1.FunctionBehavior_MODE_COMMAND},
		Security:     &functionv1.FunctionSecurity{RiskLevel: functionv1.FunctionSecurity_RISK_LEVEL_HIGH},
		Extensions: map[string]string{
			"x-entity":    "Player",
			"x-operation": "delete",
		},
	}

	jsonData, err := MetadataToLocalFunctionDescriptorJSON(metadata)
	if err != nil {
		t.Fatalf("MetadataToLocalFunctionDescriptorJSON failed: %v", err)
	}

	if len(jsonData) == 0 {
		t.Error("Expected non-empty JSON data")
	}

	// Verify it can be parsed back
	parsed, err := LocalFunctionDescriptorJSONToMetadata(jsonData)
	if err != nil {
		t.Fatalf("Failed to parse generated JSON: %v", err)
	}

	if parsed.Id != metadata.Id {
		t.Errorf("Parsed ID = %v, want %v", parsed.Id, metadata.Id)
	}
	if parsed.Security.RiskLevel != functionv1.FunctionSecurity_RISK_LEVEL_HIGH {
		t.Errorf("Parsed RiskLevel = %v, want RISK_HIGH", parsed.Security.RiskLevel)
	}
}

func TestMetadataToFunctionDescriptorJSON(t *testing.T) {
	metadata := &functionv1.FunctionMetadata{
		Id:          "player.get",
		Version:     "1.0.0",
		Category:    "player",
		Name:        "Get Player",
		InputSchema: `{"type":"object"}`,
		Behavior:    &functionv1.FunctionBehavior{Mode: functionv1.FunctionBehavior_MODE_QUERY},
		Security:    &functionv1.FunctionSecurity{RiskLevel: functionv1.FunctionSecurity_RISK_LEVEL_LOW},
		Extensions: map[string]string{
			"x-entity": "Player",
		},
	}

	jsonData, err := MetadataToFunctionDescriptorJSON(metadata)
	if err != nil {
		t.Fatalf("MetadataToFunctionDescriptorJSON failed: %v", err)
	}

	if len(jsonData) == 0 {
		t.Error("Expected non-empty JSON data")
	}

	// Verify it can be parsed back
	parsed, err := FunctionDescriptorJSONToMetadata(jsonData)
	if err != nil {
		t.Fatalf("Failed to parse generated JSON: %v", err)
	}

	if parsed.Id != metadata.Id {
		t.Errorf("Parsed ID = %v, want %v", parsed.Id, metadata.Id)
	}
}

func TestMetadataToFunctionDescriptor(t *testing.T) {
	metadata := &functionv1.FunctionMetadata{
		Id:          "player.get",
		Version:     "1.0.0",
		Category:    "player",
		Name:        "Get Player",
		Description: "Get a player by ID",
		Behavior:    &functionv1.FunctionBehavior{Mode: functionv1.FunctionBehavior_MODE_QUERY},
		Security:    &functionv1.FunctionSecurity{RiskLevel: functionv1.FunctionSecurity_RISK_LEVEL_LOW},
		Extensions: map[string]string{
			"x-entity": "Player",
		},
	}

	desc := MetadataToFunctionDescriptor(metadata)

	if desc.ID != metadata.Id {
		t.Errorf("ID = %v, want %v", desc.ID, metadata.Id)
	}
	if desc.Category != metadata.Category {
		t.Errorf("Category = %v, want %v", desc.Category, metadata.Category)
	}
	if desc.Risk != "low" {
		t.Errorf("Risk = %v, want low", desc.Risk)
	}

	// Check that entity is in extensions
	if desc.Entity != "Player" {
		t.Errorf("Entity = %v, want Player", desc.Entity)
	}

	// Check enabled field
	if !desc.Enabled {
		t.Error("Expected Enabled to be true")
	}
}
