package normalizer

import (
	"encoding/json"
	"testing"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
)

func TestNormalizeCarriesCapabilityAndExecution(t *testing.T) {
	result := Normalize(DescriptorInput{
		ID:                "reward.batch_grant",
		Version:           "1.0.0",
		Resource:          "reward",
		Operation:         "batch_grant",
		Capability:        "task",
		InputSchema:       `{"type":"object"}`,
		Execution:         "task",
		ApprovalRequired:  true,
		ApprovalPolicyKey: "two_person",
		Enabled:           true,
	})

	if result.Function.Capability != spec.CapabilityTask {
		t.Fatalf("expected function capability task, got %q", result.Function.Capability)
	}
	if result.Function.Execution != spec.FunctionExecutionTask {
		t.Fatalf("expected function execution task, got %q", result.Function.Execution)
	}
	if !result.Function.Approval.Required || result.Function.Approval.PolicyKey != "two_person" {
		t.Fatalf("expected approval policy to be carried, got %#v", result.Function.Approval)
	}
	if result.Operation == nil {
		t.Fatal("expected operation")
	}
	if result.Operation.Capability != spec.CapabilityTask {
		t.Fatalf("expected operation capability task, got %q", result.Operation.Capability)
	}
	if result.Operation.Execution != spec.FunctionExecutionTask {
		t.Fatalf("expected operation execution task, got %q", result.Operation.Execution)
	}
	if !result.Operation.Approval.Required || result.Operation.Approval.PolicyKey != "two_person" {
		t.Fatalf("expected operation approval policy to be carried, got %#v", result.Operation.Approval)
	}
}

func TestNormalizeDefaultsExecutionFromCapability(t *testing.T) {
	result := Normalize(DescriptorInput{
		ID:          "reward.batch_grant",
		Capability:  "task",
		InputSchema: `{"type":"object"}`,
		Enabled:     true,
	})

	if result.Function.Execution != spec.FunctionExecutionTask {
		t.Fatalf("expected task execution default for task capability, got %q", result.Function.Execution)
	}
}

func TestNormalizeRejectsInvalidCapabilityAndExecution(t *testing.T) {
	result := Normalize(DescriptorInput{
		ID:          "player.ban",
		Capability:  "row_button",
		Execution:   "modal",
		InputSchema: `{"type":"object"}`,
		Enabled:     true,
	})

	assertDiagnostic(t, result.Diagnostics, "capability_invalid")
	assertDiagnostic(t, result.Diagnostics, "execution_invalid")
}

func TestNormalizeRejectsUnstableKeys(t *testing.T) {
	result := Normalize(DescriptorInput{
		ID:          "Reward.BatchGrant",
		Resource:    "reward",
		Operation:   "batchGrant",
		Capability:  "task",
		InputSchema: `{"type":"object"}`,
		Enabled:     true,
	})

	assertDiagnostic(t, result.Diagnostics, "function_id_invalid")
	assertDiagnostic(t, result.Diagnostics, "operation_key_invalid")
}

func TestNormalizeBatchEmpty(t *testing.T) {
	results, resources := NormalizeBatch([]DescriptorInput{})
	if len(results) != 0 {
		t.Fatalf("expected 0 results, got %d", len(results))
	}
	if len(resources) != 0 {
		t.Fatalf("expected 0 resources, got %d", len(resources))
	}
}

func TestNormalizeBatchMergesSameResource(t *testing.T) {
	inputs := []DescriptorInput{
		{
			ID:          "player.ban",
			Resource:    "player",
			Operation:   "ban",
			Capability:  "action",
			InputSchema: `{"type":"object"}`,
			Enabled:     true,
		},
		{
			ID:          "player.unban",
			Resource:    "player",
			Operation:   "unban",
			Capability:  "action",
			InputSchema: `{"type":"object"}`,
			Enabled:     true,
		},
	}
	results, resources := NormalizeBatch(inputs)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}
	playerResource, ok := resources["player"]
	if !ok {
		t.Fatal("expected player resource")
	}
	if len(playerResource.Operations) != 2 {
		t.Fatalf("expected 2 operations, got %d", len(playerResource.Operations))
	}
}

func TestNormalizeLocaleKeys(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]string
		expected map[string]string
	}{
		{
			name:     "empty",
			input:    map[string]string{},
			expected: nil,
		},
		{
			name:     "nil",
			input:    nil,
			expected: nil,
		},
		{
			name:     "zh to zh-CN",
			input:    map[string]string{"zh": "测试"},
			expected: map[string]string{"zh-CN": "测试"},
		},
		{
			name:     "en to en-US",
			input:    map[string]string{"en": "Test"},
			expected: map[string]string{"en-US": "Test"},
		},
		{
			name:     "zh-cn to zh-CN",
			input:    map[string]string{"zh-cn": "测试"},
			expected: map[string]string{"zh-CN": "测试"},
		},
		{
			name:     "en_us to en-US",
			input:    map[string]string{"en_us": "Test"},
			expected: map[string]string{"en-US": "Test"},
		},
		{
			name:     "keep custom locale",
			input:    map[string]string{"ja": "テスト"},
			expected: map[string]string{"ja": "テスト"},
		},
		{
			name:     "trim whitespace",
			input:    map[string]string{"zh": "  测试  "},
			expected: map[string]string{"zh-CN": "测试"},
		},
		{
			name:     "skip empty values",
			input:    map[string]string{"zh": "", "en": "Test"},
			expected: map[string]string{"en-US": "Test"},
		},
		{
			name:     "all empty",
			input:    map[string]string{"zh": "", "en": ""},
			expected: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeLocaleKeys(tt.input)
			if len(result) != len(tt.expected) {
				t.Fatalf("expected %v, got %v", tt.expected, result)
			}
			for k, v := range tt.expected {
				if result[k] != v {
					t.Fatalf("expected %s=%s, got %s=%s", k, v, k, result[k])
				}
			}
		})
	}
}

