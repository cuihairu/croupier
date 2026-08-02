package generator

import (
	"encoding/json"
	"testing"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGenerateOperationPageGolden tests that the generator produces stable output
// for the same input. This ensures that repeated generation with the same input
// produces byte-identical results.
func TestGenerateOperationPageGolden(t *testing.T) {
	tests := []struct {
		name     string
		op       spec.OperationSpec
		opts     GenerateOptions
		wantType spec.PageType
	}{
		{
			name: "sync operation",
			op: spec.OperationSpec{
				FunctionID:  "mail.send",
				ResourceKey: "mail",
				Operation:   "send",
				Capability:  spec.CapabilityAction,
				Execution:   spec.FunctionExecutionSync,
				Enabled:     true,
			},
			opts:     DefaultGenerateOptions(),
			wantType: spec.PageTypeOperation,
		},
		{
			name: "task operation",
			op: spec.OperationSpec{
				FunctionID:  "reward.batchGrant",
				ResourceKey: "reward",
				Operation:   "batchGrant",
				Capability:  spec.CapabilityTask,
				Execution:   spec.FunctionExecutionTask,
				Enabled:     true,
			},
			opts:     DefaultGenerateOptions(),
			wantType: spec.PageTypeTask,
		},
		{
			name: "report operation",
			op: spec.OperationSpec{
				FunctionID:  "analytics.retention",
				ResourceKey: "analytics",
				Operation:   "retention",
				Capability:  spec.CapabilityReport,
				Execution:   spec.FunctionExecutionSync,
				Enabled:     true,
			},
			opts:     DefaultGenerateOptions(),
			wantType: spec.PageTypeReport,
		},
		{
			name: "approval operation",
			op: spec.OperationSpec{
				FunctionID:  "player.ban",
				ResourceKey: "player",
				Operation:   "ban",
				Capability:  spec.CapabilityAction,
				Execution:   spec.FunctionExecutionApproval,
				Risk:        spec.RiskHigh,
				Enabled:     true,
			},
			opts:     DefaultGenerateOptions(),
			wantType: spec.PageTypeOperation,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Generate twice to verify stability
			result1 := GenerateForOperation(tt.op, tt.opts)
			result2 := GenerateForOperation(tt.op, tt.opts)

			// Verify type
			assert.Equal(t, tt.wantType, result1.Type)

			// Verify stability - byte-identical JSON
			json1, err := json.Marshal(result1)
			require.NoError(t, err)

			json2, err := json.Marshal(result2)
			require.NoError(t, err)

			assert.Equal(t, string(json1), string(json2),
				"generator should produce identical output for same input")

			// Verify required fields
			assert.NotEmpty(t, result1.PageKey)
			assert.NotEmpty(t, result1.Title)
			assert.NotEmpty(t, result1.Bindings)
			assert.NotNil(t, pageShape(result1.PageSpec))
		})
	}
}

// TestGenerateResourcePageGolden tests resource page generation stability.
func TestGenerateResourcePageGolden(t *testing.T) {
	// This test requires CapabilitySemantics which is not yet available
	// in the test environment. Skip for now.
	t.Skip("requires CapabilitySemantics")

	// TODO: Add golden test for resource page generation when
	// CapabilitySemantics is available in test fixtures
}

// TestGeneratePageKeyStability tests that page keys are deterministic.
func TestGeneratePageKeyStability(t *testing.T) {
	tests := []struct {
		name string
		op   spec.OperationSpec
		opts GenerateOptions
		want string
	}{
		{
			name: "with resource and operation",
			op: spec.OperationSpec{
				FunctionID:  "player.ban",
				ResourceKey: "player",
				Operation:   "ban",
			},
			opts: GenerateOptions{DefaultLocale: "zh-CN"},
			want: "player.ban",
		},
		{
			name: "with only function id",
			op: spec.OperationSpec{
				FunctionID: "mail.send",
			},
			opts: GenerateOptions{DefaultLocale: "zh-CN"},
			want: "mail.send",
		},
		{
			name: "with prefix",
			op: spec.OperationSpec{
				FunctionID:  "player.ban",
				ResourceKey: "player",
				Operation:   "ban",
			},
			opts: GenerateOptions{DefaultLocale: "zh-CN", PageKeyPrefix: "gm."},
			want: "gm.player.ban",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateForOperation(tt.op, tt.opts)
			assert.Equal(t, tt.want, result.PageKey)
		})
	}
}

