package versioning

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/stretchr/testify/assert"
)

// --- normalizeLocalizedText tests ---

func TestNormalizeLocalizedText(t *testing.T) {
	tests := []struct {
		name  string
		input map[string]string
		want  map[string]string
	}{
		{
			name:  "nil input",
			input: nil,
			want:  nil,
		},
		{
			name:  "empty input",
			input: map[string]string{},
			want:  nil,
		},
		{
			name:  "zh-CN key",
			input: map[string]string{"zh-CN": "玩家"},
			want:  map[string]string{"zh-CN": "玩家"},
		},
		{
			name:  "zh key normalized",
			input: map[string]string{"zh": "玩家"},
			want:  map[string]string{"zh-CN": "玩家"},
		},
		{
			name:  "en key normalized",
			input: map[string]string{"en": "Player"},
			want:  map[string]string{"en-US": "Player"},
		},
		{
			name:  "underscore normalized",
			input: map[string]string{"zh_cn": "玩家"},
			want:  map[string]string{"zh-CN": "玩家"},
		},
		{
			name:  "empty value filtered",
			input: map[string]string{"zh-CN": "  "},
			want:  nil,
		},
		{
			name:  "mixed keys",
			input: map[string]string{"zh-CN": "玩家", "en-US": "Player", "custom": "value"},
			want:  map[string]string{"zh-CN": "玩家", "en-US": "Player", "custom": "value"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeLocalizedText(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

// --- actorFromContext tests ---

func TestActorFromContext(t *testing.T) {
	tests := []struct {
		name     string
		username string
		want     string
	}{
		{
			name:     "with username",
			username: "admin",
			want:     "admin",
		},
		{
			name:     "empty username",
			username: "",
			want:     "system",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			if tt.username != "" {
				ctx = context.WithValue(ctx, "username", tt.username)
			}
			got := actorFromContext(ctx)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestActorFromContextV2(t *testing.T) {
	// Test with actual CurrentUsername implementation
	ctx := context.Background()
	// Without username in context, should return "system"
	got := actorFromContext(ctx)
	assert.Equal(t, "system", got)
}

// --- proposalKeyForPage tests ---

func TestProposalKeyForPageV2(t *testing.T) {
	tests := []struct {
		name       string
		pageType   spec.PageType
		functionID string
		want       string
	}{
		{
			name:       "task page",
			pageType:   spec.PageTypeTask,
			functionID: "player.ban",
			want:       "task:player.ban",
		},
		{
			name:       "report page",
			pageType:   spec.PageTypeReport,
			functionID: "analytics.retention",
			want:       "report:analytics.retention",
		},
		{
			name:       "resource page",
			pageType:   spec.PageTypeResource,
			functionID: "player.list",
			want:       "operation:player.list",
		},
		{
			name:       "operation page",
			pageType:   spec.PageTypeOperation,
			functionID: "player.ban",
			want:       "operation:player.ban",
		},
		{
			name:       "empty function ID",
			pageType:   spec.PageTypeTask,
			functionID: "",
			want:       "",
		},
		{
			name:       "whitespace function ID",
			pageType:   spec.PageTypeTask,
			functionID: "  ",
			want:       "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := proposalKeyForPage(tt.pageType, tt.functionID)
			assert.Equal(t, tt.want, got)
		})
	}
}

// --- firstNonEmpty tests ---

func TestFirstNonEmptyV5(t *testing.T) {
	tests := []struct {
		name   string
		values []string
		want   string
	}{
		{
			name:   "first non-empty",
			values: []string{"", "second", "third"},
			want:   "second",
		},
		{
			name:   "all empty",
			values: []string{"", "", ""},
			want:   "",
		},
		{
			name:   "whitespace only",
			values: []string{"  ", "\t", "\n"},
			want:   "",
		},
		{
			name:   "first with spaces",
			values: []string{"  hello  ", "world"},
			want:   "hello",
		},
		{
			name:   "single value",
			values: []string{"single"},
			want:   "single",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := firstNonEmpty(tt.values...)
			assert.Equal(t, tt.want, got)
		})
	}
}

// --- jsonValue tests ---

func TestJsonValueV2(t *testing.T) {
	tests := []struct {
		name  string
		value interface{}
		want  string
	}{
		{
			name:  "string",
			value: "hello",
			want:  `"hello"`,
		},
		{
			name:  "number",
			value: 42,
			want:  `42`,
		},
		{
			name:  "boolean",
			value: true,
			want:  `true`,
		},
		{
			name:  "nil",
			value: nil,
			want:  `null`,
		},
		{
			name:  "slice",
			value: []int{1, 2, 3},
			want:  `[1,2,3]`,
		},
		{
			name:  "map",
			value: map[string]int{"a": 1},
			want:  `{"a":1}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := jsonValue(tt.value)
			assert.Equal(t, tt.want, string(got))
		})
	}
}

// --- computeDigest tests ---

func TestComputeDigestV4(t *testing.T) {
	tests := []struct {
		name  string
		value interface{}
	}{
		{
			name:  "string",
			value: "hello",
		},
		{
			name:  "number",
			value: 42,
		},
		{
			name:  "nil",
			value: nil,
		},
		{
			name:  "slice",
			value: []int{1, 2, 3},
		},
		{
			name:  "map",
			value: map[string]int{"a": 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := computeDigest(tt.value)
			assert.NotEmpty(t, got)
			assert.Len(t, got, 64) // SHA256 hex string length
		})
	}
}

// --- jsonString tests ---

func TestJsonStringV3(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{
			name:  "simple string",
			value: "hello",
			want:  `"hello"`,
		},
		{
			name:  "empty string",
			value: "",
			want:  `""`,
		},
		{
			name:  "string with spaces",
			value: "hello world",
			want:  `"hello world"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := jsonString(tt.value)
			assert.Equal(t, tt.want, string(got))
		})
	}
}

// --- jsonNumber tests ---

func TestJsonNumberV3(t *testing.T) {
	tests := []struct {
		name  string
		value uint
		want  string
	}{
		{
			name:  "zero",
			value: 0,
			want:  `0`,
		},
		{
			name:  "positive",
			value: 42,
			want:  `42`,
		},
		{
			name:  "large number",
			value: 1234567890,
			want:  `1234567890`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := jsonNumber(tt.value)
			assert.Equal(t, tt.want, string(got))
		})
	}
}

// --- localizedTextToJSONMap tests ---

func TestLocalizedTextToJSONMapV2(t *testing.T) {
	tests := []struct {
		name  string
		input spec.LocalizedText
		want  map[string]string
	}{
		{
			name:  "nil input",
			input: nil,
			want:  map[string]string{},
		},
		{
			name:  "empty input",
			input: spec.LocalizedText{},
			want:  map[string]string{},
		},
		{
			name:  "valid input",
			input: spec.LocalizedText{"zh-CN": "玩家", "en-US": "Player"},
			want:  map[string]string{"zh-CN": "玩家", "en-US": "Player"},
		},
		{
			name:  "whitespace filtered",
			input: spec.LocalizedText{"zh-CN": "  "},
			want:  map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := localizedTextToJSONMap(tt.input)
			gotMap := make(map[string]string)
			for k, v := range got {
				if str, ok := v.(string); ok {
					gotMap[k] = str
				}
			}
			assert.Equal(t, tt.want, gotMap)
		})
	}
}

// --- Test for logicutils.CurrentUsername ---

func TestCurrentUsername(t *testing.T) {
	tests := []struct {
		name     string
		username interface{}
		want     string
		wantErr  bool
	}{
		{
			name:     "with username",
			username: "admin",
			want:     "admin",
			wantErr:  false,
		},
		{
			name:     "without username",
			username: nil,
			want:     "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			if tt.username != nil {
				ctx = context.WithValue(ctx, "username", tt.username)
			}
			got, err := utils.CurrentUsername(ctx)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}