func TestTrackUint(t *testing.T) {
	tracker := NewSemanticProvenanceTracker()

	// First track
	if !tracker.TrackUint("count", 42, spec.SemanticSourceSDKExplicit, "digest1", "user1") {
		t.Fatal("expected true for first track")
	}

	// Same value
	if !tracker.TrackUint("count", 42, spec.SemanticSourceSDKExplicit, "digest2", "user1") {
		t.Fatal("expected true for same value")
	}

	// Higher priority source
	if !tracker.TrackUint("count", 100, spec.SemanticSourcePlatformReview, "digest3", "user2") {
		t.Fatal("expected true for higher priority")
	}

	// Lower priority source
	if tracker.TrackUint("count", 200, spec.SemanticSourceOpenAPIRest, "digest4", "user3") {
		t.Fatal("expected false for lower priority")
	}
}

func TestAddConflict(t *testing.T) {
	tracker := NewSemanticProvenanceTracker()

	// Add first conflict
	tracker.addConflict("field1", spec.SemanticSourceSDKExplicit, []byte(`"value1"`), spec.SemanticSourceOpenAPIRest, []byte(`"value2"`))
	if len(tracker.conflicts) != 1 {
		t.Fatalf("expected 1 conflict, got %d", len(tracker.conflicts))
	}

	// Add second conflict for same field
	tracker.addConflict("field1", spec.SemanticSourceSDKExplicit, []byte(`"value1"`), spec.SemanticSourcePlatformReview, []byte(`"value3"`))
	if len(tracker.conflicts) != 1 {
		t.Fatalf("expected 1 conflict (updated), got %d", len(tracker.conflicts))
	}
	if len(tracker.conflicts[0].Values) != 3 {
		t.Fatalf("expected 3 values, got %d", len(tracker.conflicts[0].Values))
	}

	// Add conflict for different field
	tracker.addConflict("field2", spec.SemanticSourceSDKExplicit, []byte(`"a"`), spec.SemanticSourceOpenAPIRest, []byte(`"b"`))
	if len(tracker.conflicts) != 2 {
		t.Fatalf("expected 2 conflicts, got %d", len(tracker.conflicts))
	}
}

func TestCanonicalJSON(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:  "object",
			input: `{"b":2,"a":1}`,
		},
		{
			name:  "array",
			input: `[1,2,3]`,
		},
		{
			name:  "nested",
			input: `{"b":{"d":4},"a":{"c":3}}`,
		},
		{
			name:    "invalid object",
			input:   `{invalid}`,
			wantErr: true,
		},
		{
			name:    "invalid array",
			input:   `[invalid`,
			wantErr: true,
		},
		{
			name:  "empty",
			input: ``,
		},
		{
			name:  "whitespace only",
			input: `   `,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := canonicalJSON(json.RawMessage(tt.input))
			if (err != nil) != tt.wantErr {
				t.Fatalf("expected error=%v, got %v", tt.wantErr, err)
			}
			// whitespace only returns nil, which is expected
			if tt.name == "whitespace only" && result != nil {
				t.Fatal("expected nil for whitespace only")
			}
		})
	}
}

func TestValuesEqual(t *testing.T) {
	tests := []struct {
		name     string
		a        json.RawMessage
		b        json.RawMessage
		expected bool
	}{
		{
			name:     "both empty",
			a:        nil,
			b:        nil,
			expected: true,
		},
		{
			name:     "one empty",
			a:        []byte(`"test"`),
			b:        nil,
			expected: false,
		},
		{
			name:     "equal strings",
			a:        []byte(`"test"`),
			b:        []byte(`"test"`),
			expected: true,
		},
		{
			name:     "different strings",
			a:        []byte(`"test1"`),
			b:        []byte(`"test2"`),
			expected: false,
		},
		{
			name:     "equal objects different order",
			a:        []byte(`{"a":1,"b":2}`),
			b:        []byte(`{"b":2,"a":1}`),
			expected: true,
		},
		{
			name:     "different objects",
			a:        []byte(`{"a":1}`),
			b:        []byte(`{"a":2}`),
			expected: false,
		},
		{
			name:     "equal arrays",
			a:        []byte(`[1,2,3]`),
			b:        []byte(`[1,2,3]`),
			expected: true,
		},
		{
			name:     "different arrays",
			a:        []byte(`[1,2,3]`),
			b:        []byte(`[1,2,4]`),
			expected: false,
		},
		{
			name:     "invalid JSON a",
			a:        []byte(`{invalid}`),
			b:        []byte(`{"a":1}`),
			expected: false,
		},
		{
			name:     "invalid JSON b",
			a:        []byte(`{"a":1}`),
			b:        []byte(`{invalid}`),
			expected: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := valuesEqual(tt.a, tt.b)
			if result != tt.expected {
				t.Fatalf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestConfidenceForSource(t *testing.T) {
	tests := []struct {
		source   spec.SemanticSource
		expected string
	}{
		{spec.SemanticSourcePlatformReview, "high"},
		{spec.SemanticSourceSDKExplicit, "high"},
		{spec.SemanticSourceOpenAPIRest, "low"},
		{"unknown", "low"},
	}
	for _, tt := range tests {
		result := confidenceForSource(tt.source)
		if result != tt.expected {
			t.Fatalf("source %s: expected %s, got %s", tt.source, tt.expected, result)
		}
	}
}

func assertDiagnostic(t *testing.T, diagnostics []spec.Diagnostic, code string) {
	t.Helper()
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == code {
			return
		}
	}
	t.Fatalf("diagnostic %s not found in %#v", code, diagnostics)
}
