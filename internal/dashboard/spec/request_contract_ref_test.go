package spec

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// 独立契约的未内联 schema 引用（SUI-008）：诊断字段 = 引用指针原文，
// message 命名阻断内联的边界，绝不把坍缩后的匿名对象当成真 schema。
func TestSchemaReferenceUnresolvedDiagnostic(t *testing.T) {
	d := SchemaReferenceUnresolvedDiagnostic("#/components/schemas/Missing", "missing target in the document")
	assert.Equal(t, DiagSchemaReferenceUnresolved, d.Code)
	assert.Equal(t, SeverityWarning, d.Severity)
	assert.Equal(t, "#/components/schemas/Missing", d.Field)
	assert.True(t, strings.Contains(d.Message, "#/components/schemas/Missing"))
	assert.True(t, strings.Contains(d.Message, "missing target"))
	assert.True(t, strings.Contains(d.Message, "pending review"))

	external := SchemaReferenceUnresolvedDiagnostic("other.yaml#/Thing", "reference outside the document")
	assert.Equal(t, "other.yaml#/Thing", external.Field)
	assert.True(t, strings.Contains(external.Message, "outside the document"))
}
