// Package freshness evaluates whether published page bindings still match
// the latest normalized FunctionSpec contracts.
package freshness

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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
	selectorChecked := false
	if strings.TrimSpace(fn.Version) != strings.TrimSpace(contract.FunctionVersion) {
		out = append(out, bindingFreshnessDiagnostic(
			bindingID,
			functionID,
			spec.BindingFreshnessFunctionVersionStale,
			"binding_function_version_stale",
			"bound function version changed; revalidate generated page bindings and publish a new page snapshot",
			"bindingContracts."+bindingID+".functionVersion",
		))
	}
	// digest 为空 = 发布于 digest 机制之前的旧快照——无法比对，
	// 视为兼容跳过（否则旧页面在新代码下恒为 stale 被误阻断）。
	if c := strings.TrimSpace(contract.InputSchemaDigest); c != "" && !digestMatch(fn.InputSchema, c) {
		out = append(out, bindingFreshnessDiagnostic(
			bindingID,
			functionID,
			spec.BindingFreshnessInputSchemaStale,
			"binding_input_schema_stale",
			"bound function input schema changed; synchronize the page input selector before publishing",
			"bindingContracts."+bindingID+".inputSchemaDigest",
		))
		out = append(out, selectorFreshnessDiagnostics(bindingID, functionID, binding, fn)...)
		selectorChecked = true
	}
	if c := strings.TrimSpace(contract.OutputSchemaDigest); c != "" && !digestMatch(fn.OutputSchema, c) {
		out = append(out, bindingFreshnessDiagnostic(
			bindingID,
			functionID,
			spec.BindingFreshnessOutputSchemaStale,
			"binding_output_schema_stale",
			"bound function output schema changed; revalidate page output selectors before publishing",
			"bindingContracts."+bindingID+".outputSchemaDigest",
		))
		if !selectorChecked {
			out = append(out, selectorFreshnessDiagnostics(bindingID, functionID, binding, fn)...)
		}
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
	if fn.Approval.Required != contract.Approval.Required || strings.TrimSpace(fn.Approval.PolicyKey) != strings.TrimSpace(contract.Approval.PolicyKey) {
		out = append(out, bindingFreshnessDiagnostic(
			bindingID,
			functionID,
			spec.BindingFreshnessGovernanceStale,
			"binding_approval_stale",
			"bound function approval policy changed; review governance and publish a new page snapshot",
			"bindingContracts."+bindingID+".approval",
		))
	}
	if executionModeForFunction(fn) != contract.ExecutionMode || binding.Execution.Mode != contract.ExecutionMode {
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

func executionModeForFunction(fn spec.FunctionSpec) spec.PageExecutionMode {
	if fn.Execution == spec.FunctionExecutionTask {
		return spec.PageExecutionModeTask
	}
	return spec.PageExecutionModeSync
}

func selectorFreshnessDiagnostics(
	bindingID string,
	functionID string,
	binding spec.PageFunctionBinding,
	fn spec.FunctionSpec,
) []spec.BindingFreshnessDiagnostic {
	if binding.Selectors == nil {
		return nil
	}
	rawDiags := spec.SelectorStaleDiagnostics(
		binding.Selectors.Input,
		binding.Selectors.Output,
		nil,
		fn.InputSchema,
		nil,
		fn.OutputSchema,
	)
	out := make([]spec.BindingFreshnessDiagnostic, 0, len(rawDiags))
	for _, diag := range rawDiags {
		diag.FunctionID = functionID
		out = append(out, spec.BindingFreshnessDiagnostic{
			BindingID:  bindingID,
			FunctionID: functionID,
			Status:     selectorFreshnessStatus(diag.Field),
			Diagnostic: diag,
		})
	}
	return out
}

func selectorFreshnessStatus(field string) spec.BindingFreshnessStatus {
	if strings.HasPrefix(field, "output.") {
		return spec.BindingFreshnessOutputSchemaStale
	}
	return spec.BindingFreshnessInputSchemaStale
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

// digestRawBytes 原始字节 digest（旧快照在发布时存的算法）。
func digestRawBytes(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

// digestMatch 兼容比对：canonical 或原始字节 digest 任一命中即视为
// 一致——旧快照（发布时存原始字节 digest）与新快照（canonical）都能
// 正确比对；schema 语义不变时永不误判 stale。
func digestMatch(schema []byte, stored string) bool {
	stored = strings.TrimSpace(stored)
	if stored == "" {
		return true // 旧快照无 digest：视为兼容
	}
	if digestRaw(schema) == stored {
		return true
	}
	return digestRawBytes(schema) == stored
}

// CanonicalDigest 计算语义化（canonical）JSON digest，供发布端与
// 校验端共用：JSON 解析后按字典序重排键再序列化哈希——同一 schema 的
// 不同字节形态（键序/空格，六语言 SDK 各自序列化）digest 恒定。
func CanonicalDigest(raw []byte) string { return digestRaw(raw) }

// digestRaw 计算语义化（canonical）digest：JSON 解析后按字典序重排键再
// 序列化哈希。原始字节哈希不可用——六语言 SDK 对同一 schema 的 JSON
// 字节序不同（键序/空格），字节比较会导致"形状一致却被判 stale"。
// 解析失败（非 JSON）时回退原始字节哈希以保持可确定性。
func digestRaw(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	var v interface{}
	if err := json.Unmarshal(raw, &v); err != nil {
		sum := sha256.Sum256(raw)
		return hex.EncodeToString(sum[:])
	}
	canonical, err := json.Marshal(canonicalizeJSON(v))
	if err != nil {
		sum := sha256.Sum256(raw)
		return hex.EncodeToString(sum[:])
	}
	sum := sha256.Sum256(canonical)
	return hex.EncodeToString(sum[:])
}

// canonicalizeJSON 递归排序 map 键（json.Marshal 对 map[string]X 按
// 字典序输出），并将 json.Number 保留原文本以避免数值精度扰动。
func canonicalizeJSON(v interface{}) interface{} {
	switch t := v.(type) {
	case map[string]interface{}:
		out := make(map[string]interface{}, len(t))
		for k, val := range t {
			out[k] = canonicalizeJSON(val)
		}
		return out
	case []interface{}:
		for i := range t {
			t[i] = canonicalizeJSON(t[i])
		}
		return t
	default:
		return v
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if v := strings.TrimSpace(value); v != "" {
			return v
		}
	}
	return ""
}
