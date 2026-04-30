// Package migration provides compatibility layers for migrating between
// legacy function descriptors and the new FunctionMetadata format.
package migration

import (
	"encoding/json"
	"strings"

	functionv1 "github.com/cuihairu/croupier/pkg/pb/croupier/function/v1"
)

// LocalFunctionDescriptor represents the legacy SDK descriptor format.
// This is defined here to avoid circular dependencies with the SDK.
type LocalFunctionDescriptor struct {
	ID           string   `json:"id"`
	Version      string   `json:"version"`
	Category     string   `json:"category"`
	Tags         []string `json:"tags"`
	Summary      string   `json:"summary"`
	Description  string   `json:"description"`
	OperationID  string   `json:"operation_id"`
	Deprecated   bool     `json:"deprecated"`
	InputSchema  string   `json:"input_schema"`
	OutputSchema string   `json:"output_schema"`
	Risk         string   `json:"risk"`
	Entity       string   `json:"entity"`
	Operation    string   `json:"operation"`
}

// FunctionDescriptor represents the simplified function descriptor format.
type FunctionDescriptor struct {
	ID        string `json:"id"`
	Version   string `json:"version"`
	Category  string `json:"category"`
	Risk      string `json:"risk"`
	Entity    string `json:"entity"`
	Operation string `json:"operation"`
	Enabled   bool   `json:"enabled"`
}

// LocalFunctionDescriptorJSONToMetadata converts a legacy LocalFunctionDescriptor JSON
// to the new FunctionMetadata protobuf format.
func LocalFunctionDescriptorJSONToMetadata(jsonData []byte) (*functionv1.FunctionMetadata, error) {
	var desc LocalFunctionDescriptor
	if err := json.Unmarshal(jsonData, &desc); err != nil {
		return nil, err
	}
	return LocalFunctionDescriptorToMetadata(desc), nil
}

// LocalFunctionDescriptorToMetadata converts a legacy LocalFunctionDescriptor
// to the new FunctionMetadata protobuf format.
func LocalFunctionDescriptorToMetadata(desc LocalFunctionDescriptor) *functionv1.FunctionMetadata {
	if desc.ID == "" {
		return nil
	}

	metadata := &functionv1.FunctionMetadata{
		Id:           desc.ID,
		Version:      desc.Version,
		Category:     desc.Category,
		Tags:         desc.Tags,
		Name:         desc.Summary,
		Description:  desc.Description,
		InputSchema:  desc.InputSchema,
		OutputSchema: desc.OutputSchema,
		Extensions:   make(map[string]string),
	}

	// Add extension fields
	if desc.OperationID != "" {
		metadata.Extensions["x-operation-id"] = desc.OperationID
	}
	if desc.Deprecated {
		metadata.Extensions["x-deprecated"] = "true"
	}
	if desc.Entity != "" {
		metadata.Extensions["x-entity"] = desc.Entity
	}
	if desc.Operation != "" {
		metadata.Extensions["x-operation"] = desc.Operation
	}

	// Set default behavior
	metadata.Behavior = &functionv1.FunctionBehavior{
		Mode:          inferModeFromID(desc.ID),
		Idempotent:    inferIdempotency(desc.ID),
		TimeoutMs:     30000,
		RouteStrategy: functionv1.FunctionBehavior_ROUTE_STRATEGY_LB,
		Cacheable:     false,
	}

	// Set default security
	metadata.Security = &functionv1.FunctionSecurity{
		RiskLevel:         inferRiskLevel(desc.Risk),
		RequiresApproval:  inferRequiresApproval(desc.Risk),
		AuditLog:          true,
		MaskSensitiveData: desc.Risk == "danger" || desc.Risk == "high",
	}

	return metadata
}

// FunctionDescriptorJSONToMetadata converts a legacy FunctionDescriptor JSON
// to the new FunctionMetadata protobuf format.
func FunctionDescriptorJSONToMetadata(jsonData []byte) (*functionv1.FunctionMetadata, error) {
	var desc FunctionDescriptor
	if err := json.Unmarshal(jsonData, &desc); err != nil {
		return nil, err
	}
	return FunctionDescriptorToMetadata(desc), nil
}

