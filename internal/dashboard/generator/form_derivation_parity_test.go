package generator

// SUI-025 共享预期结果：与 web/src/utils/__tests__/form-presentation-parity.test.ts
// 读取同一 fixture（testdata/form_presentation_parity.json），断言 Go 生成器与
// Web 调试页默认表单派生一致（顶层字段顺序、控件、分组、layout hints）。

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
)

// deriveFormFieldsForTest 取有 hints 场景的字段列表（无 hints 时 fields 为 nil，
// 由 parity fixture 的 no-fields case 直接断言）。
func deriveFormFieldsForTest(t *testing.T, schema spec.JSONSchema) []spec.FormFieldSpec {
	t.Helper()
	derived := deriveFormPresentation(schema)
	require.True(t, derived.hasHints, "schema must carry at least one hint for this test")
	return derived.fields
}

func mustMarshalJSON(t *testing.T, value any) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(value)
	require.NoError(t, err)
	return raw
}

func TestFormPresentationParityFixture(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "form_presentation_parity.json"))
	require.NoError(t, err)

	var fixture struct {
		Cases []struct {
			Name     string          `json:"name"`
			Schema   json.RawMessage `json:"schema"`
			Expected json.RawMessage `json:"expected"`
		} `json:"cases"`
	}
	require.NoError(t, json.Unmarshal(raw, &fixture))
	require.NotEmpty(t, fixture.Cases)

	for _, testCase := range fixture.Cases {
		testCase := testCase
		t.Run(testCase.Name, func(t *testing.T) {
			derived := deriveFormPresentation(spec.JSONSchema(testCase.Schema))
			actual := map[string]json.RawMessage{"layout": mustMarshalJSON(t, string(derived.layout))}
			if derived.hasHints {
				actual["fields"] = mustMarshalJSON(t, derived.fields)
				if len(derived.groups) > 0 {
					actual["groups"] = mustMarshalJSON(t, derived.groups)
				}
			}
			var expectedAny, actualAny any
			require.NoError(t, json.Unmarshal(testCase.Expected, &expectedAny))
			require.NoError(t, json.Unmarshal(mustMarshalJSON(t, actual), &actualAny))
			if !reflect.DeepEqual(expectedAny, actualAny) {
				expectedBytes, _ := json.Marshal(expectedAny)
				actualBytes, _ := json.Marshal(actualAny)
				t.Fatalf("parity mismatch\nexpected: %s\nactual: %s", expectedBytes, actualBytes)
			}
		})
	}
}
