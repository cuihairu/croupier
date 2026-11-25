package functioncontract

import (
	"strings"
	"testing"

	"github.com/cuihairu/croupier/internal/validation/validationtest"
)

// SUI-016: the direct function invoke entry asserts the shared fixture table
// verbatim — the same schema and payload must yield the same verdict here as
// on the console execution entry.
func TestValidateInputSharedFixtures(t *testing.T) {
	for _, fixture := range validationtest.SchemaValidationFixtures() {
		t.Run(fixture.Name, func(t *testing.T) {
			schema := fixture.Schema
			err := ValidateInput(schema, []byte(fixture.Payload))
			switch {
			case fixture.WantDefect:
				if err == nil {
					t.Fatalf("expected defect rejection, got nil")
				}
				if schema != nil && len(schema) == 0 {
					t.Fatalf("defect fixtures must declare a schema")
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