// FunctionDescriptorToMetadata converts a legacy FunctionDescriptor
// to the new FunctionMetadata protobuf format.
func FunctionDescriptorToMetadata(desc FunctionDescriptor) *functionv1.FunctionMetadata {
	if desc.ID == "" {
		return nil
	}

	metadata := &functionv1.FunctionMetadata{
		Id:           desc.ID,
		Version:      desc.Version,
		Category:     desc.Category,
		Tags:         []string{},
		Name:         desc.ID,
		InputSchema:  "{}",
		OutputSchema: "{}",
		Extensions:   make(map[string]string),
	}

	// Add entity and operation as tags
	if desc.Entity != "" {
		metadata.Tags = append(metadata.Tags, desc.Entity)
		metadata.Extensions["x-entity"] = desc.Entity
	}
	if desc.Operation != "" {
		metadata.Tags = append(metadata.Tags, desc.Operation)
		metadata.Extensions["x-operation"] = desc.Operation
	}

	// Set enabled status
	if !desc.Enabled {
		metadata.Extensions["x-enabled"] = "false"
	}

	// Set default behavior
	metadata.Behavior = &functionv1.FunctionBehavior{
		Mode:          inferModeFromOperation(desc.Operation),
		Idempotent:    inferIdempotencyFromOperation(desc.Operation),
		TimeoutMs:     30000,
		RouteStrategy: functionv1.FunctionBehavior_ROUTE_STRATEGY_LB,
		Cacheable:     false,
	}

	// Set default security
	metadata.Security = &functionv1.FunctionSecurity{
		RiskLevel:         inferRiskLevel(desc.Risk),
		RequiresApproval:  inferRequiresApproval(desc.Risk),
		AuditLog:          true,
		MaskSensitiveData: desc.Risk == "danger" || desc.Risk == "high",
	}

	return metadata
}

// MetadataToLocalFunctionDescriptorJSON converts a FunctionMetadata
// to legacy LocalFunctionDescriptor JSON format.
func MetadataToLocalFunctionDescriptorJSON(metadata *functionv1.FunctionMetadata) ([]byte, error) {
	desc := MetadataToLocalFunctionDescriptor(metadata)
	return json.Marshal(desc)
}

// MetadataToLocalFunctionDescriptor converts a FunctionMetadata
// back to legacy LocalFunctionDescriptor format.
func MetadataToLocalFunctionDescriptor(metadata *functionv1.FunctionMetadata) LocalFunctionDescriptor {
	if metadata == nil {
		return LocalFunctionDescriptor{}
	}

	desc := LocalFunctionDescriptor{
		ID:           metadata.Id,
		Version:      metadata.Version,
		Category:     metadata.Category,
		Tags:         metadata.Tags,
		Summary:      metadata.Name,
		Description:  metadata.Description,
		InputSchema:  metadata.InputSchema,
		OutputSchema: metadata.OutputSchema,
	}

	// Extract extension fields
	if val, ok := metadata.Extensions["x-operation-id"]; ok {
		desc.OperationID = val
	}
	if val, ok := metadata.Extensions["x-deprecated"]; ok && val == "true" {
		desc.Deprecated = true
	}
	if val, ok := metadata.Extensions["x-entity"]; ok {
		desc.Entity = val
	}
	if val, ok := metadata.Extensions["x-operation"]; ok {
		desc.Operation = val
	}

	// Extract security info
	if metadata.Security != nil {
		desc.Risk = normalizeRiskLevel(metadata.Security.RiskLevel)
	}

	return desc
}

// MetadataToFunctionDescriptorJSON converts a FunctionMetadata
// to legacy FunctionDescriptor JSON format.
func MetadataToFunctionDescriptorJSON(metadata *functionv1.FunctionMetadata) ([]byte, error) {
	desc := MetadataToFunctionDescriptor(metadata)
	return json.Marshal(desc)
}

