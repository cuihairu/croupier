package spec

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 能力媒体类型（SUI-007）：执行器可执行集合 = JSON 媒体类型；声明但不
// 可执行的条目（form-urlencoded / multipart）被跳过并记录原因。

func TestIsJSONMediaType(t *testing.T) {
	cases := []struct {
		mediaType string
		want      bool
	}{
		{"application/json", true},
		{"application/json; charset=utf-8", true},
		{"Application/JSON", true},
		{"APPLICATION/JSON; CHARSET=UTF-8", true},
		{"application/vnd.api+json", true},
		{"application/hal+json", true},
		{"application/problem+json; charset=utf-8", true},
		{"application/x-www-form-urlencoded", false},
		{"multipart/form-data", false},
		{"application/xml", false},
		{"text/plain", false},
		{"*/*", false},
		{"", false},
		{"application/jsonp", false}, // 后缀不是 +json
	}
	for _, tc := range cases {
		assert.Equal(t, tc.want, IsJSONMediaType(tc.mediaType), "IsJSONMediaType(%q)", tc.mediaType)
	}
}

func TestIsExecutableRequestBodyMediaType(t *testing.T) {
	assert.True(t, IsExecutableRequestBodyMediaType("application/json"))
	assert.True(t, IsExecutableRequestBodyMediaType("application/vnd.api+json"))
	assert.False(t, IsExecutableRequestBodyMediaType("application/x-www-form-urlencoded"))
	assert.False(t, IsExecutableRequestBodyMediaType("multipart/form-data"))
}

func TestSelectRequestBodyMediaType_PreferenceAndTieBreak(t *testing.T) {
	objSchema := JSONSchema(`{"type":"object"}`)
	arrSchema := JSONSchema(`{"type":"array"}`)

	t.Run("json preferred over suffix", func(t *testing.T) {
		mt, schema, reason := SelectRequestBodyMediaType(map[string]JSONSchema{
			"application/vnd.api+json": objSchema,
			"application/json":         arrSchema,
		})
		require.Empty(t, reason)
		assert.Equal(t, "application/json", mt)
		assert.Equal(t, arrSchema, schema)
	})

	t.Run("suffix selected when no exact json", func(t *testing.T) {
		mt, schema, reason := SelectRequestBodyMediaType(map[string]JSONSchema{
			"application/x-www-form-urlencoded": objSchema,
			"application/vnd.api+json":          arrSchema,
		})
		require.Empty(t, reason)
		assert.Equal(t, "application/vnd.api+json", mt)
		assert.Equal(t, arrSchema, schema)
	})

	t.Run("suffix tie-break lexicographic", func(t *testing.T) {
		mt, _, reason := SelectRequestBodyMediaType(map[string]JSONSchema{
			"application/hal+json":     objSchema,
			"application/vnd.api+json": arrSchema,
		})
		require.Empty(t, reason)
		// application/hal+json < application/vnd.api+json
		assert.Equal(t, "application/hal+json", mt)
	})

	t.Run("form only is not executable", func(t *testing.T) {
		_, _, reason := SelectRequestBodyMediaType(map[string]JSONSchema{
			"application/x-www-form-urlencoded": objSchema,
		})
		require.Contains(t, reason, "no executable request body media type")
		require.Contains(t, reason, "application/x-www-form-urlencoded")
	})

	t.Run("empty content reports declared-nothing", func(t *testing.T) {
		_, _, reason := SelectRequestBodyMediaType(map[string]JSONSchema{})
		require.Equal(t, "no request body media types declared", reason)
	})
}

func TestSelectRequestBodyMediaType_Deterministic(t *testing.T) {
	content := map[string]JSONSchema{
		"application/xml":                   JSONSchema(`{"type":"string"}`),
		"application/x-www-form-urlencoded": JSONSchema(`{"type":"object"}`),
		"application/hal+json":              JSONSchema(`{"type":"object"}`),
		"application/vnd.api+json":          JSONSchema(`{"type":"array"}`),
		"multipart/form-data":               JSONSchema(`{"type":"object"}`),
		"application/json":                  JSONSchema(`{"type":"object","properties":{"a":{"type":"string"}}}`),
	}
	first, firstSchema, firstReason := SelectRequestBodyMediaType(content)
	require.Empty(t, firstReason)
	// 同一输入重复 100 次恒选同一媒体类型与 schema（重复导入结果一致）。
	for i := 0; i < 100; i++ {
		mt, schema, reason := SelectRequestBodyMediaType(content)
		require.Empty(t, reason)
		if mt != first || string(schema) != string(firstSchema) {
			t.Fatalf("iteration %d: %q/%q != %q/%q", i, mt, string(schema), first, string(firstSchema))
		}
	}
	assert.Equal(t, "application/json", first)
}

func TestSelectExecutableMediaType_ConsumesOrder(t *testing.T) {
	// 声明顺序保留：首个可执行条目获胜。
	mt, reason := SelectExecutableMediaType([]string{"application/xml", "application/json"})
	require.Empty(t, reason)
	assert.Equal(t, "application/json", mt)

	mt, reason = SelectExecutableMediaType([]string{"application/vnd.api+json", "application/xml"})
	require.Empty(t, reason)
	assert.Equal(t, "application/vnd.api+json", mt)

	// 无一可执行 → reason。
	_, reason = SelectExecutableMediaType([]string{"application/xml", "text/plain"})
	require.Contains(t, reason, "no executable request body media type")

	// 空列表 → 调用方落缺省。
	mt, reason = SelectExecutableMediaType(nil)
	require.Empty(t, reason)
	require.Empty(t, mt)
}

func TestRequestBodyMediaDiagnostic(t *testing.T) {
	d := RequestBodyMediaDiagnostic("no executable request body media type; declared [application/x-www-form-urlencoded]")
	assert.Equal(t, DiagRequestBodyNoExecutableMedia, d.Code)
	assert.Equal(t, SeverityWarning, d.Severity)
	assert.Equal(t, "body", d.Field)
	assert.True(t, strings.Contains(d.Message, "application/x-www-form-urlencoded"))
}
