package api

import (
	"testing"

	functionv1 "github.com/cuihairu/croupier/pkg/pb/croupier/function/v1"
)

func TestNormalizeMode(t *testing.T) {
	tests := []struct {
		input    functionv1.FunctionBehavior_Mode
		expected string
	}{
		{functionv1.FunctionBehavior_MODE_QUERY, "query"},
		{functionv1.FunctionBehavior_MODE_COMMAND, "command"},
		{functionv1.FunctionBehavior_MODE_UNSPECIFIED, "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := normalizeMode(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeMode(%v) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestNormalizeRouteStrategy(t *testing.T) {
	tests := []struct {
		name     string
		input    functionv1.FunctionBehavior_RouteStrategy
		expected string
	}{
		{"lb", functionv1.FunctionBehavior_ROUTE_STRATEGY_LB, "lb"},
		{"broadcast", functionv1.FunctionBehavior_ROUTE_STRATEGY_BROADCAST, "broadcast"},
		{"targeted", functionv1.FunctionBehavior_ROUTE_STRATEGY_TARGETED, "targeted"},
		{"hash", functionv1.FunctionBehavior_ROUTE_STRATEGY_HASH, "hash"},
		{"default", functionv1.FunctionBehavior_ROUTE_STRATEGY_UNSPECIFIED, "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeRouteStrategy(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeRouteStrategy(%v) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestNormalizeRiskLevel(t *testing.T) {
	tests := []struct {
		name     string
		input    functionv1.FunctionSecurity_RiskLevel
		expected string
	}{
		{"low", functionv1.FunctionSecurity_RISK_LEVEL_LOW, "low"},
		{"medium", functionv1.FunctionSecurity_RISK_LEVEL_MEDIUM, "medium"},
		{"high", functionv1.FunctionSecurity_RISK_LEVEL_HIGH, "high"},
		{"danger", functionv1.FunctionSecurity_RISK_LEVEL_DANGER, "danger"},
		{"default", functionv1.FunctionSecurity_RISK_LEVEL_UNSPECIFIED, "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeRiskLevel(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeRiskLevel(%v) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestNormalizeApprovalType(t *testing.T) {
	tests := []struct {
		name     string
		input    functionv1.FunctionSecurity_ApprovalType
		expected string
	}{
		{"unspecified", functionv1.FunctionSecurity_APPROVAL_TYPE_UNSPECIFIED, "none"},
		{"single", functionv1.FunctionSecurity_APPROVAL_TYPE_SINGLE, "single"},
		{"two_person", functionv1.FunctionSecurity_APPROVAL_TYPE_TWO_PERSON, "two_person"},
		{"unknown", 999, "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeApprovalType(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeApprovalType(%v) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestParseMode(t *testing.T) {
	tests := []struct {
		input    string
		expected functionv1.FunctionBehavior_Mode
	}{
		{"query", functionv1.FunctionBehavior_MODE_QUERY},
		{"command", functionv1.FunctionBehavior_MODE_COMMAND},
		{"unknown", functionv1.FunctionBehavior_MODE_UNSPECIFIED},
		{"", functionv1.FunctionBehavior_MODE_UNSPECIFIED},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseMode(tt.input)
			if result != tt.expected {
				t.Errorf("parseMode(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestParseRouteStrategy(t *testing.T) {
	tests := []struct {
		input    string
		expected functionv1.FunctionBehavior_RouteStrategy
	}{
		{"lb", functionv1.FunctionBehavior_ROUTE_STRATEGY_LB},
		{"broadcast", functionv1.FunctionBehavior_ROUTE_STRATEGY_BROADCAST},
		{"targeted", functionv1.FunctionBehavior_ROUTE_STRATEGY_TARGETED},
		{"hash", functionv1.FunctionBehavior_ROUTE_STRATEGY_HASH},
		{"unknown", functionv1.FunctionBehavior_ROUTE_STRATEGY_UNSPECIFIED},
		{"", functionv1.FunctionBehavior_ROUTE_STRATEGY_UNSPECIFIED},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseRouteStrategy(tt.input)
			if result != tt.expected {
				t.Errorf("parseRouteStrategy(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestParseRiskLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected functionv1.FunctionSecurity_RiskLevel
	}{
		{"low", functionv1.FunctionSecurity_RISK_LEVEL_LOW},
		{"medium", functionv1.FunctionSecurity_RISK_LEVEL_MEDIUM},
		{"high", functionv1.FunctionSecurity_RISK_LEVEL_HIGH},
		{"danger", functionv1.FunctionSecurity_RISK_LEVEL_DANGER},
		{"unknown", functionv1.FunctionSecurity_RISK_LEVEL_UNSPECIFIED},
		{"", functionv1.FunctionSecurity_RISK_LEVEL_UNSPECIFIED},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseRiskLevel(tt.input)
			if result != tt.expected {
				t.Errorf("parseRiskLevel(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestParseApprovalType(t *testing.T) {
	tests := []struct {
		input    string
		expected functionv1.FunctionSecurity_ApprovalType
	}{
		{"none", functionv1.FunctionSecurity_APPROVAL_TYPE_UNSPECIFIED},
		{"single", functionv1.FunctionSecurity_APPROVAL_TYPE_SINGLE},
		{"two_person", functionv1.FunctionSecurity_APPROVAL_TYPE_TWO_PERSON},
		{"unknown", functionv1.FunctionSecurity_APPROVAL_TYPE_UNSPECIFIED},
		{"", functionv1.FunctionSecurity_APPROVAL_TYPE_UNSPECIFIED},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseApprovalType(tt.input)
			if result != tt.expected {
				t.Errorf("parseApprovalType(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestMetadataToProto_Complete(t *testing.T) {
	dto := &FunctionMetadata{
		ID:           "test.function",
		Version:      "1.0.0",
		Category:     "test",
		Name:         "Test Function",
		Description:  "A test function",
		Tags:         []string{"tag1", "tag2"},
		InputSchema:  `{"type":"object"}`,
		OutputSchema: `{"type":"string"}`,
		Behavior: &FunctionBehavior{
			Mode:          "command",
			Idempotent:    true,
			TimeoutMs:     60000,
			RouteStrategy: "hash",
			Cacheable:     true,
		},
		Security: &FunctionSecurity{
			RiskLevel:         "high",
			Permission:        "test.invoke",
			RequiresApproval:  true,
			ApprovalType:      "two_person",
			AuditLog:          true,
			MaskSensitiveData: true,
		},
		Extensions: map[string]string{
			"x-custom": "value",
		},
	}

	pb := MetadataToProto(dto)

	if pb.Id != "test.function" {
		t.Errorf("Id = %v, want test.function", pb.Id)
	}
	if pb.Category != "test" {
		t.Errorf("Category = %v, want test", pb.Category)
	}
	if pb.Name != "Test Function" {
		t.Errorf("Name = %v, want Test Function", pb.Name)
	}
	if len(pb.Tags) != 2 {
		t.Errorf("Tags length = %d, want 2", len(pb.Tags))
	}
	if pb.Behavior == nil {
		t.Fatal("Behavior should not be nil")
	}
	if pb.Behavior.Mode != functionv1.FunctionBehavior_MODE_COMMAND {
		t.Errorf("Mode = %v, want COMMAND", pb.Behavior.Mode)
	}
	if pb.Behavior.RouteStrategy != functionv1.FunctionBehavior_ROUTE_STRATEGY_HASH {
		t.Errorf("RouteStrategy = %v, want HASH", pb.Behavior.RouteStrategy)
	}
	if pb.Security == nil {
		t.Fatal("Security should not be nil")
	}
	if pb.Security.RiskLevel != functionv1.FunctionSecurity_RISK_LEVEL_HIGH {
		t.Errorf("RiskLevel = %v, want HIGH", pb.Security.RiskLevel)
	}
	if pb.Security.ApprovalType != functionv1.FunctionSecurity_APPROVAL_TYPE_TWO_PERSON {
		t.Errorf("ApprovalType = %v, want TWO_PERSON", pb.Security.ApprovalType)
	}
	if pb.Extensions["x-custom"] != "value" {
		t.Errorf("x-custom = %v, want value", pb.Extensions["x-custom"])
	}
}

func TestProtoToMetadata_Complete(t *testing.T) {
	pb := &functionv1.FunctionMetadata{
		Id:           "test.function",
		Version:      "1.0.0",
		Category:     "test",
		Name:         "Test Function",
		Description:  "A test function",
		Tags:         []string{"tag1"},
		InputSchema:  `{"type":"object"}`,
		OutputSchema: `{"type":"string"}`,
		Behavior: &functionv1.FunctionBehavior{
			Mode:          functionv1.FunctionBehavior_MODE_COMMAND,
			Idempotent:    true,
			TimeoutMs:     60000,
			RouteStrategy: functionv1.FunctionBehavior_ROUTE_STRATEGY_HASH,
			Cacheable:     true,
		},
		Security: &functionv1.FunctionSecurity{
			RiskLevel:         functionv1.FunctionSecurity_RISK_LEVEL_HIGH,
			Permission:        "test.invoke",
			RequiresApproval:  true,
			ApprovalType:      functionv1.FunctionSecurity_APPROVAL_TYPE_TWO_PERSON,
			AuditLog:          true,
			MaskSensitiveData: true,
		},
		Extensions: map[string]string{
			"x-custom": "value",
		},
	}

	dto := ProtoToMetadata(pb)

	if dto.ID != "test.function" {
		t.Errorf("ID = %v, want test.function", dto.ID)
	}
	if dto.Behavior.Mode != "command" {
		t.Errorf("Mode = %v, want command", dto.Behavior.Mode)
	}
	if dto.Behavior.RouteStrategy != "hash" {
		t.Errorf("RouteStrategy = %v, want hash", dto.Behavior.RouteStrategy)
	}
	if dto.Security.RiskLevel != "high" {
		t.Errorf("RiskLevel = %v, want high", dto.Security.RiskLevel)
	}
	if dto.Security.ApprovalType != "two_person" {
		t.Errorf("ApprovalType = %v, want two_person", dto.Security.ApprovalType)
	}
}

func TestProtoToMetadata_Nil(t *testing.T) {
	result := ProtoToMetadata(nil)
	if result != nil {
		t.Errorf("Expected nil for nil input, got %v", result)
	}
}

func TestMetadataToProto_Nil(t *testing.T) {
	result := MetadataToProto(nil)
	if result != nil {
		t.Errorf("Expected nil for nil input, got %v", result)
	}
}

func TestProtoToMetadata_WithNilBehavior(t *testing.T) {
	pb := &functionv1.FunctionMetadata{
		Id:   "test.minimal",
		Name: "Minimal",
	}

	dto := ProtoToMetadata(pb)

	if dto.ID != "test.minimal" {
		t.Errorf("ID = %v, want test.minimal", dto.ID)
	}
	if dto.Behavior != nil {
		t.Error("Behavior should be nil when proto behavior is nil")
	}
	if dto.Security != nil {
		t.Error("Security should be nil when proto security is nil")
	}
}

func TestMetadataToProto_WithNilBehavior(t *testing.T) {
	dto := &FunctionMetadata{
		ID:   "test.minimal",
		Name: "Minimal",
	}

	pb := MetadataToProto(dto)

	if pb.Id != "test.minimal" {
		t.Errorf("Id = %v, want test.minimal", pb.Id)
	}
	if pb.Behavior != nil {
		t.Error("Behavior should be nil when dto behavior is nil")
	}
	if pb.Security != nil {
		t.Error("Security should be nil when dto security is nil")
	}
}
