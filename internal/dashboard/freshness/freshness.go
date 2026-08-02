// Package freshness evaluates whether published page bindings still match
// the latest normalized FunctionSpec contracts.
package freshness

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
)

// EvaluatePublishedBindings returns diagnostics for every published binding
// whose frozen contract no longer matches the latest FunctionSpec.
func EvaluatePublishedBindings(
	bindings []spec.PageFunctionBinding,
	contracts []spec.BindingContractSnapshot,
	functions map[string]spec.FunctionSpec,
) []spec.BindingFreshnessDiagnostic {
	contractsByBinding := make(map[string]spec.BindingContractSnapshot, len(contracts))
	for _, contract := range contracts {
		id := strings.TrimSpace(contract.BindingID)
		if id != "" {
			contractsByBinding[id] = contract
		}
	}

	var out []spec.BindingFreshnessDiagnostic
	for _, binding := range bindings {
		bindingID := strings.TrimSpace(binding.ID)
		functionID := strings.TrimSpace(binding.FunctionID)
		contract, ok := contractsByBinding[bindingID]
		if !ok {
			out = append(out, bindingFreshnessDiagnostic(
				bindingID,
				functionID,
				spec.BindingFreshnessContractMissing,
				"binding_contract_missing",
				"published binding contract snapshot is missing; re-publish the page before execution",
				"bindingContracts",
			))
			continue
		}
		out = append(out, EvaluateBinding(binding, contract, functions)...)
	}
	return out
}

// EvaluateBinding compares a single published binding contract against the
// latest FunctionSpec and returns the explicit stale reasons.
func EvaluateBinding(
	binding spec.PageFunctionBinding,
	contract spec.BindingContractSnapshot,
	functions map[string]spec.FunctionSpec,
) []spec.BindingFreshnessDiagnostic {
	bindingID := strings.TrimSpace(binding.ID)
	functionID := strings.TrimSpace(firstNonEmpty(binding.FunctionID, contract.FunctionID))
	fn, ok := functions[functionID]
	if !ok {
		return []spec.BindingFreshnessDiagnostic{bindingFreshnessDiagnostic(
			bindingID,
			functionID,
			spec.BindingFreshnessFunctionMissing,
			"binding_function_missing",
			"bound function no longer exists; synchronize the page binding and publish again",
			"bindings."+bindingID,
		)}
	}

	var out []spec.BindingFreshnessDiagnostic
	if strings.TrimSpace(fn.Version) != strings.TrimSpace(contract.FunctionVersion) {
		out = append(out, bindingFreshnessDiagnostic(
			bindingID,
			functionID,
			spec.BindingFreshnessFunctionVersionStale,
			"binding_function_version_stale",
			"bound function version changed; revalidate generated Function Form and publish a new page snapshot",
			"bindingContracts."+bindingID+".functionVersion",
		))
	}
	if digestRaw(fn.InputSchema) != strings.TrimSpace(contract.InputSchemaDigest) {
		out = append(out, bindingFreshnessDiagnostic(
			bindingID,
			functionID,
			spec.BindingFreshnessInputSchemaStale,
			"binding_input_schema_stale",
			"bound function input schema changed; synchronize the page input selector before publishing",
			"bindingContracts."+bindingID+".inputSchemaDigest",
		))
	}
	if digestRaw(fn.OutputSchema) != strings.TrimSpace(contract.OutputSchemaDigest) {
		out = append(out, bindingFreshnessDiagnostic(
			bindingID,
			functionID,
			spec.BindingFreshnessOutputSchemaStale,
			"binding_output_schema_stale",
			"bound function output schema changed; revalidate page output selectors before publishing",
			"bindingContracts."+bindingID+".outputSchemaDigest",
		))
	}
	if fn.Risk != contract.Risk || strings.TrimSpace(fn.Permission) != strings.TrimSpace(contract.Permission) {
		out = append(out, bindingFreshnessDiagnostic(
			bindingID,
			functionID,
			spec.BindingFreshnessGovernanceStale,
			"binding_governance_stale",
			"bound function risk or permission changed; review governance and publish a new page snapshot",
			"bindingContracts."+bindingID+".governance",
		))
	}
	if binding.Execution.Mode != contract.ExecutionMode {
		out = append(out, bindingFreshnessDiagnostic(
			bindingID,
			functionID,
			spec.BindingFreshnessExecutionModeStale,
			"binding_execution_mode_stale",
			"page binding execution mode no longer matches the published contract; save and publish the page again",
			"bindingContracts."+bindingID+".executionMode",
		))
	}
	return out
}

func bindingFreshnessDiagnostic(
	bindingID string,
	functionID string,
	status spec.BindingFreshnessStatus,
	code string,
	message string,
	field string,
) spec.BindingFreshnessDiagnostic {
	return spec.BindingFreshnessDiagnostic{
		BindingID:  bindingID,
		FunctionID: functionID,
		Status:     status,
		Diagnostic: spec.Diagnostic{
			Code:       code,
			Severity:   spec.SeverityError,
			Message:    message,
			FunctionID: functionID,
			Field:      field,
		},
	}
}

func digestRaw(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if v := strings.TrimSpace(value); v != "" {
			return v
		}
	}
	return ""
}
