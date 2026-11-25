package functioncontract

import (
	"strings"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/validation"
)

// ValidateInput validates a payload against a function descriptor's input
// schema. The verdict engine and its unknown-field policy are shared with the
// console execution path (SUI-016), so the same schema and payload yield the
// same verdict on both server entries:
//
//   - a missing schema passes through: there is no contract to enforce and the
//     game server remains the authoritative runtime validator;
//   - a schema that fails to compile, or a payload that is not valid JSON, is
//     rejected instead of silently degrading to "no schema" (fail-closed, same
//     direction as ARCH-027);
//   - contract violations map to a 400 with field-level details.
func ValidateInput(schema map[string]interface{}, payload []byte) error {
	if len(schema) == 0 {
		return nil
	}
	verdict, err := validation.ValidateJSONSchemaVerdict(normalizeRequired(schema), payload)
	if err != nil {
		// A declared contract that cannot be enforced must not be treated as
		// absent: surface the defect instead of passing the payload through.
		return err
	}
	if !verdict.HasViolations() {
		return nil
	}
	details := make(map[string]any, len(verdict.Details))
	for field, reason := range verdict.Details {
		details[field] = reason
	}
	if len(verdict.MissingRequired) > 0 {
		return errorx.NewBadRequestWithDetails(
			"缺少必填参数: "+strings.Join(verdict.MissingRequired, ", "),
			details,
		)
	}
	return errorx.NewBadRequestWithDetails(
		"输入契约校验失败: "+strings.Join(verdict.OtherMessages, "; "),
		details,
	)
}

func normalizeRequired(schema map[string]interface{}) map[string]interface{} {
	normalized := make(map[string]interface{}, len(schema))
	for key, value := range schema {
		normalized[key] = value
	}

	rawRequired, ok := normalized["required"].([]interface{})
	if !ok {
		return normalized
	}
	required := make([]interface{}, 0, len(rawRequired))
	for _, entry := range rawRequired {
		if name, ok := entry.(string); ok && name != "" {
			required = append(required, name)
		}
	}
	normalized["required"] = required
	return normalized
}
