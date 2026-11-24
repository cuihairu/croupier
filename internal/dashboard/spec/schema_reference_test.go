package spec

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// SUI-008 验收：无法内联的 schema 引用显式进入待审，取代旧「缺失/循环引用
// 静默坍缩为匿名对象、外部引用保留但无信号」——诊断携带指针原文与边界原因。
func TestSchemaReferenceUnresolvedDiagnostic(t *testing.T) {
	d := SchemaReferenceUnresolvedDiagnostic("#/components/schemas/Missing", "missing target in the document")
	assert.Equal(t, DiagSchemaReferenceUnresolved, d.Code)
	assert.Equal(t, SeverityWarning, d.Severity)
	assert.Equal(t, "#/components/schemas/Missing", d.Field)
	assert.True(t, strings.Contains(d.Message, "#/components/schemas/Missing"))
	assert.True(t, strings.Contains(d.Message, "missing target in the document"))
}
