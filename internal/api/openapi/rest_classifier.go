package openapi

import (
	"fmt"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
)

// restClassificationDiagnostic creates a diagnostic for REST classification.
func restClassificationDiagnostic(method, path string, capability spec.CapabilityKind) spec.Diagnostic {
	return spec.Diagnostic{
		Code:     "rest_capability_inferred",
		Severity: spec.SeverityInfo,
		Message:  "capability inferred from REST method/path: " + string(capability),
		Field:    "x-capability",
	}
}

// restClassificationDiagnosticWithConfidence creates a diagnostic for REST classification with confidence info.
func restClassificationDiagnosticWithConfidence(method, path string, capability spec.CapabilityKind, confidence string) spec.Diagnostic {
	return spec.Diagnostic{
		Code:     "rest_capability_inferred",
		Severity: spec.SeverityInfo,
		Message:  fmt.Sprintf("capability inferred from REST method/path: %s (confidence: %s)", string(capability), confidence),
		Field:    "x-capability",
	}
}
