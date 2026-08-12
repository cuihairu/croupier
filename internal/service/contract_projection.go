package service

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/model"
)

// FunctionSpecsByScope projects persisted FunctionContract rows into the
// canonical FunctionSpec map consumed by PageSpec validation and runtime.
func FunctionSpecsByScope(ctx context.Context, contractModel *model.FunctionContractModel, gameID, env string) (map[string]spec.FunctionSpec, error) {
	contracts, err := contractModel.ListByScope(ctx, strings.TrimSpace(gameID), strings.TrimSpace(env))
	if err != nil {
		return nil, err
	}
	return FunctionSpecsFromContracts(contracts), nil
}

func FunctionSpecsFromContracts(contracts []*model.FunctionContract) map[string]spec.FunctionSpec {
	out := make(map[string]spec.FunctionSpec, len(contracts))
	for _, contract := range contracts {
		if contract == nil || strings.TrimSpace(contract.FunctionID) == "" {
			continue
		}
		out[strings.TrimSpace(contract.FunctionID)] = FunctionSpecFromContract(contract)
	}
	return out
}

// normalizeJSONSchema ensures raw is a native JSON object, not a JSON string.
// Some code paths store schema as a JSON string value (e.g. "{\"type\":\"object\"}")
// instead of native JSON ({\"type\":\"object\"}). This function unwraps such strings.
func normalizeJSONSchema(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return raw
	}
	// If it starts with a quote, it's a JSON string — unwrap it
	if raw[0] == '"' {
		var s string
		if err := json.Unmarshal(raw, &s); err == nil {
			return json.RawMessage(s)
		}
	}
	return raw
}

func FunctionSpecFromContract(contract *model.FunctionContract) spec.FunctionSpec {
	if contract == nil {
		return spec.FunctionSpec{}
	}
	return spec.FunctionSpec{
		ID:           strings.TrimSpace(contract.FunctionID),
		Version:      strings.TrimSpace(contract.Version),
		Enabled:      contract.Enabled,
		Deprecated:   contract.Deprecated,
		InputSchema:  spec.JSONSchema(normalizeJSONSchema(json.RawMessage(contract.InputSchema))),
		OutputSchema: spec.JSONSchema(normalizeJSONSchema(json.RawMessage(contract.OutputSchema))),
		Summary:      LocalizedTextFromJSONMap(contract.Summary),
		Description:  LocalizedTextFromJSONMap(contract.Description),
		Resource:     strings.TrimSpace(contract.ResourceKey),
		Operation:    strings.TrimSpace(contract.OperationKey),
		Capability:   spec.CapabilityKind(contract.Capability),
		Execution:    spec.FunctionExecution(contract.Execution),
		Approval:     ApprovalPolicyFromJSONMap(contract.Approval),
		Risk:         spec.RiskLevel(contract.Risk),
		Permission:   strings.TrimSpace(contract.Permission),
	}
}

func OperationSpecFromContract(contract *model.FunctionContract) spec.OperationSpec {
	if contract == nil {
		return spec.OperationSpec{}
	}
	return spec.OperationSpec{
		FunctionID:  strings.TrimSpace(contract.FunctionID),
		ResourceKey: strings.TrimSpace(contract.ResourceKey),
		Operation:   strings.TrimSpace(contract.OperationKey),
		Capability:  spec.CapabilityKind(contract.Capability),
		Execution:   spec.FunctionExecution(contract.Execution),
		Approval:    ApprovalPolicyFromJSONMap(contract.Approval),
		Risk:        spec.RiskLevel(contract.Risk),
		Permission:  strings.TrimSpace(contract.Permission),
		Enabled:     contract.Enabled,
	}
}

func ApprovalPolicyFromJSONMap(values map[string]interface{}) spec.ApprovalPolicy {
	if len(values) == 0 {
		return spec.ApprovalPolicy{}
	}
	required, _ := values["required"].(bool)
	policyKey, _ := values["policyKey"].(string)
	if policyKey == "" {
		policyKey, _ = values["policy_key"].(string)
	}
	return spec.ApprovalPolicy{
		Required:  required,
		PolicyKey: strings.TrimSpace(policyKey),
	}
}

func LocalizedTextFromJSONMap(values map[string]interface{}) spec.LocalizedText {
	if len(values) == 0 {
		return nil
	}
	out := make(spec.LocalizedText, len(values))
	for key, value := range values {
		text, ok := value.(string)
		if !ok || strings.TrimSpace(text) == "" {
			continue
		}
		out[strings.TrimSpace(key)] = strings.TrimSpace(text)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
