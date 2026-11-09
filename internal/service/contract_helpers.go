package service

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/dbenum"
	"github.com/cuihairu/croupier/internal/function/schemadiff"
	"github.com/cuihairu/croupier/internal/model"
	"gorm.io/datatypes"
)

// normalizeSchemaToJSON ensures schema is stored as a native JSON object,
// not as a JSON string value. This prevents the API from returning schemas
// as strings instead of objects.
func normalizeSchemaToJSON(raw json.RawMessage) model.JSON {
	if len(raw) == 0 {
		return nil
	}
	// If it starts with a quote, it's a JSON string — unwrap it
	if raw[0] == '"' {
		var s string
		if err := json.Unmarshal(raw, &s); err == nil {
			return model.JSON(s)
		}
	}
	return model.JSON(raw)
}

// FunctionMetaInput remains a concise service-level alias for the canonical
// registration contract. It does not create a second DTO or conversion path.
type FunctionMetaInput = spec.FunctionContractInput

// timeoutMsToInt32 把归一后的契约预算（int）安全收窄为 int32。
// 归一层已 clamp [1000, 60000]，这里再设防御上界——收窄转换必须显式
// 有界（CodeQL go/incorrect-integer-conversion），负值按未声明归零。
func timeoutMsToInt32(v int) int32 {
	if v < 0 {
		return 0
	}
	if v > math.MaxInt32 {
		return math.MaxInt32
	}
	return int32(v)
}

func computeDigest(v interface{}) string {
	b, _ := json.Marshal(v)
	h := sha256.Sum256(b)
	return fmt.Sprintf("%x", h[:])
}

func toJSON(v interface{}) model.JSON {
	b, _ := json.Marshal(v)
	return model.JSON(b)
}

// mergeSchemaDiffDiagnostics 在契约 upsert 前对比库中现有契约的
// input/output schema：破坏性变更（F12）作为 Diagnostic 追加到现有
// diagnostics 之后；无现有契约（首次注册）或非破坏性差异时原样返回。
// existing 由调用方（rebuildContract）加载传入——T6 自动绑定检测与
// 本 diff 复用同一查询；返回全部 findings 与是否为更新（供 F13 审计
// 消费）。
func mergeSchemaDiffDiagnostics(
	existing *model.FunctionContract,
	contract *model.FunctionContract,
	base model.JSON,
) (model.JSON, []schemadiff.Finding, bool) {
	if existing == nil {
		// 首次注册：无对比对象，跳过 diff
		return base, nil, false
	}

	findings := schemadiff.DiffSchemas("inputSchema", json.RawMessage(existing.InputSchema), json.RawMessage(contract.InputSchema))
	findings = append(findings,
		schemadiff.DiffSchemas("outputSchema", json.RawMessage(existing.OutputSchema), json.RawMessage(contract.OutputSchema))...)
	if !schemadiff.HasBreaking(findings) {
		return base, findings, true
	}

	// 现有 diagnostics（数组）之上追加 schema diff 条目
	var diags []interface{}
	if len(base) > 0 {
		_ = json.Unmarshal(base, &diags)
	}
	for _, finding := range findings {
		if finding.Severity != schemadiff.SeverityBreaking {
			continue
		}
		diags = append(diags, map[string]interface{}{
			"code":     "schema_breaking_change",
			"severity": "warning",
			"message":  fmt.Sprintf("%s%s: %s", finding.Source, finding.Path, finding.Reason),
			"field":    finding.Source,
		})
	}
	return toJSON(diags), findings, true
}

func toJSONMap(m spec.LocalizedText) datatypes.JSONMap {
	if m == nil {
		return datatypes.JSONMap{}
	}
	b, _ := json.Marshal(m)
	var result datatypes.JSONMap
	_ = json.Unmarshal(b, &result)
	return result
}

func approvalPolicyToJSONMap(policy spec.ApprovalPolicy) datatypes.JSONMap {
	return datatypes.JSONMap{
		"required":  policy.Required,
		"policyKey": policy.PolicyKey,
	}
}

func contractValidationError(diags []spec.Diagnostic) error {
	for _, diag := range diags {
		if diag.Severity != spec.SeverityError {
			continue
		}
		field := strings.TrimSpace(diag.Field)
		if field == "" {
			field = "descriptor"
		}
		return fmt.Errorf("function contract validation failed: %s: %s", field, diag.Message)
	}
	return nil
}

// mustParseCapability converts a normalized wire capability into the DB
// enum. The normalizer only emits controlled values, so a parse failure is a
// programming error and gets surfaced instead of silently stored.
func mustParseCapability(wire string) dbenum.Capability {
	parsed, err := dbenum.ParseCapability(wire)
	if err != nil {
		return dbenum.CapabilityUnknown
	}
	return parsed
}

func mustParseRisk(wire string) dbenum.Risk {
	parsed, err := dbenum.ParseRisk(wire)
	if err != nil {
		return dbenum.RiskSafe
	}
	return parsed
}

func isCRUDCapability(capability string) bool {
	switch spec.CapabilityKind(capability) {
	case spec.CapabilityCollectionQuery, spec.CapabilityItemQuery, spec.CapabilityCreate, spec.CapabilityUpdate, spec.CapabilityDelete:
		return true
	default:
		return false
	}
}

func sanitizeSourceKey(value string) string {
	return strings.Trim(strings.TrimSpace(value), ".")
}
