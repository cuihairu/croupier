package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"

	"github.com/cuihairu/croupier/internal/dbenum"
	"github.com/cuihairu/croupier/internal/function/schemadiff"
	"github.com/cuihairu/croupier/internal/model"
)

// B2 写路径纯函数：contractScalarDiff / schemaDiffEntries / canonicalEqualJSON
// 直接决定「是否写新契约版本」与 diff 条目形态，此前仅经 Lifecycle 端到端
// 间接覆盖 risk/schema 两字段。

func TestContractScalarDiff_AllScalarFields(t *testing.T) {
	existing := &model.FunctionContract{
		Version:        "1.0.0",
		Enabled:        true,
		Deprecated:     false,
		ResourceKey:    "player",
		OperationKey:   "ban",
		Capability:     dbenum.CapabilityAction,
		Execution:      "sync",
		ExecutionState: "bound",
		TimeoutMs:      1000,
		Risk:           dbenum.RiskSafe,
		Permission:     "player:ban",
		Approval:       datatypes.JSONMap{"required": false},
		Summary:        datatypes.JSONMap{"zh-CN": "封禁"},
		Description:    datatypes.JSONMap{},
		Tags:           model.JSON(`["a"]`),
	}
	updated := *existing
	updated.Version = "1.1.0"
	updated.Enabled = false
	updated.Deprecated = true
	updated.ResourceKey = "mail"
	updated.OperationKey = "send"
	updated.Capability = dbenum.CapabilityCreate
	updated.Execution = "task"
	updated.ExecutionState = "unbound"
	updated.TimeoutMs = 2000
	updated.Risk = dbenum.RiskDanger
	updated.Permission = "mail:send"
	updated.Approval = datatypes.JSONMap{"required": true}
	updated.Summary = datatypes.JSONMap{"zh-CN": "发送"}
	updated.Description = datatypes.JSONMap{"zh-CN": "说明"}
	updated.Tags = model.JSON(`["b"]`)

	entries := contractScalarDiff(existing, &updated)
	fields := map[string]contractFieldChange{}
	for _, e := range entries {
		fields[e.Field] = e
	}
	for _, want := range []string{
		"version", "enabled", "deprecated", "resourceKey", "operationKey",
		"capability", "execution", "executionState", "timeoutMs", "risk",
		"permission", "approval", "summary", "description", "tags",
	} {
		require.Contains(t, fields, want, "全部标量字段变更必须各出一条 entry")
	}
	assert.Equal(t, "1.0.0", fields["version"].From)
	assert.Equal(t, "1.1.0", fields["version"].To)
	assert.Equal(t, "true", fields["enabled"].From)
	assert.Equal(t, "false", fields["enabled"].To)
	assert.Equal(t, "false", fields["deprecated"].From)
	assert.Equal(t, "true", fields["deprecated"].To)
	assert.Equal(t, "1000", fields["timeoutMs"].From)
	assert.Equal(t, "2000", fields["timeoutMs"].To)
	assert.Equal(t, "safe", fields["risk"].From)
	assert.Equal(t, "danger", fields["risk"].To)
}

func TestContractScalarDiff_NoChangeAndNilExisting(t *testing.T) {
	assert.Nil(t, contractScalarDiff(nil, &model.FunctionContract{Version: "1.0.0"}))

	base := &model.FunctionContract{Version: "1.0.0", Enabled: true, ResourceKey: "player"}
	same := *base
	assert.Empty(t, contractScalarDiff(base, &same), "无变更 → 空 diff")
}

func TestSchemaDiffEntries_MapsFindingsBySource(t *testing.T) {
	findings := []schemadiff.Finding{
		{Severity: schemadiff.SeverityBreaking, Source: "inputSchema", Path: "/playerId", Reason: "required removed"},
		{Severity: schemadiff.SeverityCompatible, Source: "outputSchema", Path: "/ok", Reason: "added"},
	}
	entries := schemaDiffEntries(findings, true, false)
	require.Len(t, entries, 1)
	assert.Equal(t, "inputSchema", entries[0].Field)
	assert.Equal(t, "schema_replaced", entries[0].Change)
	require.Len(t, entries[0].Findings, 1)
	assert.Equal(t, "inputSchema", entries[0].Findings[0].Source)

	assert.Empty(t, schemaDiffEntries(findings, false, false), "两侧 schema 均未变 → 空")
}

func TestCanonicalEqualJSON_KeyOrderAndWhitespace(t *testing.T) {
	a := model.JSON(`{"a":1,"b":2}`)
	b := model.JSON(` { "b" : 2 , "a" : 1 } `)
	assert.True(t, canonicalEqualJSON(a, b), "键序/空白无关的语义相等")

	c := model.JSON(`{"a":1}`)
	assert.False(t, canonicalEqualJSON(a, c))

	// 非法 JSON 按原文比较：不同垃圾不等，相同垃圾相等。
	assert.False(t, canonicalEqualJSON(model.JSON(`{bad`), model.JSON(`{worse`)))
	assert.True(t, canonicalEqualJSON(model.JSON(`{bad`), model.JSON(`{bad`)))
}

func TestJSONHelpers_BoolIntCompact(t *testing.T) {
	assert.Equal(t, "true", jsonBool(true))
	assert.Equal(t, "false", jsonBool(false))
	assert.Equal(t, "587", jsonInt(587))
	assert.Equal(t, `{"required":true}`, jsonCompact(map[string]bool{"required": true}))
}
