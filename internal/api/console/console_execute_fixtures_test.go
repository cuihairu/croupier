package console

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/validation/validationtest"
)

// SUI-016: the console execution entry asserts the shared fixture table
// verbatim — the same schema and payload must yield the same verdict here as
// on the direct function invoke path.
func TestValidateBindingExecutePayloadSharedFixtures(t *testing.T) {
	for _, fixture := range validationtest.SchemaValidationFixtures() {
		t.Run(fixture.Name, func(t *testing.T) {
			binding := spec.PageFunctionBinding{
				ID:         "b-" + fixture.Name,
				FunctionID: "fn",
			}
			functions := map[string]spec.FunctionSpec{}
			if fixture.Schema != nil {
				functions["fn"] = spec.FunctionSpec{
					ID:          "fn",
					InputSchema: spec.JSONSchema(fixture.RawSchema(t)),
				}
			}
			err := validateBindingExecutePayload(json.RawMessage(fixture.Payload), binding, functions)
			switch {
			case fixture.WantDefect:
				if err == nil {
					t.Fatalf("expected defect rejection, got nil")
				}
			case fixture.WantPass:
				if err != nil {
					t.Fatalf("expected pass, got %v", err)
				}
			default:
				if err == nil {
					t.Fatalf("expected violation, got nil")
				}
				if fixture.WantField != "" && !strings.Contains(err.Error(), fixture.WantField) {
					t.Fatalf("expected field %s in message, got %v", fixture.WantField, err.Error())
				}
			}
		})
	}
}