// MetadataToFunctionDescriptor converts a FunctionMetadata
// back to legacy FunctionDescriptor format.
func MetadataToFunctionDescriptor(metadata *functionv1.FunctionMetadata) FunctionDescriptor {
	if metadata == nil {
		return FunctionDescriptor{}
	}

	desc := FunctionDescriptor{
		ID:       metadata.Id,
		Version:  metadata.Version,
		Category: metadata.Category,
		Enabled:  true,
	}

	// Extract entity and operation
	if val, ok := metadata.Extensions["x-entity"]; ok {
		desc.Entity = val
	}
	if val, ok := metadata.Extensions["x-operation"]; ok {
		desc.Operation = val
	}

	// Extract security info
	if metadata.Security != nil {
		desc.Risk = normalizeRiskLevel(metadata.Security.RiskLevel)
	}

	// Check if disabled
	if val, ok := metadata.Extensions["x-enabled"]; ok && val == "false" {
		desc.Enabled = false
	}

	return desc
}

// Helper functions for inferring metadata values

func inferModeFromID(id string) functionv1.FunctionBehavior_Mode {
	id = strings.ToLower(id)

	// Query operations
	if strings.Contains(id, "get") ||
		strings.Contains(id, "list") ||
		strings.Contains(id, "find") ||
		strings.Contains(id, "query") ||
		strings.Contains(id, "fetch") ||
		strings.Contains(id, "info") ||
		strings.Contains(id, "check") {
		return functionv1.FunctionBehavior_MODE_QUERY
	}

	// Default to command for write operations
	return functionv1.FunctionBehavior_MODE_COMMAND
}

func inferModeFromOperation(operation string) functionv1.FunctionBehavior_Mode {
	operation = strings.ToLower(operation)

	if operation == "read" || operation == "query" || operation == "list" {
		return functionv1.FunctionBehavior_MODE_QUERY
	}

	return functionv1.FunctionBehavior_MODE_COMMAND
}

func inferIdempotency(id string) bool {
	id = strings.ToLower(id)

	// These operations are generally idempotent
	if strings.Contains(id, "get") ||
		strings.Contains(id, "set") ||
		strings.Contains(id, "update") ||
		strings.Contains(id, "delete") ||
		strings.Contains(id, "ban") ||
		strings.Contains(id, "mute") ||
		strings.Contains(id, "enable") ||
		strings.Contains(id, "disable") {
		return true
	}

	// These operations are generally NOT idempotent
	if strings.Contains(id, "create") ||
		strings.Contains(id, "add") ||
		strings.Contains(id, "send") ||
		strings.Contains(id, "grant") {
		return false
	}

	return false
}

func inferIdempotencyFromOperation(operation string) bool {
	operation = strings.ToLower(operation)

	// Idempotent operations
	if operation == "update" || operation == "delete" || operation == "read" {
		return true
	}

	return false
}

func inferRiskLevel(risk string) functionv1.FunctionSecurity_RiskLevel {
	risk = strings.ToLower(risk)

	switch risk {
	case "low", "safe", "info":
		return functionv1.FunctionSecurity_RISK_LEVEL_LOW
	case "medium", "moderate", "warning":
		return functionv1.FunctionSecurity_RISK_LEVEL_MEDIUM
	case "high", "error":
		return functionv1.FunctionSecurity_RISK_LEVEL_HIGH
	case "critical", "fatal":
		return functionv1.FunctionSecurity_RISK_LEVEL_DANGER
	default:
		return functionv1.FunctionSecurity_RISK_LEVEL_MEDIUM
	}
}

func inferRequiresApproval(risk string) bool {
	risk = strings.ToLower(risk)
	return risk == "high" || risk == "danger" || risk == "critical" || risk == "fatal" || risk == "error"
}

func normalizeRiskLevel(level functionv1.FunctionSecurity_RiskLevel) string {
	switch level {
	case functionv1.FunctionSecurity_RISK_LEVEL_LOW:
		return "low"
	case functionv1.FunctionSecurity_RISK_LEVEL_MEDIUM:
		return "medium"
	case functionv1.FunctionSecurity_RISK_LEVEL_HIGH:
		return "high"
	case functionv1.FunctionSecurity_RISK_LEVEL_DANGER:
		return "danger"
	default:
		return "medium"
	}
}
