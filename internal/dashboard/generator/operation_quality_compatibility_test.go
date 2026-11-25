package generator

import (
	"testing"

	"github.com/cuihairu/croupier/internal/dashboard/spec"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// SUI-011: an operation whose request the executor cannot send must not get a
// directly executable default proposal, and the degraded tier must stay
// generable because its JSON editing path is still usable. The contract-level
// findings reach the generator through OperationSpec.Diagnostics.

func sui011OperationSpec(code string) spec.OperationSpec {
	return spec.OperationSpec{
		FunctionID: "player.get",
		Operation:  "get",
		Enabled:    true,
		Diagnostics: []spec.Diagnostic{{
			Code:       code,
			Severity:   spec.SeverityWarning,
			FunctionID: "player.get",
			Field:      "$.body.content",
			Message:    "request body declares no application/json media type",
		}},
	}
}

func sui011Options() GenerateOptions {
	return GenerateOptions{
		DefaultLocale: "zh-CN",
		Functions: map[string]spec.FunctionSpec{
			"player.get": {ID: "player.get", InputSchema: spec.JSONSchema(`{"type":"object"}`)},
		},
	}
}

func TestGenerateOperationPage_UnsendableRequestNeedsReview(t *testing.T) {
	page := GenerateOperationPageForOperation(sui011OperationSpec(spec.DiagSchemaCompatibilityManual), sui011Options())
	assert.Equal(t, spec.GeneratedPageQualityNeedsReview, page.Quality)
	require.NotEmpty(t, page.Diagnostics)
	assert.Contains(t, page.Diagnostics, spec.Diagnostic{
		Code:       spec.DiagSchemaCompatibilityManual,
		Severity:   spec.SeverityWarning,
		FunctionID: "player.get",
		Field:      "$.body.content",
		Message:    "request body declares no application/json media type",
	})
}

func TestGenerateOperationPage_DegradedRequestStaysBasic(t *testing.T) {
	page := GenerateOperationPageForOperation(sui011OperationSpec(spec.DiagSchemaCompatibilityDegraded), sui011Options())
	assert.Equal(t, spec.GeneratedPageQualityBasic, page.Quality)
}

func TestGenerateOperationPage_CleanRequestStaysBasic(t *testing.T) {
	page := GenerateOperationPageForOperation(
		spec.OperationSpec{FunctionID: "player.get", Operation: "get", Enabled: true}, sui011Options())
	assert.Equal(t, spec.GeneratedPageQualityBasic, page.Quality)
}
