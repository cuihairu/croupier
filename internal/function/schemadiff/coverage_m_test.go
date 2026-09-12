package schemadiff

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 同为 object 结构但 "type" 关键字收窄（object→string）→ breaking finding。
func TestDiffSchemasTypeKeywordChange(t *testing.T) {
	findings := DiffSchemas("inputSchema",
		json.RawMessage(`{"type":"object"}`),
		json.RawMessage(`{"type":"string"}`),
	)
	require.Len(t, findings, 1)
	assert.Equal(t, SeverityBreaking, findings[0].Severity)
	assert.Equal(t, "inputSchema", findings[0].Source)
	assert.Equal(t, "$", findings[0].Path)
	assert.Contains(t, findings[0].Reason, "schema type")
	assert.True(t, HasBreaking(findings))
}

// 根节点同为 array（JSON 层类型一致、非 map 结构）→ 无结构差异可下钻，直接返回。
func TestDiffSchemasNonMapRootNoFindings(t *testing.T) {
	findings := DiffSchemas("outputSchema",
		json.RawMessage(`[1,2,3]`),
		json.RawMessage(`[4,5]`),
	)
	assert.Empty(t, findings)
	assert.False(t, HasBreaking(findings))
}

// sortFindings：breaking 优先，其次按 source、path 字典序稳定排序。
func TestSortFindingsMixedSeverity(t *testing.T) {
	findings := []Finding{
		{Severity: SeverityCompatible, Source: "outputSchema", Path: "/b"},
		{Severity: SeverityBreaking, Source: "outputSchema", Path: "/a"},
		{Severity: SeverityCompatible, Source: "inputSchema", Path: "/a"},
		{Severity: SeverityBreaking, Source: "inputSchema", Path: "/z"},
	}
	sortFindings(findings)
	expected := []Finding{
		{Severity: SeverityBreaking, Source: "inputSchema", Path: "/z"},
		{Severity: SeverityBreaking, Source: "outputSchema", Path: "/a"},
		{Severity: SeverityCompatible, Source: "inputSchema", Path: "/a"},
		{Severity: SeverityCompatible, Source: "outputSchema", Path: "/b"},
	}
	assert.Equal(t, expected, findings)
}