// TestQualityAssessmentGolden tests quality assessment stability.
func TestQualityAssessmentGolden(t *testing.T) {
	tests := []struct {
		name    string
		op      spec.OperationSpec
		wantQ   spec.GeneratedPageQuality
		wantErr bool
	}{
		{
			name: "complete operation - basic quality",
			op: spec.OperationSpec{
				FunctionID:  "player.ban",
				ResourceKey: "player",
				Operation:   "ban",
				Capability:  spec.CapabilityAction,
				Execution:   spec.FunctionExecutionSync,
				Risk:        spec.RiskHigh,
				Permission:  "player.ban.invoke",
				Enabled:     true,
			},
			wantQ: spec.GeneratedPageQualityBasic,
		},
		{
			name: "task without semantics - needs_review",
			op: spec.OperationSpec{
				FunctionID:  "reward.batchGrant",
				ResourceKey: "reward",
				Operation:   "batchGrant",
				Capability:  spec.CapabilityTask,
				Execution:   spec.FunctionExecutionTask,
				Enabled:     true,
			},
			wantQ: spec.GeneratedPageQualityNeedsReview,
		},
		{
			name: "report without semantics - needs_review",
			op: spec.OperationSpec{
				FunctionID:  "analytics.retention",
				ResourceKey: "analytics",
				Operation:   "retention",
				Capability:  spec.CapabilityReport,
				Enabled:     true,
			},
			wantQ: spec.GeneratedPageQualityNeedsReview,
		},
		{
			name: "disabled function - blocked",
			op: spec.OperationSpec{
				FunctionID: "player.ban",
				Enabled:    false,
			},
			wantQ: spec.GeneratedPageQualityBlocked,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateForOperation(tt.op, DefaultGenerateOptions())
			assert.Equal(t, tt.wantQ, result.Quality)
		})
	}
}

// TestBindingStability tests that bindings are deterministic.
func TestBindingStability(t *testing.T) {
	op := spec.OperationSpec{
		FunctionID:  "player.ban",
		ResourceKey: "player",
		Operation:   "ban",
		Capability:  spec.CapabilityAction,
		Execution:   spec.FunctionExecutionSync,
		Enabled:     true,
	}

	// Generate multiple times
	for i := 0; i < 10; i++ {
		result := GenerateForOperation(op, DefaultGenerateOptions())

		// Verify binding structure
		require.Len(t, result.Bindings, 1)
		binding := result.Bindings[0]

		assert.NotEmpty(t, binding.ID)
		assert.Equal(t, "player.ban", binding.FunctionID)
		assert.Equal(t, spec.BindingUsageAction, binding.Usage)
		assert.Equal(t, spec.PageExecutionModeSync, binding.Execution.Mode)
	}
}

// TestPageSpecStability tests that generated PageSpec output is deterministic.
func TestPageSpecStability(t *testing.T) {
	op := spec.OperationSpec{
		FunctionID:  "mail.send",
		ResourceKey: "mail",
		Operation:   "send",
		Capability:  spec.CapabilityAction,
		Execution:   spec.FunctionExecutionSync,
		Enabled:     true,
	}

	// Generate multiple times
	var specs []string
	for i := 0; i < 10; i++ {
		result := GenerateForOperation(op, DefaultGenerateOptions())
		raw, err := json.Marshal(result.PageSpec)
		require.NoError(t, err)
		specs = append(specs, string(raw))
	}

	for i := 1; i < len(specs); i++ {
		assert.Equal(t, specs[0], specs[i],
			"PageSpec should be stable across multiple generations")
	}
}

func pageShape(page spec.PageSpec) interface{} {
	switch page.Type {
	case spec.PageTypeResource:
		return page.Resource
	case spec.PageTypeOperation:
		return page.Operation
	case spec.PageTypeTask:
		return page.Task
	case spec.PageTypeReport:
		return page.Report
	default:
		return nil
	}
}
