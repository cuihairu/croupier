package agent

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTruncateString(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		max      int
		expected string
	}{
		{
			name:     "empty string",
			s:        "",
			max:      10,
			expected: "",
		},
		{
			name:     "string shorter than max",
			s:        "hello",
			max:      10,
			expected: "hello",
		},
		{
			name:     "string equal to max",
			s:        "hello",
			max:      5,
			expected: "hello",
		},
		{
			name:     "string longer than max",
			s:        "hello world",
			max:      8,
			expected: "hello...",
		},
		{
			name:     "max is zero",
			s:        "hello",
			max:      0,
			expected: "",
		},
		{
			name:     "max is negative",
			s:        "hello",
			max:      -1,
			expected: "",
		},
		{
			name:     "max is 3 with long string",
			s:        "hello",
			max:      3,
			expected: "hel",
		},
		{
			name:     "string with spaces trimmed",
			s:        "  hello  ",
			max:      10,
			expected: "hello",
		},
		{
			name:     "string with spaces and truncation",
			s:        "  hello world  ",
			max:      8,
			expected: "hello...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateString(tt.s, tt.max)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestValueString(t *testing.T) {
	tests := []struct {
		name     string
		m        map[string]any
		key      string
		expected string
	}{
		{
			name:     "nil map",
			m:        nil,
			key:      "key",
			expected: "",
		},
		{
			name:     "empty map",
			m:        map[string]any{},
			key:      "key",
			expected: "",
		},
		{
			name:     "key not found",
			m:        map[string]any{"other": "value"},
			key:      "key",
			expected: "",
		},
		{
			name:     "key found with string value",
			m:        map[string]any{"key": "hello"},
			key:      "key",
			expected: "hello",
		},
		{
			name:     "key found with int value",
			m:        map[string]any{"key": 42},
			key:      "key",
			expected: "42",
		},
		{
			name:     "key found with nil value",
			m:        map[string]any{"key": nil},
			key:      "key",
			expected: "",
		},
		{
			name:     "key found with spaces",
			m:        map[string]any{"key": "  hello  "},
			key:      "key",
			expected: "hello",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := valueString(tt.m, tt.key)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestValueStringSlice(t *testing.T) {
	tests := []struct {
		name     string
		m        map[string]any
		key      string
		expected []string
	}{
		{
			name:     "nil map",
			m:        nil,
			key:      "key",
			expected: nil,
		},
		{
			name:     "empty map",
			m:        map[string]any{},
			key:      "key",
			expected: nil,
		},
		{
			name:     "key not found",
			m:        map[string]any{"other": "value"},
			key:      "key",
			expected: nil,
		},
		{
			name:     "key found with nil value",
			m:        map[string]any{"key": nil},
			key:      "key",
			expected: nil,
		},
		{
			name:     "key found with non-slice value",
			m:        map[string]any{"key": "string"},
			key:      "key",
			expected: nil,
		},
		{
			name:     "key found with empty slice",
			m:        map[string]any{"key": []any{}},
			key:      "key",
			expected: []string{},
		},
		{
			name:     "key found with string slice",
			m:        map[string]any{"key": []any{"a", "b", "c"}},
			key:      "key",
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "key found with mixed types",
			m:        map[string]any{"key": []any{"a", 42, true}},
			key:      "key",
			expected: []string{"a", "42", "true"},
		},
		{
			name:     "key found with nil items filtered",
			m:        map[string]any{"key": []any{"a", nil, "b"}},
			key:      "key",
			expected: []string{"a", "b"},
		},
		{
			name:     "key found with empty strings filtered",
			m:        map[string]any{"key": []any{"a", "", "  ", "b"}},
			key:      "key",
			expected: []string{"a", "b"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := valueStringSlice(tt.m, tt.key)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSanitizeNodeKey(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		expected string
	}{
		{
			name:     "empty string",
			s:        "",
			expected: "",
		},
		{
			name:     "simple string",
			s:        "hello",
			expected: "hello",
		},
		{
			name:     "string with spaces",
			s:        "hello world",
			expected: "hello_world",
		},
		{
			name:     "string with slashes",
			s:        "hello/world",
			expected: "hello_world",
		},
		{
			name:     "string with special chars",
			s:        "hello@world!",
			expected: "helloworld",
		},
		{
			name:     "string with valid chars",
			s:        "hello-world_123.test",
			expected: "hello-world_123.test",
		},
		{
			name:     "string with leading/trailing spaces",
			s:        "  hello  ",
			expected: "hello",
		},
		{
			name:     "string with leading/trailing dots",
			s:        ".hello.",
			expected: "hello",
		},
		{
			name:     "string with leading/trailing dashes",
			s:        "-hello-",
			expected: "hello",
		},
		{
			name:     "string with leading/trailing underscores",
			s:        "_hello_",
			expected: "hello",
		},
		{
			name:     "uppercase to lowercase",
			s:        "Hello World",
			expected: "hello_world",
		},
		{
			name:     "mixed case with special chars",
			s:        "Hello World/Test!",
			expected: "hello_world_test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeNodeKey(tt.s)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDefaultAgentMenuPath(t *testing.T) {
	tests := []struct {
		name     string
		fid      string
		entity   string
		expected string
	}{
		{
			name:     "with entity",
			fid:      "player.getList",
			entity:   "player",
			expected: "/game/entities/player",
		},
		{
			name:     "empty entity",
			fid:      "player.getList",
			entity:   "",
			expected: "/game/functions/invoke?fid=player.getList",
		},
		{
			name:     "entity with spaces",
			fid:      "player.getList",
			entity:   "Player Entity",
			expected: "/game/entities/player_entity",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := defaultAgentMenuPath(tt.fid, tt.entity)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestOperationToVerbs(t *testing.T) {
	tests := []struct {
		name      string
		operation string
		expected  []string
	}{
		{
			name:      "create",
			operation: "create",
			expected:  []string{"create", "write"},
		},
		{
			name:      "read",
			operation: "read",
			expected:  []string{"read", "view"},
		},
		{
			name:      "get",
			operation: "get",
			expected:  []string{"read", "view"},
		},
		{
			name:      "list",
			operation: "list",
			expected:  []string{"read", "view"},
		},
		{
			name:      "update",
			operation: "update",
			expected:  []string{"update", "edit", "write"},
		},
		{
			name:      "edit",
			operation: "edit",
			expected:  []string{"update", "edit", "write"},
		},
		{
			name:      "delete",
			operation: "delete",
			expected:  []string{"delete", "remove"},
		},
		{
			name:      "remove",
			operation: "remove",
			expected:  []string{"delete", "remove"},
		},
		{
			name:      "invoke",
			operation: "invoke",
			expected:  []string{"invoke", "execute"},
		},
		{
			name:      "execute",
			operation: "execute",
			expected:  []string{"invoke", "execute"},
		},
		{
			name:      "custom",
			operation: "custom",
			expected:  []string{"invoke"},
		},
		{
			name:      "unknown",
			operation: "unknown",
			expected:  []string{"invoke"},
		},
		{
			name:      "empty",
			operation: "",
			expected:  []string{"invoke"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := operationToVerbs(tt.operation)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestInferFunctionResource(t *testing.T) {
	tests := []struct {
		name       string
		functionID string
		expected   string
	}{
		{
			name:       "with dot",
			functionID: "player.getList",
			expected:   "player",
		},
		{
			name:       "no dot",
			functionID: "getPlayer",
			expected:   "getPlayer",
		},
		{
			name:       "multiple dots",
			functionID: "game.player.getList",
			expected:   "game",
		},
		{
			name:       "empty string",
			functionID: "",
			expected:   "",
		},
		{
			name:       "dot at start",
			functionID: ".getList",
			expected:   ".getList",
		},
		{
			name:       "spaces trimmed",
			functionID: "  player.getList  ",
			expected:   "player",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := inferFunctionResource(tt.functionID)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestInferFunctionOperation(t *testing.T) {
	tests := []struct {
		name       string
		functionID string
		expected   string
	}{
		{
			name:       "with dot",
			functionID: "player.getList",
			expected:   "getList",
		},
		{
			name:       "no dot",
			functionID: "getPlayer",
			expected:   "",
		},
		{
			name:       "multiple dots",
			functionID: "game.player.getList",
			expected:   "player.getList",
		},
		{
			name:       "empty string",
			functionID: "",
			expected:   "",
		},
		{
			name:       "dot at end",
			functionID: "player.",
			expected:   "",
		},
		{
			name:       "spaces trimmed",
			functionID: "  player.getList  ",
			expected:   "getList",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := inferFunctionOperation(tt.functionID)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCloneLabels(t *testing.T) {
	tests := []struct {
		name     string
		src      map[string]string
		expected map[string]string
	}{
		{
			name:     "nil map",
			src:      nil,
			expected: nil,
		},
		{
			name:     "empty map",
			src:      map[string]string{},
			expected: nil,
		},
		{
			name:     "single entry",
			src:      map[string]string{"key": "value"},
			expected: map[string]string{"key": "value"},
		},
		{
			name:     "multiple entries",
			src:      map[string]string{"key1": "value1", "key2": "value2"},
			expected: map[string]string{"key1": "value1", "key2": "value2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cloneLabels(tt.src)
			assert.Equal(t, tt.expected, result)
			// Verify it's a deep copy (not the same map)
			if tt.src != nil && len(tt.src) > 0 {
				result["new_key"] = "new_value"
				assert.NotEqual(t, tt.src, result)
			}
		})
	}
}
